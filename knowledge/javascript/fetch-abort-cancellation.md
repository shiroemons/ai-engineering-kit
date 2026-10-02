---
{
  "id": "javascript-fetch-abort-cancellation",
  "title": "JavaScript fetch cancellation with AbortController and AbortSignal timeout scope and cleanup",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "WHATWG DOM snapshot b2e32dc (2026-09-24) + Fetch snapshot 357bd98 (2026-09-21); Chrome 154 release statement (2026-09-22); MDN references rechecked 2026-10-02",
  "tags": [
    "research-domain:frontend",
    "fetch",
    "AbortController",
    "AbortSignal",
    "AbortError",
    "TimeoutError",
    "timeout",
    "cancellation",
    "signal",
    "cleanup",
    "removeEventListener",
    "throwIfAborted",
    "reason",
    "dependent-signal",
    "Chrome-154",
    "ReadableStream",
    "garbage-collection"
  ],
  "sources": [
    {
      "id": "whatwg-dom-abort-reason-20261002",
      "url": "https://dom.spec.whatwg.org/commit-snapshots/b2e32dc730eb0dc0cce1a431393fe4a17fda1d54/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-fetch-abort-reason-20261002",
      "url": "https://fetch.spec.whatwg.org/commit-snapshots/357bd98924d94b81fbe8608192a2ee1f123b82f4/",
      "type": "official_docs"
    },
    {
      "id": "chrome-154-fetch-abort-reason-20261002",
      "url": "https://developer.chrome.com/release-notes/154",
      "type": "release_notes"
    },
    {
      "id": "mdn-abort-signal-cleanup-20261002",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal",
      "type": "official_docs"
    },
    {
      "id": "mdn-abort-signal-any-20261002",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal/any_static",
      "type": "official_docs"
    },
    {
      "id": "mdn-abort-signal-timeout-20261002",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal/timeout_static",
      "type": "official_docs"
    },
    {
      "id": "mdn-window-fetch-errors-20261002",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/Window/fetch",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# JavaScript fetch cancellation with AbortController and AbortSignal timeout scope and cleanup

## 問いと結論

`AbortSignal.any()` で利用者の取り消しと timeout を合成すると、理由は失われるか。`fetch()` が解決した後の body 読み取りまで、何を取り消せるか。

**合成後も選ばれた abort reason は保持される。** 標準の `TimeoutError` と既定の `AbortError` を区別できる構成はある。ただし reason の分類と、原因となった入力 signal の識別は別問題である。以下は DOM / Fetch の規範契約、Chrome の実装リリース、独自の利用判断を分けた訂正であり、`any()` の新機能として紹介するものではない。

## DOM の契約: reason を保持する dependent signal

[WHATWG DOM §3 の固定 snapshot](https://dom.spec.whatwg.org/commit-snapshots/b2e32dc730eb0dc0cce1a431393fe4a17fda1d54/#interface-AbortSignal) で確認した。

- `abort()` の理由が省略または `undefined` なら `AbortError` の `DOMException`。`abort(reason)` には任意の JavaScript 値を渡せる。`AbortSignal.timeout()` は期限到来時に `TimeoutError` を理由にする
- `any(signals)` は dependent signal を作る。生成時に既に abort 済みの入力があれば、**入力列挙順で最初のもの**の reason を採用する。過去に実際に abort した時刻の比較ではない
- 未 abort の入力から合成した場合、最初に合成 signal を abort させる入力の reason がそのまま代入される。DOM 内のこの代入で clone や名前の正規化は行われず、後の abort で上書きされない
- `signal.throwIfAborted()` はその reason を throw する。abort 状態は元に戻せない。一方、まだ abort していない同じ signal で複数処理をまとめて取り消すことはできる。「単回性」は一回の fetch にしか渡せないという意味ではない

[MDN any() 個別ページ](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal/any_static) も reason の保持を説明する。一方、[MDN AbortSignal 総合ページ](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal#examples) の合成例直後には timeout の判別を否定する注記が残っている。これを「合成時に理由が必ず消える」と一般化せず、規範算法と個別ページを根拠に判断する。

### reason の分類と source identity の境界

以下は上記算法からの導出である。

- 自分が管理する `controller.abort()` と `AbortSignal.timeout()` だけなら、合成 signal の `reason.name` は先に選ばれた方に応じて `AbortError` / `TimeoutError` になる
- `any()` は公開の winning-source 属性を持たない。`abort(new DOMException(..., "TimeoutError"))` も可能なので、名前だけでは「本物の timer が原因」と証明できない。文字列・`null` などの custom reason に `name` があるとも限らない
- 入力ごとに一意な理由を自分で管理し、同一 realm の DOM signal 間で `Object.is(combined.reason, source.reason)` を比較する設計は可能。ただし同じ値・オブジェクトを複数入力で再利用すると曖昧になる
- `catch` が実行されるまでに複数の入力が abort する場合がある。そこでの `timeoutSignal.aborted` だけを見て先着を推定しない。入力の発火履歴が必要なら別途記録し、生成前に abort 済みの入力の順序も定義する

## Fetch の契約: promise と body は別の完了段階

[WHATWG Fetch §5.4・§5.6 の固定 snapshot](https://fetch.spec.whatwg.org/commit-snapshots/357bd98924d94b81fbe8608192a2ee1f123b82f4/#fetch-method) が規範の根拠である。

`fetch(url, { signal })` は `RequestInit.signal` を受け取り、Request はそれに依存する signal を作る。正常に Request を構築した後、既に abort 済みなら取得処理を開始せず理由で拒否する。Request 構築自体が失敗する場合はその例外で拒否するため、常に abort 理由が優先されるとは限らない。

`Promise<Response>` の解決は body 全体の読了を意味しない。local abort の手順は未確定の fetch promise を reason で拒否し、response body が non-null かつ readable なら同じ reason で stream を error にする。既に解決した fetch promise の結果は変わらず、完了済みまたは null の body を遡って失敗にはしない。body の読取中に abort すれば、その読取が拒否され得る。

[Fetch §2 の直列化手順](https://fetch.spec.whatwg.org/commit-snapshots/357bd98924d94b81fbe8608192a2ee1f123b82f4/#fetch-controller) は別の境界である。理由の `StructuredSerialize` が失敗すれば既定の `AbortError` に fallback する。復元失敗等にも fallback がある。従って、DOM 内での reason の同一性を、直列化経路を含むすべての fetch/body 拒否値のオブジェクト同一性へ拡張しない。

### HTTP status と拒否を混同しない

[MDN Window.fetch()](https://developer.mozilla.org/en-US/docs/Web/API/Window/fetch) の説明どおり、HTTP 4xx / 5xx だけでは promise は拒否されない。`Response.ok` / `Response.status` を用途に合わせて確認する。

拒否理由を network 失敗・不正 URL・block・既定 `AbortError` だけに限定しない。MDN は不正な RequestInit 値も挙げ、Fetch は Request 構築例外を引き継ぐ。custom abort reason もあり、続く `response.json()` 等には読み取り・変換の失敗もある。`TypeError` だけでネットワーク障害や未対応 API と断定する分岐は不十分である。

## Chrome 154 の実装差分

[Chrome 154 release notes](https://developer.chrome.com/release-notes/154) は stable 日付を **2026-09-22** とし、開発者が渡した abort reason を fetch promise だけでなく **Response のメソッドと ReadableStream** へ伝播する変更を記載する。これは Fetch 標準への整合を述べるリリース情報であり、DOM `any()` の理由選択規則の変更ではない。

このため `catch` の `err.name === "AbortError"` だけで全取り消しを捕捉する実装は、custom reason や timeout を見落とす。Chrome のこの変更を他ブラウザや旧版の保証には使わない。リリース記述は確認したが、Chrome 153/154 の実機比較は行っていない。

## GC と listener cleanup の正確な条件

[DOM timeout / garbage collection](https://dom.spec.whatwg.org/commit-snapshots/b2e32dc730eb0dc0cce1a431393fe4a17fda1d54/#garbage-collection) と [MDN の cleanup 説明](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal#removing_the_abort_event_listener) に基づく。

- timeout signal の明示的な強参照条件は「timeout が継続中 **かつ** abort listener がある」である
- dependent signal の回収禁止条件は「未 abort **かつ** source signals が空でない **かつ**（abort listener または内部 abort algorithm がある）」である
- controller 由来の signal も、listener が付いているだけで永久に保持されるわけではない。MDN の説明は controller と signal の両方が unreachable になった場合であり、到達可能なオブジェクトまで回収可能とする意味ではない

`{ once: true }` はイベントが実際に発火したときの解除であり、正常完了時には自作 listener が残る。自分で登録したものは処理終了時に `removeEventListener` する。body を読むだけでは自作 listener は消えない。逆に解除しただけで fetch 内部の abort 処理や timer の停止、即時 GC まで保証されるわけではない。

[MDN timeout()](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal/timeout_static) は timeout 自体を取り消す API がなく、操作の早期完了や合成相手の abort でも timer が取り消されないことを説明する。[MDN any()](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal/any_static) によれば合成 signal の abort は他の入力も abort させない。

## 利用判断と最小例

以下は公式仕様の追加要件ではなく、この契約からの独自の設計判断である。

- 取り消し単位ごとに controller を所有する。独立した再試行では新しいものを使い、意図した共同取り消しには未 abort の signal を共有する
- 利用者の abort と timeout を合成するだけなら `any()` を使える。「理由が消えるから」という理由で自前 timer へ置き換える必要はない
- `timeout()` は **active time** 基準で、suspended worker や bfcache では進行が止まる。壁時計の厳密な deadline と同一視しない。timer を早期に確実に解除する必要があるときは `setTimeout` / `clearTimeout` を検討する。この代替で停止中の実行が保証されるわけではない
- body の利用までを同じ cancellation scope に入れる。使用しない body の終了方針も決めるが、GC を約束するために巨大な body を全量読むような設計は避ける

次は本文用に作成した例。小さな text response を全量読み、その後に HTTP status を確認する方針を採る。任意のサイズのダウンロード向け汎用 wrapper ではない。呼出側が渡す `onAbort` は例外を投げない軽量な観測処理とする。

```js
async function fetchText(url, userSignal, onAbort) {
  const deadline = AbortSignal.timeout(5000);
  const signal = AbortSignal.any([userSignal, deadline]);
  signal.throwIfAborted();
  signal.addEventListener("abort", onAbort, { once: true });
  try {
    const response = await fetch(url, { signal });
    const text = await response.text();
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    return text;
  } finally {
    signal.removeEventListener("abort", onAbort);
  }
}
```

`await response.text()` を `try` の中で待つことで、body の成功・失敗まで自作 listener の寿命を揃える。例は理由を `AbortError` に置換せず拒否値を呼出側へ返す。実装差がある環境で `catch` の値と `signal.reason` が一致するかは別途確認する。`signal.aborted` が true という事実だけで、直前の network / HTTP / parse failure を取り消しへ分類し直さない。

## 版・provenance・確認範囲

- DOM は 2026-09-24 更新の snapshot `b2e32dc730eb0dc0cce1a431393fe4a17fda1d54`、Fetch は 2026-09-21 更新の snapshot `357bd98924d94b81fbe8608192a2ee1f123b82f4`。規範算法の参照先を固定し、2026-10-02 UTC に再取得した
- Chrome 154 は 2026-09-22 の stable release statement。ページ更新日も同日。一般の仕様書と区別して `release_notes` とした
- MDN の AbortSignal / any() / timeout() は 2026-09-01 更新、Window.fetch() は 2025-12-16 更新。Mozilla Contributors の各ページを 2026-10-02 に開き直した。長い引用・コードの転載はなく、上の例は独自作成
- WHATWG の本文ライセンスは **CC-BY-4.0**、ソースコードに取り込む部分は **BSD-3-Clause**。Chrome ページは本文 CC-BY-4.0 / サンプル Apache-2.0。MDN 本文は CC-BY-SA-2.5-or-later。旧 catalog の CC0 記載や誤った要約は履歴として保存するが、現行本文の根拠には使用しない
- 取得日から release notes の TTL 30日を適用し、再確認期限は **2026-11-01**。DOM/Fetch の説明がこの日に無効になるという意味ではない
- 未確認: ブラウザ別の全対応版、Service Worker / cross-realm の実装比較、GC の実測、React lifecycle との組合せ。本稿の検索 eval は検索到達性を検証するもので、ブラウザ挙動のテストではない

実機検証では、生成前の abort、入力順、timeout 先行、custom reason、headers 前と body 読取中の abort、body 完了後の abort、正常完了時の自作 listener 解除を別ケースにする。

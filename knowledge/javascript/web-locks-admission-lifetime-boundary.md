---
{
  "id": "javascript-web-locks-admission-lifetime-boundary",
  "title": "Web Locks: 待機取消・callback寿命・steal・storage bucketの境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "Web Locks W3C Working Draft 2025-09-24; Storage Living Standard updated 2026-10-19; WPT 5af3c135c9c97381733866150a7ada27d1029c50; verified 2026-10-24 UTC; browser runtime unverified",
  "tags": [
    "research-domain:frontend",
    "Web-Locks",
    "navigator.locks",
    "AbortSignal",
    "callback",
    "waiting-promise",
    "released-promise",
    "steal",
    "ifAvailable",
    "storage-bucket",
    "partitioning",
    "cross-tab"
  ],
  "sources": [
    {
      "id": "w3c-web-locks-lifetime-wd20250924-20261024",
      "url": "https://www.w3.org/TR/2025/WD-web-locks-20250924/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-storage-web-locks-scope-20261024",
      "url": "https://storage.spec.whatwg.org/",
      "type": "official_docs"
    },
    {
      "id": "wpt-web-locks-lifetime-5af3c135-20261024",
      "url": "https://github.com/web-platform-tests/wpt/tree/5af3c135c9c97381733866150a7ada27d1029c50/web-locks",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2027-01-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Web Locksの取得待ちと実行中の仕事を分ける

## 問いと採用範囲

複数tabで同じキャッシュ更新や同期処理を重ねないために `navigator.locks.request()` を使うとき、待機timeoutは実行中の仕事も止めるか。`steal: true` で旧tabを安全に追い出せるか。結論はどちらも否定である。Web Locksは、同じlock managerに参加する処理の**入場と保持期間を調整する協調API**であって、JavaScript実行の強制中断や外部資源の更新権限を提供するものではない。

2026-10-24 UTCに [Web Locksの固定版][spec]、current TR、Editor's Draftを開き、いずれも表示日付が **2025-09-24** であることを確認した。固定版はW3C Working Draftであり最終勧告ではない。本稿は2026年の新機能の紹介ではなく、未収録のブラウザ内の取得・寿命境界を補う。実装対応版や全ブラウザの一致は認定しない。

[IndexedDBのtransaction完了](indexeddb-transaction-completion-durability-boundary.md)、[Workerへのbuffer所有権移動](worker-transfer-ownership-retry-boundary.md)、[etcdの資源側fencing](../distributed-systems/etcd-lease-fencing-resource-boundary.md)とは別の問題を扱う。Web Locksの取得成功を、それらのcommit・配送・fencingの証拠へ読み替えない。

## 最初に区別する観測

| 観測 | 判断できること | 判断できないこと |
|---|---|---|
| `request()` がPromiseを返す | 要求の結果を受け取るPromiseを得た | lock取得済み、callback開始済み |
| callbackに非nullの `Lock` が渡る | その要求の取得callbackが開始した | 仕事の完了、将来のstealからの保護 |
| callbackが返したPromiseがsettleする | 通常の保持期間を終了する契機 | detached処理の終了、失敗前の副作用のrollback |
| `request()` のPromiseが通常fulfillする | 通常取得なら解放後の結果。null時は未取得callbackの結果 | `ifAvailable` で実際に取得したこと、データの永続化 |
| 旧 `request()` がstealによりrejectする | 旧lockが保持集合から除かれた | 旧callback・そのfetch・書込みが停止したこと |
| `query()` に `held` / `pending` が出る | ある時点の診断snapshot | 次の処理時点の所有権、取得の予約 |

根拠は [仕様 §2.4、§3.2、§4.1–4.4][spec]。`ifAvailable` で取得できない場合にもcallbackが `null` で呼ばれ、その戻り値は要求の結果になる。[固定WPT ifAvailable][ifavailable] も、この場合の値の返却とcallback例外による拒否を別々に検査している。単に `await request()` が成功したことだけを「排他処理を実施した」と記録しない。

## 保持期間はcallbackが返すPromiseに接続する

仕様は、callbackの結果に追従する **waiting promise** と、呼出元へ返す **released promise** を区別する。通常は前者のfulfill / reject後にlockが解放され、後者が同じ結果に追従する。`async` キーワードがあるかではなく、**保護したい仕事の終了が返却Promiseに含まれるか**をレビューする。

[固定WPT held][held] は、非async callbackから返したPromiseでも、resolve / rejectまで次の要求を待たせるケースを持つ。仕様 §3.2.1の「non-async callback」という略記だけを読んで、非asyncなら必ず即解放されると解釈しない。§2.4と§4.4のPromiseの扱いを根拠にする。

次は独自の対照例。`updateDraft()` は、自身の管理する読み取り・書込み・後始末がすべて終了してからsettleするPromiseを返す前提。ブラウザ実行は未検証である。

```js
// 誤り: callbackはundefinedを返し、更新を保持期間から切り離す。
await navigator.locks.request("draft:42", () => {
  void updateDraft();
});

// 仕事の終了をcallbackの結果に接続する。asyncでも同じ考え方。
await navigator.locks.request("draft:42", () => updateDraft());
```

この契約からの設計上の帰結は次のとおり。

- callbackが始まる前に `updateDraft()` を呼んでPromiseだけを渡しても、先に始まった副作用は保護されない。保護対象を開始する位置もcallback内に置く。
- `setTimeout()`、event listener、`void asyncWork()`、待たない `forEach(async ...)` は、callbackの返却Promiseへ自動参加しない。`Lock` オブジェクトを変数に保存しても保持期間を延ばせない。
- callback内の `Promise.race([work, timeout])` がtimeout側でsettleすれば、workが継続中でも解放へ進む。呼出側で `request()` とtimeoutをraceしただけなら、そもそも待機要求の取消にもならない。
- 複数の仕事を `Promise.all()` で束ねても、一件のrejectで他の仕事が止まるわけではない。途中失敗があり得る並列処理は、必要な取消を通知し、開始済みの仕事の終了を `Promise.allSettled()` 等で確認してからcallbackを終える設計を検討する。終了待ちと「全件成功」の判定は分ける。

最後の二項はWeb Locks固有の追加機能ではなく、Promiseの終了と処理実体の終了を混同した場合の帰結である。終了しない仕事が残れば待ち続けるため、取消協調のない処理へ無条件にdrainを導入すればよいわけでもない。lockを早く解放することと、旧処理を安全に止めることの両方が必要なら、仕事自体の停止契約を設計する。

## signalは取得待ちの取消。実行中の取消は別の責務

`request(name, { signal }, callback)` の `signal` は、まだ実行へ入っていない要求を取り消すために使う。**callback開始後にabortしても、Web Locksがcallbackを中断したりlockを解放したりはしない。** その時点ではcallback結果のsettlementが通常の解放経路である。

[固定WPT signal][signal] は、取得callbackに入ってからabortし、その後callbackのPromiseをresolveすると、外側のPromiseも通常の値でfulfillするケースを持つ。一方、取得可能な空のlockでも、`request()` 直後に同期的にabortするケースではcallbackを実行しない。単に競合がないことを「既に取得した」判定に使わない。仕様 §4.4はcallbackを呼ぶ直前にもsignalを確認し、取消アルゴリズムを外してからcallbackを呼ぶ。

### AbortErrorという名前だけへ正規化しない

固定版のsignal説明には `AbortError` という略記があるが、規範手順 §3.2.1の既abort検査と§4.3は **signalのabort reason** を拒否値にする。[固定WPT signal][signal] は既abort・待機取消の両方でcustom reasonの一致を検査する。アプリが文字列や独自値を理由にすれば、拒否値が常に `DOMException` や `AbortError` とは限らない。

そのため、`catch (error)` を名前だけで「待機timeout」「仕事の失敗」に分類し直さない。要求したoperation、待機段階、callback開始、既知の取消理由を区別して記録する。fetch側のsignalとbody読取りは [既存のFetch cancellation文書](fetch-abort-cancellation.md) の責務であり、lockのsignalを渡しただけでfetchにも自動伝播するわけではない。

### 独自例: 待機用timerをcallback開始時に片付ける

以下は説明用の最小helper。通常のexclusive要求のみを扱い、全参加者が `steal` を使わないことを前提にする。`waitMs` はアプリが検証した正の小さな待機予算、`work` は終了までのPromiseを返す関数。timerの遅延実行があり得るため、厳密な壁時計deadlineではない。

```js
async function withQueueBudget(name, waitMs, work) {
  const waiting = new AbortController();
  const reason = new DOMException("Lock queue budget exceeded", "TimeoutError");
  const timer = setTimeout(() => waiting.abort(reason), waitMs);
  try {
    return await navigator.locks.request(
      name,
      { signal: waiting.signal },
      async () => {
        clearTimeout(timer);
        return await work();
      },
    );
  } finally {
    clearTimeout(timer);
  }
}
```

外側の `return await` により、取得前の `finally` でtimerを消してしまわない。callback開始時に待機timerを解除し、取得前の拒否でも後始末する。このhelperは**待機予算だけ**を扱い、実行時間上限・手動キャンセル・再試行・互換性fallbackは実装しない。

仕事も取り消したいなら、別に所有するwork用signalを `work` が使うAPIへ明示的に渡す。単に仕事のtimeoutでcallbackのPromiseをrejectさせて、実体の終了を待たずに次へ進めない。仕事のcleanupまで待つ設計は、外部に送信済みの副作用の取消保証とは別である。

## stealは旧callbackへの例外注入ではない

仕様 §4.1は `steal: true` の要求について、同名の保持中lockを除き、その **released promiseをAbortErrorでreject** し、新要求をqueueの先頭へ置く。既存の待機要求を一括削除する手順ではない。[固定WPT steal][steal] は旧要求のreject、新要求の優先、待機要求が後で取得できることを検査する。

重要なのは、**旧callbackのwaiting promiseを強制終了する手順ではない**こと。仕様自身も、旧コードとstealした側のコードが同時に資源を使い得ると警告している。例えばAがcallback内で仕事を待っている間にBがstealすると、Aの呼出元はrejectを観測しても、Aのcallbackが後で再開して書き込む可能性が残る。

次はこの契約からの設計判断である。

- 旧callback内の `catch` / `finally` が、外側の要求のrejectによって直ちに走るとは期待しない。JavaScript threadの割込みでも、旧fetchのabortでもない。
- `query()` で長く見えたlockや一定時間応答しないtabを、stealして安全に交代できるとみなさない。snapshotは障害判定や停止確認の証拠にならない。
- 通常の二重更新防止では `steal` を使わず、待機取消、明示的な協調停止、ユーザーに見える競合状態を先に設計する。stealは旧処理が継続しても正しさを失わない回復手順が別にある場合だけ検討する。
- サーバー更新の受理条件、version照合、重複防止が必要ならサーバー側で成立させる。Web Locksの `name`・`clientId`・`Lock` はサーバーが検証するfencing tokenではない。

オプションも自由な合成ではない。[固定WPT acquire][acquire] と規範手順では、`signal` と `steal: true`、`signal` と `ifAvailable: true`、`steal: true` と `ifAvailable: true` は組合せ不可。stealはexclusiveのみで、`shared` と併用できない。「timeout付きsteal」を同じoptionsへ足して実装しない。

## queryの事前確認ではなくrequestで入場を判定する

`query()` は診断snapshotで、返却までに状態が変わり得る。`held` が空なら直接書き込む、というcheck-then-actを作らない。待てるなら通常のrequest、待たずに諦めるなら `ifAvailable: true` を使い、callbackの `lock === null` を明示的な「実施せず」として扱う。

同じclientでも同名のexclusive lockを再取得できるわけではない。[固定WPT held][held] は自分が保持中のlockに対する `ifAvailable` がnullになるケースを持つ。外側のcallbackが内側の通常requestをawaitすると自己待ちになり得る。独自の推奨は、取得を入口一箇所へ集めて内部処理へ渡すことと、複数lockが必要なら全呼出経路で取得順序を統一すること。lock保持中に無関係なユーザー入力や長い外部待ちを増やすと、他tabの待ち時間も増える。

## 同一originだけでscopeを説明し切らない

Web Locks §2.1–2.2は、resource nameとlock managerを **同じuser agentのstorage bucket** に対応づける。協調範囲はagent clusterをまたぎ得る一方、別profileやprivate browsing sessionを一つのlock空間とは扱わない。§6.1はstorageがfirst-party / third-party等でpartitionされるならlockingも対応させるよう求める。「同じorigin文字列ならどこでも同じlock」という要約は十分でない。

[Storage Standard][storage] は **2026-10-19更新**。storage key、shelf、bucket、API別bottleというモデルを持つ。ただし取得時の本文ではstorage keyはoriginのtupleで将来のpartitioning変更に言及し、bucket mapもdefaultのみと記す。Web Locks側にもStorageとの統合を精緻化する未解決記述が残る。これらを、全ブラウザのpartition keyやnamed Storage Buckets APIが確定・配備済みという証拠にはしない。

固定WPTでは次のテストを**静的に読んだ**。

- [partitioned-web-locks.tentative.https.html][partitioned] は、第三者site内の同origin iframeとtop-level、およびcross-site ancestorを持つ入れ子iframeが別に同名lockを取得するケースを対象にする。
- [storage-buckets.tentative.https.any.js][buckets] は、defaultとnamed bucket、異なるnamed bucket、同じnamed bucketを再openしたhandleのlock共有を比較する。

どちらもファイル名に `tentative` があり、本調査では未実行。期待値が書かれたテストの存在を、Chrome・Firefox・Safariの実測結果や一律のsupport statementへ置き換えない。通常の `navigator.locks` を説明するために、利用者へnamed bucket APIの採用を要求するものでもない。

アプリの設計上は、同じ業務資源へ触る全経路が、同じscope・同じname・適切なmodeで協調する必要がある。resource nameはアプリのlabelであり、実際のIndexedDB storeやファイルを自動的にlockしない。`shared` を指定しても書込みAPIがread-onlyになるわけではない。別bucket・別profile・別deviceやlockを使わない経路の競合は、別に扱う。

## 受入試験案と未確認事項

以下は導入時に行う**未実行の試験案**。callback開始と終了を制御できるPromiseを使い、単に一定時間sleepした結果だけで排他の成立を判定しない。

1. Aのcallback結果を未確定にしてBを要求する。Aのresolve / reject後にBが開始することと、Aが返さないdetached仕事が残る対照例を別に観測する。
2. 待機要求をcustom reasonでabortし、callback未実行を確認する。取得callback開始後にabortするケースでは、callbackの仕事を別に終了させるまで保持することを確認する。
3. callback内と呼出元の二種類の `Promise.race` を試し、timeout後の要求と仕事の残存を観測する。並列処理の一件rejectでも他の仕事が残らないか確認する。
4. Aのcallbackを残したままBがstealし、Aの外側rejectとAの内側継続を別々に記録する。この試験結果を本番でstealを安全に使える証拠にはしない。
5. `ifAvailable` のnull、同clientでの再取得、オプションの禁止組合せを検査する。queryのsnapshotから直接副作用へ進む経路がないかレビューする。
6. 必要なwindow / worker、first-party / iframe、通常 / private profileの組合せを対象ブラウザ版で試す。named bucketを使う場合だけその組合せも追加する。

本調査で実施したのは一次文書の照合と固定commitのテスト読解。実ブラウザ、WPT runner、複数tab、worker、steal後の旧処理継続の実測は未実施。browser互換性、background throttling、page freeze / bfcache、crash回復の時間上限、性能・公平性の上限、server-sideの整合性は未検証である。検索evalは知識の検索到達性を検証するもので、これらのruntime試験の代わりではない。

## 出典・provenance

- W3Cの固定版とcurrent TR / Editor's Draft、WHATWG Storageの実ページを2026-10-24 UTCに確認した。Storageの表示に対応するsnapshotは `115691c5158395b89063dee09278703f10494171`。更新日をAPIの導入日やWeb Locksの新機能日とは扱わない。
- WPTはGitHub connectorでmasterを **5af3c135c9c97381733866150a7ada27d1029c50** へ解決し、上記7ファイルとLICENSEをそのcommitで取得した。commit日時は2026-10-23T17:53:17Zであり、Web Locksの変更日を意味しない。native web経由の固定signal / stealページはcache missだったため、本文は取得に成功した固定commitのconnector応答で確認した。
- W3C文書は [Software and Document License 2023][w3c-license]、WHATWG文書は本文CC-BY-4.0 / source codeへ組み込む部分BSD-3-Clauseを [IPR Policy §7.1.1][whatwg-license] とStorageの末尾で確認。WPT contributorsの [固定LICENSE.md][wpt-license] はBSD-3-Clauseで、読んだテストheaderに別ライセンス指定はなかった。
- 本文は事実の独自の日本語要約、例と運用判断は本稿の独自作成。upstream本文・テストコードの転載、実装採用、module昇格は行わない。新規catalogのみ追加し、共有sourceの取得日は延長しない。official_docs / repository analysisの90日TTLに合わせ、再確認期限を2027-01-22とした。

[spec]: https://www.w3.org/TR/2025/WD-web-locks-20250924/
[storage]: https://storage.spec.whatwg.org/
[held]: https://github.com/web-platform-tests/wpt/blob/5af3c135c9c97381733866150a7ada27d1029c50/web-locks/held.https.any.js
[signal]: https://github.com/web-platform-tests/wpt/blob/5af3c135c9c97381733866150a7ada27d1029c50/web-locks/signal.https.any.js
[steal]: https://github.com/web-platform-tests/wpt/blob/5af3c135c9c97381733866150a7ada27d1029c50/web-locks/steal.https.any.js
[acquire]: https://github.com/web-platform-tests/wpt/blob/5af3c135c9c97381733866150a7ada27d1029c50/web-locks/acquire.https.any.js
[ifavailable]: https://github.com/web-platform-tests/wpt/blob/5af3c135c9c97381733866150a7ada27d1029c50/web-locks/ifAvailable.https.any.js
[partitioned]: https://github.com/web-platform-tests/wpt/blob/5af3c135c9c97381733866150a7ada27d1029c50/web-locks/partitioned-web-locks.tentative.https.html
[buckets]: https://github.com/web-platform-tests/wpt/blob/5af3c135c9c97381733866150a7ada27d1029c50/web-locks/storage-buckets.tentative.https.any.js
[w3c-license]: https://www.w3.org/copyright/software-license-2023/
[whatwg-license]: https://whatwg.org/ipr-policy
[wpt-license]: https://github.com/web-platform-tests/wpt/blob/5af3c135c9c97381733866150a7ada27d1029c50/LICENSE.md

---
{
  "id": "javascript-indexeddb-transaction-completion-durability-boundary",
  "title": "IndexedDB: request成功・transaction完了・非同期待ちとdurabilityの境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "IndexedDB 3.0 W3C Working Draft 2025-08-13; Chrome 121 historical durability change; idb 8.0.3 commit 77dd8bebf3669bbce9628e470a021ff63eb4acaf; verified 2026-10-05 UTC; browser runtime unverified",
  "tags": [
    "research-domain:frontend",
    "IndexedDB",
    "IDBTransaction",
    "IDBRequest",
    "complete",
    "abort",
    "durability",
    "TransactionInactiveError",
    "preventDefault",
    "idb"
  ],
  "sources": [
    {
      "id": "w3c-indexeddb3-transaction-lifecycle-20261005",
      "url": "https://www.w3.org/TR/2025/WD-IndexedDB-3-20250813/",
      "type": "official_docs"
    },
    {
      "id": "mdn-idbtransaction-lifetime-errors-20261005",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/IDBTransaction",
      "type": "official_docs"
    },
    {
      "id": "mdn-idbtransaction-commit-return-20261005",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/IDBTransaction/commit",
      "type": "official_docs"
    },
    {
      "id": "mdn-idbobjectstore-add-success-boundary-20261005",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/IDBObjectStore/add",
      "type": "official_docs"
    },
    {
      "id": "mdn-indexeddb-guide-error-recovery-review-20261005",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/IndexedDB_API/Using_IndexedDB",
      "type": "official_docs"
    },
    {
      "id": "chrome-indexeddb121-relaxed-durability-20261005",
      "url": "https://developer.chrome.com/blog/indexeddb-durability-mode-now-defaults-to-relaxed",
      "type": "maintainer_article"
    },
    {
      "id": "idb803-done-lifetime-source-20261005",
      "url": "https://github.com/jakearchibald/idb/blob/77dd8bebf3669bbce9628e470a021ff63eb4acaf/README.md",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2027-01-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# IndexedDB の保存完了・非同期待ち・耐久性を混同しない

## 問いと適用範囲

オフライン編集で `add()` の `success` を見て「保存済み」と表示してよいか。`await fetch()` の後に同じ transaction を使えるか。`await tx.commit()` や `durability: "strict"` は何を保証するか。ブラウザーのローカル保存について、要求の受付・個別操作の結果・transaction の確定・媒体への書込みを分けて判断する。

2026-10-05 UTC に確認した [W3C IndexedDB 3.0][spec] は **2025-08-13 Working Draft**。最終勧告ではなく、仕様本文だけで全ブラウザーの実装一致を保証しない。Chrome の default durability 変更は **Chrome 121** を対象とした歴史的変更で、[告知][chrome] の更新日は **2023-11-03**。本稿は2026年の新機能の紹介ではなく、既存コーパスにない保存境界の穴を埋める。

対象は通常の `readonly` / `readwrite` transaction。schema upgrade、複数tabのversion変更、容量計画、暗号化、サーバー同期プロトコルの実装は対象外。[event loop](event-loop-microtask-rendering-order.md) の一般論とは異なり、「この transaction にいつ要求を追加でき、何を成功と判定するか」を扱う。

## 最初に分ける五つの出来事

| 観測 | ここから判断できること | まだ判断できないこと |
|---|---|---|
| `store.add()` / `put()` が返った | native APIが要求を作成した | 操作成功、transaction確定 |
| `request` の `success` | 個別要求の結果を利用できる | 後続要求やcommitで失敗しないこと |
| `tx.commit()` が返った | 明示commitの開始要求が通った | commit完了。戻り値はPromiseではない |
| `tx` の `complete` | transactionのcommitが成功した | サーバー同期完了、永久保存、すべてのdurability設定で同じflush条件 |
| `tx` の `abort` | transactionが中止された | `tx.error` が必ず非nullであること |

`add()` の戻り値は `IDBRequest`。[MDN add()][add] も、個別 `success` の後にtransactionが失敗し得るため `complete` を確認するよう説明している。`complete` / `abort` の定義は [仕様 §2.7.1][lifecycle]、`commit()` の戻り値 `undefined` は [MDN commit()][commit] で確認した。

**設計判断:** 「ローカル保存済み」を表示する責任はtransaction単位の完了待ちに置く。個別requestのログは原因調査用に残しても、UIの保存成功には直結させない。「同期済み」は別の状態にする。

## transaction の寿命は async 関数の寿命ではない

[仕様の状態機械][lifecycle] と [MDN IDBTransaction][tx] では、要求を追加できるのは `active` の期間だけ。生成時と関連requestのイベント配送でその機会があり、他のtaskでは `inactive` になる。未処理要求があることと、任意のcallbackから新しい要求を出せることは同じではない。要求と結果処理が終わり、新しい要求もなければ自動commitが進む。

次の三つを分ける。

- **native request:** `await store.put(value)` と書いても、native `IDBRequest` は操作完了用Promiseではない。この書き方を完了待ちとして採用しない
- **Promise wrapper:** [idb 8.0.3 README][idb] は、自身が返すrequest Promiseを順にawaitする例を示す。すべての `await` が必ずtransactionを閉じる、と一般化しない
- **外部の非同期待ち:** 同READMEはtransactionの途中に `await fetch()` を入れる例を失敗例として区別する。HTTP応答、timer、ユーザー確認待ちをtransactionの延命手段にしない

`TransactionInactiveError` を見て同じstore handleへの再試行を繰り返しても、閉じたtransactionは復活しない。**独自の推奨手順**は次のとおり。

1. ネットワーク取得、入力検証、保存payloadの準備を先に済ませる
2. 必要なstoreだけを対象に短いtransactionを作り、`complete` / `abort` の監視を直ちに登録する
3. 既存値に依存しない書込みなら同じ同期処理で要求を登録する
4. read-modify-writeなら、そのtransactionの読取requestの `success` から同期的に書込み要求を追加する
5. 外部待ちを挟まなければ計算できない場合は、transactionを分ける。後半でrevisionを再読取・照合し、古い計算結果で新しい編集を上書きしない

5は楽観的更新の設計案であり、IndexedDBがHTTP通信まで原子的にする機能ではない。衝突時に再計算・再確認・中断のどれを選ぶかはアプリケーションの契約として決める。

## request error と terminal event の役割

[MDN IDBTransaction][tx] は、制約違反、request handlerの未捕捉例外、I/O・quota問題などのabort要因を挙げる。個別requestが一度成功しても、別requestや確定処理でtransaction全体が中止され得る。明示的な `abort()` では `tx.error` が `null` の場合もある。

### preventDefault と stopPropagation は別の操作

[仕様 §5.10][errors] のrequest errorはbubblingかつcancelable。通常のactiveなtransactionでは、配送後にeventがcancelされていなければabortへ進む。**`stopPropagation()` だけではdefault abortを解除しない。** 既知の重複等を意図的に回復するなら `preventDefault()` が関係するが、何を許容するかと後続処理を先に設計する。

取得時点の [MDN Using IndexedDB「Adding data」][guide] には、error処理に `stopPropagation()` を挙げる説明と、pending request中はactiveと読むことのできる説明がある。本稿ではそれらを「伝播停止だけで回復」「未完了ならどこからでも書ける」という根拠に採用しない。API詳細と規範アルゴリズムの条件を優先する。

**独自の推奨:** 全件保存を約束するbatchではrequest errorのdefault動作を止めず、`abort` で失敗を確定する。全errorをcatchallで `preventDefault()` する実装は、失敗したレコードを落としたまま残りだけcommitする可能性を作る。単なるログ採取と、業務上許す部分成功を分離する。

`tx.onerror` を「transactionは終了した」という通知として扱わない。回復可能なrequest errorが伝播しているだけの場合がある。失敗の原因をrequest側に記録し、terminal outcomeは `complete` / `abort` で記録すると区別しやすい。

### idb 8.0.3 の tx.done を使う場合の限定的な注意

固定commit [77dd8be…][idb-impl] の `cacheDonePromiseForTransaction` は、`complete` でresolveし、`error` または `abort` でrejectしてlistenerを解除する。**静的読解からの帰結:** request errorを `preventDefault()` で回復しても、そのerrorをtransactionへ伝播させた場合、native transactionが後で `complete` に進み得る一方で `tx.done` はすでにrejectし得る。

これはwrapperの失敗観測とnative terminal eventの違いであり、「`tx.done` のrejectは必ずrollback完了」の証拠にはしない。意図的なerror回復を行うならnative request・event伝播・個別request Promise・`tx.done` の組合せを採用版で試験する。本稿の簡略な全件保存案はerror回復を行わない。idb実装のブラウザー実測やlibrary採用を済ませたという主張ではない。

## 独自例: 全件保存のPromiseをcompleteで解決する

次はnative API用の説明例。`drafts` は `keyPath: "id"` の既存storeで、入力は事前に準備した通常の配列とplain data、全レコードの新規追加を一単位にする。外部のevent listenerはerrorのdefault動作を変更せず、この関数がtransactionを所有する前提。connection取得とschema定義は省略した。**ブラウザー実行未検証で、moduleとしての再利用を認定したコードではない。**

```javascript
function insertDraftBatch(db, drafts, durability = "default") {
  return new Promise((resolve, reject) => {
    const tx = db.transaction("drafts", "readwrite", { durability });
    let enqueueFailed = false;
    let enqueueError;

    tx.addEventListener("complete", () => resolve(), { once: true });
    tx.addEventListener("abort", () => {
      reject(enqueueFailed
        ? enqueueError
        : tx.error ?? new DOMException("Draft batch aborted", "AbortError"));
    }, { once: true });

    try {
      const store = tx.objectStore("drafts");
      for (const draft of drafts) {
        store.add(draft);
      }
    } catch (error) {
      enqueueFailed = true;
      enqueueError = error;
      tx.abort();
    }
  });
}
```

この例が選んだ方針は次のとおり。

- transaction生成時の同期例外はPromise executorのrejectになる
- `add()` の同期例外を捕まえたら、先に登録した要求も含めてabortし、`abort` の受領時に元の例外でrejectする。このcatchの中まで外部awaitも明示commitも行わない
- 非同期のrequest errorはdefault abortへ進める。重複を黙って成功に変えない
- 全件成功のresolveは `complete` だけから行う。要求数ゼロは空のtransactionの正常終了として扱う
- request成功から通信・旧データ削除等の外部副作用を起こさない。必要な副作用はこのPromiseの成功後に、別の失敗・再試行契約で実行する

入力の中にfunction等の保存できない値が混ざる同期失敗も試験する。「catchしたので安全」ではなく、すでに登録済みの先行書込みを残してよいかが判断点になる。特定のブラウザーで対応しないoptionsを無言で捨てるfallbackも、この例には含めない。

## commit() は完了待ちでも延命でもない

[MDN commit()][commit] と [仕様 §4.10][transaction-api] によれば、`commit()` はactiveなtransactionに対して確定開始を要求し、返り値は `undefined`。要求の結果イベントを待たずにcommitを開始できるが、新しい要求は受け付けなくなる。

- `await tx.commit()` の次の行を保存完了後とみなさない。待つ対象はnativeの `complete`、または採用wrapperの適切な完了Promise
- 読取requestの結果を受けてさらにwriteする予定なら、先に `commit()` して要求追加の機会を閉じない
- inactiveになったtransactionを `commit()` で救済しない。activeでなければ `InvalidStateError`
- 通常は自動commitがある。明示commitを使うなら、要求登録が終わったことを誰が保証するかを明確にする

取得時点のMDN commit例には `transaction.onsuccess` というコメントがあるが、同ページの実コードと仕様が示すtransaction完了イベントは `complete`。requestの `success` と混同して `tx.onsuccess` を実装しない。

## durability は「何を失わないか」の別軸

[W3Cのdurability hint][durability] は `strict` / `relaxed` / `default` を区別する。`strict` は永続媒体への書込み確認、`relaxed` はOSへの書込み後の完了判断、`default` はstorage bucketに対するUA既定動作を用いる指示。hintという位置付けを維持し、ハードウェアのあらゆる故障への保証へ広げない。

[Chromeの解説][chrome] は121でreadwriteの既定をstrictからrelaxedへ変更すると説明する。`strict` でも `put()` の直後に媒体へ保存し終わったわけではなく、まずtransaction完了を待つ必要がある。`relaxed` の `complete` はOS crashや電源断に対して同じflush境界を意味しない。記事の性能倍率は本稿の環境で再測定していない。

**独自の選択基準:** 再取得可能なキャッシュと、ユーザーがまだ他へ保存していない唯一の編集結果を同じpolicyで扱う必要はない。損失許容度、性能、端末への負荷を踏まえて明示選択を検討する。重要な移行では「新保存先の `complete` を確認する前に旧保存先を削除する」手順を避け、途中停止からの回復も設計する。

`strict` はバックアップ、サーバー側の受領確認、ユーザーによるサイトデータ削除からの保護を実装するものではない。「ローカル保存済み」「サーバーへ同期済み」「復元可能なコピーあり」を一つの成功フラグへ畳み込まない。

## 採用時に実行する境界試験

以下は本稿の独自試験案で、成功済みの実行ログではない。各ケースを新しい一時databaseで実行し、terminal event後に別transactionで実際のレコードを読む。例外名だけでrollbackを推定しない。

| ケース | 確認する結果 |
|---|---|
| 最初のaddがsuccess、次のaddが重複キー | default処理ではabortし、最初の追加も残らない |
| 重複errorでstopPropagationだけ | 伝播が止まってもdefault abortは解除されない |
| 既知の重複errorをpreventDefaultで回復 | 意図した残りの操作のみcommitするか。UIは全件成功と誤表示しない |
| idbでerror回復とbubblingを併用 | native completeとtx.done rejectが併存するかを、固定版で観測する |
| 最初の要求登録後に同期DataCloneError | batch Promiseがrejectし、先行書込みも残らない |
| 明示abortでtx.errorがnull | fallbackのAbortError等で失敗を通知できる |
| transaction途中で外部fetch/timer待ち | 旧handleを使えない場合を扱い、保存成功を誤表示しない |
| idbのrequest Promiseだけをawait | 外部awaitのケースと区別し、採用版・各エンジンで成立条件を確認する |
| commit直後、pending requestのsuccessで追加write | 新規要求を受け付けない境界と、誤った完了待ちを検出する |
| 取得revisionと保存時revisionが異なる | 新しいローカル編集を旧計算結果で上書きしない |
| strict/relaxedの通常完了 | event配送の検証と、実際の電源断耐久性の検証を分離する |

## 検証記録・限界

- 2026-10-05 UTC、knowledge・patterns・modulesを索引化し `IndexedDB` / `IDBTransaction` を横断検索した。既存収録はなく、transaction / durability検索では別技術の文書だけが見つかった
- W3C snapshotとMDN各ページ、Chrome記事をnative webで開き、日付・該当本文・ライセンスを確認した
- idbのmainは取得時に40桁commit `77dd8bebf3669bbce9628e470a021ff63eb4acaf` を指していた。`package.json` の8.0.3、README、実装、ISC LICENSEをそのcommitに固定してGitHub connectorで読んだ。commit日時は2025-05-07。最新版一般の保証ではない
- pinned READMEのnative web取得はcache missだったため、その部分はGitHub connectorで補った。可変mainの内容だけを固定版の証拠として使っていない
- cloud環境のChromium **154.0.8037.57** で独立したIndexedDB probeを起動したが、fixture読込み前に `socket() failed: Operation not permitted` で終了した。**IndexedDBのブラウザーruntime試験は実行できていない**。Firefox・Safari・WPT・quota/電源断試験も未実行
- コード例の構文確認・文書metadata検証・検索evalはブラウザーのtransaction動作を証明しない。DOMのmockやNode上のPromise試験を実機durability検証として扱わない

## 出典とprovenance

W3C仕様はSoftware and Document License 2023。MDN各文書はMozilla ContributorsによるCC-BY-SA-2.5以降、コードの扱いは [MDNのライセンス説明](https://developer.mozilla.org/en-US/docs/MDN/Writing_guidelines/Attrib_copyright_license) を確認した。Chrome記事の本文はCC-BY-4.0、例示コードはApache-2.0。idbは固定commitのISC LICENSEを確認した。本稿はAPI事実の独自整理と明示した設計・試験案で、sourceの例示コードや実装はコピーしていない。

[spec]: https://www.w3.org/TR/2025/WD-IndexedDB-3-20250813/
[lifecycle]: https://www.w3.org/TR/2025/WD-IndexedDB-3-20250813/#transaction-lifecycle
[transaction-api]: https://www.w3.org/TR/2025/WD-IndexedDB-3-20250813/#transaction
[errors]: https://www.w3.org/TR/2025/WD-IndexedDB-3-20250813/#fire-error-event
[durability]: https://www.w3.org/TR/2025/WD-IndexedDB-3-20250813/#transaction-durability-hint
[tx]: https://developer.mozilla.org/en-US/docs/Web/API/IDBTransaction
[commit]: https://developer.mozilla.org/en-US/docs/Web/API/IDBTransaction/commit
[add]: https://developer.mozilla.org/en-US/docs/Web/API/IDBObjectStore/add
[guide]: https://developer.mozilla.org/en-US/docs/Web/API/IndexedDB_API/Using_IndexedDB#adding_data_to_the_database
[chrome]: https://developer.chrome.com/blog/indexeddb-durability-mode-now-defaults-to-relaxed
[idb]: https://github.com/jakearchibald/idb/blob/77dd8bebf3669bbce9628e470a021ff63eb4acaf/README.md#transaction-lifetime
[idb-impl]: https://github.com/jakearchibald/idb/blob/77dd8bebf3669bbce9628e470a021ff63eb4acaf/src/wrap-idb-value.ts

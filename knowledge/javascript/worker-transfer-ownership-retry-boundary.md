---
{
  "id": "javascript-worker-transfer-ownership-retry-boundary",
  "title": "Workerのstructured cloneとtransfer: ArrayBuffer所有権・失敗・再試行の境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "WHATWG HTML Living Standard 2026-10-22; Draft ECMA-262 2026-10-23 (ECMAScript 2027); Node.js v24.19.0 Linux probes only; browser runtime unverified",
  "tags": [
    "research-domain:frontend",
    "Worker",
    "postMessage",
    "structuredClone",
    "ArrayBuffer",
    "SharedArrayBuffer",
    "transfer",
    "detachment",
    "DataCloneError",
    "messageerror",
    "retry"
  ],
  "sources": [
    {
      "id": "whatwg-structured-transfer-ownership-20261023",
      "url": "https://html.spec.whatwg.org/multipage/structured-data.html#structuredserializewithtransfer",
      "type": "official_docs"
    },
    {
      "id": "whatwg-worker-postmessage-lifecycle-20261023",
      "url": "https://html.spec.whatwg.org/multipage/workers.html#dom-worker-postmessage",
      "type": "official_docs"
    },
    {
      "id": "whatwg-messageport-delivery-transfer-20261023",
      "url": "https://html.spec.whatwg.org/multipage/web-messaging.html#message-port-post-message-steps",
      "type": "official_docs"
    },
    {
      "id": "whatwg-shared-memory-agent-boundary-20261023",
      "url": "https://html.spec.whatwg.org/multipage/webappapis.html#integration-with-the-javascript-agent-cluster-formalism",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-arraybuffer-detachment-20261023",
      "url": "https://tc39.es/ecma262/multipage/structured-data.html#sec-detacharraybuffer",
      "type": "official_docs"
    },
    {
      "id": "node24-worker-transfer-aliases-20261023",
      "url": "https://nodejs.org/download/release/v24.19.0/docs/api/worker_threads.html#considerations-when-transferring-typedarrays-and-buffers",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2027-01-21",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Workerのstructured cloneとtransfer: ArrayBuffer所有権・失敗・再試行の境界

## 問いと採用判断

画像処理やバイナリ解析を Worker に渡す際、`postMessage()` の戻りを「処理成功」、`catch` を「入力は無傷」と扱ってよいか。どちらも一般には成立しない。転送で失うのは送信側のメモリ所有権であり、受信・計算・業務上の成功とは別の境界である。

本稿は新しい機能の速報ではない。既存の Fetch cancellation やイベントループの説明では扱っていない、バッファの所有権移動と再試行の契約を整理する。対象は主にブラウザの dedicated Worker と通常の固定長 `ArrayBuffer`。任意の transferable、すべての Worker 種別、性能順位を一括して保証しない。

取得日は2026-10-23 UTC。[HTML structured data][clone]・[Worker][worker]・[MessagePort][port]・[agent cluster][agents]の各ページは Last Updated 22 October 2026。[ECMA-262][ecma]は October 23, 2026 の ECMAScript 2027 draft であり、2027年版の正式発行を意味しない。[Node.js v24.19.0公式文書][node]は実装側の補助資料として読み、ブラウザのセキュリティ条件と区別した。

## clone・transfer・sharedは別の所有モデル

| 選択 | 送信側 | 受信側 | 判断 |
|---|---|---|---|
| ArrayBufferを通常のstructured clone | 元のバイト列を保持する | 独立したバイト列を受け取る | 両側で入力が必要、再実行用に保持したい |
| ArrayBufferをtransfer listに入れる | 対象bufferがdetachされる | 転送された格納領域に対応する新しいbufferを受け取る | 送信側のすべての利用者が手放せる |
| SharedArrayBufferをcloneする | 格納領域を引き続き参照する | 別のwrapperから同じ共有領域を参照する | 同期と利用環境の設計が別途必要 |

これは [StructuredSerializeInternal / WithTransfer][clone] の区別である。`structuredClone(value, { transfer: [...] })` も所有権を消費する。単なる安全な deep copy 検証として本番の buffer に実行しない。通常オブジェクトの prototype やメソッドをそのまま復元する RPC 機構でもないため、アプリの入力は明示したデータ形式として扱う。

[DetachArrayBuffer][ecma] は元のデータブロックとの関連を切り、byte length を0にする。detach後も元の JavaScript オブジェクトへの参照自体は残る。「あらゆる読み書きが必ず例外になる」とは説明しない。例えば detach 済みのインスタンスに対する `buffer.byteLength` は0を返す。最初から空のbufferも長さ0なので、この値だけでdetachを一般判定できない。`detached` accessorを使う場合も対象実装の対応確認が必要であり、アプリの所有状態は別に記録する。

## transfer listはpayloadでも部分範囲でもない

1. transfer list は何を移すかの指定であり、メッセージのフィールドを自動追加しない。`worker.postMessage({ taskId }, [buffer])` だけでは、通常の ArrayBuffer は受信データから辿れず、送信側だけがそれを失い得る。payloadの中にもbufferまたはそのviewを入れる。[clone]
2. `Uint8Array` や `DataView` はviewであり、通常そのview自体をtransfer listに入れない。移す対象は背後の `ArrayBuffer`。同じbufferを複数のviewが参照するなら、影響は送ったviewだけに限定されない。[clone][node]
3. `subarray()` で狭めたviewを渡しても、その `.buffer` の転送は格納領域全体の移動である。受信側がviewの `.buffer` から別のviewを作れば、元の範囲外のバイトにも到達できる。小さいviewを作るだけでは、境界外データの開示防止にも小さい転送単位にもならない。[clone]

次は本稿の独自例。`retained` はアプリが保持する通常の `Uint8Array`、`worker` は初期化済みdedicated Workerとする。転送する範囲を新しいbufferへ明示的にコピーする。

```js
const retained = new Uint8Array([11, 22, 33, 44]);
const portion = retained.subarray(1, 3);
const outgoing = new Uint8Array(portion.byteLength);
outgoing.set(portion);
worker.postMessage(
  { taskId: 'parse-17', attempt: 1, bytes: outgoing },
  [outgoing.buffer],
);
// outgoing は以後利用しない。retained は別のbufferを保持する。
```

コピーは余分な時間とメモリを使うため、無条件にtransferより速いとはしない。全bufferの所有権を移せるなら追加コピーを避ける判断もできる。保持する入力、in-flight件数、最大byte数を合わせて制限することは本稿の設計提案であり、postMessageに自動backpressureがあるという主張ではない。

## 送信例外をrollbackと同一視しない

[StructuredSerializeWithTransfer][clone] は、transfer listの型・重複等を検査し、payloadをserializeしてから、リスト順に転送処理を進める。転送前に失敗できるようになっているが、すべての副作用を元に戻すtransactionではない。

| 失敗地点 | 確認した契約 | アプリの扱い |
|---|---|---|
| 重複した同一buffer、transfer不能なview、SharedArrayBufferをtransfer listに指定 | 初期検査でDataCloneError | listを修正する。毎回新しい試行を無制限に作らない |
| functionなどserialize不能な値をpayloadに含める | payloadのserializationが外側の転送処理より先に失敗 | 単純な副作用のないpayloadならbufferはまだ移されていない。入力schemaを修正 |
| payloadのgetterが例外または副作用を起こす | propertyの読み出しでユーザーコードが走り得る | catchだけで入力の無変化を断定しない。失敗が常にDataCloneErrorとも限らない |
| 転送処理の後半で失敗 | 個別処理には失敗経路があり、仕様には既に行ったdetachを復旧する手順がない | 複数resourceの原子的な移動を前提にしない |

例えばgetterが別の `structuredClone(..., { transfer: [...] })` を実行してから例外を投げると、外側のcloneが失敗しても、内側で行った転送は戻らない。以下は危険性を確認するための独自の反例であり、推奨するpayloadではない。

```js
const buffer = new ArrayBuffer(9);
const payload = {
  get value() {
    structuredClone(buffer, { transfer: [buffer] });
    throw new Error('original getter failure');
  },
};
try { structuredClone(payload); } catch { /* 失敗を記録する */ }
// Node.js v24.19.0でbuffer.byteLength === 0を確認。外側の失敗は復元しない。
```

本稿の推奨は、送信直前に任意のclass instanceやgetter付きobjectを渡すのでなく、信頼できる入力から副作用のないpayloadを構築すること。`structuredClone()`を事前検証に使っても、getterが二度評価され得るうえ、転送付きなら検証自体が所有権を消費する。事前検証の成功を、本番送信の成功保証にはしない。

実装差の留保も重要である。Node.js v24.19.0で「二番目が既にdetach済みのbuffer」のケースを実測すると、最初のbufferは長さ9のまま残った。これを、すべての実装・transferableに対する原子的rollbackの証拠にはできない。逆に、その例だけで全ブラウザに部分detachが起きるとも主張しない。後半失敗の一般的な注意は仕様の手順に基づくもので、各ブラウザでの再現結果は未確認である。

## postMessageの戻り・messageerror・完了通知を分ける

[Worker.postMessage][worker] は内部のMessagePortへ委譲する。[message port post message steps][port] ではserializeとtransferを同期的に行い、その後に配送先を確認し、受信処理をtaskとして積む。したがって、正常returnは受信handlerの実行や計算完了の通知ではない。宛先portがない場合でも、転送を試みた後でreturnする経路がある。

受信時のdeserialize失敗は、受信側のmessage event targetで `messageerror` になる。[port] 送信側の `try/catch` が将来の受信失敗を捕まえるわけではない。また、受信側のアプリがpayloadを検査して拒否する場合や、計算中の業務エラーは、deserialize失敗と別の結果として返す。`messageerror` に業務taskIdが必ず入ると仮定しない。

Workerの `terminate()` は待機taskの破棄や実行中scriptの中断を含む。[worker] 受信済みbufferの所有権を送信側へ戻す操作ではない。終了、クラッシュ、タイムアウトで返信が来ないことは、未着・未実行・実行済みのどれかを一意に示さない。

## 再試行は同じbufferでなく論理入力から作る

ここからは上記契約に基づく本稿独自の設計提案であり、Web APIが提供するexactly-once保証ではない。

- 所有状態を `local → sent → result` と、`send-failed` / `outcome-unknown` に分ける。`sent` は「手放した」であり「処理済み」ではない。転送済みviewをReact等のUI状態に残して再描画時に使わない。
- retryには同じ `taskId` と異なる `attempt` を使い、再生成可能な入力から新しいbufferを作る。必要なら保管したバイト列、バージョン固定の取得元、内容hashを組み合わせる。ライブURLを再fetchして同じ入力だったとみなさない。
- `accepted` と `completed` を分け、UI更新は期待したtaskId・attempt・入力versionに対応する結果だけで行う。古いWorkerから遅着した結果を、新しい試行の成功に数えない。
- 再実行可能な純粋計算なら再生成で復旧しやすい。Workerが外部への副作用を持つ場合は、buffer再生成とは別に重複実行対策が必要。タイムアウトだけで副作用を二重実行しない。
- Workerからbufferをtransferして返しても、受信した新しいbuffer/viewを採用する。最初の送信側オブジェクトが再び有効になる契約ではない。poolは返却されたhandleで更新する。
- 入力を保持する方針にはメモリ負荷、保持しない方針には再取得・再計算コストがある。何を復旧可能性として残すかを先に決め、性能は実際のサイズとin-flight数で測る。

## SharedArrayBufferは失敗回避の代用品ではない

`SharedArrayBuffer` はtransfer listに入れられない。cloneで得るのは同じデータブロックを参照する別wrapperであり、独立snapshotではない。[clone] 送信側がdetachしなくなる代わりに、複数agentが同じ領域を扱う設計になる。これは「retry用の安全なコピーを残す」と異なる。

ブラウザのserializationには `cross-origin isolated capability` の検査があり、共有データは同一agent clusterの外へ出せない。[clone] [agent clusterの対応表][agents]ではWindowとそれが作ったdedicated Workerは同じclusterに含まれる一方、Windowとshared workerやservice workerは同じとは扱われない。同一origin、`SharedArrayBuffer` constructorの存在、`crossOriginIsolated`の値だけを、任意の相手への配送成功保証にしない。

本稿の設計判断は、SABを使うなら共有領域のwriter、読み出してよい時点、状態遷移、終了時の処理を別途定義し、必要なAtomics同期を検証すること。`postMessage` の通知だけを、後から変更される共有領域の不変snapshotとみなさない。Node.jsではブラウザのCOOP/COEP・secure contextを再現しないため、NodeのSAB成功をブラウザ導入条件の検証に使わない。header配備や共有メモリprotocolの実装は本稿の範囲外である。

## 検証結果と未確認事項

2026-10-23 UTCにNode.js v24.19.0、V8 13.6.233.17-node.51、Linux x64で独自probe 11件を実行し、11件のassertion groupが成功した。cloneの独立性、全aliasのdetach、transfer-onlyの到達不能、view/重複list/functionの拒否、getter副作用、部分範囲の明示コピー、SAB共有、既にdetach済みの後続項目の実装観測、MessageChannelの送信直後detachを含む。検索evalはこの実行試験とは別である。

- 上記のWorker送信例そのものはブラウザで未実行。コピーとtransfer部分はstructuredClone / Node MessageChannelで検証したが、dedicated Workerの起動・終了・配送を検証したことにはしない。
- Chromium 155.0.8059.79のheadless起動を試みたが、実行環境の `socket() failed: Operation not permitted` で起動前に失敗した。権限やsandbox設定は変更していない。Chrome・Firefox・Safariの横断試験は未実施。
- browserのcross-origin isolation、別agent clusterへのmessageerror、Worker終了の競合、メモリ不足、platform transferableの後半失敗、resizable/growable buffer、性能値は未検証。
- 本番の回帰試験では正常配送だけでなく、送信前の拒否、送信直後の終了、返信の遅着、同一bufferのalias、view範囲外のデータ、再試行の重複と所有状態を判定する。sleep後に値が届いたという試験だけにしない。

## 出典とprovenance

本文は一次資料を読んだ独自の日本語要約であり、仕様本文やupstreamコードを転載していない。コード例・probe・再試行の状態設計は本調査で作成したもので、公式サンプルや動作保証ではない。Node公式文書のBuffer poolやmarkAsUntransferableをブラウザAPIとして移植しない。既存source recordの日付は更新せず、本稿用の新規recordを参照した。

WHATWG文書は[IPR Policy §7.1.1][whatwg-license]のCC BY 4.0、source codeに取り込む部分はBSD-3-Clause。Ecmaは[独自copyright noticeおよびsoftware license][ecma-license]、Node.js本体と付属文書は[v24.19.0 LICENSE][node-license]を確認した。採録は知識文書までであり、module昇格やブラウザ互換性の認定ではない。

[clone]: https://html.spec.whatwg.org/multipage/structured-data.html#structuredserializewithtransfer
[worker]: https://html.spec.whatwg.org/multipage/workers.html#dom-worker-postmessage
[port]: https://html.spec.whatwg.org/multipage/web-messaging.html#message-port-post-message-steps
[agents]: https://html.spec.whatwg.org/multipage/webappapis.html#integration-with-the-javascript-agent-cluster-formalism
[ecma]: https://tc39.es/ecma262/multipage/structured-data.html#sec-detacharraybuffer
[node]: https://nodejs.org/download/release/v24.19.0/docs/api/worker_threads.html#considerations-when-transferring-typedarrays-and-buffers
[whatwg-license]: https://whatwg.org/ipr-policy
[ecma-license]: https://tc39.es/ecma262/multipage/copyright-and-software-license.html
[node-license]: https://github.com/nodejs/node/blob/v24.19.0/LICENSE

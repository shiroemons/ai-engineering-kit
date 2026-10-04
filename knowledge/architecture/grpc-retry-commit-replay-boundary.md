---
{
  "id": "architecture-grpc-retry-commit-replay-boundary",
  "title": "gRPC retry の境界: headers・replay buffer・deadline と業務確定を分ける",
  "kind": "knowledge",
  "technology": "architecture",
  "version": "gRFC A6 (document 2024-08-23, pinned dc9fd4fe5b94b90b82fe2833ad1d80938e6a49c1); grpc-go v1.84.0 (e84aa5ab15d1d2b29d54f838312ad490cb7551a8, 2026-09-17); live guides checked 2026-10-04 UTC; source review only",
  "tags": [
    "research-domain:api-distributed",
    "architecture",
    "gRPC",
    "retry",
    "commit-point",
    "transparent-retry",
    "Trailers-Only",
    "replay-buffer",
    "deadline",
    "idempotency"
  ],
  "sources": [
    {
      "id": "grpc-retry-guide-commit-20261004",
      "url": "https://grpc.io/docs/guides/retry/",
      "type": "official_docs"
    },
    {
      "id": "grpc-a6-retry-design-20261004",
      "url": "https://github.com/grpc/proposal/blob/dc9fd4fe5b94b90b82fe2833ad1d80938e6a49c1/A6-client-retries.md",
      "type": "official_docs"
    },
    {
      "id": "grpc-deadlines-retry-budget-20261004",
      "url": "https://grpc.io/docs/guides/deadlines/",
      "type": "official_docs"
    },
    {
      "id": "grpc-status-retry-outcome-20261004",
      "url": "https://grpc.io/docs/guides/status-codes/",
      "type": "official_docs"
    },
    {
      "id": "grpc-core-cancel-outcome-20261004",
      "url": "https://grpc.io/docs/what-is-grpc/core-concepts/",
      "type": "official_docs"
    },
    {
      "id": "grpc-cancellation-handler-20261004",
      "url": "https://grpc.io/docs/guides/cancellation/",
      "type": "official_docs"
    },
    {
      "id": "grpc-go184-retry-implementation-20261004",
      "url": "https://github.com/grpc/grpc-go/tree/e84aa5ab15d1d2b29d54f838312ad490cb7551a8",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# gRPC retry の境界: headers・replay buffer・deadline と業務確定を分ける

## 問いと位置付け

`UNAVAILABLE` を retry 対象にしたのに再試行されないのはなぜか。逆に retry を無効化したつもりで複数の transport attempt が発生するのはなぜか。さらに、再試行しなくなった RPC や期限切れ RPC の業務更新を、成功・失敗のどちらと扱うべきか。

結論は、**retry eligibility、最終 RPC status、業務状態の確定を別々に扱う**こと。gRPC の retry commit は「この attempt から別 attempt へ切り替えない」という client 側の境界であり、DB commit の通知ではない。

これは最近追加された retry 機能の紹介ではなく、未収録だった持続的な設計上の gap を補う調査である。[HTTP の冪等メソッド](../http/retry-idempotency.md)は HTTP semantics、[retry budget と circuit breaker](circuit-breaker-retry-budget-cascading-failure.md)は集団的な負荷制御を扱う。本稿は同じ論理 RPC の replay 可否と、失われた応答の意味に対象を絞る。

## 1. 同じ status でも retry できるとは限らない（公式設計）

[Retry guide](https://grpc.io/docs/guides/retry/)と[gRFC A6](https://github.com/grpc/proposal/blob/dc9fd4fe5b94b90b82fe2833ad1d80938e6a49c1/A6-client-retries.md#when-retries-are-valid)を合わせて読む。

- configured retry は method に適用される policy の status、試行上限、残り期限などの条件付き。`UNAVAILABLE` が出ただけでは実行されない
- **Response-Headers を受信すると retry commit** になる。初期 metadata が application に見えるため、その後の失敗を別 attempt の結果へ透明に置き換えない。最初の response message をまだ受け取っていなくてもこの境界へ到達する
- headers/message より先に server handler が error で終了した場合、A6 は **Trailers-Only** を使う設計を定める。末尾だけで status を返す経路と、先に Response-Headers を送り後から同じ error status を返す経路は異なる
- client が replay 用に保持できる送信履歴を超えても commit になる。buffer overflow は送信禁止や業務拒否を意味せず、進行中の attempt は継続できる。残りの attempt 数を増やしても失われた履歴は復元できない

したがって、診断用 initial metadata を早く送る server interceptor は retry 可能期間を短くしうる。これは上記契約からの設計上の帰結であり、すべての interceptor が headers を送るという主張ではない。

### Transparent retry と configured retry

[A6 Transparent Retries](https://github.com/grpc/proposal/blob/dc9fd4fe5b94b90b82fe2833ad1d80938e6a49c1/A6-client-retries.md#transparent-retries)は、server application に未到達と library が判断できる場合を区別する。

| 観測された段階 | A6 の扱い |
|---|---|
| load balancing 後も client から送信されていない | deadline 内で transparent retry の候補 |
| server library までは届いたが application には未到達 | transparent retry は一度。その後の扱いは configured policy に従う |
| application が処理した可能性がある | 未処理保証のない configured retry として、method の再実行安全性が必要 |
| load balancer が DROP と判定 | 即失敗。通常の一時失敗と同一に再試行しない |

transparent retry は configured `maxAttempts` に数えない。`maxAttempts=4` は初回を含む configured attempt の上限で、全 transport attempt が必ず4以下という保証ではない。application が自分で「応答がないから未処理」と推測して transparent retry を模倣してはいけない。

## 2. 固定実装で確認した差: grpc-go v1.84.0

以下は gRPC 全実装の普遍的な API ではない。[grpc-go v1.84.0](https://github.com/grpc/grpc-go/tree/e84aa5ab15d1d2b29d54f838312ad490cb7551a8) のソースに限定する。版を固定した理由は、一般ガイドの要約だけでは実装固有の停止条件を落とすためである。

### WithDisableRetry は transparent retry を残す

[Retry guide](https://grpc.io/docs/guides/retry/)の総論には、channel の retry 無効化で transparent retry も止まるという記述がある。しかしこの版の[WithDisableRetry](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/dialoptions.go#L678-L686)は、**transparent retry に影響しない**と明記する。

[shouldRetry](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/stream.go#L800-L822)も、終了/commit/DROPの拒否、transparent retry 判定、`disableRetry` 判定の順である。`WithDisableRetry` を「wire送信は最大一回」の制御としない。総論との不一致はここで解消せず、この版を採用する判断では版固定の API と実装を根拠にする。

### 読取に見える ClientStream.Context() も commit を起こす

[ClientStream の契約](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/stream.go#L94-L125)は、`Header` または `RecvMsg` が戻るより前に `Context()` を呼ぶべきではなく、呼ぶと後続 client-side retry が無効になるとする。[Context 実装](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/stream.go#L920-L928)は実際に `commitAttempt()` を呼ぶ。

これは server handler の `stream.Context()` や、RPC 開始前から保持している入力の cancellation handle と同一ではない。stream interceptor が client stream の情報取得だけのつもりで早期 `Context()` を呼ぶと、retry の挙動を変える。観測用コードも retry 契約の review 対象にする、というのが独自の推奨である。

また `Header()` の呼出し開始だけを commit と扱わない。[Header 実装](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/stream.go#L976-L1012)は内部で retry を行いうる。待機中に Trailers-Only の retryable error が返る経路と、実際に headers を受信した経路を分ける。nil metadata だけから最終成功を決めず、文書化された `RecvMsg` の終端 status を確認する。

### Replay buffer と message limit は別の設定

[defaultCallInfo / MaxRetryRPCBufferSize](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/rpc_util.go)では per-RPC replay budget の既定が `256 * 1024` bytes、変更 API は Experimental。これを gRPC 共通の既定値や、単一 message の送受信上限とは呼ばない。

[bufferForRetryLocked](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/stream.go#L1039-L1051)は累積サイズが上限を**超えた**場合に commit する。`SendMsg` が加算するのは message header と wire payload の長さで、元の application object の heap 容量ではない。圧縮も影響しうる。この値は process の全メモリ使用量の上限でもない。

client-streaming / bidirectional-streaming では、個々の message が小さくても送信履歴の累積で replay 不能になりうる。上限超過を `RESOURCE_EXHAUSTED` の送信サイズ拒否と混同しない。**retry を失うことと現在の attempt が失敗することは別**である。

## 3. 回数と待機の予算は業務結果を証明しない

### Pushback は追加の実行許可ではない

[A6 Pushback](https://github.com/grpc/proposal/blob/dc9fd4fe5b94b90b82fe2833ad1d80938e6a49c1/A6-client-retries.md#pushback)の `grpc-retry-pushback-ms` は retry の待機時間を指示する。負数/解析不能なら再試行しない。非負なら、**ほかの retry 条件を満たす場合にだけ**指定 delay を使い、その後の通常 backoff は initial 値へ戻る。

固定した[Go の shouldRetry](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/stream.go#L824-L893)では同フィールドの複数値も再試行停止になる。positive pushback が retryable status、retry throttling、maxAttempts、deadline を上書きするわけではない。例えば期限まで残り200msで1000msのpushbackなら、期限を延ばして再試行しない。

`maxAttempts` は初回を含む。[WithMaxCallAttempts](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/dialoptions.go#L791-L806)による client 側上限の既定は5で、設定値が2未満でも既定5になる。1を渡して retry を無効化する使い方にはならない。server policy に大きな回数を記述しても、この client 側上限を自動的には超えない。

### Deadline は全 attempt の待機上限

[A6 Maximum Number of Retries](https://github.com/grpc/proposal/blob/dc9fd4fe5b94b90b82fe2833ad1d80938e6a49c1/A6-client-retries.md#maximum-number-of-retries)では、call deadline が全 attempt に適用される。[Deadlines guide](https://grpc.io/docs/guides/deadlines/)は、既定で deadline を設定しないため明示設定すること、下流への伝播では経過分を引いた timeout を使うことを説明する。言語/APIごとの伝播条件も確認する。

「3回なら1回につき2秒」ではなく、最初に決めた全体期限までに処理と retry 待機を収める。外側で新規 RPC を作る再試行ループへ fresh deadline を与えると、元の論理操作の予算を超える。この外側ループへの注意は独自の設計上の帰結である。

[Status Codes](https://grpc.io/docs/guides/status-codes/)は、状態変更が成功済みでも応答遅延で `DEADLINE_EXCEEDED` になりうると明記する。`UNAVAILABLE` も非冪等操作を安全に再実行できる保証ではない。`ABORTED` は read-modify-write 全体のやり直しなど上位の単位、`FAILED_PRECONDITION` は前提状態の修復が必要な場合に使い分ける。すべてを同じ RPC の retryableStatusCodes に入れる判断を避ける。

[Core concepts](https://grpc.io/docs/what-is-grpc/core-concepts/#rpc-termination)では client/server の成功判定が食い違いうる。[取消以前の変更は rollback されない](https://grpc.io/docs/what-is-grpc/core-concepts/#cancelling-an-rpc)ことも明記される。[Cancellation guide](https://grpc.io/docs/guides/cancellation/)が補足するように、library は任意の handler を一般に強制中断できず、handler 側の協調停止が必要である。deadline/cancel を業務副作用ゼロの証拠にしない。

## 4. 判断を間違えやすいケース

下表は上記契約を組み合わせた判断例で、実行結果の報告ではない。

| ケース | transport/runtime の判断 | application の判断 |
|---|---|---|
| headers 未受信、全送信履歴あり、Trailers-Only `UNAVAILABLE` | policy と各予算が許せば retry 候補 | 非冪等な更新を無条件で再実行しない |
| initial headers の後に `UNAVAILABLE` | 同じ論理 call の透明な replay は不可 | 新規 call は別判断。既存の更新を照合 |
| 小さい message を連送し replay buffer のみ超過 | 現 attempt は継続可能、後続 retry は不可 | 現 attempt の終端を確認。buffer不足を業務拒否としない |
| grpc-go の早期 `ClientStream.Context()` | その呼出しで commit | interceptor を調査。server 側未処理を推定しない |
| grpc-go の `WithDisableRetry` 設定後に transparent attempt 発生 | この版の契約と両立 | policy不適用と即断せず attempt種別を観測 |
| server 側更新後、返信が deadline を超過 | client は失敗、server は成功済みがありうる | unknown outcome として操作IDで照合 |
| server-streaming で最初の response を読んだ後に切断 | 既に response headers を受けたため自動 replay は不可 | continuation cursor・重複排除等を明示契約にする |

## 5. 実装と運用の推奨（独自の設計案）

1. **method ごとに安全性を書く。** 読取、同じ操作IDなら結果再取得、非冪等更新を分け、server status と許される再実行単位を合わせる。gRPC の policy が業務上の重複排除を生成するわけではない
2. **unknown outcome の経路を作る。** 更新と操作ID/結果を同じ永続化単位で記録し、同じID・同じ入力は結果照合できるようにする。外部サービスへの副作用まで同じDB transactionで保護できるとはしない。保持期限と別入力の同一ID拒否も定義する
3. **headers の送出位置を review する。** 認証、trace、診断、streaming準備の middleware がいつ initial metadata を送るか把握する。retryのために業務応答を無期限にbufferする設計へ置き換えない
4. **buffer と期限を別々に決める。** リクエスト量・圧縮・同時call数から replay コストを計測する。buffer を増やすだけで既受信 response をreplay可能にできない。全体deadlineは外側の再呼出しにも引き継ぐ
5. **観測対象を分ける。** 論理 call、transport attempt、server handler の実行回数、業務更新件数を混ぜない。Retry guideの attempt metrics と call metricsを使い、操作IDで業務結果を照合する。`grpc-previous-rpc-attempts` は重複排除キーではない
6. **stream 再開を別契約にする。** sequence/cursor の意味、cursor失効、最後に永続化した位置、再送範囲を決める。自動retry不可になった後の新規streamを「続きだけ届く」と仮定しない

## 6. 受け入れ試験案と実際の確認範囲

固定した[upstream retry tests](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/test/retry_test.go)を読んだ。`TestRetryStreaming` は headers前後、早期Context、buffer overflow、negative/non-numeric/multiple pushback、送信履歴 replay を含む。`TestMaxCallAttempts` は service/client 両上限の組合せを含む。**upstream試験は今回実行していない**。

自サービスでは以下を fault injection と永続状態の検査で確かめる。

- 同じ `UNAVAILABLE` を、headers前の Trailers-Only と headers後の二経路で返し、handler回数とclient終端を比べる
- `Header()` が待機する途中の retry と、早期 `ClientStream.Context()` による停止を分ける
- replay budget 直下・同値・直上を、実際の符号化サイズで作る。単一message上限は十分高くし、replay不可と送信拒否を区別する
- pushback の0・正数・負数・不正値・複数値を試し、残余期限が足りない場合や最大試行数消費後に追加attemptが出ないことを調べる
- serverが更新した直後に応答を失わせ、再実行が安全でも、結果照合でも、業務更新が二重にならないことを確かめる
- cancel後もhandlerが処理を続けるfixtureを用意し、client側の失敗だけでrollback成功と表示しないことを確かめる

## 出典・版・未確認事項

- 取得日は全出典とも **2026-10-04 UTC**。gRPCの一般ガイドは固定製品版を持たない。Retry guideの表示更新日は2025-11-26、Deadlinesは2025-07-07、Status Codesは2024-08-21、Core conceptsは2026-05-11、Cancellationは2024-02-29。表示更新日をretry契約の導入日とは扱わない
- A6はStatus Implemented、本文のLast updatedは2024-08-23。file history上の2024-08-29 commit `dc9fd4fe5b94b90b82fe2833ad1d80938e6a49c1` を取得した。歴史的Backgroundの「retryしない」を現在の機能説明として引用しない
- grpc-goの[version更新commit](https://github.com/grpc/grpc-go/commit/e84aa5ab15d1d2b29d54f838312ad490cb7551a8)は2026-09-17T20:03:25Z、`version.go` は1.84.0。挙動をこのreleaseで新設したという主張ではない。Java/.NET/C-Core等の同等APIや既定値、hedgingの全実装対応表は未検証
- ライセンス: [grpc.io LICENSE](https://github.com/grpc/grpc.io/blob/main/LICENSE)のdocumentationはCC-BY-4.0、code samplesはApache-2.0。[proposal LICENSE](https://github.com/grpc/proposal/blob/dc9fd4fe5b94b90b82fe2833ad1d80938e6a49c1/LICENSE)と[grpc-go LICENSE](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/LICENSE)および確認ファイルのheadersはApache-2.0。本文は独自要約、上流コードの転載・module化なし
- 固定commitのGitHub fileをnative webで開いた際のcache missは成功扱いにしていない。一般ガイドと履歴はnative webで、固定ファイルの本文とLICENSEはread-only GitHub connectorで確認した
- gRPC client/serverを起動した通信試験、mesh経由のretry統合、handlerの業務冪等性、実際のメモリ使用量は未検証。検索evalは可発見性の確認であり、RPCの適合試験やexactly-onceの証明ではない

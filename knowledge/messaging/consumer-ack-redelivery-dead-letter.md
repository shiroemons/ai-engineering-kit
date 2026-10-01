---
{
  "id": "messaging-consumer-ack-redelivery-dead-letter",
  "title": "メッセージ消費の acknowledgment 範囲・再配送上限・dead-letter 処理: RabbitMQ と SQS による at-least-once 処理の設計",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "RabbitMQ Documentation 4.3 / AMQP 0-9-1, 4.3 release 2026-04-23; Amazon SQS live Developer Guide, reverified 2026-10-01",
  "tags": [
    "research-domain:api-distributed",
    "messaging",
    "acknowledgment",
    "redelivery",
    "dead-letter",
    "dead-letter-queue",
    "dlq",
    "at-least-once",
    "rabbitmq",
    "quorum-queues",
    "delivery-limit",
    "poison-message",
    "sqs",
    "visibility-timeout",
    "redrive-policy",
    "idempotent-consumer",
    "unlimited-returns",
    "basic.nack",
    "basic.reject",
    "delayed-retry-type"
  ],
  "sources": [
    {
      "id": "rabbitmq-consumer-ack-confirms-docs-20261001",
      "url": "https://www.rabbitmq.com/docs/confirms",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-dlx-docs-20261001",
      "url": "https://www.rabbitmq.com/docs/dlx",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-quorum-queues-docs-20261001",
      "url": "https://www.rabbitmq.com/docs/quorum-queues",
      "type": "official_docs"
    },
    {
      "id": "aws-sqs-visibility-timeout-docs-20261001",
      "url": "https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html",
      "type": "official_docs"
    },
    {
      "id": "aws-sqs-dead-letter-queues-docs-20261001",
      "url": "https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-43-unlimited-return-release-20261001",
      "url": "https://www.rabbitmq.com/blog/2026/04/23/rabbitmq-4.3-release",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# Consumer acknowledgment と再配送予算: RabbitMQ 4.3 / SQS

## 問いと今回の訂正

処理が失敗したとき、broker に返す操作と「何回で隔離されるか」はどう対応するか。[RabbitMQ 4.3 の発表](https://www.rabbitmq.com/blog/2026/04/23/rabbitmq-4.3-release)（2026-04-23）は、4.2 までと異なり、4.3 では失敗として扱わない return が配送上限に数えられなくなったことを説明する。

この版差は運用上重要である。旧稿の「一時失敗には nack」「delivery-limit を設定すれば poison message を隔離する」という組み合わせでは、4.3 の無制限 return を防げない。新しい重複文書を増やす代わりに、その推奨を訂正した。RabbitMQ 3資料と SQS 2資料も再読し、本文全体を確認した。

## RabbitMQ の契約（公式文書の要約）

### ack の範囲

[Consumer Acknowledgements and Publisher Confirms](https://www.rabbitmq.com/docs/confirms)（表示版 4.3）による。

- delivery tag は channel 単位。同じチャネルで `basic.ack` する。未知の tag、二重 ack、別チャネルの ack はチャネルエラーを起こす
- auto-ack は送信時点で受理とみなす fire-and-forget。処理完了前の切断で失うため、業務処理完了の保証に使うのは unsafe
- manual ack されていない配送は、接続・チャネルが閉じると自動で requeue される。redelivered（再配送フラグ）に備え、consumer は冪等にする
- `basic.nack` / `basic.reject` の `requeue=true` は再キューイング、`false` は dead-letter または破棄の対象。再配送されることと失敗計数は別の契約

### 4.3 の poison message 上限と unlimited returns

[Quorum Queues: Poison Message Handling](https://www.rabbitmq.com/docs/quorum-queues#poison-message-handling) は `x-delivery-count` が失敗配送、`x-acquired-count` が consumer への割当を追跡すると説明する。後者はアプリが実際に読んだ証拠ではない。`delivery-limit` の判定対象は失敗の方で、4.0 以降の既定は 20。超過時は設定済み DLX へ移すか破棄する。`-1` による無効化は非推奨。

4.3 の再キューイングで、AMQP 0-9-1 `basic.nack` は失敗数を増やさず、`basic.reject` は増やす。client crash / connection loss も失敗、consumer timeout やクラスタ内 network partition で consumer 接続先 node が到達不能と疑われる場合は非失敗。AMQP 1.0 は `modified` の `delivery-failed` 等で区別されるため、0-9-1 のメソッド名をそのまま対応付けない。

同ページの delayed retry は `disabled`（既定）/ `all` / `failed` / `returned` を選び、失敗・非失敗のどちらを遅らせるか指定する。**遅延を入れるだけでは終了条件を増やさない。** `basic.nack` を常に使う設計では、delivery-limit があっても無制限 return が続き得る。

### DLX の条件と保証

[Dead Letter Exchanges](https://www.rabbitmq.com/docs/dlx) では、`requeue=false`、message TTL、queue length limit、quorum の delivery-limit が主要な発火条件。`dead-letter-exchange` / `dead-letter-routing-key` を policy で指定できる。既定の内部 republish は confirms を使わず、宛先障害で失う可能性がある。

同ページの循環検出は「循環の全体に rejection がなかった場合」に drop する。あらゆる循環が自動で安全に止まるとは読まない。

[Quorum Queues: at-least-once dead-lettering](https://www.rabbitmq.com/docs/quorum-queues#activating-at-least-once-dead-lettering) の条件は `dead-letter-strategy=at-least-once`、`overflow=reject-publish`、DLX 設定と必要な `stream_queue` feature flag。consumer-timeout / delayed-retry は有効化の必須条件ではない。宛先が確認できないと送信元に滞留し、再送重複もあり得る。

## SQS の対照契約（公式文書の要約）

[Visibility Timeout](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html) は、受信しても message はキューに残り、`DeleteMessage` が完了処理となると説明する。visibility timeout は既定30秒。`ChangeMessageVisibility` で変更できるが、最大12時間の起点は初回受信であり、延長してもリセットされない。これは排他処理の絶対保証ではなく、期間内にも再配送の可能性がある。

同資料では standard の in-flight 上限が約120000で、short polling は `OverLimit`、long polling は新規配送を返さなくなる。FIFO の同じ message-group-ID は、処理中の配送が削除されるか可視性が戻るまで後続を止める。head-of-line blocking を並行度設計に含める。

[Dead-Letter Queues](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html) では `maxReceiveCount` を redrive policy に設定する。redrive-allow-policy は移動元を制御する。standard の保持期限は元の enqueue 時刻が起点なので DLQ の保持期間を長くし、FIFO は DLQ 移動時に起点がリセットされる。同ページは厳密な処理順序が必要な FIFO での DLQ 利用に注意を促す。

## 実務の判断手順（独自の設計案）

1. consumer の終了経路を、成功・再試行する処理失敗・未処理の一時返却・恒久失敗に分類する。ライブラリがすべての例外を nack に変換していないか調べ、メソッドと requeue 値を記録する
2. RabbitMQ 4.3 で失敗回数による隔離を意図する場合、失敗として数える返却方法を選ぶ。非失敗 return を選ぶなら、業務期限や独立の attempt budget、隔離へ進む条件をアプリ側にも定義する。x-acquired-count を見た回数だけで「処理が何回走った」と断定しない
3. 一時障害でも「個別 message だけ処理不能」と「下流全体が停止」を分ける。全体停止で全 message を高速循環させない。consumer の一時停止・流量制御を検討し、復旧を判定する責務を決める
4. delayed-retry-type と実際の返却操作が一致するか確認する。nack に failed の遅延を期待するような組み合わせを避ける。待機時間の予算と試行回数の予算を別々にレビューする
5. 成功 ack / DeleteMessage は、副作用の永続化後に行う。副作用と応答の間の crash に備え、安定した業務 ID で重複排除する。delivery tag や redelivered フラグを業務 ID の代用にしない
6. 隔離までの経路を最後まで試す。DLX/DLQ の存在・routing・受理・保持期限・監視を確認し、「上限を設定した」だけを完了条件にしない。RabbitMQ の at-least-once DLX で下流が止まる場合、送信元の滞留予算と復旧手順も用意する
7. SQS は処理時間を見て visibility を延長し、12時間を超える仕事は処理単位を分割する。FIFO では隔離後の順序を要求仕様と照らす

## 移行時の受け入れ試験案（未実行）

- nack loop: 4.3 quorum queue で低い delivery-limit を用意し、`basic.nack(requeue=true)` を繰り返す。失敗数だけでは隔離されないケースでも、アプリの停止条件が作動するか確認する
- reject failure: 同じ負荷を `basic.reject(requeue=true)` で返し、失敗数による上限と DLX への到達を確認する。境界値は採用 patch / client の組み合わせで観測する
- delay selection: 同じ返却操作を `failed` と `returned` に分け、想定した方だけに遅延が適用されるか確認する。遅延の効果と隔離の効果を一つの結果にまとめない
- crash before ack: 副作用の commit 後に接続断を起こし、再配送でも業務結果を二重作成しないことを確認する
- dead-letter target unavailable: at-least-once DLX の宛先停止で送信元の滞留と回復後の重複を確認し、通常配送の容量を使い切らない警報を試す
- SQS expiry: 処理中に visibility timeout を切らし、並行する再処理でも冪等性を保つか確認する。DLQ 保持の起点と FIFO の順序要件も別項目で確認する

## 版・provenance・未確認事項

- 取得日は UTC 2026-10-01。RabbitMQ の live docs は4.3表示で patch 固定ではない。4.3 変更の発表日は2026-04-23。SQS の live guide と RabbitMQ reference は公開・更新日を表示していない
- `/docs/4.3/` の版付き URL は取得できず、現行 URL の版表示を確認した。公開資料が更新された場合、表と実装の差を再確認する
- RabbitMQ website の LICENSE は404、AWS 旧 docs repository の LICENSE は取得失敗。live ページへの適用ライセンスを確認できないため、新規 source record は unknown とした。引用コードや図の転載はせず、独自要約と設計・試験案のみを記載した
- 旧 catalog record は変更せず、新しい6記録を使う。release_notes の30日 TTL により2026-10-31が再確認期限
- broker / AWS の実環境試験、クライアント別の例外変換、境界回数、遅延計算の実測は未検証。検索 eval は文書の可発見性だけを確認する。特に quorum reference の遅延数値例には式と整合しない段階があり、本稿はその数値例を採用しない
- 本文の対象は consumer 側。publisher confirms と outbox の送信済み判定は[別文書](publisher-confirms-mandatory-retry.md)を参照する

---
{
  "id": "messaging-consumer-ack-redelivery-dead-letter",
  "title": "メッセージ消費の acknowledgment 範囲・再配送上限・dead-letter 処理: RabbitMQ と SQS による at-least-once 処理の設計",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "RabbitMQ Documentation v4.3 (3ページ)、Amazon SQS Developer Guide current (2ページ)、いずれも 2026-09-28 取得",
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
    "idempotent-consumer"
  ],
  "sources": [
    {
      "id": "rabbitmq-consumer-ack-confirms-docs",
      "url": "https://www.rabbitmq.com/docs/confirms",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-dlx-docs",
      "url": "https://www.rabbitmq.com/docs/dlx",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-quorum-queues-docs",
      "url": "https://www.rabbitmq.com/docs/quorum-queues",
      "type": "official_docs"
    },
    {
      "id": "aws-sqs-visibility-timeout-docs",
      "url": "https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html",
      "type": "official_docs"
    },
    {
      "id": "aws-sqs-dead-letter-queues-docs",
      "url": "https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# メッセージ消費の acknowledgment 範囲・再配送上限・dead-letter 処理: RabbitMQ と SQS による at-least-once 処理の設計

at-least-once 処理で「いつ配送を完了とみなすか」「何回失敗したら諦めて隔離するか」を、RabbitMQ と Amazon SQS の公式文書で整理する。ack の有効範囲は [Consumer Acknowledgements and Publisher Confirms](https://www.rabbitmq.com/docs/confirms)、dead-letter の発火条件は [Dead Letter Exchanges](https://www.rabbitmq.com/docs/dlx)、quorum queue の poison-message 対策は [Quorum Queues](https://www.rabbitmq.com/docs/quorum-queues)（いずれも Documentation v4.3）、可視性制御は [Visibility Timeout](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html)、DLQ への移動条件は [Dead-Letter Queues](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html)（いずれも SQS Developer Guide の current ページ）を 2026-09-28 に取得して確認した。以下で「記載事実」「設計案」を明示的に分ける。

## 要点（公式文書の記載事実）

### RabbitMQ: acknowledgment の有効範囲と再配送（Confirms v4.3）

- delivery tag はチャネル単位（per channel）の有効範囲を持ち、ack はその配送を受け取った**同じチャネル**で行わなければならない。
- auto-ack（受信と同時に肯定応答する方式）は fire-and-forget であり unsafe とされる。コンシューマが処理前に落ちるとメッセージは失われる。
- `basic.ack` は正常完了の肯定、`basic.nack` と `basic.reject` は否定的応答で、`requeue` フラグが `true` なら再キューイング、`false` なら dead-letter 条件（後述）に該当しうる。
- チャネルまたは接続が閉じられた時点で未 ack の配送は自動的に再キューイングされ、再配送時には redelivered フラグが立つ。すなわち close 自体が暗黙の「未完了」扱いになる。
- 既に ack したタグへの二重 ack や未知タグへの ack はチャネルを閉じる原因になる。ack の呼び出し側はタグの有効性を自前で管理する必要がある。

### RabbitMQ: dead-letter の発火条件と設定（DLX v4.3）

- dead-letter が発火する条件は次の4つである: `requeue=false` の `nack`/`reject`、メッセージ単位 TTL の失効、キュー長制限の超過、quorum queue の delivery-limit 超過。
- dead-letter 先は `dead-letter-exchange` と `dead-letter-routing-key` のポリシーで設定する。
- dead-letter の循環検出があり、ループするメッセージは drop される。DLX 同士を相互参照させる構成はメッセージを失う。
- 既定の republish 方式は confirms を使わないため at-most-once であり、quorum queue には at-least-once の選択肢がある。dead-letter 自体の到達保証はキューの種類と設定に依存する。

### RabbitMQ: quorum queue の poison-message 対策（Quorum Queues v4.3）

- 繰り返し失敗するメッセージ（poison message）は `delivery-limit` ポリシーで上限を設ける。4.0 以降の既定は 20 で、`-1` は無効化を意味するが非推奨とされる。
- 再配送回数は `x-delivery-count` / `x-acquired-count` ヘッダで追跡し、v4.3 の文書は failure と acquire のカウント方式の対応表を示す。どちらが増えるかは失敗の形態に依存するため、カウント表を確認して上限設計に織り込む。
- 上限を超過したメッセージは drop されるか dead-letter される。
- `dead-letter-strategy` に `at-least-once` を指定すると at-least-once の dead-lettering が可能になる。関連する選択肢として、overflow 時の `reject-publish`、および consumer-timeout / delayed-retry がある。

### SQS: visibility timeout による at-least-once 再配送（current）

- `ReceiveMessage` で受信したメッセージは不可視（invisible）になり、他のコンシューマからは見えなくなる。既定の visibility timeout は 30秒である。
- `ChangeMessageVisibility` で可視化までの猶予を変更でき、上限の 12時間は初回受信起点で数える。
- `DeleteMessage` なしに timeout が切れるとメッセージは再び可視化され、at-least-once の再配送になる。すなわち SQS では「削除」が acknowledgment の役割を果たす。
- standard キューの in-flight（処理中）上限は約 120000 で、short polling では超過時に `OverLimit` が返る。
- FIFO キューでは、処理中のメッセージと同じ message-group-ID を持つメッセージはブロックされる。グループ単位の順序と head-of-line blocking の関係を前提に並行度を設計する。

### SQS: redrive policy による DLQ 移動（current）

- redrive policy の `maxReceiveCount` が、DLQ へ移動するまでの受信回数を制御する。
- `redrive-allow-policy` が、DLQ への移動元として許可されたソースキューを制御する。
- standard キューでは保持期間が元の enqueue 時刻起点で決まるため、DLQ の保持期間はソースキューより長く設定する。そうしないと隔離したメッセージが DLQ で十分に保たない。
- FIFO キューでは DLQ への移動時に enqueue 時刻がリセットされる。standard と FIFO で保持期間の起点が異なる点に注意する。

## 推奨方法（独自の設計案。上記文書の規定ではない）

- acknowledgment は「処理完了の宣言」として使う。RabbitMQ では処理が成功してから同じチャネルで `basic.ack` し、SQS では処理が成功してから `DeleteMessage` する。auto-ack や受信直後の削除は、処理前クラッシュでメッセージを失うため既定にしない。
- 失敗時は再試行の意思を明示する。RabbitMQ では一時的失敗に `requeue=true` の `nack`、もう試す価値がない失敗に `requeue=false` の `nack`/`reject` を使い、後者は DLX 経由で隔離する。
- poison message の上限を必ず設定する。RabbitMQ quorum queue では `delivery-limit`（既定 20 を起点に調整）、SQS では `maxReceiveCount` を設定し、上限超過分は DLQ へ逃がす。`delivery-limit: -1` の無効化は公式に非推奨のため使わない。
- at-least-once を前提にコンシューマを冪等にする。処理済みメッセージ ID の記録と処理を同じローカルトランザクションで行う方式は [トランザクショナル・アウトボックス](../messaging/transactional-outbox.md) の MessageLog 方式と共通である。redelivered フラグや再受信回数は重複検出の補助には使えるが、重複排除キーそのものにはメッセージ ID を使う。
- SQS の visibility timeout は処理時間の見積もりに合わせる。処理が既定 30秒を超えるなら `ChangeMessageVisibility` で延長する設計にし、12時間の上限（初回受信起点）を意識したバッチ分割にする。
- SQS standard キューを使う場合、DLQ の保持期間はソースキューより長く設定する（保持期間が元の enqueue 時刻起点のため）。FIFO では起点がリセットされる点を踏まえて保持期間を見直す。
- quorum queue で dead-letter の到達を重視するなら `dead-letter-strategy: at-least-once` を検討し、overflow 時の `reject-publish` と consumer-timeout / delayed-retry を組み合わせる。既定 republish が at-most-once であることを前提に選ぶ。
- DLX 構成に循環を作らない。`dead-letter-exchange` と `dead-letter-routing-key` の参照先を図に起こし、相互参照がないことをレビュー項目にする。

## 避ける使い方

- 受け取ったチャネルと別のチャネルで ack すること（二重 ack や未知タグと同様にチャネルを閉じる原因になる）。
- 処理完了前に auto-ack すること（fire-and-forget で unsafe と明記されている）。
- 一時的失敗のたびに無条件で `requeue=true` の `nack` を繰り返すこと（上限なしでは poison message が無限に再配送される。`delivery-limit` / `maxReceiveCount` を設定しない構成）。
- `delivery-limit` を `-1` にして poison-message 対策を外すこと（非推奨）。
- DLX 同士を相互参照させてループを作ること（循環検出で drop されメッセージを失う）。
- SQS で `DeleteMessage` せずに visibility timeout の失効に頼る通常運用（意図しない再配送が常態化する。「削除が ack」の契約に反する）。
- SQS standard キューで DLQ の保持期間をソースキュー以下にすること（元の enqueue 時刻起点のため隔離後にすぐ期限切れになりうる）。
- redelivered フラグや受信回数を「初回配送の証拠」や重複排除キーとして使うこと（再送ごとに変わらない識別子ではない。メッセージ ID を使う）。
- FIFO の同一 message-group-ID が処理中にブロックされることを考慮せず並行度を見積もること（head-of-line blocking でスループットが頭打ちになる）。

## 適用版と本番での注意

- 適用版: RabbitMQ Documentation v4.3 の3ページ（Consumer Acknowledgements and Publisher Confirms、Dead Letter Exchanges、Quorum Queues）、Amazon SQS Developer Guide の current 2ページ（Visibility Timeout、Dead-Letter Queues）。いずれも 2026-09-28 取得。
- 保証の範囲: 両者とも at-least-once が基本であり、exactly-once は提供されない。多重配送と順不同（standard キュー・classic キュー）を受け止める冪等設計が必須である。
- 版による差の注意: `delivery-limit` の既定 20 は 4.0 以降の規定であり、それより前の版の既定は本調査の範囲外（未確認）。`x-delivery-count` / `x-acquired-count` のカウント対応表は v4.3 の記載であり、他版の挙動は未確認。
- SQS の数値の注意: in-flight 上限の約 120000 と `OverLimit` は standard キューの short polling での記載である。long polling や FIFO での上限挙動、12時間上限の例外条件は本調査で確認していない。
- 未確認・範囲外: 各クライアントライブラリの ack API の詳細、visibility timeout 延長の最適間隔、DLQ からの redrive-back の運用手順、各方式のスループット・レイテンシ目標は本調査の範囲外。再試行間隔・バックオフの具体値は両文書の本ページに規定がなく、要件に応じて別途確認する。
- 再確認期限: 全 source が official_docs（TTL 90日）で、技術（messaging）固有 TTL は設定されていない。2026-12-27 に5ページを再取得し、ack 契約・発火条件・既定値・上限値の変更を確認する。

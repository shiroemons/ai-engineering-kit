---
{
  "id": "messaging-rabbitmq-consumer-timeout-cancel-recovery-boundary",
  "title": "RabbitMQ 4.3.7 consumer timeout: cancel-ok互換性・後片付けと再購読の境界",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "RabbitMQ 4.3.7, released 2026-10-20, commit cd87622aee0d4fd8b119d713f2e03420b0642a89; AMQP 0-9-1 quorum queues; Documentation 4.3 checked 2026-10-23",
  "tags": [
    "research-domain:api-distributed",
    "rabbitmq",
    "messaging",
    "consumer-timeout",
    "consumer-cancel",
    "capability",
    "recovery",
    "amqp-0-9-1",
    "quorum-queue"
  ],
  "sources": [
    {
      "id": "rabbitmq-timeout-release-437-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.7",
      "type": "release_notes"
    },
    {
      "id": "rabbitmq-timeout-channel-cd87622a-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/src/rabbit_channel.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-timeout-capability-cd87622a-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/src/rabbit_reader.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-timeout-client-cd87622a-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/amqp_client/src/amqp_channel.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-timeout-schema-cd87622a-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/priv/schema/rabbit.schema",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-timeout-tests-cd87622a-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/test/consumer_timeout_SUITE.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-timeout-quorum-docs-43-20261023",
      "url": "https://www.rabbitmq.com/docs/4.3/quorum-queues",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-timeout-consumers-docs-43-20261023",
      "url": "https://blog.rabbitmq.com/docs/4.3/consumers",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# RabbitMQ 4.3.7でconsumer timeout後の購読をどう復旧するか

## 問いと採用判断

RabbitMQを4.3系へ更新した後、接続とchannelは生きているのにconsumerが配送を受けなくなった。`basic.cancel`を受信したらchannel再接続を待てばよいか。応答がないconsumerの状態はいつ消えるか。`basic.cancel-ok`をすべてのbrokerへ送る修正で安全に解決できるか。

**判断:** connection、channel、consumer subscription、個々の業務処理を別々に監視する。4.3.7ではtimeoutによるconsumer取消しの後片付けが修正されたが、購読の再登録や実行中の業務処理の停止まで自動で保証するものではない。consumer取消しへの回復経路を持ち、相手が広告したcapabilityに従うSDKを使う。以下の「確認した事実」は一次資料・固定commitの読解、「設計案」「試験案」は独自提案である。

対象は**RabbitMQ 4.3.7、AMQP 0-9-1、manual acknowledgementのquorum queue consumer**。2026-10-20公開の[4.3.7 release](https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.7)に実際の修正がある。取得日2026-10-23を公開日と混同せず、4.4系や他系列への適用は断定しない。v4.3.7のcommitは`cd87622aee0d4fd8b119d713f2e03420b0642a89`に固定した。

既存の[consumer ackとdead-letter](consumer-ack-redelivery-dead-letter.md)は配送失敗の計数と隔離、[priorityとprefetch](rabbitmq-strict-priority-prefetch-boundary.md)は配送順、[transaction publish cap](rabbitmq-transaction-publish-cap-commit-boundary.md)は発行時のbuffer上限を扱う。本稿はtimeout通知後のconsumer/channel状態と互換性に絞る。執筆前の全文検索で`consumer_timeout_response`、`accept_consumer_cancel_ok`、`basic.cancel-ok`に一致する既存文書はなかった。`consumer-timeout`はack文書に現れたが、DLX有効化の必須条件ではないという言及だけだった。

## 確認した事実

### 1. 4.3.7は「通知したのにbroker側consumerが残る」を修正する

[4.3.7 release notes](https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.7)は、timeout時の`basic.cancel`にclientが`basic.cancel-ok`を返さなくてもconsumer状態を掃除する修正を記載する。多くの既存clientは応答を返さず、4.3.0より前のbrokerはclientからの`basic.cancel-ok`を受理しない。したがって**無条件にcancel-okを追加することは旧brokerとの互換策にならない**。

4.3の[Quorum Queues / Consumer Timeout](https://www.rabbitmq.com/docs/4.3/quorum-queues#consumer-timeout)は、期限を超えた配送をqueueへ戻し、AMQP 0-9-1ではclientが取消通知に対応すればconsumerを取消し、対応しなければchannelを閉じると説明する。classic queueやstreamへの同じtimeout保護を想定しない。

一方、取得時の[4.3 Consumers / Delivery Acknowledgement Timeout](https://blog.rabbitmq.com/docs/4.3/consumers#acknowledgement-timeout)はchannelの`PRECONDITION_FAILED`による閉鎖を条件なしで説明している。これだけを4.3.7の全経路の説明として使うと不十分である。以下ではrelease、設定schema、実際のchannel分岐を優先し、一般guideとの不一致を残す。

### 2. `consumer_timeout_response`は応答方式であり処理期限ではない

固定commitの[rabbit_channel.erl](https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/src/rabbit_channel.erl)の`handle_consumer_delivery_timeout`と[rabbit.schema](https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/priv/schema/rabbit.schema)で確認した分岐:

| 条件 | 4.3.7の動作 |
|---|---|
| `notify_consumer`、clientが`consumer_cancel_notify=true`、対象tagがconsumer mappingにある | `basic.cancel`を送り、cancel-ok待ちを登録する |
| `notify_consumer`でも上のcapabilityまたはmapping条件を満たさない | `PRECONDITION_FAILED`のchannel例外へ進む |
| `close_channel` | 通知対応clientでもchannel例外へ進む |

schemaの選択肢は`notify_consumer`と`close_channel`で、channel実装の既定は`notify_consumer`。**`consumer_timeout_response=close_channel`は新しい互換用選択肢であり、古い4.3.xにも同じ設定があるとは扱わない**。channel閉鎖では同居consumerにも影響するため、旧来の回復経路を使う代わりの隔離範囲の変更になる。

`basic.get`のmanual ackは取消すsubscription tagを持たない。上記mapping条件を満たせず、`consumer_timeout_basic_get`試験もtimeoutでchannelが閉じ、connectionは残ることを確認する形で書かれている。`notify_consumer`を「どんな利用法でもchannelを閉じない設定」と呼ばない。

### 3. 10秒はcancel-ok待ちの後片付けtimer

同じ[rabbit_channel.erl](https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/src/rabbit_channel.erl)の読解:

1. 通知経路は`basic.cancel`を送った後、`CONSUMER_CANCEL_OK_TIMEOUT=10_000`msのtimerを登録する。既に同じconsumerの待ちがあれば通知とtimerを重ねない。
2. 待ち登録のあるtagについて`basic.cancel-ok`が届くと、timerを消し、consumer mapping・queue側consumer等の取消処理へ進む。
3. 応答が来なくてもtimer発火時に**tagとtimer referenceが現在の待ち登録に一致するときだけ**同じ取消処理を行う。待ちのない古いtimerは無視する。
4. このfallbackはconsumerを取り除くもので、**cancel-ok待ちの10秒が過ぎたからchannelを閉じる経路ではない**。
5. 待ち登録のない`basic.cancel-ok`は無視する。active consumerへ要求していないcancel-okを送っても、取消しの代用にはならない。

この10秒は`consumer_timeout`の配送処理期限と別の時計である。30分等のack期限を10秒へ短縮する設定ではなく、10秒以内の掃除完了を保証するwall-clock SLAでもない。timer配信とqueue処理には実行遅延があり、今回の確認はソース読解である。

### 4. 二つのcapabilityは向きが逆

[rabbit_reader.erl / server_capabilities](https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/src/rabbit_reader.erl)と、同releaseに含まれる[Erlang clientのamqp_channel.erl](https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/amqp_client/src/amqp_channel.erl)で確認:

- **`consumer_cancel_notify`: clientからserverへ。** clientがserver起点の取消通知を扱えるという広告。serverが通知かchannel閉鎖かを選ぶ条件の一つ。
- **`accept_consumer_cancel_ok`: serverからclientへ。** serverがclientからのcancel-okを受け入れるという広告。4.3.7のserverはこれをtrueとして送る。
- 同梱Erlang clientはserver capabilityが明示的にtrueで、かつ自身が把握している既知のconsumer tagの場合にcancel-okを送る。capabilityなしはfalse扱い。これは確認した当該client実装であり、Java・Go・.NET・Python等の全SDKが同じrelease時点で対応したという主張ではない。

この分離からの**設計上の帰結**: 新clientへ替えるだけで、広告を持たない旧4.3 brokerの残留状態が必ず直るとは言えない。server側の修正版とclient側のcapability処理を別々に確認する。逆に4.3.7のserverには無応答client向けfallbackがあるため、新SDKへの一斉切替をその後片付けの成立条件にしない。

`basic.cancel-ok`はconsumer tagに対する取消し応答で、delivery tagに対する`basic.ack`ではない。通知応答が届いたことを、処理成功・業務commit・メッセージの一度だけの実行と読み替えない。

### 5. 購読の取消しは新しい購読の成立でも業務rollbackでもない

[Consumers / Cancelling a Consumer](https://blog.rabbitmq.com/docs/4.3/consumers#cancelling-a-consumer)は、通常の取消し後も既に配送中のmessageがあり得ることを説明している。通常のclient起点cancel一般論と、timeoutで期限切れ配送を返す処理も同一視しない。

固定commitの`cancel_consumer`は既存subscriptionを削除するが、新しい`basic.consume`を発行しない。connectionやchannelが残る経路では、接続断だけを契機にした回復処理が呼ばれるとは限らない。**「channelがopen」と「consumerが登録済み」は別の状態**として扱うべきである。

またbrokerの取消処理は外部DB・HTTP要求・worker threadの停止を実行していない。従って**timeout通知は業務rollbackや外部書込みのfencing証明ではない**。旧workerの遅い完了と別consumerへの再配送が重なる可能性に対し、業務側の冪等性・実行権管理は別途必要となる。この最後の判断はbrokerの責務から導いた設計案であり、新しいexactly-once保証ではない。

## 適用手順の設計案

1. 接続先brokerのpatch版、queue種別、protocol、manual ackか、SDK版、受信したserver capabilityを記録する。clusterの任意の一台を見ただけで接続先も4.3.7だと判断しない。
2. 通常は`notify_consumer`を維持し、clientの取消通知callbackから購読状態を停止へ遷移させる。再登録は原因解消、重複起動防止、backoffを含めて設計し、接続回復callbackだけに依存しない。
3. 取消通知へ対応できない既存アプリでは、4.3.7の`close_channel`を互換策として検討できる。ただし無関係のconsumerを同一channelに載せている影響と、channel例外からの復旧を試験する。設定変更をそのまま本番へ適用する推奨ではない。
4. 監視をconnection/channelの生存だけで済ませず、取消通知件数、登録consumer数、broker側残留consumer数、ready/unackedの推移、業務完了を分ける。応答待ち中の一時的なconsumer残留と恒久的な消費停止を区別する。
5. timeoutを延ばすこと、cancel-ok待ちのcleanup、再購読、業務の重複防止を別々に修正する。timeout値の変更だけで互換性問題を解消したと報告しない。

## upstreamの試験を根拠にした受入れ試験案

固定commitの[consumer_timeout_SUITE.erl](https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/deps/rabbit/test/consumer_timeout_SUITE.erl)を読んだ。**upstream試験は今回実行していない**。次はそのassertionと独自追加案を区別した確認項目であり、検索evalの合格はこれらのbroker動作試験の代わりではない。

| 根拠・試験 | 確認する境界 |
|---|---|
| `consumer_cancel_ok_after_timeout_removes_consumer` | cancel-ok後にqueue・channel・consumer metricsからconsumerが消え、connection/channelは残る |
| `consumer_removed_when_cancel_ok_never_arrives` | 応答を抑止すると3秒後にはまだconsumerがあり、その後の待機で消える。10秒のhard SLAをassertする試験ではない |
| `unsolicited_cancel_ok_keeps_active_consumer` / `consumer_cancel_ok_after_queue_delete` | 非pending consumerへのcancel-okや、既に削除済みtagへの応答で不用意にactive consumer/channelを壊さない |
| `consumer_timeout_late_ack_after_cancel_ok` | 取消後に元配送のlate ackが届いても、試験のmessageはreadyに残り、取消済みconsumerへ再配送されない。late ackを復旧手段として推奨しない |
| `consumer_timeout_response_close_channel` / `consumer_timeout_basic_get` | close_channel明示時やbasic.getではchannelが閉じ、connectionとは故障範囲が異なる |
| 独自追加案: 取消callbackと接続回復callbackを別々に観測 | connectionが生きたままのtimeoutでも業務consumerが安全に再登録されるか |
| 独自追加案: 旧workerの外部書込みを遅延させる | 別consumerへの再配送後も重複業務処理を防げるか。brokerの取消しをworker停止の証明にしない |

## 適用版、provenance、未確認事項

- 新規の事実は2026-10-20公開のRabbitMQ 4.3.7の修正。一般guideの不一致は2026-10-23の取得時点の観測であり、将来も未修正のままとは主張しない。4.3.7をRabbitMQ全体の最新版とは呼ばない。
- native webでreleaseと4.3の両guide本文を開いた。www側の4.3 Consumersと固定SHAのGitHub blobはnative webで取得できない場合があり、Consumersは公式blog hostの4.3版、ソース・schema・テストはGitHub connectorで同じ40桁SHAに固定して取得した。失敗したページだけを根拠にした主張はない。
- server/client/testのErlangファイルのMPL-2.0ヘッダーと同commitの[LICENSE](https://github.com/rabbitmq/rabbitmq-server/blob/cd87622aee0d4fd8b119d713f2e03420b0642a89/LICENSE)を確認した。schemaはroot LICENSEのserver source適用範囲として記録した。Web文書・release本文の再利用licenseは未確定のためunknownとし、独自の日本語要約だけを保存する。upstreamコード・設定例は転載していない。
- 未確認: broker実機・ネットワーク障害試験、他言語SDKのリリース済み対応版、4.4系列、混在clusterの全挙動、consumer timeout値の詳細な優先順位、AMQP 1.0やMQTTの実装。AMQP 0-9-1のcancel-ok手順を他protocolへ転用しない。
- sourceの取得日は2026-10-23。release_notesのTTL 30日が最短なので2026-11-22に再確認する。既存source recordの取得日だけを更新してはいない。

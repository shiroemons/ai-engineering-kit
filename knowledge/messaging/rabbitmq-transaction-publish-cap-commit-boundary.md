---
{
  "id": "messaging-rabbitmq-transaction-publish-cap-commit-boundary",
  "title": "RabbitMQ 4.3.6 transaction: publish上限・channel close・commit結果の境界",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "RabbitMQ v4.3.6 @ 7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095; AMQP 0-9-1; live semantics displayed 4.3; retrieved 2026-10-02 UTC",
  "tags": [
    "research-domain:api-distributed",
    "rabbitmq",
    "amqp-0-9-1",
    "transaction",
    "channel_tx_message_max",
    "tx.commit",
    "tx.rollback",
    "precondition_failed",
    "publish-buffer",
    "batching"
  ],
  "sources": [
    {
      "id": "rabbitmq-tx-cap-release-4-3-6-20261002",
      "url": "https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.6",
      "type": "release_notes"
    },
    {
      "id": "rabbitmq-tx-semantics-4-3-20261002",
      "url": "https://www.rabbitmq.com/docs/semantics",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-tx-channel-7a34a0ca-20261002",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbit/src/rabbit_channel.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-tx-schema-7a34a0ca-20261002",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbit/priv/schema/rabbit.schema",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-tx-test-7a34a0ca-20261002",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbit/test/transactions_SUITE.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-release-dates-4-3-6-20261002",
      "url": "https://www.rabbitmq.com/release-information",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# RabbitMQ 4.3.6 transaction: publish上限・channel close・commit結果の境界

## 問いと結論

AMQP 0-9-1 の大きなtransactionを使うpublisherは、RabbitMQ 4.3.6への更新でどの失敗条件を見直すべきか。[4.3.6 release](https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.6) は、未commitのpublishをchannel内に保持する件数に `channel_tx_message_max` を導入した。既定は10,000件で、上限を超えるpublishは `precondition_failed` のchannel例外になる。

実務上は「batchをboundedにする」「channel障害として検出する」「commit結果と業務処理の一度性を別々に扱う」の三点が必要になる。通常publishの流量制御、queue長、consumer prefetchの値を見ても、このtransaction bufferの上限を設定したことにはならない。既存のpublisher confirms文書は非transactionalなpublishの確認・再送を扱っており、ここでは新しいtransaction上限とcommitの意味に限定する。

## 確認した版と公開日の扱い

対象はtag v4.3.6のcommit `7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095`。導入説明はrelease、細部はその固定版の実装・schema・upstream testで照合した。GitHub release APIの `published_at` は2026-09-14T07:24:14Z、[公式release一覧](https://www.rabbitmq.com/release-information) は2026-09-16と表示する。前者をGitHubでの公開日時、後者を一覧掲載のrelease日として記録し、不一致の理由を推測しない。全資料の取得日は2026-10-02 UTC。

## 上限が数える対象（固定版の実装観察）

[固定版 rabbit_channel.erl](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbit/src/rabbit_channel.erl) の `check_tx_size` と `basic.publish` 処理を確認した。

- 開いたtransactionのpublish buffer `Msgs` の件数を、次のpublishを追加する前に比較する。有限上限NならN件の保持は可能で、N+1件目が例外になる。transactionを開始していない通常publishではこの件数検査を適用しない。
- 数えるのはpublishの件数で、payloadのbyte数、routing先queue数、保留ack数ではない。ackは同じtransaction状態の別の一覧に保持される。複数queueへfan-outしても、この検査が各queueの蓄積量を制限するわけではない。
- コメントは、transaction内publishはcommitまでchannel processに保持され、transaction外に適用されるcredit-flowのbackpressureを受けないことを動機としている。上限を無効にするとそのbufferの保護を失う。broker全体のmemory上限をこの設定だけで証明することもできない。
- 値はchannelの初期化時に取得され、channelの設定として保持される。runtime環境値を変えれば既存channelにも直ちに反映される、と考えない。新しい設定を適用したnode・新規channelで確認する設計にする。

[固定版 rabbit.schema](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbit/priv/schema/rabbit.schema) は、正の整数または `infinity` を受理する。0を「無制限」の値として使わない。`infinity` が構文上可能であることは、本番で選ぶ推奨理由にならない。上限を引き上げる前に、最大batch件数、payload分布、同時transactional channel数を確認する。

## 超過をpublish一件の失敗として握りつぶさない

[固定版 transactions_SUITE.erl](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbit/test/transactions_SUITE.erl) は、上限3を設定してから新しいchannelを作り、transactionに3件publishした後、4件目でserver起因の406 channel closeを受けるcaseを持つ。これは既定10,000件の全負荷testを実行したという意味ではない。

独自の運用設計として、channel close通知と同期API例外の両方をclientの責務に入れる。AMQPの非同期操作を含むため、「publish呼出しが返ったからbatch全件の成功」とは判定しない。既に閉じたchannelへ `tx.commit` / `tx.rollback` を送り続けるretry loopを作らず、そのchannelの未確定batchを上位へ返す。新規channelでの再送方針もbatchの業務IDと照合して決める。

406という番号だけから原因を決めない。同じchannel exceptionは別のprecondition違反でも起き得るため、診断文、操作、対象node、client側batch件数を記録し、上限超過か確認する。上限を上げるだけの自動復旧は、無制限bufferへ近づける可能性がある。

## commit / rollbackをDBのACIDと同一視しない（公式契約）

[Broker Semantics、表示版4.3](https://www.rabbitmq.com/docs/semantics) は、txをDBのACIDよりbatchingに近いものと説明する。

- publishとack、RabbitMQが追加したrejectはtransaction対象だが、queue/exchange/bindingの作成・削除とメッセージの受信自体は対象外。`tx.rollback` で受信済みメッセージが自動再キューイングされるわけではない。ack/rejectを後のtransactionで行える。
- broker障害がcommit中に発生した場合、複数queue間だけでなく単一queueでもpublishの一部が見える可能性があり、atomicityを保証しない。
- `tx.commit-ok` はtransactionの効果が可視になり、brokerがpublishしたメッセージの責任を受け入れたことを示す。consumerの業務処理完了は意味しない。ackについてはserverへの到達の指標で、後続障害による再配送まで排除しない。
- mandatory publishの `basic.return` は `tx.commit-ok` より前に届く。commit-okだけを数えて、routingできなかったメッセージのreturnを無視しない。

「結果を受信できなかったcommit」を「確実にrollbackした」と扱わないのが独自の設計上の帰結となる。batch全体の再送で重複を作る可能性に備え、個々のメッセージに安定した業務IDを持たせ、受信処理の重複排除と再開可能性を別に設計する。この上限の導入はexactly-once契約を追加していない。

## batchを分割するときの判断（独自案）

1. publisherの実最大件数を調べ、brokerの有限上限より余裕を持ったbatch件数にする。さらにbyte数と滞留時間のbudgetを独立に設ける。件数上限内でも大きなpayloadや多数channelでmemory負荷は増える。
2. 小分けにcommitする変更では、途中まで完了したbatchをどう再開するか決める。分割前後で業務上の公開タイミングが変わり得るため、単にN件ずつ送る書換えを互換とみなさない。
3. publishだけの確認にtransactionを使っていた場合はpublisher confirmsを別途比較する。ただし固定版の実装は同じchannelでtx modeとconfirm modeを相互に切替える操作を拒否する。両方を有効にして二重に保護する変更はできない。方式変更には新しいchannelと別の失敗設計が要る。
4. rolling upgradeや再接続の検証では、接続したnodeの版・設定と新規channelを対応付ける。4.3.6以前の全版に同じ上限が存在しないとは断定しない。backportの有無とmanaged service固有値は別途確認する。

## 移行test案と未検証範囲

次はアプリ側で追加するtest案であり、この調査でbrokerやErlangを起動した結果ではない。

- 小さい上限Nで、N件をcommitできるcaseと、未commitでN+1件を送り406を検出するcaseを分離する
- commit後・rollback後の次のtransactionでbuffer予算が再利用されることを確認する。固定版はそれぞれ新しいtransaction状態に戻すが、アプリclientを含む動作は未実行
- publish上限に達したchannelへ別の業務batchを誤って載せないこと、channel closeがconnection全体の成功・失敗判定に混同されないことを確認する
- ackだけのtransaction、通常publish、payloadを大きくしたbatchを区別し、件数以外のbudgetも監視する
- mandatoryでrouting不能なpublish、commit応答前の接続喪失、途中batchからの再開を試し、二重の業務副作用を検知する

throughput、memory削減量、broker障害時の各queue種別の詳細、各clientの例外型・自動復旧、AMQP 1.0のtransaction、他のrelease系列へのbackportは未確認。既存sourceの取得日を延長せず、新しいcatalog recordを追加した。

## 出典とprovenance

release・Broker Semantics・Release Informationは実ページを開いて確認した。公開日がページ固有に表示されないlive docsへ公開日を補っていない。GitHubの一部PR/blob URLはweb取得で失敗したため、固定commitの実装・schema・test・licenseはGitHub connectorで読んだ。取得失敗を成功扱いしていない。

RabbitMQ coreは同commitの [LICENSE](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/LICENSE) と [LICENSE-MPL-RabbitMQ](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/LICENSE-MPL-RabbitMQ) でMPL-2.0を確認し、channel/testのheaderも照合した。schemaはroot LICENSEのcore範囲として識別した。release本文とWeb文書の個別ライセンス適用は確定しておらずunknownと記録した。いずれも独自要約で、ソースコードの転載・module昇格は行わない。

---
{
  "id": "messaging-kafka-transaction-timeout-lso-recovery-boundary",
  "title": "Kafka transaction: commit応答喪失・offset再開・LSOの完了境界",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "Apache Kafka Java API 4.3.1 (2026-06-25 release) / 4.3 configuration and operations; transaction protocol v2 introduced in 4.0; verified 2026-10-04 UTC; broker/client runtime untested",
  "tags": [
    "research-domain:api-distributed",
    "kafka",
    "transactions",
    "commit-timeout",
    "read-committed",
    "last-stable-offset",
    "consumer-offset",
    "fencing",
    "recovery"
  ],
  "sources": [
    {
      "id": "kafka431-transaction-producer-api-20261004",
      "url": "https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/producer/KafkaProducer.html",
      "type": "official_docs"
    },
    {
      "id": "kafka431-transaction-consumer-api-20261004",
      "url": "https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/KafkaConsumer.html",
      "type": "official_docs"
    },
    {
      "id": "kafka431-transaction-next-offsets-api-20261004",
      "url": "https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/ConsumerRecords.html",
      "type": "official_docs"
    },
    {
      "id": "kafka431-transaction-fenced-api-20261004",
      "url": "https://kafka.apache.org/43/javadoc/org/apache/kafka/common/errors/ProducerFencedException.html",
      "type": "official_docs"
    },
    {
      "id": "kafka43-transaction-producer-config-20261004",
      "url": "https://kafka.apache.org/43/configuration/producer-configs/",
      "type": "official_docs"
    },
    {
      "id": "kafka43-transaction-consumer-config-20261004",
      "url": "https://kafka.apache.org/43/configuration/consumer-configs/",
      "type": "official_docs"
    },
    {
      "id": "kafka43-transaction-design-20261004",
      "url": "https://kafka.apache.org/43/design/design/",
      "type": "official_docs"
    },
    {
      "id": "kafka43-transaction-protocol-20261004",
      "url": "https://kafka.apache.org/43/operations/transaction-protocol/",
      "type": "official_docs"
    },
    {
      "id": "kafka-transaction-release-list-20261004",
      "url": "https://kafka.apache.org/community/downloads/",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# Kafka transaction の応答喪失と再開位置

## 問いと採用判断

Kafka topicを読み、変換結果を別topicへ書く処理で、commitの返事を受け取れなかったらabortして同じ入力を再送してよいか。さらに `read_committed` のlagが0なら、生成された仕事をすべて処理したと言えるか。

結論は、transactionの結果不明とabort確定を分け、入力offsetと出力を同じtransactionへ含めて回復すること。可視性の上限LSO、consumerの取得位置、業務完了位置も別々に記録する。これは4.3で新しく入った機能の紹介ではなく、未収録だった運用上の契約を4.3.1 APIで確認する調査である。

既存の [outbox](transactional-outbox.md) はDB更新からのイベント発行、[RabbitMQ/SQS ack](consumer-ack-redelivery-dead-letter.md) と [Kafka share groups](kafka-share-lock-renewal-group-limits.md) は再配送・取得権を扱う。本稿は通常の `KafkaConsumer` と `KafkaProducer` のconsume-transform-produceに限定する。全knowledge・patterns・modulesと直近16件のknowledge更新を調べ、執筆前の検索 `commitTransaction`、`last stable offset`、`transactional.id` は0件、`read_committed` はMySQL関連文書のみだった。

## 1. Kafka内の原子的結果と、処理コードの実行回数

[Design: Using Transactions](https://kafka.apache.org/43/design/design/#using-transactions) は、出力recordと入力consumerのoffsetをproducerの同一transactionで更新する方式を説明する。consumer側は `enable.auto.commit=false` と `isolation.level=read_committed`、producer側は `transactional.id` を設定する。rebalanceを含む扱いを単純にするため、一つのconsumerに一つのproducerを対応させることが推奨される。

ここで束ねられるのはKafka内の更新である。変換処理の途中で外部HTTPを呼び、料金請求やメール送信をしても、その効果がKafkaのabortで取り消されるわけではない。公式Designも外部宛先のexactly-onceには宛先側の協力が必要と説明する。処理コードが再実行されることと、commit済みの論理結果が二重になることを同じ意味にしない。

**設計案:** Kafka内の結果だけで処理を完結できるならtransactionを使う。外部副作用があるなら、宛先で受け付ける業務ID、結果照合、同じDBに置く重複排除記録などを別途設計する。本稿のJava API契約だけで一般的なXA/2PC連携の可否を決めない。

## 2. commitのtimeoutをabortへ読み替えない

[KafkaProducer API](https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/producer/KafkaProducer.html#commitTransaction()) の `commitTransaction()` は、`max.block.ms` 内に応答を得られなければ `TimeoutException`、待機中の割込みでは `InterruptException` を返し得る。どちらもcommitがbrokerに届かなかった証明ではない。同じcommit操作は再試行できるが、すでに完了へ進んでいる可能性があるため `abortTransaction()` へ切り替えられない。再試行しない場合の選択肢はproducerのcloseである。

`abortTransaction()` のtimeout・interruptも対称的で、abortを再試行するかcloseする。結果未確定のabortからcommitへ反転しない。これは各methodの明示契約であり、class冒頭の短い例にある「広いKafkaExceptionを捕捉してabortする」という形だけを汎用回復ループへ拡張しない。

同APIの `close(Duration)` は、既に完了処理中でないongoing transactionをabortすると説明する。従ってcloseした事実を、結果不明だったcommitのrollback証明に使わない。

**独自の回復状態案:**

- `OPEN`: 入力・出力を準備する。失敗時は例外別の契約を確認する
- `COMMITTING`: commit開始後のtimeout/interruptではここに留め、同じproducerでcommitを再試行するかcloseする。新しいsend・別transaction・abortへ進まない
- `ABORTING`: abort開始後のtimeout/interruptでは、同じabortの再試行かcloseに限定する
- `CLOSED_UNRESOLVED`: 成否不明のままcloseした場合。業務失敗確定として入力を独自に再送せず、次節の再開手順へ進む

これらの名前はKafka公開enumではない。本稿の設計案であり、終了予算を超えたら無限retryせず「未確定」を保持して停止する。`TimeoutException` という型だけで全APIを同じ分岐へ送らず、どの操作で発生したかを保持する。

## 3. offsetの受領・transaction確定・再取得を分ける

[sendOffsetsToTransaction](https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/producer/KafkaProducer.html#sendOffsetsToTransaction(java.util.Map,org.apache.kafka.clients.consumer.ConsumerGroupMetadata)) の正常復帰は、offset要求がcoordinatorに受領された段階であり、transactionのcommitまではoffset確定ではない。この方式ではconsumer独自の `commitSync` / `commitAsync` を併用しない。`consumer.groupMetadata()` で取得した完全なmetadataはgroup IDだけより強いfencingを提供し、broker 2.5以上が必要である。

保存するoffsetは最後に処理したrecordそのものではなく、**次に処理する位置**である。[ConsumerRecords.nextOffsets()](https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/ConsumerRecords.html#nextOffsets()) はそのpollで位置が進んだpartitionの次offsetとmetadataを返す。取得batchを全件処理した場合の候補となるが、一部をworkerへ渡しただけならbatch末尾をcommitしない。未処理recordを越えない位置を選ぶことはアプリ側の責務である。

`initTransactions()` は同じ `transactional.id` の前instanceが残したtransactionを解決する。未完了のまま失敗したものはabortし、既に完了へ進んでいたものはその完了を待つ。したがって「再起動すれば前回のcommitは必ずabortされる」という読み方は誤りである（Producer API）。

**独自の再開手順:**

1. 同時稼働する別workerで `transactional.id` を共有せず、引継ぎ対象の論理workerと対応づける。前instanceを解決する必要がある再起動では同じIDを維持する
2. 結果不明の終了から復帰したらproducerの初期化・前transaction解決を先に済ませ、現在のpartition割当てと確定済みgroup offsetを基に入力位置を回復する。古いメモリ上の「失敗batch」をそのまま再送しない
3. abort確定後に同じconsumerで継続する場合も、pollで進んだpositionを別途戻す。producerのabortはconsumerの `seek` の代用品ではない。現在所有するpartitionだけを対象に、記録したbatch開始位置または確定済みoffsetから再開する
4. rebalanceで所有権を失ったpartitionを古いworkerが進めない。offsetの欠落・保持期限切れに遭遇したら、無条件のlatestへのresetで完了扱いせず、欠損を明示して方針を決める

[ProducerFencedException](https://kafka.apache.org/43/javadoc/org/apache/kafka/common/errors/ProducerFencedException.html) は、同じIDの新producerが旧instanceを締め出したことを示し、旧producerはcloseが必要である。単にbackoffして旧instanceを使い続ける回復対象ではない。このfencingから外部HTTP副作用の取消まで推論しない。

## 4. read_committedの終端はhigh watermarkとは限らない

[Consumer Configs](https://kafka.apache.org/43/configuration/consumer-configs/#isolation.level) の既定は `read_uncommitted`。`read_committed` ではabort済みtransactionのrecordを返さず、open transactionより後ろにあるrecordも、そのtransactionの解決まで保留される。後続が別transactionでcommit済み、または非transactionalであっても、この順序の壁を越えて先に取得できるという契約ではない。

[KafkaConsumer.endOffsets](https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/KafkaConsumer.html#endOffsets(java.util.Collection)) が返すLSO（last stable offset）は、high watermarkとopen transactionの最小offsetの小さい方という**排他的な終端**である。`seekToEnd` もこの可視性の終端を使う。取得offsetにはcommit/abort markerやabort済みrecordによるgapがあり、連番欠落だけでデータ損失とは判定しない。同APIのfetch lagもread_committedではLSOを基準とする。

**数値例（本稿の説明用）:** high watermarkが108、未解決transaction Aの先頭が100なら、LSOは100。後続104のcommit済みtransaction Bや106の非transactional recordも、Aが解決するまで取得を保留され得る。consumer positionが100でも「108まで業務完了」とは言えない。

取得した設定表には、LSOをopen transaction先頭の一つ前と説明する表現がある。一方、同版のendOffsets method契約は最小offsetを排他的上限として返す。本稿はAPIの戻り値を扱う際に後者を採用する。`endOffsets=100` を勝手に99へ減算しない。これは文書間の表現差の記録であり、upstream修正済みという主張ではない。

**監視案:** fetch lag=0、アプリが受け取ったposition、業務確定済みoffsetを別々に観測する。LSOとhigh watermarkの差、未解決transactionの継続時間、最後に業務結果が進んだ時刻も照合する。`currentLag()` の空値は不明であり、0へ置換しない。これらの差は進捗診断用であり、そのまま未処理recordの正確な件数とするものではない。

## 5. 三つの時間予算とprotocol移行

[Producer Configs](https://kafka.apache.org/43/configuration/producer-configs/) の `max.block.ms` はtransaction関連APIがclient側で待機する予算、`transaction.timeout.ms` はcoordinatorがopen transactionをabortするまでの予算である。後者の起点は最初のpartitionをtransactionへ追加した時点で、`beginTransaction()` 呼出しの瞬間とは限らない。両方の掲載既定は60000msだが、同じ時計ではない。`transaction.timeout.ms` がbrokerの `transaction.max.timeout.ms` を超えると要求が拒否される。

さらにConsumer Configsの `max.poll.interval.ms` はgroup管理下で次のpollまで空けられる時間である。commit再試行中にこの予算を使い切る可能性を処理量・終了時間の設計に含める。max.blockだけ延ばせばtransactionとgroup membershipも同じだけ延命される、とは考えない。

本稿は通常の `transaction.two.phase.commit.enable=false` を前提にする。4.3設定表はtrue時のtransactionを期限切れにしないと説明しているため、上のtimeout説明をその構成へ無条件に適用しない。2PC連携の手順・適合性は未調査である。

[Transaction Protocol](https://kafka.apache.org/43/operations/transaction-protocol/) は4.0での強化を説明する。server側の `transaction.version=2` と4.0以上のproducerの組合せではtransactionごとにepochを進め、遅延したrecordが次のtransactionへ混入することを防ぐ。既存clientの切替はserver更新を認識した後の次transactionからで、途中のtransactionを新protocolへ変換しない。これはbroker/clientの組合せを確認する理由であり、応答喪失時の成否不明やアプリ側再送の判断を不要にする保証ではない。

## 受け入れ試験案と未確認事項

以下は独自の試験案であり、Kafka broker・Java clientを実際に起動して実行した結果ではない。

- commit要求到達後の応答だけを失わせ、timeoutからabortや新規sendへ分岐せず同じcommitを再試行できることを確認する
- abort応答を失わせ、commitへ反転せず、同じabortの再試行かcloseになることを確認する
- commit成否が不明のまま停止し、同じtransactional.idで初期化した後、確定offsetから再開して出力を二重確定しないことを確認する。commit済み・abort済みの両結果を用意する
- batchの一部だけ処理して、`nextOffsets()` の末尾を誤ってcommitしないことを確認する。abort後のconsumer位置も記録する
- 同じIDの新旧producerを重ね、旧instanceのfencingでcloseすることと、外部副作用の冪等性を別に確認する
- 長いopen transactionの後ろへ別のcommit済みrecordを置き、LSO、high watermark、fetch lag、業務完了offsetを比較する。lagが0でも未可視recordが残るケースを含める
- transaction markerとabort済みrecordによるoffset gap、`currentLag()` の不明値を、欠損確定・完了確定へ誤分類しないことを確認する
- max.block、transaction timeout、max.poll.intervalのうち異なるものを先に超過させ、同じ「timeout」という一語へ集約しないことを確認する

検索evalは本文の発見可能性だけを検証する。ネットワークfault injection、複数partitionでの実測、KRaft mixed-version環境、サードパーティclient、Kafka Streams内部の状態復元、2PC、メトリクス名・性能・サーバ実装のcommit固定読解は未検証である。

## 出典・日付・ライセンス

全sourceを2026-10-04 UTCに実際に開いた。Java API表示は4.3.1、設定・Design・operationsは4.3系列。APIページ個別の公開日は表示がない。[Downloads](https://kafka.apache.org/community/downloads/) で4.3.1は2026-06-25、別系列の4.2.2は2026-09-29公開と確認したが、この日付をtimeout契約の導入日とは扱わない。Transaction Protocolページの更新表示は2026-05-22で、機能導入は4.0と記す。

[Kafka 4.3.1 LICENSE](https://raw.githubusercontent.com/apache/kafka/4.3.1/LICENSE) と [website LICENSE](https://raw.githubusercontent.com/apache/kafka-site/markdown/LICENSE) でApache-2.0を確認した。コード・図・長い原文の転載はせず、帰属付きの独自要約と設計案を記載した。既存catalogは保持し、新しい取得記録を参照する。release_notesの30日TTLを最短として再確認期限は2026-11-03とする。

---
{
  "id": "messaging-kafka-share-lock-renewal-group-limits",
  "title": "Kafka 4.3 share groups: acquisition lock の更新・明示ack・group設定の実効値",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "Apache Kafka 4.3.1 Java API（2026-06-25公開）/ 4.3 versioned configuration; KIP-1240（Accepted、2026-02-09更新）の4.3導入を照合; 2026-10-02 UTC確認",
  "tags": [
    "research-domain:api-distributed",
    "kafka",
    "share-groups",
    "acquisition-lock",
    "renewal",
    "acknowledgement",
    "record-limit",
    "effective-configuration",
    "delivery-count"
  ],
  "sources": [
    {
      "id": "kafka-share-release-list-20261002",
      "url": "https://kafka.apache.org/community/downloads/",
      "type": "release_notes"
    },
    {
      "id": "kafka43-share-upgrade-20261002",
      "url": "https://kafka.apache.org/43/getting-started/upgrade/",
      "type": "release_notes"
    },
    {
      "id": "kafka43-share-design-20261002",
      "url": "https://kafka.apache.org/43/design/design/",
      "type": "official_docs"
    },
    {
      "id": "kafka431-share-consumer-api-20261002",
      "url": "https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/KafkaShareConsumer.html",
      "type": "official_docs"
    },
    {
      "id": "kafka431-share-ack-types-20261002",
      "url": "https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/AcknowledgeType.html",
      "type": "official_docs"
    },
    {
      "id": "kafka431-share-ack-callback-20261002",
      "url": "https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/AcknowledgementCommitCallback.html",
      "type": "official_docs"
    },
    {
      "id": "kafka43-share-consumer-config-20261002",
      "url": "https://kafka.apache.org/43/configuration/consumer-configs/",
      "type": "official_docs"
    },
    {
      "id": "kafka43-share-group-config-20261002",
      "url": "https://kafka.apache.org/43/configuration/group-configs/",
      "type": "official_docs"
    },
    {
      "id": "kafka43-share-broker-config-20261002",
      "url": "https://kafka.apache.org/43/configuration/broker-configs/",
      "type": "official_docs"
    },
    {
      "id": "kafka-kip1240-share-config-20261002",
      "url": "https://cwiki.apache.org/confluence/spaces/KAFKA/pages/399278516/KIP-1240+Additional+group+configurations+for+share+groups",
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

# Kafka share groups の lock 更新と実効設定

## 調査した問いと適用範囲

処理時間の長い独立ジョブを share groups で分配するとき、heartbeat・RENEW・業務完了・acknowledgement のどれが何を保証するか。さらに Kafka 4.3 で追加された group 設定を変更すれば、in-flight record にも即座に同じ上限が成立するかを整理する。

[Upgrading](https://kafka.apache.org/43/getting-started/upgrade/) は4.2で share groups を production-ready とし、4.3で配送回数・lock数・renew可否の group 設定追加を記録する。本稿は4.3.1 Java APIと4.3設定表の契約を扱う。[公開一覧](https://kafka.apache.org/community/downloads/)で4.3.0は2026-05-22、4.3.1は2026-06-25と確認した。別系列の4.2.2は2026-09-29公開だが、その新しさを4.3設定の対応根拠にはしない。

既存の [RabbitMQ/SQS consumer ack](consumer-ack-redelivery-dead-letter.md) は他の配送方式、[outbox](transactional-outbox.md) はDB更新とイベント発行の整合性を扱う。本稿の新規範囲は KafkaShareConsumer の poll 契約と group 単位の配送所有権である。検索語 `share groups`、`KafkaShareConsumer`、`acquisition timeout`、`share.renew.acknowledge.enable`、`RENEW Kafka` に一致する既存文書は執筆前のindexに無かった。

## 確認できた契約

### 1. partition の共有と時間制限付き所有権

[Design: The Share Consumer](https://kafka.apache.org/43/design/design/#the-share-consumer) によれば、一つのpartitionを複数consumerが分担でき、consumer数はpartition数を超えられる。異なるshare groupは同じtopicを独立して消費する。acquisition lock が有効な間、そのrecordは同じgroupの別consumerには渡らない。期限が切れると再配送可能になる。既定のlock期間は30秒。

これは無期限の排他権ではない。共有による並行処理を採ると、従来のpartition単位の逐次業務処理をそのまま期待できない。APIが一回の取得結果について保証するoffset順と、複数workerの副作用が完了する順序は別の観点である。

### 2. ack種別、poll、brokerへの確定

[AcknowledgeType](https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/AcknowledgeType.html) の区別は、ACCEPTが成功、RELEASEが再配送可能な失敗、REJECTが再配送を許さない失敗、RENEWが処理継続のためのlock更新である。RENEWを成功完了の記録として扱わない。

[Consumer Configs](https://kafka.apache.org/43/configuration/consumer-configs/) の既定 `share.acknowledgement.mode=implicit` は次のpollまたはcommitで直前の配送を暗黙にackする。個別の `acknowledge()` はexplicitで使う。したがってworkerへ処理を渡しただけで次のpollへ進む構成には、暗黙の成功ackが早過ぎる危険がある（設計上の含意）。

[KafkaShareConsumer API](https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/KafkaShareConsumer.html) の明示ack契約では、次のpoll前に取得した全recordをackしないと `IllegalStateException` になる。`acknowledge()` はlocal状態の更新で、broker確定とは別。次のpollで送るackの失敗は、そのpollからの例外だけでは検出できない。`commitSync()` の戻り値も確認し、topic-partition別の結果に入った例外を見落とさない。

### 3. RENEWの成功と、配送権の喪失を扱う

同APIは、explicitで定期的にpollし、継続中recordへRENEWを行い、更新成功したrecordを再びpollで受け取る流れを定義する。partition leader変更またはleaderへの接続断では現在の配送試行が終わる。consumerはthread-safeではない。

[AcknowledgementCommitCallback](https://kafka.apache.org/43/javadoc/org/apache/kafka/clients/consumer/AcknowledgementCommitCallback.html) はackの完了を通知し、pollを呼ぶthreadで実行され得る。通知は内部再試行後であり、retriableな例外でも未完了recordを再fetchする必要がある。`InvalidRecordStateException`、`NotLeaderOrFollowerException`、`DisconnectException` などを成功扱いしない。

従ってheartbeatが続いていることからrecordのlock保持を推定しない。業務処理側の古いworkerが止まったか、外部副作用が取り消せたかは、brokerの配送権だけでは確定しない（独自の設計上の結論）。

### 4. client・group・brokerの上限を分ける

[Consumer Configs](https://kafka.apache.org/43/configuration/consumer-configs/) は `share.acquire.mode=batch_optimized` を既定とし、この場合 `max.poll.records` を超えてbatch境界まで返す場合がある。厳密な件数制限には `record_limit` を使う。アプリ内のworker数を増やすだけで取得量の制限が変わる契約ではない。

[Group Configs](https://kafka.apache.org/43/configuration/group-configs/) にある4.3の設定名と掲載既定値は次の通り。

- `share.record.lock.duration.ms`: recordの取得lock期間、30000ms
- `share.partition.max.record.locks`: share-partition単位の取得lock数、2000
- `share.delivery.count.limit`: recordの配送試行回数、5
- `share.renew.acknowledge.enable`: renewの許可、true

[Broker Configs](https://kafka.apache.org/43/configuration/broker-configs/) 側には `group.share.*` の既定値・上下限が別にある。lock期間のgroup上限・下限を定めるのは `group.share.max.record.lock.duration.ms` と `group.share.min.record.lock.duration.ms` で、掲載既定値は60000msと15000ms。group設定表の型・最小値だけを見て許可範囲を決めない。

ここから先は実装実験ではなく、4.3導入をupgrade notesと照合した [KIP-1240](https://cwiki.apache.org/confluence/spaces/KAFKA/pages/399278516/KIP-1240+Additional+group+configurations+for+share+groups) の設計記述である。group値の更新時はbroker boundsで検証するが、broker側の変更では既存group値を検証し直して起動を止める方式ではなく、評価時に実効値をbounds内へ制限する。したがって保存値と実効値を同一視しない。

同KIPでは、配送回数上限を下げた時点でin-flightだったrecordに、縮小後の上限を超える最後の試行が残り得る。lock数上限の引下げも配送の完了につれて収束する。renewをfalseにしたgroupのRENEWは `INVALID_RECORD_STATE` で拒否する設計である。これらを即時取消・同期的なworker停止と解釈しない。

## 設計判断: 独立ジョブの長時間処理

以下は本資料の独自の設計案であり、公式の実装例の転載ではない。

1. key単位の処理完了順が不可欠なら、share group採用前に順序制約を満たす別の直列化方法を検討する。partition数を増やさずworker数だけ増やせる利点と、その追加設計を比較する
2. 成功・一時失敗・恒久失敗・継続中を業務状態として区別する。副作用が確定してからACCEPTへ進む。REJECTが別topicへの保存を保証するという根拠は本稿に無いので、調査用記録が必要なら保存手順を別に設計する
3. consumer APIの操作を所有threadに集め、workerは結果を返す。進行中処理の索引はoffsetだけにせず、topic識別子・partition・offsetと配送試行の世代を含める。古いworkerの完了通知を新しい取得権の成功に転用しない
4. `record_limit` と `max.poll.records` を処理可能量から決める。取得済みrecordの待ち時間、実処理、ack送信に必要な時間を見積もり、長いものだけrenew対象にする。並列化と無制限の取得を混同しない
5. brokerの実効lock期間に余裕を持って更新し、ackの結果を観測する。renew失敗を検知したら古いworkerの処理継続を止めるよう試みるが、それだけで外部副作用を取り消せるとは約束しない
6. 外部システムへの書込みには業務IDによる冪等性や重複排除を設ける。「副作用が完了→ACCEPT応答を受ける前に接続断→再配送」の窓を受け入れる。acquisition lockをexactly-onceの業務保証に使わない
7. group設定の縮小はdrainを含む運用変更として扱う。保存値・broker bounds・新規取得量・既存in-flight数を別々に観測し、旧上限を前提に始まった仕事が収束するまで待つ

## 避ける判断と確認すべき境界

- heartbeat正常ならRENEW不要とする。group membershipとrecord取得権を分けて検査する
- `acknowledge(ACCEPT)` 呼出しの成功だけをbroker確定とする。local更新・送信・結果通知・業務副作用の四段階を混同しない
- `commitSync()` が例外を投げなければ全partition成功とする。戻り値の失敗も確認する
- `max.poll.records` だけでhard limitだと考える。batch_optimizedとrecord_limitを区別する
- 上限変更直後にin-flight件数が新上限以下でなければ故障と断定する。KIPの収束条件と新規取得の状況を確認する
- 配送回数5を「何があっても業務処理が5回以下」とする。配送試行のカウントを副作用回数と同じ単位にしない

## 導入時の試験案と未確認事項

以下は提案する試験であり、実行済みの結果ではない。

- 1partition・複数consumer・長短混在の仕事で、取得順と副作用完了順を別々に記録する
- explicitで未ack recordを残してpollし、例外と回復手順を確認する。implicitではworker未完了のまま次へ進む対照ケースを試す
- batch_optimizedとrecord_limitで大きなproducer batchを消費し、返却件数・処理待ち時間を比較する
- RENEW中のbroker接続断・leader変更・更新拒否でcallback失敗、再fetch、古いworkerの完了通知を検査する
- 外部副作用確定後にconsumerを停止し、再配送されても業務結果が重複しないことを試す
- group値保存後にbroker boundsを狭め、保存値と実効値の差を確認する。配送回数・lock数の上限引下げ中のin-flight収束も測る

取得した4.3.1 Javadocには設定名の不一致がある。group用lock期間を `group.share.record.lock.duration.ms` と説明し、lock数には `group.share.record.lock.partition.limit` と記す箇所がある。本稿は4.3の生成済みGroup/Broker Configsに合わせた。Javadocのimplicit例の説明には同期と読める表現もあるが、Record Delivery節はpoll経由ackを非同期と説明する。確定結果が必要な設計では曖昧な例の説明を根拠にせず、明示的なcommit結果またはcallbackを確認する。

Kafka broker・Java clientを起動した動作試験、network fault injection、メトリクス名や性能値の検証は未実行。4.3.1より後のAPI、サードパーティclient、Kafka Streams、DLQ機能の対応範囲、実装のcommit固定による読解は対象外。4.2.0についてはupgrade notesに4.2.1で修正したshare-group deadlockの記録があるので、「production-ready」の語だけで旧patchの選択を正当化しない。

## 出典・ライセンス・鮮度

全参照を2026-10-02 UTCに実際に開いた。Javadocの表示版は4.3.1、設定・Designは4.3系列であり、これを全系列の最新保証にはしない。KIP-1240はAccepted、最終更新2026-02-09。契約・提案・本稿の設計判断を区別した。

Kafka公式サイトの[ソース側LICENSE](https://raw.githubusercontent.com/apache/kafka-site/markdown/LICENSE)と[4.3.1のLICENSE](https://raw.githubusercontent.com/apache/kafka/4.3.1/LICENSE)でApache-2.0を確認した。KIPページ個別の再利用ライセンスは未確認のため、そのcatalogはunknownとし、帰属付きの独自要約だけに限定する。コード・例文の転載やmodule昇格は行っていない。release_notesの30日TTLを最短として明示期限を2026-11-01に置く。

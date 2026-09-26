---
{
  "id": "messaging-transactional-outbox",
  "title": "トランザクショナル・アウトボックスによるイベント駆動の整合性: DB トランザクションとイベント発行の原子性、polling publisher と CDC の選択、at-least-once 配送での重複排除",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "Debezium Documentation 3.6 (Rev: eb98831e)、microservices.io パターン3件 (版表記なし)、Debezium blog 2019-02-19 公開・2019-09-13 更新",
  "tags": [
    "research-domain:api-distributed",
    "messaging",
    "transactional-outbox",
    "outbox",
    "event-driven",
    "dual-write",
    "cdc",
    "polling-publisher",
    "transaction-log-tailing",
    "debezium",
    "at-least-once",
    "idempotent-consumer",
    "event-ordering",
    "distributed-systems"
  ],
  "sources": [
    {
      "id": "microservices-io-transactional-outbox",
      "url": "https://microservices.io/patterns/data/transactional-outbox.html",
      "type": "architecture_pattern"
    },
    {
      "id": "microservices-io-polling-publisher",
      "url": "https://microservices.io/patterns/data/polling-publisher.html",
      "type": "architecture_pattern"
    },
    {
      "id": "microservices-io-transaction-log-tailing",
      "url": "https://microservices.io/patterns/data/transaction-log-tailing.html",
      "type": "architecture_pattern"
    },
    {
      "id": "debezium-outbox-event-router-docs",
      "url": "https://debezium.io/documentation/reference/stable/transformations/outbox-event-router.html",
      "type": "official_docs"
    },
    {
      "id": "debezium-exactly-once-docs",
      "url": "https://debezium.io/documentation/reference/stable/configuration/eos.html",
      "type": "official_docs"
    },
    {
      "id": "debezium-outbox-pattern-blog-2019",
      "url": "https://debezium.io/blog/2019/02/19/reliable-microservices-data-exchange-with-the-outbox-pattern/",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "maintainer",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# トランザクショナル・アウトボックスによるイベント駆動の整合性: DB トランザクションとイベント発行の原子性、polling publisher と CDC の選択、at-least-once 配送での重複排除

サービスが自分のデータベースを更新しつつイベントをブローカへ流すとき、DB 更新とイベント発行を原子的に扱えない問題（dual write）と、送信リレーの方式選択、多重配送の受け止め方を、パターン定義と公式実装文書で整理する。パターン定義は microservices.io の [Transactional outbox](https://microservices.io/patterns/data/transactional-outbox.html)、リレー実装は [Polling publisher](https://microservices.io/patterns/data/polling-publisher.html) と [Transaction log tailing](https://microservices.io/patterns/data/transaction-log-tailing.html)、Debezium の実装は [Outbox Event Router](https://debezium.io/documentation/reference/stable/transformations/outbox-event-router.html)、配送保証は [Exactly-once delivery](https://debezium.io/documentation/reference/stable/configuration/eos.html)（いずれも Documentation 3.6）、実装例と重複排除は [Reliable Microservices Data Exchange With the Outbox Pattern](https://debezium.io/blog/2019/02/19/reliable-microservices-data-exchange-with-the-outbox-pattern/)（Debezium 公式ブログ、2019-02-19 公開・2019-09-13 更新）を 2026-09-26 に取得して確認した。以下で「記載事実」「設計案」を明示的に分ける。

## 要点（パターン文書・公式文書の記載事実）

### dual write: 原子性を保てない2つの書き込み

- microservices.io の Context は「The command must atomically update the database and send messages in order to avoid data inconsistencies and bugs」と述べる。伝統的な DB とブローカを跨ぐ 2PC は不可行で、「The database and/or the message broker might not support 2PC」、仮に対応していても「it's often undesirable to couple the service to both the database and the message broker」。
- 2PC を使わない場合、トランザクションの途中で送ると「There's no guarantee that the transaction will commit」、コミット後に送ると「there's no guarantee that it won't crash before sending the message」でメッセージが消失しうる。これが dual write 問題の両面。
- Debezium ブログも同じ理由を記述する: サービスの DB と Apache Kafka の間に shared transaction は張れない（Kafka は distributed (XA) transaction に enlist できない）。結果、注文がローカル DB に保存されるのに Kafka に送られない、または送られたのに DB に残らない、のどちらかが起こる。
- Forces（順序を含む要求）: DB transaction が commit したら messages は送信されなければならず、rollback なら送信してはならない。さらに「Messages must be sent to the message broker in the order they were sent by the service」で、「This ordering must be preserved across multiple service instances that update the same aggregate」。T1 → E1、T2 → E2 なら T1 が先なので E1 が先に publish される必要がある。

### アウトボックスと message relay

- Solution: 送信側サービスが「store the message in the database as part of the transaction that updates the business entities」し、「A separate process then sends the messages to the message broker」。
- 参加者は Sender、Database、Message outbox、Message relay。outbox はリレーショナル DB ならテーブル、NoSQL なら各レコードのプロパティ（document や item）。
- Benefits: 2PC を使わない。「Messages are guaranteed to be sent if and only if the database transaction commits」。メッセージはアプリケーションが送信した順にブローカへ送られる。
- Drawback: 「Potentially error prone since the developer might forget to publish the message/event after updating the database」。
- Issue（多重 publish）: relay は「crash after publishing a message but before recording the fact that it has done so」場合、再起動後にもう一度 publish する。よって「a message consumer must be idempotent, perhaps by tracking the IDs of the messages that it has already processed」。パターン文書は、ブローカ自体も多重配送しうるため消費者は通常すでに冪等だと述べる。
- 関連: Saga と Domain event パターンがこのパターンの需要を生み、Event sourcing が代替の一つとして挙げられる。リレーの実装は transaction log tailing と polling publisher の2つ。
- Debezium ブログの整理: outbox により送信側は "read your own writes" を得て、他サービスへの伝播は reliable かつ eventually consistent になる。イベントを outbox レコードとして明示的に emit することで、外部消費者向けに構造化されたイベントになり、内部の domain model やテーブル変更で消費者が壊れにくい。イベント構造は emitting service の API の一部として扱い、互換的に変更する。

### リレー方式の選択: polling publisher と transaction log tailing

- Polling publisher: 「Publish messages by polling the database's outbox table」。利点は「Works with any SQL database」。欠点は「Tricky to publish events in order」「Not all NoSQL databases support this pattern」。例として Eventuate Tram による polling 実装が挙がる。
- Transaction log tailing: 「Tail the database transaction log and publish each message/event inserted into the outbox to the message broker」。仕組みは DB 依存で、MySQL binlog、Postgres WAL、AWS DynamoDB table streams が例示される。利点は「No 2PC」「Guaranteed to be accurate」。欠点は「Relatively obscure although becoming increasing common」「Requires database specific solutions」「Tricky to avoid duplicate publishing」。例として Eventuate Tram による transaction log tailing 実装が挙がる。
- 2つのリレーパターンは互いを alternative solution として参照し合う。
- Debezium ブログは CDC の観点を補う: log-based CDC は「As opposed to any polling-based approach, event capture happens with a very low overhead in near-realtime」。Debezium には MySQL・Postgres・SQL Server などの CDC コネクタがある。

### Debezium の outbox event router（Documentation 3.6）

- 実装は2段階: コネクタが outbox テーブルの変更を capture し、`outbox.EventRouter` SMT を適用する。最小設定は `transforms.outbox.type=io.debezium.transforms.outbox.EventRouter`。
- 既定の outbox 列は `id` uuid、`aggregatetype` varchar(255)、`aggregateid` varchar(255)、`type` varchar(255)、`payload` jsonb。
- `id` は emit されるメッセージのヘッダになり、「You can use this ID, for example, to remove duplicate messages」。
- `aggregateid` はメッセージキーに使われ、「This is important for maintaining correct order in Kafka partitions」。
- `aggregatetype` は既定で `route.by.field`。`route.topic.replacement` の既定は `outbox.event.${routedByValue}` で、`aggregatetype` が `customers` なら `outbox.event.customers` に届く。
- outbox テーブルは queue として機能し「All changes in an outbox table are expected to be INSERT operations」。UPDATE には `table.op.invalid.behavior`（既定 warn、error、fatal）が当たる。DELETE は SMT が自動的にフィルタする。
- 適用は outbox テーブルの変更に限定すべきで、SMT predicate か `route.topic.regex` で絞る。複数の outbox テーブルを capture できるのは構造が同じ場合のみ。この SMT は MongoDB コネクタと非互換で、MongoDB は MongoDB outbox event router を使う。
- ペイロードは JSON が既定（列型は JSON / jsonb）。Avro も `BinaryDataConverter` 等の設定で可能。

### at-least-once と重複排除（Documentation 3.6）

- 「Debezium provides at-least-once delivery guarantees」。変更は取りこぼさないが、レコードは複数回届くことがある。「Currently, Debezium does not implement an internal deduplication layer to enforce exactly-once semantics」。
- Kafka Connect の source コネクタ向け exactly-once は KIP-618（KIP-98 のトランザクション support 上に構築）。前提は Kafka の distributed mode と Kafka Connect 3.3.0 以上。worker に `exactly.once.source.support=enabled`、コネクタに `exactly.once.support=required`、`transaction.boundary` は `poll` が既定で Debezium source コネクタでは poll 必須。対応コネクタは MariaDB・MongoDB・MySQL・Oracle・PostgreSQL・SQL Server。
- ただし文書自身が「it remains unclear whether the implementation is fully correct or if there are edge cases where exactly-once semantics might be violated」と警告し、open issues KAFKA-17734・KAFKA-17754・KAFKA-17582 と Jepsen report（Redpanda、Bufstream）を挙げる。
- 順序の実装（Debezium ブログの観測事実）: `aggregateid` を Kafka キーにすると一つの aggregate の全イベントが同じ partition に入り、消費者は「in the exact order as they were produced」で消費できる。
- 消費側の重複排除（Debezium ブログの観測事実）: 消費者は MessageLog テーブルを「checked and written in the same local transaction as processing」で使い、at-least-once に对抗する。Debezium コネクタや消費者が ack 前に失敗・再起動すると数件が再処理される。イベント UUID を Kafka ヘッダに載せれば「efficient duplicate detection」ができ、本文を読まずに済む。処理がローカルトランザクションで rollback されれば MessageLog の記録も rollback され、後で再試行できる。

## 推奨方法（独自の設計案。上記文書の規定ではない）

- 業務更新と同じトランザクションで outbox 行を insert し、サービスコードからブローカへの直接 publish を外す。パターンが明記する「publish 忘れ」の drawback を構造的に減らすための判断。
- リレーは運用条件で選ぶ。既存の SQL DB だけで素早く始めたいなら polling publisher（どの SQL DB でも動く）。DB ログを読めるうえ near-real-time・低オーバーヘッドが要件なら CDC / transaction log tailing（Debezium）。polling は順序維持が tricky と明記されているため、polling を選ぶなら ordering を実装側で担保する前提を置く。
- at-least-once を前提に消費者の冪等を必須要件にする。イベント ID を重複排除キーに、処理と同じローカルトランザクションで処理済みを記録する（ブログの MessageLog 方式）。ID はヘッダから取得できるようにする。
- `aggregateid` をメッセージキーに固定し、同一 aggregate の順序を partition 内に閉じる。複数のサービスインスタンスが同じ aggregate を更新するケースでも要求順序を満たしやすくなる。
- outbox のイベント構造をバージョン付き API として管理し、互換的に進化させる。消費者は未知の属性に lenient に扱う（ブログの推奨）。
- Debezium コネクタは outbox テーブルだけを capture する（predicate / `route.topic.regex`）。heartbeat やスキーマ変更など別構造のレコードが SMT を通らないようにする。
- exactly-once が必要なら Kafka Connect の exactly-once を候補にはするが、公式文書が correctness を未確立と警告する以上、本番の完全性は消費者の冪等で受けることを既定にする。

## 避ける使い方

- トランザクション中にブローカへ直接送る、あるいはコミット後に送って完了を想定する dual write（commit 不確実・crash で消失、いずれもパターン文書が明記）。
- DB とブローカの 2PC を前提に設計すること（非対応の可能性があり、両者への結合は often undesirable とされる）。
- outbox への insert を書き忘れても気づかない構成（パターンが明記する drawback）。
- relay の多重 publish を考慮しない非冪等な消費者（publish 後 recording 前の crash で再送される）。
- Debezium outbox router で outbox 行を UPDATE・DELETE することを想定した設計（queue は INSERT のみ。UPDATE は既定 warn で黙って続行され気づきにくく、DELETE は自動フィルタされる）。
- outbox コネクタに DB 全体の変更を capture させること（SMT は outbox 由来のレコードだけを対象にすべき）。
- Debezium は exactly-once だと信じること（at-least-once で、内部 dedup layer を持たない）。
- Kafka Connect exactly-once を無条件の完全性基準にすること（公式文書が correctness unclear・open issues・Jepsen report を明記）。
- 同一 aggregate を複数インスタンスが更新するのに順序要件を設計に織り込まないこと（パターンの Forces に明記）。

## 適用版と本番での注意

- 適用版: Debezium Documentation 3.6（ページ表示 Version 3.6、site Rev: eb98831e、2026-09-26 取得）。microservices.io の3ページは版・日付の表記なし（footer は Copyright © 2026 Chris Richardson）。Debezium ブログは 2019-02-19 公開・2019-09-13 更新で、更新注記により「The custom SMT discussed in this blog post is not needed any longer」（SMT が提供済み）。
- 保証の範囲: outbox が保証するのは「DB transaction が commit した iff メッセージが送信される」ことと、アプリケーション順の送信まで。relay → ブローカ → 消費者の各段は多重配送しうる（パターン文書・Debezium 文書）。消費者側の配信順序は transactional outbox パターンのスコープ外と明記される（"that's outside the scope of this pattern"）。
- 方式の代償: CDC は DB 固有（binlog / WAL / table streams）で「Requires database specific solutions」、polling は SQL DB 全般で動くが順序維持が tricky。polling 間隔・レイテンシ目標・relay のスループットは両文書に規定がない（未確認）。
- NoSQL: polling は「Not all NoSQL databases support this pattern」。NoSQL の outbox は record property と記述されるが、本調査で具体例は未確認。
- outbox 行の後始末: ブログの例は insert と直後の delete を同じトランザクションで行い、テーブルは常に空で housekeeping 不要と説明する（CDC はログを読むため成り立つ）。Debezium 文書は DELETE を自動フィルタする。一般解は文書に規定されていない。
- topic の増大: ブログは retention 設定や compaction を論じ、削除すれば最初からの再プレイはできなくなると明記する。再プレイ要否の要件で選ぶ。
- MongoDB: outbox event router SMT は MongoDB コネクタ非互換、MongoDB 用は別 SMT（詳細は本調査の範囲外）。
- 再確認期限: official_docs・maintainer_article は TTL 90日、architecture_pattern は 180日。technology（messaging）固有 TTL は設定されていない。2026-12-25 までに Debezium 文書を再取得して内容を確認する。

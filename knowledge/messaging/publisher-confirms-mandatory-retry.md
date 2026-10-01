---
{
  "id": "messaging-publisher-confirms-mandatory-retry",
  "title": "RabbitMQ publisher confirms: mandatory return、永続化、再送重複の境界",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "RabbitMQ documentation 4.3 / AMQP 0-9-1; verified 2026-10-01",
  "tags": [
    "research-domain:api-distributed",
    "rabbitmq",
    "publisher-confirms",
    "mandatory",
    "basic.return",
    "retry",
    "durability"
  ],
  "sources": [
    {
      "id": "rabbitmq-publisher-confirms-4-3-20261001",
      "url": "https://www.rabbitmq.com/docs/confirms",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-publisher-publishers-4-3-20261001",
      "url": "https://www.rabbitmq.com/docs/publishers",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-publisher-reliability-4-3-20261001",
      "url": "https://www.rabbitmq.com/docs/reliability",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-12-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# RabbitMQ publisher confirms と routing の成功条件

## 調査した問い

publish 後に `basic.ack` を受けたら「意図したキューへ届き、業務処理まで終わった」と扱ってよいか。対象は RabbitMQ Documentation 4.3 の AMQP 0-9-1 publisher 側。既存の [consumer ack / dead-letter](consumer-ack-redelivery-dead-letter.md) と [outbox](transactional-outbox.md) を補うが、consumer の再配送上限は再説明しない。2026-10-01 に現行文書で既存契約を確認したもので、新機能の主張ではない。

## 確認できた契約

### confirm が確定する範囲

[Confirms](https://www.rabbitmq.com/docs/confirms) は publisher と broker のやり取りを対象とし、consumer の処理結果とは独立している。`confirm.select` で channel を confirm mode にし、channel 単位の sequence number を追跡する。`multiple=true` の ack は指定番号までをまとめて対象にできる。応答順を publish 順と仮定しない。

同ガイドの [confirmation timing](https://www.rabbitmq.com/docs/confirms) では、routable message の ack は全配送先キューの受理後、persistent message + durable queue ではディスク永続化後、quorum queue では過半数 replica が leader へ確認した後となる。confirm の最大応答時間は保証されない。

重要なのは unroutable message も ack され得る点である。`mandatory=true` なら `basic.return` が `basic.ack` より先に送られる。したがって ack だけを「routing 成功」と読むのは誤りである。

### mandatory と返却処理

[Publishers: Unroutable Message Handling](https://www.rabbitmq.com/docs/publishers) によれば、routing 先がなければ既定の `mandatory=false` では破棄、または設定済み alternate exchange への再発行となる。`mandatory=true` では返却を扱う handler が必要である。socket write の完了だけでは broker の受信・処理を確認できない（同ページ Data Safety）。

[Reliability Guide](https://www.rabbitmq.com/docs/reliability) は、障害後に確認を受け取れなかったメッセージを再送する方針と、その結果の重複を明示する。broker が受理済みでも confirm だけ失われることがあるため、consumer の冪等性または重複排除が必要になる。同ページの routing 条件は少なくとも一つのキューへの到達確認であり、全ての意図した購読者への業務処理完了を証明するものではない。

## 推奨する成功判定（独自の設計案）

1. 発行前に return / confirm の handler と outstanding publish の記録を準備する。業務 ID と channel 世代・publish sequence を結び付け、再接続後に古い sequence を誤適用しない
2. 必ず routing 先が必要なイベントは mandatory を使い、返却があれば後から ack が来ても「配送先なし」を成功に上書きしない。alternate exchange を使う場合は、fallback への到達を成功と数えるか別に決める
3. DB outbox の送信済み更新は、採用した routing 条件と confirm の両方を満たした後に行う。confirm と DB 更新の間の crash では重複が残るため、broker confirm を分散 transaction とみなさない
4. ack が届かない timeout は「失敗確定」ではなく「結果不明」と記録する。未確定 publish の再送時にも業務 ID を保ち、同じ ID の再処理で副作用を二重発生させない
5. 非同期 confirm の outstanding 件数・最古の未確定時間・return/nack 件数を監視し、上限に達したら送信を抑える。バッファ無制限で遅い broker を吸収しない

## 避ける解釈

- persistent 指定だけで、確認前 crash に対する durability が証明されたとすること
- confirm 成功を consumer の commit 成功と同一視すること
- return を無視して ack だけで outbox を削除すること
- 再接続が成功したので、それ以前の未確認 publish も成功したと推測すること
- delivery tag を永続的な業務重複排除キーにすること

## 確認したい障害境界（実装テスト案、未実行）

- bindings のない exchange への mandatory publish で return と ack を観測し、送信済みに進めない
- broker 受理後・confirm 受信前に接続断を作り、同一業務 ID の再送でも consumer の副作用が一回に収まる
- 複数 outstanding の部分 ack / multiple ack / 順序違いを投入し、対象だけが確定する
- routing 先が複数ある場合に、想定した queue 種別・durability と障害時の成功定義を照合する

## 版・限界・出典

3資料とも取得時の selector は 4.3、ページ公開日は表示なし。クライアント API、AMQP 1.0 / MQTT / Streams protocol、queue overflow による部分受理、alternate exchange の詳細な返却条件は未検証であり、この AMQP 0-9-1 の手順を転用しない。ライセンス確認先の取得に失敗したため source は unknown、コード転載はない。検索 eval は可発見性のみを検証し、broker 障害試験の代わりではない。official_docs の90日後、2026-12-30 に再確認する。

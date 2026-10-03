---
{
  "id": "messaging-rabbitmq-strict-priority-prefetch-boundary",
  "title": "RabbitMQ 4.3 quorum priority: 公平性の変更・返却順・prefetchと完了時刻の境界",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "RabbitMQ 4.3 strict priorities introduced 2026-04-23; 4.2 versioned baseline and live Documentation 4.3 verified 2026-10-03 UTC; exact patch activation not verified",
  "tags": [
    "research-domain:api-distributed",
    "messaging",
    "rabbitmq",
    "quorum-queues",
    "strict-priority",
    "starvation",
    "prefetch",
    "returns-queue",
    "scheduling",
    "migration"
  ],
  "sources": [
    {
      "id": "rabbitmq-43-strict-priority-release-20261003",
      "url": "https://www.rabbitmq.com/blog/2026/04/23/rabbitmq-4.3-release",
      "type": "release_notes"
    },
    {
      "id": "rabbitmq-42-quorum-priority-baseline-20261003",
      "url": "https://www.rabbitmq.com/docs/4.2/quorum-queues",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-43-priority-dispatch-20261003",
      "url": "https://www.rabbitmq.com/docs/priority",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-43-quorum-return-order-20261003",
      "url": "https://www.rabbitmq.com/docs/quorum-queues",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-43-consumer-prefetch-scope-20261003",
      "url": "https://www.rabbitmq.com/docs/consumer-prefetch",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-43-consumer-priority-selector-20261003",
      "url": "https://www.rabbitmq.com/docs/consumer-priority",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# RabbitMQ quorum priority を上げても処理期限は保証されない

## 問いと採用判断

RabbitMQ 4.2 から4.3へ更新した後も、「緊急ジョブを先に流しつつ、通常ジョブも一定割合で進む」と期待してよいか。メッセージの priority を上げれば、既に consumer が持つ仕事や再配送も追い越せるか。

[4.3の公式発表](https://www.rabbitmq.com/blog/2026/04/23/rabbitmq-4.3-release)は2026-04-23公開で、quorum queue が従来の2段階の相対優先から32段階のstrict priorityへ変わったと説明する。これは10月の新機能という主張ではなく、未収録だった移行時のスケジューリング変更の確認である。

本稿の判断は、低優先度にも待機時間や処理枠の保証が必要なら、単一quorum queueのpriorityだけへ依存しないこと。クラス別queueと処理能力の割当を検討する。高優先度のため通常処理を止めてよい用途ではstrict priorityを使えるが、初回の配送選択、consumer内の実行、業務完了、返却後の経路を分けて評価する。これは以下の公式契約から導く設計案である。

既存の[consumer ack・再配送上限](consumer-ack-redelivery-dead-letter.md)は失敗計数と隔離、[publisher confirms](publisher-confirms-mandatory-retry.md)は発行成功の境界を扱う。本稿はどの仕事に次の配送枠を渡すかに絞り、再試行手順やoutboxを再説明しない。

## 1. 4.2の公平性を4.3へ持ち越さない

[4.2 Quorum Queues / Priorities](https://www.rabbitmq.com/docs/4.2/quorum-queues#priorities)では、priority未指定または0–4はnormal、5以上はhighに分類される。両方があるとhighを2件、normalを1件の割合で優遇するが、highがnormalより先にpublishされていた場合は、normalの番でもhighが先になるという例外がある。厳密な全配送履歴を常に `H,H,N` にできるという契約ではない。

[4.3 Priority Support](https://www.rabbitmq.com/docs/priority)の現行本文では、次のように変わる。

| 観点 | quorum queueの4.3契約 |
| --- | --- |
| 値の解釈 | 0–31の32段階。範囲外のpriorityはこの範囲へclampする |
| priority未指定 | 4として扱う。classicの未指定0と異なる |
| 有効化 | 常時有効。`x-max-priority`はclassic用で、quorumでは無視する |
| 次の配送候補 | 高い値を先に選ぶ。prefetchとreturned messageの例外がある |
| 通常ジョブの進行 | 高優先度が途切れなければ、低優先度が無期限に待つstarvationがあり得る |

以下は版差から導く移行上の帰結である。旧版で同じnormalだった「明示priority=1」と「未指定」は、4.3では1対4になり、同列ではない。旧版で同じhighだった9と30も分かれる。業務クラス名を変えなくても、publish時に送っていた値の違いが配送順へ影響する。

したがってproducerごとに、明示値だけでなく省略値・client libraryの既定値・上限を超える値を棚卸しする。AMQP 0-9-1のpriority propertyはunsigned byteであり、例えば255はwire上の表現範囲にあってもquorumでは31相当になる。負数や256以上までbrokerが安全に補正してくれると期待して、不正なwire入力を認める設計にはしない。

classic queueの宣言・優先度実装へこの表を流用しない。quorumに `x-max-priority=1` を付けても、旧版のfair shareや単一優先度へ戻すスイッチにはならない。

## 2. 配送優先度と完了時刻を分ける

[Consumer Prefetch](https://www.rabbitmq.com/docs/consumer-prefetch)は、`basic.qos`が未ackの配送数を制限し、RabbitMQでは各consumerへ個別に適用されると説明する。prefetchはworkerのCPU時間や業務完了の順序を予約するものではない。

[Priority Support / Consumers](https://www.rabbitmq.com/docs/priority#how-priority-queues-work-with-consumers)が示す重要な境界は、既に低優先度の配送でprefetchが埋まっていると、新着の高優先度も空き枠を待つこと。queueが空で、届いた順にconsumerへ直ちに渡せる場合も、brokerが優先順へ並べ直す待機集合がない。

独自の設計案として、時間を次の4区間へ分けて記録する。

1. broker内で配送候補になるまでの待機
2. 配送後、client libraryやアプリのローカルキューに滞在する時間
3. workerの処理開始から外部依存先を含む実行終了まで
4. 業務結果の確定からackまで

priorityが直接扱うのは主に最初の配送選択である。残りを測らず、brokerのready件数が減っただけで緊急ジョブのSLO改善としない。複数consumerへ渡した後の処理時間が違えば、先に配送された仕事が先に完了するとは限らない。priorityは処理中の仕事を取り消すpreemptionではなく、後から来た緊急処理のためにworkerを解放する保証もない。

prefetchを小さくする試験は有用だが、常に1が最善とは限らない。独自の調整案は、実worker並列数とローカル待機数を確認し、throughputと高優先度の待機時間を同時に計測すること。clientが受け取った全件を別の無制限FIFOへ移せば、broker側の設定だけでは実行待ちを制御できない。

[Quorum Queues / Global QoS](https://www.rabbitmq.com/docs/quorum-queues#global-qos)では、channel全体を一つの枠で制限するglobal QoSは非対応で、そのchannelからquorumをconsumeするとchannel errorになる。複数consumerの合計を抑えたいからと一般のprefetch資料にあるglobalの例を流用せず、per-consumer QoSを使い、必要な合計並列数はアプリ側で管理する。

## 3. returned messageには別の順序がある

[Quorum Queues / Redelivery of Returned Messages](https://www.rabbitmq.com/docs/quorum-queues#redelivery-of-returned-messages)では、reject・nack・modifyでqueueへ返されたメッセージは元のpriority順を維持せず、returns queueへ入り、返却された順で再キューイングされると記載される。

この契約から、初回配送で優先された仕事が再配送でも必ず先頭になる、とは言えない。本稿で「元のpriorityを維持しない」とはこのスケジューリングの説明であり、配送されたmessageのpriority propertyが削除・書換えされるとまでは断定しない。また、returns queueと新着の優先度別待機列のどちらが常に先か、すべての切断・遅延再配送で同じmerge順になるかも、本資料からは確定しない。

運用案として、初回配送と返却後の待機を別集計にする。初回だけの試験に成功しても、返却を多用する本番負荷では完了期限が変わり得る。順序を直す目的だけで毎回republishへ置き換える変更は、別の配送・重複・永続化契約を持ち込むため、本稿のpriority調整だけで承認しない。

## 4. message priorityとconsumer priorityは別の選択

[Consumer Priorities](https://www.rabbitmq.com/docs/consumer-priority)の `basic.consume` の `x-priority` は、どのconsumerへ渡すかを制御する。高priorityのconsumerが受信可能ならそちらを優先し、blockedなら低priorityのconsumerにも渡す。messageごとの `basic.properties.priority` と同じ設定ではない。

「緊急メッセージだけ速いworkerへ送る」ためにconsumerのx-priorityを高くしても、ジョブ種別とworkerを対応付けるrouting規則を表すことにはならない。これは二つの独立した選択からの設計上の帰結である。特定クラスへ計算資源を予約したい場合は、queue分離とconsumer配置を明示する。

## 5. どこを変更するかの判断手順（独自案）

### 通常ジョブにも期限がある場合

- 緊急・通常のそれぞれに許容待機時間と負荷上限を定める。平均latencyだけでなく、クラス別の最古未完了時間と期限超過率を追う
- 別queueに分け、通常側にも最低限のworker枠を残す。単にqueue名だけ分けて同じ飽和worker poolを共有すれば、通常側の保証にはならない
- 共有DB pool、外部API枠、CPUもボトルネックとして確認する。worker数の割当だけで下流資源の公平性まで得たとはしない
- 高priorityを外部入力の任意値として受け入れず、業務クラスからサーバ側で決める運用を検討する。全producerが最大値を選ぶ競争では優先度の意味が失われる

### 高優先度が低優先度を止めてよい場合

- 使用するpriorityを少数の業務クラスへ固定し、未指定の扱いを明文化する
- 低優先度の待ち続けを仕様として許す条件と、期限切れ・受付制限・運用判断を決める。starvationを仕様に含めても、無制限の滞留を許す必要はない
- 返却後にも厳格な順序が必要なら、その要件は未達として別設計へ進む。初回のstrict priorityだけで満たしたと判断しない
- 同一注文の「作成」と「確定」など因果関係がある仕事では、priorityによる追い越しが許されるか確認する。brokerの優先順を業務状態機械の正しさの代用にしない

移行の完了条件は「4.3へ更新できた」ではなく、実際のproducer値、queue型、consumer QoS、ローカル待機、return率を含む負荷で、クラス別の許容条件を満たすこととする。

## 6. 受け入れ試験案（未実行）

本番とは分離したquorum queueを使う。初回選択の試験では、consumerを開始する前に対象メッセージがすべてqueueへ入ったことを確認し、一つのconsumer・manual ack・小さいper-consumer prefetchで観測する。publish直後に消費させる試験と区別する。

| 試験 | 確認する境界 |
| --- | --- |
| 旧4.2でpriority=1と未指定を比較し、同じ入力を4.3で比較 | 旧版では同じnormal、新版では1対4。未指定を0扱いするアプリの誤判定を検出 |
| 未返却のpriority=9、30を同時に待機させる | 旧highクラス内にも4.3で順位ができることを確認 |
| 31と255を入れる | 255を31より上の特別クラスと期待しない。上限外のwire値を送る試験とは別 |
| `x-max-priority=1`を付けたquorumで複数値を送る | full rangeが無効化されないことを確認。classic用の制御と混同しない |
| 高priorityの待機が途切れない負荷と、通常ジョブを併存させる | 通常の2:1進行を合格条件にしない。運用で決めた最古待機の警報・制限が作動するか検証 |
| 低priorityの配送でprefetchを埋め、処理を保留してから高priorityを送る | 高priorityでも既存の未ack配送を追い出さず、配送枠を待つ。処理開始時刻も測る |
| 再配送できるconsumerがいない状態で、低priorityを先、高priorityを後に返す。両方の返却後にconsumerを開始し、新着なし・遅延なしで再配送する | 返却順の扱いを確認。返却途中に即時配送された結果では比較せず、初回の優先順を再利用した期待値にしない |
| 同じ配送列を複数workerで異なる処理時間にする | 配送順と完了順を別々に記録する。優先順での完了を仕様と誤認しない |
| quorum用channelでglobal QoSを有効にしてconsumeする | channel errorを検出し、per-consumer設定へ是正する。設定成功だけを起動成功としない |
| class別queueの通常worker枠を残し、緊急側を飽和させる | 通常側のSLOを測る。共用DB pool等による二次的なstarvationも確認 |

有限時間の負荷試験だけで「永遠に飢餓にならない」ことは証明できない。ここでは公式のstarvation可能性を前提に、必要な運用上限と防御が観測できることを合格条件にする。

## 7. 適用版・provenance・限界

- 2026-10-03 UTCに6つの一次資料本文を開いた。4.3の機能発表日は2026-04-23。旧版比較は `/docs/4.2/quorum-queues` の4.2表示、現行referenceは4.3表示を確認した
- 現行URLはpatch固定ではない。0–31等は取得時の4.3 referenceに対する記述で、個々の補足説明や修正が4.3.0から存在したと実装で検証したものではない。mixed-version cluster、feature flag、既存queueの移行時点、rollbackの可否は未確認
- `/docs/4.3/priority` と `/docs/4.3/quorum-queues` は取得失敗し、上記の現行URLで版表示と本文を確認した。websiteのLICENSE取得も失敗したため、資料の再利用ライセンスはunknownと記録する。footerの権利表記をOSSコードのライセンスと取り違えない
- 文書は独自要約と設計・試験案だけであり、公式のコード、図、表現例は転載していない。broker実装のcommit解析、Java等のclient実装、AMQP 1.0固有のcredit、実機負荷・障害試験は行っていない
- 本文はupstream RabbitMQを対象とし、Amazon MQ等のmanaged broker独自の機能制限・設定・更新日程へ一般化しない。本文の主対象はquorum queueとAMQP 0-9-1のpublish/QoS。classicのstarvation対策、TTL掃除順、overflow時の選択、Single Active Consumerの組合せを同時に設計・検証したものではない
- 検索evalはこの文書へ到達できるかの検証であり、上表の実行結果ではない。release_notesの30日TTLが最短のため、再確認期限は2026-11-02とする

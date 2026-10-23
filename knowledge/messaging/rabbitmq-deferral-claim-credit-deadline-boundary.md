---
{
  "id": "messaging-rabbitmq-deferral-claim-credit-deadline-boundary",
  "title": "RabbitMQ 4.4 deferred message: claim・credit・期限後の通常配送の境界",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "RabbitMQ v4.4.0 at 24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f; GitHub release 2026-10-19T06:38:13Z; official release table/highlights 2026-10-20; AMQP 1.0 docs Version 4.4 verified 2026-10-23",
  "tags": [
    "research-domain:api-distributed",
    "rabbitmq",
    "quorum-queue",
    "amqp-1.0",
    "deferral",
    "claim",
    "credit",
    "delivery-time",
    "deadline",
    "ordering"
  ],
  "sources": [
    {
      "id": "rabbitmq-deferral-highlights-44-20261023",
      "url": "https://www.rabbitmq.com/blog/2026/10/20/rabbitmq-4.4-release",
      "type": "maintainer_article"
    },
    {
      "id": "rabbitmq-deferral-amqp-docs-44-20261023",
      "url": "https://www.rabbitmq.com/docs/amqp",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-deferral-release-440-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.4.0",
      "type": "release_notes"
    },
    {
      "id": "rabbitmq-deferral-release-table-20261023",
      "url": "https://www.rabbitmq.com/release-information",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-deferral-session-24d5157c-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f/deps/rabbit/src/rabbit_amqp_session.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-deferral-fifo-24d5157c-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f/deps/rabbit/src/rabbit_fifo.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-deferral-fifo-tests-24d5157c-20261023",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f/deps/rabbit/test/rabbit_fifo_SUITE.erl",
      "type": "github_repository_analysis"
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

# RabbitMQのdeferral claimを処理完了や期限延長と取り違えない

## 問いと判断

順序が前後したeventや承認待ちのmessageをquorum queueへ保留した後、deferral tokenを送れば必ずそのconsumerへ届くのか。creditがなくてもclaimできるなら、期限を超えて独占し続けられるのか。

**判断: 保留・claim・実配送・業務完了は別の状態として扱う。** RabbitMQ 4.4.0のclaimは、credit不足でも成立し得る一方、messageのdelivery timeを延ばさない。期限後は通常配送へ戻り得るため、すべてのconsumerで順序条件や承認状態を再確認する。tokenを送っただけ、何も届かなかっただけでは業務成功を記録しない。

対象はAMQP 1.0のquorum queue。以下では開発元の公開説明と固定tagの実装読解を区別し、アプリ側の設計・試験案は別節に置く。brokerやclientを実行した結果ではない。

## 新規性と版の識別

[4.4 highlights][highlights]は2026-10-20公開。[release表][release-table]も4.4.0を10月20日とする一方、[GitHub release][release]の公開時刻はAPIで`2026-10-19T06:38:13Z`だった。両者を同じ時刻へ丸めず、実装対象はtagが指す`24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f`に固定した。取得日は2026-10-23 UTCである。

直近16件のknowledge commitと既存文書を確認し、執筆前のKB検索`deferral`・`x-opt-delivery-time`は該当なしだった。既存の[consumer timeout](rabbitmq-consumer-timeout-cancel-recovery-boundary.md)は4.3.7のAMQP 0-9-1取消通知と後片付け、[priorityとprefetch](rabbitmq-strict-priority-prefetch-boundary.md)は配送済みmessageの優先順位、[consumer ack](consumer-ack-redelivery-dead-letter.md)は失敗計数・dead-letterを扱う。本稿は**明示的に保留したmessageのtoken claimとcredit・期限の相互作用**に限定する。

## 公開資料が説明する機能

[highlights][highlights]は次の三つを区別する。

- Delayed message: producerが将来の配送時刻を指定する。routingとenqueueは送信時に行う
- Delayed retry: consumerが戻したmessageをbackoff後に再配送する
- Deferred message: consumerがtoken付きで保留し、後でそのtokenを指定して取得する。期限まで取得されなければ通常のconsumerへ再び配送可能になる

[AMQP 1.0 docs / Features][amqp]もDeferred MessagesをAMQP 1.0固有の機能として列挙する。保留は`modified` outcomeに`x-opt-deferral-token`と`x-opt-delivery-time`を付け、取得要求は`flow`の`rabbitmq:deferral-tokens`で表す。[release notes][release]にあるproducerのdelayed messageだけを根拠に、AMQP 0-9-1にも同じtoken取得があると推論しない。

## 固定4.4.0実装で確認した境界

### 1. tokenだけでは明示的なdeferralにならない

[rabbit_fifo.erl / should_delay][fifo]は、今回のmodified annotationsに整数のdelivery timeがあり、broker処理時刻より未来の場合だけ、そこからdeferral tokenを取り出す。通常のdelayed-retry計算へ進む経路はtoken用の登録を作らない。過去時刻・欠落時刻・backoff設定で代用できるとはしない。単位はUnix epochからのミリ秒である。[highlights][highlights]

[rabbit_amqp_session.erl][session]はmodified内のtokenにAMQP `utf8`型を要求し、長さは256 **bytes**以下を検査する。取得側もAMQP `utf8`配列で、1 FLOWあたり最大256 tokens。処理待ちFLOWが重なった場合は蓄積したtoken数にも同じ上限をかける。これは256文字という制約ではなく、長い日本語tokenはwire bytesで評価する必要がある。違反は`amqp:invalid-field`として処理される。

### 2. claimは配送ではなく、creditを待つ状態を作る

[fifo / assign_deferred][fifo]はtokenに対応するkeyを共有mapからconsumerのclaimへ移すが、message自体はdelayed treeに残す。checkoutがcreditの範囲で取り出す。inactiveなsingle active consumerもclaimを保持し得るが、すぐに配送されるとは限らない。

同じtokenに複数messageがある場合、token内は保留された順に取り出す。tokenを横断する順序は指定した配列の順ではなく、実装も保証しない。取得候補はready backlogより先に扱われるが、それをアプリ全体のevent順序や既に実行中の処理の追越し制御と同一視しない。[fifo / take_claimed_up_to][fifo]・[session / claim_deferred_then_credit][session]

固定[test suite][tests]の`delayed_assign_deferred_insufficient_credit_test`は、creditゼロで2件をclaimし、creditを1ずつ増やして残りもtoken再送なしで配送する構成。`delayed_assign_deferred_exclusive_test`は先にclaimしたconsumerにcreditがなくても、別consumerの同じtoken取得ではその保留messageを取れないことを検査している。**これはテストコードの期待値の確認であり、本調査での成功実行ではない。**

### 3. claimしても期限は止まらない

[fifo][fifo]はdelivery timeに達したmessageをdelayed treeから通常配送経路へ進め、claim側に残った古いkeyは参照時に捨てる。[tests][tests]の`delayed_assign_deferred_delivery_time_still_fires_test`は、creditゼロのままclaimしたmessageについて、期限でreadyが1件となり、後のcreditで通常経路から配送されることを検査する。

したがってclaimは期限を更新するleaseでも、永久の所有権でもない。ここでの期限は通常配送へ戻る条件であり、そのwall-clock時刻ちょうどの受信や業務完了を保証しない。実際の処理遅延・credit・consumer可用性は別に評価する。

### 4. token不在は成功した業務を意味しない

[fifo / claim_deferred][fifo]は、共有mapにないtokenを黙って飛ばす。未登録、他consumerがclaim済み、期限で移動済みなどをこの操作だけで区別しない。consumerの取消・切断時には、まだdelayed tree内にある未配送claimを共有mapへ戻す経路もある。[tests][tests]のnot-found・cancel・disconnectのケースはその意図を検査する構成である。

これは常に同じconsumerへexactly-once配送する契約ではない。[AMQP docs / Limitations][amqp]もlink recoveryとexactly-once deliveryを非対応としている。通常配送や再配送で再登場したmessageを、tokenを以前送ったという理由で無条件に業務実行しない。

## 独自の設計案: 条件の成立をmessageの到着とは別に記録する

以下はRabbitMQの必須仕様ではなく、上記の境界を使うアプリへの提案である。

1. **識別子を分ける。** event ID・業務対象・順序番号・承認要求の版と、brokerでのdeferral token・deadlineを対応付ける。tokenが同じでも複数messageを表せるため、tokenを一意の業務完了IDにしない。tokenを知っていることも承認の証拠にはしない
2. **配送入口を一つの条件検査へ集約する。** 明示取得・期限後の通常配送・障害後の再配送のどれでも、前提eventが反映済みか、該当版の承認が有効かを確かめる。未成立なら再保留・隔離・人への確認のどれに進むかを明示する。期限到達を「承認された」に変換しない
3. **業務完了を永続化し、重複適用を防ぐ。** database更新と処理済みeventの記録をアプリに適した一貫した境界へ置き、成功後にconsumerとしてsettleする。外部APIとの原子性が必要なら既存の[outbox設計](transactional-outbox.md)等を別途検討する。deferralはそれを置換しない
4. **claim後の待ち時間を見分ける。** 取得要求の送信、broker側credit、実受信、条件判定、業務完了を別の観測点にする。受信ゼロなら直ちに新tokenへ変更したり、未処理messageがないと判断したりせず、credit・競合claim・期限・consumer状態を調べる
5. **tokenとdeadlineを運用予算に含める。** 生成時にbyte数を検査し、FLOWを分割する。再保留でdeadlineを更新する場合も、業務上の最大待機期間と再試行回数を別途設ける。全messageを処理できない依存障害では、個別deferralを増やすよりconsumerを停止して復旧後に再開する方針を検討する

## 未実行の受入試験案

次は採用するSDKとbrokerを組み合わせて実施するための提案であり、検索evalとは別物である。

| 操作・障害 | 確認したい境界 |
|---|---|
| tokenあり、delivery time欠落・過去・未来を比較 | 未来の明示期限だけが意図したdeferral登録になるか |
| 256 byteと257 byte、多byte文字、型違い | 文字数でなくwire型・byte制約を検出できるか |
| 1 tokenで複数messageを保留、creditゼロ→1→追加 | claimと実配送を分離し、残数を業務完了と混同しないか |
| 2 consumersが同じtokenを取得 | 取得要求を送った側が必ずmessageを得ると思い込まないか |
| claimしたままdeadline超過 | 通常配送後も未承認・順序未成立の条件を再検査するか |
| token不在・別consumerのclaim・期限移動済み | 何も届かない理由を取得要求だけで断定しないか |
| 未配送claim保持中のcancel・disconnect | 残ったmessageを再取得でき、完了eventを重複適用しないか |
| 異なるtokensの配列順を入れ替える | token間の処理順をFLOWの入力順へ依存させていないか |
| 業務更新成功直後にconsumer接続を切断 | 再配送が業務の二重実行にならないか |

## 来歴・限界

- native webでhighlights、AMQP docs、release本文、release表を開いた。固定SHAのcodeページはwebのcache missだったため、公式GitHub connectorでsession・fifo・test全文とLICENSEを取得して読解した。Webの検索要約だけで採用した主張はない
- codeの適用版は上記v4.4.0の1 commit。実稼働clusterのfeature flag、混在version、SDKのdeferral API・型encoding・復旧時の再送、ネットワーク障害下の時系列は未検証。内部state-machineの版番号からRabbitMQの製品版を逆算しない
- [repository LICENSE][license]と[deps/rabbit/LICENSE][rabbit-license]はMPL-2.0を指定する。sessionファイルのヘッダーも確認した。公開記事・web docsの再利用licenseは特定できず`unknown`として、独自要約のみを記す。コード・テストの転載、module昇格は行わない
- RabbitMQ・Erlang・clientの実行試験はしていない。KBの検索evalは発見性だけを検証し、メッセージ処理の安全性やsource記述の正しさを証明しない
- 明示期限は取得日から30日後の2026-11-22。4.4 patch、SDK更新、deferral仕様変更時には期限前でも固定版との差を再確認する

[highlights]: https://www.rabbitmq.com/blog/2026/10/20/rabbitmq-4.4-release
[amqp]: https://www.rabbitmq.com/docs/amqp
[release]: https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.4.0
[release-table]: https://www.rabbitmq.com/release-information
[session]: https://github.com/rabbitmq/rabbitmq-server/blob/24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f/deps/rabbit/src/rabbit_amqp_session.erl
[fifo]: https://github.com/rabbitmq/rabbitmq-server/blob/24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f/deps/rabbit/src/rabbit_fifo.erl
[tests]: https://github.com/rabbitmq/rabbitmq-server/blob/24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f/deps/rabbit/test/rabbit_fifo_SUITE.erl
[license]: https://github.com/rabbitmq/rabbitmq-server/blob/24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f/LICENSE
[rabbit-license]: https://github.com/rabbitmq/rabbitmq-server/blob/24d5157cd415ef396c8ecd40a2d08c4d7ad3da9f/deps/rabbit/LICENSE

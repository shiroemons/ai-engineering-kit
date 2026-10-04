---
{
  "id": "api-design-webhook-redelivery-signature-dedup",
  "title": "Webhook の再送保証と署名検証・重複排除: 受信側の acknowledgment と処理完了の分離、再配信順不同への対応",
  "kind": "knowledge",
  "technology": "api-design",
  "version": "Stripe webhook rolling docs（event destination API例: Stripe-Version 2026-09-30.preview）+ GitHub webhook公式4ページ（版・公開日表示なし）; 全文再確認 2026-10-04 UTC",
  "tags": [
    "research-domain:api-distributed",
    "api-design",
    "webhook",
    "signature-verification",
    "hmac-sha256",
    "replay-attack",
    "duplicate-detection",
    "idempotency",
    "acknowledgement",
    "out-of-order-delivery",
    "redelivery",
    "event-id",
    "stripe",
    "github",
    "durable-inbox",
    "crash-recovery",
    "receipt-before-ack",
    "effect-idempotency"
  ],
  "sources": [
    {
      "id": "stripe-webhooks-durable-receipt-20261004",
      "url": "https://docs.stripe.com/webhooks",
      "type": "official_docs"
    },
    {
      "id": "github-webhooks-signature-revalidation-20261004",
      "url": "https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries",
      "type": "official_docs"
    },
    {
      "id": "github-webhooks-ack-revalidation-20261004",
      "url": "https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks",
      "type": "official_docs"
    },
    {
      "id": "github-webhooks-redelivery-revalidation-20261004",
      "url": "https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks",
      "type": "official_docs"
    },
    {
      "id": "github-webhooks-handling-revalidation-20261004",
      "url": "https://docs.github.com/en/webhooks/using-webhooks/handling-webhook-deliveries",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# Webhook の durable な受領と業務処理を分ける

## 今回の訂正と問い

受信した webhook に速く2xxを返すため、永続キューへの投入を後回しにしてよいか。**採用する event は durable な受領を確認してから ack し、重い業務処理はその後へ分離する**。以前の本書が推奨した「2xx → durable queue」では、その間にprocessが停止すると、送信側は成功扱いなのに受信側には仕事が残らない。これを本書の設計上の誤りとして訂正する。

合わせて Stripe の署名対象に余分な `t=` を含めていた式、GitHub の再送を「手動のみ」と狭めた説明、event IDと外部副作用を一つのlocal transactionで守れるように読めた推奨、object ID + event.typeによる無条件の重複排除、常に最新版へ置換する復元方針を修正した。日付だけを更新せず、[Stripe webhook][stripe]とGitHubの[署名検証][gh-verify]・[best practices][gh-best]・[再配送][gh-redeliver]・[受信tutorial][gh-handle]の全対象箇所を2026-10-04 UTCに再確認した。

以下の「確認した契約」は各社の記載事実、「設計案」は本書が障害ケースから導く判断である。durable inboxの構成やexactly-onceをプロバイダの保証として記載しない。

## 確認した契約: signature verification の入力を変えない

### Stripe: header の表記と署名対象は異なる

`Stripe-Signature` headerではtimestampに `t=`、有効なlive署名に `v1=` が付く。一方、HMAC-SHA256のメッセージは **timestamp文字列 + '.' + raw body** であり、`t=` prefixは含めない。raw bodyを改変せず、endpoint signing secretで検証する。比較にはconstant-timeを使う。[Verify webhook signatures manually][stripe-manual]

Stripe libraryのtimestamp toleranceは既定5分で、0はrecency checkの無効化。再配送ごとにtimestampと署名が作られるため、署名が新しいことは初回配信の証明ではない。鍵のrotationでは旧secretの失効を最大24時間遅らせられる。[Preventing replay attacks / Roll endpoint signing secrets][stripe]

設計案: JSONの空白・改行・Unicode表現を変えた再シリアライズ結果で検証しない。署名検証用のraw bytesと、検証後の業務解析結果を分ける。`t=1700000000` を抽出したなら、署名対象の先頭は `1700000000.` であって `t=1700000000.` ではない。この数値は本書の説明用であり、実際の認証情報ではない。

### GitHub: secret があることを前提に検証する

[Validating webhook deliveries][gh-verify]は、webhook secretとpayloadからHMAC-SHA256を計算し、`sha256=` + hex digestである `X-Hub-Signature-256` と照合する。secret未設定ではheaderは付かない。旧 `X-Hub-Signature` はHMAC-SHA1のlegacy用であり、新headerの代わりに自動採用しない。

同資料はUTF-8、payload / headerをproxy等で改変しないこと、素の `==` を避けたconstant-time比較を指示する。公式の確認用vectorはsecret `It's a Secret to Everybody`、payload `Hello, World!`、期待値 `sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17`。これは公開されたテスト入力であり、本番secretではない。[Testing the webhook payload validation][gh-verify]

設計案: header欠落を「検証不要」や成功へ変換しない。署名の成否は出所・改変の検査であり、処理済み判定や業務実行の許可判定とは別に扱う。鍵や不要なraw payloadを通常の監視ログへ流さない。

GitHubが説明するHMAC入力はpayload本文である。その署名一致だけから、`X-GitHub-Delivery` 等の全headerが暗号学的に結び付いているとは推論しない。delivery IDの台帳は通常の再配送の重複排除に使えても、headerを変更する攻撃者を含む完全なreplay防止の証明にはならない。業務effectの一意性や許可範囲も独立に検査する。

## 確認した契約: acknowledgment と再送の範囲

| 項目 | Stripe | GitHub |
|---|---|---|
| 応答時間 | timeoutを招く複雑な処理より前に速やかに2xx | 受信から10秒以内に2xx。遅れると接続を終了し失敗扱い |
| 非同期処理 | queueによる処理を推奨 | queue等でbackground処理する構成を案内 |
| 送信側の自動再送 | liveで最大3日、指数backoff。sandboxは数時間で3回 | GitHub自身は失敗配信を自動再送しない |
| 利用者が要求する再配送 | Dashboardはevent作成から15日、CLIは30日。手動再送の2xxは自動retryを停止させない | 過去3日の配信。repository / organization / GitHub App等ではWeb UIまたはREST APIを使える |

根拠: [Stripe delivery behaviors][stripe-delivery]、[GitHub respond within 10 seconds][gh-best]、[GitHub redelivery][gh-redeliver]。Stripeの「最大3日」は必着保証ではなく、destinationの無効化・削除時には将来retryが止まる条件もある。GitHubのREST API経由の再配送を自社の回復jobで呼ぶ設計は可能であり、「GitHubが自動再送しない」と「人手でしか再送できない」を混同しない。

[GitHub Handling tutorial][gh-handle]の例は202を受領応答として記述し、`X-GitHub-Event` とpayloadの `action` で処理を分ける。これはtutorialであり、durable queueのcommitを保証する例ではない。特にソース上でstatusを設定した行の位置だけから、実際のwire送信完了時刻まで断定しない。

## 独自の設計案: durable receipt-before-ack

ここからは両社の応答時間と再送契約を前提にした受信側の障害安全設計である。

### 受領・実行中・完了を別に保存する

採用するeventの基本順序を **raw body保持 → 署名検証と対象確認 → durable inboxへ保存 → 2xx → workerで業務処理** とする。inboxは永続DBでもdurable queueでもよいが、「enqueue関数が返った」ことだけで耐久性を推測しない。選んだstoreのcommit / producer確認と障害モデルを照合する。不要なeventを意図的に無視する方針と、採用したeventを保存せず失うことは別である。

短い受領transactionでeventの識別子、処理に必要なpayloadまたは再取得可能な参照、対象tenant / endpoint、状態 `received` を記録する。業務処理は `processing` を経て `processed` へ移す。`received` があるだけで処理済みと判定しない。workerが途中で止まった仕事には再取得・再試行の経路を設ける。

| crash / 通信失敗の位置 | 回復方針 |
|---|---|
| durable保存前 | 採用eventの受領成功を返さない。非2xx / 切断を失敗として扱い、provider別の再配送・照合へつなぐ |
| durable保存後、ackが相手に届く前 | 再配送を想定し、同じeventのinbox重複を検出する。未処理なら内部の仕事を残す |
| ack後、worker開始前 | providerのretryに依存せず、自社inboxから再開する |
| 業務effectの適用後、processed記録前 | effectの所在に応じた冪等性・照合が必要。後述のlocal / external境界で分ける |

保存に失敗した場合、10秒を守るためだけに成功を返して処理を失うことはしない。ただし、GitHubでは失敗を返しても自動再送されない。受信応答の失敗と、回復手段の有無を別に監視する。ackを返した後のworker失敗も、自社queueのretry・隔離・アラートで扱う。

DB inboxから別brokerへ配送する場合、DB commitとbroker publishの間にも窓ができる。受領した仕事はDB側から再走査できるようにするか、[transactional outbox](../messaging/transactional-outbox.md)のように送信意図を同じDB transactionへ残す。プロセス内メモリへのhandoffだけを唯一の起動条件にしない。

### duplicate detection の単位を二段にする

[Stripe][stripe-duplicates]は処理したevent IDを記録して再処理を避ける方法を案内し、別Eventとして生じる重複には `data.object` のIDと `event.type` を使うとしている。[GitHub][gh-best]では `X-GitHub-Delivery` がeventの配信を識別し、要求した再配送でも同じ値である。これはattemptごとのnonceではない。

独自案として、まずprovider / account・endpoint等の必要なscopeとevent ID（GitHubではdelivery ID）の組で受領の重複を扱う。同じIDの再受領時に `received` / `processing` / `processed` を区別し、「IDを見たことがある」だけで未完の処理を捨てない。重複で2xxを返せるのは、必要な仕事をdurableに所有できている場合である。

次に、業務effectの重複を別のキーで制御する。たとえば「一つの注文へ一度だけ権利付与」はその業務操作のIDで判断する。**object ID + event.typeだけを永続的なunique keyにして、以降の正当な別更新を捨てない**。同じobjectの同じ種類のeventでも、異なる変更を処理すべきworkflowがあるためである。providerの重複識別の助言を、全event typeに共通する業務一意性の保証へ広げない。イベント別契約・業務ID・版などから同一effectを証明できなければ、無条件に同一視しない。

### local transaction と外部effectを分ける

業務データ更新とprocessed記録が**同じDB transactionに参加できる場合**は、そのtransactionで両方をcommitし、必要なunique制約で並行workerも制御する。ローカルDB内の状態更新についての設計であり、ネットワーク先の処理まで原子的になるわけではない。

メール送信、外部API変更、別storeへの書込みでは、自社DBのprocessed記録だけでexternal effectをexactly-onceにできない。外部側が対応する業務冪等キーを安定して再利用する、外部operation IDで結果を照合する、送信意図をoutboxへ残す、といった方法を相手の契約に合わせて選ぶ。outboxだけでは「外部effect成功・返信喪失」の重複を消せない。相手が重複排除や結果照合を提供しなければ、未確定として隔離し、無条件再実行と処理済み扱いのどちらにも寄せない。

## 確認した契約と設計案: event時点と現在状態

Stripeは配信順序を保証せず、不足objectをAPI取得する方法を案内する。またsnapshot handlerではevent時点の `data.object` を使う経路と、最新resourceを取得する経路を区別する。[Event ordering][stripe-order] / [Snapshot event handler][stripe-snapshot]

独自案として、最新の表示やキャッシュの収束が目的なら現在objectの再取得を使える。一方、監査・遷移ごとの通知・変更当時の値に依存する処理では、**最新objectは履歴snapshotの代用品ではない**。古いeventが遅れて届いたとき、最新版をそのevent時点の事実として記録しない。thin notificationとsnapshotのpayload契約も区別し、必要な履歴を取得できるか確認してから復元方式を決める。

`created` やsignature timestampは本書のduplicate detection keyにしない。署名timestampは配送試行の鮮度用であり、event IDではない。createdのような時刻だけでは、同時刻の別eventや業務遷移の完全な順序を一意に決められない。これは本書の設計判断であり、「Stripeが全用途のtimestamp利用を禁止した」という主張ではない。

通知が来ない時間が続いただけでは、event自体がなかったのか配送を失ったのか区別できない。欠測監視は、必要な業務event・provider側delivery記録・自社inbox・業務完了を照合する形にする。opaqueなevent IDの大小や隣接から連番の穴を推測しない。現在状態の照合で回復できる用途と、過去eventを再配送・履歴取得しなければ回復できない用途を分ける。

## 運用前の受入れ試験案

以下は設計案であり、実際のStripe / GitHub deliveryやworkerで検証済みではない。

| 入力・故障 | 期待する観測 |
|---|---|
| JSONをparse・再シリアライズしてraw bodyを変える | 署名検証が不一致となり業務処理しない |
| Stripe signed_payloadへ余分なt=を挿入 | 正しいtimestamp + '.' + raw bodyとの差を検出する |
| GitHub secret未設定でheaderなし | 未検証のまま採用しない |
| inbox commit前に停止 | 成功ackを返しておらず、provider別の回復対象として追跡する |
| inbox commit後にack喪失、同じIDが再配送 | inboxは一つ。未処理のworker仕事は消えない |
| ack後にworker停止 | received状態から内部回復し、providerの自動retry待ちだけにしない |
| 同じIDを二つのworkerが同時取得 | local効果と完了記録の重複をtransaction / unique制約等で抑える |
| 外部API成功直後に返信喪失 | external effectをlocal transactionで済んだことにせず、相手のキー・operation状態で照合する |
| 同じobject ID・event.typeで正当な別更新が二回 | 無条件の永久dedupで二回目を捨てない |
| 古いsnapshot受信時に現在objectが更新済み | event時点と現在時点を混ぜて履歴へ保存しない |
| GitHubの受信失敗後 | provider自動再送を待つだけにせず、許可済みREST再配送等の回復手順へ進む |
| Stripeの手動再送と自動retryが両方到着 | 同じ受領識別子 / 業務effectをそれぞれ適切な単位で照合する |

通常運用では未処理inboxの滞留、受領失敗、worker再試行、外部effect未確定、重複受領を別々に観測する。HTTPSと証明書検証を有効にし、IP allowlistingを使うならproviderの現在の一覧へ追従する。GitHubは `GET /meta` と定期更新を案内するが、allowlistだけを署名検証の代替にしない。[GitHub best practices][gh-best] / [Stripe Verify events][stripe]

## 適用版・provenance・未確認事項

- Stripeのevent destination API例にある `2026-09-30.preview` はその例の版であり、webhook署名や全eventに必要な最小API版と解釈しない。GitHub4ページは固定revision・独立した公開日を確認できないrolling docs。取得日は全5資料2026-10-04 UTCであり、機能導入日ではない。
- 5資料をnative webで実際に開いて確認した。既存の共有catalog recordは変更せず、新しい5recordを参照する。公式記述の引用と独自の障害安全設計を区別するため、文書trustをprimary-sourceとした。
- 確認した各ページだけでは再利用に適用するopen licenseを特定できず、新sourceのlicenseはunknownとした。サンプルコードや図を転載せず、独自要約・設計案に限定する。GitHubの公開検証vectorは相互運用の確認値として示した。moduleやコードの変更はない。
- 未確認: 各providerの全event typeの業務一意性、thin / snapshot全形式の互換性、内部retry時刻表、受信側storeの耐久性設定、外部APIの冪等契約と保持期間、実配信と障害注入。GitHubの再配送可能期間を超えた履歴復元も今回保証しない。
- 受信inbox・dedup記録の保持期間は自社のreplay / 回復期間とデータ削除方針で決める。TTLを短くすれば古い再配送の重複を見逃し得るが、無期限保存も一律の正解ではない。providerの再配送期間だけで業務effectの一意性が消えるとは決めつけない。
- 検索evalは文書が指定queryで取得できることだけを確認する。表の障害試験や公開vectorの実行結果とは区別する。official_docsの90日TTLにより再確認期限は2027-01-02。

[stripe]: https://docs.stripe.com/webhooks
[stripe-manual]: https://docs.stripe.com/webhooks#verify-webhook-signatures-manually
[stripe-delivery]: https://docs.stripe.com/webhooks#event-delivery-behaviors
[stripe-duplicates]: https://docs.stripe.com/webhooks#handle-duplicate-events
[stripe-order]: https://docs.stripe.com/webhooks#event-ordering
[stripe-snapshot]: https://docs.stripe.com/webhooks#snapshot-event-handler
[gh-verify]: https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
[gh-best]: https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks
[gh-redeliver]: https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks
[gh-handle]: https://docs.github.com/en/webhooks/using-webhooks/handling-webhook-deliveries

---
{
  "id": "api-design-webhook-redelivery-signature-dedup",
  "title": "Webhook の再送保証と署名検証・重複排除: 受信側の acknowledgment と処理完了の分離、再配信順不同への対応",
  "kind": "knowledge",
  "technology": "api-design",
  "version": "Stripe Docs webhooks ページ (イベント宛先の API 例は Stripe-Version 2026-08-26.preview)、GitHub Docs webhooks 4ページ (版表記なし)、いずれも 2026-09-27 取得",
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
    "github"
  ],
  "sources": [
    {
      "id": "stripe-webhooks-docs",
      "url": "https://docs.stripe.com/webhooks",
      "type": "official_docs"
    },
    {
      "id": "github-docs-validating-webhook-deliveries",
      "url": "https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries",
      "type": "official_docs"
    },
    {
      "id": "github-docs-best-practices-webhooks",
      "url": "https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks",
      "type": "official_docs"
    },
    {
      "id": "github-docs-redelivering-webhooks",
      "url": "https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks",
      "type": "official_docs"
    },
    {
      "id": "github-docs-handling-webhook-deliveries",
      "url": "https://docs.github.com/en/webhooks/using-webhooks/handling-webhook-deliveries",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# Webhook の再送保証と署名検証・重複排除: 受信側の acknowledgment と処理完了の分離、再配信順不同への対応

Webhook の受信エンドポイントが、署名検証・即時 ack・重複排除・順不同配信をどう契約するかを、2つのプロバイダの公式文書で整理する。署名と再送の契約は [Stripe Docs — Receive Stripe events in your webhook endpoint](https://docs.stripe.com/webhooks)、署名検証手順は [GitHub Docs — Validating webhook deliveries](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries)、10秒制約と配信 ID は [GitHub Docs — Best practices for using webhooks](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks)、手動再送の範囲は [GitHub Docs — Redelivering webhooks](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks)、受領応答の例は [GitHub Docs — Handling webhook deliveries](https://docs.github.com/en/webhooks/using-webhooks/handling-webhook-deliveries) を2026-09-27に取得して確認した。以下で「記載事実」「設計案」を明示的に分ける。

## 要点（公式文書の記載事実）

### 署名検証: raw body 上の HMAC と constant-time 比較

- Stripe: 受信エンドポイントは POST JSON を受け、`Stripe-Signature` ヘッダを `whsec_` の signing secret で検証する。検証は raw body に対して行う必要があり、**body を操作すると検証は失敗する**。署名は HMAC-SHA256 で、`t=<timestamp>.<raw payload>` に対して計算される。live mode で有効な scheme は `v1` のみ。
- Stripe の replay 対策: ライブラリは timestamp の許容（tolerance）を既定で5分にしている。tolerance 0 はこの再検査（recency check）を無効化する。**再送のたびに新しい署名と新しい timestamp が生成される**ため、署名の新しさは「初回受信」の証拠にならない。タイミング攻撃に抗するための constant-time な文字列比較が推奨される。
- GitHub: 各配信に `X-Hub-Signature-256` ヘッダが付く。中身は `sha256=` に HMAC hex digest を連結したもので、webhook secret を鍵として payload に対して SHA-256 で計算する。比較は constant-time に行う（例: `crypto.timingSafeEqual`、`hmac.compare_digest`、`Rack::Utils.secure_compare`）。素の `==` は使わない（タイミング攻撃で秘密を推測されるリスク）。payload は UTF-8 として扱い、検証前にプロキシ等で改変されていてはならない。
- GitHub の周辺契約: secret を設定していないと `X-Hub-Signature-256` ヘッダは**そもそも付かない**。旧 `X-Hub-Signature`（HMAC-SHA1）は後方互換のためにのみ存在する。公式のテストベクタ: secret `It's a Secret to Everybody`、payload `Hello, World!` なら `sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17`。

### acknowledgment と処理完了の分離

- GitHub: 受信側は**10秒以内に 2xx を返さないと** GitHub は接続を終了し、その配信を失敗として扱う（Best practices、Handling の公式例コメント）。Handling の例は受領時に `202` を返してから実作業を行う順序を示す。Best practices は受領を acknowledge してから queue などで非同期処理する方法を推奨する。イベント種別は `X-GitHub-Event` ヘッダ、action はトップレベルの `action` キーから得る。
- Stripe: エンドポイントは複雑な処理より**先に 2xx を速やかに返す**必要がある。つまり acknowledgment は処理完了に先行する。だからこそ受信側は「応答は成功したのに、実処理は遅れて完了したり、再送で複数回完了したりする」状況を受け入れる前提になる。イベントの処理はキューでの非同期処理が推奨される。

### 再送の契約はプロバイダごとに違う

- Stripe（live mode）: 自動再送は最大3日間、指数バックオフで実行される。sandbox は数時間で3回。手動再送は Dashboard から15日以内、CLI から30日以内。**手動再送が 2xx を返しても自動再送は停止しない**。
- GitHub: 失敗した配信の**自動再送はない**。再送は手動のみで（Web UI または REST API）、対象は過去3日以内の配信（repository / organization / GitHub App / Marketplace / Sponsors webhooks）。Best practices は停止後に取りこぼした配信を再送する手順を案内する。
- 共通して、再送は元と同じ payload が**別の HTTP リクエストとして再び届く**ことである。GitHub の再送では `X-GitHub-Delivery` は元配信のまま同じ値が残る。

### 順不同・重複は仕様として許容される

- Stripe は**配信順序を保証しない**。イベントが生成された順に届くと前提してはならない。別の snapshot イベントは同じ `created` 秒を共有しうるので、`created` を順序判定にも重複判定にも使ってはならない。
- 重複排除は、処理した event ID を記録しておき、記録済みの event をスキップする方法が示される。さらに、**2つの異なる Event オブジェクトが同じ underlying change を表す場合**があり、その識別には `data.object` の ID と `event.type` の組を使う。
- 順序に頼らない欠落検出として、不足したオブジェクトは API から取得する。
- GitHub の `X-GitHub-Delivery` はイベントごとに配信を一意に識別し、replay 攻撃の guard に使われる。ただし手動再送でも値は変わらないため、これは「配信（イベント）の識別子であって、attempt ごとの nonce ではない」。

## 推奨方法（独自の設計案。上記文書の規定ではない）

- 受信処理は「raw body をパース前に保持 → 署名検証 → 2xx 即時応答 → durable なキューへ投入 → 非同期で実処理」という順序にする。10秒制約（GitHub）と「処理より先に 2xx」（Stripe）の両方に合う形で、acknowledgement と処理完了を構造的に分離する。
- 署名検証は constant-time 比較を必須にし、Stripe は tolerance を既定の5分のまま運用する（0 にするのは検証用途外の特殊な場合）。timestamp は再送ごとに変わるため、重複判定キーには使わない。
- 冪等性は event ID ベースで実装する。処理と同じローカルトランザクションで処理済み event ID を記録し、記録済みならスキップする（[トランザクショナル・アウトボックス](../messaging/transactional-outbox.md)の MessageLog 方式と共通）。Stripe を使う場合はさらに `data.object` ID + `event.type` の組で「別の Event だが同じ変更」を検出できるようにする。
- 順序依存の復元をやめ、状態の正はオブジェクトの最新版を API から取得して確定する。受信順・`created`・signature timestamp を信頼しない。
- 欠落検出を受信側の責任に置く。GitHub は自動再送しないため、署名検証済みの配信が来ないこと自体を監視対象にし（配信間隔の想定は自組織で定義）、停止分は手動再送または API 取得で補う。Stripe は event ID を軸に欠落を検出し、必要なら API で取得する。
- 秘密鍵のローテーションは重複期間を作る。Stripe は複数の active secret を最大24時間重ねられるので、新しい鍵を有効化してから旧鍵を失効させる手順にする。検証は許容される複数鍵を試す実装にする。
- IP allowlisting と HTTPS を併用する（Stripe は IP allowlisting、GitHub は `GET /meta` の一覧で IP allow list を設定し、SSL 検証付き HTTPS を推奨）。

## 避ける使い方

- 受け取った body をパース・再シリアライズしてから署名検証する（Stripe: body を操作すると検証が失敗する）。検証前にプロキシが payload を書き換える構成（GitHub: 改変されてはならない）。
- 署名比較に素の `==` を使う（GitHub 明示: constant-time 比較が必須）。
- `created` や signature timestamp で順序判定・重複判定を行う（Stripe が明示的に不可と記載）。
- 2xx を「処理完了」とみなすこと。ack は受領の確認にすぎず、Stripe は ack 後の遅延・重複した完了を前提にしている。GitHub の10秒制約内に実処理を完了させようと inline で処理すること。
- プロバイダの自動再送を前提に欠落検出を省くこと（GitHub は自動再送なし。Stripe の自動再送も最大3日に限られ、手動再送は範囲が別）。
- `X-Hub-Signature-256` の欠如を検証成功として処理すること（secret 未設定時はヘッダ自体が付かない）。
- 手動再送を「初回配信」として処理すること、または逆に自動再送が手動再送の 2xx で止まる想定で設計すること（Stripe: 手動再送は自動再送を停止しない）。
- `X-GitHub-Delivery` を attempt ごとの一意値としてリプレイ nonce に使うこと（再送でも同一値が残る）。
- tolerance 0 で timestamp の再検査を無効化したまま運用すること（replay 対策の recency check が外れる）。

## 適用版と本番での注意

- 適用版: Stripe Docs の webhooks ページは2026-09-27取得で、イベント宛先の API 例は Stripe-Version `2026-08-26.preview`。GitHub Docs の4ページは版表記なし（同一日取得）。記載内容は各社の Webhook 契約であり、IETF・W3C の標準仕様ではない。
- 再送範囲の差は実運用に直結する。Stripe live は自動再送3日 + 手動15/30日、GitHub は自動再送なし + 手動3日。「届くはず」の想定は使っているプロバイダごとに設計する。
- 署名スキームの差: Stripe は `t=` と `v1=` を持つ独自形式（live で有効は `v1` のみ）、GitHub は `sha256=` の hex digest。共通するのは raw payload に対する HMAC-SHA256 と constant-time 比較で、ヘッダ名・正規表現・鍵の持ち方はプロバイダ固有。
- 未確認・範囲外: 各社の配信順序保証・重複排除・リトライ時刻表の内部実装は非公開で未確認。GitHub のイベント種別ごとの payload スキーマ、Stripe の `data.object` 識別が適用できない event type の範囲、受信側で推奨する event ID 保持期間（再送可能期間との整合）は本調査で確認していない。
- 再確認期限: 全 source が official_docs（TTL 90日）で、技術（api-design）固有 TTL は設定されていない。2026-12-26 に両社の文書を再取得し、再送条件・署名仕様・header 名の変更を確認する。

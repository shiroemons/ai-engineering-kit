---
{
  "id": "oauth-token-exchange-delegation-audience-revocation-boundary",
  "title": "OAuth Token Exchange: downstream の権限縮小・actor・失効を別々に設計する",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 8693 (January 2020) + RFC 8707 (February 2020) + RFC 9700 / BCP 240 (January 2025); verified 2026-10-03 UTC",
  "tags": [
    "research-domain:security",
    "oauth",
    "RFC8693",
    "token-exchange",
    "delegation",
    "downscoping",
    "audience",
    "resource",
    "scope",
    "act",
    "may_act",
    "revocation"
  ],
  "sources": [
    {
      "id": "rfc8693-token-exchange-policy-20261003",
      "url": "https://www.rfc-editor.org/rfc/rfc8693.html",
      "type": "official_docs"
    },
    {
      "id": "rfc8707-resource-audience-mapping-20261003",
      "url": "https://www.rfc-editor.org/rfc/rfc8707.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9700-downstream-privilege-boundary-20261003",
      "url": "https://www.rfc-editor.org/rfc/rfc9700.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# OAuth Token Exchange の downstream 認可境界

## 調査した問い

利用者 U の要求を受けるサービス A が、請求 API B と文書 API C を呼ぶとき、受け取った token を交換するだけで「必要な権限だけを渡す」「代理実行者を識別する」「ログアウト時に派生 token も止める」が成立するか。

本稿の判断は、token exchange を導入する際に、宛先・権限・代理関係・失効の契約を明示すること。交換成功だけをその代わりにしない。既存の [authorization code の引換え](authorization-code-pkce-state-redirect.md) や [refresh token rotation](refresh-token-rotation-reuse-detection.md) とは別の、RFC 8693 の `urn:ietf:params:oauth:grant-type:token-exchange` を扱う。新しい仕様変更の報告ではなく、未収録だったサービス間交換の実務上の隙間を2026-10-03 UTCに確認した記録である。

## 確認した仕様上の契約

### RFC 8693: 交換の形式と、別途決める認可

[RFC 8693 §1・§2.1](https://www.rfc-editor.org/rfc/rfc8693.html#section-2.1) は STS 型の要求・応答を定めるが、配備の trust model を規定しない。AS は `subject_token` と、存在すれば `actor_token` を各 token 型に従って検証する MUST がある。client authentication の採否・方式は配備側の判断であり、省略は盗まれた token を別 token に交換される危険を増やす。

[§2.1.1](https://www.rfc-editor.org/rfc/rfc8693.html#section-2.1.1) の要求権限は、全 target services と全 scope の **Cartesian product（直積）**。複数の `resource` / `audience` と `scope` は、宛先別に異なる権限を割り当てる組の表現ではない。

[§4.1・§4.4](https://www.rfc-editor.org/rfc/rfc8693.html#section-4.1) の `act` は代理実行者、`may_act` は代理できる者の表明。認可判断では top-level claims と最外側の current actor のみを考慮する MUST があり、nested act の prior actors は履歴情報にとどまる。

[§2.1](https://www.rfc-editor.org/rfc/rfc8693.html#section-2.1) によれば、一回限り等の token 固有の意味がなければ交換は入力 token の有効性を変えない。入出力に密な連動は作られず、入力の更新や延長は出力へ反映される想定ではない。revocation の伝播も実装・token 型・配備に依存する。

### resource は宛先を識別し、scope は許可内容を表す

[RFC 8707 §1–2](https://www.rfc-editor.org/rfc/rfc8707.html#section-2) では、scope が主に「何を許可するか」を示す一方、resource は「どこで使うか」の情報を AS に渡す。resource は absolute URI、fragment は禁止、query は SHOULD NOT。一般には対象 API の適切な base URI を使う SHOULD がある。必ずしもネットワーク上の取得先 URL ではない。

AS は指定 resource への audience restriction を SHOULD とするが、token の audience に resource の文字列をそのまま使うことも、別識別子へ mapping することもできる。したがって、要求パラメータと token の `aud` が常に同一文字列になる、または URL prefix が合えばよい、とは導けない。RFC 8693 の resource は query を MAY としており、両 RFC の要件表現も同一視しない。

### 制限は発行と利用の両側で成立させる

[RFC 9700 §2.3](https://www.rfc-editor.org/rfc/rfc9700.html#section-2.3) は必要最小限の権限と、単一 RS、難しければ少数 RS への audience restriction を SHOULD とする。RS は毎回の要求で宛先を検証し、対象でなければ拒否する MUST がある。resource と action の許可も要求ごとに検証する。

[§2.2.1](https://www.rfc-editor.org/rfc/rfc9700.html#section-2.2.1) の sender-constraining は SHOULD で、audience 制限とは別の保護である。代理者名を記録したことを秘密鍵の所持証明の代用にしない。[DPoP の別文書](dpop-proof-binding-nonce-replay.md) で扱う鍵結合を利用する場合にも、B/C での業務上の認可は残る。

## A→B/C の設計判断（独自の提案）

以下は上の契約を踏まえた本稿の設計案であり、RFC が一律に要求する token 格納形式や設定値ではない。例では A を認証済みの交換 client、B/C を別の RS とし、ユーザー不在の継続処理は許可しない構成から始める。

### 1. AS に「交換を許可する組」を用意する

AS の policy は単なる有効 token 判定から分け、認証済み client A、入力の issuer と subject、許可する downstream、対象操作、代理関係、寿命をまとめて審査する。入力と出力で scope の語彙が違うなら、文字列の集合包含だけを downscoping の証明にしない。「案件閲覧」から「当該案件の請求参照」への意味上の mapping を定義する。

A に token endpoint への接続資格があることと、任意の U を任意のサービスへ代理できることを分ける。policy 未登録の組は拒否する。別の issuer が同じ `sub` を使った場合の取り違えを防ぐため、主体の識別に issuer の文脈を残す。client identity と actor identity を別に扱う配備では、その許可された関係も明示する。

### 2. downstream ごとに要求と cache を分ける

B には `invoice:read`、C には `document:write` が必要だとする。これを一つの要求へ平坦にまとめず、B 用と C 用の交換に分けることを初期方針にする。AS の audience mapping と RS の受理設定は、登録済みの対応表で管理する。新しい path や tenant を追加しただけで既存 audience の権限が拡大しないかレビューする。

交換後 token を cache するなら user U だけを key にしない。少なくとも issuer/subject の文脈、client/actor、宛先、許可範囲、配備で使う tenant を分離する設計とする。キャッシュ再利用の条件は要求の相関 ID とは別に検証する。key の具体形式、暗号化、保管期間は本稿では実装しない。

### 3. RS と監査で identity の役割を固定する

B/C は署名等の token 型固有の検証に加え、受理 issuer、audience、必要な操作と対象データの条件を照合する。サービス A で受理済みであることを、そのまま B/C の認可結果へ転用しない。

監査では subject U と current actor A を別項目にし、「誰のために」「誰が」の区別を保つ。過去の actor に管理サービスがいた、という履歴だけで現在の要求を昇格させる規則は作らない。`may_act` を採用する場合も、AS が信頼して受理する issuer と配備ポリシーの下で評価し、任意の入力 JSON の表明を許可証として扱わない。

### 4. 停止までの時間を別契約にする

この例の初期方針は、出力を短命にし、その期限を検証済み入力の残存期間より長くしないこと。これは本稿の方針であり、RFC 8693 全体の必須条件ではない。入力が opaque で期限を A が読めないなら、AS 側で制御し、A に未検証の decode 値を採用させない。

ログアウトや権限剥奪後に何秒以内で B/C が停止すべきかを先に定める。その要件が token の自然失効まで待てないなら、出力識別子・grant 関係を保持する revoke 連携や、RS が現在の有効性を確認する仕組みを別途選定する。採用時には cache、配信遅延、確認先障害時の扱いまで含めて保証を検証する。どれも導入しない構成では、即時停止を保証したという運用説明をしない。

## 導入レビューの反例（テスト案、未実行）

| 境界 | 投入する条件 | 本稿の設計で確認する結果 |
|---|---|---|
| 交換 client | 正しい U の token を未登録 client D が提示 | 有効性だけで通さず、交換 policy で拒否 |
| downscoping | B と C を同時指定し、両方の scope を平坦に送る | この配備の単一宛先方針で拒否し、意図しない組を発行しない |
| audience mapping | B 用 token を C または別 tenant へ提示 | 文字列 prefix や署名成功だけで許可しない |
| nested act | 過去 actor は管理者、current actor は非許可 | 履歴に基づく権限昇格が起きない |
| cache | 同じ U から B/C を連続要求 | user-only key による token 取り違えがない |
| revoke 連携 | 入力失効直後、交換済み token を複数 replica へ提示 | 採用した停止時間の上限と障害時方針を実測できる |

これらは相互運用試験や負荷試験の結果ではなく、設計をレビューするための反例である。検索 eval はこの文書を発見できることだけを確認する。

## 適用版・未確認事項・由来

- RFC 8693 は2020年1月、RFC 8707 は2020年2月の Standards Track 文書。Security BCP は RFC 9700 / BCP 240、2025年1月。本稿はこの3本文の確認範囲に限定する
- 特定の AS/SDK が actor token、複数宛先、opaque token、派生 token 失効を実装するか、既定で何を許可するかは未確認。RFC 対応の製品表示だけから、本稿の配備方針が満たされるとは判断しない
- クロスドメイン federation、JWT client authentication の方式比較、交換後 refresh token の運用、独自 impersonation API は対象外。これらを使う場合は別途 profile と製品版を確認する
- 3件とも公式 RFC 本文を開いて取得した。copyright notice の BCP 78 / IETF Trust Legal Provisions を確認。RFC 8693/8707 の code components 表記は Simplified BSD、RFC 9700 は Revised BSD。本稿は独自の日本語要約と設計提案で、ソースコード・HTTP 例・JWT 例をコピーしていない。repository analysis は行っていないため commit SHA は該当しない
- catalog は既存 record を変更せず、今回確認した新 ID を追加した。取得日は2026-10-03 UTC、official_docs の90日を適用した再確認期限は2027-01-01

---
{
  "id": "oauth-step-up-auth-time-action-binding-boundary",
  "title": "OAuth step-up: 新しいトークン・新しい認証・操作への承認を分ける",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 9470 (September 2023); RFC 9068 (October 2021); OIDC Core errata set 2 (2023-12-15); Mozilla accounts guide (2026-08-27)",
  "tags": [
    "research-domain:security",
    "oauth",
    "step-up",
    "acr_values",
    "auth_time",
    "max_age",
    "refresh-token",
    "action-binding"
  ],
  "sources": [
    {
      "id": "rfc9470-step-up-freshness-20261024",
      "url": "https://www.rfc-editor.org/rfc/rfc9470.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-core-step-up-refresh-20261024",
      "url": "https://openid.net/specs/openid-connect-core-1_0.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9068-authentication-claims-20261024",
      "url": "https://www.rfc-editor.org/rfc/rfc9068.html",
      "type": "official_docs"
    },
    {
      "id": "mozilla-step-up-session-action-20261024",
      "url": "https://mozilla.github.io/ecosystem-platform/relying-parties/how-tos/step-up-authentication",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2027-01-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# OAuth step-up: 新しいトークン・新しい認証・操作への承認を分ける

## 問いと適用範囲

ログイン済みのユーザーに重要な設定変更を許すとき、「refresh したばかり」「認可 redirect が成功」「MFA 済み」を、その操作に必要な本人確認の証拠として使っていないか。認証の強度、経過時間、操作への結び付けを別々に評価する。

本稿は既存標準の実務上の欠落を埋める調査であり、2026年10月の新リリースではない。[refresh token rotation](refresh-token-rotation-reuse-detection.md) の漏えい検知、[introspection](introspection-jwt-active-freshness-cache-boundary.md) の失効状態とは別の問いを扱う。MCP の scope を追加する step-up authorization と、ユーザー認証を強化する RFC 9470 の step-up authentication も同じではない。

## 確認した契約

### 1. challenge は認証要件の伝達であって、満たした証明ではない

[RFC 9470 §3–6](https://www.rfc-editor.org/rfc/rfc9470.html#section-3) は、リソースサーバー（RS）が `insufficient_user_authentication` と `WWW-Authenticate` の `acr_values` / `max_age` で不足を知らせる方式を定義する。前者は受理する認証コンテキストの選択肢、後者は最後の能動的認証から許す秒数であり、両方を要求できる。scope 不足も同時に存在し得る。

client は challenge の条件を認可サーバー（AS）への新しい authorization request に反映する。RS は戻った access token の認証情報を検証する。単なる再送成功や新 token 発行を代替証拠にしない。RFC 9470 §5 は、要求した acr を満たせない場合に失敗させる動作を SHOULD として推奨するが、すべての AS がその動作を保証するとは扱わない。方式・業務要件・利用可能な端末を決める方針は仕様外である。

### 2. OIDC の任意要求と必須要求を混ぜない

[OIDC Core §3.1.2.1 / §5.5.1.1](https://openid.net/specs/openid-connect-core-1_0.html#acrSemantics) では `acr_values` による ID Token の acr 要求は Voluntary。希望値に届かない認証状態で応答され得る。claims parameter 対応の実装で、ID Token の acr に `essential: true` と特定 value/values を指定する場合は別で、満たせなければ認証失敗になる。同じ acr を `acr_values` と個別 claims 要求の両方で指定した場合の動作は未規定。

`max_age` を使うと返される ID Token に `auth_time` が必要になる。これは token の発行時刻 `iat` とは別。Core §12.2 では refresh で ID Token が返り、auth_time を含む場合も、元の認証時刻を保持する。新しく発行したから本人確認が新しくなったとは言えない。

### 3. access token の有効性と認証の鮮度は独立して検証する

[RFC 9068 §2.2.1 / §4](https://www.rfc-editor.org/rfc/rfc9068.html#section-2.2.1) の認証情報 `auth_time` / `acr` / `amr` は OPTIONAL claim である。含める場合の値は、同じ authorization response から派生する refresh や token exchange を経ても不変だが、すべての token に存在するとは限らない。欠落を過去の token から補わない。`iat` が新しく `exp` 以内でも、認証が古い token は存在する。JWT access token の署名・issuer・audience・期限・型を検証する通常のゲートは省略しない。ID Token を RS 用 access token として代用しない。

RFC 9470 では JWT claim または introspection response を通じて auth_time / acr を運べる。auth_time は Unix epoch からの秒。access token の更新を、追加要素の提示や再認証の代わりにしない。

### 4. 実装固有の session freshness は、操作単位の承認と違う

[Mozilla accounts の公式ガイド](https://mozilla.github.io/ecosystem-platform/relying-parties/how-tos/step-up-authentication) は2026-08-27更新の rolling 文書である。ここでの `AAL2`、`max_age=0` に対する5秒の猶予、未知 acr 値の無視は Mozilla 固有の説明で、OAuth 全体の既定値ではない。

同ガイドでは認証イベントがブラウザーの Mozilla accounts session を共有する RP 間で共用され、別 RP での認証でも鮮度を満たし得る。max_age は grant 発行時に評価されるため、RS 側でも利用時の鮮度を判定する必要がある。特定操作への結び付けはアプリ側の責務であり、短い max_age だけで代替できない。さらに同ガイドは、Mozilla では refresh による token に acr / auth_time / amr を含めず、再昇格には新しい authorization flow が必要だと明記する。`active: true` だけで昇格済みとは判定できない。これは一般的な claim の値の不変性と、製品での claim の有無を分ける例である。文書確認であり、実アカウントでの動作を試験したわけではない。

## 採用する場合の設計案

以下は本稿独自の実装・運用案であり、標準が一律に要求するアルゴリズムではない。

1. 操作ごとに許容する issuer、acr 集合、認証からの最大秒数、許容 clock skew を決める。acr は任意文字列なので、数字や辞書順による大小比較で強度を推測しない。
2. token 自体の検証と、対象ユーザー・テナント・操作権限を先に確認する。本稿は情報漏えいを抑えるためこの順序を選ぶ。RFC 9470 §9 は token 検証前の challenge も許すが、要求条件の漏えいに注意を求める。
3. 検証済み auth_time が欠落、型不正、許容範囲を超える未来時刻なら受理しない。現在時刻との差を秒で計算し、認証鮮度を検査する。秒とミリ秒を混ぜない。別 token の auth_time と acr を寄せ集めない。
4. 条件不足なら信頼済み AS の既知の authorization endpoint へ誘導する。challenge 内の文字列から任意のログイン先を作らない。取消し・満たせない条件・同じ不足の繰返しで停止し、無限 redirect loop を避ける。
5. 重要な単一操作には、サーバー側に短命の操作意図を保存し、ユーザー、対象、変更内容、nonce、期限へ結び付ける。戻り時に元の session / subject と照合し、内容が変われば再確認する。使用済み意図の再利用を拒否し、実処理の冪等性も別に管理する。
6. 成功判定から業務処理まで長時間 queue に置く場合、いつの時点で鮮度を要求するかを決める。認証条件を満たしたことを、ユーザーがその具体的変更を理解して承認した証拠と混同しない。

## 受入試験の案

- refresh 後に iat だけが新しくなっても、古い auth_time の操作は拒否する。
- max_age は満たすが acr が不足する場合、および acr は満たすが認証が古い場合を別々に拒否する。
- Voluntary acr 要求に AS が別値を返す場合、redirect 成功だけで通さない。
- 別 issuer / audience の token、別ユーザーの戻り、未来の auth_time、欠落した claim を受理しない。
- 別 RP で直前に認証されても、対象操作の確認を省略しない。操作意図の二重使用と変更後の再利用を拒否する。
- AS が必要条件を実現できないとき、無限 loop にせず理由を返す。

これらは設計上の受入条件であり、追加した検索 eval は文書が取得できることだけを検証する。認証サーバーへの適合性試験や攻撃試験を実行したという意味ではない。

## 確認範囲・未確認事項

2026-10-24 UTC に四つの一次資料を確認。RFC 9470 errata の取得は RFC Editor で429、代替検索ページも取得不可だったため、errata が存在しないとは判断しない。本稿は公開本文を基準にする。Mozilla ガイドの記述と一般的な OIDC 動作を同一視せず、実導入時に使用する AS/RS 製品の profile と claim 契約を確認する。

本文は独自要約と設計案であり、出典コードの転載・module 化なし。Mozilla 文書の適用ライセンスは未確認として catalog に unknown を記録した。AS の実設定、時刻同期、RP 間のポリシー、操作意図の保存実装、障害時の許容動作は未検証。

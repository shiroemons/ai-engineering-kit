---
{
  "id": "oauth-bff-cookie-csrf-proxy-boundary",
  "title": "OAuth BFF の境界: RFC 10017 の cookie・API CSRF・proxy allowlist と残存する client hijacking",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 10017 (BCP 212, August 2026) + RFC 9700 (BCP 240, January 2025)",
  "tags": [
    "research-domain:security",
    "oauth",
    "BFF",
    "cookie",
    "CSRF",
    "SameSite",
    "CORS",
    "allowlist",
    "client-hijacking"
  ],
  "sources": [
    {
      "id": "rfc10017-browser-bff-20261001",
      "url": "https://www.rfc-editor.org/rfc/rfc10017.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9700-bff-callback-csrf-20261001",
      "url": "https://www.rfc-editor.org/rfc/rfc9700.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-12-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# OAuth BFF の cookie・API CSRF・proxy 境界

## 問いと更新理由

SPA の token を backend に移せば、どの攻撃を防げて、何を別途検証する必要があるか。2026年8月公開の RFC 10017 は browser-based OAuth の BCP として BFF を整理した。本稿は token 保存場所の選択だけでなく、cookie 認証の API と outbound proxy の境界を対象にする。既存の PKCE・refresh rotation・DPoP 文書の再説明はしない。

## 確認した契約

### BFF と token-mediating backend

[RFC 10017 Sections 6.1–6.2](https://www.rfc-editor.org/rfc/rfc10017.html#section-6.1) の BFF は confidential client として Authorization Code を利用し、token を browser のアプリへ直接渡さず、API 通信を中継する。token-mediating backend は access token を frontend に返し、API へ直接アクセスさせる別構成である。同じ安全性とは扱えない。

### Cookie と API CSRF

[RFC 10017 Sections 6.1.3.2–6.1.3.3](https://www.rfc-editor.org/rfc/rfc10017.html#section-6.1.3.2) は Secure と HttpOnly を MUST、SameSite=Strict・Path=/ を SHOULD、Domain 設定を SHOULD NOT とする。HTTP 設定を示す cookie 名 prefix（例 __Host-Http-）も SHOULD。CSRF 防御自体は MUST である。

SameSite は same-origin ではない。兄弟 subdomain がある配置では Strict だけを十分とみなせない。CORS のみでも safelisted request は送信され得る。CORS を防御に使う場合、custom header を要求することが SHOULD、この方式を採る BFF はすべての incoming request にその static header があることを MUST で確認する。

### Outbound と残存リスク

[RFC 10017 Section 6.1.3.6](https://www.rfc-editor.org/rfc/rfc10017.html#section-6.1.3.6) は転送先 host の allowlist 検証を MUST とし、動的 proxy でも明示許可した host/path に制限する。任意 URL 中継は token 漏洩につながる。

[Section 6.1.4](https://www.rfc-editor.org/rfc/rfc10017.html#section-6.1.4) の限界として、悪意ある同一 origin の JavaScript による client hijacking は残る。HttpOnly は XSS による認証済み操作まで停止する仕組みではない。

### OAuth callback の CSRF は別に確認する

[RFC 9700 Sections 2.1 / 4.7.1](https://www.rfc-editor.org/rfc/rfc9700.html#section-4.7.1) の CSRF は redirection endpoint に対する攻撃を扱う。AS の PKCE 対応を確認した client は PKCE の保護に依存できる。対応未確認なら state または OIDC nonce が必要で、PKCE 非対応 AS では state または OIDC nonce を MUST で使用する。state にアプリ状態を運ぶ場合は改ざん・入替えを防ぐ。confidential client の PKCE は Section 2.1.1 で RECOMMENDED。これを、ログイン後の cookie-authenticated API に CSRF 防御が不要という意味に拡張しない。

## 実装時の判断と検証案（本リポジトリ独自の設計提案）

1. API 一覧を「OAuth navigation/callback」「session API」「resource proxy」に分類する。callback は通常の navigation なので、全 URL に JavaScript 専用 header を機械的に要求する設計は避ける。各 endpoint に合う検証を定義し、例外が proxy の抜け道にならないことを確認する。
2. Cookie 属性を設定する層を一つに決め、TLS 終端や reverse proxy の後ろでも実レスポンスを確認する。開発環境だけの動作から本番を推定しない。callback 復帰時の session 対応付けと SameSite 設定の両立も実ブラウザで検証する。
3. 同じ site の別 origin、完全な別 site、許可 origin の三種類で API テストを作る。header のない POST、通常の form submission、拒否された preflight から副作用が発生しないことを観測する。CORS エラー表示だけでは、サーバ処理が止まった証拠にならない。
4. Proxy の route は server 管理の固定対応表を基本にする。未登録 host、許可 host 上の未許可 path、encoding や path traversal、redirect 応答を含め、token の付与先が変わらないことを試験する。redirect の追従可否はライブラリ依存なので、暗黙の既定値を採用しない。
5. Browser の network response とログに token を返していないことを確認する。token を返す endpoint を追加する変更は BFF の境界変更としてレビューし、単なる frontend 最適化として扱わない。
6. XSS 対策と API ごとの業務認可を独立に残す。BFF 導入だけで「ブラウザ内の攻撃者がユーザー権限を使えない」とする受入条件は設定しない。

## 避ける判断

- callback の state/PKCE 検証を、すべての業務 API の CSRF 対策の代わりにする
- SameSite と same-origin を同一視し、運用していない兄弟 subdomain を無視する
- 「CORS を有効化済み」という設定名だけで、simple request の拒否を確認しない
- frontend が渡す URL をそのまま転送し、あとから認証 header を付ける
- token-mediating backend を BFF と呼び、ブラウザへ返した access token の露出を評価から落とす

## 適用範囲・未確認事項

RFC 10017 は August 2026 公開の BCP 212、RFC 9700 は January 2025 公開の BCP 240。両原文を2026-10-01 UTCに取得した。新しいのは BCP の公開であり、各対策がこの月に初めて発明されたとは主張しない。

本稿は framework 固有の middleware や製品の既定値を検証していない。cookie prefix のブラウザ別対応、AS の PKCE enforcement、logout/失効の伝播、session 保存方式の性能、実装 proxy の DNS/redirect 処理は未確認。個別実装と対象ブラウザで追加確認する。ライブラリ横断で安全を保証するコードは含めない。

原文の normative 要件と上記の設計・テスト提案を区別するため trust は primary-source とした。RFC の copyright notice と [IETF Trust Legal Provisions](https://trustee.ietf.org/documents/trust-legal-provisions/) を確認し、原文コードの転載はしない。official_docs の90日 TTL に合わせ再確認期限を2026-12-30とする。

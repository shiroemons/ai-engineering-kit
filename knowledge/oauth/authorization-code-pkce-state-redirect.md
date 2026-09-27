---
{
  "id": "oauth-authorization-code-pkce-state-redirect",
  "title": "OAuth Authorization Code flow with PKCE, state, redirect_uri validation and token exchange",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 6749 (Oct 2012) + RFC 7636 (Sep 2015) + RFC 9700 BCP 240 (Jan 2025)",
  "tags": ["research-domain:security", "oauth", "authorization-code", "pkce", "code_verifier", "code_challenge", "S256", "state", "csrf", "redirect_uri", "redirect", "token", "grant_type", "authorization_code", "injection", "downgrade"],
  "sources": [{"id": "rfc6749-oauth2-framework", "url": "https://www.rfc-editor.org/rfc/rfc6749.html", "type": "official_docs"}, {"id": "rfc7636-pkce", "url": "https://www.rfc-editor.org/rfc/rfc7636.html", "type": "official_docs"}, {"id": "rfc9700-oauth-security-bcp", "url": "https://www.rfc-editor.org/rfc/rfc9700.html", "type": "official_docs"}],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# OAuth Authorization Code flow with PKCE, state, redirect_uri validation and token exchange

Authorization Code grant (RFC 6749 Section 4.1) に PKCE (RFC 7636) と RFC 9700 (BCP 240, January 2025) の security 対策を組み合わせた client 実装の要点。認可要求 (authorization request) の `response_type=code`、`code_challenge`、`state`、`redirect_uri` と、token 交換 (token exchange) の `grant_type=authorization_code`、`code`、`code_verifier` の扱いを範囲とする。

## 要点 (文書化された事実)

### 認可要求と code の交付 (RFC 6749 Sections 4.1, 4.1.1)

- Authorization Code flow は authorization endpoint への認可要求から始まり、`response_type=code` を使う (Section 4.1.1)。
- resource owner が access を grant すると authorization server は `code` (authorization code) を登録済み redirection endpoint に返す (Section 4.1)。
- 認可要求の必須級パラメータは `response_type`、`client_id`、`redirect_uri`、`scope`、`state` である (Section 4.1.1)。`state` は client が request と callback の対応付けと CSRF 対策に使う (Section 4.1.1, Section 10.12)。

### redirect_uri の登録と照合 (RFC 6749 Sections 3.1.2-3.1.2.4; RFC 9700 Section 2.1)

- redirection endpoint は事前登録が前提であり、要求の `redirect_uri` は登録値と照合される。登録・照合と無効 endpoint の扱いは Sections 3.1.2-3.1.2.4 に規定される。
- RFC 9700 Section 2.1 は `redirect_uri` の exact-string matching (完全一致照合) を要求し、open redirector (開放 redirect 先) を redirect 先に使ってはならないとする。
- RFC 6749 Section 10.6 は redirect 操作・endpoint 偽装の脅威を、Section 10.15 は client impersonation 時の `redirect_uri` 照合の役割を扱う。

### PKCE の verifier/challenge (RFC 7636 Sections 4.1, 4.2, 7.1, 7.2)

- `code_verifier` は unreserved 文字集合 (`[A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~"`) で長さ 43-128 文字 (Section 4.1)。十分な entropy が必要で、256-bit のランダム値を使う指針が Section 7.1 にある。
- S256 の `code_challenge` は `BASE64URL-ENCODE(SHA256(ASCII(code_verifier)))` で導出する (Section 4.2)。S256 が MTI (MUST 実装) の method であり、`plain` は S256 を利用できない client との互換目的に限定される (Section 7.2)。
- 認可要求では `code_challenge` と `code_challenge_method` を送り (Section 4.3)、token 要求では `code_verifier` を送って authorization server が challenge と照合する (Section 4.4)。検証失敗時の error 応答は `invalid_request` / `invalid_grant` で扱われる (Sections 4.5-4.6)。

### token 交換 (RFC 6749 Section 4.1.3)

- token endpoint への token 要求は `grant_type=authorization_code`、`code`、認可要求と同じ `redirect_uri` を含める (Section 4.1.3)。`redirect_uri` は認可要求に含めた場合に token 要求でも REQUIRED で、同一値でなければならない。
- authorization code は single use (一度きり) で、発行 client に bind される。code が漏洩・再利用された場合の扱いは Section 4.1.2 の失効規定に従う。

### RFC 9700 の client/AS 要件 (Sections 2.1, 2.1.1, 4.5, 4.7, 4.8)

- public client は PKCE を MUST で使う。confidential client も PKCE を SHOULD で使うべきであり、使う場合は S256 のみ (Section 2.1.1)。
- authorization server は PKCE を MUST で support し、client 種別に応じた enforced PKCE を行い、PKCE downgrade attack (攻撃者が `code_challenge` を除去・`plain` に切替える攻撃) を mitigation する (Section 2.1.1, Section 4.8)。
- authorization-code injection (攻撃者の code を被害者の session に注入する攻撃) への対策として、client は `state` の照合と PKCE の `code_verifier` の紐付け確認を行う (Section 4.5)。CSRF 対策として `state` (または同等の per-request 値) の検証を行う (Section 4.7)。

## 推奨方法 (上記からの設計上のまとめ)

- public client (SPA・mobile・CLI 等、secret を保持できない client) では Authorization Code + PKCE (S256) を必須構成にする。`code_verifier` は 256-bit 乱数から 43 文字以上で生成し、認可要求ごとに使い捨てる。
- `state` は認可要求ごとに暗号学的乱数で生成し、server 側 session (または署名付き cookie) に保存して callback 時に値の一致を検証する。`state` の検証省略は CSRF と code injection の経路になる。
- `redirect_uri` は完全一致で事前登録した固定値のみを使い、query の動的組立てや open redirector を redirect 先にしない。token 要求では認可要求と同一の `redirect_uri` を送る。
- token 要求の `code_verifier` は認可要求の `code_challenge` に対応する値を送り、`invalid_grant` が返った場合は verifier/challenge の不一致・code 再利用・`redirect_uri` 不一致を疑う。`plain` への fallback は行わない。
- confidential client でも PKCE (S256) を併用する (RFC 9700 Section 2.1.1 の SHOULD)。client authentication (secret 等) は PKCE の代替にならない。

## 避ける使い方

- Implicit flow や Resource Owner Password Credentials flow の新規採用。RFC 9700 で非推奨であり、本書の Authorization Code + PKCE 構成を使わない。
- PKCE なしの public client。authorization code が redirect 経路で漏洩した場合に token 交換を止められない。
- `plain` の `code_challenge_method` の常用。S256 が使える環境での `plain` は downgrade 余地を残す (互換目的に限定)。
- `state` の固定値使い回し・検証省略。CSRF (Section 4.7) と code injection (Section 4.5) の対策にならない。
- `redirect_uri` の部分一致・正規表現マッチ・運用時のワイルドカード登録。exact-string matching に反し、open redirect 経由の code 漏洩につながる。
- authorization code の再利用・複数 token endpoint への使い回し。single-use に反し、失効・`invalid_grant` の対象になる。
- error 応答 (`invalid_request` / `invalid_grant`) の詳細を end user 画面にそのまま表示すること。攻撃者の試行に情報を与えるため、log に残して汎用 error を返す。

## 適用版と本番での注意

- `RFC 6749 (Standards Track, October 2012)`、`RFC 7636 (Standards Track, September 2015)`、`RFC 9700 (BCP 240, January 2025)` の公開ページ (2026-09-27 取得) で確認した。将来も最新とは扱わない。
- 本文は `expires_at` 2026-12-26 (official_docs TTL 90 日)。
- RFC 9700 は RFC 6749/7636 の個別規定を置き換える BCP であり、両立しない場合は BCP 側の要件 (exact match、enforced PKCE、S256 限定) を優先する。
- **未確認**: 各言語・framework の OAuth client library が enforced PKCE・S256 限定・`state` 検証を既定で満たすか、authorization server 側の PKCE enforcement 設定項目、DPoP・PAR・JAR 等の追加対策 (RFC 9700 の他節)。実装前に対応表を公式文書で確認する必要がある。
- 本文中の MUST/SHOULD/MAY は仕様の normative 表記の引用であり、本リポジトリの推奨事項 (「推奨方法」節) と区別している。

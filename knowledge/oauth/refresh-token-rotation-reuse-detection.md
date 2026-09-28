---
{
  "id": "oauth-refresh-token-rotation-reuse-detection",
  "title": "OAuth refresh token rotation and reuse detection semantics (RFC 9700, RFC 6749, RFC 7009)",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 9700 (BCP 240, January 2025) + RFC 6749 (October 2012) + RFC 7009 (August 2013)",
  "tags": ["research-domain:security", "oauth", "refresh-token", "rotation", "reuse-detection", "token-family", "invalid_grant", "revocation", "sender-constrained", "authorization-server", "public-client", "confidential-client", "grant"],
  "sources": [{"id": "rfc9700-oauth-security-bcp", "url": "https://www.rfc-editor.org/rfc/rfc9700.html", "type": "official_docs"}, {"id": "rfc6749-oauth2-framework", "url": "https://www.rfc-editor.org/rfc/rfc6749.html", "type": "official_docs"}, {"id": "rfc7009-oauth-token-revocation", "url": "https://www.rfc-editor.org/rfc/rfc7009.txt", "type": "official_docs"}],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# OAuth refresh token rotation and reuse detection semantics (RFC 9700, RFC 6749, RFC 7009)

refresh token の rotation (更新ごとの差し替え) と reuse detection (再利用・再生の検知) の仕様上の意味論を、RFC 9700 (BCP 240, January 2025) Section 2.2.2 / Section 4.14、RFC 6749 (October 2012) Section 6 / 5.2 / 10.4、RFC 7009 (August 2013) の規定から整理する。AS は authorization server。IETF RFC は "token family" という語を使わないため、family は実装概念として扱う (根拠は RFC 9700 Section 4.14.2 の implementation note と RFC 7009 の grant-based cascade)。

## 要点 (文書化された事実)

### RFC 9700 Section 2.2.2: public client には sender-constrained か rotation が MUST

- RFC 9700 Section 2.2.2: "Refresh tokens for public clients MUST be sender-constrained or use refresh token rotation as described in Section 4.14"。public client の refresh token は、sender-constrained (RFC 8705 / RFC 9449) か rotation のどちらかが MUST。
- 同節は、RFC 6749 が confidential client の refresh token を発行元 client 以外には使わせないことを既に要求している点を前提にしている。

### RFC 6749 Section 6: refresh 要求の必須項目と permissive な rotation

- token endpoint への refresh 要求は `grant_type=refresh_token` (REQUIRED)、`refresh_token` (REQUIRED)、`scope` (OPTIONAL)。scope を含める場合、元の grant で許可されたものにない scope は MUST NOT。
- refresh token は発行元 client に bind される。confidential client は token endpoint で認証が MUST。AS は (a) confidential client に client 認証を要求し、(b) 資格情報が示された場合は認証して、その認証済み client に refresh token が発行されたものであることを確認し、(c) refresh token を検証する。
- AS MAY issue a new refresh token。発行した場合、client MUST discard the old one and replace it。AS は新しい token の発行後に古い refresh token を revoke することができる (MAY)。新しい refresh token の scope は提示されたものと同一でなければならない。
- つまり RFC 6749 の rotation は MAY であり必須ではない。reuse 検知や失効の連鎖 (family revocation) の挙動は Section 6 に規定がない。

### RFC 6749 Sections 5.2, 10.4: invalid_grant と濫用検知の基盤規定

- Section 5.2: `invalid_grant` は、grant または refresh token が "invalid, expired, revoked, does not match the redirection URI used in the authorization request, or was issued to another client" のいずれかに該当する場合に返す。
- Section 10.4: refresh token MUST は機密保持され、発行元 client とのみ共有される。AS MUST は token-client binding を維持し、client 身份を認証できる場面ではいつでも検証する。client auth ができない場合、AS SHOULD は濫用検知の他の手段を講じる。例として、前の refresh token を無効化しつつ保持し、無効化済み token が提示されたことを攻撃の兆候とする方式が挙がる (提示者が攻撃者か正規 client かは判別できない)。
- この RFC 6749 の例は「無効 token の提示が AS に breach を知らせる」点のみで、active token の revoke は要求していない (active token の revoke は RFC 9700 Section 4.14.2)。
- AS MUST refresh token が生成・改ざん・推測できないようにする。

### RFC 9700 Section 4.14: rotation の必須化と reuse 検知の意味論

- Section 4.14.1 の基盤は RFC 6749 の保護: 通信・保存での機密性、TLS、refresh 時の client binding 確認、unguessability。
- Section 4.14.2:
  - AS MUST は refresh token の発行を risk assessment で判断する。発行する refresh token は、同意された scope と resource server に MUST で bind される。
  - public client では AS MUST、sender-constrained (RFC 8705 / RFC 9449) か rotation のどちらかを用いて replay を検知する。
  - rotation の意味論: refresh 応答ごとに新しい refresh token を発行し、前の token は無効化するが、その関係情報は AS が保持する。
  - reuse 時: 攻撃者と正規 client の双方が同じ token を使うと、いずれか一方が無効化済み token を提示することになり、その提示自体が breach を AS に知らせる。AS はどちらの当事者による提示か判別できないが "will revoke the active refresh token" とする。攻撃はここで止まり、代償として正規 client は新しい authorization grant を取得し直す必要がある。
  - implementation note: grant を refresh token に符号化しておけば、AS は失効が必要なすべての refresh token を効率よく見つけられる (grant / family 単位の一括失効)。AS MUST token の integrity を確保する (例: 署名)。
  - refresh token は inactivity 後に期限切れにするべき (SHOULD。期間は AS の裁量)。AS MAY パスワード変更やログアウトで自動的に revoke する。

### RFC 7009: grant 単位の連鎖失効

- revocation 要求は "will invalidate the actual token and, if applicable, other tokens based on the same authorization grant and the authorization grant itself" — 対象 token のほか、同一 authorization grant に基づく token とその grant 自体を無効化できる。
- 実装 MUST は refresh token revocation を support し、SHOULD は access token revocation を support する。
- client 検証と token の所有確認が済むと失効は即時 (伝播の遅延を除く)。token が無効でも AS は HTTP 200 を返す。
- revoked token が refresh token なら AS SHOULD 同一 grant に基づくすべての access token を無効化する。access token が渡された場合は関連する refresh token を revoke する MAY。cascade 方針は server 依存で、RFC 6749 Section 6 はこの cascade を要求していない。RFC 6749 の client は予期しない token 無効化に備えていなければならない。

用語注記: "token family" は IETF RFC の用語ではない。family 単位の一括失効は、RFC 9700 Section 4.14.2 の implementation note (grant の符号化) と RFC 7009 の grant-based cascade を実装した場合の概念として扱う。

## 推奨方法 (上記からの設計上のまとめ。仕様の引用ではない)

- public client は sender-constrained を導入できない場合、rotation を既定にする (RFC 9700 Section 2.2.2 の MUST)。confidential client には RFC 6749 の client binding と token endpoint 認証が既に要求されており、rotation の併用は本リポジトリの設計判断とする。
- refresh token に grant ID (または family ID) を符号化するか、server 側で generation の関連を保持し、reuse 検知時に同一 grant の active な refresh token とその grant 由来の access token をまとめて revoke する (RFC 9700 Section 4.14.2 の implementation note と RFC 7009 の cascade に沿う)。token integrity は署名などで確保する。
- 無効 token の提示ログを breach 検知シグナルとして監視し、検知時は RFC 9700 に従い active token を revoke する。
- client 側は refresh 応答で新しい refresh token を受けたら旧 token を即座に破棄して置き換える (RFC 6749 Section 6 の MUST)。refresh 要求に含める scope は現行 grant の範囲に限定する。
- `invalid_grant` が返ったら保存済み grant を破棄し再認可へ戻る。revoke の連鎖で正規 client が巻き込まれた場合の復旧経路 (自動再認可または明示的な再ログイン) を用意する。
- refresh token は機密情報として扱い、ログ・URL・クエリに載せない (RFC 6749 Section 10.4)。

## 避ける使い方

- public client で rotation も sender-constrained も導入しないこと (RFC 9700 Section 2.2.2 の MUST 違反)。
- RFC 6749 Section 6 の MAY を根拠に rotation を任意と扱い BCP に従わないこと。RFC 9700 は public client で MUST にしている。
- rotation 適用後も旧 refresh token を client が保持し再送すること。無効 token の再提示として reuse 検知を自ら引き起こし、family revoke の対象になる。
- reuse 検知で無効 token の提示を検出しても active token を revoke せず見過がすこと。RFC 9700 の revoke 要件を満たさず、攻撃が継続する。
- RFC 6749 Section 6 だけを根拠に「無関係な token は失効しない」と client が前提にすること。RFC 7009 の cascade は server 依存で、client は予期しない失効に耐える必要がある。
- refresh token を第三者と共有したり平文保存したりすること (RFC 6749 Section 10.4 の機密保持義務違反)。

## 適用版と本番での注意

- 確認した版: RFC 9700 (BCP 240, January 2025; Updates: RFC 6749, 6750, 6819)、RFC 6749 (Standards Track, October 2012)、RFC 7009 (Standards Track, August 2013)。catalog の取得日は RFC 9700 / RFC 6749 が 2026-09-27、RFC 7009 が 2026-09-28。本文の明示期限は 2026-12-26 (official_docs TTL 90 日)。将来も最新とは扱わない。
- RFC 9700 は RFC 6749 を更新する BCP であり、両立しない場合は BCP 側 (public client の rotation or sender-constrained、reuse 検知時の active token revoke) を優先する。
- MUST / SHOULD / MAY は仕様の normative 表記の引用であり、「推奨方法」節の本リポジトリの設計案と区別している。
- **未確認**: 複数端末やリトライで同時に refresh が走る競合 (正規 client が直前の token を提示してしまうケース) に対する grace period 等の扱いは、確認した節に記載がない。sender-constrained の具体 (RFC 8705 / RFC 9449) は本稿の確認範囲外。実装前に該当 RFC を公式文書で確認する。

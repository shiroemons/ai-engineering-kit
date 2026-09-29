---
{
  "id": "security-oidc-discovery-jwks-kid-rotation",
  "title": "OIDC Discovery document handling with JWKS retrieval, caching scope, and kid rotation",
  "kind": "knowledge",
  "technology": "oidc",
  "version": "OpenID Connect Discovery 1.0 Final + errata set 2 (2023-12-15), OpenID Connect Core 1.0 Final + errata set 2 (2023-12-15), RFC 7517 (May 2015), RFC 7515 (May 2015)",
  "tags": ["research-domain:security", "oidc", "discovery", "openid-configuration", "jwks", "jwks_uri", "issuer", "kid", "rotation", "caching", "jwk", "jws"],
  "sources": [{"id": "oidc-discovery-1-0", "url": "https://openid.net/specs/openid-connect-discovery-1_0.html", "type": "official_docs"}, {"id": "oidc-core-1-0", "url": "https://openid.net/specs/openid-connect-core-1_0.html", "type": "official_docs"}, {"id": "rfc7517-jwk", "url": "https://www.rfc-editor.org/rfc/rfc7517.txt", "type": "official_docs"}, {"id": "rfc7515-jws", "url": "https://www.rfc-editor.org/rfc/rfc7515.txt", "type": "official_docs"}],
  "retrieved_at": "2026-09-29",
  "expires_at": "2026-12-28",
  "trust": "official",
  "status": "active"
}
---

# OIDC Discovery document handling with JWKS retrieval, caching scope, and kid rotation

Relying Party (RP) が OpenID Provider (OP) の Discovery 文書 (`/.well-known/openid-configuration`) と JWKS (`jwks_uri`) を取得・保持し、`kid` で署名検証鍵を選ぶ範囲の要点。ID Token の claim 検証手順 (iss/aud/exp/nonce) 自体は `oidc-id-token-validation` を参照し、本書では扱わない。

## 要点 (文書化された事実)

### Discovery 文書の取得 ([Discovery §4](https://openid.net/specs/openid-connect-discovery-1_0.html))

- Discovery 文書は Issuer Identifier に `/.well-known/openid-configuration` を付した URL への HTTPS GET で取得し、応答は `application/json` の JSON 文書である。
- 取得時は TLS の certificate checking が必須である。
- 応答中の `issuer` は、取得に使った Issuer URL の接頭辞と完全一致しなければならず (MUST)、ID Token の `iss` とも一致する (MUST)。文字列比較は Unicode コードポイントの等価比較で行い、正規化を適用してはならない (MUST NOT, §5)。
- 仕様は Discovery 文書や JWKS の cache TTL を定めない。TTL の値は仕様から導けない。

### `jwks_uri` の扱い ([Discovery §3](https://openid.net/specs/openid-connect-discovery-1_0.html))

- `jwks_uri` は Provider metadata の REQUIRED 項目であり、署名検証鍵の取得先となる `https` URL である。
- 公開する JWK Set に秘密鍵・対称鍵値を含んではならない (MUST NOT)。RP 側は `jwks_uri` から秘密・対称鍵材料を受け取ることを期待しない。

### 署名と鍵伝達の原則 ([Core §10](https://openid.net/specs/openid-connect-core-1_0.html))

- ID Token は JWS Compact Serialization で署名されなければならず (MUST)、`none` は Authorization Endpoint から ID Token を返さない Response Type (例: Authorization Code Flow) かつ Registration で明示要求した場合を除き MUST NOT である。
- `x5u` / `x5c` / `jku` / `jwk` ヘッダで鍵を運ぶことは SHOULD NOT であり、鍵は Discovery / Registration (§10) で事前伝達する。
- 非対称の署名鍵・暗号鍵の rotation は §10.1 / §10.2 の範囲であり、RP は回転後の鍵を `jwks_uri` の公開値から得る。

### JWK Set と `kid` ([RFC 7517](https://www.rfc-editor.org/rfc/rfc7517.txt))

- JWK Set は `keys` 配列を持つ JSON object である。配列に既定の優先順位 (default preference order) はないため、順序を信頼順位とみなしてはならない。
- `kid` (Key ID) は OPTIONAL の case-sensitive な文字列であり、特定の鍵に対応付けるために使う。鍵の rollover 中は選択対象の各鍵に異なる `kid` を付与すべきである (SHOULD)。
- `use` パラメータは鍵用途を示し、`sig` (signature) と `enc` (encryption) が定義されている。

### JWS ヘッダの `kid` と `alg` ([RFC 7515](https://www.rfc-editor.org/rfc/rfc7515.txt))

- JWS ヘッダの `kid` は OPTIONAL の case-sensitive なヒントであり、鍵変更の合図として JWK 側の `kid` と対応付ける。
- `alg` は MUST で存在し、受信者が理解できる値でなければならない。
- `jku` / `jwk` / `x5u` による鍵解決ヘッダは OPTIONAL であり、`jku` / `x5u` の URL 取得には TLS と server-identity validation を要する。

## 推奨方法 (上記からの設計上のまとめ)

- Discovery 文書と JWKS の cache は Issuer 単位 (issuer-scoped) に保持し、異なる Issuer 間で鍵・文書を使い回さない。`issuer` 不一致の文書・鍵は使わず拒否する。
- 署名検証の鍵選択は JWS ヘッダの (`alg`, `kid`) と JWKS 側の対応値で行う。rotation 期間中は新旧どちらの `kid` の token も検証できるよう、OP の公開する両方の鍵を保持する。
- 未知の `kid` や署名検証失敗に遭遇した場合は、JWKS を再取得して一度だけ再試行し、それでも解決しなければ拒否する。無制限の再取得ループにしない。再取得の回数・間隔の規定は仕様にないため自決めする。
- 取得と検証は既製の OIDC/JWT ライブラリに委ね、token 埋め込みの `jku` / `x5u` / `jwk` ヘッダを鍵取得に使わない (SHOULD NOT の範囲)。
- 再取得・再試行の方針は本書の設計上のまとめであり、仕様の引用ではない。

## 避ける使い方

- `issuer` / `iss` の正規化つき比較 (trailing slash 除去・小文字化など)。Discovery §4.3 の完全一致と §5 の無正規化比較に反する。
- 未検証の Issuer の metadata・鍵の使用。別人の Issuer を名乗る文書によるなりすまし (Discovery §7.2 の脅威) の経路になる。
- HTTP での Discovery 取得や TLS certificate checking の省略。
- JWK 配列の順序を優先順位とみなすこと。RFC 7517 は既定の順序を定義しない。
- 単一の `kid` への固定 (pinning) や、TTL なし・再取得手段なしの無期限 cache。rotation 時に正規 token を拒否する。
- 登録外の `alg: none` の受け入れ。例外条件 (Core §2) を満たさない `none` は MUST NOT である。
- token 埋め込み鍵 (`x5u` / `x5c` / `jku` / `jwk` ヘッダ) の信頼。SHOULD NOT とされ、鍵は Discovery / Registration の事前値を使う。

## 適用版と本番での注意

- `OpenID Connect Discovery 1.0 Final + errata set 2 (2023-12-15)`、`OpenID Connect Core 1.0 Final + errata set 2 (2023-12-15)`、`RFC 7517 (Standards Track, May 2015)`、`RFC 7515 (Standards Track, May 2015)` の公開ページで確認した。将来も最新とは扱わない。
- 本文は `expires_at` 2026-12-28 (official_docs TTL 90 日)。実効期限は catalog の取得日が古い source (OIDC 2 件は 2026-09-26 取得) に引きずられる。
- **未確認**: HTTP の `Cache-Control` 等を TTL 根拠にできるか (仕様は TTL を定めないため OP ごとの文書確認が必要)、OP の rotation 時の新旧鍵の並行公開期間、各言語の既製ライブラリの JWKS cache・再取得動作。実装前に対応 OP と利用ライブラリの文書で確認する必要がある。
- 本文中の MUST / SHOULD / MAY は仕様の normative 表記の引用であり、本リポジトリの推奨事項 (「推奨方法」節) と区別している。

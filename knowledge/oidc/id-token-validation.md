---
{
  "id": "oidc-id-token-validation",
  "title": "OIDC ID Token の validation (iss/aud/exp/nonce/signature)",
  "kind": "knowledge",
  "technology": "oidc",
  "version": "OpenID Connect Core 1.0 Final + errata set 2 (2023-12-15)",
  "tags": ["research-domain:security", "oidc", "id-token", "validation", "iss", "issuer", "aud", "audience", "exp", "expiration", "nonce", "replay", "signature", "JWS", "jwks_uri", "azp", "at_hash"],
  "sources": [{"id": "oidc-core-1-0", "url": "https://openid.net/specs/openid-connect-core-1_0.html", "type": "official_docs"}, {"id": "oidc-discovery-1-0", "url": "https://openid.net/specs/openid-connect-discovery-1_0.html", "type": "official_docs"}],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# OIDC ID Token の validation (iss/aud/exp/nonce/signature)

Relying Party は Token Response の ID Token を使う前に OpenID Connect Core 1.0 §3.1.3.7 の手順で validation する。順序は復号→issuer 照合→audience 照合→署名検証→有効期限 (expiration) 確認→nonce 照合である。[Core §3.1.3.7](https://openid.net/specs/openid-connect-core-1_0.html)

## 要点 (文書化された事実)

### 検証手順 (§3.1.3.7, Authorization Code Flow)

1. 暗号化されている場合は Registration 時に指定した鍵・アルゴリズムで復号する。暗号化を交渉済みで平文なら SHOULD で拒否する。
2. issuer (iss) は Discovery で得た Issuer Identifier と完全一致 (MUST exactly match) でなければならない。
3. audience (aud) は `iss` の Issuer に登録した自らの `client_id` を含まなければならず (MUST)、含まない場合や信頼できない追加 audience を含む場合は拒否する (MUST be rejected)。`aud` は単一文字列または配列のどちらでもよい。
4. 拡張で `azp` (authorized party) が存在する場合はその拡張の規定に従い SHOULD で検証する。`client_id` との一致確認を含めてよい (MAY)。
5. Token Endpoint との直接通信で受け取った ID Token に限り、TLS サーバ検証で issuer 検証を代替してよい (MAY)。それ以外の ID Token は JWT ヘッダの `alg` で JWS 署名を MUST で検証し、鍵は Issuer 提供のものを使う (MUST)。`alg` は既定 RS256 または Registration の `id_token_signed_response_alg` が SHOULD。HS256/HS384/HS512 の鍵は `aud` の `client_id` に対応する `client_secret` の UTF-8 オクテットであり、複数値 `aud` と MAC 系の組み合わせの動作は未定義である。
6. 現在時刻は `exp` の表す時刻より前でなければならない (MUST)。`iat` で発行時刻が現在から離れすぎた token を拒否してよい。許容範囲は Client 固有である。
7. Authentication Request で nonce を送った場合、`nonce` クレームの存在と値の一致は MUST である。replay 攻撃の検査は SHOULD で行い、具体的な検出方法は Client 固有である。
8. `acr` を要求した場合は値の適切性を SHOULD で確認する (意味論は仕様の範囲外)。
9. `auth_time` を要求した場合 (または `max_age` 使用時) は値を SHOULD で確認し、最終認証から時間が経ちすぎていれば再認証を求める。

### クレーム定義 (§2)

- `iss` は REQUIRED。`https` の case-sensitive な URL で query/fragment を含まない。
- `sub` は REQUIRED。255 ASCII 文字以内、case-sensitive。
- `aud` は REQUIRED。自 RP の `client_id` を MUST で含む。他 audience を含めてよい (MAY)。
- `exp` は REQUIRED。以降は認証に使ってはならない (MUST NOT)。時計ずれ用の小さな leeway (通常は数分以内) を設けてよい (MAY)。
- `iat` は REQUIRED。発行時刻。
- `nonce` はリクエスト値を無改変で運ぶ case-sensitive 文字列。ID Token に存在すれば Request の値との一致検証が MUST。リクエストに存在すれば OP 側の付与は MUST であり、OP はそれ以外の加工を SHOULD で行わない。リクエスト側の nonce には推測不能な十分な entropy が MUST である。
- `auth_time` は `max_age` 要求時または Essential 要求時は REQUIRED、それ以外は OPTIONAL。
- `azp` は OPTIONAL。存在すれば当事者の `client_id` を MUST で含む。拡張を使わない実装は `azp` を使わず、存在しても無視することが推奨される。
- ID Token は JWS で署名が MUST であり、暗号化する場合は署名してから暗号化する (MUST、Nested JWT)。`alg: none` は Authorization Endpoint から ID Token を返さない Response Type (例: Authorization Code Flow) かつ Registration で明示要求した場合を除き MUST NOT である。`x5u/x5c/jku/jwk` ヘッダの使用は SHOULD NOT で、鍵参照は Discovery/Registration で事前伝達する。

### Discovery 側の検証 ([Discovery §3, §4.3, §5, §7.2](https://openid.net/specs/openid-connect-discovery-1_0.html))

- Provider metadata の `issuer` は `/.well-known/openid-configuration` 取得に使った Issuer URL の接頭辞と完全一致しなければならず (MUST)、ID Token の `iss` とも一致する (MUST)。
- `jwks_uri` は REQUIRED で `https` を使い、署名検証鍵の取得先になる。JWK Set に秘密鍵・対称鍵値を含んではならない (MUST NOT)。
- 文字列比較は Unicode コードポイントの等価比較で行い、正規化を適用してはならない (MUST NOT)。
- 攻撃者が別人の Issuer URL を名乗る metadata を公開するなりすましに備え、RP は上記の issuer 一致を MUST で確認する。

## 推奨方法 (上記からの設計上のまとめ)

- 検証は既製の OIDC/JWT ライブラリに委ね、署名・時刻・audience・nonce を自作チェックで省略しない。特に `aud` の `client_id` 含有と信頼できない追加 audience の拒否、`exp` の厳密確認、送信済み `nonce` の一致確認は省略しない。
- `nonce` はリクエストごとに暗号学的乱数で生成し、セッションに保存して使い捨てにする。Implicit Flow では `nonce` が REQUIRED (§3.2.2.1) である点に注意する (本書の手順は Code Flow §3.1.3.7 を基準とする)。
- `jwks_uri` の鍵はキャッシュし、署名検証失敗時や鍵ローテーション時に再取得できる構成にする。
- ID Token に `at_hash` が含まれる場合は Access Token の検証に MAY で使える (§3.1.3.8、詳細手順は §3.2.2.9)。

## 避ける使い方

- `exp` 切れの受け入れ。`exp` 以降の受け入れは MUST NOT 違反である。
- `aud` 検証の省略。他者向け token の受け入れ (token substitution, §16.11) につながる。
- `nonce` の一致確認や replay 検査の省略。replay 攻撃の余地を残す。
- 登録外の `alg: none` の受け入れ。例外条件 (§2) を満たさない `none` は MUST NOT である。
- `x5u/jku` 等のヘッダ埋め込み鍵の信頼。SHOULD NOT とされ、鍵は Discovery/Registration の事前値を使う。
- `iss`/`issuer` の正規化つき比較 (trailing slash 除去・小文字化など)。Discovery §4.3 の完全一致と §5 の無正規化比較に反する。
- 未検証の issuer の metadata・鍵の使用。なりすまし (Discovery §7.2) の経路になる。
- MAC 系署名 (HS256 等) での複数値 `aud` の前提。動作未定義のためライブラリの文書化された動作を確認せずに使わない。

## 適用版と本番での注意

- `OpenID Connect Core 1.0 Final + errata set 2 (2023-12-15)` および `OpenID Connect Discovery 1.0 Final + errata set 2 (2023-12-15)` の公開ページ (2026-09-26 取得) で確認した。将来も最新とは扱わない。
- 本文は `expires_at` 2026-12-25 (official_docs TTL 90 日)。
- **未確認**: `at_hash`/`c_hash` の計算法 (JWA のハッシュ手順。Core §3.1.3.8/§3.2.2.9 の原文確認が必要)、Hybrid Flow 各 validation 節の差分 (§3.3.2.12/§3.3.3.7)、UserInfo 応答検証 (§5.3.4)、各言語の既製ライブラリが上記 MUST を満たすか。実装前に対応表を公式文書で確認する必要がある。
- 本文中の MUST/SHOULD/MAY は仕様の normative 表記の引用であり、本リポジトリの推奨事項 (「推奨方法」節) と区別している。

---
{
  "id": "oidc-token-hash-eddsa-xof-validation-boundary",
  "title": "OIDC token hash: EdDSA の鍵条件と SHAKE256 出力長を固定する",
  "kind": "knowledge",
  "technology": "oidc",
  "version": "OpenID Connect Token Hash Algorithm Implementer's Guide 1.0 draft 00 (2026-09-04); Core 1.0 errata set 2 (2023-12-15); RFC 9864 (2025-10); RFC 9964 (2026-05)",
  "tags": ["research-domain:security", "oidc", "token-hash", "at_hash", "c_hash", "EdDSA", "Ed25519", "Ed448", "ML-DSA", "SHAKE256", "XOF", "algorithm-negotiation"],
  "sources": [
    {"id": "oidc-token-hash-guide-draft00-20260904-20261002", "url": "https://openid.net/specs/openid-connect-token-hash-algorithms-1_0.html", "type": "official_docs"},
    {"id": "oidc-token-hash-guide-status-20261002", "url": "https://openid.net/wg/connect/specifications/", "type": "official_docs"},
    {"id": "oidc-core-hash-flow-errata2-20261002", "url": "https://openid.net/specs/openid-connect-core-1_0-errata2.html", "type": "official_docs"},
    {"id": "rfc9864-fully-specified-hash-boundary-20261002", "url": "https://www.rfc-editor.org/rfc/rfc9864.html", "type": "official_docs"},
    {"id": "rfc9964-mldsa-hash-boundary-20261002", "url": "https://www.rfc-editor.org/rfc/rfc9964.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# OIDC token hash: EdDSA の鍵条件と SHAKE256 出力長を固定する

## 問いと実務上の結論

署名方式を RS256 から Ed25519・Ed448・ML-DSA に変更するとき、`at_hash` / `c_hash` を「alg の末尾の数字に対応する SHA」で計算してよいか。答えは否である。**署名の許可ポリシー、token hash の計算法、claim の必須性を別々に扱う**。特に `EdDSA` は実際に署名を検証した鍵の parameter set が必要で、SHAKE256 は半分に切る前の出力長も契約になる。

本書は [ID Token 全体の検証](id-token-validation.md) が未確認としていた hash 計算法の補足である。issuer・audience・nonce・JWKS の一般的な手順や flow 選定は再定義しない。

## 確認した版と最近の変化

- [Token Hash Algorithm Implementer's Guide](https://openid.net/specs/openid-connect-token-hash-algorithms-1_0.html) の表示版は draft 00、公開日は **2026-09-04**。2026-10-02 UTC に本文・表・Notices を確認した。新しい署名方式に対する hash 選択を明確にする資料であり、ID Token の新 claim や新 validation 手順を導入するものではない。
- [Working Group の仕様一覧](https://openid.net/wg/connect/specifications/) はこれを Implementer's Guides に分類し、必要に応じた継続更新を予定し、Final 化を目的としないと説明している。ページ上の draft 表記や定型の Standards Track 表示だけから **Final Specification 承認済み** と判定しない。
- [RFC 9864 §2.2 / §4.1 / §4.4](https://www.rfc-editor.org/rfc/rfc9864.html#section-2.2) は 2025年10月の Standards Track。JOSE の `Ed25519` / `Ed448` を登録し、曖昧な `EdDSA` 識別子を Deprecated とした。Deprecated は新規採用で代替を優先する意味であり、既存 token を無条件に Prohibited とする意味ではない。
- [RFC 9964 §5 / §8.1.4](https://www.rfc-editor.org/rfc/rfc9964.html#section-5) は 2026年5月の Standards Track。`ML-DSA-44` / `ML-DSA-65` / `ML-DSA-87` の JOSE 登録を確認した。登録済みであることと、利用する OP/RP の対応済みであることは別に検証する。

## 一次資料で確認した計算契約

### 入力と切り取り順序

[Core §3.1.3.6 / §3.2.2.9 / §3.3.2.10](https://openid.net/specs/openid-connect-core-1_0-errata2.html#ImplicitTokenValidation) の構成は、対象値の ASCII octets を digest に入力し、その**左半分のバイト列**を base64url 化するもの。`at_hash` の対象は access token、`c_hash` の対象は authorization code。基準になる `alg` は **ID Token の JWS ヘッダ**である。

[Guide §2 / §4](https://openid.net/specs/openid-connect-token-hash-algorithms-1_0.html#section-2) は padding を付けないこと、および XOF の指定長を確定してから半分を取ることを明示する。`EdDSA` の場合は、署名検証に使用した鍵の `crv` 等から parameter set を決める。`alg` 文字列単独から推測してはならない。

| 署名側の識別子・鍵条件 | hash 側の入力処理 | 半分にした後のバイト数 | padding なしの文字数 |
|---|---|---:|---:|
| RS256 など SHA-256 を使う既存方式 | SHA-256 | 16 | 22 |
| RS384 など SHA-384 を使う既存方式 | SHA-384 | 24 | 32 |
| RS512 など SHA-512 を使う既存方式 | SHA-512 | 32 | 43 |
| Ed25519、または EdDSA + 検証鍵 crv=Ed25519 | SHA-512 | 32 | 43 |
| Ed448、または EdDSA + 検証鍵 crv=Ed448 | SHAKE256 の 114-byte 出力 | 57 | 76 |
| ML-DSA-44 / ML-DSA-65 / ML-DSA-87 | SHAKE256 の 64-byte 出力 | 32 | 43 |

digest 選択は Guide Table 1 の要約、右2列はそこから算出した確認値である。`none` に hash 方式は定義されていない。SLH-DSA の行は WG の決定待ちであり、掲載された推奨値や例を確定した相互運用契約としない。

### claim の有無と照合強度は別の契約

以下は [Core 1.0 errata set 2](https://openid.net/specs/openid-connect-core-1_0-errata2.html) の §3.1.3.6–8、§3.2.2.9–10、§3.3.2.10–11、§3.3.3.6–9 で確認した範囲である。

- Code Flow の Token Endpoint: `at_hash` の発行は OPTIONAL、RP がこれを使う検証は MAY。
- Authorization Endpoint で ID Token と access token を一緒に返す Implicit / Hybrid 応答: `at_hash` は REQUIRED。
- Hybrid の Authorization Endpoint で ID Token と code を一緒に返す応答: `c_hash` は REQUIRED。
- Authorization Endpoint の対象値の検証手順は SHOULD と記され、その手順内で `at_hash` は計算値との一致が MUST。`c_hash` は存在時の一致が MUST。
- Hybrid の Token Endpoint では両 hash を省略できる。両 endpoint の access token は同一とは限らない。

したがって「署名方式を追加したので全 flow の hash を必須にする」「Code Flow の MAY を理由に front-channel の必須 claim も省略する」のどちらも、この guide からは導けない。本書は旧 flow の新規採用を推奨するものではない。

## 実装へ落とす判断（独自の設計案）

### 検証 API に必要な文脈を残す

RP 内部では、少なくとも次を入力として持つ。

1. 利用する profile、flow、response_type、ID Token を受け取った endpoint
2. その ID Token と同じ応答に含まれる対象 access token / code
3. 許可リストとの照合と署名検証を終えた JWS の alg、および実際の検証鍵の識別子・parameter set
4. 対象 claim の存在、型、値

これにより「hash claim が存在するか」「この場面で必要か」「何を何で hash するか」を独立にテストできる。Code Flow でも存在する hash は照合する方針を追加できるが、それは RP/profile の強化ポリシーとして記録し、Core の MUST へ書き換えない。

### 署名の承認と digest 表を分離する

- hash 計算表へ Ed448 や ML-DSA の行を追加しても、署名の許可リストを自動拡張しない。`id_token_signed_response_alg`、ライブラリ、OP、RP、鍵形式の組合せを別に審査する。
- legacy EdDSA を扱う場合、未検証の token 埋め込み `jwk` や、同じ `kid` を持つ別の鍵で hash 方式を決めない。署名検証の結果から鍵条件を渡し、不明な条件は未対応として失敗させる。
- SHAKE256 API の既定出力長に依存しない。Ed448 は 114→57 bytes、ML-DSA は 64→32 bytes と明示し、同じ XOF 名でまとめない。
- RFC 9964 §7.2 は HashML-DSA を定義していない。token hash の SHAKE256 処理を、署名の事前 hash 方式への切替えと混同しない。
- 移行期の legacy EdDSA と fully-specified 識別子を両方受ける期間は、issuer ごとの互換性方針として設定する。「Deprecated だから今日から全件拒否」や無期限の自動 fallback を避ける。

### 避ける変換と成功扱い

- access token が JWT の見た目でも payload を decode して hash しない。API に渡す対象値そのものを使い、Bearer 接頭辞、JSON の引用符、URL の percent-encoding を加えない。
- digest の hex 文字列や base64url 文字列を半分に切らない。切り取るのは digest bytes である。
- 同じ Hybrid transaction の front-channel token と back-channel token を取り違えない。「同じログインなら同じ access token」という仮定を置かない。
- hash 不一致や未対応 alg を、署名だけ通ったことを理由に成功へ落とさない。必要な検証が成立しないことと、claim が仕様上省略可能であることは異なる。
- 成功した hash 照合だけで ID Token 全体や resource server 側の認可が検証できたとは扱わない。ログには対象 token/code の生値を記録しない。

## 確認済みの算術例と受入れ試験案

本書独自の ASCII 入力 `token-for-hash-20261002` で Python 標準ライブラリの hashlib / base64 を用いた計算を2026-10-02に実行した。署名や実 OP/RP を使った検証ではない。

- SHA-256 の左16 bytes: `Zmca1zwLaUoHoOEh8em_mw`（22文字）
- SHA-512 の左32 bytes: `0xMYFuysAlBd1o_EYusMjIzh--GKB5pFF4xhgi_tUGc`（43文字）
- SHAKE256 の114 bytesから左57 bytes: `83KE4VXkTjjoYlVjIh3j6d84oh5r4XV-ZK2ybYItxkyGGN7AdQy2qaIkHUbeE19ghcdBYRDWnzPm`（76文字）
- SHAKE256 の64 bytesから左32 bytes: `83KE4VXkTjjoYlVjIh3j6d84oh5r4XV-ZK2ybYItxkw`（43文字）

追加の実装受入れ試験案（未実行）:

1. EdDSA + Ed25519 と fully-specified Ed25519 が同じ入力で同じ hash を生成する一方、EdDSA + Ed448 は Ed448 の長さになる。
2. Ed448 に ML-DSA 用の64-byte設定を誤適用した claim を拒否する。XOF 出力長を切り取り後の長さと取り違えた実装も検出する。
3. 正しい署名・誤った access token、別応答の code、大小文字変更、padding 追加を使い、採用した照合ポリシーに沿って失敗することを確認する。
4. Code Flow の省略可能な `at_hash` 欠落と、Hybrid `code id_token` の必須 `c_hash` 欠落を別ケースにする。
5. SLH-DSA の暫定行、未知 alg、EdDSA の鍵条件不明を「対応済み」と報告しない。

## 限界・未確認事項・provenance

- この調査は公式ページ5件を実際に開いた原文確認と算術確認に限る。任意の言語ライブラリの対応版、OP/RP 間の接続試験、FAPI の `s_hash` / CIBA の `rt_hash` の必須性、実鍵の署名検証、PQC の安全性評価は未確認。
- guide は更新可能な URL であり、取得時点の draft 00 と日付を記録した。SLH-DSA の決着を推測しない。変化があり得るため明示期限を2026-11-01に短縮した。
- 公開通知メールは検索で見つかったが本文の取得が失敗したため、根拠に採用していない。公開日は取得に成功した guide 自身の表示による。
- Guide Appendix A と Core Appendix C の OpenID Foundation copyright notice、および両 RFC の BCP 78 / IETF Trust Legal Provisions を確認した。WG 一覧ページの独立した再利用ライセンスは未確認。原文の長文、コード、署名鍵を転記していない。
- 規定の MUST/SHOULD/MAY、guide の実装指針、独自設計、実行済み算術例、未実行試験案を区別する。検索 eval の成功は暗号プロトコルの適合性を証明しない。

---
{
  "id": "oidc-rp-metadata-choices-registration-runtime-boundary",
  "title": "OIDC RP Metadata Choices 1.0: 配列の登録入力・単一値の応答・実行時検証の境界",
  "kind": "knowledge",
  "technology": "oidc",
  "version": "OpenID Connect RP Metadata Choices 1.0 Final (2026-03-25); Registration, Discovery and Core 1.0 errata set 2 (2023-12-15); verified 2026-10-02 UTC",
  "tags": [
    "research-domain:security",
    "oidc",
    "rp-metadata-choices",
    "registration",
    "algorithm-negotiation",
    "scalar",
    "array",
    "downgrade",
    "invalid_client_metadata"
  ],
  "sources": [
    {
      "id": "oidc-rp-metadata-choices-final-20260325-20261002",
      "url": "https://openid.net/specs/openid-connect-rp-metadata-choices-1_0-final.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-registration-choices-errata2-20261002",
      "url": "https://openid.net/specs/openid-connect-registration-1_0.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-discovery-rp-op-capabilities-errata2-20261002",
      "url": "https://openid.net/specs/openid-connect-discovery-1_0.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-core-negotiated-alg-validation-errata2-20261002",
      "url": "https://openid.net/specs/openid-connect-core-1_0.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# OIDC RP Metadata Choices: 登録候補と採用済み設定を分ける

## 問いと採用判断

複数の署名方式を利用できる RP が登録値を配列へ広げると、古い OP でも希望した方式が採用され、実行時にも全候補を受け入れてよいのか。本書の判断は、**能力の宣言、登録結果、token の検証設定を別に保存・検査する**ことである。

対象は [RP Metadata Choices 1.0 Final][choices] の **2026-03-25 公開版**。当週の更新ではなく、未収録だった2026年の確定仕様を調べた。比較する Registration / Discovery / Core はいずれも **1.0 incorporating errata set 2、2023-12-15**。既存の [ID Token validation](id-token-validation.md) と [JWKS rotation](../security/oidc-discovery-jwks-kid-rotation.md) の代替ではなく、検証器に渡す登録状態を決める前段を扱う。

## 確認した契約

### 1. 新しい array は登録入力だけ

[Choices §2・§4][choices] の要点は次の通り。

- 追加された複数値パラメータは registration request の input-only。registration response / read response の関連出力は対応する scalar にする（MUST）
- 例として RP の `id_token_signing_alg_values_supported` は検証可能な署名方式の配列。`id_token_signed_response_alg` も送るなら、その単一値は配列に含まれる必要がある（MUST）
- 旧実装との互換性のため、希望する scalar を array と併記することは SHOULD
- scalar が AS 非対応でも別の候補に対応できるなら、それだけをエラーにすべきではない（SHOULD NOT）。対応候補を選び、候補が全く対応しなければ `invalid_client_metadata` で拒否する方針が SHOULD とされる

[§3][choices] は `token_endpoint_auth_methods_supported` を、存在する revocation・introspection・PAR endpoint にも及ぶ認証方式の能力として扱う。同じ方式のサポートは MUST。新たな RP 用の revocation / introspection 専用 metadata は作っていない。これは既存の AS Discovery パラメータが削除されたという意味ではない。

### 2. 旧 OP の成功応答は拡張対応の証拠にならない

[Registration §3.2][registration] では、AS は理解しない入力フィールドを無視しなければならない。要求値を無視したり、`redirect_uris` を除く値を拒否・置換したりでき、置換した項目は応答へ含める必要がある。成功時には `client_id` と登録済み metadata が返る。

従来の [§2][registration] では `id_token_signed_response_alg` の省略時は `RS256`、`token_endpoint_auth_method` の省略時は `client_secret_basic`。`id_token_encrypted_response_alg` の省略時は ID Token の暗号化なしである。新しい配列だけを送る構成では、旧 OP がそれを無視して従来の省略時規則へ進む可能性がある、というのが本書の互換性上の推論である。HTTP 成功だけで意図したアルゴリズムや暗号化が設定されたとは判定しない。

### 3. 同じ名前でも OP capability と RP capability は別

[Discovery §3][discovery] の `id_token_signing_alg_values_supported` は OP が ID Token の署名に使える方式を示す。Choices 側の同名項目は RP が検証できる方式であり、二つは主体が違う。Discovery では `RS256` の掲載が MUST だが、その一覧をそのまま RP のローカル許可リストとする規定ではない。

### 4. 登録後も Core の検証境界は残る

[Core §3.1.3.7][core] は登録時の鍵・方式による復号、`iss`・`aud`・`exp`、要求した `nonce` 等の検証を定める。暗号化を交渉したのに平文なら拒否は SHOULD。`alg` は既定 `RS256` または登録要求の `id_token_signed_response_alg` とする SHOULD がある。

本書では、署名の検証も行う方針を推奨する。Code Flow で Token Endpoint から直接受け取る ID Token にある TLS による issuer 検証の例外を、任意の token の署名省略へ一般化しない。Core §2 の `none` に関する条件も、候補配列を導入しただけで解除されない。

## 移行設計（本リポジトリ独自の提案）

以下は仕様が要求するDB形式や一律のアルゴリズム順位ではない。対象を応答のある Dynamic Registration に絞った安全側の設計案である。

1. **候補を作る**: ライブラリが読める全方式ではなく、サービスとして許可する方式を候補にする。署名・暗号化・client authentication を別項目としてレビューする
2. **送信前に検査する**: scalar-in-list を確かめ、旧 OP 用 scalar がローカル方針にも適合するか検査する。互換性のために禁止方式を足すのではなく、その OP との接続を不成立にできるようにする
3. **応答を照合する**: `client_id`、issuer、送信候補、返された scalar、適用ポリシー版を関連付ける。requested と effective を同じオブジェクトへ上書きせず、AS の置換を検知できるようにする
4. **実行設定を作る**: 採用済み scalar とローカル制約を照合して検証器へ渡す。RP が三方式に対応することから、特定client登録でも三方式すべてを無条件で受理できるとは推論しない。候補外や禁止方式の応答では利用開始を止める
5. **移行を観測する**: 登録成功、拡張非対応、許可できない置換、実行時 alg 不一致を別の診断理由にする。失敗時に scalar を消す、`none` を足す、認証方式を弱める自動 fallback は設けない

例えば RP が `PS256` と `ES256` を候補とし、互換用 scalar を `PS256` にする検証環境を用意する。拡張対応 OP が `ES256` を返したなら、その選択がローカル方針と一致するかを先に確かめる。その後の token 検証器を送信時の `PS256` に固定したままにしておくと、交渉結果と実行設定が食い違う。一方、曖昧さを避けるため全アルゴリズムを許す修正も採らない。この例は方式の暗号学的優劣を順位付けするものではない。

issuer ごとの登録で異なる結果を許すなら、共有のグローバル設定を書き換える構造は避ける。更新中の認証をどう扱うか、旧設定に結び付いた処理をいつ終了させるかも別途設計する。本仕様だけから、交渉内容の無停止更新や自動 rollback の安全性は導けない。

## 受入れ試験の境界（未実行の提案）

- **scalar-in-list**: 配列外の scalar を送る入力を送信前に検出する
- **input-only / scalar response**: 対応OPの登録・read応答が、追加した候補配列を採用済み設定の代わりに返していないことを確認する
- **旧 OP / ignored array**: 未知の候補配列が無視される fixture で、成功 status だけでは設定済み扱いしない
- **alternative supported**: 希望 scalar と違う許可済み方式が選ばれた場合、保存状態と検証器が一致する
- **no common value**: 共通候補がない場合の `invalid_client_metadata` を処理し、無限の再登録や弱い方式への自動変更を起こさない
- **runtime allowlist**: 候補にはあるが当該登録では未採用の方式やローカル禁止方式を提示し、採用した明示的ポリシー通りに拒否する
- **OP capability / RP capability**: OP の一覧にあるだけの方式が RP の許可へ流入しない
- **endpoint scope**: client authentication が必要な revocation・introspection・PAR も調べ、token endpoint だけの試験で移行完了にしない

検索 eval は文書の到達性だけを検査し、上記の provider 適合試験や暗号実装試験を実行しない。

## 限界・版差・provenance

- Choices Final の §4 は選択に SHOULD を使う。実際のOPが常に候補内を返すという runtime 保証として扱わず、返却値を確認する。array の順序を全OP共通の優先順位とする契約も本調査では確認していない
- Automatic Registration では登録応答がないことが Choices の動機に含まれる。ただし本書の scalar 応答を保存する手順は、そのまま Automatic Registration に適用できない。Federation の trust chain・policy・有効な metadata の決定は別調査が必要
- 特定OP・SDKの対応版、空配列や重複要素の製品別処理、JWE `alg` と `enc` の組合せ選択、client認証の資格情報配布・更新、既存登録の管理APIは未確認。新仕様の公開を製品の対応完了と同一視しない
- 4仕様を2026-10-02 UTCに開いて確認した。いずれも official_docs の90日TTLで再確認期限は2026-12-31。既存catalogの取得日は延長せず、今回の確認範囲を持つ別recordを作った
- OpenID Foundation を出典とする独自の日本語要約と移行・試験提案である。Choices Appendix A、Registration / Discovery Appendix B、Core Appendix C の specification copyright license（帰属表示、仕様策定・実装目的）を確認した。OIDFの推奨・認証を受けた文書ではない。コード・長文・図の転載やmodule化はしていない

[choices]: https://openid.net/specs/openid-connect-rp-metadata-choices-1_0-final.html
[registration]: https://openid.net/specs/openid-connect-registration-1_0.html
[discovery]: https://openid.net/specs/openid-connect-discovery-1_0.html
[core]: https://openid.net/specs/openid-connect-core-1_0.html

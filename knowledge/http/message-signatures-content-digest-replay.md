---
{
  "id": "http-message-signatures-content-digest-replay",
  "title": "HTTP Message Signatures: 署名対象・Content-Digest・replay 防止を分けた受信契約",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9421 / RFC 9530 (ともに February 2024); RFC 9421 Verified errata 8102/8103、RFC 9530 errata 状態を2026-10-02確認",
  "tags": ["research-domain:api-distributed", "http-message-signatures", "covered-components", "content-digest", "repr-digest", "replay", "nonce", "signature-profile", "idempotency"],
  "sources": [
    {"id": "rfc9421-message-signatures-20261002", "url": "https://www.rfc-editor.org/rfc/rfc9421.html", "type": "official_docs"},
    {"id": "rfc9530-digest-fields-20261002", "url": "https://www.rfc-editor.org/rfc/rfc9530.html", "type": "official_docs"},
    {"id": "rfc9421-signatures-errata-20261002", "url": "https://errata.rfc-editor.org/search/?rfc_number=9421", "type": "official_docs"},
    {"id": "rfc9530-digest-errata-20261002", "url": "https://errata.rfc-editor.org/search/?rfc_number=9530", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/api-distributed.json"]
}
---

# HTTP Message Signatures: 署名対象・Content-Digest・replay 防止を分けた受信契約

## 問いと適用範囲

「暗号学的に正しい署名」が届いたとき、別の URL、改変された body、同じ操作の再送を受け入れていないか。新しい署名付き HTTP API を設計する際、暗号ライブラリの成功だけでは決まらない受信条件を整理する。

これは2024年公開の標準を使った実務上の知識不足を埋める調査であり、新機能の発表ではない。[既存の Webhook 記事](../api-design/webhook-redelivery-signature-dedup.md)が扱う Stripe/GitHub 固有方式との互換性は主張しない。両端を管理できる、小さな JSON リクエストを主な設計対象にする。

## 確認した標準の契約

### RFC 9421: 何を、どの文脈で検証するか

- `Signature-Input` は covered components とパラメータ、`Signature` は署名値を運ぶ。同じ label を対応付ける。signature base は指定順を保ち、末尾に `@signature-params` を置く。これを covered components に重ねて列挙しない（§2.3、§3.2、§4）
- profile は必須署名対象、鍵の解決・信頼、許可アルゴリズム、導出時の文脈などを決め、検証時に強制する。未署名部分は改変されても署名が成立する（§1.4、§3.2.1、§7.2.1）
- body の直接署名は定義されない。`Content-Digest` を署名対象にしても、実受信 content との digest 照合が別途必要（§7.2.8）
- 時刻の制約と nonce の重複検出は replay を抑える。label は署名対象外で、署名者の権限を表さない。TLS による機密性も別に必要（§7.1.2、§7.2.2、§7.2.5）

根拠: [RFC 9421](https://www.rfc-editor.org/rfc/rfc9421.html#section-1.4)。この要約は暗号プリミティブや正規化アルゴリズムの実装仕様を代替しない。

### RFC 9530: digest の対象を取り違えない

`Content-Digest` は実際の message content、`Repr-Digest` は選択された representation 全体が対象である。HEAD や Range による部分応答では対象が異なり得る。両者はアルゴリズム名を key、Byte Sequence を値とする Structured Fields の Dictionary。旧 `Digest` / `Want-Digest` を定義した RFC 3230 は廃止されている（§1.3、§2、§3）。

digest 単独には送信者認証がなく、攻撃者は content と digest を一緒に差し替えられる。署名に使う等の敵対的環境では Deprecated アルゴリズムを使ってはならない（§5）。`Content-Type`、`Content-Encoding` 等の metadata も保護対象を検討する。trailer は中継点で落ち得るため、trailer 到着前の不可逆処理は検証完了を待てない（§6.1、§6.3、§6.4）。根拠: [RFC 9530](https://www.rfc-editor.org/rfc/rfc9530.html#section-2)。

## 小さな書き込み API 用の profile 案

以下は独自の設計案であり、RFC の既定値や全 API 共通の要件ではない。最初は対象を狭くし、例外を追加するたびに試験を増やす。

### 1. 入力と署名範囲を固定する

- 対象は HTTPS の POST JSON。`Content-Type` は合意した値、body サイズには上限を設ける。初期版は `Content-Encoding` が存在する要求と digest/署名の trailer 利用を拒否する。無署名の encoding 追加や、middleware の自動展開を黙認しない
- 必須対象を `@method`、`@target-uri`、`content-type`、`content-digest` とする。digest は Dictionary として型を固定し、この component だけは `"content-digest";sf` として厳密な Structured Fields の再シリアライズを要求する方針にする。通常の文字列連結と混在させず、両端で同じ RFC 正規化実装を使う
- 初期 profile の digest は `sha-256` の1個に固定する。欠落・未知または Deprecated を含む別アルゴリズム・複数 digest・不正な型を拒否し、Byte Sequence の復号後の長さは32バイトを要求する。署名アルゴリズムの許可リストとは別の制約であり、署名が強くても弱い digest への変更を許さない。アルゴリズム移行は明示的な profile 改訂として扱う
- 冪等操作 ID を body に持たせ、その body を digest で結び付ける。認可・テナント選択・操作内容に影響する追加 header を導入するなら、署名必須化または値の固定・拒否を同時に設計する
- `keyid` は管理済みの送信者・鍵・許可アルゴリズムへ解決する。入力文字列を任意 URL として取得しない。鍵の保有と、そのテナントへ書き込む権限は別々に照合する
- `created`、`expires`、`nonce`、`keyid`、合意した profile を表す `tag` を必須パラメータとし、型も検査する。`tag` が一致するだけで送信者を信用しない。`alg` を付ける場合は設定済みアルゴリズムとの一致も要求する

`@target-uri` を採用する前に、外部 URI と reverse proxy 後の内部 URI を取り違えない構成が必要になる。本案では URI 書き換え前の入口で検証する。後段で検証する変更は、信頼できる経路から元の値を復元できることを条件とする。クライアントが任意に送れる転送 header をそのまま元 URI と認定しない。

### 2. 検証完了まで業務処理を始めない

受信処理を次の段階に分け、どの段階でも失敗した要求は業務 queue に渡さない。

1. サイズ・構文・対応 profile を検査し、必須 component と署名パラメータの欠落を拒否する
2. 信頼済み鍵と許可アルゴリズムを選び、署名対象を受信要求から再構成して検証する。送信者が渡した別の文字列を「正規化済み」と信じない
3. 受信 content の SHA-256 を独立して再計算し、復号した digest と照合する。JSON の整形、キー順の変更、改行追加より前のバイトを使う。署名チェック成功・digest 不一致は失敗とする
4. 時刻と nonce の条件を強制し、検証済み JSON のスキーマ・操作権限も確認する
5. 業務操作 ID の重複排除と durable な受領記録を行い、受領を約束できる状態になってから成功応答する

暗号検証と digest 計算の順序は負荷対策に応じて変えられるが、どちらかを省く最適化にはしない。大きな body に拡張するなら、一時保存・streaming hash・上限・タイムアウトを先に設計する。メモリに全量保持する現在の案をそのまま流用しない。

### 3. nonce と業務操作 ID に別の寿命を持たせる

試験用の時刻条件を、`created <= now + 30秒`、`0 < expires - created <= 300秒`、`now <= expires + 30秒` と定める例を考える。この30秒/300秒は本案の仮定であり、RFC の推奨値ではない。運用では時計ずれ、ネットワーク遅延、再送契約から決め直す。

nonce は毎回新しくし、検証済み送信者・profile・nonce の組を原子的な一意制約で予約する。全受信ノードが同じ判定を使い、少なくともその署名を受理し得る最終時刻まで記録を残す。本例では `expires + 30秒` を過ぎるまで削除しない。先に読み、後で別々に保存する方式は同時到着を通すため使わない。時計の巻き戻り・複製遅延も試験対象にする。

業務操作 ID は、署名の寿命より長い業務再送期間に合わせて保持する。応答紛失後の正当な retry は「新しい nonce と署名、同じ業務操作 ID」にする案である。同一 ID・同一操作なら保存済み結果を返し、同一 ID・異なる操作なら競合として拒否する。nonce が新しくても、業務をもう一度実行してよい理由にはならない。

nonce の消費後に障害が起きる場合もある。受領記録前なら新しい署名で再送でき、受領済みなら業務操作 ID で結果を取り戻せる契約にする。受領記録と queue 投入が別システムなら、その間の crash を扱う inbox/outbox 等を別途設計する。署名は配信保証や分散トランザクションを提供しない。

## 避ける判定と受け入れ試験案

以下は実装前の試験仕様であり、実行済みテストではない。正常系だけで署名処理の有効性を判定しない。

| 入力・障害 | 本案で期待する結果 |
| --- | --- |
| 署名・digest・時刻・権限が正しい初回要求 | durable 受領後に成功。業務操作は1回 |
| body の1バイトだけ改変し、digest header と署名は維持 | digest 不一致で拒否 |
| body と digest を両方変更し、元の署名を維持 | 署名不一致で拒否 |
| 正しく署名された digest が弱い・未知の key、複数値、不正型、または31バイト | profile 違反で拒否し、副作用なし。SHA-256 の再計算省略も禁止 |
| 別 URL または別 method に正しい署名を転用 | 署名検証失敗。署名対象を省いた新署名も profile 違反 |
| `Content-Encoding` を後から付加 | 初期 profile の入力制約で拒否 |
| nonce が同じ要求を2ノードへ同時送信 | 原子的な重複検出で受領は最大1件 |
| 有効時刻内の同じ要求を再生 | timestamp が新しくても nonce 重複で拒否 |
| 応答紛失後、新 nonce・同一業務操作 ID で再送 | 保存済み結果へ収束し、二重実行しない |
| nonce 記録の保存先が利用不能 | 受理せず再試行可能なエラー。検証省略へ切り替えない |
| 既知鍵だが別テナント、または未知鍵による数学的に正しい署名 | 認可または鍵の信頼条件で拒否 |
| label の双方を同じ新 label に変更 | label の名前で権限を変えない。対応付けと全検証は維持 |
| 設定と `alg` の不一致、必須対象の欠落、期限切れ | 拒否し、業務副作用がないことも確認 |

検索 eval はこの文書へ到達できることだけを検査する。暗号の適合性、負荷、並行実行、障害復旧を通した証拠にはしない。

## Errata・版・provenance・限界

- RFC 9421 と RFC 9530 はともに2024年2月公開の Standards Track。2026-10-02 UTC に本文と errata を確認した。最新の全関連仕様を網羅したという意味ではない
- [RFC 9421 errata](https://errata.rfc-editor.org/search/?rfc_number=9421): 8102（§7.2.8、Editorial）と8103（§7.5.3、Technical）は2024-10-29に Verified。いずれも誤記 `@signature-input` を `@signature-params` に直す。本稿は訂正後の名称を用いる
- [RFC 9530 errata](https://errata.rfc-editor.org/search/?rfc_number=9530): 8158（2024-10-29 Verified）は図14の名前を `Repr-Digest` に訂正、8273（2025-02-06 Verified）は英文の文法訂正。8890は2026-04-25に報告された Brotli 例のバイト列・digest 不整合で、取得時点では **Reported**。確定した改訂として扱わず、該当例を適合性試験ベクタへ転用していない
- RFC 本文のライセンスは BCP 78 / IETF Trust Legal Provisions、抽出 Code Components は Revised BSD と各 Copyright Notice に記載される。errata ページ単体のライセンス表示は確認できず catalog は `unknown`。本文は独自の日本語要約と設計案で、コード・署名値・鍵・圧縮例の転記はない
- 未確認: 具体的ライブラリの相互運用性、暗号実装、署名鍵の配布・失効手順、proxy 製品ごとの正規化、Brotli erratum の独立再現。RFC 9421 全 component、署名付き応答、複数署名の組合せ規則、圧縮・trailer の本番採用も対象外
- 再確認期限は official_docs の90日後の2026-12-31。errata 状態、採用 profile の脅威モデル、実装の差を確認してから更新する

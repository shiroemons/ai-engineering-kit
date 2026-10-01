---
{
  "id": "go-json-v2-migration-strict-input-wire-contract",
  "title": "Go 1.27 JSON v2 移行: 厳格な入力検証と wire format の互換境界",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1（2026-09-01公開）; JSON v2 は Go 1.27.0（2026-08-19公開）で正式導入; 2026-10-01 UTC 確認",
  "tags": [
    "research-domain:backend",
    "json",
    "migration",
    "compatibility",
    "validation",
    "wire-format",
    "duplicate-names",
    "utf8",
    "omitempty"
  ],
  "sources": [
    {
      "id": "go127-json-release-20261001",
      "url": "https://go.dev/doc/go1.27",
      "type": "release_notes"
    },
    {
      "id": "go-release-history-json-20261001",
      "url": "https://go.dev/doc/devel/release",
      "type": "release_notes"
    },
    {
      "id": "go127-json-v1-migration-20261001",
      "url": "https://pkg.go.dev/encoding/json@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-json-v2-api-20261001",
      "url": "https://pkg.go.dev/encoding/json/v2@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-jsontext-api-20261001",
      "url": "https://pkg.go.dev/encoding/json/jsontext@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go-json-v2-migration-guide-20261001",
      "url": "https://go.dev/doc/jsonv2-migration",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Go 1.27 JSON v2 移行: 厳格な入力検証と wire format の互換境界

## 問いと適用範囲

Go の更新だけで JSON 入力は厳格になるか。`encoding/json` を `encoding/json/v2` へ置き換えても既存 API の応答形式を保てるか。

Go 1.27 では JSON v2 が標準提供され、従来の `encoding/json` も内部では v2 実装を利用する。ただし v1 の既定動作を保つ互換 options が適用されるため、**toolchain 更新だけでは入力契約は v2 化されない**。v1 は引き続きサポートされ、移行は必須ではない。エラーメッセージの正確な文字列は変化し得る。[Go 1.27 release notes](https://go.dev/doc/go1.27#encodingjsonv2)

適用版は [公開履歴](https://go.dev/doc/devel/release) で Go 1.27.0 が 2026-08-19、1.27.1 が 2026-09-01 と確認した。以下は版固定の 1.27.1 API を読む。将来版や外部の JSON ライブラリへ一般化しない。

## 確認した契約

### 1. 構文の厳格化と schema の厳格化は別

- v2 の既定では重複キーと不正 UTF-8 を拒否する。`jsontext.AllowDuplicateNames(true)` と `jsontext.AllowInvalidUTF8(true)` はこの検査を緩める。後者は不正な文字列を置換文字へ変える。[jsontext API](https://pkg.go.dev/encoding/json/jsontext@go1.27.1#AllowDuplicateNames)
- struct のフィールド名は既定で大文字小文字を区別する。しかし一致しない未知フィールドは既定で無視する。拒否したい入力では `jsonv2.RejectUnknownMembers(true)` が別途必要である。[v2 struct 契約](https://pkg.go.dev/encoding/json/v2@go1.27.1#hdr-JSON_Representation_of_Go_structs)
- `UnmarshalRead` は reader を EOF まで読み、単一の JSON 値と任意の空白だけを受理する。複数値のストリームを読む `UnmarshalDecode` とは用途が違う。[UnmarshalRead](https://pkg.go.dev/encoding/json/v2@go1.27.1#UnmarshalRead)

したがって「case-sensitive だから大文字の誤記はエラー」「v2 だから未知フィールドも拒否」は誤り。特に JSON 名 `name` のフィールドに `Name` を送った場合、既定では値が入らないまま成功し得る。

### 2. 応答の wire format は import の置換でも変わる

[v1 の比較一覧](https://pkg.go.dev/encoding/json@go1.27.1#hdr-Migrating_to_v2) で確認した、移行前に契約テストへ入れる代表例:

| 値・タグ | v1 既定 | v2 既定 |
|---|---|---|
| nil の `[]string` / map | `null` | `[]` / `{}` |
| bool / number の `omitempty` | false / 0 を省略 | false / 0 を出力 |
| map の出力順 | 決定的 | 非決定的 |
| `time.Duration` | ナノ秒の数値 | 既定表現がなくエラー |

`omitempty` は v2 では空の JSON 値を基準にする。Go のゼロ値を省略する意図なら `omitzero` を検討する。`FormatNilSliceAsNull(true)` / `FormatNilMapAsNull(true)` は nil の表現を、`encoding/json.FormatDurationAsNano(true)` は Duration の互換動作を指定する。表は全差分の一覧ではなく、bytes・array・custom marshaler 等は元資料の比較一覧も確認する。

`jsonv2.Deterministic(true)` の保証は同一バイナリのインスタンス間に限られ、toolchain・ソース・GOOS/GOARCH 等が違うビルド間には及ばない。[Deterministic](https://pkg.go.dev/encoding/json/v2@go1.27.1#Deterministic)

### 3. 互換 options は順序を含む契約

`jsonv2.Marshal(value, jsonv1.DefaultOptionsV1())` は v1 の marshal 動作と意味的に等価である。追加 options は後勝ちなので、v1 互換を起点に個別の動作を切り替えられる。[公式 migration guide](https://go.dev/doc/jsonv2-migration#option-by-option)

入力でも `DefaultOptionsV1()` を使えば重複キー・不正 UTF-8 の許容まで持ち込む。互換性のために付けた option set を「v2 の安全な既定」と呼ばない。例えば v1 互換の後に `jsontext.AllowDuplicateNames(false)` を置けば、その項目だけを厳格化できる。[DefaultOptionsV1](https://pkg.go.dev/encoding/json@go1.27.1#DefaultOptionsV1)

## 限定実測: 1.27.1 で確認した境界

以下は今回作成した独立した小さな probe の観察であり、網羅的な API 保証や性能評価ではない。Linux amd64、`go version go1.27.1`、追加 GOEXPERIMENT 指定なしで実行した。リポジトリの製品コードは変更していない。

入力先を `Name string` に `json:"name"` タグを付けた struct として比較した:

| 入力 | v1 | v2 既定 | v2 + RejectUnknownMembers |
|---|---|---|---|
| `{"name":"a","name":"b"}` | 成功、`b` | エラー | エラー |
| `{"Name":"a"}` | 成功、`a` | 成功、空文字 | エラー |
| `{"name":"a","extra":1}` | 成功 | 成功 | エラー |
| name 文字列内に 0xff | 成功 | エラー | この組合せは未実測 |

追加で確認した事項:

- `count` / `enabled` に `omitempty`、`items []string` / `labels map[string]string` はタグ名だけを付けたゼロ値 struct: v1 は `{"items":null,"labels":null}`、v2 は `{"count":0,"enabled":false,"items":[],"labels":{}}`。v2 + DefaultOptionsV1 はこの入力では v1 と同じバイト列になった
- `UnmarshalRead` へ `{"name":"a"} {"name":"b"}` を渡すと末尾の2個目を拒否した
- 重複キーを拒否した v2 の出力先には、先に読んだ `a` が残っていた。エラー時の出力値を業務処理へ渡してはいけない
- `null` を既存値 7 の int に unmarshal すると 0 になり成功した。`time.Second` の v2 marshal は error だった

この観察から、型付き decode 成功だけでは必須項目・業務上の妥当性を保証できないことが分かる。必要な項目、null の可否、値域をアプリケーション側で検証する。

## 推奨する移行手順（独自の設計案）

以下は上記の契約に基づく設計判断であり、Go 公式が個別サービスの互換性を保証するものではない。

1. **更新を二段階にする。** toolchain 更新と JSON 呼出し箇所の移行を別の変更にする。外部 API、保存ファイル、署名・ハッシュ対象など、同じ struct がどの相手に渡るかを先に列挙する
2. **旧動作と新動作を比較する。** 代表的な正常データに加えて、nil/空配列、false/0、大小文字違い、重複キー、不正 UTF-8、未知項目、null、末尾の別 JSON 値を固定 fixture にする。エラー文の全文一致だけで判定しない
3. **必要な互換性を局所化する。** 既存の出力契約が必要な呼出しには DefaultOptionsV1 等を明示し、項目単位で新動作へ進める。未知項目を将来拡張として受ける公開 API と、誤記を拒否したい内部設定を同じ設定で一括処理しない
4. **入力を確定前の値へ読む。** 新しい一時値へ decode し、error がないことと必須・値域検証を確認してから実データを更新する。HTTP 入力では UnmarshalRead の前に受信量と読取時間も制限する。EOF 検査はサイズ・時間制限の代わりではない
5. **出力を利用する相手まで確認する。** nil と空配列の意味、false/0 の存在、Duration の単位を consumer テストで確認する。JSON を再 encode したバイト列に依存する署名やキャッシュキーを、Deterministic だけで版をまたいで安定すると判断しない
6. **移行比較の副作用を考える。** 本番で二重処理する場合は CPU とログへ出すデータを制限する。custom marshaler の副作用も事前確認し、失敗 payload を無条件でログへ残さない

公式 guide は本番比較用に外部 package `jsonsplit` も紹介し、二重 marshal や AutoDetectOptions の追加コストを説明している。本調査では同 package を導入・実測していない。[migration guide](https://go.dev/doc/jsonv2-migration#jsonsplit)

## 実験版から移る場合・未確認事項

Go 1.27 の release notes は、実験中の v2 から `format` / `unknown` タグ、`DiscardUnknownMembers`、`SkipFunc` の削除と `inline` → `embed` の改名を記録する。古い experimental 記事の API をそのまま採用せず、対象版の署名・タグを読む。[Go 1.27 の変更一覧](https://go.dev/doc/go1.27#encodingjsonv2)

`GOEXPERIMENT=nojsonv2` は旧 v1 実装へ戻す build-time の暫定 opt-out であり、将来削除予定。通常の v2 移行をこの設定に依存させない。同設定での build、全 custom marshaler、全 option 組合せ、性能、本番互換性、他アーキテクチャは未検証である。

## 出典とライセンス

- Go project の release notes / release history / migration guide は [go.dev 著作権表示](https://go.dev/copyright) に従い本文 CC-BY-4.0、コード BSD。公開日は release history で確認し、guide の独立した公開日はページ上で確認できなかった
- 版固定 pkg.go.dev の v1 / v2 / jsontext は表示 `go1.27.1`・公開日 2026-09-01・BSD-3-Clause を確認し、[Go LICENSE](https://go.dev/LICENSE) も照合した
- すべて 2026-10-01 UTC 取得。公式本文の独自要約と今回の独自観察・設計判断を区別した。外部コードの転載・改変はなく、module への昇格を意味しない

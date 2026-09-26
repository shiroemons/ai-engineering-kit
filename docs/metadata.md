# Metadata 契約

CLI の Go 型と validator を機械判定の正本とする。設定と catalog は JSON、検索対象 Markdown は `---` で囲んだ JSON object を先頭に置く。YAML の構文解析は行わない。

## 文書

`knowledge/**/*.md`・`patterns/**/*.md` を読み込む。module は `modules/<technology>/<name>/README.md` だけを対象にする。`*.template.md` は除外する。knowledge と patterns の直下・技術直下の README は、Front Matter がない場合に案内文として除外する。記入例は [knowledge template](../examples/knowledge.template.md) を参照する。

| 項目 | 契約 |
|---|---|
| id | 一意。英小文字・数字で始まり、英小文字・数字・_・- を使う |
| title | 空でないタイトル |
| kind | knowledge / pattern / module |
| technology | 空でない技術名 |
| version | 確認した適用版。最新と推測しない |
| tags | 文字列の配列 |
| sources | 1件以上の id・url・type |
| retrieved_at | YYYY-MM-DD の取得日 |
| expires_at | 取得日より後の明示期限 |
| trust | 設定にある信頼区分 |
| status | active / stale |
| evals | 任意の repository 相対パス配列 |

source の ID・URL・type は catalog と一致させる。文書の信頼度は参照 source のうち最も低いものを超えない。独自の設計案には `primary-source` を使い、公式情報との区別を本文にも書く。

未知の JSON フィールド、重複キー、不正な UTF-8、末尾の別 JSON 値を拒否する。フィールド名は大文字小文字も一致させる。未来の取得日を拒否し、日付の検証基準も `--as-of` に従う。

## Source と設定

`sources/catalog/*.json` は次の項目を持つ。

- id
- url
- repository_url
- commit_sha
- version
- retrieved_at
- license
- trust
- type
- purpose

公式文書では commit_sha を空にできる。`github_repository_analysis` は40桁の commit SHA で固定する。module は license が unknown の source を参照できない。

`config/sources.json` の trust_order は固定順を記録する。順序は official → maintainer → primary-source → community → unknown。許可 URL scheme の初期値は https のみ。設定値は http または https を受け付ける。

`config/freshness.json` は source_types と technologies に1〜36500日を整数で指定する。未定義の技術は技術 TTL を適用せず、source type と明示期限で判定する。source TTL は公式文書90日、release notes 30日、repository 分析90日、architecture pattern 180日。

開発元の技術記事は `maintainer_article` として90日、運用当事者の障害報告は `incident_report` として180日を設定する。信頼区分は出典の所有者と主張の根拠を確認し、`maintainer` または `primary-source` を使う。公式ドメイン上の記事であっても、事例を公式のAPI契約と同一視しない。期限は過去の出来事の真偽が変わる日ではなく、適用条件を再確認する期限である。

## 有効期限

有効期限は次の候補の最小日とする。

1. 文書の expires_at。
2. 各 source について、文書と catalog の古い方の取得日 + source type の TTL。
3. 文書の取得日 + technology の TTL。技術が設定にある場合だけ適用する。

基準日の UTC 00:00 が期限以上なら stale。手動の `status: stale` も維持する。判定は検索時に再計算し、Markdown の status を書き換えない。文書の日付だけを延ばしても古い catalog の期限は延びない。

検索結果の `expires_at` は計算した有効期限であり、Front Matter の明示期限と異なる場合がある。

## Module

module の README は kind を module にする。同じディレクトリに module.json と provenance を置く。module.json は api、tests、evals、provenance、reviewed_by、reviewed_at、concurrency を持つ。

tests は module 内の相対パス、evals は repository 内の相対パスとする。provenance は `PROVENANCE.md` を指定する。ファイル存在の検査とテストの実行は別なので、昇格には `just check` とレビューが必要。

---
{
  "id": "search-postgres-pgtrgm-trigram-similarity-gin-gist",
  "title": "PostgreSQL pg_trgm の trigram 類似検索範囲と similarity 演算子・threshold および GIN 対 GiST 選択",
  "kind": "knowledge",
  "technology": "search",
  "version": "PostgreSQL 18.6 (retrieved page label; not a latest-version claim)",
  "tags": ["research-domain:data", "postgres", "pg_trgm", "trigram", "similarity", "word_similarity", "GIN", "GiST", "gin_trgm_ops", "gist_trgm_ops", "threshold", "LIKE", "similarity_threshold"],
  "sources": [
    {"id": "postgres-pgtrgm-docs", "url": "https://www.postgresql.org/docs/current/pgtrgm.html", "type": "official_docs"},
    {"id": "postgres-gin-indexes-docs", "url": "https://www.postgresql.org/docs/current/gin.html", "type": "official_docs"},
    {"id": "postgres-gist-indexes-docs", "url": "https://www.postgresql.org/docs/current/gist.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# PostgreSQL pg_trgm の trigram 類似検索範囲と similarity 演算子・threshold および GIN 対 GiST 選択

曖昧な名称・誤記を含む検索を `pg_trgm` の trigram 類似度で扱い、真偽条件の threshold と距離順の `ORDER BY` を、GIN と GiST のどちらで支えるかを決める範囲の確認済み契約だけを書く。全文検索の語彙素ランキング、日本語形態素解析、ベンチマークによる一律の index 推奨は本調査の検証範囲外であり、推測で埋めない。

## 要点

以下は確認した公式文書の記載であり、数値の推奨ではない。

- trigram は連続する 3 文字である。文字列の先頭に 2 空白、末尾に 1 空白を付加し、単語文字でない文字は無視して抽出する。類似度の基礎はこの trigram 集合の共有度合いである。[F.35 pg_trgm](https://www.postgresql.org/docs/current/pgtrgm.html)
- 類似度関数は `similarity()`、`word_similarity()`、`strict_word_similarity()` である。距離演算子は `a <-> b` が `1 - similarity(a, b)`、`a <<-> b` が `1 - word_similarity(a, b)`、`a <<<-> b` が `1 - strict_word_similarity(a, b)` の距離を返す。`word_similarity` は第 1 引数全体と第 2 引数側の単語境界での連続範囲との類似を取り、`strict_word_similarity` は両引数での単語境界の類似を取る。[F.35 pg_trgm](https://www.postgresql.org/docs/current/pgtrgm.html)
- 真偽演算子は `%`、`<%` / `%>`、`<<%` / `%>>` の 3 系統である。`%` は `similarity` が `pg_trgm.similarity_threshold` (既定 0.3) 以上で真、`<%` / `%>` は `word_similarity` が `pg_trgm.word_similarity_threshold` (既定 0.6) 以上で真、`<<%` / `%>>` は `strict_word_similarity` が `pg_trgm.strict_word_similarity_threshold` (既定 0.5) 以上で真になる。交換子 (`%>`、`%>>`) は引数順を入れ替えた同一判定である。[F.35 pg_trgm](https://www.postgresql.org/docs/current/pgtrgm.html)
- index 支援の opclass は、GiST が `gist_trgm_ops`、GIN が `gin_trgm_ops` である。`gist_trgm_ops` の `siglen` 任意パラメータは既定 12 バイト、範囲 1-2024 バイトである。[F.35 pg_trgm](https://www.postgresql.org/docs/current/pgtrgm.html)
- 両 index 型とも `LIKE` / `ILIKE` を 9.1 以降、`~` / `~*` を 9.3 以降に trigram 抽出で支援する。問い合わせから trigram を抽出できない場合は全表走査に帰着する。既定ビルドでは大文字小文字を区別せず抽出する。不等号 (`NOT LIKE` / `!=` 等) は支援しない。[F.35 pg_trgm](https://www.postgresql.org/docs/current/pgtrgm.html)
- 距離による `ORDER BY ... <-> ... LIMIT n` は GiST で効率的に実行できるが、GIN では効率的でない。GIN は真偽条件の絞り込み向き、GiST は近傍順の取り出し向きという契約上の向き不向きがある。[F.35 pg_trgm](https://www.postgresql.org/docs/current/pgtrgm.html)
- GIN は汎用転置 (generalized inverted) index であり、各 key を posting list とともに key ごとに 1 回だけ格納する。`pg_trgm` は GIN を提供する contrib モジュールの一つとして挙げられている。[65.4 GIN Indexes](https://www.postgresql.org/docs/current/gin.html)
- GIN は `fastupdate` による pending-list 緩衝を持つ。未整理の pending list が大きいと検索時に penalty が生じ、`gin_pending_list_limit` と `VACUUM` で整理する。一括投入では index を削除して再作成する指針が示され、構築は `maintenance_work_mem` に感受性がある。[65.4 GIN Indexes](https://www.postgresql.org/docs/current/gin.html)
- GiST は均衡汎用探索木のテンプレートであり、各 opclass が `consistent` / `union` / `same` を実装し、近傍順走査には任意の `distance` を実装する。`pg_trgm` は GiST を提供する contrib モジュールの一つとして挙げられている。構築は `sortsupport` の有無で sorted build と buffering build に分かれ、分岐品質は `penalty` / `picksplit` に依存する。[65.2 GiST Indexes](https://www.postgresql.org/docs/current/gist.html)

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する threshold 値や index を指すものではない。

- 問い合わせの意味で演算子系を選ぶ。全体の綴りの近さなら `%` + `similarity`、問い合わせ語が長い文中のどこに現れるかなら `<%` + `word_similarity`、両側の語境界を厳密に見るなら `<<%` + `strict_word_similarity` を使う。threshold は既定 (0.3 / 0.6 / 0.5) から動かさず、実データの適合・取りこぼしを見て変える。
- 真偽の絞り込みが主なら GIN (`gin_trgm_ops`) を第一候補にし、上位 n 件の距離順取り出し (`ORDER BY col <-> 'query' LIMIT n`) が主なら GiST (`gist_trgm_ops`) を第一候補にする。両方を 1 本の index で満たす前提を置かず、問い合わせ形ごとに `EXPLAIN` で index 使用を確認する。
- GiST の `siglen` は既定 12 バイトのままにする。範囲 1-2024 バイトの調整は false match と index 寸法の交換であり、理由なく変えない。
- GIN を書き込みの多い表に置く場合は pending-list の整理を見込む。`gin_pending_list_limit` を理由なく緩めず、`VACUUM` の整理と検索 penalty の関係を自前の更新頻度で確認する。初期投入時は index 付きで逐次投入する前に drop/recreate の指針を検討する。
- `LIKE '%word%'` や `~ 'pattern'` を trigram で支援させる場合は、問い合わせから trigram が抽出できる形かを先に確認する。短いパターンや非単語文字だけのパターンは全走査に帰着し得るため、index がある前提で計画を読まない。

## 避ける使い方

- `%` なのに `word_similarity_threshold` を変えて調整する。確認した契約では `%` は `pg_trgm.similarity_threshold` (既定 0.3) に従い、`<%` 系は `word_similarity_threshold` (既定 0.6)、`<<%` 系は `strict_word_similarity_threshold` (既定 0.5) に従う。
- 距離演算子を混同する。`1 - similarity` は `<->`、`1 - word_similarity` は `<<->`、`1 - strict_word_similarity` は `<<<->` であり、真偽演算子 `%` / `<%` / `<<%` と対で使う。
- `ORDER BY col <-> 'q' LIMIT 10` を GIN で効率的に取れると期待する。確認した記載ではこの形が効率的なのは GiST であり、GIN ではない。
- `NOT LIKE` や `!=` に trigram index が効くと期待する。確認した記載では不等号は支援しない。
- `siglen` を大きくすれば常に良くなると扱う。確認した範囲は既定 12 バイト・上限 2024 バイトの signature 長の指定であり、衝突と寸法の交換を無視した一律の拡大指針は確認していない。

## 適用版と本番での注意

- 適用版: `PostgreSQL 18.6` の文書 (`current` = 18) の F.35 節、65.4 節、65.2 節で確認する。将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs` (TTL 90 日) で technology `search` に技術固有 TTL がないため、2026-12-27 に再取得して内容を確認する。文書の日付だけを延ばさず、該当節の文言と版表示を再確認する。
- 本文の推奨構成は設計案であり、特定ベンチマークや単一事例の一般化ではない。threshold と index 選択の効果はデータ量・更新頻度・問い合わせ形に依存するため、対象構成で測定して採用する。
- 未確認事項 (本調査の範囲外として推測で埋めない): 各 threshold 変更時の実行計画の変わり方の定量、GIN `fastupdate` 等の storage parameter の調整効果の定量、`maintenance_work_mem` の具体値、timeout・cancellation・shutdown 時の trigram 問い合わせの扱い、版間の動作差、大文字小文字を区別する非既定ビルドの条件。これらは該当版の該当節を別途確認する。

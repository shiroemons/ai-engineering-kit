---
{
  "id": "search-postgres-fulltext-tsvector-gin-rank",
  "title": "PostgreSQL full-text search の tsvector/tsquery 照合と GIN 選択および ts_rank ランキング範囲",
  "kind": "knowledge",
  "technology": "search",
  "version": "PostgreSQL 18.6 (retrieved page label; not a latest-version claim)",
  "tags": ["research-domain:data", "postgres", "full-text", "tsvector", "tsquery", "GIN", "GiST", "ts_rank", "ts_rank_cd", "ranking", "normalization", "lexeme"],
  "sources": [
    {"id": "postgres-textsearch-tables-docs", "url": "https://www.postgresql.org/docs/current/textsearch-tables.html", "type": "official_docs"},
    {"id": "postgres-textsearch-indexes-docs", "url": "https://www.postgresql.org/docs/current/textsearch-indexes.html", "type": "official_docs"},
    {"id": "postgres-textsearch-controls-docs", "url": "https://www.postgresql.org/docs/current/textsearch-controls.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# PostgreSQL full-text search の tsvector/tsquery 照合と GIN 選択および ts_rank ランキング範囲

`to_tsvector` で作った tsvector と `to_tsquery` の tsquery を `@@` で照合し、GIN で高速化し、`ts_rank` / `ts_rank_cd` で文書内順位付けする範囲の確認済み契約だけを書く。日本語形態素解析、類義語辞書の調整、外部ランキング基盤との比較は本調査の検証範囲外であり、推測で埋めない。

## 要点

以下は確認した公式文書の記載であり、数値の推奨ではない。

- 照合の基本形は `to_tsvector('english', body) @@ to_tsquery('english', 'query')` のように、tsvector 側と tsquery 側に同じ text search configuration を渡す。式 index を使う場合、`CREATE INDEX USING GIN (to_tsvector('english', body))` で作った index が使われるのは、問い合わせ側も同じ 2 引数 (`'english'`, `body`) の `to_tsvector` を使うときに限られる。config が一致しない式は index の式と別物として扱われる。[12.2 Tables and Indexes](https://www.postgresql.org/docs/current/textsearch-tables.html)
- 代替構成として、tsvector を格納する `GENERATED` 列を作り、その列に GIN index を作る方法がある。式 index と異なり、tsvector 自体が列値として格納される。どちらを選ぶかの性能比較は本調査の検証範囲外であり、公式が一律の選択を指示するものとして扱わない。[12.2 Tables and Indexes](https://www.postgresql.org/docs/current/textsearch-tables.html)
- 全文検索の index 型は GIN が優先される。GIN は転置 (inverted) index であり、出現する lexeme ごとに entry を持つ。格納されるのは lexeme のみで weight ラベルは格納されないため、weight を使う問い合わせでは heap 行の再検査が必要になる。[12.9 Preferred Index Types for Text Search](https://www.postgresql.org/docs/current/textsearch-indexes.html)
- GiST は全文検索では損失あり (lossy) である。各行を固定長の signature で表し、signature 長は既定 124 バイト、最大 2024 バイトである。signature の衝突により false match が生じる。GIN のような lexeme 単位の転置構造ではない。[12.9 Preferred Index Types for Text Search](https://www.postgresql.org/docs/current/textsearch-indexes.html)
- ランキング関数は `ts_rank` と `ts_rank_cd` である。`ts_rank` は頻度に基づく方式、`ts_rank_cd` は位置情報を用いる cover-density 方式である。`ts_rank_cd` を使う場合は位置情報が必要になる。[12.3.3 Ranking Search Results](https://www.postgresql.org/docs/current/textsearch-controls.html)
- 重み付けは `weights` 引数の配列順 `{D, C, B, A}` で指定し、既定値は `{0.1, 0.2, 0.4, 1.0}` である。正規化は `normalization` 引数のビットマスク `0 / 1 / 2 / 4 / 8 / 16 / 32` で指定する。具体的な各値の意味の暗記ではなく、適用版の該当節の表を直接確認する。[12.3.3 Ranking Search Results](https://www.postgresql.org/docs/current/textsearch-controls.html)
- ランキング関数はいずれも全体 (global) の情報を持たない。`rank / (rank + 1)` は値を 0 から 1 の範囲に見やすく整形するだけの外見上の変換であり、文書集合全体での正規化得点にはならない。[12.3.3 Ranking Search Results](https://www.postgresql.org/docs/current/textsearch-controls.html)

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する数値や構成を指すものではない。

- 照合式と index 式の config を文字どおり一致させる。`to_tsvector('english', body)` で GIN を作ったなら、問い合わせの `@@` の左辺も同じ 2 引数の呼び出しにする。config を変数化する場合は index 定義と問い合わせ生成の両方が同じ値を見るようにし、片方だけ `'simple'` や別言語に変えない。
- 書き込み頻度と tsvector 格納の要否で格納方式を選ぶ。式 GIN は tsvector 列を持たない代わりに式の一致が条件になる。`GENERATED` 列 + GIN は tsvector が行に格納される代わりに列の格納量が増える。採用前に自前の更新頻度と問い合わせ形で `EXPLAIN` の index 使用を確認する。
- 大規模な全文検索の第一候補は GIN にする。weight 付き問い合わせを使う場合は再検査が発生することを前提に実行計画を読む。GiST は signature 衝突による false match を許容できる場合に限って検討し、既定 124 バイトという上限を理由なく変更しない。
- ランキングは文書内の順序付けとして使う。`ts_rank` を既定の第一候補にし、語の近接を反映したい場合だけ `ts_rank_cd` を試す。`weights` と `normalization` を変えたら順位の変化を実データで確認し、`rank / (rank + 1)` を異なる問い合わせ間の比較可能な得点として扱わない。

## 避ける使い方

- 問い合わせ側の config が index 式と違うのに GIN が使われることを期待する。確認した契約では同一の 2 引数呼び出しが条件である。
- GIN に weight ラベルが格納されている前提で計画を読む。確認した記載では格納されるのは lexeme のみで、weight 付き問い合わせには行の再検査が要る。
- GiST を損失なしの転置 index として扱う。確認した記載では固定長 signature の損失あり index であり、false match を生じる。
- `ts_rank_cd` を位置情報なしで使う。確認した記載では cover-density 方式は位置情報を要する。
- `weights` の配列順を `{A, B, C, D}` と思い込んで指定する。確認した順序は `{D, C, B, A}` であり、既定値は `{0.1, 0.2, 0.4, 1.0}` である。
- `rank / (rank + 1)` を全体で正規化された関連度得点として異なる問い合わせ間で比較する。確認した記載では全体情報を持たず、外見上の 0-1 整形に過ぎない。

## 適用版と本番での注意

- 適用版: `PostgreSQL 18.6` の文書 (`current` = 18) の第 12.2 節、第 12.9 節、第 12.3 節 (12.3.3 Ranking) で確認する。将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs` (TTL 90 日) で technology `search` に技術固有 TTL がないため、2026-12-26 に再取得して内容を確認する。文書の日付だけを延ばさず、該当節の文言と版表示を再確認する。
- 本文の推奨構成は設計案であり、特定ベンチマークや単一事例の一般化ではない。index 選択とランキング方式の効果は文書量・更新頻度・問い合わせ形に依存するため、対象構成で測定して採用する。
- 未確認事項 (本調査の範囲外として推測で埋めない): parser・dictionary・configuration の個別契約、GIN `fastupdate` 等の storage parameter の調整効果、trigger による tsvector 保守手順の詳細、headline 生成の契約、timeout・cancellation・shutdown 時の全文検索問い合わせの扱い、版間の動作差。これらは該当版の該当節を別途確認する。

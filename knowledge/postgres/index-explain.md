---
{
  "id": "postgres-index-explain",
  "title": "PostgreSQL のインデックス選択と EXPLAIN による実行計画の確認",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18.6 (retrieved page label; not a latest-version claim)",
  "tags": ["postgres", "index", "btree", "gin", "gist", "brin", "explain", "query-plan", "performance", "research-domain:data"],
  "sources": [
    {"id": "postgres-index-types-docs", "url": "https://www.postgresql.org/docs/current/indexes-types.html", "type": "official_docs"},
    {"id": "postgres-indexes-intro-docs", "url": "https://www.postgresql.org/docs/current/indexes-intro.html", "type": "official_docs"},
    {"id": "postgres-create-index-docs", "url": "https://www.postgresql.org/docs/current/sql-createindex.html", "type": "official_docs"},
    {"id": "postgres-using-explain-docs", "url": "https://www.postgresql.org/docs/current/using-explain.html", "type": "official_docs"},
    {"id": "postgres-explain-ref-docs", "url": "https://www.postgresql.org/docs/current/sql-explain.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# PostgreSQL のインデックス選択と EXPLAIN による実行計画の確認

クエリの述語に合う index 種別を選び、`EXPLAIN` で planner が実際に選んだ plan を確かめる。以下は公式文書の契約であり、数値の推奨ではない。

## 要点

- 既定は B-tree。`CREATE INDEX` は指定がなければ B-tree を作り、他の種別は `USING` の後に種別名を書いて選ぶ（例: `CREATE INDEX name ON table USING HASH (column)`）。`USING method` の既定は `btree`。[11.2 Index Types](https://www.postgresql.org/docs/current/indexes-types.html)・[CREATE INDEX](https://www.postgresql.org/docs/current/sql-createindex.html)
- B-tree は順序付け可能な値の等価・範囲比較（`< <= = >= >`、および `BETWEEN`・`IN` のような等価な組み合わせ、`IS NULL` / `IS NOT NULL`）に使える。`LIKE`・`~` はパターンが定数かつ文字列先頭に固定されている場合（`col LIKE 'foo%'`、`col ~ '^foo'`）に使えるが、`col LIKE '%bar'` には使えない。C ロケール以外ではパターンマッチ用に専用の operator class が要る。`ILIKE`・`~*` は大文字小文字変換の影響を受けない非英字で始まる場合に限られる。ソート順の取得にも使えるが、単純な scan + sort より速いとは限らない。[11.2.1 B-Tree](https://www.postgresql.org/docs/current/indexes-types.html#INDEXES-TYPES-BTREE)
- Hash は等価比較（`=`）だけを扱う。index 化列から導いた 32-bit hash 値を格納する。[11.2.2 Hash](https://www.postgresql.org/docs/current/indexes-types.html#INDEXES-TYPES-HASH)
- GiST と SP-GiST は単一の index ではなく実装基盤で、使える演算子は operator class 次第。標準配布の GiST は二次元幾何型（`<< &< &> >> <<| &<| |&> |>> @> <@ ~= &&`）等を扱い、対応 class では `<->` による最近傍探索を最適化できる。SP-GiST は quadtree・k-d tree・radix tree 等の非平衡構造を実装でき、標準配布は二次元 point 用 class を含む。[11.2.3 GiST](https://www.postgresql.org/docs/current/indexes-types.html#INDEXES-TYPE-GIST)・[11.2.4 SP-GiST](https://www.postgresql.org/docs/current/indexes-types.html#INDEXES-TYPE-SPGIST)
- GIN は転置 index で、配列 (array) のように複合値を含むデータに向く。成分値ごとに entry を持ち、存在検査を効率化する。標準の配列用 class は `<@ @> = &&` を扱う。[11.2.5 GIN](https://www.postgresql.org/docs/current/indexes-types.html#INDEXES-TYPES-GIN)
- BRIN (Block Range INdexes) は連続した物理 block range ごとの要約（線形順序型では各 range の最小・最大）を格納し、物理順序と相関の強い列に最も有効。`< <= = >= >` を扱う。[11.2.6 BRIN](https://www.postgresql.org/docs/current/indexes-types.html#INDEXES-TYPES-BRIN)
- index 作成後は table 更新のたびに同期され、データ操作の overhead になる。クエリに殆ど使われない index は削除する。planner が賢く判断できるよう `ANALYZE` で統計を更新する（通常は autovacuum が担う）。[11.1 Introduction](https://www.postgresql.org/docs/current/indexes-intro.html)
- 通常の index 構築は table への書き込み（`INSERT`・`UPDATE`・`DELETE`）を完了まで待たせ、読み取り（`SELECT`）は並行できる。`CREATE INDEX CONCURRENTLY` は書き込みを止める lock を取らずに構築する代わりに、table を二回 scan し、table を変更し得る既存 transaction の終了を待つため、総作業量が多く完了まで著しく長い。CPU・I/O 負荷で他の操作が遅くなることもある。[Building Indexes Concurrently](https://www.postgresql.org/docs/current/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY)
- `CREATE INDEX CONCURRENTLY` は transaction block 内で実行できない。同一 table への concurrent 構築は同時一本まで（通常構築同士の同時実行は可）。table の schema 変更はどちらの構築中も不可。失敗時は `INVALID` な index が残り、問い合わせには無視されるが更新 overhead は残る。`psql` の `\d` は `INVALID` と表示する。復旧は drop して `CREATE INDEX CONCURRENTLY` を再試行する（または `REINDEX INDEX CONCURRENTLY`）。unique index の concurrent 構築では二回目の scan 開始時点から uniqueness が他 transaction に強制され、index が利用可能になる前に別クエリで違反が報告され得る。[Building Indexes Concurrently](https://www.postgresql.org/docs/current/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY)
- `EXPLAIN` の plan は plan node の木で、下端が Seq Scan・Index Scan・Bitmap Index Scan 等の scan node、上部が join・集約・sort 等である。各 node に起動 cost・総 cost・推定行数・推定幅 (bytes) が付く。上位 node の cost は子 node を含み、`rows` は scan 数ではなく node が出力する行数である。cost は planner の cost パラメータが定める任意単位で、慣習上 disk page fetch 単位（`seq_page_cost = 1.0` 基準）。出力の文字化や client 送信の時間は含まない。[14.1.1 EXPLAIN Basics](https://www.postgresql.org/docs/current/using-explain.html#USING-EXPLAIN-BASICS)
- `EXPLAIN ANALYZE` は実際に文を実行し、推定値に加えて各 node の実測時間（ミリ秒）と実返回行数を付ける。`loops` は node 実行回数で、表示の時間・行数は実行あたり平均（総量は掛けて求める）。`ANALYZE` 指定時は `BUFFERS` 情報が自動で付く（`BUFFERS OFF` で抑止可）。`Rows Removed by Filter` は filter で捨てられた行の手掛かりになる。[14.1.2 EXPLAIN ANALYZE](https://www.postgresql.org/docs/current/using-explain.html#USING-EXPLAIN-ANALYZE)・[EXPLAIN](https://www.postgresql.org/docs/current/sql-explain.html)
- `ANALYZE` の副作用に注意する。`SELECT` の出力は破棄されるが、他の副作用は通常どおり起きる。`INSERT`・`UPDATE`・`DELETE`・`MERGE`・`CREATE TABLE AS`・`EXECUTE` を変えずに調べるには `BEGIN; EXPLAIN ANALYZE ...; ROLLBACK;` で囲む。`ANALYZE` の既定は `FALSE`、`TIMING` の既定は `TRUE`、`SERIALIZE` は `ANALYZE` 併用時のみ可、`GENERIC_PLAN` は `ANALYZE` と併用不可。[EXPLAIN](https://www.postgresql.org/docs/current/sql-explain.html)
- `Planning Time` は解析済みクエリからの plan 生成・最適化の時間で、parse・rewrite を含まない。`Execution Time` は executor の開始終了と trigger 実行を含み、parse・rewrite・planning を含まない。`BEFORE` trigger は対応 node に含まれ、`AFTER` trigger は別表示、deferred 制約 trigger は数えない。頂点 node の時間に出力の表示化・送信は含まない（`SERIALIZE` 指定で計測可）。[14.1.2 EXPLAIN ANALYZE](https://www.postgresql.org/docs/current/using-explain.html#USING-EXPLAIN-ANALYZE)
- planner が賢い判断をするには `pg_statistic` が最新である必要があり、内容が大きく変わった table は autovacuum を待たず手動 `ANALYZE` する。[EXPLAIN Notes](https://www.postgresql.org/docs/current/sql-explain.html)

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する数値や構成を指すものではない。

- 述語の形から種別を絞る。等価・範囲・`ORDER BY` には B-tree、配列の含有検査には GIN、物理順序と相関する大表の範囲絞りには BRIN を第一候補にし、`EXPLAIN` で `Index Cond`・`Index Scan` / `Bitmap Heap Scan` の選択を確認する。
- 本番 table への追加は `CREATE INDEX CONCURRENTLY` を使い、失敗時の `INVALID` 残存を `\d` で確認して作り直す手順を用意する。concurrent 構築は長時間化と周辺負荷を見込む。
- 読み取り専文の検証は `EXPLAIN (ANALYZE, BUFFERS)` で行い、推定行数と実測行数の乖離、`Rows Removed by Filter`、`Heap Blocks: exact` を見る。機械処理には `FORMAT JSON` を使う。
- 書き込み文の検証は必ず transaction で囲んで rollback する。`SELECT` でも client 送信 cost は測れないことを前提に読む。
- 統計が古い疑いがあれば `ANALYZE` 後に取り直し、toy-sized table の結果を大表に外挿しない（1 page の table はほぼ Seq Scan になる）。

## 避ける使い方

- `ANALYZE` なしの `EXPLAIN` の cost をミリ秒と読む。cost は任意単位であり、実時間との突き合わせには `EXPLAIN ANALYZE` が要る。
- `EXPLAIN ANALYZE` を書き込み文にそのまま使う。副作用は実際に起きるため、囲まず本番で実行しない。
- `col LIKE '%bar'` に B-tree を期待する。先頭固定でないパターンは対象外。
- Hash に範囲検索を期待する。等価比較専用。
- BRIN を物理順序と無相関の列に使う。要約が効かず再検査が増える（公式は相関の強い列に最も有効と明記）。
- 使われない index を残す。更新 overhead と HOT 阻害の原因になる（公式は削除を明記）。
- `enable_*` flag で plan を禁止できると考える。多くは使用を discouraging するだけで、無理な場合は `Disabled: true` 付きで選ばれる。
- `LIMIT` 付き plan の node 表示を全件実行の実測と読む。親が途中で止めるため、子 node の推定は全件前提で表示される。

## 適用版と本番での注意

- `PostgreSQL 18.6` の文書（`current` = 18）で確認する。将来の最新とは扱わない。cost の数値や選ばれる戦略は release 間の planner 改良で変わり得ることが文書に明記されている。
- 本文の種別選択と運用手順は設計判断であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。
- 未確認事項: GIN `fastupdate` や B-tree `deduplicate_items` 等の storage parameter の調整効果。必要になれば `CREATE INDEX` の該当節を別途確認する。

---
{
  "id": "postgres-zero-downtime-migration-concurrently",
  "title": "PostgreSQL の CONCURRENTLY 構築と制約分離によるゼロダウンタイム schema migration",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18.6 (retrieved page label; not a latest-version claim)",
  "tags": ["research-domain:data", "postgres", "migration", "schema-migration", "CONCURRENTLY", "CREATE INDEX CONCURRENTLY", "NOT VALID", "VALIDATE CONSTRAINT", "USING INDEX", "zero-downtime", "lock"],
  "sources": [
    {"id": "postgres-create-index-docs", "url": "https://www.postgresql.org/docs/current/sql-createindex.html", "type": "official_docs"},
    {"id": "postgres-alter-table-docs", "url": "https://www.postgresql.org/docs/current/sql-altertable.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# PostgreSQL の CONCURRENTLY 構築と制約分離によるゼロダウンタイム schema migration

index 追加と制約追加を「重い構築」と「短い制約化」に分け、書き込みを止める lock を避ける。以下は確認した公式文書の契約であり、性能数値の推奨ではない。

## 要点

以下は確認した公式文書の記載である。[CREATE INDEX](https://www.postgresql.org/docs/current/sql-createindex.html)・[ALTER TABLE](https://www.postgresql.org/docs/current/sql-altertable.html)

- 通常の index 構築は table への書き込み (`INSERT`・`UPDATE`・`DELETE`) を完了まで待たせるが、`CREATE INDEX CONCURRENTLY` は並行する insert・update・delete を妨げる lock を取らずに構築する。代償として table を二回 scan し、table を変更し得る既存 transaction の終了を待つため、総作業量が多く完了まで著しく長い。
- `CREATE INDEX CONCURRENTLY` は transaction block 内で実行できない。同一 table への concurrent 構築は同時一本まで (通常構築同士の同時実行は可)。table の schema 変更はどちらの構築中も不可。
- concurrent 構築が失敗すると `INVALID` な index が残る。問い合わせには無視されるが更新 overhead は残る。復旧は drop して `CREATE INDEX CONCURRENTLY` を再試行する (または `REINDEX INDEX CONCURRENTLY`)。
- unique index の concurrent 構築では、二回目の scan 開始時点から uniqueness が他 transaction に強制される。index が利用可能になる前に別クエリで違反が報告され得る。
- `ADD table_constraint ... NOT VALID` は FK・CHECK・NOT NULL 制約の追加時に既存行の検証 scan を省き、追加後の insert・update には制約を強制する。後から `VALIDATE CONSTRAINT` で既存行を走査して制約を満たすことを確認する。
- `VALIDATE CONSTRAINT` は制約対象 table に `SHARE UPDATE EXCLUSIVE` lock のみを取り、FK の場合は参照先 table に `ROW SHARE` lock を取る。長時間の検証中も並行書き込みを止める強い lock を取らない分離手段として文書化されている。
- `ADD PRIMARY KEY USING INDEX index_name` および `ADD UNIQUE USING INDEX index_name` は、事前に `CREATE UNIQUE INDEX CONCURRENTLY` で作った index を主キー・unique 制約に転用する。重い構築を先に終わらせ、制約化自体は速い操作として行う rollout 手順になる。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する数値や構成を指すものではない。

- 本番 table への index 追加は `CREATE INDEX CONCURRENTLY` を単文で実行する (transaction block に入れない)。同一 table への concurrent 構築を並列に投げず、一本ずつ順番に作る。
- 新しい unique 制約・主キーは二段階にする。先に `CREATE UNIQUE INDEX CONCURRENTLY` で index を作り、次に `ADD PRIMARY KEY USING INDEX` または `ADD UNIQUE USING INDEX` で制約化する。unique 違反は二回目の scan 開始時点から別 transaction に報告され得るため、切り替え前に重複混入の有無を確認する手順を用意する。
- FK・CHECK の追加は `ADD ... NOT VALID` と `VALIDATE CONSTRAINT` に分ける。既存行が多い table で一発の `ADD CONSTRAINT` を投げない。`VALIDATE CONSTRAINT` は弱い lock で走査するが長時間化し得るため、負荷の低い時間帯に回す。
- 失敗時の復旧手順を用意する。concurrent 構築後は `INVALID` が残っていないか確認し、残っていれば drop して作り直す。

## 避ける使い方

- `CREATE INDEX CONCURRENTLY` を transaction block (migration framework が既定で transaction を張る場合を含む) の中で実行する。文書の制約に反し、実行できない。
- 同一 table への concurrent 構築を同時多発させる。同時に走らせられるのは一本まで。
- index 構築中にその table の schema 変更 (列追加・型変更など) を並行させる。どちらの構築方式でも構築中の schema 変更は不可。
- 失敗後の `INVALID` index を放置する。問い合わせに使われない一方で更新 overhead が残る。
- 大表への FK・CHECK を `NOT VALID` なしの一発 `ADD CONSTRAINT` で入れる。既存行の検証 scan が書き込みを止める長時間 lock になる。
- unique 制約の切り替え前に重複があり得ないと決めつける。二回目の scan 開始後は他 transaction に違反が強制され、切り替え前に違反が表面化し得る。

## 適用版と本番での注意

- 適用版: `PostgreSQL 18.6` の文書 (`current` = 18) の `CREATE INDEX` (Building Indexes Concurrently 節) と `ALTER TABLE` (ADD table_constraint / VALIDATE CONSTRAINT / USING INDEX 節) で確認する。将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs` (TTL 90 日) で technology `postgres` の技術 TTL も 90 日のため、2026-12-26 に再取得して該当節の文言と版表示を確認する。文書の日付だけを延ばさず、該当節を再確認する。
- 本文の二段階 rollout は設計案であり、特定ベンチマークや単一事例の一般化ではない。concurrent 構築は総作業量が多く周辺負荷で他の操作が遅くなり得るため、対象 table の大きさと書き込み量で所要を見積もって採用する。
- 未確認事項 (本調査の範囲外として推測で埋めない): `NOT VALID` が使える制約種別の完全な一覧、`VALIDATE CONSTRAINT` の具体的な待機・タイムアウト・cancellation 時の振る舞い、statement_timeout や lock_timeout との相互作用、shutdown 時の扱い、レプリカへの影響、版間の動作差。これらは該当版の該当節を別途確認する。

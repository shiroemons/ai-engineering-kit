---
{
  "id": "postgres-pgdump-snapshot-restore-atomicity-boundary",
  "title": "PostgreSQL pg_dump: snapshot・並列lock失敗・restore原子性の境界",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18 versioned manuals verified 2026-10-23 UTC; patch-specific implementation not tested; PostgreSQL 19 RC1 announcement dated 2026-10-15 is prerelease context only",
  "tags": [
    "research-domain:data",
    "postgres",
    "pg_dump",
    "pg_restore",
    "pg_dumpall",
    "backup",
    "restore",
    "snapshot",
    "NOWAIT",
    "single-transaction",
    "exit-on-error",
    "transaction-size",
    "globals-only"
  ],
  "sources": [
    {
      "id": "postgres-pgdump-snapshot-18-20261023",
      "url": "https://www.postgresql.org/docs/18/app-pgdump.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-pgrestore-atomicity-18-20261023",
      "url": "https://www.postgresql.org/docs/18/app-pgrestore.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-sql-dump-scope-18-20261023",
      "url": "https://www.postgresql.org/docs/18/backup-dump.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-psql-restore-transaction-18-20261023",
      "url": "https://www.postgresql.org/docs/18/app-psql.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-pgdumpall-globals-18-20261023",
      "url": "https://www.postgresql.org/docs/18/app-pg-dumpall.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-pgrestore-sequence-rollback-18-20261023",
      "url": "https://www.postgresql.org/docs/18/functions-sequence.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-pgdump-documentation-license-18-20261023",
      "url": "https://www.postgresql.org/docs/18/legalnotice.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-pgdump-version-status-19rc1-20261023",
      "url": "https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# PostgreSQL logical backup の成功を一つのフラグにしない

## 問いと判断

**問い:** 並列 `pg_dump` が成功し、restore を「エラー時停止」にすれば、完全で all-or-nothing な復旧になるか。

**判断:** ならない。単一 database の snapshot、cluster 全体の対象範囲、dump の正常終了、restore の transaction、検査後の切替を分ける。`--exit-on-error` は失敗を早く見つける設定であり、先に commit された復元結果を消す設定ではない。transactional な変更の全体 rollback が要件なら、適用可能な単一 database restore に `--single-transaction` を使う。既存 sequence など非transactionalな状態まで巻き戻るとは扱わない。並列化や transaction 分割を選ぶなら、途中失敗した候補を利用者へ公開しない別の境界が必要になる。

本稿は [schema migration](zero-downtime-migration-concurrently.md) の index・制約追加、[transaction isolation](transaction-isolation-retry.md) のアプリケーション再試行、[SQLite backup](../sqlite/backup-done-finish-publication-boundary.md) の DONE・finish 判定の言い換えではない。PostgreSQL の logical dump と restore command の契約を扱う。以下で「公式契約」と「運用設計案」を分離する。

## 適用版・取得日・ライセンス

2026-10-23 UTC に PostgreSQL **18 の versioned manual** を実際に確認した。取得ページでは 18 が supported、19 が development に区分されている。個別 manual の公開日・最終更新日と特定 patch の実装差は表示されていないため推測しない。

[19 RC1 announcement](https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/) は 2026-10-15 公開。19 は RC1 で、GA は 2026-11-19 の予定と記載されている。予定をリリース済みの証拠にせず、本稿の 18 契約を 19 の新機能や「最新 PostgreSQL」へ読み替えない。旧版すべての同一動作も主張しない。

[Legal Notice](https://www.postgresql.org/docs/18/legalnotice.html) と [license 名の説明](https://www.postgresql.org/about/licence/) により、software と documentation の PostgreSQL License を確認した。本文は独自の要約で、upstream の実装コードは転載していない。下の command は独自の未実行例である。

## 公式契約1: snapshot の一貫性と対象範囲は別

根拠: [SQL Dump の冒頭・Using pg_dumpall](https://www.postgresql.org/docs/18/backup-dump.html)、[pg_dumpall Description / Options / Notes](https://www.postgresql.org/docs/18/app-pg-dumpall.html)。

- `pg_dump` は単一 database の一貫した snapshot を出力する。通常の並行処理は続けられるが、exclusive lock が必要な操作は別である。「online backup」は DDL 競合がないという約束ではない。
- `pg_dump` だけでは cluster 共通の roles・tablespaces・configuration parameter の privilege grants を保存しない。個別 database の dump で cluster を復元する設計なら、`pg_dumpall --globals-only` などで globals を別に確保する必要がある。
- `pg_dumpall` は database ごとに `pg_dump` を起動する。各 database は内部的に一貫していても、database 間の snapshot は同期されない。複数 database をまたぐ業務時点を揃える保証ではない。
- 元の ownership・grant を再現するには、対応する roles を restore 前に用意する。tablespace を再現するなら、移行先の場所を確認し、必要な directories を先に用意する。新規 database は、追加 object が混ざり得る `template1` ではなく `template0` を基に作る。
- globals 用 SQL は別の実行単位である。`pg_dumpall` 文書は bootstrap superuser の `CREATE ROLE` 重複などを想定済みのエラーとして説明する。すべての診断を無視するのでも、一件で database dump 自体を破損扱いするのでもなく、対象と原因を区別する。

したがって、単一 archive が存在することを「cluster 全体の同一時点復旧が可能」と表示しない。globals の取得と適用も、後述の単一 database transaction に自動的には含まれない。

## 公式契約2: 並列dumpは snapshot を共有しても lock 競合で中止する

根拠: [pg_dump の --jobs / --lock-wait-timeout](https://www.postgresql.org/docs/18/app-pgdump.html)。

`pg_dump --jobs=N` は directory format に限られ、leader と workers で N+1 connections を使う。worker ごとの接続は synchronized snapshots により同じ dataset を見る。この機能は primary では 9.2、standby では 10 からという互換条件が文書化されている。

lock の失敗経路は次の順序で説明される。

1. leader が、後で worker が読む table に `ACCESS SHARE` lock を取る。
2. 別 session の exclusive lock 要求が leader 待ちで queue に入る。
3. worker の追加 shared lock はその exclusive 要求の後ろに並び得る。
4. worker は `NOWAIT` で競合を検出する。lock を取れなければ dump 全体を中止する。

snapshot の共有は、この queue を回避する機能ではない。`--lock-wait-timeout` は開始時の shared lock 取得を無期限に待たないための設定であり、worker の NOWAIT 失敗を待機・再開へ変えたり、backup 全体の期限を保証したりしない。

## 公式契約3: stop-on-error と all-or-nothing を選び分ける

根拠: [pg_restore の --exit-on-error / --single-transaction / --jobs / --transaction-size](https://www.postgresql.org/docs/18/app-pgrestore.html)。

| restore方式 | SQLエラー時と確定境界 | 主な制約 |
|---|---|---|
| 既定の `pg_restore` | エラー後も続行し、最後に件数を表示する | 復元途中の状態が残り得る |
| `--exit-on-error` | エラー時に停止する | 先行commitは取り消さない |
| `--single-transaction` | 全commandを一transactionにまとめる。transactionalな変更は成功かrollbackに揃える | exit-on-errorを含む。複数jobsと併用不可 |
| `--transaction-size=N` | 最大N objectsずつtransactionに分割する | exit-on-errorを含むが全体atomicではない |

並列 restore は custom/directory archives の regular file または directory が必要で、pipe・標準入力は対象外である。dump を並列に作ったことと restore を並列にすることは別の選択になる。

一つの巨大 transaction は復元objectsのlockを保持し、lock table を使い切る可能性がある。transaction-size はその圧力を抑える選択肢だが、途中までの確定を許す。ここでは安全な N や jobs の固定値を提示しない。

**rollbackにも適用境界がある。** [Sequence Manipulation Functions の Caution](https://www.postgresql.org/docs/18/functions-sequence.html) は、`setval` による sequence 状態の変更が直ちに他transactionへ見え、rollbackしても取り消されないと明記する。したがって single-transaction を「任意の既存状態を完全に元へ戻す」オプションにしない。特に既存 sequence の `setval` を含むSQLや、既存schemaへ戻すdata-only経路は別確認が必要である。下の設計例は空の候補databaseへの復元を前提とし、既存本番databaseの全状態復元や任意の関数の外部副作用取消しを保証しない。

## 公式契約4: plain SQL は psql の停止条件と実行形式も揃える

根拠: [SQL Dump の Restoring the Dump](https://www.postgresql.org/docs/18/backup-dump.html)、[psql の --single-transaction / Exit Status](https://www.postgresql.org/docs/18/app-psql.html)。

plain SQL は `psql`、archive は `pg_restore` を使う。psql は既定で SQL エラー後も処理を続ける。`ON_ERROR_STOP=on` は script の SQL エラーで停止し exit status 3 を返すが、それだけでは先行commitをrollbackしない。

psql の `--single-transaction` は `-c` または `-f` と組み合わせる必要がある。`ON_ERROR_STOP` と合わせることで失敗時の rollback を行う。script 自身が `BEGIN`・`COMMIT`・`ROLLBACK` を含む場合は期待する囲み方にならず、transaction 内で禁止される command があれば全体が失敗する。

以下は、事前に用意した空の単一 database へ、transaction 内で実行できる SQL file を戻す場合だけの未実行例である。

```sh
psql -X --set=ON_ERROR_STOP=on --single-transaction --dbname=restore_candidate --file=database_candidate.sql
```

`-X` は利用者の psqlrc による設定混入を避ける。`-f` を落として単なる標準入力redirectの例に変えない。database作成・接続切替を含む pg_dumpall script を、この例にそのまま当てはめて cluster 全体のatomic restoreと主張しない。

## 版と入力の信頼境界

根拠: [pg_dump Notes](https://www.postgresql.org/docs/18/app-pgdump.html)、[pg_restore Warning](https://www.postgresql.org/docs/18/app-pgrestore.html)。

- pg_dump client は自分より新しい major の server を dump できず、試行を拒否する。18 manual は source server を 9.2 まで遡れると記す。一方、出力を client より古い major へロードできる保証はなく、元serverがその古い版だった場合も例外ではない。source server、dump client、restore client、target server を別々に記録し、downgradeを当然に成功する経路にしない。
- restore は source superuser が選んだコードを実行する。partial dump・partial restore を使ってもこの信頼境界は消えない。不信なsourceは復元前に内容を確認する。archiveは `pg_restore --file` でSQLを出力して検査できるが、出力できたことはコードの安全性を証明しない。

## 運用設計案: 候補の復元と利用開始を分離する

以下は公式のbackup製品・切替protocolではなく、本稿の設計提案である。

1. **回復の単位を決める。** 単一databaseのsnapshotで足りるか、globalsや複数databaseの整合も必要かを記録する。後者をpg_dumpの成功へ暗黙に含めない。対象、除外、tool/server版、開始・終了、dump結果、globals取得結果をmanifestへ残す。
2. **生成先を候補として扱う。** DDLとの実行時間を調整し、並列dumpの中止を捕捉する。失敗後のdirectoryや一部fileを最新の検証済みbackupへ昇格しない。再試行は原因を調べて新しい候補として行い、前試行の残骸に依存するresume契約を発明しない。
3. **restore要件を先に選ぶ。** 単一transactionが適用可能で全体rollbackが必要なら、隔離した空のtargetへ `pg_restore --single-transaction --dbname=restore_candidate archive_candidate` を使う。これは未実行の組立例で、target作成・globals適用・本番切替を同じtransactionにする例ではない。
4. **部分commitを選ぶなら公開を止める。** 並列またはchunked restoreでは、エラー時停止に加え、失敗したcandidateを通常利用へ回さない。再試行前に既存objectsや投入済みdataを把握する。同じtargetへそのまま重ねれば安全という仮定を置かない。処分・再作成は対象を確認して別手順で行う。
5. **終了コードと復元検査を分ける。** エラーの原因を記録した上で、必要なroles、schema、constraints、件数や業務不変条件、代表的なapp操作を検査する。archive一覧が読めることやexit codeだけを、必要な機能まで復旧した証拠にしない。検査後のrouting切替とrollback先の保持は別設計とする。

この設計が狙うのは「途中の復元状態を利用可能扱いにしない」こと。遠隔保存先の耐久性、PITR、物理backup、replication slot、extensionの導入方法や認証情報の保存は本稿の対象外である。

## 未実行のfault試験案と残る制約

PostgreSQL runtime試験は実行していない。以下は仕様から作った確認項目で、実測結果ではない。使い捨ての隔離clusterで実配布版を固定し、期待と実測を比較する。

- 並列dump中にexclusive DDLをqueueし、workerのNOWAIT失敗・dump終了・候補の非公開を確認する。timing依存なので再現できなかった一回を競合なしの証拠にしない。
- 復元の中盤にSQLエラーを起こし、既定・exit-on-error・single-transactionで停止点と残存objectsを比較する。
- transaction-sizeで複数groupが確定した後に失敗させ、前groupが全体rollbackされないことを確認する。
- 元owner roleやtablespace directoryが不足するtargetで、データ不足と前提条件不足をログ上で区別できるか確かめる。
- plain SQLのエラーを使い、psqlのexit statusとtransactionの結果を別々に確認する。既存sequenceへのsetval後に失敗するcaseでは、通常のtable変更のrollbackとsequence値を別々に観測する。

lock競合の実測、巨大restoreの資源上限、extension・collation差、client archive formatの版間互換性、電源断、network断後の結果確定、実運用切替は未検証である。single-transactionを任意の外部副作用まで取り消す仕組みとも主張しない。

再確認期限は 2026-11-22。19 RC1 release_notes の30日TTLが、18 manualとpostgres technologyの90日TTLより短いためである。更新時はreleaseの実際の状態と全参照節を再確認し、日付だけ延ばさない。

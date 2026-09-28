---
{
  "id": "postgres-transaction-isolation-retry",
  "title": "PostgreSQL のトランザクション分離レベルと serialization-failure 再試行契約",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18.6 (retrieved page label; not a latest-version claim)",
  "tags": ["postgres", "transaction", "isolation-level", "serializable", "repeatable-read", "read-committed", "serialization-failure", "40001", "retry", "research-domain:data"],
  "sources": [
    {"id": "postgres-transaction-iso-docs", "url": "https://www.postgresql.org/docs/current/transaction-iso.html", "type": "official_docs"},
    {"id": "postgres-errcodes-appendix-docs", "url": "https://www.postgresql.org/docs/current/errcodes-appendix.html", "type": "official_docs"},
    {"id": "postgres-set-transaction-docs", "url": "https://www.postgresql.org/docs/current/sql-set-transaction.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# PostgreSQL のトランザクション分離レベルと serialization-failure 再試行契約

どの分離レベルで何が起き、`40001` を受けたら何を再試行するかを決める。以下は公式文書の契約であり、再試行の回数や待機時間の推奨ではない。

## 要点

- 内部実装は三つの分離レベルだけであり、`READ UNCOMMITTED` を指定しても `READ COMMITTED` として扱われる。[13.2 Transaction Isolation](https://www.postgresql.org/docs/current/transaction-iso.html)
- 既定は `READ COMMITTED` で、各コマンドが実行開始時点の snapshot を取得する。あるクエリで見えた行が後のクエリで変更・削除済みになることがあり、更新対象行が同時 transaction に更新・削除・ロックされていた場合はその完了を待つ。[13.2.1 Read Committed Isolation Level](https://www.postgresql.org/docs/current/transaction-iso.html#XACT-READ-COMMITTED)
- `REPEATABLE READ` は全 snapshot を transaction 開始時点に固定する。自分の更新対象行が同時 transaction に更新・削除されていた場合は待機し、待ち相手が commit するとその更新はキャンセルされ、`ERROR: could not serialize access due to concurrent update` で abort する。公式は「transaction 全体を再試行せよ」と明記する。[13.2.2 Repeatable Read Isolation Level](https://www.postgresql.org/docs/current/transaction-iso.html#XACT-REPEATABLE-READ)
- `SERIALIZABLE` は全 snapshot を transaction 開始時点に固定した上で、シリアル実行と矛盾し得る read/write 依存を監視し、述語ロック (predicate locking) で追跡する。並行実行をコミットすると直列順序が存在し得ない場合、関与した transaction のいずれかを `ERROR: could not serialize access due to read/write dependencies among transactions` でロールバックする。公式は「serialization failure を起こす transaction を再試行せよ」と明記する。[13.2.3 Serializable Isolation Level](https://www.postgresql.org/docs/current/transaction-iso.html#XACT-SERIALIZABLE)
- serialization failure の SQLSTATE は `40001` (serialization_failure) であり、`40P01` (deadlock_detected) とは別の符号である。どちらも Class 40 (Transaction Rollback) に属する。アプリケーションは局所化され得るメッセージ文面ではなく SQLSTATE の符号で判定すべきである。[Appendix A PostgreSQL Error Codes](https://www.postgresql.org/docs/current/errcodes-appendix.html)
- 分離レベルの指定構文は `SET TRANSACTION ISOLATION LEVEL { SERIALIZABLE | REPEATABLE READ | READ COMMITTED | READ UNCOMMITTED }` である。最初のクエリまたはデータ変更文の実行後に分離レベルを変更することはできない。[SET TRANSACTION](https://www.postgresql.org/docs/current/sql-set-transaction.html)
- 同時実行する serializable read/write ワークロードでは、一貫性が崩れるパターンが `serialization_failure` でロールバックされることが文書に明記されている。[SET TRANSACTION](https://www.postgresql.org/docs/current/sql-set-transaction.html)
- `DEFERRABLE` は `SERIALIZABLE` かつ `READ ONLY` の transaction にだけ効果を持つ。それ以外の transaction では無視される点に注意する（公式は「効果は SERIALIZABLE READ ONLY に限られる」旨を明記）。[SET TRANSACTION](https://www.postgresql.org/docs/current/sql-set-transaction.html)

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する再試行回数や待機方式を指すものではない。

- `SET TRANSACTION` は transaction の最初の文の前に実行する。最初のクエリ以降は変更できないため、分離レベルを切り替える用途では新しい transaction を開始する。
- `40001` を受けたら文単位ではなく transaction 全体を最初から再試行する。`REPEATABLE READ` の concurrent-update エラーと `SERIALIZABLE` の read/write 依存エラーは、どちらも abort 後の再試行が公式の契約である。
- エラー分岐は SQLSTATE 符号で行う。`40001` (serialization_failure) と `40P01` (deadlock_detected) を区別し、メッセージ文面の一致では判定しない。
- `READ COMMITTED` 既定のまま強い不変条件を守ろうとしない。transaction 開始時点の一貫した snapshot が要る読み取りは `REPEATABLE READ` 以上、直列化可能性が要る読み書き混在は `SERIALIZABLE` を選び、再試行路を用意する。
- `DEFERRABLE` は `SERIALIZABLE READ ONLY` の長い読み取り専用 transaction に限定して使う。読み書き transaction では効果がない。

## 避ける使い方

- `READ UNCOMMITTED` でダーティリードを期待する。内部では `READ COMMITTED` として扱われる。
- `SERIALIZABLE` を選びながら `40001` の再試行路を用意しない。同時実行の read/write パターンはロールバックされ得ることが文書化されている。
- 失敗した文だけを再実行して transaction を継続する。`REPEATABLE READ` の concurrent-update と `SERIALIZABLE` の serialization failure は transaction の abort と全体再試行が公式の扱いである。
- 文面一致で `40001` を検出する。メッセージは局所化され得るため、公式の符号表に従い SQLSTATE で判定する。
- transaction 途中で `SET TRANSACTION ISOLATION LEVEL` を変えて効くと考える。最初のクエリ以降の変更はできない。
- `DEFERRABLE` を読み書き transaction や `REPEATABLE READ` 以下で使う。効果は `SERIALIZABLE READ ONLY` に限られる。

## 適用版と本番での注意

- `PostgreSQL 18.6` の文書 (`current` = 18) で確認する。将来の最新とは扱わない。述語ロックの実装方式やエラー文面は release 間で変わり得るため、符号 (`40001` / `40P01`) を契約として扱う。
- 本文の再試行の回数・待機・冪等化の判断は設計案であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。
- 未確認事項: `default_transaction_isolation` の既定値変更の影響、`SERIALIZABLE` 下での具体的なスループット低下量、述語ロックのメモリ上限。必要になれば該当節を別途確認する。

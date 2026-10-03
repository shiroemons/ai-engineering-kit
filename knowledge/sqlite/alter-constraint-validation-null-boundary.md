---
{
  "id": "sqlite-alter-constraint-validation-null-boundary",
  "title": "SQLite 3.53 制約 migration: ADD CHECK の既存行検査と NULL・NOT NULL の境界",
  "kind": "knowledge",
  "technology": "sqlite",
  "version": "SQLite 3.53.0 (2026-04-09) introduced direct NOT NULL/CHECK edits; source observations pinned to 3.53.0 and 3.53.4 (2026-07-24); documentation retrieved 2026-10-03 UTC; runtime migration tests not executed",
  "tags": [
    "research-domain:data",
    "sqlite",
    "alter-table",
    "schema-migration",
    "check",
    "not-null",
    "constraint-validation",
    "null",
    "transaction",
    "rollback"
  ],
  "sources": [
    {
      "id": "sqlite-353-constraint-release-20261003",
      "url": "https://www.sqlite.org/releaselog/3_53_0.html",
      "type": "release_notes"
    },
    {
      "id": "sqlite-352-constraint-withdrawal-20261003",
      "url": "https://www.sqlite.org/releaselog/3_52_0.html",
      "type": "release_notes"
    },
    {
      "id": "sqlite-3534-constraint-release-20261003",
      "url": "https://www.sqlite.org/releaselog/3_53_4.html",
      "type": "release_notes"
    },
    {
      "id": "sqlite-alter-constraint-contract-20261003",
      "url": "https://www.sqlite.org/lang_altertable.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-check-null-semantics-20261003",
      "url": "https://www.sqlite.org/lang_createtable.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-constraint-boolean-expressions-20261003",
      "url": "https://www.sqlite.org/lang_expr.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-constraint-migration-transactions-20261003",
      "url": "https://www.sqlite.org/lang_transaction.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-353-constraint-implementation-20261003",
      "url": "https://github.com/sqlite/sqlite/tree/4ebc7fdcf459e8d88eb5b019c2949bda86565528",
      "type": "github_repository_analysis"
    },
    {
      "id": "sqlite-3534-constraint-implementation-20261003",
      "url": "https://github.com/sqlite/sqlite/tree/b09c88c14082339b66c7b7158d609a771e64ca69",
      "type": "github_repository_analysis"
    },
    {
      "id": "sqlite-constraint-public-domain-20261003",
      "url": "https://www.sqlite.org/copyright.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# SQLite 3.53 の制約追加と既存行検査

## 問いと結論

既存列へ NOT NULL や CHECK を追加するとき、table の再構築を省けるようになったことは「検査なし・無停止」を意味するか。nullable な列へ通常の CHECK を付ける場合、INSERT と同じ NULL の扱いだけを前提に migration を組めるか。

**判断:** 3.53 の直接変更を候補にする一方、既存行の検査、制約の意味、transaction の確定を別々に確認する。特に ADD CHECK の NULL 境界は、通常の CHECK の説明から移行結果を推測せず、採用する runtime で試す。以下は公式契約、固定ソースの静的観察、独自の移行案を分けた資料であり、実行済み benchmark や不具合再現報告ではない。

既存の [foreign key 施行](foreign-key-enforcement.md) は接続設定と参照整合性、[stale expression index](stale-expression-index-reindex-migration.md) は保存済み index key の問題を扱う。本書は既存 table の制約を変更する経路を対象にする。

## 導入版と資料の読み分け

[3.53.0 release notes](https://www.sqlite.org/releaselog/3_53_0.html) は 2026-04-09 公開で、ALTER TABLE による NOT NULL / CHECK の追加・削除を列挙する。[3.52.0](https://www.sqlite.org/releaselog/3_52_0.html) は Withdrawn とされ、予定機能は3.53.0へ移された。3.52.0を採用基準にはしない。

[ALTER TABLE reference](https://www.sqlite.org/lang_altertable.html) の第6節は SET / DROP NOT NULL の導入版を3.53.0と明記する。一方、同じページの概要と一般変更節には直接変更を旧来の4操作に限る記述が残り、CHECK の追加・削除の本文説明も十分でない。新構文がないと即断せず、release notes と下記の固定実装・公式テストを照合した。

- 3.53.0: Git commit `4ebc7fdcf459e8d88eb5b019c2949bda86565528`、commit日時 2026-04-09 11:41:38Z
- 3.53.4: Git commit `b09c88c14082339b66c7b7158d609a771e64ca69`、commit日時 2026-07-24 19:02:57Z。[patch release](https://www.sqlite.org/releaselog/3_53_4.html) の日付も2026-07-24

公式 Git mirror の tag を解決し、各 `VERSION` と `manifest.uuid` を読んだ。Fossil source ID は順に `4525003a53a7fc63ca75c59b22c79608659ca12f0131f52c18637f829977f20b` と `bf7c7f30031888f4e796e429ab3978879485813aaca6f641c7b33e4e09459bcc`。Git SHAとFossil IDは別の識別子である。両端の固定ソースの確認であり、間の全patch、vendor backport、最新開発版を調べたとは扱わない。

## 直接変更できる範囲

次の SQL は用途を示すために作成した独自例。`inventory` は説明用の通常tableであり、既存データを変更する本番手順として無検査で実行しない。

```sql
ALTER TABLE main.inventory ALTER COLUMN sku SET NOT NULL;
ALTER TABLE main.inventory ALTER COLUMN sku DROP NOT NULL;
ALTER TABLE main.inventory ADD CONSTRAINT qty_nonnegative CHECK (qty >= 0);
ALTER TABLE main.inventory DROP CONSTRAINT qty_nonnegative;
```

NOT NULL は [第6節](https://www.sqlite.org/lang_altertable.html)、CHECK は3.53.0の [parser](https://github.com/sqlite/sqlite/blob/4ebc7fdcf459e8d88eb5b019c2949bda86565528/src/parse.y) と両版の `altercons.test` を根拠にする。`COLUMN` は省略可能で、CHECK は名前のない `ADD CHECK (...)` も実装されている。ただし削除・差分確認が必要な運用では名前を付ける案が扱いやすい。

- 一般的な `ADD CONSTRAINT` 全種別の実装ではない。調べた構文で追加できるのは CHECK。PRIMARY KEY / UNIQUE / FOREIGN KEY の追加へ拡張解釈しない
- [3.53.4 altercons.test](https://github.com/sqlite/sqlite/blob/b09c88c14082339b66c7b7158d609a771e64ca69/test/altercons.test) の2節は、名前付き UNIQUE の削除を拒否する。存在しない名前の削除もエラーになる
- [3.53.4 altercons2.test](https://github.com/sqlite/sqlite/blob/b09c88c14082339b66c7b7158d609a771e64ca69/test/altercons2.test) の8節は、名前付き FOREIGN KEY の削除を拒否する。名前が付いていれば任意の制約を削除できるわけではない
- `altercons.test` の6.4節は既存名と同名の CHECK 追加を拒否し、元のschemaが残ることを期待する。再送を「同じ名前だから no-op」として設計しない
- 同7節は generated column に対する NOT NULL の追加と既存値検査を扱う。view や隠し rowid を同じ列変更経路へ渡すことは拒否される。アプリのschema introspectionも対象列の種類を保持する

ここでいう公式テストの確認はテスト内容を読んだことを意味し、この調査でテストスイートを実行したという意味ではない。

## 追加時は既存行を読む。NULL は別の境界になる

### NOT NULL: 欠損行を直さずに制約だけ付けない

3.53.0 / 3.53.4の `src/alter.c` にある `sqlite3AlterSetNotNull` は、対象列が NULL の既存行を検索し、見つかれば `SQLITE_CONSTRAINT` を発生させてからschema更新へ進まない構造になっている。`altercons.test` の5.2節は、NULL行を含むtableへの変更が `constraint failed` となることを期待する。generated columnについても7節に同種の境界例がある。

したがって、直接変更は全行を別tableへコピーする方式を避けるが、既存行の確認まで不要にはしない。読み取り量・所要時間はデータと計画次第であり、「必ず一定回数のI/O」「常に瞬時」という保証にしない。事前に欠損行を調べ、業務上の正しい補完・除外方針を決める。

### CHECK: 通常書込みと ADD CHECK の検査を同一視しない

[CREATE TABLE の CHECK 契約](https://www.sqlite.org/lang_createtable.html) は、式の結果を数値へ変換し、ゼロなら違反、NULL または非ゼロなら違反ではないとする。たとえば `CHECK (qty >= 0)` 自体は qty の NULL 禁止を意味しない。通常の読込み時に制約を再検査する保証もなく、CHECK 検査は `ignore_check_constraints` で無効化され得る。

一方、固定して読んだ [3.53.0 の実装](https://github.com/sqlite/sqlite/blob/4ebc7fdcf459e8d88eb5b019c2949bda86565528/src/alter.c) と [3.53.4 の実装](https://github.com/sqlite/sqlite/blob/b09c88c14082339b66c7b7158d609a771e64ca69/src/alter.c) の `sqlite3AlterAddConstraint` は、schemaを書き換える前の検索条件に `(expr) IS NOT TRUE` を使う。両方にこの形を確認した。

[式の文書](https://www.sqlite.org/lang_expr.html) が定める通常のboolean literalの解釈では、NULLに対する `IS NOT TRUE` は真になる。この静的な読み合わせから、**ADD CHECK 時の既存 NULL 評価行が、通常の CHECK なら許される行でも拒否対象になる可能性を移行試験に入れる**。これは公式に確認された runtime bug や、その修正状況の断定ではない。テストの6.2節は偽になる既存行の拒否と、正常追加後のINSERT拒否を確認するが、NULL評価の実行結果までは今回得ていない。

設計案として、NULLを許す業務条件なら `CHECK (qty IS NULL OR qty >= 0)` のように許容を明示した式も比較候補になる。NULLを禁止したいなら NOT NULL と値のCHECKを分けて表す。どちらも採用runtimeで既存NULL・負数・ゼロ・正数と追加後の書込みを試す。移行を通すためだけにNULLを0へ置換したり、検査を無効化したりしない。

## 削除・再試行の完了判定

[ALTER TABLE reference](https://www.sqlite.org/lang_altertable.html) は、既にNOT NULLの列への SET NOT NULL は冗長な制約を追加しないとする。一方、CREATE TABLEで重複したNOT NULLを持つschemaは存在し得て、DROP NOT NULLは少なくとも一つを除去するが、必ず全部を除去するとは限らない。

したがってエラーなく一回DROPできたことと、期待どおりnullableになったことは別に照合する。名前付きCHECKでも、migration記録と実際の `sqlite_schema` の定義を比較する。タイムアウトや接続消失の後は「未実行」と決めて同じDDLを再送せず、現在の定義とtransaction状態を確認する。期待する制約が既にある場合は、名前だけでなく式と対象列も一致させる。これらは独自の運用上の判断である。

また、直接制約変更の成功を任意のschema破損の修復とは扱わない。`writable_schema=ON` をエラー回避策にすると、通常なら拒否される解析不能なschemaを無視し得る。DDL拒否を抑制するための設定変更は、本書の移行手順に含めない。

## 移行の組み立て方（独自の提案）

1. **対象を固定する。** アプリが実際に使用するSQLiteの版とbuild条件、database/schema、生成列、依存index・trigger・view、制約名と式を記録する。OS付属CLIだけの版確認では足りない。3.53の構文を旧runtimeで実行する経路を残さない
2. **復元できるコピーで評価する。** writerの停止許容と復帰条件を決める。既存行の検査と移行後の INSERT / UPDATE を別ケースにし、NULL・空table・不正値・重複名を含める。大表で検査時間を測り、schema編集が小さいことから無停止を公約しない
3. **事前検査と適用の間を管理する。** 単独migration runnerが、必要な検査・修正・DDLを同じ明示transactionで扱う構成を比較する。先に別接続で調べた件数は診断用であり、並行writerが動く中での最終保証ではない
4. **writer競合を扱う。** [transaction契約](https://www.sqlite.org/lang_transaction.html) では同時writerは一つ。BEGIN IMMEDIATEは開始時にwrite transactionを確保するが、既存writerがいればBUSYになり得る。採用時は待機・中断・再試行の期限を設ける。migration frameworkが既にBEGINしている場合は二重BEGINにしない
5. **失敗を分類する。** 既存行の制約違反、構文非対応、名前衝突、BUSYを同じ「retry可能」にしない。途中DDLの失敗後に後続処理とCOMMITだけ進めない。runnerは自身のtransaction状態を把握し、失敗時のrollbackと記録を実装する
6. **COMMITと事後状態を確認する。** COMMITがBUSYならtransactionはactiveのままの場合がある。statement成功だけで移行済み記録を確定しない。期待したschema、通常writerでの制約施行、関連参照の健全性、復帰手順を確認してから切替完了にする

NOT NULL / CHECKの直接変更だけを目的として、既存のforeign key施行を無条件にOFFへする必要は導けない。既存の [foreign key 検査](foreign-key-enforcement.md) は別の検査対象として維持する。

## 旧版・非対応変更は再構築経路へ分ける

直接構文が使えない旧版や、型・主キー・外部キーなど別の変更をする場合は、[公式の一般schema変更手順](https://www.sqlite.org/lang_altertable.html) を対象版で確認する。要点は新しいtableを望むschemaで作成、明示列でデータを移す、旧tableを削除、新tableを元名へrename、依存objectを再構築して検査する順序である。旧tableを先に仮名へrenameする手順は参照を書き換えてしまう危険がある。

これは12段階手順の短い案内であり、省略可能な部分だけを実行する指示ではない。foreign_keysの扱いはtransaction前後の規定も含めて確認し、元の状態を復元する。データをコピーできたことだけでtrigger・index・viewまで移ったとは扱わない。`CREATE TABLE AS SELECT` では元の制約を継承しないという [CREATE TABLE契約](https://www.sqlite.org/lang_createtable.html) にも注意する。

## 導入前の受入試験と未確認事項

この調査で追加する検索evalは文書の発見可能性だけを検査する。以下のSQLite実行試験は未実行である。

- SET NOT NULL: NULLあり・なし、空table、generated columnで、失敗後のschemaと成功後の書込みを確認する
- ADD CHECK: 偽・真・NULLに評価される既存行を分け、通常INSERTのCHECK検査と比較する。3.53.0/.4の静的観察を別patchの実測へ流用しない
- NULL許容を明示する案: `qty IS NULL OR qty >= 0` と元の式で、受理する既存値・新規値が業務要件に一致するかを確かめる
- DROPと再試行: 冗長NOT NULL、同名CHECK、存在しない名前、UNIQUE/FKの誤削除、応答消失後のschema照合を試す
- transaction: 別writer、長いreader、BEGIN時BUSY、COMMIT時BUSY、中断を入れ、runnerが半端な移行を成功記録しないかを確認する
- 旧版fallback: 依存object・foreign key・generated column・row識別子・schema属性を照合し、復元からアプリ疎通まで試す

NULL境界の実行再現、SQLite開発側の評価・修正有無、全patch差分、bindingのerror変換、特定ORMのDDL生成、WAL/rollback journal別の実測lock時間、全vendor buildでの互換性は未確認。release 3.53.2ページはこの取得経路でアクセスに失敗し、当該patch固有の修正主張には使っていない。3.53.0初版への更新推奨や、3.53.4を最新・無欠陥とする主張も行わない。

## 出典・日付・ライセンス

取得日はすべて2026-10-03 UTC。公開日・ページ更新日・commit日時は区別し、catalogに記録した。

- 3.53.0 / 3.53.4の公開日は2026-04-09 / 2026-07-24。3.52.0ページの見出しは2026-03-06で、撤回自体の日付は同ページに明示されない
- ALTER TABLE更新表示は2026-06-04 01:35:31Z、CREATE TABLEは2025-04-30 20:02:34Z、Expressionsは2026-08-14 19:21:30Z、Transactionは2026-02-18 11:56:50Z
- [Copyright](https://www.sqlite.org/copyright.html) 更新表示は2026-01-12 12:33:24Z。SQLiteコードと文書のpublic domain宣言を確認した。両固定commitのLICENSE.mdはsrc/とtest/も対象に含め、読んだ実装・テストのheaderも著作権放棄を示す
- 原文の長文・実装コード・テストコードの転載はなく、SQL例は本書用の独自例。module化、実SQLiteのビルド・実行は行っていない。当repository自体の配布ライセンスを決める記録ではない

release_notesのTTL 30日が最短なので再確認期限は2026-11-02。更新時は新構文の説明、NULL検査の実装、対象patchと試験結果を読み直し、取得日だけを延長しない。

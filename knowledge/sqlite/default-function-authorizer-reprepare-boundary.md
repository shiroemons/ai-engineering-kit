---
{
  "id": "sqlite-default-function-authorizer-reprepare-boundary",
  "title": "SQLite 3.54 DEFAULT関数の認可: prepare・IGNORE・prepared statementの寿命",
  "kind": "knowledge",
  "technology": "sqlite",
  "version": "DEFAULT-function authorization introduced in SQLite 3.54.0 (2026-10-09); static implementation review pinned to 3.54.1 (2026-10-16), Git 57520227c95396ce39ba6ecaac8d475b78dd3964; rolling C API docs verified 2026-10-24 UTC; runtime not tested",
  "tags": [
    "research-domain:data",
    "sqlite",
    "DEFAULT",
    "authorizer",
    "SQLITE_FUNCTION",
    "SQLITE_IGNORE",
    "prepare",
    "reprepare",
    "statement-cache",
    "trusted_schema",
    "SQLITE_DIRECTONLY"
  ],
  "sources": [
    {
      "id": "sqlite-354-default-auth-release-20261024",
      "url": "https://www.sqlite.org/releaselog/3_54_0.html",
      "type": "release_notes"
    },
    {
      "id": "sqlite-3541-default-auth-patch-20261024",
      "url": "https://www.sqlite.org/releaselog/3_54_1.html",
      "type": "release_notes"
    },
    {
      "id": "sqlite-authorizer-compile-contract-20261024",
      "url": "https://www.sqlite.org/c3ref/set_authorizer.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-authorizer-action-arguments-20261024",
      "url": "https://www.sqlite.org/c3ref/c_alter_table.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-function-schema-flags-20261024",
      "url": "https://www.sqlite.org/c3ref/c_deterministic.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-application-function-safety-20261024",
      "url": "https://www.sqlite.org/appfunc.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-prepare-recompile-contract-20261024",
      "url": "https://www.sqlite.org/c3ref/prepare.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-3541-default-auth-implementation-20261024",
      "url": "https://github.com/sqlite/sqlite/tree/57520227c95396ce39ba6ecaac8d475b78dd3964",
      "type": "github_repository_analysis"
    },
    {
      "id": "sqlite-default-auth-public-domain-20261024",
      "url": "https://www.sqlite.org/copyright.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# DEFAULT関数の認可とprepared statementの寿命を分ける

## 問いと判断

SQLiteを更新した後、従来動いていたDEFAULT式付きDDLやINSERTがauthorizerで拒否されるのはどの段階か。関数の許可表を更新すれば、statement cacheや実行中queryにも直ちに新policyが適用されるか。

**判断:** SQLite 3.54のDEFAULT関数チェックは、schemaを開く検査、関数の各実行、既存処理の取消しと同じものではない。upgradeではcallbackの返値を見直し、policy変更時には物理接続・prepare済みstatement・実行中処理の寿命を合わせて管理する。本稿は公式契約、固定実装の静的観察、独自の運用案を分ける。

既存の[schema・trigger上限](schema-trigger-admission-budget-boundary.md)は構造の受付予算であり、実行権限を扱わない。[CHECK / NOT NULL migration](alter-constraint-validation-null-boundary.md)の既存行検査や[expression indexの再構築](stale-expression-index-reindex-migration.md)とも別の問いである。

## 1. 3.54で増えた呼出しと適用版

[3.54.0 release notes][release]は2026-10-09、CREATE TABLEのDEFAULT式に含まれるSQL関数をauthorizerへ渡す変更を記載する。CREATE TABLE時とDEFAULTを利用する経路が対象で、databaseを開く際のschema読込みでは呼ばない。この変更を3.53へ遡って適用しない。

本稿は[3.54.1][patch]（2026-10-16）を固定して実装を読む。Git tag `version-3.54.1` は `57520227c95396ce39ba6ecaac8d475b78dd3964`、`VERSION` は3.54.1、`manifest.uuid` はreleaseのFossil source ID `555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76` と一致した。Git SHAとFossil IDは別の識別子である。

3.54.1の公開日はこの機能の導入日ではない。patch告知の32-bit build修正も、authorizerを設定すれば古いbinaryを安全に使えるという意味ではない。以下はvendor build、全patch、driverの実行確認ではなく、rolling文書と固定ソースの照合結果である。

## 2. 「DEFAULTを利用する」は各rowの実行時callbackではない

[authorizer API][authorizer]はSQLのcompile時に認可する。通常の `sqlite3_step()` 評価ごとの認可ではなく、schema変更によるreprepareがstep内で起きるとcallbackが再度呼ばれ得る。そのためprepareだけ一時的にcallbackを設定し、step前に外す運用をしない。接続には一つしか登録できず、次の登録は前を置換する。

固定3.54.1の確認箇所は次のとおり。[implementation]

- `src/build.c:sqlite3AddDefaultValue()` はschema初期化中でなくauthorizerがある場合、DEFAULT式を `sqlite3FuncAuth()` で走査する。`db->init.busy` の分岐がschema読込みと新DDLを区別する
- 同ファイルの `sqlite3ColumnExprAuth()` は列の式を返す際に関数認可を追加する。認可なしの `sqlite3ColumnExpr()` とは別の入口である
- `src/insert.c:sqlite3Insert()` はINSERTの列リストから省かれた列、`INSERT ... DEFAULT VALUES` などのコード生成で認可付き入口を使う。callbackは生成するSQL programに対する検査であり、関数評価回数の計数器ではない

**独自の帰結:** callbackの件数をINSERT件数、成功row数、関数の副作用回数のaudit logとして使わない。defaultが評価されるかを行ごとに問い合わせる仕組みと考えず、prepare / reprepareの失敗も実行経路のerrorとして扱う。schemaを開けたことは、そのDEFAULTを使うINSERTが現在のpolicyで許可される証拠にならない。

generated columnのauthorizerは[APIの別の注意書き][authorizer]でschema解析時の契約が説明される。DEFAULTの変更から、generated column・trigger・viewすべてが同じ時点に検査されるとは推定しない。

## 3. SQLITE_IGNOREはDEFAULT関数のNULL置換ではない

次は**3.54.1固定実装と上流testの静的観察**であり、本調査で実行した結果ではない。[implementation]

| 調べた経路 | SQLITE_FUNCTIONへの返値 | 確認した扱い |
|---|---|---|
| 通常の式を解決する `src/resolve.c` | `SQLITE_IGNORE` | 関数式を `TK_NULL` にする |
| DEFAULTを走査する `src/build.c:authFunctions()` | `SQLITE_IGNORE` | 非zeroとしてerrorを設定し、走査を中止する |
| 同DEFAULT経路 | `SQLITE_DENY` | 拒否としてerrorを設定する |
| 同DEFAULT経路 | `SQLITE_OK` | 当該関数の認可検査を通過する。他の制約成功までは保証しない |

`test/auth2.test` の `auth2-1.3` は `min()` にIGNOREを返したSELECTがNULLになること、`auth2-1.5` はその式を `coalesce(...,999)` が受け取ることを期待する。一方、DEFAULT経路のwalkerは `if( rc )` でerrorにする。一般のSELECT関数で成立するNULL置換をDEFAULTへ持ち込まない。

**独自のupgrade判断:** 「許可していない関数は常にIGNORE」というcallbackは、3.54でDDLやdefault利用文を拒否し得る。まず必要な関数を棚卸しし、許可 / 拒否の意味を明示する。互換性回復のために全関数をOKへ変更したり、error時だけcallbackを外してretryしたりしない。NULLにしたいという業務仕様は、認可callbackの副作用に頼らず、許可されたSQL・schema変更として別に設計する。

この比較は確認した二つの経路に限る。すべてのactionでIGNOREがNULLになる、またはすべてのcontextでIGNOREがDENYと同じになるという一般規則ではない。

## 4. callbackの識別情報から引数値やschema安全性を推定しない

[action code reference][actions]で `SQLITE_FUNCTION` の第3引数はNULL、第4引数が関数名である。第5引数のdatabase名は適用可能な場合だけで、第6引数は原因となる最も内側のtrigger / view名またはNULL。文字列のcaseは固定されないので、比較にはcase-insensitiveな処理を用いる。

**独自の注意点:** 第6引数がNULLだからschema由来の式ではない、という認可条件を作らない。DEFAULTはtrigger / viewではなく、固定 `authFunctions()` も関数名を渡すだけでDEFAULT列名や関数引数値を新たに渡していない。[implementation]

関数の同名overload、引数の機密性、tenantごとの利用目的までこの通知だけで確定できるとは言えない。callbackはNULLを扱い、必要な認可単位に足りない情報はapplication側で設計する。callbackから同じ接続をprepare / stepして情報を追加取得しない。APIは呼出し元接続を変更する操作を禁止する。[authorizer]

## 5. setterによるcache失効は実行中statementの即時取消しではない

固定3.54.1の `src/auth.c:sqlite3_set_authorizer()` はcallbackとuserdataを設定し、`sqlite3ExpirePreparedStatements(db,1)` を呼ぶ。`src/vdbeaux.c` はこの引数1を**advisory expiration**と説明する。再開始前にreprepareすべき状態にするが、すでに実行中なら完了まで進むことを許す。[implementation]

さらに `src/vdbeapi.c` はready状態でexpiredを検査し、schema errorからreprepareへ進む経路を持つ。[prepare API][prepare]もv2 / v3のstatementが元SQLを保持し、schema変更時にstep内で再compileし得ることを文書化している。したがって「setterを呼んだので旧statementは常に使われ続ける」も、「setter成功で実行中queryが即座に止まる」も正しくない。

区別する三つの操作:

1. **許可表の内容だけを変更する:** callbackのuserdataなどapplication memoryの変更をSQLiteへ通知する契約は確認していない。prepare済みprogramが必ず新policyで再認可されるとは保証しない
2. **sqlite3_set_authorizerを再登録する:** 固定実装ではprepared statementをadvisoryに失効させる。再開始と実行中継続の境界を区別する
3. **実行中処理を取消し・完了させ、statementを破棄して再prepareする:** applicationが別に所有する切替手順。setterの完了だけでこれを代用しない

**独自設計案:** policy世代ごとに接続とcacheを分けるか、切替時は新規受付を止め、実行中処理の終了方針を決め、旧statementを解放してから新policyでprepareする。厳密な権限剥奪が必要なら、既存処理を継続させてよいかも要件に含める。poolから論理handleを返すだけで、物理接続のcallbackやstatement cacheが消えるとは扱わない。

## 6. DEFAULTの認可とSQL関数の安全な登録は別の防御

[function flags][flags]の `SQLITE_DIRECTONLY` は関数をtop-level SQLに限定し、DEFAULTを含むschema内利用を禁止する。`SQLITE_INNOCUOUS` は副作用や情報漏えいのない性質に関する宣言であり、同じ入力で同じ結果になる `SQLITE_DETERMINISTIC` と同じではない。

[application-defined functions guide][functions]は各接続を開いた直後に `trusted_schema` を無効化する方法と、危険な副作用や情報漏えいを持つ関数へのDIRECTONLYを案内する。trusted schema無効時のschema内関数制限にはINNOCUOUSの例外があるため、「OFFなら自作関数はすべて必ず禁止」という単純化もしない。[flags]

**独自の判断:** defaultが拒否されたとき、INNOCUOUSを互換性回復用の逃げ道として付けない。schemaから呼んでよい安全な計算なのか、top-levelだけに限定すべき機能なのかを先に決める。authorizerは操作の許否、function flagは利用contextと関数の性質、既存のresource limitは計算資源の別の問題を扱う。

## 7. 導入前の境界試験と未確認事項

以下は**未実行の試験案**。各試験はSQLite版・build・binding・statement cache設定を固定し、callback記録とprepare / stepの返値を分けて確認する。

| 試験 | 確認する境界 |
|---|---|
| 許可 / 拒否関数を含むDEFAULT付きCREATE TABLE | 3.54の追加チェックがDDLのcompileへ反映されるか |
| 既存schemaを開き、列を省略したINSERTをprepareする | schema読込み成功とDEFAULT利用許可を取り違えないか |
| 同一関数にIGNOREを返す通常SELECTとDEFAULT経路 | NULL置換とerrorの差がdriverでも保たれるか |
| 許可済みstatementをcacheし、userdataのpolicyだけを変更 | cache hitを新policyによる認可済みと誤認しないか |
| setter再登録の前後で未開始statementを実行・再開始 | reprepareの失敗がprepare済みhandleからも伝わるか |
| 複数rowを返す実行中statementの途中でpolicyを変更 | setterを即時取消しや既送出結果の回収と扱わないか |
| 別接続からschema変更後に既存statementをstepする | 正しいcallbackを保持し、再compile失敗を処理できるか |
| 関数名のcase・NULL context・poolの別物理接続 | callbackの識別と初期化のscopeを取りこぼさないか |
| DIRECTONLY / trusted_schema / INNOCUOUSを別々に設定 | 認可とschema内使用可否を同じflagで代用しないか |

本調査では3.54.1のbinaryをbuild / 実行せず、upstream testも実行していない。比較した `test/auth2.test` は通常SELECTのIGNOREを確認する既存testで、DEFAULT変更の専用runtime検証ではない。旧版との差の実測、各driverのerror変換・cache破棄、transaction rollback、実行中取消し、virtual table固有の処理は未検証である。検索evalは資料を発見できることだけを検査する。

## 出典・日付・再利用条件

- UTC取得日は2026-10-24。releaseの公開日は3.54.0が2026-10-09、3.54.1が2026-10-16。C APIページの個別公開日は確認できないため取得日と混同しない。application guideの更新表示は2026-04-21、copyright pageは2026-01-12
- release_notesのTTL30日に合わせ再確認期限は2026-11-23。再確認時はrelease、callback契約、実装とcacheの境界を読み直し、日付だけを延長しない
- 固定repositoryで読んだ主なファイルは `src/build.c`、`src/insert.c`、`src/resolve.c`、`src/auth.c`、`src/vdbeaux.c`、`src/vdbeapi.c`、`test/auth2.test`、`VERSION`、`manifest.uuid`、`LICENSE.md`。GitHub connectorで固定commitの本文を取得した。ソースの観察を全将来版のAPI契約へ広げない
- [SQLite copyright][license]はcode / documentationのpublic-domain dedicationを説明し、固定 `LICENSE.md` は `src/` と `test/` を含める。build scriptすべてのlicenseを一括判定したものではない。実装コードは転載せず、独自の要約・設計案・試験案を記録し、moduleへ昇格しない

[release]: https://www.sqlite.org/releaselog/3_54_0.html
[patch]: https://www.sqlite.org/releaselog/3_54_1.html
[authorizer]: https://www.sqlite.org/c3ref/set_authorizer.html
[actions]: https://www.sqlite.org/c3ref/c_alter_table.html
[flags]: https://www.sqlite.org/c3ref/c_deterministic.html
[functions]: https://www.sqlite.org/appfunc.html
[prepare]: https://www.sqlite.org/c3ref/prepare.html
[implementation]: https://github.com/sqlite/sqlite/tree/57520227c95396ce39ba6ecaac8d475b78dd3964
[license]: https://www.sqlite.org/copyright.html

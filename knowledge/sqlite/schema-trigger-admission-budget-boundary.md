---
{
  "id": "sqlite-schema-trigger-admission-budget-boundary",
  "title": "SQLite 3.54 の schema・trigger 上限: 接続単位の受付予算と実行量の境界",
  "kind": "knowledge",
  "technology": "sqlite",
  "version": "SQLite 3.54.0 introduced SQLITE_LIMIT_SCHEMA / SQLITE_LIMIT_TRIGGER_STEPS on 2026-10-09; static implementation review pinned to 3.54.1 (2026-10-16), Git 57520227c95396ce39ba6ecaac8d475b78dd3964; runtime not tested",
  "tags": [
    "research-domain:data",
    "sqlite3_limit",
    "SQLITE_LIMIT_SCHEMA",
    "SQLITE_LIMIT_TRIGGER_STEPS",
    "schema",
    "trigger",
    "admission-budget",
    "connection",
    "prepare",
    "resource-limits"
  ],
  "sources": [
    {
      "id": "sqlite-354-limits-release-20261023",
      "url": "https://www.sqlite.org/releaselog/3_54_0.html",
      "type": "release_notes"
    },
    {
      "id": "sqlite-3541-limits-patch-context-20261023",
      "url": "https://www.sqlite.org/releaselog/3_54_1.html",
      "type": "release_notes"
    },
    {
      "id": "sqlite-limits-category-contract-20261023",
      "url": "https://www.sqlite.org/c3ref/c_limit_attached.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-limit-setter-contract-20261023",
      "url": "https://www.sqlite.org/c3ref/limit.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-schema-limit-sizing-20261023",
      "url": "https://www.sqlite.org/limits.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-limits-progress-contract-20261023",
      "url": "https://www.sqlite.org/c3ref/progress_handler.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-3541-limits-implementation-20261023",
      "url": "https://github.com/sqlite/sqlite/tree/57520227c95396ce39ba6ecaac8d475b78dd3964",
      "type": "github_repository_analysis"
    },
    {
      "id": "sqlite-limits-public-domain-20261023",
      "url": "https://www.sqlite.org/copyright.html",
      "type": "official_docs"
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

# SQLite 3.54 の schema・trigger 上限: 接続単位の受付予算と実行量の境界

## 問いと採用判断

外部から受け取ったSQLite databaseや生成SQLを処理するとき、schema object数とtrigger本文の長さを制限すれば、読込memory・実行時間・全接続の総資源量まで制限したことになるか。

本稿の独自判断は、3.54で追加された二つのlimitを**構造の受付予算**として導入し、設定値のreadback、適用する物理接続、schemaを解析する時点を一組で管理すること。制限の受付成功と、全既存構造の検査完了、queryの実行量制限は別である。[制約migration](alter-constraint-validation-null-boundary.md)の既存行検査や[WALの競合待ち](wal-busy-timeout.md)とは重複しない。

## 1. 変更点と版を固定する

[3.54.0 release notes][release]は2026-10-09に `SQLITE_LIMIT_SCHEMA` と `SQLITE_LIMIT_TRIGGER_STEPS` を `sqlite3_limit()` へ追加したと記載する。3.53の機能として扱わない。

本稿ではその公式契約に加え、[3.54.1][patch]（2026-10-16）の実装を固定commitで静的に確認した。3.54.1は32-bit buildに対するheap-buffer overflow修正などを記載するが、これは新しいlimitを設定すれば旧版の不具合を防げるという説明ではない。64-bitがその特定修正の対象外という記載も、全不具合の影響を受けない保証へ広げない。対象binaryの更新判断とlimit policyの導入を分ける。

## 2. 同じ「limit」でも数えている単位が違う

[Run-Time Limit Categories][categories]の公式契約:

| 指定 | 制限対象 | 誤って読み替えないもの |
|---|---|---|
| `SQLITE_LIMIT_SCHEMA` | schemaに定義されたtable・index・trigger・viewのobject数 | tableの行数、接続全体のbyte数 |
| `SQLITE_LIMIT_TRIGGER_STEPS` | 一つのtrigger本文に含まれるSQL statement数 | trigger発火回数、変更行数、処理時間 |
| `SQLITE_LIMIT_TRIGGER_DEPTH` | triggerの再帰および別triggerの入れ子の深さ | 一つのtrigger本文のstatement数 |
| `SQLITE_LIMIT_VDBE_OP` | 一つのprepared statementを実装するVM programの命令数 | 実行中に繰り返された命令の累積回数 |

表の右列は制限対象から導いた独自の注意点である。`VDBE_OP` はprepare時に必要な命令領域が上限を超えると `SQLITE_NOMEM` になるという契約で、実行回数をN命令で打ち切る仕組みではない。[categories]

したがって、SQL文が1個だけのtriggerでも大量行を更新する処理はあり得る。小さいprogramでもloopの反復は多くなり得る。「TRIGGER_STEPS=1なので安価」「VDBE_OPを下げたのでtimeoutは不要」は保証されない、というのが本稿の設計上の帰結である。

## 3. setterの戻り値は設定成功コードではない

[sqlite3_limit API][limit]は**接続単位**で設定する。`newVal` が負なら変更せず、変更の有無にかかわらず戻り値は変更前の値になる。compile-timeのhard upper boundを超える要求は、エラーを返すのではなく上限へ切り詰める。

独自の初期化手順案:

1. 実際にSQLを実行する物理接続を開き、その接続のSQLite版・build条件を記録する
2. 初回のschema解析より前に、二つのcategoryそれぞれの旧値を `newVal=-1` で取得する
3. 非負のpolicy値を設定し、もう一度 `newVal=-1` でreadbackする。setter返値との大小比較を成功判定に使わない
4. 実効値がpolicyを満たすかを確認する。要求より小さくclampされた場合も、業務上必要なschemaやtriggerを処理できるか別途判定する
5. 版・category・要求値・旧値・実効値と初期化結果を記録し、必要な制限を確認できなければ外部入力を処理しない

「0なら無制限」という共通規約はこのAPIにはない。負値は無制限化ではなく照会である。具体的に許すobject数やstep数はアプリのschemaと負荷測定から決め、公式の推奨値を装って固定値を配らない。

固定3.54.1の `src/main.c:sqlite3_limit()` は無効なcategory IDに `-1` を返す。これを `SQLITE_OK` 型の成功コードとして処理しない。新しいheaderやwrapperがcategory名を持つことだけでは、配備先の古いruntimeがその制限を実装している証拠にならない。古い版へ数値IDを直接送る代替策ではなく、実際のlinked runtimeとbindingの対応を検証する。[implementation]

## 4. schema capは一つのschemaごとの計数であり、全接続のquotaではない

[Limits in SQLite][sizing]はschemaの走査・解析とmemory上の構造保持を説明し、初期化時間と初期memoryがschemaの大きさに比例するとする。schema capの既定は10 millionと記載する。これは通常のschemaで積極的に使い切る目標ではない。

以下は**3.54.1固定実装の観察**で、今後の全版に対する追加API契約ではない。

- `src/sqliteLimit.h` はsingle database schemaの対象として、`sqlite_schema` 自身、virtual table、名前付きindex、UNIQUE / PRIMARY KEY制約が実際に作る自動index、trigger、viewを挙げる。すべてのPRIMARY KEYが別indexを一つ作るという意味ではない
- `src/prepare.c:sqlite3InitCallback()` は対象database `iDb` のschemaのtable・index・trigger用hashの件数を加え、接続の `SQLITE_LIMIT_SCHEMA` を超えたらエラーにする。他のattached databaseの件数を合算していない
- `sqlite3InitOne()` は `sqlite_schema` / `sqlite_temp_schema` の内部表現もこのcallbackで作る。従って、application table数だけ、または `SELECT count(*) FROM sqlite_schema` の返値だけをそのまま実装上の全object数と同一視しない
- 同callbackはschema初期化だけでなく `OP_ParseSchema` からも使われる。検査はobjectの定義を解析して内部構造を作った後に位置する。schema capは「次のobjectを解析する前にmemory確保をゼロに抑える」契約ではない

独自の帰結: それぞれ上限内の複数schemaをATTACHできる場合、object数の総和は一つのschema capを超え得る。`SQLITE_LIMIT_ATTACHED`、接続数、file size、process memoryの予算を別々に定義する。新しいschema capだけをtenant全体のquotaやdatabase fileのbyte上限として扱わない。[categories][limit][implementation]

## 5. trigger capの検査位置と既存構造を分ける

固定3.54.1の `src/trigger.c:sqlite3FinishTrigger()` は、trigger actionを解析した後、stepのlinked listを数えて `SQLITE_LIMIT_TRIGGER_STEPS` と比較する。既定のcompile-time `SQLITE_MAX_TRIGGER_STEPS` は65,000である。これは実行中のstep消費カウンターではなく、作成・解析するtriggerの形に対する検査である。[implementation]

独自の注意点: すべてのactionを解析した後の検査なので、trigger本文を読むための一時的なmemoryやparseコストまでこの数値で厳密に抑えるとは言えない。SQL text長、parser / expressionの深さ、memoryなど他の制限を残す。trigger step数と `SQLITE_LIMIT_TRIGGER_DEPTH` の両方を評価し、一方で他方を代用しない。[categories][implementation]

さらに `src/main.c:sqlite3_limit()` は設定配列を更新するだけで、既存schemaやprepared statementの全再検査・縮小処理はしない。関数のコメントも、低いlimitは既存構造を縮小せず、新たな構造の形成を制限すると説明する。[implementation]

従って「schemaを読み込んだ後にlimitを下げ、readbackできた」ことは、既存schemaが新policy内に収まる証拠ではない。既存triggerがいつ再解析されるか、すでにprepare済みのstatementが新policyの下でどう扱われるかは、利用するbuild / binding / cacheを含めた確認対象とする。readbackを全既存triggerの即時失効通知と見なさない。

## 6. pool・実行中断・認可の責務を分ける

以下は本稿の**独自設計案**であり、SQLiteが特定のpool実装を保証するものではない。

- poolの論理handleではなく、全物理接続の作成時にpolicyを適用する。一接続のreadbackからpool全体の設定完了を結論しない。ORMが先にschema introspectionを実行するなら、初回解析より前に設定できる初期化hookや接続経路を確認する
- 内部管理用と外部入力用の接続を分ける。緩いpolicyでschema / statement cacheを作ってから同じ接続を外部入力向けに貸す運用を避ける
- migrationで一時的に上限を上げる必要があるなら、対象接続、処理範囲、後処理を明示する。「finallyで元の数値へ戻す」だけでは既存構造の再検証にならない
- エラーを自動的に上限緩和してretryする分岐を置かない。正当な大きいschemaを別経路で扱う判断と、攻撃的な入力を拒否する判断を分ける

[progress handler][progress]は長いprepare / step処理中にcallbackを呼べる。間隔NはVM命令数の概数で、非zeroを返すと処理を中断する。接続に一つだけで、後の登録は前を置換する。3.41.0以降は複雑なqueryの解析・code生成中に直接呼ばれる場合もある。

独自の帰結: deadline判定と中断を別途設計し、schema / trigger capの存在をwall-clock timeout保証へ読み替えない。callback間隔は厳密な時間上限ではない。driverやmiddlewareがcallbackを上書きする可能性も管理する。read / writeの認可はさらに別であり、limit API自身もauthorizerや `max_page_count` の併用を別の役割として案内する。[limit][progress]

## 7. 本番前の境界試験と未確認事項

次は**未実行のruntime試験案**。SQLite binaryのbuild・実行、bindingの統合試験、upstream test suite実行を行った結果ではない。

| 試験 | 確認する観点 |
|---|---|
| 同categoryへ設定し、旧値とreadbackを比較 | setter返値を新値やbooleanとして扱わず、clamp後の実効値を記録する |
| connection Aだけ設定し、Bを照会 | 接続単位のscopeをpool初期化で取りこぼさない |
| 通常table、UNIQUEの自動index、view、triggerを増やす | application table数だけでschema capを見積もらない |
| mainとattached schemaをそれぞれ上限近くにする | per-schema上限と全接続の総object数を区別する |
| trigger本文N文とN+1文をprepareする | step数境界を検査し、エラーがprepare / stepのどちらから伝わるかbinding上で記録する |
| 長いtrigger本文と短いが大量行を更新するtriggerを別々に試す | 構造の受付予算と実行量を別々に測定する |
| schema / statement読込前と読込後で同じ低いlimitを設定 | readbackの一致を既存構造の再検査完了と誤認しない |
| schema reload、pool再接続、migration後の再貸出 | 初期化順序とcacheの寿命を検証する |
| progress callbackとdriver cancellationを併用 | callback置換・prepare中の取消し・エラー後のtransaction状態を調べる |

共有cache、virtual table module固有の資源消費、schema破損時の回復、exact error codeのbinding変換、DDL失敗時のrollback境界、各driverのstatement cache実装は未検証。任意のSQL functionの実行時間やmemoryまでこの二つのlimitで制御できるとは主張しない。検索evalは本稿を発見できることだけを検査する。

## 出典・provenance

- UTC取得日は2026-10-23。3.54.0 / 3.54.1の公開日はそれぞれ2026-10-09 / 2026-10-16でrelease pageを確認した。release_notesのTTL30日に合わせ、再確認期限は2026-11-22
- C API / limits guideはrolling documentationであり、個別patchの実行証拠ではない。limits guideのfooterは2026-07-10だが本文は3.54.0の2026-10-09導入を記載するため、footerを公開日として採用しない
- tag `version-3.54.1` はGit commit `57520227c95396ce39ba6ecaac8d475b78dd3964` に解決した。`VERSION` の3.54.1と `manifest.uuid` の `555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76` はreleaseのFossil source IDと照合した。Git SHAとFossil IDを取り違えない
- 固定repositoryの確認対象は `src/main.c`、`src/prepare.c`、`src/trigger.c`、`src/sqliteLimit.h`、`test/sqllimits1.test`、`VERSION`、`manifest.uuid`、`LICENSE.md`。upstream `sqllimits1-1.13` / `1.14` は二つのcategoryのclampと照会を検査するが、本調査でそのtestを実行したわけではない
- native webで公式文書を開いた。GitHub blob URLのweb取得はcache missだったため、その成功を装わず、同一固定commitのfile内容をGitHub connectorで取得した
- [公式copyright][license]と固定 `LICENSE.md` は今回参照したSQLite文書、`src/`、`test/` をpublic domainとする。build script全体への一括のlicense判定ではない。実装コードは転載せず、独自の要約と試験案のみを追加し、moduleへの昇格は行わない

[release]: https://www.sqlite.org/releaselog/3_54_0.html
[patch]: https://www.sqlite.org/releaselog/3_54_1.html
[categories]: https://www.sqlite.org/c3ref/c_limit_attached.html
[limit]: https://www.sqlite.org/c3ref/limit.html
[sizing]: https://www.sqlite.org/limits.html
[progress]: https://www.sqlite.org/c3ref/progress_handler.html
[implementation]: https://github.com/sqlite/sqlite/tree/57520227c95396ce39ba6ecaac8d475b78dd3964
[license]: https://www.sqlite.org/copyright.html

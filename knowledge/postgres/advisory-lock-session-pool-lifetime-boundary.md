---
{
  "id": "postgres-advisory-lock-session-pool-lifetime-boundary",
  "title": "PostgreSQL advisory lock: session所有権・再取得回数・pool返却の寿命境界",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18 versioned manual; PgBouncer 1.26.0 bundled configuration docs at 7d38761c8f6c757238fde9f942cf9fe0cd272ae3 (released 2026-09-23); rolling feature map verified 2026-10-04 UTC; runtime not tested",
  "tags": [
    "research-domain:data",
    "postgres",
    "advisory-lock",
    "session",
    "transaction",
    "pooling",
    "rollback",
    "reentrant",
    "pg_locks",
    "LIMIT",
    "snapshot"
  ],
  "sources": [
    {
      "id": "postgres-advisory-locking-18-20261004",
      "url": "https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS",
      "type": "official_docs"
    },
    {
      "id": "postgres-advisory-functions-18-20261004",
      "url": "https://www.postgresql.org/docs/18/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS",
      "type": "official_docs"
    },
    {
      "id": "postgres-advisory-observation-18-20261004",
      "url": "https://www.postgresql.org/docs/18/view-pg-locks.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-advisory-autocommit-18-20261004",
      "url": "https://www.postgresql.org/docs/18/sql-begin.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-advisory-snapshot-18-20261004",
      "url": "https://www.postgresql.org/docs/18/transaction-iso.html",
      "type": "official_docs"
    },
    {
      "id": "pgbouncer-advisory-feature-map-20261004",
      "url": "https://www.pgbouncer.org/features.html",
      "type": "official_docs"
    },
    {
      "id": "pgbouncer-advisory-pool-config-1260-20261004",
      "url": "https://github.com/pgbouncer/pgbouncer/blob/7d38761c8f6c757238fde9f942cf9fe0cd272ae3/doc/config.md",
      "type": "github_repository_analysis"
    },
    {
      "id": "pgbouncer-1260-publication-advisory-scope-20261004",
      "url": "https://www.pgbouncer.org/2026/09/pgbouncer-1-26-0",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# PostgreSQL advisory lock: session所有権・再取得回数・pool返却の寿命境界

## 問いと採用判断

「lock関数が成功し、finallyでunlockした」だけで、二重実行やlock残留を防げるか。advisory lockの所有者は業務job名やアプリのrequestではない。保護区間とPostgreSQLのsession / transactionが一致するかを先に決める。

本稿の独自判断は、単一DB transaction内で完結する処理ではtransaction-levelを第一候補にし、複数transactionをまたぐsession-levelは同じserver sessionの専有と解放責務を保証できる場合に限る、というもの。pool越しの接続handle、lock取得成功、業務commit完了は別の確認項目である。既存の[分離レベルと再試行](transaction-isolation-retry.md)、[CONCURRENTLY migration](zero-downtime-migration-concurrently.md)、[Go接続pool](../go/database-sql.md)の一般論を繰り返さず、advisory lockの寿命と論理的な仕事の対応を扱う。

新機能の紹介ではない。PostgreSQL 18の版付き公式文書と、PgBouncer 1.26.0の固定commit同梱文書を再確認したdurableな実務上の空白であり、導入版や新たな修正を推測しない。

## 1. 所有権と終了条件の公式契約

[Explicit Locking §13.3.5][locking]と[Advisory Lock Functions §9.28.10][functions]で確認した契約を整理する。

| APIの種類 | 取得と終了条件 | 呼び出し側で区別すること |
|---|---|---|
| session-level: `pg_advisory_lock` / `pg_try_advisory_lock` | session終了または対応する明示unlockまで保持 | COMMIT / ROLLBACKだけでは解放しない |
| transaction-level: `pg_advisory_xact_lock` / `pg_try_advisory_xact_lock` | transaction終了時に自動解放、明示unlock APIなし | 保護する仕事を同じtransactionに置く |
| shared版: 名前末尾の `_shared` | 同じkeyのshared同士は競合せず、exclusiveと競合 | 複数writerの排他にsharedだけを使わない |
| try版 | 直ちに取得できればtrue、できなければ待機せずfalse | falseは「保護されていない」分岐 |

session lockの取得は、そのtransactionを後からROLLBACKしても残る。逆に明示unlockは、その後のROLLBACKで取り消されない。同じsessionが同じexclusive lockを再取得すると他sessionの待機中でも成功し、session-levelの取得回数は積み重なる。同じ識別子のsession-levelとtransaction-levelは別session間で互いに競合する。[locking][functions]

advisory lockは利用者の協調に依存し、同じ表への通常のSQLを自動で禁止しない。全writerに共通の規約が必要である。[locking]

## 2. 再入可能は冪等ではない

以下は公式契約から導く独自の時系列例で、runtimeで観測したログではない。AとBは同一databaseへの別server sessionで、同じexclusive keyを使う。

| 順序 | 操作 | 契約から期待する結果 |
|---|---|---|
| 1 | Aがsession-level lockを取得 | Aが所有する |
| 2 | Aが「まだ取れているか」のつもりでtry-lockを再実行 | true、取得回数は2になる |
| 3 | Aがunlockを1回実行 | trueでも残り1回分を保持する |
| 4 | Bが同じkeyをtry-lock | falseのまま |
| 5 | Aがもう1回unlock | 他に競合保有がなければBが取得可能になる |

`pg_advisory_unlock` のtrueは、その1回の解放要求が成功したという意味として扱い、key全体が無保有になった証拠にしない。falseならそのsessionが該当session-level lockを持たず、serverはwarningも出す。`pg_advisory_unlock_all()` は呼出元sessionの全session-level lockが対象であり、別sessionやtransaction-levelのlockを消す操作ではない。[functions]

独自の設計案:

- 成功した取得と解放の責任を同じ所有者に集約し、try-lockを状態照会やheartbeatに使わない
- 通信断で取得結果を受け取れなかった場合、sessionが残っている可能性と既に取得した可能性を捨てない。同じsessionへの盲目的なretryは取得回数を増やし得るため、業務処理を始める前に接続の継続可否・所有状態を扱う復旧手順へ移す
- 内側のhelperが「念のためunlock_all」を呼ぶと、外側の処理が所有していた別keyも解放し得る。同じsessionを共有する部品間でcleanup範囲を暗黙に広げない
- 同じserver session上の二つの論理jobには排他が成立するとは限らない。再入可能性をアプリ内mutexの代替にせず、sessionの専有も管理する

## 3. pool返却はserver session終了とは限らない

[PgBouncerのfeature map][pool-features]はsession-level advisory lockをsession poolingでは対応、transaction poolingでは非対応としている。[1.26.0同梱config][pool-config]の `pool_mode` では、session modeはclient切断時、transaction modeはtransaction終了時にserver connectionをpoolへ戻す。statement modeは複数statementにまたがるtransactionを許可しない。

次はこのpooling modelとPostgreSQLの契約を組み合わせた**失敗例の推論**である。各transactionで別backendが必ず割り当てられる、という意味ではない。

1. client Aのtransactionをserver S1が担当し、Aがsession-level lockを取得してCOMMITする
2. S1はlockを保持したまま再利用可能になる。Aは後続transactionでS2へ割り当てられ、unlockがS1へ届く保証を失う
3. client BがS1へ割り当てられて同じkeyをtry-lockすると、serverからは同一sessionの再取得として成功し得る。Aの論理jobとの排他を証明できない

アプリ側で同じclient connectionを確保するだけでは、transaction pool内のserver session固定を保証できない。直接接続、session pooling、transaction pooling、アプリ側poolを分けて経路を点検する。session poolingを採用しても、アプリ内で未解放の接続を別jobへ返せば所有者の混線は残る。poolerの対応表はアプリ側のcleanupまで証明しない。

### reset設定を寿命の代用品にしない

固定1.26.0文書では `server_reset_query` の既定は `DISCARD ALL` だが、transaction poolingでは通常実行しない。`server_reset_query_always` の既定は0で、有効にして全pool modeでresetしても、transactionを越えてsession stateを保持する契約にはならない。公式文書も、毎transaction後にstateを失う挙動になると説明する。[pool-config]

独自判断: これらの設定変更でsession lockによる複数transactionの保護を修復しようとせず、保護区間の設計を変更するか、必要なsession専有を実現できる接続経路を選ぶ。特定driverがpool返却時にresetするか、close APIが物理切断か論理返却かは本稿では未検証である。

## 4. transaction-levelでもautocommitとsnapshotを確認する

[BEGIN][begin]の既定では、明示transactionがなければ各statementが独立したtransactionとなる。したがってautocommitで `SELECT pg_advisory_xact_lock(91007)` を実行し、その後の別statementで更新する案は、更新時までlockを保持しない。[functions][begin]

以下は実装前に満たすべき**独自の手順案**であり、実行済みSQL例ではない。

1. 明示transactionを開始し、そのtransaction用の接続・APIに処理を束縛する
2. keyを確定し、`pg_try_advisory_xact_lock` または待機型APIを呼ぶ
3. tryのfalseは競合として扱い、保護対象のread/writeを実行せずtransactionを閉じる。SQL batchへ後続更新を無条件に連結しない
4. 成功後に保護対象を読み直し、同じtransaction内で変更し、COMMIT / ROLLBACKの結果を確認する
5. commit結果不明の場合は業務上の完了記録で照合する。lockが解放されたことから成功commitを逆算しない

### lock取得とsnapshot更新は別

[Transaction Isolation][snapshot]では、READ COMMITTEDの通常のSELECTはstatement開始時点のsnapshot、REPEATABLE READは最初の非transaction制御statement開始時点のsnapshotを使う。

独自の帰結: advisory lock取得と通常の表readを一つのSELECTに詰めると、lock待機中に他transactionがCOMMITしても、そのSELECTのsnapshotが取得完了時へ更新されると期待できない。READ COMMITTEDで「先行所有者のcommit後の値」が必要なら、lock取得成功後の別statementでreadし、lockを保持している同じtransaction内で判断する。REPEATABLE READでは別statementに分けても既に確立したsnapshotを更新しない。分離レベルを上げればこの手順をそのまま強化できる、とは扱わない。

これはadvisory lockを取得するすべてのアプリにREAD COMMITTEDを強制する勧告ではない。整合性要件、通常のrow lockや制約、再試行規約を含めた設計が必要で、一般的な分離レベルの選択は既存文書に譲る。

## 5. key集合・取得順序・観測を別々に管理する

[functions]では、64-bitの単一bigint keyと32-bit integer二つのkeyは重ならない別空間である。[pg_locks][locks-view]ではadvisory keyはdatabaseローカルであり、bigintは `classid` / `objid` に上位/下位を分け `objsubid=1`、二つのintegerは `objsubid=2` で表現される。数値が似ている、または同じclusterを使うだけでは同じlockを競合させたことにならない。

独自案として、keyのdatabase、型、用途namespace、identifierの対応を全呼出元で共有する。hashで縮約する場合の衝突や型変換を排他保証から除外せず、個々のORMが同じoverloadを呼ぶか確認する。

[locking]はlock関数を含むqueryではLIMITより先に関数が評価され、返却されない行のlockが残る危険を示す。LIMIT付きsubqueryで候補を限定してから外側でlockする形を公式に例示している。ただし、返却行数の上限、lock取得順序、取得後に業務条件がまだ成立することは別々の設計事項である。

独自の実装案は、候補keyを副作用のないqueryで上限付き抽出し、重複を除去して共通の順序に並べ、同じ所有者が一件ずつ取得する方法。取得後に対象条件を再確認し、途中で競合・エラーになった場合の部分取得cleanupを決める。選択と取得の間に行が変化し得るため、この分離をsnapshot固定やjob claimの原子性とは呼ばない。大量keyの無制限取得は通常lockと共有するlock用メモリにも影響する。[locking]

### pg_locksは取得権限を与えない

`pg_locks` は対象・要求mode・processごとの行であり、`pid` と `granted` から保有と待機を観測できる。[locks-view] この形式から、行数を再入取得回数と読み替えてunlock回数を自動決定しない、という運用判断になる。try-lockに置き換えた観測も状態を変更する。

独自の監視案: database・key形式・pid・保有/待機をアプリのjob記録と結び付ける。ただし「観測時点で行なし→安全に開始」のcheck-then-actを作らず、実際のlock取得で競合を判定する。lock行の消失は業務成功の証拠ではなく、session終了やROLLBACKでも起こり得る。

## 6. 本番前に試す境界と未確認事項

次は**未実行のruntime試験案**。PostgreSQL / PgBouncerのserver、driver、ネットワーク障害を起動・注入して確認した結果ではない。

- session取得後のROLLBACKでも別sessionのtryがfalse、session unlock後のROLLBACKでも解放が取り消されないこと
- 同一sessionで二回取得し、一回unlockがtrueでも別sessionがまだ取得できないこと
- autocommitのxact lockと、明示transaction中のxact lockで、後続更新までの保護範囲が異なること
- tryのfalse経路で業務更新が一切走らず、成功経路もCOMMIT結果まで照合できること
- 同じclient connectionを保持したtransaction poolingとsession poolingを別々に試し、backend再割当とアプリpool返却時のcleanupを確認すること
- lock待機を挟むREAD COMMITTEDの同一SELECT / 後続SELECTと、REPEATABLE READの既存snapshotを区別すること
- bigint形式とinteger二つ形式、別databaseを混ぜた否定例、取得途中の失敗、結果応答消失を試すこと

savepoint/例外blockとの詳細な相互作用、2PC prepared transaction、lock upgrade、公平性、deadlock victim選択、timeoutごとのエラー分類、client切断をserverが検出するまでの時間、failoverを越えた保護、外部serviceへの副作用のfencing、性能値は未検証。session終了によるcleanupを、ネットワーク断の瞬間に業務workerが停止した保証へ拡張しない。検索evalは知識の発見性を検査し、DB適合試験の代わりではない。

## 出典・版・provenance

- UTC取得日はすべて2026-10-04。PostgreSQLはURL `/18/` のmanualを使用した。該当ページには個別更新日やpatch版表示がないため、18.6等の特定patchで実行したとは扱わない
- PgBouncer 1.26.0は[公式告知][pool-release]で2026-09-23公開を確認。tag `pgbouncer_1_26_0` の指すcommitは `7d38761c8f6c757238fde9f942cf9fe0cd272ae3`。repositoryの確認範囲は同commitの `doc/config.md` と `COPYRIGHT` で、pooling実装の監査ではない。新たなadvisory lock機能が1.26.0で導入されたという主張もしない
- PgBouncer feature map / 公開configはrolling pageとしてnative webで開き、対象pooling/reset契約を固定同梱configと照合した。その他の現行説明と1.26.0との完全一致は検証していない
- PostgreSQLの[Legal Notice](https://www.postgresql.org/docs/18/legalnotice.html)と[License](https://www.postgresql.org/about/licence/)はdocumentationを含むPostgreSQL Licenseを示す。PgBouncerの[固定COPYRIGHT](https://github.com/pgbouncer/pgbouncer/blob/7d38761c8f6c757238fde9f942cf9fe0cd272ae3/COPYRIGHT)でISCを確認した。Web feature map・release告知の個別licenseはunknown。`license.html` のWeb取得は失敗しており、成功した根拠には含めない
- 原文・実装コードを転載せず、出典付きの独自要約・推論・運用案を記載した。moduleへの昇格なし。release_notesのTTL30日に合わせ再確認期限は2026-11-03

[locking]: https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS
[functions]: https://www.postgresql.org/docs/18/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS
[locks-view]: https://www.postgresql.org/docs/18/view-pg-locks.html
[begin]: https://www.postgresql.org/docs/18/sql-begin.html
[snapshot]: https://www.postgresql.org/docs/18/transaction-iso.html
[pool-features]: https://www.pgbouncer.org/features.html
[pool-config]: https://github.com/pgbouncer/pgbouncer/blob/7d38761c8f6c757238fde9f942cf9fe0cd272ae3/doc/config.md
[pool-release]: https://www.pgbouncer.org/2026/09/pgbouncer-1-26-0

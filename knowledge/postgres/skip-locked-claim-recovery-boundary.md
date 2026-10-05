---
{
  "id": "postgres-skip-locked-claim-recovery-boundary",
  "title": "PostgreSQL SKIP LOCKED queue: 行の取得・lease回収・旧worker更新の境界",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18 versioned manuals, verified 2026-10-05 UTC; no patch-version or recent-feature claim; queue protocol and SQL are original unexecuted design examples",
  "tags": [
    "research-domain:data",
    "postgres",
    "skip-locked",
    "job-queue",
    "row-lock",
    "claim",
    "lease",
    "recovery",
    "generation",
    "read-committed",
    "starvation"
  ],
  "sources": [
    {
      "id": "postgres-queue-select-18-20261005",
      "url": "https://www.postgresql.org/docs/18/sql-select.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-update-18-20261005",
      "url": "https://www.postgresql.org/docs/18/sql-update.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-row-locks-18-20261005",
      "url": "https://www.postgresql.org/docs/18/explicit-locking.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-read-committed-18-20261005",
      "url": "https://www.postgresql.org/docs/18/transaction-iso.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-clocks-18-20261005",
      "url": "https://www.postgresql.org/docs/18/functions-datetime.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-timeouts-18-20261005",
      "url": "https://www.postgresql.org/docs/18/runtime-config-client.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-begin-18-20261005",
      "url": "https://www.postgresql.org/docs/18/sql-begin.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-commit-18-20261005",
      "url": "https://www.postgresql.org/docs/18/sql-commit.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-partial-index-18-20261005",
      "url": "https://www.postgresql.org/docs/18/indexes-partial.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-vacuum-18-20261005",
      "url": "https://www.postgresql.org/docs/18/routine-vacuuming.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-queue-license-18-20261005",
      "url": "https://www.postgresql.org/docs/18/legalnotice.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2027-01-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# PostgreSQL SKIP LOCKED queue: 行の取得・lease回収・旧worker更新の境界

## 問いと採用判断

PostgreSQLの表をjob queueとして複数workerから読むとき、`FOR UPDATE SKIP LOCKED` でどこまで重複を防げるか。claimをCOMMITした後の実行、worker停止からの回収、古いworkerから遅れて届く完了更新を、一つの所有権プロトコルとして設計する。

結論は、短いtransactionで候補選択と `ready → running` をまとめ、その後は行lockではなく保存したstateと世代番号で更新権を確認すること。lease期限は回収候補になる条件であり、停止命令や外部副作用の取り消しではない。各操作が同じ規約を守ることが必要である。

これは2026年の新機能紹介ではなく、18の版指定manualで確認した未収録の実務上の空白である。[分離レベル](transaction-isolation-retry.md)、[advisory lock](advisory-lock-session-pool-lifetime-boundary.md)、[outbox](../messaging/transactional-outbox.md)とは対象が異なる。以下のSQL、世代管理、回収手順、運用基準は独自の設計案で、公式job queue実装や実行済みの検証結果ではない。

## 1. SKIP LOCKEDが与える契約

公式文書で確認した事実を、設計案と分けて整理する。

- `SKIP LOCKED` はすぐ取得できない行lockの対象を飛ばす。全件の整合した読み取りには向かないが、queue風の表を複数consumerで読む用途が明示されている。[SELECT][select]
- 対象は行lockであり、必要なtable lockは通常どおり取得する。`NOWAIT` は同じ行競合をスキップせずエラーにする別の選択肢である。[SELECT][select]
- locking clauseを外側に書くだけでは参照先の `WITH` queryまで及ばない。CTE内でlockしたいならそのCTEに書く。joinでは `OF q` などで対象表を限定できる。[SELECT][select]
- `LIMIT` は必要な返却行を得るとlock取得を止めるが、`OFFSET` で読み飛ばす行もlockされる。`LIMIT NULL` は無制限である。[SELECT][select]
- `FOR UPDATE` で取った行lockはtransaction終了または該当savepointへのrollbackで解放される。通常のSELECTを止めるものではなく、同じ行への競合する更新・lockを制御する。[Explicit Locking][locks]
- READ COMMITTEDの更新では、先行transactionが更新してCOMMITした対象行に対して `WHERE` を再評価する。REPEATABLE READ / SERIALIZABLEでは、開始後に変更された行のlockでエラーになり得る。[Transaction Isolation][isolation] [Explicit Locking][locks]
- `UPDATE 0` 自体はSQLエラーではない。`RETURNING` は実際に更新した行の値を返すが、明示transactionのCOMMITとは別である。[UPDATE][update] [COMMIT][commit]

独自の帰結: lockはDB transaction中の競合制御、claimはCOMMIT後にも残る業務状態である。前者だけを取得してCOMMITし、その後に別のUPDATEで実行権を宣言する二段階方式には隙間がある。autocommitでSELECTとUPDATEを別々に送る場合も同じ問題になる。[BEGIN][begin]

## 2. 短いclaimを一つの原子的な更新にする

次は**実行前レビュー用の独自SQL例**。単一の通常表 `job_queue`、一意で再利用しない `id`、READ COMMITTED、claimを変更・抑止するtriggerなしを前提とする。テーブルDDLや完成したqueueライブラリではない。

- `state`: `ready` / `running` / `done` / `failed`
- `available_at`: 実行可能時刻、`priority`: 大きいほど優先
- `generation`: jobごとの非NULL bigint世代。初期0で、claimと権利取り消しで増加させる
- `lease_until`: runningの回収期限、`worker_id`: 診断用のworker識別子
- `attempts`: claimされた回数。業務副作用が実行された回数とは区別する

state・priority・available_at・generation・attemptsのNULLを禁止し、runningではlease_untilとworker_idが必須、非runningではlease情報を整理する等の不変条件をDDLと更新経路で揃える。queue行を書ける権限自体を世代tokenが認証するわけではない。

```sql
WITH candidate AS (
  SELECT id
  FROM job_queue
  WHERE state = 'ready'
    AND available_at <= statement_timestamp()
  ORDER BY priority DESC, available_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT $1
)
UPDATE job_queue AS j
SET state = 'running',
    generation = j.generation + 1,
    worker_id = $2,
    lease_until = clock_timestamp() + $3::interval,
    attempts = j.attempts + 1
FROM candidate AS c
WHERE j.id = c.id
RETURNING j.id, j.generation, j.lease_until, j.payload;
```

`$1` は非NULLかつ正で運用上限以下の整数、`$2` はworker識別子、`$3` は正の短い固定時間間隔として呼出元で検証する。NULLを「既定batch size」の意味で渡さない。generation / attemptsの桁あふれやNULL初期値を放置せず、上限に近づいたjobは明示的に停止・調査する。idを削除して同じ値・世代で再作成すると古い要求を再び受け付け得るため、再利用しない。

この例の候補は同一表の一意idなので、target一行に候補が複数joinしない。将来tenantや依存jobをjoinする拡張では候補の一意性とlock対象を再点検する。`UPDATE ... FROM` のtargetに複数のjoin行が対応すると、どれが使われるか予測できないことが公式に注意されている。[UPDATE][update]

実行側は次の順序を守る（独自推奨）。

1. batchをすぐ開始できる空き実行枠以下にする。大量claimしてローカル待機列へ置くと、処理前にleaseを消費する
2. claim SQLの結果を保存し、明示transactionならCOMMITの成功まで確認する。autocommitでもdriverのcommand完了を確認する
3. COMMIT後に外部処理へ進む。長いAPI待ち・sleep・人の判断をclaim transaction内に置かない
4. job idと取得世代を全ての更新要求に渡す。worker名だけを所有権tokenにしない
5. COMMITや結果応答が不明なら、そのclaimから処理を始めない。既知のid/世代を新しいtransactionで照合するか、回収へ委ねる

lockを保持したまま長い処理をすれば、他workerの同じ行取得は抑えられる。しかし接続占有・長期transactionの代償を持ち、外部APIの成功後にDBがrollbackする問題は残る。長時間transactionを避けるという判断と、外部副作用の冪等化は別々に必要である。[Explicit Locking][locks] [Client Defaults][timeouts]

## 3. renewal・completion・recoveryを同じ世代規約に揃える

以下は本稿が提案する保守的な規約である。実行権の更新点を、queue行の排他lockを保持して確認・変更する短いtransactionに集約する。

| 操作 | 保持する条件 | 変更と結果確認 |
|---|---|---|
| claim | `state='ready'`、時刻条件、候補行lock | generationを増やしrunningへ。COMMIT成功後の世代を利用 |
| renewal | id・`state='running'`・要求generationが一致し、取得後のDB時刻で未失効 | lease_untilを延長。更新行数とCOMMITを確認 |
| completion | renewalと同じ所有条件・期限条件 | doneへ、leaseとworker欄を整理。必要なDB内業務更新と同じtransactionで確定 |
| failure / retry | 同じ所有条件。業務上の再試行可否を判定 | readyへ戻すかfailedへ。旧世代を無効化し、再試行ならavailable_atを設定 |
| recovery | running、取得後に見直して期限切れ | generationを増やし旧workerを無効化。ready / failedへの遷移をCOMMIT |

### worker側の更新

idで `SELECT ... FOR UPDATE` し、lock取得後の行のstateとgenerationを確認する。**その後の別statement**で現在のDB時刻を取り、leaseを検証してから更新する。待機前の時刻を期限判定へ流用しない。同じtransaction内で最後のUPDATEにもid・state・generation条件を残し、更新0件や予期しない件数なら業務上の成功にしない。

renewalでは延長幅の上限と、既存期限を意図せず短縮しない規則も決める。clock補正や重複したrenewal要求がある場合でも、成功応答だけから新しい期限を推測せず保存した値を使う。

COMMIT前にlockを放すことはしない。completionでDB内の業務表も変えるなら、世代チェック、業務更新、done更新を同一transactionに置く。先に業務表だけCOMMITし、その後の条件付きdone更新が0件なら「重複なし」と判断する構成は保護にならない。

ここで期限は**lock取得後の確認時点で操作を受理する条件**であり、COMMITや外部処理が必ず期限前に終わる保証ではない。期限に近い更新を受理する余裕を設けるかは、処理時間・遅延の測定で決める。worker側は未失効を確認できない間、新しい外部副作用を始めない。既に送信済みの要求はこの停止方針だけでは取り消せない。

### 回収側の更新

独立したrecovery処理は `state='running' AND lease_until <= statement_timestamp()` を上限付き・`FOR UPDATE SKIP LOCKED` で探す。取得後に同じtransaction内で最新のstate・generation・期限を見直し、まだ失効している行だけ世代を増やしてreadyまたはfailedへ移す。最初のscan時刻から取得後までに期限が延長された場合や、DB時計が後退した場合を、無条件UPDATEで上書きしない。

復旧候補のidだけを通常SELECTで集め、後でid条件だけのUPDATEをする方式は避ける。その間にrenewalや別のrecoveryが成功したことを見落とす。別transactionに分ける必要がある場合は、観測した世代を比較し、更新時のstate・期限も条件に含め、0件を競合として扱う。単に前のSELECTが返ったという事実は権利にならない。[Transaction Isolation][isolation]

### 古いworkerが戻ってきた時系列

以下は独自の故障モデルであり、実サーバーの観測ログではない。

1. Aがjob 81を世代12でclaimしてCOMMITし、その後停止する
2. 期限後、recoveryが行をlockし、世代13でreadyへ戻してCOMMITする
3. Bが世代14でclaimする
4. Aが世代12のrenewalまたはcompletionを送る
5. Aの要求は世代不一致になり、Bのleaseやdoneを上書きしない。Aは現在世代を読み取って自分のtokenに差し替えてはならない

recoveryとAのcompletionが競合した場合は同じqueue行のlockで直列化する。Aが有効期限内に先に受理されdoneにしたならrecoveryのstate条件が外れ、recoveryが先に世代を変えたならAの条件が外れる。状態遷移の全経路がこの規約を守ることが前提であり、管理者の無条件UPDATEや世代を戻すrestoreまで防ぐ仕組みではない。

## 4. lease期限だけではexactly-onceにならない

DB内で古い世代を拒否できても、既に走っている外部処理を巻き戻せるわけではない。次の判断表は独自の運用案である。

| 失敗点 | DBから確定できること | 安全側の復旧 |
|---|---|---|
| claim前または明確なrollback | そのtransactionのclaimは成立しない | 新しいclaimを行う |
| claim COMMIT後、処理開始前に停止 | runningの保存状態だけが残り得る | 期限回収し、新しい世代で再試行 |
| COMMIT応答を喪失 | clientからは成立・不成立を確定できない | 既知のclaimを照合。idも不明なら未確認のまま実行せず回収へ |
| 外部API成功後、done前に停止 | 外部成功がqueue行に未記録の可能性 | 同じ業務idempotency keyで結果照合・再試行 |
| lease失効後、旧workerが再開 | 新workerと実行が重なる可能性 | 旧世代のDB更新を拒否し、外部の重複対策も実施 |
| done COMMITの応答を喪失 | doneが既に確定した可能性 | durableな完了記録を照合し、外部処理を即再実行しない |

応答喪失後に普通のSELECTでまだrunningが見えないことも、元transactionのrollback確定とは限らない。照合で再実行権を得るときは同じ行lock・世代・取得後期限の規約を使い、未解決の元transactionと競合する間は開始しない。古いsnapshotやread replicaの未反映を根拠にclaim失敗と決めない。

claim応答を失った時も即時に所有jobを回収したい要件なら、事前発行したclaim-request-idと結果を同じtransactionで記録する別の台帳が必要になる。worker_idでrunningを検索するだけでは、並行claimや再起動との識別が足りない。台帳の一意制約・保存期間・再照合手順は本稿のSQLには実装していない。

外部APIへの重複排除keyは、試行世代ではなく同じ業務操作を表す安定したkeyにすることを検討する。毎回generationをkeyにすると、外部serviceからは別操作に見える。一方、fencing tokenとして世代を送る方式は、相手が対象resource単位で古い世代を検査・拒否して初めて保護になる。同じ数値を送るだけでは効かず、job単位の世代が別jobと共有するresourceにそのまま使えるとも限らない。具体的な外部APIの契約は未確認である。

同一DB内で完結する効果は、所有権チェックと結果保存を一つのtransactionへ入れる設計を優先する。DBとbrokerへの二重書き込みが必要なら既存のoutbox文書を参照する。SKIP LOCKEDの採用だけから、実行回数・配送回数・業務効果のexactly-onceを宣言しない。

## 5. 空の取得・順序・時間の読み違い

### 空の取得は全件完了ではない

候補が全て別transactionにlockされていれば、readyが残っていても取得0件になり得る。将来時刻のjobや、まだCOMMITしていない投入もそのclaimには現れない。独自推奨として、0件は「このscanでclaimできなかった」と記録し、backoff付きpollへ戻す。連続0件だけでqueue drain完了を宣言しない。

有限batchを終わらせるなら対象集合を固定し、producer終了、対象のready/running件数、失敗終端、回収待ちを別に照合する。継続投入されるqueueに全体完了という終端を勝手に作らない。

[UPDATE文書][update]は一括更新の取りこぼし防止として、最後にSKIP LOCKED / LIMITを外したUPDATEを行う例を説明する。ただし、それをworker queueの無制限な「残件を全部claimする」最終処理へ移植しない。保守用の有限集合と、継続稼働する実行枠制限付きqueueでは終了条件が違う。

### ORDER BYでもstrict FIFOではない

`ORDER BY priority DESC, available_at, id` は候補選択の意図を明示するが、lock中の先頭jobは飛ばされる。後のjobが先に開始・完了することは設計上許容しなければならない。優先度の低いjobのstarvationも自動的には解消されない。厳密なtenant内順序や同じaggregateの逐次実行が必要なら、その単位のschedulerまたは直列化機構を別に設ける。

独自の監視案として、全体throughputだけでなく最古readyの待ち時間とtenant別の待ち分布を追う。多数のworkerが同じ先頭領域を繰り返しscanする負荷、優先度変更とlockの組み合わせは、本番に近い競合率で測る。返却batchの順がそのまま業務完了順になると仮定しない。

### DB時刻も用途を分ける

公式には `now()` / `CURRENT_TIMESTAMP` はtransaction開始時刻、`statement_timestamp()` はstatement開始に対応する時刻、`clock_timestamp()` は呼出時の時刻で同一statement内でも変化する。[Date/Time][clocks]

独自判断: scan対象を一つの時点に揃える用途にはstatement時刻、lock取得後の期限再確認にはその後に取得するDB時刻を使う。長いtransactionで `now()` を繰り返しても時計は進まない。statement開始直後からlock待ちした場合、statement時刻も「lock取得時刻」にはならない。DBを時刻基準に揃えても、時計補正・failover先の時刻差・通信遅延をなくせるとは言わない。leaseはmonotonic clockのAPIではなく、旧所有者拒否の主軸は世代条件である。

## 6. 待機時間と維持コストを別に制御する

SKIP LOCKEDを使っても、table lock、追加の業務更新、trigger等の別のlockやI/Oは残る。公式の `lock_timeout` は各lock取得の待機上限、`statement_timeout` はstatement全体の上限である。`idle_in_transaction_session_timeout` は開いたtransactionのidle sessionを終了させる。[Client Defaults][timeouts]

独自推奨:

- queue専用transactionの待機・statement・全体の時間予算を設ける。lock待ちtimeoutをstatement上限以上にしても先にstatement側が発火し得る
- timeout / 切断を「queue空」に変換しない。DB失敗、取得0件、世代不一致、期限切れを別の理由として観測する
- worker停止時は新規claimを止め、進行中jobの残り時間と更新結果を確認する。leaseを無期限に延長してshutdown完了扱いにしない
- ready検索用の部分indexと、running回収用の部分indexを候補にする。例えばready側はpriority・available_at・id、running側はlease_until・idを検討するが、実行計画を測って選ぶ
- 部分indexのpredicateがqueryから推論できることを確認する。`state='ready'` を汎用parameterへ置換したprepared query等では、同じplanがそのindexを使えると決めつけない。[Partial Indexes][partial]
- 更新の多いqueueは古い行version、index維持、vacuum、統計を測定する。heartbeatを頻繁にすれば無償で安全になるわけではない。[Routine Vacuuming][vacuum]
- 取得件数だけでなく、claim latency、最古ready年齢、期限超過running、世代不一致、renewal失敗、再試行回数、最終failedを監視する

性能上限、最適batch数、lease時間、heartbeat間隔には共通の正解を置かない。外部処理の遅延分布、停止検知、DB負荷、許容する重複試行から選び、数値を変える際も世代規約は省略しない。

## 7. 本番前の境界試験と未確認事項

以下は**未実行のruntime試験案**であり、検索evalの成功とは別に実施する。

1. 二つ以上のworkerが同時claimし、同じready行の同じ世代を取得しないこと。片方をrollbackした時に再取得できること
2. 全候補を別transactionでlockし、取得0件でもreadyが残ること。解除後に取得されること
3. table-levelの競合を作り、SKIP LOCKEDだけでは待機を防げず、timeout経路が空取得と区別されること
4. claim後・外部成功後・done更新後の各位置で応答を失わせ、照合せず二重実行へ進まないこと
5. expired行を回収し、新世代claim後に旧世代のrenewal / completion / failureが全て拒否されること
6. renewalとrecoveryを競合させ、期限を延長済みの行を古いscan情報でreadyへ戻さないこと
7. worker更新のlock待ち中に期限を越え、取得後の時刻検査で拒否できること。長いtransactionのnowと新しいDB時刻を混同しないこと
8. 同一DBの業務更新とdone更新を途中失敗させ、同じtransactionでrollbackされること
9. NULLまたは過大なbatch size、世代の桁あふれ、triggerによる更新抑止、id再利用を防ぐこと
10. 高優先度の連続投入と長期lockで、低優先度・tenant別の待ち時間が許容範囲に収まること

PostgreSQL server、driver、poolerを起動した試験は未実施。RLS、partition / 継承、trigger、prepared transaction、replication/failoverのdurability設定、clock補正、multi-database、外部sinkのidempotency/fencing契約は未検証で、この例をそのまま適用する根拠にしない。REPEATABLE READ / SERIALIZABLEへ変更する場合はtransaction全体の再試行設計も再確認する。

## 出典・版・provenance

- UTC取得日は全て2026-10-05。native webで実ページを開いたPostgreSQL `/18/` manualを使用し、検索snippetだけを根拠にしない
- 対象ページに個別の公開日・更新日・patch版は表示されていない。共通ヘッダーの2026-09-24告知日はmanual各節の更新日ではない。18.6等の実行確認や19の動作、SKIP LOCKEDの新規導入を主張しない
- [Legal Notice][license]と[公式License](https://www.postgresql.org/about/licence/)で、documentationを含むPostgreSQL Licenseを確認。コードの転載なし。SQL・状態遷移・試験案は本稿独自、module昇格なし
- 公式の行lock・時刻・SQL契約からqueue運用案を導いており、文書のtrustはprimary-source。期限はofficial_docs / postgresの90日TTLに合わせ2027-01-03

[select]: https://www.postgresql.org/docs/18/sql-select.html
[update]: https://www.postgresql.org/docs/18/sql-update.html
[locks]: https://www.postgresql.org/docs/18/explicit-locking.html
[isolation]: https://www.postgresql.org/docs/18/transaction-iso.html
[clocks]: https://www.postgresql.org/docs/18/functions-datetime.html
[timeouts]: https://www.postgresql.org/docs/18/runtime-config-client.html
[begin]: https://www.postgresql.org/docs/18/sql-begin.html
[commit]: https://www.postgresql.org/docs/18/sql-commit.html
[partial]: https://www.postgresql.org/docs/18/indexes-partial.html
[vacuum]: https://www.postgresql.org/docs/18/routine-vacuuming.html
[license]: https://www.postgresql.org/docs/18/legalnotice.html

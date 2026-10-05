---
{
  "id": "kubernetes-indexed-job-success-retry-terminal-boundary",
  "title": "Kubernetes Indexed Job: retry予算・部分成功・終端conditionの境界",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes v1.37.0 controller f54c212e3a2f75d674b717a9b29052b20b60aefc / rolling docs v1.37; PodFailurePolicy GA v1.31、backoffLimitPerIndex・successPolicy GA v1.33、podReplacementPolicy GA v1.34; 2026-10-05確認・実クラスタ未検証",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "Indexed Job",
    "backoffLimitPerIndex",
    "maxFailedIndexes",
    "podFailurePolicy",
    "successPolicy",
    "podReplacementPolicy",
    "SuccessCriteriaMet",
    "FailureTarget"
  ],
  "sources": [
    {
      "id": "kubernetes-job-policy-concepts-20261005",
      "url": "https://kubernetes.io/docs/concepts/workloads/controllers/job/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-job-policy-api-20261005",
      "url": "https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-job-success-policy-ga-20261005",
      "url": "https://kubernetes.io/blog/2025/05/15/kubernetes-1-33-jobs-success-policy-goes-ga/",
      "type": "maintainer_article"
    },
    {
      "id": "kubernetes-job-replacement-policy-ga-20261005",
      "url": "https://kubernetes.io/blog/2025/09/05/kubernetes-v1-34-pod-replacement-policy-for-jobs-goes-ga/",
      "type": "maintainer_article"
    },
    {
      "id": "kubernetes-job-policy-source-v1370-20261005",
      "url": "https://github.com/kubernetes/kubernetes/tree/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2027-01-03",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/infrastructure.json"]
}
---

# Indexed Job の再試行・部分成功・終了待ちを分ける

## 問いと適用範囲

並列バッチで「一部のindexが成功すれば十分」と指定したのにJobがFailedになる、あるいは成功通知後もPodが動いているのはなぜか。retry予算、Job全体の終了判定、残存Podの停止という三段階を区別する。対象は組込みJob controllerが管理する `batch/v1` のIndexed Job。取得日に登場した新機能の紹介ではなく、既存の実用上の空白を埋める調査である。

既存の[Pod graceful shutdown](pod-graceful-shutdown.md)は停止時のsignalとhook、[PDB](pdb-eviction-drain-unhealthy-policy.md)はeviction許可、[topology spread](topology-spread-eligible-domain-boundary.md)は配置を扱う。本稿はJobが再実行・成功・失敗をいつ決定するかを扱い、業務成果物の永続化やexactly-once実行を保証するものではない。

## 1. retryの単位を先に決める

[JobSpec API](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/#JobSpec)における `backoffLimitPerIndex` は `completionMode: Indexed` とPodの `restartPolicy: Never` が必要で、作成後はimmutable。Pod内のcontainer再起動と、失敗したPodを新しいPodへ置き換えるretryを混同しない。

- `backoffLimit` の既定は通常6だが、`backoffLimitPerIndex` を指定すると、明示していない全体の `backoffLimit` は **2147483647** になる。index単位に変えただけで、以前の全体6回という予算が保たれるわけではない
- `backoffLimitPerIndex: 1` は初回失敗のあと1回retryできる。count対象の失敗が2回続けば、そのindexは `failedIndexes` へ入る。0なら最初のcount対象の失敗でindex終了。`FailIndex` actionはこの残りretryを待たずにindexを失敗させる
- 一つのindexが失敗しても、直ちに残りのindexが止まるとは限らない。全体の `backoffLimit` を別途小さく指定していれば、そちらが先にJobを止めることはある

公式の[per-index例](https://kubernetes.io/docs/concepts/workloads/controllers/job/#backoff-limit-per-index)は、1 retry後に失敗した5 indexを `failed: 10` と表示する。固定版の[isIndexFailed](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/indexed_job_utils.go#L96-L110)と[getNewIndexFailureCounts](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/indexed_job_utils.go#L348-L375)では、replacement Podへ過去のcountを引き継ぎ、今回の失敗とlimitを評価する。したがって `failed` というPod件数を失敗index数に読み替えない。

## 2. maxFailedIndexesは成功を許す件数ではない

`maxFailedIndexes` は `backoffLimitPerIndex` を設定したJobだけで使い、失敗index数が指定値を **超えた** ときに残りの実行を打ち切る。値と等しいだけでは、この条件による打切りは起きない。未指定なら、この早期打切り条件を設けない。[Job概念: per-index backoff](https://kubernetes.io/docs/concepts/workloads/controllers/job/#backoff-limit-per-index)

ただし、早期に打ち切らなかったことと最終成功は別。v1.37.0の[controller](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/job_controller.go#L1094-L1116)は、まだ終了結果が確定していないJobについて次の順に評価する。

1. `maxFailedIndexes` があり、失敗index数がそれを超える → `MaxFailedIndexesExceeded`
2. 失敗indexが一つ以上あり、成功index数と失敗index数の合計が `completions` 以上 → `FailedIndexes`
3. まだ終了条件がなければ `successPolicy` を評価する

例として `completions: 8`、`maxFailedIndexes: 2`、失敗indexが2、成功が6、未処理なしという状態を初めて観測すると、1の上限超過はないが2により失敗になる。失敗を2件まで含んで成功してよい、という契約ではない。この例は分岐から導いた独自例で、クラスタで実行した結果ではない。

取得時のgenerated APIの `maxFailedIndexes` 説明には、未指定で全indexを実行するとCompleteになるという文言がある。これは概念文書および固定版の実装と一致しない。本稿は実装と[未指定の上流test](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/job_controller_test.go#L5521-L5561)が示すFailedを採用する。API説明だけを根拠に部分失敗を成功へ変換しない。

## 3. Pod failure policyは先頭から最初に一致するruleを使う

`podFailurePolicy` はPodの `restartPolicy: Never` で使う。ruleは順番に評価され、最初に一致したactionを採用し、一つも一致しなければ通常のcountへ戻る。各ruleは `onExitCodes` と `onPodConditions` のどちらか一方を使う。[Pod failure policy](https://kubernetes.io/docs/concepts/workloads/controllers/job/#pod-failure-policy)、[rule API](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/#PodFailurePolicyRule)

- `Count`: failureをretry予算へ加算する
- `Ignore`: failureを `backoffLimit` と、指定済みなら `backoffLimitPerIndex` のcountから外す。Jobの成功indexを増やすactionではない
- `FailIndex`: そのindexのretryを止める。`backoffLimitPerIndex` が必要
- `FailJob`: Job全体の失敗処理を始める。残りindexを続けるためのactionではない

policyの照合対象は `phase: Failed` のPodである。PendingのままのImagePullBackOffや、まだ終了中のPodを、condition名だけで即座にFailJobへ変換する仕組みではない。待ち続ける状態には別途deadlineと診断手順を用意する。[Pod failure policyのphase条件](https://kubernetes.io/docs/concepts/workloads/controllers/job/#pod-failure-policy)

両方のretry予算に対するIgnoreと先頭一致は[matchPodFailurePolicy](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/pod_failure_policy.go#L27-L75)でも確認した。例えばDisruptionTargetをIgnoreするruleと特定exit codeをFailJobにするruleの両方が一致し得るなら、順序そのものが終了方針になる。

`containerName` を省略したexit-code ruleは通常containerとinit containerを対象にし、成功exit code 0は判定から除外される。補助containerの失敗をmainの致命的エラーと同一視したくない場合は、対象名を絞る。[exit-code API](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/#PodFailurePolicyOnExitCodesRequirement)、[照合実装](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/pod_failure_policy.go#L78-L85)

Ignoreによってretry予算を消費しない失敗が続くことも考える。実行時間を制限したいならJobレベルの `activeDeadlineSeconds` を別に設計する。これはPod templateの同名fieldとはスコープが違い、Jobのsuspend/resumeではstartTimeとtimerがリセットされるため、単純な作成時刻からの永久上限でもない。[Job termination](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup)、[JobSpec API](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/#JobSpec)

## 4. successPolicyのcountは指定subset内で数える

`successPolicy` はIndexed Job限定でimmutable。ruleに `succeededIndexes` だけなら列挙した全index、`succeededCount` だけなら任意の成功index数、両方なら指定subset内の成功index数で判定する。successPolicyのruleは最大20件。複数ruleは代替条件であり、順番に評価して一つ一致すれば残りを評価しない。AND条件ではない。[SuccessPolicy API](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/#SuccessPolicy)、[matchSuccessPolicy](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/success_policy.go#L28-L49)

独自の計算例: `succeededIndexes: "0,4-6"` と `succeededCount: 2` なら、成功済みが0と3だけではsubset内は1件なので未達。0と4が成功すれば2件で達成する。全体の成功件数だけではこの判定を再現できない。

successPolicy達成時には残るPodの終了処理が始まる。途中のworkerが最後まで走って成果物を作ることを当然の後続処理にしない。leaderのexit 0を条件にする場合は、必要な出力・checkpointの永続化が完了してからleaderが成功終了するよう、アプリ側の契約を定める。これは運用設計案であり、Kubernetesが出力内容を検証するという意味ではない。[GA解説](https://kubernetes.io/blog/2025/05/15/kubernetes-1-33-jobs-success-policy-goes-ga/)

## 5. 「失敗が優先」は既に確定した成功を反転する規則ではない

概念文書のterminating policy優先という説明は、時系列を落とすと誤解しやすい。v1.37.0の[終了判定順序](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/job_controller.go#L1056-L1116)は次の区別を持つ。

| controllerが観測した状態 | この版の扱い |
|---|---|
| 既存の `SuccessCriteriaMet=True` | 前回の成功判定を保持し、新しい失敗policyで上書きしない |
| 既存の `FailureTarget=True` | 前回の失敗判定を保持し、新しいsuccessPolicy達成で救済しない |
| どちらもなく、同じreconcileで失敗条件と成功条件を満たす | 失敗条件を先に評価する |
| どちらもなく、失敗条件は未達でsuccessPolicyを満たす | `SuccessCriteriaMet=True` を追加する |

これはwall clockでどのPodが先にexitしたかを競う仕組みではなく、controllerの観測と記録済みconditionを含む判定である。失敗indexを許すsuccessPolicyを作るときも、「同時に全indexが終われば必ず成功を拾う」とは考えない。前節の `FailedIndexes` 判定が先に成立する場合がある。

上流の[TestSyncJobWithJobSuccessPolicy](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/job_controller_test.go#L4294-L5042)は、同時にFailJob ruleとsuccessPolicyを満たすケースでFailureTargetを期待し、既存SuccessCriteriaMetのあとに同じ失敗が来るケースではCompleteを期待する。backoffLimit・per-index失敗、既存FailureTargetに対する反例もある。上流testの本文と期待値を読んだのであり、上流testを実行して合格させたわけではない。

## 6. 判定通知・停止完了・replacementを分ける

`FailureTarget` と `SuccessCriteriaMet` はPodの終了処理を始めるためのcondition。`Failed` と `Complete` がJobのterminal conditionである。`active: 0` だけでは停止完了にならない。APIのactiveはdeletionTimestamp付きPodを除外し、`terminating` は削除中でPending/RunningのPodを数える。[JobStatus](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/#JobStatus)

v1.31以降の通常動作ではterminal conditionを全Podの終端まで待つ。v1.30以前はterminal conditionが付いても一部Podが実行中・終了中の場合があった。固定版の[enactJobFinished](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/job_controller.go#L1602-L1624)も、未計上の終端Podやterminating Podが残る間は確定を遅らせる。[Terminal Job conditions](https://kubernetes.io/docs/concepts/workloads/controllers/job/#terminal-job-conditions)

別Jobへのやり直しをFailureTarget時点で開始すると早いが、旧JobのPodと重なり得る。資源の重複を避けたい運用はFailedまで待つ。これはJob内のreplacement Podを制御する `podReplacementPolicy` とは別の判断である。

Job内の `podReplacementPolicy` は、podFailurePolicyなしで省略すると `TerminatingOrFailed`。削除中と分かった時点でreplacementを作れるため、瞬間的なPod数はparallelismを超え得る。`Failed` なら終端まで待ち、podFailurePolicyがあるJobでは **Failedが既定かつ唯一の許可値**。[replacement GA解説](https://kubernetes.io/blog/2025/09/05/kubernetes-v1-34-pod-replacement-policy-for-jobs-goes-ga/)、[JobSpec API](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/#JobSpec)

概念文書前半にはpodFailurePolicyがあっても削除開始時にreplacementするという記述が残るが、後半の専用節・API・固定版の実装とは一致しない。本稿は専用契約と実装を採用する。[manageJob](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/job_controller.go#L1811-L1838)はFailed policy時にterminating数を差し引き、[firstPendingIndexes](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/job/indexed_job_utils.go#L242-L258)はterminating中のindexを追加候補から除く。APIの終端にはSucceededも含まれ、成功済みindexは新規候補から除かれる。「Failedという名前だから成功終了したPodも必ずもう一度起動する」とは読まない。

このpolicyで通常の終了待ちを改善しても、exactly-once実行や外部ストレージへの排他書込みの保証にはならない。公式はIndexed Jobで同じindexの複数Podが開始され得ると注意する。出力をindex別に分ける、確定書込みを冪等にする、必要なら外部の所有権・fencingを設けるのは別のアプリ設計である。[Completion mode](https://kubernetes.io/docs/concepts/workloads/controllers/job/#completion-mode)

## 7. 実装前の確認手順（独自の運用案）

1. 対象cluster・Job UID・controller版・実効specを記録する。`managedBy` が外部controllerなら、その実装契約を別途調べる。組込みcontrollerの読解をそのまま適用しない
2. 全index必須かsubset成功で十分かを先に決める。成功後に止めてもよいworkerと、保存必須の成果物を特定する
3. indexごとのretry、全体backoff、maxFailedIndexes、Job deadlineを別々の予算として表にする。Ignoreのrule順序と対象containerも含める
4. 成功index集合、失敗index集合、Pod数、terminating数、conditionsのtype/status/reasonを一緒に見る。未設定conditionをFalseやCompleteへ変換しない
5. 隔離環境で「一つのindexだけ失敗して継続」「上限と等しい／超える」「全index終了時の部分失敗」「subset外の成功」「同一reconcileで両条件」「成功確定後のworker失敗」「長い終了猶予」を別caseにする
6. 再実行を許可する前に、旧Jobの終了待ち方針と出力の重複防止を確認する。Failed Jobを同じPodのrestartPolicyが自動再始動するとは期待しない

本リポジトリのevalは検索で知識を再発見するためのもの。Job controller、scheduler、アプリの冪等性や成果物の永続化を検証する実行testではない。

## 適用版・出典・未確認事項

取得日は2026-10-05 UTC。Job概念文書はv1.37表示・最終更新表示2026-07-31、Job APIはv1.37生成・更新表示2026-08-26。successPolicyのGA記事は2025-05-15公開、replacement policyのGA記事は2025-09-05公開で、ともにfooter更新表示は2026-01-03。ページの再編日を機能導入日へ読み替えない。

PodFailurePolicyはv1.31、per-index backoffとsuccessPolicyはv1.33、podReplacementPolicyはv1.34でstable。以前の版にはfeature gateの有効化条件や終端通知時期の差がある。本稿は過去の全組合せを検証していない。現在版のgate名を旧clusterへそのまま追加する手順としては使わない。

コードとtestはv1.37.0 tagの[annotated tag object](https://api.github.com/repos/kubernetes/kubernetes/git/tags/157e582fcc3ebba3c22b16721f49d6890f784c1f)から確認したcommit `f54c212e3a2f75d674b717a9b29052b20b60aefc` に固定。公式文書の不整合は上記二箇所を明示して解釈した。patch版、vendor差分、外部controllerへの一般化は行わない。

Kubernetes contributorsのwebsite文書は[CC-BY-4.0](https://github.com/kubernetes/website/blob/main/LICENSE)、固定commitのコード・testは[Apache-2.0](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/LICENSE)と各headerを確認した。本文は出典付きの独自要約・実装読解・設計案で、上流manifestやコードは転載していない。

実クラスタ・upstream unit/e2e testは未実行。Jobの強制削除、node分断時の実プロセス停止、全version skew、WorkloadWithJob/gang scheduling、JobSet等の外部controller、storage側commitの成否は本稿の保証外。native webで固定版testページを開けなかったため、同じSHAの本文をGitHub connectorで取得して補った。

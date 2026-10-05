---
{
  "id": "kubernetes-resize-generation-actuation-preemption-boundary",
  "title": "Kubernetes in-place resize: 世代・資源反映とv1.37 preemptionの完了境界",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes rolling docs v1.37 / Pod・Node v1 API generated 2026-08-26; core resize・observedGeneration stable v1.35, scheduler preemption alpha/default-off v1.37; verified 2026-10-05 UTC, cluster/runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "in-place-resize",
    "observedGeneration",
    "allocatedResources",
    "PodResizePending",
    "PodResizeInProgress",
    "Deferred",
    "Infeasible",
    "preemption",
    "disableResizePreemption",
    "resizePolicy"
  ],
  "sources": [
    {
      "id": "kubernetes-resize-task-v137-20261005",
      "url": "https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-resize-pod-api-v137-20261005",
      "url": "https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-resize-preemption-v137-20261005",
      "url": "https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-resize-node-api-v137-20261005",
      "url": "https://kubernetes.io/docs/reference/kubernetes-api/core/node-v1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-resize-gates-v137-20261005",
      "url": "https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-resize-preemption-blog-20261005",
      "url": "https://kubernetes.io/blog/2026/09/10/kubernetes-v1-37-scheduler-preemption-for-in-place-pod-resize-alpha/",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2027-01-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Kubernetes in-place resize の受付と反映完了を分ける

## 問い・結論・対象

CPUやmemoryの増量をPodの`/resize`へ送信し、patchが成功した。これでアプリケーションが新しい資源を使えると判断してよいか。容量不足の`Deferred`が続くとき、Kubernetes 1.37への更新だけで解消するか。

結論は、**spec保存、kubeletによる割当て、runtimeへの反映、アプリケーションの正常稼働を別々に確認する**こと。v1.37のscheduler preemptionは、同じnodeの低優先度Podを終了させて容量を確保する追加のalpha機能である。中核のresizeがstableでも、この追加機能の有効化や、増量の成功・無停止を意味しない。

本稿は2026-10-05 UTCに確認したKubernetes rolling docsのv1.37表示を対象にする。中核の`InPlacePodVerticalScaling`と`PodObservedGenerationTracking`はv1.35でstable。preemptionの導入は[2026-09-10のmaintainer記事][blog]と[feature gate表][gates]でv1.37 alpha・既定無効と確認した。新機能の紹介を起点に、未収録のresize完了判定を整理する。MemoryQoSの保護係数やrollback、Job終了判定、topology spreadの配置計算は扱わない。

## 1. 保存・割当て・反映の観測値

[公式taskのKey Concepts][task]と[Pod API][pod]に基づく区別を示す。

| 観測値 | 分かること | これだけでは分からないこと |
|---|---|---|
| `spec.containers[*].resources` | 保存された希望のrequests/limits | node上で反映済みか |
| `status.containerStatuses[*].allocatedResources` | kubeletが認めた割当て値。内部の資源計数に使う | running containerへの適用完了 |
| `status.containerStatuses[*].resources` | running containerに現在設定されたrequests/limits | アプリが健全に利用できるか |
| `status.observedGeneration` | kubeletが認識したspecの世代 | その世代のresizeが完了したか |
| `status.conditions` | 割当て待ちや反映途中の理由・世代 | 全処理に共通の一個の成功フラグ |

`resources`にも注意がある。未起動または再起動したcontainerでは、次回起動に割り当てられた値を示す。したがって値一致だけではrunningを証明できない。`PodStatus`はnodeからcontrol planeへの通信状況によって実態より遅れ得る。Pod名が同じというだけで過去のstatusを流用しない。

`PodResizePending=True`の`reason: Deferred`は、現在は割当てできないが後で可能になる場合を示し、kubeletが再試行する。`Infeasible`は現在のnodeでは実現できない要求を示す。これはAPIに保存できた希望値が適用されない経路でもある。APIが要求そのものを拒否した場合とは別に扱う。

`PodResizeInProgress=True`は割当てを受け入れた後の適用中を示し、問題があれば`reason: Error`と`message`を調べる。`status.observedGeneration`が進んでもこのconditionは残り得る。APIの旧`status.resize`はdeprecatedであり、現在の二つのconditionを監視する設計へ移す。[状態と世代の契約][task]、[PodStatus][pod]

### 条件ごとの世代を読む

トップレベルの`status.observedGeneration`は最新の認識済みspecに対応する。一方、`PodResizeInProgress`の`observedGeneration`はその反映を開始した世代、`PodResizePending`ではその割当てを最後に試みた世代を表す。新しいresizeを発行した直後に、前回のconditionやcondition不在を成功判定へ使わない。[observedGenerationの説明][task]

独自の例として、要求世代12の後で世代13の増量が保存され、statusがまだ12を示している場合、12の成功は13の成功ではない。世代13で希望を縮小し直した場合も、古い増量が最終的に完了するのを機械的に待つ設計は避ける。

## 2. 完了判定の実務手順（独自の設計案）

以下は公開契約を組み合わせたcontroller・運用監視の提案であり、公式の一つの判定APIや実測済み実装ではない。

1. 変更前にPod UID、対象container名、現在の資源値と状態を保存する。変更応答の`metadata.generation`と、defaulting後の希望requests/limitsを対象の要求として記録する
2. 監視対象のUIDが変われば、同名でもreplacementとして元の試行と分離する。別の更新で希望資源値が変わったら、後続要求に置き換わったことを記録し、元の成功と混ぜない
3. 現在のspecが目標と一致し、`status.observedGeneration`がそのspecの世代に追いついたことを確認する。世代未観測・status欠落・通信途絶は未確認のままとする
4. 対象resizeに関わる`PodResizePending`と`PodResizeInProgress`の`status`、`reason`、`message`、`observedGeneration`を調べる。現在世代で未解消の待機・反映中を成功へ読み替えない。古い世代のconditionが残る場合も更新を待ち、都合よく無視しない
5. 対象containerを配列位置ではなく名前で照合する。必要な全containerがrunningで、`status.containerStatuses[*].resources`のrequestsとlimitsが目標に一致することを確認する。CPUの`1`と`1000m`などQuantityは文字列ではなく値として比較する
6. ここまでを資源反映の確認とし、readiness、エラー率、OOM、アプリ内heapやworker数などの利用状態を別に確認する。資源上限の変更にアプリ自身が適応するとは限らない
7. 運用の時間予算を超えたら成功にも自動rollbackにもせず、未完了として理由と最後の証拠を残す。`Deferred`ならnodeの空きと優先度・policy、`Infeasible`なら要求値やnode適合性、`InProgress/Error`ならruntimeとnode側エラーを切り分ける

複数containerのCPUとmemoryを一度に変える場合、最初の一つだけを確認しない。メトリクスや画面も「要求を受付」「反映を確認」「業務を確認」を分け、どのUID・世代を見た結果かを残す。これらは独自の運用判断であり、反映時間や全container間の原子的な更新をKubernetesが保証するという主張ではない。

## 3. v1.37 preemptionで変わること・変わらないこと

[preemptionの公式説明][preemption]では、`InPlacePodVerticalScalingSchedulerPreemption`が有効なとき、容量不足で`Deferred`となる高優先度Podについて、schedulerが**現在のnodeだけ**で低優先度Podのpreemptionを試みる。

- 必要な容量は希望値と割当て済み値の差分を基に計算する。resize要求中の資源はschedulerの計数上使用済みとして扱い、低優先度Podへの二重割当てを防ぐ
- Podを別nodeへ再配置したり再bindしたりする処理ではない。`nominatedNodeName`もこのためには設定しない。容量を空けた後の実際の反映はkubeletが行う
- 同じnodeに十分な対象victimがなければ、gateを有効にしても増量できない。別nodeの空き容量や新nodeの追加を、このrunning Podへの自動移動と同一視しない

[2026-09-10の記事][blog]が案内する構成は、control planeとworkerをv1.37以降に揃え、kube-apiserver、kube-scheduler、kubeletで当該gateを有効にするもの。ここではv1.37文書を確認したのであり、異なるminor版が混在する配布物や将来版まで検証したわけではない。

独自の設計判断として、これを単なる「高優先度Podの性能改善」設定として導入しない。同居する低優先度workloadに終了が発生し得る変更として、PriorityClassの所有者、再実行可能性、可用性をレビューする。公式preemption文書でもPDBの尊重はbest effortであり、PDBを絶対的なpreemption拒否権と扱わない。被害Podの終了待ちがあるため、preemptionイベント発生時刻をresize完了時刻にしない。[preemptionの制約][preemption]

### node単位の拒否はbooleanではなくownerのリスト

[Node API][node]の`spec.podPreemptionPolicy.disableResizePreemption`は、無効化を要求したownerの文字列配列である。一件でもあれば、そのnodeでresizeに起因するpreemptionは無効になる。空または省略時の許可は、クラスタのalpha gate有効という前提の中で解釈する。空配列を指定してもgate自体は有効にならない。

[公式ガイド][preemption]の入力制約はlabel key形式、最大20件。独自の運用案として、複数のautoscalerやoperatorが使うnodeでは、自分のowner要素だけを追加・除去する更新方法を選ぶ。自分の保守が終わったことを理由にリスト全体を空にすると、他ownerが必要としている抑止も失われる。更新競合・権限・所有者消失時の回収は別に設計する。

このpolicyを通常のPod配置時のpreemptionや、あらゆるnode-pressure evictionを止める設定へ一般化しない。

## 4. 無停止と縮小の限界

`resizePolicy`の既定`NotRequired`は、その資源変更のためのrestartを要求しない設定である。`RestartContainer`を選んだ資源を変えればcontainerが再起動する。CPUがNotRequiredでもmemoryがRestartContainerで両方変わればrestartが必要になる。Podを作り直さないことと、containerが一度も止まらないことを分ける。[container resize policy][task]

memory limit縮小時、NotRequiredでは現在使用量が新limitより大きいと変更を見送り、InProgressが続く場合がある。事前確認の後に使用量が増える競合も残るため、OOMを完全に防ぐ保証ではない。独自の運用案として、使用量低下を促すアプリ側の手順やrestart許容性を決めてから縮小し、待機中を処理済みと数えない。

対象は通常のLinux container-level CPU/memory resizeに絞る。QoS class変更、Windows、非restartable init containerやephemeral containerなど、taskにある制約を適用環境で確認する。taskはstatic CPU/Memory managerを制約として挙げる一方、[feature gate表][gates]には`InPlacePodVerticalScalingExclusiveCPUs`と`InPlacePodVerticalScalingExclusiveMemory`という別のalpha拡張がある。したがってstatic policyを全構成で永久に未対応と断定せず、本稿の確認範囲外とする。

Pod全体の`.spec.resources`変更、memory-backed emptyDirのsizeLimit、swapとexclusive resourceの全組合せは本稿の手順をそのまま適用しない。それぞれのgate、node条件、statusの粒度が必要である。

## 5. 受入試験案と資料の読み分け

以下は未実行の受入試験案であり、検索evalとは別に行う。

| 入力・状況 | 誤って成功にしないための観測 |
|---|---|
| patch後、kubeletはまだ旧世代 | observedGenerationが追いつくまで未確認 |
| 新割当て済み、runtime更新中 | allocatedResources一致だけでは完了しない |
| 1回目の増量中に2回目の要求 | 世代・希望値を照合し、古い完了を流用しない |
| 同名Podが作り直された | UIDで元の試行と分離 |
| 複数containerの一つだけ更新済み | 対象名すべての反映値とrunningを確認 |
| memory縮小先より使用量が多い | InProgress継続、OOM、利用状態を観測 |
| preemption gate無効、またはnode ownerリストあり | Deferredを想定し、勝手にpolicyを解除しない |
| gate有効だがvictim不足・終了途中 | preemption開始と容量確保・資源反映を分離 |

[maintainer記事][blog]の末尾例はallocatedResources.cpuの取得を成功確認として示すが、直前にResizeCompletedイベントを伴う一連の例である。これを任意の時点でallocatedResourcesだけを比較する汎用判定へ切り出さない。また同記事の`containerStatuses[].resizeStatus`という説明は、今回の[Pod API][pod]で確認したdeprecatedな`status.resize`および二つのPod conditionの契約と一致しないため、本稿の監視先には採用しない。公開記事は機能の動機と日付の根拠とし、機械判定のfieldはtask/APIを優先して照合した。

## 出典・取得日・ライセンス・未確認事項

各資料の本文を2026-10-05 UTCに取得した。taskの更新表示は2026-07-01、preemptionは2026-06-24、Pod/Node APIはv1.37生成・更新表示2026-08-26、feature gatesのfooterは2026-01-27。記事はNatasha Sarkar (Google)、公開日2026-09-10、更新表示2026-09-03。footerはページ全体の表示であり、個別機能の導入日と同一視しない。

Kubernetes contributorsの[website LICENSE][license]でCC-BY-4.0を確認した。出典への帰属を付けた独自の日本語要約と設計案であり、原文の長文・実装コード・YAML例の転載はない。catalogは今回確認した資料用に新規作成し、既存recordは変更しない。official_docs / maintainer_articleのTTLは90日、再確認期限は2027-01-03。

実クラスタ・container runtime・cgroup値・VPA・managed service・version skewでの動作、具体的な反映時間やvictim選定、exclusive resource拡張は未検証。generated API v1.37の旧形式URLとPod Conditionsの単独URLは取得エラーだったため根拠に使わず、取得できた現行Pod/Node APIとtaskを参照した。検索evalの通過は本番挙動を証明しない。

[task]: https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/
[pod]: https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/
[preemption]: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/
[node]: https://kubernetes.io/docs/reference/kubernetes-api/core/node-v1/
[gates]: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/
[blog]: https://kubernetes.io/blog/2026/09/10/kubernetes-v1-37-scheduler-preemption-for-in-place-pod-resize-alpha/
[license]: https://raw.githubusercontent.com/kubernetes/website/main/LICENSE

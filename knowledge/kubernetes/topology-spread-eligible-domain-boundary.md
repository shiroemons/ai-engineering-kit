---
{
  "id": "kubernetes-topology-spread-eligible-domain-boundary",
  "title": "Kubernetes topology spread: minDomains・計数対象・rollout が Pending を変える境界",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes rolling docs v1.37 / Pod v1 API (generated 2026-08-26); minDomains v1.30+、node inclusion policies GA v1.33、matchLabelKeys beta v1.27+ / selector merge beta v1.34+; 2026-10-04確認・実クラスタ未検証",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "topologySpreadConstraints",
    "maxSkew",
    "minDomains",
    "nodeAffinityPolicy",
    "nodeTaintsPolicy",
    "matchLabelKeys",
    "autoscaling"
  ],
  "sources": [
    {
      "id": "kubernetes-topology-spread-concepts-20261004",
      "url": "https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-topology-spread-pod-api-20261004",
      "url": "https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-topology-spread-gates-20261004",
      "url": "https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-topology-node-autoscaling-20261004",
      "url": "https://kubernetes.io/docs/concepts/cluster-administration/node-autoscaling/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-topology-node-affinity-20261004",
      "url": "https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-topology-taints-20261004",
      "url": "https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Kubernetes topology spread の計数と配置の境界

## 問いと適用範囲

zone 分散を指定したのに rollout の新 Pod が Pending になる、または `maxSkew: 1` なのに全体の最大・最小差が1を超えるのはなぜか。`spec.topologySpreadConstraints` が何を数え、どの配置候補を拒否するかを整理する。通常の kube-scheduler による Pod 単位の配置が対象で、取得日に追加された新機能という主張ではない。

既存の [PDB と drain](pdb-eviction-drain-unhealthy-policy.md) は eviction の許可、[probes と EndpointSlice](probes-endpoints.md) は実行後の健康判定・トラフィック反映を扱う。本稿は replacement を含む新 Pod の配置段階に限定し、Pod の分散を可用性や Ready replica 数の保証に置き換えない。

## 確認した契約

### 1. maxSkew は候補 domain と global minimum を比較する

`topologyKey` とその node label の値の組が domain になる。例えば `topology.kubernetes.io/zone` なら zone ごとであり、node 数そのものを数える設定ではない。`labelSelector` に一致する同一 namespace の Pod が計数候補になる。[Pod API: TopologySpreadConstraint](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#TopologySpreadConstraint)、[Implicit conventions](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#implicit-conventions)

`whenUnsatisfiable: DoNotSchedule` では、配置先 domain の matching Pod 数に今回の Pod を加えた値と、global minimum の差が `maxSkew` を超える候補を拒否する。自分の selector に一致しない Pod はその加算に入らない。`maxSkew` は正の整数で明示する。`ScheduleAnyway` は skew 改善を優先する soft 制約であり、同じ数値を hard 上限として使えない。[API の maxSkew / whenUnsatisfiable](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#TopologySpreadConstraint)

以下は API の契約から計算した独自例で、既存 Pod が同じ selector に一致し、他の配置制限がないものとする。

- eligible な A/B/C の Pod 数が `4/1/1`、`maxSkew: 1`、`minDomains` 省略なら global minimum は1
- 新 Pod を B に置く判定は `(1 + 1) - 1 = 1` なので、この spread 条件を満たす。配置後は `4/2/1` で、全体の最大・最小差はなお3
- A への追加は `(4 + 1) - 1 = 4` で拒否される

したがって hard 制約は既存の偏りを即座に修復する invariant ではない。公式も Pod 削除後の分布維持を保証しておらず、scale-down によって再び偏り得ると明記する。[Known limitations](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#known-limitations)

### 2. minDomains は不足時に比較下限をゼロにする

eligible domain 数が `minDomains` 未満なら global minimum は **0**。以上なら eligible domain の最小 Pod 数を使う。`minDomains` の省略は1相当で、指定値は正の整数かつ `DoNotSchedule` との組合せに限られる。[API の minDomains](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#TopologySpreadConstraint)

独自の境界例: 現在 eligible な domain が A/B の2つだけで、計数が `1/1`、`maxSkew: 1` とする。

| 設定 | global minimum | A または B への追加 | 意味 |
|---|---:|---|---|
| `minDomains: 3` | 0 | `2 - 0 = 2` で拒否 | 3 domain が揃わない間の増設を抑制する |
| `minDomains` 省略 | 1 | `2 - 1 = 1` で条件を満たす | 今ある2 domain の中で配置できる |

`minDomains: 3` は3つの zone を作成する指示でも、各 zone に必ず Pod を持つ保証でもない。この例で値を外せば Pending が解消し得る一方、3 zone 分散という運用意図も弱くなる。この判断は公式の算術に基づく設計上の解釈である。

### 3. eligible は「配置可能な node すべて」と同義ではない

計数対象は node inclusion policy に依存する。

| 項目 | 省略時 | `Honor` | `Ignore` |
|---|---|---|---|
| `nodeAffinityPolicy` | `Honor` | Pod の nodeAffinity / nodeSelector に合う node を計数対象にする | これらの条件による計数対象の絞込みをしない |
| `nodeTaintsPolicy` | `Ignore` | taint のない node、または incoming Pod が tolerate する tainted node を含める | taint による計数対象の絞込みをしない |

根拠: [Pod API の node inclusion fields](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#TopologySpreadConstraint)。この表は skew の母集団の指定である。通常の配置では required node affinity / nodeSelector を満たす必要があり、toleration のない `NoSchedule` taint も配置を拒否する。`Ignore` にしてもそれらを解除しない。また toleration だけでは配置は保証されない。[Node affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity)、[Taints and Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/#concepts)

独自の読み方: tainted な空の zone が既定の `nodeTaintsPolicy: Ignore` で計数に入ると、そこが global minimum を下げる一方、新 Pod はその zone に配置できない場合がある。`Honor` への変更を検討する際も、除外後に `minDomains` を下回れば下限が再び0になるため、両項目を一緒に再計算する。単独の値変更で復旧を保証しない。

### 4. rollout の selector は何を均等化するかを変える

`matchLabelKeys` は incoming Pod の label から値を取り、`labelSelector` と AND で計数対象を絞る。例えば `pod-template-hash` を使うと Deployment の revision ごとの分散になる。旧 revision と新 revision の合計を一つの集合として均等化する意図とは区別する。`labelSelector` は必要で、同じ key を両方に指定できず、incoming Pod に存在しない key は無視される。[API の matchLabelKeys](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#TopologySpreadConstraint)

v1.34 以降の selector merge が有効な場合、kube-apiserver は **Pod 作成時** に key/value を `labelSelector` へ明示的にマージする。Pod の label を後から直接変更してもマージ済み selector は追従しない。再ラベルで計数グループを切り替える設計を避け、Deployment template と作成された Pod の両方で実際の selector を確認する。[Spread constraint definition](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#spread-constraint-definition)

### 5. node 数ゼロの zone を scheduler は先回りして数えない

scheduler は既存 node の label から topology domain を知る。node group がゼロまで縮小されると、その domain に node が現れるまで分散計算の domain として認識できない。全 topology domain と spread 制約を理解する node autoscaler が対処方法として挙げられている。[Known limitations](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#known-limitations)

node autoscaler は Pod の scheduling 制約と自身の node 設定を材料に provisioning するが、設定上限・不適合な node 設定・cloud provider の容量不足で配置可能化に失敗し得る。provider integration によって機能も異なる。[Node provisioning](https://kubernetes.io/docs/concepts/cluster-administration/node-autoscaling/#node-provisioning)、[Autoscalers](https://kubernetes.io/docs/concepts/cluster-administration/node-autoscaling/#autoscalers)

したがって「`minDomains` を書けば空の zone に自動増設される」という設計にはしない。対象 autoscaler / provider / node-group 設定で、ゼロから復帰する domain の label・taint・capacity が意図どおり推定されるかを別途検証する。本稿は特定製品の対応を保証しない。

## Pending を切り分ける手順（独自の設計案）

1. incoming Pod の namespace、実 label、保存済み selector、全 `topologySpreadConstraints` を読む。自身が selector に一致しないと自分を数えないため、単に replica 数を数える診断は外れる
2. `DoNotSchedule` と `ScheduleAnyway` を区別する。hard 制約を複数置いたときは AND であり、zone 側と hostname 側が別々に通っても共通の配置先がない場合がある。hard 制約で必要な topology key を欠く node も確認する。[公式の複数制約例・暗黙規約](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#example-conflicting-topology-spread-constraints)
3. inclusion policy に従って domain と matching Pod 数を列挙し、`minDomains` 判定後の global minimum を計算する。Ready 数や全 namespace の Pod 数で代用しない
4. spread を満たす候補に対して required affinity、taint、resource requests、volume 条件などの別の配置制約を確認する。scheduler の拒否理由と node autoscaler の provisioning 判断を突き合わせる
5. rollout で全 revision を一緒に数えるか、`pod-template-hash` ごとに数えるかを決める。後者を選ぶなら旧新の合計配置・surge 時の余力も評価する
6. zone/node group のゼロ台状態、増設上限、容量不足を含む復旧経路を確認する。`maxSkew` 緩和や `ScheduleAnyway` への変更は分散要件の変更として扱い、Pending 解消だけを成功基準にしない

### 導入前の検証ケース

以下は提案であり、実行結果ではない。

- `4/1/1` からの追加: `maxSkew: 1` でも B/C への追加が許され、既存の最大・最小差を即時修復しないこと
- A/B の `1/1` と `minDomains: 3`: 両方への追加が拒否され、第3 domain の node 出現で再評価されること
- 空の tainted domain: `Ignore` と `Honor` の両方で、計数と実際の `NoSchedule` 拒否を別々に記録すること
- rollout: revision を含む/含まない selector の差と、Pod 作成後の label 直接変更が selector に追従しないこと
- zone と hostname の hard 制約: 各候補の積集合が空になるケースを確認すること
- scale-down / zero-node-zone: 削除後の偏り、autoscaler の再増設、provider 容量不足時の Pending を観測すること

## 適用版・出典・未確認事項

2026-10-04 UTC に6件の Kubernetes 公式本文を native web で確認した。各ページの version selector は v1.37。Pod API は2026-08-26の v1.37 生成と表示される。他の表示更新日は topology spread が2025-10-27、feature gates が2026-01-27、node autoscaling が2026-06-14、node affinity が2026-02-10、taints が2026-07-27。更新日は feature 導入日とは限らない。

- `minDomains`: v1.30 より前は `MinDomainsInPodTopologySpread` gate が必要で、既定有効は v1.28 からと概念ページに記載
- node inclusion policies: v1.26 beta、v1.33 GA と概念ページに記載。beta 時代の gate 無効化説明を v1.37 の運用手順に流用しない
- `MatchLabelKeysInPodTopologySpread`: v1.27 から beta・既定 true。`MatchLabelKeysInPodTopologySpreadSelectorMerge`: v1.34 から beta・既定 true。v1.37 の [Feature Gates 表](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/) でも確認した。Pod affinity 用の同名 field と状態・gate を混同しない

未確認: 実クラスタでの scheduling、scheduler profile の独自設定、preemption / descheduler、削除中・終端状態 Pod の細かな計数、特定 autoscaler の版別 scale-from-zero 対応、provider の容量・復旧時間。v1.37 の Workload / PodGroup / CompositePodGroup による group scheduling は対象外。検索 eval は文書を取得できるかの確認であり、上記のクラスタ検証を代替しない。

Kubernetes contributors の資料について [website LICENSE](https://github.com/kubernetes/website/blob/main/LICENSE) の CC-BY-4.0 を確認した。本文は出典を示した独自要約・算術例・設計案であり、YAML や実装コードは転載していない。全 source は `official_docs`（TTL 90日）で、再確認期限は2027-01-02。

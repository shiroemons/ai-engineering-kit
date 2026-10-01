---
{
  "id": "kubernetes-pdb-eviction-drain-unhealthy-policy",
  "title": "Kubernetes PDB と node drain: voluntary eviction の境界・unhealthy policy・status の確認",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes rolling docs 2026-10-01確認（selector v1.37 / API reference v1.37）; policy/v1 PDB stable v1.21、Eviction v1.22+、unhealthy policy stable v1.31; cluster未検証",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "PodDisruptionBudget",
    "PDB",
    "eviction",
    "drain",
    "minAvailable",
    "maxUnavailable",
    "AlwaysAllow",
    "IfHealthyBudget",
    "observedGeneration"
  ],
  "sources": [
    {
      "id": "kubernetes-disruptions-20261001",
      "url": "https://kubernetes.io/docs/concepts/workloads/pods/disruptions/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-configure-pdb-20261001",
      "url": "https://kubernetes.io/docs/tasks/run-application/configure-pdb/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-api-eviction-20261001",
      "url": "https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-pdb-api-v1-20261001",
      "url": "https://kubernetes.io/docs/reference/kubernetes-api/policy/pod-disruption-budget-v1/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-12-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Kubernetes PDB と node drain

## 調査の問いと範囲

node メンテナンスで `kubectl drain` が止まったとき、PodDisruptionBudget（PDB）をどの順で確認するか。既存の [Pod 終了処理](pod-graceful-shutdown.md) は終了開始後の順序を扱う。本稿は終了を許可する前の admission と可用性の条件を扱い、新機能の発表としては扱わない。

## 確認した契約

### PDB が保護する経路

PDB を尊重する運用ツールは Pod の直接 DELETE ではなく Eviction API を使う。`kubectl drain` は拒否された eviction を再試行し、終了または指定した timeout まで待つ。Pod/Deployment の直接削除は PDB を迂回する。障害による involuntary disruption は防げず、使用済み budget には数えられる。Deployment/StatefulSet の rolling update も PDB で制限されず、更新時の可用性は各 workload の設定で管理する。[Disruptions](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/#pod-disruption-budgets)

### selector と小さい replica 数

- `minAvailable` と `maxUnavailable` は同時指定できない。百分率は両方とも切上げる。`minAvailable` は必要な healthy 数を切上げ、`maxUnavailable` は許可する unavailable 数を切上げるため、後者では特に小さい replica 数で停止割合が設定値を超え得る。`maxUnavailable` は選択 Pod が同じ controller に管理される場合に用いる
- `policy/v1` の空 object selector `{}` は namespace の全 Pod を選ぶ。null selector は何も選ばない。旧 `policy/v1beta1` の empty selector の意味を引き継がない
- Ready condition が True の Pod が healthy として数えられる。desired replica 数と healthy 数は同じではない

根拠: [PDB 設定](https://kubernetes.io/docs/tasks/run-application/configure-pdb/#specifying-a-poddisruptionbudget)、[v1 API selector](https://kubernetes.io/docs/reference/kubernetes-api/policy/pod-disruption-budget-v1/#PodDisruptionBudgetSpec)

### Running だが unhealthy な Pod

`unhealthyPodEvictionPolicy` の既定 `IfHealthyBudget` では、Running かつ未 Ready の Pod は `currentHealthy >= desiredHealthy` の場合に eviction できる。既に不足しているアプリの回復を待つため、CrashLoopBackOff 等で drain が止まり得る。`AlwaysAllow` では、その unhealthy Running Pod は budget 不足でも eviction できるが、回復の機会を失う。healthy Pod には引き続き PDB 制約が適用される。[設定の policy 比較](https://kubernetes.io/docs/tasks/run-application/configure-pdb/#unhealthy-pod-eviction-policy)、[v1 API](https://kubernetes.io/docs/reference/kubernetes-api/policy/pod-disruption-budget-v1/#PodDisruptionBudgetSpec)

### 応答と観測の限界

- Eviction API の `200` は許可を表し、その後に kubelet の終了処理が続く。`terminationGracePeriodSeconds` は尊重される
- `429` は PDB 拒否だけでなく API rate limiting でも生じる。`500` には同じ Pod を複数の PDB が選択する等の設定不備がある。HTTP code だけで原因を断定しない
- PDB status は実状態から遅れ得る。`disruptionsAllowed` 等は `status.observedGeneration` が object の `metadata.generation` と一致する場合に有効。正数を一度読んでも将来の eviction 枠を予約したことにはならない

根拠: [Eviction API](https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/#how-api-initiated-eviction-works)、[PDB status](https://kubernetes.io/docs/reference/kubernetes-api/policy/pod-disruption-budget-v1/#PodDisruptionBudgetStatus)。最後の「予約ではない」は status が観測値であることからの設計上の解釈。

## 実務での判断手順（独自の設計案）

以下は公式要件を組み合わせた runbook 案であり、クラスタで動作確認した手順ではない。

1. 操作が Eviction API か、直接 DELETE / rollout かを特定する。後者を PDB で安全化できると考えない
2. 対象 namespace・Pod label・owner・PDB selector を読み、空 selector や重複 PDB がないかを見る
3. `observedGeneration`、`currentHealthy`、`desiredHealthy`、`disruptionsAllowed` を一緒に記録し、Ready にならない Pod と新規 Pod の scheduling failure を調べる
4. drain の失敗応答とサーバの説明を突き合わせる。replacement の Pending、容量不足、長い終了処理、readiness の不成立を分けて解決する
5. unhealthy Pod をその場で回復させたいのか、別 node に作り直したいのかを決め、`IfHealthyBudget` / `AlwaysAllow` の選択を workload 所有者とレビューする。変更によるデータ損失や復旧失敗は PDB だけでは防げない
6. 必要な quorum・負荷容量に対して budget を決め、実際に用いる最小/通常/最大 replica 数で整数へ展開してレビューする。例えば単一 replica に非ゼロの百分率 `maxUnavailable` を設定すると切上げで1 Podの停止を許し得るため、無停止の保証にしない

## 避ける使い方と検証観点

- drain が遅いだけで自動的に PDB を削除したり直接 DELETE に切り替えたりしない。可用性ガードを外す操作は別の判断とする
- `AlwaysAllow` を「healthy Pod も自由に停止してよい」設定として使わない
- Ready 数だけで余裕を判断しない。eviction 後の replacement が別 node に配置できる余力と、実トラフィック上の可用性を検証する
- 検証案: healthy 不足、unhealthy Running、容量不足 Pending、overlapping selector、古い observedGeneration をそれぞれ用意して、drain の待機・拒否理由・復旧条件を確認する

## 適用版・出典・未確認事項

2026-10-01 UTC に4件の公式本文を確認した。rolling docs の selector は v1.37 で、generated API reference は 2026-08-26 の v1.37 生成と表示される。ページ変更日は Disruptions が2025-05-16、PDB 設定が2024-10-21、Eviction API が2025-04-09。取得日を機能の発表日と扱わない。

PDB は v1.21 stable、`policy/v1` Eviction は v1.22+、unhealthy policy は v1.31 stable と各本文に記載。古い control plane・provider 固有の drain/autoscaler 実装への対応は未確認。実クラスタの可用性、eviction の競合、具体的 retry 間隔・メンテナンス所要時間も未検証。

Kubernetes contributors の資料は [website LICENSE](https://github.com/kubernetes/website/blob/main/LICENSE) の CC-BY-4.0 を確認。本文は帰属リンク付きの独自要約と設計案で、YAML・実装コードは転載していない。official_docs TTL 90日につき再確認期限は2026-12-30。検索 eval は検索可能性の確認であり、クラスタ挙動のテストではない。

---
{
  "id": "kubernetes-probes-endpoints",
  "title": "Kubernetes liveness/readiness/startup probes の判定・再起動と Service Endpoint 反映範囲",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes Docs v1.37 (Configure Probes 改訂 2026-06-30 / EndpointSlices 改訂 2025-06-22 / Pod Lifecycle 改訂 2026-07-27; EndpointSlices は v1.21 stable、endpoint 条件 serving/terminating/ready は v1.26 stable)",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "liveness",
    "readiness",
    "startup",
    "probes",
    "kubelet",
    "restartPolicy",
    "CrashLoopBackOff",
    "Service",
    "EndpointSlice",
    "serving",
    "terminating",
    "ready"
  ],
  "sources": [
    {
      "id": "kubernetes-configure-probes-docs",
      "url": "https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-endpoint-slices-docs",
      "url": "https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-pod-lifecycle-docs",
      "url": "https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active"
}
---

# Kubernetes liveness/readiness/startup probes の判定・再起動と Service Endpoint 反映範囲

liveness / readiness / startup の3種の probe が「コンテナ再起動」と「Service からのトラフィック有無」のどちらに影響するのかを、3ページの公式文書（上記の改訂日で確認）の記載事実と、それを組み立てる設計案に分けて記録する。対象は probe 失敗時の kubelet の動作と、readiness から EndpointSlice への反映範囲である。

## 要点（公式文書に記載された事実）

### 実行主体と再起動の範囲

- probe は kubelet が管理し、kubelet が実行する。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- liveness probe の失敗は、kubelet がコンテナを kill して再起動する。[Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- startup probe の失敗はコンテナを kill し、再起動は Pod の `restartPolicy` に従う。[Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- Pod の `restartPolicy`（Always / OnFailure / Never）は、probe 失敗後のコンテナ再起動を制御する。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- liveness / startup の Failure は CrashLoopBackOff に至り得る。kubelet は指数バックオフで再起動し、上限は 300 秒、10 分間の成功でリセットされる。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)

### readiness と startup の判定スコープ

- readiness probe の失敗は Pod を unready とし、Service 経由のトラフィックを受けない。readiness はコンテナのライフサイクル全体で実行される。[Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- startup probe の成功は liveness / readiness の開始条件になる。startup が成功するまで liveness / readiness は動作しない。[Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- startup 失敗の猶予は `failureThreshold * periodSeconds` の予算で表され、公式の例では 30 * 10 秒 = 300 秒である。[Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- HTTP probe の成功はステータスコード 200-399 である。[Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)

### Service から EndpointSlice への反映

- selector 付き Service に対して control plane は EndpointSlice を自動作成し、kube-proxy の source of truth になる。[EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)
- endpoint 条件 serving / terminating / ready は v1.26 以降 Stable である。[EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)
- ready は「serving かつ terminating でないこと」の shortcut である。例外は `publishNotReadyAddresses=true` の場合である。[EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)
- Pod の Ready 条件は serving に対応する。[EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- **「再起動」と「トラフィック遮断」を別の probe に分ける。** liveness 失敗は kill と再起動、readiness 失敗は unready 化と Service からの遮断という事実に基づく。一時的な下流依存の不調でコンテナごと再起動させたくない場合は readiness 側で遮断し、プロセス自体が回復不能な場合だけ liveness で再起動させる。
- **起動が遅いコンテナは startup probe で liveness / readiness を抑止する。** startup 成功が両者の開始条件になるため、初期化に数分かかるワークロードでは startup の予算（`failureThreshold * periodSeconds`、例 30 * 10 秒 = 300 秒）を初期化時間の実測に合わせて決める。起動時間の見積もりなしに liveness の閾値だけを緩めない。
- **readiness の失敗を EndpointSlice の ready 条件で観測する。** Pod Ready が serving に対応し、ready が serving かつ非 terminating の shortcut であるため、トラフィックから外れたかの確認は Pod の phase ではなく EndpointSlice の `ready` / `serving` / `terminating` で行う。`publishNotReadyAddresses=true` の Service だけはこの shortcut が成り立たない前提で監視を分ける。
- **CrashLoopBackOff を前提に再起動の上限を見積もる。** kubelet のバックオフ上限 300 秒と 10 分成功でのリセットを前提に、連続失敗時の再起動間隔と復旧までの時間を計算する。短い間隔での再起動を期待したアラート閾値にしない。

## 避ける使い方

- **liveness 失敗で Pod が Service から外れると考える。** 公式が Service からの遮断に結びつけているのは readiness 失敗である。liveness 失敗の直接の効果は kill と再起動である。
- **readiness 失敗でコンテナが再起動すると考える。** readiness 失敗の効果は unready 化とトラフィック遮断であり、再起動は liveness / startup の経路である。
- **startup なしで起動遅延を liveness の猶予だけで吸収する。** startup 成功が liveness / readiness の開始条件になるという仕組みを使わず、liveness の閾値を一律に大きくすると、起動後の異常検出まで遅くなる。
- **HTTP 200 以外を一律に失敗と決めつける。** 公式の成功範囲は 200-399 であり、リダイレクト帯（3xx）も成功に含まれる。リダイレクトを返すヘルスエンドポイントを失敗扱いの前提で作らない。
- **`publishNotReadyAddresses=true` の Service で ready 条件の shortcut を仮定する。** この設定は ready の shortcut の例外として公式が明記している。未-ready Pod への送信を許す Service では EndpointSlice の ready だけを見て排水完了と判断しない。
- **単一ワークロードの起動時間（例 300 秒）を全 workload の既定値として一般化する。** 公式の 30 * 10 秒 = 300 秒は例示の予算であり、普遍的な推奨値ではない。初期化時間は workload ごとに実測して決める。

## 適用版と本番での注意

- 適用版: Kubernetes Docs v1.37 の Configure Probes（改訂 2026-06-30）、EndpointSlices（改訂 2025-06-22、EndpointSlices 自体は v1.21 stable、serving / terminating / ready 条件は v1.26 stable）、Pod Lifecycle（改訂 2026-07-27）。将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で、kubernetes は技術固有 TTL の対象外のため、文書の明示期限は 2026-12-27。ただし再利用した `kubernetes-pod-lifecycle-docs` の取得日（2026-09-26）との組み合わせでは実効期限は 2026-12-25 になるため、その日までに再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: `timeoutSeconds` の適用スコープとキャンセル挙動、probe ハンドラ種別ごとの既定値や版差、readiness から EndpointSlice 反映までの遅延の定量値、kubelet 再起動時の probe 状態の引き継ぎ、カスタムスケジューラや headless Service での `publishNotReadyAddresses` 以外の例外。これらは該当ページの該当節か関連ページを別途確認する。
- 本ドキュメントの推奨方法は設計案であり、単一のベンチマークや障害事例の一般化ではない。起動猶予と再起動間隔はワークロードごとに実測して決める。

---
{
  "id": "kubernetes-pod-graceful-shutdown",
  "title": "Kubernetes Pod のグレースフルシャットダウン: terminationGracePeriodSeconds・preStop フックと kubelet の終了順序",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes Docs v1.33–v1.37 (Pod Lifecycle 改訂 2026-07-27 / Container Lifecycle Hooks 改訂 2026-02-19 / Sidecar Containers 改訂 2026-01-18; sidecar は v1.33 stable、RestartAllContainers は v1.36 Beta 既定有効)",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "pod",
    "graceful shutdown",
    "terminationGracePeriodSeconds",
    "preStop",
    "SIGTERM",
    "SIGKILL",
    "kubelet",
    "sidecar",
    "container lifecycle hook",
    "EndpointSlice"
  ],
  "sources": [
    {
      "id": "kubernetes-pod-lifecycle-docs",
      "url": "https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-container-lifecycle-hooks-docs",
      "url": "https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-sidecar-containers-docs",
      "url": "https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active"
}
---

# Kubernetes Pod のグレースフルシャットダウン: terminationGracePeriodSeconds・preStop フックと kubelet の終了順序

Pod 削除（graceful shutdown、グレースフルシャットダウン）で kubelet がどこまでを保証し、どこからをアプリ側に任せるのかを、3ページの公式文書（いずれも 2026-09-26 に確認）の記載事実と、それを組み立てる設計案に分けて記録する。対象は通常の Pod 削除フローと、そこに sidecar・強制削除・in-place 再起動という例外が絡む場合のスコープである。

## 要点（公式文書に記載された事実）

### 削除要求から SIGKILL までの順序

- Pod の削除要求を受けた kubelet は、まず各コンテナへ TERM（SIGTERM）を送る。送信リクエストは container runtime が非同期に処理するため、**複数コンテナへの停止要求の完了順序は保証されない**。送る信号はイメージの `STOPSIGNAL` を尊重し、containerd / CRI-O の既定は SIGTERM である。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- TERM 送信の前に、`terminationGracePeriodSeconds`（既定 30 秒）が 0 でなければ、定義された preStop フックを kubelet が実行する。フックが grace period 中に完了した場合はその後に TERM が送られる。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- grace period の期限が来て preStop フックがまだ実行中なら、kubelet は **2 秒の一回限りの延長**を要求する。延長中にフックが完了すれば TERM が送られ、それでも終わらなければ SIGKILL になる。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- grace period の満了でコンテナは SIGKILL される。kubelet は Pod を terminal phase（Succeeded / Failed）へ移行させ、API server から Pod を削除する。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- kubelet または runtime が終了処理の実行中に再起動した場合、再起動後は**元の grace period 全量で再試行**される。残り時間の繰り越しではない。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)

### コンテナ間の停止順序

- TERM は各コンテナへ任意のタイミングで別々に届く。順序は不定で、コンテナ間の同期を取る方法として公式は **preStop フック**か **sidecar コンテナ**を挙げる。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- sidecar コンテナ（`initContainers` + `restartPolicy: Always`）が Pod に定義されている場合に限り、kubelet は**最後の main コンテナが完全に停止するまで sidecar への TERM を送り遅らせ**、その後 sidecar を Pod spec に書いた**逆順**で shutdown させる。sidecar がない Pod ではこの延期は起きない。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/) / [Sidecar Containers](https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/)
- sidecar は開始時と同じく終了時も順序が制御される一方、他コンテナが grace period 全体を使い切った場合、sidecar は**優雅に終了する時間がないまま SIGTERM → SIGKILL** を受ける。このとき sidecar の終了コードが非 0 でも通常は無視される。main コンテナの終了が遅れると sidecar の終了も遅れ、grace period 満了時は残った全コンテナが短い grace period で同時に強制終了され得る。[Sidecar Containers](https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/)

### preStop フック（container lifecycle hook）のスコープ

- PreStop は API による削除だけでなく、liveness / startup probe の失敗、preemption、リソース不足（resource contention）などでも、コンテナの終了**直前**に呼ばれる。[Container Lifecycle Hooks](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/)
- フックは **TERM 送信前に完了する必要がある**（フック実行中の TERM 送信はしない）。一方で Pod の grace period のカウントダウンは **PreStop の実行前**に始まるため、フックの結果にかかわらずコンテナは grace period の範囲内で必ず kill される。公式の例では `terminationGracePeriodSeconds: 60` に対しフック 55 秒 + 正常停止 10 秒の計 65 秒が 60 秒を超えるため、コンテナは正常停止に達する前に kill される。**grace period は PreStop + 正常停止の合計に適用**される。[Container Lifecycle Hooks](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/)
- フックが hang した場合、Pod phase は Terminating のまま grace period の満了まで継続し、最終的に kill される。フック自体が失敗した場合もコンテナは kill される。[Container Lifecycle Hooks](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/)
- フックの配信保証は **at-least-once**。kubelet の再起動などにより二重に実行され得る。[Container Lifecycle Hooks](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/)
- ハンドラは Exec / HTTP / Sleep の3種。`httpGet`・`tcpSocket`（deprecated）・`sleep` は kubelet プロセス側で、`exec` だけがコンテナ内で実行される。[Container Lifecycle Hooks](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/)

### トラフィックと例外

- Pod が削除中でも EndpointSlice から即座に消えるわけではない。Pod は terminating として（`ready: false` の状態で）公開され続け、control plane と ReplicaSet は削除中の Pod を in-service replica として数えない。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- `kubectl delete --grace-period=0 --force` の強制削除は node 上でコンテナが本当に終了したかを待たない。node 側では強制指定後も短い grace period が付くため、API 上の削除と実終了は一致し得ない。例外として static Pods、および finalizer のない force-deleted Pod（v1.27 以降の terminal phase 移行の対象）が挙げられる。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- in-place リスタート（RestartAllContainers、v1.36 で Beta・既定有効）では、`terminationGracePeriodSeconds` も preStop も尊重・実行されない。この経路は Pod 削除のフローとは別の挙動をする。[Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- **grace period は「preStop の所要時間 + SIGTERM 受信後の正常停止時間」の合計で決める。** 公式が grace period を合計に適用すると明記しているため、片側だけを積んで 30 秒既定のまま動かすと、後半が SIGKILL で切られる。発火頻度が低い重い切断処理は preStop に、毎回のリクエスト完遂はアプリの shutdown ハンドラ側に置く。
- **コンテナ間の同期が必要なら順序制御を preStop か sidecar のどちらか一つに集約する。** TERM の到着順は不定なので、別の仕組み（sleep の散布、コンテナ内スクリプトの相互監視）で順序を仮定しない。既存の preStop による順序制御は、開始順序の保証も欲しくなった時点で sidecar（`initContainers` + `restartPolicy: Always`）へ移行できる。
- **sidecar を使うなら、main コンテナの停止時間までを sidecar の残存時間として逆算する。** sidecar の TERM は main 完了待ちで後ろにずれ、grace period 満了時は残存全コンテナが同時に短い grace period で kill される。main の停止を長く想定する Pod では全体の grace period を先に大きくする。
- **preStop を `httpGet` にすると切り分けやすい。** 実行主体が kubelet 側なので、コンテナ内のバイナリが壊れていても受信側の挙動が観測できる。ただし実行はフック完了まで TERM を待つ点と、at-least-once で二重実行され得る点を前提にする。
- **削除中の Pod は EndpointSlice に `ready: false` として残ることを前提に排水（drain）の待機時間と監視を組む。** レプリカ数の減算は control plane / ReplicaSet 側で行われるため、API から Pod が消えることとトラフィックが切れるタイミングは同じではない。
- **終了処理は実測で検証する。** TERM 受信時刻、preStop の完了時刻、SIGKILL の時刻をログに残し、想定した grace period の内訳（フック + 停止）と一致するかをデプロイのたびに確認する。kubelet 再起動時に grace period 全量で再試行される点も、リトライを含めた合計時間の想定に織り込む。

## 避ける使い方

- **コンテナ間の停止順序が一定だと考えて設計する。** 公式は TERM の到着順序が保証されないと明記している。順序が必要なら preStop か sidecar を使う。
- **preStop を長くしても grace period は自動では延びない前提にする。** カウントダウンはフック実行前から始まり、期限超過で SIGKILL される。公式の 60 秒の例（フック 55 秒 + 停止 10 秒）が示す通り、合計が grace period を超えると正常停止に達せずに切られる。
- **preStop の成否で grace period が変わると考える。** フックが失敗しても hang しても、コンテナは grace period の範囲で kill される。フックを「失敗したら停止を保留する手段」として使わない。
- **preStop に非冪等な処理（課金・外部への確定送信など）をそのまま置く。** 配信は at-least-once で kubelet 再起動時に二重実行され得る。
- **`--grace-period=0 --force` を「確実に即時停止する安全策」として常用する。** node 上の実終了を待たず、API 上の削除と実終了がずれる。static Pods や finalizer のない force-deleted Pod は terminal phase の扱いも例外に入る。
- **in-place リスタートで preStop や grace period が走ると期待する。** RestartAllContainers の経路では両者を尊重しないと公式が明記している。クリーンアップが必要なコンテナはこの経路でリスタートさせない前提で組む。
- **sidecar の終了コード 0 を検証の必須条件にする。** grace period 耗尽時は sidecar に終了時間なしで SIGTERM → SIGKILL が渡り、非 0 終了は通常無視される。

## 適用版と本番での注意

- 適用版: Kubernetes Docs の Pod Lifecycle（版選択 v1.33–v1.37、改訂 2026-07-27）、Container Lifecycle Hooks（改訂 2026-02-19）、Sidecar Containers（改訂 2026-01-18）。sidecar は v1.28 で利用可能、feature gate は v1.29 以降既定有効、v1.33 で stable。RestartAllContainers は v1.36 で Beta・既定有効。将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で、kubernetes は技術固有 TTL の対象外のため、2026-12-25 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: kubelet が要求する 2 秒延長の内部発行条件の詳細、`STOPSIGNAL` を変更したイメージでの個別 runtime の挙動差、`kubectl drain` と graceful shutdown の相互作用、preStop 実行中の Pod 側からの API 削除要求を含むエッジケースの詳細、sidecar 以外の依存コンテナ（通常の init container は開始時のみ）の終了時挙動。これらは該当ページの該当節か関連ページを別途確認する。
- 本ドキュメントの推奨方法は設計案であり、単一のベンチマークや障害事例の一般化ではない。必要な停止時間（フック + 正常停止）はワークロードごとに実測して決める。

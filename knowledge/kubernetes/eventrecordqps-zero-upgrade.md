---
{
  "id": "kubernetes-eventrecordqps-zero-upgrade",
  "title": "Kubernetes 1.37 eventRecordQPS: 明示ゼロの無制限化と5・50 QPSの移行判断",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes v1.36.0 → v1.37.0 / KubeletConfiguration v1beta1; fixed upstream commits ecf6decece6a6de25a57aad9ba90b6ce580f6f78 and f54c212e3a2f75d674b717a9b29052b20b60aefc; verified 2026-10-02, runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "eventRecordQPS",
    "eventBurst",
    "client-go",
    "configz",
    "upgrade",
    "rate-limit"
  ],
  "sources": [
    {
      "id": "kubernetes-event-qps-release-v137-20261002",
      "url": "https://raw.githubusercontent.com/kubernetes/kubernetes/master/CHANGELOG/CHANGELOG-1.37.md",
      "type": "release_notes"
    },
    {
      "id": "kubernetes-event-qps-api-v137-20261002",
      "url": "https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-event-qps-config-file-20261002",
      "url": "https://kubernetes.io/docs/tasks/administer-cluster/kubelet-config-file/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-event-qps-old-client-v1360-20261002",
      "url": "https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/cmd/kubelet/app/server.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "kubernetes-event-qps-rest-default-v1360-20261002",
      "url": "https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/staging/src/k8s.io/client-go/rest/config.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "kubernetes-event-qps-new-client-v1370-20261002",
      "url": "https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/cmd/kubelet/app/server.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "kubernetes-event-qps-recording-v1370-20261002",
      "url": "https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/staging/src/k8s.io/client-go/tools/record/event.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Kubernetes 1.37 eventRecordQPS の移行判断

## 問いと結論

既存 kubelet 設定の `eventRecordQPS: 0` を残したまま v1.36 から v1.37 に更新すると、Event の送信制限は維持されるか。答えは「維持されない」。v1.37.0 は明示ゼロを event client の rate limit 無効として扱う。設定ファイルが同じでも、送信量が変わる可能性がある。

重要なのは **未指定の既定50、旧版の明示ゼロが通る5 QPS fallback、新版の明示ゼロによる無制限** を区別すること。本稿はこの移行と観測上の落とし穴に限定する。全クライアントの rate limit、API server の容量設計、Event の永続的監査基盤を網羅するものではない。

[公式 v1.37.0 Urgent Upgrade Notes](https://raw.githubusercontent.com/kubernetes/kubernetes/master/CHANGELOG/CHANGELOG-1.37.md) は、ゼロの扱いを文書どおり無制限へ修正し、従来の制限に依存する利用者は非ゼロ値を明示するよう案内している。`50` はそこで挙げられた例であり、すべての旧明示ゼロ設定で実際に使われていた値を意味しない。

## 確認した契約と実装

### 1. API の公開契約

`KubeletConfiguration v1beta1` の `eventRecordQPS` は Event 作成の毎秒上限を表し、既定値は `50`、`0` は無制限、負数は不可。`eventBurst` の既定値は `100` で、QPS が正の場合の一時的 burst を扱う。ゼロQPS時に burst だけで制限を復活させる設定ではない。[生成 API reference](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/)

公開設定で `-1` を指定してはいけない。後述の `-1` は kubelet が REST client 用に内部変換する値であり、kubelet config の受理値とは異なる。

### 2. 未指定と明示ゼロを defaulting で混同しない

v1.36.0 と v1.37.0 の `SetDefaults_KubeletConfiguration` は、`EventRecordQPS` が nil の場合にだけ `50` を補う。明示した整数ゼロはこの分岐を通らない。したがって、値を省略して通常の既定に戻すことと、ゼロを残して更新することは別の変更になる。[v1.36.0 defaulting](https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/pkg/kubelet/apis/config/v1beta1/defaults.go)、[v1.37.0 defaulting](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/kubelet/apis/config/v1beta1/defaults.go)

### 3. なぜ旧版のゼロは5で、新版のゼロは無制限か

以下は指定commitの読解から得た実装上の結論であり、実クラスタの実測ではない。

- v1.36.0 の kubelet は通常API clientとは別の EventClient を作り、`eventRecordQPS` を REST config の QPS へそのまま渡す。通常 client の生成は shallow copy に rate limiter を設定するため、その生成によって元の config に同じ limiter が注入されるわけではない。[kubelet server](https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/cmd/kubelet/app/server.go)、[clientset](https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/staging/src/k8s.io/client-go/kubernetes/clientset.go)
- EventClient の typed core client はその config で REST client を生成する。`RateLimiter` が未指定で QPS がゼロなら `DefaultQPS = 5` に補い、token bucket を作る。これは kubelet の未指定既定50とは別階層の fallback。[typed core client](https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/staging/src/k8s.io/client-go/kubernetes/typed/core/v1/core_client.go)、[REST config](https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/staging/src/k8s.io/client-go/rest/config.go)
- v1.37.0 は EventClient を作る直前に QPS `0` を内部値 `-1` に変換する。REST client は正の QPS にだけ token bucket を生成するので、この通常生成経路では当該 client-side limiter がなくなる。[新 kubelet server](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/cmd/kubelet/app/server.go)、[新 REST config](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/staging/src/k8s.io/client-go/rest/config.go)

| 設定の入力 | v1.36.0 の通常生成経路 | v1.37.0 の通常生成経路 | 移行判断 |
|---|---|---|---|
| 未指定、上書きなし | kubelet既定50 | kubelet既定50 | 既定QPSの意味は維持 |
| 明示 `0` | client-go fallback 5 | EventClientの当該limiterなし | 同じ設定でも制限解除 |
| 明示 `5` | 5 | 5 | 旧explicit-zeroの送信token budgetを再現する候補 |
| 明示 `50` | 50 | 50 | 通常のkubelet既定にそろえる候補 |
| 正の任意値 | 指定値 | 指定値 | 容量・観測要件に合わせて選ぶ |

この表は QPS の refill rate を示す。短時間の送信数は `eventBurst` も関わるため、毎秒の計測値が常に整数5や50になる保証ではない。また HTTP request の制限であり、「保存される異なるEvent objectの数」と単純に等置しない。

### 4. 無制限は lossless delivery ではない

v1.37.0 の `makeEventRecorder` は既定の broadcaster と correlator を使う。`event.go` は correlation結果による skip、既存Eventの PATCH、必要時の CREATE、再試行上限を持つ。また queue が満杯の際に drop する broadcaster を生成する。[record/event.go](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/staging/src/k8s.io/client-go/tools/record/event.go)

`events_cache.go` の default correlator には別の spam filter と aggregation がある。source/object等で作るspam keyごとに burst 25、refill 1/300 QPS を既定とする。この値は `eventBurst: 100` と別物で、eventRecordQPS の変更では解除されない。同一系統のEventを繰り返すだけの負荷試験は、このフィルターを測っている可能性がある。[correlator と spam filter](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/staging/src/k8s.io/client-go/tools/record/events_cache.go)

したがってゼロは、API server の制約、通信失敗、集約、drop をなくす設定ではない。重要な事実を確実に保存する要件を「Eventを無制限にした」だけで満たしたと判定しない。

## 実務での判断手順（独自の設計案）

以下は一次資料から組み立てた移行・検証案であり、実行済み手順や全環境共通の推奨値ではない。

1. node pool ごとに kubelet の正確な版、起動引数、`--config` のファイル、`--config-dir` の drop-in を記録する。明示ゼロ、未指定、正の指定値を別グループにする
2. ファイルのgrepだけで終えず、認可済みの node proxy / `configz` で最終実効設定を確認する。公式手順では通常設定→名前順のdrop-in→command line（feature gate以外）の順で上書きされる。後から渡された `--event-qps` がファイル変更を覆していないか確認する。[設定手順とconfigz](https://kubernetes.io/docs/tasks/administer-cluster/kubelet-config-file/)
3. 目標を先に決める。通常既定へそろえるなら50を候補とし、旧explicit-zeroのREST client制限を保ちたいなら5と以前のburstを候補にする。どちらも「必ず同じEvent数になる」とは約束しない。無制限を望む場合は、その理由とAPI server側の許容量を別途確認する
4. 既存設定の当該フィールドだけをレビューして変更する。一般的なファイル全体で置き換えたり、無関係な認証・eviction設定まで移植したりしない。適用方法とkubelet再起動・node交換の可用性手順は配布物の管理方式に合わせる
5. canaryで、同じ負荷条件の更新前後を比較する。最終QPS/burst、kubelet版、event関連のCREATE/PATCH request rate・失敗・待ち時間、API serverとetcdの遅延を観測候補にする。Event objectの単純な件数は集約や更新の影響を受けるため、単独の合否指標にしない
6. rollback時も設定値と実装版を対にして確認する。v1.36.0へ戻してゼロを残すと、当該通常経路は再び5へfallbackする。正の値を明示しておけば、このゼロ解釈差を避けられる。制限を戻しても既にdropしたEventが復元されるとは扱わない

最終設定で `eventRecordQPS: 0` と読めても、それだけではclient内部のtoken bucketの有無は分からない。`configz` はkubelet設定を表示し、v1.37の内部REST値 `-1` を利用者設定へ書き戻すものではない。版と送信観測を合わせる必要がある。

## 検証観点と避ける判断

- 未指定、0、5、50を同一burstで比較する。burstを変える試験は分け、定常状態と短いburstを別に見る
- 同じobjectの反復Eventと、多様なobjectに関するEventを分ける。テスト負荷がspam filterに吸収される場合を含める
- ファイル・drop-in・起動flagが競合する場合、意図した実効設定になったことを確認する
- `kubeAPIQPS` だけを変えてEventClientのQPSを制御できると仮定しない。上記経路ではeventRecordQPSで上書きされる
- 公式release noteの例示50を「旧ゼロの厳密な再現値」と読まない。5も全clusterの安全値とは断定しない
- 無制限にすれば全Eventを保存できる、または大規模clusterでも送信量は変わらない、と推測しない

## 適用版・provenance・未確認事項

一次資料は2026-10-02 UTCに本文を取得した。v1.37.0のrelease表示・tag日付は2026-08-26、比較対象v1.36.0のtag日付は2026-04-22。API referenceはv1.37生成版・更新表示2026-08-26、設定手順ページの更新表示は2026-07-28。日付は取得日や全機能の導入日と混同しない。

実装はv1.36.0の `ecf6decece6a6de25a57aad9ba90b6ce580f6f78`、v1.37.0の `f54c212e3a2f75d674b717a9b29052b20b60aefc` に固定した。Kubernetes Authorsの[repository LICENSE](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/LICENSE)と確認したGoファイルのheaderはApache-2.0。Kubernetes contributorsの[website LICENSE](https://github.com/kubernetes/website/blob/main/LICENSE)はCC-BY-4.0。本文は帰属リンク付きの独自要約・読解であり、実装コードや資料の設定例は転載していない。

PR #117119と一部rawページはweb取得が失敗したため、PRの議論を根拠にせず、開けた公式文書・changelog・releaseページと固定commitのファイル本文で確認した。repoファイルの一部はGitHub connectorでも本文を取得した。release notesのTTL 30日に合わせ再確認期限を2026-11-01とする。

実クラスタ、負荷試験、バックポート・各patch版、マネージドKubernetes固有の設定変換、独自に注入されたRateLimiter/Recorder、可観測性製品の具体的metric名は未検証。表は指定したupstream通常生成経路の比較であり、ベンダー配布物に無条件で一般化しない。検索eval成功は実行時挙動や無損失を証明しない。

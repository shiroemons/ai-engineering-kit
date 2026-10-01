---
{
  "id": "kubernetes-memoryqos-beta-defaults-rollback",
  "title": "Kubernetes 1.37 MemoryQoS: beta既定値変更・明示的throttling・rollbackの残存設定",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes v1.37 / KubeletConfiguration v1beta1; MemoryQoS beta; Linux cgroup v2; 2026-10-01確認、実クラスタ未検証",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "MemoryQoS",
    "memoryThrottlingFactor",
    "memoryReservationPolicy",
    "TieredReservation",
    "cgroup-v2",
    "rollback"
  ],
  "sources": [
    {
      "id": "kubernetes-memoryqos-beta-blog-20261001",
      "url": "https://kubernetes.io/blog/2026/09/14/kubernetes-v1-37-memory-qos-graduates-to-beta/",
      "type": "maintainer_article"
    },
    {
      "id": "kubernetes-memoryqos-pod-qos-20261001",
      "url": "https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-memoryqos-kubelet-config-v137-20261001",
      "url": "https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/",
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

# Kubernetes 1.37 MemoryQoS の移行判断

## 問いと範囲

v1.37 の MemoryQoS が既定で有効になったとき、既存 workload に自動的なメモリ保護が加わるのか。以前の alpha 設定を維持・解除するには何を確認するか。既存の Docker メモリ制限文書とは異なり、本稿は kubelet の版移行と cgroup 設定の適用時点を扱う。

## 確認した契約

### feature gate と実際の動作を分ける

MemoryQoS は v1.37 で beta・既定有効となった。ただし新しい既定設定では throttling も reservation も開始しない。以前の alpha の `memoryThrottlingFactor` 既定 `0.9` が `null` に変わったためである。既存ファイルに明示した係数は維持されるが、省略して旧既定値に依存していた利用者は、throttling を続けるなら係数を明示する必要がある。[2026-09-14 の移行説明](https://kubernetes.io/blog/2026/09/14/kubernetes-v1-37-memory-qos-graduates-to-beta/)

`KubeletConfiguration` API では係数の既定を `nil` と表記し、未設定なら `memory.high` を設定しない。係数を小さくすると reclaim 圧力が強まる。`memoryReservationPolicy` の既定は `None`。`TieredReservation` は requests を基に Guaranteed の `memory.min`、Burstable の `memory.low` を設定する。両設定は別々に選択できる。[v1.37 生成 API reference](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/)

### throttling と reservation の境界

- Linux cgroup v2 が必要。kernel 5.9 以上が推奨され、古い kernel には throttling の livelock リスクがある
- 係数は `0 < factor <= 1`。Burstable の `memory.high` は `requests + factor * (limits - requests)`。limit がなければ node allocatable を使う。BestEffort は request をゼロとして計算し、Guaranteed には設定しない
- TieredReservation の `memory.min` は強い reclaim 保護、`memory.low` は極端な圧力下では回収され得る保護。BestEffort は保護対象外
- 保護は ancestor cgroup にも依存する。Guaranteed では `memory.min = memory.max` となり、大きな page cache を保持する workload は OOM の危険がある

根拠: [Pod QoS / Memory QoS](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#memory-qos-with-cgroup-v2)。これはメモリ不足を消す保証ではなく、reclaim の扱いを変える機能である。

### rollback は即座の全解除ではない

gate を false にする場合、予約ポリシーを未設定または None に戻し kubelet を再起動する。root kubepods と Burstable 階層の保護がリセットされ、子に値が残っても ancestor の保護なしでは有効にならない。一方、既存 container の `memory.high` はその container が restart / in-place resize されるまで残り得る。[解除の適用時点](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#disabling-or-rolling-back-memory-qos)

移行記事には、gate 無効時は旧既定の `0.9` 以外の係数や TieredReservation が設定されていると kubelet が設定を拒否するとある。無効化する際は gate だけでなく関連フィールドを外す／整合させる。また予約ポリシーは node 単位で、Pod ごとの opt-out はない。[設定互換性と node-wide 制限](https://kubernetes.io/blog/2026/09/14/kubernetes-v1-37-memory-qos-graduates-to-beta/)

## 実務での判断手順（独自の設計案）

以下は資料から組み立てたレビュー・検証案であり、公式の移行コマンドや実測済み手順ではない。

1. node pool ごとに kubelet 版、kernel、cgroup 版、配布物の実効設定を記録する。設定管理のテンプレートに値がなくても、配布物が係数を注入していないか確認する
2. 更新前を「gate 無効」「alpha 有効・係数省略」「alpha 有効・係数明示」に分類する。gate の既定変更だけを見て挙動が同じと判定しない
3. throttling 継続と reservation 導入を別の変更としてレビューする。両方を同時に変えると遅延・OOM の原因を切り分けにくい
4. canary node で新規 container と更新前から動く container を比較する。設定ファイルだけでなく、container と ancestor の実際の memory.high / memory.min / memory.low を確認する
5. latency、reclaim、OOM、page cache を含むメモリ使用、隣接 workload の影響を観測する。係数を下げれば安全性だけが増すとは仮定しない
6. rollback は kubelet 再起動だけの試験と、container restart / resize 後の試験を分ける。残存 memory.high を確認せず「解除完了」としない。restart が必要なら別途可用性・drain 計画を立てる

## 避ける判断と検証観点

- beta / 既定有効という見出しを「全 Pod に自動的に reservation が付く」と読む
- None と TieredReservation、未設定係数と明示 0.9 を同じ設定として扱う
- Guaranteed の名前から page cache を含む OOM 回避を保証する
- node 全体の予約方針を単一 Pod の最適化だけで決める

検証案: Burstable の limit 有無、BestEffort、Guaranteed の page-cache-heavy workload、旧設定省略・明示、新規・継続稼働 container の組合せで期待 cgroup 値と挙動を確認する。負荷生成や設定変更自体は本調査では実施していない。

## 適用版・出典・未確認事項

3つの一次資料を2026-10-01 UTCに本文まで確認した。移行記事は Qi Wang / Sohan Kunkerkar（Red Hat）、公開日2026-09-14、ページ更新表示2026-09-11。Pod QoS は selector v1.37・更新表示2026-07-28、API reference は v1.37 用に2026-08-26生成。取得日をリリース日として扱わない。

Kubernetes contributors の [website LICENSE](https://github.com/kubernetes/website/blob/main/LICENSE) で CC-BY-4.0 を確認。帰属リンクを付けた独自要約であり、実装コード・YAML例の転載はない。公式文書と maintainer article の TTL は90日なので再確認期限は2026-12-30。

provider 固有の設定変換、実 runtime による再適用時点、全 patch 版での差、既存 workload の latency/OOM、kernel backport の影響は未検証。検索 eval の成功はクラスタ挙動の証明ではない。

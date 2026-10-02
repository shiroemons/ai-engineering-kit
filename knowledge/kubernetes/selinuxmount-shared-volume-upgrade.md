---
{
  "id": "kubernetes-selinuxmount-shared-volume-upgrade",
  "title": "Kubernetes 1.37 SELinuxMount: 共有volumeの起動競合とRecursive opt-outの移行判断",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes v1.36→v1.37 / Pod v1; SELinuxMount GA・既定有効はv1.37、SELinuxChangePolicy GAはv1.36; Linux SELinux + 対応CSI volume; 2026-10-02確認、実クラスタ未検証",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "SELinuxMount",
    "seLinuxChangePolicy",
    "Recursive",
    "MountOption",
    "CSI",
    "shared-volume",
    "upgrade"
  ],
  "sources": [
    {
      "id": "kubernetes-selinux-security-context-20261002",
      "url": "https://kubernetes.io/docs/tasks/configure-pod-container/security-context/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-selinux-pod-api-v137-20261002",
      "url": "https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-selinux-feature-gates-20261002",
      "url": "https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-selinux-upgrade-blog-20261002",
      "url": "https://kubernetes.io/blog/2026/04/22/breaking-changes-in-selinux-volume-labeling/",
      "type": "maintainer_article"
    },
    {
      "id": "kubernetes-v137-selinux-release-20261002",
      "url": "https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/",
      "type": "release_notes"
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

# Kubernetes 1.37 SELinuxMount と共有volumeの移行

## 問いと対象

v1.36で動いていたPodが、v1.37更新後に同じvolumeを使おうとしてContainerCreatingに留まるのはなぜか。起動高速化を保つ範囲と、Recursiveへ戻す単位をどう決めるか。本稿はLinuxでSELinuxを使い、対応CSI volumeを共有するworkloadの移行に絞る。SELinuxが無効・非対応のnodeにはこの変更による影響はないが、無効化を対処策として推奨するものではない。

## 確認した契約

### 1. 「SELinux機能がGA」を一括りにしない

[Feature Gatesの版別表](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/)は次のように区別している。

- SELinuxMountReadWriteOncePod: ReadWriteOncePod volume向けの最適化。v1.28から既定true、v1.36でGA
- SELinuxChangePolicy: Podの適用方式を選ぶfieldと関連controller。v1.33–v1.35は既定trueのbeta、v1.36でGA
- SELinuxMount: 最適化を他の適格volumeへ広げるgate。v1.33–v1.36は既定falseのbeta、v1.37でGA・既定true

したがって今回の移行境界は、policy fieldの追加そのものではなく、従来SELinuxMountが無効だった環境で対象volumeのmount方式が変わることである。すでに明示的に有効化していた環境とは区別する。

[v1.37 release announcement](https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/)もSELinuxMountの既定有効化を告知している。ただし同段落はSELinuxChangePolicyもv1.37でGAになったようにまとめ、policyの位置を「.spec.seLinuxChangePolicy」と略記している。本稿では版別表と生成Pod APIを優先し、GA時期と正式なfield pathを上記・下記のように区別する。概要の見出しや省略形からmanifestを作らない。

### 2. CSI driver・Pod・volumeの条件が揃って初めて変わる

[Security Contextの詳細説明](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#efficient-selinux-volume-relabeling)によると、CSI経路でmount最適化を使うには、PodがPVCを利用し、policyが未設定またはMountOptionであり、Podまたは当該PVCを使う全containerにseLinuxOptionsが設定され、CSIDriverがspec.seLinuxMount: trueを宣言している必要がある。さらに[移行記事の条件4](https://kubernetes.io/blog/2026/04/22/breaking-changes-in-selinux-volume-labeling/#what-kubernetes-is-improving)は、少なくともseLinuxOptions.levelが必要と明記する。typeだけの指定や空のseLinuxOptionsではこの条件を満たさない。levelをKubernetesが把握できなければ、container runtimeがmount後にランダムなlevelを割り当て、再帰的relabelを行う。適格なvolumeではmountのcontextによりlabelを適用する。条件外のvolumeは従来の再帰的relabelを使う。secret・configMap・projectedを含め全volumeが一斉に切り替わるわけではない。

[Pod v1 APIのPodSecurityContext](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#podsecuritycontext)での正式な設定位置はspec.securityContext.seLinuxChangePolicy。値はMountOptionまたはRecursiveで、v1.37の未設定時はMountOptionを使う。Recursiveはcontainer runtimeによる再帰的relabelであり、大きなvolumeでは時間がかかり得る。同一volumeを使うPod間でpolicyを揃える必要があり、混在もContainerCreatingの原因になり得る。Windows向けに設定できるfieldではない。

### 3. subPath分割はmount contextを分けない

[移行記事](https://kubernetes.io/blog/2026/04/22/breaking-changes-in-selinux-volume-labeling/)は、従来の再帰方式では動いた二例を挙げる。異なるSELinux labelのPodが別々のsubPathを使う場合と、privileged Podとunprivileged Podが共有する場合である。mount方式ではこれらの共有が成立せず、一方がContainerCreatingに留まる。

[詳細文書](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#efficient-selinux-volume-relabeling)が示す競合境界は、同じ適格volumeを同じnodeで同時に使うPod群のlabel不一致。異なるlabelで使うPodが残る間、後続Podは起動できない。単なる起動待ち時間の延長では解消しない条件である。

Recursiveはこの最適化からのopt-outであり、SELinuxそのものを外す設定ではない。互換性維持には共有するPod群のpolicyも揃える。labelを一律に同じ値へ変更する案は、起動問題だけで決めない。詳細文書は同じMCS labelのPod間でvolumeアクセスが可能になり、Pod間保護には一意のMCS labelが必要と注意している。

### 4. 予兆と実際の起動失敗を別々に観測する

[移行記事の監査手順](https://kubernetes.io/blog/2026/04/22/breaking-changes-in-selinux-volume-labeling/#suggested-upgrade-path)は、次の三つを使い分ける。

- selinux_warning_controller_selinux_volume_conflict: 競合するPodを特定する。現在別nodeにいても、将来同居し得る組合せを警告する
- volume_manager_selinux_volume_context_mismatch_warnings_total: SELinuxMount無効時に起動できたが、有効化後には競合するPodについてkubeletが記録する。Pod名labelはない
- volume_manager_selinux_volume_context_mismatch_errors_total: SELinuxMount有効時にlabel競合で起動できなかったPodについて記録する

[SELinuxWarningController](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#selinuxwarningcontroller)は既定無効。kube-controller-managerの--controllers=*,selinux-warning-controllerなどで有効化でき、SELinuxChangePolicyを必要とする。feature gateが既定trueでもcontrollerの稼働を仮定しない。移行記事は、Eventで他namespaceのPod名が隠されてもmetricにはnamespace名が現れることを注意点として挙げている。監視の公開先も確認する。

## 実務での判断手順（独自の設計案）

以下は資料を組み合わせたレビュー案であり、公式手順の転載や実測済みrunbookではない。

1. node poolごとにKubernetes版、SELinuxの状態、SELinuxMountの実効設定、CSI driverとCSIDriver宣言を棚卸しする。gateの版だけで影響あり／なしを決めず、Podへのadmission後の設定も確認する
2. volumeを単位に利用Pod・その生成元・label・policy・subPath・privileged利用を並べる。今の配置で同居していない組合せも候補に入れ、再配置やrolling updateで同時起動し得るかを確認する
3. v1.36側で警告controllerの稼働とmetricsの取得を確かめ、潜在競合とkubeletの実際の警告を照合する。metricが見えない状態をゼロ件と扱わず、観測期間に起動しなかったworkloadも残す
4. 対象ごとに共有方法を見直すか、共有Pod群の生成元templateをRecursiveに統一するかを選ぶ。一方のPodだけの例外ではpolicy混在が残る。互換性のための例外は所有者・理由・見直し条件を記録する
5. canary環境で同じnode上の同時起動と順番を入れ替えた再起動を試す。ContainerCreating時にはvolume conflict Eventを確認し、一般的なimage pullやstorage接続の失敗と切り分ける
6. opt-out後はPod起動成功だけでなく、大量ファイルの再帰処理時間、必要な共有アクセス、禁止したい別Podからのアクセスを確認する。障害対処でlabel統一や権限拡大を先に行わない

## 検証すべき組合せと落とし穴

独自の試験案として、少なくとも「同一label・同一policy」「異なるlabel・別subPath」「privileged / unprivileged共有」「Recursive / MountOption混在」を、実際のCSI driverで確認する。共有Pod群をRecursiveへ統一した場合と、driverがseLinuxMountを宣言しない場合も別に扱う。期待する起動可否だけでなく、Event、該当metrics、起動時間、データアクセスを記録する。

全PodにMountOptionを明示しただけ、ReadWriteOncePod向けGAの確認だけ、現在の配置で競合が出なかっただけでは、v1.37への移行確認として足りない。逆に、他の起動失敗をSELinuxMountだけの問題と断定することも避ける。

## 適用版・出典・未確認事項

2026-10-02 UTCに5つの一次資料の本文を確認した。移行記事はJan Šafránek（Red Hat）とSwathi Rao（Independent）、公開・更新2026-04-22で、当時のv1.37は予定として書かれている。今回の既定変更はv1.37を示す詳細docs、gate表、2026-08-26公開・2026-09-02更新のrelease announcementで確認した。Security Contextの更新表示は2026-07-28、Pod APIはv1.37用として2026-08-26生成。Feature Gatesのページ更新表示は2026-01-27で、表の適用版とページ更新日を混同しない。

release announcementはcluster-wide opt-outについてv1.38でlockedになる予定にも言及するが、将来版の実効動作は本稿で保証しない。将来版へ進める際は再確認する。in-tree pluginの網羅的対応表は扱わず、CSI経路を対象とした。distribution固有のadmission、driver/kernel実装、metricsの実環境での露出、既存mountからの切替時点、全patch版差分、実クラスタの起動・性能・アクセス分離は未検証。検索evalは文書の発見性を確認するもので、これらの動作を保証しない。

資料はKubernetes contributorsの[website LICENSE](https://github.com/kubernetes/website/blob/main/LICENSE)でCC-BY-4.0を確認した。帰属リンクを付けた独自要約と独自の検証案で、実装コードやYAML例の転載はない。release_notesのTTL 30日を含むため、再確認期限は2026-11-01とした。

---
{
  "id": "kubernetes-emptydir-mode-fsgroup-enforcement-boundary",
  "title": "Kubernetes 1.37 emptyDir.mode: gate・fsGroup・既存volumeの実効権限の境界",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes v1.37 Docs / Pod v1; implementation v1.37.0 @ f54c212e3a2f75d674b717a9b29052b20b60aefc (2026-08-26); Linux default-medium emptyDir only; verified 2026-10-24 UTC; cluster/runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "emptyDir.mode",
    "EmptyDirVolumeMode",
    "fsGroup",
    "sticky-bit",
    "setgid",
    "nil",
    "explicit-zero",
    "ready-volume",
    "version-skew"
  ],
  "sources": [
    {
      "id": "kubernetes-emptydir-mode-task-20261024",
      "url": "https://kubernetes.io/docs/tasks/configure-pod-container/configure-emptydir-volume-mode/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-emptydir-mode-api-20261024",
      "url": "https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-emptydir-mode-volumes-20261024",
      "url": "https://kubernetes.io/docs/concepts/storage/volumes/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-emptydir-mode-hardening-20261024",
      "url": "https://kubernetes.io/blog/2026/09/16/kubernetes-v1-37-hardening-container-storage/",
      "type": "maintainer_article"
    },
    {
      "id": "kubernetes-emptydir-mode-volume-core-1370-20261024",
      "url": "https://github.com/kubernetes/kubernetes/tree/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/volume",
      "type": "github_repository_analysis"
    },
    {
      "id": "kubernetes-emptydir-mode-api-core-1370-20261024",
      "url": "https://github.com/kubernetes/kubernetes/tree/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/api/pod",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-unlink-sticky-owner-20261024",
      "url": "https://man7.org/linux/man-pages/man2/unlink.2.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2027-01-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Kubernetes emptyDir.mode と実効権限の確認

## 問いと結論

shared scratch directory に mode を指定し、Pod 作成が成功したのに、期待した権限でないのはなぜか。kubelet の gate を有効化すれば、すでにある volume まで修復されたと判断できるか。

`emptyDir.mode` は volume directory の初期権限を選ぶ設定であり、API受理・node上の適用・継続的な権限強制は別である。v1.37では alpha・既定無効なので、API server と kubelet の両方が `EmptyDirVolumeMode` を有効にする必要がある。さらに fsGroup による後処理や、すでに準備済みの volume の早期returnを考慮する。[公式task guide](https://kubernetes.io/docs/tasks/configure-pod-container/configure-emptydir-volume-mode/)

本稿の実装分析は Linux の default-medium（medium が空文字または省略）の emptyDir に限定する。既存の [Docker umask](../docker/umask-api-exec-initial-permission-boundary.md) は process の生成mask、[SELinuxMount](selinuxmount-shared-volume-upgrade.md) は共有volumeのlabel競合を扱う。本稿は kubelet が作る一つの directory の mode・group ownership・適用時点という別の問いである。

## 確認した契約と固定実装

### 1. 未指定と明示zeroを同じにしない

正式なfieldは `spec.volumes[*].emptyDir.mode`。Pod v1 APIは整数を受け付け、8進数で0000から01777まで、未指定は0777とする。sticky bitは指定できるが、setgidやsetuidをこのfieldで指定する範囲ではない。Windowsでは効果がない。[EmptyDirVolumeSource API](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#emptydirvolumesource)

v1.37.0の `newMounterInternal` は gate が有効なときだけ `*int32` の Mode を取り込む。`setupDir` は nil でなければ値をchmodするため、明示zeroは0000であり、nilの0777 fallbackとは異なる。設定生成でzeroを「空」と判定して省略しない。以下の整数対応はAPIの値域から算出した例である。

| 意図する8進mode | JSONで送る整数 | 意味 |
|---|---:|---|
| 0000 | 0 | 初期の通常permission bitをすべて落とす |
| 0750 | 488 | owner rwx・group r-x・otherなし |
| 01777 | 1023 | rwxにsticky bitを加える |
| 未指定 | fieldなし | 既定0777を使う |

JSONに0750という8進リテラルは書けない。JSON整数750は0750とは別値なので、templateの見た目だけでなくserialize後の値を検査する。Modeの値が正しくてもfsGroup後の実効modeが同じとは限らない。[固定emptyDir実装](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/volume/emptydir/empty_dir.go)

### 2. APIに残ったfieldとkubeletが使うfieldを別に確かめる

API側の `DropDisabledPodFields` は gate が無効で、old Podにもmode利用がなければ、新しいModeをnilへ落とす。逆にold Podがすでに利用していれば、この除去は行わない。これは既存利用を保持する互換処理であり、「gateを無効にすれば保存fieldが必ず消える」という仕様ではない。`TestDropEmptyDirVolumeMode` はgateとold/new利用有無を組み合わせている。[固定API処理とtest](https://github.com/kubernetes/kubernetes/tree/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/api/pod)

API serverで有効でも対象kubeletで無効なら、固定volume実装はModeを取り込まず既定0777へ進む。`TestEmptyDirVolumeModeFeatureGateDisabled` にもこの期待値がある。導入記事はこのskewを、Podが拒否されるのではなくmodeが無視されるケースとして明示する。[上流volume test](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/volume/emptydir/empty_dir_test.go) / [2026-09-16導入記事](https://kubernetes.io/blog/2026/09/16/kubernetes-v1-37-hardening-container-storage/)

したがって「kubectl apply成功」「保存Podにmodeあり」「PodがRunning」は別々の観測で、最後の実効権限の証拠にはならない。mode自体はkubelet側で処理され、導入記事がいうcontainer runtimeの追加対応は不要である。ただしこれを全storage条件や全OSでの成功保証に拡張しない。

### 3. fsGroupはmodeの上限を守る設定ではない

Volumes文書はfsGroupがmodeのgroup permissionを上書きし得ると注意する。固定実装ではdirectory作成・mode設定の後に `NewVolumeOwnership` を呼ぶ。Linuxの `changeFilePermission` はgroupを変更し、既存modeにpermission maskをORする。書込み可能なemptyDir directoryではowner/groupのrwxとsetgidが加わる。既存のother bitやsticky bitを削る処理ではない。[Volumes](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir) / [固定Linux ownership処理](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/volume/volume_linux.go)

通常のLinux permission操作がすべて成功し、ACL等を別途考慮する前提では、固定処理から次の数値を導ける。これは実測結果ではなくコードのOR演算による期待値である。

| 初期mode | fsGroupなし | fsGroupありの計算値 |
|---|---|---|
| 0000 | 0000 | 02770 |
| 0750 | 0750 | 02770 |
| 01777 | 01777 | 03777 |

fieldの許容上限が01777でも、後処理がsetgidを加えた02770になることはAPI検証との矛盾ではない。異なる段階の値だからである。またemptyDirの呼出しは `fsGroupChangePolicy` 引数をnilで渡す。Podに `OnRootMismatch` を書けば、emptyDirのmode拡張を止められるとは判断しない。

ownership処理のLchown/Chmod失敗はログに記録され、emptyDir側もChangePermissionsの戻り値をそのままsetup失敗にはしない。したがってfsGroupありの計算値を、成功したPodが必ず達成した実測値として扱わない。nodeの関連ログと実directoryを確認する。

### 4. ready volumeは毎回chmodされるわけではない

固定 `SetUpAt` は、ready記録がありdefault-mediumのdirectoryが存在する場合、quota確認の経路からreturnし、`setupDir` とownership変更を通らない。`RequiresRemount` もfalseを返す。このため、同じPod・volumeが準備済みである状況では、gateを有効にしてkubeletを再起動しただけでmodeが再適用されたとは言えない。[固定setupの早期return](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/volume/emptydir/empty_dir.go)

これは「必ず再適用されない」「すべての再起動で同じ経路になる」という主張ではなく、再設定を省く成立条件があるという観察である。アプリ等が後から変更したmodeを継続的に修復する機能として使わない。新規Podで確認する場合も、emptyDirはPodの寿命に結び付くため、データ消失を伴う再作成を無条件の対処策にしない。[emptyDirの寿命](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir)

### 5. sticky bitはcontainer名による分離ではない

Linuxのunlink契約は、sticky directoryで削除するprocessのeffective UIDがfile ownerまたはdirectory ownerと一致するか、CAP_FOWNERがあるかを区別する。同じUIDで動く二つのcontainerは、別containerという理由だけでは削除を隔離されない。directory ownerや該当capabilityを持つprocessも別に評価する。[unlink(2)](https://man7.org/linux/man-pages/man2/unlink.2.html)

したがって01777だけで「他containerが作ったartifactを保護できた」としない。sticky bitは削除制限であり、file自体のread/write permissionの代替でもない。共有先のUID/GID・file mode・capabilityと、volumeをread-onlyでmountできるconsumerを分けて設計する。readOnlyはそのcontainerのmountに対する設定であり、同一Pod内の別mountまでread-onlyにするものではない。[read-only mounts](https://kubernetes.io/docs/concepts/storage/volumes/#read-only-mounts)

## 移行と受入条件（独自の設計案）

以下は資料を組み合わせたレビュー案であり、公式runbookの転載や実行済みのクラスタ試験ではない。

1. node poolごとにAPI server/kubeletの版と実効gate、対象medium、Pod admission後のfsGroup・UID/GIDを記録する。新規に書いたmodeがAPIに保存されたかを最初に確認する。
2. 初期modeと、fsGroup後の期待modeを別欄で管理する。0750の見た目をgroup書込み禁止の最終証拠にしない。明示0と省略を別caseにする。
3. 本番dataを使わない新規Podでdirectoryのmode・UID/GID・ACLと実アクセスを観測する。Runningだけで合格にせず、権限不足ログがないかも確認する。
4. gateを無効にしたnodeへ配置されるcaseを検証環境で確認する。API field保持とnodeでのfallbackを別に記録し、modeが要件のworkloadは未確認nodeへ出さない方針にする。
5. 新規volumeとready記録のある既存volumeを別々に試す。既存volumeが期待外なら、稼働data・停止影響・所有者を確認したうえで修正方法を決める。単にkubelet再起動を「修復完了」と報告しない。
6. sticky bitの受入れは、異なるUID、同じUID、directory owner、必要capabilityを持つ場合を区別する。削除だけでなく、fileの読取り・上書きと正当なproducer/consumerの動作も別に確認する。

最小の境界caseは「gate on/off」「nil/0/0750/01777」「fsGroupなし/あり」「新規/ready volume」「同一UID/別UID」。すべてを単一のPod成功結果へ潰さない。

## 出典・適用版・限界

- 2026-10-24 UTCにtask guide、Volumes、Pod API、導入記事、unlink manualの実ページをnative webで確認した。guide更新表示は2026-06-30、Volumesは2026-09-24、v1.37生成APIは2026-08-26。導入記事はNispriha Jagan / Neeraj Krishna Gopalakrishna（Red Hat）、公開2026-09-16・更新表示2026-09-11。取得日と公開日を混同しない。
- source解析はv1.37.0のtagが指す完全SHA f54c212e3a2f75d674b717a9b29052b20b60aefcに固定した。tag metadataは2026-08-26。これは導入版の観察であり、現在の推奨patch版という主張ではない。v1.37.2 release URLは404、固定blobのnative web取得はcache missだったため、その成功を数えず、GitHub connectorで完全SHAのファイル・test・LICENSEを読んだ。
- `TestEmptyDirVolumeMode` はdefault-mediumで0750・01777・nilを検査し、別testがgate offで0777を検査する。今回は上流testの静的読解だけであり、testコマンドは動かしていない。explicit zero・fsGroup後の数値・ready経路は静的分析による判断である。
- 導入記事はMemory/HugePagesも対象と説明するが、本稿の実効modeの計算と確認範囲はdefault-mediumに限定する。tmpfs/hugetlbfs mount後のmode、user namespace、ACL・SELinuxとの合成、vendor backport、後続patch、全skew構成、実クラスタのdata保持・可用性は未検証。
- Kubernetes websiteは[CC-BY-4.0](https://github.com/kubernetes/website/blob/main/LICENSE)、固定Kubernetes実装は[Apache-2.0](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/LICENSE)を確認。man-pagesは6.19、本文日付2026-02-08、HTML生成2026-09-09で、ページ固有licenseは未確認。出典付き独自要約・分析のみで、上流コード・manifest・長文の転載はない。
- 再確認期限は90日後の2027-01-22。検索evalは発見性だけを検証し、mode強制・削除防止・runtime互換性を保証しない。

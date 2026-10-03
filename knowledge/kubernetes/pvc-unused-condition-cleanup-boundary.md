---
{
  "id": "kubernetes-pvc-unused-condition-cleanup-boundary",
  "title": "Kubernetes 1.37 PVC Unused: Bound限定・観測空白・削除保護の境界",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes v1.37.0 / PersistentVolumeClaim v1; PersistentVolumeClaimUnusedSinceTime beta・既定trueはv1.37、alphaはv1.36; 2026-10-03確認、実クラスタ未検証",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "PersistentVolumeClaimUnusedSinceTime",
    "PVC",
    "Unused",
    "lastTransitionTime",
    "Bound",
    "cleanup",
    "rollback"
  ],
  "sources": [
    {
      "id": "kubernetes-pvc-unused-blog-20261003",
      "url": "https://kubernetes.io/blog/2026/09/21/kubernetes-v1-37-pvc-last-used-time/",
      "type": "maintainer_article"
    },
    {
      "id": "kubernetes-pvc-unused-pv-guide-20261003",
      "url": "https://kubernetes.io/docs/concepts/storage/persistent-volumes/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-pvc-unused-api-v137-20261003",
      "url": "https://kubernetes.io/docs/reference/kubernetes-api/core/persistent-volume-claim-v1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-pvc-unused-gates-20261003",
      "url": "https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-pvc-unused-controller-v1370-20261003",
      "url": "https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/volume/pvcprotection/pvc_protection_controller.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "kubernetes-pvc-unused-tests-v1370-20261003",
      "url": "https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/volume/pvcprotection/pvc_protection_controller_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "kubernetes-pvc-unused-terminal-v1370-20261003",
      "url": "https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/volume/util/util.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "kubernetes-pvc-unused-statefulset-20261003",
      "url": "https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# PVC の「未使用」を、削除可能性と取り違えない

## 問いと今回の変更

PVC に古い `Unused=True` があれば、放置されたストレージとして回収してよいか。Kubernetes 1.37 の新しい既定動作を使う際に、PVC 自身の phase、参照する Pod の状態、観測の継続性、実データの保持要件をどう分けるかを扱う。

[2026-09-21 のbeta紹介記事](https://kubernetes.io/blog/2026/09/21/kubernetes-v1-37-pvc-last-used-time/)と[feature gate表](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/)を確認した。`PersistentVolumeClaimUnusedSinceTime` は v1.36 では alpha・既定false、v1.37 では beta・既定true。新規導入の機能全体を9月21日に公開したという意味ではなく、この日はbetaの解説記事の公開日である。

結論は、`Unused` を回収候補の発見に使い、削除の承認・完了や最後のディスクI/Oの証明には使わないこと。本稿の実装読解は v1.37.0 のみに固定する。既存のSELinux共有volume、Pod終了順序、PDBの知識とは異なり、管理API上の利用観測と保存データの処分判断が主題である。

## 1. 値だけでなく condition の型と時刻の向きを読む

[PVC API reference](https://kubernetes.io/docs/reference/kubernetes-api/core/persistent-volume-claim-v1/)では、`status.conditions` の要素に `type`、`status`、`lastTransitionTime` などがある。`status` の一般的な値は文字列の `True` / `False` / `Unknown`。`lastTransitionTime` は状態が切り替わった時刻で、`lastProbeTime` は別のfieldである。

[Persistent Volumes guide](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#unused-pvc-tracking)が説明する通常状態は次の二つ。

| type が Unused の status | reason | 時刻の読み方 |
|---|---|---|
| `True` | `NoPodsUsingPVC` | 未使用側へ切り替わった時刻 |
| `False` | `PodUsingPVC` | 使用中側へ切り替わった時刻 |

`False` の古い時刻を「その日から使っていない」と逆読みしない。配列の先頭が必ず `Unused` とも仮定しない。`Resizing` 等の別conditionの時刻を未使用期間へ転用しない。

この利用判定はPod参照を基にする。running中でも書き込みを全く行わないPodは使用側に残り、未scheduleのPending Podも参照者になる。したがって `Unused=False` はmount成功やread/writeの発生を証明しない。反対に `Unused=True` はデータが空であること、業務上不要であること、将来の再利用予定がないことを示さない。[beta紹介のPending Pod説明](https://kubernetes.io/blog/2026/09/21/kubernetes-v1-37-pvc-last-used-time/)

## 2. Pending PVC と Pending Pod は別の境界

「すべてのPVCへ追加される」という紹介文だけでは、欠落値の扱いを決められない。固定した[v1.37.0 controller](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/volume/pvcprotection/pvc_protection_controller.go)の `processPVC` は、gate有効、PVCの `deletionTimestamp == nil`、PVCの `status.phase == Bound` を満たすときだけUnusedを評価する。コメントはPV controllerのbinding処理との競合回避を理由としている。

[同版のunit test](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/volume/pvcprotection/pvc_protection_controller_test.go)は、未BoundのPVCにcondition更新を行わないケースと、Bound PVCを未scheduleのPending Podが参照しているためFalseを追加するケースを別々に持つ。ここではtest本文と期待されるAPI actionを読んだのであり、上流testを実行して合格させたわけではない。

| 観測対象 | v1.37.0での読み方 | 運用上の分類案 |
|---|---|---|
| Pending PVCにconditionなし | 未Boundは更新対象外 | binding/provisioningの調査へ |
| Bound PVC + 未scheduleのPending Pod参照 | 使用側、False | 利用意思あり。mount成功は未証明 |
| Bound PVC + Succeeded/Failed Podだけ | 通常は未使用側、True | 完了データの保存方針を確認 |
| Bound PVCを複数Podが参照、一つだけ終了 | 残る参照者で使用側を維持 | Pod一件の削除から推定しない |
| PVC自身にdeletionTimestampあり | condition更新対象外 | 回収候補ではなく既存削除の進行を調査 |

欠落を `False` の別表記やゼロ日とせず、まず `type=Unused` の有無とPVCのphaseを記録する。「Pending」という語だけで分岐すると、PVCとPodを取り違える。

## 3. 最後のI/O・unmountと同じ時計ではない

v1.37.0の `setUnusedCondition` はcontroller側の現在時刻を使って `LastTransitionTime` を書く。既存Trueのまま未使用が続くときは、`updateUnusedCondition` は書き直さない。conditionの古さはheartbeatの停止時間を直接示さず、毎回のGETに応じて更新される時計でもない。[controller実装](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/volume/pvcprotection/pvc_protection_controller.go)

公式guideも、処理遅延や機能を後から有効にしたことによって、表示される未使用期間が実際より短くなり得ると注意する。初めてTrueが付いた日を、過去の最終利用日時まで復元したものと扱わない。[時刻に関する注意](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#unused-pvc-tracking)

Podの終端判定にも実装上の細部がある。`podUsesPVCForUnusedSince` は[v1.37.0のIsPodTerminated](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/volume/util/util.go)を呼ぶ。このhelperはSucceeded/Failedのほか、PodにdeletionTimestampがあり、init・通常・ephemeralの全container statusがwaiting/terminated、または各リストが空の場合も終端扱いにする。単にphaseだけを読む独自collectorと結果が違う余地がある。

helper自身にもkubelet statusだけでは削除済みPodを無視して安全とは保証し切れない旨の注意がある。この分岐は「全volumeのunmountを確認した」「storage側flushが完了した」というAPIではない。強制削除やnodeとの通信断を含む実運用では、Unusedをstorageの安全な切離しの代用にしない。

## 4. rollbackや監視の空白を期間へ足さない（実装からの推論）

controllerのgateを無効にするとUnused更新の分岐へ入らない。既存conditionを消して「不明」にする分岐も当該controllerにはない。再有効化後、同じ状態の再評価では時刻を更新しないため、以下の履歴は古いTrueと矛盾しない。

1. 時刻AにPVCが未使用となり、Trueが保存される
2. controller停止またはgate無効の期間中にPodが利用し、終了する
3. 再開後の現在状態も未使用で、Trueのままになる

この場合、Aから現在まで連続して未使用だったとは証明できない。この三段階はソース上の分岐から導いた反例であり、実クラスタで再現した試験結果ではない。公式guideの「処理遅延で短めに出る」という説明を、更新停止をまたいでも古いTrueが常に保守的だという保証に拡張しない。

PVCイベントのhandlerも、gateが有効でUnused conditionが未作成ならqueueへ入れる一方、既存conditionがある通常PVCの更新すべてを強制再計算するわけではない。Podイベントなどの再評価経路はあるが、「再起動したから全件の時刻が最新になった」「有効化から一定秒待てば完全な履歴が戻る」とはしない。[event handlerと同一状態の分岐](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/volume/pvcprotection/pvc_protection_controller.go)

運用上は、gate無効・旧controller・API失敗等で観測が途切れた集合を別扱いにする。復旧後の現在のPod参照を確認しても、失われた過去の利用履歴は復元できない。自動的な候補化を再開する場合は、外部collector側で新たな観測窓を設ける、または人が利用者へ確認する方針が必要になる。Kubernetesが管理するconditionの時刻を手で書き換えて穴を隠さない。

## 5. UnusedとPVC削除保護は別predicate

同じPVC protection controllerにあっても、使用状況表示は `podUsesPVCForUnusedSince`、削除時のfinalizer判定は `podUsesPVCForDeletion` を渡して調べる。後者はschedule済みPodを条件にし、通常の `persistentVolumeClaim` 参照では前者と同じterminal除外を行わない。[二つのpredicate](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/volume/pvcprotection/pvc_protection_controller.go)

したがって、schedule済みの完了Pod objectが通常PVC参照を残しているケースでは、UnusedはTrueでも削除保護の判定は参照ありになり得る。逆に未schedule Podの参照はUnusedをFalseにしても、そのPodだけを理由に後者が保持するとは限らない。generic ephemeral volumeには所有関係等の追加判定もあるので、この通常PVC参照の例をそのまま当てはめない。

これは「Trueなのにfinalizerが残るからfinalizerを外してよい」という根拠にはならない。削除中PVCのUnusedは更新されず、testにも既存Trueを保持したままfinalizerを取り除く期待がある。最後までTrueが見えることは削除の成功報告ではない。[削除中の上流test](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/controller/volume/pvcprotection/pvc_protection_controller_test.go)

さらに、Unused観測、PVC削除、PV回収、外部storage資産の削除は別段階。`Delete` reclaim policyをサポートするvolumeでは、PVと外部資産が削除対象になり、`Retain` では後続の手動回収が必要になる。動的provisioningのPVはStorageClassのpolicyを継承するため、候補PVCだけでなく実際にboundされたPVのpolicyを確認する。[reclaim policy](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming)

## 6. 回収候補を作るための独自の判断手順

以下は一次資料に基づく設計案。公式の自動cleanup手順でも、実行済みの運用でもない。

1. **観測が有効な範囲を決める。** API serverと実際にcontrollerを動かしているkube-controller-managerの正確な版、feature gate、直近の停止・切替、status更新失敗を記録する。APIから古いconditionが読めることだけで、更新主体の健全性を証明しない
2. **identityを保存する。** cluster、namespace、PVC名に加えてUID、resourceVersion、観測時刻、phase、bound PV名、deletionTimestampを記録する。削除・同名再作成のあとに以前の期間や承認を引き継がない
3. **未知を候補から分ける。** condition欠落、Unknown、重複するUnused、時刻欠落・parse失敗・未来時刻、未Bound、削除中、観測停止を通った値は判定保留にする。Falseも候補に含めない。これらを暗黙に古い日付へ変換しない
4. **期間に業務上の根拠を持たせる。** Trueと有効な時刻を得ても、例えば「30日」は利用者が決める方針でありKubernetesの既定TTLではない。月次Jobや季節処理など、次回使用予定と比較する。クラスタ時計と外部collector時計のずれも確認する
5. **所有者と復元要件を照会する。** 完了Jobの成果物、休止中の環境、scale-to-zeroされたworkload、復旧用データを区別する。バックアップが存在するという申告だけでなく、必要な復元点・保持期間・復元手順を確認する
6. **承認直前に読み直す。** 以前の候補一覧をそのまま削除命令にしない。同じUID、現在のcondition、参照Pod、所有関係、PVと実資産の扱いを再照合する。read-check-deleteの間にも新しい利用は起こり得るため、再読だけを排他ロックと呼ばない。必要な変更管理・利用者との停止合意を別途設ける
7. **結果を段階別に記録する。** 承認済みの回収を実施する場合でも、API削除要求の受付、PVC消失、PVの状態、provider側の資産状態を区別する。タイムアウトしたら即座に再要求やfinalizer除去へ進まない

StatefulSetには既に `persistentVolumeClaimRetentionPolicy` の `whenDeleted` / `whenScaled` があり、Retainが既定、Deleteは指定されたlifecycleでのPVC削除に関わる。これはUnusedの経過日数とは別のpolicyである。既存の保持設計を確認せず、新しい掃除処理で上書きしない。[StatefulSet PVC retention](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#persistentvolumeclaim-retention)

## 7. 検証計画と検索で守る境界

導入前の検証は、対象配布物・実効gateを揃えた隔離環境で次を観測する案とする。必要な作成・削除はそれぞれ認可された範囲でのみ行う。

- Pending PVCと、Bound PVCを参照するPending Podを別ケースにする
- 参照者なし→一つ→二つ→一つ→ゼロの推移で、FalseとTrueの方向を確認する
- 完了Pod objectを保持したままUnusedと削除保護の違いを見る。成果物の保持要件を判定条件から落とさない
- status更新失敗時は誤った新しい時刻を書いたと見なさず、更新に成功するまでの遅れとログを確認する。上流testは失敗からのretryを扱うが、環境ごとの更新時間を保証しない
- gate無効をまたいだ利用サイクル、旧controllerへの切替、再有効化の後で、古いTrueを連続未使用と誤認しないことを確認する
- 同一状態が続いて時刻が変わらないケースと、controller障害で時刻が変わらないケースを、conditionだけでなく外部観測で区別する
- 欠落・Unknown・False・未来時刻・同名別UIDを、候補出力へ混ぜないことをcollector側のtestにする

本リポジトリのevalはこの文書を検索で見つけるためのもの。KubernetesのAPI実装、回収処理の安全性、CSIの動作やデータ復元の成否を試すものではない。

## 適用版・出典・未確認事項

取得日は2026-10-03 UTC。Kubernetes Authorsのv1.37.0 tagは2026-08-26付で、[tag object](https://api.github.com/repos/kubernetes/kubernetes/git/tags/157e582fcc3ebba3c22b16721f49d6890f784c1f)からcommit `f54c212e3a2f75d674b717a9b29052b20b60aefc` への対応を確認した。コード・test・helperはすべてこのcommitに固定した。以後のpatch版やベンダー差分の同一性は断定しない。

beta記事の公開・更新表示は2026-09-21、API referenceはv1.37生成・更新表示2026-08-26。Persistent Volumes guideのfooterは2026-06-17、feature gateページは2026-01-27、StatefulSet guideは2026-08-23と表示される。ページ全体のfooterが個別段落の変更日を表すとは限らないため、機能の導入・昇格日はgate表とrelease版を根拠にした。

Kubernetes contributorsによるwebsite文書は[CC-BY-4.0](https://github.com/kubernetes/website/blob/main/LICENSE)、固定commitのコードとtestは[Apache-2.0](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/LICENSE)およびファイルheaderを確認した。本稿は帰属リンク付きの独自要約と読解、運用設計案であり、上流のコード・manifest・cleanup用queryは転載していない。

GitHubコードページとraw URLの一部はnative web取得で失敗したため、同じ固定commitの本文をGitHub connectorで取得して補った。実クラスタ、upstream unit/e2e test、各マネージド環境、API serverとcontroller-managerの全version skew組合せ、feature gate無効時のAPI受付差、全CSI driverでのunmount・回収動作は未検証。特定の反映秒数やmetric名を本稿の稼働保証にしない。GA時期も確約しない。

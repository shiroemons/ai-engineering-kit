---
{
  "id": "docker-lazy-pull-archive-completeness-boundary",
  "title": "Docker Engine 29.9 lazy-pull: 起動成功と配布archive完全性の境界",
  "kind": "knowledge",
  "technology": "docker",
  "version": "Docker Engine 29.9.0 (2026-10-08); docker-v29.9.0 at a5b58c94d334674eae3a3207d4f05417643a90d0; rolling official docs verified 2026-10-23 UTC; Docker/runtime tests not executed",
  "tags": [
    "research-domain:infrastructure",
    "docker",
    "lazy-pull",
    "containerd",
    "snapshotter",
    "stargz",
    "overlayfs",
    "WithUnpackFetchAllContent",
    "docker save",
    "docker push",
    "content store",
    "platform",
    "archive",
    "cross-repo mount"
  ],
  "sources": [
    {
      "id": "docker-lazy-pull-release-2990-20261023",
      "url": "https://docs.docker.com/engine/release-notes/29/",
      "type": "release_notes"
    },
    {
      "id": "docker-lazy-pull-storage-guide-20261023",
      "url": "https://docs.docker.com/engine/storage/containerd/",
      "type": "official_docs"
    },
    {
      "id": "docker-lazy-pull-daemon-reload-20261023",
      "url": "https://docs.docker.com/reference/cli/dockerd/",
      "type": "official_docs"
    },
    {
      "id": "docker-lazy-pull-digest-platform-20261023",
      "url": "https://docs.docker.com/reference/cli/docker/image/pull/",
      "type": "official_docs"
    },
    {
      "id": "docker-lazy-pull-save-platform-20261023",
      "url": "https://docs.docker.com/reference/cli/docker/image/save/",
      "type": "official_docs"
    },
    {
      "id": "docker-lazy-pull-push-platform-20261023",
      "url": "https://docs.docker.com/reference/cli/docker/image/push/",
      "type": "official_docs"
    },
    {
      "id": "moby-lazy-pull-selection-export-2990-20261023",
      "url": "https://github.com/moby/moby/tree/a5b58c94d334674eae3a3207d4f05417643a90d0/daemon/containerd",
      "type": "github_repository_analysis"
    },
    {
      "id": "moby-lazy-pull-retained-snapshot-test-2990-20261023",
      "url": "https://github.com/moby/moby/blob/a5b58c94d334674eae3a3207d4f05417643a90d0/integration/image/pull_test.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Docker Engine 29.9 lazy-pull: 起動成功と配布archive完全性の境界

## 解く問いと結論

コンテナが起動し、`docker pull` や `docker save` も成功した。これだけで、registryから切り離した環境へ渡せる完全なimageを確保したと言えるか。

**言えない。実行に使うsnapshot、配布に必要なlayer blob、配布対象のplatform一覧は別の状態である。** 特にlazy pullでは、imageが動いてもcontainerd content storeに必要なblobがない場合がある。公式storage guideは、`docker save` がerrorになる場合だけでなく、errorなしで不完全なarchiveを作る場合も明記している。[containerd image store](https://docs.docker.com/engine/storage/containerd/#saving-and-pushing-images)

本稿はDocker Engineのcontainerd image storeで、短い起動時間とimageの持ち出し・mirror・air-gap搬送をどう分けて検証するかを扱う。既存のembedded-containerd文書が扱うdaemon process・保存先の選択、BuildKit文書が扱うbuild cacheとは別の問いである。以下の固定実装の観察と公式契約を根拠とし、検証手順は独自の設計案として分ける。

## 版と調査範囲

- **29.9.0、公開2026-10-08**: 必要なlayer blobを既存snapshotだけで取得済みと判断する問題の修正と、remote snapshotter向けの `lazy-pull` featureが同じreleaseに含まれる。したがって「29.9.0にすれば、すべてのsnapshotterでpull後に全blobが揃う」という解釈はしない。[release notes](https://docs.docker.com/engine/release-notes/29/#2990)
- 固定読解の対象は製品tag `docker-v29.9.0` のcommit `a5b58c94d334674eae3a3207d4f05417643a90d0`。annotated tagをGitHub APIでcommitへ解決した。rolling guideは2026-10-23 UTCに再取得した。
- 29.9.0を最新・推奨更新先とは述べない。以降のpatch、vendor backport、Docker Desktop全体、独立したcontainerd/CRIへの適用は別に確認する。Docker daemonやsnapshotterの実行試験は今回行っていない。

## 契約1: lazy-pullはblob取得方針であり、remote機能の自動検出ではない

[29.9.0のimage_pull.go](https://github.com/moby/moby/blob/a5b58c94d334674eae3a3207d4f05417643a90d0/daemon/containerd/image_pull.go) の `lazyPull` は、明示feature値があればそれを使い、なければsnapshotter名の固定listを調べる。これはsnapshotterの能力probeではない。

| snapshotter / feature | 29.9.0の選択 | 運用上の意味 |
|---|---|---|
| nydus・overlaybd・soci・stargz / 未指定 | lazy pull | snapshotで賄えるlayer blobを省略できる |
| overlayfsなど上記以外 / 未指定 | full-content取得 | 既存snapshotがあっても不足blobを取得する |
| 任意 / `lazy-pull: false` | full-content取得 | 名前による既定値より明示falseが優先 |
| 任意 / `lazy-pull: true` | lazy pull | 名前による既定値より明示trueが優先 |

full-content取得側では `WithUnpackFetchAllContent()` をpullのunpack optionに追加する。lazy側はこれを追加しない。trueは「そのsnapshotterにremote mount実装を追加する」という意味ではない。local snapshotterへtrueを指定してもremote snapshotterへ変わらず、既存snapshotを使う際にblob取得を省く選択になる。独自名で登録したremote snapshotterがlistと一致するとも限らない。

固定コードのコメントは、snapshotter APIにremote/lazy能力を確実に識別する標準手段がなく、proxy pluginであるだけでも判定できないと説明する。`containerd.io/snapshot/remote` の有無や製品名を独自に推測して既定動作を決めず、実際の登録名とfeature値を確認する。[固定実装とTestLazyPull](https://github.com/moby/moby/tree/a5b58c94d334674eae3a3207d4f05417643a90d0/daemon/containerd)

lazy pullでもすべてのdownloadを省くわけではない。公式guideの例ではstargzはeStargz layerをremote mountできるが、通常のgzip layerは既存snapshotがなければdownloadが必要である。「lazy=trueならnetwork転送ゼロ」という性能予測には使えない。[Lazy pulling](https://docs.docker.com/engine/storage/containerd/#lazy-pulling)

## 契約2: snapshotが使えることと、配布blobを所有することは別

既存snapshotはコンテナの起動に再利用できる。一方、imageを移送するにはmanifestが指すblobが必要になる。この差を直接検査しているのが、固定版の [TestImagePullWithExistingSnapshot](https://github.com/moby/moby/blob/a5b58c94d334674eae3a3207d4f05417643a90d0/integration/image/pull_test.go) である。

同testは次の順でregressionを検査する。

1. fixture imageをpullし、saveしたarchive内の各layerの内容からdigestを計算して元のmanifestと照合する。
2. コンテナを作ってunpacked snapshotを保持しながらimage参照を削除する。
3. layer blobがcontent storeに残っていないことを確認する。ここを省くとcacheが不具合を隠す。
4. 同じimageを再pullし、archive内のlayer内容とdigestを再び検査する。

これは上流testの読解であり、本稿でimage削除・GC操作を行った記録ではない。本番で再現する手順としてコピーしない。testにはWindows、remote daemon、containerd image store以外をskipする条件があり、このtestだけを全platformの保証へ広げない。

また、lazy imageを起動して長時間動かしても、必要blobがcontent storeへ戻るとは限らない。公式guideはstargzが取得データを自身のcacheへ保持する例を挙げる。**アプリが全ファイルを読んだように見えることは、配布用content storeの修復完了ではない。**[Saving and pushing images](https://docs.docker.com/engine/storage/containerd/#saving-and-pushing-images)

## 契約3: save/pushの成功判定とplatform範囲を分ける

### save: exit codeだけではarchive完全性を判定しない

29.9.0の [ExportImage](https://github.com/moby/moby/blob/a5b58c94d334674eae3a3207d4f05417643a90d0/daemon/containerd/image_exporter.go) は、export optionに `WithSkipMissing(i.content)` を含める。公式guideの不完全archive警告と合わせ、save成功を「manifestが要求する配布blobが全て含まれた」という証拠にはしない。

さらに [save CLI reference](https://docs.docker.com/reference/cli/docker/image/save/#save-a-specific-platform---platform) の既定対象は、daemonのimage storeに存在するplatform variantsである。registryのimage indexに載っている全architectureをsave時に取得する契約ではない。`--platform` で指定したvariantがローカルに存在しなければerrorになる。必要なplatformを一つだけ検証して、他platformまで含むbackupと呼ばない。

### push: remote側の再利用成功はlocal blob所有の証拠ではない

公式guideでは、不足blobをuploadできなくても、target registryがsource repositoryからのmountを許す場合には再利用できる。固定版の [image_push.go](https://github.com/moby/moby/blob/a5b58c94d334674eae3a3207d4f05417643a90d0/daemon/containerd/image_push.go) も `findMissingMountable` で候補を収集してcross-repo mountを試す。したがって、あるregistryへのpush成功は、別registryへのuploadやオフライン持ち出しの成功を保証しない。[公式のpush制約](https://docs.docker.com/engine/storage/containerd/#saving-and-pushing-images)

固定版では、platform未指定でindexのpushがmissing contentによって失敗した場合、利用可能なplatform manifestを選んで再試行する分岐もある。成功時には `ManifestPushedInsteadOfIndex` の追加情報を返す。元indexと同じ配布範囲を要求するpipelineは、最終結果のdescriptor・platformを別に検査する必要がある。すべてのpushがこのfallbackに入るという意味ではない。

また、明示の `docker push --platform` はsingle-platform manifestを送る操作で、image index、他manifest、attestationsを保存しないと[CLI reference](https://docs.docker.com/reference/cli/docker/image/push/)が定める。**不足layerの補充、platform網羅性、attestation保持はそれぞれ別の完了条件**である。errorを消すためのplatform限定を、同等な複製へ読み替えない。

### full-content取得も、remote index全体の複製とは限らない

固定 `pullTag` は指定platform、未指定ならhost platformを `WithPlatform` に渡す。`lazy-pull: false` が変えるのはそのpullのlayer取得方針であり、index内の全platformを選ぶflagではない。29.9.0の `PullImage` は複数platform指定を非対応errorにする。CLIの `--all-tags` もplatform網羅の代替ではない。[固定pull実装](https://github.com/moby/moby/blob/a5b58c94d334674eae3a3207d4f05417643a90d0/daemon/containerd/image_pull.go) / [pull reference](https://docs.docker.com/reference/cli/docker/image/pull/)

## 取得方針の変更と既存imageの補充

公式guideの対処は、`features` 内の `lazy-pull` をfalseにし、daemon設定をreloadして、その後imageを再pullするという三段階である。設定変更だけで、過去にlazy pullしたimageを一括補完するとは書かれていない。[Configuration](https://docs.docker.com/engine/storage/containerd/#configuration)

以下は既存 `features` objectへ統合する項目の説明用抜粋であり、daemon.json全体を置換する設定ではない。

```json
{
  "features": {
    "lazy-pull": false
  }
}
```

`containerd-snapshotter`、data-root、runtime、network等の既存値は変えない。image store切替をこの修復の必須手順にしない。設定の所有者・実際の `--config-file` ・起動flagを確認してから変更計画を作る。

[dockerd reference](https://docs.docker.com/reference/cli/dockerd/#configuration-reload-behavior) はLinuxでSIGHUPによるreloadを定め、`features` も再設定対象に含める。ただしCLI flagとのconflictがあれば再設定は失敗し、daemon自体は動き続ける。**daemonが生存していることは新設定を受理した証拠ではない。** 変更前の `dockerd --validate --config-file=...` と、reload後のログ・実効挙動の確認を分ける。validate成功だけでreload成功やblob補充を合格にしない。

再pull時に可変tagだけを使うと、欠けた旧imageの修復ではなく別versionの取得になる場合がある。元のdigestと必要platformを記録し、同じ対象を補充する設計にする。[digest指定の契約](https://docs.docker.com/reference/cli/docker/image/pull/#pull-an-image-by-digest-immutable-identifier) は固定版の指定を可能にするが、新しいsecurity updateを自動取得する意味ではない。修復とversion更新を別の承認・検証対象として扱う。

## 実務での受入れ手順（独自の設計案）

以下は一次資料を組み合わせた提案で、公式migration procedureや実行済み試験結果ではない。

1. **成果物の要求を先に決める。** 起動専用cacheか、air-gap搬送用archiveか、単一platform配布か、index・attestationsを含む複製かを明記する。運用nodeの起動最適化と配布用builderの取得方針を無理に同一にしない。
2. **対象を固定する。** 接続先daemon、server版、image store、snapshotter登録名、lazy-pullの明示値/未指定、source digest、必要platform一覧を保存する。CLI版だけでserver挙動を決めない。
3. **変更の受理と再取得を別々に記録する。** 必要なら承認された隔離環境でfalseを設定し、reload受理を確かめてから、要求platformごとに元digestを再pullする。pull失敗や容量不足を握りつぶしてarchive公開へ進まない。
4. **archiveの要求内容を照合する。** tarができた、サイズが大きい、saveの終了値が0、という条件だけにしない。必要platformのmanifest/config/layer参照を基準に、対象blobの存在とdigestを検査する。platformで絞ったexportではindex構造が変わり得るため、元indexのdigest一致だけを唯一の条件にはしない。
5. **cacheのない別環境で復元を確認する。** 元snapshotter cacheを共有しない隔離daemonへarchiveをloadし、対象platformの起動・必要機能を検査する。オフライン配布の要件はその環境で検査し、オンライン元環境の起動成功で代用しない。短いsmoke testだけでは全アプリ挙動まで保証しない。
6. **registry配布も最終内容を見る。** push成功後、targetの実際のdescriptor、必要platform、要求したattestationsを比較する。cross-repo mount可能な同一registryと、別registryへのuploadを別ケースとして記録する。
7. **検証済み成果物だけ公開する。** 不完全archiveは再利用されないよう未承認扱いにし、検証したsource digest・platform・archive checksum・実行結果を揃える。元imageやcacheのpruneを修復の前提にせず、既存の復旧材料を保持する。

## 最低限の試験ケースと誤った合格条件

| ケース | 確認する状態 | 合格の根拠にしないもの |
|---|---|---|
| overlayfs・feature未指定・snapshotあり/blobなし | 再pull後の必要blobとarchive digest | snapshotが残っていること |
| stargz・feature未指定 | 起動経路とcontent store欠落を別確認 | コンテナが起動したこと |
| stargz・明示false・reload前後 | 受理された設定と以後のpull | JSONを書き換えただけ |
| 明示false後の過去lazy image | 同一digestの再pullとarchive検査 | daemonが再起動/reloadしただけ |
| overlayfs・明示true | 既存snapshotがある場合の省略方針 | remote snapshotter化したという仮定 |
| save終了値0 | 要求platformと全参照blob | tarの存在・終了値だけ |
| 同一registry内push成功 | targetのmanifest範囲と再利用の有無 | local blobが全てあるという推測 |
| multi-platform imageの一部だけ取得済み | 要求platformごとの完全性 | 一つのplatformの起動・push成功 |
| index pushからplatformへfallback | 最終descriptorと配布範囲 | push全体の成功だけ |

TestLazyPullと既存snapshotのregression testは上流sourceを読んだだけであり、上の表の全ケースを実測したものではない。repository内の検索evalも文書を発見するための検査で、Dockerの実行保証ではない。

## 未確認事項とprovenance

- 対応snapshotter各版のperformance、cache eviction、offline時の全ファイルread、registryごとのcross-repo mount権限、Windows/rootless、distribution固有patch、daemon再設定と進行中pullの競合は実測していない。進行中pullまで新設定へ遡及的に切り替わるとは主張しない。
- `lazy-pull: false` が全attestations/referrersを必ず取得・保持するとは確認していない。本文はlayer完全性・platform範囲・追加metadataの検証を分離しており、万能なimage複製手順を提供するものではない。
- native webでrelease notes、storage、dockerd、pull/save/push各reference、関連Moby PR #53615/#53877を開いた。固定blobのnative web取得には503/cache missがあり、実装・unit/integration test・LICENSEはGitHub connectorで固定commitから読み直した。PR中の自動レビューは契約の根拠にしていない。
- Mobyは固定commitのLICENSE、Docker Docsは[README](https://github.com/docker/docs/blob/main/README.md)と[LICENSE](https://github.com/docker/docs/blob/main/LICENSE)でApache-2.0を確認した。本文は原著要約と独自設計案で、上流コード・長文引用の転載やmodule昇格は行わない。

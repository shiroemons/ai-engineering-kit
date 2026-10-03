---
{
  "id": "docker-embedded-containerd-fallback-storage-boundary",
  "title": "Docker Engine 29.8: embedded-containerd fallback と保存先・起動成功の境界",
  "kind": "knowledge",
  "technology": "docker",
  "version": "Docker Engine 29.7.0/29.8.0 release boundaries; docker-v29.8.2 implementation at 8af9fe3a36bab3e039862a2ab1cef1880c9b4d03; rolling docs verified 2026-10-03; runtime未実検証",
  "tags": [
    "research-domain:infrastructure",
    "docker",
    "containerd",
    "embedded-containerd",
    "containerd-snapshotter",
    "fallback",
    "data-root",
    "exec.ErrNotFound",
    "29.8.2"
  ],
  "sources": [
    {
      "id": "docker-embedded-release-2980-20261003",
      "url": "https://docs.docker.com/engine/release-notes/29/",
      "type": "release_notes"
    },
    {
      "id": "docker-embedded-guide-20261003",
      "url": "https://docs.docker.com/engine/daemon/embedded-containerd/",
      "type": "official_docs"
    },
    {
      "id": "docker-containerd-store-boundary-20261003",
      "url": "https://docs.docker.com/engine/storage/containerd/",
      "type": "official_docs"
    },
    {
      "id": "docker-daemon-data-root-20261003",
      "url": "https://docs.docker.com/engine/daemon/",
      "type": "official_docs"
    },
    {
      "id": "moby-embedded-startup-2982-20261003",
      "url": "https://github.com/moby/moby/tree/8af9fe3a36bab3e039862a2ab1cef1880c9b4d03/daemon/command",
      "type": "github_repository_analysis"
    },
    {
      "id": "moby-embedded-fallback-observation-20261003",
      "url": "https://github.com/moby/moby/pull/53388",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Docker Engine 29.8: embedded-containerd fallback と保存先・起動成功の境界

## 問いと適用範囲

「29.8への更新後、containerd process が見当たらなくても正常か。embedded-containerd を false にすれば常に元の実行・保存方式へ戻るか」を扱う。最小構成のCI hostや独自配布で、**process選択、image store選択、参照するデータ領域、実際のcontainer起動**を別々に検証するための文書である。既存のregistry DNS/TLS文書とは異なり、daemon起動時のruntime接続先を対象とする。

- **29.7.0（2026-07-30）**: 実験機能 embedded-containerd を追加
- **29.8.0（2026-09-03）**: system containerd が構成されず、containerd実行ファイルもない場合に embedded へ fallback する変更を追加
- **29.8.2（2026-09-30）**: 本書の実装確認版。Docker製品tag `docker-v29.8.2` を commit `8af9fe3a36bab3e039862a2ab1cef1880c9b4d03` に解決して読んだ。最新バージョン一般の保証ではない

版・日付は[Engine 29 release notes](https://docs.docker.com/engine/release-notes/29/)による。公式embeddedガイドの警告例には「将来、system containerdがない場合の既定動作になる可能性」という古い将来形が残る。一方29.8.0のrelease notesと下記固定実装には限定条件のfallbackがある。ガイドの文例から「29.8でも明示opt-inだけ」と判断しない。

## 確認した契約

### 1. embeddedとimage storeは別の選択

`embedded-containerd` は、containerd serverを dockerd と同じprocess内で動かす選択である。daemonとBuildKitはin-memory接続を使うが、task shimは別processのままで、socket接続も残る。embedded serverにはCRIがなく、`embedded-containerd` と `cri-containerd` の同時有効化はできない。[embedded guide](https://docs.docker.com/engine/daemon/embedded-containerd/)

`containerd-snapshotter` はimage storeの選択である。29.0以降の新規installはcontainerd storeが既定だが、従来版からupgradeしたdaemonは明示切替までlegacy graph driverを継続する。ただし `userns-remap` を使う構成ではcontainerd image storeは利用できず、新規installの既定値説明の例外となる。storeを切り替えると他方で作ったimages/containersが見えなくなり、元のstorage設定へ戻すと再び見える。データが自動削除された意味ではない。[containerd image store](https://docs.docker.com/engine/storage/containerd/)

したがって `docker info -f '{{ .DriverStatus }}'` が `io.containerd.snapshotter.v1` を示しても、embeddedの証明にはならない。これは上記二つの契約からの区別であり、process方式はログ等で別に確認する。

### 2. 29.8.2の起動分岐は単純なfeature booleanではない

以下は固定した[daemon/command実装](https://github.com/moby/moby/tree/8af9fe3a36bab3e039862a2ab1cef1880c9b4d03/daemon/command)の読解結果である。主にUnixの `initContainerd` と共通の `initializeContainerd` を順に追った。

| 条件・段階 | 選択される経路 | 誤認しやすい境界 |
|---|---|---|
| `features["embedded-containerd"]` が true | embeddedを先に初期化 | 設定済み `--containerd` addressより優先する |
| 上記がtrueでなく、containerd addressが明示済み | その外部endpointを利用 | ここではmanaged/embeddedを起動しない。外部の接続障害を自動救済する分岐ではない |
| address未指定でsystem containerdを検出 | 検出したendpointを利用 | 「未指定」だけでembeddedになるわけではない |
| systemも検出せずmanaged containerdを初期化 | 通常は別processを起動 | binaryがある場合までembeddedを全体の既定と説明しない |
| 上記初期化エラーが `exec.ErrNotFound` に合致 | embeddedへfallback | permission・config・runtime等、別種の起動エラーはそのまま返す |

system containerdの検出関数 `systemContainerdRunning` は、既定address（XDG利用時は対応runtime directory）のpathを `os.Lstat` し、存在を確認している。接続やhealthの確認ではないため、socket pathが残っていて不健全な場合を「containerd不在」と同じfallback条件に置き換えない。

`embedded-containerd: false` と未指定はこの分岐ではいずれも最初のtrue判定を通らない。その後の `exec.ErrNotFound` 判定はfeature値を再確認しないため、**falseだけでは自動fallbackの禁止にならない**。これは29.8.2固定コードの観察で、将来版への設定契約として一般化しない。

Windows側にも同じfallbackがあるが、明示embedded/外部addressの判定後にlegacy non-containerd runtimeを使う早期returnがある。Unixの全分岐を全Windows構成へ機械的に適用しない。また `no_embedded_containerd` build tagではembedded初期化が非対応エラーになる。独自buildの成功保証はrelease番号だけでは得られない。

### 3. system containerdとmanaged/embeddedでは保存先が違う

公式ガイドはembeddedの既定stateを `/var/lib/docker/containerd/daemon`、hostに別途installしたcontainerdの例を `/var/lib/containerd` と示す。以前system containerd側に保存したimages/containersは、そのままではembedded側から利用できない。[embedded guide](https://docs.docker.com/engine/daemon/embedded-containerd/)

29.8.2の `containerdRootDir` は通常 `data-root/containerd/daemon` を返し、**managedとembeddedの両方が同じ関数を使う**。よって「embeddedへ変えれば必ず別storeになりデータが隠れる」とも限らない。元がsystem containerdかmanagedか、実際のrootがどこかで判断する。[固定daemon.goとembedded初期化](https://github.com/moby/moby/tree/8af9fe3a36bab3e039862a2ab1cef1880c9b4d03/daemon/command)

通常のsystem containerd構成では、Dockerの `data-root` を変えても `/var/lib/containerd` のimage/snapshotデータは移らない。Docker側のvolumes/configsとcontainerd側の保存場所は別に把握する、というのが[daemon configuration guide](https://docs.docker.com/engine/daemon/#daemon-data-directory)の説明である。この一般ガイドの配置例を、embeddedにも一律に当てはめない。

固定コードには `DOCKER_CONTAINERD_ROOT` という一時的なテスト用overrideもある。本書はこれを本番の移行手順として推奨しない。異なるdaemonから同じstateを同時に使わせたり、ディレクトリの単純コピーで互換性が保証されると考えたりしない。

### 4. daemon起動やimage pullだけではruntime検証が終わらない

fallback追加の[maintainer PR #53388](https://github.com/moby/moby/pull/53388)（説明2026-08-16、merge2026-08-26）は、`containerd-shim-runc-v2` がない構成でもdaemon起動とimage pullが成功し、container startでは失敗する観測を記載している。これは作者の検証報告であり、本書で29.8.2を実機再現した結果ではない。custom runtimeでは必要なshimも変わるため、特定binary一つの存在確認だけで全runtimeを合格にしない。

embeddedのsocketはdebug用途に利用できるが、stateの所有者はDocker daemonである。一般目的のcontainerd endpointとして別clientから変更すると競合し得る。[embedded guide](https://docs.docker.com/engine/daemon/embedded-containerd/#connect-directly-to-embedded-containerd)

## 運用判断の提案（独自の設計案）

以下は根拠を組み合わせた検証案であり、公式migration procedureや実行済み作業の記録ではない。

1. 更新前に、対象daemonの版・service引数・実際のconfig file・`--containerd` address・feature値・image store・data-root・system containerd rootを別項目で記録する。接続先とrootが未確認なら、データ移行や削除へ進まない
2. 最小化したhost imageでは、containerd binaryを省いたことによるfallbackと、必要なshim/runtimeを省いたことによる起動不能を分けて検証する。単にdaemonがreadyになった、pullできた、という二つの結果で完了にしない
3. 承認された隔離環境で、利用するruntimeごとにcontainerの起動・終了、既存volume参照、daemon再起動後の状態を確認する。外部endpointあり、binary欠如、別種起動エラーを別ケースにし、ログと期待した経路が合うか記録する
4. images/containersが消えたように見えたら、process接続先とimage storeの両方を比較する。空の一覧を削除の証拠として扱わず、旧領域を保持したまま元のendpoint/root/storeへ戻せるか検討する。`prune`や旧ディレクトリ削除を復旧手順に混ぜない
5. rollbackでは `embedded-containerd: false` だけを完了条件にしない。元のsystem endpointまたはmanaged binary・rootを復元し、実際に選ばれた経路とinventoryを照合する。別rootで新しく作ったデータが自動統合される保証はない
6. 性能面は、自分のimage/store/workloadで計測する。in-memory接続の説明だけで起動時間・pull時間の改善率や可用性向上を約束しない

## 版差・未確認事項・provenance

- 実験機能であり、将来の仕様固定やGA化は主張しない。今回確認したのはEngine 29.7.0/29.8.0の告知と29.8.2の固定コード、取得日2026-10-03のrolling docsである
- Docker Desktop全体、Kubernetesの独立CRI runtime、distribution独自patch、rootless/Windowsの全組合せ、live-restore保証は未検証。ホストの設定変更、実daemon起動、速度測定は実行していない
- native webでrelease notes・embedded・storage・daemon各ページとPR本文を開いた。GitHubのweb表示では固定blob取得がcache missになったため、固定ファイルとLICENSEはGitHub connectorで取得した。最初に試した `v29.8.2` は製品tagではなく取得できず、`docker-v29.8.2` のannotated tagをAPIで解決し直した
- 実装の対象は `daemon_unix.go`、`daemon_windows.go`、`daemon.go`、`daemon_embedded_containerd.go`、`daemon_no_embedded_containerd.go`。Apache-2.0のLICENSEを同じcommitで確認した。Docker Docsは[README](https://github.com/docker/docs/blob/main/README.md)と[LICENSE](https://github.com/docker/docs/blob/main/LICENSE)でApache-2.0を確認。PR会話本文のライセンスはunknownとして原著要約に限定し、コードや長文の転記・module昇格は行わない
- 検索evalはこの文書を発見できるかを検証する。実runtimeのfallback、データ保存、復旧の正しさを試験するものではない

---
{
  "id": "messaging-nats-jetstream-desired-state-evacuation",
  "title": "NATS JetStream 2.15: desired stateで行うstream移動・server退避と保守完了の判定",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "NATS Server v2.15.0 (released 2026-09-17) @ eb763679aa3c24a40dcd3012aa046ad1996d851c; ADR-62 rev.1 @ 7dfdbb4f17c848263499eb96ad4f6ccbe810ed14; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:api-distributed",
    "messaging",
    "nats",
    "jetstream",
    "desired-state",
    "reconciliation",
    "evacuation",
    "membership",
    "rolling-upgrade",
    "catchup",
    "maintenance"
  ],
  "sources": [
    {
      "id": "nats-server-2150-release-20261003",
      "url": "https://github.com/nats-io/nats-server/releases/tag/v2.15.0",
      "type": "release_notes"
    },
    {
      "id": "nats-215-upgrade-reconciliation-20261003",
      "url": "https://docs.nats.io/release-notes/upgrade-to-2.15",
      "type": "official_docs"
    },
    {
      "id": "nats-adr62-desired-state-7dfdbb4f-20261003",
      "url": "https://github.com/nats-io/nats-architecture-and-design/blob/7dfdbb4f17c848263499eb96ad4f6ccbe810ed14/adr/ADR-62.md",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-server-2150-evacuate-api-20261003",
      "url": "https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_api.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-server-2150-reconcile-20261003",
      "url": "https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_cluster.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-server-2150-cluster-info-20261003",
      "url": "https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/stream.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-server-2150-evacuate-matrix-20261003",
      "url": "https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_cluster_1_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-server-2150-cancel-move-tests-20261003",
      "url": "https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_super_cluster_test.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# NATS JetStream 2.15: desired stateで行うstream移動・server退避と保守完了の判定

## 問いと採用判断

稼働中のbrokerを入れ替えるとき、streamのデータとconsumerの進捗を残し、どの時点で旧serverを止めてよいか。

**生きているコピーを退避する計画保守では、移動先の余力を確保してevacuationを使い、対象assetごとの収束を確認してから停止する。peer-removeやAPIのsuccessだけを停止条件にしない。** とくにR1の最後のpeerをremoveすると、空の代替peerへ移りデータを失うため、退避との選択は実害がある。

2026-09-17公開の[v2.15.0 release](https://github.com/nats-io/nats-server/releases/tag/v2.15.0)は、stream・consumerの構成変更をdesired stateへ統一し、evacuateとcancelのAPIを追加した。本稿はこの正式版に固定する。既存の[Raft解説](../distributed-systems/raft-leader-election-log-replication.md)の合意アルゴリズム一般論や[consumer ACK](consumer-ack-redelivery-dead-letter.md)とは異なり、broker内部の配置変更と保守手順が対象である。技術分類は既存の`messaging`を使用する。

## 確認した構成変更モデル

[ADR-62](https://github.com/nats-io/nats-architecture-and-design/blob/7dfdbb4f17c848263499eb96ad4f6ccbe810ed14/adr/ADR-62.md)は2026-08-26付、Implemented・2.15指定。meta leaderは配置のdesired stateを記録し、各stream/consumerのgroup leaderが実際のRaft membershipを段階的に変える。旧来の「peer数が設定replica数より多ければ移動中」という暗黙状態から、移動元・目標・進行状態を持つ方式へ変わった。

[固定実装のrunStreamMigration / reconcileDesiredState](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_cluster.go)から確認できる流れは次のとおり。

1. meta assignmentへ目標peer集合を記録し、asset leaderのtermを合わせる
2. 必要なsnapshotを作り、membership変更を一つずつlogへ提案する。前の変更がcommitする間は待つ
3. 新peerを追加し、stream storeのcatch-upを確認する。Raft logがcurrentなだけでは、データコピー済みとは数えない
4. consumerを新配置へ移す。consumerはstreamが既に存在するpeerへしか移れず、旧peerにconsumerが残ればstreamの取り外しを待つ
5. 必要なデータコピー数とquorumを満たすことを確認し、旧peerを取り除く。設定・実体のpeer集合とconsumer側の条件が揃えばdesired stateを解消する

これは「全新peerの完全追従を常に待つ」という契約ではない。実装は目標集合のquorumを基準とする判定も持つ。保守側が通常時の冗長性を回復したいなら、収束後のreplica数・offline・currentも別に確認する。

構成更新にはdesired stateの`id`とasset leaderの`term`が使われる。古いtermまたは別idの提案は捨て、新しいtermは先に記録してから更新を受け付ける。移動途中のleader交代を扱う仕組みであり、利用者が任意のfieldを直接書き換えるAPIではない。

## 退避・削除・leadership移譲を選ぶ

以下は[2.15.0 API定義とhandler](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_api.go)および固定testを照合したもの。`<stream>`などは置換するsubject tokenである。

| 操作 | subject・要求 | 選択時の注意 |
|---|---|---|
| streamの1peerを退避 | `$JS.API.STREAM.PEER.EVACUATE.<stream>`、`peer`にserver名またはpeer ID | 配下consumerも移す。必要replica数を維持できる置換先がなければ拒否し、元peer集合を保つ |
| serverからまとめて退避 | `$JS.API.SERVER.EVACUATE`、`peer`にserver名、または優先される`peer_id` | system account専用。既存assignmentを対象にしたbest effort。server自体はmeta groupに残る |
| consumerの1peerだけを退避 | `$JS.API.CONSUMER.PEER.EVACUATE.<stream>.<consumer>`、`peer` | streamのpeer集合は変えず、そのstreamが既にあるpeerから置換先を選ぶ |
| peer-remove | `$JS.API.STREAM.PEER.REMOVE.<stream>`またはsystem側の`$JS.API.SERVER.REMOVE` | コピーを先に退避する操作ではない。server removeはmeta membershipも外す。戻る予定のpeerへ安易に使わない |
| leader step-down | `$JS.API.STREAM.LEADER.STEPDOWN.<stream>`など | 既存peer間でleaderを替えるだけ。データ配置から旧serverを除く退避にはならない |

stream単位のhandlerは`requireReplicas:true`を使う。一方、server単位のhandlerはunsupportedなassignmentをskipし、個々のremap結果を集計せず、ループ後にsuccessを返す。**server全体のsuccessは、各streamの置換成功や移行完了を保証しない。** また、stream単位でも成功応答は配置変更の提案後であり、データ移動を最後まで待った証拠ではない。

[上流のPeerRemoveAndEvacuateMatrix](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_cluster_1_test.go)は、この違いを3台clusterで明示的に検査する。

- R1のremoveは最後のコピーを外し、代替先のmessage数は0になる。evacuateはデータを移してから外す
- 3台すべてがR3のpeerでspareなしなら、stream単位の退避は拒否される
- 同じ条件でserver単位の退避は成功応答を返せるが、streamは設定上R3のまま実体2peerへ収束し得る。後から適格なserverを追加すると補充される
- server evacuation後もmeta groupのpeer数は減らない。移行完了はserverの廃止完了と同じではない

したがって、guideの「既存replica数を維持する」という概説は、移行先容量が不足している場合の実体replica数まで保証するものとして読まない。testを読んだ結果であり、この調査環境でNATS clusterを起動して再現したわけではない。

## 進行状態を読む

`STREAM.INFO`と`CONSUMER.INFO`の`cluster.desired`は目標と現在の処理を示す。[2.15.0の公開response型](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/stream.go)では、`status.type`に次の値がある。右列は本稿の運用上の調査案であり、公式の自動復旧手順ではない。

| status.type | 状態の意味 | 最初に確認する対象 |
|---|---|---|
| `meta` | meta側の記録・更新待ち | mixed-version、meta leader、配置条件 |
| `membership` | peer追加・削除・leader移譲の進行 | membership commitとpeer疎通 |
| `snapshot` | snapshot生成・導入待ち | `status.err`、storage容量とI/O |
| `catchup` | データ追従待ち | 転送帯域、書込速度、移動先storage、進捗の変化 |
| `quorum` | 安全な構成変更に必要なpeer待ち | offline node、分断、移行中の追加停止 |
| `blocked` | stream/consumerの順序依存待ち | 対応するもう一方のassetのINFO |
| `unavailable` | shutdown中など、そのassetで進められない | assetの存在、shutdown状態、leader |

自動判定にはdescriptionの自然文ではなく`status.type`を使い、継続するfaultは`status.err`と併記する。`status`の不在だけで完了と判断しない。固定実装のstream/consumer双方の`isMigrating`は、replica不足をmeta leaderが補う別状態として扱うため、`migrating=false`相当でも必要な冗長性が揃っていない場合がある。

**設計資料と正式版のresponse型の差に注意する。** ADRの例は`desired.replicas`にcurrent・lag等を描くが、2.15.0の`DesiredPeerInfo`は`name`、`offline`、`peer`のみである。`current`・`lag`・`pending`を読む対象は実際の`cluster.replicas`であり、目標側に同じfieldがあると仮定してparserや監視を作らない。scale-downで最終集合が未選定ならdesired側のreplica列は省略され得る。

また、`cluster.replicas`は通常leaderを別fieldで表すため、配列長だけを総replica数として数えない。leaderが存在する通常状態ではleaderとreplica列を合わせてpeer集合を確認する。leader不在・pending・重複の扱いも監視側で定義する。

## 2.14からの導入と取消

[公式upgrade guide](https://docs.nats.io/release-notes/upgrade-to-2.15)は、2.15への移行前に**2.14.7以上の互換版へ揃えることを推奨**する。2.14.7はdesired stateを保持できるが、それを収束させる処理は2.15側にある。mixed-version期間のscale/moveは停滞し得るため、全node更新後まで延期する。create/deleteは同じ理由で延期する必要はない。これは無条件の無停止保証や2.12からの直接移行許可ではない。

取消は`$JS.API.STREAM.CANCEL_MOVE.<stream>`で行う。名前にmoveとあるが、進行中のscale up/downやretention変更も対象になる。[実装のjsClusteredStreamCancelMoveLocked](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_cluster.go)は、最初に記録したoriginへreplica数・placement・該当時のretentionとstream peer集合を戻すdesired stateを提案する。API応答はその提案適用後であり、実体の復帰完了後ではない。

- originは途中の再更新で上書きされないため、直前1操作だけのundoとは限らない
- streamは元peer集合を目指すが、consumerの最終配置は移行の進み方により元と異なり得る
- 進行中のdesired state/originがなければreconfiguration-not-in-progressのerrorとなる
- move中の別moveやscale/retention変更は受け付けない。取消して元へ戻る途中もmoveとして扱い、次の変更を待たせる
- 取消は書き込まれた業務messageを過去へ戻す操作ではない。失われた唯一のコピーを復元する機能としても扱わない

[上流のcancel test](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_super_cluster_test.go)には、移動先catch-upを停止させ、移動元の一部nodeを止めても、残る移動元にデータとquorumがある条件で取消を収束させる例がある。任意の障害数やstorage損失からのrollback保証へ拡張しない。

downgrade先もguideは2.14.7以上を推奨する。残るdesired stateは2.14で進まないが、assignmentを更新せず2.15へ戻せば再評価できる。2.14で同じconfigを送るno-op更新はdesired stateを捨てて旧動作へ戻す意味を持つため、単なる無害な再適用として自動実行しない。

## 計画保守の実行チェックリスト（独自の設計案）

以下は確認した契約を使うための手順案であり、本番で実行済みのrunbookではない。

1. **変更前の記録**: 対象server上のstreamとconsumer、R1の有無、設定replica数、実体peer、placement制約、message状態、consumer ACK進捗を採取する。期限やconsumer稼働でmessage数は変わるので、単純な前後件数一致だけをデータ検証にしない
2. **容量と版の準備**: 推奨upgrade経路を完了し、必要なreplica数を満たす適格なspareを確保する。移行中は新旧コピーが併存するため、storageだけでなくcatch-upの帯域・I/O余力も用意する
3. **小さく退避**: 最初はstream単位で置換を試し、INFOの状態遷移を観測する。まとめてserver退避する場合も、最初のasset一覧との照合を残す。新規配置や並行変更を管理する保守窓のルールも別に設ける
4. **完了を照合**: 対象stream/consumerすべてについて移行のdesired stateが解消し、旧serverが実体peerから外れ、意図するreplica数・追従状態が回復したことを確認する。既知のmessageを読めること、consumerの進捗とpublish/consumeが正常であることも確認する
5. **停滞時に原因へ戻る**: statusとerrを分類し、容量・疎通・依存assetを直す。観測期限を超えてもsuccessや時間経過で完了扱いにせず、停止を保留する。取消するならoriginへ戻れる条件を確認し、取消後も収束を待つ
6. **旧serverの停止・廃止**: asset退避とmeta membershipの処理を別の承認済み保守手順として扱う。evacuationの応答だけで電源停止・server removeを連続実行しない

検証用clusterでは、R1の退避、R3のspare不足、consumer依存待ち、移動先storage満杯、catch-up中のleader交代、取消後の次操作、mixed-versionの停滞を試験する。成功応答が出る正常系だけでは、保守の停止条件を検証できない。

## 適用版・出典・未確認事項

- serverは正式release v2.15.0、commit `eb763679aa3c24a40dcd3012aa046ad1996d851c`。releaseページの2026-09-17を確認し、tagを完全SHAへ解決した。RCやmainの実装を正式版へ混入していない
- ADRはrevision 1 / 2026-08-26を、repository commit `7dfdbb4f17c848263499eb96ad4f6ccbe810ed14`（commit日時2026-10-02）で固定した。upgrade guideには公開日の表示がなく、取得日は2026-10-03 UTC
- webでrelease・upgrade guide・ADRの実ページを開いた。ADRの固定URLと一部大きなsource fileはwebでcache missになったため、同一固定SHAのGit checkoutから全文を検証した。Editリンクはrobots拒否であり、編集経路には進んでいない
- 両repositoryのLICENSEとserver file headerはApache-2.0を確認した。release本文とlive docsの独立した再配布ライセンスは未確認のためunknownとし、独自要約だけを記載した。コードやtestのコピーはしていない
- upstream testは読んだが実行していない。任意のstorage backend・大規模clusterでの移動時間、負荷下の無停止性、client/CLI別の対応版、認証・permission設定の運用適合性は未検証
- disaster recovery向けMETA.RESCUEはquorum要件を一時的に下げる別機能であり、本稿の通常保守手順には含めない

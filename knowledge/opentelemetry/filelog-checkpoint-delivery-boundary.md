---
{
  "id": "opentelemetry-filelog-checkpoint-delivery-boundary",
  "title": "OpenTelemetry filelog: checkpoint復旧・copytruncateと配送完了の境界",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "Collector Contrib v0.162.0 (2026-09-29) at ae8c507510f48f433ab47dd1c6b01a59d6c388b5; Collector core v0.162.0 at 62cdad2ea133239380b44d20d84eb26e114779b6; source review only, verified 2026-10-05 UTC",
  "tags": ["research-domain:quality-operations", "filelog", "file_log", "checkpoint", "start_at", "copytruncate", "on_truncate", "fingerprint", "file_storage", "retry_on_failure", "delivery"],
  "sources": [
    {"id": "otel-filelog-release-0162-20261005", "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.162.0", "type": "release_notes"},
    {"id": "otel-filelog-contract-0162-20261005", "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/receiver/filelogreceiver/README.md", "type": "official_docs"},
    {"id": "otel-filelog-lifecycle-0162-20261005", "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/pkg/stanza/fileconsumer", "type": "github_repository_analysis"},
    {"id": "otel-filelog-storage-0162-20261005", "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/extension/storage/filestorage", "type": "github_repository_analysis"},
    {"id": "otel-filelog-resilience-example-0162-20261005", "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/examples/fault-tolerant-logs-collection/README.md", "type": "official_docs"},
    {"id": "otel-filelog-exporter-queue-0162-20261005", "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/README.md", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2026-11-04",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/quality-operations.json"]
}
---

# Filelogの「読んだ位置」と「届いたログ」を別々に確認する

## 問いと結論

`filelog` receiverに`storage`を設定し、Collectorが再起動後も同じoffsetから読めれば、ログを失わず一度だけ配送できたと判断してよいか。

結論は、**receiverのcheckpointはファイル読取位置の再開情報であり、backendの保存済み台帳ではない**。receiver retry、exporterのpersistent queue、元ログの保存期間、backendでの重複処理を別々に設計する。正常終了も、checkpoint保存errorや未配送分がないことを単独では証明しない。

本稿は未収録の持続的な運用上のgapを扱う。既存の[OTLPサイズ制限とretry](otlp-message-size-retry-boundary.md)は通信応答の再送可否、[巨大logのbatch分割](collector-log-batch-oversize-survivor-boundary.md)はexporter内のrecord保持、[partition cache](collector-partition-cache-eviction-boundary.md)はbatcherの退避を扱う。ここでは元ファイルから再起動後の再読までに対象を絞る。

## 適用版と調査の範囲

- Contrib [v0.162.0 release](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.162.0)の公開日時は2026-09-29 10:11:33 UTC。tag objectをcommit `ae8c507510f48f433ab47dd1c6b01a59d6c388b5`まで解決した。filelog receiverとfile_storage extensionはいずれもbeta
- exporterhelperはcore v0.162.0のcommit `62cdad2ea133239380b44d20d84eb26e114779b6`で照合した。二つのrepositoryのSHAを混同しない
- 以下は固定版の公式文書・実装・upstream testの静的確認。Collectorの起動、停電、disk full、backend障害の実測結果ではない。v0.162.0で本稿の全機能が新設・修正されたという主張もしない

## 1. checkpointが保存するもの

[receiver READMEのOffset tracking](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/receiver/filelogreceiver/README.md#offset-tracking)は、file fingerprint、byte offset、file attributesなどを保持し、`storage`未設定ならoffsetはmemoryだけにあると説明する。同じ節は、後続componentでログがdropされ得ることも明示する。

[file.goのStart / poll / Stop](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/pkg/stanza/fileconsumer/file.go)から、保存と復旧の境界は次のように読める。

1. 起動時にcheckpointをloadし、errorならStartは失敗する。保存済みmetadataをtrackerへ戻してreaderを再構成する
2. pollの通常末尾とStopにcheckpoint.Saveがある。1 recordのemitごとにdisk同期が完了する契約ではなく、任意のpoll経路すべてが保存まで到達する保証でもない
3. Saveの失敗は`save offsets`としてerror logへ出る。特にStopはこの失敗を記録した後もnilを返す。したがって上位の停止処理が成功を返したという事実だけでは、その保存の成否は分からない

さらに[reader.goのreadContents](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/pkg/stanza/fileconsumer/internal/reader/reader.go)は、emit callbackのerrorを記録した後にもOffsetを進める経路を持つ。これはretryが有効な全経路で即dropするという意味ではない。**最終的にerrorがreaderへ戻っても、自動的に未配送recordまで巻き戻す仕組みとは扱えない**という実装上の境界である。

### start_atは再開位置の強制上書きではない

保存済みの同一file metadataが見つかれば、そのoffsetを使う。`start_at: beginning`へ変更しただけで全件再読が始まるとは限らず、`start_at: end`でも停止中に追記された部分を捨てるとは限らない。

固定版Startは、loadしたoffsetが1件以上あると新規reader用の`FromBeginning`もtrueにする。このため、復旧時に未知のファイルが混ざる場合まで「end指定なので起動前の全内容をskipする」と一般化しない。[TestRestartOffsets](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/pkg/stanza/fileconsumer/file_test.go)はbeginning/endと短い/長い行の組合せで、停止中の追記が再開後に届くことを確認している。永続diskの耐久性を測ったtestではなくmock persisterを使う。

## 2. receiver retryとexporter queueは別の失敗を扱う

[receiver設定](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/receiver/filelogreceiver/README.md#configuration)と[exporterhelper設定](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/README.md)を区別する。

| 設定・段階 | 確認した契約 | そこからは言えないこと |
|---|---|---|
| receiver `retry_on_failure.enabled` | 既定false。有効なら下流error時にfile読取を止め、現在のbatchを再送する | 下流が一度成功を返した後の非同期dropまで把握する |
| receiver `max_elapsed_time` | 既定5m。期限に達するとそのbatchを破棄。0ならretryの時間制限をなくす | 元fileやdiskを無制限に保持できる |
| exporter `sending_queue.storage` | 指定storageへbatchを保持し、再起動時に残存分のexportを続ける | receiverのoffsetだけでqueueも永続化される |
| exporter `wait_for_result` | 既定falseで、request処理結果まで呼出元を待たせない | enqueue成功がbackend保存完了を表す |
| exporter enqueue拒否 | 容量やstorage errorで入らないデータはexporter retry処理へ到達しない | exporterのretry期限を延ばせば入口拒否も救える |

enqueue拒否は`otelcol_exporter_enqueue_failed_*`の観測対象である。exporter側もretryには期限があり、永久errorやretry上限到達はpersistent queueでも救えない。receiverとexporterの同名設定を一つのretry予算として説明しない。

[公式のfault-tolerant例](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/examples/fault-tolerant-logs-collection/README.md)も、offset tracking用storageと収集済みログ配送用persistent queueを別に挙げる。[同版設定例](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/examples/fault-tolerant-logs-collection/otel-col-config.yaml)はreceiverとexporterへ個別にstorage IDを接続し、両extensionをserviceで有効化する。extensionを宣言するだけでは二つの利用箇所への配線にならない。

### 二つの障害シナリオから導く設計判断

以下は上の契約から導く独自の設計上の例であり、本調査で再現した観測値ではない。

- **欠落の例**: fileから読み、下流の非永続bufferが受け付け、offsetだけ保存できた。その後backendへ届く前にCollectorが停止すると、再起動後のreaderは先へ進んだ位置から再開し得る。offset保存だけを強化しても、このbufferの内容は復元しない
- **重複の例**: backendまで届いたが、その読取に対応するcheckpoint保存前に停止した。旧offsetから再開すれば同じrecordをもう一度送る可能性がある。checkpointを保持することとend-to-end exactly-onceは異なる

設計時には、元file・reader・中間processor・queue・backendの各段階で「何を受理し、何を永続化してから成功を返すか」を記録する。`wait_for_result: true`だけで、離れた二つのstorageを原子的にcommitできると考えない。

## 3. copytruncateはcheckpointが正しくても欠落を作る

[固定版file.go](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/pkg/stanza/fileconsumer/file.go)は既知fingerprintに一致し、保存offsetが現在のfile sizeより大きい場合に`on_truncate`を適用する。開いたreader、閉じたmetadata、archiveからの復旧に同じ3択がある。

| `on_truncate` | offsetの扱い | 判断時点に存在する短いfileの内容 |
|---|---|---|
| `ignore`（既定） | 旧offsetを保持 | そのoffsetより大きくなるまで読まない |
| `read_whole_file` | 0へ戻す | 先頭から再読する。重複し得る |
| `read_new` | 判断時のfile sizeへ移す | その時点の内容をskipし、それ以後の追記を読む |

`read_new`のnewは「truncate後に書かれた全行」を意味しない。truncate後から検出までの間に書かれた行も、検出時点では既存内容なのでskipされる。この違いは[rotation_test.go](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/pkg/stanza/fileconsumer/rotation_test.go)の`TestOnTruncateReadNew`で明示されている。

[fileconsumer設計文書のKnown Limitations](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/pkg/stanza/fileconsumer/design.md#known-limitations)は、copytruncateのcopy先が監視pattern外で、読取前に元fileが縮むと未読部分を失い得ると説明する。`read_whole_file`でも既に消えたbyteは戻らない。move/createでも並行file数の上限とpattern外への移動が重なる条件に注意がある。

また同設計文書では、同じfingerprintの複数fileを一つとして扱う。fingerprintは既定で先頭1000 byteなので、同一の長いheaderを持つ別fileを区別できないことがある。ファイル名だけを変えて再投入しても、新規データとして必ず取り込むとは言えない。receiver READMEは`fingerprint_size`を小さくすると再取り込みを招く場合も記載しており、変更はoffset移行として試験する。

## 4. file_storageの復旧は元状態の復元と同義ではない

[file_storage README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/extension/storage/filestorage/README.md)と[factory.go](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/extension/storage/filestorage/factory.go)の確認結果:

- `fsync`は既定false。trueならDB writeごとにfsyncを行う設定であり、性能とのtrade-offがある。disk・filesystem・電源断条件を問わない無損失保証には拡張しない
- storageのファイル名は利用componentのtype/nameから作られる。設定のreceiver名やexporter名、directoryの変更を、保存状態へ影響しないrenameだと決めつけない
- `recreate`は特定のDB破損panic時に旧fileをbackupへ退避し、新しいDBで稼働を続ける。旧checkpointや未配送batchを自動的に復元する機能ではなく、重複やcomponent state喪失があり得る
- queue用storageが枯渇すればenqueueを拒否し得る。offset用storageへのwrite失敗とqueue用storageへのwrite失敗を、一つの「永続化error」だけで集計すると復旧判断ができない

独自の運用案として、再配備時はvolume、mount path、所有権、component IDを差分確認する。backendが復旧してqueueが空になっても、checkpoint保存errorが続いていれば再起動時のreplayは別に残る。自動再作成の成功をログ欠落なしの証拠にせず、退避fileを使った調査・復旧は別手順にする。

## 5. 受入試験と観測の設計案

実ログに近いサイズ・rotation周期で、合成した一意IDと連番を各行に付ける。入力側の生成記録とbackendで取得できるID集合を照合し、件数だけで欠落と重複が相殺されることを防ぐ。以下は未実行の試験案である。

1. **復旧位置**: 初回beginning/end、保存済みoffsetあり/なし、停止中の追記、停止中に新規file作成を分ける。設定値だけで期待する読取範囲を決めない
2. **保存error**: offset storageへのwriteを失敗させ、`save offsets`と再起動後の重複を照合する。停止APIの成功と保存成功を別々に記録する
3. **配送error**: backend停止、queue満杯、queue disk full、receiver retry期限超過を別々に注入する。enqueue拒否と送信失敗、file内の未読backlogを区別する
4. **停止位置**: 通常停止とhard killを分け、offset保存前後・enqueue前後・backend受理前後で試験する。同一の合成IDが欠落するか複数届くかを調べる
5. **rotation**: copy先がinclude内/外、同じfingerprint prefix、検出前に追記した短いfile、file数が並行上限を超える条件を組み合わせる。read_newとread_whole_fileで期待値を変える
6. **運用変更**: component ID、volume、fingerprint_size、recreate、checkpoint encodingを変更する前に、小さな保存状態を使って再開範囲を照合する。保存fileを直接改造する試験を本番で行わない

[receiverが公開する](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/receiver/filelogreceiver/README.md#telemetry-metrics)`otelcol_fileconsumer_open_files`と`otelcol_fileconsumer_reading_files`はfile消費の状態であり、backendが受理したrecord数ではない。これらが平常でも、保存error・queue拒否・合成IDの欠落を独立に確認する。

長期障害を許容するなら、receiver retryを0へする判断と同時に、元fileの保持容量・rotationによる削除時刻・復旧後の追いつき速度を決める。無期限retryは無期限保管の代替ではない。重複を許容できない用途では、安定したevent IDとbackend側の冪等な取り込み契約が必要かを検討する。

## 版の差・制約・未確認事項

- `filelog.protobufCheckpointEncoding`はreceiver README上でv0.148.0にalpha、v0.156.0にbeta・既定有効。固定版はgate設定にかかわらずJSON/protobufの両checkpointを読める。ただし、古い任意のCollector版へのdowngradeも安全という意味ではない。形式互換と配送保証を分ける
- 読取の中心は通常の非圧縮log。gzip、複雑なmultiline、header parser、delete_after_read、platformごとのfile handle、全storage backendの整合性は検証していない
- upstreamの`TestRestartOffsets`、`TestOnTruncateReadWholeFile`、`TestOnTruncateReadNew`、`TestOnTruncateIgnore`、`TestCopyTruncateResetsOffsetOnRestart_IdenticalFirstKB`を読んだ。mock persisterや手動metadata loadによるtestを、実diskのcrash recovery testとは扱わない。これらupstream testは実行していない
- native webではtag版receiver・filestorage・exporterhelper・releaseページを開き、固定SHAの本文はGitHub connectorでも再確認した。一部SHA/raw URLはweb取得失敗のためconnectorで確認した。mainだけの記述を固定版の根拠として採用していない
- 両repositoryの固定版LICENSEはApache-2.0。読んだGo実装のSPDXも確認した。本文は独自の日本語要約・設計案であり、upstream実装や設定例の転載・module昇格は行っていない

検索evalは本稿を取得できることの確認であり、ログ配送の無損失・重複排除・本番設定の妥当性を証明するものではない。

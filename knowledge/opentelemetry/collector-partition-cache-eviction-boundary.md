---
{
  "id": "opentelemetry-collector-partition-cache-eviction-boundary",
  "title": "Collector v0.162.0 partition cache: eviction flush・idle churn・tenant分離と完了の境界",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "OpenTelemetry Collector v0.162.0 / v1.68.0 (2026-09-28) @ 62cdad2ea133239380b44d20d84eb26e114779b6; compared with v0.161.0 @ 0bf928af5487d3c4e0b4174eabb7ba075c322517; source review only",
  "tags": [
    "research-domain:quality-operations",
    "Collector",
    "exporterhelper",
    "sending_queue",
    "metadata_keys",
    "partition",
    "cache_size",
    "idle_timeout",
    "LRU",
    "num_consumers",
    "tenant",
    "eviction",
    "backpressure"
  ],
  "sources": [
    {
      "id": "otel-partition-release-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.162.0",
      "type": "release_notes"
    },
    {
      "id": "otel-partition-readme-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/README.md",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-config-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/config.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-multi-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/multi_batcher.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-partition-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/partition_batcher.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-multi-tests-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/multi_batcher_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-partition-tests-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/partition_batcher_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-metadata-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/metadata_partitioner.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-metadata-tests-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/metadata_partitioner_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-telemetry-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/metadata.yaml",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-wiring-0162-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/tree/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-partition-prior-0161-20261004",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/tree/0bf928af5487d3c4e0b4174eabb7ba075c322517/exporter/exporterhelper/internal/queuebatch",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# Partition cache の上限を、配送保証や tenant 制限と取り違えない

## 問いと採用判断

Collector の tenant 別 batching で `cache_size` を小さくしたとき、あふれた tenant の telemetry は捨てられるのか。cache が上限以下なら、メモリ・送信完了・tenant 分離まで保証できるのか。

[2026-09-28 公開の v0.162.0 / v1.68.0](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.162.0) で確認した答えは、**cache の eviction は既存 partition を flush して外す経路であり、新しい tenant を拒否する定員制ではない**。ただし flush の開始と backend の保存完了は異なる。worker pool の待機、選ぶ metadata、idle による再作成も別に評価する。

本稿の判断は「partition 数を制限して終わりにせず、pending batch を持つ eviction、metadata の欠落、送信完了を受け入れ条件にする」。根拠は固定 commit `62cdad2ea133239380b44d20d84eb26e114779b6` の静的読解であり、Collector 実機試験結果ではない。

既存の [巨大 log の分割と後続保持](collector-log-batch-oversize-survivor-boundary.md) は1 request 内の record の生存範囲を扱う。本稿は metadata ごとの batcher の寿命・容量・共有 worker を扱い、巨大 record の修正を再説明しない。[tail sampling の decision cache](tail-sampling-late-span-decision-cache.md) も sampling 判定の記憶であり、この sending queue の LRU とは用途が異なる。

## 版と設定の契約

[同版 README](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/README.md) と [config.go](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/config.go) で次を確認した。

| 設定・対象 | v0.162.0 での意味 | 別に評価するもの |
|---|---|---|
| `sending_queue.batch` | batching は既定で無効。`batch: {}` で既定設定を有効化 | queue 自体の有効・無効 |
| `batch.partition.metadata_keys` | 選択した client.Metadata の値の組合せで partition を分ける。空なら metadata による multi-batcher を使わない | resource 属性、認証主体、送信先 tenant の検証 |
| `batch.partition.cache_size` | LRU に保持する partition batcher 数。既定10000、正値必須 | queue のデータ量、累積 tenant 数、全 heap 量 |
| `batch.partition.idle_timeout` | 空の partition を削除する idle 判定。既定90s、正値必須 | batch の送信期限や retry の時間予算 |
| `batch.flush_timeout` | 未達の batch を送る timer。既定200ms、正値必須 | partition の idle 保持期間 |
| `sending_queue.queue_size` と `sizer` | queue 側の容量とその単位 | partition 数という別単位 |
| `sending_queue.num_consumers` | queue consumer 数に加え、batcher の共有 worker pool の容量に渡る | tenant ごとの専用枠や公平性 |

`cache_size: 0` を無制限、`idle_timeout: 0s` を無効化として設定しない。固定版の `PartitionConfig.Validate` はどちらも0以下を拒否する。Go の独自 component が `BatchConfig` を組み立てる場合も、ゼロ値 literal が既定値へ自動変換されるとは考えず `NewDefaultBatchConfig` から明示変更する。configuration の省略に適用される default と、構築済み struct の0を区別する。

### 旧版は「cache が無制限」だったわけではない

[比較した v0.161.0 の実装](https://github.com/open-telemetry/opentelemetry-collector/tree/0bf928af5487d3c4e0b4174eabb7ba075c322517/exporter/exporterhelper/internal/queuebatch) は、すでに LRU 容量を10000に固定し、eviction 時の shutdown callback を持っていた。v0.162.0 の変更を「初めて有限になった」と説明すると誤る。今回確認できる差は、容量を設定可能にしたこと、現在値と容量の gauge を追加したこと、idle 時間を独立した設定にしたこと。

v0.161.0 の idle 閾値は `10 * flush_timeout` なので、既定200msなら2秒だった。v0.162.0 の90sは、たとえば60秒おきの到着で毎回作り直す churn を減らす方向の変更である。全旧版・vendor patch・後続版は比較していない。

## eviction は何をし、何を保証しないか

### LRU の外へ出すことと、データの完了通知は別

[multi_batcher.go](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/multi_batcher.go) の `getPartition` は、key があればその batcher を取得し、なければ新規作成して LRU に追加する。容量によって追い出された旧 batcher には、同じ共有 pool を通して `shutdownInternal` を実行する callback が呼ばれる。新しい key 自体を拒否したり、旧 partition のデータを新しい tenant の batch に移したりする処理ではない。

[partition_batcher.go](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/partition_batcher.go) の shutdown は batcher を inactive にし、最後の pending batch があれば flush する。flush が呼ぶ `consumeFunc` の結果を `done.OnDone` へ渡す。すでに参照を取得した呼出しが inactive な batcher に到達した場合も、通常の timer 待ちに残さず flush する分岐を持つ。

ここから分かるのは保持データを送出する意図と呼出し経路である。exporter の失敗、retry の終了、queue 投入前の拒否、process の異常終了をすべて解決した証拠ではない。LRU から消えた時刻には、shutdown や送出がまだ進行中の可能性がある。「cache から除去済み」を「backend 保存済み」の代用にしない。

容量2に対して A、B、C と異なる key を投入すると、古い A を外して B、C を保持する形になる。A が再来すれば再作成の対象となるため、累積3 tenant を2 tenantへ制限する admission control ではない。保持する key の数を制御しても、値の長さ、pending data、送出中の request、timer・goroutine などを含む総メモリの byte 上限にはならない。

### worker を使う eviction 自身が、次の worker を必要とする

重要な静的観察がある。上記の eviction callback は `workerPool.execute(shutdownInternal)` を呼ぶ。この `execute` は token を取得してから goroutine を起動する。一方、shutdown した partition に pending batch があると、その flush も同じ pool の `execute` を呼ぶ。さらに [queue_batch.go / batcher.go](https://github.com/open-telemetry/opentelemetry-collector/tree/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch) では、その pool の大きさに `NumConsumers` を渡している。

[upstream の `TestMultiBatcher_CacheSizeEviction`](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/multi_batcher_test.go) は、この二段階の worker 必要数をコメントで説明して、worker を2にしている。設定 validator の `num_consumers` 条件は正値であり、partition eviction 用に2以上を要求する guard はこの固定実装にはない。

したがって **`num_consumers=1` で pending batch を持つ partition の eviction がこの経路を通る場合**、shutdown が唯一の token を保持したまま内側の flush の token を待つ、という待機関係が生じ得る。これは固定 source とテストの意図に基づく、条件付きの停止リスクである。本調査では実行して再現しておらず、全構成で production deadlock が起きるとは断定しない。

また、2にすればあらゆる並行 eviction が安全という保証も導けない。`getPartition` は共有 lock の内側で LRU を更新し、その callback は worker の空きを待ち得るので、churn と遅い exporter の組合せは tenant 間の待ち時間へ波及し得る。cache を極端に縮める変更や consumer の削減は、実際の pool 数・複数 eviction・backend 遅延を組み合わせた有界の負荷試験を採用条件にする。

## idle_timeout は配送期限ではない

固定版の idle 判定は、timer が動き、`currentBatch == nil` であり、`lastDataTime` から `idle_timeout` 以上経過した時に removal callback を呼ぶ形である。`lastDataTime` は Consume のほか、pending batch を flush に取り出す時にも更新される。期限専用の独立 timer で90秒後ちょうどに破棄する仕組みとは異なる。

データが残る時は先に batch を flush し、空になってから idle の条件を調べる。この90sを「受信後90秒待ってから送る」「90秒で未送信データを捨てる」「90秒まで backend 到達を保証する」と読まない。送信を促す200msの `flush_timeout` と idle 保持90sを分けて観測する。

独自の調整方針としては、平常の到着間隔だけでなく jitter と短い停止を含めた余裕を取る。60秒の scrape を90sなら常に保持できると一般化せず、LRU の容量圧力で先に外れるケースも含める。idle を長くすれば batcher の再利用機会は増えるが、既知 key が cache に残る時間も増える。容量と idle の片方だけを性能つまみとして動かさない。

## metadata による batching は tenant の認証ではない

[metadata_partitioner.go](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/metadata_partitioner.go) が読むのは `client.FromContext(ctx).Metadata` である。telemetry の resource 属性を自動的に tenant として読むわけではない。選んだ key に値がなければ key の構築から省略し、全部欠落なら空の partition key になる。つまり tenant metadata が欠落した複数 request が、欠落を理由に拒否されず同じ分類になる経路を持つ。

「空」の意味にも注意する。値リストが空の `[]` は欠落と同様に省略されるが、空文字を1個含む `[""]` は長さ1なので key に反映され、欠落とは異なる。これらと正常 tenant ID を別ケースにする。設定名の重複検査は大文字小文字を区別しないが、metadata の存在・信頼性を検証する機能とは別である。

同じファイルの `NewMetadataKeysMergeCtx` は、選んだ metadata key だけを最初の context から取り出して新しい context を作る。[upstream tests](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/metadata_partitioner_test.go) にも、`other` のような未選択の metadata は merge 後に残らないケースがある。単一 request が通った試験だけで「未選択 header も常に送出へ引き継がれる」と結論しない。

独自の設計判断として、tenant の認証・許可・欠落時の扱いは入口で別途定義する。backend routing に必要な metadata が receiver から当該 context まで届くか、batch merge 後にも必要項目が残るかを synthetic data で確認する。高 cardinality な request ID を partition key に無制限に追加して header 保持を解決しようとすると、別の churn 問題を作る。cache_size は不正 tenant を防ぐ security boundary ではない。

## 新しい gauge から言える範囲

[telemetry 定義](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/metadata.yaml) は次の2つを、partition 単位の非同期 integer gauge として定義する。安定度は development で、partitioning 有効時だけ記録する。

- `otelcol_exporter_queue_batch_partition_cache_size`: 現時点の LRU entry 数
- `otelcol_exporter_queue_batch_partition_cache_capacity`: 設定した LRU 容量

固定 multi-batcher の callback は `exporter` と `data_type` を付け、size には lock 下の `Len()`、capacity には設定値を渡す。tenant 別の delivery 状態や eviction 回数はこの2値には含まれない。

独自の読み方として、size/capacity が高い状態は容量圧力を調べる入口にはなる。しかし size=capacity のまま同じ partition が再利用されている場合と、毎回 key が入れ替わる場合を、この比率だけでは区別できない。低い値も、健全な余裕、短すぎる idle、metadata 欠落で1分類へ合流した状態のいずれでもあり得る。gauge が下がったことを loss、上限以下を配送成功と判定しない。

監視では cache、queue、実際の送信量・送信失敗、到着遅延、process 資源を組み合わせ、synthetic tenant ごとの入力IDと backend 到着IDを照合する。alert の閾値や観測周期は採用先で決める。ここでは未提供の eviction counter や、metric 名の将来互換性を仮定しない。

## 受け入れ試験案と upstream が実際に assert していること

以下は独自の検証案であり、検索 eval や upstream test の読了を runtime 合格に置き換えない。

| 入力・状況 | 採用先で確かめる結果 |
|---|---|
| 容量2、A/B/C の3 key、各 batch は min_size 未満 | LRU entry の入替だけでなく、追い出された A のIDが送出・完了すること |
| A が eviction 後に再来 | 新しい A の分類でも B/C とデータを混ぜず、必要 metadata と到着IDが保たれること |
| 同一 key の継続、60秒ごと、idle 閾値越えの停止 | batching timeout と idle removal の時刻を別々に測り、再作成頻度を比較すること |
| tenant 欠落、空リスト、空文字、正しい tenant ID | 入口で定めた拒否・分類と一致し、欠落が単一 partition に集まることを見逃さないこと |
| 選択外 header にだけ routing 情報がある | 2 request 以上の merge 後も採用要件を満たすか。単発試験との差を調べること |
| consumer=1、実際の運用値、複数 eviction、遅い exporter | 試験全体に外部 deadline を置き、token 待ち・未完了ID・停止時の排出を調べること |
| cache gauge が容量で一定、入力 key は入替 | 安定した hit と激しい churn を、gauge だけで取り違えないこと |

`TestMultiBatcher_CacheSizeEviction` が直接 assert しているのは、容量2で p1/p2/p3 を入れた際に count が2となり、p1がなくp2/p3があること。テストは2 worker を使うが、このケースに backend 到着IDや全件送出数の assertion はない。別の [TestPartitionBatcher_Shutdown](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/partition_batcher_test.go) は単体 shutdown で3 itemsが1 requestへ送られ、2つの Done が成功することを検査する。この2つを合わせて読む価値はあるが、LRU 圧力下の実 exporter と同じ試験ではない。

cache metrics のテストは size=2、capacity=5 と exporter/data_type 属性を確認する。idle removal のテストは短い timer で flush 後に partition 数が0になることを確認する。どれも本調査では実行していない。負荷性能、共有 pool の全 interleaving、クラッシュ後の復元、backend 永続化や重複排除、distribution 固有の receiver 設定は未確認である。

## 取得・出典・ライセンス

取得日は2026-10-04 UTC。release ページと v0.162.0 tag の exporterhelper README を native web で開いた。release 公開時刻は GitHub REST の `2026-09-28T14:09:31Z`、annotated tag の参照先 commit は上記40桁で照合した。固定 source の native web HTML/raw は cache miss となったため、公式 raw URLを read-only 取得して実装・テスト・LICENSE を読んだ。取得失敗したページを読了とはしていない。

[固定版 LICENSE](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/LICENSE) と対象 Go ファイルの SPDX は Apache-2.0。GitHub release 本文そのもののライセンスは別途確認できていないため、その source record は unknown とする。独自の日本語要約と設計・試験提案のみを追加し、release 本文や OSS コードはコピーしない。source catalog は今回の取得記録を新規追加し、同releaseを扱う既存文書や旧取得記録の日付は変更しない。本稿は構成変更の提案を評価する資料であり、Collector の修正実装や、後続 release がこの停止リスクを解消したとの主張は含まない。

---
{
  "id": "opentelemetry-tail-sampling-late-span-decision-cache",
  "title": "OpenTelemetry tail sampling: late span・decision cache・early eviction と v0.162.0 の threshold 修正",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "Collector Contrib v0.162.0 (published 2026-09-29), tailsamplingprocessor beta, pinned ae8c507510f48f433ab47dd1c6b01a59d6c388b5; verified 2026-10-02 UTC",
  "tags": [
    "research-domain:quality-operations",
    "tail_sampling",
    "late-spans",
    "decision_cache",
    "decision_wait",
    "num_traces",
    "eviction",
    "tracestate",
    "threshold",
    "trace-affinity"
  ],
  "sources": [
    {
      "id": "otel-tail-sampling-v0162-release-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.162.0",
      "type": "release_notes"
    },
    {
      "id": "otel-tail-sampling-v0162-implementation-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-tail-sampling-scaling-guide-20261002",
      "url": "https://opentelemetry.io/docs/collector/scaling/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# Tail sampling の「判定済み」と「完全な trace」を分ける

## 問いと追加理由

「ERROR を含む trace は残す」と設定したのにエラー span が欠ける、同じ trace の後半だけ保存される、採用 span の sampling threshold が一致しない。この3つを、待機時間・容量・判定履歴・版の違いへ切り分ける。

調査の契機は [Collector Contrib v0.162.0 のリリース](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.162.0)。2026-09-29 に公開され、採用済み trace の late-arriving spans に元の threshold を適用する修正 #50623 が含まれる。これは late span 自体を新しくサポートした変更でも、過去に欠けた span を復元する変更でもない。

既存の [ParentBased / TraceIdRatioBased](sampling-parentbased-traceidratio.md) は SDK での生成時判定、[BatchSpanProcessor](batch-span-processor.md) は SDK の送信 queue、[OTLP retry](otlp-message-size-retry-boundary.md) は配送失敗の再送境界を扱う。本稿は Collector が trace ごとに保持する判定状態と、その消失を対象にする。metric cardinality の属性欠落とも別の問題である。

## 固定版で確認した契約

対象は `processor/tailsamplingprocessor` の v0.162.0。trace signal の stability は beta。以下の通常経路は `sampling_strategy: trace-complete`、`num_shards: 1`、既定の in-memory tail storage を前提とする。異なる strategy・shard・storage は後述の制約を加える。

### decision_wait は完了証明ではない

[README・factory.go・config.go](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor) による既定値は `decision_wait: 30s`、`num_traces: 50000`、両 decision cache size は0、`sampling_strategy: trace-complete`。`expected_new_traces_per_sec` はデータ構造の割当てに使う予測値であり、入力量を制限する rate limiter ではない。

[processor.go の processTrace / samplingPolicyOnTick](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor/processor.go) は初回受信時に trace ID を decision batcher へ登録し、timer でその時点の蓄積データを評価する。`trace-complete` という名前は「すべての span の到着を確認した」という保証ではない。任意のサービスで span が遅延すれば、ERROR・最長duration・属性が判定時に未到着である可能性は残る。

`decision_wait_after_root_received` を正の値にすると、root span の受信を契機に timer 処理を早められる。root の受信も全子spanの到着証明ではない。処理は timer と batch に依存するため、指定秒数ちょうどの判定 deadline とも解釈しない。

### late span の分岐は「前回の状態を覚えているか」

late span は、trace の sampling decision 後に到着した span。判定後の経過時間だけで一律に捨てる TTL モデルではない。固定版の [processCachedTrace / processTrace / releaseSampledTrace / releaseNotSampledTrace](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor/processor.go) では次の順になる。

| 到着時に残る状態 | 固定版の通常経路 | 運用上の意味 |
|---|---|---|
| sampled decision cache の hit | policy を再評価せず採用側へ渡す | 元の採用判断を維持する |
| non-sampled decision cache の hit | policy を再評価せず採用側へ渡さない | 遅れて ERROR が来ても、その trace を再採用する仕組みではない |
| cache miss、trace map に判定済み状態あり | 保存済み decision を継承する | late span の内容では元の判断を変えない |
| cache miss、trace map にも状態なし | 新しい trace と同様に蓄積・判定する | 同一 trace ID の別部分が別判定になる可能性がある |

`sampled_cache_size` と `non_sampled_cache_size` は別々の LRU 容量で、0は無効。cache は span 本文の保存先ではない。採用または不採用が cache に保持された場合、実装は trace map の状態を解放する。採用した trace だけに cache を用意しても、不採用側の再判定は抑えられない。

LRU には「必ずN秒覚える」という設定はない。[cache/lru_cache.go とテスト](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor/cache) は容量超過で least-recently-used key を追い出し、hit が使用順序を更新することを示す。定常的な新規decision数だけで保持時間を保証しない。また、この実装の内部keyは trace ID の後半64 bitであり、全128 bitの照合ではない。独自ID生成器や試験fixtureで後半を固定しない。衝突確率や実運用への影響は本調査では測定していない。

## v0.162.0 の threshold 修正が効く範囲

`processor.tailsamplingprocessor.usetracestate` はこの版で alpha・既定off。明示的に有効にした場合、採用traceの W3C `tracestate` にある OpenTelemetry `ot` の `th` を有効thresholdで更新する。通常の採否継承と、この確率情報の継承は別の確認項目である。

固定版 [processor.go](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor/processor.go) と [cache/types.go](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor/cache/types.go) は、元の有効thresholdを trace map の `FinalThreshold` と cache の `DecisionMetadata.Threshold` に保持する。late span が map 経由でも sampled cache 経由でも、この値を再利用する。複数policyが採用へ投票した場合の policy名も、有効thresholdを選んだpolicyに対応させる。

[processor_decisions_test.go](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor/processor_decisions_test.go) の次のケースを静的に確認した。本調査で upstream test を実行したという意味ではない。

- `TestLateArrivingSpanUsesDecisionCacheThreshold`: 元の50%相当の `th:8` が cache 経由のlate spanにも残り、policyを再評価しない
- `TestLateArrivingSpanBeforeEvictionUsesEffectiveThreshold`: cacheなしでも、mapに残る採用済みtraceから同じ有効thresholdを再利用する
- `TestSamplingMultiplePoliciesWithDifferentThresholds_WithRecordPolicy`: 25%と50%のpolicyが採用を返す例で、緩い50%側のthresholdとpolicy名が対応する

cacheテストは同じ Go cache object を別processorへ注入している。通常のプロセス再起動後もcacheが永続化・共有されることの証明にはならない。既定実装は各processor側で新しいLRUを生成する。

さらに [WriteEffectiveThreshold](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor/internal/sampling/util.go) は、受信spanに既に厳しいthresholdがあれば弱めず、parseできないtracestateは更新をスキップしてカウントする。「元の有効thresholdを適用する」と「全spanの文字列を無条件に同じthへ書き換える」は異なる。`sample_on_first_match` とこのfeature gateの併用は、後続policyのより緩いthresholdを評価しない可能性があるため、READMEは避けるよう注意している。

## 容量と遅延を調整する順序（独自の設計提案）

以下は上記実装から組み立てた運用判断であり、公式の推奨サイズや実測値ではない。

1. まず trace affinity を確認する。[Collector scaling guide](https://opentelemetry.io/docs/collector/scaling/#scaling-stateful-collectors) は、同一traceのspanを同じCollectorへ送るためのload-balancing exporter層を説明する。単なるround-robinでtail samplerを増やさない。DNS更新時は前段間でbackend一覧の認識が一時的に違い得る。cacheを増やしても別instanceへ行ったspanの判断履歴は復元しない。
2. 判定までの蓄積時間と、判定後の記憶時間を別予算にする。`decision_wait` を長くすれば遅いERRORを評価対象に含められる可能性が上がるが、未判定データの滞留も長くなる。cacheは判断を長く覚えるためのもので、未到着の情報を補完したり、すでに決定前にevictされたspan本文を復元したりする機構ではない。判定済み状態の早期解放で容量圧力を減らし得る効果とは区別する。
3. `num_traces` はtrace数の上限であり、span数・bytes・heap上限ではない。1 traceが非常に大きい場合は、`maximum_trace_size_bytes` と `traces_dropped_too_large` を別に確認する。このサイズは実装が計上するprotobuf ResourceSpansのsizeであり、プロセスの実メモリと同値ではない。
4. 既定の `block_on_overflow: false` では、新規traceを入れる空きがないと古いtrace状態をevictする。判定前なら、後でそのIDの評価時刻が来ても元データがない。trueは空き待ちへ変えるが、上流の待機・queue・timeoutの問題も含めた負荷試験が必要であり、無損失保証として採用しない。
5. cache容量は採用・不採用それぞれの新規decision率とlate到着分布で決める。configコメントは `num_traces` より少なくとも一桁大きい容量を目安にしているが、LRU churn・偏り・burstを測らず秒数へ換算しない。

単純化した見積り例として、1 instanceへ新規traceが毎秒2,000、実効判定待ちが30秒なら、判定待ちだけで約60,000 trace相当になる。既定50,000を十分とは判断できない。これは定常状態で「到着率×滞留時間」を計算した独自例であり、timer遅延・判定後の状態・traceサイズ・burst・偏りを除外している。メモリ見積りや本番設定値へそのまま転用しない。

## メトリクスで見える範囲と見えない範囲

[metadata.yaml](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/processor/tailsamplingprocessor/metadata.yaml) と呼出し箇所を照合すると、名前だけから全欠損を数えられるとはいえない。下記は `otelcol_processor_tail_sampling_` から始まる計装名のsuffix。backendでのprefix・suffix変換と内部telemetry設定は実環境で確認する。

| suffix | 確認できること | 取り違えないこと |
|---|---|---|
| `sampling_trace_dropped_too_early` | 判定timerで必要なtraceが見つからない事象を追う | end-to-endの全span欠落数ではない |
| `sampling_trace_removal_age` | 初回到着からmap削除までの秒数 | cache移行による通常解放も含むため、全削除をearly eviction扱いしない |
| `sampling_decision_timer_latency` | sampling tickの処理時間 | 同期的な次段consumer処理も含み、policyのCPU時間だけではない |
| `sampling_late_span_age` | mapに残る判定済みtraceのlate到着までの秒数 | cache hit経路はこのhistogramを更新しない |
| `early_releases_from_cache_decision` | cache hitで直ちに処理したspan数、sampled別 | cache miss数・eviction数・全late span数ではない |
| `count_spans_with_unparseable_tracestate` | th更新時にparse不能でskipしたspan数 | samplingそのもののdrop数ではない |

`sampling_late_span_age` は `processTrace` 呼出しに対する記録で、1回に複数spanを扱い得る。`count_spans_sampled` はpolicy別の計数で、さらにalpha feature gateの有効化が必要。そのまま割り算して「全late spanの割合」と解釈しない。特にcache有効化後にlate ageが減っても、到着遅延そのものが改善したとは限らない。この点はREADMEの簡易監視例だけに依存せず、固定版の計装箇所を確認した結果である。

## 受け入れ試験案（未実行）

小さな容量と無害な合成spanで、送信したtrace ID・span ID一覧を採用後の保存結果まで照合する。

1. ERROR spanを判定前、判定直後、trace map削除後に遅延させる。cacheなし・sampledのみ・non-sampledのみ・両方で、初回decisionとlate部分の採否を比較する。採否が揃うことと全spanが残ることを別判定にする。
2. LRU容量を小さくして既知のIDを追い出す。cache hit中は再評価しないこと、eviction後に同じIDのlate部分が新しい判定を受け得ることを確認する。時間待ちだけでevictionしたと判断せず、異なるID投入とhit順を制御する。
3. `usetracestate` を明示的に有効にし、採用済みmap経由とsampled cache経由を分けて `th` を照合する。既に厳しいth、parse不能なtracestate、複数policy、gate offも別fixtureにする。v0.162.0での修正を、欠けたspanの復元や全backendの統計補正保証と混同しない。
4. `num_traces`を超えるburst、巨大trace、遅い次段consumerを別々に注入する。early eviction、サイズdrop、timer遅延、上流backpressureを分け、監視の分母と実際の欠落spanを突き合わせる。
5. replica追加・削除、DNS更新のずれ、プロセス再起動、graceful shutdownを試す。decision cacheは既定では共有永続化されず、shutdown時は `drop_pending_traces_on_shutdown` により未判定を捨てるか、その時点のデータで処理するかが変わる。どちらも完全traceの保存保証ではない。

## 適用限界・版・provenance

- v0.162.0のannotated tagをたどり、release commitを `ae8c507510f48f433ab47dd1c6b01a59d6c388b5` に固定した。release APIの `published_at` は `2026-09-29T10:11:33Z`。取得日は2026-10-02 UTC。release_notesの30日TTLに合わせて再確認期限を2026-11-01とする。
- `span-ingest` は受信batchごとに評価し、過去batchを再評価しない。pendingはcleanup時に不採用へ確定し、stateful policyを受け入れない。上記の「待って蓄積全体を再評価する」説明を流用しない。`num_shards > 1` ではtrace数・cache容量等がshardへ分割されるため、総数に空きがあっても一部shardが先に上限へ達し得る。trace affinityはCollector間にも引き続き必要。
- `tail_storage` はalpha gateが必要な別機構。本稿は外部storageの耐久性、restart recovery、複数instanceでのcache共有、`adaptive_tail_sampling` processorを検証していない。通常LRUのテストを永続化の証拠にしない。
- READMEのDropped Traces節には、`num_traces`増加と`decision_wait`短縮の両方がメモリを増やすという記述がある。本稿はこれを一般的な容量式として採用せず、span本文・未判定数・判定後状態・cacheを分けた実測を要求する。
- native webでreleaseページとscaling guide、およびupstream mainのREADMEを開いて確認。full-SHA URLのnative web取得はcache missになったため、GitHub REST APIでtagを解決し、raw.githubusercontent.comの同SHAファイルを直接HTTP取得して全文を静的解析した。mainの内容を固定版の代用にはしていない。scaling guideの表示更新日は2026-09-10で、非推奨aliasの置換による文書更新。特定distributionの挙動保証には用いない。
- OpenTelemetry Authorsの資料を日本語で独自要約した。固定commitの[LICENSE](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/ae8c507510f48f433ab47dd1c6b01a59d6c388b5/LICENSE)とGoファイルのSPDXはApache-2.0。サイトの[LICENSE](https://raw.githubusercontent.com/open-telemetry/opentelemetry.io/main/LICENSE)はCC-BY-4.0。コード転載・依存追加・module昇格は行っていない。表、数値例、採用手順と試験案は独自の整理。
- 未確認: 採用distributionのバックポート、実際のthroughput・heap使用量・LRU保持時間、custom cacheのmetadata保持、upstream単体テストの実行結果、backendでのth利用・query名・完全性照合。静的解析と検索evalの成功を、これらの動作検証済みという意味にしない。

---
{
  "id": "opentelemetry-metric-temporality-reset-conversion",
  "title": "OpenTelemetry Metrics の temporality と reset: Delta/Cumulative 変換で rate を壊さない条件",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "OpenTelemetry Specification 1.61.0（2026-10-01確認）; temporality は Stable、reset/gap と変換例は Development",
  "tags": [
    "research-domain:quality-operations",
    "metrics",
    "temporality",
    "delta",
    "cumulative",
    "reset",
    "single-writer",
    "metricreader"
  ],
  "sources": [
    {
      "id": "otel-metrics-data-model-1-61-20261001",
      "url": "https://opentelemetry.io/docs/specs/otel/metrics/data-model/",
      "type": "official_docs"
    },
    {
      "id": "otel-metrics-sdk-reader-1-61-20261001",
      "url": "https://opentelemetry.io/docs/specs/otel/metrics/sdk/",
      "type": "official_docs"
    },
    {
      "id": "otel-metrics-supplement-1-61-20261001",
      "url": "https://opentelemetry.io/docs/specs/otel/metrics/supplementary-guidelines/",
      "type": "official_docs"
    },
    {
      "id": "otel-spec-release-1-61-20261001",
      "url": "https://github.com/open-telemetry/opentelemetry-specification/releases/tag/v1.61.0",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# OpenTelemetry Metrics の temporality と reset

## 調査した問いと現在の適用範囲

Counter の値が下がったとき、プロセス再起動・収集欠落・別 writer の衝突をどう区別するか。SDK → Collector → backend の途中で Delta/Cumulative を変換しても、二重加算や架空の rate を作らない条件を整理する。trace sampling と BatchSpanProcessor は対象外。

2026-10-01 に公開仕様の表示 **1.61.0** を確認した。これは既存機能を現行仕様で再確認した記録であり、新機能の紹介ではない。[1.61.0 release](https://github.com/open-telemetry/opentelemetry-specification/releases/tag/v1.61.0) の Metrics 変更には周期 Reader の `maxExportBatchSize` 安定化などがあるが、本稿の reset 規則を新設・安定化した根拠にはしない。言語 SDK や Collector の実装版をこの番号から推定しない。

## 確認できた事実

### 時間窓と writer を値と一緒に読む

`Sum`・`Histogram`・`ExponentialHistogram` は temporality を持つ。Cumulative は同じ開始時刻からの集計、Delta は前回以降の時間窓である。`TimeUnixNano` は観測時刻、`StartTimeUnixNano` は系列の開始や連続性を識別するために重要で、両者を捨てた値だけの変換では再起動を扱えない。OTLP は一系列につき一つの論理 writer を要求する。別インスタンスを同じ Resource・Scope・名前・属性の系列へ混ぜると、見かけの reset が起こりうる。[Data Model: Temporality / Single-Writer](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#temporality)

### reset と gap の成熟度を混同しない

1.61.0 の `Temporality` と `Single-Writer` は **Stable**、`Resets and Gaps` と `Stream Manipulations` は **Development**。後者の説明では、新しい連続区間の最初の点が `StartTimeUnixNano < TimeUnixNano` なら既知開始時刻の true reset、両者が等しければ開始時刻不明の zero-duration reset と扱う。不明開始の最初の値をそのまま rate の増分にしない。これらは現時点の仕様案として参照し、全 backend の実装保証にしない。[Data Model: Resets and Gaps](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#resets-and-gaps)

Delta-to-Cumulative 変換は系列ごとの累積状態を必要とし、その系列の点を同じ変換先へ集める必要がある。仕様に載る変換手順は一つの例で、唯一のアルゴリズムではない。区間の欠落・重なりを無視して足す方法を正当化するものではない。[Data Model: Stream Manipulations](https://opentelemetry.io/docs/specs/otel/metrics/data-model/#stream-manipulations)

### MetricReader の選択と観測値の違い

- `MetricReader` の出力 temporality は instrument kind ごとに選ぶ。exporter から設定を得ること、未設定なら Cumulative を使うことは SHOULD。すべての実装が同じ既定値で動くという MUST ではない
- SDK instrument の出力を指定 temporality に合わせることは MUST。外部の非 SDK `MetricProducer` に同じ変換を強制する要件はない
- 同期 Cumulative は前回までに存在した属性集合も収集する。同期 Delta は前回以降の測定がある点だけ。非同期 instrument はどちらの temporality でも今回観測された点だけが対象になる
- 複数 Reader を登録でき、一方の Delta 収集が他方の収集区間を進めるような副作用を起こさないことは SHOULD

根拠: [Metrics SDK: MetricReader](https://opentelemetry.io/docs/specs/otel/metrics/sdk/#metricreader)。この節の出力規則は「同期 Counter の API に Delta 値を渡すか」という入力側の話と別である。[Supplementary Guidelines](https://opentelemetry.io/docs/specs/otel/metrics/supplementary-guidelines/#aggregation-temporality) は、同期 Counter などが増分を受け、非同期 Counter などが絶対値を観測する例を示す。補足文書自体は新しい規範要件を追加しない。

## 設計判断（独自の提案）

1. 最初に backend が受理する temporality と、使う SDK/exporter の実装版を固定する。入力の API・Reader の出力・Collector の変換後という三つの境界を別々に記録する
2. 変換不要で受理できるなら、運用上の理由がない往復変換を増やさない。必要な場合は系列 identity、状態の保持先、再起動時の扱い、遅着点の許容範囲を一緒に決める
3. stateful 変換を複数台に置くなら、系列単位で同じ担当へ送る構成を検討する。単純なリクエスト単位のラウンドロビンを採用する前に、担当変更時に累積値がどう切り替わるかを試験する
4. Cumulative の中間点が抜けても、開始時刻が同じ後続点から欠落期間を含む差を計算できる場合がある。ただし期間内の細かい変動は復元できない。Delta の未到着区間も、根拠なくゼロ補完しない
5. 「Delta なら状態ゼロ」とは見積もらない。非同期の絶対値を差分化する場所や downstream の累積化に状態が移るため、経路全体の cardinality とメモリ上限を測る

## 確認用の独自数値例と試験項目

以下は仕様の例の転載ではなく、受入試験の案である。

- 正常系: 同一系列の Cumulative が開始 `100`、終了 `110`・値 `7`、次が開始 `100`、終了 `120`・値 `11` なら、連続した二点間の増分は `4`。二つの累積値を足して `18` にしない
- 再起動: 次の点が開始 `125`、終了 `130`・値 `3` になったら、前系列の `11` との差を rate に使わない。reset の扱いを backend と確認する
- 不明開始: 開始と終了が `140`、値 `50` の初期点は「その瞬間に50件発生」の意味ではない。zero-duration reset として解釈する機能が使う実装にあるかを試す
- 欠落: Delta の `[開始100,終了110]` の次が `[開始120,終了130]` なら、間の10秒を勝手に埋めない。欠落を可視化できるかを確認する
- 衝突: 同名 metric を持つ二つの process の Resource を意図的に同一にした試験で、正常な合算と誤認しないかを確認する
- 無観測: 同期と非同期を分け、測定がない区間、二つの Reader の異なる収集周期、Collector 再起動を試す

ここで角括弧はテスト入力の開始・終了フィールドの表記であり、OTLP の時間区間の包含規則を変更する記号ではない。これらの動作試験は今回実行していない。検索 eval は本文が取得できることだけを検証する。

## 避ける使い方・未確認事項

値の減少だけを再起動と断定する、Resource を間引いて writer を衝突させる、時刻の重なりを無視して Delta を加算する、Development の説明を安定 API 保証として使うことを避ける。どの Collector processor がどの版で遅着・重複・欠落を処理するか、状態の永続化、言語別 SDK の環境変数対応は未確認。特定の性能改善率や完全配送も主張しない。

参照先本文に公開日の表示がないものは取得日を使う。Web 文書は footer の CC BY 4.0、upstream specification は Apache-2.0 を確認した。release 本文固有のライセンスは未確認で、コードや長文の引用は持ち込んでいない。release source の TTL 30日が最短のため、再確認日は2026-10-31。

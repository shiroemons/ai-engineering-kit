---
{
  "id": "opentelemetry-metric-cardinality-overflow-slo-boundary",
  "title": "OpenTelemetry metric cardinality: overflow が属性別 SLO を欠損させる境界",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "OpenTelemetry Specification 1.61.0 live display; Cardinality limits Stable; concept update 2026-07-02 / operating guide 2026-08-06",
  "tags": [
    "research-domain:quality-operations",
    "cardinality",
    "otel.metric.overflow",
    "aggregation_cardinality_limit",
    "View",
    "MetricReader",
    "SLO",
    "cumulative",
    "delta",
    "Exemplar"
  ],
  "sources": [
    {
      "id": "otel-cardinality-operating-guide-20261002",
      "url": "https://opentelemetry.io/blog/2026/cardinality-limits-in-opentelemetry/",
      "type": "maintainer_article"
    },
    {
      "id": "otel-cardinality-metrics-concepts-20261002",
      "url": "https://opentelemetry.io/docs/concepts/signals/metrics/",
      "type": "official_docs"
    },
    {
      "id": "otel-cardinality-sdk-contract-20261002",
      "url": "https://opentelemetry.io/docs/specs/otel/metrics/sdk/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# OpenTelemetry metric cardinality と overflow の運用境界

## 問いと追加理由

メトリクスの総リクエスト数は合っているのに、tenant 別・成功失敗別の SLO が良化して見えるのはなぜか。cardinality 上限によるプロセスメモリ保護と、属性別の観測精度を別々に判定する。

Cijo Thomas (Microsoft) による [2026-08-06 の運用解説](https://opentelemetry.io/blog/2026/cardinality-limits-in-opentelemetry/) と、7月に更新された [Metrics concept](https://opentelemetry.io/docs/concepts/signals/metrics/#cardinality-limits) が調査の契機。これは既存機能を説明する最近の文書更新であり、「2026年8月に初めて上限が導入された」という主張ではない。既存の temporality / reset 文書は時間区間の解釈を扱い、本稿は measurement 属性の消失と受入条件を補う。

## 確認した仕様と適用範囲

### 上限は属性の個数でも backend の系列数でもない

[Metrics concept](https://opentelemetry.io/docs/concepts/signals/metrics/#cardinality-limits) の cardinality は、measurement に付けた属性値の組合せ数。属性キーが2個だけでも、値の組合せは大量になり得る。上限に達すると、別々に保持できない measurement は `otel.metric.overflow=true` の点へ集約され、元の measurement 属性集合は残らない。原因となった高cardinalityキーだけが除かれるのではない。

Resource と instrumentation scope はこの measurement 属性集合とは別であり、overflow にも残る。ただし request ID を Resource へ移して回避する設計の根拠にはならない。concept は、同期 cumulative では過去の組合せを保持し、同期 delta では collection cycle 単位で状態を更新する違いも説明する。非同期 instrument を同じ寿命のモデルで扱わない。

### MUST と SHOULD を区別する

[Metrics SDK / Cardinality limits](https://opentelemetry.io/docs/specs/otel/metrics/sdk/#cardinality-limits) は Stable。cardinality 制限のサポート、属性 filtering 後の適用、および View の `aggregation_cardinality_limit`・MetricReader 既定値・未設定時の2000という設定規則には SHOULD が使われる。「全言語・全過去版が必ず2000で動く」という保証ではない。

同期 instrument は、各 measurement を通常の Aggregator または overflow Aggregator のどちらか一方へ反映し、overflow を理由とする二重計上や欠落を起こしてはならない。cumulative は overflow 開始前の属性集合を引き続き出力する。delta は制限を満たすために出力する属性集合の部分集合を選べる。非同期は callback で先に観測した属性を優先する SHOULD という別規則であり、同期の説明をそのまま一般化しない。

同じ [View の注意](https://opentelemetry.io/docs/specs/otel/metrics/sdk/#stream-configuration) は、metric stream から除外した属性が Exemplar の filtered attribute に残り得ると明記する。cardinality 削減と機密情報の除去は別に検証する。

### 合計と内訳は別の品質条件

[運用解説](https://opentelemetry.io/blog/2026/cardinality-limits-in-opentelemetry/) は、総量を維持しても属性 filter / group の内訳が過少になると説明する。低cardinalityの boolean も overflow 点には残らないため、成功失敗の分類にも影響する。SDK 上限は fleet 全体や長期間の backend 系列数を制限しない。

検出には overflow marker を用いるが、実際の保存名を確認する。Prometheus系では `otel_metric_overflow` へ変換される場合がある。検索が空でも、SDK未対応、保持期間外、変換・filteringによる見落としは排除できない。記事の query を無条件に貼るだけで監視済みとしない。

## 独自の数値例

以下は実測ではなく、counter の欠損内訳を示す設計上の例。ある区間で100リクエストを記録し、通常点が「成功70・失敗10」、overflow 点が20だったとする。全点の合計は100でも、overflow の20がどちらだったかは復元できない。失敗率は既知の10件だけから確定できず、全20件が成功なら10%、全20件が失敗なら30%となる。

この例では、通常点だけの10/80=12.5%や、失敗filterの10を全体100で割った10%を真の失敗率として報告しない。元の属性を失った点に backend が成功・失敗を推測補完しても、観測事実にはならない。histogram や gauge には、このcounter用の加算例を無条件に適用しない。

## 採用判断（独自の設計提案）

1. paging / SLO / autoscaling が依存する metric stream と、必要な measurement 属性を先に一覧化する。正常なtenant増加と、raw URL・無制限のメッセージを誤って記録する事故を区別する。
2. 属性を削る前に、その内訳を使う利用者を確認する。重要なtenant別指標なら、単にtenant属性を落として「overflow解消」とするのは要件の変更になる。cardinality上限の引上げも、使用メモリと受信先系列数を見積もって判断する。
3. 正規化・View filtering・上限変更のそれぞれについて、instrument / stream / Reader ごとの実設定を記録する。2000を環境全体の予算として掛け算なしに使わない。複数Reader・複数View・複数pod・再起動・長期保持を含む容量試験を別に置く。
4. overflow が出た時間帯は、影響する属性別グラフに不完全である旨を表示する。alert が発火しなくなったことを回復の証拠にしない。総量の到達性、内訳の完全性、export失敗を別の指標で扱う。
5. cardinality を抑えるために属性を除外するとき、Exemplar・logs・tracesなど他の経路にも残らないか、必要なデータ取り扱い条件に沿って点検する。Viewだけを包括的なredaction境界にしない。

## 受け入れ試験案（未実行）

- 固定した言語SDK・version・exporterに対し、configured limit の前後で異なる属性集合を入力する。通常点とoverflow点の数、既存集合への再記録、counter総和の保存を検査する。overflow点を含む正確な点数や境界挙動は、概説だけで推定せず採用SDKで確かめる。
- 成功失敗とtenantの組合せを制御し、総量が合うことに加えて、filtered query の不足を検出できるか確認する。marker がCollector経由でも保存され、必要なダッシュボードへ伝わるか試す。
- cumulative / delta と同期 / 非同期を分け、同じ属性、毎周期変わる属性、callback順序変更、再起動を含める。制限内へ戻ったときの回復条件を実測する。
- Viewで除外する属性を含むmeasurementを送り、metric pointとExemplarを別々に確認する。機密値を本番へ送る試験ではなく、識別用の無害なダミー値を使う。
- 複数podの長時間試験で、各SDKの上限とbackend総系列数・保持コストの両方を測る。エクスポート停止や受信側dropも別の障害として注入する。

## 版・取得日・未確認事項・provenance

取得日は2026-10-02 UTC。SDKページの表示版は OpenTelemetry Specification 1.61.0、Cardinality limits節はStableだが、ページ全体はMixedであり、個々の言語SDKの対応版は保証しない。conceptの表示更新日は2026-07-02、記事は公開・更新とも2026-08-06。liveページの確認であり、特定SDK実装のcommit固定解析や実行検証ではない。期限は90日後の2026-12-31とする。

SDK別の既定値・ゼロや無効値の解釈・上限の変更API・境界での点数・メモリ使用量、非同期の詳細な集約結果、backendでのlabel名・query費用は未検証。公開版タグのLICENSE URLはweb取得に失敗したため、upstream mainのLICENSEを別途確認し、タグ固定のライセンス検証とは区別した。

OpenTelemetry Authors と記事著者 Cijo Thomas の一次資料を日本語で独自要約し、独自の判断・数値例・試験案を付加した。サイト文書の [LICENSE](https://github.com/open-telemetry/opentelemetry.io/blob/main/LICENSE) は [CC-BY-4.0](https://creativecommons.org/licenses/by/4.0/)、upstream specificationの [LICENSE](https://github.com/open-telemetry/opentelemetry-specification/blob/main/LICENSE) はApache-2.0。コードやqueryの転載、依存追加、module昇格は行っていない。

---
{
  "id": "opentelemetry-sampling-parentbased-traceidratio",
  "title": "OpenTelemetry trace sampling ParentBased TraceIdRatioBased 判定と確率規則",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "Tracing SDK spec Stable Sampling section / General SDK Configuration OTEL_TRACES_SAMPLER (verified 2026-09-28)",
  "tags": ["research-domain:quality-operations", "opentelemetry", "sampling", "parentbased", "traceidratio", "tracesampler", "trace", "probability", "observability"],
  "sources": [{"id": "otel-trace-sdk-sampling-spec", "url": "https://opentelemetry.io/docs/specs/otel/trace/sdk/", "type": "official_docs"}, {"id": "otel-sdk-configuration-general", "url": "https://opentelemetry.io/docs/languages/sdk-configuration/general/", "type": "official_docs"}],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active"
}
---

# OpenTelemetry trace sampling ParentBased TraceIdRatioBased 判定と確率規則

トレースの sampling で「親の判定を継承するか、確率で新規判定するか」を決める仕様をまとめる。対象の問いは、SDK の既定サンプラー、ParentBased の委譲規則、TraceIdRatioBased の確率規則、環境変数 OTEL_TRACES_SAMPLER と OTEL_TRACES_SAMPLER_ARG の契約である。各文の根拠は節末のリンクで示す。2件はいずれも2026-09-28に確認した内容に基づく。

## 要点（公式情報に記載された事実）

### SamplingResult: IsRecording と Sampled の組み合わせ

- Sampler は SamplingResult を返し、その中核は Decision（DROP / RECORD_ONLY / RECORD_AND_SAMPLE）である。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- Decision は span の IsRecording フラグと Sampled フラグ（trace-flags の sampled ビット）に写像される。DROP は IsRecording=false かつ Sampled=false、RECORD_ONLY は IsRecording=true かつ Sampled=false、RECORD_AND_SAMPLE は IsRecording=true かつ Sampled=true である。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- IsRecording=false かつ Sampled=true の組み合わせは無効であり、仕様の組み合わせ表で禁止されている。この組み合わせを作ってはならない。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)

### ShouldSample の入力と Decision

- ShouldSample の入力は、親の SpanContext、trace_id、span 名、span kind、属性、リンクである。サンプラーは span 生成時にこの入力から Decision を一つ返す。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- Decision の意味は DROP（記録も出力もしない）、RECORD_ONLY（記録するがサンプリングしない）、RECORD_AND_SAMPLE（記録してサンプリングする）である。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- SamplingResult は Decision に加えて、判定時に付与する属性と TraceState を運ぶ。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)

### 既定サンプラー

- 仕様の既定サンプラーは ParentBased(root=AlwaysOn) である。明示設定がなければ、ルート span は常にサンプリングされ、子 span は親の判定に従う。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- SDK 設定の OTEL_TRACES_SAMPLER の既定値も parentbased_always_on であり、仕様の既定と対応する。[General SDK Configuration](https://opentelemetry.io/docs/languages/sdk-configuration/general/)

### ParentBased の5ケース委譲規則

- ParentBased は、有効な親 SpanContext がないルート span では root サンプラーに委譲する。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- 親がある場合は親の Sampled フラグに従い、親が remote か local かで場合分けした計5ケースの委譲表（親なし1件＋親あり4件）で結果が決まる。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- 親あり4件の結果は次の通り。remote かつ sampled の親はサンプリングする。remote かつ not sampled の親はドロップする。local かつ sampled の親はサンプリングする。local かつ not sampled の親はドロップする。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)

### TraceIdRatioBased の確率規則

- TraceIdRatioBased は親の Sampled フラグを無視し、trace_id だけから確率判定する。親がサンプリング済みでも確率は再計算される側であり、親の判定は引き継がない。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- 同じ trace_id に対しては同じ判定を返す決定論的規則であり、分散した SDK が独立に判定しても同一トレースの判定が一致する。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)
- サンプリング集合はネストする。すなわち、低い比率でサンプリングされる trace_id の集合は、高い比率でサンプリングされる集合の部分集合になる。[Tracing SDK specification](https://opentelemetry.io/docs/specs/otel/trace/sdk/)

### OTEL_TRACES_SAMPLER と OTEL_TRACES_SAMPLER_ARG

- OTEL_TRACES_SAMPLER の既定値は parentbased_always_on である。[General SDK Configuration](https://opentelemetry.io/docs/languages/sdk-configuration/general/)
- 受理値には parentbased_traceidratio が含まれ、これは ParentBased(root=TraceIdRatioBased) を意味する。[General SDK Configuration](https://opentelemetry.io/docs/languages/sdk-configuration/general/)
- OTEL_TRACES_SAMPLER_ARG は traceidratio 系（traceidratio / parentbased_traceidratio）の比率を指定し、範囲は 0 から 1、既定値は 1.0 である。[General SDK Configuration](https://opentelemetry.io/docs/languages/sdk-configuration/general/)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- トレースの連続性を保ちたい場合は既定の parentbased_always_on から変えない。上流の判定を下流が尊重するため、分散トレースが途切れない。
- 量を絞りたい場合は parentbased_traceidratio を選び、OTEL_TRACES_SAMPLER_ARG で比率を指定する。親なしのヘッド span だけが確率判定の対象になり、子は親に従うため、トレース単位の all-or-nothing が保たれる。
- 比率は 0 から 1 の範囲で指定し、未指定時の 1.0（全量）を起点に下げていく。比率変更時はネスト性により、上げれば既存の保持集合を含んだまま広がり、下げればその部分集合に絞られる前提で容量を見積もる。
- 言語別 SDK が OTEL_TRACES_SAMPLER の受理値をどこまで実装しているかは使う SDK の対応表で別途確認し、未対応値の指定時の挙動（無視・エラー・既定への退行）を検証環境で確かめる。

## 避ける使い方

- TraceIdRatioBased が親の Sampled フラグを尊重する前提で設計する。仕様上は無視されるため、親あり span でも確率で落とされる。
- ParentBased のもとで下流だけ比率を変えれば上流の判定を上書きできる前提で設計する。親あり span は親の判定に従うため、下流単独の設定ではトレース全体の保持は変わらない。
- OTEL_TRACES_SAMPLER_ARG に 0 から 1 の範囲外を指定する。契約外であり、解釈は未確認として扱う。
- IsRecording=false かつ Sampled=true の span を正当な状態として扱う。仕様で禁止された組み合わせであり、後段の exporter やバックエンドの前提を壊す。
- サンプラーが span 生成後の属性追加や終了時判断で判定を変える前提で設計する。判定は ShouldSample の一時点で行われ、入力は生成時の親・trace_id・名前・kind・属性・リンクである。

## 適用版と本番での注意

- 適用版: Tracing SDK specification の Stable Sampling 節、および General SDK Configuration の OTEL_TRACES_SAMPLER 節。ページに版番号の表示はないため、将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で技術固有 TTL の対象外のため、2026-12-27 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: jaeger_remote や xray など本ドキュメントが断定しない OTEL_TRACES_SAMPLER 受理値の一覧、言語別 SDK の環境変数対応差分、AlwaysOn/AlwaysOff/TraceIdRatioBased 単体の詳細契約、ヘッドサンプリングとテールサンプリングの使い分け基準。これらは該当ページの該当節を別途確認する。
- 本ドキュメントの推奨構成は設計案であり、単一事例の一般化ではない。比率の適正値はトレース生成速度とバックエンド容量に依存するため、対象ワークロードで測定して採用する。

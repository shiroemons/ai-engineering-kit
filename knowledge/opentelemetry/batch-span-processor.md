---
{
  "id": "opentelemetry-batch-span-processor",
  "title": "OpenTelemetry BatchSpanProcessor のバッチ化・キュー上限・終了時フラッシュ",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "Go SDK trace v1.46.0 / Python SDK trace.export latest (verified 2026-09-26)",
  "tags": [
    "research-domain:quality-operations",
    "opentelemetry",
    "batchspanprocessor",
    "batching",
    "queue",
    "shutdown",
    "forceflush",
    "trace",
    "span-exporter",
    "observability"
  ],
  "sources": [
    {
      "id": "otel-go-sdk-trace-docs",
      "url": "https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace",
      "type": "official_docs"
    },
    {
      "id": "otel-go-batch-span-processor-code",
      "url": "https://github.com/open-telemetry/opentelemetry-go/blob/sdk/v1.46.0/sdk/trace/batch_span_processor.go",
      "type": "official_docs"
    },
    {
      "id": "otel-python-sdk-trace-export-docs",
      "url": "https://opentelemetry-python.readthedocs.io/en/latest/sdk/trace.export.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active"
}
---

# OpenTelemetry BatchSpanProcessor のバッチ化・キュー上限・終了時フラッシュ

BatchSpanProcessor は span をキューに溜めてバッチで exporter に渡す。キュー満杯時の扱いと終了時のフラッシュ保証は SDK の契約で決まる。以下は Go SDK v1.46.0 と Python SDK の確認事実と、それを組み立てる設計案を分けて書く。各文の根拠は節末のリンクで示す。3件はいずれも2026-09-26に確認した内容に基づく。

## 要点（公式情報に記載された事実）

### Go SDK の既定値とオプション

- 既定値は DefaultMaxQueueSize 2048、DefaultScheduleDelay 5000ms、DefaultExportTimeout 30000ms、DefaultMaxExportBatchSize 512 である。[trace package](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace)
- 調整用オプションは WithMaxQueueSize、WithMaxExportBatchSize、WithBatchTimeout、WithExportTimeout、WithBlocking であり、BatchSpanProcessorOptions に記載されている。[trace package](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace)
- キュー満杯時は既定でドロップし、BlockOnQueueFull（WithBlocking 有効時）はドロップの代わりにブロックする。BatchSpanProcessorOptions に記載されている。[trace package](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace)
- SpanProcessor の Shutdown と ForceFlush は context のタイムアウト・キャンセルに従い、Shutdown は一度だけ実行される。[trace package](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace)

### Go 実装の振る舞い（sdk/v1.46.0 のソース）

- OnEnd はプロセッサ停止後または exporter が nil の場合にドロップする。サンプリングされなかった span はキューに入れない。[batch_span_processor.go](https://github.com/open-telemetry/opentelemetry-go/blob/sdk/v1.46.0/sdk/trace/batch_span_processor.go)
- enqueueDrop はノンブロッキングでドロップし dropped カウンタを増やす。enqueueBlockOnQueueFull はキューに空きが出るまでブロックする。[batch_span_processor.go](https://github.com/open-telemetry/opentelemetry-go/blob/sdk/v1.46.0/sdk/trace/batch_span_processor.go)
- Shutdown は停止状態を立てて stopCh を閉じ、processQueue を閉じて drainQueue で排出し、その後 exporter の Shutdown を呼ぶ。一度きりの実行意味論を持つ。[batch_span_processor.go](https://github.com/open-telemetry/opentelemetry-go/blob/sdk/v1.46.0/sdk/trace/batch_span_processor.go)
- ForceFlush はフラッシュマーカーをキューに入れ、ExportTimeout でキャンセルされる exportSpans を伴って未送信分を排出する。[batch_span_processor.go](https://github.com/open-telemetry/opentelemetry-go/blob/sdk/v1.46.0/sdk/trace/batch_span_processor.go)
- exportSpans は BatchTimeout タイマーをリセットする。exporter 失敗時のリトライは exporter 側の責務であり、プロセッサは再試行しない。[batch_span_processor.go](https://github.com/open-telemetry/opentelemetry-go/blob/sdk/v1.46.0/sdk/trace/batch_span_processor.go)

### Python SDK の対応

- BatchSpanProcessor(span_exporter, max_queue_size, schedule_delay_millis, max_export_batch_size, export_timeout_millis) の引数構成であり、OTEL_BSP_SCHEDULE_DELAY、OTEL_BSP_MAX_QUEUE_SIZE、OTEL_BSP_MAX_EXPORT_BATCH_SIZE、OTEL_BSP_EXPORT_TIMEOUT の環境変数と対応する。[trace.export](https://opentelemetry-python.readthedocs.io/en/latest/sdk/trace.export.html)
- force_flush(timeout_millis) はタイムアウト時に False を返す。shutdown と force_flush はプロセッサと exporter の双方に存在する。[trace.export](https://opentelemetry-python.readthedocs.io/en/latest/sdk/trace.export.html)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- キュー上限とバッチ上限は exporter の受信能力とメモリ上限から決める。既定の 2048 と 512 を起点にし、ドロップ数とエクスポート遅延を観測して調整する。
- プロセス終了時は必ず Shutdown を呼び、context に期限を付けてフラッシュを待つ。Shutdown 開始後に作られた span は OnEnd でドロップされるため、終了シーケンスに入ったら新しい span を作らない。
- リクエスト経路の遅延に影響させたくない場合は既定のドロップを選び、dropped を監視対象にする。span の欠落を避けたいバッチ処理では WithBlocking を選び、ブロックによる上流の停滞をタイムアウト監視と組み合わせる。
- Python では環境変数で上書きできることを前提に、コード既定と環境変数のどちらを正とするか運用方針で決め、両方での二重管理を避ける。
- リトライは exporter に委ねられるため、使う exporter のリトライ契約（回数・間隔・冪等性）を別途確認してからキュー長を決める。

## 避ける使い方

- Shutdown なしにプロセスを終了する。キュー内の未送信 span が失われる。
- 期限なしの context で ForceFlush や Shutdown を待つ。ExportTimeout と BatchTimeout の契約と噛み合わず、停止が長引く。
- キュー満杯時のドロップを監視せずに放置する。enqueueDrop は静かに捨てるため、欠落に気づけない。
- 非サンプリング span がキューに入る前提で流量を見積もる。非サンプリング span は enqueue されない。
- プロセッサが exporter 失敗を再試行する前提で設計する。リトライ責務は exporter 側にある。

## 適用版と本番での注意

- 適用版: Go SDK trace v1.46.0（pkg.go.dev の表示版および sdk/v1.46.0 タグのソース）、Python SDK trace.export の latest ページ。将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で技術固有 TTL の対象外のため、2026-12-25 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: 言語別 SDK 間の既定値の差分一覧、OTLP exporter 側のリトライ・タイムアウト契約、dropped カウンタのメトリクス露出方法、SimpleSpanProcessor との使い分け基準。これらは該当ページの該当節を別途確認する。
- 本ドキュメントの推奨構成は設計案であり、単一事例の一般化ではない。キュー長の適正値は span 生成速度とエクスポート遅延に依存するため、対象ワークロードで測定して採用する。

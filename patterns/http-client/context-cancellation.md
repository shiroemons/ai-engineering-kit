---
{
  "id": "context-cancellation",
  "title": "HTTP client の context を待機にも伝える",
  "kind": "pattern",
  "technology": "go",
  "version": "Go 1.27.1 (basic context API)",
  "tags": [
    "context",
    "http",
    "client",
    "cancellation"
  ],
  "sources": [
    {
      "id": "go-context-docs",
      "url": "https://pkg.go.dev/context",
      "type": "official_docs"
    },
    {
      "id": "go-time-docs",
      "url": "https://pkg.go.dev/time",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-23",
  "expires_at": "2026-12-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/patterns/search.json"
  ]
}
---

# HTTP client の context を待機にも伝える

通信の前後で待機する処理にも、呼び出し元の Context を渡す。これは公式 API を組み合わせた設計案で、特定 OSS の分析結果ではない。

## 設計判断

待機時間と中止通知の両方を監視する。中止時は理由を返し、呼び出し側が再試行を止められるようにする。タイマーは呼び出し単位で所有する。

## 境界

再試行する操作の安全性や上限回数は呼び出し側が決める。キャンセルと完了が競合する場合の契約を明記する。Context に応答しない外部処理まで停止できるとは扱わない。

[context API](https://pkg.go.dev/context) と [Timer API](https://pkg.go.dev/time#NewTimer) が根拠となる。待機の実装候補は [contextwait](../../modules/go/contextwait/README.md)。

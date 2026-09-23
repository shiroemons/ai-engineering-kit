---
{
  "id": "go-contextwait",
  "title": "contextwait: キャンセル可能な待機",
  "kind": "module",
  "technology": "go",
  "version": "Go 1.27+ (verified with Go 1.27.1)",
  "tags": [
    "context",
    "wait",
    "timer",
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
    },
    {
      "id": "go-synctest-docs",
      "url": "https://pkg.go.dev/testing/synctest",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-23",
  "expires_at": "2026-12-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/modules/contextwait.json"
  ]
}
---

# contextwait

待機中のキャンセルに応答する小さな Go パッケージ。外部コードを転記せず、このリポジトリ用に実装する。

```go
func Wait(ctx context.Context, d time.Duration) error
```

| 条件 | 結果 |
|---|---|
| nil Context | ErrNilContext |
| 開始前にキャンセル済み | ctx.Err() |
| d が 0 以下 | 待機せず ctx.Err() |
| 待機中にキャンセル | ctx.Err() |
| 待機完了 | ctx.Err() を再確認して返す |

完了と中止が競合する場合、最後の確認で検出した中止を優先する。確認後の中止は戻り値に反映できない。独自の Cause ではなく ctx.Err() を返す。

```go
import "github.com/shiroemons/ai-engineering-kit/modules/go/contextwait"

err := contextwait.Wait(ctx, 100*time.Millisecond)
if err != nil {
    return err
}
```

共有状態と内部 goroutine はない。各呼び出しがタイマーを所有し、終了時に停止する。

`go test -race ./modules/go/contextwait` で単体・境界・並行テストと [JSON eval](../../../evals/modules/contextwait.json) を実行する。[testing/synctest](https://pkg.go.dev/testing/synctest) の仮想時計を使い、実時間の待機をなくす。API の根拠と実装履歴は [PROVENANCE.md](PROVENANCE.md) に記録する。

---
{
  "id": "go-context",
  "title": "Go context のキャンセルを伝える",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 (basic context API)",
  "tags": [
    "context",
    "cancellation",
    "concurrency"
  ],
  "sources": [
    {
      "id": "go-context-docs",
      "url": "https://pkg.go.dev/context",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-23",
  "expires_at": "2026-12-22",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/search.json"
  ]
}
---

# Go context のキャンセルを伝える

Context は処理期限と中止通知を呼び出し先へ渡す。親のキャンセルは派生 Context に伝わる。[公式 API](https://pkg.go.dev/context)

## 推奨方法

必要な関数の第1引数で受け取り、作成した CancelFunc を呼ぶ。共有 Context は複数 goroutine から使える。

## 避ける使い方

nil を渡さず、任意のオプションを Value へ詰めない。呼び出し元の Context を Background に置き換えると中止通知が途切れる。

## 適用範囲

基本 API を Go 1.27.1 で検証する。参照ページの表示版は source に記録し、将来も最新とは扱わない。中止に応答しない処理は自動で停止しない。

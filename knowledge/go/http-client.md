---
{
  "id": "go-http-client",
  "title": "Go net/http の production 向け HTTP Client 設計",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 (net/http client API)",
  "tags": [
    "http",
    "client",
    "transport",
    "timeout",
    "connection-pooling",
    "idle-connection",
    "redirect",
    "response-body",
    "retry",
    "context",
    "cancellation"
  ],
  "sources": [
    {
      "id": "go-net-http-docs",
      "url": "https://pkg.go.dev/net/http",
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

# Go net/http の production 向け HTTP Client 設計

`net/http` の `Client` は cookie と redirect を扱う上位層、`Transport` は接続・プロキシ・TLS・keep-alive を持つ下位層である。[公式 API](https://pkg.go.dev/net/http)

## 要点

- `Client` と `Transport` は並行利用可能で、接続キャッシュを持つため一度作って再利用する。リクエストごとに作ると connection pooling が効かない。
- `Do` のエラーはクライアント方針や HTTP 話し方の失敗に限定され、non-2xx はエラーにならない。
- `err == nil` なら `Response.Body` は非 nil で、呼び出し側が閉じる責務を持つ。閉じないと HTTP/1.x keep-alive の再利用ができないことがある。
- timeout は層が違う。`Client.Timeout` は接続・redirect・body 読み取りの全体を含み、`Do` 後も body 読み取りを中断できる。`Transport.ResponseHeaderTimeout` はリクエスト書き込み後のレスポンスヘッダ待ちだけで、body 読み取りは含まない。
- outgoing request の context は接続取得・送信・レスポンスヘッダと body の読む lifetime 全体を制御する。context cancellation はこの lifetime 全体に効く。
- `Transport` の retry は network error かつ特定条件の idempotent request に限られ、`Client` は汎用 retry を提供しない。

## 推奨方法

- production では `http.Client` を使い、必要なら専用 `http.Transport` を1つ持つ。proxy・TLS・keep-alive・buffer を制御したい場合に Transport を作る。公式の package example は `MaxIdleConns` と `IdleConnTimeout` を設定する。
- pooling を明示するなら `Transport.MaxIdleConns`、`MaxIdleConnsPerHost`、`IdleConnTimeout` を設定する。既定の `MaxIdleConnsPerHost` は `DefaultMaxIdleConnsPerHost` の 2 である。`DisableKeepAlives` は keep-alive を無効化し1接続1リクエストになる。
- deadline と cancel は `http.NewRequestWithContext` で渡し、全体の上限が要るときは `Client.Timeout` も併用する。必要なら `Transport.TLSHandshakeTimeout` や `ResponseHeaderTimeout` で段階別に制限する。
- response body は必ず `Close` する。keep-alive 再利用を確実にしたい大きい body は EOF まで読む。`Close` 時 Transport は保守的な上限まで非同期で EOF 読みを試みる。
- redirect 方針は `Client.CheckRedirect` で持つ。追従しないで直近レスポンスを body 未クロースで返す場合は `http.ErrUseLastResponse` を返す。既定は10回連続 redirect で停止する。301/302/303 は後続が GET（元が HEAD なら HEAD）で body なし、307/308 は `Request.GetBody` が有るとき method と body を保つ。
- retry の責務分離は次の通り。`Transport` は「接続が既に成功済み・request が idempotent・body 無しあるい `GetBody` 有り」の network error だけを再試行する。idempotent は GET/HEAD/OPTIONS/TRACE か `Idempotency-Key` / `X-Idempotency-Key` ヘッダ。status code に基づく backoff retry は呼び出し側が持つ。`RoundTripper` は redirect・認証・cookie を解釈しない契約であり、retry を上位層の責務として `Client`/`Transport` の外に置く。これは公式契約からの設計上のまとめであり、公式が推奨する retry ライブラリを指すものではない。

## 避ける使い方

- リクエストごとの `Client` / `Transport` 作成。idle connection が捨てられ pooling が効かない。
- `Response.Body` を閉じない、または non-2xx を自動 error として扱えるという想定。
- deprecated の `Request.Cancel`、`Transport.CancelRequest`、`Transport.Dial`。cancel は context、dial は `DialContext` を使う。
- `Transport.ResponseHeaderTimeout` だけで全体 timeout と考える。body 読み取りは対象外。
- WHATWG Fetch と同じ redirect 挙動という想定。公式は Fetch 準拠ではなく、subdomain や同一 host 別 scheme への転送で sensitive header を残す等、許容範囲が広いと明記する。
- `Client` が 429/503 等を自動 retry するという想定。その判断は呼び出し側。
- `TLSClientConfig` 等を設定しただけで HTTP/2 が有効と判断する。非 nil の `TLSClientConfig` や custom dialer は既定で HTTP/2 を止めがちで、明示には `ForceAttemptHTTP2` か `Transport.Protocols` が要る。

## 適用版と本番での注意

- `Go 1.27.1 (net/http client API)` と pkg.go.dev の表示 `go1.27.1` で確認する。将来の最新とは扱わない。
- `http.DefaultTransport` の既定値は `MaxIdleConns: 100`、`IdleConnTimeout: 90s`、`TLSHandshakeTimeout: 10s`、`ExpectContinueTimeout: 1s`、dial `Timeout: 30s`、`ForceAttemptHTTP2: true`。custom Transport はこの値を起点に必要分だけ上書きする。
- `Client.Timeout` が 0 なら timeout なし。`MaxConnsPerHost` 超過時は dial がブロックする。
- redirect 追従時、`Authorization` / `WWW-Authenticate` / `Cookie` は初期ドメインの subdomain 一致または完全一致以外へ転送しない。Jar 有り時の `Cookie` ヘッダは mutation 分を省く。
- ログとメトリクスで区別する値は、`url.Error.Timeout()`、`Client.CheckRedirect` 側の error、`Response.StatusCode`、idle/active 接続数。status code と transport error は別系列として記録する。
- 本文の retry 方針と pooling 上限は設計判断であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。

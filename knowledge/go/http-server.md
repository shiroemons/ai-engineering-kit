---
{
  "id": "go-http-server",
  "title": "Go net/http の production 向け HTTP Server 設計",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 (net/http server API)",
  "tags": [
    "http",
    "server",
    "handler",
    "serve-mux",
    "timeout",
    "graceful-shutdown",
    "tls",
    "http2",
    "middleware",
    "context",
    "cancellation",
    "response-writer",
    "request",
    "max-bytes",
    "hijack",
    "push"
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

# Go net/http の production 向け HTTP Server 設計

`net/http` の `Server` は HTTP/1.x と HTTP/2 を持つサーバ実装で、`Handler` インターフェースと `ServeMux` でルーティングを行う。[公式 API](https://pkg.go.dev/net/http)

## 要点

- `Server` は並行利用可能で、`ListenAndServe`・`ListenAndServeTLS`・`Serve`・`ServeTLS` で起動する。`DefaultServeMux` を使う `ListenAndServe` は簡便だが、production では専用 `Server` と `ServeMux` を作る。
- `Handler` インターフェースは `ServeHTTP(ResponseWriter, *Request)` だけ。`HandlerFunc` で関数をアダプトし、`ServeMux.Handle`・`HandleFunc` で登録する。パスは最長一致で、`/` は全てにマッチする。
- `ResponseWriter` は `Header() Header`、`Write([]byte) (int, error)`、`WriteHeader(int)` を持つ。`Header` は `WriteHeader` 前まで変更可能。`Write` は暗黙的に `WriteHeader(200)` を呼ぶ。`Hijacker`・`Flusher`・`Pusher` は型アサーションで取得する。
- `Request.Context()` はクライアント接続の終了、HTTP/2 リクエストのキャンセル、`ServeHTTP` の終了でキャンセルされる。`Server.Shutdown` は進行中の通常リクエストを待つため、その開始だけで各リクエストの context がキャンセルされるとは考えない。
- タイムアウトは層が違う。`Server.ReadTimeout` は body を含むリクエスト全体の読み取り、`WriteTimeout` はレスポンス書き込み、`IdleTimeout` は keep-alive アイドル、`ReadHeaderTimeout` はヘッダ読み取りを制限する。`MaxHeaderBytes`・`MaxHeaderValueCount` はヘッダの上限で、body サイズは制限しない。
- `Server.Shutdown(ctx)` は graceful shutdown で、context が完了するまでアクティブ接続の完了を待つ。`Server.Close()` は即座に全接続を閉じる。`RegisterOnShutdown` でコールバックを登録できる。
- HTTP/2 は HTTPS で自動有効。`Server.Protocols`・`Server.HTTP2` で制御する。`GODEBUG=http2server=0` で無効化できる。
- ミドルウェアは `Handler` をラップする関数で実現し、`http.StripPrefix`・`http.TimeoutHandler`・`http.MaxBytesHandler` など標準装備もある。`ResponseController`（Go 1.20+）で `SetReadDeadline`・`SetWriteDeadline`・`Flush`・`Hijack`・`EnableFullDuplex` を動的に制御できる。
- `MaxBytesReader` でリクエスト body 上限を強制し、超過時は `MaxBytesError` を返す。`Request.ParseMultipartForm(maxMemory)` の `maxMemory` はファイル部分をメモリに置く量を制限し、残りは一時ファイルに保存する。リクエスト全体のサイズ制限としては扱わない。
- エラー変数は `ErrBodyNotAllowed`、`ErrHijacked`、`ErrContentLength`、`ErrAbortHandler`（panic でログ抑制）、`ErrHandlerTimeout`、`ErrServerClosed` など。`ProtocolError` は deprecated。

## 推奨方法

- production では専用 `http.Server` と `http.ServeMux` を作り、`ReadTimeout`・`WriteTimeout`・`IdleTimeout`・`ReadHeaderTimeout`・`MaxHeaderBytes` を明示する。`ListenAndServe` ではなく `Server.ListenAndServe` を使う。
- graceful shutdown は `Shutdown(ctx)` で実装し、context に適切な deadline を設ける。WebSocket など hijack 済み接続は自動で閉じられないので、`RegisterOnShutdown` で終了を通知し、別途完了を待つ。
- TLS は `ListenAndServeTLS` または `ServeTLS` で証明書・鍵を指定する。`Server.TLSConfig` で詳細設定（最小バージョン・暗号スイート・クライアント証明書検証等）を制御する。HTTP/2 要件を満たす設定にする。
- ミドルウェアチェーンは `Handler` を返す関数で組み、ログ・メトリクス・リカバリ・認証・CORS 等を分離する。`http.TimeoutHandler` でハンドラ単位の上限を、`MaxBytesHandler` で body 上限を付与できる。
- `ResponseController` で対応する `ResponseWriter` の deadline や flush を制御する。HTTP/1 の同時読み書きが必要な場合は `EnableFullDuplex` の戻り値を確認する。
- `Request.Context()` のキャンセルを下流（DB・外部API）へ伝播させる。`Request.Clone(ctx)` で timeout 付き派生 context を作る。
- ヘルスチェック・メトリクス・pprof エンドポイントは別ポートや別 `ServeMux` で分離し、本番トラフィックの timeout 影響を受けないようにする。
- エラーハンドリングは `Error` 関数や独自ハンドラで統一し、スタックトレースをクライアントに返さない。`ErrAbortHandler` で意図的中止をログから除外する。

## 避ける使い方

- `DefaultServeMux` へグローバル登録し続ける。ライブラリ同士で衝突し、テストで隔離できない。
- `ListenAndServe` だけで production 運用する。timeout 未設定で slowloris 攻撃を受ける。
- 通常の response header を `WriteHeader` や `Write` の後で変更する。送信済みのヘッダには反映されない。
- server 側の `Request.Body` を Handler が必ず閉じる必要があると考える。server が閉じる。巨大 body を読み込む処理では `MaxBytesReader` 等によるサイズ制限を検討する。
- `Server.Close()` だけで終了する。進行中リクエストが強制切断され、クライアントエラーになる。
- Handler 内の panic を通常のエラー処理に使う。server は `ServeHTTP` の panic を回復して stack trace をログに残し、該当接続を閉じるか HTTP/2 stream を reset する。
- `GODEBUG` で HTTP/2 を無効化し続ける。パフォーマンスと互換性を損なう。設定で制御する。
- Handler の終了後に `ResponseWriter` を使う。`ResponseController` も Handler のライフサイクル管理を代替しない。

## 適用版と本番での注意

- `Go 1.27.1 (net/http server API)` と pkg.go.dev の表示 `go1.27.1` で確認する。将来の最新とは扱わない。
- `DefaultServeMux` は `http.Handle`・`HandleFunc` で登録されるグローバル。テストでは `NewServeMux()` を使う。
- `Server.ReadTimeout` の既定は 0（無制限）。`WriteTimeout` も 0。production では必ず設定する。
- `IdleTimeout` が 0 の場合は `ReadTimeout` を使う。両方が 0 以下なら keep-alive の待機期限はない。
- `MaxHeaderBytes` 既定 1MB、`MaxHeaderValueCount` 既定 500。巨大ヘッダ攻撃対策で必要なら調整する。
- `Shutdown` は `ListenAndServe` 等が返す `ErrServerClosed` を正常終了として扱う。`Close` と併用しない。
- `ResponseController` は Go 1.20 で導入。古いバージョンでは型アサーションで `Hijacker`・`Flusher`・`Pusher` を取得していた。
- `CrossOriginProtection`（Go 1.25+）は unsafe な cross-origin browser request を拒否する。`NewCrossOriginProtection().Handler(h)` でラップする。非 browser request など許可される条件も確認する。
- 本文の timeout 推奨値と middleware 構成は設計判断であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。

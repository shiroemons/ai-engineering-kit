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
    "routing",
    "pattern",
    "wildcard",
    "path-value",
    "precedence",
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
  "retrieved_at": "2026-09-25",
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
- `Handler` インターフェースは `ServeHTTP(ResponseWriter, *Request)` だけ。`HandlerFunc` で関数をアダプトし、`ServeMux.Handle`・`HandleFunc` で登録する。`ServeMux` は登録済みパターンに対してリクエスト URL を照合し、「最も近くマッチする」パターンのハンドラを呼ぶ。
- パターン構文（pattern）: 登録パターンは `[METHOD ][HOST]/[PATH]` の形で、3要素はいずれも省略でき、`/` も有効なパターンである。リテラル部分は大文字小文字を区別する。メソッドを持たないパターンは全メソッドにマッチし、`GET` のパターンは GET と HEAD の両方にマッチし、それ以外は完全一致する。ホストを持たないパターンは全ホストに、持つ場合はそのホストのみにマッチする。
- ワイルドカード（wildcard）: `{NAME}` は1つのパスセグメント、末尾の `{NAME...}` は残りのセグメント全部にマッチし、`...` を含むワイルドカードはパターン末尾以外に置けない。NAME は有効な Go 識別子で、ワイルドカードは完全なセグメント（直前にスラッシュ、直後にスラッシュか文字列末尾）でなければならない。マッチ値は `Request.PathValue(name)` で取得し、返す値は unescape 済み、マッチしない場合は空文字になる。パス末尾のスラッシュは匿名の `...` ワイルドカードとして扱われる。特殊ワイルドカード `{$}` は URL の終わりだけにマッチし、`/{$}` は `/` のみ、`/` は全てのパスにマッチする。
- 優先順位（precedence）と競合（conflict）: 複数のパターンがマッチしたら最も具体的な方が優先され、P1 が P2 のリクエストの厳密な部分集合なら P1 の方が具体的と判定する。どちらでもなければ競合で、互換性のために片方だけがホストを持つ場合はホスト付きが優先する。競合するパターン、または構文的に不正なパターンを `Handle` / `HandleFunc` に渡すと panic する。
- パスの unescape と整形（sanitizing）: 照合の際、パターン側もリクエスト側もセグメントごとに unescape する。`/a%2Fb/100%25` は「`a/b`」「`100%`」の2セグメントとして扱われ、パターン `/a%2fb/` はマッチし `/a/b/` はマッチしない。`ServeMux` は Host のポート番号を除去し、`.`・`..`・連続スラッシュを含むリクエストを整形した URL へのリダイレクトで片付ける。`%2e` のような escape は区切り文字として扱わない。
- 末尾スラッシュ（trailing slash）のリダイレクト（redirect）: 末尾スラッシュや `...` ワイルドカードで登録したサブツリーのルートを、末尾スラッシュなしで要求すると追加してリダイレクトする。`/images/` を登録すると `/images` は `/images/` へ送られ、`/images` を別途登録すればこの挙動を抑止できる。
- 互換性: パターン構文と照合挙動は Go 1.22 で大きく変わった。旧挙動は `GODEBUG=httpmuxgo121=1` で復元でき、この値はプログラム開始時に1回だけ読まれる。Go 1.21 では `/{` や `/a{x}` のような `{` を含むパターンも文字通りマッチしたが、以降は構文的に不正なパターンの登録が panic する。
- `ServeMux.Handler(r)` は引数を変更せずワイルドカード値を埋めないため、返した handler で `r.PathValue` は常に空文字を返す。
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
- パターン登録は起動時に集約し、競合や不正なパターンによる panic を起動時検出にする。優先順位はパターン同士の具体的さだけで決まる前提で、登録順に頼ったフォールバックを組まない。
- パス変数は `{name}` パターンと `Request.PathValue` で受け取り、`StripPrefix` による手動の切り出しと同一パスで混在させない。
- `ServeMux.Handler(r)` をログやメトリクス用途で使う場合は `PathValue` が埋められない前提で書く。
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
- 「長いパスほど優先される」前提でルーティングを組む。具体的さは厳密な部分集合で判定され、判断できない組み合わせは登録時に panic する。
- 登録の順序で優先順位やフォールバックを制御する。優先順位は具体的さの規則で決まり、決まらない組み合わせは競合として登録時に panic する。
- `Handle` に渡すパスへ `{` や `}` を含める。Go 1.22 以降では不正なパターンとして panic するか、ワイルドカードとして解釈される。
- `mux.Handler(r)` の戻り値から `PathValue` を読む。`Handler` はワイルドカード値を埋めない。
- 末尾スラッシュ付きサブツリーを登録しておきながら、ルートパスのリダイレクトを想定外の挙動として扱う。別途登録しない限り行われる。

## 適用版と本番での注意

- `Go 1.27.1 (net/http server API)` と pkg.go.dev の表示 `go1.27.1` で確認する。将来の最新とは扱わない。
- `DefaultServeMux` は `http.Handle`・`HandleFunc` で登録されるグローバル。テストでは `NewServeMux()` を使う。
- ワイルドカード（`{NAME}` / `{NAME...}`）と `Request.PathValue` / `SetPathValue` は Go 1.22 で導入された。pkg.go.dev の `added in go1.22.0` 表記と ServeMux の互換性節で確認する。Go 1.21 ではワイルドカードは通常のリテラル区切りとして扱われていた。
- 旧い照合挙動は `GODEBUG=httpmuxgo121=1` で復元できる。この値はプログラム開始時に1回だけ読まれ、実行中の変更は無視される。
- `Server.ReadTimeout` の既定は 0（無制限）。`WriteTimeout` も 0。production では必ず設定する。
- `IdleTimeout` が 0 の場合は `ReadTimeout` を使う。両方が 0 以下なら keep-alive の待機期限はない。
- `MaxHeaderBytes` 既定 1MB、`MaxHeaderValueCount` 既定 500。巨大ヘッダ攻撃対策で必要なら調整する。
- `Shutdown` は `ListenAndServe` 等が返す `ErrServerClosed` を正常終了として扱う。`Close` と併用しない。
- `ResponseController` は Go 1.20 で導入。古いバージョンでは型アサーションで `Hijacker`・`Flusher`・`Pusher` を取得していた。
- `CrossOriginProtection`（Go 1.25+）は unsafe な cross-origin browser request を拒否する。`NewCrossOriginProtection().Handler(h)` でラップする。非 browser request など許可される条件も確認する。
- 本文の timeout 推奨値・middleware 構成・パターン登録の運用は設計判断であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。

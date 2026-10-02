---
{
  "id": "go-x509-platform-root-override-migration",
  "title": "Go 1.27 x509: SSL_CERT_FILE と platform verifier 切替の移行境界",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.0 introduces behavior; API and source verified at go1.27.1 (862c888e612ac346c7c4d99c9392bdfd265f33b0); GODEBUG effective defaults depend on main module/workspace",
  "tags": [
    "research-domain:backend",
    "crypto/x509",
    "SystemCertPool",
    "SSL_CERT_FILE",
    "SSL_CERT_DIR",
    "GODEBUG",
    "RootCAs",
    "platform-verifier",
    "trust-store",
    "migration"
  ],
  "sources": [
    {
      "id": "go-x509-release-1-27-20261002",
      "url": "https://go.dev/doc/go1.27",
      "type": "release_notes"
    },
    {
      "id": "go-x509-release-history-20261002",
      "url": "https://go.dev/doc/devel/release",
      "type": "release_notes"
    },
    {
      "id": "go-x509-api-1-27-1-20261002",
      "url": "https://pkg.go.dev/crypto/x509@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go-tls-roots-1-27-1-20261002",
      "url": "https://pkg.go.dev/crypto/tls@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go-x509-godebug-20261002",
      "url": "https://go.dev/doc/godebug",
      "type": "official_docs"
    },
    {
      "id": "go-x509-root-source-1-27-1-20261002",
      "url": "https://raw.githubusercontent.com/golang/go/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/crypto/x509/root.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go-x509-godebugs-source-1-27-1-20261002",
      "url": "https://raw.githubusercontent.com/golang/go/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/internal/godebugs/table.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Go 1.27 x509 の trust store 移行

## 問い・対象

Windows / macOS で Go アプリを更新した後、OS では信頼される社内証明書を拒否するようになった。以前から設定していた `SSL_CERT_FILE` は何を変えるのか。toolchain の更新だけで全バイナリの動作が揃うのか。

[Go 1.27 release notes](https://go.dev/doc/go1.27#crypto/x509) は、これらの環境変数を Windows / Darwin でも尊重し、disk 上の roots と Go verifier を使う変更を記す。[release history](https://go.dev/doc/devel/release#go1.27.0) の公開日は1.27.0が2026-08-19、今回 API・実装を固定した1.27.1が2026-09-01。取得日は2026-10-02 UTCであり、将来版の最新性を保証しない。

既存の HTTP client 記事は timeout・pooling が中心。本稿は Go backend / CLI のサーバー証明書検証における trust store と verifier の選択を扱う。OS trust store の書換え、秘密鍵、mTLS client certificate 選択は対象外。以下は公開契約、固定版の実装観察、独自の運用判断を分ける。

## 公開契約: CA の追加だけではない

[go1.27.1 SystemCertPool](https://pkg.go.dev/crypto/x509@go1.27.1#SystemCertPool) と [GODEBUG history](https://go.dev/doc/godebug#go-127) による境界:

- `SSL_CERT_FILE` は証明書file、`SSL_CERT_DIR` は証明書directory の既定探索先を上書きする。directory list の区切りは通常 `:`、Windows は `;`
- Windows / macOS では、この環境変数による上書きが有効になると platform certificate verification APIs を使わなくなる。OS store にCAを追加する操作でも、OS storeとPEM bundleの自動unionでもない
- Go 1.27側の設定は `x509sslcertoverrideplatform=1`。`GODEBUG=x509sslcertoverrideplatform=0` は環境変数を尊重せず platform certificate store を使う旧経路へ戻す。実際の既定値は後述の build 設定にも依存する
- `SystemCertPool()` が返すpoolはcopy。変更してもdiskや他の返却poolへ反映されない。後からのsystem roots変更が次の呼出しへ反映される保証もない

[Certificate.Verify](https://pkg.go.dev/crypto/x509@go1.27.1#Certificate.Verify) は platform verifier を使う場合に検証の詳細が異なり得ると記し、Go側の検証には失効確認を行わない警告がある。「同じCAを含めたから、失効・policyを含むOS側の検証結果も完全一致する」とは扱わない。特定OSの失効確認方式やネットワーク取得の挙動は本稿では検証していない。

[tls.Config](https://pkg.go.dev/crypto/tls@go1.27.1#Config) の `RootCAs=nil` はhostのroot CA setを使う。したがって、明示的に `SystemCertPool` を呼ぶコードだけを調べても影響調査は終わらない。一方、独自poolを `RootCAs` に渡す経路は、poolの作り方とcallbackも別に確認する。非nilという事実だけで、環境変数の影響を受けないとは決めない。

## 固定実装の確認: 空値・読込失敗・キャッシュ

分析対象は golang/go の `go1.27.1` tag、commit `862c888e612ac346c7c4d99c9392bdfd265f33b0`。tagは `git ls-remote` で照合し、[root.go](https://raw.githubusercontent.com/golang/go/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/crypto/x509/root.go) を読んだ。以下はこの固定版の実装観察であり、将来の内部実装の契約ではない。

1. `loadSystemRoots` は `os.Getenv` で両変数を読む。Windows / Darwin / iOS の分岐で両方が空文字ならplatform poolを返す。少なくとも一つが非空で、設定値が `0` でなければdisk読込へ進む。「環境変数名が存在するだけ」と「非空値」を区別する
2. `loadOnDiskRoots` は新しい空poolから構築する。指定file / directoryが見つからないケースでは、存在しないというエラーを `firstErr` に保存しない経路がある。空poolでもerrorがnilになり得るため、`SystemCertPool` のerror確認だけで必要なtrust anchorがあるとは保証できない
3. 一部の読込でエラーがあっても、証明書を取得できていればpoolを返す経路がある。設定した全ファイルを完全にロードできたことを、呼出し成功と同一視しない
4. system roots の初期化には `sync.Once` とcacheを使う。実行中に環境変数やbundleを変更してから再呼出しするだけのreload設計を、このAPIへ期待しない

この分岐はLinuxへplatform verifierを追加するものではない。`x509sslcertoverrideplatform=0` を全OS共通の「SSL_CERT_*を無視する設定」として配布しない。`SetFallbackRoots` や独自検証callbackを使用するアプリは追加の経路を持つため、本稿の単純な分岐説明だけでは判定しない。

## GODEBUG: toolchain と実効既定値を分ける

[公式のdefault選択規則](https://go.dev/doc/godebug#default-godebug-values) では、toolchainの既定をmain module / workspaceのGo版へ合わせ、`godebug`、main packageの `//go:debug`、実行時 `GODEBUG` による指定を考慮する。workspace使用時は `go.work` が対象で、依存moduleの `godebug` は採用しない。

固定版の [internal/godebugs/table.go](https://raw.githubusercontent.com/golang/go/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/internal/godebugs/table.go) では当該設定が Go 1.27変更、旧値 `0` と登録されている。したがって「Go 1.27.1で再buildした」という情報だけでは、環境変数上書きが有効か判定できない。`go 1.26` の互換既定を保つmain moduleと、`go 1.27` のmain moduleを分ける。

ローカルの限定確認として、locked toolchain **go1.27.1 linux/amd64**、外部依存のない一時main module、`GOWORK=off` で `go list -buildvcs=false -f '{{.DefaultGODEBUG}}' .` を実行した。

- `go 1.26`: `GOOS=linux` / `darwin` / `windows` のいずれでも `x509sslcertoverrideplatform=0` を表示
- `go 1.27`: 当該overrideを表示しない。`.DefaultGODEBUG` はtoolchain既定との差分だけを返すため、省略を「機能なし」と解釈しない
- `go 1.26` に `godebug x509sslcertoverrideplatform=1` を明示: `...=1` を表示

初回の一時directoryでのprobeはVCS stampingの失敗で結果を得られず、`-buildvcs=false` を付けて再実行した。これはcompile metadataの確認であり、macOS / WindowsでTLSを実行した結果ではない。実際のdeployでは同じmain package / workspaceで既定を調べ、実行環境の `GODEBUG` と照合する。

## 移行判断とrollback（独自の運用案）

以下は上記の契約から導く設計案で、公式が特定のアプリ構成を推奨するという意味ではない。

1. **現在の信頼方針を先に決める。** OS storeを使い続けるのか、アプリ専用bundleで信頼範囲を置き換えるのか、既存poolへCAを追加するのかを分ける。shell profile、サービス定義、IDE、CIから意図せず継承される `SSL_CERT_FILE` / `SSL_CERT_DIR` も棚卸しする
2. **OS store維持の場合。** 不要な環境変数をそのプロセスの起動設定から外すことを検討する。緊急の互換対応は `x509sslcertoverrideplatform=0` を起動時から適用して再起動する。これは検証全体を無効化する手段ではないが、bundleだけにあった社内CAを再び見失う可能性はある
3. **bundle置換の場合。** 新動作を選び、必要なCA、パス、permission、Windowsの `;` 区切り、更新・配布責任を管理する。追加CA一つだけのbundleを、OSの全rootに「足すだけ」のつもりで設定しない。外部APIや社内proxyを含め、従来成功した接続も確認する
4. **アプリ単位の追加の場合。** `SystemCertPool` から得たcopyへ `AppendCertsFromPEM` する設計なら、その元poolが既に環境変数によって置き換わっていないかを確認する。戻り値trueは少なくとも一つのcertをparseできたという意味で、bundle全体の完全性検証ではない。`NewCertPool` は空から始まる。poolを設定へ渡した後の並行変更は避ける
5. **rollbackの寿命を管理する。** [GODEBUG history](https://go.dev/doc/godebug#go-127) はGo 1.31での当該設定の削除を予定する。予定を恒久保証とせず、回避策を入れた理由・対象・解消条件を残す。古い `go` directiveを残すだけの暗黙rollbackも、将来の更新時に再発し得る

`InsecureSkipVerify=true` をこの移行の回避策にしない。通常のchain / hostname検証を省き、独自検証がなければ中間者攻撃に弱くなる契約であり、trust storeの切替とは別の危険を作る。[tls.Config](https://pkg.go.dev/crypto/tls@go1.27.1#Config)

## 本番前の確認案と未確認事項

- 対象OSの別プロセスで、環境変数未設定・非空bundle指定・一時互換設定の3条件を比較する。cacheの影響を避け、実際のdaemon起動経路でも確認する
- OSだけで信頼されるCA、bundleだけに含めたCA、意図的に含めないCAの接続を用意し、「成功すべき」「拒否すべき」を両方検査する。実行環境のtrust storeを変更することを、この文書が許可するわけではない
- 不存在path、空file、parseできないPEM、一部だけ有効なbundle、directory listの区切り違いを試す。初期化のerrorだけでなくTLS handshakeの結果で判定する
- 既存poolや `SetFallbackRoots`、`VerifyConnection` / `VerifyPeerCertificate` を使う箇所は別経路として試験する。`SetFallbackRoots` はcustom rootsがない場合等のfallback用で、OS storeとの自動unionではない。[API](https://pkg.go.dev/crypto/x509@go1.27.1#SetFallbackRoots)
- 本稿ではmacOS / Windows / iOSの実機handshake、組織固有CA、失効確認、bundle reload、負荷性能は未検証。Linux上のmetadata probeはこれらの代替にならない。検索evalは発見可能性の確認に限る

## 出典・ライセンス・鮮度

Go Authors の資料を日本語で独自に整理した。go.devの文章は [CC-BY-4.0、コードはBSD](https://go.dev/copyright)。go1.27.1のAPIと固定実装は [BSD-3-Clause](https://raw.githubusercontent.com/golang/go/862c888e612ac346c7c4d99c9392bdfd265f33b0/LICENSE)。コード転載・module昇格は行っていない。

主要公開文書とroot.goはnative webで本文を開いた。固定table.goのnative web取得はcache missだったため、同じ40桁SHAのpublic raw HTTPで取得しlocked toolchainの同ファイルとbyte単位で一致を確認した。継続更新のGODEBUGページには個別の公開日表示がないため、取得日を変更の公開日に読み替えない。全sourceの取得日は2026-10-02 UTC、release_notesの30日TTLに合わせた明示期限は2026-11-01。

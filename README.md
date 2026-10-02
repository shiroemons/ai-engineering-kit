# AI Engineering Kit

個人用の Engineering Knowledge Base と再利用ライブラリ。一次情報の収集から、期限判定・検索・実装・テスト・再利用までをローカルで回す。保守者はリポジトリ所有者で、AI が追記した内容も検証対象とする。

## 開始する

[mise](https://mise.jdx.dev/) で Go・[just](https://github.com/casey/just)・golangci-lint を管理する。`mise.toml` は各ツールに `latest` を指定する。実装は Go 標準ライブラリだけを使う。リポジトリのルートで実行する。

macOS arm64 と Linux amd64 を対象とする。先に [mise の公式手順](https://mise.jdx.dev/installing-mise.html) で mise を導入し、Bash・Git・jq と C コンパイラを用意する。`just test` の `go test -race` は [C コンパイラと cgo](https://go.dev/doc/articles/race_detector#Requirements) を必要とする。macOS では Xcode Command Line Tools と jq を使う。Linux では手動調査時の排他制御に util-linux の `flock`、プロセス確認に procps の `ps` も使う。

```sh
# Debian / Ubuntu
sudo apt-get update
sudo apt-get install --no-install-recommends build-essential bash git jq util-linux procps

# Fedora
sudo dnf install gcc bash git jq util-linux procps-ng
```

```sh
mise trust
mise install --locked go just golangci-lint
mise exec -- just test
mise exec -- just validate
mise exec -- just freshness
mise exec -- just index
mise exec -- just build
./bin/kb search context --json
```

検索結果に `knowledge`・`pattern`・`module` が各1件現れる。seed は公式 API を確認した小さな例で、技術全体の網羅や最新バージョンの保証を目的としない。`mise exec -- just check` で書式・modernizer・go vet・golangci-lint・race test・検証・検索をまとめて確認する。

以降のコマンドは mise を有効にしたシェルで使う。有効化していない場合は `mise exec --` を前に付ける。`mise.lock` は検証した Go 1.27.1・Just 1.58.0・golangci-lint 2.13.2 を固定する。macOS arm64 と Linux amd64（mise の表記では `linux-x64`）用の取得 URL と checksum を記録する。`--locked` は対象プラットフォームの lock 情報が不足すると失敗するため、未検証の版へ自動的に更新しない。

[CI](.github/workflows/check.yml) は Ubuntu amd64 と macOS arm64 で同じ `mise exec -- just check` を実行し、Go の race test と共通の shell test を検証する。launchd の plist 検証は macOS で引き続き実行する。

更新時は次を個別に実行し、差分を確認する。`latest` は更新候補の指定、lock は再現する版の指定として使う。

```sh
mise lock --bump --platform linux-x64,macos-arm64 go just golangci-lint
mise install --locked go just golangci-lint
mise exec -- just fix
mise exec -- just check
```

`just fix` は Go 1.26 以降の modernizer を適用する。`just fix-check` は変更せず残る提案を検出する。`just vet` と `just lint` は静的解析を個別に実行する。Go 1.27 の厳密な JSON 検証など、採用理由は [ADR](docs/adr/0001-local-knowledge-base.md) に記録する。

版を変更せず対応プラットフォームの情報だけを再生成する場合は `mise lock --platform linux-x64,macos-arm64 go just golangci-lint` を使い、既存の版・URL・checksum が意図せず変わっていないか確認する。

## 配置先を選ぶ

| ディレクトリ | 責務 |
|---|---|
| knowledge/ | 技術の要点・適用範囲・注意点 |
| patterns/ | 設計判断とトレードオフ |
| modules/ | テストと由来を確認した再利用コード |
| sources/catalog/ | URL・版・取得日・ライセンス |
| sources/licenses/ | ライセンス確認の記録 |
| evals/ | 検索とコードの期待する振る舞い |
| rag/ | 再生成できる検索データ |
| config/ | 鮮度と信頼度の方針 |
| cmd/・internal/ | CLI と検索の実装 |
| docs/・examples/ | 運用と記入用サンプル |
| .workbench/repositories/ | Git 管理しない調査用 clone |

## 知識を追加して再利用する

[運用手順](docs/workflows.md) の順で進める。Source → Research → Knowledge → Pattern → Prototype → Tests → Evals → Review → Module。Knowledge の保存で止めてもよい。module 化は用途と再利用価値が明確な場合に限る。

knowledge の記入例は [template](examples/knowledge.template.md)、必須項目は [metadata 契約](docs/metadata.md) を使う。OSS は commit を固定して設計を分析し、ライセンスと由来を記録する。8種類の pattern template は未調査の問いを示し、検索対象から除外する。

## PR を作成する

コード変更・調査のどちらも [共通 PR テンプレート](.github/pull_request_template.md) を使う。概要・変更内容・出典・検証結果・未確認事項を同じ順序で記録する。記入方法は [運用手順](docs/workflows.md#pr-を作成する) を参照する。

## 鮮度を確認して検索する

```sh
./bin/kb validate
./bin/kb freshness --as-of 2026-12-22 --json
./bin/kb index
./bin/kb search "context cancellation" --limit 10 --json
./bin/kb stats --json
```

全コマンドで `--root PATH`・`--json`・`--as-of YYYY-MM-DD` を指定できる。既定の基準日は現在の UTC 日付。`expires_at` と設定の TTL から最も早い期限を採用し、その日の開始から stale とする。source の古い取得日も期限に反映する。詳細は [metadata 契約](docs/metadata.md) を参照する。

期限切れは削除しない。検索順は active → stale、信頼度、関連度、パスの順で決める。本文や metadata の大文字小文字を区別せず、空白区切りの全語が含まれる文書を探す。日本語の形態素解析はしない。

index は事前に生成する。入力や設定が変わると検索が再生成を要求するため、`kb index` を実行する。鮮度は検索時にも判定する。JSON 結果にはパス、タイトル、種別、技術、版、関連度、信頼度、取得日、期限、stale を含む。

## Codex から利用する

[AGENTS.md](AGENTS.md) を読み、実装前に `kb search` で候補を探す。結果の本文、適用版、source、provenance を開いて採用を判断する。stale は再調査し、module のテストと eval を通してから使う。

### GPT で一次情報を収集する

[GPT research skill](.agents/skills/research-gpt/SKILL.md) は現在の GPT タスクが直接調査する専用入口。dot では GPT-6 Astra を選び、1回につき最大3テーマを扱う。外部 API キーや OpenCode は不要で、既存の OpenCode 版と設定を共有するのは調査領域・選定方針・検証契約だけ。remote lease、latest-main の隔離 branch、成果物の厳格検証、PR/CI/merge の手順は [GPT Research](docs/research-gpt.md) を参照する。

`mise exec -- just research-gpt-plan 3` で調査領域の候補を確認できる。これはモデルや定期実行を起動しない。毎時の実行はタスクサービス側で設定し、前回の未完了作業がある場合は新規調査を重ねない。

## 適用範囲

Linux では CLI と検証に加えて、`just research-preflight`・`just research-dry-run`・`just research`・`just research-loop` の手動実行を対象とする。これら OpenCode 版の調査には OpenCode と利用モデルの設定も必要になる。`research-install`・`research-uninstall`・`research-status` による launchd の定期実行は macOS 専用で、Linux の systemd / cron の登録は実装していない。Linux での設定ファイル・ログの既定パスと必要な環境変数は [Research Pipeline](docs/research-automation.md) を参照する。

自動のネット調査は毎時、無料モデル2つで別テーマを並列に調べる。予定上は1日24回・最大48テーマ。`just research-loop` はMuse Sparkだけで最長8時間繰り返す。定期実行と連続実行は別ワークツリーで同時に動く。8領域の未調査分野を優先し、公式文書に加えてOSS設計や実務事例を調べる。設定と停止条件は [Research Pipeline](docs/research-automation.md) を参照する。

Embedding・Vector DB は未実装。検索の差し替え方針は [ADR](docs/adr/0001-local-knowledge-base.md) に記録する。検証は形式や参照整合性を確かめるもので、記述の真実性や人間のレビュー完了を保証しない。

本リポジトリの配布ライセンスは未選定。source のライセンス表記は参照資料の識別であり、本リポジトリへ適用する宣言ではない。

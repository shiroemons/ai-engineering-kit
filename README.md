# AI Engineering Kit

個人用の Engineering Knowledge Base と再利用ライブラリ。一次情報の収集から、期限判定・検索・実装・テスト・再利用までをローカルで回す。保守者はリポジトリ所有者で、AI が追記した内容も検証対象とする。

## 開始する

[mise](https://mise.jdx.dev/) で Go・[just](https://github.com/casey/just)・golangci-lint を管理する。`mise.toml` は各ツールに `latest` を指定する。実装は Go 標準ライブラリだけを使う。リポジトリのルートで実行する。

```sh
mise trust
mise install go just golangci-lint
mise exec -- just test
mise exec -- just validate
mise exec -- just freshness
mise exec -- just index
mise exec -- just build
./bin/kb search context --json
```

検索結果に `knowledge`・`pattern`・`module` が各1件現れる。seed は公式 API を確認した小さな例で、技術全体の網羅や最新バージョンの保証を目的としない。`mise exec -- just check` で書式・modernizer・go vet・golangci-lint・race test・検証・検索をまとめて確認する。

以降のコマンドは mise を有効にしたシェルで使う。有効化していない場合は `mise exec --` を前に付ける。`mise.lock` は検証した Go 1.27.1・Just 1.58.0・golangci-lint 2.13.2 を固定する。macOS arm64 用の取得 URL と checksum を記録する。

更新時は次を個別に実行し、差分を確認する。`latest` は更新候補の指定、lock は再現する版の指定として使う。

```sh
mise lock --bump go just golangci-lint
mise install go just golangci-lint
mise exec -- just fix
mise exec -- just check
```

`just fix` は Go 1.26 以降の modernizer を適用する。`just fix-check` は変更せず残る提案を検出する。`just vet` と `just lint` は静的解析を個別に実行する。Go 1.27 の厳密な JSON 検証など、採用理由は [ADR](docs/adr/0001-local-knowledge-base.md) に記録する。

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

## 適用範囲

自動のネット調査は3時間ごと、1日8回の予定で1テーマずつ進める。8領域の未調査分野を優先し、公式文書に加えてOSS設計や実務事例を調べる。設定と停止条件は [Research Pipeline](docs/research-automation.md) を参照する。

Embedding・Vector DB は未実装。検索の差し替え方針は [ADR](docs/adr/0001-local-knowledge-base.md) に記録する。検証は形式や参照整合性を確かめるもので、記述の真実性や人間のレビュー完了を保証しない。

本リポジトリの配布ライセンスは未選定。source のライセンス表記は参照資料の識別であり、本リポジトリへ適用する宣言ではない。

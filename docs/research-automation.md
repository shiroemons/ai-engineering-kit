# 自動 Research Pipeline

`launchd` のユーザー LaunchAgent が毎日ローカル時刻 04:00 に `scripts/research-next.sh` を起動する。1回に1テーマを選び、OpenCode の `knowledge-researcher` が一次情報を確認して knowledge・source catalog・検索 eval を更新する。runner は検証と変更範囲の確認後、専用 branch に commit して push し、PR を作る。PR に競合がなければ squash の auto-merge を有効にする。GitHub の必須チェックなどがあれば、条件を満たした後に GitHub がマージする。

## セットアップ

OpenCode v2、GitHub CLI (`gh`) と認証済みアカウント、`mise`、`just`、`jq`、`curl` を利用する。GitHub repository の auto-merge を有効にする（例: `gh repo edit --enable-auto-merge`）。runner は `opencode models --print-logs --log-level debug` で利用可能なモデルを調べ、models.dev の最新料金情報で input/output がともにゼロの候補だけを使う。設定モデルを最初に試し、利用できない場合は一覧にある無料モデルを最大2つ候補に加える。モデル一覧と料金情報の取得は最大3回、各モデルでの Research は最大2回試す。OpenCode が失敗して作業ツリーに変更が残った場合は、重複編集を防ぐため再試行せず停止する。

現在の CLI では `models --refresh --verbose` および `agent list` は使えず、agent の確認は `opencode debug agents` を使う。

`~/Library/Application Support/ai-engineering-kit/research.env` に以下の1行を保存する。このローカル設定は Git 管理しない。設定モデルが利用不能、または最新の料金をゼロと確認できない場合は、利用可能な無料モデルへ切り替える。無料と確認できる候補がなければ実行を中止する。

```text
OPENCODE_RESEARCH_MODEL=opencode/<verified-free-model-id>
```

```sh
just research-install
just research-status
just research-dry-run
just research
```

時刻変更は `bash scripts/install-research-agent.sh --hour 5 --minute 30`。停止は `just research-uninstall`、再開は `just research-install`。失敗理由は端末の標準エラーと `~/Library/Logs/ai-engineering-kit/research-error.log` に表示・記録する。`research.log` には実行結果を記録する。

## 安全条件

runner は main・clean working tree・排他 lock を確認する。`git pull --ff-only` は最新化のために試みるが、失敗しても現在のローカル main で Research を続け、`research.log` に警告を記録する。Research は `research/<日時>` branch 上で行う。OpenCode は repo 内の custom agent とコマンドを使い、編集範囲を `knowledge/`・`sources/catalog/`・`evals/knowledge/` に制限する。runner が実際の差分を再検査し、knowledge と eval の成果物、source catalog の参照整合性、検証成功、staged diff を確認してから commit・push・PR 作成する。競合のない PR には `gh pr merge --squash --auto` を依頼する。競合がある場合や GitHub が auto-merge を受け付けない場合は PR を open のまま残す。PR 作成後、runner はローカル checkout を main に戻す。push や PR 作成に失敗した場合、commit はローカルの research branch に残し、clean tree なら checkout は main に戻す。未 commit の差分が残った場合は branch を保持し、次の自動実行前に調査が必要。ログには OpenCode の応答全文を保存しない。

dry run は OpenCode に読み取りだけの topic 選択を依頼し、変更がなかったことと既存データの validation を確認する。dry run では pull・branch 作成・commit・push・PR 作成をしない。

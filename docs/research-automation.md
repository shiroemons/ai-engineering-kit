# 自動 Research Pipeline

`launchd` のユーザー LaunchAgent が毎日ローカル時刻 04:00 に `scripts/research-next.sh` を起動する。1回に1テーマを選び、OpenCode の `knowledge-researcher` が一次情報を確認して knowledge・source catalog・検索 eval を更新する。runner は `just validate`・`just index`・`just check` が成功し、変更範囲が許可内の場合に限り main にローカル commit する。push はしない。

## セットアップ

OpenCode v2、`mise`、`just`、`jq`、`curl` を利用する。`opencode models --print-logs --log-level debug` で現在利用可能なモデルを確認し、[OpenCode の料金表](https://opencode.ai/docs/zen) と models.dev の料金情報で input/output がゼロのモデルだけを選ぶ。現在の CLI では `models --refresh --verbose` および `agent list` は使えず、agent の確認は `opencode debug agents` を使う。

`~/Library/Application Support/ai-engineering-kit/research.env` に以下の1行を保存する。このローカル設定は Git 管理しない。モデルが利用不能、または最新の料金をゼロと確認できない場合、runner は実行を中止する。

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

runner は main・clean working tree・排他 lock を確認し、`git pull --ff-only` の失敗時に停止する。OpenCode は repo 内の custom agent とコマンドを使い、通常 Research の編集範囲を `knowledge/`・`sources/catalog/`・`evals/knowledge/` に制限する。runner が実際の差分を再検査し、knowledge と eval の成果物、source catalog の参照整合性、検証成功、staged diff を確認してから commit する。既存の検証済み source は再利用できる。異常時に自動 rollback はしない。失敗後の変更は調査してから手動で扱う。ログには OpenCode の応答全文を保存しない。

dry run は OpenCode に読み取りだけの topic 選択を依頼し、変更がなかったことと既存データの validation を確認する。dry run では pull と commit をしない。

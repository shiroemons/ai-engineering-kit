# 自動 Research Pipeline

`launchd` のユーザー LaunchAgent が3時間ごと、1日8回の予定でリサーチを起動する。実行時刻はローカル時刻の 00:00・03:00・06:00・09:00・12:00・15:00・18:00・21:00。1回に1テーマを選び、OpenCode の `knowledge-researcher` が8領域の一次情報を調べる。

更新対象は knowledge・source catalog・検索 eval。`scripts/research-next.sh` が検証と変更範囲を確認し、専用 branch に commit して push し、PR を作る。競合のない PR では squash の auto-merge を有効にする。必須チェックなどの条件を満たした後に GitHub がマージする。

## 調査対象

対象技術と候補は [config/research.json](../config/research.json) の `domains` を正本とする。例示したテーマや URL は調査の出発点であり、固定の消化順ではない。

| 領域 | 調査例 |
|---|---|
| 言語・バックエンド | バックグラウンドジョブの設計 |
| フロントエンド・UX | フォームのエラー表示 |
| AIアプリケーション | RAGの検索評価 |
| データ基盤 | 無停止マイグレーション |
| API・分散システム | Webhookの冪等性 |
| セキュリティ | 認証と認可の責務 |
| インフラ・開発環境 | CI/CDのロールバック |
| 品質・運用 | 障害報告から学ぶ監視設計 |

選定方針は `coverage_first` とし、未調査の領域、蓄積の少ない領域、最近扱っていない領域の順に優先する。直近16件の knowledge の変更履歴も参照し、別の有用な候補があれば同じ技術の連続調査を避ける。重大な誤り、破壊的な変更、module が利用する重要知識の期限切れは優先できる。その場合は選定理由に例外を記録する。件数合わせのために価値の低いテーマを選ばない。

文書には主題に対応する `research-domain:<domain-id>` タグを1つ付ける。既存文書にタグがなければ、設定の技術名から領域を判定する。同じ文書の再編集を別文書として数えない。設定に含まれる技術は未調査でも新規技術の月1件制限から除外する。制限の対象は設定外の技術だけとし、取得日の更新ではなく最初の commit で月を判定する。

公式文書に加え、OSSの実装、開発元の技術記事、運用当事者の障害報告を調べる。APIの契約は公式文書、OSSの挙動は固定した commit を根拠にする。事例には対象版や負荷条件を残し、確認した事実と独自の設計提案を区別する。開発元の記事には `maintainer_article`、障害報告には `incident_report` を使う。信頼区分と期限は [metadata 契約](metadata.md) に従う。

領域の優先順や調査の切り口はエージェントへの指示であり、runner が選択結果を機械的に保証するものではない。選定結果はログの `selected topic` と成果物で確認する。

## セットアップ

OpenCode v2、GitHub CLI (`gh`) と認証済みアカウント、`mise`、`just`、`jq`、`curl` を利用する。GitHub repository の auto-merge を有効にする（例: `gh repo edit --enable-auto-merge`）。

runner は `opencode models --print-logs --log-level debug` で利用可能なモデルを調べる。models.dev の最新料金情報で input/output がともにゼロの候補だけを使う。設定モデルを最初に試し、利用できない場合は一覧にある無料モデルを最大2つ候補に加える。モデル一覧と料金情報の取得は最大3回、各モデルでの Research は最大2回試す。OpenCode が失敗して作業ツリーに変更が残った場合は、重複編集を防ぐため再試行せず停止する。

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

頻度の既定値は `config/research.json` の `schedule.interval_hours` と `schedule.minute` で指定する。設定変更後は `just research-install` で再登録し、`just research-status` の calendar interval を確認する。インストーラーは稼働中のリサーチを中断しないよう、lock が存在する間は停止する。

一時的な上書きと、登録せずに plist だけを生成する例を示す。間隔には24を割り切る正の整数、分には0〜59を指定する。`--hour` は従来どおり1日1回の指定で、`--interval-hours` とは併用できない。引数による上書きは設定ファイルを変更せず、次の既定インストールでは設定値に戻る。

```sh
bash scripts/install-research-agent.sh --interval-hours 2 --minute 0
bash scripts/install-research-agent.sh --hour 5 --minute 30
bash scripts/install-research-agent.sh --output /tmp/research-preview.plist
```

Macのスリープ中に複数の予定時刻を過ぎた場合、復帰時の起動は1回にまとめられる。実行中の重複起動は lock で抑止する。このため1日8件の成果物を保証する設定ではない。

停止は `just research-uninstall`、再開は `just research-install`。失敗理由は端末の標準エラーと `~/Library/Logs/ai-engineering-kit/research-error.log` に表示・記録する。`research.log` には実行結果を記録する。繰り返し失敗する場合は停止し、保守者がモデルの利用可否、GitHub接続、作業ツリーを確認する。lock は実行中のプロセスがないことを確認してから除去する。

## 安全条件

runner は main・clean working tree・排他 lock を確認する。設定変更が未 commit の間も起動条件を満たさない。`git pull --ff-only` は最新化のために試みる。失敗した場合は警告を記録し、現在のローカル main で Research を続ける。

Research は `research/<日時>` branch 上で行う。OpenCode の編集範囲は `knowledge/`・`sources/catalog/`・`evals/knowledge/` に制限する。runner は実際の差分、knowledge と eval の成果物、source catalog の参照整合性を検査する。検証と staged diff の確認を通過した後に commit・push・PR 作成を行う。

競合のない PR には `gh pr merge --squash --auto` を依頼する。競合や auto-merge の受付失敗があれば PR を open のまま残す。PR 作成後、runner はローカル checkout を main に戻す。push や PR 作成に失敗した場合、commit はローカルの research branch に残す。作業ツリーが clean なら main に戻す。未 commit の差分があれば branch を保持し、次の自動実行前に保守者が調査する。ログには OpenCode の応答全文を保存しない。

dry run は OpenCode に読み取りだけの topic 選択を依頼し、変更がなかったことと既存データの validation を確認する。dry run では pull・branch 作成・commit・push・PR 作成をしない。

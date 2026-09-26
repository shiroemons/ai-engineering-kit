# 自動 Research Pipeline

`launchd` のユーザー LaunchAgent が毎時00分、1日24回の予定でリサーチを起動する。無料モデル2つが別領域を並列に調べるため、予定上は最大48テーマ/日になる。OpenCode の `knowledge-researcher` は1モデルにつき1テーマを担当する。

手動の `just research-loop` はMuse Sparkだけで1テーマずつ繰り返す。既定の上限は8時間。定期実行と連続実行は独立したワークツリーとlockを使い、同時に動かせる。重なった場合は最大3モデル実行となる。

更新対象は knowledge・source catalog・検索 eval。`cmd/research-batch` が各モデルの成果を検証し、成功分を統合する。`scripts/research-next.sh` が全体のチェックを通した後、専用branchにcommitしてpushし、PRを作る。定期実行と連続実行は別PRになる。競合のないPRではsquashのauto-mergeを有効にする。必須チェックなどの条件を満たした後にGitHubがマージする。

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

選定方針は `coverage_first` とし、runnerが未調査の領域、蓄積の少ない領域、最近扱っていない領域の順に割り当てる。直近16件のknowledgeの変更履歴も参照する。定期実行と連続実行は領域の使用状況を共有し、調査中の領域を別workerに割り当てない。workerは担当領域の設定済み技術から具体的な問いを選ぶ。件数合わせのために価値の低いテーマを選ばない。

文書には主題に対応する `research-domain:<domain-id>` タグを1つ付ける。既存文書にタグがなければ、設定の技術名から領域を判定する。同じ文書の再編集を別文書として数えない。設定に含まれる技術は未調査でも新規技術の月1件制限から除外する。制限の対象は設定外の技術だけとし、取得日の更新ではなく最初の commit で月を判定する。

公式文書に加え、OSSの実装、開発元の技術記事、運用当事者の障害報告を調べる。APIの契約は公式文書、OSSの挙動は固定した commit を根拠にする。事例には対象版や負荷条件を残し、確認した事実と独自の設計提案を区別する。開発元の記事には `maintainer_article`、障害報告には `incident_report` を使う。信頼区分と期限は [metadata 契約](metadata.md) に従う。

領域・技術と成果物のタグの一致はrunnerが検査する。具体的な問いの価値と主張の正確さはエージェントへの指示と保守者のレビューで確認する。選定結果はログの `worker passed`・`selected topic` と成果物で確認する。

## セットアップ

OpenCode v2、GitHub CLI (`gh`) と認証済みアカウント、`mise`、`just`、`jq`、`curl` を利用する。GitHub repository の auto-merge を有効にする（例: `gh repo edit --enable-auto-merge`）。

runnerは `opencode models --print-logs --log-level debug` で利用可能なモデルを調べる。models.devの最新料金情報で入力・出力・キャッシュの料金がゼロで、tool callingに対応する候補だけを使う。料金未設定のキャッシュ項目は追加料金なしとして扱う。モデル一覧と料金情報の取得は最大3回試す。

定期実行では `parallel.preferred_models` のMuse SparkとMiMoを優先する。使えない候補はローカル設定モデル、利用可能な無料モデルの順で補う。1モデルしか使えない場合は1テーマに減らす。連続実行では `continuous.model` のMuse Sparkだけを使い、利用不能・有料化・料金取得失敗では終了する。

2026-09-26時点で `opencode/muse-spark-1.3-contributor-free` は期間限定の無料モデル。プロンプトと回答はMetaの学習に使われ得る。無料の具体的な上限や終了日は保証されていない。[OpenCodeの料金・プライバシー条件](https://opencode.ai/docs/zen/)を確認し、公開情報の調査に使う。

各workerは最大45分で停止する。失敗したworkerを別モデルで再試行しない。片方だけ成功した場合は、その成果を検証してPRにまとめ、失敗分のワークツリーを保持する。利用制限・quotaを示すエラーを受けたら実行中のworkerを止め、共通のcooldownを3時間設定する。他方の実行もcooldownを検知して停止する。これは無料上限の推定値ではなく、runnerの再試行抑制時間である。

現在の CLI では `models --refresh --verbose` および `agent list` は使えず、agent の確認は `opencode debug agents` を使う。

`~/Library/Application Support/ai-engineering-kit/research.env` に以下の1行を保存する。このローカル設定はGit管理しない。定期実行の補欠候補として使う。無料と確認できる候補がなければ実行を中止する。

```text
OPENCODE_RESEARCH_MODEL=opencode/<verified-free-model-id>
```

```sh
just research-install
just research-status
just research-preflight
just research-dry-run
just research
just research-loop
```

頻度の既定値は `config/research.json` の `schedule.interval_hours` と `schedule.minute` で指定する。設定変更後は `just research-install` で再登録し、`just research-status` の calendar interval を確認する。インストーラーは稼働中のリサーチを中断しないよう、lock が存在する間は停止する。

一時的な上書きと、登録せずに plist だけを生成する例を示す。間隔には24を割り切る正の整数、分には0〜59を指定する。`--hour` は従来どおり1日1回の指定で、`--interval-hours` とは併用できない。引数による上書きは設定ファイルを変更せず、次の既定インストールでは設定値に戻る。

```sh
bash scripts/install-research-agent.sh --interval-hours 2 --minute 0
bash scripts/install-research-agent.sh --hour 5 --minute 30
bash scripts/install-research-agent.sh --output /tmp/research-preview.plist
```

Macのスリープ中に複数の予定時刻を過ぎた場合、復帰時の起動は1回にまとめられる。同じ実行モードの重複起動はlockで抑止する。このため1日48件の成果物を保証する設定ではない。スリープ解除やスリープ防止は設定しない。

連続実行は `continuous.hours` の8時間以内で開始・調査を行い、完了後に30秒待って次へ進む。期限に達したworkerは停止する。PRなどの後処理は期限後に終了する場合がある。次のテーマを選ぶ前に、そのPRのマージを最大10分待つ。マージ待ちが続く場合、競合、検証失敗、モデルエラーでは連続実行を終了する。端末で `Ctrl+C` を押すとworkerとその子プロセスを停止する。

停止は `just research-uninstall`、再開は `just research-install`。失敗理由は端末の標準エラーと `~/Library/Logs/ai-engineering-kit/research-error.log` に表示・記録する。`research.log` には実行結果を記録する。繰り返し失敗する場合は停止し、保守者がモデルの利用可否、GitHub接続、作業ツリーを確認する。lock は実行中のプロセスがないことを確認してから除去する。

`just research-preflight` は、main・作業ツリー・lock・cooldown・必要コマンド・GitHub CLI認証・モデル一覧・無料料金を確認する。選んだモデルを表示して終了し、調査・worktree作成・fetch・commit・push・PR作成は行わない。モデルへの推論要求は送らないが、一覧と料金の取得には通信する。GitHubへのpush権限や調査の成功までは保証しない。

`working tree is dirty` の場合、`research.log` に変更パスを最大20件記録する。評価成果物の `.workbench/evaluations/` はGitの除外対象とし、レポートを保存したまま起動できる。それ以外の未commit変更に対する停止条件は維持する。

OpenCode v2.0.18では初期化直後にモデル一覧が空でも終了コード0となることを確認した。[公式API仕様](https://github.com/anomalyco/opencode/blob/v2.0.18/packages/protocol/src/groups/model.ts)でも初期化完了前の一覧を返し得る。空応答とコマンド失敗をログで区別し、同じ常駐サービスへの再試行で確認する。初期化を毎回やり直す `models --standalone` へ置き換えない。3回とも空なら無料モデルを推測せず停止する。

## 安全条件

runnerは元のcheckoutがmainで未commit変更がないことを確認する。設定変更が未commitの間も起動条件を満たさない。開始時に `git fetch origin main` を試み、成功時はremote main、失敗時は警告を記録してlocal mainから専用ワークツリーを作る。元のcheckoutのブランチやファイルは変更しない。マージ後のlocal main更新は保守者が `git pull --ff-only` で行う。

統合用のbranchは `research/<日時>-<PID>` とする。各モデルは同じcommitから作ったdetached worktreeで調べる。配置先は `.workbench/repositories/research/`。OpenCodeの編集範囲は `knowledge/`・`sources/catalog/`・`evals/knowledge/` に制限する。workerはknowledge文書を1つ、検索evalを `evals/knowledge/<領域ID>.json` に書く。既存のevalを保持し、今回の文書IDを検索で検出できるケースを追加する。

runnerは変更パス・タグ・技術・source参照・検索evalを検査する。削除・リネーム・symlinkを受け付けない。成功分を順番に仮統合して再検証し、同じファイルへの異なる変更は競合として保持する。最終成果は統合用ワークツリーへまとめて適用し、`just check` とstaged diffの確認後にcommit・push・PR作成を行う。

競合のないPRには `gh pr merge --squash --auto` を依頼する。定期実行と連続実行のPRが競合した場合はopenのまま残し、失敗を報告する。pushやPR作成の失敗でもcommitはresearch branchに残る。統合済みのworkerとcleanな統合用ワークツリーは削除し、失敗したworkerや未commitの成果は復旧用に保持する。保守者はログの `recovery` と `git worktree list` で場所を確認し、差分の要否を判断してから片付ける。ログにはOpenCodeの応答全文を保存しない。

実行lockは `research.lock`・`research-continuous.lock`・`research-loop.lock`。領域lockは `domains/<ID>.lock`、待機期限は `cooldown-until`。いずれもログディレクトリに置く。異常終了後にlockが残った場合は、該当プロセスが終了していることを確認してから除去する。

dry runはdetached worktreeで読み取りだけのtopic選択を依頼し、変更がなかったことと既存データのvalidationを確認する。モデル利用は発生する。dry runではfetch・branch作成・commit・push・PR作成をしない。

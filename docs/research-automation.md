# 自動 Research Pipeline

`launchd` のユーザー LaunchAgent が毎時00分、1日24回の予定でリサーチを起動する。1回につき最大2つの無料モデルが別領域を並列に調べるため、予定上は最大48テーマ/日になる。OpenCode の `knowledge-researcher` は1モデルにつき1テーマを担当する。

手動の `just research-loop` はMuse Sparkを先頭に、利用可能な無料モデルを最大3つ使って調査する。既定の上限は8時間。workerは前のworkerがテーマを選び、重複確認を終え、一次資料の確認を始めた後に起動する。定期実行と連続実行はworkerごとに別のdetached worktreeを使う。`research-next.guard` のOS advisory lockで起動時の古いlock回収を直列化する。実行全体は共通の `research-pipeline.lock` で保護する。定期実行は進行中のパイプラインを待ち、連続実行は重なった場合status 3で終了して次の間隔で再試行する。同じ実行モードの重複起動もstatus 3で終了する。テーマ予約から検証、PR作成、マージ待ちまで同時に1パイプラインだけが進む。調査中のテーマは選定プロンプトに渡し、意味的な重複をモデルで判定してから予約する。表記を正規化した完全一致も予約時に検出する。

更新対象は knowledge・source catalog・検索 eval。`cmd/research-batch` が各モデルの成果を検証し、成功分を統合する。`scripts/research-next.sh` は全体のチェック後、件名に調査テーマを入れ、本文で変更の理由を説明する Conventional Commit を作ってから、専用branchにpushしてPRを作る。件名は `docs: <テーマ>の根拠と適用条件を記録`、本文は「一次資料に基づく知識と検索 eval を残し、後続の実装・運用で根拠を再利用できるようにする。」とする。定期実行と連続実行は別PRになる。競合のないPRではsquashのauto-mergeを有効にする。必須チェックなどの条件を満たした後にGitHubがマージする。

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

## 進捗表示

`just research-loop` は複数workerの平均推定進捗と、調査中のテーマ一覧を表示する。新しいworkerが加わった時に表示が後退しないよう、進捗バーは最高到達値を保つ。JSON進捗にはworkerごとの領域・テーマ・段階も残す。TTYでは進行中の同じ行を更新し、成功したテーマだけを100%の行として残す。失敗した試行の進捗行は消し、理由は端末と `research-error.log` に記録する。標準出力をファイルへリダイレクトした場合は、成功したテーマの100%行だけを記録する。進捗は8時間の実行時間に対する割合ではない。各試行の開始時は3%（領域割り当て後、テーマ候補の一次資料を確認中）、テーマ選定後に10%、一次資料の確認後に30%、knowledge 文書の完成後に55%、検索 eval の作成後に70%、検索結果の確認後に85%、runner 検証後に95%、成果の統合後に100%となる。TOPIC_SELECTED はテーマ選定の中間報告、TOPIC は成果物作成と最終検証の完了報告として区別する。10〜90%は調査エージェントが完了を報告した段階、95〜100%はrunnerが確認した段階を示すため、厳密な作業量や内容の完全性ではなく目安として扱う。

選定方針は `coverage_first` とし、runnerが未調査の領域、蓄積の少ない領域、最近扱っていない領域の順に割り当てる。直近16件のknowledgeの変更履歴も参照する。定期実行と連続実行は領域の使用状況を共有し、調査中の領域を別workerに割り当てない。workerは担当領域の設定済み技術から具体的な問いを選ぶ。先行workerが選定したテーマと同じ問いを扱わない。予約時には別のモデル呼び出しで、言い換えや細分化を含めて調査対象が重なるかを判定する。判定が曖昧または応答形式が不正なら、そのテーマを開始せず再選定する。件数合わせのために価値の低いテーマを選ばない。

文書には主題に対応する `research-domain:<domain-id>` タグを1つ付ける。既存文書にタグがなければ、設定の技術名から領域を判定する。同じ文書の再編集を別文書として数えない。設定に含まれる技術は未調査でも新規技術の月1件制限から除外する。制限の対象は設定外の技術だけとし、取得日の更新ではなく最初の commit で月を判定する。

公式文書に加え、OSSの実装、開発元の技術記事、運用当事者の障害報告を調べる。APIの契約は公式文書、OSSの挙動は固定した commit を根拠にする。事例には対象版や負荷条件を残し、確認した事実と独自の設計提案を区別する。開発元の記事には `maintainer_article`、障害報告には `incident_report` を使う。信頼区分と期限は [metadata 契約](metadata.md) に従う。

領域・技術と成果物のタグの一致はrunnerが検査する。具体的な問いの価値と主張の正確さはエージェントへの指示と保守者のレビューで確認する。選定結果はログの `worker passed`・`selected topic` と成果物で確認する。

## セットアップ

OpenCode v2、GitHub CLI (`gh`) と認証済みアカウント、`mise`、`just`、`jq`、`curl` を利用する。GitHub repository の auto-merge を有効にする（例: `gh repo edit --enable-auto-merge`）。

runnerは `opencode models --print-logs --log-level debug` で利用可能なモデルを調べる。models.devの最新料金情報で入力・出力・キャッシュの料金がゼロで、tool callingに対応する候補だけを使う。料金未設定のキャッシュ項目は追加料金なしとして扱う。モデル一覧と料金情報の取得は最大3回試す。

定期実行では `parallel.preferred_models` のMuse SparkとMiMoを優先し、使えない候補はローカル設定モデル、利用可能な無料モデルの順で補う。連続実行では `continuous.model` を先頭にし、続けて `parallel.preferred_models`、ローカル設定モデル、利用可能な無料モデルから補う。定期実行は `topics_per_run`、連続実行は `continuous.topics_per_run` までの異なるモデルを使い、候補が足りなければ実行数を減らす。Muse Sparkが利用不能・有料化・料金取得失敗の場合、連続実行は開始しない。

2026-09-26時点で `opencode/muse-spark-1.3-contributor-free` は期間限定の無料モデル。プロンプトと回答はMetaの学習に使われ得る。無料の具体的な上限や終了日は保証されていない。[OpenCodeの料金・プライバシー条件](https://opencode.ai/docs/zen/)を確認し、公開情報の調査に使う。

各workerは最大45分で停止する。失敗したworkerを別モデルで再試行しない。成功したworkerの成果だけを検証してPRにまとめ、失敗分のworktreeはrunの後処理が完了するまで保持する。利用制限・quotaを示すエラーを受けたら実行中のworkerを止め、共通のcooldownを3時間設定する。他方の実行もcooldownを検知して停止する。これは無料上限の推定値ではなく、runnerの再試行抑制時間である。

quota cooldown中は、provider呼び出しが必要な未完了batchの再開確認をcooldown終了まで延期する。`batch.json` が完了済みなら、保存済み成果の検証・統合にはproviderを使わないため、cooldown中でも再開できる。

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

連続実行は `continuous.hours` の8時間以内で開始・調査を行い、完了後に30秒待って次へ進む。期限に達したworkerは停止する。PRなどの後処理は期限後に終了する場合がある。次のテーマを選ぶ前に、そのPRのマージを最大10分待つ。調査に失敗した場合は即時終了せず、`pause_seconds` から最大300秒まで段階的に待って期限まで再試行する。成功後は失敗回数と待ち時間を初期化する。プロバイダのcooldown中も同じ間隔で再試行する。設定不備や認証エラー、PRの競合・未マージなど、再試行で解消しない状態では連続実行を終了する。端末で `Ctrl+C`、または実行プロセスへの `SIGTERM` を送ると、新しい工程を開始せずworkerを終了する。OpenCodeのプロセスグループにはまず `SIGTERM` を送り、3秒後も残っている場合だけ `SIGKILL` を送る。`SIGKILL` 自体は捕捉できないため、異常終了したrunは次回起動時に記録から判定する。

停止は `just research-uninstall`、再開は `just research-install`。失敗理由は端末の標準エラーと `~/Library/Logs/ai-engineering-kit/research-error.log` に表示・記録する。`research.log` には実行結果を記録し、`just check` の失敗時は最初の120行も `research-error.log` に保存する。再試行が続く場合はログを確認し、モデルの利用可否、GitHub接続、作業ツリーを調べる。lock は実行中のプロセスがないことを確認してから除去する。

`just research-preflight` は、main・作業ツリー・lock・cooldown・必要コマンド・GitHub CLI認証・モデル一覧・無料料金を確認する。選んだモデルを表示して終了し、調査・worktree作成・fetch・commit・push・PR作成は行わない。モデルへの推論要求は送らないが、一覧と料金の取得には通信する。GitHubへのpush権限や調査の成功までは保証しない。

`working tree is dirty` の場合、`research.log` に変更パスを最大20件記録する。評価成果物の `.workbench/evaluations/` はGitの除外対象とし、レポートを保存したまま起動できる。それ以外の未commit変更に対する停止条件は維持する。

OpenCode v2.0.18では初期化直後にモデル一覧が空でも終了コード0となることを確認した。[公式API仕様](https://github.com/anomalyco/opencode/blob/v2.0.18/packages/protocol/src/groups/model.ts)でも初期化完了前の一覧を返し得る。空応答とコマンド失敗をログで区別し、同じ常駐サービスへの再試行で確認する。初期化を毎回やり直す `models --standalone` へ置き換えない。3回とも空なら無料モデルを推測せず停止する。

## 安全条件

runnerは元のcheckoutがmainで未commit変更がないことを確認する。設定変更が未commitの間も起動条件を満たさない。開始時に `git fetch origin main` を試み、成功時はremote main、失敗時は警告を記録してlocal mainから専用ワークツリーを作る。調査・PR作成中は元のcheckoutを変更しない。PRのmerge後は元のcheckoutで `git pull --ff-only --prune` を実行してmainをfast-forwardし、originの古いremote-tracking refsもpruneする。refspecを省略して、remoteに設定された全ブランチのfetch範囲を使う。

完了済みbatchのresumeは保存済みworktreeで後処理するため、元のmainに未commit変更があっても続行できる。この場合は変更を保持し、PR merge後のlocal main同期を行わない。新規runとprovider呼び出しが必要なresumeでは、引き続きcleanなmain checkoutが必要。

統合用のbranchは `research/<日時>-<PID>` とする。各モデルは同じcommitから作ったdetached worktreeで調べる。配置先は `.workbench/repositories/research/`。OpenCodeの編集範囲は `knowledge/`・`sources/catalog/`・`evals/knowledge/` に制限する。workerはknowledge文書を1つ、検索evalを `evals/knowledge/<領域ID>.json` に書く。既存のevalを保持し、今回の文書IDを検索で検出できるケースを追加する。

runnerは変更パス・タグ・技術・source参照・検索evalを検査する。削除・リネーム・symlinkを受け付けない。成功分を順番に仮統合して再検証し、同じファイルへの異なる変更は競合として保持する。最終成果は統合用ワークツリーへまとめて適用し、`just check` とstaged diffの確認後にcommit・push・PR作成を行う。

競合のないPRには `gh pr merge --squash --auto` を依頼する。定期実行と連続実行のパイプラインは共通lockで直列になるが、外部PRなどとの競合でマージできない場合はopenのまま残し、失敗を報告する。pushやPR作成の失敗でもcommitはresearch branchに残る。正常終了時はrun専用のworker・統合worktree、ローカルbranch、journalを片付ける。未完了runは `~/Library/Logs/ai-engineering-kit/research-runs/<run-id>/` のjournalに、base commit、workerごとのmodel・domain・topic・工程、worktree、子プロセスPIDと開始時刻を保存し、該当するworktreeとleaseを保持する。子プロセスはPIDをjournalへ記録するまでOpenCodeを起動しない。次回のTTY起動では同じworktree・テーマから未完了工程をやり直すか、run所有のローカル状態を破棄するか選べる。resumeは中断したOpenCode session自体の再利用ではなく、その工程を保存済みworktreeで再実行する。LaunchAgentなどTTYのない起動では質問せず、runを保持したまま終了するため、新しい調査は始まらない。ローカル状態の破棄はrun IDとの一致を検証したworktree・lease・ローカルbranchだけを対象にし、push済みremote branchやPRは変更しない。起動時のlock回収は `lockf` のadvisory lockで直列化する。`research-next.guard` と `research-loop.guard` はそのために残る空のguard fileで、実行lockではない。ログにはOpenCodeの応答全文を保存しない。

実行lockは `research.lock`・`research-continuous.lock`・`research-loop.lock` と、PRの後処理まで含む共通の `research-pipeline.lock`。領域lockは `domains/<ID>.lock`、テーマlockは `topics/<SHA-256>.json`、テーマ選定のプロセス間lockはOSのファイルlock `topic-selection.lock`、待機期限は `cooldown-until`。いずれもログディレクトリに置く。テーマlockにはテーマ・領域・PID・run ID・開始時刻を記録し、正常終了時に削除する。次の選定では所有PIDが終了したテーマlockと、開始から48時間を超えたlockを排他lock内で削除する。領域lockには所有run IDとPIDを記録し、resume時には同じrunだけが再取得する。停止後はPIDとプロセスグループが残っていないことを確認してから、journalに記録された所有物を再利用または削除する。以前の形式など所有元を確認できないlockは自動で削除しない。`topic-selection.lock` はファイルを残したままにし、プロセス終了時にOSが排他lockを解放する。

dry runはdetached worktreeで読み取りだけのtopic選択を依頼し、変更がなかったことと既存データのvalidationを確認する。モデル利用は発生する。dry runではfetch・branch作成・commit・push・PR作成をしない。

# GPT による調査

GPT 版は [.agents/skills/research-gpt/SKILL.md](../.agents/skills/research-gpt/SKILL.md)
を現在の GPT タスクに読ませて使う。dot ではタスクのモデルを GPT-6 Astra にする。
API のモデル名を指定する仕組みではなく、外部 API キーや OpenCode は不要。
既存の [OpenCode Research Pipeline](research-automation.md) とモデル設定、parser、
launchd の設定はそのまま使える。共通なのは領域・coverage_first・metadata・source・
検索 eval の契約と検証処理で、GPT 版は1回につき最大3領域・各1テーマを扱う。

## 新しいタスクの開始

既存の認可済み環境でリポジトリを clone/fetch し、最新 main のスキルを読む。
クラウドの作業ディレクトリが前回から残っているとは仮定しない。恒久的な結果は
GitHub の branch/PR/main、実行状態は専用 coordinator branch を使う。
インストールは [README](../README.md) の Linux/macOS 共通手順に従う。

```sh
git clone https://github.com/shiroemons/ai-engineering-kit.git
cd ai-engineering-kit
mise trust
mise install --locked go just golangci-lint
mise exec -- just research-gpt-status
```

実行指示の例:

> 最新 main の .agents/skills/research-gpt/SKILL.md に従い、現在の GPT タスクで
> 最大3テーマを調査する。既存の未完了PRを先に確認し、remote leaseを取得する。
> 一次資料の確認、検証、PR、exact-head CI確認、承認済みのmerge、main同期まで進める。
> OpenCode設定やコードは変更しない。失敗や競合は保存して報告する。

定期実行は利用するタスクサービスで設定する。このスキルや `just` recipe は
モデルやタイマーを起動しない。毎時起動されても前の実行が残れば新規調査を止める。

## 小さな portable helper

```sh
# clean な作業ツリーで、領域の件数・順番と baseline を JSON 表示
mise exec -- just research-gpt-plan 3

# 取得済みの remote lease から latest-main の隔離 worktree を作成
bash scripts/research-gpt-prepare.sh RUN_ID CLAIM_SHA /path/to/new-worktree

# 調査 worktree で baseline からの全差分を検証（Git の stage/commit はしない）
mise exec -- just research-gpt-validate BASE_SHA 3
mise exec -- just check
```

plan の topics は1〜3。最大件数なので根拠の十分なテーマだけを採用できる。
validate は変更前の source record と eval case の保持、変更文書の domain/technology、
参照 source と検索結果、許可 path、ファイル種別とサイズを確認する。source の
再取得時は既存 record を書き換えず、新しい ID の record を追加する。

## 複数コンピューターでの排他

`research/gpt-coordinator` は main に merge しない運用状態専用 branch。
`state.json` の履歴を通常の fast-forward だけで追記する。claim は active、release
は idle の commit とし、次の claim は直前 commit を親にする。同じ親から作った
2つの claim は片方しか fast-forward できない。ローカルの flock は共有を仮定しない。
これは協調動作を前提とする lease であり、サーバー側の fencing や調査成果の
publication/merge と同時の原子的な所有権チェックはない。直前の lease 検証も
その後の失効や所有権変更を防がないため、各 worker が停止条件を守る必要がある。
既存 OpenCode はこのプロトコルを使わず、変更しない。OpenCode の更新は
調査 branch での最新 main の再取得・統合・重複確認・再検証で扱う。

coordinator の各 commit は親の tree 全体を保持し、通常ファイル（mode `100644`）の
`state.json` だけを置換する。初回の親は取得済みの main で、その tree に
`state.json` が既に存在する場合は種類を問わず停止する。初回に保持した main の
ファイル群は不変の baseline であり、coordinator を後の main と同期・merge しない。
既存の state.json-only coordinator もその親の tree をそのまま保持して利用できる。
不足ファイルの自動復元・移行はしない。coordinator は研究成果を置く branch ではない。

状態 helper の claim/release は候補 JSON を出すだけで claim を公開しない。
`tree` はローカル Git object を作り、`verify-commit` は候補 commit を検証する。
どちらも checkout・index・ref を変更せず、helper 自体は publication や remote ref
更新を行わない:

```sh
bash scripts/research-gpt-state.sh status
bash scripts/research-gpt-state.sh claim RUN_ID > /tmp/gpt-transition.json
# publication の後で実際の remote commit SHA を指定
bash scripts/research-gpt-state.sh verify RUN_ID CLAIM_SHA
# 終了時。元の worker の停止と作業保存を先に確認する
bash scripts/research-gpt-state.sh release RUN_ID > /tmp/gpt-transition.json
```

transition の `base_tree` は `expected_parent^{tree}` の正確な SHA。
`tree TRANSITION_FILE` と `verify-commit TRANSITION_FILE COMMIT_SHA` は transition
全体とこの一致を検証し、不正・欠落・ファイル種別の不一致は停止する。
`verify-commit` は親が `expected_parent` だけであること、`state.json` 以外の
全 tree が不変であること、state が mode `100644` の通常ファイルで候補 `state` と
JSON として等しいことを確認する（空白・object のキー順は一致不要）。

GitHub connector での publication は次の順序を守る:

1. transition の `state` を UTF-8 JSON blob にする
2. `create_tree` に transition の `base_tree` を必ず渡し、変更 entry は
   `state.json`（mode `100644`、作成した blob）1件だけにする。削除 entry や
   `base_tree` を省いた state.json-only tree は作らない
3. `expected_parent` だけを親に持つ commit を作る
4. 候補 commit SHA を control checkout に fetch し、
   `bash scripts/research-gpt-state.sh verify-commit /tmp/gpt-transition.json CANDIDATE_SHA`
   を実行する。失敗した候補は公開しない
5. 検証成功後にだけ、`create=true` なら branch を原子的に新規作成し、
   それ以外は `force:false` で ref を更新する

競合・guard の拒否は停止して報告する。失敗した操作を別の ref 操作や削除・再作成に
変えて回避しない。force push、branch 削除、既存 branch の上書き、認証情報のコピーは
しない。CLI push の認証がなくても connector で同じ検証と公開手順を使える。

既に Git の認証と author identity がある場合の publication は以下。各コマンドの
成功を確認する。Bash で実行し、失敗を無視して後続へ進めない。`tree` helper を使い、
空の tree から `git mktree` で state.json-only tree を作らない。

```sh
set -euo pipefail
tree=$(bash scripts/research-gpt-state.sh tree /tmp/gpt-transition.json)
parent=$(jq -r .expected_parent /tmp/gpt-transition.json)
commit=$(printf 'research: update GPT run lease\n' | git commit-tree "$tree" -p "$parent")
bash scripts/research-gpt-state.sh verify-commit /tmp/gpt-transition.json "$commit"
git push origin "$commit:refs/heads/research/gpt-coordinator"
git ls-remote --heads origin refs/heads/research/gpt-coordinator
```

push/ref 更新の成否が不明なら、再試行の前に remote ref/state と所有権を確認する。
claim の公開後は実際の remote commit SHA を使って `verify RUN_ID CLAIM_SHA` を
実行し、調査開始・各 publication・merge の直前にも再検証する。

有効期限は2時間。期限後は owner の処理も停止するが、別のタスクは自動で奪取しない。
再開時は元タスク/PR/branchを確認し、worker停止と保存を確認してから owned release
と新しい claim を追記する。残った調査 branch を意図的に再開し、main と統合して再検証する。
確認できなければ停止して報告する。ネットワーク失敗も「空いている」と扱わない。

PR 本文は [共通 PR テンプレート](../.github/pull_request_template.md) を使う。
見出しの順序を保ち、テーマ・変更ファイル・一次資料 URL・適用版・baseline/head SHA・
実際の検証結果・未確認事項を記入する。CI は未確認で開始し、対象 head の両OSの
結果を読んでから run / job URL と結果を追記する。該当なしには理由を添える。

merge前は lease、最新 main、現在の PR head、両OSのCI、未解決reviewを再確認する。
APIの expected_head_sha は base を固定しないため、merge後の main CI も確認する。
ブランチ保護や認証設定を変えてチェックを迂回しない。

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
このプロトコルは GPT 同士の排他であり、既存 OpenCode は使っていない。
OpenCode の更新は最新 main の再取得・統合・重複確認・再検証で扱う。

状態 helper は候補 JSON を出すだけで remote 書き込みをしない:

```sh
bash scripts/research-gpt-state.sh status
bash scripts/research-gpt-state.sh claim RUN_ID > /tmp/gpt-transition.json
# publication の後で実際の remote commit SHA を指定
bash scripts/research-gpt-state.sh verify RUN_ID CLAIM_SHA
# 終了時。元の worker の停止と作業保存を先に確認する
bash scripts/research-gpt-state.sh release RUN_ID > /tmp/gpt-transition.json
```

GitHub connector では JSON の state から blob → state.json だけの tree →
expected_parent を親に持つ commit を作る。create=true のときは branch を新規作成し、
それ以外は force=false で ref を更新する。競合は停止する。既存 branch の上書き、
force push、branch削除、認証情報のコピーは不要。CLI 認証がなくても同じ方式を使える。

既に Git の認証と author identity がある場合の publication は以下。各コマンドの
成功を確認する。Bash で実行し、失敗を無視して後続へ進めない。

```sh
set -euo pipefail
blob=$(jq -c .state /tmp/gpt-transition.json | git hash-object -w --stdin)
tree=$(printf '100644 blob %s\tstate.json\n' "$blob" | git mktree)
parent=$(jq -r .expected_parent /tmp/gpt-transition.json)
commit=$(printf 'research: update GPT run lease\n' | git commit-tree "$tree" -p "$parent")
git push origin "$commit:refs/heads/research/gpt-coordinator"
git ls-remote --heads origin refs/heads/research/gpt-coordinator
```

有効期限は2時間。期限後は owner の処理も停止するが、別のタスクは自動で奪取しない。
再開時は元タスク/PR/branchを確認し、worker停止と保存を確認してから owned release
と新しい claim を追記する。残った branch を意図的に再開し、main と統合して再検証する。
確認できなければ停止して報告する。ネットワーク失敗も「空いている」と扱わない。

merge前は lease、最新 main、現在の PR head、両OSのCI、未解決reviewを再確認する。
APIの expected_head_sha は base を固定しないため、merge後の main CI も確認する。
ブランチ保護や認証設定を変えてチェックを迂回しない。

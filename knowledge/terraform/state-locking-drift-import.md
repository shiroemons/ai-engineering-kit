---
{
  "id": "terraform-state-locking-drift-import",
  "title": "Terraform の state locking と並行実行、refresh-only による drift 検出、import ブロック",
  "kind": "knowledge",
  "technology": "terraform",
  "version": "Terraform Docs v1.16.x（State Locking / terraform plan command / Import resources overview / import block reference / local backend、2026-09-27 確認）+ release notes v1.5.0・v1.6.0",
  "tags": [
    "research-domain:infrastructure",
    "terraform",
    "state locking",
    "concurrent runs",
    "force unlock",
    "lock timeout",
    "drift detection",
    "refresh-only",
    "speculative plan",
    "detailed exitcode",
    "import block",
    "generate-config-out",
    "local backend"
  ],
  "sources": [
    {
      "id": "terraform-state-locking-docs",
      "url": "https://developer.hashicorp.com/terraform/language/state/locking",
      "type": "official_docs"
    },
    {
      "id": "terraform-plan-command-docs",
      "url": "https://developer.hashicorp.com/terraform/cli/commands/plan",
      "type": "official_docs"
    },
    {
      "id": "terraform-import-overview-docs",
      "url": "https://developer.hashicorp.com/terraform/language/import",
      "type": "official_docs"
    },
    {
      "id": "terraform-import-block-docs",
      "url": "https://developer.hashicorp.com/terraform/language/block/import",
      "type": "official_docs"
    },
    {
      "id": "terraform-local-backend-docs",
      "url": "https://developer.hashicorp.com/terraform/language/backend/local",
      "type": "official_docs"
    },
    {
      "id": "terraform-v1-5-0-release-notes",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.5.0",
      "type": "release_notes"
    },
    {
      "id": "terraform-v1-6-0-release-notes",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.6.0",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-10-27",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/infrastructure.json"]
}
---

# Terraform の state locking と並行実行、refresh-only による drift 検出、import ブロック

1人の人間や1つのジョブが state を触る前提をやめた瞬間に出てくる3つの問い（誰が state をロックしているか、Terraform の外で変わったものを見つけるにはどの計画を使うか、既存リソースを設定に取り込むにはどう書くか）を、HashiCorp 公式文書5ページと release notes 2件（いずれも 2026-09-27 に原文を確認）の**記載事実**としてまとめる。文書に書かれていない挙動は「未確認」として残し、設計判断は最後の節に分けて記す。

## 要点（公式文書に確認した事実）

### state locking と並行実行

- backend が対応している場合、Terraform は **state を書き込み得るすべての操作で state をロックする**。目的は、他人がロックを取得して state を破壊することを防ぐこと。[State Locking](https://developer.hashicorp.com/terraform/language/state/locking)
- ロックは自動的に行われ、**発生時は何もメッセージが出ない**。ロック取得に時間がかかりすぎる場合に限りステータスメッセージが出る。メッセージが出ていない場合でも、backend が対応していればロックは進行中である。[State Locking](https://developer.hashicorp.com/terraform/language/state/locking)
- **state locking に失敗すると Terraform は続行しない。**[State Locking](https://developer.hashicorp.com/terraform/language/state/locking)
- `-lock=false` でほとんどのコマンドのロックを無効化できるが、公式は推奨しない。`terraform plan` の項では「他の誰かが同じ workspace に対して同時にコマンドを実行しうる場合には危険」と明記されている。[State Locking](https://developer.hashicorp.com/terraform/language/state/locking) / [plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- **すべての backend がロックをサポートするわけではない。** 対応の有無は各 backend のドキュメントに記載される。[State Locking](https://developer.hashicorp.com/terraform/language/state/locking)
- `-lock-timeout=DURATION` は、`-lock=false` で無効化していない限り、一定時間ロックの取得をリトライしてからエラーを返す。duration の構文は数値と単位文字の連結で、例は `3s`（3秒）。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- `force-unlock` は自動解除が失敗したときに手動でロックを外すコマンド。**他人がロックを保持している状態で unlock すると multiple writers になり得る**ため、公式は「自分のロックが自動解除に失敗した場合にのみ使う」と指定している。コマンドは一意のロック ID を要求し、Terraform は解除失敗時にこの ID を出力する。ID はノンスとして働き、lock/unlock が正しいロックを対象にすることを保証する。[State Locking](https://developer.hashicorp.com/terraform/language/state/locking)
- local backend は state をローカルファイルシステムに保存し、**システム API によってその state をロック**し、操作をローカルで実行する。[local backend](https://developer.hashicorp.com/terraform/language/backend/local)
- local backend では `-state` / `-state-out` / `-backup` が残っているが、公式は「後方互換のためのレガシー機能。新規システムでは使わない」と明記し、代わりに root module 内で remote state backend を設定する方針を推奨する。`-state` を `-state-out` なしで使うと両方に同じファイル名が使われ、新しい state snapshot を作ると**入力ファイルを上書きする**。3引数は別 backend 型の構成では効果を持たない。[local backend](https://developer.hashicorp.com/terraform/language/backend/local)

### drift 検出: refresh-only mode と refresh オプション

- **Refresh-only mode** は「Terraform の外部で行われたリモートオブジェクトの変更に合わせて、Terraform state と root module output 値だけを更新すること」を目標とする plan を作る。意図して帯域外（out-of-band）で変更したリソース（例: incident 対応中の変更）と Terraform の記録を整合させたい場面で有用。`-refresh-only` で有効化する。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- `-refresh-only` は **Terraform v0.15.4 以降でしか使えない**。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- planning modes（既定の normal、`-destroy` の destroy、`-refresh-only` の refresh-only）は**すべて相互排他**。1つの非既定モードを有効化すると normal は無効化され、同時に2つ以上の代替モードは指定できない。これらのモードは `terraform plan` と `terraform apply` の両方で使える。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- `-refresh=false` は「構成変更をチェックする前に remote objects と state を同期する既定挙動」を無効化する。API リクエストを減らして高速になるが、**外部変更を無視するため不完全または不正確な plan になり得る**。refresh-only モードでは `refresh=false` は使えない（計画操作そのものが無効化されるため）。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- `-detailed-exitcode` は終了コードを細分化する: **0 = 空の diff 成功（変更なし）、1 = エラー、2 = 非空の diff 成功（変更あり）**。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- `-out=FILENAME` で保存した plan は後で `terraform apply` に渡せる**非投機的 plan**。`-out` を付けない plan は **speculative plan**（実際に適用する意図を伴わず、plan の効果だけを記述したもの）で、その後にターゲットシステムへ加わった変更により最終効果は変わり得るため、適用前に最終の非投機的 plan を再確認するよう公式は注意している。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- 保存された plan ファイルは標準的な公開形式ではないが、完全な構成、計画変更に関わるすべての値、input variables を含む plan オプションを含む。端末出力では伏せられた sensitive なデータでも**平文で保存**されるため、機微な成果物として扱う。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- `-target` は誤りからの復旧や Terraform の制限回避など**例外的な場面向け**で、日常運用には非推奨。検出されない configuration drift と混乱を招き得ると明記されている。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)
- local backend 向けの `terraform plan` はレガシーの `-state` 引数のみ追加で受け付ける。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)

### import ブロック

- 既存リソースの import には、インフラのユニークなリソース ID を指定する `import` ブロックと、state 上のインポート先アドレスの宣言が必要。さらに、**`import` ブロックが宣言したアドレスに一致する宛先の `resource` ブロックを作成する**必要がある。[Import resources overview](https://developer.hashicorp.com/terraform/language/import)
- 単一・小規模: ID を容易に取得できるなら `import` と `resource` ブロックを手書きして `terraform apply`。あるいは **`import` ブロックだけを書いて `terraform plan` に `generate-config-out` フラグ**を付け、`resource` ブロックを生成する。[Import resources overview](https://developer.hashicorp.com/terraform/language/import)
- 大規模（bulk）: 未管理リソースの集合を発見する **HCL のクエリ**を定義し、結果を構成に加え、`terraform apply` で import する。[Import resources overview](https://developer.hashicorp.com/terraform/language/import)
- リソース同一性: Terraform は **provider が割り当てた ID、または provider が定義する属性の集合**でリソースを一意に識別する。構成では `id` または `identity` 属性で参照する。例として AWS `s3_bucket` の identity は `account_id`・`bucket`・`region`、`aws_instance` は `id`。[Import resources overview](https://developer.hashicorp.com/terraform/language/import)
- `import` ブロックの引数（[import block reference](https://developer.hashicorp.com/terraform/language/block/import)）:
  - `to`（Address、必須）: インスタンスアドレス。**既存の `resource` ブロックのアドレスと一致すること**。
  - `id`（String）: provider の ID。文字列、または文字列に評価される式を指定する。**plan 操作中に既知でなければならない**。同じブロックで `identity` と併用できない。
  - `identity`（map、`id` と相互排他）: リソースを一意に識別する identity オブジェクト。キーと値はリソースタイプと provider 固有。
  - `for_each`（map または string の set、メタ引数）: 個別のブロックなしで同種のリソースを import する。
  - `provider`（reference、メタ引数）: 指定した provider 構成に従って import する（例: `provider = aws.east`）。
- 置き場所: どの構成ファイルに書いてもよいが、公式は `import` 専用の **`imports.tf` を作るか、各 `import` ブロックを宛先 `resource` ブロックの隣に置く**ことを推奨している。[import block reference](https://developer.hashicorp.com/terraform/language/block/import)

### 版による差（release notes）

- **v1.5.0（2023-06-12、タグ f25d764）**: `import` ブロックを導入。configuration-driven で plannable な操作となり、**通常の plan の一部として処理される**。`terraform plan` は import 予定のリソースの概要を他の変更と併せて表示する。既存の `terraform import` CLI コマンドは変更されていない。初期バージョンは **`id` フィールドに interpolation をサポートしない**（文字列でなければならない）。`terraform plan` に `-generate-config-out=PATH` を追加（import ブロックがあり対応する構成を持たないリソースの HCL を新規ファイル PATH に生成。**適用前に生成構成をレビューし、必要なら編集する**）。`check` ブロックも同時導入。[v1.5.0](https://github.com/hashicorp/terraform/releases/tag/v1.5.0)
  - 同リリースで、local operations の apply ステップ中に state snapshot を定期的に backend へ永続化するようになり、プロセスが異常終了した場合のデータ喪失ウィンドウが狭まった。SIGINT 受信時も直ちに最新 snapshot の保存を試みる（#32680）。[v1.5.0](https://github.com/hashicorp/terraform/releases/tag/v1.5.0)
- **v1.6.0（2023-10-04、タグ edb8ea4）**: `import` ブロックの `id` が**リソース属性など他の値を参照する式を受け付ける**ようになった（値は plan 時に既知の文字列である必要、#33618）。また `force-unlock` 実行時に潜在的なリモート Terraform バージョンの不一致を無視するようになった（#28853）。[v1.6.0](https://github.com/hashicorp/terraform/releases/tag/v1.6.0)
- 現行 v1.16.x の文書では `-generate-config-out` は **(Experimental)** と表記されている。[plan](https://developer.hashicorp.com/terraform/cli/commands/plan)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- **並行実行の制御は「ロックを待つ」より「state を書き込むジョブを1つに絞る」で行う。** ロックは state 破壊を防ぐ最後の防衛線であり、成功時は何も出力されないため、待機の成否はログの有無でしか確認できない。同一 state に対する apply をスケジューラ側で直列化する。
- **CI では `-lock-timeout` を短めに設定し、競合は明示的な失敗として扱う。** ロック取得のリトライは `-lock-timeout` が担い、公式は値の推奨を示していない（ワークロードごとに決める）。時間を長く伸ばすより、失敗として拾ってリトライをワークフロー側に置くほうが、二重適用の兆候を見落とさない。
- **drift 検出は通常 plan と refresh-only plan を別ジョブに分ける。** planning modes が相互排他のため、1回のコマンドでは「構成と実環境の差」と「state と実環境の差」を同時に問えない。refresh-only の差は「実環境を構成に合わせて直す」のではなく「state を帯域外変更に合わせる提案」としてレビューする。
- **自動化で変更有無を分岐するなら `-detailed-exitcode` を付け、0 / 1 / 2 を別々に扱う。** 2 のみを変更あり、1 はエラーとして処理する。変更ありを表す 2 はこのフラグを付けたときだけ返る。
- **適用する plan は `-out` で保存して非投機的なものにする。** 投機的 plan をレビュー根拠のまま apply に流し込まず、apply は保存した plan ファイルを渡して行う。plan ファイルは sensitive を平文で含み得るので、CI 成果物でも秘匿区分を付けて保持・破棄する。
- **`force-unlock` は「自分の実行で自動解除が失敗した場合」だけに限定し、そのとき出力されたロック ID を記録する。** ID はノンスなので、失敗時のロック ID と照合できる「誰がいつ unlock したか」の記録を取る。
- **import は `imports.tf` に集約し、`-generate-config-out` の出力を必ずレビューしてから apply する。** v1.5.0 のリリースノートが適用前レビューを指示している。生成物は宛先 `resource` と重複しないか、意図しない属性が埋まっていないかを確認する。v1.6.0 未満では `id` に式を書けない前提で値を固定する。
- **共有環境では local backend を使わず、remote state backend を root module 内で設定する。** 公式の推奨どおり。コマンドライン引数で state ファイルを差し替える既存ワークフローは、backend 設定へ移行するまでを期限付きの暫定扱いにする。

## 避ける使い方

- **他人と同一 workspace で `-lock=false` を使う。** 公式は非推奨、plan の項では「他者が同時にコマンドを実行しうるなら危険」と明記されている。
- **ロック非対応の backend で「ロックが効いている」と判断する。** すべての backend がサポートするわけではなく、対応の有無は各 backend のドキュメントにしかない。
- **メッセージが出ていないことを「ロックしていない」と読む。** 成功時は何も出力されず、取得に時間がかかる場合でなければメッセージすら出ない。出力の有無とロックの成否は別の話である。
- **誰のロックか確認せずに `force-unlock` を実行する。** 他人のロックを外すと multiple writers になり得る。
- **`-refresh=false` を drift 検出や通常の確認に使う。** 高速化の代償は外部変更の無視で、不完全・不正確な plan を招く。
- **refresh-only と別の planning mode を1回のコマンドに混ぜる。** 相互排他で指定できない。refresh-only で `-refresh=false` を使うこともできない。
- **`-detailed-exitcode` なしの終了コードで変更有無を判定する。** 変更ありを表す 2 はフラグを付けたときにのみ返る。
- **`-out` なしの投機的 plan をそのまま適用判断に使う。** その後の他者の変更で実効が変わる。保存 plan を「秘密情報を含まない」前提で扱うのも危険で、平文保存される。
- **v1.5.x で `import` の `id` に式を書く。** 式が使えるのは v1.6.0 以降（#33618）で、それ以前は文字列必須。
- **local backend の `-state` / `-state-out` / `-backup` を新しいシステムで使う。** 公式はレガシーと明記し非推奨。`-state` 単独指定は入力ファイルを上書きし得る。
- **`-target` を日常的な部分適用の手段にする。** 検出されない configuration drift を招くと公式が明記している。

## 適用版と本番での注意

- 適用版: Terraform Docs **v1.16.x**（State Locking / terraform plan command / Import resources overview / import block reference / local backend の5ページを 2026-09-27 に developer.hashicorp.com で全文確認。各ページの "Edit this page on GitHub" は `content/terraform/v1.16.x/docs/...` を指す）。release notes は **v1.5.0**（2023-06-12、タグコミット f25d764edfc89d0d7e42fb99be433558ae45d7a3）と **v1.6.0**（2023-10-04、タグコミット edb8ea4baa43421a73d7ba50fed957e6be599b45）。ドキュメントの版選択には v1.17.x が beta と表示されており、v1.16.x は現行 stable だが将来の最新と同一視しない。
- 再確認期限: 公式文書5件は `official_docs`（TTL 90日）で 2026-12-26、release notes 2件は TTL 30日で **2026-10-27**。terraform は `config/freshness.json` の technologies に含まれないため技術 TTL は適用されない。文書の明示期限も最短に合わせて 2026-10-27 とする。
- 未確認事項（推測で埋めない）: backend ごとのロック対応状況、HCP Terraform および各リモート backend でのロック・force-unlock の実挙動の差、`-lock-timeout` の既定値とタイムアウト時の出力形式、`terraform plan -refresh-only` が state を実際に書き戻す正確なタイミング（plan と apply の役割分担は今回の5ページに記載がない）、import 時に生成される HCL の属性の揺れや provider ごとの identity 取得方法、import block reference の Summary 表が `identity` の Data type を String と記載する一方 configuration model は map としている表記不整合。これらは該当ページの該当節か provider 側のドキュメントを別途確認する。
- 本ドキュメントの推奨方法は設計案であり、単一のベンチマークや障害事例の一般化ではない。`-lock-timeout` やリトライの値はワークロードごとに検証して決める。

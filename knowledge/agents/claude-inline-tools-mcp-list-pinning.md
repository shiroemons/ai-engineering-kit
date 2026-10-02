---
{
  "id": "agents-claude-inline-tools-mcp-list-pinning",
  "title": "Claude の会話途中の tool 変更: inline definition・MCP tool-list pinning と append-only 履歴",
  "kind": "knowledge",
  "technology": "agents",
  "version": "Claude API beta inline-tools-2026-09-15 / mcp-client-2026-09-15 (announced 2026-09-22); reference-only predecessor mid-conversation-tool-changes-2026-07-01; thinking-binding-controls-2026-08-01; verified 2026-10-02 UTC",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "claude-api",
    "inline-tools",
    "tool_addition",
    "tool_removal",
    "mcp_tool_listing",
    "tool-list-pinning",
    "append-only",
    "prompt-cache",
    "preserved-thinking",
    "beta"
  ],
  "sources": [
    {
      "id": "claude-inline-tools-release-20260922-verified-20261002",
      "url": "https://platform.claude.com/docs/en/release-notes/overview",
      "type": "release_notes"
    },
    {
      "id": "claude-mid-conversation-tools-20260915-verified-20261002",
      "url": "https://platform.claude.com/docs/en/build-with-claude/mid-conversation-system-messages",
      "type": "official_docs"
    },
    {
      "id": "claude-mcp-list-pinning-20260915-verified-20261002",
      "url": "https://platform.claude.com/docs/en/agents-and-tools/mcp-connector",
      "type": "official_docs"
    },
    {
      "id": "claude-preserved-thinking-binding-20261002",
      "url": "https://platform.claude.com/docs/en/build-with-claude/preserved-thinking",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/ai-engineering.json"]
}
---

# Claude の会話途中の tool 変更

## 問いと適用範囲

長い agent セッションの途中で tool を発見した、schema が変わった、MCP server が追加された。そのたびに最初の `tools` 配列を作り直してよいか。また、MCP の一覧が変わった時点を、保存・再開した会話で再現できるか。

本書は Claude Messages API の tool catalog 更新と応答の再送に絞る。[2026-09-22 の公式 release notes](https://platform.claude.com/docs/en/release-notes/overview) は、会話内の `tool_addition` に完全な定義を渡す beta と、MCP 一覧を応答に記録する beta を発表した。**日付入り header は機能の識別子であり、MCP 本体の protocol version ではない。**

cache の TTL・課金境界は[既存の prompt caching 文書](../llm/prompt-caching-breakpoints-ttl-scope.md)、履歴削減は[compaction 文書](long-loop-context-growth-compaction.md)、実行許可は[tool 失敗・権限ループ](tool-permissions-failure-loop.md)を参照する。ここでは cache hit の改善率や thinking の品質効果を測定したとは主張しない。

## 一次資料で確認できた契約

### 1. beta と参照方式を区別する

[Mid-conversation system messages and tool changes](https://platform.claude.com/docs/en/build-with-claude/mid-conversation-system-messages) の対応表・tool changes 節で確認した。

- `inline-tools-2026-09-15` は Claude API の beta。`tool_addition` の `tool` を `tool_definition` とし、`definition` に定義を置ける。旧 `mid-conversation-tool-changes-2026-07-01` は参照方式だけを扱い、完全な定義は扱わない。
- 同ページの対応モデルは Claude Opus 5.5・5・4.8、Sonnet 5.5、Fable 5.1・5、Mythos 5.1・5。Sonnet 5 は対象外。参照方式の旧 header は Amazon Bedrock / Google Cloud にも対応するが、inline 定義や新 MCP header の対応をそこまで広げて解釈しない。
- 既知の tool は最初の `tools` に置き、必要なら `defer_loading: true` と参照で公開を遅らせる。未知・変更後の定義は履歴へ追記する。同じ名前で schema を置換すると、その位置以降に反映される。異種 tool との同名衝突は `tool_name_conflict`。
- 初期 `tools` に non-deferred tool がゼロだと、最初の inline 定義で full cache miss が発生する。computer use など一部の tool 型は inline 未対応。
- 変更は `role: "system"` の content に置く。content を持つ system message は `messages` の先頭に置けず、通常は `user` turn（`tool_result` を含む turn も可）の直後、次の `assistant` turn の前または配列末尾に置く。連続する system messages は一まとまりとして判定される。一般の text は server tool result で終わる assistant turn の直後にも置けるが、tool 変更は paused assistant turn 直後に置けないため先に再開する。`tool_use` と `tool_result` の間も不可。

### 2. MCP の一覧を response とともに保存する

[MCP connector の Pin an MCP server's tool list 節](https://platform.claude.com/docs/en/agents-and-tools/mcp-connector#pin-an-mcp-servers-tool-list-beta) の契約は次のとおり。

- `mcp-client-2026-09-15` は Claude API の beta で、`mcp-client-2025-11-20` の機能を含む。旧 header を重ねる必要はない。
- server の一覧を取得した応答では、server ごとの `mcp_tool_listing` が content の先頭側に入る。`content[0]` を text と決めつけない。
- assistant message は listing を含めて変更せず再送し、その block を含む request では新 header を継続する。後続 request は記録済み一覧を利用し、再取得しない。
- 明示的な pinning では `MCPToolset.tools` に `name`・`description`・`input_schema` を渡す。この一覧に `default_config` / `configs` が適用される。
- 同 connector は公開 HTTP server の tool calls を対象とし、local STDIO へ直接接続しない。ZDR 対象外との表示もある。新 header の採用だけで接続・保持条件が変わるとは扱わない。

会話途中の MCP 追加には、新 MCP header と inline header を併用する。定義は `mcp_toolset`、URL と token は `mcp_servers` に分離されることを[inline ガイド](https://platform.claude.com/docs/en/build-with-claude/mid-conversation-system-messages#add-an-mcp-server-mid-conversation-beta)で確認した。

### 3. prefix の書き換えは cache だけの問題ではない

[Preserved thinking](https://platform.claude.com/docs/en/build-with-claude/preserved-thinking) は、対象モデルで `system`・`tools`・先行 `messages` の変更が後続 thinking block の prefix check に影響すると説明する。Claude Fable 5.1・Opus 5.5・Sonnet 5.5 の会話はこの境界を考慮する。Mythos 5.1 と Fable 5.1 より前のモデルはこの prefix check を実行しない。

チェックの既定強制は **2026-08-31 00:00 UTC 以降に作成された account** に適用される。それ以前の account では `thinking.block_binding.prefix_mismatch_behavior` を設定した request だけが強制対象になる。古い account で通ったことは新しい account での互換性の証明にならない。

`thinking-binding-controls-2026-08-01` は `input_transformations` と上記設定を提供する。`error` は不整合を 400 にし、`drop_block` は不整合 block と後続 thinking を落とす。後者でも prefix を編集した原因は残る。Message Batches API は設定省略時に強制対象の不整合 block を落とすため、失敗を検出したい評価では `error` を明示する。モデル非互換の `model_binding_mismatch` は prefix 不一致と別原因である。

**Sonnet 5.5 の適用条件:** `block_binding` は `thinking.type: "adaptive"` との組合せだけで利用できる。`between_tools` と併用すると 400 になる。この mode では append-only 履歴を維持するか、編集した turn 以降の thinking blocks を手動で除去する。後者は残っていた reasoning を失う回復策であり、通常運用での履歴編集を推奨するものではない。

## 本書の設計提案: catalog 更新を履歴イベントにする

以下は上記契約から導く設計案であり、Anthropic が保証する SDK 実装や唯一の推奨構成ではない。

1. **開始時の構成と、後から起きた変更を分けて保存する。** 初期 `tools`、利用モデル、送った beta header をセッションに結び付ける。後から見つかった定義は tool-change event として追加し、再起動時に「現在の plugin 一覧」から過去の request を再構築しない。
2. **表示用の応答と、再送用の応答を別に持つ。** UI は block type で表示対象を選ぶ一方、再送用には assistant message 全体を保存する。画面に出さない listing を捨てる変換を persistence 層へ流用しない。
3. **pinning と remote implementation の互換性を別管理する。** 固定されるのはモデルに見せる定義であり、remote server のコード・権限・可用性ではない。schema を固定していても server が古い引数を受け付けなくなる場合を、自分の integration の失敗経路として用意する。
4. **同名更新の前後を実行側でも区別する。** schema v1 で生成された未完了 call を、schema v2 の validator に無条件で通さない。更新時点、call ID、その時点の schema 識別子を結び付ける。この識別子はアプリ独自の監査用情報で、API の追加必須フィールドではない。
5. **提供された tool と、実行を許可した tool を分ける。** `tool_addition` や listing が届いた事実を、送信・削除・課金操作への承認と扱わない。実行直前の許可判定は別に維持する。
6. **限定的に導入する。** beta 非対応環境では過去の `tools` を黙って書き換える fallback を使わず、初期 tool 集合を固定するか、明示的に新しい会話を開始する。どちらを選ぶかは履歴継続性と利用環境の要件で決める。

### 未実施の境界テスト案

これは検証結果ではなく、採用する integration で実行すべきテスト案である。

- 初期に未知だった tool を追加し、保存・再起動後も追加位置と定義が同じになる
- 一覧 block が text より先に返る応答でも表示が壊れず、再送から listing が失われない
- MCP server の一覧を変更しても pinning 中の model-facing 定義は変化しない一方、remote 実行エラーは別に検出できる
- schema v1 の call が進行中に v2 を追加したとき、call と validator の対応が曖昧にならない
- non-deferred tool がある場合とゼロの場合を比較し、cache 条件の差を観測する
- thinking block のある履歴で top-level `tools` の編集を意図的に起こし、`error` と `drop_block`、古い account の観測と強制を区別する
- old header だけで inline 定義を送る、paused turn 直後に追加する、未対応 tool 型を inline 化する場合を正常系に混ぜない

## 避ける運用と未確認事項

- listing の pinning を server の版固定、権限固定、tool 実行成功の保証と呼ばない
- beta header の日付をリリース日や MCP protocol version として記録しない
- tool schema の追加を cache の有効化と混同しない。cache 設定・最小長・TTL の確認は別途必要
- `drop_block` で 400 が消えたことだけを移行完了条件にしない

今回の調査は API を呼ばず公式資料を直接読んだもの。SDK の最低対応版、全 tool 型の inline 対応表、モデル別の全 platform 組合せ、cache hit の実測、remote server 変更時の具体的な error、beta の将来互換性は未検証である。完全な definition サイズ・件数上限の列挙も本書の範囲外なので、本番の制限値は採用時に inline ガイドで再確認する。

## 出典・鮮度・ライセンス

4件とも Anthropic 公式一次資料を 2026-10-02 UTC に取得した。release notes の対象 entry は 2026-09-22 公開。他3件は更新される公式文書で、ページに独立した公開日・改訂日を確認できなかったため取得日で区別し、header の機能版を併記した。repository 分析ではなく、commit pin はない。

取得した各ページでは資料の再配布を許す open license を確認できないため、source catalog の license は `unknown` とする。リンク付きの独自要約だけを記録し、コード・図表・サンプルの転載や module への取り込みは行わない。release_notes の TTL 30日が最短なので、明示期限は 2026-11-01 UTC。beta を本番採用する際には期限内でも対応表を再確認する。

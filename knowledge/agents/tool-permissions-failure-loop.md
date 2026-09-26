---
{
  "id": "agents-tool-permissions-failure-loop",
  "title": "Agents の tool 実行ループ: 権限境界と失敗処理",
  "kind": "knowledge",
  "technology": "agents",
  "version": "Claude Handle tool calls (current) + Anthropic Building effective agents (2024-12-19) + OpenAI Function calling Responses API (examples use gpt-6-astra)",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "tool-use",
    "tool-result",
    "tool_use_id",
    "is_error",
    "tool_choice",
    "parallel_tool_calls",
    "permissions",
    "failure-handling",
    "stopping-conditions",
    "sandbox",
    "untrusted-content"
  ],
  "sources": [
    {
      "id": "claude-handle-tool-calls-docs",
      "url": "https://platform.claude.com/docs/en/agents-and-tools/tool-use/handle-tool-calls",
      "type": "official_docs"
    },
    {
      "id": "anthropic-building-effective-agents",
      "url": "https://www.anthropic.com/engineering/building-effective-agents",
      "type": "maintainer_article"
    },
    {
      "id": "openai-function-calling-guide",
      "url": "https://platform.openai.com/docs/guides/function-calling",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "maintainer",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# Agents の tool 実行ループ: 権限境界と失敗処理

tool を使うエージェントの実行権限をどこで絞り、失敗をどうループに戻すかを、3件の一次情報だけに絞って整理する。事実 (文書に書かれた動作) と設計上のまとめ (本書の推奨方法節) を区別する。

## 要点

### 文書化された事実: 実行ループの形

- [Claude Handle tool calls](https://platform.claude.com/docs/en/agents-and-tools/tool-use/handle-tool-calls) (same API version, current): `tool_result` は対応する `tool_use_id` と一致させる。`user` メッセージ内で `tool_result` ブロックを先頭に置く。実行エラーは `is_error: true` と説明的な content で信号する。無効・欠落パラメータにも `is_error` で返し、モデルは 2-3 回再試行する。`tool_result` の content は間接プロンプトインジェクションに備えて untrusted として扱う。
- [OpenAI Function calling guide](https://platform.openai.com/docs/guides/function-calling) (Responses API, examples use `gpt-6-astra`): 5段階ループは (1) tool 付きで要求、(2) tool call を受け取る、(3) アプリ側で実行、(4) `call_id` に対応する `function_call_output` を返す、(5) 最終回答のために再要求する。
- [Anthropic Building effective agents](https://www.anthropic.com/engineering/building-effective-agents) (published Dec 19, 2024): エージェントは環境フィードバックに基づいて tool を使う LLM であり、各段階で ground truth に照らして動作するループである。

### 文書化された事実: 権限境界の手段

- OpenAI ガイドの `tool_choice` モードは `auto` / `required` / forced-function (特定関数への強制) / `allowed_tools` / `none`。`parallel_tool_calls: false` は tool 呼び出しを zero-or-one に制限する。`strict: true` は schema 準拠を強制する。
- Anthropic 記事は、タスクに最大反復回数のような停止条件 (stopping conditions) が必要とし、サンドボックスでのテストとガードレールを併用する。toolset 設計には明確な文書化、使用例、境界 (boundaries) が必要で、絶対ファイルパスを要求するようなポカヨケ (poka-yoke) 強化を挙げる。
- Claude ドキュメントは `tool_result` を untrusted として扱い、間接プロンプトインジェクション対策とする。これは権限境界の一部であり、tool 出力を指示として昇格させないことを意味する。

### 文書化された事実: 失敗処理の契約

- Claude ドキュメント: 実行エラーは `is_error: true` + 指示的な content で返す。無効・欠落パラメータの場合も `is_error` で返し、モデルは 2-3 回再試行する。`tool_use_id` の不一致や `tool_result` 配置の誤りは、この契約を壊す。
- Anthropic 記事: 各段階の ground truth と停止条件 (例: 最大反復回数) が失敗の無限循環を止める。サンドボックスでのテストとガードレールは本番前の権限制御である。
- OpenAI ガイド: アプリ側実行の結果を `call_id` に対応付けて `function_call_output` で返す。再要求で最終回答を得る。`parallel_tool_calls` と `strict` は呼び出し数と形式の逸脱を抑える。

## 推奨方法

以下は上記 3件からの設計上のまとめであり、公式が定める実装構成そのものではない。

- ループは「要求 (tool 宣言付き) → tool call 受領 → アプリ側で実行 → ID 対応付けした結果返却 (`tool_use_id` / `call_id`) → 再要求」の順に固定する。ID の対応付けは Claude の `tool_use_id` と OpenAI の `call_id` のいずれでも省略しない。
- 権限は宣言側と実行側の両方で絞る。宣言側は `tool_choice` (`auto` を既定にし、必要時のみ `required`・forced-function・`allowed_tools`・`none`) と `parallel_tool_calls: false` (同時実行を許さない場合) と `strict: true` で絞る。実行側は公開する tool 自体を最小集合にし、引数に絶対ファイルパスのようなポカヨケ制約を入れる。
- 失敗は結果オブジェクトで返す。実行エラーと無効・欠落パラメータは `is_error: true` と「次に何を直せばよいか」の説明的な content で返し、モデル側の再試行 (Claude 文書では 2-3 回) に委ねる。例外でループを落とすのではなく、ID 対応付けを保ったまま回復可能な失敗として戻す。
- 終わらないタスクに備えて最大反復回数の停止条件を必ず付ける。サンドボックスでのテストとガードレールを本番適用の前提にする。
- `tool_result` / `function_call_output` の内容は untrusted として扱う。tool 出力中の指示文をそのまま実行せず、表示・引用と実行を分ける。

## 避ける使い方

- **`tool_result` と `tool_use_id` の対応付けを省く、または `tool_result` を `user` メッセージの先頭以外に置く。** Claude 文書の配置・対応付けの契約に反する。
- **実行エラーを `is_error` なしの通常結果として返す。** モデルが失敗を成功と誤認し、2-3 回の再試行の契機を失う。
- **無効・欠落パラメータを無言で補完する。** 文書化された動作は `is_error` で返して再試行させることであり、アプリ側の推測補完は境界を広げる。
- **`tool_choice` を常に `required` や特定関数強制のまま運用する。** 権限境界の選択肢 (`auto` / `allowed_tools` / `none` による絞り込み) を使わず、不要な tool 呼び出しを誘発する。
- **並列呼び出しの制限 (`parallel_tool_calls: false`) と schema 強制 (`strict: true`) を検討せずに並列実行を許す。** 呼び出し数と形式の逸脱を抑える手段を外すことになる。
- **停止条件なしで環境フィードバックのループを回す。** Anthropic 記事が求める最大反復回数などの停止条件がなく、失敗の無限循環になる。
- **サンドボックスでのテストとガードレールを省いて本番 tool を公開する。** 記事が前提とする権限制御を欠く。
- **tool 出力を信頼できる指示として扱う。** `tool_result` の content は untrusted であり、間接プロンプトインジェクションの経路になる。
- **境界の曖昧な toolset を文書化・使用例なしで公開する。** 記事が求める明確な文書化・使用例・境界・ポカヨケ (例: 絶対ファイルパス要求) がない状態である。

## 適用版と本番での注意

- Claude `Handle tool calls` は same API version の現行ページとして確認した。版ラベルなしのため、将来の版とは扱わない。
- Anthropic `Building effective agents` は 2024-12-19 公開の開発元記事 (`maintainer_article`) であり、API 契約ではなく設計指針として扱う。本文書の `trust: maintainer` は 3件中の最低信頼度に合わせたものである。
- OpenAI `Function calling / Tool calling guide` は Responses API のガイドで、例は `gpt-6-astra` を使用する。 `tool_choice` の表記や `parallel_tool_calls`・`strict` の既定値はガイドの改訂で変わり得るため、実装前に対応表を当該ガイドで再確認する。
- 本文は `expires_at` 2026-12-25 (取得 2026-09-26 + `official_docs` / `maintainer_article` TTL 90日)。技術 TTL (agents) の設定はないため、source type と明示期限で判定する。
- **未確認**: 各プロバイダーのタイムアウト範囲、キャンセル、リソース所有権、シャットダウン順序、版ごとの導入履歴。本書の inventory には含まれないため、推測で埋めず未確認とする。
- **未確認**: 2-3 回の再試行を超えた後のモデル動作や、最大反復回数の推奨値。本文書の根拠は「2-3 回再試行する」「最大反復回数のような停止条件が必要」という記述までであり、具体的な上限値は独自に定める必要がある。

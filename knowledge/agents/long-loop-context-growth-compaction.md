---
{
  "id": "agents-long-loop-context-growth-compaction",
  "title": "長時間 agent ループのコンテキスト増加: tool result の切り詰め、compaction の適用タイミングと失われる情報の再取得",
  "kind": "knowledge",
  "technology": "agents",
  "version": "Anthropic Effective context engineering (2025-09-29) + Claude context editing (beta context-management-2025-06-27, clear_tool_uses_20250919 / clear_thinking_20251015) + Compaction at a token threshold (beta compact-2026-01-12, strategy compact_20260112; on-demand beta compact-2026-09-04) + Claude Code model config / Agent SDK agent loop (accessed 2026-09-28) + OpenAI Agents SDK Tool Output Trimmer (openai-agents 0.22.3, released 2026-09-17)",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "context-management",
    "compaction",
    "context-rot",
    "tool-result",
    "tool-result-clearing",
    "trimming",
    "token-threshold",
    "compact-boundary",
    "just-in-time-retrieval",
    "note-taking",
    "sub-agent",
    "prompt-cache"
  ],
  "sources": [
    {
      "id": "anthropic-effective-context-engineering",
      "url": "https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents",
      "type": "maintainer_article"
    },
    {
      "id": "claude-context-editing-docs",
      "url": "https://platform.claude.com/docs/en/build-with-claude/context-editing",
      "type": "official_docs"
    },
    {
      "id": "claude-compaction-threshold-docs",
      "url": "https://platform.claude.com/docs/en/build-with-claude/compaction-threshold",
      "type": "official_docs"
    },
    {
      "id": "claude-code-model-config-autocompact",
      "url": "https://code.claude.com/docs/en/model-config.md",
      "type": "official_docs"
    },
    {
      "id": "claude-agent-sdk-agent-loop",
      "url": "https://code.claude.com/docs/en/agent-sdk/agent-loop.md",
      "type": "official_docs"
    },
    {
      "id": "openai-agents-tool-output-trimmer",
      "url": "https://openai.github.io/openai-agents-python/ref/extensions/tool_output_trimmer/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "maintainer",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# 長時間 agent ループのコンテキスト増加: tool result の切り詰め、compaction の適用タイミングと失われる情報の再取得

長時間の tool 実行ループでコンテキストが増えきるまでの管理を、6件の公式一次情報だけに絞って整理する。「文書化された事実」と本書の「推奨方法」を区別し、失われる情報の境界と再取得手段を明示する。

## 要点

### 文書化された事実: なぜコンテキストが増えるか

- [Effective context engineering for AI agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents) (Anthropic Engineering, 2025-09-29 公開) は、長時間のエージェントではコンテキストが増え続けると context rot が起きる前提で議論する。
- [How the agent loop works](https://code.claude.com/docs/en/agent-sdk/agent-loop.md) (Claude Agent SDK, 2026-09-28 取得) は、セッション中の context window はリセットされず、system prompt・tool 定義・会話履歴・tool 入出力が蓄積すると明記する。大きな tool output は 1 ターンで数千 token を消費する。

### 文書化された事実: tool result の抑制 (最も軽い切り詰めから)

- Anthropic の記事は tool result clearing を「最も軽い切り詰め」と位置づける (Claude Developer Platform の機能)。
- [Context editing](https://platform.claude.com/docs/en/build-with-claude/context-editing) (beta `context-management-2025-06-27`, 2026-09-28 取得): `clear_tool_uses_20250919` は入力トークンが設定 threshold を超えたときに会話履歴の**古い tool result を時系列で**自動クリアし、placeholder テキストで置換する。`clear_tool_inputs: true` を付けると tool call 側も消す。調整パラメータは `trigger` (`input_tokens`)、`keep` (`tool_uses`)、`clear_at_least` (`input_tokens`)、`exclude_tools`。
- 同ページ: 適用はサーバー側で prompt 到達前に実行され、クライアントは無編集の全履歴を保持する。tool result clearing は prompt cache prefix を無効化するため、`clear_at_least` で最低限の消去量を確保する記述がある。thinking には `clear_thinking_20251015` (`keep: thinking_turns|all`、デフォルトはモデルクラス依存) を使い、複数戦略を指定する場合は `clear_thinking_20251015` を先に指定する。ページ冒頭では「長時間の会話では server-side compaction が主要戦略」と明記する。
- [Tool Output Trimmer](https://openai.github.io/openai-agents-python/ref/extensions/tool_output_trimmer/) (OpenAI Agents SDK `openai-agents` 0.22.3, 2026-09-17 リリース, 2026-09-28 取得) は `CallModelInputFilter` を実装し、`RunConfig.call_model_input_filter` に渡して**各 model call の直前に**実行される。スライディング窓方式で、直近 `recent_turns` (既定 2) の user message とその後の全 item は一切変更されない。それより古いターンの tool output のうち `max_output_chars` (既定 500) 超、かつ指定時は `trimmable_tools` に属するものだけを preview (`preview_chars`, 既定 200) に置換する。namespaced tool は bare name と `namespace.name` の両方で指定でき、`trimmable_tools=None` なら全 tool 出力が対象。元の items は mutate せず shallow copy。structured output は Python/JSON 表記を除いた model-facing 文字列長で判定し、置換後も `max_output_chars` に収まる。

### 文書化された事実: compaction の適用タイミングと失われる境界

- [Compaction at a token threshold](https://platform.claude.com/docs/en/build-with-claude/compaction-threshold) (beta `compact-2026-01-12`, strategy `compact_20260112`, 2026-09-28 取得): `trigger` は `input_tokens` のみで、既定は `{"type":"input_tokens","value":150000}`、最小 50,000。閾値に達した通常リクエスト内で API が要約を生成し `compaction` ブロックを作って継続する。**以降のリクエストでは `compaction` ブロックより前の content block を API が自動削除する** (= 失われる境界)。
- 同ページ: `pause_after_compaction` (既定 false) を付けると `stop_reason: "compaction"` に停下し、直前の recent turns を再挿入できる。`instructions` は既定の要約プロンプトを完全置換する。Claude 5.1+ では custom instructions 時の要約入力は visible conversation のみで、過去の thinking は含まれない。複数回 compaction は可能で、最後の compaction block がそれ以前を置換する。対応モデルに claude-opus-5-5 / claude-sonnet-5 / claude-sonnet-4-6 / claude-fable-5-1 等が挙げられる。概要ページでは on-demand 版は beta `compact-2026-09-04` とトップレベル `compaction` パラメータで、決定主体は「あなた (on-demand) / API (threshold)」。
- [Model configuration](https://code.claude.com/docs/en/model-config.md) (Claude Code, 2026-09-28 取得): auto-compact window は「コンテキスト上限に達するまでに会話がどれだけ埋まってから compact するか」を指す。設定は `/autocompact <値|auto>` (`autoCompactWindow` として user settings 保存)、起動時 `--autocompact`、環境変数 `CLAUDE_CODE_AUTO_COMPACT_WINDOW` (優先順位 env > flag/setting)、`DISABLE_COMPACT` で全 compact を無効化。既定は「モデルのコンテキスト上限到達時に compact」で、例外は (a) cloud sessions は上限接近時、(b) Sonnet 4.6 / Opus 4.6 は extended context なしで 200K 境界 (`CLAUDE_CODE_DISABLE_1M_CONTEXT=1` 時も 200K)、(c) ネイティブ 1M ウィンドウのモデル (Sonnet 5、Fable 系、Anthropic API 上 Opus 4.7+) は既定約 967K tokens で compact、(d) 不明なモデル ID は Claude Code が想定するウィンドウで compact。`CLAUDE_CODE_MAX_CONTEXT_TOKENS` 等の境界条件も併記される。
- [How the agent loop works](https://code.claude.com/docs/en/agent-sdk/agent-loop.md): automatic compaction は「context window が上限に近づくと」古い履歴を要約し、要約後の recent exchanges と key decisions を残す。発火時は `type: "system"` / `subtype: "compact_boundary"` メッセージを emit する (TypeScript は `SDKCompactBoundaryMessage`)。compaction は古いメッセージを要約で置換するため**会話冒頭の指示は保証されず**、恒久ルールは `settingSources` 経由の CLAUDE.md に置くべきと明記する。カスタム手段として、CLAUDE.md の要約指示セクション (ヘッダー名は自由)、`PreCompact` フック (`trigger: manual|auto`、例として full transcript の archive)、`/compact` プロンプトによる手動実行が挙げられる。

### 文書化された事実: 失われる情報の再取得手段

- Anthropic の記事は、失われ得る情報のために (1) **just-in-time retrieval** — file path / stored query / web link 等の軽量識別子を残し、実行時に読み直す、(2) **structured note-taking** — NOTES.md や memory tool (public beta) に書き出す、(3) **sub-agent** — 詳細探索を隔離し親へは 1,000〜2,000 token の要約のみ返す、を挙げる。
- Claude Agent SDK の agent loop ドキュメントも、コンテキスト節減策として subagent (fresh context、最終応答のみ tool result として親に返る)、tool の最小化、MCP tool search による schema 遅延ロード、低 effort 指定を明記する。

## 推奨方法

以下は上記 6件から導く本書の設計上のまとめであり、公式が定める唯一の構成ではない。

- 抑制を段階的に重ねる。まず各 model call 直前の trim (Tool Output Trimmer) と閾値到達時の tool result clearing で毎ターンの増加を削り、それでも上限に近づくなら compaction で会話をまとめる。Anthropic の記事の分類では clearing が最も軽く、compaction が文脈を要約で置換する重い操作にあたる。
- compaction 後に要約だけが残ると考える。threshold 版では `compaction` より前の content block が次のリクエストから API に自動削除され、Agent SDK では会話冒頭の指示が保証されない。したがって恒久ルール (手順・制約・スタイル) は会話ではなく CLAUDE.md (`settingSources`) 側に置く。
- 要約プロンプト (`instructions`) をカスタマイズする場合、Anthropic の記事の指針どおり complex な trace で recall を最大化してから precision を改善する順で設計する。過度な compaction は後で重要になる文脈を失う。
- 失われ得る情報には再取得手段を先に用意する。file path・stored query・web link のような軽量識別子を要約に残して just-in-time retrieval で読み直し、確定した判断は NOTES.md / memory tool へ書き出し、長い探索は sub-agent に隔離して短い要約だけを親に戻す。
- tool result clearing を使うときは prompt cache prefix の無効化を計算に入れる。公式の `clear_at_least` による最低消去量の確保と同じ意図で、細かい間隔での小出しクリアを避ける。
- 適用タイミングは自分で決めるか API に委ねるかを区別する。Claude API の threshold 版は API が決定し (既定 150,000、最小 50,000)、on-demand 版は呼び出し側が決定する。Claude Code では `autoCompactWindow` / `CLAUDE_CODE_AUTO_COMPACT_WINDOW` で「どれだけ埋まってから compact するか」を調整でき、`DISABLE_COMPACT` で全 compact を切れる。
- compaction が発生した時点を観測可能にする。Agent SDK は `compact_boundary` を emit するので、これを記録すれば要約で情報が消えた時点を追跡できる。Claude API の threshold 版では `pause_after_compaction` で停下させ、recent turns の再挿入を組み合わせる。

## 避ける使い方

- **会話冒頭の指示やプロンプトで恒久ルールを維持しようとする。** compaction は古いメッセージを要約で置換するため、冒頭指示は保証されない。CLAUDE.md 等の外部保存に置く。
- **tool result clearing を prompt cache に無配慮な間隔で細かく発火させる。** clearing は cache prefix を無効化する。`clear_at_least` で最低消去量を確保する公式の趣旨に反する。
- **`instructions` で要約プロンプトを完全置換しつつ、過去の thinking まで要約入力に含まれると考える。** Claude 5.1+ の custom instructions 時、要約入力は visible conversation のみで thinking は含まれない。
- **直近 `recent_turns` の tool output まで切り詰める設計にする。** Tool Output Trimmer の契約は直近 `recent_turns` (既定 2) の user message 以降を一切変更しないとするもので、直近ターンは現在のタスクの文脈として保持される。抑制は `max_output_chars` と `trimmable_tools` で古いターンに限定する。
- **compaction 後も「会話に書けば恒久的に残る」と想定して大量の tool output を生み続ける。** 大きな tool output は 1 ターンで数千 token を消費し、compaction では要約による置換と `compaction` より前の block 削除が起きる。入力を小さくする (tool 最小化・MCP tool search による遅延ロード) 方向で抑える。
- **`trigger` / `keep` / `clear_at_least` の単位を取り違える。** `trigger` と `clear_at_least` は `input_tokens`、`keep` は `tool_uses` である。
- **不明なモデル ID で Claude Code の compact タイミングを固定値と信じる。** 不明なモデル ID は Claude Code が想定するウィンドウで compact され、既定値はモデルクラスごとに異なる (200K / 約 967K 等)。

## 適用版と本番での注意

- 本書の `trust: maintainer` は 6件のうち最も低い信頼区分 (Anthropic のエンジニアリング記事 `maintainer_article`) に合わせたもの。API 契約にあたる beta 機能は `official_docs` 4件と Claude Code / Agent SDK 公式ドキュメント2件で確認したが、beta header・strategy ID・既定値は改訂で変わり得るため、実装前に該当ページを再確認する。
- 適用版: context editing は beta `context-management-2025-06-27` (`clear_tool_uses_20250919` / `clear_thinking_20251015`)、threshold compaction は beta `compact-2026-01-12` (`compact_20260112`、on-demand は beta `compact-2026-09-04`)、Tool Output Trimmer は `openai-agents` 0.22.3 (2026-09-17 リリース)。Claude Code / Agent SDK ドキュメントは版ラベルなしの現行ページとして 2026-09-28 に確認した。
- 取得日は 2026-09-28、source TTL (`official_docs` 90日 / `maintainer_article` 90日) により明示期限は 2026-12-27。技術 TTL (agents) は config にないため、source type と明示期限で判定する。
- **未確認**: 要約 (compaction) が長いループ後のタスク成功率に与える実測影響。Anthropic の記事は「過度な compaction は後で重要になる文脈を失う」と定性的に述べるだけで、本書の inventory には数値 benchmark がないため推測で埋めない。
- **未確認**: `clear_tool_uses_20250919` の placeholder テキストの具体文言、閾値超過時に必ず 1 つの tool result 以上を残すか等の最低保持数の契約。inventory には `keep` (直近 `tool_uses` 数) の記述のみある。
- **未確認**: Tool Output Trimmer 以外の OpenAI Agents SDK における compaction / 自動要約機構。本 inventory は trimmer のみを確認した。
- **未確認**: memory tool (public beta) の API 形状と GA 有無。Anthropic 記事の記述は public beta と表明するものだけを引用した。

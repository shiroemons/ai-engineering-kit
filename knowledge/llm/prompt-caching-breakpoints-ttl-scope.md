---
{
  "id": "llm-prompt-caching-breakpoints-ttl-scope",
  "title": "LLM prompt caching: cache breakpoint 配置、TTL、cache-hit scope (Anthropic / OpenAI)",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Anthropic Prompt caching (api-version 2023-06-01; examples use claude-opus-5-5) + OpenAI Prompt caching (Responses API; GPT-5.6 and later vs earlier models)",
  "tags": [
    "research-domain:ai-engineering",
    "llm",
    "prompt-caching",
    "cache-breakpoint",
    "cache-hit",
    "TTL",
    "cache_creation_input_tokens",
    "cache_read_input_tokens",
    "prompt_cache_key",
    "prompt_cache_breakpoint",
    "prompt_cache_options",
    "prompt_cache_retention"
  ],
  "sources": [
    {
      "id": "anthropic-prompt-caching-docs",
      "url": "https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching",
      "type": "official_docs"
    },
    {
      "id": "openai-prompt-caching-guide",
      "url": "https://platform.openai.com/docs/guides/prompt-caching",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active"
}
---

# LLM prompt caching: cache breakpoint 配置、TTL、cache-hit scope (Anthropic / OpenAI)

prompt caching の再利用単位 (cache breakpoint)、有効期限 (TTL)、cache-hit が成立する範囲 (cache-hit scope) を、Anthropic と OpenAI の公式文書 2件だけに絞って整理する。文書に書かれた動作 (事実) と本書の推奨方法節 (設計上のまとめ) を区別する。

## 要点

### 文書化された事実: breakpoint は再利用可能な prefix の区切りである (両社共通の考え方)

- [Anthropic Prompt caching](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching): `cache_control` による明示 breakpoint と、top-level での automatic caching が区別されている。breakpoint はキャッシュ書き込み位置の指定であり、自動 caching は最上位レベルでの自動的な扱いである。
- [OpenAI Prompt caching](https://platform.openai.com/docs/guides/prompt-caching): cache breakpoint は再利用可能な KV prefix の終端を示す。cache-hit 判定は longest-prefix backward lookup で行われ、後方から最も長い一致 prefix を探す。
- 含意として、breakpoint より前の安定した prefix が長いほど再利用の機会が増える。breakpoint より前の1文字の差異でも、その後方の prefix は再利用できない。

### 文書化された事実: breakpoint の数と探索範囲に上限がある

- Anthropic: breakpoint は最大 4つであり、backward lookback は 20 block である。20 block より後方の位置にある breakpoint 指定は探索範囲外として扱われる。
- OpenAI (GPT-5.6 以降の明示 mode): `prompt_cache_options.mode` と `prompt_cache_breakpoint` で明示指定し、write は最大 4つ、minimum は 1024 token である。明示指定がない場合、最新の eligible message に implicit breakpoint が置かれる。
- 含意として、5つ以上の breakpoint 配置や、1024 token 未満の細切れ prefix への書き込みは、文書化された上限・下限の範囲外である。

### 文書化された事実: TTL は既定値・更新条件・計測起点が文書化されている

- Anthropic: 既定 TTL は 5分で、再利用されると更新される。TTL は request 開始時点から計測される。選択肢として 1h TTL があり、base input 料金の 2倍の価格設定である。
- OpenAI: GPT-5.6 以降は `prompt_cache_options.ttl` で指定し、既定は 30m である。それ以前のモデルでは `prompt_cache_retention` が `in_memory` / `24h` を取る。
- 含意として、再利用間隔が TTL を超えると cache-hit は成立しない。Anthropic の 5分既定は再利用で延長されるが、計測起点は request 開始時点である点に注意する。

### 文書化された事実: cache-hit scope は prefix 全体一致と分離境界で決まる

- Anthropic: cached prefix の順序は tools → system → messages である。この順序で prefix が構成され、一致判定の対象になる。
- Anthropic: cache は workspace / org 単位で分離される。異なる workspace / org 間で cache は共有されない。
- OpenAI: cache-hit scope は rendered-prefix 全体の一致を条件とする。組織や region を跨いだ共有はされず、routing は `prompt_cache_key` で補助される。
- 含意として、tools 定義の差し替えや system prompt の並べ替えは prefix 全体一致を崩し、cache-hit を無効化する。`prompt_cache_key` は一致条件を緩和するものではなく、routing の補助である。

### 文書化された事実: cache の利用状況は usage field で観測する

- Anthropic: usage には `cache_creation_input_tokens` (書き込み)、`cache_read_input_tokens` (読み取り命中)、`input_tokens` が含まれる。書き込みと読み取り命中を区別して観測できる。
- OpenAI: モデル世代で TTL と breakpoint の扱いが異なる (GPT-5.6 以降とそれ以前)。世代を混同した TTL・breakpoint 設定は文書化された動作と一致しない。
- 含意として、`cache_read_input_tokens` が増えなければ breakpoint 配置か prefix 安定性のどちらかが機能していない。世代別の差異は実装前に対応節で再確認する。

## 推奨方法

以下は上記 2件からの設計上のまとめであり、公式が定める実装構成そのものではない。

- 安定した内容を prefix の先頭に寄せ、breakpoint で区切る。Anthropic の tools → system → messages 順序を前提に、変わりにくい tools 定義と system prompt を先頭に置き、変動する user message を後方に置く。OpenAI では `prompt_cache_breakpoint` を安定部分の終端に置き、1024-token minimum を満たす単位で区切る。
- breakpoint は上限 (両社とも最大 4) の範囲で、再利用したい prefix 境界だけに置く。Anthropic では 20-block backward lookback の範囲内に収める。多数の細かい breakpoint は上限消費と minimum 未達の原因になるため避ける。
- 再利用間隔から TTL を選ぶ。Anthropic の 5分既定で再利用間隔を満たせるかを確認し、満たせない場合のみ 1h TTL を cost (base input の 2倍) と組で判断する。OpenAI の GPT-5.6 以降は `prompt_cache_options.ttl` (既定 30m) を再利用間隔に合わせ、旧モデルでは `prompt_cache_retention` (`in_memory` / `24h`) のどちらが適用世代かを先に確認する。
- 同一 routing が必要な traffic には `prompt_cache_key` を付ける。組織・region を跨ぐ共有の手段としてではなく、同一 scope 内での routing 補助として使う。
- `cache_creation_input_tokens` と `cache_read_input_tokens` で効果を測る。書き込みだけが増えて読み取り命中が増えない場合は、prefix 全体一致の崩れ (tools・system の差異) か TTL 切れを疑い、breakpoint 配置と再利用間隔のどちらを直すかを切り分ける。

## 避ける使い方

- **変動部分を prefix 先頭に置いたまま breakpoint を後方に置く。** 先頭の差異で後方 prefix 全体の cache-hit が崩れるため、安定部分を先頭に寄せる順序に反する。
- **上限を超える breakpoint 配置や minimum 未満の細切れ書き込みを期待する。** Anthropic の最大 4・20-block backward lookback、OpenAI 明示 mode の最大 4 write・1024-token minimum の範囲外であり、文書化された動作ではない。
- **TTL 切れの間隔で再利用を期待する。** Anthropic の 5分既定 (再利用で更新、request 開始時点から計測)、OpenAI の `prompt_cache_options.ttl` 既定 30m・旧モデルの `prompt_cache_retention` を超える間隔では cache-hit は成立しない。
- **異なる workspace / org、異なる組織・region 間での cache 共有を前提にする。** 両社とも分離が文書化されており、共有は成立しない。`prompt_cache_key` は一致条件の緩和ではなく routing 補助である。
- **tools 定義や system prompt の差異を無視して cache-hit を期待する。** Anthropic の tools → system → messages 順序、OpenAI の rendered-prefix 全体一致の条件に反し、prefix の差異は命中を無効化する。
- **世代別の差異を混同する。** OpenAI の GPT-5.6 以降 (`prompt_cache_options.mode`・`prompt_cache_breakpoint`・`prompt_cache_options.ttl`) とそれ以前 (`prompt_cache_retention` `in_memory` / `24h`) を取り違えた設定は、文書化された動作と一致しない。
- **usage を見ずに caching の効果を主張する。** `cache_creation_input_tokens` (書き込み cost) だけが発生し `cache_read_input_tokens` (命中) が伴わない構成は、cost だけが残る。

## 適用版と本番での注意

- Anthropic `Prompt caching` は api-version 2023-06-01、例示に claude-opus-5-5 を用いる版として確認した。モデル別の minimum cacheable length の具体値は本書の inventory に含まれないため、実装前に対応節で再確認する。
- OpenAI `Prompt caching` は Responses API の現行ページとして確認し、GPT-5.6 以降とそれ以前で扱いが異なる。`prompt_cache_options.mode`・`prompt_cache_breakpoint`・`prompt_cache_options.ttl`・`prompt_cache_retention` の parameter 詳細や版ごとの導入履歴は本書の inventory に含まれないため、実装前に対応節で再確認する。
- 1h TTL の「base input 料金の 2倍」は文書化された価格倍率であり、絶対金額や他料金要素との組み合わせは本書の inventory に含まれない。cost 判断は最新の料金表で再確認する。
- 本文は `expires_at` 2026-12-26 (取得 2026-09-27 + `official_docs` TTL 90日)。技術 TTL (llm) の設定はないため、source type と明示期限で判定する。本文書の `trust: official` は 2件とも `official_docs` であることに対応する。
- **未確認**: モデル別の minimum cacheable length の具体値、Anthropic automatic caching の詳細条件、OpenAI の eligible message の厳密な定義と routing の内部動作、`prompt_cache_key` の衝突時の振る舞い。本書の inventory には含まれないため、推測で埋めず未確認とする。
- **未確認**: タイムアウト範囲、キャンセル、リソース所有権、シャットダウン順序。本書の inventory には含まれないため、推測で埋めず未確認とする。

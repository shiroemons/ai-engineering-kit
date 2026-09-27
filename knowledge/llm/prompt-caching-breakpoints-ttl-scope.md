---
{
  "id": "llm-prompt-caching-breakpoints-ttl-scope",
  "title": "LLM prompt caching: breakpoint 配置、TTL、cache-hit scope (Anthropic / OpenAI)",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Anthropic Prompt caching (accessed 2026-09-27; Claude API and platform-specific behavior) + OpenAI Prompt caching (GPT-5.6 and later vs earlier models)",
  "tags": [
    "research-domain:ai-engineering",
    "llm",
    "prompt-caching",
    "cache-breakpoint",
    "cache-hit",
    "ttl",
    "cache_control",
    "prompt_cache_breakpoint",
    "prompt_cache_options",
    "prompt_cache_retention",
    "prompt_cache_key"
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

# LLM prompt caching: breakpoint 配置、TTL、cache-hit scope (Anthropic / OpenAI)

prompt caching の再利用単位、期限、一致範囲を、Anthropic と OpenAI の公式文書に基づいて整理する。文書に書かれた動作を「事実」、そこから導く運用上の提案を「推奨方法」として区別する。

## 要点

### Anthropic: prefix の順序、breakpoint、lookback

- [Prompt caching](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching) は top-level の `cache_control` による automatic caching と、block-level の明示 breakpoint を提供する。cached prefix の順序は `tools` → `system` → `messages` で、cache-hit には breakpoint までの内容が一致する必要がある。
- 明示 breakpoint は最大 4 件。automatic caching と明示 breakpoint を併用すると automatic breakpoint もこの 4 slots のうち 1 件を使う。明示 breakpoint が 4 件ある状態で automatic caching を追加すると API は 400 を返す。
- 各 breakpoint の read lookup は最大 20 block positions を後方へ調べる。ただし、lookup は以前に書き込まれた cache entry を探すもので、途中の未書き込み位置に entry を新規作成しない。連続した `tool_use` blocks と `tool_result` blocks はそれぞれ 1 position と数える。

### Anthropic: TTL、minimum length、分離範囲

- TTL は既定 5 分で、cache が再利用されると更新される。寿命は write/read request の開始時点から計測され、応答生成時間も含む。1 時間 TTL は選択可能で、cache write の価格は base input token 価格の 2 倍。
- cache 対象になる最小 prompt 長はモデルごとに異なる。下限より短い prompt は `cache_control` を指定しても cache されないことがあり、API error にならない。利用モデルに対応する現行 minimum を公式文書で確認する。
- cache は organization をまたいで共有されない。workspace 分離は Claude API、Claude Platform on AWS、Microsoft Foundry で適用され、Bedrock と Google Cloud は organization 単位の分離と記載されている。
- response usage の `cache_creation_input_tokens` は書き込み、`cache_read_input_tokens` は読み取り、`input_tokens` は最後の breakpoint より後の token 数を示す。合計 input token 数はこれら 3 項目の合計。

### OpenAI: model 世代別の breakpoint と一致範囲

- [Prompt caching](https://platform.openai.com/docs/guides/prompt-caching) は rendered prefix 全体が一致する場合に cache を再利用する。内容または関連設定が prefix 内で変わると、その変更位置以降の既存 entry と一致しない。
- GPT-5.6 以降は `prompt_cache_options.mode` で implicit / explicit-only を選べ、`prompt_cache_breakpoint` で明示境界を指定できる。implicit mode は最新 eligible message などの境界を使う。prefix の最小長は 1,024 visible input tokens。
- GPT-5.6 より前のモデルは breakpoint と retention の扱いが異なる。GPT-5.5 / GPT-5.5 Pro は implicit breakpoint が 2,048-token 間隔で、他の旧モデルは model-dependent interval を使う。明示 breakpoint は GPT-5.6 以降の機能として記載されている。
- GPT-5.6 以降の `prompt_cache_options.ttl` は `30m` が既定かつ唯一の値で、直近の write または reuse から少なくとも 30 分の再利用可能期間を指定する。より長く保持される場合もある。旧モデルの `prompt_cache_retention` では、`in_memory` は通常 5〜10 分の非使用後まで、最大 1 時間、`24h` は通常約 30 分から最大 24 時間の保持として説明されている。既定値は組織の data retention policy に依存する。
- cache は organization 間および regional processing boundary をまたいで共有されない。GPT-5.6 より前の `prompt_cache_key` は関連 request の routing を助けるが、machine 固定や hit を保証しない。GPT-5.6 以降は routing が自動化され、key は customer / user 別の cache accounting に使える。
- GPT-5.6 以降の Responses API usage では `input_tokens_details.cached_tokens` と `input_tokens_details.cache_write_tokens` で read/write token 数を観測できる。旧モデルでは報告の丸め方などが異なる。

## 推奨方法

以下は上記の公式仕様から導く運用上の提案であり、公式が指定する唯一の prompt 構成ではない。

- 再利用する静的内容を prefix の先頭に集め、変わる request 内容を breakpoint の後ろに置く。Anthropic は `tools` → `system` → `messages` の順序を前提にし、OpenAI は rendered prefix 全体と breakpoint の位置を確認する。
- Anthropic では最大 4 slots を必要な再利用境界に割り当て、20-block lookback から外れる長い会話には、再利用対象位置へ事前に breakpoint を置く。OpenAI では model 世代に合った implicit / explicit 設定を使う。
- TTL は実際の request 間隔と組織の retention policy に合わせる。OpenAI の `30m` は最低限の再利用可能期間であり、30 分を超えた cache-hit は保証されない。Anthropic の 1h TTL は追加 write cost と比較して選ぶ。
- OpenAI の `prompt_cache_key` はモデル世代に合わせて使う。旧モデルでは再利用する request 間で安定させて routing を助け、GPT-5.6 以降では分けたい usage / accounting の範囲に対応させる。
- Anthropic の `cache_creation_input_tokens` / `cache_read_input_tokens` と OpenAI の `cached_tokens` / `cache_write_tokens` を追い、read が増えない場合は prefix の変化、breakpoint、model minimum、TTL と routing を切り分ける。

## 避ける使い方

- **変動内容を含む位置に breakpoint を置く。** その位置までの prefix が一致しないと既存 cache entry に当たらない。Anthropic の後方 lookup は安定しているが未書き込みの prefix へ戻って cache を作る機能ではない。
- **Anthropic の上限を超えて automatic と明示 breakpoint を設定する。** automatic breakpoint も 4 slots の一つを使い、4 件の明示 breakpoint に automatic を追加すると 400 error になる。
- **OpenAI の GPT-5.6 以降に旧モデルの breakpoint / retention 設定をそのまま適用する。** explicit breakpoint、`prompt_cache_options.ttl`、`prompt_cache_retention` の対応世代を確認する。
- **TTL や key だけで hit を保証できると考える。** prefix の一致、model の minimum length、組織・region の分離、routing と保持状態が hit に影響する。OpenAI の `prompt_cache_key` も hit を保証しない。
- **複数組織や地域をまたぐ cache 共有を前提にする。** Anthropic と OpenAI の分離範囲を超えて再利用することはできない。

## 適用版と本番での注意

- Anthropic の公式ページを 2026-09-27 に確認した。対象モデルと platform によって minimum length、cache isolation、usage fields の扱いが異なるため、実装対象の API / platform の節を確認する。
- OpenAI の公式ガイドを 2026-09-27 に確認した。GPT-5.6 以降と旧モデルで breakpoint、minimum length、usage、TTL、`prompt_cache_key` の役割が異なる。モデルと組織の data retention policy を確認する。
- 取得日は 2026-09-27、`official_docs` の freshness TTL は 90 日で、明示 `expires_at` は 2026-12-26。料金、対象モデル、キャッシュ保持は更新され得るため、実装時には一次資料を再確認する。
- 実際の cache-hit 率やコスト削減量は workload の prefix 再利用率と request 間隔に依存し、この文書だけでは定量化しない。

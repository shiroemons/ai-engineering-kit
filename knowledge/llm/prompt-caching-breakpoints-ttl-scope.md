---
{
  "id": "llm-prompt-caching-breakpoints-ttl-scope",
  "title": "LLM の prompt caching: breakpoint・TTL・cache-hit 範囲 (Anthropic / OpenAI)",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Anthropic Prompt caching (current; Claude Opus 5.5 / Sonnet 5 / Haiku 4.5) + OpenAI Prompt caching (GPT-5.6 and later vs earlier models)",
  "tags": [
    "research-domain:ai-engineering",
    "llm",
    "prompt-caching",
    "prompt-caching-breakpoint",
    "cache_control",
    "prompt_cache_breakpoint",
    "prompt_cache_key",
    "ttl",
    "cache-hit",
    "prefix-match",
    "anthropic",
    "openai"
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

# LLM の prompt caching: breakpoint・TTL・cache-hit 範囲 (Anthropic / OpenAI)

prompt caching の cache-hit を決める 3 要素 (breakpoint の置き方、TTL、hit の一致範囲) を、Anthropic と OpenAI の公式文書 2 件だけに絞って整理する。文書に書かれた動作 (事実) と本書の推奨方法節 (設計上のまとめ) を区別する。

## 要点

### 文書化された事実: Anthropic の breakpoint と prefix 順序

- [Anthropic Prompt caching](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching) (current; 対象モデルに Claude Opus 5.5 / Sonnet 5 / Haiku 4.5 を含む): 明示の `cache_control` breakpoint は最大 4 件であり、これに加えて自動の top-level `cache_control` がある。
- prefix の評価順序は tools → system → messages であり、cache-hit には exact-prefix match が必要である。後方探索は 20-block backward lookup の範囲で行われる。
- 含意として、breakpoint より後の 1 文字の差異でもそれ以降の cache-hit は崩れる。安定した prefix を前に、変動部分を後に置く順序が hit 率を左右する。

### 文書化された事実: Anthropic の TTL と分離範囲

- TTL の既定は 5m であり、任意で 1h (`ttl` 指定) を選べる。lifetime は書き込み/読み取り要求の開始時点から測られ、再利用 (reuse) で refresh される。
- モデル別の minimum cacheable lengths (最小 cache 可能長) があり、下限未満の prefix は cache 対象にならない。
- cache は workspace / organization 単位で分離 (isolation) される。分離境界をまたいだ cache-hit は起きない。

### 文書化された事実: OpenAI の breakpoint (GPT-5.6 境界)

- [OpenAI Prompt caching](https://platform.openai.com/docs/guides/prompt-caching) (GPT-5.6 and later vs earlier models): GPT-5.6 以降は implicit breakpoint が latest eligible message に置かれる。明示指定は `prompt_cache_breakpoint` で行い、`prompt_cache_options.mode` に `explicit` / `implicit` を取る。書き込み (writes) は最大 4 件である。
- 最小条件は 1024 visible tokens であり、これを満たす可視 token がなければ cache は作られない。
- 旧モデル (earlier models) は implicit interval breakpoints に従い、`prompt_cache_retention` で `in_memory` (約 5-10m、最大 1h) と `24h` (約 30m、最大 24h) を選べる。

### 文書化された事実: OpenAI の TTL と一致・適用範囲

- GPT-5.6 以降の TTL は 30m であり、既定値かつ唯一の値である。再利用 (reuse) により refresh される。
- cache-hit には exact rendered-prefix match が必要である。レンダリング後の prefix が完全に一致しないと hit しない。
- `prompt_cache_key` は routing / accounting の scope であり、cache の振り分けと課金集計の単位になる。

## 推奨方法

以下は上記 2 件からの設計上のまとめであり、公式が定める実装構成そのものではない。

- prefix を「固定→変動」の順に並べる。system prompt・tool 定義・few-shot 例のような不変部分を先頭に集め、user 発話・検索結果・時刻のような変動部分を末尾に寄せる。Anthropic の tools → system → messages 順序と両社の exact-prefix match を前提にすると、先頭の安定が hit 率に直結する。
- breakpoint 予算 (両社とも最大 4 writes) を層の境界に割り当てる。例: (1) tool 定義の終わり、(2) system prompt の終わり、(3) 長文 context (RAG 抜粋など) の終わり、(4) 直近の会話履歴の切れ目。OpenAI の GPT-5.6 以降では implicit (latest eligible message) を既定にし、固定したい層だけ `prompt_cache_breakpoint` + `mode: explicit` で上書きする。
- TTL は再利用間隔から逆算する。Anthropic は既定 5m のまま短周期の連続呼び出しに使い、1h を選ぶのは再利用間隔が 5m を恒常的に超える場合に限る。OpenAI の GPT-5.6 以降は 30m 固定のため、30m を超える間隔の再利用は hit を期待しない。旧モデルの `24h` は約 30m の最低滞留と最大 24h の幅を理解した上で選ぶ。
- 最小長を確認してから breakpoint を置く。Anthropic はモデル別の minimum cacheable lengths、OpenAI は 1024 visible tokens の下限を満たさない prefix に breakpoint を置いても cache は作られない。短い system prompt だけを cache しようとせず、hit 対象の層が必要な長さを持つ構成にする。
- `prompt_cache_key` は用途別に分ける。OpenAI の routing / accounting の scope であるため、環境 (本番/検証) やテナントで key を混ぜると振り分けと課金集計の区別がつかなくなる。Anthropic 側の workspace / organization isolation と合わせて、分離したい単位と key・所属組織の対応を事前に決める。

## 避ける使い方

- **変動部分を prefix の先頭に置く。** 両社とも exact-prefix match が条件であり、Anthropic は breakpoint 以降の差異で hit が崩れる。時刻・リクエスト ID・毎回変わる検索結果を先頭に入れると cache が実質無効になる。
- **breakpoint の上限 (最大 4 件) を超えて細かく区切る。** Anthropic の明示 `cache_control` も OpenAI の writes も最大 4 件であり、5 件目以降は期待どおりに cache されない。
- **TTL の既定値を変えずに長周期の再利用を期待する。** Anthropic 既定 5m・OpenAI (GPT-5.6 以降) 30m・旧モデル `in_memory` 約 5-10m を超える間隔では、reuse による refresh が起きず hit しない。間隔が長い用途では 1h や `24h` の選択を検討する。
- **下限未満の短い prefix に breakpoint を置く。** OpenAI の 1024 visible tokens や Anthropic のモデル別 minimum cacheable lengths を満たさない層は cache 対象外であり、breakpoint だけ置いても効果がない。
- **render 前の一致で hit を期待する (OpenAI)。** 条件は exact rendered-prefix match であり、template 変数の解決後・tool 定義の展開後の文字列が一致する必要がある。render 前の論理的な同値では hit しない。
- **`prompt_cache_key` を混在させて routing / accounting を読む。** key は振り分けと課金集計の単位であり、混ぜた key の集計値から用途別の hit 率や cost を分離できない。
- **分離境界をまたいだ hit を期待する (Anthropic)。** cache は workspace / organization 単位で isolation されるため、別組織の同一 prompt でも hit しない。

## 適用版と本番での注意

- Anthropic `Prompt caching` は current 版として確認した。対象モデルとして Claude Opus 5.5 / Sonnet 5 / Haiku 4.5 が含まれる。20-block backward lookup の範囲、TTL の lifetime 起点 (要求開始時点)、reuse 時の refresh は改訂で変わり得るため、実装前に対応節で再確認する。
- OpenAI `Prompt caching` は GPT-5.6 を境界とする版として確認した。GPT-5.6 以降 (implicit at latest eligible message、`prompt_cache_breakpoint`、`prompt_cache_options.mode`、`30m` 固定 TTL) と旧モデル (implicit interval breakpoints、`prompt_cache_retention` の `in_memory` / `24h`) で契約が異なるため、利用モデルの側で読む。`24h` の約 30m〜最大 24h、`in_memory` の約 5-10m〜最大 1h は目安の幅であり、保証 latency ではない。
- 本文書の `trust: official` は 2 件とも `official_docs` であることに対応する。本文は `expires_at` 2026-12-26 (取得 2026-09-27 + `official_docs` TTL 90日)。技術 TTL (llm) の設定はないため、source type と明示期限で判定する。
- **未確認**: cache hit 時の料金・latency の定量効果、breakpoint 配置と token 節約の実測値、Anthropic のモデル別 minimum cacheable lengths の具体値、OpenAI の `visible tokens` の計数方法、両社のタイムアウト範囲・キャンセル・リソース所有権・シャットダウン順序。本書の inventory には含まれないため、推測で埋めず未確認とする。
- **未確認**: `prompt_cache_key` の具体的な指定方法 (parameter 名・形式・制約) や `prompt_cache_options.mode` の既定値。本書の inventory は scope (routing / accounting) と mode の選択肢 (explicit / implicit) までであり、実装前に対応ガイドで再確認する。

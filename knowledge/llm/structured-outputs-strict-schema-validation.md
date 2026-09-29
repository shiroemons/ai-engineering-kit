---
{
  "id": "llm-structured-outputs-strict-schema-validation",
  "title": "LLM structured outputs: strict-mode schema contract and validation failure handling (OpenAI)",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Structured Outputs guide + Function calling strict mode (examples gpt-6-astra / GPT-5.6; floor gpt-4o-mini, gpt-4o-mini-2024-07-18, gpt-4o-2024-08-06+) + openai-python helpers.md on main; verified 2026-09-29",
  "tags": [
    "research-domain:ai-engineering",
    "llm",
    "structured-outputs",
    "strict-mode",
    "strict",
    "json-schema",
    "text.format",
    "additionalProperties",
    "required",
    "validation",
    "refusal",
    "parse",
    "function-calling",
    "Pydantic"
  ],
  "sources": [
    {
      "id": "openai-structured-outputs-guide",
      "url": "https://platform.openai.com/docs/guides/structured-outputs",
      "type": "official_docs"
    },
    {
      "id": "openai-function-calling-guide",
      "url": "https://platform.openai.com/docs/guides/function-calling",
      "type": "official_docs"
    },
    {
      "id": "openai-python-helpers-structured-outputs",
      "url": "https://github.com/openai/openai-python/blob/main/helpers.md",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-29",
  "expires_at": "2026-12-28",
  "trust": "official",
  "status": "active"
}
---

# LLM structured outputs: strict-mode schema contract and validation failure handling (OpenAI)

strict-mode の schema 契約と、schema 不一致・refusal・finish 理由別の失敗時の扱いを、OpenAI の公式ガイド2件と openai-python の parsing helper 文書で確認できた範囲に限定して整理する。文書に書かれた動作を「確認できた動作」、そこから導く運用上の提案を「推奨方法」として区別する。

## 要点

### text.format の契約 (Responses API, Structured Outputs guide)

- Structured outputs は `text.format` に `{type: "json_schema", name, strict: true, schema}` を指定する形で使う。
- ガイド内の全 example が `additionalProperties: false` と `required` を含む。schema 作成時はこの2点を満たす。
- Structured Outputs と JSON mode の対照表では、schema に準拠するのは Structured Outputs のみ。JSON mode に schema 準拠を期待しない。
- ガイドが挙げる利点は explicit refusals と type-safety である。

### strict mode の schema 規則 (Function calling guide, Strict mode 節)

- `strict: true` の定義は、各 object に `additionalProperties: false` が必須で、全 property を `required` に列挙する。optional の値は `type: ["string", "null"]` のように null との union で表す。
- この規則に合わない `strict: true` リクエストは拒否される。strict と矛盾する schema は送信前に直す。
- Responses API は可能な場合 strict に自動正規化し、できない場合は `strict: false` に fallback する。Chat Completions は既定で non-strict である。
- `parse()` は strict な function tools を要求する。non-strict の tools とは組み合わせない。

### SDK parse helper の成功・失敗の扱い (openai-python helpers.md, main)

- `chat.completions.parse` / `responses.parse` を Pydantic モデルと組み合わせて使う。
- 成功と拒否は分けて返す。Chat Completions は `message.parsed` 対 `message.refusal`、Responses は `output_parsed` 対 `output_text` + refusal events である。
- 無効な JSON や schema に合わない text は validation error になり、commentary への fallback はない。
- `length` / `content_filter` の finish では `LengthFinishReasonError` / `ContentFilterFinishReasonError` が上がる。

## 推奨方法

以下は上記の確認動作から導く運用上の提案であり、公式が指定する唯一の構成ではない。

- schema は `strict: true` + オブジェクトごとの `additionalProperties: false` + 全 property の `required` 列挙で固定し、optional は `["T", "null"]` の union で表す。request 拒否にならない形を先に満たす。
- 応答の受け取りは parse helper (Pydantic) に寄せ、`parsed` 系と `refusal` 系で分岐する。`refusal` は正常出力として後段に流さず、再試行や縮退の経路に入れる。
- validation error (無効 JSON / schema 不一致) は catch して再送・修復の経路に入れる。commentary が返って修復できる前提にしない。
- `length` と `content_filter` は専用 error 型で区別する。length は出力上限や入力長の見直し、content_filter はフィルタ方針の確認に振り分ける。
- Responses API 利用時は、strict への自動正規化が効かない場合に `strict: false` fallback になることを前提にする。strict 適用を必須とする schema では、正規化の可否を左右する schema 側の条件 (strict 規則への適合) を満たしておく。

## 避ける使い方

- **JSON mode で schema 準拠を期待する。** 準拠するのは Structured Outputs のみである。
- **strict:true と矛盾する schema を送る。** `additionalProperties: false` の欠落や `required` の列挙漏れがあるとリクエスト自体が拒否される。
- **optional を required からの除外だけで表す。** 除外ではなく `type: ["T", "null"]` の union で表す。
- **parse() に non-strict の tools を渡す。** parse() は strict な function tools を要求する。
- **schema 不一致時に commentary が返ることを期待する。** 無効 JSON / schema 不一致は validation error になるだけで、修復文は返らない。
- **refusal を parsed として扱う。** refusal は `message.refusal` / refusal events の別経路で返るため、parsed 側だけを読むと見落とす。

## 適用版と本番での注意

- Structured Outputs guide の example は gpt-6-astra を使用し、対応下限は gpt-4o-mini / gpt-4o-mini-2024-07-18 / gpt-4o-2024-08-06+ と記載されている。Function calling guide の strict mode 節は Responses + Chat Completions を対象とし、gpt-6-astra / GPT-5.6 に触れている。利用モデルの対応状況は実装時に対象ガイドで再確認する。
- openai-python の helpers.md は main ブランチの文書として確認したもので、commit pin はしていない。SDK の振る舞いをコード実装として引用する場合は、利用版のタグや commit を固定して再確認する。
- 取得日は 2026-09-29、`official_docs` の freshness TTL は 90 日で、明示 `expires_at` は 2026-12-28。モデルやガイドの更新で契約が変わり得るため、実装時には一次資料を再確認する。
- 未確認事項 (この文書の範囲外であり記載しない): timeout scope、cancellation、完全な JSON Schema subset の対応表、refusal 発生率の定量化。これらは今回の3件では確認していない。

---
{
  "id": "mcp-tool-schema-json-value-ref-validation",
  "title": "MCP tool schema の移行: 任意 JSON 出力・object 入力・外部 $ref 検証の境界",
  "kind": "knowledge",
  "technology": "mcp",
  "version": "MCP 2026-07-28 vs 2025-11-25; SEP-2106 Final (created 2026-01-06); specification schema pinned at 3098fe94caa1b9e0afaaa6d30e040b61d5802471 (commit 2026-10-01 UTC); verified 2026-10-02 UTC",
  "tags": [
    "research-domain:ai-engineering",
    "mcp",
    "json-schema",
    "2020-12",
    "inputSchema",
    "outputSchema",
    "structuredContent",
    "$ref",
    "schema-validation",
    "compatibility",
    "SEP-2106"
  ],
  "sources": [
    {
      "id": "mcp-tool-schema-tools-2026-07-28-20261002",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/server/tools",
      "type": "official_docs"
    },
    {
      "id": "mcp-tool-schema-basic-2026-07-28-20261002",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic",
      "type": "official_docs"
    },
    {
      "id": "mcp-tool-schema-tools-2025-11-25-20261002",
      "url": "https://modelcontextprotocol.io/specification/2025-11-25/server/tools",
      "type": "official_docs"
    },
    {
      "id": "mcp-tool-schema-changelog-2026-07-28-20261002",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/changelog",
      "type": "official_docs"
    },
    {
      "id": "mcp-tool-schema-sep-2106-20261002",
      "url": "https://modelcontextprotocol.io/seps/2106-json-schema-2020-12",
      "type": "official_docs"
    },
    {
      "id": "mcp-tool-schema-repository-3098fe9-20261002",
      "url": "https://github.com/modelcontextprotocol/modelcontextprotocol/tree/3098fe94caa1b9e0afaaa6d30e040b61d5802471",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# MCP tool schema を広げるときの互換性と検証境界

## 問いと結論

2026-07-28 の MCP server が配列・数値を返す tool や複合条件の inputSchema を公開するとき、旧 client と同じ型・validator をそのまま使えるか。**出力を常に object として読む実装と、schema を単純な properties の列として扱う adapter は見直しが必要**。ただし入力引数の最上位は引き続き object であり、schema の表現力拡張は validator の外部通信を許可するものでもない。

対象は server-produced な tool 結果と client/server 間の契約である。[LLM structured outputs](../llm/structured-outputs-strict-schema-validation.md) のモデル生成制約や provider の strict mode と混同しない。transport、認可、Tasks、tool-list 履歴の詳細は既存文書に委ねる。以下の「設計提案」「検証候補」は独自の運用判断で、MCP が追加で義務付けたものではない。

## 適用版と証拠の優先順序

- [2026-07-28 changelog][change] の Minor changes 10 が、schema keyword の拡張・任意 JSON 出力・参照解決と検証資源の制約を SEP-2106 に対応する変更として記録している。単に調査日の「最新仕様」として扱わない。
- [SEP-2106][sep] は Created 2026-01-06、Final の履歴資料である。ページ自身が現行の規範には仕様を参照するよう注意している。この Created は仕様公開日でも SDK release 日でもない。
- メッセージ型は [固定 schema.ts][schema-new] と [生成 schema.json][json-new] を照合した。公式 Base Protocol は TypeScript schema を正本とする。固定した repository commit は `3098fe94caa1b9e0afaaa6d30e040b61d5802471`、commit 時刻は 2026-10-01 12:55:32 UTC。これも 2026-07-28 revision の release 日を示す値ではない。
- 取得日は 2026-10-02 UTC。参照した仕様ページに独立した記事公開日は表示されていない。個別 SDK の対応版や配布日を確認した主張はしない。

## 何が広がり、何が残るか

| 対象 | 2025-11-25 の確認範囲 | 2026-07-28 の契約 |
|---|---|---|
| inputSchema | TypeScript 型は `$schema`、`type`、`properties`、`required` を列挙 | 最上位 `type: "object"` を維持し、追加の JSON Schema keyword を許す |
| tools/call の arguments | キーと値を持つ object | 引き続き object。配列引数を最上位へ直接送れる変更ではない |
| outputSchema | 最上位 `type: "object"` を要求 | 値としての出力を object に限定しない。array、primitive、composition を表現できる |
| structuredContent | JSON object | object、array、string、number、boolean、null の JSON 値 |

根拠: [旧 Tools][old-tools]、[新 Tools][tools]、固定 commit の [旧 schema.ts][schema-old] と [新 schema.ts][schema-new]。

**schema を表す値と、その schema が検証するデータを分ける**。2026-07-28 の `outputSchema` フィールド自体は object 型で、生成 JSON Schema も `type: "object"` を指定する。出力データに boolean を認めることから、`outputSchema: false` を受理する、と推論してはいけない。`inputSchema` も、最上位 `$ref` のみで `type: "object"` を省く形にはできない。[固定型・生成 schema][json-new]

旧版の型に keyword が列挙されていることと、全旧 validator が追加 keyword を拒否することも同一ではない。今回確認した旧生成 schema の inputSchema に `additionalProperties: false` はない。したがって「旧 client は oneOf を必ず拒否する」とは断定できず、利用 SDK と変換処理の試験が必要である。[旧生成 schema][json-old]

## 検証を四つの段階に分ける

1. **tool 定義の受理**: schema フィールドの object 形状、inputSchema の最上位型、schema 自身の妥当性を検査する。
2. **dialect の選択**: `$schema` がなければ JSON Schema 2020-12。client/server はこれをサポートし、明示または既定 dialect で schema を検証する。未対応 dialect は対応していないことを示す error にし、別 dialect として黙って解釈しない。[Base Protocol][basic]
3. **tool 入力の検証**: server は全 tool 入力を検証する MUST を持つ。モデルが schema に沿うはず、あるいは client が既に検査したはず、という期待だけで省略しない。[Tools Security Considerations][tools]
4. **結果の検証**: outputSchema がある場合、server はそれに準拠する structured result を返す MUST、client は照合する SHOULD を持つ。型を `unknown` から cast しただけでは準拠を確かめたことにならない。[Tools Output Schema][tools]

設計提案: validator の「schema 不正」「dialect 未対応」「外部参照未解決」「時間・深さの予算超過」と、実際の値の「schema 不一致」を別の理由として保存する。validator を完走できなかった結果を、値が妥当だった結果へまとめない。予算値や error の UI 表示はアプリケーションで決め、MCP 共通の数値閾値があるかのように扱わない。

## $ref は通信命令ではない

[Base Protocol の $ref Resolution][basic] は network URI に解決される `$ref` の自動 dereference を **MUST NOT** とする。非ローカル参照を取得する opt-in 機能は MAY だが、既定で無効にすることは MUST。ホスト allowlist、または少なくとも loopback / link-local / private address の拒否、timeout・取得サイズ上限・参照 URI の記録は SHOULD の対策である。未解決 external `$ref` のため検証に失敗した schema は、何でも許可する schema として扱わず拒否する SHOULD がある。

設計提案: 通常は同梱した `$defs` と同一文書の参照で完結させる。`$id` 等を踏まえた解決先を確認するため、文字列が `https://` で始まるかだけの判定ではなく、採用 validator の URI 解決機構と fetch hook を確認する。参照が新たな外部データ取得になる構成では、tool server への接続許可をそのまま任意ホストへの許可に使わない。

設計提案: opt-in を実装する場合は redirect 後の宛先や名前解決後のアドレスも接続前に検査し、許可済み接続先の条件から外れたら停止する。これは本書の具体化であり、今回の MCP 節が redirect/DNS の全手順まで規定しているという主張ではない。認証情報を外部 schema URL に引き継がず、URI ログに query の秘密が混ざる運用も避ける。

## ネットワークを止めても検証コストは残る

`anyOf`、`oneOf`、`allOf`、`if/then/else`、`$defs` は表現力と計算量の両方に関係する。仕様は schema 深さ、subschema 総数、1回の validation time budget など合理的な上限を設ける SHOULD を記す。ネットワーク fetch を無効にしただけで CPU DoS 対策が完了するわけではない。[Base Protocol Composition-Keyword Resource Use][basic]

設計提案: schema のコンパイル時と実データ検証時を別々に計測する。許可するサイズ・深さ・分岐数を定め、validator が中断可能か確認する。予算超過をモデルの「引数修正で治る誤り」と一律に扱って無限再試行しない。上限の妥当性は実際の schema 分布と応答時間から決める必要があり、この調査では推奨ミリ秒値を導出していない。

## 型・互換 adapter・text fallback の実務判断

- **結果は JSON 値として受け取る**: 固定型の `structuredContent?: unknown` は object の property を無条件に読めないことを表す。null / array / object を区別して narrowing し、さらに outputSchema と業務上の条件を検証する。TypeScript の unknown は wire 上で関数や undefined を返してよいという意味ではない。[固定 schema][schema-new]
- **null と欠落を区別する**: 設計提案として field の存在を判定し、truthiness によって `false`・`0`・空文字・`null` を「結果なし」へ潰さない。payload を object に変換してキーを付け替える場合は、schema も含めて明示的な変換契約にする。
- **古い client へ新しい配列を一律配信しない**: SEP の互換性説明は、旧 server の object 出力を新 client が受ける方向と、新 server の非 object 出力を旧 client が受ける方向を区別する。後者で型検査が失敗する可能性がある。provider の schema subset への変換も別の検証対象である。[SEP compatibility][sep]
- **TextContent は互換性の保証書ではない**: 版固定 Tools は、structured content の JSON serialization を TextContent にも返すことを SHOULD とする。履歴 SEP の compatibility 節では旧 client 向けに MUST と記すが、本書は現行 Tools の規範強度を採用する。設計上は併記しても、旧 parser が結果全体を先に拒否するなら fallback へ到達しない。実際の client の処理順を確認する。[新 Tools][tools] / [SEP][sep]
- **provider adapter で制約を削らない**: 設計提案として、MCP schema と利用モデル/provider の受理する schema は別に保持する。composition を送れない場合に無言で削除するより、検証可能な adapter を用意するか、その tool の利用を明示的に制限する。入力生成用 schema を縮小しても server 側の元の入力検証を残す。

## 採用前の検証候補

以下は今回実行したテストではなく、この契約を実装するときの境界テスト案である。

| 条件 | 確かめたいこと |
|---|---|
| structuredContent が array、0、false、空文字、null | object 前提の parse と truthiness による欠落判定をしない |
| structuredContent が欠落、または outputSchema と不一致 | 存在・妥当性・成功状態を独立して分類する |
| inputSchema が object + oneOf、または object の指定なし | composition の意味を保ち、最上位入力型の制約も検査する |
| outputSchema 自体が boolean | 出力データの boolean 許可と取り違えずフィールド型を検査する |
| 未解決 external $ref | 自動 HTTP fetch が発生せず、検証を許可扱いにしない |
| 深い同梱 schema、大量の分岐 | ネットワークなしでも処理予算で終了できる |
| 旧 client が新 server の array と text を受ける | text fallback 到達前の envelope 拒否を観測する |
| 明示した未対応 dialect | 2020-12 へ黙って読み替えず原因を返す |

検索 eval はこの文書を取り出せるかの検査であり、上の runtime 挙動や validator の安全性の検証ではない。

## Provenance・制約・未確認事項

[固定 LICENSE][license] は MIT から Apache-2.0 への移行中で、新規 code/specification contributions は Apache-2.0、再許諾同意のない過去の contributions は MIT に残ると記す。CC-BY-4.0 は仕様を除く documentation の区分であり、この仕様全体を一律 CC-BY-4.0 や単一 Apache-2.0 として処理しない。ここでは原文のコード・example をコピーせず、日本語の独自要約と設計判断を記した。モジュールへのコード昇格はしていない。

公開仕様・changelog・SEP を native web で開き、固定 SHA の schema URL は取得 timeout だったため、公式 repository の認証不要 clone で同 SHA の TypeScript・生成 JSON・文書を照合した。固定 SHA の LICENSE は web でも取得できた。SDK や MCP server/client は実行しておらず、具体的な SDK の対応版、実 client の fallback、remote fetch の抑止、検証時間上限、provider 間の schema 変換は未検証である。

[tools]: https://modelcontextprotocol.io/specification/2026-07-28/server/tools
[basic]: https://modelcontextprotocol.io/specification/2026-07-28/basic
[old-tools]: https://modelcontextprotocol.io/specification/2025-11-25/server/tools
[change]: https://modelcontextprotocol.io/specification/2026-07-28/changelog
[sep]: https://modelcontextprotocol.io/seps/2106-json-schema-2020-12
[schema-new]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/3098fe94caa1b9e0afaaa6d30e040b61d5802471/schema/2026-07-28/schema.ts
[json-new]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/3098fe94caa1b9e0afaaa6d30e040b61d5802471/schema/2026-07-28/schema.json
[schema-old]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/3098fe94caa1b9e0afaaa6d30e040b61d5802471/schema/2025-11-25/schema.ts
[json-old]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/3098fe94caa1b9e0afaaa6d30e040b61d5802471/schema/2025-11-25/schema.json
[license]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/3098fe94caa1b9e0afaaa6d30e040b61d5802471/LICENSE

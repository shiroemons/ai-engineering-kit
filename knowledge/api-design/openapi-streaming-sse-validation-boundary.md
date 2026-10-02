---
{
  "id": "api-design-openapi-streaming-sse-validation-boundary",
  "title": "OpenAPI 3.2.1 の streaming 契約: itemSchema・SSE data・内包 JSON 検証の境界",
  "kind": "knowledge",
  "technology": "api-design",
  "version": "OAS 3.2.1 (2026-09-10), streaming/SSE rules compared with 3.2.0 (2025-09-19); JSON Schema Draft 2020-12 validation-01; WHATWG SSE checked 2026-10-02 UTC",
  "tags": [
    "research-domain:api-distributed",
    "openapi",
    "streaming",
    "sse",
    "itemSchema",
    "contentSchema",
    "validation",
    "migration"
  ],
  "sources": [
    {
      "id": "openapi-321-streaming-contract-20261002",
      "url": "https://spec.openapis.org/oas/v3.2.1.html",
      "type": "official_docs"
    },
    {
      "id": "openapi-320-streaming-baseline-20261002",
      "url": "https://spec.openapis.org/oas/v3.2.0.html",
      "type": "official_docs"
    },
    {
      "id": "openapi-321-release-20261002",
      "url": "https://github.com/OAI/OpenAPI-Specification/releases/tag/3.2.1",
      "type": "release_notes"
    },
    {
      "id": "json-schema-202012-content-vocabulary-20261002",
      "url": "https://json-schema.org/draft/2020-12/json-schema-validation",
      "type": "official_docs"
    },
    {
      "id": "whatwg-sse-parsing-contract-20261002",
      "url": "https://html.spec.whatwg.org/multipage/server-sent-events.html",
      "type": "official_docs"
    },
    {
      "id": "openapi-schema-iterations-20261002",
      "url": "https://spec.openapis.org/oas/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# OpenAPI 3.2.1 の streaming 契約: itemSchema・SSE data・内包 JSON 検証の境界

## 問いと結論

SSE や JSONL の API を OpenAPI 3.2 で記述すれば、受信ごとの検証、JSON payload の型検証、生成 SDK の逐次処理まで保証されるか。

保証の境界を分ける。`itemSchema` は逐次データの各 item を独立に記述する。SSE の `data` は解析後も文字列であり、文字列内の JSON は別の検証対象である。仕様書の構造検証、受信 item の検証、内包 JSON の検証、業務上の受理は別々に確認する。以下の実装手順・例・テスト案は一次資料に基づく独自提案で、製品の対応保証ではない。

既存の [Webhook の受信・重複排除](webhook-redelivery-signature-dedup.md) は配送後の業務処理を扱う。本稿は HTTP ストリームを API description からどう検証するかに絞り、再配送保証や認可を追加で保証しない。

## 適用版と今回確認した差分

- [OpenAPI 3.2.0](https://spec.openapis.org/oas/v3.2.0.html) の公開日は 2025-09-19、[3.2.1](https://spec.openapis.org/oas/v3.2.1.html) は 2026-09-10。2026-10-02 UTC に両方の公式 HTML を開いた
- 両版の `Complete vs Streaming Content` と `Special Considerations for Server-Sent Events` を HTML tag 除去・空白正規化後に比較し、対象節の本文は一致した。**itemSchema と SSE の規則は 3.2.1 の新機能ではない**
- [3.2.1 release notes](https://github.com/OAI/OpenAPI-Specification/releases/tag/3.2.1) は仕様文の訂正・明確化を説明している。実際に変化した隣接節の一例は multipart の positional encoding。3.2.0 の「itemSchema または array schema が必要」という文から、3.2.1 は data instance に対応する item の存在、対応しない encoding の無視、form-data を配列で表す場合の名前対応へ説明を整理している。本稿ではこの差分を streaming の仕様追加とは扱わない
- JSON Schema の参照版は Draft 2020-12 Validation、掲載文書は `draft-bhutton-json-schema-validation-01`（2022-06-16）。RFC と呼ばない。WHATWG SSE は 2026-10-02 取得時点の Living Standard で、ページ表示の更新日も 2026-10-02。特定ブラウザー版の検証結果ではない

## 1. item と stream 全体を取り違えない

[OAS 3.2.0 §4.14.3](https://spec.openapis.org/oas/v3.2.0.html#complete-vs-streaming-content) から確認した契約は次のとおりで、3.2.1 の同節でも同じである。

- `schema` は content 全体に適用する。sequential media type を JSON Schema の data model へ対応させるときは、順序を保った配列として扱う
- `itemSchema` は各 item に独立して適用する。Media Type Object 直下のフィールドであり、JSON Schema の `items` と配置も意味も同一ではない
- `schema` と `itemSchema` の併用は許される。片方がもう片方を無効化する規則ではない

設計上の選択は「受信単位をいつ受理するか」から決める。長時間接続の各イベントを届き次第処理するなら `itemSchema` を中心にする。有限の履歴を全部取得して件数や全体構造を検証したいなら配列の `schema` を使う。配列としてモデル化しても、JSONL の wire format が角括弧付き JSON array に変わるわけではない。

特に、`itemSchema` の下に `maxItems: 100` を書いて「接続全体で最大100イベント」と解釈してはいけない。各 item 自体が配列ならその配列長の制約になり、object item には件数制約にならない。接続の累積件数、最大継続時間、最大保留件数、重複 ID、イベント順序は別途状態を持つ業務契約として定義する。これらを記述しただけで SDK がメモリ上限や backpressure を実装すると期待しない。

## 2. SSE は wire の行ではなく解析済みイベントを検証する

[OAS 3.2.1 §4.14.4](https://spec.openapis.org/oas/v3.2.1.html#special-considerations-for-server-sent-events) は WHATWG の SSE parsing を先に行うことを要求する。`data`・`event`・`id` は文字列、`retry` は整数という対応を用いる。`data: {"count":3}` を送っても、JSON document を内包する**文字列**である点は変わらない。

[WHATWG の解析規則](https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation) を踏まえ、受信 fixture は最低限次を区別する。

- 複数の `data:` 行は LF で結合される。各行を独立 JSON として検証しない
- `:` で始まる comment と未知 field は無視する。heartbeat comment を業務 item に数えない
- 空行がイベントの区切り。EOF までに最後の空行が来なかった未完イベントは dispatch されない
- `retry` は ASCII digits だけの値を整数として扱い、それ以外は無視する。単に schema が integer を要求するからといって、wire 上の `retry: later` をアプリの整数変換例外にしてはならない
- UTF-8 の途中や CRLF の途中で受信 chunk が分割されるケースも用意する。network chunk、行、イベントの境界を一致させない

この検証段階は OpenAPI のイベント data model に対応する。ブラウザーの `MessageEvent` は `type`・`data`・`lastEventId` などを持つため、wire の `event`・`id`・`retry` をそのまま公開する object と同一視せず、利用ライブラリとの adapter を確認する。

## 3. contentSchema は自動的な拒否条件ではない

[JSON Schema Validation §8.1–8.5](https://json-schema.org/draft/2020-12/json-schema-validation#section-8) では `contentMediaType`・`contentSchema` は annotation である。内包する文書が壊れていても、それだけを理由に外側 instance の schema 検証を invalid としない。安全性と性能のため、既定で自動 decode・parse・validate を行ってはならない。opt-in の機能を提供する場合も、内包文書の結果を外側とは別に返す。

したがって `contentSchema` を付けたのに不正 JSON が外側の validator を通過すること自体は仕様違反とは限らない。`contentSchema` は文字列に作用し、`contentMediaType` がない場合は無視が推奨される。

独自の受理手順を明示するなら、次の順序が扱いやすい。

1. SSE parser で解析し、itemSchema に対応するイベント object を組み立てる
2. 外側の `type`・`required`・`const` などを検証する
3. API profile で許可した event 種別と media type に限り、`data` の JSON parse と内包 schema 検証を明示的に実行する
4. 外側結果・JSON parse 結果・内包 schema 結果を別々に記録した上で、アプリの受理条件を満たす場合だけ副作用を実行する

第三段階で失敗したイベントを業務上拒否することは可能だが、それはアプリが追加した受理条件である。annotation を汎用 validator の assertion と偽って説明しない。内包文字列のサイズ、nesting depth、解析時間、未知 event の扱いも profile に含め、受信データが指定する任意 parser や外部参照を無制限に実行しない。

## 4. 最小の記述例と二段階検証

以下は本稿の独自例であり、実製品の endpoint ではない。`stock.changed` の `data` を JSON として別途検証する profile を想定する。`maxLength` は外側文字列の文字数制約であり、transport の byte 上限の代わりにはしない。

```yaml
openapi: 3.2.1
info:
  title: Stock event example
  version: '1.0'
paths:
  /stock-events:
    get:
      responses:
        '200':
          description: A sequence of stock.changed events
          content:
            text/event-stream:
              itemSchema:
                type: object
                required: [event, data]
                properties:
                  event:
                    type: string
                    const: stock.changed
                  data:
                    type: string
                    maxLength: 4096
                    contentMediaType: application/json
                    contentSchema:
                      type: object
                      required: [sku, count]
                      additionalProperties: false
                      properties:
                        sku:
                          type: string
                          minLength: 1
                        count:
                          type: integer
                          minimum: 0
```

この例の期待結果は、外側 validator が content vocabulary の自動検証を無効にしている前提で次のようになる。これは全 OpenAPI tool の実測値ではなく、受理 profile のテスト設計である。

| 解析後の data / 条件 | 外側 item | 内包 JSON | アプリの扱い |
|---|---|---|---|
| `'{"sku":"A1","count":3}'` | valid | valid | 受理候補 |
| `'{"sku":"A1","count":-1}'` | valid | minimum 違反 | 副作用前に拒否 |
| `'{"sku":"A1","count":"3"}'` | valid | integer 違反 | 暗黙数値変換せず拒否 |
| `'{broken'` | valid | JSON parse 失敗 | 拒否、外側 invalid とは記録しない |
| JSON object を data に直接代入 | string 違反 | 実行しない | adapter の不一致を調査 |
| `event` が欠落または未知の値 | required / const 違反 | 実行しない | profile に従って拒否 |
| `data` が4096文字超 | maxLength 違反 | 実行しない | サイズ超過として扱う |

複数 event 型に広げる場合は、既知型ごとに envelope と内包 schema を結び付ける。単に `oneOf` の分岐候補を増やすだけでなく、未知型を無視・隔離・接続終了のどれにするかを SDK と server で揃える。JSON parse 成功だけを業務 schema 適合と扱わない。

## 5. 移行と運用の判定手順

1. **OpenAPI ファイルの構造**: [公式 schema index](https://spec.openapis.org/oas/) は、`schema` が Schema Object を検証しないこと、`schema-base` が OAS base dialect に限定して Schema Object も検証することを区別している。3.2 の iteration は取得時点で `2026-08-30`。iteration 日付は patch 公開日と別で、3.2.x 全体に対応する。schema pass を、runtime payload 検証の pass と読まない
2. **validator の処理範囲**: OpenAPI importer、JSON Schema validator、SSE parser、内包 JSON checker を個別に確認する。`itemSchema` を消して通常の response string に downgrade する変換を、同じ契約を保つ変換として通さない
3. **生成 client の挙動**: 最初の item が接続終了より前に利用側へ届くことを確認する。全文 buffer してから array を返す client なら長時間 stream の用途に適さない。parser、generated SDK、gateway ごとの版を固定して記録する
4. **障害境界**: malformed JSON、複数 data 行、空行前の切断、comment-only heartbeat、遅い consumer、最大サイズ超過を用意する。disconnect を成功完了と数えず、完了イベントが必要な API ではその条件を別に定義する
5. **監視**: wire parsing 失敗、outer item 失敗、inner JSON parse 失敗、inner schema 失敗、consumer lag を分ける。機密情報を含み得る payload 本文をそのまま失敗ログに保存しない

本稿の schema 例だけから、delivery の exactly-once、再接続時の replay window、ordering、認可、transaction commit は導けない。必要なら API の別の契約として定義し、既存の冪等性・メッセージ処理資料と組み合わせる。

## 検証・provenance・未確認事項

- 公式仕様、release notes、JSON Schema、WHATWG の本文を実際に開き、日付と対象節を照合した。Repository 実装を分析・転記していないため、catalog の commit_sha は空欄
- OpenAPI 3.2.0/3.2.1 公式 HTML の対象節比較を実施。SSE/streaming の二節は同一、隣接する positional encoding 節は差分あり
- 2026-10-02 に掲載 YAML を PyYAML 6.0.3 で読み込み、Python jsonschema 4.26.0 / Draft202012Validator と公式 `schema-base/2026-08-30`（参照先 `schema/2026-08-30`、`dialect/2026-02-26`、`meta/2026-02-26` を取得）で構造検証し、エラー0件だった。negative control として itemSchema 内の type を未知の型名へ変更した複製は同じ schema-base に拒否された。公式 JSON の browser/web 取得は content-type 非対応で失敗したため、同じ公式 HTTPS URL を Python urllib で取得して照合した
- 同じ validator で outer/inner schema 自体を check_schema し、独自 fixture 8件（正常、負数、数値文字列、不正 JSON、data が object、event 欠落、未知 event、4097文字）を実行。期待した外側結果・内包結果と8件すべて一致し、不正 JSON は外側 valid / 内包 parse 失敗となることを確認した。SSE wire parsing、ネットワーク分割、SDK はこの fixture では実行していない
- `just index` 後に追加13件の検索 query を個別実行し、全件で文書 ID を取得。既存167 eval case の値と順序を保持し、`git diff --check` も成功。検索 eval は発見性を確認するもので、OpenAPI runtime 実装の適合試験ではない
- OpenAPI 仕様本文と publication index は Apache-2.0。WHATWG 本文は現行の [Intellectual property rights](https://html.spec.whatwg.org/multipage/acknowledgements.html#ipr) に従い CC-BY-4.0、code への取り込み部分は BSD-3-Clause。JSON Schema 掲載文書は IETF Trust の条項に従う。license を確認できなかった release-page 本文は unknown と記録し、独自要約のみを用いた
- Swagger 等の具体的な generator・gateway・validator、ブラウザー、server の組合せによるネットワーク実行は未検証。`contentSchema` の opt-in 設定や結果形式は tool ごとに確認が必要。公式の入門例より規範本文を優先し、掲載例を実装対応の証拠にしない

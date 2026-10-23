---
{
  "id": "api-design-openapi-querystring-duplicate-decoding-boundary",
  "title": "OpenAPI 3.2.1 querystring: 重複キー・入れ子値・復号順序の契約",
  "kind": "knowledge",
  "technology": "api-design",
  "version": "OAS 3.2.1 (2026-09-10), compared with 3.2.0 / 3.1.2 (2025-09-19); OAS 3.2 schema-base iteration 2026-08-30; WHATWG URL updated 2026-09-10; verified 2026-10-05 UTC",
  "tags": [
    "research-domain:api-distributed",
    "openapi",
    "querystring",
    "duplicate-parameters",
    "serialization",
    "deserialization",
    "deepObject",
    "percent-encoding",
    "dependentRequired",
    "path-item-inheritance"
  ],
  "sources": [
    {
      "id": "openapi-321-querystring-deserialization-20261005",
      "url": "https://spec.openapis.org/oas/v3.2.1.html",
      "type": "official_docs"
    },
    {
      "id": "openapi-320-querystring-baseline-20261005",
      "url": "https://spec.openapis.org/oas/v3.2.0.html",
      "type": "official_docs"
    },
    {
      "id": "openapi-312-query-content-baseline-20261005",
      "url": "https://spec.openapis.org/oas/v3.1.2.html",
      "type": "official_docs"
    },
    {
      "id": "whatwg-url-query-tuple-decoding-20261005",
      "url": "https://url.spec.whatwg.org/",
      "type": "official_docs"
    },
    {
      "id": "json-schema-202012-query-dependencies-20261005",
      "url": "https://json-schema.org/draft/2020-12/json-schema-validation",
      "type": "official_docs"
    },
    {
      "id": "openapi-querystring-schema-iterations-20261005",
      "url": "https://spec.openapis.org/oas/",
      "type": "official_docs"
    },
    {
      "id": "openapi-32-querystring-schema-base-20260830-20261005",
      "url": "https://spec.openapis.org/oas/3.2/schema-base/2026-08-30",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2027-01-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# OpenAPI 3.2.1 querystring: 重複キー・入れ子値・復号順序の契約

## 問いと結論

複数の検索条件をOpenAPIで定義したのに、SDK・gateway・serverで異なる値になるのはなぜか。`in: querystring`へ変更すれば、重複キーや入れ子objectまで安全に往復できるか。

全queryを一つのデータとして扱うことと、wire文字列を正しく復号することは別である。まずmedia typeに従って名前と値を復元し、配列・JSON値を組み立て、その後に全体schemaを検証する。重複値を途中でlast-winsに潰すと、後段のschemaには元の違反が見えない。

本稿の受理方針・検索API例・テスト手順は一次資料から導いた独自設計である。既存の[OpenAPI streaming/SSE](openapi-streaming-sse-validation-boundary.md)はresponseの逐次検証を扱う。本稿はrequestのquery serializationと、schema検証に渡す前の情報保存を扱う。

## 適用版と実際に変わった箇所

2026-10-05 UTCに公式HTMLを再確認し、3.2.0と3.2.1の対象節をtag除去・空白正規化して比較した。

| 確認対象 | 本稿での扱い |
|---|---|
| [OAS 3.1.2][oas312]、2025-09-19公開 | `in`の候補は4種類で、`querystring`はない。個別queryの`content`は既に使用できる |
| [OAS 3.2.0][oas320]、2025-09-19公開 | 全queryを表す`querystring`、個別`query`との併用禁止、`content`必須はこの版にもある |
| [OAS 3.2.1][oas321]、2026-09-10公開 | §4.14.5.1に、objectへの復号時は同名の複数値を順序を保った配列へまとめる規範文が追加されている。3.2.0にもarray propertyの要素ごとの直列化は書かれていた |
| [WHATWG URL][url] | Living Standardの表示更新日は2026-09-10。特定ブラウザー版ではない |

**querystring自体を3.2.1の新機能と呼ばない。** 今回の実務上の差分は、復号側にも明示された重複値の扱いをparser設定・schema・受入試験へ結び付ける点である。既存製品が3.2.1公開と同時に挙動を変更したという検証はしていない。

## 1. 個別queryと全queryのモデルを選ぶ

[3.2.1 Parameter Locations / Fixed Fields][parameters]の要件を整理すると、次のようになる。

- `in: querystring`はURLのquery全体を一つの値として記述する。同じoperationに複数置けず、個別の`in: query`とも共存できない。この制約にはPath Itemのparameterも含まれる
- `querystring`ではParameter Object直下の`content`が必須。直下の`schema`・`style`・`explode`・`allowReserved`を並べない。schemaは`content`内のMedia Type Objectへ置く
- Parameter Objectの`content`はmedia typeを1件だけ持つ。複数の媒体から自動選択する表ではない
- `name`は必須だが、`querystring`のwire上にその名前を付けない。`name: searchInput`だから`?searchInput=...`になるわけではない

独自の選択指針:

| 必要な表現 | 選択候補 | 確認点 |
|---|---|---|
| 独立したscalarや単純array | 個別`in: query` + `schema` | `style`と`explode`のwire契約をfixtureで固定する |
| 一つの入れ子filterと他の独立parameter | `in: query` + `content: application/json` | JSON文字列化の後にURL用encodingが要る。既存3.1系で表現できる候補 |
| parameter間の依存やquery全体の制約 | `in: querystring` + form media type | 一つのobjectへ復号した後に全体schemaを評価する |
| framework固有の多段bracket構文 | 明示した独自formatまたはmedia type | `deepObject`だけで互換と宣言しない |

`querystring`へ移すoperationでは、Path Itemから継承する`page`などの個別queryも確認する。operationの`parameters: []`で継承parameterを除去することはできない（[Operation Object][operation]）。共通parameterの置き場所を整理し、実効parameter集合を検査してから移行する。

## 2. 重複キーは型検証より前に失われる

[3.2.1 Encoding By Name][by-name]は、array propertyを同名の値の繰返しとして直列化し、objectへ復号するときの同名の複数値を入力順のarrayへまとめる。この規則は本稿の`querystring` + `application/x-www-form-urlencoded`モデルに適用する。あらゆるHTTP query APIに共通の拒否規則だとは主張しない。

独自例で`tags`をarray、`q`をstringと宣言した場合:

- `tags=alpha&tags=beta`は`["alpha", "beta"]`を保つ。順序を入れ替えたりset化したりしない
- `q=first&q=second`を`"second"`だけに縮約しない。複数値を保持すると`q`はarrayになり、本例のstring制約に適合しない
- `tags=alpha`の単一値も、本例ではarray schemaを手掛かりに`["alpha"]`へ組み立てる。単一値と複数値の両方を試す
- `filter`がobjectなら、同名のJSON値が二つ届いたときにobjectをmergeしない。各値を復号してarrayとして保持すれば、本例のobject制約と不一致になる

受信直後に一般的なmapへ変換してから重複チェックする設計は避ける。ordered pair listまたは同等のmultimapを保持し、wire解析結果・型付きobject・schema判定をそれぞれ検査する。どこかの中継層が先に値を選んでいれば、serverのvalidatorを追加するだけでは直らない。

## 3. deepObject、JSON、encodingの責務を混ぜない

[Style Values / Extending Support for Querystring Formats][styles]で、`deepObject`が定義するのはscalar propertyを持つobjectの表現である。property自身がarrayやobjectの場合の表現は未定義であり、frameworkが受け入れる`filter[status][eq]`を汎用OAS契約と見なせない。

JSONで入れ子値を表すなら、次の二つを区別する。

1. 個別`in: query`の`content: application/json`は、一つのparameter値をJSON化する
2. 全queryをformで表す場合は、`content: application/x-www-form-urlencoded`内の`encoding.filter.contentType: application/json`で、`filter`の値をJSON化する

どちらもJSON serializationとURL encodingは別工程である。後者で`encoding.filter`へ「明示した方が安全」と考えて`style`・`explode`・`allowReserved`のいずれかを足すと、[Encoding Object][encoding]の規則により`contentType`は無視され、RFC6570-styleの経路へ切り替わる。`explode: false`や`allowReserved: false`も明示値なので例外ではない。JSONを意図する箇所にはこの3フィールドを足さない。

`querystring`の`content`を丸ごと`application/json`にする別設計も可能だが、そのwireは名前付きformではない。[Parameter Object Examples][examples]に示されるとおり、JSONをURL向けに符号化した文字列全体が`?`の後へ入る。既存の`?filter=...`との互換変更だと判断しない。

## 4. 全query schemaの独自例

`filter`を指定したときだけ`tenant`も必要、`q`は非空、`tags`は最大10件とする検索APIの例。これは入力形状の説明であり、tenantへのアクセス許可を与える契約ではない。

[JSON Schema Validation §6.5.4][validation]の`dependentRequired`は、あるpropertyが存在する場合に別propertyの存在を要求する。値の同一性や業務上の正しさまでは判定しない。本例では`tenant`を空文字にできない制約も別に付ける。

```yaml
openapi: 3.2.1
info:
  title: Search query example
  version: '1.0'
paths:
  /search:
    get:
      parameters:
        - name: searchInput
          in: querystring
          required: true
          content:
            application/x-www-form-urlencoded:
              schema:
                type: object
                additionalProperties: false
                required: [q]
                dependentRequired:
                  filter: [tenant]
                properties:
                  q:
                    type: string
                    minLength: 1
                  tenant:
                    type: string
                    minLength: 1
                  tags:
                    type: array
                    maxItems: 10
                    items:
                      type: string
                      minLength: 1
                  filter:
                    type: object
                    additionalProperties: false
                    required: [status]
                    properties:
                      status:
                        type: string
                        enum: [active, archived]
              encoding:
                q:
                  contentType: text/plain
                tenant:
                  contentType: text/plain
                tags:
                  contentType: text/plain
                filter:
                  contentType: application/json
      responses:
        '200':
          description: Search result
```

対応する独自wire例は次のとおり。parameterの`name`である`searchInput`は含めない。

```text
/search?q=red+%2B+blue&tags=alpha&tags=beta&filter=%7B%22status%22%3A%22active%22%7D&tenant=acme
```

復号後は`q: "red + blue"`、`tags: ["alpha", "beta"]`、`filter: {"status":"active"}`、`tenant: "acme"`となる。form全体は既にqueryに使える符号化になっているため、完成した`q=...&tags=...`全体をさらにpercent-encodeして一つの値へ包まない。

## 5. 復号順序と境界fixture

[WHATWGのform parser][form-parser]は、まず`&`で分け、各部分を最初の`=`で名前と値へ分け、その後に生の`+`をspaceへ変換し、percent-decodeとUTF-8 decodeを行う。結果は名前と値のtuple listである。`q`と`q=`はいずれも空文字の値になり、parameter自体の欠落とは異なる。

したがってquery全体を先にpercent-decodeしてから`&`で分割したり、復号済みの文字列へもう一度form decodeしたりしない。`allowReserved: true`も無制限な素通し指定ではなく、[OASのURL Percent-Encoding][percent]が要求する媒体・位置ごとの区切り文字の扱いは残る。

以下は本例の受理契約に対する独自fixture。表の結果はPython標準parserと独自adapter、JSON Schema validatorを組み合わせて実行し確認した。実製品へのネットワーク試験ではない。

| 入力・変更 | 復元・期待結果 | 検出したい誤り |
|---|---|---|
| 上記wire例 | `q`のliteral plus、tags順序、filter objectを保持しvalid | 正常形の不一致 |
| `q=x&tags=alpha` | `tags`は1要素arrayでvalid | 単一値だけscalarになる |
| `q=first&q=second` | `q`のarrayがstring制約に違反 | first-wins / last-wins |
| `q=` / `tenant=acme` | 前者はminLength違反、後者はq欠落 | 空文字と欠落の混同 |
| `q=x&filter=%7B%22status%22%3A%22active%22%7D` | tenant欠落でdependentRequired違反 | parameter単体検証だけで通す |
| `q=x&tenant=acme&filter=%7Bbad` | JSON parse失敗 | 型検証前の壊れた値を黙って捨てる |
| `q=A%26B` | 値は`A&B`、valid | 復号後の`&`を新parameterと誤認 |
| `q=%252B` | 値はliteral `%2B`、valid | 二重復号で`+`やspaceへ変える |
| `q=%2B+%20` | literal `+`とspace二つ、valid | `+`と`%2B`の混同 |
| `q=x&filter%5Bstatus%5D=active&tenant=acme` | 未定義propertyで拒否 | bracket構文を暗黙にJSONモデルへ変換 |
| `filter`を異なるJSON objectで2回指定 | arrayとなりobject制約に違反 | objectの暗黙merge |

本例のschemaは機密値、query全体のbyte数、pair数、JSON nestingの上限をすべて表現するものではない。実運用ではparse前後それぞれに処理量の上限を置く。値全体をログに残さず、失敗段階・parameter名・件数など必要な情報に絞る。

## 6. 構造検証が通っても継承の制約を確認する

[公式Schema Iterations][schema-index]は、schemaが全仕様違反を検出する保証はなく、本文と食い違えば本文を優先すると明記する。3.2.x用の取得時点のiterationは`2026-08-30`で、patch公開日を表す日付ではない。`schema-base`はOAS base dialectのSchema Objectも検査する。

本稿では[公式schema-base][schema-base]をPython jsonschema 4.26.0のDraft202012Validatorに渡し、参照先の`schema/2026-08-30`、`dialect/2026-02-26`、`meta/2026-02-26`も同じ公式siteから取得して6件を実行した。

- 掲載例は構造検証に成功
- `querystring`を`content`なしの直下`schema`だけにした変種、media typeを2件にした変種、`name`を消した変種は拒否
- 一つのoperationの同じparameters配列に`query`と`querystring`を置く変種も拒否
- **Path Itemへ`in: query`を置き、Operationへ`in: querystring`を置く変種は構造検証を通過した**。しかし§4.12.1の併用禁止に違反する

最後の結果は、このiteration・validator・自己作成fixtureでの観測である。OpenAPIが併用を許しているという意味ではなく、全validatorに同じ抜けがあるとも断定しない。importerやlintの受入試験では、参照解決とPath Item継承を反映した実効集合に対して、`querystring`の個数と`query`との共存を別途確かめる。

## 移行判定と未確認事項

1. **API表現**: 個別queryから全queryへの変更が必要かを決める。一つのJSON filterだけなら、3.1系にもある個別parameterの`content`で足りる場合がある
2. **静的検査**: 仕様本文、構造schema、継承・参照を解決したlintの3つを分ける。Schema Objectだけを検査してOpenAPI document全体を適合扱いしない
3. **wire往復**: raw query、tuple list、型付きobjectを比較する。重複scalar、1件/複数array、空/欠落、JSON破損、符号化された区切り文字を試す
4. **配備経路**: SDK・gateway・serverの版とparser設定を固定する。生成SDKが`querystring`を無視する、未知の`in`を個別queryへ変換する、proxyが重複値を潰す場合は同じ契約を保てていない
5. **運用判断**: 旧clientを残すなら、旧wireを受ける明示経路と新wireを分けて検証する。specの版番号を上げただけで旧入力の受理範囲が安全に変わるとは期待しない

検証とprovenance:

- Python 3.12.14、PyYAML 6.0.3、jsonschema 4.26.0を使用。掲載YAMLの読込、全体構造6件、bounded profileのwire/decode/schema fixture12件を実行し、記載の結果を確認した。独自adapterはknown fieldの型組立てだけを行う小さな検証用コードで、汎用OAS実装や本番parserではない
- 公式HTMLのnative web閲覧とダウンロードによる対象節比較を実施。schema-baseのnative web取得はエラーだったため、同じ公式HTTPS URLをPython urllibで取得した。JSON Schema本文のshell取得は403だったが、native webで必要節・日付・licenseを読めた。取得できない経路の成功を推測しない
- OAS仕様とschema publicationはApache-2.0。仕様repositoryの3.2.1・3.2.0・3.1.2タグのLICENSEとpublication siteの表記を確認した。WHATWG本文はCC-BY-4.0、source codeに取り込む部分はBSD-3-Clause。JSON Schema掲載文書はIETF Trust/BCP 78の条項に従う。本文・例・fixtureは独自記述で、外部実装コードを転載していない
- JSON Schema参照文書はDraft 2020-12 Validationの`draft-bhutton-json-schema-validation-01`（2022-06-16掲載、IETF draftとしての期限は2022-12-18）であり、RFCや新規公開仕様とは呼ばない
- 具体的なOpenAPI generator、gateway、server frameworkによるHTTP round trip、invalid UTF-8、不正percent triplet、大量入力、数値/boolean/nullの一般的な型変換は未検証。文字列だけの例から、それらの相互運用性を一般化しない

[oas321]: https://spec.openapis.org/oas/v3.2.1.html
[oas320]: https://spec.openapis.org/oas/v3.2.0.html
[oas312]: https://spec.openapis.org/oas/v3.1.2.html
[parameters]: https://spec.openapis.org/oas/v3.2.1.html#parameter-object
[operation]: https://spec.openapis.org/oas/v3.2.1.html#operation-object
[by-name]: https://spec.openapis.org/oas/v3.2.1.html#encoding-by-name
[styles]: https://spec.openapis.org/oas/v3.2.1.html#extending-support-for-querystring-formats
[encoding]: https://spec.openapis.org/oas/v3.2.1.html#encoding-object
[examples]: https://spec.openapis.org/oas/v3.2.1.html#parameter-object-examples
[percent]: https://spec.openapis.org/oas/v3.2.1.html#url-percent-encoding
[url]: https://url.spec.whatwg.org/
[form-parser]: https://url.spec.whatwg.org/#concept-urlencoded-parser
[validation]: https://json-schema.org/draft/2020-12/json-schema-validation#section-6.5.4
[schema-index]: https://spec.openapis.org/oas/
[schema-base]: https://spec.openapis.org/oas/3.2/schema-base/2026-08-30

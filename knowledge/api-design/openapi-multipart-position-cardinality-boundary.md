---
{
  "id": "api-design-openapi-multipart-position-cardinality-boundary",
  "title": "OpenAPI multipart: prefixEncoding・必須part数・Content-Typeの受理境界",
  "kind": "knowledge",
  "technology": "api-design",
  "version": "OAS 3.2.1 (2026-09-10), compared with 3.2.0 (2025-09-19); JSON Schema Draft 2020-12 Core/Validation (2022-06-16); RFC 2046 / RFC 7578; verified 2026-10-24 UTC",
  "tags": [
    "research-domain:api-distributed",
    "openapi",
    "multipart",
    "prefixEncoding",
    "itemEncoding",
    "prefixItems",
    "cardinality",
    "Content-Type",
    "ordered-bundle",
    "schema-validation"
  ],
  "sources": [
    {
      "id": "openapi-321-multipart-position-20261024",
      "url": "https://spec.openapis.org/oas/v3.2.1.html",
      "type": "official_docs"
    },
    {
      "id": "openapi-320-multipart-position-baseline-20261024",
      "url": "https://spec.openapis.org/oas/v3.2.0.html",
      "type": "official_docs"
    },
    {
      "id": "openapi-321-release-multipart-20261024",
      "url": "https://github.com/OAI/OpenAPI-Specification/releases/tag/3.2.1",
      "type": "release_notes"
    },
    {
      "id": "json-schema-202012-multipart-prefix-20261024",
      "url": "https://json-schema.org/draft/2020-12/json-schema-core",
      "type": "official_docs"
    },
    {
      "id": "json-schema-202012-multipart-length-20261024",
      "url": "https://json-schema.org/draft/2020-12/json-schema-validation",
      "type": "official_docs"
    },
    {
      "id": "rfc2046-multipart-envelope-order-20261024",
      "url": "https://www.rfc-editor.org/rfc/rfc2046.html",
      "type": "official_docs"
    },
    {
      "id": "rfc7578-multipart-form-order-20261024",
      "url": "https://www.rfc-editor.org/rfc/rfc7578.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# OpenAPI multipart: prefixEncoding・必須part数・Content-Typeの受理境界

## 問いと結論

JSON manifest と CSV payload を順番に送る API で、`prefixEncoding` を2件書けば「必ず2 part、最初が manifest」と保証できるか。**encoding の位置対応、schema の必須個数、wire の媒体型・終端を別々に検証する。** 位置ごとの encoding を列挙するだけでは、欠けた part を要求できない。

本稿は有限の multipart bundle を受理するための契約を扱う。[querystring の重複・復号](openapi-querystring-duplicate-decoding-boundary.md)や[streaming/SSE の itemSchema](openapi-streaming-sse-validation-boundary.md)とは別の問いである。後者には3.2.1の multipart 改訂への短い言及があるが、必須part数・順序・媒体型を組み合わせた受入設計はない。2026-10-24 の事前 index/search でも `prefixEncoding` は該当なし、`multipart manifest` は HTTP Range 文書だけに一致した。

## 1. 3.2.1で明確化されたこと、以前から同じこと

[3.2.1 release notes][release]は仕様文章の訂正・明確化を説明する。**3.2.1の公開は runtime feature の出荷や SDK の対応完了を意味しない。** 2026-10-24 UTCに[3.2.0][oas320]と[3.2.1][oas321]の該当本文を比較した範囲は次のとおり。

| 対象 | 3.2.0 | 3.2.1と本稿の判断 |
|---|---|---|
| §4.14.5.2 positional encoding の前提 | `itemSchema` または array `schema` が必要と記述 | data instance の対応itemを対象にし、対応のない encoding entry は無視すると明記。schema構造と実際のpartの存在を区別する |
| instance が `prefixEncoding` より短い場合 | 既に error ではなく、余ったencodingを無視すると記述 | 同じ扱い。欠落partが許されることを3.2.1で新設された挙動と呼ばない |
| §4.14.5.3 ordered form-data | `prefixEncoding` と手動の `Content-Disposition` / name header を説明 | array schema（または `itemSchema`）と、各itemのproperty名からpart名を作る説明へ整理。§4.14.5.2にも name/value object を並べるarrayの説明を追加 |

以下の推奨は、この版の公式契約を組み合わせた独自設計である。実装が仕様の各patch版を識別するか、generatorが同じwireを出すかは未検証。自動的に既存APIのwire形式が変わるとも主張しない。

## 2. 位置の指定は、存在の要求ではない

[OAS 3.2.1 §4.14.5.2][position]では、`multipart/mixed` 等を到着順のarrayとして扱い、`prefixEncoding` を同じindexのitemへ適用する。`itemEncoding` はその後のitemへ適用する。どちらも存在しないitemを生成せず、余った設定は無視される。

[JSON Schema Core §10.3.1][core]も、`prefixItems` 自体はarrayの長さを制約しない。長いarrayでは指定されたprefixだけを検査する。同じschema objectにある `items` は残りのitemへ適用される。[Validation §6.4][validation]の `minItems` / `maxItems` が長さを制約し、`minItems` の省略は0と同じである。

したがって、独自の2-part契約なら次を分担させる。

- `type: array` と `prefixItems`: first manifest、second payload の型と内容
- `minItems: 2`: 二つとも必要。`prefixItems` が2件あるだけでは代わりにならない
- `items: false`: prefix後の追加partを拒否。`itemEncoding` を省略しただけでは追加part拒否にならない
- `prefixEncoding`: 各位置のmedia type。`prefixItems` と同じ数で管理するのは本例の設計方針であり、OAS一般の必須条件ではない

`items: false` を別の `allOf` branchへ移すだけでは、別branchの `prefixItems` とこの対応を共有しない。schemaを組み替えるときは、見た目の重複除去ではなく実際の評価対象を試す。本稿の例は同じschema objectへ置く。

また、第3part用の `itemEncoding` を足しても、`items: false` が許可へ変わるわけではない。serialization設定とvalidation条件を同じスイッチとして扱わない。

## 3. 独自例: JSON manifest + CSV textの有限bundle

次は本稿で設計したOpenAPI fragmentであり、上流の例のコピーではない。有限の全体を `schema` で検証し、無期限streamやpart単位の逐次commitは対象にしない。

```yaml
requestBody:
  required: true
  content:
    multipart/mixed:
      schema:
        type: array
        minItems: 2
        prefixItems:
          - type: object
            additionalProperties: false
            required: [batchId, kind]
            properties:
              batchId:
                type: string
                minLength: 1
              kind:
                const: ledger-import
          - type: string
            minLength: 1
        items: false
      prefixEncoding:
        - contentType: application/json
        - contentType: text/csv; charset=utf-8
```

`requestBody.required` はbodyの存在、`minItems` はpart数、manifest内の `required` はpropertyの存在を制約する。三つを代替関係にしない。ここでは `maxItems: 2` を重ねなくても `items: false` が余分なpartを拒否する。

想定する復号後の独自instanceは次のとおり。CSVはこの段階では文字列であり、列構造までJSON Schemaで検査したという意味ではない。

```json
[
  {"batchId": "batch-17", "kind": "ledger-import"},
  "entry,amount\r\nx1,18\r\n"
]
```

送信側はouter `Content-Type` に `multipart/mixed` と実際のboundaryを載せ、part 1を `application/json`、part 2を `text/csv; charset=utf-8` として出力する。本例はpartを名前で探して並べ直す契約ではない。順序を逆転した入力を、metadataらしい内容を見つけて自動修復しない。

CSVの列名、数値、件数、manifestとの関連は別の業務検証である。`batchId` は単なるstringであり、重複排除やtransactionを自動で提供しない。受理したbatchの確定方式は別途設計する。

## 4. contentTypeと実際のpart headerを混同しない

[OAS Encoding Object][encoding]の `headers` に `Content-Type` を書いても、そのentryは無視される。media typeは専用の `contentType` へ書く。`headers.Content-Type.required: true` を足す方法で欠落headerを検査できたつもりにならない。

別の許可されたheaderを [Header Object](https://spec.openapis.org/oas/v3.2.1.html#header-object) の `required: true` としても、要求するのは対応する既存part内のheaderであってpart自体の存在ではない。part数は引き続き `minItems` 等で検査する。

[Multipart guidance][multipart]は複数の `contentType` 候補を許すが、送信時にはどれを使うかapplicationが指定できる仕組みを要求し、media type sniffingを既定の選択方法にしない。本例は各位置を一つのtypeへ限定する。候補リストをwireの `Content-Type` へそのまま一行で送る設計ではない。

[RFC 2046 §5.1 / §5.1.1][mime]ではpart header自体は省略でき、`multipart/mixed` のpartで `Content-Type` がなければ通常 `text/plain` として扱う。OASのencoding defaultが、既に到着したwireへJSON headerを補充する保証にはならない。

独自の受理profileとして、本例では両partの `Content-Type` 明示を要求し、位置ごとのtypeとcharsetを照合してからJSON/文字列へ復号する。省略や不一致を拒否するのはこのAPIの制約であり、あらゆるmultipartが同じheader必須条件を持つという主張ではない。schemaに渡った値が同じでも、途中で不正なheaderを無視していたらその違反はschemaから見えない。

型の宣言は内容の正しさの証明でもない。`application/json` のpartは実際にJSON parseし、CSVは別にparseする。将来画像やbinaryへ拡張するとき、empty schemaを「ファイルの検査済み」に読み替えない。標準JSON Schemaだけではraw binaryやMIME envelope全体を検査できないため、現在の文字列例と同じfixtureだけで移行済みとしない。

## 5. mixedの位置対応とform-dataの名前対応を分ける

[OAS §4.14.5.1–3][position]に従うと、object型の `multipart/form-data` では `encoding` のkeyがpartの `Content-Disposition` のnameへ対応する。一方、順序を保持するarrayモデルではname/valueのobjectを一itemとして並べる。普通のobjectへ潰してから、元の全part順序を復元できるとは限らない。

[RFC 7578 §4.2–4.3 / §5.2][form]は、form-dataの各partへnameを要求し、同じform fieldの複数ファイルを同名の別partで表す。intermediaryは結果を並べ替えず、同名partをcoalesceしない。これは「重複名なら何でも拒否」という規則でもない。

本例をbrowserのformへ置換したいなら、`mixed` のarrayをそのままnamed form APIとみなさず、名前・重複・順序のwire契約を再設計する。過去のOASが説明した非form-data multipartへの `Content-Disposition: form-data` 名付けworkaroundも、一般の `multipart/mixed` 実装が理解するとは限らない。generated client / gateway / server間で実物のfixtureを比較する。

## 6. 受入試験はschemaとwireを分ける

以下は本例の独自fixtureである。schema欄は、掲載YAMLからschemaを取り出し、PyYAML 6.0.3 / jsonschema 4.26.0のDraft202012Validatorで2026-10-24に実行した結果。MIME parser、CSV parser、HTTP endpoint、SDK、gatewayの実行試験ではない。

| 入力・変更 | schema評価 | 別途必要な判定 |
|---|---|---|
| 上記manifest + CSV文字列 | valid | 媒体型、CSV、業務値を検証 |
| 空array、manifestだけ | invalid (`minItems`) | bodyやpartの欠落を成功にしない |
| payloadが先、manifestが後 | invalid (`type`) | receiverが順序を並べ直して隠さない |
| 第3partを追加 | invalid (`items`) | parserが未知partを先に捨てない |
| 空のbatchId、またはkind不一致 | invalid | schema入力前の値の改変を避ける |
| payloadが空文字列 | invalid (`minLength`) | 空partと欠落partは別のケース |
| 非空だが想定CSV列を満たさない文字列 | valid | CSV・業務validatorで拒否する |
| 正常な復号値のままpart headerだけ欠落/不一致 | schemaだけでは検出不能 | wire側で明示type契約を検査する |

対照fixtureでは `minItems` を除くと空arrayとmanifestだけがvalidになり、`items` を除くと第3partがvalidになった。`prefixEncoding` の数はこのJSON Schema評価に介入しない。

[RFC 2046][mime]が定めるboundary、part区切り、終端をMIME parserで検査し、途中切断を正常な全体としない。本profileでは必要な二つの値が得られても、それだけで全bodyの受信成功や業務commitを確定しない。byte数・part数・header量・parse期限・一時保存量の上限を入口で設け、全体検証後に公開するのは独自の推奨である。`minItems` は入力byte上限ではない。

## 適用限界と採用前の確認

- 検証したのは仕様本文の比較と、独自の復号後instanceに対するschema評価。掲載fragmentの完全なOAS metaschema検証、wire round-trip、特定製品の3.2.1対応は未確認。
- JSON Schema資料はOASが参照するDraft 2020-12に固定した。表示は2022-06-16公開、IETF draftとしては2022-12-18失効。新しいIETF標準として扱わない。
- nested multipart、raw binary、transfer encoding、文字コードの不一致、Content-Typeの重複、悪意あるboundary、途中切断のfault testは今回実行していない。必要なものを採用先のparser・HTTP経路で試す。
- OASとJSON Schemaの型契約だけでCSVの正当性・業務atomicity・再送の冪等性は得られない。それぞれの失敗段階を記録し、不完全なbundleを「一部受理で成功」に変えない。
- 原文と既存コードはコピーせず独自に要約・設計した。release-page proseとRFC 2046の具体的なlicense grantはページ上で確認できなかったためcatalogではunknownとした。moduleへのコード取り込みはない。

[oas321]: https://spec.openapis.org/oas/v3.2.1.html
[oas320]: https://spec.openapis.org/oas/v3.2.0.html
[release]: https://github.com/OAI/OpenAPI-Specification/releases/tag/3.2.1
[position]: https://spec.openapis.org/oas/v3.2.1.html#encoding-by-position
[encoding]: https://spec.openapis.org/oas/v3.2.1.html#encoding-object
[multipart]: https://spec.openapis.org/oas/v3.2.1.html#encoding-multipart-media-types
[core]: https://json-schema.org/draft/2020-12/json-schema-core#section-10.3.1
[validation]: https://json-schema.org/draft/2020-12/json-schema-validation#section-6.4
[mime]: https://www.rfc-editor.org/rfc/rfc2046.html#section-5.1
[form]: https://www.rfc-editor.org/rfc/rfc7578.html#section-5.2

---
{
  "id": "llm-claude-api-sdk-version-rollback-boundary",
  "title": "Claude API 2026-10-21: API版・SDK更新・認証transportを分けるrollback境界",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Claude API 2026-10-21 (released 2026-10-21) vs 2023-06-01; Python SDK v2.0.0 @ ecfee6cd504cfbf634919bdbd058fb74afdbd533; official docs verified 2026-10-23 UTC; runtime未検証",
  "tags": [
    "research-domain:ai-engineering",
    "llm",
    "claude",
    "anthropic-version",
    "sdk-migration",
    "rollback",
    "MessageParam",
    "Authorization",
    "credential-precedence",
    "sampling",
    "history-replay"
  ],
  "sources": [
    {
      "id": "claude-api-version-release-20261021-verified-20261023",
      "url": "https://platform.claude.com/docs/en/release-notes/overview",
      "type": "release_notes"
    },
    {
      "id": "claude-api-version-contract-20261023",
      "url": "https://platform.claude.com/docs/en/api/versioning",
      "type": "official_docs"
    },
    {
      "id": "claude-api-sdk-migration-20261021-verified-20261023",
      "url": "https://platform.claude.com/docs/en/api/migration-2026-10-21",
      "type": "official_docs"
    },
    {
      "id": "anthropic-python-v200-client-contract-20261023",
      "url": "https://github.com/anthropics/anthropic-sdk-python/tree/ecfee6cd504cfbf634919bdbd058fb74afdbd533",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# Claude API 2026-10-21: API版・SDK更新・認証transportを分けるrollback境界

## 問いと適用範囲

Claude SDKを更新したあとに失敗が増えたとき、`anthropic-version` だけを旧値へ戻せば復旧するか。答えは、失敗がAPI契約の変更によるものか、SDKの型・credential選択・送信headerによるものかで異なる。HTTPを送る前の例外やgatewayでの認証不一致は、API側の版選択だけでは元に戻らない。

[2026-10-21の公式告知](https://platform.claude.com/docs/en/release-notes/overview#october-21-2026)は、新API版と、その版を既定で送るSDK major releaseを同時に公開した。例はPython 2.0.0、TypeScript 1.0.0、Go 2.0.0、`ant` CLI 2.0.0。本書はその移行を一つのpackage更新として扱わないための記録であり、SDK全体の移行項目一覧ではない。具体的な実装確認はPython v2.0.0に限定する。

既存の[Message Batchesの結果回収](claude-batch-outcome-reconciliation-retry-boundary.md)や[引用座標の検証](../rag/citation-evidence-coordinate-evaluation.md)とは異なり、ここではリクエストを組み立てる側の互換性とrollbackを扱う。

## 一次資料で確認した契約

### 1. API版はモデル名やSDK版と別の軸である

[Versions](https://platform.claude.com/docs/en/api/versioning)では、API版を `anthropic-version` で指定する。同じAPI版でも任意入力の追加、出力値・enum variantの追加、特定errorの発生条件変更が可能とされる。版の固定は、応答のあらゆる値やエラー条件を永久固定する意味ではない。

2026-10-21での主なMessages入力差分は次のとおり。同じモデルを呼び続けても、API版を変更すれば影響する。

| 入力 | 新API版の契約 | 移行時に分けること |
|---|---|---|
| `temperature` / `top_p` / `top_k` | どのモデルでも、指定すると400。既定値の明示でも拒否 | 値を既定値へ変更するのではなくfieldを省く |
| `assistant` 直下の `document` block | 400 | documentをuserへ渡す場合と、tool result内の入れ子を区別 |
| `user` のtext blockに非空 `citations` | 400。`search_result` 内のtextも対象 | 引用付きassistant出力をuser入力へ変換する経路を点検 |

[移行ガイドのMessages API節](https://platform.claude.com/docs/en/api/migration-2026-10-21#messages-api)は、これらをMessages、token counting、新API版で作成するMessage Batch内の要求へ適用すると説明する。一方、tool result等の入れ子document、assistant textのcitations、document/search resultで引用を有効化する設定は引き続き受け付ける。

sampling省略時は従来の省略時と同じ既定動作で、`temperature` は1.0。以前に非既定値を使えたモデルでは、fieldの削除によりその調整効果が失われる。移行ガイドは同等の代替設定を示していない。入力を受理できることと、従来と同じ出力品質を得ることは別の判定にする。

### 2. SDKのcredentialとheaderはAPI版を読んで元に戻るわけではない

Python v2.0.0の固定commit [ecfee6cd…](https://github.com/anthropics/anthropic-sdk-python/tree/ecfee6cd504cfbf634919bdbd058fb74afdbd533)で次を確認した。これはソースの静的確認であり、実通信の測定ではない。

- [`_constants.py`](https://github.com/anthropics/anthropic-sdk-python/blob/ecfee6cd504cfbf634919bdbd058fb74afdbd533/src/anthropic/_constants.py)のAPI既定値は `2026-10-21`。
- [`_client.py`](https://github.com/anthropics/anthropic-sdk-python/blob/ecfee6cd504cfbf634919bdbd058fb74afdbd533/src/anthropic/_client.py)の `_api_key_auth` は `Authorization: Bearer` を生成する。API版headerを組み立てる `default_headers` とは別の処理であり、旧API版を指定しても自動で `X-Api-Key` に戻す分岐はない。
- 基本 `Anthropic` clientの `api_key`、`auth_token`、`credentials`、`config`、`profile` は明示指定が排他的。複数の非None値を渡すとconstructorで `ValueError` になる。
- 明示credentialがなければ、環境のAPI keyがauth tokenより先に選ばれる。両方があっても二つの認証方式を併送せず、keyを選び警告する。明示credentialがあれば他のcredentialを環境から補わない。
- `copy()` / `with_options()` のcredential指定は継承元との合成ではなく差替え。元のprofile/configに付随したbase URL・headersもそのcredentialとともに切り替わる一方、呼出側が明示したbase URLは保持される。

[移行ガイドの認証header節](https://platform.claude.com/docs/en/api/migration-2026-10-21#api-keys-are-sent-as-authorization-bearer)は、従来のrequest単位の `X-Api-Key` 上書きが新clientのkeyを置換せず、両headerを送ることになる点を警告する。API自体がどちらのheaderも受理することは、途中のproxy、log filter、test stubまで互換である証拠にはならない。

ここは基本clientの説明である。同ガイドは `AnthropicFoundry` などplatform clientの例外も区別しており、「全SDK・全platformが必ず同じheader」へ一般化しない。

### 3. Pythonの型alias変更はHTTPより前に失敗する

固定版の [`types/message_param.py`](https://github.com/anthropics/anthropic-sdk-python/blob/ecfee6cd504cfbf634919bdbd058fb74afdbd533/src/anthropic/types/message_param.py)で、`MessageParam` はuser / assistant / systemのrole別型のunionになっている。[移行ガイド](https://platform.claude.com/docs/en/api/migration-2026-10-21#messageparam-is-a-union-with-one-type-for-each-role)では、型注釈としての利用やdict literalは使える一方、`MessageParam(...)` というconstructor呼出しは `TypeError` になると説明する。

これはwire JSONのfield名変更ではない。HTTPエラーが記録されていない失敗を、API版拒否やモデル障害と一括集計しない。`BetaMessageParam` や各言語の差分は移行ガイドで個別に確認する。

## 独自の設計案: rollback単位を一つのheaderにしない

以下は本書の提案であり、Anthropicが保証するdeployment方式ではない。

リリース記録に少なくとも `SDK版 / API版 / model識別子 / platform client / gateway設定版 / serializer版 / credentialの選択元` を揃えて保存する。credentialの選択元は「明示key」「明示token」「profile名」などで、秘密値そのものは記録しない。

| 障害の境界 | 観測するもの | 戻す候補 |
|---|---|---|
| message構築・constructor | 例外型、呼出箇所、SDK版。HTTP送信があったか | 呼出コードとSDKの組合せ |
| gatewayまで | 送信先、認証header名、credential種別、gatewayの認証結果 | transport adapter・credential選択・gateway設定 |
| API入力検証 | 実際のAPI版header、送信fieldの有無、API errorとrequest ID | API版とrequest serializer |
| 受理後の品質 | 固定taskでの品質・失敗・費用指標 | model・prompt・sampling廃止への対応を含む評価済み構成 |

HTTP成功をもって認証主体が正しいと判断しない。検証用の許可されたidentityで実行し、意図したworkspace・権限・課金先かを、その環境で利用できる監査情報と照合する。秘密headerを診断ログへ書かず、伏せたcaptureやmock transportではheader名・個数・送信先を検査する。

「新SDK＋旧API版」は旧構成と同一ではない。旧headerに戻せる経路があっても、SDKの型やcredential処理まで復元されるとは限らない。逆にSDKだけ戻してgatewayのheader変換を残せば、元の構成とも新構成とも異なる。rollback用には、事前検証した組合せを丸ごと残す。新SDKで旧API版の全機能が使えるという互換性保証は本調査では確認していない。

## 独自の設計案: 移行を検証する順序

1. **比較基準を保存する。** 現行の依存lock、gateway設定、serializerと代表的な入力形状を残す。履歴の元データは保全し、API向け変換結果と別に扱う。監査・引用情報を消すmigrationを元データへ直接適用しない。
2. **送信前検証を先に行う。** role別message生成、constructorのcredential排他、credential差替え後の送信先をテストする。SDK更新とAPI版更新を独立変数として記録し、起きた変更をどちらに帰属できるか確認する。
3. **gateway境界を確認する。** API keyが旧headerにあると仮定した認証・redaction・mockを点検する。gateway用tokenとupstream用keyを使う構成では、各hopの受取人を明示し、意図しないcredentialのforwardを拒否する。
4. **保存履歴を新serializerで再生する。** 単純な新規会話だけでなく、引用付き回答を別roleへ移す処理、tool resultの入れ子、検索結果、batch生成を対象にする。拒否回避のため全citationsを一律削除せず、roleとblock位置ごとの規則を適用する。
5. **品質とrollbackを別々に判定する。** 非既定samplingを使っていたtaskでは、400が消えたあとも評価を続ける。旧構成へ戻す演習では、成功だけでなく送信先・identity・API版・SDK版が基準と一致することを確認する。

上記は移行時に実施する検証案であり、本書の追加時に実施済みのintegration testではない。

## 境界テストの提案

| 条件 | 確認すべき結果・見落とし |
|---|---|
| 新API版へ `temperature=1.0` を明示 | 既定値でも拒否される契約を検出。field省略ケースを別に持つ |
| user textの非空citationsとassistant textのcitations | 前者だけを拒否対象として区別し、引用機能全体を無効にしない |
| assistant直下documentとtool result内document | 同じdocument型だから同じ扱い、と平坦化しない |
| Python SDK v2.0.0で `MessageParam(...)` を使う旧呼出し | SDK側失敗とAPIの400を区別する |
| 基本clientへ明示keyと明示token | constructorの排他エラーを捕捉し、network retryを開始しない |
| 環境keyと環境tokenを両方設定 | 選択されたidentityが期待どおりかを確認。tokenが選ばれると仮定しない |
| key差替えに旧 `X-Api-Key` 上書きを使用 | key置換ではなく二重送信になり得る経路をmockで検出する |
| API版だけを旧値へ戻す | SDK側の例外・認証headerが元へ戻ったと判定しない |

`evals/knowledge/ai-engineering.json` の追加caseはこの文書を検索できることを確かめるだけで、上記の製品動作を実行して検証するものではない。

## 適用版・制約・未確認事項

- 取得日は2026-10-23 UTC。公式migration / versionページは当日のrolling文書であり、固定公開snapshotではない。実装分析はPython v2.0.0の40桁commitに固定した。後続patchへ無条件に適用しない。
- [Versions](https://platform.claude.com/docs/en/api/versioning)では、Amazon Bedrockの新API版はClaude 4.7以降のモデルが対象。Bedrockの旧モデルを、headerだけの変更で対応済みにしない。platformごとの実通信は未検証。
- SDKをinstallせず、credentialを使用せず、live requestも実行していない。ソース上の分岐を確認したことと、利用者のgateway・型checker・runtimeで成功したことを区別する。
- streamingの未知event、全Admin API field改名、SDK全言語の移行、server-side compaction移行は対象外。固定API版でも出力variantが増え得るため、完全な旧応答集合を前提としたparserの安全性は別途確認する。
- SDKの固定版LICENSEはApache-2.0。公式文書のオープンライセンスは確認できず、外部実装コードのコピーは行っていない。元のSDK licenseが新しい版により遡って変わるとは扱わない。
- release notesを含むためTTLの最短値は30日。再確認期限は2026-11-22であり、旧API版の廃止日を意味しない。旧API版の将来の提供終了日は今回の根拠から確定していない。

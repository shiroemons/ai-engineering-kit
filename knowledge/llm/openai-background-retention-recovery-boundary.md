---
{
  "id": "llm-openai-background-retention-recovery-boundary",
  "title": "OpenAI Responses background: 一時保持・Webhook再送・cursorと回収完了の境界",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Responses API background mode; rolling official guides and normal HTTP reference verified 2026-10-04 UTC; fixed API/SDK version and retention-change publication date not stated; live behavior untested",
  "tags": [
    "research-domain:ai-engineering",
    "llm",
    "openai",
    "responses-api",
    "background",
    "retention",
    "polling",
    "webhook",
    "stream-resume",
    "cancellation",
    "recovery"
  ],
  "sources": [
    {
      "id": "openai-background-lifecycle-20261004",
      "url": "https://developers.openai.com/api/docs/guides/background",
      "type": "official_docs"
    },
    {
      "id": "openai-background-retention-controls-20261004",
      "url": "https://developers.openai.com/api/docs/guides/your-data",
      "type": "official_docs"
    },
    {
      "id": "openai-background-response-retrieve-20261004",
      "url": "https://developers.openai.com/api/reference/python/resources/responses/methods/retrieve",
      "type": "official_docs"
    },
    {
      "id": "openai-background-response-cancel-20261004",
      "url": "https://developers.openai.com/api/reference/python/resources/responses/methods/cancel",
      "type": "official_docs"
    },
    {
      "id": "openai-background-response-delete-20261004",
      "url": "https://developers.openai.com/api/reference/resources/responses/methods/delete",
      "type": "official_docs"
    },
    {
      "id": "openai-background-webhook-delivery-20261004",
      "url": "https://developers.openai.com/api/docs/guides/webhooks",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# Responses background の実行・保持・回収を分ける

## 問いと採用判断

長時間の生成を `background=true` にし、終了Webhookだけを待てば、worker再起動後も確実に結果を回収できるか。

**生成の終端、結果を再取得できる期間、自社側への取込完了は別の条件である。** とくに一時保持のbackground応答では、Webhookの再送期間を結果の保存期間と解釈しない。response ID・保持条件・取込済みの証拠を保存し、切断時は同じIDの照会を優先する。取得不能を理由に同じ業務を新規生成へ自動再投入しない、というのが本書の設計判断である。

対象は通常のResponses APIのbackground実行。既存の[Multi-agent終了・注入境界](../agents/openai-multi-agent-response-completion-boundary.md)が扱うbetaのagent協調、[MCP Tasks](../mcp/tasks-extension-polling-cancellation-migration.md)のwire仕様、[Claude Batch](claude-batch-outcome-reconciliation-retry-boundary.md)の個別結果照合は引き継がない。一般Webhook実装の重複排除は[既存文書](../api-design/webhook-redelivery-signature-dedup.md)を参照し、本書では生成結果の保持期間との組合せを扱う。

## 適用版・確認事実

2026-10-04 UTCのrolling公式文書と通常Responses referenceを読んだ。固定API schema版、以下の保持条件の導入日、SDK最低版はページから確認していない。取得日は公開日ではなく、特定modelの新機能リリースとも主張しない。

- 作成に `background=true` を指定し、`GET /v1/responses/{response_id}` で照会する。guideは `queued` / `in_progress` の間にpollingし、そこを離れた状態を終端とする。[Background][background]
- retrieve referenceのstatus列挙は `completed` / `failed` / `in_progress` / `cancelled` / `queued` / `incomplete`。`error` と `incomplete_details` は別fieldである。`output` は複数の型のitemを持ち、配列先頭が回答とは限らない。[Retrieve][retrieve]
- streamingで再接続するには、作成時に `background=true` と `stream=true` の両方が必要。受信eventの `sequence_number` に対応するcursorを保持する。`starting_after` は指定したeventの後から配信するための整数である。[Background][background] / [Retrieve][retrieve]
- `POST /v1/responses/{response_id}/cancel` の対象はbackgroundで作成した応答に限る。guideは同じ取消の再呼出しが冪等で、後続呼出しは最終Responseを返すと説明する。[Cancel][cancel] / [Background][background]
- `DELETE /v1/responses/{response_id}` は応答削除用の別endpointで、返却値に `deleted` がある。cancelの成否確認をdeleteで代用する契約ではない。[Delete][delete]

## 保持条件をrequestとprojectで固定する

[Your data][retention]は、例外を除くResponsesの通常Application State保持を少なくとも30日と記す。同じページと[Background guide][background]に、次のbackground向け条件がある。

| 対象 | 確認した記載 | 採用時の判断 |
| --- | --- | --- |
| ZDR projectのbackground | `store=false` で実行し、非同期処理・polling用に応答を概ね10分diskへ一時保持 | `store=false` をdiskへの書込み皆無と説明しない |
| MAM / enhanced MAMのbackgroundでstore省略またはfalse | 一時polling期間後に応答を削除 | foregroundの省略時保持をそのまま適用しない |
| MAM / enhanced MAMのbackgroundで明示store=true | 一時期間後も通常の保持期間に従う | 回収要件と許容する保存方針を照合して明示する |

ZDR有効時は `store=true` を送ってもfalse扱いになるという記載もある。以上は取得時の資料の条件分けであり、全projectへ同じ省略時挙動を一般化しない。顧客固有の契約、その他の保持policy、toolが使う外部サービスの保存条件は別に確認する。[Your data][retention]

**観察:** 概ね10分という説明だけでは、厳密な削除時刻、期間の起算点、長時間実行との関係、最低可用期間のSLAは確定しない。`created_at + 600秒` をAPIが保証する期限として表示しない。この数字を生成時間の上限や、期限まで待ってから取得すればよい根拠にも使わない。

**独自の運用案:** 要求を投入する前にprojectの保持policyとstore指定を記録し、復旧に許容する遅延を決める。一時保持を選ぶ場合は早期に回収して、許可された自社保存先へ最小限の結果を確定する。長い回収猶予が必要なら、許可された保持設定で満たせるか先に確認する。自社保存でも同意・保存期限・削除義務を別途設計し、providerの一時保存終了を自社copyの削除と同一視しない。

## 72時間の通知再送は結果保持の延長ではない

[Webhooks guide][webhooks]の `response.completed` 例は `data.id` にresponse IDを持ち、受信後にretrieveする構成である。同guideは、受信先の失敗・応答timeoutに対して最大72時間の再送、まれな重複配信、`webhook-id` による重複排除を説明する。署名検証にはraw bodyが必要である。

**資料を組み合わせた推論:** 一時保持を選んだ応答で、Webhookが後から届いても、その時点で結果をretrieveできる保証にはならない。最大72時間は通知の再試行についての説明であり、結果保持も72時間へ延長されるという記載は確認していない。

独自の受信設計では、次の二つの状態を分離する。

1. 署名を検証し、event ID・response IDと受領記録を耐久的に保存した段階
2. 必要なResponseを取得し、自社の結果storeへ保存・検証した段階

`2xx` は受領の応答として返し、重い処理はworkerへ渡す。ただし「queueにIDが入ったから回収完了」とせず、一時保持の結果を長時間queue待ちにしない。受信後の速やかな取得と、Webhook受信経路の停止も考慮した既知IDの照会を組み合わせる。pending件数だけでなく、終端を知っているのに結果未保存の件数・経過時間を監視する。

Webhookとpollingが同じ結果を持ち込む場合は、配信重複用の `webhook-id` と別に、自社job ID・response IDで取込を一意化する。通知を二度受けても業務の反映は一度にする。retrieveが取得不能なら、自社保存済み結果を先に照合し、未回収として記録する。取得不能だけから生成失敗・取消完了・特定の期限切れ原因を断定しない。

## 復旧台帳とstream cursorの確定順

以下はAPIの追加保証ではなく、本書の設計提案である。

| 台帳に保持する情報 | 復旧での用途 |
| --- | --- |
| 自社job IDと試行ID | 再取得と新しい生成試行を区別する |
| response IDと接続先projectの識別 | 異なるproject・異なるjobの応答を取り違えない |
| 実際のbackground・stream・store指定、確認した保持条件 | 再配信の可否と回収方法を選ぶ |
| 最後に保存したstatus、error、incomplete_details | 最後の観測と現在の実行結果を混同しない |
| 保存済み結果の参照と取込状態 | providerから消えても自社の取込成否を確認する |
| streamを使う場合のresponse ID別cursor | 処理済みeventの後から再接続する |

作成応答でIDを受け取ったら、利用者へ受付済みと返す前に台帳へ確定する。streamでは最初に得たResponse IDも保存対象とする。一方、POST送信後に応答そのものを失いID不明となったケースは、この台帳だけでは解決しない。作成がなかったとは断定せず `creation_unknown` などの自社状態で保留し、独自の照合・再投入判断へ分ける。今回の資料から作成POSTのexactly-onceや汎用の再送重複排除を導いていない。

stream eventを受け取っただけでcursorを先へ進めると、そのeventの保存前にprocessが落ちた際に取り落とす。eventを自社状態へ反映してからcursorを確定するか、両方を同じtransactionでcommitする。再配信され得る範囲はresponse IDとsequence番号で重複処理を抑える。異なるResponseのcursorを流用しない。

たとえばcursor=41の状態でevent 42を受け取り、保存前に停止したなら、復旧は41を使う。42を永続化済みでもcursor更新だけ失った場合は42が再処理対象となり得るため、反映側にも重複防止が要る。複数workerで処理を並列化する場合は、未処理のeventを飛び越えた最大番号をcursorにしない。番号が常に1ずつ連続する保証や、providerと自社DBの分散transactionは前提にしない。

初回を `stream=false` で作ったjobへ、後からstream再開を追加できるとは扱わない。そのjobはretrieveで回収し、次回以降に必要な作成optionを選ぶ。guideにはSDK再開例のコメントとreference表示の差もあるため、ここではHTTPの契約を採用し、SDK method名や最低対応版は保証しない。

## 終端・取消・削除の扱い

自社のpollerは `queued` / `in_progress` なら同じIDを照会し、間隔・jitter・全体の観測予算を持たせる。通信timeoutや自社workerの停止はResponseの終端として保存しない。観測期限に達したら未確認として引き継ぎ、再起動のたびに新規Responseを作らない。

- `completed`: 生成の終端を記録し、必要な型のoutputを抽出・保存・検証してから自社jobを成功にする。output配列先頭の存在や空でない文字列だけを取込完了条件にしない
- `failed`: errorを保存する。再試行の適否は原因と外部toolの副作用を確認して決める
- `incomplete`: incomplete_detailsと得られた部分結果を保存し、完全な成功回答へ昇格しない
- `cancelled`: 取消状態を保存する。応答取消を既に起きた外部操作のrollbackと扱わない
- status欠落・未知値: 解釈未確認として止め、pending以外だから成功とする分岐を避ける

これらは[Retrieve][retrieve]のfieldを使う独自のアプリケーション判断である。業務toolの完了まで必要な場合は別に確認する。

取消要求を送った後にHTTP応答を失った場合は、同じresponse IDに対する再取消・照会で確認する。[Background][background]の冪等性は取消操作についての契約で、新規生成POSTに拡張しない。返された最終statusを読み、cancelを呼んだという理由だけで台帳をcancelledへ上書きしない。完了との競合時の細かなHTTP応答・課金・tool停止伝播は未検証である。

[Cancel][cancel]と[Delete][delete]は別操作なので、生成停止、保存物の削除、自社copyの削除を別々に扱う。deleteを先に使って取消確認の代わりにしない。安全なcleanup時点、保存要件、関係する外部資源を確認してから削除する。

## 採用前の境界試験案

以下は実行済みのAPI試験ではない。mockと実環境で別々に検証するための受入れ条件である。

| 条件 | 確認する結果 |
| --- | --- |
| MAM backgroundでstore省略 | foregroundと同じ長期保持を仮定しない |
| ZDRでstore=trueを指定 | 明示trueだけを長期回収可能の根拠にしない |
| 一時保持対象のWebhookが遅延 | 通知到着を結果取得可能の証拠にせず、自社保存先も照合 |
| Webhookとpollingが競合 | 同じresponse IDの業務反映を重複させない |
| event 42の保存前に停止 | cursor 41から回収し、42を取り落とさない |
| event保存後・cursor確定前に停止 | 再配信による二重反映を抑止 |
| 初回stream=falseで切断 | stream再開へ切替えずretrieveで照会 |
| 作成POSTの応答喪失・ID不明 | creation_unknownを生成失敗に書き換えない |
| incompleteかつ部分textあり | 完全成功として公開しない |
| 取消応答喪失 | 同じIDを確認し、新しい生成を始めない |
| cancelの返却最終statusがcompleted | cancelledへ強制変更せず実際のstatusを記録 |
| statusに未知値または欠落 | 空の成功扱いにしない |
| retrieveが取得不能 | 未回収と処理失敗を区別し、無条件再投入を止める |

## provenance・未確認事項

6件の公式ページをnative webで開き、関連本文を確認した。共通言語版retrieve/cancel URLはサイズ上限で取得できなかったため、読めた公式Python referenceを出典に採用した。SDKコードの解析は行っていない。ページに固定版・公開日・再利用可能なオープンライセンスの記載を確認できず、licenseはunknownとした。本文は独自の要約・推論・設計案で、SDK sampleや実装コードを転載していない。repository分析・module昇格はない。

実API呼出し、ZDR/MAM projectの実設定、保持・消去時刻の計測、障害注入、cancel競合、stream再開の各SDK対応版、Webhook再送と一時保持の実挙動は未検証。検索evalは文書の発見性を確認するだけで、これらの実験成功を証明しない。全sourceはofficial_docsの90日TTLに合わせ、2027-01-02を再確認期限とする。

[background]: https://developers.openai.com/api/docs/guides/background
[retention]: https://developers.openai.com/api/docs/guides/your-data
[retrieve]: https://developers.openai.com/api/reference/python/resources/responses/methods/retrieve
[cancel]: https://developers.openai.com/api/reference/python/resources/responses/methods/cancel
[delete]: https://developers.openai.com/api/reference/resources/responses/methods/delete
[webhooks]: https://developers.openai.com/api/docs/guides/webhooks

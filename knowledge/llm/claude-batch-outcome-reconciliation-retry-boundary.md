---
{
  "id": "llm-claude-batch-outcome-reconciliation-retry-boundary",
  "title": "Claude Message Batches: 終了・個別成功・結果取込を分ける再送境界",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Claude API /v1/messages/batches; unversioned HTTP reference and guides retrieved 2026-10-03 UTC; examples use anthropic-version: 2023-06-01; live API and SDK behavior untested",
  "tags": [
    "research-domain:ai-engineering",
    "llm",
    "claude",
    "message-batches",
    "custom_id",
    "jsonl",
    "reconciliation",
    "partial-retry",
    "pause_turn",
    "cancellation"
  ],
  "sources": [
    {"id": "claude-batch-guide-20261003", "url": "https://platform.claude.com/docs/en/build-with-claude/batch-processing", "type": "official_docs"},
    {"id": "claude-batch-create-reference-20261003", "url": "https://platform.claude.com/docs/en/api/messages/batches/create", "type": "official_docs"},
    {"id": "claude-batch-results-reference-20261003", "url": "https://platform.claude.com/docs/en/api/messages/batches/results", "type": "official_docs"},
    {"id": "claude-batch-cancel-reference-20261003", "url": "https://platform.claude.com/docs/en/api/messages/batches/cancel", "type": "official_docs"},
    {"id": "claude-stop-reasons-batch-review-20261003", "url": "https://platform.claude.com/docs/en/build-with-claude/handling-stop-reasons", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/ai-engineering.json"]
}
---

# Claude Message Batches の結果照合と部分再送

## 問いと採用判断

夜間の大量抽出や評価で、batch が `ended` になったら全件を成功として公開してよいか。途中で cancel した、結果の download が切れた、一部の Message が `pause_turn` だった場合に、何を取り直し、何を新しく実行するべきか。

結論は、batch全体の終了、個別requestの結果、Messageの停止理由、アプリへの取込完了を別々に記録することである。とくに `custom_id` は結果との対応付けに使い、バッチをまたぐ冪等性の保証として扱わない。本書は最近の機能追加を主張するものではなく、既存のLLM知識で未収録だった非同期一括処理の回復判断を埋める。

ツールの権限・副作用の一般論は[既存の実行ループ](../agents/tool-permissions-failure-loop.md)、SDKによる先行tool実行は[eager streaming](../agents/claude-eager-tool-streaming-approval-retry-boundary.md)を参照する。本書の対象はClaude APIのMessage Batchesであり、他社のBatch APIやクラウド各社の一括推論へ同じ状態名を当てはめない。

## 確認した契約: batchの終了は各requestの成功ではない

[Batch guide](https://platform.claude.com/docs/en/build-with-claude/batch-processing)の作成・追跡・結果取得節では、作成直後は `in_progress`、全requestの処理が終わると `ended` になる。個別の `params` 検証も非同期で、検証失敗が全体終了後に返る。したがって作成受付を入力の検証合格に置き換えられない。

同guideの個別結果は次の4種類であり、1件の失敗が他のrequestの処理を止める契約ではない。

| `result.type` | guideで確認できた意味 |
|---|---|
| `succeeded` | Messageが作成された |
| `errored` | 入力不備・サーバーエラーなどでMessageが作成されなかった |
| `canceled` | そのrequestがモデルへ送られる前にbatchが取消された |
| `expired` | モデルへ送られる前に24時間の処理期限に達した |

`request_counts` はこれらの集計である。24時間以内の全件成功保証ではなく、期限切れの個別結果も起こり得る。また、結果のdownload期限は `ended_at` からではなく `created_at` から29日である。終了待ちだけを監視し、結果の回収を後回しにしない。

## 確認した契約: identityと取消を分ける

### custom_idは行番号でも全体的な重複排除キーでもない

[Create reference](https://platform.claude.com/docs/en/api/messages/batches/create)は `custom_id` をバッチ内で一意なdeveloper指定IDと定義し、1〜64文字、`^[a-zA-Z0-9_-]{1,64}$` を要求する。[Results reference](https://platform.claude.com/docs/en/api/messages/batches/results)は結果をJSONLで返し、1行は1requestの結果、順序は入力と同じとは限らないとする。

ここからの設計判断は、入力manifestと結果を `custom_id` でjoinし、入力の第n行と出力の第n行を結合しないことである。上記の定義は同じIDを別batchで使った場合の実行抑止を約束していない。`custom_id` の再利用を、create再送や複数batchにまたがるidempotencyの根拠にしない。「重複抑止が絶対に存在しない」という実測結果ではなく、確認した契約だけでは保証できないという意味である。

### cancelが受理されてもcanceled件数は0になり得る

[Cancel reference](https://platform.claude.com/docs/en/api/messages/batches/cancel)では、取消開始後に `canceling` へ移るが、中断できない実行中requestを完了させてから取消を確定し得る。全requestがその状況なら、取消操作をしても個別 `canceled` が0件という結果も許される。どのrequestが止まったかは個別結果で確認する。

したがって取消ボタンの成功応答を「何も実行されなかった」「結果を捨てて全件やり直せる」に変換しない。取消は既に作られたMessageや、アプリが既に取り込んだデータのrollback命令ではない、というのが本書の設計上の結論である。取消理由が利用者の中止なら、その後の自動再送も止める。期限切れ回復と利用者による中止を同じretryキューに入れない。

## 確認した契約: succeededでも会話は続き得る

[Stop reasons guide](https://platform.claude.com/docs/en/build-with-claude/handling-stop-reasons)は `stop_reason` を成功したMessage応答の一部とし、HTTP/APIエラーと区別する。以下は本書で必要な停止理由だけを抜き出したもので、将来の値を含む完全な一覧ではない。

| 停止理由 | 確認した意味 | 本書のアプリ側判定案 |
|---|---|---|
| `end_turn` | 自然に応答を終了した | 出力schemaや業務条件を別途検証する |
| `max_tokens` | 出力上限に達した | 未完の抽出結果を完成品として公開しない |
| `pause_turn` | server tool loopが反復上限に達した | assistant contentを保った続行候補にする |
| `tool_use` | client toolの処理待ち | 権限を確認したtool実行と結果返却へ進む |
| `refusal` | 応答を拒否した | 拒否として記録し、一般の一時障害と分ける |

`pause_turn` は応答contentをそのまま返して続行する経路であり、clientの `tool_use` 待ちは `tool_result` を返す別経路である。Batch guideのserver tools節は、batchでも `pause_turn` が起こり、batchまたは同期requestで続行できると明記する。一括処理なのでtool loopが必ず完走する、という保証はない。

設計上、Message作成の成功率と業務完了率を分ける。`succeeded` と `pause_turn` の組合せを `errored` に書き換える必要はないが、文書抽出・評価taskの完了にも数えない。続行は失敗requestの同一入力再送とは異なり、履歴を進めた新しいrequestである。続行回数・token・時間の予算を独立に持つ。

## 独自の設計案: manifestと結果台帳で回復する

以下は上記契約を使った本書の提案であり、Anthropicが提供するtransaction、保存形式、exactly-once保証ではない。

### 1. create前に入力集合を固定する

業務上の `job_id` とその入力版、model識別子、prompt/schemaの版、入力digest、試行番号、予定する `custom_id` の集合をmanifestへ保存する。同じ `job_id` でも入力内容が変われば別の入力版として扱う。個人情報や長いファイル名をIDへ直接埋め込まず、内部キーと対応させる。

createの結果を受け取ったら、providerの `batch_id`、Workspace、`created_at` とmanifestを紐付ける。応答前に通信が切れ、受付結果が不明なcreateは `submission_unknown` などの別状態にする。入力集合を送った事実だけで失敗扱いして再送しない。確認済みbatchの探索・人手照合など、配備環境で使える回復手順を決めてから再実行を許可する。今回の出典は、createの成否不明を解く一意な検索キーやcross-batchの重複抑止を保証していない。

### 2. provider結果を保存してから業務反映する

本書では `(Workspace, batch_id, custom_id)` をprovider結果の取込キー、`(job_id, input_version)` を業務結果の採用キーとして分ける。試行番号と親requestの対応も残し、再実行で前の失敗・費用・採用理由を上書きしない。

JSONLを1行ずつ読み、構文、既知のID、重複、結果型を確認して保存する。同じ取込キーの同一内容は再取込時に業務反映を重複させない。同じキーに異なる内容が現れたら、後勝ちで上書きせず整合性異常として保留する。途中で切れた最後の行を、存在する `custom_id` だけ拾って成功扱いしない。

結果downloadの通信失敗は推論失敗ではない。既知の同じbatchから結果を再取得し、取込台帳との照合を再開する。JSONLの再取得だけのために新しいbatchを作らない。ここではHTTP Rangeやbyte offsetからの再開を仮定しておらず、全体を再読しても取込が重複しない設計にする。

### 3. 集合と件数の両方を照合する

manifestにある入力ID集合を `E`、構文・型を検査して取り込めた一意な結果ID集合を `R` とする。batch終了後に、`E = R`、重複IDなし、想定外IDなし、個別結果型の件数が `request_counts` と一致することを確認する。件数だけでは同数の欠落と混入が相殺されるため、件数一致だけを完全性の条件にしない。

一部しか読めなかった間の `E - R` は結果未回収であり、`errored` や `expired` の集合ではない。provider終端と取込照合が揃う前に、欠けたIDを推論の部分再送へ回さない。未知の結果型も成功に落とさず、互換性の確認待ちにする。

業務結果を別DBへ反映するなら、取込台帳と採用キーの一意性を使う。外部副作用を伴う場合はその実行先での冪等性・結果照合がさらに必要であり、この台帳だけでexactly-onceにはならない。

### 4. 再取得・再送・続行・停止を分類する

| 観測した状況 | 提案する次の操作 |
|---|---|
| download切断、JSONL末尾欠け、ID未回収 | 同じbatchの結果を再取得し、集合照合を完了する |
| `errored` + `invalid_request_error` | 入力不備を直してから、新しい試行へ紐付ける |
| 原因を確認した一時的なサーバーエラー | 回数・費用上限内で対象IDだけ再送候補にする |
| `expired` | 締切と残予算を確認し、必要なら対象IDだけ再送する |
| 利用者中止による `canceled` | 再送しない。再開の意思が得られた場合だけ新しい試行へ進む |
| `succeeded` + `pause_turn` | 保存済みassistant contentを使って続行し、親子関係を記録する |
| `succeeded` + `tool_use` | tool権限・実行状態を確認し、対応する結果を返す |
| create応答が失われ受付成否不明 | `submission_unknown` の照合を行い、無条件の全件再送を止める |

入力エラーは修正が必要という分類はBatch guideの例とも一致するが、retry予算、対象の選定、取込キーは本書の設計である。予期しないエラーをすべて一時障害とみなさない。利用者の中止やpolicy上の拒否を、成功するまで回す再試行で打ち消さない。

## 受入試験案と観測点

以下は実施済みAPI試験ではなく、adapter導入時のテスト案である。

| 条件 | 期待する判定 |
|---|---|
| 3件が `succeeded`・`errored`・`expired` で `ended` | batch終了を表示し、全件成功とはしない |
| 入力順A/B、結果順B/A | `custom_id` で正しい入力へ紐付ける |
| 10件中9件までdownload後に切断 | 1件を推論失敗へ変換せず、結果再取得後も業務反映は10件分だけ |
| 入力A/B、結果A/Cで総数2件 | 件数一致でも集合不一致として公開を保留する |
| 同じ結果行を2回取り込む | 重複の記録・検出を行い、業務反映を繰り返さない |
| cancel受理後、全件が `succeeded` | `canceled=0` を仕様違反とはせず、実際の結果を照合する |
| `succeeded` + `pause_turn` | Message成功・業務未完として続行経路へ入れる |
| 利用者中止と処理期限切れが同じbatchに存在 | 取消理由を確認し、同じ自動retry条件を適用しない |
| 受付成否不明のcreate | 新規batchの作成で成否不明を隠さない |
| `created_at` + 29日を超えてから回収開始 | 結果保存に成功したことにせず、回収期限超過を報告する |

監視はproviderの `processing_status`、結果型別件数、未回収ID数、取込整合性エラー、業務未完件数、再送・続行回数を分ける。既に成功した生成をdownload失敗のたびにやり直す構成や、再送後の成功だけ残す評価は、費用と成功率の両方を歪める。

## 版・provenance・未確認事項

- 2026-10-03 UTCに5件の公式ページ本文をnative webで開いて照合した。いずれも固定release版の文書ではなく、ページ固有の公開・更新日表示は取得本文で確認できなかった。HTTP例の `anthropic-version: 2023-06-01` はAPIヘッダー値であり、本機能の公開日ではない。
- 公式ページ本文のオープンライセンスは確認できず、catalogは `unknown` とした。外部のコード・長い引用は取り込まず、API識別子を使った独自の要約と設計案だけを記載した。repository実装の分析・SDK版固定はしていない。
- 生APIの作成・取消・期限切れ・JSONL回収、SDKの自動retry、サーバーツールの実行は試していない。取消確定までの所要時間、createの成否不明回復、各SDKの未知enum対応、実環境での保存・削除要件は配備前に検証する。
- `pause_turn` の正確なbatch反復上限は本書の保証対象外。一般Messagesの上限値をそのままbatchへ移植しない。出力上限・モデル別対応・価格の一覧も本書の対象外である。
- 検索evalは本書の発見可能性を検査する。APIの動作・費用・業務transactionの安全性を証明するものではない。公式文書のTTL 90日で再確認し、後日のAPIやSDKへ無条件に適用しない。

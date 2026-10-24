---
{
  "id": "llm-gemini-argon-long-decode-usage-context-boundary",
  "title": "Gemini 4 Argon long decode: hop累積usage・コンテキスト残量・回答回収の境界",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Gemini API gemini-4-argon (GA 2026-10-13); v1beta Interactions background long decode and GenerateContent comparison; Google GenAI automatic-continuation minimum Python 2.30.1 / JavaScript 2.29.1 per docs; rolling docs verified 2026-10-24 UTC; no live API execution",
  "tags": [
    "research-domain:ai-engineering", "llm", "Gemini", "Argon", "long-decode",
    "hop", "usage", "token-budget", "initial_prompt_token_count",
    "total_input_tokens", "automatic_continuation", "model_output"
  ],
  "sources": [
    {"id": "google-argon-release-20261024", "url": "https://ai.google.dev/gemini-api/docs/changelog", "type": "release_notes"},
    {"id": "google-argon-model-window-20261024", "url": "https://ai.google.dev/gemini-api/docs/models/gemini-4-argon", "type": "official_docs"},
    {"id": "google-argon-migration-budget-20261024", "url": "https://ai.google.dev/gemini-api/docs/latest-model", "type": "official_docs"},
    {"id": "google-argon-interactions-long-decode-20261024", "url": "https://ai.google.dev/gemini-api/docs/long-decode", "type": "official_docs"},
    {"id": "google-argon-generatecontent-long-decode-20261024", "url": "https://ai.google.dev/gemini-api/docs/generate-content/long-decode", "type": "official_docs"},
    {"id": "google-argon-background-storage-20261024", "url": "https://ai.google.dev/gemini-api/docs/background-execution", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/ai-engineering.json"]
}
---

# Gemini 4 Argon long decodeのusageをcontext量へ誤変換しない

## 問いと新しい適用範囲

長い一回答を生成したとき、usageの入力token数が実際のcontextより大きくなった。会話を圧縮すべきか、課金集計が二重なのか、それとも正常なのか。本書はこの判定に必要な**計数範囲の区別**を扱う。

[Release notesの2026-10-13項目][release]は `gemini-4-argon` のGAを告知した。[モデルページ][model]はinput/output共通の1,048,576-token windowと、通常の65,536-token出力、long decodeによる拡張を区別する。1Mの入力枠に、独立した1Mの出力枠が追加されたわけではない。

対象はGemini APIの同モデルに対するlong decode。Antigravity agentのtool ABIやIDE設定、一般的な多turn agentのretry設計は対象外である。既存の[長時間loopのcompaction](../agents/long-loop-context-growth-compaction.md)が「何をいつ削るか」を扱うのに対し、本書は**削る判断へ渡してよいtoken量は何か**を扱う。[OpenAI backgroundの保持・回収](openai-background-retention-recovery-boundary.md)の契約をGeminiへ流用しない。

## 一次資料で確認した契約

### 1. 上限を上げることとthinkingを調整することは違う

[Argon移行ガイド][migration]は `max_output_tokens` がthinkingと回答の合計を制限し、`thinking_level` は推論の深さを制御すると説明する。levelは `low` / `medium` / `high`、既定はmedium。小さい上限で止めても、同じ推論が短く圧縮される保証ではない。

Interactionsでは65,536を超える出力に `background: true` と、それを超える明示的な `max_output_tokens` が必要である。backgroundだけ指定しても既定の65,536に留まる。上限は入力を差し引いた共有window内で選ぶ。[Background guide][background]は `background: false` で65,536超を指定すると400、backgroundと `store: false` の組合せも400とする。保存を許容しない経路を、待ち時間対策だけのつもりでbackgroundへ切り替えない。

### 2. Interactionsは一つのIDの中で複数hopを集計する

[Interactions long decode][interactions]の契約を要約する。

- serverが最大64K-tokenのhopを継続し、同じinteraction IDの `steps` に各 `model_output` を追加する。最終回答は全該当stepのtextを順に合わせる
- `usage.total_input_tokens` は各hopの入力の合計。後続hopは元promptとそれまでの生成を再入力するため、同じtokenが再計数される
- context量の概算は元prompt量と `total_output_tokens`、`total_thought_tokens` の和であり、累積inputではない
- streamの `step.stop` / `interaction.status_update` に現れるusageもinteraction全体の累積値。最新値を使い、通知ごとに足さない
- `incomplete` はtoken上限到達で、thinkingだけに使い切るとtextが空の場合もある。`completed` とは区別する

同guideの課金説明では、thinking-onlyのhopも対象となり、繰り返す入力はimplicit cacheの割引対象になる。これはcontextから該当tokenを除去する機能ではない。

### 3. GenerateContentではSDKとRESTで集計単位が異なる

[GenerateContent long decode][generate]は、自動継続の最低版をPython `google-genai` 2.30.1、JavaScript `@google/genai` 2.29.1とする。対応SDKのautomatic continuationは既定で有効だが、旧SDKやOpenAI互換endpointへ同じ前提を広げない。

usageはSDKの自動継続なら全hop合計、manual/RESTなら応答したhop単位。manual stream内では累積なので、各streamの最後のusageを採り、それらをhop間で加算する。自動継続の合計値をさらにhop数倍しない。

元prompt量は自動継続の `initial_prompt_token_count`、manual/RESTでは最初のhopの `prompt_token_count` で得る。前者はSDKがclient側で計算し、REST応答や `model_dump()` 等のSDK JSONには含まれないため、後から必要なら別途保存する。単一hopでは通常のprompt countと同じ値になり、短い試験だけでは差を発見できない。

### 4. 一般的なfield説明だけで複数hopの意味を上書きしない

API名、SDKの自動継続設定、stream内かhop間かを外すと、同じ「input tokens」でも意味が変わる。InteractionsとGenerateContentではfield名も異なる。これは本書の読み取り上の注意であり、両APIのusage objectが同じschemaという主張ではない。

[Background guide][background]は `requires_action` をclient入力待ちとして区別する。出力のある最終stepが見つかっても、interactionの成功を判定したことにはならない。またlong decode guideのpolling例がloopを抜けたことを、アプリケーションの成功条件に置き換えない。

## 独自の設計案: 計数を四つの台帳へ分ける

以下は公式の保存schemaや運用保証ではなく、本書の提案である。

| 記録 | 意味・用途 | 混ぜてはいけないもの |
|---|---|---|
| 元prompt量 P | その生成を開始した入力の概算token数と算出方法 | 後続hopで再計数された入力の累計 |
| 生成量 O + T | 回答とthinkingのtoken数。出力上限の消費 | 表示textの文字数、textのないhopを0とする集計 |
| usage snapshot U | provider/API経路ごとの請求照合用観測値 | context占有量、一回の差分と累積snapshot |
| 回収済み成果物 A | 保存・検査した各stepと最終出力の版 | stream表示済みの断片、最後のhopだけのtext |

Pには計数した入力版、model ID、API経路、SDK版、自動継続の有無を結び付ける。Uには「interaction累積」「SDK全hop合計」「manual hop合計」のどれかを明示する。単なる `tokens` という一fieldへ正規化すると、compactionと利用量報告の両方が同じ誤りを参照してしまう。

### contextの入口判定

この文書の単純なtext生成例では、共有windowをW、許容するthinkingと出力の合計をBとすると、入口の概算は `P + B <= W`。Wは本モデルで1,048,576である。安全余裕やtool定義など自社adapterが実際に渡す入力を別途考慮し、満杯まで使えることを保証する式にはしない。

生成中・生成後の概算は `P + O + T` として管理し、課金用の累積inputを代入しない。cache-hit tokenをPから引くことも避ける。価格の割引とwindowの空きは異なる軸である。多turn履歴、tool、multimodal入力を含む完全なtokenizer再現は本書では検証していないため、この単純式を全requestの厳密な受理判定に使わない。

Pが保存されていない復元ログは、累積inputをPと見なして埋めない。記録を「context計数不能」とし、必要なら元requestを対応する計数経路で再評価する。欠落を0として巨大な空きがあるように表示する処理も避ける。

### 独自の数値例: 正常な再計数を二重請求と誤判定しない

以下はAPI実測でも出典の例の転載でもない。元promptを12,000 tokens、3hopの生成量を65,536、65,536、10,000 tokensと仮定した計算例である。各生成量はthinkingと回答の合計とする。

| hop | そのhopへ渡る入力 | 今回の生成 |
|---|---:|---:|
| 1 | 12,000 | 65,536 |
| 2 | 77,536 | 65,536 |
| 3 | 143,072 | 10,000 |
| 合計 | 232,608 | 141,072 |

この仮定で最終contextは `12,000 + 141,072 = 153,072` tokens、入力usageは232,608 tokensになる。入力usageの方が大きくても、それだけでは障害ではない。独自のcompaction閾値を200,000とした場合、誤って232,608を渡すと発火するが、この例のcontext概算153,072では発火しない。200,000は説明用の自社閾値で、Googleの推奨値ではない。

これと別に、streamから累積inputが12,000、89,536、232,608と順に届いたfixtureを考える。最終値は232,608であって、3値の和334,144ではない。前者はhopによる正当な再計数、後者は累積snapshotを加算した自社実装の二重計数である。cache割引・単価・他の課金要素を含む実際の請求額は、このtoken表だけでは計算していない。

## 独自の設計案: 回収と評価を同じ予算条件で比較する

1. **経路を固定する。** Interactions背景実行、GenerateContent自動継続、manual RESTを一つの既定adapterとして隠さず、別の契約として識別する。モデル文字列だけ変更したA/B試験で、片側だけ自動継続・上限増加が有効にならないようにする。
2. **元prompt量を先に永続化する。** SDK objectをJSON化してから必要fieldを探すのでは遅い。`initial_prompt_token_count` と計数方法を独立して保存し、JSON round-trip後にも復元できるか試験する。秘密情報やprompt本文を追加保存することは、このメトリクス保存から自動的には認可されない。
3. **累積usageは置換する。** 同一interaction内では最新snapshotを原記録と結び付けて保存する。manual経路はstream内の最終値を確定してからhop間で集計する。接続retry回数をhop数と見なさず、再受信を新しい利用量として加算しない。
4. **全stepから回答を回収する。** Interactionsの元 `steps` とその順序を保存し、`model_output` 内のtextを抽出する。最後のstepだけ、最初のtextだけ、全種類のcontentを無差別連結する方式を避ける。SDKの便利なtext propertyが全hopを含むかは、利用版のfixtureで確認する。
5. **token切れをモデル不正解と分解する。** `incomplete` や空textを、無条件に「モデルが答えられない」へ分類しない。一方、得られた部分回答を完全成功へ昇格させない。taskの採点とは別に、上限、thinking設定、終了状態、回収件数を残す。
6. **再試験は別trialにする。** 上限を上げた再実行は追加予算を使う別条件であり、元試行を上書きしない。API経路によって継続・再実行の方法が違うため、GenerateContentのcontinuation tokenをInteractionsの再開IDへ読み替えない。外部toolの作用を伴う場合は別途再送安全性を検証する。

streamが再接続した際の表示、event保存、重複除去も必要になるが、本書の計数案はexactly-once配送や課金の再送保証を新たに与えるものではない。ここでの完了条件は「成功状態を確認し、全hopの対象出力を保存・検査し、その生成と同じ計数範囲のusageを対応付けたこと」である。

## 採用前のfixture案

以下は将来のadapter試験案であり、本調査で実APIに対して実行した結果ではない。

| fixture | 確認すること |
|---|---|
| background=true、出力上限は省略 | backgroundだけで64K超が有効になったと表示しない |
| background=false、65,536超の上限 | 契約不一致を入口で検出し、保存ありへ黙ってfallbackしない |
| 3hop数値例 | context153,072と入力usage232,608を別々に再現 |
| 累積snapshotが12,000→89,536→232,608 | 最終232,608を採用し、334,144にしない |
| SDK合計と同内容のmanual hop列 | 経路別の集計規則で一致し、SDK合計を再加算しない |
| 単一hopのみの試験 | 元prompt量と累積量の違いを検査したことにしない |
| SDK objectをmodel_dump()して復元 | 別保存したinitial_prompt_token_countが復元できる |
| 元prompt量なしの過去ログ | context計数不能を明示し、0や累積inputで埋めない |
| 3つのmodel_outputと他種類stepが混在 | 順序を保って全対象textを回収し、他種類を回答へ混ぜない |
| textなし・thinkingあり・incomplete | token利用を0にせず、完全成功にも分類しない |
| requires_actionでtextあり | client入力待ちを成功回答と区別する |
| 上限を増やした再実行だけ成功 | 元trialと条件差・追加利用を保持する |

## 版・provenance・未確認事項

- 6件のGoogle公式ページをnative webで2026-10-24 UTCに開いた。GA告知は2026-10-13。表示更新日はchangelog/modelが10月14日、移行ガイドが10月16日、両long decode guideが10月13日、backgroundが10月21日。ページ更新日を機能導入日と同一視しない。固定revisionがないrolling文書で、URL内の `latest-model` も取得時点のArgonを指す
- footerは文書CC-BY-4.0、sample code Apache-2.0とする。本書は出典を付した独自要約・設計・計算例で、sample codeをコピーしていない。repository解析やmodule昇格はない。`trust: primary-source` は独自の提案を含むため
- **文書の不一致**: GenerateContent guideのDefault output limitsは、初回non-streaming RESTで `max_output_tokens > 32768` かつcontinuation tokenなしを400と説明する一方、同ページのmanual REST例は初回に262,144を指定する。今回どちらの挙動か実証していない。動作確認済みのREST手順として転載せず、導入時に選ぶSDK/endpointで確かめる
- SDK最低版は公式guideの記載であり、SDK実装をcommit固定して解析した結果ではない。自動継続の全言語互換、SDK内部のretry/serialization、continuation tokenの期限、実請求、cache-hit率、長時間stream、上限境界、multimodal/tool込みのcontext量は未検証
- 生成上限を増やしたときの品質改善や完了時間は測定していない。long decodeが任意の文字数の回答を保証するとも主張しない。金額見積りは料金表を別に確認する。token数の計算例はAPIや請求を実行した証拠ではない
- 最短TTLはrelease_notesの30日なので再確認期限は2026-11-23。検索evalは文書の発見性だけを確認し、上記fixtureやlive APIの合格を意味しない

[release]: https://ai.google.dev/gemini-api/docs/changelog
[model]: https://ai.google.dev/gemini-api/docs/models/gemini-4-argon
[migration]: https://ai.google.dev/gemini-api/docs/latest-model
[interactions]: https://ai.google.dev/gemini-api/docs/long-decode
[generate]: https://ai.google.dev/gemini-api/docs/generate-content/long-decode
[background]: https://ai.google.dev/gemini-api/docs/background-execution

---
{
  "id": "agents-openai-multi-agent-response-completion-boundary",
  "title": "OpenAI Responses Multi-agent beta: response完了・tool結果注入・root最終回答の判定境界",
  "kind": "knowledge",
  "technology": "agents",
  "version": "Responses API beta responses_multi_agent=v1; GPT-6.1 Sol support announced 2026-09-29; rolling guide / beta create reference checked 2026-10-04 UTC; SDK version and live behavior untested",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "openai",
    "responses-api",
    "multi-agent",
    "response.inject",
    "function_call_output",
    "final_answer",
    "completion",
    "continuation"
  ],
  "sources": [
    {
      "id": "openai-multi-agent-support-changelog-20261004",
      "url": "https://developers.openai.com/api/docs/changelog",
      "type": "release_notes"
    },
    {
      "id": "openai-multi-agent-lifecycle-guide-20261004",
      "url": "https://developers.openai.com/api/docs/guides/responses-multi-agent",
      "type": "official_docs"
    },
    {
      "id": "openai-multi-agent-beta-create-schema-20261004",
      "url": "https://developers.openai.com/api/reference/resources/beta/subresources/responses/methods/create",
      "type": "official_docs"
    },
    {
      "id": "openai-multi-agent-deployment-phase-20261004",
      "url": "https://developers.openai.com/api/docs/guides/deployment-checklist",
      "type": "official_docs"
    },
    {
      "id": "openai-multi-agent-model-sol61-20261004",
      "url": "https://developers.openai.com/api/docs/models/gpt-6.1-sol",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# Responses Multi-agent の「終わった」を取り違えない

## 問いと採用判断

複数agentがtoolを使う処理で、`response.completed` を受け取ったら接続を閉じ、最終回答を利用者へ公開してよいか。

**一つのResponseの終了、toolの実行完了、結果注入の受領、業務全体の完了を分けて判定する。** HTTPではclient function待ちでもResponseが終了する。WebSocketでは終了通知を見た後にも注入のackを回収する必要がある。これを同じ完了flagで扱うと、必要なtool結果を捨てたり、既に成功した外部操作を再実行したりする。[Multi-agent guide][multi]

本書は、このbeta APIを既存の単一agent用tool loopへ導入するときの終了・継続判断を扱う。[一般的なtool権限と失敗処理](tool-permissions-failure-loop.md)や[Claude SDKのeager実行](claude-eager-tool-streaming-approval-retry-boundary.md)の分割ではなく、Responses固有のhosted協調item、複数agentを含むResponse、結果注入の競合が対象である。Agents APIの永続sessionやAgents SDKのrunnerへ同じ契約を移植しない。

## 適用版と新しい対応範囲

- [公式changelog][release]の **2026-09-29** の項目は、GPT-6.1 Sol公開とResponses APIでのMulti-agent beta対応を告知する。この日は同モデルの対応告知日であり、Multi-agent機能全体の初公開日や、以下の全fieldの導入日ではない
- 2026-10-04 UTCに開いたguideは、`multi_agent.enabled=true` とbeta識別子 **responses_multi_agent=v1** を使う。raw HTTP・WebSocketでは `OpenAI-Beta` headerへ指定する。HTTPのPython/JavaScript例はbeta Responses SDKと `betas` 引数を使うが、WebSocket connectorには同じ引数を流用せずheaderを渡す
- 対応例は **gpt-6.1-sol** に限定する。[モデルページ][model]もtool callingはResponses APIを使うとしている。guideが挙げるGPT-5.6系や、別guideの広いGPT-6表記から、任意のモデル・tier・projectで使えるとは推測しない
- guideとreferenceはrolling docsで固定revision・独立した公開日は未表示。betaのitem schemaは変更され得る。SDKの最低版やインストール済み版の対応は今回確認していない

## まず四つの観測を別々に残す

[beta create reference][schema]はResponse statusとして `completed` / `failed` / `in_progress` / `cancelled` / `queued` / `incomplete` を列挙する。一方、guideの `response.created`・`response.completed` はResponse全体のイベントであり、特定agentに属するイベントではない。

| 観測 | 確認できること | そこから決めつけないこと |
|---|---|---|
| Responseのstatus・lifecycle event | そのResponseの生成状態 | 全subagentの業務成功、外部操作のrollback、全結果の取込 |
| agentのitem・協調actionの結果 | 発言や協調処理の記録 | 一つのsubagentの報告だけによるrootの最終判断 |
| 自社tool executorの完了記録 | client functionで観測した実行結果 | その結果をモデルが受領したこと |
| `response.inject.created` / `response.inject.failed` | 注入が受理されたか、受理されなかったか | 前者による業務成功、後者によるtool実行失敗 |

この表の右列は公式が定める新しいstatusではなく、過剰な解釈を避ける本書の判断である。subagentの全status enum、agent tree全体の完了を外部から一度で保証するAPIは今回確認していない。`list_agents` はモデルが使うhosted協調actionとして説明されており、アプリから任意に実行できる別endpointを仮定しない。

## HTTP: response終了時に残るfunction callを集める

guideでは、HTTPのResponseは**全active agentが終了したか、client側functionの結果待ちで停止した時点**で完了する。rootだけでなくsubagentからも `function_call` が出る。アプリは未処理のcallを扱い、対応する `call_id` を持つ `function_call_output` を次のResponses requestへ渡して継続する。[multi]

実装上は、`response.completed` を「このrequestを読み終える境界」と「業務を公開して終了する境界」に分ける。たとえば子Aが検索tool待ち、子Bが照合tool待ちでResponseが終わった場合、Aだけを実行して最終回答を採用してはいけない。未処理callの集合を確認し、実行が許可されたcallの結果を揃えて継続する。認可されないcallを無条件に実行する意味ではなく、拒否や保留も自社の実行規則で扱う。

ここではitemの種別を最初に分岐する。

- `function_call`: clientが実行を担当する。rootとsubagentを同じ受付経路で扱う
- `multi_agent_call`: serverが処理する協調action。client executorへ渡さず、結果を捏造して返さない
- `multi_agent_call_output`: hosted actionの結果。元のcallと `call_id` で対応する
- `agent_message`: agent間の情報。表示用assistant messageやclient functionへ変換しない

後三種はbetaで追加されるitemであり、一般的なtool名だけを見るdispatcherでは取り違えやすい。`spawn_agent` という文字列があるから自社processを起動する、といった処理を追加しない。guideはhosted callとそのoutputをreplay・traceに必要なら保持するよう案内する。HTTP例もoutput itemsを履歴へ戻しており、表示textだけから履歴を作り直す構成ではない。

これは「全outputを無期限保存せよ」という要件ではない。継続用に必要なbeta item・識別子・phaseと、利用者へ見せる内容を別の表現として管理し、自社の保持・アクセス方針の下で扱う設計が必要になる。

## WebSocket: 終了通知の後にも注入ackを回収する

### 確認した送信・受領の契約

guideは、`response.created` で得たResponse IDを保存し、そのIDを `response.inject.response_id` に指定してclient functionの結果を注入すると説明する。`input` には、元の `call_id` に対応する結果itemを入れる。

正常なschemaの注入には `response.inject.created` または `response.inject.failed` が返る。前者は入力を検証して受け入れたという意味である。**Responseの完了と、送信した全注入のack確定の両方**を確認するまで読み取りを続ける。[multi]

| 注入で観測した結果 | guideで示される扱い | 自社実装で避ける誤判定 |
|---|---|---|
| `response.inject.created` | active Responseへ結果が追加された。読み取り継続 | 「tool結果を受領した」だけで業務完了へ進む |
| `response.inject.failed` + `response_already_completed` | failure eventの `input` を、完了済みResponseから続く新しい `response.create` へ渡す | toolそのものをもう一度実行する |
| `response.inject.failed` + `response_not_found` | `response.created` から得たIDを使っているか確認 | toolが失敗した、またはResponseは削除済み、と断定する |
| schema不適合によるgeneric 400 | WebSocketは閉じられる。requestを直して新規接続する | 同じ不正eventをそのまま再送し続ける |

`response_already_completed` の継続例は `previous_response_id` に完了したIDを設定する。戻すのはfailure eventが示す**既に得られたinput**であり、元toolの再実行ではない。これはモデルへの結果配送を続ける手順であり、外部書込の冪等性保証ではない。

### 具体例: toolは成功したが注入が間に合わない

以下は文書化された分岐を組み合わせた設計例であり、実APIで再現した試験結果ではない。

1. Response R1のsubagentがcall C1を要求し、自社toolが業務operation O1を完了する
2. アプリはC1の結果を保存し、R1へ注入する
3. `response.completed` が先に届く。アプリは注入ackが未確定なので読み取りをやめない
4. `response.inject.failed` が `response_already_completed` とC1の結果inputを返す
5. R1からの継続request R2にそのinputを渡し、O1は再実行しない
6. R2の結果と残作業を確認してから、業務の完了を判定する

3でsocketを閉じると4の継続材料を捨て得る。4を「関数が失敗した」と解釈してC1を再実行すると、副作用を二重に発生させ得る。したがって実行結果の保存と結果配送のretryを同じ処理にしない。

送信後・ack受信前に切断した場合は別の不確定状態である。今回の資料から、任意の注入のexactly-once、再注入の包括的な冪等保証、失われたackを照会する専用手段は確定できない。`delivery_unknown` などの自社状態を残し、保存済み結果と利用可能なprovider状態を照合する。単に「ackを見ていない」だけでtoolをやり直さず、照合手段が足りない場合は回復判断を保留する。

## 最終回答: rootのmessageとphaseを確認する

[Deployment checklist][checklist]と[beta schema][schema]では、assistant messageの `phase` は途中の `commentary` と `final_answer` を区別する。継続requestではこのfieldを保持する。

guideの完成回答抽出例は、`type=message`、`agent.agent_name=/root`、`phase=final_answer` を揃えたうえで、そのcontentの `output_text` を取り出す。これはrootが統合した回答を選ぶ例であり、`output` 内の全textや、最後に届いたsubagentの文章を最終回答とする例ではない。[multi]

本書の設計判断として、表示用の暫定deltaと公開対象の完成messageを別に持つ。途中のroot commentaryが利用者に見えても、それだけで処理を閉じない。想定するroot finalが欠ける場合も空文字を成功として返さず、未処理call、Response終端、refusal等を調べる。referenceのmessage contentはtext以外にrefusalも持つため、textだけを抽出して拒否を見失わない。

beta schemaでは `agent`・`phase` はoptionalである。値が欠けたitemを自動でroot finalに昇格させるのは本書の採用条件にしない。公式のstreaming表示例にあるrootへのfallbackを、出所確認や権限判定の保証へ読み替えない。UIのprogress表示と、完成物の採用規則は別途決める。

### agent_messageの帰属にある文書上の差

2026-10-04に観察した差として、guideは `agent_message` の `agent.agent_name` を**受信者**と説明し、例でも `author=/root/agent_a`、`recipient=/root`、`agent.agent_name=/root` としている。一方、同日取得したbeta create referenceのAgentMessageでも、`agent` の共通説明はitemを生成したagentという表現になっていた。[multi] / [schema]

これは取得した二つの文書の説明差であり、本番がどちらの挙動をするか実測したという意味ではない。メッセージの方向は専用の `author` / `recipient` を保持して追跡し、すべてのitemで `agent` が送信者を意味すると仮定しない。矛盾した値を黙って正規化するより、元fieldを残してadapterの互換性確認対象にする。

## 独自の設計案: 一つのrunを複数Responseとして管理する

以下はOpenAIの指定database schemaではなく、上記契約を安全に扱うための案である。

- **業務run IDとResponse IDを分ける。** 一つの業務runがR1、R2へ続く関係を保存する。Response IDごとにproviderの終端と未確定の結果配送を記録し、R1終了でrunを閉じない
- **実行台帳と配送台帳を分ける。** callの出所、`call_id`、許可した対象、実行済み結果、送信先Response、注入の確定結果を対応付ける。外部operationの重複抑止キーはcall IDだけから勝手に保証せず、対象サービスの契約に合わせる
- **全agentからのclient callを同じgateへ通す。** guideでは全agentがrequestに設定されたtoolsを利用できる。subagent化を権限分離とみなさず、実行側で利用者・tenant・対象を検証する。共有資源への書込は、必要ならアプリ側で直列化する
- **公開前の条件を明文化する。** 必要なclient callの扱いが確定し、送信した注入の結果が判定され、継続待ちがなく、rootの回答と業務の受入れ条件が揃ったことを確認する。注入失敗がすべて解消しただけでも、回答品質の合格にはしない
- **互換性エラーを成功へ丸めない。** 未知itemや欠落fieldはraw形式を必要な範囲で保持し、decoder・SDK・beta識別子の対応を確認する。イベントを無言で捨て、見えているtextだけを完成回答にしない

監視対象はResponse時間だけでなく、tool実行終了から結果受領までの遅延、未確定注入数、`response_already_completed` による継続回数、root final欠落件数を分ける。ログには必要な識別子と状態を残し、tool本文・秘密情報を無差別に複製しない。

## 終了規則に影響するbetaの制限

[beta reference][schema]の `max_concurrent_subagents` は最小1、既定3で、rootを除く全子孫の同時active数に適用される。総subagent生成数や木の深さには固定上限を設けないと記される。したがって **同時数3は総数3でも費用上限でもない**。自社の許可するtool実行量、時間、継続回数、業務予算は別に設計する。ただし独自budgetを置くだけで、既に走るprovider推論や外部操作の即時停止まで保証できるとはしない。

[Deployment checklist][checklist]が挙げる非対応項目は `/responses/compact`、`reasoning.summary`、`max_tool_calls`。rootと各subagentにはserver側の自動compactionが適用される。既存の単一agent向け `max_tool_calls` や手動compact endpointを、そのままこのrunの停止保証として使わない。自社の実行・承認・配送台帳をモデルの要約だけに依存させない理由にもなる。

## 導入時の受入れ試験案

以下は未実施のアプリ試験案である。本KBの検索evalと混同しない。

| 入力・障害 | 期待する自社側の判定 |
|---|---|
| HTTP終了時にrootと子のfunction callが残る | rootだけでなく全未処理callを照合し、必要な結果を継続requestへ渡す |
| `multi_agent_call` のactionがspawn_agent | client executorを呼ばず、hosted結果を保持する |
| tool成功後に `response_already_completed` | 既存inputで継続し、外部operationを二重実行しない |
| Response終了後も注入ackが未着 | 読み取りを継続し、run完了へ進めない |
| 注入送信後に切断しackを観測できない | `delivery_unknown` を残し、tool失敗へ変換しない |
| 子のfinal_answerだけがある | rootの最終回答が揃ったとは扱わない |
| rootのcommentaryだけがある | phaseを保持し、完成物として公開しない |
| `agent` または `phase` が欠ける | 自動でroot finalへ昇格させない |
| agent_messageのauthorとagent名が異なる | 専用author/recipientで方向を表示し、説明差の確認対象にする |
| 同時数3のまま子が順次入れ替わる | 総生成数・費用が3に制限されたとは記録しない |
| rootのtext抽出結果が空でrefusalがある | 空の成功回答にせず拒否として扱う |

## provenanceと未確認事項

5件の公式一次資料を2026-10-04 UTCに確認した。changelog、Multi-agent、deployment checklist、モデルページはnative webで本文を開いた。beta create referenceのHTMLはnative webのサイズ上限を超え、`.md`版も取得できなかったため、同じ公式HTMLをread-only取得し、schemaの該当節を読んだ。取得できないページだけを根拠にせず、native guideと実取得referenceを照合した。

各ページに再利用可能なオープンライセンスを確認できず、catalogのlicenseはunknown。SDK sample・実装・図は転載せず、限定した契約の独自要約と設計案を記した。repository分析・コード導入・module昇格はない。release_notesの30日TTLに合わせ、再確認期限は2026-11-03とした。

未確認なのは、実APIでの完了競合・ack喪失、注入の重複排除と再接続回復、個々のsubagentの全状態遷移、cancel時の子と外部toolへの伝播、agent属性の文書差の実挙動、SDK最低版、全対応モデル・tier・regionの組合せ、課金のagent別集計である。今回、API keyを使う生成・tool実行や障害注入は行っていない。検索evalは本書が検索可能であることだけを検証し、上記試験の成功を証明しない。

[release]: https://developers.openai.com/api/docs/changelog
[multi]: https://developers.openai.com/api/docs/guides/responses-multi-agent
[schema]: https://developers.openai.com/api/reference/resources/beta/subresources/responses/methods/create
[checklist]: https://developers.openai.com/api/docs/guides/deployment-checklist
[model]: https://developers.openai.com/api/docs/models/gpt-6.1-sol

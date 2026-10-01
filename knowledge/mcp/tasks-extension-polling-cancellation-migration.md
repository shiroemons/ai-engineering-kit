---
{
  "id": "mcp-tasks-extension-polling-cancellation-migration",
  "title": "MCP Tasks の版移行: 2025-11-25 core から 2026-07-28 extension への polling・入力・取消契約",
  "kind": "knowledge",
  "technology": "mcp",
  "version": "MCP core 2025-11-25 experimental Tasks vs Tasks extension stable 2026-07-28; ext-tasks 0.2.2 source at 5246bc3d0253c1c4b09e682f690b7e8b97362500 (commit 2026-09-30 UTC); verified 2026-10-01 UTC",
  "tags": [
    "research-domain:ai-engineering",
    "mcp",
    "tasks",
    "async",
    "polling",
    "cancellation",
    "version-migration",
    "SEP-2663",
    "tasks/get",
    "tasks/update",
    "tasks/cancel",
    "isError",
    "eventual-consistency"
  ],
  "sources": [
    {
      "id": "mcp-tasks-legacy-2025-11-25-20261001",
      "url": "https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks",
      "type": "official_docs"
    },
    {
      "id": "mcp-tasks-changelog-2026-07-28-20261001",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/changelog",
      "type": "official_docs"
    },
    {
      "id": "mcp-tasks-extension-2026-07-28-20261001",
      "url": "https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks",
      "type": "official_docs"
    },
    {
      "id": "mcp-tasks-extension-overview-20261001",
      "url": "https://tasks.extensions.modelcontextprotocol.io/",
      "type": "official_docs"
    },
    {
      "id": "mcp-tasks-extension-typescript-20261001",
      "url": "https://tasks.extensions.modelcontextprotocol.io/typescript/",
      "type": "official_docs"
    },
    {
      "id": "mcp-tasks-extension-repository-5246bc3-20261001",
      "url": "https://github.com/modelcontextprotocol/ext-tasks/tree/5246bc3d0253c1c4b09e682f690b7e8b97362500",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-12-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# MCP Tasks の版移行と長時間 tool 実行

## 問いと結論

長い tool 実行を「受付済み・入力待ち・処理結果・取消要求」に分けて管理するとき、旧 MCP Tasks client をそのまま使えるか。**2025-11-25 の実験的 core 機能と、2026-07-28 の optional extension は wire contract が異なる**。とくに結果の取得経路、tool error の終端分類、取消応答の保証を移行対象にする。新 extension があることは、すべての host が対応したことを意味しない。[旧仕様][mcp-tasks-source-1] / [変更履歴][mcp-tasks-source-2]

このページは一般的な agent loop の停止条件や transport 再接続の説明ではなく、**同じ Tasks という名称の世代差**を扱う。現行版の根拠は [版固定 Tasks 仕様][mcp-tasks-source-3] と [固定 commit の schema][mcp-tasks-source-4]。以下の「設計提案」は本書の判断であり、公式の追加要件ではない。

## 版と変更の根拠

- 2025-11-25: core 仕様自身が Tasks の導入版と experimental 状態を明示する。`tasks` capability を初期化時に交換し、requestor が task 化を選ぶ。
- 2026-07-28: core changelog の Major changes #6 は SEP-2663 による公式 extension への移動を記録する。`tasks/result` と `tasks/list` が除去され、`tasks/get` による polling と `tasks/update` が中心になる。単なる URL 移動ではない。
- extension リポジトリ README は `schema/2026-07-28/` を Stable、`schema/draft/` を Development と区別する。ここでいう stable は版固定 schema の状態であり、各 SDK の実装完成度の保証ではない。
- 調査時に固定した repository commit は `5246bc3d0253c1c4b09e682f690b7e8b97362500`、commit 時刻は 2026-09-30 22:23:34 UTC、package version は 0.2.2。**これはソースの版と commit 日であり、npm 公開日時を確認した主張ではない**。[固定 repository][mcp-tasks-source-5]

## 旧実装から変える境界

| 境界 | 2025-11-25 core Tasks | 2026-07-28 Tasks extension |
|---|---|---|
| 合意 | 初期化時の `tasks` capability、tool ごとの `execution.taskSupport` | 毎リクエストの `io.modelcontextprotocol/clientCapabilities.extensions` 内に `io.modelcontextprotocol/tasks` を宣言。server は `server/discover` で宣言 |
| 作成の選択 | requestor が `params.task` を付け、必要なら `ttl` を指定 | 対応 client に対し server がその呼出しを task 化するか決める。別の task 希望フラグはない |
| 対象 | tools/call に加え、逆方向の sampling / elicitation にも capability がある | この版で task 化できるのは `tools/call` |
| 作成応答 | JSON-RPC result の中に `task` object | JSON-RPC result 自体が flat な Task と `resultType: "task"` を持つ |
| 結果取得 | `tasks/get` は状態、`tasks/result` が元リクエストの結果 | `tasks/get` が状態と終端の `result` または `error` を返す。get 応答の `resultType` は `"complete"` |
| 入力待ち | `input_required` では `tasks/result` を呼び、関連 task の server request を扱う | `inputRequests` を読み、対応する `inputResponses` を `tasks/update` で送る |
| 時間フィールド | `ttl` / `pollInterval` | `ttlMs` / `pollIntervalMs`。ミリ秒単位 |
| 一覧 | capability に応じて `tasks/list` | `tasks/list` はない |

根拠: [旧仕様の Capabilities・Protocol Messages][mcp-tasks-source-1]、[移行 changelog][mcp-tasks-source-2]、[版固定仕様][mcp-tasks-source-3]、[固定 schema][mcp-tasks-source-4]。新旧の shape を推測で混在させない。

## 終端状態と業務結果は別に判定する

2026-07-28 の `DetailedTask` は状態ごとに payload を分ける。`working` は処理中、`input_required` は入力要求、`completed` は `result`、`failed` は JSON-RPC `error`、`cancelled` は取消済み状態を表す。`completed` / `failed` / `cancelled` が終端である。[固定 schema][mcp-tasks-source-4] / [extension overview][mcp-tasks-source-6]

**tool が `isError: true` を返したときの分類が変わった**。旧版はこれを `failed` に含める。新版は tool result を取得できたなら `completed` の `result` に保持し、`failed` を JSON-RPC error に限定する。したがって `status == completed` だけで「処理成功」と表示すると、tool の業務上の失敗を隠す。[旧版 Task Status][mcp-tasks-source-1] / [新版 Task Execution Errors][mcp-tasks-source-7]

設計提案: 保存する観測を「task の終端状態」と「元 tool result の成功・失敗」に分ける。`failed` の件数だけを失敗率にしない。重複実行を避けたい tool では、業務識別子と task ID を結び付け、結果の失敗と transport の失敗に同じ自動再実行を適用しない。Tasks 自体が業務処理の exactly-once を保証するという根拠は今回確認していない。

## 入力応答の ack と状態更新は一致しない

新版では `tasks/get` の `inputRequests` が未回答要求の snapshot となり、client は同じ key の表示・回答を重複させないことが推奨される。key は task の生存期間中で再利用できない。`tasks/update` の成功応答は回答の受領 ack であり、その直後の get に回答済み状態が見えなくても仕様違反とは限らない。部分回答を受け取る server では、未回答分が残る間は入力待ちが続く。[新版 Task Update Requests][mcp-tasks-source-8]

設計提案: client は task ID ごとに「表示済み key」「回答送信済み key」「最後に観測した status」を別々に保持する。ack 直後の同じ要求を新規依頼と解釈して利用者へ再提示しない。入力の同意・秘密情報の扱いには既存の [elicitation 境界](elicitation-consent-sensitive-input.md) を適用する。task 経由になっても server の入力要求がより信頼できるものに変わるわけではない。

task 作成前に入力を求める通常の Multi Round-Trip Requests は元メソッドの retry で応答するが、**task 作成後の入力は tasks/update** で返す。この二つは交換可能な経路ではない。[新版 Example Message Flow の注記][mcp-tasks-source-9]

## 取消要求を取消完了として表示しない

旧版は有効な `tasks/cancel` を受けると、返答前に `cancelled` 状態へ移すことを要求していた。新版は協調的 cancellation で、空の ack（共通フィールド `resultType: "complete"` を含む）が示すのは取消意思の受領までである。実行停止や将来の `cancelled` 到達は保証されず、完了との競合で別の終端に達し得る。[旧版 Task Cancellation][mcp-tasks-source-1] / [新版 Task Cancellation][mcp-tasks-source-10]

新版は client が取消送信後に状態を破棄してよく、`cancelled` になるまで polling を義務付けない。これは「止まったと確認できた」という意味ではない。[版固定仕様 Cancellation][mcp-tasks-source-10]

設計提案: UI は ack 後を「取消要求を受け付けた」と表示する。外部の deployment / batch が本当に停止したことまで必要な業務では、task または背後の job 状態を追加観測する独自方針を設ける。補償や rollback の有無は job ごとの契約で確認する。単に browser を閉じたことや HTTP 接続が切れたことを、永続 task の取消と扱わない。

## 永続化・期限・通知・認可の実装判断

新版の server は task を永続化し、返した ID が `tasks/get` で解決できる状態になるまで作成応答を返してはならない。client は ID を保存して再起動後の polling に利用できる。一方 `ttlMs` は作成時刻からの保有期限で、値は途中変更可能、`null` は無期限。期限は完了 SLA ではなく、期限後には task や結果が消えることがある。`pollIntervalMs` も変わり得るため、最初の値を固定した busy polling を避ける。[版固定仕様 Task Creation・Polling][mcp-tasks-source-3] / [overview lifecycle][mcp-tasks-source-6]

通知は `notifications/tasks` に変わり、`subscriptions/listen` で対象 task ID を指定する。応答と同等の task 状態が通知され、利用時に必ず polling を併走する必要はない。旧版 `notifications/tasks/status` の名前を流用しない。[版固定仕様 Task Status Notifications][mcp-tasks-source-11]

設計提案: 再起動時に復元するのは task ID だけでなく接続先識別子・使用した仕様世代・最後の期限情報とする。task が見つからない場合を「業務処理失敗」と自動断定せず、期限切れ・アクセス不可・結果未確認を区別する。作成要求の応答そのものを失って ID が分からないケースは、既知 ID の再取得とは別問題であり、自動再送前に tool の冪等性を確認する。

Streamable HTTP の `tasks/get` / `tasks/update` / `tasks/cancel` では `Mcp-Name` に `params.taskId`、`Mcp-Method` に RPC 名を送る。task ID を推測困難にすることと、各 task 関連リクエストで認証・認可を検査することがともに要求される。ID を知っていることだけで無条件に許可する設計にはしない。[版固定仕様 Routing Headers・Security Considerations][mcp-tasks-source-3]

## SDK と資料のずれを見分ける

- 公式 TypeScript guide の対象は MCP TypeScript SDK v2 を利用する application。`/client` は世代横断の requester、`/receiver` は **2025-11-25 sampling / elicitation receiver**、`/core/v1` と `/core/v2` は wire schema を分ける。package 名や SDK の v2 だけから、server-side の全機能が新 Tasks に移行済みと推論しない。[TypeScript API][mcp-tasks-source-12] / [固定 package 0.2.2][mcp-tasks-source-13]
- 2026-10-01 の資料照合で、extension の rolling overview は capability error を `-32003` と掲載しているが、版固定仕様と core changelog は `-32021` を使う。本書は **版固定の -32021** を採用し、overview の数値を実装へコピーしない。[overview][mcp-tasks-source-6] / [版固定仕様 Capability Negotiation][mcp-tasks-source-14] / [core Minor changes #12][mcp-tasks-source-2]
- 新版仕様の作成説明には `task.taskId` という文言も残るが、同じページの型・例と固定 schema は **flat CreateTaskResult**。本書は schema に従って旧版の nested `task` と区別する。[固定 schema][mcp-tasks-source-4]

## 移行時の確認項目（本書の設計提案、実行済み test ではない）

1. extension 宣言のない request に task handle を返さず、必要な場合の capability error を扱えるか
2. 同じ tool の同期結果と task handle の両方を decode し、get の `resultType: "complete"` を task 自体の完了と取り違えないか
3. `completed` + `isError: true` と、`failed` + JSON-RPC error の双方を保存・表示できるか
4. 入力回答の ack 後に古い `input_required` が見えても、同じ同意を二度求めないか
5. cancel の ack 後に `working` または `completed` が見える競合を扱えるか
6. process 再起動、task 期限切れ、通知 stream 切断時に、同じ ID の再取得と新規業務処理の実行を混同しないか
7. 別利用者の task ID に対するアクセスが認可で拒否され、ログに bearer として扱い得る ID を不要に露出しないか

## 適用範囲・未確認事項・provenance

- 取得日は全 source 2026-10-01 UTC。仕様の revision 日と repository commit 日を区別した。公式ページには独立した記事公開日がないものがあり、それを取得日で代用していない。
- この調査は仕様と固定ソースの読解。npm 配布物の検証、実クライアント相互運用、永続 store の故障復旧、負荷試験は未実施。特定 host の対応可否、取消までの最大時間、最小保持期間も保証しない。
- native web で複数の一次ページを実際に開いた。固定 SHA の一部 URL は cache miss だったため、公式 repository の read-only clone で上記 SHA と対象ファイルを照合した。取得失敗を source が存在しない根拠にはしていない。
- ext-tasks は固定 commit の LICENSE により Apache-2.0 を確認。core 仕様 repository は Apache-2.0 / MIT の移行条件を [LICENSE][mcp-tasks-source-15] に記載している。本書は原文・sample code を転載せず独自の要約と設計判断に限定し、module へ昇格していない。
- `official_docs` と `github_repository_analysis` の TTL は90日。`mcp` の追加 technology TTL はなく、明示期限は2026-12-30。版固定でも対応状況や資料間のずれは再確認する。

## 参照リンク

[mcp-tasks-source-1]: https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks
[mcp-tasks-source-2]: https://modelcontextprotocol.io/specification/2026-07-28/changelog
[mcp-tasks-source-3]: https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks
[mcp-tasks-source-4]: https://github.com/modelcontextprotocol/ext-tasks/blob/5246bc3d0253c1c4b09e682f690b7e8b97362500/schema/2026-07-28/schema.ts
[mcp-tasks-source-5]: https://github.com/modelcontextprotocol/ext-tasks/tree/5246bc3d0253c1c4b09e682f690b7e8b97362500
[mcp-tasks-source-6]: https://tasks.extensions.modelcontextprotocol.io/
[mcp-tasks-source-7]: https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks#task-execution-errors
[mcp-tasks-source-8]: https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks#task-update-requests
[mcp-tasks-source-9]: https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks#example-message-flow
[mcp-tasks-source-10]: https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks#task-cancellation
[mcp-tasks-source-11]: https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks#task-status-notifications
[mcp-tasks-source-12]: https://tasks.extensions.modelcontextprotocol.io/typescript/
[mcp-tasks-source-13]: https://github.com/modelcontextprotocol/ext-tasks/blob/5246bc3d0253c1c4b09e682f690b7e8b97362500/packages/ext-tasks/package.json
[mcp-tasks-source-14]: https://tasks.extensions.modelcontextprotocol.io/specification/2026-07-28/tasks#capability-negotiation
[mcp-tasks-source-15]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/3098fe94caa1b9e0afaaa6d30e040b61d5802471/LICENSE

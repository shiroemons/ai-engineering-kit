---
{
  "id": "mcp-elicitation-consent-sensitive-input",
  "title": "MCP elicitation: form と URL の同意・キャンセル・秘密情報境界",
  "kind": "knowledge",
  "technology": "mcp",
  "version": "MCP specification 2026-07-28 revision (current resolver verified 2026-10-01 UTC)",
  "tags": [
    "research-domain:ai-engineering",
    "elicitation",
    "form",
    "url",
    "consent",
    "cancellation",
    "mrtr",
    "input-required",
    "request-state",
    "sensitive-input",
    "phishing"
  ],
  "sources": [
    {
      "id": "mcp-elicitation-spec-2026-07-28-2026-10-01",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/client/elicitation",
      "type": "official_docs"
    },
    {
      "id": "mcp-mrtr-spec-2026-07-28-2026-10-01",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr",
      "type": "official_docs"
    },
    {
      "id": "mcp-elicitation-changelog-2026-07-28-2026-10-01",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/changelog",
      "type": "official_docs"
    },
    {
      "id": "mcp-elicitation-current-spec-2026-10-01",
      "url": "https://modelcontextprotocol.io/specification/latest",
      "type": "official_docs"
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

# MCP elicitation: form と URL の同意・キャンセル・秘密情報境界

## 問いと適用範囲

ツールの途中で利用者へ追加入力を求めるとき、どの経路に何を流し、何をもって完了と判定するか。
本書は 2026-07-28 改訂の elicitation と Multi Round-Trip Requests (MRTR) に限定する。
2026-10-01 UTC に [latest](https://modelcontextprotocol.io/specification/latest) が同改訂へ転送されることを確認した。将来の最新を保証しない。
既存の [OAuth 認可](authorization.md) と [HTTP transport](streamable-http-session-resumability.md) に対し、入力要求・同意・再試行の境界を補う。
以下の「仕様上の事実」は公式の要件、「設計判断」「検証案」は本書の提案である。

## 仕様上の事実: 入力経路を選ぶ

根拠は [Elicitation](https://modelcontextprotocol.io/specification/2026-07-28/client/elicitation) の User Interaction Model、Capabilities、Protocol Messages。

- form mode はクライアント経由の構造化入力で、回答はクライアントから見える。`mode` を省略した要求も form として扱う。
- URL mode はクライアント外で入力する経路。要求には明示的な `mode: url`、理由を示す `message`、有効な `url` が必要。
- パスワード、API key、access token、payment credentials を form で求めることは MUST NOT。これらの入力には URL mode が MUST。
- この規定の sensitive information はアクセスや決済を許す秘密・資格情報を指す。氏名・メール・ユーザー名は一律禁止ではないが、レビューと拒否の余地が必要。
- クライアントは要求元サーバーを明示し、decline / cancel を選べるようにし、form では送信前の確認・修正を可能にする（MUST）。
- capability は毎リクエストの `_meta.io.modelcontextprotocol/clientCapabilities` に宣言する。`elicitation: {}` は form のみ対応と同義。
- サーバーは未対応 mode を要求してはならない（MUST NOT）。URL 非対応だから秘密を form に落とす、という例外はない。
- form は `requestedSchema` を持つ。平坦な object と基本型が中心で、列挙値の複数選択配列は使えるが、ネストした object や object の配列は対象外。
- クライアントとサーバーの双方に schema 検証が SHOULD。一般の JSON Schema 全機能を利用できる契約とは異なる。

## 仕様上の事実: 同意と完了は別の状態

同じ Elicitation の Response Actions、URL Mode Elicitation Requests、Error Handling が根拠。

- `accept` は明示的な受諾。form では schema に沿う回答を `content` に載せ、URL mode では `content` を省略する。
- `decline` は明示的な拒否、`cancel` は決定せず閉じた状態。後者には Escape、ダイアログを閉じる操作、ブラウザ読み込み失敗などが含まれる。
- URL の `accept` は外部操作への同意であり、その入力・認可・支払いが完了したという通知ではない。
- 元の要求を再試行したとき、サーバーが `requestState` または保存状態から外部操作の完了を判定し、最終結果か再度の `InputRequiredResult` を返す。
- クライアントは元の要求を手動で再試行・キャンセルできる UI を提供することが SHOULD。
- サーバーは拒否・取消・クライアント側処理失敗を扱うことが MUST。成功や再試行を当然視してはならない。

## 仕様上の事実: MRTR で元の要求を再試行する

[MRTR](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr) が wire contract の根拠。

1. サーバーは `resultType: input_required` の `InputRequiredResult` に追加入力要求を含める。以前の独立した server-initiated request 方式は現行では非対応。
2. `inputRequests` は要求内で一意のサーバー指定 key と要求の map。クライアントは対応する key の `inputResponses` を元の要求の再試行へ載せる。
3. この中間結果が許されるメソッドは `tools/call`、`resources/read`、`prompts/get`。他のメソッドに返すことは MUST NOT。
4. `InputRequiredResult` は `inputRequests` と `requestState` の少なくとも一方を含むことが MUST。全応答にフォームが必要なわけではない。
5. `requestState` があれば、クライアントはそのまま返す（MUST）。内容の解釈・変更は禁止され、元の応答になければ再試行にも付けない。
6. 初回と再試行の JSON-RPC `id` は別であることが MUST。入力と状態は元の要求にだけ適用し、並行する別要求へ流用しない。
7. サーバーは `requestState` を攻撃者が制御し得る入力として扱う。認可・資源アクセス・業務判断に影響するなら完全性を保護し、検証失敗を拒否する（MUST）。
8. 認証主体・短い TTL・元のメソッドや重要引数への結合と検証が SHOULD。これだけで一回限りの利用は保証されず、必要ならサーバー側で消費済みを管理する（MUST）。

クライアントにとって opaque であることは、サーバーにとって信頼できることを意味しない。
暗号化された状態でも、別ユーザーの要求への転用や二重使用の検査を省けるわけではない。

## 仕様上の事実: URL にも信頼境界がある

Elicitation の Safe URL Handling、Identifying the User、Phishing が根拠。

- クライアントへ渡す URL に資格情報・個人識別情報などを含めてはならない（MUST NOT）。保護資源へアクセスできる事前認証済み URL も禁止。
- クライアントは URL とその metadata を自動で pre-fetch してはならず、利用者の明示同意なしに開いてはならない（MUST NOT）。
- 同意前に完全な URL を見せること、およびクライアントや LLM がページ内容・入力を検査できない安全な経路で開くことが MUST。
- 本番では HTTPS が SHOULD。ドメインを強調し、不審な URI に警告し、form の説明欄など別の場所の URL をクリック可能にしないことも SHOULD 系の要件。
- サーバーは elicitation を client / user identity に結び付ける（MUST）。クライアントの自己申告ユーザー名だけを本人確認に使ってはならない。
- URL を開いて入力する本人と、MCP から入力要求を開始した本人が同一であることをサーバーが確認する（MUST）。URL 改変に耐える識別方法が必要。
- 外部サービスの OAuth は MCP client → MCP server の認可とは別。外部資格情報をクライアントへ返すことは禁止され、取得・保管・利用は MCP サーバー側の責務。

## 版の差と混同しやすい点

[2026-07-28 Key Changes](https://modelcontextprotocol.io/specification/2026-07-28/changelog) の Major 2 / 7 / 8 と Minor 11 を確認した。

- URL mode 自体は 2025-11-25 で導入済み。2026-07-28 で初めて登場した機能とは扱わない。
- 現行は per-request capability と MRTR。旧版の initialization 宣言や単独の `elicitation/create` RPC をそのまま使わない。
- `elicitationId` と `notifications/elicitation/complete` は削除済み。通知待ちで完了を検出する実装はこの改訂に適合しない。
- retry 間の独自識別子が必要なら `requestState` に入れる。識別子があることと、認可・完全性・単回性が守られることは別問題。
- tool の `inputSchema` / `outputSchema` の拡張を、elicitation の `requestedSchema` に適用したと解釈しない。

## 設計判断: 停止可能な入力待ちにする

以下は本書の設計案であり、仕様の追加要件ではない。原則は [Specification の同意と制御](https://modelcontextprotocol.io/specification/latest) に沿う。

- 状態を「追加入力待ち」「同意済み・外部操作未確認」「完了確認済み」「拒否」「取消」に分け、URL accept 直後の成功表示を防ぐ。
- 拒否と取消は別イベントとして記録するが、どちらも資格情報取得や業務処理の成功に変換しない。再提示は利用者が文脈を理解できる時点に限る。
- mode 非対応時は処理を止め、対応するクライアントか別の安全な導線を案内する。例外処理の利便性より秘密情報の経路制約を優先する。
- URL の preview 取得やリンクスキャンを表示層が自動実行しないかも確認する。明示的なページ遷移だけを見て安全と判定しない。
- URL の画面を通常の埋め込みブラウザで実装する前に、ホストから DOM・画面・入力が取得できないか確認する。分離が成立しなければ別経路を選ぶ。
- 再試行は同意の再利用と業務副作用の再実行を分離する。決済などではサーバー上の処理 ID と完了記録で重複を抑える。
- 監査には要求元・mode・応答 action・状態検証結果を残す。秘密、完全な状態 blob、回答全文を通常ログに出すことを既定にしない。

## 検証案: 実装レビューの最小ケース

以下は実装を作る場合の受入条件案であり、本調査で SDK やブラウザを動かした結果ではない。

- form 専用 capability のクライアントに URL 要求が送られず、秘密入力が form に切り替わらない。
- フォーム修正後の内容だけが送信され、拒否ボタンと Escape がそれぞれ decline / cancel として区別される。
- URL accept 後も外部操作未完了なら成功扱いせず、元の要求を再試行するまで完了判定を行わない。
- URL 表示だけでは DNS / HTTP の preview 通信が発生せず、同意後の画面からホストが入力を取得できない。
- 改ざん済み・期限切れ・別主体・別要求の `requestState` が拒否され、一回限りの操作では同じ状態の再使用も拒否される。
- URL を別ユーザーへ転送して開いても、元の要求者に他人の外部アカウントが結び付かない。
- 並行する二つの元要求で inputResponses / requestState を取り違えず、再試行には新しい JSON-RPC id を使う。

## 出典・期限・未確認事項

- 取得日は全4件とも 2026-10-01 UTC。改訂識別子は 2026-07-28 であり、個別ページの独立した公開・更新日は表示を確認できていない。
- official_docs の TTL は90日。mcp の技術 TTL は未設定のため、明示期限を 2026-12-30 とした。
- [公式 LICENSE](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/LICENSE) を確認した。新規・再ライセンス済み仕様寄稿は Apache-2.0、同意未取得の旧寄稿は MIT のまま。全体を選択式 dual license と断定しない。
- 寄稿単位のライセンス対応は未確認。本書は独自の日本語要約で、コードのコピーや module 昇格は行っていない。
- SDK ごとの 2026-07-28 対応版、ブラウザ分離の実効性、URL 操作の完了検出の製品別実装は未確認。特定 SDK で動くという保証には使わない。
- Tasks extension の待機・取消や、業務処理の rollback は対象外。elicitation の cancel からそれらの完了を推測しない。

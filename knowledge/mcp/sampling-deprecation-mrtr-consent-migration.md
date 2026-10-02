---
{
  "id": "mcp-sampling-deprecation-mrtr-consent-migration",
  "title": "MCP Sampling の非推奨移行: 削除時期・MRTR・同意・tool loop の境界",
  "kind": "knowledge",
  "technology": "mcp",
  "version": "MCP Sampling 2026-07-28 vs 2025-11-25; feature lifecycle and deprecated registry verified 2026-10-02 UTC",
  "tags": [
    "research-domain:ai-engineering",
    "sampling",
    "deprecation",
    "migration",
    "mrtr",
    "consent",
    "client-capabilities",
    "tool-loop",
    "provider-api"
  ],
  "sources": [
    {
      "id": "mcp-sampling-spec-2026-07-28-20261002",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/client/sampling",
      "type": "official_docs"
    },
    {
      "id": "mcp-sampling-legacy-2025-11-25-20261002",
      "url": "https://modelcontextprotocol.io/specification/2025-11-25/client/sampling",
      "type": "official_docs"
    },
    {
      "id": "mcp-sampling-deprecated-registry-2026-07-28-20261002",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/deprecated",
      "type": "official_docs"
    },
    {
      "id": "mcp-sampling-feature-lifecycle-20261002",
      "url": "https://modelcontextprotocol.io/community/feature-lifecycle",
      "type": "official_docs"
    },
    {
      "id": "mcp-sampling-mrtr-2026-07-28-20261002",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr",
      "type": "official_docs"
    },
    {
      "id": "mcp-sampling-consent-principles-2026-07-28-20261002",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28",
      "type": "official_docs"
    },
    {
      "id": "mcp-sampling-changelog-2026-07-28-20261002",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/changelog",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# MCP Sampling の非推奨移行: 削除時期・MRTR・同意・tool loop の境界

## 問いと適用範囲

Sampling が非推奨になった MCP サーバーを保守するとき、今すぐ消すもの、移行中に守る契約、LLM provider API への移行で作り直す権限境界は何か。

2026-10-02 UTC に開いた **2026-07-28 改訂**と、比較用の **2025-11-25 改訂**を対象にする。将来の最新仕様や SDK の実装済み機能を保証しない。本書では「仕様上の事実」と「設計判断・検証案」を分ける。

既存の [elicitation と MRTR](elicitation-consent-sensitive-input.md) は利用者入力と requestState の保護、[HTTP transport](streamable-http-session-resumability.md) は接続・再送を扱う。本書はサーバーがクライアント側の LLM に生成を依頼する Sampling と、その依存を外す判断に絞る。Sampling を、フォーム入力を得る elicitation の別名として扱わない。

## 仕様上の事実: deprecated は removed ではない

- 2026-07-28 の [Sampling][sampling] と [Key Changes の Deprecated][changes] は Sampling を非推奨にした。新規実装は採用を SHOULD NOT、既存実装は **LLM provider API への直接統合**へ移行することを SHOULD とする。移行期間中も Sampling の仕様は残る。
- [Deprecated Features][registry] における最早削除対象は **2027-07-28 以降に公開される最初の改訂**。2027-07-28 に自動停止する契約でも、その日に必ず新改訂が出る保証でもない。実際の削除は Core Maintainer の判断で遅くなり得る。
- [Feature Lifecycle][lifecycle] の通常の最低期間は、非推奨化を含む仕様改訂の公開から12か月。SEP の承認日や利用中の SDK の公開日を起点にしない。仕様からの削除と SDK が旧版サポートを終了する時期も別である。
- 同 policy には、対処不能な実害のあるセキュリティリスクを根拠に、承認された SEP で短縮する例外もある。その場合も非推奨化から最早削除までは最低90日。通常の最早日を無条件の長期サポート保証として利用しない。

設計判断: 新規の機能要件を Sampling に積み増さず、既存利用者が依存する互換経路を把握して段階的に外す。「非推奨なので即座に capability を消す」と「期限までは設計を変えなくてよい」の両極端を避ける。公開済みの仕様改訂、採用 SDK、接続相手の対応状況を別々に台帳へ記録する。

## 仕様上の事実: 機能の非推奨化と wire 移行は独立する

[旧 Sampling][legacy]、[現行 Sampling][sampling]、[Key Changes の Major 2 / 7][changes] を比較する。

| 境界 | 2025-11-25 | 2026-07-28 |
|---|---|---|
| Sampling の capability | initialization で宣言 | 各要求の `_meta.io.modelcontextprotocol/clientCapabilities` に宣言 |
| 生成依頼の封筒 | サーバー発の独立した `sampling/createMessage` JSON-RPC request | 元要求への `InputRequiredResult.inputRequests` 内に `sampling/createMessage` を含める |
| 生成結果を返す先 | その JSON-RPC request への応答 | 元要求の再試行に `inputResponses` を付ける |
| 利用者が拒否した場合 | エラー `-1` を返すことを SHOULD | client はエラーを伝えるためだけに元要求を再試行する必要がない |
| `includeContext` の `thisServer` / `allServers` | soft-deprecated | lifecycle policy 上の Deprecated。省略または `none` へ移行 |

**Sampling が残っているから旧い逆方向 RPC も残っている、とは言えない。** [MRTR][mrtr] は従来の server-initiated request パターンを置き換えた破壊的変更である。逆に MRTR へ載せ替えても、Sampling 自体の非推奨依存は解消しない。

### 移行期間の MRTR の最小境界

MRTR の詳細な状態保護は既存文書を参照し、ここでは Sampling adapter が必要とする点だけを挙げる。[MRTR][mrtr]

1. `InputRequiredResult` を返せる元メソッドは `tools/call`、`resources/read`、`prompts/get`。Sampling を独立した常時 push 経路として設計しない。
2. `inputRequests` の key と返す `inputResponses` の key を対応させる。`requestState` があれば client は解釈・改変せず同じ値を返す。
3. 再試行は新しい JSON-RPC id を使う。回答と状態を、並行する別の元要求へ流用してはならない。
4. サーバーは client が宣言していない入力要求を送ってはならず、依頼の実行や再試行も前提にできない。

設計判断: UI の状態を「生成依頼を受信」「生成を承認」「生成済み・返送未承認」「元要求を再試行済み」に分ける。拒否やローカル失敗を、元ツールの成功・rollback 完了・provider 呼び出し成功へ変換しない。サーバー側で確保した資源がある場合の失効処理は別途設計する。MRTR が server の待機継続を要しないことだけでは、業務資源の解放は保証されない。

## 仕様上の事実: capability・生成・ツール実行の権限を混同しない

[現行 Sampling][sampling] の Capabilities / Tools in Sampling / User Interaction Model が根拠。

- `sampling: {}` と `sampling.tools` の宣言は別。tool-enabled sampling には後者が必要で、未宣言 client へ送るのは MUST NOT。
- Sampling 内の `tools` はその要求に閉じた定義で、登録済み MCP tool と一致する必要はない。モデルが tool use を返した後、サーバーが実行し、結果を次の Sampling に追加する流れが示されている。
- 利用者が生成依頼を拒否できること、prompt の確認・編集、サーバーへの返送前の生成結果レビューは SHOULD。特定の UI 形式を必須にはしていない。

さらに [仕様全体の Security and Trust & Safety][consent] は、利用者データをサーバーへ公開する前、resource data を他所へ送る前、tool を起動する前の明示的な同意を求める。これらを MCP 単体がプロトコル上で強制するものではないため、ホスト側の実装責任が残る。

設計判断: capability は「処理できる形式」を示すものとして扱い、個々のデータ開示や副作用を承認した証拠にはしない。同名の tool がホストに登録されていても、Sampling の tool 定義からホストの実行器へ無条件で名前解決しない。要求元サーバー、定義、実行先、操作の権限を照合する。モデルが返した `tool_use` も実行許可そのものではない。

設計判断: 利用者が生成や返送を拒否したとき、サーバー所有の API key を使う直接 provider 呼び出しへ黙って fallback しない。新しい送信先・課金主体・データ用途が生じるなら別の同意判断が必要であり、非推奨化はそれを省略する理由にならない。

## 仕様上の事実: client の裁量と adapter の不変条件

### モデルと context

[現行 Sampling][sampling] の Model Preferences / System Prompt / Context Inclusion / Sampling Parameters を確認した。

- `modelPreferences.hints` は助言であり、client が最終的にモデルを選ぶ。`systemPrompt` は client が変更・無視できる。
- `includeContext` は client が変更・無視できる。非推奨の `thisServer` / `allServers` は避け、追加 context は省略または `none` にする方向が示されている。
- `maxTokens` は必須で client が尊重することは MUST。一方 `temperature`、`stopSequences`、`metadata` は変更・無視できる。

設計判断: サーバーが指定したモデル名や prompt がそのまま採用された前提で、品質・コスト・安全性を保証しない。`includeContext: none` でも、最初から `messages` に入っている秘密や不要な本文は消えない。送信直前に入力データの必要性を検査する。`maxTokens` は1回の生成上限であり、独自に設ける入力サイズ・総支出・再試行回数・tool loop の上限とは分けて管理する。

### tool loop の内容対応

[旧 Sampling の Message Content Constraints / Security Considerations][legacy] で次の契約を確認し、[現行 Sampling][sampling] でも維持されていることを確認した。

- assistant の `ToolUseContent` があるメッセージの直後には、各 id に対応する `toolUseId` を持つ `ToolResultContent` だけの user メッセージが必要（MUST）。全件を解決する前に別のメッセージを挟まない。
- tool result と通常の text / image / audio を同じ user メッセージで混在させない（MUST）。両当事者には tool loop の反復上限が SHOULD。

設計判断: provider adapter は role 名の変換だけで済ませず、未回答 id、重複 id、余分な結果、混在 content、並行呼び出しの結果取り違えを検査する。推論自体の retry と外部 tool の再実行を分離し、課金・書込みを伴う操作では業務処理 ID による重複抑制を設ける。モデルの返答形式が整っていることは、副作用の exactly-once を証明しない。

## 設計判断: 直接 provider API へ移す順序

以下は公式が規定した移行実装ではなく、[公式の移行先][changes]を採用する際のレビュー手順案である。特定 provider の認証方式、料金、保持条件、API schema は本調査では検証していない。

1. **依存を数える**: 元 tool と利用者・client ごとに Sampling 利用有無を把握する。本文を収集せず、成功・拒否・未対応・上限到達を別計測する。
2. **実行責任を決める**: 誰が provider を選び、課金し、秘密情報を保管し、削除要求や障害を扱うかを決める。MCP client のモデル契約や予算が自動的に server へ引き継がれるとはしない。
3. **情報の流れを描き直す**: prompt、ファイル、生成結果が通る主体を列挙し、新しい provider への送信と server への返送の両方をレビューする。旧 client 内の承認 UI を外すなら、同等の制御をどこに残すか明示する。
4. **adapter を分離する**: 業務処理が直接 wire format に依存しない境界を作る。旧 Sampling、MRTR Sampling、直接 provider は別経路として選択し、利用者拒否を契機に自動で経路を変えない。
5. **失敗を注入する**: provider timeout、予算超過、tool の部分成功、元要求の再試行を検証する。同意済みでも回数制限を超えて外部副作用を繰り返さない。
6. **切替を確認する**: 対象利用者が使う経路を移した証拠を得てから Sampling capability と互換コードを縮退させる。機能停止日、失敗時の表示、戻し方は自分の製品の方針として決める。

## 検証案: 互換実装の受入ケース

ここでのケースは今後実装する場合の提案。SDK 実行試験や provider への実通信の結果ではない。追加した `evals/knowledge/ai-engineering.json` は文書を検索できるかだけを確認する。

| 入力・条件 | 確認する境界 |
|---|---|
| `sampling` capability なし | server が生成依頼を出さず、明示的に未対応を扱う |
| `sampling: {}` のみ、`tools` 未宣言 | tool-enabled sampling を送らない |
| 2025-11-25 と 2026-07-28 の相手 | initialization / 独立 RPC と per-request capability / MRTR を混在させない |
| 利用者が生成を拒否、または生成後の返送を拒否 | 返送・元要求 retry・別 provider fallback が意図せず始まらない |
| Sampling の tool 名とホスト登録 tool 名が衝突 | 名前だけでホストの実行器に到達しない |
| 二つの元要求から同時に生成依頼 | inputResponses / requestState を取り違えず、新しい JSON-RPC id で再試行 |
| tool result が一件欠落、または text が混在 | adapter が不正な履歴を provider へ渡さない |
| tool 部分成功の直後に通信が切断 | 推論 retry で同じ書込みを再実行しない |
| `maxTokens` が小さいが大量の入力・反復がある | 生成上限と総予算の上限を別々に適用 |
| context を `none` にしたが `messages` に秘密が残る | context 設定だけをデータ最小化の保証としない |

## 出典・鮮度・未確認事項

- 全7資料の取得日は **2026-10-02 UTC**。仕様ページの版は URL の改訂日で固定した。個別の記事公開・更新日は表示を確認できず、版日付を記事公開日として補っていない。lifecycle は版付き URL がないため取得日時点の方針である。
- official_docs の TTL は90日、明示期限は **2026-12-31**。通常の最早削除時期も将来の方針変更・新 SEP を再確認する。
- [公式 LICENSE][license] は新規・再許諾済みの仕様寄稿を Apache-2.0、未再許諾の過去寄稿を MIT とし、仕様を除く documentation を CC-BY-4.0 とする。寄稿単位のライセンス対応は未確認。原文 sample code のコピー、OSS 実装分析、module 昇格は行っていない。
- SDK ごとの MRTR 対応版、Sampling の実採用率、runtime warning の有無、各 provider のトークン数計算・コスト・データ保持は未確認。仕様で許されることを、任意の SDK で動くという保証には使わない。
- 本書は通常の MRTR Sampling を対象とする。非同期 Tasks extension の wire contract は [別文書](tasks-extension-polling-cancellation-migration.md) を参照し、本書の retry 方針をそのまま流用しない。

[sampling]: https://modelcontextprotocol.io/specification/2026-07-28/client/sampling
[legacy]: https://modelcontextprotocol.io/specification/2025-11-25/client/sampling
[registry]: https://modelcontextprotocol.io/specification/2026-07-28/deprecated
[lifecycle]: https://modelcontextprotocol.io/community/feature-lifecycle
[mrtr]: https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr
[consent]: https://modelcontextprotocol.io/specification/2026-07-28
[changes]: https://modelcontextprotocol.io/specification/2026-07-28/changelog
[license]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/LICENSE

---
{
  "id": "llm-claude-google-cloud-organization-cutover-boundary",
  "title": "Claude Google Cloud移行: 接続成功・新organization・実行停止の境界",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Claude Models on Google Cloud (Anthropic managed), launch 2026-10-22; rolling provider/IAM/retention/Managed Agents beta docs verified 2026-10-24 UTC; SDK versions and runtime untested",
  "tags": [
    "research-domain:ai-engineering",
    "llm",
    "Claude",
    "AnthropicGoogleCloud",
    "AnthropicVertex",
    "organization",
    "cutover",
    "IAM",
    "ZDR"
  ],
  "sources": [
    {
      "id": "claude-gcp-cutover-release-20261024",
      "url": "https://platform.claude.com/docs/en/release-notes/overview",
      "type": "release_notes"
    },
    {
      "id": "claude-gcp-cutover-comparison-20261024",
      "url": "https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/overview",
      "type": "official_docs"
    },
    {
      "id": "claude-gcp-cutover-migration-20261024",
      "url": "https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/anthropic-managed",
      "type": "official_docs"
    },
    {
      "id": "claude-gcp-cutover-google-boundary-20261024",
      "url": "https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/partner-models/claude/anthropic-managed",
      "type": "official_docs"
    },
    {
      "id": "claude-gcp-cutover-retention-20261024",
      "url": "https://platform.claude.com/docs/en/manage-claude/api-and-data-retention",
      "type": "official_docs"
    },
    {
      "id": "claude-gcp-cutover-residency-20261024",
      "url": "https://platform.claude.com/docs/en/manage-claude/data-residency",
      "type": "official_docs"
    },
    {
      "id": "claude-gcp-cutover-iam-20261024",
      "url": "https://platform.claude.com/docs/en/api/claude-models-on-google-cloud-anthropic-managed-iam-permissions",
      "type": "official_docs"
    },
    {
      "id": "claude-gcp-cutover-interrupt-20261024",
      "url": "https://platform.claude.com/docs/en/managed-agents/events-and-streaming",
      "type": "official_docs"
    },
    {
      "id": "claude-gcp-cutover-workflow-stop-20261024",
      "url": "https://platform.claude.com/docs/en/managed-agents/workflow-runs",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# Claude の Google Cloud 移行は、URL 変更ではなく organization の切替として扱う

## 問いと適用範囲

既存の Claude 呼出しを、新しい Google Cloud の Anthropic-managed 経路に変更するだけで本番移行は完了するか。答えは **いいえ**。2026-10-22 に提供が発表された Anthropic-managed は、Google Cloud の認証・課金を入口に Anthropic が Claude API を運用する別の構成である。通信成功、旧リソースからの移行、データ取扱条件、旧実行の停止を別々に受け入れる必要がある。

本書は 2026-10-24 UTC に確認した公開仕様の整理。SDK の全版や利用組織での提供可否を実測したものではない。既存の [API/SDK 版移行](claude-api-sdk-version-rollback-boundary.md) は wire version と package version の互換性を扱う。本書は **provider・organization・workspace の変更を伴う cutover** を扱う。

## 1. モデル名ではなく経路で提供方式を識別する

[公式比較](https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/overview) による区別:

| 観測対象 | Anthropic-managed | Google-managed |
|---|---|---|
| SDK client | `AnthropicGoogleCloud` | `AnthropicVertex` |
| API host | `claude.googleapis.com` | `aiplatform.googleapis.com`、regional / multi-region 系 host |
| メッセージ経路 | workspace を含む URL の `.../invoke/v1/messages` | `.../publishers/anthropic/models/{model}:rawPredict`、stream は `:streamRawPredict` |
| request 契約 | body の `model`、header の `anthropic-version`、body の `stream` | URL の model、body の `anthropic_version`、stream 専用 endpoint |
| rate limits | Anthropic 管理 | Google Cloud 管理 |

両方とも Google Cloud credentials / ADC を使えるが IAM roles は別。Claude 4.6 以降の model ID は共通なので、ID の一致は提供方式や権限の一致の証拠にならない。`api.anthropic.com` と API key の組合せは、どちらの Google Cloud 方式でもない。

[Google 側の API reference](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/partner-models/claude/anthropic-managed) では、workspace scope と organization scope で base URL が異なる。後者は `.../claudeOrganization/invoke` を使う。単一の workspace URL をすべての Admin 操作へ流用しない。Bearer の Google access token を使い、`x-api-key` は送らない。

## 2. 移行元を二つに分け、HTTP 成功を完了にしない

**直接 Claude API などの既存 Anthropic organization から移る場合:** [移行ガイド](https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/anthropic-managed#moving-from-an-existing-anthropic-organization) は、既存 organization の変換ではなく、新しい organization への切替と説明している。既存の API keys、workspaces、Console 設定は引き継がれず、files、skills、batches、Managed Agents resources は旧 organization に残る。新旧は並行稼働できる。

**Google-managed から移る場合:** 公式比較上、移行元には Anthropic organization がない。新方式で organization を新設するのであって、「旧 Anthropic organization の資源を移す」という上記の前提を当てはめない。`AnthropicVertex` から `AnthropicGoogleCloud` への変更と、使う feature・model ID・IAM・rate limits の適合を確認する。

新方式の SDK 設定では、明示引数が環境変数に優先し、project の未指定時は ADC 由来の project に解決され得る。workspace ID は通常必須だが、base URL 指定時と organization-level routes には例外がある。暗黙の値に任せず、移行検証では解決済み project・workspace・host・model を記録する。

**独自の設計案:** resource ID を裸の文字列として新旧 client 間で使い回さず、`provider / organization / workspace / resource_id` を一組として保存する。切替台帳に各 workload の移行先、参照ファイル・skill の再作成結果、旧 batch の結果回収先、戻し先を記録する。トークンや本文をこの台帳・ログへ記録する必要はない。新規 Message が一度成功しても、既存 batch の取得や旧 agent session の継続が成功したとは判定しない。

## 3. Google Cloud 経由とデータ境界を同一視しない

[Google の 2026-10-22 更新資料](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/partner-models/claude/anthropic-managed) は、Google の IAM・routing・audit logs・Marketplace 層と、Anthropic の API / Console 層を分け、各社が自分の担当部分の独立した data processor と説明する。Anthropic-managed の保存・推論は Anthropic-managed infrastructure で行われ、Google Cloud の外へ route し得る。「Google Cloud に請求されるから Google-managed と同じ処理境界」と判断しない。

[Data residency](https://platform.claude.com/docs/en/manage-claude/data-residency) は、推論場所を制御する `inference_geo` と、保存・endpoint processing を制御する Workspace geo を独立の設定としている。接続先 location は公開ガイドどおり `global` のままにし、これらの条件を別途確認する。`inference_geo` は Claude 4.6 以降が対象で、以前のモデルはこの parameter を拒否する。

[Data retention](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention) によれば ZDR は organization ごとに有効化され、新 organization に自動継承されない。また ZDR はすべての機能を対象にせず、Managed Agents のような stateful resource や code execution などには対象外の条件がある。必要な model・feature と組織の設定を組にして確認する。

**独自の受入条件:** endpoint 疎通とは別に、担当者が新 organization のデータ処理・保持・地理設定と利用する機能を確認した証拠を残す。契約の適用可否・法的判断や、利用者固有の ZDR 承認は本書では確認していない。

## 4. Managed Agents を使う場合だけ追加する停止条件

Managed Agents は `managed-agents-2026-04-01` header の beta として扱う。[Anthropic-managed の Managed Agents 節](https://platform.claude.com/docs/en/build-with-claude/claude-models-on-google-cloud/anthropic-managed#claude-managed-agents) は、session / deployment 作成時に access を検査し、実行中には creator の access を再検査しないと明記する。Google Cloud access の失効後も既存 session と scheduled deployment は動き続け得る。これは **Anthropic-managed で作成した実行** の契約であり、Google-managed や直接 Claude API に共通する失効仕様とは主張しない。新方式導入後の identity 切替・退役でも、権限削除だけを workload の停止手順にしない。

[Session event stream](https://platform.claude.com/docs/en/managed-agents/events-and-streaming#interrupt-the-agent) では、`user.interrupt` の送信成功は queue への受理であり、処理済みの証拠ではない。停止目的では次の `user.message` を同送して再始動させない。処理前は `processed_at` が未設定で、tool call 中は適用が遅れる場合がある。

[Workflow runs](https://platform.claude.com/docs/en/managed-agents/workflow-runs#interrupt-a-session-with-runs-open) は、interrupt が workflow run を終了させないことも明記する。paused / idle は終端ではない。run の終了確認や open run による archive 拒否などの詳細は [既存の session 更新・run 手順](../agents/claude-managed-web-policy-session-update-boundary.md) と公式資料を参照する。scheduled deployment は別に pause 等を行い、その状態を確認する。

**独自の切替条件:** 新方式で退役する identity の失効と同じ台帳に、その identity が作成した session、open run、scheduled deployment の停止・継続責任者を列挙する。意図した二重稼働期間以外に新旧が外部副作用を重ねないよう、worker の認証変更と workload の停止完了を分離して確認する。

## 5. 最小の受入チェック

以下は公式要件を材料にした独自の検証計画であり、検証済み runtime 結果ではない。

1. 実際に使う SDK / CLI 版を固定し、秘密情報を除いた resolved routing tuple を採取する。`ant` CLI では既存 API key / profile の認証優先順位も確認する。一方 `AnthropicGoogleCloud` SDK は `ANTHROPIC_API_KEY` / `ANTHROPIC_AUTH_TOKEN` / `ANTHROPIC_BASE_URL` を無視するので、CLI と同じ環境変数解決を期待しない。
2. [IAM permission 表](https://platform.claude.com/docs/en/api/claude-models-on-google-cloud-anthropic-managed-iam-permissions) を操作単位で照合する。`roles/claude.viewer` は messages / token counting を実行できず、`roles/claude.user` は推論だけの役割ではなく workspace resource 操作も含む。正常経路に加え、許可していない操作の拒否も確認する。
3. text / stream / tools / saved resource をそれぞれ試す。機能同等性は model ID や基本応答から推定しない。
4. 新 organization の保持・地理・quota 条件を確認後、workload 単位で段階切替する。旧結果の回収と rollback の行先を保持する。
5. 停止すべき非同期実行を提供方式別に確認する。Anthropic-managed の実行では、IAM revoke、interrupt の queue 受理、session idle、run 終了、deployment pause を別の証跡にする。

## 限界・出典の扱い

- 新方式の提供日は [2026-10-22 release notes](https://platform.claude.com/docs/en/release-notes/overview) で確認した。rolling docs の全記述がその日に実装されたとは主張しない。
- SDK の constructor / CLI 認証優先順位、機能対応、account eligibility は変わり得る。本書は package 全版の契約ではなく、取得日の公開文書を対象にする。
- Google 文書は CC BY 4.0（code samples は Apache 2.0）表記を確認した。Anthropic ページの転載ライセンスは未確認。独自の日本語要約のみで、コードの転載・実アカウントの作成や変更はしていない。

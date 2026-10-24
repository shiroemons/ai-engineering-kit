---
{
  "id": "agents-claude-managed-web-policy-session-update-boundary",
  "title": "Claude Managed Agents の web 制限: 組織・host照合・session更新の境界",
  "kind": "knowledge",
  "technology": "agents",
  "version": "Claude Managed Agents beta managed-agents-2026-04-01 / agent_toolset_20260401; network and URL-context changes 2026-10-07, organization web policy changes 2026-10-20; rolling guides verified 2026-10-23 UTC; runtime not tested",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "claude",
    "managed-agents",
    "web_search",
    "web_fetch",
    "allowed_domains",
    "allowed_hosts",
    "url_not_allowed",
    "url_not_in_prior_context",
    "workflow_run_open",
    "full-replacement",
    "policy-composition"
  ],
  "sources": [
    {
      "id": "claude-managed-web-policy-rollout-20261023",
      "url": "https://platform.claude.com/docs/en/release-notes/overview",
      "type": "release_notes"
    },
    {
      "id": "claude-managed-web-policy-composition-20261023",
      "url": "https://platform.claude.com/docs/en/managed-agents/tools-web-restrictions",
      "type": "official_docs"
    },
    {
      "id": "claude-managed-web-network-hosts-20261023",
      "url": "https://platform.claude.com/docs/en/managed-agents/environments",
      "type": "official_docs"
    },
    {
      "id": "claude-managed-web-session-replacement-20261023",
      "url": "https://platform.claude.com/docs/en/managed-agents/session-operations",
      "type": "official_docs"
    },
    {
      "id": "claude-managed-web-workflow-update-gate-20261023",
      "url": "https://platform.claude.com/docs/en/managed-agents/workflow-runs",
      "type": "official_docs"
    },
    {
      "id": "claude-managed-web-tool-enable-boundary-20261023",
      "url": "https://platform.claude.com/docs/en/managed-agents/tools",
      "type": "official_docs"
    },
    {
      "id": "claude-managed-web-permission-gate-20261023",
      "url": "https://platform.claude.com/docs/en/managed-agents/permission-policies",
      "type": "official_docs"
    },
    {
      "id": "claude-managed-web-shared-sandbox-20261023",
      "url": "https://platform.claude.com/docs/en/managed-agents/multiagent-orchestration",
      "type": "official_docs"
    },
    {
      "id": "claude-web-domain-surface-comparison-20261023",
      "url": "https://platform.claude.com/docs/en/agents-and-tools/tool-use/server-tools",
      "type": "official_docs"
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

# Claude Managed Agents の web 制限と更新境界

## 問いと採用判断

調査agentに許可したサイトが取得できないとき、どの制限が働いたかを区別できるか。また、長いsessionの途中で取得先を狭めたいとき、「設定を保存した」ことと「そのsessionに適用できた」ことを混同していないか。

**本書の設計判断は、到達先・URLの出所・実行承認・設定世代を別々に管理すること。** `allowed_domains` を一か所へ書くだけでは、組織設定、cloud環境、委譲元の制限を上書きできない。失敗時に `unrestricted` や別toolへ自動で切り替えるのではなく、制約を維持したまま原因を分類する。

[一般的なtool実行ループ](tool-permissions-failure-loop.md)とは異なり、対象はClaude Managed Agentsのhosted web toolである。[SSRF対策](../security/ssrf-fetch-dns-redirect-boundary.md)のDNS・接続先検証を実装したという話でもない。以下は取得した公式契約の要約と独自の導入案であり、実APIを動かした結果ではない。

## 変更日と適用範囲

[release notes][release]で確認した変更を、記事の見出し日と実際の適用日を分けて読む。

| 日付 | 本書に関係する変更 |
|---|---|
| 2026-10-07 | cloudの `limited` networkingがhosted `web_search` / `web_fetch`にも適用される。session作成・更新時に一部の不整合を400で拒否する。web_fetchのprior-context制限も記載された |
| 2026-10-20 | 組織のweb fetch有効化・domain設定がManaged Agentsにも適用されることを告知。redirect先も組織とtoolの両方を通る必要がある |
| 10月20日記事が説明する先行適用 | web searchの組織有効化設定は10月6日から、組織domain制限は10月14日から適用されていた |

したがって、以前に通ったagent設定や固定したSDKだけを根拠に、取得経路が変わっていないとは判断しない。一方、すべてのfieldが10月20日に追加されたとも書かない。

対象guideのbeta headerは `managed-agents-2026-04-01`、toolsetは `agent_toolset_20260401`。`multiagent_20261001` のworkflowが開いている場合の更新制約も確認した。これらはHTTP API versionの代替ではない。本書は特定SDKの最低版、全region、self-hosted sandbox内部の実装、他社クラウド経由の同等性を保証しない。取得日は2026-10-23 UTCで、guide自体の独立した公開・更新日は表示されていなかった。

## 1. 一つの「許可リスト」へ平坦化しない

[Web tool restrictions][filters]では、組織とagent側の許可条件を同時に満たす必要がある。predefined subagentについても、自身、呼出元、session agentの各制限が重なる。allow側はすべてを満たす範囲、block側はどれかが拒否する範囲になる。子は範囲を狭められるが広げられない。許可範囲がなくてもtoolは残り、callごとに `url_not_allowed` となり得る。

本書の概念モデルでは、あるhostに対し「組織の条件」「そのthreadまでのtool条件」「適用される環境条件」がすべて真かを評価する。これは公式の内部評価順序やAPI schemaではない。複数条件に同時違反した際、どのerrorが先に現れるかは未確認である。

重要なのは、**ドメイン文字列の配列を単純に集合積にしない**ことだ。たとえば親の `docs.example.com` と子の `example.com` は文字列として一致しなくても、hostとしての許可範囲には重なりがある。この解釈は後述のhost照合規則を組み合わせた設計上の推論であり、providerへの境界試験を別に用意する。

### web_search と web_fetch を同じ結果型にしない

[filters]で確認した拒否の形は次のとおり。

- searchは許可外の結果を除外する。空の検索結果だけで「対象サイトに資料がない」と断定しない。
- fetchのdomain拒否は `agent.tool_result` の `is_error: true` と `url_not_allowed` で観測する。
- 組織がweb toolを無効化した場合も、Managed Agentsではtoolエラーとなり、sessionは継続する。[filters] Messages APIでweb fetchを含むrequestが400になる場合と分ける。[release]

独自の監視案として、取得成功数だけでなく、組織設定の変更時刻、tool別の拒否数、検索母集団の欠落を記録する。「sessionがまだrunningだから取得できた」「例外が返らなかったから全候補を調べた」という判定を避ける。

## 2. allowed_domains と allowed_hosts の照合規則は違う

[Server tools][server]のdomain filteringでは、toolの `example.com` はsubdomainも含む。`www.` は特別扱いするprefixではなく、host名の一部として考える。web_searchのpath指定とweb_fetchのdomain照合も同一ではない。Messages APIではpath付きのfetch entryはURLに一致しないという説明だが、Managed Agentsのfetch entryはpathを含められない。[filters]

一方、[Cloud environment setup][network]の `allowed_hosts` は完全一致で、`*.example.com` だけがsubdomainを表し、bare `example.com` 自身は含まない。tool設定用の表現を環境へそのままコピーしない。

| 設定 | docs.example.com | example.com | www.docs.example.com |
|---|---:|---:|---:|
| toolの `allowed_domains: ["docs.example.com"]` | 含む | 含まない | 含む |
| 環境の `allowed_hosts: ["docs.example.com"]` | 含む | 含まない | 含まない |
| 環境の `allowed_hosts: ["*.example.com"]` | 含む | 含まない | 含む |

表は両資料の照合規則から作った説明用の例で、実通信試験ではない。実際の取得には他の条件も残る。

`limited` では `allow_package_managers` / `allow_mcp_servers` による追加許可はhosted web toolへ引き継がれない。`allowed_hosts` が空ならページ・検索結果は返らない。web用にhostを追加するとsandboxからも到達できるため、検索だけを許可したつもりでshellの送信先まで増やす変更になる。[network]

`allowed_domains` のentryが環境の範囲外なら、session作成・該当更新は400になる。たとえばtoolに `docs.example.com`、環境にbare `example.com` だけという設定は不整合である。[filters]

**独自の採用案:** 許可対象を「文書サイトを読む」だけで記録せず、host、tool、環境、対象session、変更理由をひと組でレビューする。必要なhostが分からないことを理由に、検索担当へpackage registry全体や任意外向き通信を自動で与えない。

## 3. domainを通っても prior context と承認は別に残る

10月7日の[release]は `web_fetch` が受け付けるURLの出所を限定する。user message、web_search結果、先にfetchしたページ等が対象で、モデル自身の出力、system prompt、添付文書、bash/read/MCPの結果だけにあるURLは対象外とする。後者の拒否は `url_not_in_prior_context` で、hostの許可違反とは別である。

この違いから、次の二つを分離する。

- `url_not_allowed`: 到達範囲の診断対象。無制限通信へ切り替えて解消しない。
- `url_not_in_prior_context`: URLの出所の診断対象。domain listを広げても原因の修正にならない。

本書の設計案として、外部文書から抜いたURLを機械的に `user.message` へ載せ直す処理は設けない。文書内容を利用者の意図へ昇格させず、必要ならURLと取得目的を確認する。許可されている別の証拠で続ける場合も、未取得のページを取得済みとして引用しない。

[Permission policies][permissions]は別の実行gateである。`always_allow` は人の確認を待たない設定であり、domain条件を消す設定ではないと整理する。`auto` はcallを実行・拒否・確認待ちに分けるが、人が必ず見る仕組みではない。custom toolはこのpolicyの対象外で、実行判断はアプリ側が持つ。

[Tools][tools]ではtoolsetを含めるとbuilt-in toolは既定で有効になる。全面禁止が必要なら `enabled: false` を使い、最小構成なら `default_config.enabled: false` から必要なtoolだけを有効化する。空のdomain配列を全面禁止の表現にしない。Managed Agentsでは空配列は無効で、省略・nullはそのfieldによる制限を置かない意味になる。[filters]

ここでの「無効化」はproviderの設定契約であり、外部操作の承認方針を本書が決めるわけではない。実行権限を確認したうえで設定する。

## 4. mid-session更新は保存先と受付条件を確認する

[Session operations][sessions]で確認した事項:

1. sessionの `agent.tools` / `agent.mcp_servers` を変えても、元agentの定義は変わらない。
2. `tools` と `mcp_servers` はそれぞれ、送信した配列の**全置換（full replacement）**である。`tools` 内のMCP toolset entryと、別配列 `mcp_servers` の接続定義を混同しない。現在のsessionをGETし、更新する各配列で残すentryを維持する。
3. 更新にはsessionが `idle` であることが必要。runningならinterrupt後にidleを待つが、それだけで十分とは限らない。
4. runningでもpausedでも、workflow runが開いている間はagent設定更新が400になる。

[Multiagent orchestration][multi]は、元agentの後日の変更が既存sessionへ入らないことを説明している。したがって「新規session用のagentを修正」「既存sessionを修正」は別作業として追う。またcontextが別threadでも、sandbox・filesystem・vault credentialsは共有される。web制限の合成をtenant隔離の保証へ読み替えない。

### interrupt を「全run終了」に置き換えない

[Workflow runs][runs]では、`user.interrupt` はagentのturnを止めるがrunを終了しない。runはpauseする場合も続く場合もある。`workflow_run_open` の400に対して同じ設定更新を連打せず、すべてのrunの `workflow_run.status_ended` を確認するか、許可された手順で停止を求める。

停止要求にも前提がある。`requires_action` なら待機中toolへの応答が先に必要で、`budget_reached` ではuser messageが400になり得る。budgetを増やす・外す操作は、停止中のrunを再開させることがある。設定変更を通すためだけに予算を増やす回復処理は採用しない。途中のphase終了やsession idleをrun終了へ読み替えない。[runs]

[filters]が説明するsession内の新しいdomain設定は、各threadの次のturnから適用される。predefined subagent自身の設定はsession作成時のものを保つ。処理中の取得が遡って取り消される、保存済み本文が消える、という保証にはしない。

## 独自の更新手順案

以下はproviderが提供するtransaction手順やcompare-and-swap APIではなく、本書の運用案である。

1. **目的を固定する。** 変更対象が組織、元agent、既存session、環境のどれかと、増える到達先・減る到達先を記録する。
2. **受付条件を確認する。** session status、未回答tool、open runを分けて調べる。状態が不明なら「更新待ち」とし、完了を報告しない。
3. **現在値から差分を作る。** tools全置換で既存のMCP toolset entry、enabled、permission_policyが意図せず消えないよう比較する。mcp_serversを送る場合は、その配列の既存接続定義も別に比較する。同時編集の検出方法は自社側で設計し、未確認のETagやrevision条件をAPIへ仮定しない。
4. **許可された範囲だけ更新する。** 400を一律に一時障害へ分類しない。domain不整合、open run、設定形式を別に修正し、無関係な権限を広げない。
5. **read-backで一致を確認する。** sessionと元agentを必要に応じて別々に確認する。保存成功だけで全threadへ即時反映したとは言わない。
6. **新しい条件で継続を検証する。** 次のturnで安全な許可先と拒否先を試す試験を別に用意する。実行中に読み込まれた情報の利用まで止める必要があるなら、設定更新だけで要件を満たすとしない。

初回保存時に有効だった設定がtool初期化時に無効になっていれば、`session.error` の後idleへ戻り、自動retryしない場合がある。[filters] その場合もidleを成功終了とせず、承認された設定修正と新しい入力の要否を判断する。

## 導入時の受入れ試験案

以下は未実施のアプリ試験であり、KBの検索evalとは別である。

| ケース | 観測したい判定 |
|---|---|
| 組織はblock、agentはallow | agent側の指定で組織の制約を広げない |
| 許可URLから禁止hostへredirect | 起点だけの判定で取得成功にしない |
| 親と子のallow範囲が重ならない | toolの存在を到達可能の証拠にしない |
| domain表記は違うがhost範囲が重なる | 文字列配列の集合積だけで誤拒否しない |
| toolはdocs.example.com、環境はbare example.com | 作成・更新400と、取得途中のtoolエラーを区別する |
| 環境はpackage manager許可のみ | hosted webの到達先へ自動転用しない |
| 許可hostのURLがMCP結果にしかない | url_not_in_prior_contextをdomainエラーへ丸めない |
| domain配列が空 | 全面禁止の意図で投入せず、enabledで表す |
| 更新配列にMCP entryを入れ忘れる | 差分レビューで検出し、保存へ進まない |
| idleだがworkflow runはpaused | workflow_run_openを解消済みと判定しない |
| interrupt後にrun終了が未観測 | 設定更新完了を宣言しない |
| 元agentだけ修正した既存session | read-backで別設定のままであることを検出する |
| custom toolで独自HTTP fetchを実装 | built-in web policyが守ってくれると仮定せず、自前gateを通す |

単に成功率を測るだけでなく、「どの段階で拒否したか」「未取得の対象を回答に混ぜなかったか」「設定変更のために権限を余分に広げなかったか」を評価する。host照合の例はASCIIの予約example domainを用い、秘密情報をURLへ埋めた試験は行わない。

## provenance・未確認事項

9件の公式ページをnative webで開き、release日・beta識別子・適用面を照合した。公開された再利用ライセンスは確認できずcatalogのlicenseはunknown。文章・コード・図を転載せず、必要なfield名を使った独自要約と判断案を記した。repository実装の分析、コード導入、module昇格はない。

release_notesの30日TTLに合わせ、再確認期限は2026-11-22とした。検索evalは本文が検索できることのみを確認し、providerの安全性や上表の実試験成功を示さない。

未確認なのは、policy間の内部評価順序、組織設定変更の伝播遅延、実行中callの取消保証、redirectとURL prior-context判定の細かな相互作用、subagentへのURL出所引継ぎ、SDK最低版、各配備先の同等性、DNS再束縛等に対するprovider内部実装、同時session更新の競合制御である。特にhost制限は操作・送信データの制限と同じではない。必要な場合は、権限のある検証環境でこれらを別途確認する。

[release]: https://platform.claude.com/docs/en/release-notes/overview
[filters]: https://platform.claude.com/docs/en/managed-agents/tools-web-restrictions
[network]: https://platform.claude.com/docs/en/managed-agents/environments
[sessions]: https://platform.claude.com/docs/en/managed-agents/session-operations
[runs]: https://platform.claude.com/docs/en/managed-agents/workflow-runs
[tools]: https://platform.claude.com/docs/en/managed-agents/tools
[permissions]: https://platform.claude.com/docs/en/managed-agents/permission-policies
[multi]: https://platform.claude.com/docs/en/managed-agents/multiagent-orchestration
[server]: https://platform.claude.com/docs/en/agents-and-tools/tool-use/server-tools

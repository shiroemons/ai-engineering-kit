---
{
  "id": "openai-compaction-approval-replay-boundary",
  "title": "OpenAI Agents SDK 0.24.0: 承認待ちcompactionの延期とRunState replayの境界",
  "kind": "knowledge",
  "technology": "agents",
  "version": "openai-agents v0.24.0 (2026-10-15) @ 17d994bd3045dae71acda676a4ef4762ed8ecd94; approval-compaction fix 5eb47a5bdeab8ba9f6087de288c7e606816c76c6; rolling guides checked 2026-10-24 UTC; runtime未検証",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "openai",
    "compaction",
    "approval",
    "pending-approvals",
    "RunState",
    "replay",
    "session",
    "streaming",
    "hosted-mcp",
    "manual-compaction"
  ],
  "sources": [
    {
      "id": "openai-agents-compaction-release-0240-20261024",
      "url": "https://github.com/openai/openai-agents-python/releases/tag/v0.24.0",
      "type": "release_notes"
    },
    {
      "id": "openai-agents-compaction-source-17d994b-20261024",
      "url": "https://raw.githubusercontent.com/openai/openai-agents-python/17d994bd3045dae71acda676a4ef4762ed8ecd94/src/agents/memory/openai_responses_compaction_session.py",
      "type": "github_repository_analysis"
    },
    {
      "id": "openai-agents-compaction-fix-5eb47a5-20261024",
      "url": "https://github.com/openai/openai-agents-python/commit/5eb47a5bdeab8ba9f6087de288c7e606816c76c6",
      "type": "github_repository_analysis"
    },
    {
      "id": "openai-agents-compaction-maintainer-report-5363-20261024",
      "url": "https://github.com/openai/openai-agents-python/pull/5363",
      "type": "github_repository_analysis"
    },
    {
      "id": "openai-agents-hitl-compaction-resume-20261024",
      "url": "https://openai.github.io/openai-agents-python/human_in_the_loop/",
      "type": "official_docs"
    },
    {
      "id": "openai-agents-sessions-compaction-scope-20261024",
      "url": "https://openai.github.io/openai-agents-python/sessions/",
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

# 承認待ちの履歴を「終わった会話」としてcompactしない

## 問いと採用判断

人の承認を待って停止したagent runに、保存履歴を短くするautomatic compactionを組み合わせたとき、未解決のtool callを保持したまま承認画面へ戻り、後で安全にresumeできるか。

**承認待ちは、履歴を捨ててよい完了状態ではない。** OpenAI Agents SDK 0.24.0の修正は、要約の品質ではなく、未解決callをcompact APIへ送ることと、再開に必要な履歴を置き換えることの境界を扱う。pending approvals中はautomatic compactionを延期し、承認または拒否をRunStateへ反映して再開する。callと応答がそろった後も、ほかの候補選択条件を満たすまで圧縮が走るとは限らない。

[長時間ループのコンテキスト管理](long-loop-context-growth-compaction.md)はClaudeの要約とOpenAIのTool Output Trimmerが中心で、OpenAI SDKのcompaction機構を未確認としている。本書はそのうち**承認待ちcheckpointと再開可能な履歴の保全**に限定する。[Claude eager tool streaming](claude-eager-tool-streaming-approval-retry-boundary.md)の早期実行や、[Responses Multi-agentのresponse完了](openai-multi-agent-response-completion-boundary.md)とは別のSDK契約である。

## 適用版と修正された失敗

- [v0.24.0 release notes][release]は2026-10-15付。調査対象はrelease commit `17d994bd3045dae71acda676a4ef4762ed8ecd94`であり、最新版や他言語SDKの保証ではない
- 修正は[固定commit 5eb47a5][fix]。従来は限定的なsnapshotに対して行っていたtool-group検査を、完全な履歴にも適用する。hosted MCPの承認pairには別のID照合を追加した
- [修正作者のPR報告][report]は2026-10-13付。v0.23.1と当時のbaselineで、未解決callがcompact APIへ送られ、**approval interruptionを呼出し元へ返す前にHTTP 400**となったと報告する。修正後は承認・拒否の双方で次のuser turnまで進めたという報告である
- この再現結果はupstreamが実行したもの。本調査はnative webで資料・固定実装・テストdiffを読んだ静的確認であり、同じAPI実験を実行していない。PRが明記する変更対象はautomatic compactionで、manual compactionとpublic APIは変更していない

運用上の判断: この条件に当たる旧版で、HTTP 400を「ユーザーが拒否した」「toolが失敗した」と表示しない。承認UIへ到達できないcompaction経路の障害を切り分ける。単なるretryで解消すると決めつけず、版、承認待ちcallの有無、compact送信の時刻を先に確認する。この切り分け手順は本書の設計提案である。

## 固定実装で確認したautomaticの受入れ条件

[OpenAIResponsesCompactionSessionの固定実装][implementation]における観察をまとめる。内部メソッド名やprivate fieldをアプリから直接操作する提案ではない。

1. `should_trigger_compaction`は「圧縮を検討する」条件であり、送信を必ず行う保証ではない
2. automatic経路は保存履歴を直近のmodel exchangeと照合する。対応づけられない履歴は、無条件に要約対象へ広げない
3. tool groupから孤立したcall等を除いた結果が元の候補と変わる場合、候補を書き換えて送るのではなく、そのautomatic compactionを中止する。未解決callは後の承認・拒否のために保持する
4. hosted MCPではfunctionのcall IDだけでは足りない。承認request IDの集合と、`mcp_approval_response`が持つ`approval_request_id`の集合が一致しなければ進めない
5. 上記検査はcompact API呼出しより前にあり、automatic分岐の内部にある。閾値到達を理由に未解決pairを消す設計にはしない

ここでの「延期」は、承認・拒否をSDKが自動決定することではない。また、IDの集合一致はこの実装の候補検査であって、任意の改変済みsnapshotについて内容・所有者・承認者の真正性を証明する仕組みではない。

## 承認待ちから次turnまでを一つの経路として扱う

[Human-in-the-loop guide][hitl]が示すpublic flowは、resultの`interruptions`を確認し、`result.to_state()`でRunStateを得て、対象をapproveまたはrejectして元のtop-level agentでresumeするもの。nested agentの承認も外側runで扱う。

streamingでは、最後のtext tokenを見ただけでこの処理へ移らず、`stream_events()`のiteratorが終わるまで消費してから`interruptions`を読む。同じsession instance、または同じsession IDと同じ保存backendを使い続ける。RunStateの代わりに要約文だけを新規入力として渡す方法は、このresume flowではない。

本書のアプリケーション設計案では、次を別々に記録する。

| 段階 | 保存・確認すること | 同一視しないこと |
|---|---|---|
| 承認待ちを検出 | runの識別子、pending call、再開用RunState、sessionの対応 | compactの失敗とユーザーの拒否 |
| 承認または拒否を受理 | 認証・権限を確認した判断と対象callの対応 | UIボタンのクリックとtool実行完了 |
| resume | 同じ保存履歴を使った再開結果、追加のinterruptions | 一度の承認と以後すべての承認 |
| 後続turn | call/応答を含むreplay、圧縮が実施されたか | pair完成と即時compactionの保証 |
| 履歴置換後 | 次回入力が置換後履歴を参照できるか | summaryの生成と保存成功 |

[固定修正の回帰テスト][fix]では、streaming有無と承認・拒否を組み合わせ、RunStateをJSON経由で復元する。関数toolのケースはSQLiteと単純なlegacy backendを含む。承認待ち中にcompactが未呼出しであること、再開時にcall/応答pairが残ることを検査した後、さらに次turnでcompactし、その後のturnがsummaryを再利用することを検査している。hosted MCPもrequestと判断responseの対応を確認する。**「approveを呼んだ瞬間にcompactする」というテストではない。**

## manual compactionはautomaticの代替スイッチではない

`run_compaction()`を直接呼ぶmanual経路、特に`force`を指定する運用に、今回のautomatic延期検査が同じように適用されるとは考えない。[固定実装][implementation]の分岐と[作者報告][report]の適用範囲が根拠である。

設計提案: 承認待ちの待ち時間が長いからという理由だけで、そのsessionへmanual compactionを差し込まない。別の定期保守処理が圧縮する場合も、pending runとの所有関係をアプリ側で確認し、再開可能な保存状態を保つ。未解決callを履歴から手作業で除き、compactを通してから同じcallを作り直す処理は、元の承認対象や実行済み副作用との対応を壊すおそれがある。

一方、automaticが延期された事実だけで実行バグと判断しない。modelに見えていない保存item、候補の境界、履歴変更等により、承認pair以外の条件で延期される場合もある。調査ログでは「未実施」「送信失敗」「保存失敗」を区別し、単一のcompaction失敗カウンタへ混ぜない。これは診断のための独自提案で、SDKがこの分類の監視指標を提供するという主張ではない。

## 保存・並行性・認可の別境界

[Sessions guide][sessions]は、automatic compaction完了を待ってRunnerがreturnし、stream iteratorも閉じると説明する。したがって表示上の最後のtokenとrun完了は同時とは限らない。OpenAIResponsesCompactionSessionはOpenAIConversationsSessionを包む構成には対応しない。

同guideのwrapper内mutation直列化や履歴のrecoveryを、**全worker共通の排他や無条件の復旧保証へ拡張しない**。underlying sessionの直接変更はwrapperの保証を迂回し、復旧時にbackendが失敗すれば旧履歴が戻らない場合もある。本書は、同じsession IDを使う別process・別wrapperの全競合が安全だと検証していない。pause/resumeの重複送信とcompactionの競合は、別に扱う必要がある。

[HITL guide][hitl]では、復元API自体はsnapshotや提出者を認証しない。serverが所有する信頼できるRunStateを保持し、承認者と対象runの権限を検証する。長く残るpending taskにはSDK・agent定義等の版識別子も保存して、適合するコードで再開する方針が示される。これらは今回のcompaction修正を入れても省略できない。

## 導入時の受入れ試験案

以下は自環境用の設計案であり、この調査で実行済みのテスト一覧ではない。モデル応答を制御できるfixtureと、必要に応じた実API検証を分ける。

| 条件 | 失敗させるべき取り違え・観測点 |
|---|---|
| 常にtrueのtrigger、未承認の関数call | trigger成立だけでcompactが送信されない。interruptionとpending callが残る |
| 承認と拒否の両方 | 実行許可の有無にかかわらず、判断後のreplayが壊れない |
| hosted MCP承認待ち | function call IDではなく承認request IDとresponseの対応を検査する |
| streamと非stream | streamの最終tokenだけで状態を確定せず、iterator終了後のinterruptionsを扱う |
| serialize・restoreを挟むresume | 同じsession保存先へ戻り、同一pending callへの判断として続く |
| 再開後の次turnと圧縮後の次turn | compact成功だけで止めず、置換した履歴が実際に再利用できる |
| 手動圧縮の保守jobが並行する | automatic検査への依存で安全とせず、pending runを対象から除外する運用を確認する |
| 同じ承認の重複提出 | 一つのRunStateを重複resumeしない。既に起きた副作用を確認せず再実行しない |
| 復旧中のbackend障害 | rollbackを当然視せず、元履歴・置換履歴・不確定状態を調べる経路を持つ |
| SDK更新中に古い承認が到着 | 保存した版識別子で互換性を確認し、無条件に新しい定義で復元しない |

## provenance・確認範囲・未確認事項

- release、固定実装、修正commit、作者PR報告、公式guide 2件を2026-10-24 UTCにnative webで実際に開いた。実装分析は40桁commitで固定した。guideの公開日は表示を確認できないため、取得日と混同しない
- [同releaseのLICENSE][license]はMIT。コードは転載せず独自に要約した。deployed guide、release、PR本文の転載条件は独立に確定できずcatalogはunknownとした。固定diffの読解、作者の実験報告、本書の運用案を区別する
- 確認済みは資料と静的実装の照合。upstream SDKテストのローカル実行、実APIのHTTP 400再現、実保存backendの障害注入、圧縮精度・費用・latencyの実測は未実施。採用前に使用中のモデル・backend・wrapper構成で確認する
- hosted MCPのID集合検査を、任意の不正履歴を完全検証するvalidatorとして使えるとは確認していない。別process間の全履歴更新の直列化、強制終了時の復旧、他言語SDKへの同時導入も未確認
- release_notesのTTL 30日に合わせ、明示期限は2026-11-23。修正自体がその日に無効になる意味ではなく、API・SDK・guideの適用条件を再確認する期限である

[release]: https://github.com/openai/openai-agents-python/releases/tag/v0.24.0
[implementation]: https://raw.githubusercontent.com/openai/openai-agents-python/17d994bd3045dae71acda676a4ef4762ed8ecd94/src/agents/memory/openai_responses_compaction_session.py
[fix]: https://github.com/openai/openai-agents-python/commit/5eb47a5bdeab8ba9f6087de288c7e606816c76c6
[report]: https://github.com/openai/openai-agents-python/pull/5363
[hitl]: https://openai.github.io/openai-agents-python/human_in_the_loop/
[sessions]: https://openai.github.io/openai-agents-python/sessions/
[license]: https://github.com/openai/openai-agents-python/blob/17d994bd3045dae71acda676a4ef4762ed8ecd94/LICENSE

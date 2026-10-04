---
{
  "id": "agents-gemini-antigravity-tool-migration-hook-boundary",
  "title": "Gemini Antigravity 09-2026: tool名変更・行範囲編集・hook適用範囲の移行境界",
  "kind": "knowledge",
  "technology": "agents",
  "version": "Gemini API v1beta/interactions: antigravity-preview-05-2026 → antigravity-preview-09-2026 (release 2026-09-17; old endpoint shutdown notice 2026-10-05); rolling docs last updated 2026-09-24/09-29/10-01, verified 2026-10-04 UTC; runtime not tested",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "gemini",
    "antigravity",
    "tool-migration",
    "local_environment",
    "PascalCase",
    "replace_file_content",
    "view_file",
    "hook",
    "matcher",
    "fail-open",
    "function_result",
    "previous_interaction_id"
  ],
  "sources": [
    {
      "id": "google-antigravity0926-release-20261004",
      "url": "https://ai.google.dev/gemini-api/docs/changelog",
      "type": "release_notes"
    },
    {
      "id": "google-antigravity0926-deprecations-20261004",
      "url": "https://ai.google.dev/gemini-api/docs/deprecations",
      "type": "official_docs"
    },
    {
      "id": "google-antigravity0926-guide-20261004",
      "url": "https://ai.google.dev/gemini-api/docs/antigravity-agent",
      "type": "official_docs"
    },
    {
      "id": "google-antigravity0926-hooks-20261004",
      "url": "https://ai.google.dev/gemini-api/docs/agent-hooks",
      "type": "official_docs"
    },
    {
      "id": "google-antigravity0926-interactions-20261004",
      "url": "https://ai.google.dev/gemini-api/docs/interactions-overview",
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

# Gemini Antigravity 09-2026: tool名変更・行範囲編集・hook適用範囲の移行境界

## 問いと適用範囲

Gemini API の managed agent を `antigravity-preview-05-2026` から `antigravity-preview-09-2026` へ切り替えるとき、agent IDだけの更新で足りるのは誰か。local executor、tool履歴parser、remote sandboxのhookが、異なる処理を旧契約で実行したり見逃したりしないための判断を整理する。

対象は Google の **Gemini API /v1beta/interactions のAntigravity agent**。Antigravity IDEそのものの更新手順ではない。公式ガイドはIDEと同じharnessを使うと説明するが、製品の更新・設定を同一視しない。[Antigravity agent guide](https://ai.google.dev/gemini-api/docs/antigravity-agent)

## 確認できた契約

### 1. 版と停止告知を、実停止の観測から分ける

2026-09-17の[release notes](https://ai.google.dev/gemini-api/docs/changelog#september-17-2026)は09版を公開し、05版を置換・非推奨化して、05版のshutdownを2026-10-05と告知する。[deprecationsのManaged agents表](https://ai.google.dev/gemini-api/docs/deprecations#managed-agents)も同じ日付と後継09版を掲げる。ただし同ページ全体の注記は表の日付をearliest possibleと説明している。

したがって2026-10-04の確認結果は「10月5日の停止告知があり、移行対象」である。「既に停止した」「必ずUTC 00:00に止まる」「10月5日以降も利用できる」のいずれも確認していない。endpointへの実リクエストは行っていない。

### 2. remote output-only とtool契約を読む実装は移行範囲が違う

同release entryは、remote sandbox (`environment: "remote"`) で `output_text` / `model_output` だけを読む場合、agent文字列の更新でよいとする。一方、`local_environment` でtoolを実行する実装、または `function_call` を解析する実装では組み込みtoolの契約が変わる。

| 操作 | 05版の表記 | 09版の表記・変更点 |
|---|---|---|
| 作成 | `write_file(path, content)` | `write_to_file`: `TargetFile`, `CodeContent`, `Overwrite`, `Description` |
| 編集 | `write_file` による全体書換え | `replace_file_content`: `TargetFile`, `StartLine`, `EndLine`, `TargetContent`, `ReplacementContent`。行範囲置換 |
| 読取り | `read_file(path, offset, limit)`、byte offset | `view_file`: `AbsolutePath`, `StartLine`, `EndLine`, `ContentOffset` |
| 一覧 | `list_files(path)` | `list_dir(DirectoryPath)` |
| 検索 | shell commandで実施 | `find_by_name(SearchDirectory, Pattern, MaxDepth)` と `grep_search(SearchPath, Query, IsRegex)` |

これはrelease entryの差分表であり、完全なJSON Schemaではない。file系引数のPascalCase化から、全tool引数を機械的にPascalCase化する規則を作らない。同表では `code_execution(command, timeout_seconds)` と `google_search(queries)` は変更なしとされる。

### 3. 履歴にあるfunction_callを全部client側で再実行しない

[agent guideのFunction calling](https://ai.google.dev/gemini-api/docs/antigravity-agent#function-calling)では、remote環境のfilesystem処理も `function_call` として現れるが、環境側で自動実行される。Python/JavaScript例は `function_result.call_id` の集合と `function_call.id` を照合し、既に結果のあるcallを除いて、`requires_action` の未応答callを扱う。

この照合は実行主体の判定と組み合わせる必要がある。履歴に `write_to_file` があるという理由だけでlocal diskへ同じ操作を再実行してはならない。通信障害で結果が未確認の場合も、call未実行の証明にはならない。

同guideのLimitationsはagentのfunction callingをstateful限定とし、続行には `previous_interaction_id` を要求する。公式例は同じ `environment_id` も渡す。[Interactions overview](https://ai.google.dev/gemini-api/docs/interactions-overview#server-side-state-management)の一般APIはstatelessも扱うが、その一般論でagent固有の制約を解除しない。また履歴IDが引き継ぐのは会話履歴で、`tools` 等のrequest単位設定をすべて継承する保証ではない。

### 4. hookの名前一致・拒否・事後処理を別々に確認する

[Hooks guide](https://ai.google.dev/gemini-api/docs/agent-hooks#matcher-syntax-and-rules)はcontainer tool名をRE2 matcherで照合する。`.*_file` は `replace_file_content` と `list_dir` を含まない。新しい編集操作を監査するなら名前を明示して被覆する必要がある。

同guideの契約上、pre hookの明示 `deny` は実行を止めるが、post hookは完了した作用を取り消せない。crash・timeout・失敗や、認識されない出力は実行を許す **fail-open** になる。custom function / external MCPはcontainer hookの対象外である。書込み可能なhook設定も不変の権限境界とはいえない。[Hooks limitations](https://ai.google.dev/gemini-api/docs/agent-hooks#limitations)

**文書内の不一致の観測**: 同じHooksページのaudit logging例ではPython/JavaScript/RESTが新tool名を使う一方、Java/Go例のmatcherには `read_file|write_file` が残っていた。例の言語を選ぶだけで09版の監査範囲が保証されると扱わず、本文の名前一覧と実際のtraceを照合する。 またprivacy例は、matcherが `view_file` のPython/JavaScriptでも `tool_call.args.path` を読むが、release差分表は `AbsolutePath` を示す。hook payloadで引数名が正規化されるかは未検証であり、runtime/SDKの欠陥と断定しない。matcher更新だけで引数検査まで移行済みとせず、実payloadと拒否判定を確認する。

## 移行の判断手順（本書の設計案）

以下は独自の推奨であり、Google実装の保証ではない。

1. **consumerを棚卸しする。** 表示UIだけでなくaudit、承認、編集diff、local executorがtool名・引数に依存していないか調べる。remote利用という一点で全componentを「ID変更だけ」と判定しない。
2. **agent版とadapter版を一組にする。** 09版を旧handlerへ流さず、tool名ごとに明示dispatchする。未知tool・必須field欠落はadapter errorとし、shellや全体書換えへ自動fallbackしない。元の引数を保持し、一律key変換を避ける。
3. **編集を全体書換えへ縮退させない。** `ReplacementContent` をファイル全体の新内容として書くと対象外の行を失う。`TargetContent` をCAS保証と推測せず、local実装では許可path、読取り時revision、期待内容、書込みの排他を独立に検査する。競合なら再読取り・再計画へ戻す。
4. **read単位を推測しない。** 旧byte offsetを `StartLine` に代入しない。Unicode・CRLF・末尾改行なしをfixtureにし、`ContentOffset` の単位や行端の包含規則を利用する実装のschemaと照合する。未確認のままadapterを配備しない。
5. **拒否が届く経路を試験する。** 新編集名をhookへ通し、正常deny、matcher未一致、timeout、壊れた出力を分けて観測する。hookがfail-openでも守る必要のある書込み禁止は、toolへ渡す権限やOS/container制御で独立に施行する。post hookの失敗を「編集は未実行」と扱わない。
6. **実行と結果返却を記録する。** 実行主体・interaction ID・call ID・作用結果・返却状態を対応付ける。成功済みcallの再受信は記録照会へ進め、結果返却の通信失敗だけで再実行しない。idは相関キーであって、外部作用のexactly-once保証ではない。

## 回帰確認の具体例

以下は配備前に行う実行テストの提案。今回実行した検索evalとは異なり、サービス実機で合格した記録ではない。

| 入力・状況 | 確認する結果 |
|---|---|
| remote履歴にfilesystemのcall/resultと未応答custom callが混在 | 実行済みfilesystemの二重実行なし。対象custom callだけを承認・処理 |
| 09版の行編集を旧write handlerへ渡す | schema/dispatch段階で拒否。周辺行を消す全体上書きなし |
| view_fileへ旧byte offsetをそのまま流す | 単位不一致を検出し、行番号として黙認しない |
| 同一fileが読取り後に別writerで変更された | revision不一致を検出。古いTargetContentで上書きしない |
| matcherが旧名のみ、または `.*_file` のみ | 新編集toolの監査漏れを検出し、承認済みとは判定しない |
| hook timeout / malformed output / post deny | fail-openと事後取消不可を再現し、独立した権限制御の有効性を確認 |
| local action成功後に結果返却がtimeout | execution_unknownとdelivery_unknownを区別。作用を無条件再実行しない |
| 05版のままshutdown告知日を迎える | 成功を仮定せず、移行済み09版か、明示した停止・手動対応へ分岐 |

## 限界・鮮度・重複との区別

- 詳細未確認: `StartLine` / `EndLine` の基数と包含規則、`ContentOffset` の単位、`TargetContent`不一致時の正確な応答、atomic write / CAS /再送冪等性、local_environmentの全schema。release差分表だけでは確定しない。IDEの同名toolの挙動も根拠に転用しない。
- 新旧agentをまたぐ進行中interactionの再開互換性は未確認。一般APIの会話連結機能から互換を推測しない。利用SDKの最小対応版、shutdownの実施時刻と実際の稼働状態も未検証。
- 既存の[tool権限と失敗ループ](tool-permissions-failure-loop.md)は一般的な実行権限と結果対応付け、本書は2026-09-17の具体的なtool ABI移行と監査hook被覆を扱う。Claude/MCP機能やEvals終了の記事を分割・言換えしたものではない。knowledge・patterns・modules全218文書のtitle/technologyと本文keywordを横断確認し、書込み前AND検索でも同一テーマはなかった。
- 出典はGoogle一次資料5件を2026-10-04 UTCに開いて確認。release entryは2026-09-17、ページ更新日はchangelog/deprecations/Interactions overviewが2026-10-01、agent guideが2026-09-29、Hooksが2026-09-24。ページ更新日は個々の機能導入日を意味しない。
- 各footerは文書をCC-BY-4.0、sample codeをApache-2.0と表示する。本書は出典を付した独自要約・設計案でありコードはコピーしていない。`trust: primary-source` は独自設計案を含むため。release_notes TTL 30日に合わせ再確認期限は2026-11-03。これは停止予定の延期・利用猶予を意味しない。

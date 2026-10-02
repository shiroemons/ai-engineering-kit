---
{
  "id": "performance-k6-scenario-selection-threshold-coverage",
  "title": "k6 v2.3: scenario 選択・once・archive で変わる threshold の検証範囲",
  "kind": "knowledge",
  "technology": "performance",
  "version": "k6 v2.3.0 (2026-09-21), commit e0887846143ab176d4b5483c9d52cf3b3e009f1a; v2.3.x docs verified 2026-10-02 UTC",
  "tags": [
    "research-domain:quality-operations",
    "k6",
    "scenario",
    "once",
    "threshold",
    "archive",
    "smoke-test",
    "coverage",
    "load-test"
  ],
  "sources": [
    {
      "id": "k6-v230-selection-release-20261002",
      "url": "https://github.com/grafana/k6/releases/tag/v2.3.0",
      "type": "release_notes"
    },
    {
      "id": "k6-v23-scenario-selection-20261002",
      "url": "https://grafana.com/docs/k6/v2.3.x/using-k6/scenarios/",
      "type": "official_docs"
    },
    {
      "id": "k6-v23-threshold-selection-20261002",
      "url": "https://grafana.com/docs/k6/v2.3.x/using-k6/thresholds/",
      "type": "official_docs"
    },
    {
      "id": "k6-v230-scenario-once-implementation-20261002",
      "url": "https://github.com/grafana/k6/tree/e0887846143ab176d4b5483c9d52cf3b3e009f1a/internal/cmd",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# k6 の部分実行を全体の合格と取り違えない

## 問いと今回の追加理由

同じ負荷試験scriptを日々の smoke testにも使うとき、CLIの選択だけで何が実行され、どの threshold が残るか。[v2.3.0 release](https://github.com/grafana/k6/releases/tag/v2.3.0) は2026-09-21公開で、`--scenario` と `--once` を追加した。前者は一部の workload を選び、後者は試行規模を変える。その組合せと archive 再実行では、元scriptが同じでも検証した範囲が変わる。

既存の [arrival-rate と測定](k6-coordinated-omission-arrival-rate-latency.md) や [終了結果の回収](k6-completion-exit-code-linger.md) とは別に、本稿は「合格した判定が何を覆うか」を扱う。以下は公式契約、固定実装の観察、それを使った独自の運用提案を区別する。

## 確認した契約

### 選択だけなら負荷と通常lifecycleを保持する

[v2.3.x Scenarios](https://grafana.com/docs/k6/v2.3.x/using-k6/scenarios/#run-selected-scenarios) では、`--scenario inventory,purchase` は明示的な `scenarios` の名前を選ぶ。名前は初期化後の設定に必要で、空・未知の名前はエラー。`default` 関数があるだけでは `--scenario default` の対象にならない。

選ばれた scenario は executor・負荷・開始時刻・exec・env・tags・browser設定を保持する。`setup()` / `teardown()` を含む通常lifecycleも残る。したがって選択は「その関数だけを副作用なく呼ぶ」機能ではない。

[固定版 scenarios.go](https://github.com/grafana/k6/blob/e0887846143ab176d4b5483c9d52cf3b3e009f1a/internal/cmd/scenarios.go) でも、選択とCLIの `--vus`・`--duration`・`--iterations`・`--stage` の併用を拒否する。script / config / 環境変数の対応するトップレベルshortcutは警告付きで除かれ、scenario内の値を使う。単なる選択では execution segment は有効なため、実際の担当負荷がさらに分割され得る。

### threshold は名前付きscenarioの除外に合わせて変わる

[Thresholds](https://grafana.com/docs/k6/v2.3.x/using-k6/thresholds/#set-thresholds-for-specific-tags) と固定版 `dropScenarioThresholds` を照合した。

| threshold の種類 | `--scenario` 選択時の扱い |
|---|---|
| `scenario` tag が設定済み・選択から除外した名前を指す | 削除し警告する |
| `scenario` tag が選択した名前を指す | 残す |
| `scenario` tag が設定にない名前を指す | 残す |
| global、または `scenario` tag のないfilter | 残す |

例えば `inventory` と `purchase` が設定され、`purchase` だけを選ぶと、`errors{scenario:inventory}` の threshold は除外される。`purchase` 側が意図的に `scenario:inventory` のsampleを送っても、その判定は復帰しない。これは「sample が届かなければskipする」という規則ではなく、選択時に判定定義を除く処理である。

共有の `checks` threshold は残るが、`check()` だけではtestの終了状態を失敗にしない。部分実行であっても失敗させたいassertionには、残るthresholdなどの明示的な失敗条件が必要となる。逆に、全workloadの件数を想定したglobal count条件は部分実行で失敗し得る。未知のscenario名を含むfilterも自動で都合よく除かれない。

### once はシナリオ選択とは別の書き換え

[固定版 once.go](https://github.com/grafana/k6/blob/e0887846143ab176d4b5483c9d52cf3b3e009f1a/internal/cmd/once.go) は、対象を `shared-iterations`、1 VU、1 iterationへ変換し、元のexec・env・tags・browser設定を引き継ぐ。Scenarios文書が示す時刻等の値は `startTime: '0s'`、`maxDuration: '10m'`、`gracefulStop: '30s'` である。

`--once` 単独は複数scenarioを拒否する。一方 `--scenario inventory,purchase --once` は**各選択scenarioにつき1回**で、全test合計1回ではない。元の遅延開始・負荷順序は保証されない。CLIの `--execution-segment` / `--execution-segment-sequence` との併用は拒否される。一方script / config / 環境変数のsegment設定は警告付きで除かれるため、元の分割範囲が維持されるとも考えない。releaseの「at most one scenario」という単独モードの説明を、明示選択との合成へ広げない。ここは公開ドキュメント、導入PR、release固定実装を照合した。

### archive は選択結果を保存する

Scenarios文書では `k6 archive` も選択に対応し、残ったscenarioとthresholdをarchiveに保存する。そのarchiveの再実行時に `--scenario` を省略しても、除いたscenarioやthresholdは戻らない。元scriptで全体を再構成してarchiveを作る工程と、既に縮小されたarchiveの再利用を区別する。

## 実務の判断手順（独自の設計案）

1. 成果物を `full-load` / `selected-load` / `selected-smoke` など用途別に識別し、script revisionだけでなく選択名、onceの有無、実効executor、archive識別子、残したthreshold一覧を記録する。ラベル名は本稿の例で、k6の予約語ではない。
2. 部分実行の成功を全体の性能ゲートへそのまま昇格させない。対象外のAPI経路、競合する並列workload、warm-upが必要な経路、選択で削除したassertionは未検証として残す。
3. `--once` では開始時刻が揃うため、元scriptの `startTime` による見かけの順序へ依存したfixtureを使わない。初期化で必要条件を用意するか、選択した各scenarioが独立して動くことを確認する。1回の成功からtail latencyや持続負荷下のSLOを判断しない。
4. 常時評価したい不変条件とscenario固有の性能条件を分ける。たとえば在庫と購入を跨ぐ整合性assertionを、除外可能なscenario名のfilterだけに置かない。共有条件の対象sampleが部分実行でも生成されることを別に点検する。
5. archiveには実効設定の縮小が残るので、CIで使うartifactの用途をmanifest等へ記録する。smoke用archiveからflagを外しただけで夜間のfull-load試験を組み立てない。後者は全scenarioを含む入力から作り直す。
6. wrapperが付加するshortcutと環境変数も調べる。CLI併用のエラーと、下位設定が警告付きで落ちるケースを別に扱い、警告を黙殺して「希望した負荷だった」と判断しない。

## 導入時の受け入れケース（独自の検証案）

| 試験 | 確かめたい結果 |
|---|---|
| 未知名・空選択・暗黙default | 間違った対象を走らせず設定エラーになる |
| 一部選択 + 除外scenarioのthreshold | skip警告を保存し、未実行を成功件数に含めない |
| 選択scenarioから除外名のtagをemit | 消えた判定が復帰しないため、共有条件で検知できる設計か確認 |
| 部分選択 + 全体前提のglobal count | 自動skipを期待せず、意図した成否を明示 |
| 複数選択 + once + 遅延開始設定 | 各1回かつ開始条件が変わること、fixture競合を確認 |
| 選択済みarchiveをflagなしで再実行 | 全体へ戻ったと誤認せず、保存した選択範囲を確認 |

## 適用限界・来歴

版はk6 v2.3.0、文書はv2.3.xを2026-10-02 UTCに取得した。tagの実装commitは `e0887846143ab176d4b5483c9d52cf3b3e009f1a`。GitHub HTMLで固定ファイルを取得できなかったため、同commitの公式raw URLで `scenarios.go`・`once.go`・`scenarios_test.go` と `LICENSE.md` を読んだ。実装の観察を後続版の互換保証にはしない。

この調査ではk6自体、Cloud、browser、分散segmentの実機試験を実行していない。上の受け入れケースは提案であり、追加した検索evalは文書到達だけを確かめる。公開ページの固定公開日は示されていないため、取得日をrelease日と扱わない。

Grafanaの[文書copyright](https://grafana.com/docs/copyright-notice/)はall rights reserved。k6実装は同commitのAGPL-3.0を確認した。本文は帰属付きの独自要約で、コードの転載・改変・module化はしていない。release_notesのTTL30日を最短期限として2026-11-01に再確認する。

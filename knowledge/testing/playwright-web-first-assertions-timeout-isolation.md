---
{
  "id": "testing-playwright-web-first-assertions-timeout-isolation",
  "title": "testing Playwright web-first assertions の auto-waiting と timeout scope、browser-context 隔離で安定させる E2E",
  "kind": "knowledge",
  "technology": "testing",
  "version": "Playwright Docs current (Node.js test runner) (verified 2026-09-28)",
  "tags": [
    "research-domain:quality-operations",
    "testing",
    "playwright",
    "web-first assertions",
    "auto-waiting",
    "actionability",
    "expect",
    "timeout",
    "test timeout",
    "expect timeout",
    "browser-context",
    "isolation",
    "TimeoutError",
    "stable E2E"
  ],
  "sources": [
    {
      "id": "playwright-docs-test-assertions",
      "url": "https://playwright.dev/docs/test-assertions",
      "type": "official_docs"
    },
    {
      "id": "playwright-docs-test-timeouts",
      "url": "https://playwright.dev/docs/test-timeouts",
      "type": "official_docs"
    },
    {
      "id": "playwright-docs-browser-contexts",
      "url": "https://playwright.dev/docs/browser-contexts",
      "type": "official_docs"
    },
    {
      "id": "playwright-docs-actionability",
      "url": "https://playwright.dev/docs/actionability",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active"
}
---

# testing Playwright web-first assertions の auto-waiting と timeout scope、browser-context 隔離で安定させる E2E

「E2E が時々落ちるのは待機不足か、timeout 設定の混同か、テスト間の状態漏れか」を切り分けるため、4件の検証済み公式文書の事実と組み立て提案に分けて書く。4件はいずれも 2026-09-28 時点の検証済み inventory に基づく Playwright Docs current（Node.js）の記述である。

## 要点（出典に記載された事実）

### web-first assertion は条件成立まで auto-retry する（Test Assertions）

- web-first の非同期 matcher は、条件が満たされるか timeout に達するまで自動的に retry される。[Test Assertions](https://playwright.dev/docs/test-assertions)
- 既定の assertion timeout は 5秒であり、`testConfig.expect` で変更できる。[Test Assertions](https://playwright.dev/docs/test-assertions)
- generic な matcher は retry されない。retry が必要な待機は web-first matcher 側で行う。[Test Assertions](https://playwright.dev/docs/test-assertions)
- 同文書の範囲として `expect.poll`、`expect.toPass`、`expect.configure`、soft assertions がサポートされる。[Test Assertions](https://playwright.dev/docs/test-assertions)
- すなわち「表示が遅れて落ちた」は、generic matcher で即時判定していた場合、web-first matcher の retry 対象外だった可能性があるという分類ができる。個別テストの原因断定は観測なしには行わない。

### timeout は scope ごとに別物である（Test Timeouts）

- test timeout の既定は 30000ms、expect timeout の既定は 5000ms であり、互いに独立した scope を持つ。[Test Timeouts](https://playwright.dev/docs/test-timeouts)
- expect timeout は config の `expect.timeout`、または assertion ごとの `timeout` オプションで設定する。[Test Timeouts](https://playwright.dev/docs/test-timeouts)
- `actionTimeout` と `navigationTimeout` は既定で timeout なし、`globalTimeout` は既定で none である。[Test Timeouts](https://playwright.dev/docs/test-timeouts)
- つまり「全体は余裕があるのに assertion だけ落ちる」事象は、test timeout ではなく expect timeout の scope で起きている可能性がある。どちらの上限に当たったかはエラーと設定の突き合わせで確認する。

### 各テストは隔離された BrowserContext を得る（Browser contexts）

- 各テストは、incognito プロファイルのような隔離された BrowserContext を得る。cookies と local / session storage は context ごとに独立する。[Browser contexts](https://playwright.dev/docs/browser-contexts)
- runner がテストごとに context を作成するため、テスト間の状態漏れは既定の使い方では context 境界で断ち切られる。[Browser contexts](https://playwright.dev/docs/browser-contexts)
- 1つのテスト内で複数ユーザーのような多重の隔離状態が必要な場合は、`browser.newContext` でテストごとに複数の隔離 context を作れる。[Browser contexts](https://playwright.dev/docs/browser-contexts)
- context を跨ぐ共有（同一 storage の再利用など）は既定の隔離の外側の操作であり、本 inventory の検証範囲として手順は扱わない。

### action は actionability check を auto-wait する（Actionability）

- click などの action は、actionability check（attached / visible / stable / receives-events / enabled / editable）が通るまで自動待機し、timeout 後に `TimeoutError` で失敗する。[Actionability](https://playwright.dev/docs/actionability)
- どの check がどの action に適用されるかの matrix が文書に記載されている。[Actionability](https://playwright.dev/docs/actionability)
- `force` オプションは必須でない check を skip する。[Actionability](https://playwright.dev/docs/actionability)
- したがって action 直前の `waitForTimeout` による固定 sleep は、文書の auto-wait 機構と重複する。必要な待機条件が check にない場合の扱いは文書の matrix で確認する。

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- 判定は web-first matcher を既定にする。UI の到達状態を確認する assertion には auto-retry される matcher を使い、generic matcher は「既に確定した値の即時比較」に限定する。`expect.poll` / `expect.toPass` は「DOM assertion の形を取らない到達条件の待機」に使う。
- timeout は scope を指名して設定する。(1) assertion の猶予は `expect.timeout`（全体既定）か assertion ごとの `timeout` オプションで延ばす。(2) テスト全体の上限は test timeout で別に管理する。(3) `actionTimeout` / `navigationTimeout` / `globalTimeout` は既定（なし / none）のまま変えないことを起点にし、変える場合はどの scope の問題だったかを記録してから変える。
- 隔離は runner の既定に寄せる。テスト間のログイン状態・storage を引き継ぐ作りにせず、1テスト1隔離 context を前提に書く。多ユーザー操作が必要なテストだけ `browser.newContext` で明示的に複数 context を作り、どの context で誰として操作したかをテスト内で区別する。
- action 前の固定 sleep を足さない。actionability check が通らない 실패は、check 名（visible / stable / enabled など）と `TimeoutError` の内容から「どの条件が未達だったか」に読み替えて対処する。`force` は check を skip する手段であり、安定化の既定手段にしない。使う場合は「どの check を skip したか」と理由を記録する。

## 避ける使い方

- generic matcher で UI の到達を即時判定する。retry されない matcher で非同期の到達を判定すると、表示タイミングの揺れがそのまま flaky になる。
- expect timeout の不足を test timeout の延長で解消しようとする。scope が異なるため、assertion の上限には効かない。
- `actionTimeout` / `navigationTimeout` / `globalTimeout` の既定（なし / none）を把握せずに全体の上限を論じる。どの scope の上限に当たったかを混同し、誤った箇所を延ばす。
- テスト間で cookies や storage を共有する作りにする。runner がテストごとに作る隔離 context の前提を崩し、順序依存・並列干渉を招く。
- action 前に固定 sleep を置いて安定させる。auto-wait の check と二重管理になり、遅い環境では依然落ち、速い環境では時間を浪費する。
- `force` を常用して check を skip する。本来検出すべき非 actionability（操作不能な要素への操作）を隠し、テストの意味を壊す。
- 単一の失敗事例から timeout 値を全体既定に一般化する。timeout 値・retry 方針は対象アプリの表示所要時間と失敗率に照らして決め、未測定の全体引き上げはしない。

## 適用版と本番での注意

- 適用版: Playwright Docs current（Node.js test runner、2026-09-28 検証）。将来の改訂とは扱わない。
- 再確認期限: `official_docs` 4件はいずれも TTL 90日のため 2026-12-27。`testing` は `config/freshness.json` に技術 TTL の定義がないため技術期限は適用されない。文書の期限は最短に合わせて 2026-12-27。
- 文書の trust は `official` とする。参照 source がすべて `official` のため、最も低い trust を超えない範囲で `official` とする。
- 未確認事項（本調査の範囲外として推測で埋めない）: web-first matcher の一覧と各 matcher の retry 意味、`expect.poll` / `expect.toPass` / `expect.configure` / soft assertions の具体的構文と既定値、config の `expect.timeout` 以外の expect 配下項目、action ごとの check matrix の行単位の内容、`force` が skip する check の範囲、隔離 context の生成コストや並列実行との相互作用。これらは各出典の該当節を別途確認する。
- 本ドキュメントの推奨構成（web-first 既定・scope 指名の timeout 設定・runner 既定の隔離・固定 sleep なし）は設計案であり、単一事例の一般化ではない。timeout 値と context 構成は対象ワークロードの所要時間と失敗率に照らして決める。

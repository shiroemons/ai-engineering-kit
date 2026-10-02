---
{
  "id": "testing-playwright-test-locks-shared-resource-boundary",
  "title": "Playwright 1.63 Test locks: 共有資源の排他と file・project・shard の境界",
  "kind": "knowledge",
  "technology": "testing",
  "version": "Playwright v1.63.0 (2026-09-04 release); live Node.js references retrieved 2026-10-02, not patch-pinned",
  "tags": [
    "research-domain:quality-operations",
    "Playwright",
    "test locks",
    "shared resource",
    "lock",
    "test.describe",
    "fullyParallel",
    "serial",
    "sharding",
    "BrowserContext",
    "retry"
  ],
  "sources": [
    {
      "id": "playwright-163-test-locks-release-20261002",
      "url": "https://github.com/microsoft/playwright/releases/tag/v1.63.0",
      "type": "release_notes"
    },
    {
      "id": "playwright-test-locks-parallel-20261002",
      "url": "https://playwright.dev/docs/test-parallel",
      "type": "official_docs"
    },
    {
      "id": "playwright-test-locks-api-20261002",
      "url": "https://playwright.dev/docs/api/class-test",
      "type": "official_docs"
    },
    {
      "id": "playwright-sharding-lock-boundary-20261002",
      "url": "https://playwright.dev/docs/test-sharding",
      "type": "official_docs"
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

# Playwright 1.63 Test locks と共有資源の境界

## 問いと追加理由

別の BrowserContext で動くテストが同じ外部アカウントの設定を書き換えるとき、スイート全体の並列性を失わず競合を防げるか。

[Playwright v1.63.0](https://github.com/microsoft/playwright/releases/tag/v1.63.0) は named test locks を導入した。GitHub release API の公開日時は2026-09-04T22:40:31Z。2026-10-02 UTC に確認した1.63の変更として扱い、既存の web-first assertions・timeout・browser-context 隔離の文書とは別に、外部の共有資源を扱う判断を補う。

## 確認した契約

### 宣言と対象

[`test` / `test.describe` API](https://playwright.dev/docs/api/class-test) の details.lock は string または string[]。test.describe に指定すると group 内の各 test に適用する。同じ lock 名を宣言したテスト同士は、別 file / worker / project であっても同時実行されない。宣言していない別のテストの外部アクセスまで禁止する API とは説明されていない。

複数資源を使う場合は複数名を宣言できる。[Parallelism の Test locks 節](https://playwright.dev/docs/test-parallel#test-locks) は、必要な lock がすべて利用可能になってから開始し、テストの終了時に解放すると説明する。

### file 単位へ広がる保持期間

同じ [Test locks 節](https://playwright.dev/docs/test-parallel#test-locks) には重要な例外がある。default / serial mode では file 内のテストが順番にまとまって実行されるため、その file のどれかに宣言した lock は file 全体の実行期間にわたって保持される。「共有設定を変更する1テストだけに lock を付けたから、その短い本体の間だけ保持される」と見積もらない。

一方、serial mode 自体は失敗後の後続テストを skip し group 全体を retry する。[API の configure](https://playwright.dev/docs/api/class-test#test-describe-configure) は default mode の独立した retry と区別する。lock は共有資源への同時アクセスを抑える用途であり、serial の依存シナリオと同義ではない。

### ブラウザ隔離と実行配置

[Parallelism](https://playwright.dev/docs/test-parallel#avoiding-shared-state-in-parallel-tests) は、BrowserContext の cookie / storage とテスト外部の共有データを区別する。外部の record を個別化できるなら、同じ record を競わせない設計を先に検討できる。

[Sharding](https://playwright.dev/docs/test-sharding) は、テストを複数マシンの CLI 実行に分割する方式を説明している。今回の lock 文書には、別 shard・別 CI job・別 Playwright invocation を横断する分散排他の保証は確認できなかった。files/workers/projects の保証を machines/runs まで拡張しない。

## 採用判断（独自の設計提案）

1. shared resource を一覧にし、所有者と競合範囲を決める。ブラウザ内の状態なのか、外部アカウント・DB・APIの状態なのかを区別する。原因が未確定の flaky test に機械的に lock を足さない。
2. 分離できる資源は分離する。別 tenant / test account / record が用意できるなら、並列性を残す方針と比較する。同じ全体設定を変更せざるを得ない場合に named lock を候補とする。
3. lock 名はテスト名ではなく資源を表す共有契約にする。たとえば account 設定を変更する全テストが同じ命名規則に従う。表記ゆれや未宣言の呼出しを検出するレビュー項目を置く。文字列を宣言するだけでは外部サービスの権限制御にはならない。
4. database と外部 API の両方を占有するなら、必要な名前をまとめて宣言する。ただし「複数 lock が書ける」ことから公平性、待ち時間の上限、異常終了時の外部 cleanup まで保証されたとは扱わない。
5. default / serial の file-wide 保持を踏まえ、無関係で長いテストを同じ file に置く費用を測る。file 分割や parallel mode の採用は fixture / hook の寿命を確認してから決める。速度を上げるためだけに依存テストを parallel に変えない。
6. 他の shard / CI 実行とも同じ account を共有するなら、run ごとに資源を分離するか、対象サービス側で管理する lease 等の別設計を用意する。本稿の lock だけを根拠に cross-run 競合を解決済みとは判断しない。
7. lock 解放と共有状態の復旧は別の完了条件にする。設定を書き換えたまま assertion が落ちるケースでも、後続テストが期待する初期状態へ戻せる cleanup / 再初期化を設計する。retry による副作用の重複も別途確認する。

## 受け入れ試験案（未実行）

以下は利用側の試験計画であり、この調査で Playwright を実行した結果ではない。

- 同じ lock を持つ2テストを別 file / project に配置し、外部資源へのアクセス区間が重ならないことを時刻付きで確認する。別名の独立資源のテストは並行できるかも確認する。
- 複数 lock の一部が使用中のケースを作り、必要な全資源が利用可能になるまで対象テストが開始しないことを確認する。
- default / serial の file に無関係な長いテストを置き、file-wide の待機コストを記録する。parallel mode との比較では fixture / hook と test data の独立性も観測する。
- 通常成功、assertion 失敗、retry、timeout、runner 停止を分ける。次の実行が入れることと、外部設定が復旧したことを別々に判定する。強制終了時の保証は文書から補わない。
- 2つの独立 invocation / shard を同時起動し、採用した外部の資源分離策が有効か確認する。Playwright lock の仕様に記載がない保証を、この試験案の存在で確認済みにしない。

## 適用版・限界・provenance

- 対象は1.63の named locks。API の details 全体に付く Added in: v1.42 は lock 自体の導入版の根拠ではなく、1.63 release を導入版の根拠に使う。更新される live reference は patch 固定の実装解析ではない。
- 取得日は2026-10-02 UTC。release notes は30日 TTLのため、明示期限は2026-11-01。reference の公開日・更新日はページ上で明記を確認できなかった。
- hook / fixture の厳密な lock 内包範囲、入れ子 group の lock 合成、順序の公平性、deadlock 回避の内部アルゴリズム、独立 shard 間の協調、異常終了時の細部と性能は未検証。これらに依存する採用時は該当実装とテストを追加で調べる。
- Microsoft と Playwright contributor の公式文書を日本語で独自要約し、設計と試験案を加えた。サイトの [LICENSE](https://github.com/microsoft/playwright.dev/blob/main/LICENSE) は [CC-BY-4.0](https://creativecommons.org/licenses/by/4.0/)。GitHub release 本文の個別ライセンスは未確認として catalog に unknown を記録した。コード・例文の転載、依存追加、module 昇格は行っていない。

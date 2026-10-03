---
{
  "id": "testing-vitest5-async-assertion-poll-cancellation-boundary",
  "title": "Vitest 5: 非同期 assertion の await と expect.poll の期限・キャンセル境界",
  "kind": "knowledge",
  "technology": "testing",
  "version": "Vitest v5.0.0, published 2026-09-03T12:19:51Z; official documentation at f441c6fab25e579c5b7dd3dd50538416f415fbae; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:quality-operations",
    "Vitest",
    "async-assertion",
    "expect.poll",
    "await",
    "timeout",
    "AbortSignal",
    "test-isolation",
    "cleanup"
  ],
  "sources": [
    {
      "id": "vitest5-release-20261003",
      "url": "https://github.com/vitest-dev/vitest/releases/tag/v5.0.0",
      "type": "release_notes"
    },
    {
      "id": "vitest5-migration-async-20261003",
      "url": "https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/migration/index.md",
      "type": "official_docs"
    },
    {
      "id": "vitest5-expect-poll-20261003",
      "url": "https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/api/expect.md",
      "type": "official_docs"
    },
    {
      "id": "vitest5-async-completion-20261003",
      "url": "https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/learn/async.md",
      "type": "official_docs"
    },
    {
      "id": "vitest5-test-context-signal-20261003",
      "url": "https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/test-context.md",
      "type": "official_docs"
    },
    {
      "id": "vitest5-cancellable-resources-20261003",
      "url": "https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/recipes/cancellable.md",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/quality-operations.json"]
}
---

# Vitest 5 の assertion 完了と処理停止を分ける

## 問いと適用範囲

Vitest 4 で通ったテストが 5 で失敗したとき、待ち時間を延長すべきか、await 漏れや終了処理を直すべきか。**assertion の完了、poll の期限、開始済み処理の停止を別々に確認する。**

[v5.0.0 release](https://github.com/vitest-dev/vitest/releases/tag/v5.0.0) は 2026-09-03 公開で、非同期 assertion の未 await を失敗にする変更と、expect.poll の期限後の完了を成功にしない変更を breaking changes に挙げる。v5.0.0 の最低要件は Node.js 22.12.0、Vite 6.4.0。これは固定版の移行メモであり、最新 patch の推奨ではない。

既存の [Playwright 待機・隔離](playwright-web-first-assertions-timeout-isolation.md) は別 runner の契約である。同名の expect.poll でも、その timeout や await 検出の履歴を Vitest へ転用しない。本稿はテスト品質の移行判断に限定し、Vitest 全体の移行手順は扱わない。

## 確認した契約

### 1. 未 await の assertion と、実行されなかった assertion

[固定版 migration guide](https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/migration/index.md#unawaited-asynchronous-assertions-fail-the-test) によると、resolves / rejects / toMatchFileSnapshot などの非同期 assertion は、await しないとテスト終了時に失敗になる。v4 では末尾で自動的に待機し、警告していた。v5 のエラーは未 await の assertion の位置を示す。

[非同期テスト guide](https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/learn/async.md) は、runner がテストから返された Promise を待つこと、callback・条件分岐・空の loop では assertion 自体が一度も実行されない場合があることを説明する。expect.hasAssertions() は少なくとも1回、expect.assertions(n) は指定回数の実行を検査する。

したがって、次の二つは異なる欠陥である（契約からの整理）。

- assertion を呼んだが、その Promise を待たない: v5 の未 await 検出の対象
- callback が呼ばれず、assertion も生成されない: 実行回数や業務結果の検証が別途必要

「v5 があらゆる未回収 Promise や未実行の検証を検出する」とは保証されない。テスト callback を async にするだけで、内部から切り離されたすべての処理が待機対象になるとは扱わない。

### 2. expect.poll の timeout は callback と matcher を含む

[固定版 Expect API](https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/api/expect.md#poll) で確認した。

- interval は再試行の間隔、timeout は polling 全体の予算。保留中の callback と非同期 matcher の実行時間も含む
- callback 内の例外も、timeout に達するまで再試行される
- callback には poll の期限到来で abort される AbortSignal が渡る
- poll は非同期 assertion。**未 await の expect.poll が失敗するのは v3 から**であり、v5 で初めて追加された検出ではない
- poll は callback の非同期結果を待つため、resolves / rejects を追加しない。snapshot matcher、toThrow とその alias も非対応

[v5 migration の timeout 節](https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/migration/index.md#expectpoll-fails-when-it-times-out) は、従来は期限後に callback が解決した場合や遅い試行で assertion が成立した場合に成功できたと説明する。v5 では callback または assertion が期限内に完了しなければ reject する。必要な処理時間が正当に予算を超えるなら、その poll の timeout を見直す。

この記述は JavaScript の実行を任意の時刻に強制中断する保証ではない。同期的に event loop を占有するコードの正確な中断時刻、プロセス終了時刻まで timeout 値から保証しない。

### 3. poll の signal とテスト全体の signal は発火条件が違う

[Test Context](https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/test-context.md#signal) の signal は v3.2.0 から存在する。発火条件はテスト timeout、Ctrl+C による取消、cancelCurrentRun、bail 設定時の並列テスト失敗。poll callback に渡る signal の説明は、その poll 自身の期限到来である。

[Cancellable Test Resources](https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/docs/guide/recipes/cancellable.md) は、テスト取消だけでは fetch・子プロセス・stream・独自 polling loop が自動的に停止しないと説明する。対応 API に signal を渡し、独自 helper でも下流まで伝える必要がある。

本稿で確認した公開文書だけから、poll の signal がテストの全取消条件を自動的に包含するとは断定しない。また、signal を受け取るだけで無視する helper や、取消 API がない外部処理まで停止できるとはいえない。判定が reject した事実を、下流の停止完了・副作用の取り消しの証拠にしない。

## 移行時の判断手順（独自の設計提案）

以下は公式の推奨構成そのものではなく、上記の契約から組み立てた運用案である。

1. 最初に Node.js・Vite・Vitest の実版を固定する。失敗を「製品コードの回帰」「未 await」「poll 期限超過」「assertion 未実行」に分類し、分類前に timeout を一括延長しない
2. 非同期 helper の戻り値を追跡する。呼出側まで assertion の Promise を返すか、helper 内で await して、その helper をテストから await する。意図的な並列検証は集約して待つ。終了順序が必要な検証を、単に Promise.all へ移して意味を変えない
3. callback が呼ばれること自体が要件なら、その到達と必要な assertion 回数を検証する。空配列・早期 return・条件分岐を成功 fixture だけで済ませない。実行回数検査を処理停止の仕組みとして使わない
4. poll callback は繰り返して安全な状態読取を基本にする。注文作成・ジョブ投入・状態更新を callback に入れると、再試行で繰り返す危険がある。操作は1回実行し、その操作 ID に対応する結果を poll する設計を検討する
5. poll 期限とテスト取消の両方を要件にする helper は、二つの取消経路を設計・検証する。どの signal を渡すかを名前で明示し、単に同名変数 signal があることを伝播の証拠にしない。合成する場合も採用 runtime と下流 API の対応を別に確認する
6. 失敗報告には assertion の位置、poll timeout、テスト timeout、callback 開始・完了、取消要求・資源解放の観測を残す。fixture の cleanup は成功・失敗の両方で実行し、worker が終わったという事実だけで外部状態の隔離を判定しない

## 説明用の反例

以下の数値は独自の例であり、実測値ではない。

- poll 予算が 100 ms で、callback が 150 ms 後に期待値を返す。v5 の契約では後着の成功を受け入れない。必要なのが「100 ms 以内の完了」なら失敗が正しく、単に 200 ms へ延長するとテスト要件を変える
- callback は 20 ms で返るが、非同期 matcher が追加で 120 ms かかる。callback だけの計測が予算内でも polling 全体は予算外になり得る
- poll は失敗したが、signal を無視する helper が150 ms後に共有 fixture を更新する。失敗を検出できたことと、次のテストを汚染しないことは独立している
- 空の一覧に対する loop 内にしか assertion がない。未 await 検出が有効でも、検証対象ゼロの成功を防ぐには一覧の要件や実行回数を別途検証する

## 採用前の受け入れ試験案（未実行）

外部サービスへ接続せず、遅延・取消応答を制御できる fixture で旧版と v5.0.0 を比較する。検索 eval は以下の runtime 試験の代わりではない。

- 意図的に await を外した resolves / rejects / file snapshot と、正しく await した対応ケース。終了状態と報告位置を照合する
- 期限より十分早い成功、十分遅い callback 成功、遅い非同期 matcher、完了しない Promise。flaky な同時刻境界だけに依存せず期限判定を確認する
- callback が例外を返した後に回復するケース。読取の再試行数を測り、書込操作が重複しない設計を検査する
- poll 期限で signal を尊重する helper と無視する helper。assertion の失敗時刻と、処理・listener・fixture の解放を別々に観測する
- poll 予算より短いテスト timeout、および明示取消・bail。テスト全体の取消経路も helper に届くか確認する
- callback 未実行、空配列、条件分岐による assertion ゼロ。必要な実行回数検査が欠落を検出することを確かめる

## Provenance・ライセンス・限界

- v5.0.0 の release API は published_at `2026-09-03T12:19:51Z`、prerelease false。tag API と release の commit link は full SHA `f441c6fab25e579c5b7dd3dd50538416f415fbae` で一致した。資料の取得日は 2026-10-03 UTC
- live の公式 release・migration・Expect API・非同期 guide・取消 recipe を web で開き、関連する公式文書を同じ release commit から GitHub connector でも取得して照合した。固定 SHA の一部 web URL は cache miss になったため、web の取得成功を装わず connector の取得内容を根拠とした。文書ごとの独立した公開日時は未確認
- 固定 commit の [LICENSE](https://github.com/vitest-dev/vitest/blob/f441c6fab25e579c5b7dd3dd50538416f415fbae/LICENSE) は MIT で、associated documentation を含む。release note 本文の独立したライセンス表示は未確認として catalog では unknown。すべて独自の日本語要約であり、コード・設定例・大きな引用は転載していない
- Vitest をインストールしての挙動確認、内部実装の全経路監査、Browser Mode・fake timers・custom matcher の組合せ試験、v4 各 patch の比較は未実施。取消後の副作用の有無は利用する helper・外部システムごとに確認する
- 再確認期限は 2026-11-02。取得日から release_notes の30日 TTLを適用した最小期限であり、release 公開日を取得日に読み替えていない

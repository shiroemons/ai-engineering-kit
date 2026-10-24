---
{
  "id": "testing-playwright-snapshot-creation-approval-boundary",
  "title": "Playwright 1.64 snapshot: missing成功・default失敗と基準画像承認の境界",
  "kind": "knowledge",
  "technology": "testing",
  "version": "Playwright Test v1.64.0 change (2026-10-07); v1.64.1 source cfe490d941f992655948eee8fd4fd94e1c4234bf (2026-10-14); live Node.js docs retrieved 2026-10-24 UTC; browser runtime not executed",
  "tags": [
    "research-domain:quality-operations",
    "Playwright",
    "snapshot",
    "baseline",
    "updateSnapshots",
    "missing",
    "default",
    "none",
    "visual regression",
    "CI",
    "retry",
    "approval"
  ],
  "sources": [
    {
      "id": "playwright-164-snapshot-release-20261024",
      "url": "https://github.com/microsoft/playwright/releases/tag/v1.64.0",
      "type": "release_notes"
    },
    {
      "id": "playwright-1641-snapshot-release-20261024",
      "url": "https://github.com/microsoft/playwright/releases/tag/v1.64.1",
      "type": "release_notes"
    },
    {
      "id": "playwright-snapshot-config-20261024",
      "url": "https://playwright.dev/docs/api/class-testconfig",
      "type": "official_docs"
    },
    {
      "id": "playwright-snapshot-cli-20261024",
      "url": "https://playwright.dev/docs/test-cli",
      "type": "official_docs"
    },
    {
      "id": "playwright-snapshot-visual-comparisons-20261024",
      "url": "https://playwright.dev/docs/test-snapshots",
      "type": "official_docs"
    },
    {
      "id": "playwright-1641-snapshot-source-20261024",
      "url": "https://github.com/microsoft/playwright/tree/cfe490d941f992655948eee8fd4fd94e1c4234bf",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/quality-operations.json"]
}
---

# Snapshotを作れたことと、承認済みの画面に一致したことを分ける

## 問いと新しさ

「新しい基準画像だけ作り、既存画像の回帰も検査する」CIが緑になったとき、新しい画面まで承認済みと言えるか。結論は言えない。Playwright 1.64では、明示した `missing` と、設定を変えない `default` の成功条件が分かれた。生成成功、既存baselineとの比較成功、baselineのレビュー承認は別々の証拠として扱う。

[1.64.0のrelease][release164]は `--update-snapshots=missing` で欠けたsnapshotを生成したテストが成功する変更を明記する。GitHub release APIの公開日時は2026-10-07T20:40:43Zで、prereleaseではない。以前と同じ「生成するがテストは失敗する」既定動作には `default` という名前が付いた。`updateSnapshots` API全体の導入を1.64とする意味ではない。

[1.64.1][release1641]の公開日時は2026-10-14T11:51:33Z。実装分析はこの版のcommit `cfe490d941f992655948eee8fd4fd94e1c4234bf` に固定した。導入版1.64.0のcommit `606ecd8b34bac100db32561b6278ffd76e5eb3cd` と比較し、本稿で読むmatcher、config、worker、CLI、snapshotテストのファイルは同一だった。取得日は2026-10-24 UTCであり、将来版の保証ではない。

既存の[web-first assertion・timeout・context隔離](playwright-web-first-assertions-timeout-isolation.md)や[test locks](playwright-test-locks-shared-resource-boundary.md)とは異なり、本稿は比較の正解となるファイルをいつ作り、その結果をどう承認するかに絞る。新機能の一覧や画像差分アルゴリズム全体の説明ではない。

## 1. modeとbaselineの有無で判定が変わる

[設定契約][config]と[固定matcher実装][matcher]に基づく。以下は `.not` を付けないファイルベースの `toMatchSnapshot` / `toHaveScreenshot` の、snapshot側の結果である。画像を取得できること、書込権限があること、他のassertionが成功することまでmodeが保証するわけではない。

| 有効なmode | baselineがない | baselineがあり、比較に不一致 |
|---|---|---|
| `default` | baselineを作るがテスト失敗 | 失敗、baselineは維持 |
| `missing` | baselineを作り、この理由では失敗しない | 失敗、baselineは維持 |
| `none` | baselineを作らず失敗 | 失敗、baselineは維持 |
| `changed` | baselineを作る | 更新できればactualで置換し、この理由では失敗しない |
| `all` | baselineを作る | 更新できればactualで置換し、この理由では失敗しない |

`missing` は「全snapshotを無条件に承認する」設定ではない。既存baselineとの不一致を失敗にする一方、存在しない比較対象には新しいactualを採用する。この非対称性が、既存画像を検査しつつ新規画像を収集できる理由であり、レビューgateを別に要する理由でもある。

### CLIを省いた場合と、値を省いた場合は逆方向

[CLI文書][cli]および[固定program.ts][program]では、flagなしの既定は `default`、値なしの `-u` / `--update-snapshots` は `changed` になる。ただしflagなしであっても、configに `updateSnapshots: 'missing'` を設定していればそれが使われる。[config.ts][configsource]はCLI override、user config、組込既定の順で選ぶ。「コマンドに-uがない」だけでは確認にならない。

既存の不一致を修正せずに `--update-snapshots` を付け直すと、回帰を検出する試験から、そのactualを正解として書く工程へ変わる。検証jobでは有効なmodeを確認し、候補生成jobと名前・結果の意味を分けるのが本稿の提案である。

`none` も「snapshot期待値を無視する」という意味ではない。`ignoreSnapshots` / `--ignore-snapshots` は別の設定で、検証gateで有効なら比較自体の証拠がなくなる。[設定][config] / [CLI][cli] また、`none` が禁じるのはbaselineの更新であり、reportやactual/diffなど全出力の作成を禁止するread-only実行の契約ではない。

## 2. defaultの失敗は自動retryで消えないが、次回の入力は変わり得る

[SnapshotHelper.handleMissing][matcher]は `default` でもexpectedファイルを書いた後、`softError` と `shouldNotRetryTest: true` を返す。[workerのstep完了処理][worker]は前者をテスト失敗にし、後者をnon-retriableとして記録する。通常の例外を投げない経路でも、テスト成功という意味ではない。

[upstream screenshotテスト][screentests]には `retries: 2` を設定し、欠けた2枚がともに書かれ、retry表示がなく、exit codeが1になる検査がある。serial groupを含むケースも同じく再試行しないことを検査する。「retriesを増やせば初回生成の赤が緑になる」という説明は、この固定版の挙動に合わない。

一方、ここから次の条件付きの運用上の帰結を導ける。これは本稿の推論であり、実行観測ではない。

1. 1回目の `default` 実行がbaseline欠落で失敗し、新しいexpectedファイルを残す。
2. 外側のCIラッパーなどが、同じファイルを保持したまま別のPlaywright実行を始める。
3. 同じbaseline path・比較条件でactualがその生成済み画像に一致し、他の失敗もなければ、2回目は既存baselineとの比較として成功し得る。

Playwright内のretry抑止は、別プロセスの再実行や、生成済みファイルを含むcache・artifactの再利用まで取り消さない。すべての再実行が成功するという主張でもない。独自の推奨は、検証を承認済みbaselineから開始し、新規生成分を検証入力へ無審査で戻さないこと。生成物の有無を調べる際はtracked差分だけでなく、untrackedな新規画像も対象にする。

## 3. path変更は回帰比較を新規生成へ変える

[Visual comparisons][visual]ではブラウザ・OS・fontなどの違いで描画が変わり、基準画像を生成した環境と同じ環境での比較を推奨する。既定のsnapshot名はbrowser/platformやproject名を反映し、`snapshotPathTemplate` で保存位置も変更できる。

本稿独自の反例: 承認済みの `checkout.png` を比較していたテストを、画面の変更と同時に `checkout-v2.png` へ改名する。新pathにbaselineがなく、`missing` が有効なら、壊れた画面であっても取得・生成に成功しただけでこのsnapshotのassertionは通り得る。旧 `checkout.png` が残っていても、新しい比較先を承認した証拠にはならない。

同様にproject名、snapshotの形式・拡張子、path templateの変更は、新規画像が大量に現れる原因になり得る。これを「全画面に回帰がなかった」と分類せず、旧pathから新pathへの対応と、新規baselineの内容を確認する。逆に複数環境を同じpathへ畳む設計も、描画条件が同じことの証明にはならない。

`toHaveScreenshot` が新baselineを作る際は、連続する2枚の比較が一致するまで撮影する。[Visual comparisons][visual] / [matcher実装][matcher] これは短時間の描画安定性の確認であり、商品価格・ボタン・権限表示などが正しいという仕様確認ではない。安定したエラー画面も候補になり得るので、重要な業務状態は画像以外のassertionでも確認する、というのが本稿の提案である。`toMatchSnapshot` に同じ撮影待機があるという意味ではない。

## 4. 生成と昇格を分ける受入手順（独自の運用案）

以下は公式機能の追加保証ではなく、上記契約から組み立てた運用である。

1. 検証の入力として、application/testのcommit、承認済みbaselineの集合、Playwright/browser版、OS・font・viewportなどの描画条件、実行するprojectを記録する。
2. 通常の回帰検証は明示的な `--update-snapshots=none` を候補にし、baselineの欠落も失敗として扱う。`ignoreSnapshots` が有効でないことと、必要なテスト・projectが実行されたことも別に確認する。modeだけで試験範囲の完全性は分からない。
3. 新規・意図した変更のbaseline生成は隔離したjobで行う。`missing` を使えば既存の不一致を残したまま新規候補を集められるが、そのexit 0を新規画像の承認結果へ昇格させない。`changed` / `all` は既存画像も更新対象になると明示する。
4. 候補を新規・変更・対応先を失った旧画像に分け、意図した画面かをレビューする。生成に使ったcommitと条件を候補へ結び付け、実際にレビューしたbaselineだけを採用する。
5. 採用後はcleanな入力で再び `none` の回帰検証を行う。二度目の比較成功は採用した基準への一致を証明するが、その基準の妥当性は前段のレビューと業務assertionが担う。

Playwrightのsnapshot機能自体には、人が内容を承認したという事実を上のmodeから生成する契約はない。「生成できた」「比較に一致」「内容を承認済み」を1つの緑表示へ潰さない。

## 5. 導入先で行う受け入れ試験

以下は未実行の試験計画。upstreamのassertionを読んだ事実と、自分の環境で試験が成功した事実を区別する。

| ケース | 観測する境界 |
|---|---|
| 安定した画面、新規path、`missing` | baseline生成とsnapshot側成功。新規画像がレビュー待ちとして残る |
| 同じ入力、`default`、`retries: 2` | baselineは生成されるが失敗し、Playwright内retryで緑化しない |
| 新規path、`none` | 失敗し、baselineが増えていない |
| 既存baselineに意図的な不一致、`missing` | 失敗し、承認済みexpectedの内容が変わらない |
| 既存不一致、値なしの `-u` | 有効modeが `changed` になり、更新工程として扱われる |
| 新規画像生成後、別の業務assertionで失敗 | `missing` はテスト全体の成功を保証しない |
| default失敗後、生成物を保持して別run | 同path・同条件・同actualなら通り得ることと、未承認生成物の混入検知を確認 |
| screenshot名・project名・path templateを変更 | 旧画像との比較ではなく、新規baseline扱いになるpathを一覧で確認 |
| configの `missing` とCLIの `none` を併用 | CLI overrideが実際に有効になり、欠落を生成しない |
| `.not` とbaseline欠落を組み合わせる | 正のmatcher向け生成表を流用せず、失敗と未生成を確認 |
| font・browser・viewportを変える | 環境差を機能回帰や承認済み新規画像と混同しない |
| `ignoreSnapshots` を意図的に有効化する | gate側が比較の欠落を検知し、単なるrun成功を採用しない |

固定版の[upstream goldenテスト][goldentests]と[screenshotテスト][screentests]は、`missing` の生成成功、`default` の生成失敗、`missing` で既存不一致が失敗しexpectedが維持されることを明示的にassertする。`.not` で新規baselineを自動作成しないケースも確認した。これらは本調査では未実行。

## 適用限界・出典・ライセンス

- 対象はNode.jsのPlaywright Test 1.64.1で確認したファイルベースsnapshotの受入判断。ARIAのinline snapshot、`updateSourceMethod` のpatch反映、独自matcher、第三者の画像レビューserviceは本稿の判定表へ一般化しない。
- ブラウザ実行、OS別rendering、実アプリのCI、GitHubのapproval運用は未検証。検索evalはこの文書の発見性だけを検査し、画像比較やレビュー完了を証明しない。
- rollingな公式ページに公開日がない場合は取得日だけを記録した。release日時はGitHub release API、実装版はfull commitで区別する。
- `microsoft/playwright` の固定commitの[LICENSE][license]および対象実装・testのheaderはApache-2.0。`playwright.dev` の[サイトLICENSE][sitelicense]はCC-BY-4.0。release本文の個別ライセンスは未確認としてunknownを記録し、すべて独自要約・分析とした。コード・画像の転載やmodule昇格は行っていない。
- 再確認期限はrelease_notesの30日TTLに合わせた2026-11-23。期限は過去のrelease事実を否定する日ではなく、適用する版・設定契約を見直す日である。

[release164]: https://github.com/microsoft/playwright/releases/tag/v1.64.0
[release1641]: https://github.com/microsoft/playwright/releases/tag/v1.64.1
[config]: https://playwright.dev/docs/api/class-testconfig#test-config-update-snapshots
[cli]: https://playwright.dev/docs/test-cli
[visual]: https://playwright.dev/docs/test-snapshots
[matcher]: https://github.com/microsoft/playwright/blob/cfe490d941f992655948eee8fd4fd94e1c4234bf/packages/playwright/src/matchers/toMatchSnapshot.ts
[worker]: https://github.com/microsoft/playwright/blob/cfe490d941f992655948eee8fd4fd94e1c4234bf/packages/playwright/src/worker/testInfo.ts
[program]: https://github.com/microsoft/playwright/blob/cfe490d941f992655948eee8fd4fd94e1c4234bf/packages/playwright/src/program.ts
[configsource]: https://github.com/microsoft/playwright/blob/cfe490d941f992655948eee8fd4fd94e1c4234bf/packages/playwright/src/common/config.ts
[screentests]: https://github.com/microsoft/playwright/blob/cfe490d941f992655948eee8fd4fd94e1c4234bf/tests/playwright-test/to-have-screenshot.spec.ts
[goldentests]: https://github.com/microsoft/playwright/blob/cfe490d941f992655948eee8fd4fd94e1c4234bf/tests/playwright-test/golden.spec.ts
[license]: https://github.com/microsoft/playwright/blob/cfe490d941f992655948eee8fd4fd94e1c4234bf/LICENSE
[sitelicense]: https://github.com/microsoft/playwright.dev/blob/main/LICENSE

---
{
  "id": "github-actions-retention-query-evidence-boundary",
  "title": "GitHub Actions の保持期間変更: 消える CI 証跡と検索件数上限の境界",
  "kind": "knowledge",
  "technology": "github-actions",
  "version": "github.com: 2026-10-01 保持対象拡張、2026-09-24 artifact非表示化、2026-09-25 検索件数の段階展開告知 / REST API 2026-03-10 表示の参照を2026-10-02 UTC確認、利用アカウントでの実測なし",
  "tags": [
    "research-domain:infrastructure",
    "github-actions",
    "retention",
    "workflow-runs",
    "artifacts",
    "check-runs",
    "commit-statuses",
    "pagination",
    "ci-evidence"
  ],
  "sources": [
    {
      "id": "github-actions-retention-active-20261002",
      "url": "https://github.blog/changelog/2026-10-01-actions-retention-now-covers-checks-runs-and-statuses/",
      "type": "release_notes"
    },
    {
      "id": "github-actions-expired-artifact-visibility-20261002",
      "url": "https://github.blog/changelog/2026-09-24-expired-github-actions-artifacts-no-longer-shown-in-ui-and-api/",
      "type": "release_notes"
    },
    {
      "id": "github-actions-run-query-count-20261002",
      "url": "https://github.blog/changelog/2026-09-25-changes-to-query-results-in-the-github-actions-api-and-ui/",
      "type": "release_notes"
    },
    {
      "id": "github-actions-retention-settings-20261002",
      "url": "https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository",
      "type": "official_docs"
    },
    {
      "id": "github-actions-evidence-workflow-runs-api-20261002",
      "url": "https://docs.github.com/en/rest/actions/workflow-runs?apiVersion=2026-03-10",
      "type": "official_docs"
    },
    {
      "id": "github-actions-evidence-workflow-jobs-api-20261002",
      "url": "https://docs.github.com/en/rest/actions/workflow-jobs?apiVersion=2026-03-10",
      "type": "official_docs"
    },
    {
      "id": "github-actions-evidence-artifacts-api-20261002",
      "url": "https://docs.github.com/en/rest/actions/artifacts?apiVersion=2026-03-10",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# GitHub Actions の保持期間変更と CI 証跡の完全性

## 問いと結論

過去のリリースについて「この commit は必要な CI を通ったか」「どの artifact を生成したか」を、GitHub の現在の一覧だけから判定できるか。**保持期限後の不在、検索の切り詰め、再実行による表示範囲を区別しなければ判定できない**。

本稿は、GitHub Actions の CI 証跡を取得・保存する運用に限定する。2026年秋の変更を受け、必要な結果を取得できた状態と、結果が取得できず不明な状態を分ける。artifact attestation の署名検証、runner 更新、workflow のキャンセル制御は別の問題である。

## 確認した変更と適用範囲

### 1. 実行結果のメタデータにも保持期限が適用される

[2026-10-01 の実施告知](https://github.blog/changelog/2026-10-01-actions-retention-now-covers-checks-runs-and-statuses/)は、github.com の checks・workflow runs・statuses が、artifact とログと同じ Actions 保持設定の対象になったと明記する。GitHub Actions 自身だけでなく、第三者アプリが作成した checks と statuses も含まれる。設定を後から延ばしても、既に削除されたデータは復元されない。

[リポジトリ設定の公式文書](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#configuring-the-retention-period-for-checks-workflow-runs-commit-statuses-artifacts-and-logs-in-your-repository)の保持設定節では次を確認した。

- 既定の保持期間は90日。public は1〜90日、private は1〜400日の範囲で設定できるが、管理する organization / enterprise の上限を超えられない
- checks には check suites と check runs が含まれ、commit statuses にも適用される。従来これらは設定値によらず400日超保持されていた
- 保持期間のカスタマイズは新たに作られるオブジェクトに適用され、既存オブジェクトへ遡及しない
- artifact 個別の保持期間も設定できる

ここで「設定変更が既存オブジェクトに遡及しない」ことを、10/1の対象拡張後も古い checks が無期限に残るという意味に読み替えない。実施告知は、対象記録が設定期間を超えると自動削除されると説明している。既存の証跡を救う目的では、設定の延長だけを安全策にしない。

これはサービス側の保持方針であり、特定の runner / action のバージョンを上げる移行ではない。個別リポジトリの削除開始時刻や全アカウントへの反映完了を実測したわけではなく、GHES 各版や別のデータ所在地への適用は本稿で確認していない。

### 2. 期限切れ artifact の「残った項目」も取得できなくなる

[2026-09-24 の告知](https://github.blog/changelog/2026-09-24-expired-github-actions-artifacts-no-longer-shown-in-ui-and-api/)は、期限切れ artifact を run summary と REST API から返さなくした。以前は実体削除後も Expired 表示が残っていた。告知は repository の artifact 一覧と個別 artifact 取得を具体例に挙げ、この変更自体は保持設定・課金を変更しないと説明している。

生成した artifact の情報を run のログから調べる方法も案内されている。ただしログも保持期限のあるデータである。これは「artifact が期限切れでも、ログさえ見れば永続的に復元できる」という契約ではない。またログに生成履歴があっても、artifact のバイト列を復元できることにはならない。

[Artifacts API](https://docs.github.com/en/rest/actions/artifacts?apiVersion=2026-03-10)の応答例には `expired` や `expires_at` が残るが、フィールドの存在と期限切れ履歴の列挙保証は別物である。現時点で artifact がないことだけから「最初から未生成」「アップロード失敗」と断定しない。

### 3. 件数表示と取得可能件数は異なる

[2026-09-25 の検索変更告知](https://github.blog/changelog/2026-09-25-changes-to-query-results-in-the-github-actions-api-and-ui/)では、workflow・event・status・branch・actor による run 検索の一致件数が2,500を超える場合に `2,500+` と報告し、ページ分割で取得する結果は最大1,000件のままとしている。大量検索のタイムアウトによる不正確な件数を避ける変更で、github.com / GitHub Enterprise Cloud に段階展開中と告知された。

したがって、**2,500件を取得できるようになった変更ではない**。REST JSON の `total_count` が文字列へ変わるなどの具体的な伝送形式までは、この告知だけから断定できない。本稿では実 API 応答による型検証をしていない。

[Workflow runs API](https://docs.github.com/en/rest/actions/workflow-runs?apiVersion=2026-03-10)は、`actor`・`branch`・`check_suite_id`・`created`・`event`・`head_sha`・`status` を使う検索につき最大1,000件とし、`per_page` は最大100、`page` の既定は1としている。件数表示の閾値、1検索の上限、1ページのサイズを混同しない。検索を狭めても保持期限切れのデータは復活しない。

## 保存すべき証跡を決める

以下は公式の保存形式ではなく、上記の制約から導いた本リポジトリの設計案である。

### 取得範囲と結論を別に記録する

- リリース対象の `head_sha` と、確認が必要な workflow / job / check の集合を先に固定する。ブランチ名や「最新の成功」だけを対応付けに使わない
- 取得できた `status` と `conclusion` はそのまま保存する。`completed` は実行状態であり、成功を意味する `success` とは別のフィールドである
- 不在を扱う際は、少なくとも「成功を確認」「失敗等の結論を確認」「進行中」「証跡を取得できず不明」を分ける。期限切れ・未生成・アクセス不備・検索漏れの原因が未確定なら、推測で埋めない
- 取得日時、使用した API 版、フィルター、期間、ページ分割の完了状況も残す。取得範囲に穴があるのに「全 run を確認済み」としない

### 再実行の証跡を混ぜない

[Workflow jobs API](https://docs.github.com/en/rest/actions/workflow-jobs?apiVersion=2026-03-10)では、run の jobs 一覧に対する `filter` は既定が `latest`。`all` は以前の実行を含め、`/attempts/{attempt_number}/jobs` は試行を指定する。API 応答例は job / step の `status`・`conclusion`、`head_sha`、`check_run_url` を持つ。

このため保存側では `run_id` と `run_attempt` を区別し、必要なら attempt ごとの job 一覧とログを保存する。例として、試行1が失敗して試行2が成功したとき、`latest` の結果だけで「一度も失敗していない」と結論しない。前の失敗を残す必要がある監査と、最終試行によるリリース判定は別の集計にする。第三者 checks / statuses も要件なら、Actions の jobs 一覧だけで収集完了とはしない。

### 一覧は小さい期間で取り、境界を検証する

新規収集では完了した run を早めに保存し、過去分の補完では `created` の期間を分割する。1,000件上限に達する窓はさらに細分化し、各窓内でページ分割する。窓の境界で重複取得した run は ID で重複除去する。

これは収集側の案であり、「日単位なら必ず全件取得できる」という保証ではない。狭い窓でも上限が残る場合や権限不足では、その範囲を未完として残す。取得件数を概数表示と一致させることを完全性の証明にしない。すでに期限を過ぎた期間は、分割しても欠損を修復できない。

### URL ではなく必要な内容を保存する

最小の保存候補は、repository・commit SHA、workflow / run / attempt / job / check の識別子、結論、実行時刻、元URL、取得時刻である。生成物が必要なら artifact の ID・名前・期限・取得時に得た digest と、実際のファイルの保存成否も結び付ける。どこまで残すかは利用目的で決め、不要なログや機密情報を無制限に集めない。

Workflow runs API の attempt 別ログと Artifacts API のダウンロードは、リダイレクト先URLが1分で失効すると文書化されている。返された `Location` を台帳に置くだけではアーカイブにならない。保存処理は内容の取得・保存を確認し、短命URLや資格情報は長期台帳に残さない。artifact ダウンロード参照には `302` と `410` があるが、これをすべての一覧・個別取得エンドポイントの期限切れ応答へ一般化しない。

保存先は元の証跡に見合うアクセス制御と保持・削除方針を持たせる。単に同じ短い保持設定の別 artifact としてまとめ直すだけでは、長期保存の要件を満たしたとは言えない。公開リポジトリで90日を超える証跡が必要なら、上限外の値を設定することより、期限前に必要な内容を別途保存できる運用を検討する。

## 避ける判断と確認例

| 観測 | 避ける判断 | 安全な扱いの例 |
|---|---|---|
| 古い SHA の runs が空 | CI 未実行だった、または失敗がないので成功 | 保持・権限・検索範囲を確認し、独立した保存証跡がなければ不明 |
| run は残るが artifact 一覧が空 | artifact は未生成だった | まだ残るログと保存済み台帳を照合。ファイル取得は別に確認 |
| 検索で `2,500+`、1,000件を取得 | 全体を取得できた | 期間を細分化し、未取得範囲を記録 |
| jobs の既定一覧がすべて成功 | 全試行で失敗なし | `latest` と `all` / attempt 指定の目的を分ける |
| 保持設定を延長した | 既存・削除済みの記録も救済された | 新規オブジェクトへの適用と削除済み非復元を前提に確認 |
| ログのダウンロードURLを保存した | ログを保存した | 1分のURL期限とは別に、内容の保存成否を確認 |

## 検証・限界・出典の扱い

- 2026-10-02 UTC に各公式ページを開き、日付・適用範囲・API の表示版を照合した。10/1の保持変更は実施後告知、9/25の検索変更は段階展開告知として扱う。すべての利用アカウントで実装が一致するという実測結果ではない
- Docs の保持節には取得時も10/1を未来形で書いた注意書きが残る。実施状態は10/1の新しい公式告知と照合した。新しい告知だけで、Docs にない応答コード・削除時刻・課金計算を補っていない
- 実アカウントの保持設定、期限切れ時の HTTP 応答、削除バッチの時刻、検索件数の JSON 型、期間境界での一貫性、ログ・artifact の実ファイル回収は未実検証。本文の収集手順も実装・稼働済みとは主張しない
- 本稿は取得・保存の運用境界を扱い、法的な保存期間や証拠能力を定めない。既存の artifact attestation 文書の署名検証手順を置き換えない
- GitHub Docs 本文の CC-BY-4.0 は公式 github/docs の README と LICENSE で確認した。GitHub Blog には本文の再利用ライセンス表示が見つからず、catalog は `unknown`。本稿は日本語による事実の独自要約と設計案であり、コード・図版を転載していない
- repository 実装の解析を行っていないため commit SHA 固定の source はない。release notes の TTL 30日を含むため、明示期限は2026-11-01。検索 eval は文書を見つける検査であり、SaaS 側の動作テストではない

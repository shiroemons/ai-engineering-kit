---
{
  "id": "evals-openai-retirement-evidence-preservation-parity",
  "title": "OpenAI Evals 終了前の証拠保全: 全件取得・入力被覆・grader移行の一致を分ける",
  "kind": "knowledge",
  "technology": "evals",
  "version": "OpenAI Evals deprecation 2026-06-03; read-only 2026-10-31 / shutdown scheduled 2026-11-30; v1 Evals/Files rolling API docs and Promptfoo migration article checked 2026-10-04 UTC; runtime/SDK version untested",
  "tags": [
    "research-domain:ai-engineering",
    "evals",
    "openai",
    "migration",
    "evidence-preservation",
    "output_items",
    "pagination",
    "result_counts",
    "grader-parity",
    "promptfoo"
  ],
  "sources": [
    {
      "id": "openai-evals-retirement-timeline-20261004",
      "url": "https://developers.openai.com/api/docs/deprecations",
      "type": "release_notes"
    },
    {
      "id": "openai-evals-definition-list-20261004",
      "url": "https://developers.openai.com/api/reference/resources/evals/methods/list",
      "type": "official_docs"
    },
    {
      "id": "openai-evals-run-list-20261004",
      "url": "https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/list",
      "type": "official_docs"
    },
    {
      "id": "openai-evals-run-retrieve-20261004",
      "url": "https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/retrieve",
      "type": "official_docs"
    },
    {
      "id": "openai-evals-output-items-list-20261004",
      "url": "https://developers.openai.com/api/reference/resources/evals/subresources/runs/subresources/output_items/methods/list",
      "type": "official_docs"
    },
    {
      "id": "openai-evals-files-content-20261004",
      "url": "https://developers.openai.com/api/reference/resources/files/methods/content",
      "type": "official_docs"
    },
    {
      "id": "openai-evals-promptfoo-migration-20261004",
      "url": "https://developers.openai.com/cookbook/examples/evaluation/moving-from-openai-evals-to-promptfoo",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-10-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# OpenAI Evals の履歴保存と評価移行を別々に完了させる

## 問いと採用判断

評価基盤の終了前に、過去の回帰判定を後から説明できる状態で残し、新しい実行基盤へ移せるか。画面の合格率や `report_url` を控えるだけでは、どの入力・生成結果・採点規則でその値になったかが残らない。

本書は **既存runの証拠保全 → 新graderの一致検証 → 新しい生成run** の順序を提案する。採点方式やjudgeの偏りは[既存のLLM-as-a-judge文書](llm-as-judge-grading-bias-mitigation.md)、agentの試行分離・資源条件は[trial reliability文書](agent-eval-resource-envelope-trial-reliability.md)を参照する。新規性は終了告知の日付自体ではなく、APIの階層を取りこぼさず保存し、移行後の数字を以前と比較してよい条件を確かめる点にある。

## 一次資料で確認した日程と移行の意味

[OpenAI DeprecationsのEvals platform節](https://developers.openai.com/api/docs/deprecations#2026-06-03-evals-platform)は、2026-06-03の非推奨告知、2026-10-31の既存evalsのread-only化、2026-11-30のdashboard/API終了予定を区別する。eval workflowのgraderも対象だが、fine-tuningの期限は別項目である。日付の時刻・タイムゾーンは確認できていない。read-only化とデータ削除を同義にせず、終了後も取得できると期待しない。

[2026-06-03公開の公式Promptfoo移行記事](https://developers.openai.com/cookbook/examples/evaluation/moving-from-openai-evals-to-promptfoo)が扱うのは、prompt・provider・test case・assertionの**手動再構築**である。記事はexport機能を前提とせず、新たに実行した評価は過去のOpenAI Evals runとは別物だと説明する。類似度指標や再構築したgrader、特にLLM judgeは数値が同一になるとは限らず、移行後の検証が必要とされる。「公式移行記事がある」からといって、旧run履歴を自動importするAPIが保証されたとは扱わない。

## 確認した取得契約: 四つの保存対象

以下のpathは `https://api.openai.com/v1` を基点とする。認証済みの実API呼出しは行っておらず、API referenceの確認である。

| 保存対象 | 確認できた契約 | 保存時の意味 |
|---|---|---|
| eval定義 | [List evals](https://developers.openai.com/api/reference/resources/evals/methods/list)はproject単位。各evalの `data_source_config` と `testing_criteria`、`id`、metadata等を返す | schema・採点定義・参照IDを、集計結果とは別に保存する |
| run | [List runs](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/list)はeval配下。[Get run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/retrieve)は `data_source`、model、error、`result_counts`、criterion別結果等を返す | データ選択・prompt・実行条件と、その実行の集計を結びつける |
| 個別output item | [List output items](https://developers.openai.com/api/reference/resources/evals/subresources/runs/subresources/output_items/methods/list)はrun配下。 `id`、`datasource_item_id`、`datasource_item`、`sample`、`results` を返す | 原入力・生成結果・個別採点を残し、合計値だけの履歴にしない |
| 参照元データ | runのsourceは `file_content`、`file_id`、stored completionsの検索条件等を取り得る。[File content](https://developers.openai.com/api/reference/resources/files/methods/content)は指定fileの内容を返す | file IDや検索条件だけでなく、再評価に必要な入力の実体を保全する |

`sample` には生成時のinput/output、model、`finish_reason`、`error`、usage等があり、`results` はgraderごとの `name`、`passed`、`score` と任意の補助情報を持つ。`results[].sample` と生成結果の `sample` は別の位置にある。平坦化して片方を上書きしない。

runの `result_counts` は `passed`・`failed`・`errored`・`total` を区別する。特に `total` の説明は**実行されたoutput item数**であり、予定入力数ではない。`report_url` はdashboard上の描画済みレポートのURLであり、自己完結した評価データではない。

## 独自の設計案: 三階層paginationを最後まで追う

ここからの保存形式、照合条件、停止条件は本書の提案であり、OpenAIのsnapshot保証や公式export形式ではない。

1. **対象projectを明示する。** projectごとにevalを列挙し、その各evalのrun、各runのoutput itemsを列挙する。最初のeval一覧を取得しただけでは配下のrunを保存したことにならない。保存キーにproject・eval・run・output item IDを含める。
2. **全件保全ではstatus filterを付けない。** run一覧は `queued`、`in_progress`、`failed`、`completed`、`canceled` で絞れる。成功runだけに絞ると失敗履歴が消える。output item一覧のstatus queryは型欄が `fail` / `pass`、説明文が `failed` / `pass` で不一致である。実際に受理する値は未検証。全件保全にこのfilterは不要なので、省略した応答を保存する。
3. **各一覧で独立してcursorを管理する。** 三階層とも `after` と `has_more` / `last_id` を持つ。初回はafterなし、ページ保存後にそのlast_idを次のafterへ渡し、has_moreがfalseになるまで続ける。`limit` 件未満だったことだけを終了条件にしない。
4. **保存の完了後にcheckpointを進める。** HTTP応答bodyと取得条件・取得時刻・hashを保存してからcursorを更新する。再送で同じIDを見た場合は内容一致を照合する。同じIDで異なる内容なら古い方を黙って上書きせず、両観測を保持して確認する。
5. **進捗のない応答を止める。** has_more=trueなのに空ページ、last_id欠落、同じcursorを繰り返す状態は取得異常とする。部分保存を残し、無限loopや「成功0件」に変換しない。GET再取得と、評価runを新規作成する再実行を混同しない。

output itemの一意性は `id` で照合する。数値の `datasource_item_id` を全project共通キーやJSONLの行番号と決めつけない。異なるrun間で同じ入力を比較したいときは、元データの安定したtest case ID、版、内容hashとの対応表を別途作る。IDの一致と内容の一致のどちらも確認する。

## 独自の設計案: 保存完了と入力被覆を分ける

`queued` / `in_progress` のrunは変化中なので、取得できたデータを暫定保存する。最終保存では、対象runが終了状態になってから、取得前後のrun状態とcounterを読み、全ページの一意なoutput item集合を照合する。終了させるための自動cancelや新run作成は保全処理に含めない。

保存完了の判定では次を個別に記録する。

- 全階層でhas_more=falseまで到達したか
- 各output itemのeval_id/run_idが取得対象と一致するか
- 一意なoutput item数と安定した `result_counts.total` が一致するか
- itemのstatus、sample.error、個別grader結果と、runの集計との不一致がないか
- 入力実体と必要な参照資産を解決できたか。取得不能なら欠損一覧を残したか

これらは取得時の整合性検査であり、複数GETが一つのtransaction snapshotになる証明ではない。終状態・counterが変わった場合は完成扱いを保留して再取得する。件数だけ一致しても、別run混入や入力集合の取り違えを見逃すため、親ID・入力対応も必要になる。未知statusやfield省略を都合よくpassへ変換しない。

たとえば、予定入力120件のrunに実行結果110件があり、内訳がpassed=70、failed=30、errored=10だったとする。110個の一意なoutput itemを保存できれば**履歴110/110の取得**は照合できるが、**予定入力120/120の評価完了**は証明できない。実行されなかった10件は、元入力集合との比較で初めて見える。failedとerroredを統合したり、erroredを黙って分母から落としたりすると、移行前後の合格率の意味が変わる。集計ポリシーは別に明記する。

## 独自の設計案: 参照を実体と版へ解決する

- `file_content` は保存したJSONの内容も検証し、`file_id` は権限内でFile contentを取得する。file IDだけを保存してarchive完了とはしない。取得不能なら、その理由と残っているoutput item内の入力で補える範囲を明記する。
- stored completionsの検索条件は、過去に選ばれた集合の実体ではない。metadata・期間・limitだけで再検索して同じ集合になると仮定せず、実際に評価された `datasource_item` と元の母集合が得られる場合のsnapshotを分ける。
- 画像・音声・外部URLを使う評価では、JSON内の参照だけで再生できるかを点検する。必要な資産を許可された保存先に保全し、取得権限や保持方針で保存できないものをmissingとして扱う。
- 元HTTP bodyは変更しない保存物とし、検索用・集計用の正規化データは派生物にする。model、prompt、grader、dataset、harnessの版を取得できる範囲で記録する。model aliasを保存しただけで過去snapshotを固定したことにしない。
- 生成input/outputには顧客情報や秘密が含まれ得る。移行を理由にpublic repositoryへ置かず、アクセス制限・保有期限を維持する。匿名化した比較用fixtureと原本を区別し、匿名化後のhashで原本一致を主張しない。

## 独自の設計案: grader parityを固定出力で先に確かめる

移行後の新しいモデル出力をいきなり旧スコアと比べると、生成の変化と採点の変化が同時に入る。順序を次のように分ける。

1. **固定出力の採点比較。** 保存済みinput・reference・旧outputを新graderへ渡す。同じ回答に対する旧criterion別結果と比較し、正規化、label対応、pass threshold、欠測・error処理の差を調べる。旧sampleを新しいモデルに生成し直したものへ差し替えない。
2. **境界fixtureの確認。** 大文字小文字、前後空白、空出力、threshold直前・直後、judgeが不正形式を返す場合などを用意する。たとえば旧系の完全一致で不合格だった余分な空白を新系だけtrimして合格にすれば、品質向上ではなく採点契約の変更である。
3. **受入基準を事前に決める。** 決定的なcheckerは定義した入力域で一致を要求する。LLM judgeや類似度指標は、固定出力ごとの判定反転・score差と重要なsliceを確認し、許容範囲と人手確認対象を決める。全体平均が同じだけでは移行合格にしない。
4. **新規runとして生成も測る。** 採点側の差を説明できてから、固定したtest case集合と実行条件で新しい生成runを行う。旧run IDと新run IDを別に保存し、比較関係だけを記録する。新runの成功で過去の欠測や履歴欠損を埋めたことにしない。

履歴の取得成功、採点の一致、新生成の品質は三つの別判定である。CLIの設定検証成功やviewerが開くことは、いずれの代わりにもならない。

## 適用版・未確認事項

- 確認日は2026-10-04 UTC。終了告知・移行記事の公開日は2026-06-03であり、新発表として扱わない。終了が近づいた既存機能の運用上の空白を補う調査である。
- API referenceはrolling documentationで、ページ固有の版・公開日は確認できない。`/v1` のpathを確認したが、特定SDK版の互換性やページ例の実行成功は未検証。
- APIを呼ぶexporter、Promptfooの設定、実データによるgrader parity試験は実装・実行していない。Promptfooの正確な版やassertionの1対1変換は本書の保証対象外。
- output itemのstatus query表記不一致、snapshot分離、削除・保持期限後の復元、終了後の救済export、すべてのmultimodal資産が取得できるかは未確認。未取得のデータを保存済みとして報告しない。
- 取得したページ上でopen licenseを確認できなかったためsourceのlicenseはunknown。公式例のコードは取り込まず、独自要約と設計案のみを保存した。
- 本文の検索evalは発見性の検証であり、上記API・移行処理の実動作テストではない。read-only予定前に再確認するため、明示期限は2026-10-30とする。

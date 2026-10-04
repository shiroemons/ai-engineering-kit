---
{
  "id": "sre-canary-promotion-evidence-population-boundary",
  "title": "Canary 昇格判定: 母集団・観測窓・bake time と証拠不足の境界",
  "kind": "knowledge",
  "technology": "sre",
  "version": "Google SRE Workbook (2018) Ch.16; AWS Builders' Library Clare Liguori PDF (copyright 2020, exact publication date unverified); retrieved 2026-10-04 UTC; no deployment runtime test",
  "tags": [
    "research-domain:quality-operations",
    "sre",
    "canary",
    "control",
    "promotion",
    "bake-time",
    "sample-size",
    "inconclusive",
    "rollout",
    "rollback"
  ],
  "sources": [
    {
      "id": "sre-workbook-canary-evidence-20261004",
      "url": "https://sre.google/workbook/canarying-releases/",
      "type": "official_docs"
    },
    {
      "id": "aws-builders-canary-bake-evidence-20261004",
      "url": "https://d1.awsstatic.com/builderslibrary/pdfs/automating-safe-hands-off-deployments-clareliguori.pdf",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/quality-operations.json"]
}
---

# Canary の無警報を、昇格に足る証拠へ変換しない

## 問い・選定理由

「30分経過してアラートがない」だけで新しい版を全体へ広げてよいか。少量の新版本番トラフィックを評価する canary では、全体指標が正常でも新版だけ壊れたり、そもそも必要な経路を観測していなかったりする。本稿は、昇格の根拠となる母集団・時間・件数を一緒に設計するための判断材料である。

直近の Collector、testing、k6 の変更と重複しない、既存 SRE 領域の恒久的な空白を選んだ。2026年の新機能という位置づけではない。[既存の SLO / burn-rate](slo-error-budget-burn-rate-alerting.md) はサービス全体のリスク判断、[Prometheus 欠測](../observability/prometheus-missing-series-alert-staleness-boundary.md) は入力系列の消失、[k6 部分実行](../performance/k6-scenario-selection-threshold-coverage.md) は負荷試験の選択範囲を扱う。本稿は本番変更を次段階へ進める証拠の十分性を扱う。

## 一次資料から確認した事実

### Google: 比較可能な母集団と観測窓が必要

[Google SRE Workbook 第16章][google] は、canary を変更の部分的・期間限定の展開と評価として説明する。特に次の制約が重要である。

- canary と旧版 control の指標を分ける。サービス全体への集約は小さい canary の異常を薄める
- 十分な件数と期間が必要で、低負荷の時間帯だけでは高負荷時の性能問題を見逃し得る。非同期処理では少なくとも一つの work unit の処理期間を含める
- 評価する集計窓は canary の期間以下にする。変更前の障害を含む長い窓は、変更の評価を混濁させる
- before/after は時間帯の影響を受ける。同時比較でも共有依存先を通じて両群が悪化し得るため、相対差に加えて SLO などの絶対指標を確認する

これはサービス固有の十分な sample 数や統計的検出力を規定する標準ではない。同章は統計の詳細を対象外とし、実務上の設計条件を説明している。

### AWS: bake time は経過時間だけを待つ工程ではない

[Clare Liguori の運用記事][aws]の本文10〜11頁は、展開後も監視する bake time と、メトリクスの必要データ点数を組み合わせる実践を説明する。例として Create API の100要求を待つ条件を示すが、普遍的な合格値とは書いていない。

同記事は one-box 専用指標による rollback を全体アラームに追加し、要求数が突然ゼロになる異常も扱う。また、bake time 中の重大なアラームで自動的に戻す。本文5〜6頁では、新旧版の混在と旧版が新形式のデータを読めるかを事前検証する。これらは AWS 内部の運用事例であり、任意のデプロイ製品が同じ機能を持つ保証ではない。

## 昇格条件の設計（独自の提案）

以下の状態名・記録項目・順序は本稿の設計案であり、Google / AWS や特定 controller の API ではない。

### 1. 先に「何を観測できれば進めるか」を固定する

変更を開始する前に、最低限次の組を決める。

- 対象: release ID、artifact digest、canary / control の所属、対象地域・API・顧客機能群、実際の配分。意図した配分値と実際の要求数を分ける
- 時間: 新版が要求処理を開始した時刻、warm-up の扱い、判定窓の開始・終了、最低 bake time、評価の締切
- 網羅性: 必須経路ごとの最低件数・必要な負荷条件・非同期処理の完了観測。総要求数だけで希少経路の不足を埋めない
- 品質: 許容する canary / control の差、絶対的な安全限界、監視データの許容遅延、観測経路が正常である条件
- 対応: 停止・rollback の担当と実行経路、戻せない変更の扱い、未判定の延長・中止を決める責任者

部署全体や全tenantの名前を無制限に metric label 化する提案ではない。比較に必要な識別と cardinality 制約を両立させ、詳細な証拠は別の記録に残す。

### 2. 合格・不合格・証拠不足を分ける

本稿では `pass`、`fail`、`inconclusive` の三つで扱う。

1. `fail`: 定義済みの停止条件に達した、または安全上の回帰を確認した。最低件数や bake time がまだ満たされていなくても、被害を広げず停止・復旧を判断する
2. `inconclusive`: 必須経路が無通信、分母が0、monitoring が欠測・遅延、対象版の識別不能、比較群の汚染、最低件数や必要期間が不足。観測不能と業務失敗は診断上区別するが、どちらも自動昇格には使わない
3. `pass`: 必須の証拠が揃い、停止条件に達せず、相対差と絶対指標が採用条件を満たした。その観測範囲について次段階へ進める判断であり、未観測の全環境の安全証明ではない

既に明確な危険があれば、別指標の欠測によって `fail` を `inconclusive` に弱めない。逆に、要求がゼロで `0/0` になった成功率を100%へ補完しない。締切到達は `pass` を作るイベントではなく、保留を延長するか中止するかの判断点とする。

この設計での `inconclusive` は、既存 controller の同名状態との一致を意味しない。導入先で空結果、NaN、query error、途中終了、手動強制昇格がどう処理されるかを別途確認する。

### 3. 比較の成立と復旧可能性もチェックする

同じ時刻窓であっても、canary が重いAPI、control が軽いAPIだけを受けるなら、差を版の影響と断定できない。経路構成を揃えるか層別に比較し、比較できない層を残す。新版の request label が旧版へ再利用されないように、release ごとの証拠を保存する。

監視画面の描画時刻ではなく、sample の対象時刻と取り込み完了を記録する。たとえば時刻12:30の照会でも最新の確定値が12:20までなら、最後の10分はまだ評価できていない。窓を更新して待つことと、異常がないと結論することを分ける。

rollback は実行受付だけで完了としない。旧版への配分、新版処理の残存、業務指標の回復を別々に確認する。コードを戻すだけでデータ更新や外部副作用まで逆転するとは仮定しない。不可逆な変更には、補償や forward fix の手順が用意できるかを事前に確認する。

## 独自の反例で判断を確かめる

### 小さい canary の異常が全体で薄まる

同じ窓に全体80,000要求、canary 800要求、canary の失敗96件、control の失敗0件と仮定する。新版の失敗率は12%、全体では0.12%。全体の閾値を0.2%としていたら無警報でも、新版が健全とは判断できない。1%は配分実測値であり、instance 数の比から要求比を推測した値ではない。

### 100件すべて成功でも、低頻度の問題を否定できない

架空のモデルで、各要求が独立し真の失敗確率が1%で一定とする。100件で一度も失敗しない確率は `0.99^100 ≈ 36.6%`。これは Python で計算した数学的反例であり、実サービスの計測ではない。AWS の100要求の例を「99%の信頼度で安全」と読み替える根拠にはならない。

現実には要求が相関し、経路の偏りや時間変動もある。そのため、この独立モデルから万能の最低件数や本番の信頼区間を決めない。必要な検出力・業務別被害・試行方法を決めてから件数を設計する。

### 総件数が多くても必須経路が未観測

10,000件の閲覧と0件の更新を処理し、変更対象が更新時の書式変換だったとする。閲覧の成功率だけで新形式の読み書き互換性を判定しない。更新経路の証拠を要求し、無通信を合格件数へ加算しない。試験用の本番書き込みを追加するなら、その副作用・対象・認可を別に設計する。

### 両群の同時悪化は相対差だけでは通ってしまう

共通DBの負荷増加により、canary と control がともに失敗率4%になったと仮定する。差が0ポイントでも、絶対安全限界が1%なら拡大を止める。新版が原因かは追加調査が必要で、停止判断と原因の断定を同じものにしない。

## 採用先で行う受け入れ試験（未実行）

- canary だけを悪化させ、全体無警報でも版別判定で止められるか
- canary と control の両方を悪化させ、差がゼロでも絶対指標が止めるか
- 無通信、0/0、空結果、NaN、query error、古いsampleを別fixtureにし、自動的な成功補完がないか
- 最低件数の直前・同値・直後と、bake time の直前・同値・直後を組み合わせ、片方だけ満たしても進まないか
- 開始前のsample、warm-up中のsample、前releaseのlabelを混ぜ、判定窓と対象版が一致するか
- 十分な総件数でも必須APIだけ未観測にし、網羅性不足として扱えるか
- 長時間jobが実行中のまま締切に達する場合、処理開始だけを成功完了にしないか
- 停止条件に早期到達した場合、残りの bake time を待たず所定の復旧手順へ移れるか
- rollback後も新版処理や副作用が残る場合、復旧済みと誤報しないか

検索evalはこの文書への到達性だけを確認する。上のruntime試験、実際の統計的検出力、データ互換性、rollbackの成功は確認していない。

## 版・取得・ライセンス・限界

- Google: Alec Warner / Štěpán Davidovič らによる2018年版 Workbook 第16章。頁末のCopyright © 2018と CC-BY-NC-ND-4.0 を確認した。章の正確な公開日・最終更新日は表示から確定できない
- AWS: Clare Liguori の公式配布PDF。表紙はCopyright © 2020 Amazon Web Servicesとall rights reserved。2020年はcopyright年であり、正確な公開日・改訂日ではない。open licenseは確認できずcatalogはunknown。HTML記事はBuilder Centerへ転送され本文を取得できなかったため、本文を読めた14頁の公式PDFに根拠を限定した
- 両資料の取得日は2026-10-04 UTC。ソフトウェアの最新releaseや2026年の仕様変更を確認した資料ではなく、記載された運用原則・当時の実践を検討した。ソースコード解析は行っていないためrepository commitの固定対象はない
- 原文の翻訳再掲、図、設定例、コードのコピーは行わず、短い帰属付き要約と独自の判断手順・架空の反例を記した。moduleへ昇格しない
- 適切なtraffic比率、sample数、bake time、検定方式、製品固有の状態遷移、セキュリティ事故の影響上限は未検証。特にデータ流出などの被害を要求比率だけで上限化しない
- official_docs / maintainer_article のTTLはともに90日。2027-01-02を適用条件の再確認期限とし、歴史的事例がその日に誤りへ変わるという意味ではない

[google]: https://sre.google/workbook/canarying-releases/
[aws]: https://d1.awsstatic.com/builderslibrary/pdfs/automating-safe-hands-off-deployments-clareliguori.pdf

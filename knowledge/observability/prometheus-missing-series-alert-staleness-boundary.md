---
{
  "id": "observability-prometheus-missing-series-alert-staleness-boundary",
  "title": "Prometheus 欠測 alert: staleness・absent_over_time と発火／解消の境界",
  "kind": "knowledge",
  "technology": "observability",
  "version": "Prometheus 3.15 versioned query/alert docs; v3.15.0 release title 2026-09-24, publication display 2026-09-25; live rule-testing docs verified 2026-10-03 UTC; no runtime test",
  "tags": [
    "research-domain:quality-operations",
    "Prometheus",
    "missing-series",
    "staleness",
    "absent_over_time",
    "present_over_time",
    "keep_firing_for",
    "lookback",
    "bool",
    "unless",
    "promtool"
  ],
  "sources": [
    {
      "id": "prometheus-staleness-release-3-15-20261003",
      "url": "https://github.com/prometheus/prometheus/releases/tag/v3.15.0",
      "type": "release_notes"
    },
    {
      "id": "prometheus-missing-alert-rules-3-15-20261003",
      "url": "https://prometheus.io/docs/prometheus/3.15/configuration/alerting_rules/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-missing-query-basics-3-15-20261003",
      "url": "https://prometheus.io/docs/prometheus/3.15/querying/basics/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-missing-functions-3-15-20261003",
      "url": "https://prometheus.io/docs/prometheus/3.15/querying/functions/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-missing-operators-3-15-20261003",
      "url": "https://prometheus.io/docs/prometheus/3.15/querying/operators/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-missing-jobs-instances-20261003",
      "url": "https://prometheus.io/docs/concepts/jobs_instances/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-missing-rule-tests-20261003",
      "url": "https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# Prometheus の欠測を復旧と取り違えない

## 問い・追加理由・版の区別

障害中のメトリクスが消えて alert が解消したとき、サービス復旧と観測不能をどう区別するか。対象全体の欠測と、複数 target のうち1台だけの消失も分けて検知したい。

[Prometheus v3.15.0 release][release] は、storage が新しい series reference を返した際、まだ公開されている系列に stale marker を追加してしまう問題の修正 #19328 を記載する。release title は2026-09-24、GitHub の公開表示は9月25日。したがって「系列が消えたから exporter が止まった」と直結させず、収集・保存側の版と既知問題も診断対象にする。

本稿の中心は既存の欠測判定の実務的な空白であり、absent_over_time や keep_firing_for が3.15で初めて導入されたとは主張しない。query / alert は3.15 URLの文書、unit testing は取得できた live latest 文書を根拠とする。実装commitの解析や Prometheus / promtool の実行確認ではない。

既存の [SLO / burn-rate](../sre/slo-error-budget-burn-rate-alerting.md) は障害の重大度、[Alertmanager inhibition](alertmanager-inhibition-source-lifecycle-boundary.md) は通知抑止、[native histogram](prometheus-native-histogram-migration.md) は収集形式と集計移行を扱う。本稿は rule の入力がなくなる場合の検知・状態遷移を扱う。

## 確認した契約

### 1. alert は数値の真偽ではなく vector 要素の存在で active になる

[Alerting rules][alerts] は、式が返した vector の各 label set を active とする。for はその要素が評価のたびに継続して存在することを要求し、期間中は pending、期間を満たすと firing になる。keep_firing_for は firing 条件が消えた後の保持時間であり、for の代わりではない。未指定なら条件を満たさなくなった評価で解除される。

[Operators][operators] の比較は通常 filter として働き、条件を満たさない要素を落とす。一方 bool は対応する要素を0または1として残す。この二つの契約を組み合わせると、`up == bool 0` をそのまま alert 式にするのは危険である。正常な up=1 も値0の要素として残り、active になり得る。`up == 0` の値0は「false」ではなく、失敗した対象を選別した結果である。これは契約からの帰結であり、独自の真偽変換ルールを追加したものではない。

### 2. up=0・系列なし・業務正常は同じ状態ではない

[Jobs and instances][jobs] では、up は scrape が成功した場合1、失敗した場合0。業務トランザクションの成功率そのものではない。

[Querying basics の Staleness][basics] は、以前あった系列が scrape / rule evaluation で返らなくなる場合や target 削除時の stale 化を説明する。stale とされた後の評価ではその系列の値を返さない。instant selector が過去の最新値を探す lookback は既定5分だが、設定で変えられる。明示 timestamp を付ける exporter では挙動が異なり、track_timestamps_staleness の設定にも依存する。

したがって `up == 0` だけでは、対象が discovery から消えて up 自体も返らなくなった場合を表現できない。また「欠測時は必ず5分待ってから消える」という一律のタイマーとして lookback を扱わない。生の系列、scrape 状態、期待する対象一覧を別に観測するのが本稿の設計方針である。

### 3. absent_over_time は失われた label の一覧を生成しない

[Functions][functions] における absent は、入力に要素があれば空 vector、全くなければ値1の単一要素を返す。absent_over_time は range vector に対して同様の存在判定を行う。出力 label の一部は入力式から導出されるが、消えた全 target の label を復元する機能ではない。公式例でも正規表現で選んだ instance の一覧は出力されず、集約後の入力からは label が失われる場合が示されている。

たとえば本稿独自の式 `absent_over_time(app_worker_heartbeat{cluster="c1"}[10m])` は、選択範囲のどれか1系列でも窓内に値があれば空になる。10台中9台が消えても、残った1台が値を出していれば全体欠測の条件は成立しない。これは「9台とも正常」という意味ではない。

present_over_time は、窓内に値がある系列ごとに1を返す。[Operators][operators] の unless は右辺に一致する要素がない左辺を残し、on は照合 label を指定する。この組合せを使う場合も、存在してほしい集合は別途必要である。

## 採用判断（独自の設計提案）

以下は公式の完成済み設定ではなく、上記契約から作った設計案。指標名・label・時間窓は架空で、導入前に利用環境で検証する。

### 監視対象を三つに分ける

1. scrape 失敗: discovery 上に残る対象の up=0 を検知する
2. 期待する telemetry の欠測: 必須 metric / target が所定の窓に現れないことを検知する
3. 業務障害: 成功率や待ち時間を評価する。1と2が正常でも、業務結果は別途評価する

計画停止・scale-to-zero・新規targetの立上げ猶予を先に定義する。意図的な不在を異常としない一方、現在見えている系列だけを「期待集合」と定義して、消失と同時に義務まで消える循環を避ける。

### 対象単位の欠測は inventory と照合する

例として、独立した管理側が `expected_heartbeat_target{cluster,target}` を、監視必須なら1、対象外なら0で継続して公開するとする。cluster / target は両方のmetricで同じ意味・非空値を持ち、inventory の各組合せは一意にする。

独自の照合案は `(expected_heartbeat_target == 1) unless on(cluster, target) present_over_time(app_worker_heartbeat[10m])`。左辺の期待対象のうち、右辺に最近の heartbeat がないものを残す。これは「過去10分内に1回でも sample があるか」の判定であり、現在の稼働や10分間の連続正常を保証しない。heartbeat の値が0でも存在として数えるため、値に異常の意味を持たせるなら別条件を設ける。

- target 名が別 cluster で重なるなら cluster を照合から落とさない。誤った label 対応は別環境の1台が欠測を隠す原因になる
- inventory 自体が途絶えると左辺も消える。inventory 生成元の到達性・更新時刻と、監視経路全体の生存確認を独立に設ける
- discovery から削除された対象も追うには、期待集合が discovery の現在値だけに依存しない必要がある。退役の承認後に期待集合を変える運用とセットにする
- worker 再起動で変わるIDと論理target IDを混同しない。複数replicaのどれか1台が存在すればよいか、全台が必要かを照合キーで明示する

### 欠測窓・for・keep_firing_for を別々に予算化する

窓10分の absent_over_time に for: 5m を重ねれば、最後の実sampleから即座に5分で通知する設計にはならない。まず窓が空になり、その条件が各評価で5分継続する必要がある。評価周期・取り込み遅延・通知経路も含めて許容検知時間を決める。新規の空DBやまだ一度も観測していない対象では、rule作成時から10分待つという保証もない。

[Range Vector Selectors][basics] の窓は左端を含まず右端を含む。独自の理想例として、最後のsampleが時刻0、以後sampleなし、1分ごとに評価、`[10m]`、for: 2m とする。履歴が正常に見えて評価も継続する条件なら、時刻10分で窓から最後のsampleが外れて pending、12分で firing を期待する。この時刻は Alertmanager の受信・通知完了の保証ではない。境界直前・同値・直後を試験するための期待値である。

keep_firing_for は、すでに firing の alert が短い欠測や改善で解除されるのを抑える候補である。まだ一度も firing になっていない障害の欠測検知には代用できない。保持中の alert を「今も生の異常値が取れている」と表示せず、最後の観測時刻と一緒に扱う。解除が遅れる費用も受け入れる。

### 診断時の時刻を揃える

range query は step ごとの評価である。[Querying basics][basics] のこの性質から、粗いグラフが滑らかでも、rule の細かい評価時刻で欠測していないとはいえない。障害時は rule の評価時刻と同じ instant query、scrape 時刻、取り込み先、元の label set を記録する。subquery を使う場合は resolution も記録し、画面の step と rule の評価周期を同一視しない。

3.15.0 release の修正が示す通り、stale marker が target の実態とずれる実装不具合も切り分け候補になる。ただし特定の欠測を #19328 と断定したり、3.15.0 への更新がすべての欠測を直すと判断したりしない。通知の抑止・配送と、Prometheus の pending / firing も分けて確認する。

## 受け入れ試験案（未実行）

[Unit testing for rules][tests] は、input_series の `_` を欠測sample、`stale` を stale sample として区別する。promql_expr_test は値・label、alert_rule_test は指定 eval_time の firing alert を照合する。後者の exp_alerts が空でも、pending か inactive かまではそれだけで証明しない。

利用版と同じ promtool を用いる試験計画は次の通り。

- up=1、up=0、up自体なしを分け、filter 比較と bool 比較の返す要素を照合する
- 一時的な `_`、明示的な stale、長時間sampleなしを別fixtureにし、instant selector と窓付き存在判定を比較する。単なる欠測には古いsampleがlookback内で選ばれる場合がある
- 全target消失と1台だけ消失を分ける。全体の absent_over_time と inventory / unless の期待結果が異なることを確認する
- 期待labelの片側欠落、cluster違い、重複した論理ID、計画退役を入れ、過剰通知と見逃しを両方検出する
- 10分窓と2分forの例を境界前後で試し、窓内にsampleが1個戻る場合、新規の空履歴、評価間隔の変更も含める
- firing後の一時欠測、実値の正常化、保持期限前の再異常、保持期限後の再異常を分ける。pending中に入力が消える場合も別に置く
- inventoryだけ停止、telemetry経路だけ停止、両方停止を注入し、欠測判定を支える情報そのものの喪失を見逃さないか確認する
- query / rule評価エラーを「正常な空vector」と同じ成功結果へ変換しない。試験fixtureの欠測と、実際の収集・転送・通知経路の故障注入は別の試験として実施する

検索 eval が通ることは、この文章を取得できる証拠であり、これらの実機試験が通った証拠ではない。

## 適用限界・provenance

取得日は2026-10-03 UTC。3.15 URLの公開文書も内容の実装commit固定ではなく、本文固有の公開日は確認できなかった。unit_testing_rules の3.15 URLはnative webで取得失敗したため、実際に開けた latest 文書に限定した。3.15への完全一致や、新しいtest記法の過去版互換は主張しない。

旧版の影響範囲、#19328 のbackport状況、keep_firing_forの再起動時の保持、HA評価器の統合、remote storage固有のstaleness処理、実際の検知・通知遅延は未検証。releaseに修正が載ることと本番での再現・解消確認を区別する。外部monitoring製品へ同じ意味を無条件に移植しない。

Prometheus Authors の一次資料を独自に日本語要約し、判断手順・metric名・反例・試験案を加えた。サイトfooterと [docs/LICENSE][license] でApache-2.0を確認した。release prose固有のlicenseは未確認としてunknownを記録し、実装・設定・長文の転載やmodule昇格は行っていない。最短TTLはrelease_notesの30日なので再確認期限は2026-11-02。

[release]: https://github.com/prometheus/prometheus/releases/tag/v3.15.0
[alerts]: https://prometheus.io/docs/prometheus/3.15/configuration/alerting_rules/
[basics]: https://prometheus.io/docs/prometheus/3.15/querying/basics/
[functions]: https://prometheus.io/docs/prometheus/3.15/querying/functions/
[operators]: https://prometheus.io/docs/prometheus/3.15/querying/operators/
[jobs]: https://prometheus.io/docs/concepts/jobs_instances/
[tests]: https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/
[license]: https://github.com/prometheus/docs/blob/main/LICENSE

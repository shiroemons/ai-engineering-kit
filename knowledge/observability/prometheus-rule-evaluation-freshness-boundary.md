---
{
  "id": "observability-prometheus-rule-evaluation-freshness-boundary",
  "title": "Prometheus rule評価: limit・後続ruleの旧値利用・query_offsetと鮮度の境界",
  "kind": "knowledge",
  "technology": "observability",
  "version": "Prometheus 3.15 docs and v3.15.0 commit 5241a27fe3c6983549fccc32f6e65917408c63cd; live rule-testing docs verified 2026-10-05 UTC; runtime not tested",
  "tags": [
    "research-domain:quality-operations",
    "Prometheus",
    "recording-rules",
    "rule-group",
    "limit",
    "query_offset",
    "lookback",
    "lastEvaluation",
    "evaluation-failure",
    "promtool"
  ],
  "sources": [
    {
      "id": "prometheus-rule-release-3-15-20261005",
      "url": "https://github.com/prometheus/prometheus/releases/tag/v3.15.0",
      "type": "release_notes"
    },
    {
      "id": "prometheus-rule-recording-docs-3-15-20261005",
      "url": "https://prometheus.io/docs/prometheus/3.15/configuration/recording_rules/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-rule-config-3-15-20261005",
      "url": "https://prometheus.io/docs/prometheus/3.15/configuration/configuration/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-rule-basics-3-15-20261005",
      "url": "https://prometheus.io/docs/prometheus/3.15/querying/basics/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-rule-api-docs-3-15-20261005",
      "url": "https://prometheus.io/docs/prometheus/3.15/querying/api/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-rule-features-3-15-20261005",
      "url": "https://prometheus.io/docs/prometheus/3.15/feature_flags/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-rule-testing-live-20261005",
      "url": "https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-rule-engine-3-15-20261005",
      "url": "https://github.com/prometheus/prometheus/tree/5241a27fe3c6983549fccc32f6e65917408c63cd/rules",
      "type": "github_repository_analysis"
    },
    {
      "id": "prometheus-rule-api-implementation-3-15-20261005",
      "url": "https://github.com/prometheus/prometheus/blob/5241a27fe3c6983549fccc32f6e65917408c63cd/web/api/v1/api.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2026-11-04",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# Prometheus の rule 評価成功と入力の鮮度を分ける

## 問い・追加理由・適用範囲

Recording rule を段階的に組み合わせたとき、後段の `health: ok` と新しい `lastEvaluation` だけで、最新の入力に基づく正常な判定といえるか。出力系列の上限、評価の遅延、取り込みの遅着で、見かけ上正常な dashboard や alert を作らない条件を整理する。

対象は Prometheus 3.15 の公式文書と v3.15.0 の実装 commit `5241a27fe3c6983549fccc32f6e65917408c63cd`。[release][release] のタイトル日は2026-09-24、公開表示は9月25日。以下の機能が3.15で初めて導入されたという主張ではなく、既存の重要な運用契約を補う調査である。

既存の [欠測 alert](prometheus-missing-series-alert-staleness-boundary.md) は入力系列が消える場合、[inhibition](alertmanager-inhibition-source-lifecycle-boundary.md) は通知抑止を扱う。本稿は評価器自身の失敗、依存rule間の古い値の再利用、評価時刻のずれを扱う。実機の Prometheus / promtool は実行していない。

## 確認した契約

### 1. group の limit は各 rule の出力に適用され、超過分だけを切り捨てない

[Recording rules][rules-doc] の `limit` は group に設定するが、各 recording rule の系列数、各 alerting rule のalert数を制限する。既定0は無制限。上限を超えると、そのruleの出力全体を捨て、alerting rule なら保持するalert状態も消去する。評価エラーとして記録され、この失敗から stale marker は書かれない。

[固定版 rules 実装][rules-src] の `RecordingRule.Eval` は query とlabel処理を終えた後のvector長が正のlimitを超える場合にerrorを返す。`AlertingRule.Eval` はpendingとfiringの対象を数え、超過時にactive map全体を空にする。したがって本稿独自の例では、limit: 100で100系列は許容、101系列は「100系列だけ保存」にならない。別々のruleが80系列ずつ返す場合、合計160という理由だけでlimit: 100に達するわけでもない。

このlimitをquery計算量や入力系列数の制限とみなさない。集約結果が1系列でも、走査入力が巨大なら重いことは [Querying basics][basics] も注意している。また [HTTP API][api-doc] のinstant queryの `limit` は返すvector / matrixの系列数を切り詰める応答側の機能であり、ruleの失敗契約とは異なる。診断用APIでlimit: 100相当の結果しか見ず、「ruleも100以下だった」と結論しない。

### 2. 同一 group の順序は、前段成功を条件にするtransactionではない

既定では同一groupのruleを順番に、同じ評価時刻で実行する。[Group.Eval][group-src] の実装では、一つのruleがerrorを返すとそのruleのhealthとerrorを更新して処理を戻すが、groupの後続ruleをすべて中断するわけではない。成功済みruleの出力をgroup単位で巻き戻す処理でもない。

一方、instant selectorはlookback内の最新sampleを使う。既定lookbackは5分だが設定可能で、stale marker後の値は選ばない。[Querying basics][basics] と前項の「評価エラーではstale markerなし」を組み合わせると、前段が今回失敗しても、後段が前回の値を利用できる場合がある。

以下はこの契約から導く独自の反例で、実機試験結果ではない。

1. Aというrecording ruleが12:00に正常な値を保存する
2. 12:01、Aの出力がlimitを超えて今回の評価は失敗する。今回分は保存されず、この失敗でAの旧系列をstaleにはしない
3. 同じgroupでAを読むBは引き続き評価される。旧値がlookback内なら、Bは12:00のAを入力として成功し得る
4. Bのhealthが良くても、Aの12:01評価が成功した証明にはならない

対照的にAが正常評価で空vectorを返した場合は、以前あった出力系列に対する通常のstaleness処理へ進む。「正常に0件」と「失敗して今回0件」を監視上同じ値に丸めると、この違いを失う。

### 3. alertの消去と lastEvaluation の更新は、業務復旧の証拠にならない

[固定版 AlertingRule.Eval][alert-src] の上限超過は既存の状態も空にするため、`keep_firing_for` がこの消去を防ぐとは扱えない。上限解消後、まだ異常なlabel setが戻れば新しいpending状態を作る。forが正なら再び待ち時間を生じ得る。長いforと件数超過の繰返しを組み合わせると、障害継続中でも発火まで進まない危険がある。

ただし、すべてのquery errorでactive mapが消えるとはいえない。同じ実装ではquery自体のerrorは状態更新前に返る。件数超過、式の実行失敗、正常な空vector、評価スキップを別の状態として診断する。Prometheus内のalert消去と、Alertmanagerでの通知・解消時刻も同一ではない。

[DefaultEvalIterationFunc][manager-src] はgroup評価を実行した後、各ruleの成否を全件成功条件にせず最終評価時刻を更新する。各ruleの評価時刻もerror経路を含むdeferで更新される。[固定版 API 実装][api-src] はこれらを `lastEvaluation` として返し、ruleごとの `health` / `lastError` / `evaluationTime` を別に返す。時刻が進むことは「試行が行われた」証拠であって、最後の成功時刻という意味ではない。

### 4. 遅いgroupの次回評価はキューに積まれず、欠番になる

[Recording rules][rules-doc] は、前回group評価が次の予定時刻までに終わらないと、その回をスキップすると説明する。完了またはtimeoutまでさらに評価を取りこぼし、recording結果に穴が生じる。間隔を短くしても、処理が追いつかない場合にすべての時点を評価できる保証はない。

[Feature flags][features] ではgroup同士は既定でも並行に動き、`concurrent-rule-eval` は同一group内で依存のないruleを並行化する。これはqueryの同時負荷を増やす。依存するA→Bを別groupに移して並列化することは、同一評価回のAを必ずBが読む保証を作らない。周期が同じであることと、実行順序が揃うことを混同しない。

## 運用判断（独自の設計提案）

### 監視するのは「試行・成功・鮮度・業務値」の四つ

次の指標名とlabelの組み立ては [v3.15.0 Group metrics][group-src] を確認したもの。閾値・監視経路の選択は本稿の提案である。

| 観測 | 確認対象 | 単独ではいえないこと |
|---|---|---|
| 評価失敗 | `prometheus_rule_evaluation_failures_total` の増分、ruleのhealth / lastError | 正常な空結果との同一視、最終通知の成否 |
| 予定評価の取りこぼし | `prometheus_rule_group_iterations_missed_total` の増分 | 対象サービスが正常であること |
| 周期に対する所要時間 | `prometheus_rule_group_last_duration_seconds` と `prometheus_rule_group_interval_seconds` | 過去最大負荷でも期限内に終わること |
| 評価試行の新しさ | `prometheus_rule_group_last_evaluation_timestamp_seconds` | group全ruleの成功、前段入力の新しさ |
| 依存する出力と元入力 | 必須ruleの成否、期待するlabel集合、入力sampleの時刻 | 最終ruleだけ見て依存全体を合格にすること |

`rule_group` labelはgroup名単体ではなく、固定版実装ではfileとnameをセミコロンで連結する。監視selectorの文字列を推測せず、実際の `/metrics` を確認する。移動・改名時は監視側の対象集合も照合する。

lastErrorは次の正常評価で消去されるため、現在のhealthだけで短い失敗を見逃さない。失敗counterの履歴、log、rule定義の版を保存する。lastEvaluationが新しいのに失敗counterが増える状態、最終評価時刻が長時間更新されない状態、全評価成功だが入力が古い状態を分ける。

固定版 `Group.run` は同期的なgroup評価から制御が戻り、次のtickを処理するときにmissed回数を加算する。評価が停止したままの間、counterが逐次増えるわけではない。missed増分が0でも、現在進行中の長時間評価や停止を除外できないため、lastEvaluationの経過時間を独立に監視する。回復後にまとめて増えたcounterを、その取得時刻に初めて発生した障害と読み違えない。

自分を監視するruleを重い業務groupだけに置くと、そのgroupの停止で診断まで失う。低負荷の独立groupや別の監視経路を検討し、Prometheus本体停止・自己scrape失敗も試す。別groupに分けるだけでは同一processの障害から独立しない。

### query_offset は取り込み遅延の余裕であり、履歴の完全性保証ではない

[Global configuration][config] の `rule_query_offset` は既定0s、groupの `query_offset` で上書きできる。評価対象時刻を過去へずらし、remote write等の取り込みが間に合う余地を作る。固定版 `TestGroup_QueryOffset` はglobal 1mのとき、group 2mは2m、明示0sは0、未指定は1mになることを検査する。0sを「global値を継承」と解釈しない。

独自の時刻例として、予定評価時刻12:00、query_offset: 2m、式の窓が `[5m]` なら、通常の評価対象は11:58、窓は左端を除く11:53から右端を含む11:58となる。さらにselectorへ `offset 1m` を付けると、そのselectorは11:57を基準にする。これはgroup周期を2分延ばす設定でも、実行を2分sleepする設定でもない。`@` 指定を含む式は個別に時刻を追う。

採用時は元sampleの時刻と保存先で読める時刻の遅れを測り、通常時だけでなく詰まりからの回復時も評価する。2m設定は2m以内の到着を保証せず、取り込み欠損・恒久拒否・より遅いsampleは別問題である。過去を評価するぶん障害検知が遅れる費用を、for・評価周期・通知時間と一緒に見積もる。offsetを増やせば短い検知時間を保てるとは主張しない。

AとBでoffsetや周期が違う場合は、Bのquery時刻に対応するAが既に保存済みかを確認する。診断用instant queryも同じ評価時刻へ固定する。ただし後から遅着sampleが入れば、後日同じ時刻を再検索した結果が当時と一致するとは限らない。過去の成功した再検索だけで当時のrule errorを否定しない。

### 修正を入れる順番

1. rule/file/group、設定limit、出力件数、評価周期、offset、実装版とerrorを記録する。limitの無制限化を最初の処置にしない
2. 意図しないlabel追加や集合拡大なら入力・集約設計を直す。正当な増加なら容量と障害時の増加余地を測ってlimitを調整する
3. 重い式を絞り、前処理やgroup分割を検討する。分割前に依存を洗い出し、同一時刻の結果が必要な経路を壊さない
4. 取り込み遅延が根拠ならoffsetを設計する。計算が遅い問題をoffsetだけで直そうとしない
5. 修正後はAPIのロード済みrule一覧と期待一覧を照合し、全必須ruleの評価成功、入力鮮度、後段の値を確認する。alertが消えただけでは復旧完了にしない

[Rules API][api-doc] のfilterでruleが除外された場合はgroup自体が返らないことがあり、`group_limit` は応答group数のpaginationである。全件確認では最後の `groupNextToken` 消失まで確認し、途中reloadでsnapshot整合性が保証されない点を扱う。件数制限の設定 `limit`、query応答のlimit、一覧のgroup_limitを別物として記録する。API envelopeのsuccessはrule全体の評価成功を意味しない。

## 受入れ試験案と未検証部分

[Unit testing for rules][tests] の `promql_expr_test` と `alert_rule_test` は、入力系列に対する値・label・発火の期待を固定する用途に使う。`group_eval_order` が与える順序は試験で指定したgroupに対するもので、本番の並行groupに同じ順序を与える設定ではない。`promtool check rules` のsyntax成功を、負荷・遅着・通知経路の合格にしない。

以下は提案であり、この調査で実行していない。

- limitの直前・同値・1超過を試す。別ruleの合計ではなく、各ruleの出力で判定されることを確認する
- 既にfiringのalertを作ってから上限超過を起こす。keep_firing_forあり／なし、上限解消後のpendingと再発火、Alertmanager側の実際の状態を別々に記録する
- A成功→A上限超過→B評価を同一groupで試し、Bが使った入力時刻を確認する。正常な空vector、query error、limit errorを別fixtureにする
- lookback内と外、明示staleを分ける。Bの成功だけを合格条件にせず、Aのhealthと元入力の欠測もassertする
- 重い評価を周期越えさせ、取りこぼしcounter、実際に記録されたtimestamp、回復後の穴を確認する。失われた全評価が自動補完されることを期待しない
- global offsetを設定して、group未指定・0s・別値を比較する。式内offset、異なる周期、遅着のあるA→Bの組合せも含める
- rule一覧のfilter・pagination、reloadによる変更、期待ruleの未ロードを試す。「返った分だけ正常」で全体成功を出さない
- 評価errorが次回成功で見えなくなる場合と、評価器そのものが止まる場合を故障注入する。自己監視だけに依存しないことを確認する

上流の `TestRecordingRuleLimit` / `TestAlertingRuleLimit` は2件入力に対する上限2の許容と上限1のerrorを確認している。これはテストの内容を読んだ結果であり、ローカル実行成功の報告ではない。また上限超過を挟む長期のalert遷移、実際の後段入力再利用、missed回数と通知遅延を全てそのテストが証明するわけでもない。

取得日は2026-10-05 UTC。公式3.15文書の本文固有の公開日は未確認。unit testingの3.15 URLはnative webで取得失敗したため、開けたlatest文書として版を分けた。実装fileのnative web取得もcache missとなり、GitHubのread-only APIで固定commitの本文を確認した。releaseを除く公開文書は更新可能なURLであり、固定実装と同じ不変snapshotとは扱わない。

Prometheus Authorsの文書・実装を日本語で独自要約した。サイトfooter、[docs/LICENSE][docs-license]、[固定版LICENSE][code-license]と各fileのheaderでApache-2.0を確認した。release prose固有のlicenseはunknown。コード・大きな設定例の転載やmodule昇格は行っていない。再確認期限はrelease_notesの最短TTL 30日に合わせ2026-11-04。

他のPrometheus互換製品、HA評価器、remote storage固有のlookback、旧版/backport、再起動時の状態復元、実負荷の改善率は未確認。検索evalは本文への到達性を検査するもので、ここに挙げたruntime試験の代替ではない。

[release]: https://github.com/prometheus/prometheus/releases/tag/v3.15.0
[rules-doc]: https://prometheus.io/docs/prometheus/3.15/configuration/recording_rules/
[config]: https://prometheus.io/docs/prometheus/3.15/configuration/configuration/
[basics]: https://prometheus.io/docs/prometheus/3.15/querying/basics/
[api-doc]: https://prometheus.io/docs/prometheus/3.15/querying/api/
[features]: https://prometheus.io/docs/prometheus/3.15/feature_flags/
[tests]: https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/
[rules-src]: https://github.com/prometheus/prometheus/tree/5241a27fe3c6983549fccc32f6e65917408c63cd/rules
[group-src]: https://github.com/prometheus/prometheus/blob/5241a27fe3c6983549fccc32f6e65917408c63cd/rules/group.go
[manager-src]: https://github.com/prometheus/prometheus/blob/5241a27fe3c6983549fccc32f6e65917408c63cd/rules/manager.go
[alert-src]: https://github.com/prometheus/prometheus/blob/5241a27fe3c6983549fccc32f6e65917408c63cd/rules/alerting.go
[api-src]: https://github.com/prometheus/prometheus/blob/5241a27fe3c6983549fccc32f6e65917408c63cd/web/api/v1/api.go
[docs-license]: https://github.com/prometheus/docs/blob/main/LICENSE
[code-license]: https://raw.githubusercontent.com/prometheus/prometheus/5241a27fe3c6983549fccc32f6e65917408c63cd/LICENSE

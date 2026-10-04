---
{
  "id": "performance-go-benchmark-harness-comparison-boundary",
  "title": "Go benchmark比較: B.Loopの計測範囲・最適化とbenchstat判定の境界",
  "kind": "knowledge",
  "technology": "performance",
  "version": "B.Loop introduced Go 1.24.0; inlining change Go 1.26.0 (2026-02-10); API/source verified Go 1.27.1 @ 862c888e612ac346c7c4d99c9392bdfd265f33b0; benchstat v0.0.0-20260908200009-22c9c6c9d4da",
  "tags": [
    "research-domain:quality-operations",
    "go",
    "benchmark",
    "B.Loop",
    "benchstat",
    "measurement",
    "inlining",
    "regression",
    "statistics"
  ],
  "sources": [
    {
      "id": "go126-bloop-inlining-release-20261004",
      "url": "https://go.dev/doc/go1.26",
      "type": "release_notes"
    },
    {
      "id": "go-benchmark-release-history-20261004",
      "url": "https://go.dev/doc/devel/release",
      "type": "release_notes"
    },
    {
      "id": "go127-bloop-testing-api-20261004",
      "url": "https://pkg.go.dev/testing@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go-bloop-design-article-20261004",
      "url": "https://go.dev/blog/testing-b-loop",
      "type": "maintainer_article"
    },
    {
      "id": "go127-bloop-timer-source-20261004",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/testing/benchmark.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go-perf-benchstat-docs-20261004",
      "url": "https://pkg.go.dev/golang.org/x/perf@v0.0.0-20260908200009-22c9c6c9d4da/cmd/benchstat",
      "type": "official_docs"
    },
    {
      "id": "go-perf-benchstat-command-source-20261004",
      "url": "https://github.com/golang/perf/blob/22c9c6c9d4da6248aedbc79f02ecedcd59f8f5f2/cmd/benchstat/main.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# Go benchmarkの数値を変更の効果として読める条件

## 問いと追加理由

アプリケーションの変更後に `ns/op` が改善したとき、実際に処理が速くなったのか、それとも計測用コード（harness）、compiler、入力状態を変えてしまったのか。CIでbenchstatを実行しただけで、性能の合否まで決まるのか。

本稿は**計測対象の同一性を確認してから統計的な比較へ進む**ための知識である。既存の[k6による負荷生成](k6-coordinated-omission-arrival-rate-latency.md)や[scenario選択](k6-scenario-selection-threshold-coverage.md)が扱う外部負荷とは異なり、process内のmicrobenchmarkの測定範囲と比較可能性を扱う。221文書の全体目録と本文の関連語、直近16件のknowledge変更、およびKB検索で重複を確認した。

新規の10月機能を発見したという主張ではない。[Go 1.26 release notes](https://go.dev/doc/go1.26#testing)のB.Loop変更は2026-02-10のものであり、[2025-04-02の導入記事](https://go.dev/blog/testing-b-loop)と実装の説明が変わっている。現在のGo 1.27.1で再確認して、k6に偏っていたperformance領域の比較手順を補う。

## 版をまたぐと変わる測定対象

[Release History](https://go.dev/doc/devel/release)とtesting文書を照合すると、B.Loopの導入はGo 1.24.0（2025-02-11）、loop内のinlining禁止撤廃はGo 1.26.0（2026-02-10）である。Go 1.27.1は2026-09-01公開で、本稿のAPI・固定source・局所試験の対象とした。Go 1.27のrelease notesも確認したが、本稿のinlining変更を1.27の新機能とはしない。

- Go 1.24の導入記事は、不要な処理除去を防ぐ実装としてloop内へのinliningを禁止したと説明する
- Go 1.26のrelease notesは、その禁止が予期しないallocationや遅さを生み得たため撤廃したと説明する。引数・戻り値・代入変数を生存させる仕組みは維持される
- [Go 1.27.1 API](https://pkg.go.dev/testing@go1.27.1#B.Loop)では、生存保持はcompilerによるKeepAlive intrinsicへの変換として説明される。公開説明上の対象はloopの波括弧内の文で、条件は `b.Loop()` の形を正確に使う

したがって「同じ関数名・同じns/opだから比較可能」とは限らない。旧toolchainの `b.N` 結果と、新toolchainでB.Loopへ書き換えた結果の差には、アプリ変更・compiler変更・harness変更が同時に入る。inlining許可はすべての呼出しが必ずinline化される約束でも、各benchmarkの速度が必ず同じになる約束でもない。

条件を `b.Loop() && ready` に変えたり、method valueや独自wrapperへ隠したりして、同じ最適化抑止が働くと期待しない。逆にB.Loopだからcompiler最適化が全面的に無効になる、とも考えない。極小関数の疑わしい結果は、実際の呼出し形・入力の定数化・最適化後の処理を個別に確認する。

## B.Loopが自動管理する範囲と、残る責務

[API](https://pkg.go.dev/testing@go1.27.1#B.Loop)が約束するのは、初回のLoopでtimerをresetし、falseを返すとtimerを止めること、1測定につきbenchmark関数を1回呼ぶこと、完了後の `b.N` に総反復数を置くことである。B.Loopと `b.N` の反復を同じbenchmarkに混在させない。

### 入力は反復ごとに同じ仕事をさせる

B.Loopは入力の状態を復元しない。たとえば同じsliceを繰り返しsortすると、初回だけ未整列で、その後は整列済みを測る。cacheのmissを調べたいのに同じkeyだけを再利用する場合も、warmな経路へ変化する。

[導入記事](https://go.dev/blog/testing-b-loop)は、反復内の入力準備は必要に応じて自分でStopTimer/StartTimerを管理する責務だと説明する。独自の設計として、実サービスで入力copyも処理の一部ならcopyを含め、sort本体だけを調べるなら準備を除く。その選択は前後で揃えて記録する。準備時間を除いても、準備によるcacheの温まりやallocationの後続影響まで消えるわけではない。

### custom metricは最初のLoopより前に確定しない

[固定版benchmark.go](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/testing/benchmark.go)の `loopSlowPath` は初回に `ResetTimer` を呼ぶ。`ResetTimer` は時間・allocation counterだけでなく、`ReportMetric` の追加値も消す。このため、setupで報告したcustom metricは、最初のLoopの後に残るとは限らない。

独自の例として、loop前に `ReportMetric(7, "before/op")` を呼び、loop終了後に反復数から `after/op` を報告すると、前者は消え、後者は残る。`b.N` を分母にするper-operation指標はLoop完了後に計算する。これは `SetBytes` などすべての設定が消えるという主張ではない。

### 手動timer管理と早期終了は試験失敗になり得る

同じ固定版はtimer停止中にLoopを呼ぶと失敗させる。反復内でStopTimerしたら次のLoop判定前にStartTimerへ戻す。setup前にStopTimerして、そのままB.Loopが自動で開始してくれると考えるのも誤りである。

また `break` 等でLoopがfalseを返す前にbenchmark関数から正常に抜けると、固定版の `runN` は未完了のloopを検出してbenchmarkを失敗させる。処理失敗を検出した場合はbenchmark自体を失敗として報告する設計にし、成功した少数回だけで予定の測定を終えたように見せない。これらはGo 1.27.1の実装観察であり、任意の過去版の診断文を保証しない。

## benchstatが判定すること、しないこと

[確認したbenchstat文書](https://pkg.go.dev/golang.org/x/perf@v0.0.0-20260908200009-22c9c6c9d4da/cmd/benchstat)は2026-09-08のsnapshotである。新しい統計方式の導入日という意味でも、最新版という意味でもない。

既定の `assume=nothing` はmedianによる要約とMann-Whitney U-testによるA/B比較を使い、差を検出できない欄には `~` を表示する。有意差と実務上大きい差は別である。`assume=exact` はbinary sizeのような無雑音の値を想定した指定で、時間測定のsample不足を隠すために使わない。

実行回数は最低10、できれば20を事前に決め、before/afterを交互に収集するのが公式の助言である。10回で必ず十分な検出力を得るという保証ではない。有意になるまで追加実行する行為や多数のbenchmarkを無調整で比較する行為にはmultiple testingの問題がある。既定のalpha 0.05を「個々の改善が95%の確率で真」とは読まない。

[固定版command](https://github.com/golang/perf/blob/22c9c6c9d4da6248aedbc79f02ecedcd59f8f5f2/cmd/benchstat/main.go)の実装は、結果をtableへ整形する経路に性能悪化を非ゼロ終了へ変換する判定を持たない。一部の `SyntaxError` はstderrへ警告し、残りのrecordの処理を続ける。さらに `main` で戻りerrorを印字して終わる経路もある。したがってexit status 0だけでは「regressionなし」も「入力が完全で妥当」も確認できない。全種類のエラーが常に0になるという主張ではなく、引数なし等には別の終了経路がある。

## 比較を承認する手順（独自の運用提案）

1. **仮説と測定単位を固定する。** 調べる変更、必須benchmark、入力集合、cold/warm状態、直列/並列、主要指標、許容劣化幅、sample数、除外条件を先に決める。結果を見てから都合よく規則を変えない
2. **同じharnessで前後を再測定する。** アプリ変更の影響なら同じGo版・同じharnessでbefore/afterを作る。toolchain比較なら同じアプリrevision・同じharnessで旧/新版を測る。移行の影響も調べるなら「旧/新compiler × 旧/新harness」の4条件を別々に保存し、1本の速度改善率へ潰さない
3. **入力と環境の証跡を残す。** source revision、harness revision、Go patch版、GOOS/GOARCH、CPU、GOMAXPROCS、build flags、PGO profileの有無、quota、sample採取順を保存する。benchstatの既定groupingはfile-level設定ごとにtableを分けるが、記録していない環境差を検出してはくれない。`-ignore` で違いを隠して比較可能にしたことにしない
4. **収集の完全性を先に検査する。** 全commandの成功、必須のbenchmark名・単位・sample件数、before/after対応、警告を確認する。ゼロ件、片側だけ、途中打切り、読めないrecordは証拠不足とする。何件か残ったtableの生成を全件の合格にしない
5. **判定を分ける。** `~` は検出できなかった結果として扱い、同等性を証明したことにしない。有意でも許容差より小さい変化と、業務上許容しない劣化を分ける。ゲートには事前に定めた差の方向・効果量・不確実性・入力完全性を使う。benchstatの終了codeをその代わりにしない
6. **必要な測定階層へ戻す。** microbenchmarkの改善からrequest全体のp99改善を直接結論しない。[RunParallelの契約](https://pkg.go.dev/testing@go1.27.1#B.RunParallel)で `ns/op` はbenchmark全体のwall timeに基づき、各operationの応答時間分布ではない。重要経路は現実的な負荷・競合・I/Oを含む測定で別に確かめる

例として、同じcompilerとharnessで100 ns/opから104 ns/opに変わり、事前に許容劣化を2%と決めていたなら、4%の観測差とその証拠の強さを調べる。一方、sample不足で `~` になったことだけを理由に許容内と認定しない。2%は説明用の仮定で、Goやbenchstatの推奨閾値ではない。

## 局所試験で確認したこと

2026-10-04 UTCにrepository外の一時directoryへ独自の最小Go testを作り、locked Go 1.27.1 / linux/amd64で `go test -run='^$' -bench='<対象名>' -benchtime=3x -count=1` を実行した。固定commitから取得したbenchmark.goとtoolchain同梱sourceはbyte一致した。

| ケース | 実際の観測 | 判断に使える範囲 |
|---|---|---|
| loop前before/op、完了後after/op、反復数一致をassert | 3反復、after/op=1、before/opなし、成功 | custom metricのresetと完了後b.Nの局所確認 |
| 各反復でStopTimer後にStartTimer | 3反復で成功 | 手動再開を含む形がこの版で動く |
| StopTimer後に再開せず次のLoopへ進む | benchmark失敗、test終了1 | timer停止中の呼出しを黙って測定しない |
| 最初の反復でbreak | 未完了loopとして失敗、test終了1 | 早期正常脱出を完了としない |

これは性能差を調べる試験ではない。表示されたns/opは3回だけの局所値なので比較結果として掲載しない。Go 1.24/1.26の比較実行、inliningのassembly比較、benchstat binaryの実行、sample数の検出力計算は実施していない。sourceで確認したことと実行したことを区別する。

## 導入時の受け入れケース（未実行の提案）

| 条件 | 期待する運用上の扱い |
|---|---|
| compilerとB.Loop移行を同時に変えたbefore/after | アプリ変更だけの効果として昇格せず、揃えた条件で再収集 |
| sort入力を反復で再初期化しない | warm入力のbenchmarkとして明示、未整列入力の代用にしない |
| before側だけ必須benchmarkが欠落 | tableの有無に関係なく証拠不足 |
| 途中の1 recordが壊れ、他の行はtable化された | stderrと件数を検査し、成功扱いを止める |
| 微小な差は有意、重要経路の劣化は不確定 | 有意/不確定と業務上の許容差を別々に報告 |
| 同じ入力で有意になるまで再測定 | 事前の採取計画へ戻し、選別された結果を正式比較に使わない |
| RunParallelのns/opが改善 | 個別requestのp99が改善したとは書かない |

## 来歴・適用限界

全sourceは2026-10-04 UTC取得。GoはtagをGitHub APIで40桁commitへ解決し、`862c888e612ac346c7c4d99c9392bdfd265f33b0` のtesting/benchmark.goを読んだ。x/perfは `22c9c6c9d4da6248aedbc79f02ecedcd59f8f5f2` のcmd/benchstat/main.goに固定した。GitHub HTMLや一部native webのraw取得は失敗したため、固定raw URLをread-only HTTPで取得して確認した。両repositoryの同commit LICENSEと対象headerはBSD-3-Clause。Goサイトの文書・記事は[CC-BY-4.0](https://go.dev/copyright)。本文は帰属付きの独自要約で、OSSコードの転載・moduleへの昇格はしていない。

本稿は汎用CI gateの実装、最適sample数、全compiler最適化の網羅を提供しない。追加した検索evalはこの文書へ到達することを検証するもので、受け入れケースの実行や性能同等性を証明するものではない。release_notesのTTL30日に合わせ、2026-11-03までに適用条件を再確認する。

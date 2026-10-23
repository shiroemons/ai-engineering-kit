---
{
  "id": "testing-go-test-timeout-json-completion-boundary",
  "title": "Go test の600s timeout: package時間予算・JSON終端・cache再利用の境界",
  "kind": "knowledge",
  "technology": "testing",
  "version": "Go 1.27.1 (2026-09-01), source @ 862c888e612ac346c7c4d99c9392bdfd265f33b0; race detector rolling docs checked 2026-10-23 UTC; runtime untested",
  "tags": [
    "research-domain:quality-operations",
    "go",
    "test",
    "timeout",
    "600s",
    "package",
    "test2json",
    "completion",
    "cache",
    "count",
    "race",
    "CI"
  ],
  "sources": [
    {
      "id": "go127-test-command-contract-20261023",
      "url": "https://pkg.go.dev/cmd/go@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-test2json-event-contract-20261023",
      "url": "https://pkg.go.dev/cmd/test2json@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-testing-deadline-contract-20261023",
      "url": "https://pkg.go.dev/testing@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-test-timeout-driver-source-20261023",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/cmd/go/internal/test/test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go127-test-timeout-alarm-source-20261023",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/testing/testing.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go-race-detector-cost-contract-20261023",
      "url": "https://go.dev/doc/articles/race_detector",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2027-01-21",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# Go test が終わったと判断するための時間と証拠

## 問いと適用範囲

完了した個別testはどれも短いのに、`go test -race ./...` が600s付近でtimeoutになるのは矛盾か。個別のPASSが並び、再実行もすぐ成功したら、そのheadの全testが新しく実行されたと判断できるか。

結論は、**個別testの所要時間、package内の共通時間予算、command全体のwall timeを分け、process終了と対象packageの終端結果、cache利用を照合する**ことである。timeoutを延長したりraceを外したりしてgateを弱める提案ではない。

これはGo 1.27.1で既存契約を再確認した文書で、2026年10月に導入された機能という主張ではない。[版固定cmd/go](https://pkg.go.dev/cmd/go@go1.27.1)・[test2json](https://pkg.go.dev/cmd/test2json@go1.27.1)・[testing](https://pkg.go.dev/testing@go1.27.1)の表示公開日は2026-09-01。通常のtest実行を対象にし、benchmark計測や継続fuzzingの時間予算は対象外とする。Go 1.27.1を現在の最新版とは呼ばない。

既存の[synctest](../go/synctest.md)は仮想時計と並行処理、[benchmark比較](../performance/go-benchmark-harness-comparison-boundary.md)は計測harnessと統計を扱う。本稿の追加点はpackage timeoutとCI完了判定である。`just index`後のKB検索で `test2json` は0件、`test timeout`・`go test cache`・`test binary`の関連文書を照合した。

## 1. 三つの時間を混ぜない

[cmd/goのtest契約](https://pkg.go.dev/cmd/go@go1.27.1#hdr-Test_packages)では指定したpackageごとにtest binaryを作って実行する。`-timeout` の既定値は10m、0は無効化であり、各Test関数に新たな10分を割り当てる指定ではない。

| 観測する時間 | 判断に使えること | 同一視しないもの |
|---|---|---|
| 個別testのElapsed | そのtestの報告された所要時間 | package全体の残予算やcommand総時間 |
| package単位の実行とtimeout | 同じbinary内のtest群が消費する共通予算 | 別packageの時間との単純合計、各test固有のtimeout |
| command/CIのwall time | build、起動、実行、出力回収などを含む観測時間 | Goの内部alarmが動いていた時間だけ |

説明用に同じpackageの逐次testが多数あり、終わったtestがそれぞれ20秒でも、全体で600秒に達し得る。timeout時に表示された実行中testが2秒しか走っていなくても矛盾しない。そのtestだけが600秒を消費した、またはそのtestが根本原因だとは断定しない。

逆にpackageを並行実行しているなら、全packageのElapsedを足した値はcommand wall timeではない。親testとsubtestの時間も独立な仕事の単純和とは限らない。時間を集約する前に測定単位を固定する。

### alarmの厳密な範囲は固定sourceで確認する

[固定版testing.go](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/testing/testing.go)の `M.Run` は通常test等の実行前に `startAlarm` を呼び、そのdeadlineを `runTests` に渡す。`-count` や `-cpu` による反復でも共通deadlineを使い、testごとの開始でresetしない。timeoutの診断にはその時点で走っているtest一覧を付ける。

このalarmはprocessの起動直後から常に動く厳密な全体watchdogではない。package `init`、`TestMain` の `m.Run` より前のsetupや後のteardownまで、同じalarmが覆うとは扱わない。`M.Run` は対象実行後に `stopAlarm` する。これは1.27.1の実装観察であり、公開flag説明を全process生命周期の正確なtimer仕様へ広げないための注意である。

[T.Deadline](https://pkg.go.dev/testing@go1.27.1#T.Deadline)が返すのもbinaryの共通期限である。各test固有のSLAではない。また `T.Context` のcancelはcleanup直前という別契約なので、「残予算に合わせたoperation deadlineが自動設定される」と推測しない。

[固定版cmd/go driver](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/cmd/go/internal/test/test.go)はbuild依存を終えてから実行へ進む。通常testの有限timeoutには別のbackup killと出力待ち猶予も持つ。このためpanic発生とcommand終了は同時とは限らず、shellで600秒を超えたことだけでは内部timeoutの不具合と判定できない。逆に外側のCIが先に停止すればGoの診断回収が完了しないこともある、というのが運用上の帰結である。

## 2. JSONの個別PASSはpackage完了ではない

[test2jsonの公開契約](https://pkg.go.dev/cmd/test2json@go1.27.1#hdr-Output_Format)では、`Test` があるeventは個別test等、`Test` がないeventはpackage全体に対応する。複数packageのeventは交互に届くため `Package` を区別する。`Elapsed` はpass/fail対象の所要時間であり、全commandのbuild時間を表すfieldではない。

本稿の独自の収集規則は次のとおり。

1. 指定head・toolchain・flags・package集合と、実際に走らせたcommandを保存する。対象集合の取得にも同じbuild tags、GOOS/GOARCH等を使う。
2. 個別 `Test` のpassと、`Test` が空のpackage終端pass/fail/skipを分ける。`start`、`run`、`output`が来ただけで完了にしない。
3. `go test` 自身のexit status、stderr、JSONの完全性を保持する。logを整形した後段processのexit statusだけで代用しない。
4. 対象packageの終端が欠ける、JSONが途中で切れる、外側のcancel理由が不明なら「証拠不足」とする。「fail eventが見当たらない」を成功へ変換しない。
5. skipや `[no test files]` はtestが実行された証拠ではない。`-run` 等のfilterで必要testが除かれた結果も、全suiteの合格に拡張しない。

たとえばAとBが対象で、Aのpackage passとBの個別test passだけ残っている場合、Bの完了はまだ確認できない。timeout/panicで各testの通常終端が欠けても、driverが生きて出力回収できればpackage failを報告する経路がある。ただしrunnerやdriver自体をkillした後まで、完全なJSONを生成できる保証ではない。[固定driverの異常終了処理](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/cmd/go/internal/test/test.go)

### build eventも同じstdoutに来る

Go 1.27.1の `go test -json` はbuildのJSONも出す。[Build -json encoding](https://pkg.go.dev/cmd/go@go1.27.1#hdr-Build__json_encoding)の `build-output` / `build-fail` はTestEventとは別で、識別子は `ImportPath`。これは `TestEvent.FailedBuild` と対応し、`TestEvent.Package` と同じとは限らない。package fieldがない行を捨てたり、すべてをtest失敗へ分類したりしない。早期の重大errorではstderrに非JSONも出るため、stdoutだけを成功判定の証拠にしない。

## 3. cache再利用は新しいtimeout試験ではない

[cmd/goのcache契約](https://pkg.go.dev/cmd/go@go1.27.1#hdr-Test_packages)では、明示packageを渡す `go test ./...` や `go test .` はpackage list modeで、成功結果を再利用し得る。引数なしの `go test` は別modeである。cache hitはsummaryに `(cached)` と表示される。

特に `-timeout` はcache利用可能なflagで、cache hitは実行時間ゼロとして扱われる。**timeout値を変えて成功しても、binaryを実行し直して新しい制限に収まった証拠にはならない。** 結果の再実行を明示する標準的な方法が `-count=1` である。これはtest-result cacheの不使用であって、build cacheを全部消す操作ではない。

「今回のrunnerで本当に走った所要時間」を調べる別試行なら、許可された環境で `go test -race -count=1 -json ./...` のようにresult cacheを除いて記録する。これは本稿で実行したcommandではなく診断案であり、失敗した正式runを上書きする手段ではない。Go版、head、package集合、timeout、race/build flags、filterが異なる試行は別結果として残す。`-count=1` は外部service、OS cache、test内の状態まで初期化する保証でもない。

JSONの時刻はcache結果では慣例上省略されることがあるが、fieldの有無だけに依存してfreshな実行を認定しない。実際のcommandとcache表示を併記する。

## 4. race実行の600s接近をどう扱うか（独自の運用提案）

[公式race detector記事](https://go.dev/doc/articles/race_detector)は、負荷がprogram依存で、典型的には実行時間2–20倍、メモリ5–10倍になり得ると説明する。これは特定suiteの予測係数でも上限でもない。race検出は実際に実行した経路に限られ、`-race` を指定しただけで全経路のrace不在を証明しない。

仮にあるpackageの非cache実行が590sでpassしたなら、600s予算に対して十分な余裕があるとはいえない。ただし590sは説明用の仮定で、このrepositoryの計測値ではない。次の順で調査する。

- まずbuild待ち、package全体の時間、個別のslow test、外側のjob timeoutを分ける。単一testの最長時間だけを監視しない。
- 同じheadとflagsで、runner資源、CPU quota、並行数、race有無、cold/warm build条件を記録する。非raceの時間へ固定倍率を掛けてrace budgetを決めない。
- timeoutのstackや進捗から、待機が必要な処理、終わらない待機、資源競合、setup/cleanupのどれを疑うかを絞る。何度か再実行してpassしたことを原因解明や安定性の証明にしない。
- budgetやsuite構成を変える必要があるなら、原因・計測・必要な診断回収時間を伴う別の変更としてreviewする。本稿はtimeout延長、`-timeout=0`、race削除、test除外を自動適用しない。

正式判定は「対象headの要求されたsuiteが、要求された設定で、終端まで完了した」ことに置く。限定filterの診断成功やcache再利用の成功は、そのscopeを明示して報告する。

## 未確認事項・出典の扱い

2026-10-23 UTCに上記一次資料を確認した。Goのrelease tagからfull commitを照合し、固定sourceとそのLICENSE/headerを確認した。Go sourceはBSD-3-Clause、race detector記事本文は[site copyright](https://go.dev/copyright)に従うCC-BY-4.0である。本文は原著要約と独自の判断手順で、upstream codeを転載・module化していない。

今回はtimeout/panic/cache/raceのruntime再現試験を実施していない。特定CIの600s到達原因、runner性能、適切な新timeout、Windows等の終了signal差も未検証である。検索evalは本稿を検索できることの検証であり、Goのruntime契約の再現試験ではない。repositoryのcode・workflow・test gateを変更する根拠や実施済み記録には使わない。

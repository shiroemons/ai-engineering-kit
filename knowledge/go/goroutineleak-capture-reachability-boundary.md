---
{
  "id": "go-goroutineleak-capture-reachability-boundary",
  "title": "Go 1.27 goroutineleak: 到達可能性・capture・全stackと差分の判定境界",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 (862c888e612ac346c7c4d99c9392bdfd265f33b0; 2026-09-01); GAは1.27.0 (2026-08-19)、実験導入は1.26; verified 2026-10-04 UTC",
  "tags": [
    "research-domain:backend",
    "go",
    "goroutineleak",
    "pprof",
    "GC",
    "reachability",
    "goroutine",
    "debug",
    "delta",
    "Count",
    "GOEXPERIMENT"
  ],
  "sources": [
    {
      "id": "go127-goroutineleak-release-20261004",
      "url": "https://go.dev/doc/go1.27",
      "type": "release_notes"
    },
    {
      "id": "go126-goroutineleak-experiment-20261004",
      "url": "https://go.dev/doc/go1.26",
      "type": "release_notes"
    },
    {
      "id": "go127-goroutineleak-pprof-api-20261004",
      "url": "https://pkg.go.dev/runtime/pprof@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-goroutineleak-http-api-20261004",
      "url": "https://pkg.go.dev/net/http/pprof@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-goroutineleak-writer-source-20261004",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/runtime/pprof/pprof.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go127-goroutineleak-count-source-20261004",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/runtime/proc.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go127-goroutineleak-gc-source-20261004",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/runtime/mgc.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go127-goroutineleak-http-source-20261004",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/net/http/pprof/pprof.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go127-goroutineleak-tests-source-20261004",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/runtime/pprof/pprof_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go127-goroutineleak-release-history-20261004",
      "url": "https://go.dev/doc/devel/release",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Go goroutineleak の検出範囲と読み方

## 問い・対象・結論

Go 1.27へ更新したserviceで、`goroutineleak` が0ならgoroutineの終了処理は正しいと判定できるか。`debug=2` に出たstackをすべてleakと数えてよいか。答えを決めるには、検出可能な待機、最後に検出した時点、snapshotとdelta、出力形式を分ける必要がある。

[Go 1.27 release notes](https://go.dev/doc/go1.27#runtime) はgoroutine leak profileを一般提供にし、1.26の `GOEXPERIMENT=goroutineleakprofile` を削除した。[release history](https://go.dev/doc/devel/release#go1.27.0) による公開日は1.27.0が2026-08-19、ここで実装を確認した1.27.1が2026-09-01。本稿はこの移行に伴う診断手順の変更を扱い、今日公開された機能や将来の最新版とは扱わない。

結論は、検出済みstackを調査の強い手掛かりにしつつ、0件を全終了の証明にしないこと。特に1.27.1では、`Count()` の0、`debug=2` のstack数、deltaの0はそれぞれ違う意味を持つ。既存の [WaitGroup / errgroup](waitgroup-errgroup-join-cancel-admission.md)、[context](context.md)、[synctest](synctest.md) の終了・取消・test契約を置き換える機能ではない。

## 1. 検出するのは長時間待機ではなく、起こせる経路がない待機（公式契約）

[1.27の説明](https://go.dev/doc/go1.27#runtime) と [1.26の導入説明](https://go.dev/doc/go1.26#runtime) では、goroutineがchannel、Mutex、Condなどの並行処理primitiveにblockedで、そのprimitiveがrunnableなgoroutine、またはそこから起こせるgoroutineから到達できなければ、待機を解除できないと判定する。GCの到達可能性を使う診断であり、「30秒待ったからleak」という経過時間の閾値判定ではない。

たとえば、結果受信用のunbuffered channelを持つ親が最初のerrorでreturnし、残りのworkerがそのchannelへsendしようとする場合、受信者への経路がなくなる。これは1.26の公式例が示す検出対象である。一方、global変数やrunnableなgoroutineのlocal変数からprimitiveに到達できると、実際には誰も解除しない設計であっても見逃す可能性がある。GCが到達可能と判断することと、業務logicに有効な受信・unlock・cancel経路があることは同じではない。

ここからの独自の運用判断は、診断結果を次のように扱うことである。

- 検出stackがある: その待機先と、所有者が先に退出したerror/timeout経路を優先して調べる
- 検出stackがない: 診断対象の制約とcapture時点を記録する。service全体のlivenessやshutdown完了を保証した扱いにしない
- goroutine総数が増え続ける: 通常のgoroutine profile、処理中件数、queue滞留、終了通知も併用する。新profileの0だけで調査を打ち切らない

GCで検出するという説明を、leaked goroutineを自動停止・回収する機能だと読み替えない。返却・close・cancel・joinの所有責任はアプリ側に残る。

## 2. Countとcaptureは別の操作（固定実装の観察）

[pprof公開API](https://pkg.go.dev/runtime/pprof@go1.27.1#Profile) では、`Lookup(name)` は見つからなければnil、`Count()` はprofile内のstack数、`WriteTo(w, debug)` はsnapshotを書き出す操作である。しかし、goroutineleakの新しい検出を開始する場所はCountではない。

[pprof.go](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/runtime/pprof/pprof.go) の `goroutineLeakProfile` はCountをruntimeへ委譲し、[proc.go](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/runtime/proc.go) の `goroutineleakcount` は最後に報告された `work.goroutineLeak.count` を返す。Countを繰り返すだけでは検出を更新しない。

対して `writeGoroutineLeak` は、排他lockを取り、[mgc.goのgoroutineLeakGC](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/runtime/mgc.go) を実行した後にprofileを出力する。検出用pending flagを設定し、検出を行うGC cycleが処理されるまでGCを呼ぶ。通常の `runtime.GC()` を呼ぶことと、この検出要求を出すことを同一視しない。

[HTTP index実装](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/net/http/pprof/pprof.go) も各profileのCountを一覧へ載せる。従って `/debug/pprof/` の一覧に表示された0だけで、新しく検出を実施済みだと判断しない。調査対象のsnapshotを取得し、その時刻・binary・取得成功を記録する。

これは1.27.1の固定sourceに基づく説明であり、Countの更新方式を将来版に固定する保証ではない。監視でCountを使うなら、値に加えて「いつ検出captureを成功させたか」を管理する、というのが本稿の設計案である。

## 3. debug=2はleakだけを出す形式ではない

[WriteToのAPI](https://pkg.go.dev/runtime/pprof@go1.27.1#Profile.WriteTo) はdebug=0をgzip圧縮protobuf、debug=1を人が読めるlegacy textと説明する。goroutineleakについては、固定版の `writeGoroutineLeak` の分岐をさらに読む必要がある。

| 指定 | go1.27.1の取得内容 | 読み方 |
|---|---|---|
| `WriteTo(w, 0)` / HTTPのdebug省略 | 検出されたgoroutineのprofile | `go tool pprof` などでstack単位の件数を分析する |
| `WriteTo(w, 1)` / `debug=1` | 検出されたgoroutineのtext profile | totalとstackを調査する |
| `WriteTo(w, 2)` / `debug=2` | 検出GCを実行した後、非leakも含む全goroutineのstack dump | すべてのstackをleakとして集計しない |

debugが2以上になるとfiltered writerを通らず `writeGoroutineStacks` を呼ぶ。名称がgoroutineleakであることや、HTTP indexの説明文だけから出力対象を推測しない。今回の限定実測ではleak側に `(leaked)` の状態表示があり、正常に動くmainや後で解除できるglobal channel待機も同じdumpに含まれた。この表示の全文parseを長期安定APIとは扱わない。

通常の `goroutine` profileは全goroutine、`block` は同期primitiveにblockedだった時間、`mutex` は競合するlockの保持側を調べるための情報である。詳しい契約は [pprof API](https://pkg.go.dev/runtime/pprof@go1.27.1#Profile) を参照する。種類の違う件数や時間をそのまま一つのleak率に足さない、というのが独自の診断方針である。

## 4. deltaが0でも既存leakは残る（固定版限定）

取得した [HTTP APIのParameters](https://pkg.go.dev/net/http/pprof@go1.27.1#hdr-Parameters) はdelta対象の列挙にgoroutineleakを含めていない。一方、固定した [HTTP source](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/net/http/pprof/pprof.go) の `profileSupportsDelta` はgoroutineleakを許可し、今回の同版runtimeでも `seconds=1` は成功した。この差は実装観察として記録し、列挙されていない全profileや旧版にもdeltaを保証する根拠にしない。

同sourceのdelta処理は、binary profileを取得し、指定秒数を待ち、もう一度取得する。その後、最初のprofileを `Scale(-1)` し、二つをMergeする。期間中に起きた全leakを逐次記録するevent logではない。前後とも同じstack・同じ件数の既存leakがあれば、差分は0になりうる。

- 絶対snapshotの1件と、その後のdeltaの0件は矛盾しない。deltaの0を「現在のleakなし」と読むと、既存障害を見落とす
- 二度の取得はそれぞれ検出GCを伴う。`seconds` は間の待機時間であって、要求全体の厳密なwall-clock上限ではない
- `seconds` は正の整数。固定実装では `seconds=1&debug=1` はHTTP 400となり、textとdeltaを同時指定できない
- delta失敗を空profileへ変換しない。HTTP status、Content-Type、parse結果と対象binaryを保持する

後二項を監視・collectorへ組み込むのは独自の運用案である。deltaで変化を追う場合も、基準となる絶対snapshotを残す。

## 5. captureの負荷と公開範囲を先に決める（独自の運用案）

固定writerは、検出GCから出力完了までgoroutine leak用lockを保持する。並行要求が検出状態のresetと競合し、不完全なprofileを出すことを避けるための処理である。このlockの存在は、取得が無料・常時一定時間で終わるという意味ではない。負荷や応答時間は今回測定していない。

1. 実行binaryのGo版とbuild設定を確認する。1.26では実験flagが必要だったが、1.27では削除済みなのでbuild環境に古いflagを残さない。異なるtoolchainを扱う収集側ではLookupのnilも失敗として記録する
2. 診断用の許可された経路だけから取得する。[net/http/pprof](https://pkg.go.dev/net/http/pprof@go1.27.1) はimport時にDefaultServeMuxへhandlerを登録し、独自muxなら明示登録を要する。packageのimportだけでHTTP serverが起動したり、既存独自muxへ自動接続されたりはしない
3. HTTPはGETで取得する。既存の管理経路の認証・到達制限を使い、stackやcommand lineなどの診断情報を無条件で外部公開しない。稼働serviceの設定変更は別途承認して行う
4. 高頻度scrapeや大量の並行captureを既定にしない。対象heap・goroutine数で、取得時間、GC負荷、通常request latency、出力サイズを測り、間隔と同時実行数を決める
5. 最初はdebug=0または1の絶対snapshotを取り、必要時にdebug=2で周囲のgoroutineも見る。stack総数やtextのheaderだけを共通parserで混用しない
6. 捕捉したstackから、親の早期return、send/receive、lock所有者、cancel後の退出通知を追う。leak profileを取れたこと自体を修復完了にしない
7. 修正後は対象の失敗経路を再現し、終了通知やjoin、総goroutine数の落ち着きも検査する。不要なglobal参照を足して検出されにくくするなど、検出結果を隠す変更を修復と扱わない

## 6. 限定実測と上流testは分けて読む

2026-10-04 UTC、repositoryが固定するGo 1.27.1のLinux/amd64環境で、独自の小さなprobeを別processとして実行した。空のGOEXPERIMENTで、局所channelに永久待機するgoroutineを1本、global channelで待機し後にcloseできるgoroutineを1本作り、通常goroutine dumpで両関数の存在を確認してから取得した。

| 操作 | この一回の実測 |
|---|---|
| 初回Count | 0 |
| 通常runtime.GC後のCount | 0 |
| goroutineleak debug=1取得後 | Countは1、局所channelのstackを含みglobal待機を含まない |
| debug=2取得 | 局所待機、global待機、mainのstackを含む |
| debug=0をpprofで解析 | goroutineleakの合計1 |
| HTTP handlerへseconds=1 | status 200、binary profile、pprof解析で合計差分0 |
| seconds=1とdebug=1を併記 | status 400、非互換のerror |
| 旧GOEXPERIMENTでgo version | exit 2、`unknown GOEXPERIMENT goroutineleakprofile` |

global待機は最後にcloseして解除したため、このprobeはglobal変数を使った本物のleakを見逃すことの再現試験ではない。公式の検出限界を、検出対象／非対象の観察と分けて扱う。HTTP試験はhttptestのrequest/recorderでhandlerを直接呼び、listen・本番endpoint・ネットワーク遅延を試していない。永久待機側はprobe processの終了で終わる。

固定した [上流pprof_test.go](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/runtime/pprof/pprof_test.go) の `TestGoroutineLeakProfileConcurrency` は、検出されるまで再取得して待ち、連続取得、並行取得、通常goroutine profileとの併用を確認する。ただし並行caseの一部は `totalLeaked-1` を下限とし、稀に1件少ない理由を調査するコメントとissue #79452を残している。これを全件・一回・即時検出の保証に読み替えない。上流testの全文実行やissueの現在の解決状態は今回確認していない。

未検証なのは、macOS/Windows、別CPU、全種類のprimitive、race build、本番規模のheap、capture競合、遅いwriter・通信切断、GC負荷・pause時間、全旧patch版との比較である。検索evalは文書の発見性のみを検査し、これらのruntime保証を追加しない。

## 出典・日付・ライセンス

- 取得日は全て2026-10-04 UTC。native Webで1.27/1.26 release notes、固定API2件、release history、Go copyrightを開いた。公開日を取得日で置き換えていない
- repository分析は全てGo 1.27.1の40桁commit `862c888e612ac346c7c4d99c9392bdfd265f33b0` に固定し、GitHub connectorでtag解決と各fileを取得した。分析したsource5件とLICENSEは実行toolchain内の同名fileとも一致した
- [Go copyright](https://go.dev/copyright) はwebsite文書のCC-BY-4.0とcodeのBSD licenseを示す。pkg.go.devの表示はBSD-3-Clause。固定commitの [LICENSE](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/LICENSE) とsource冒頭も確認した
- 本文は独自の要約・実装分析・運用案で、上流のcode sampleを転載していない。probeも独自に作成し、repositoryのmodule・application codeには追加しない

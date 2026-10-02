---
{
  "id": "go-waitgroup-errgroup-join-cancel-admission",
  "title": "Go WaitGroup.Go と errgroup: join・取消・投入制御の境界",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 / golang.org/x/sync v0.23.0（f75267d8412fc1dfd12b343644a7ea46e4d9c85d）; WaitGroup.Go は Go 1.25 追加; 2026-10-02 UTC 確認",
  "tags": [
    "research-domain:backend",
    "waitgroup",
    "errgroup",
    "goroutine",
    "join",
    "cancellation",
    "admission-control",
    "deadlock",
    "structured-concurrency"
  ],
  "sources": [
    {
      "id": "go127-waitgroup-task-lifecycle-20261002",
      "url": "https://pkg.go.dev/sync@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go-errgroup-v023-task-lifecycle-api-20261002",
      "url": "https://pkg.go.dev/golang.org/x/sync@v0.23.0/errgroup",
      "type": "official_docs"
    },
    {
      "id": "go127-context-join-boundary-20261002",
      "url": "https://pkg.go.dev/context@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go-xsync-v023-admission-source-20261002",
      "url": "https://go.googlesource.com/sync/+/f75267d8412fc1dfd12b343644a7ea46e4d9c85d/errgroup/errgroup.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Go WaitGroup.Go と errgroup: join・取消・投入制御の境界

## 問いと適用範囲

並行処理の子タスクが失敗したとき、何が取り消され、いつ親へ戻り、投入待ちは解除されるのか。`WaitGroup.Go` への書き換えや `errgroup.WithContext` の採用だけで、この三つが同時に解決するとは考えない。

本稿では join（登録した仕事の終了待ち）、取消通知、admission（新しい仕事の受け入れ）を分けて設計する。既存の [context 基本 API](context.md) と [synctest](synctest.md) に対し、グループの寿命と投入側の停止条件を補う。Python executor のキューや `asyncio.TaskGroup` の例外集約契約は対象外。

- 標準ライブラリは [sync の go1.27.1 固定 API](https://pkg.go.dev/sync@go1.27.1) を確認した。表示公開日は 2026-09-01。`WaitGroup.Go` の追加は **Go 1.25.0** であり、1.27 の新機能という意味ではない
- `errgroup` は [golang.org/x/sync v0.23.0 の固定 API](https://pkg.go.dev/golang.org/x/sync@v0.23.0/errgroup) を確認した。表示公開日は 2026-08-31。標準ライブラリとは別 module で、[この版の go.mod](https://go.googlesource.com/sync/+/refs/tags/v0.23.0/go.mod) は Go 1.26.0 を要求する
- 取得日はすべて 2026-10-02 UTC。これらが将来も最新、または旧版と同一挙動とは主張しない。公式 API、固定 source からの観察、独自の設計判断を以下で区別する

## 公式 API で確認した責務

| 選択肢 | join と error | 取消・上限 |
|---|---|---|
| `sync.WaitGroup.Go(func())` | 登録した関数の終了を `Wait()` で待つ。error の返却口はない | context や件数上限の API はない |
| zero value の `errgroup.Group` | `func() error` を登録し、全終了後に最初の non-nil error を返す | 失敗による context 取消はなく、既定は上限なし |
| `errgroup.WithContext(parent)` | 同じ join / error 契約 | 派生 context を作り、最初のタスク error または最初の `Wait` 終了で取り消す |

根拠: [WaitGroup](https://pkg.go.dev/sync@go1.27.1#WaitGroup)、[Group / WithContext / Wait](https://pkg.go.dev/golang.org/x/sync@v0.23.0/errgroup#Group)。`CancelFunc` 自体も仕事が止まるまで待つ関数ではない。[CancelFunc](https://pkg.go.dev/context@go1.27.1#CancelFunc)

### WaitGroup.Go の登録順序を保つ

[WaitGroup.Go / Add / Done](https://pkg.go.dev/sync@go1.27.1#WaitGroup.Go) の契約:

- グループが空なら `Go` の呼び出しは `Wait` より前に行う。既に空でなければ、そのタスクからさらに `Go` を呼べる
- 別の仕事集合へ再利用するなら、前のすべての `Wait` が戻ってから新しく登録する。最初の使用後に WaitGroup をコピーしない
- 渡した `f` は panic してはならない。`f` の復帰は、それが解除した `Wait` の復帰より前に同期される

独自の移行レビュー: `go wg.Go(f)` と外側に `go` を付けると、最初の登録自体が遅れ、空の `Wait` が先に戻り得る。`wg.Go` 内で同じ仕事のために `defer wg.Done()` を追加すると、計数を二重に減らす。通常の呼び出し側で `wg.Go(f)` とする。Add/Done 方式を残す場合も、空からの正の Add は待機開始前に済ませる。

join があることは、実行中の複数タスクが同じ map や変数へ安全に書けることを意味しない。結果の書き込み先を分離するか、別の同期を設計する。子がさらに通常の `go` 文で切り離した処理まで、自動で登録されるとは扱わない。

### errgroup の失敗通知と完了を混同しない

[WithContext / Wait](https://pkg.go.dev/golang.org/x/sync@v0.23.0/errgroup#WithContext) によれば、最初の non-nil error で派生 context は取り消されるが、`Wait` は登録した関数がすべて戻るまで待つ。全 error をまとめた一覧を返す API ではない。

ここからの設計判断:

1. 早い失敗通知が必要でも、join の待ち時間は別に見積もる。取消通知を無視してブロックする子があれば、親の `Wait` も終わらない
2. `Wait` が nil でも派生 `groupCtx` は取消済みになる。成功後の保存・次段処理にその context を渡すと、既に期限内でも即座に中止され得る
3. タスク error の集約結果は `g.Wait()` で判定する。成功後の `groupCtx.Err()` や `context.Cause(groupCtx)` だけでジョブ失敗を判定しない。親の取消を操作全体の失敗として返すか、既に得た成功結果を返すかは、別途その操作の契約として決める
4. 次段で同じリクエストの寿命を使うなら、元の `parent` を保持してそこから必要な期限を派生する。元の親も取消済みなら、次段を開始してよいとは限らない

## 固定 source で確認した境界

[公式 v0.23.0 tag](https://go.googlesource.com/sync/+/refs/tags/v0.23.0) は commit `f75267d8412fc1dfd12b343644a7ea46e4d9c85d`（2026-08-31T03:04:41Z）を指す。`go mod download -json golang.org/x/sync@v0.23.0` の `Origin.Hash` も一致した。[固定 errgroup.go](https://go.googlesource.com/sync/+/f75267d8412fc1dfd12b343644a7ea46e4d9c85d/errgroup/errgroup.go) の web 取得は失敗したため、取得した module archive の同ファイル、`errgroup_test.go`、`go.mod`、`LICENSE` を読んだ。

以下はその版の実装観察であり、将来の API 保証へ拡張しない。

| 観察した箇所 | 実装上の境界 | 利用側で誤解しないこと |
|---|---|---|
| `Go` | slot を得る channel send が `wg.Add` と goroutine 起動より前。context を待つ select はない | 投入待ちの呼び出しは、context 取消だけでは解除されない |
| `TryGo` | 空きがなければ false。空きがあれば起動し、context の取消状態は検査しない | false は未登録であり、保留キューへの登録ではない。取消済みでも起動し得る |
| `Wait` | 内部の WaitGroup を待った後、保存した `g.err` で cancel し、その `g.err` を返す | 親 context の error を自動的に `Wait` の戻り値へ変換するわけではない |
| error 保存 | 各 callback の non-nil error を `sync.Once` で一つ保存する | 入力順に最初の error が選ばれるという契約ではない |
| `Go` / `TryGo` の callback 実行 | panic を error へ変換する recover はない | panic を `Wait` の通常 error として回収できると仮定しない |

`WithContext` はこの版で `context.WithCancelCause` を使う。親が先に取り消されれば、その原因と後からタスクが返す error は異なり得る。`Cause` は最初の取消原因、`Wait` はグループが保存したタスク error を扱う。全 callback が nil を返せば、親が取消済みでも `Wait` が nil となり得る。一方、正常終了時の cancel(nil) は cause を `context.Canceled` にする。[Cause / WithCancelCause](https://pkg.go.dev/context@go1.27.1#WithCancelCause)

## SetLimit は取消可能な投入キューではない

[SetLimit / TryGo](https://pkg.go.dev/golang.org/x/sync@v0.23.0/errgroup#Group.SetLimit) の公開契約では、上限は active goroutine 数。負値は無制限、ゼロは新規起動を許さない。`Go` は上限内に追加できるまで呼び出し側をブロックする。active な仕事がある間に limit を変えてはならない。

### 取消後に新しい callback が始まる経路

固定 source からの時系列の推論:

1. limit が 1 で、タスク A が slot を保持する
2. producer が B を `g.Go` に渡し、slot 待ちになる
3. 親を cancel しても、A が戻らなければ producer は待ったまま
4. A が戻って slot が空くと、B の callback は取消済みの context を持って開始し得る

したがって、producer が呼び出し直前に `groupCtx.Err()` を一度見るだけでは、待機中の取消を admission に反映できない。callback の入口でも検査し、下流 I/O に同じ context を渡す。ただし検査直後の取消と外部副作用は競合し得るため、「取消後の副作用ゼロ」をこのチェックだけで保証しない。

### 再帰投入によるデッドロック

limit=1 のグループで、唯一のタスク A が同じ `g.Go(B)` を同期的に呼んでから戻るとする。B の slot を空けるには A の復帰が必要だが、A は B の投入待ちで復帰できない。これは上限契約と実装から導く循環待ちであり、通常の入れ子 `WaitGroup.Go` と同じ感覚では使えない。

`SetLimit(0)` を一時停止スイッチとみなし、active な仕事がある間に設定変更して解決しようとしない。入れ子処理を同期実行へ直す、投入の所有者を外側に移す、依存関係に沿って段階を分けるなど、待ち関係を先に直す。別グループを作るだけでは全体の資源上限が保たれるとは限らない。

## 実務での選択とレビュー項目（独自の設計案）

- **終了待ちだけ必要**: `WaitGroup.Go` を候補にする。業務 error を扱うなら集約方法と返却契約を別に用意する
- **有限の独立タスクを一つの結果へまとめる**: `errgroup.WithContext` を候補にし、全 callback が停止要求に応答する経路と、失敗後も join する所有者を決める
- **過負荷を即時拒否できる**: `TryGo` の false を明示的な拒否・延期として扱う。失敗を無視して入力を処理済みにしない
- **投入待ちにも取消が必要**: 固定数の worker と有界 channel などを検討し、producer が channel 送信と `ctx.Done()` を待てる設計にする。選択が競合する場合の最終的な実行可否は worker 側でも判断する
- **大量入力**: `SetLimit` は保持済みの入力全体、結果配列、あるいは外側に起動した producer goroutine の数を制限しない。入力保持量・結果保持量・実行並列数を別々に決める

単一 producer から登録を終えて `Wait` する形を基本にすると寿命を把握しやすい。`go g.Go(...)` を大量に起動して投入のブロックを回避すると、外側の goroutine が待機列になり、最初の登録と `Wait` の順序も崩しやすい。外部 producer を許すなら、追加登録が終わったことを別途同期してから完了を判定する。

監視では「取消要求時刻」「最後の子の終了時刻」「投入待ち時間」「拒否件数」を分ける。処理時間の短縮だけを見て、停止していない子や投入待ちを見落とさない。部分成功がある業務では、グループ error 一つとは別に各入力の成否と再実行可能性を記録する。

## 確認した観察と検証の限界

2026-10-02 に Linux amd64、Go 1.27.1、x/sync v0.23.0 の一時 module で、独自の小さな probe を `go test -race -count=20` により実行し、次の6ケースがすべて成功した。待機のケースは channel と `testing/synctest` で順序を固定し、wall-clock sleep に依存していない。

1. 正常な `Wait` は nil を返し、派生 context は取消済み、元の parent は未取消
2. 取消済み parent から作ったグループでも、callback が nil を返せば `Wait` は nil
3. 一つのタスクが error を返して派生 context が取り消されても、別の callback を解除するまで `Wait` は戻らない
4. 上限でブロックした `Go` は取消では戻らず、先行 callback の解除後に後続 callback を開始。その後続は取消済み context を観測
5. 満杯で false だった `TryGo` の callback は実行されず、後の空き slot では取消済みでも別 callback を開始できる
6. active な `WaitGroup.Go` 内から追加した子は、外側の `Wait` の終了前に完了

probe は公式テストから転記せず、リポジトリの module・依存・テストコードには追加していない。この観察は全スケジュールや本番 I/O の証明ではない。再帰投入のデッドロックは source に基づく分析で、実行してプロセスを停止させる試験はしていない。panic、終了不能なシステムコール、macOS/Windows の実行差、実アプリの性能・外部副作用は未検証。

検索 eval は本稿を取得するための確認であり、並行処理の動作を試験しない。導入先では callback の取消無視、入力の途中停止、同時 error、後続段の context、投入上限到達を別々のテストへ落とす。`errgroup` を構造化並行性の部品として使っても、登録されていない goroutine や実行済みの外部副作用まで巻き戻るとは扱わない。

## 出典・provenance

- [sync go1.27.1](https://pkg.go.dev/sync@go1.27.1)、[context go1.27.1](https://pkg.go.dev/context@go1.27.1): 固定版の公式 API。BSD-3-Clause 表示と [Go LICENSE](https://go.dev/LICENSE) を確認
- [errgroup v0.23.0 API](https://pkg.go.dev/golang.org/x/sync@v0.23.0/errgroup): 公開契約。BSD-3-Clause 表示を確認
- [v0.23.0 tag / commit](https://go.googlesource.com/sync/+/refs/tags/v0.23.0)、[同 tag の LICENSE](https://go.googlesource.com/sync/+/refs/tags/v0.23.0/LICENSE): 固定 source と取得経路は上記のとおり。module checksum は source catalog に記録

本文は原文の長い引用や OSS コードを持ち込まない独自の要約・分析。公式のサンプルをそのまま本番用途へ推奨するものではなく、再利用 module への昇格も行わない。

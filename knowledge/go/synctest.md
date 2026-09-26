---
{
  "id": "go-synctest",
  "title": "Go testing/synctest の bubble 仮想時計で決定的に並行コードをテストする",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 (testing/synctest と net/http/httptest API)",
  "tags": [
    "testing",
    "synctest",
    "bubble",
    "virtual-clock",
    "fake-clock",
    "clock",
    "deterministic",
    "concurrency",
    "goroutine",
    "wait",
    "sleep",
    "settle",
    "timeout",
    "deadline",
    "context",
    "httptest",
    "test-server",
    "in-memory-network",
    "fake-network",
    "waitgroup",
    "cond",
    "timer"
  ],
  "sources": [
    {
      "id": "go-synctest-docs",
      "url": "https://pkg.go.dev/testing/synctest",
      "type": "official_docs"
    },
    {
      "id": "go-httptest-docs",
      "url": "https://pkg.go.dev/net/http/httptest",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-25",
  "expires_at": "2026-12-24",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/search.json"
  ]
}
---

# Go testing/synctest の bubble 仮想時計で決定的に並行コードをテストする

`testing/synctest` は並行コードを決定的（deterministic）にテストする標準ライブラリパッケージで、`Test` 関数がテスト関数を隔離された "bubble" の中で実行する。[公式 API](https://pkg.go.dev/testing/synctest) / [公式 httptest API](https://pkg.go.dev/net/http/httptest)

## 要点

### bubble と Test の契約（公式 Overview / `Test`）

- `Test(t, f)`（added in go1.25.0）は `f` を新しい bubble で実行し、bubble 内の全 goroutine が退出するまで待ってから返る。bubble 内の goroutine が deadlock になるとテストは失敗する（Blocking 節では deadlock 時に `Test` が panic すると記載）。`Test` は bubble の内側から呼んではならない。
- bubble 内で開始された goroutine はすべてその bubble の一部になる。公式は各テストを完全に自己完結させるため、(1) bubble 外の goroutine とやり取りしない、(2) ネットワークを使わない（必要なら fake network 実装を使う）、(3) 外部プロセスとやり取りしない、(4) バックグラウンドタスクに goroutine を漏らさない、を「ほとんどのテストに適用すべきガイドライン」として挙げる。
- `f` に渡される `*testing.T` の契約: `T.Cleanup` は bubble の内側で `Test` の直前に実行される。`T.Context` は bubble に関連付けられた Done channel を持つ `context.Context` を返す。`T.Run`・`T.Parallel`・`T.Deadline` は呼んではならない。

### 時間の契約（公式 Overview "Time"）

- 各 bubble は独立した fake clock を使い、初期時刻は UTC 2000-01-01 00:00:00（ midnight UTC 2000-01-01 ）。
- 時間が進むのは、bubble 内のすべての goroutine が durably blocked になったときだけ。この仮想時計（virtual clock）のおかげで、公式の例では 2 秒の sleep を含むテストが実時間 2 秒を待たずに終わる。
- bubble の root goroutine が退出すると、時間の進むのは止まる。

### durably blocked の定義（公式 Overview "Blocking"）

- 「ブロックしており、同じ bubble 内の別の goroutine によってしか起こせない」状態。外部イベントで起こしうる goroutine は durably blocked ではない。
- durably blocking な操作の一覧: bubble 内で作られた channel へのブロッキングな send / receive、すべての case が bubble 内 channel であるブロッキング select、`sync.Cond.Wait`、`sync.WaitGroup.Wait`（`Add` が bubble 内で呼ばれた場合）、`time.Sleep`。
- 一覧外で durably blocking ではないもの: `sync.Mutex` / `sync.RWMutex` のロック、network socket の読み取りなどの I/O、system call。いずれも bubble 外のイベントで起こせるため。
- すべての goroutine が durably blocked になったときの挙動は順に: (1) `Wait` が呼ばれていたら `Wait` が戻る、(2) それがなければ次に少なくとも1 goroutine を起こす時刻があればその時刻へ時間を進める（root goroutine が未退出である場合）、(3) それがなければ deadlock で `Test` は panic する。

### Wait と Sleep（公式 `Wait` / `Sleep`）

- `Wait()` は「自分以外の bubble 内の全 goroutine が durably blocked になるまで」ブロックする。bubble の外から呼んではならない。同じ bubble 内の複数 goroutine が同時に呼んではならない。
- `Sleep(d)`（added in go1.27.0）は `time.Sleep(d)` の後に `synctest.Wait()` を呼ぶのと完全に等価。公式の説明どおり、テスト自身と被験コードが同じ時間だけ眠るとどちらが先に走るかは不定なので、`Sleep` は被験コードを "settle" させるための手段として `time.Sleep` より好ましい。

### isolation の契約（公式 Overview "Isolation"）

- bubble 内で作られた channel・`time.Timer`・`time.Ticker` はその bubble に紐づき、bubble の外から操作すると panic する。
- `sync.WaitGroup` は最初の `Add` / `Go` の呼び出しで bubble に紐づき、その後に bubble の外から `Add` / `Go` すると fatal error になる。技術的制限として、パッケージ変数 `var wg sync.WaitGroup` では紐づかず、その操作が durably blocking でない場合がある。`var wg = new(sync.WaitGroup)` ならこの制限は適用されない。
- `Cond.Wait` でブロック中の bubble 内 goroutine を bubble の外から起こすのは fatal error。
- cleanup function と `runtime.AddCleanup` / `runtime.SetFinalizer` に登録された finalizer は、どの bubble の外側でも実行される。

### HTTP テストとの接続（公式 httptest）

- `httptest.NewTestServer(t, handler)`（added in go1.27.0）は既定で in-memory network 実装を使う `Server` を返し、公式 Overview は「in-memory network はポート枯渇や一時的なネットワーク問題を避け、`testing/synctest` と併用するのに適する」と明記する。handler が nil なら全リクエストに 500 を返し、`http.DefaultServeMux` は使わない。
- `NewTestServer` で作ったサーバは、handler が `http.ErrAbortHandler` 以外の値で panic したらテストを失敗させ、Cleanup を登録してテスト終了時にシャットダウンする。他の作り方（`NewServer` / `NewTLSServer` / `NewUnstartedServer`）で作ったサーバは `Server.Close` で手動クローズする必要がある。
- in-memory network を使うときは `Server.Start` / `Server.StartTLS` を呼ばない。`Server.Client()` が返す `http.Client` は宛先アドレス・ホスト名に関係なく HTTP / HTTPS すべてのリクエストをこのサーバに向ける。`Server.URL` のアドレスは example.com になり、`Server.Listener` は未設定になる。loopback で listen したい場合だけ `Server.Start` / `Server.StartTLS` を呼び、システム指定ポートで listen する。
- synctest 公式の 100-continue 例は「network I/O でブロックしている goroutine は synctest bubble を idle にできないから loopback 接続は使えない」と明記し、`net.Pipe()` による in-process fake 接続を `http.Transport.DialContext` に渡している。

## 推奨方法

以下は上記の公式契約と公式 example からの設計上のまとめであり、公式が推奨する数値や構成を指すものではない。

- テスト全体を `synctest.Test(t, func(t *testing.T) { ... })` で包み、内部は実時間を待たない検証にする。
- context は `t.Context()` を基点に作る（bubble に関連付けられた Done channel を持つ）。deadline の検証は公式 `Context.WithTimeout` 例の順序どおり、`context.WithTimeout(t.Context(), timeout)` の後に timeout の 1ns 手前まで `time.Sleep` + `synctest.Wait()` で進めて未取消を確認し、残り 1ns で進めて `context.DeadlineExceeded` を確認する。
- 被験コードを動かした後の同期は、任意時間の `time.Sleep` ではなく `synctest.Wait()`、時間を進めながら待つ場合は go1.27+ の `synctest.Sleep(d)` にする。
- HTTP handler の end-to-end テストは `httptest.NewTestServer` を使い、`server.Client()` が全宛先のリクエストをテストサーバに向ける前提でアサーションを書く。`http.Transport` などの低層テストには `net.Pipe()` などの in-process fake 接続を使う（公式 100-continue 例）。
- `context.AfterFunc` の発火確認は、公式例どおり「`Wait()` で未発火を確認 → `cancel()` → `Wait()` で発火済みを確認」の順に揃える。context は `t.Context()` を基点に作り、不要になったら `cancel()` で回収する。

## 避ける使い方

- `Test` のコールバック内で `T.Run` / `T.Parallel` / `T.Deadline` を呼ぶ、または bubble の内側から `Test` を呼ぶ。いずれも公式が禁止している。
- `synctest.Wait()` を bubble の外から呼ぶ、または同じ bubble 内で複数 goroutine が同時に呼ぶ。公式が禁止している。
- bubble 内で loopback や実ネットワークを使う。I/O と system call は durably blocking ではなく、公式例も「network I/O でブロックしている goroutine は bubble を idle にできない」と明記する。時間は進まず、結果として deadlock 判定に至る。
- bubble の外から bubble 内で作った channel / `time.Timer` / `time.Ticker` を操作する（panic になる）。`Cond.Wait` 中の goroutine を外から起こすのも fatal error。
- `var wg sync.WaitGroup` のパッケージ変数を bubble をまたいで使う。紐づかず durably blocking にならない可能性があるため、`var wg = new(sync.WaitGroup)` を使う。
- `sync.Mutex` のロックや I/O 待ちを「durably blocked」とみなす。公式の一覧外であり、時間は進まない。
- 実時間の `time.Sleep` 後に `time.Now()` で経過時間を検証する。順序が不定になるため、公式は settle 後の検証を基本とする（`Sleep` = `time.Sleep` + `Wait`）。
- bubble 外の goroutine ・外部プロセス・バックグラウンドへの goroutine のリークを残したままテストする。公式の自己完結ガイドラインの違反。
- loopback の `httptest.NewServer` / `NewTLSServer` を bubble 内で使い、`Server.Close` を忘れる。手動クローズが必須で、in-memory ではないため synctest とは相性が悪い。

## 適用版と本番での注意

- `Go 1.27.1 (testing/synctest と net/http/httptest API)` と pkg.go.dev の表示 `go1.27.1`（Published Sep 1, 2026）で確認する。将来の最新とは扱わない。
- 導入版: `synctest.Test` は added in go1.25.0、`synctest.Sleep` は added in go1.27.0、`httptest.NewTestServer` は added in go1.27.0（いずれも pkg.go.dev の added in 表示）。`synctest.Wait` には追加版の表示がない。古い Go では `Sleep` と `NewTestServer` は存在しない。
- `Test` に渡す `*testing.T` は特殊で、`T.Run` / `T.Parallel` / `T.Deadline` が使えない。サブテストや並列テストの枠組みと組み合わせられない前提で構成する。
- cleanup function と finalizer（`runtime.AddCleanup` / `runtime.SetFinalizer`）は bubble の外で実行される。bubble 内の時間や状態を前提にしない。
- `Test` は「bubble 内の全 goroutine の退出を待つ」ため、ゴミ goroutine を残すとテストが終わらない。漏れは deadlock / テスト失敗として表面化する。
- 本パッケージはテスト専用の道具であり、本番の実行経路には使わない（本文の整理。公式 API が示す適用範囲はテストに限られる）。

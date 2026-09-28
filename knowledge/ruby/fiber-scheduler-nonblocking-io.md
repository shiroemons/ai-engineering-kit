---
{
  "id": "ruby-fiber-scheduler-nonblocking-io",
  "title": "Ruby Fiber scheduler interface によるノンブロッキング IO: hook 群と設定・終了の契約",
  "kind": "knowledge",
  "technology": "ruby",
  "version": "Ruby 3.4 RDoc (Fiber / Fiber::Scheduler)、sample scheduler v3_4_1 (commit 48d4efcb8)、interface は Ruby 3.0.0 で導入",
  "tags": [
    "research-domain:backend",
    "ruby",
    "Fiber",
    "scheduler",
    "Fiber.schedule",
    "Fiber.set_scheduler",
    "Fiber.blocking",
    "io_wait",
    "io_read",
    "io_write",
    "io_select",
    "kernel_sleep",
    "timeout_after",
    "address_resolve",
    "block",
    "unblock",
    "blocking_operation_wait",
    "non-blocking",
    "ノンブロッキング"
  ],
  "sources": [
    {
      "id": "ruby-fiber-scheduler-3-4-docs",
      "url": "https://docs.ruby-lang.org/en/3.4/Fiber/Scheduler.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-fiber-3-4-docs",
      "url": "https://docs.ruby-lang.org/en/3.4/Fiber.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-scheduler-sample-v3-4-1",
      "url": "https://raw.githubusercontent.com/ruby/ruby/48d4efcb85000e1ebae42004e963b5d0cedddcf2/test/fiber/scheduler.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "ruby-3-0-0-release-notes",
      "url": "https://www.ruby-lang.org/en/news/2020/12/25/ruby-3-0-0-released/",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-10-28",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/backend.json"]
}
---

# Ruby Fiber scheduler interface によるノンブロッキング IO: hook 群と設定・終了の契約

Ruby の Fiber scheduler は、ブロッキング操作をフックしてイベントループへ委譲する非ブロッキング実行の仕組みである。以下は Ruby 3.4 の公式 RDoc（[Fiber::Scheduler](https://docs.ruby-lang.org/en/3.4/Fiber/Scheduler.html)、[Fiber](https://docs.ruby-lang.org/en/3.4/Fiber.html)）、ピン留め commit のテスト用サンプルスケジューラ、[Ruby 3.0.0 リリースノート](https://www.ruby-lang.org/en/news/2020/12/25/ruby-3-0-0-released/)（2020-12-25 公開）に記載・観察された事実と、そこからの独自の設計案を分けて記録する。4 資料はいずれも 2026-09-28 に取得（サンプルは v3_4_1 タグの commit `48d4efcb8` に固定して確認）した。

## 要点

### Scheduler hook 群（公式 RDoc に記載された事実）

- Fiber の非ブロッキングモデルは、スレッドに scheduler が設定されているときに成立する。scheduler はブロッキング操作を横取り（intercept）するオブジェクトで、対応する hook メソッド群を持つ。
- RDoc が列挙する hook は `io_wait` / `io_read` / `io_write` / `io_pread` / `io_pwrite` / `io_select` / `process_wait` / `kernel_sleep` / `timeout_after` / `address_resolve` / `block` / `unblock` / `blocking_operation_wait` / `fiber` である。
- スレッド終了時には scheduler の `close`（存在すれば `scheduler_close`）が呼ばれる。終了時の後始末はこの経路で行われる契約である。
- `Fiber.schedule` は scheduler の `fiber` hook へ委譲する。すなわちスケジュール実行の作り方は scheduler 側が決める。

### Fiber 側の意味論（公式 RDoc に記載された事実）

- `Fiber.new` の `blocking` 引数は既定で `false` である。
- 非ブロッキングの振る舞いは、`Fiber.set_scheduler` で scheduler が設定されている場合にのみ有効になる。scheduler がなければ Fiber は通常のブロッキング動作のままである。
- scheduler 未設定の状態で `Fiber.schedule` を呼ぶと `RuntimeError` になる。
- `Fiber.blocking` と `Fiber.current_scheduler` の意味論が定義されている（ブロッキング区間の明示と現在の scheduler の取得のための API）。

### テスト用サンプルスケジューラの観察事実（ピン留め実装の読解であり公式の推奨構成ではない）

- `ruby/ruby` の `test/fiber/scheduler.rb`（v3_4_1、commit `48d4efcb8`）はテスト目的の toy 実装で、`kernel_sleep` / `block` / `unblock` / `io_wait` / `io_select` / `process_wait` / `address_resolve` / `timeout_after` / `blocking_operation_wait` / `fiber` を実装する。
- イベントループは `run` / `close` メソッドで回り、`IO.select` と urgent pipe（`@urgent`）による wakeup を組み合わせている。`unblock` はスレッドセーフであることが期待され、ready になった fiber をキューして urgent pipe に書き込むことで select を起こす。
- ファイル先頭コメントの明言として、この実装はテスト用であり、多数のファイルディスクリプタには非効率（`IO.select` 使用）、同一 fd への重複 `wait` と重なるイベントを正しく扱えない、という制限がある。本番用スケジューラは epoll/kqueue 等を使うべきで、簡易な出発点としては [`io-event`](https://github.com/socketry/io-event) gem の利用が示唆されている。
- `io_select` と `address_resolve` はブロッキング呼び出しを別スレッド（`Thread.new { ... }.value`）で実行して scheduler を止めない方式、`process_wait` も同様にスレッドで待つ方式で実装されている。これらはあくまで当該サンプルの選択であり、公式の必須実装ではない。
- `scheduler_close` が存在すればスレッド終了時に呼ばれ、存在しなければ旧来の振る舞いとして `close` が呼ばれる。サンプルでは `close` が `run` で残作業を流して urgent pipe を閉じ、自身を freeze する。

### 導入の経緯（リリースノートに記載された事実）

- Fiber scheduler は Ruby 3.0 で導入された。
- 横取りの対象として挙げられているのは `Mutex` / `ConditionVariable` / `Queue` / `Thread#join` / `Kernel#sleep` / `Process.wait` / `IO#wait` / `read` / `write` である。
- イベントループの提供者として Async gem が挙げられている。

## 推奨方法

以下は上記の公式記載とサンプル観察からの独自の設計案であり、公式が個別の構成を推奨するものではない。

- ノンブロッキングにしたいスレッドでは `Fiber.set_scheduler` で scheduler を設定してから `Fiber.schedule`（または非ブロッキング Fiber）で実行する。設定なしに `Fiber.schedule` を呼ばない（`RuntimeError` になる）。
- scheduler 実装者は必要な hook のみでなく、終了経路（`scheduler_close`、なければ `close`）を必ず用意し、待機中の fiber と pipe 等のリソース解放をその経路に集約する。スレッド終了時に呼ばれる契約であるため、呼び出し側の明示的な後始末に依存しない。
- ブロッキングせざるを得ない処理（名前解決など libc 側で止まる操作）は、サンプルと同様に作業スレッドへ逃がすか `blocking_operation_wait` 経路に寄せる設計にし、イベントループ自体を止めない。
- 自作の本番用スケジューラはサンプルをそのまま使わず、`IO.select` 方式の上限（fd 数・同一 fd の重複待機の不備）を前提に、epoll/kqueue ベースの実装（例: `io-event` gem や Async 等の既存提供者）への乗り方を先に検討する。
- ブロッキングで実行すべき区間は `Fiber.blocking` で明示し、scheduler の横取り対象から外す。

## 避ける使い方

- scheduler 未設定での `Fiber.schedule`。`RuntimeError` となる。
- `Fiber.new` の `blocking: false` 既定だけを見て「非ブロッキングになる」と考えること。scheduler 設定なしには非ブロッキング動作は有効にならない。
- テスト用サンプル（`test/fiber/scheduler.rb`）を本番のイベントループとして流用すること。ファイル自身が非効率と未対応条件（多数 fd、同一 fd への重複待機）を明記している。
- hook 内でイベントループを止めるブロッキング呼び出しを直接行うこと。止まる処理は別スレッドや `blocking_operation_wait` 経路へ逃がす。
- 終了処理を呼び出し側の任意の `close` 呼び出しだけに頼ること。`scheduler_close` とユーザ呼び出しの `close` は意味が異なる（サンプルコメントの区別に従う）。

## 適用版と本番での注意

- 公式 RDoc 2 件は Ruby 3.4 の表示版で確認した。将来の最新版を示すものではない。
- サンプル実装は v3_4_1 タグ（commit `48d4efcb8`）に固定して確認した観察事実で、公式 API 契約と同一視しない。将来の版で実装は変わり得る。
- scheduler interface 自体の導入版は Ruby 3.0.0（2020-12-25 リリース）である。3.0 より前の Ruby には存在しない。
- リリースノート 1 件は source type 上 `release_notes`（TTL 30 日）のため、本ドキュメントの明示期限は 2026-10-28 とし、期限前に公式文書を再確認して日付を延ばす。
- 本ドキュメントは hook の引数シグネチャの網羅、Async gem など提供者ごとの性能特性、スレッド数や負荷条件での振る舞いは対象外で未検証。提供者選定時は各提供者の文書と測定で確認する。

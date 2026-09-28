---
{
  "id": "python-asyncio-taskgroup-timeout-cancellation",
  "title": "Python asyncio の構造化並行性: asyncio.timeout と TaskGroup のキャンセル伝播",
  "kind": "knowledge",
  "technology": "python",
  "version": "Python 3.14.7 Documentation (asyncio-task 表示版、What's New 3.11 / 3.13)",
  "tags": [
    "research-domain:backend",
    "python",
    "asyncio",
    "TaskGroup",
    "asyncio.timeout",
    "timeout_at",
    "wait_for",
    "CancelledError",
    "TimeoutError",
    "ExceptionGroup",
    "structured-concurrency",
    "cancellation",
    "uncancel",
    "cancelling",
    "キャンセル",
    "伝播"
  ],
  "sources": [
    {
      "id": "python-asyncio-task-docs",
      "url": "https://docs.python.org/3/library/asyncio-task.html",
      "type": "official_docs"
    },
    {
      "id": "python-whatsnew-3-11",
      "url": "https://docs.python.org/3/whatsnew/3.11.html",
      "type": "release_notes"
    },
    {
      "id": "python-whatsnew-3-13",
      "url": "https://docs.python.org/3/whatsnew/3.13.html",
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

# Python asyncio の構造化並行性: asyncio.timeout と TaskGroup のキャンセル伝播

`asyncio.timeout()` と `asyncio.TaskGroup` は内部でキャンセルを使って実装され、期限切れと子タスク失敗を「構造化ブロック単位」へ局所化する。以下は Python 3.14.7 の公式文档 [Coroutines and tasks](https://docs.python.org/3/library/asyncio-task.html)、[What's New In Python 3.11](https://docs.python.org/3/whatsnew/3.11.html)、[What's New In Python 3.13](https://docs.python.org/3/whatsnew/3.13.html) に記載された事実と、そこからの独自の設計案を分けて記録する。3文書はいずれも 2026-09-28 に取得した。

## 要点

### asyncio.timeout / timeout_at の挙動（公式文档に記載された事実）

- `asyncio.timeout(delay)` と `asyncio.timeout_at(when)` は 3.11 で追加された非同期コンテキストマネージャで、待機時間の上限を設ける。`delay` は `None` か秒数の float/int で、`None` なら期限は設定されない（作成時点で上限が未知の場合に有用）。作成後は `Timeout.reschedule()` で再スケジュールできる（`delay` が `None` でも同様）。[timeouts](https://docs.python.org/3/library/asyncio-task.html#timeouts)
- 期限を超えると「the context manager will cancel the current task and handle the resulting `asyncio.CancelledError` internally, transforming it into a `TimeoutError`」。つまり**現在のタスク**がキャンセルされ、内部で `CancelledError` を処理して `TimeoutError` へ変換する。
- 変換はコンテキストマネージャの内部で行われるため、`TimeoutError` は **`async with` の外側でのみ捕捉できる**（公式 Note: "the `TimeoutError` can only be caught *outside* of the context manager"）。
- `Timeout` を直接生成するより `timeout()` / `timeout_at()` を使うよう公式は指示する。`Timeout.when()` は現在の期限（未設定なら `None`）、`Timeout.expired()` は期限超過の真偽を返す。`when` が `None` なら期限は永遠に発火せず、`when < loop.time()` なら次のイベントループ反復で発火する。
- 「Timeout context managers can be safely nested.」— 入れ子の使用は安全と公式に記載されている。
- `wait_for(aw, timeout)` はタイムアウト時に `aw` をキャンセルして `TimeoutError` を上げる。3.7 からキャンセル完了まで待つため「the total wait time may exceed the *timeout*」。3.11 で `asyncio.TimeoutError` ではなく組み込み `TimeoutError` を上げるよう変更され、3.12 で `asyncio.timeout()` の実装に置き換えられたため、「a coroutine passed as *aw* is no longer wrapped in a `Task` when *timeout* is positive」。[wait_for](https://docs.python.org/3/library/asyncio-task.html#asyncio.wait_for)

### TaskGroup の例外とキャンセル伝播（公式文档に記載された事実）

- `asyncio.TaskGroup` は 3.11 追加。退出時に全タスクを await する非同期コンテキストマネージャ。グループのタスクが `asyncio.CancelledError` **以外**の例外で初めて失敗すると残タスクをキャンセルし、以降タスクを追加できなくなる。[Task groups](https://docs.python.org/3/library/asyncio-task.html#task-groups)
- このとき `async with` の本文がまだアクティブなら、それを含むタスク自体もキャンセルされる。生じた `CancelledError` は await を中断するが、外側の `async with` 文からは伝播しない（"it will not bubble out of the containing `async with` statement"）。
- 全タスクの完了後、`CancelledError` 以外の失敗例外は `ExceptionGroup` / `BaseExceptionGroup` に統合されて raise される。`KeyboardInterrupt` と `SystemExit` は 2 つの特別扱いの基本例外で、残タスクをキャンセルして待った後、グループではなく元の例外がそのまま再送出される。
- `async with` の本文側の例外（`__aexit__` に設定されて到達する例外）もタスク失敗と同様に扱われる。残タスクをキャンセルして待機し、`CancelledError` 以外は（本文の例外も含めて）例外グループへ統合される。`KeyboardInterrupt` / `SystemExit` は同じ特別扱い。
- 入れ子の TaskGroup で子タスクの例外が同時に発生した場合、内側グループがまず自分の例外を処理し、その後に外側グループがもう一度キャンセルを受けて自分の例外を処理する。
- グループが外部からキャンセルされつつ `ExceptionGroup` を raise すべき場合、TaskGroup は親タスクの `cancel()` を呼ぶ。これにより次の await で必ず `CancelledError` が発生し、キャンセルが失われない（"so the cancellation is not lost"）。
- 「Task groups preserve the cancellation count reported by `asyncio.Task.cancelling()`」。`Changed in version 3.13: Improved handling of simultaneous internal and external cancellations and correct preservation of cancellation counts.`
- 非アクティブなグループ（未エントリー・終了済み・シャットダウン中）への `create_task()` は `RuntimeError` を上げ、渡されたコルーチンをクローズする。`Changed in version 3.13: Close the given coroutine if the task group is not active.`（クローズは未 await の `RuntimeWarning` を防ぐ。gh-115957）
- グループのネイティブな終了制御は標準ライブラリに無く、例外を投げるタスクを追加してその例外を握りつぶす方式が公式の "Terminating a task group" 節に記載されている。

### キャンセル状態の隔離: uncancel / cancelling（公式文档に記載された事実）

- タスクのキャンセル時、`asyncio.CancelledError` は「次の機会」にタスク内へ raise される。クリーンアップには `try/finally` を使うよう推奨され、`CancelledError` を明示的に捕捉した場合はクリーンアップ後に原則再送出する。`CancelledError` は `BaseException` を直接継承する。[Task cancellation](https://docs.python.org/3/library/asyncio-task.html#task-cancellation)
- 公式の明言: `TaskGroup` と `asyncio.timeout()` を含む構造化並行性のコンポーネントは内部でキャンセルを使って実装されており、コルーチンが `CancelledError` を飲み込むと動作不良する可能性がある（"might misbehave if a coroutine swallows `asyncio.CancelledError`"）。同様にユーザコードが `uncancel()` を呼ぶべきではない。抑制が本当に必要な場合、「it is necessary to also call `uncancel()` to completely remove the cancellation state」。
- `Task.cancel(msg=None)` はキャンセル要求。既に done/cancelled なら `False`、そうでなければ `True` を返し、イベントループの次のサイクルで `CancelledError` をコルーチンへ投げ込む。`Future.cancel()` と異なり `Task.cancel()` はキャンセルを保証しない（抑制は可能だが非推奨と公式が明記）。抑制するなら捕捉に加えて `Task.uncancel()` の呼び出しが必要。`Task.cancelled()` は要求と `CancelledError` の伝播が両方起きたときに `True`。
- `Task.uncancel()`（3.11 追加）はキャンセル要求のカウントを減らし、残数を返す。公式は「asyncio's internals 用でありエンドユーザコードでの使用は想定されていない」と位置づける。成功裏に uncancel されることで `TaskGroup` や `asyncio.timeout()` が継続でき、キャンセルはそれぞれの構造化ブロックへ隔離される。公式例では `asyncio.timeout(1)` の外で捕捉後に `await unrelated_code()` が続行する。カウントが 0 に達すると、未投げ込みの `cancel()` の安排を無効化する（内部 `_must_cancel` フラグのリセット）。`Changed in version 3.13: Changed to rescind pending cancellation requests upon reaching zero.`
- `Task.cancelling()`（3.11 追加）は pending キャンセル要求数、すなわち `cancel()` 回数から `uncancel()` 回数を減じた値を返す。正のままタスクが実行中なら `cancelled()` はまだ `False` を返し得る。こちらも internals 用でエンドユーザコード向けではない。

### gather との違い（公式文档に記載された事実）

- 公式の Note（gather の節）: サブタスクの入れ子実行では `TaskGroup` は `gather` より強い安全性を提供する。「if a task (or a subtask, a task scheduled by a task) raises an exception, `TaskGroup` will, while `gather` will not, cancel the remaining scheduled tasks」。
- `gather(return_exceptions=False)` では最初の例外が即座に伝播するが、他 awaitable はキャンセルされず実行を続け、`gather` 本体もキャンセルされない。`gather` 自体がキャンセルされた場合、未完了の submit 済み awaitable がすべてキャンセルされる。個々の Task/Future がキャンセルされた場合は、その 1 件が `CancelledError` を raise したのと同じ扱いになり、`gather` はキャンセルされない。
- `wait_for` の `aw` をキャンセルから守る手段として、公式は `shield()` のラップを挙げる。

### 版の導入と改善（リリースノートに記載された事実）

- Python 3.11（2022-10-24 リリース）: `TaskGroup` を追加し、「For new code this is recommended over using `create_task()` and `gather()` directly」（gh-90908）。`timeout()` を追加し、「For new code this is recommended over using `wait_for()` directly」（gh-90927）。`Task.cancelling()` と `Task.uncancel()` は「primarily intended for internal use, notably by `TaskGroup`」として追加。PEP 654 の `ExceptionGroup` / `BaseExceptionGroup` と `except*` も 3.11 で導入。[What's New In Python 3.11](https://docs.python.org/3/whatsnew/3.11.html)
- Python 3.13（2024-10-07 リリース）: 外部キャンセルと内部キャンセルの衝突時挙動を改善。入れ子の 2 つの TaskGroup で同時に子タスク例外が出たとき、内側グループが外側の内部キャンセルを飲み込み **外側グループがハングしうる**問題（Arthur Tacca 報告、gh-116720）。外部キャンセル+`ExceptionGroup` 時は親タスクの `cancel()` を呼んで `CancelledError` を次の await で確実に発生させる。`cancelling()` カウントが保持されるようになり、`uncancel()` はカウント 0 到達時に（非公開の）`_must_cancel` フラグをリセットしうる。非アクティブ `TaskGroup` の `create_task()` でコルーチンをクローズ（gh-115957）。[What's New In Python 3.13](https://docs.python.org/3/whatsnew/3.13.html)

## 推奨方法

以下は上記の公式記載からの独自の設計案であり、公式が個別の構成を推奨するものではない。ただし「新規コードでは TaskGroup / timeout() を優先する」点は 3.11 リリースノートの公式推奨である。

- 新規コードでは `create_task()` + `gather()` の直書きではなく `TaskGroup`、`wait_for()` ではなく `asyncio.timeout()` を既定にする（3.11 リリースノートの公式推奨）。
- `TimeoutError` のハンドラは `async with asyncio.timeout(...)` の**外側**に置く。内側では捕捉できない（変換が CM 内で行われるため）。
- 期限は実運用の締め切りとして設定し、発火時に「現在のタスク」が止まる前提で呼ぶ。子タスク単位の停止が必要なら `TaskGroup` 側で管理する（`timeout` は現在タスクをキャンセルする契約）。
- クリーンアップは `try/finally` に置き、`CancelledError` を捕捉したときは後始末完了後に再送出する（公式の推奨どおり）。
- `TaskGroup` の失敗は `ExceptionGroup` として設計し、分岐は `except*` で書く（PEP 654、3.11+。公式記載では `except*` は `except` を一般化して例外グループの部分群にマッチさせる構文として 3.11 に導入された）。
- `KeyboardInterrupt` / `SystemExit` はグループに統合されず元の形で再送出される前提で、最外側のハンドラを設計する。
- `uncancel()` / `cancelling()` はライブラリやフレームワークの実装時以外では呼ばない。キャンセルを抑制する経路をアプリケーションコードに作らない。
- 3.12 以前の Python で入れ子 TaskGroup と `cancelling()` の保持を前提にしたコードを動かすなら、3.13 への更新を検討する（3.13 リリースノート記載のハング問題）。

## 避ける使い方

- `async with asyncio.timeout(...)` の**内側**で `except TimeoutError` する。変換は CM 内部で起きるため内側では捕捉できない。
- `CancelledError` を `uncancel()` なしに握りつぶす。`TaskGroup` / `timeout` の実装は動作不良を起こし得、キャンセル状態がタスクに残る。抑制が必要なら公式が要求する `uncancel()` の併呼が必須。
- `Task.cancel()` が必ずタスクを終わらせると信じる。コルーチンは捕捉で拒否でき、公式も保証しないと明記している（抑制自体は非推奨）。
- `TaskGroup` の子タスク失敗が通常の単一例外として外へ伝播すると考える。伝播は `ExceptionGroup` / `BaseExceptionGroup` への統合であり、分岐には 3.11 導入の `except*` を使う。
- 終了済み・未エントリーの `TaskGroup` に `create_task()` を呼ぶ。非アクティブ時は `RuntimeError` となり、3.13 以降は渡されたコルーチンもクローズされる。3.13 より前はクローズが行われず、未 await のままのコルーチンによる `RuntimeWarning` が生じ得た（gh-115957 はこれを防ぐ変更として記載）。
- 3.12 以前で、入れ子 TaskGroup の同時例外や `cancelling()` 保持を安定動作として頼る。3.13 より前では外側のグループがハングしうるとリリースノートが明記している。
- `wait_for` の `timeout` を「待機時間の厳密な上限」と扱う。キャンセル完了待ちで総時間は上限を超過し得る（公式記載）。また 3.12 より前では正の timeout のコルーチンが Task 化される点を踏まえる。
- `gather` の最初の例外伝播後に「残タスクも止まる」と考える。`return_exceptions=False` でも他 awaitable は走り続ける。残タスクの一括停止が要件なら `TaskGroup` を使う。

## 適用版と本番での注意

- Python 3.14.7 Documentation の各ページを 2026-09-28 に取得して確認した。ページのソースは 3.14 ブランチ（`Doc/library/asyncio-task.rst`、`Doc/whatsnew/3.11.rst`、`Doc/whatsnew/3.13.rst`）で、将来の最新版を示すものではない。
- この文書の API 群の適用下限は **3.11**（`TaskGroup`、`timeout()` / `timeout_at()`、`uncancel()` / `cancelling()`、`except*` と `ExceptionGroup` が揃う版）。3.10 以前には存在しない。
- 3.13 で外部・内部キャンセルの同時処理と `cancelling()` 保持が改善された。3.11 / 3.12 では入れ子同時例外で外側グループがハングする問題が残る（gh-116720 は 3.13 で対応）。
- `wait_for` は 3.11 で `TimeoutError`（`asyncio.TimeoutError` ではない）、3.12 で `asyncio.timeout()` 実装に変更され、正の timeout のコルーチンが Task 化されなくなった。古い版での戻り値とタスク化挙動が異なる点に注意する。
- `TaskGroup.create_task()` は 3.14 で全 `**kwargs` を `loop.create_task()` へ転送する（公式の `Changed in version 3.14`）。カスタムタスクファクトリとの互換性は 3.13 系で一度破壊され 3.13.4 で復旧した経緯があり（gh-128307）、ファクトリを使う構成では版差を確認する。
- 「推奨方法」節は独自の設計案であり、うち新規コードでの API 選択のみ 3.11 リリースノートの公式推奨に当たる。事実節は各出典の該当節の記載に従う。
- リリースノート 2 件は source type 上 `release_notes`（TTL 30 日）のため、本ドキュメントの取得・期限の基準日は 2026-09-28 で、明示期限は 2026-10-28。期限前に公式文書を再確認して日付を延ばす。
- 本ドキュメントはキャッシュ、イベントループ実装差、サードパーティランタイム（uvloop 等）でのキャンセルタイミング差は対象外で未検証。

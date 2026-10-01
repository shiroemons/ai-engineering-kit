---
{
  "id": "python-futures-bounded-map-timeout-shutdown",
  "title": "Python concurrent.futures: buffersize による投入制御と timeout・キャンセル・shutdown の境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "Python 3.14.8 / 3.13.16 documentation（表示版を2026-10-01確認）; buffersize は3.14追加、共通契約の限定実測はCPython 3.12.14",
  "tags": [
    "research-domain:backend",
    "concurrent.futures",
    "ThreadPoolExecutor",
    "ProcessPoolExecutor",
    "Executor.map",
    "buffersize",
    "backpressure",
    "cancellation",
    "shutdown",
    "timeout"
  ],
  "sources": [
    {
      "id": "python-futures-3-14-8-20261001",
      "url": "https://docs.python.org/3.14/library/concurrent.futures.html",
      "type": "official_docs"
    },
    {
      "id": "python-futures-3-13-16-20261001",
      "url": "https://docs.python.org/3.13/library/concurrent.futures.html",
      "type": "official_docs"
    },
    {
      "id": "python-whatsnew-futures-3-14-20261001",
      "url": "https://docs.python.org/3.14/whatsnew/3.14.html",
      "type": "release_notes"
    },
    {
      "id": "python-threading-events-3-14-8-20261001",
      "url": "https://docs.python.org/3.14/library/threading.html",
      "type": "official_docs"
    },
    {
      "id": "python-multiprocessing-termination-3-14-8-20261001",
      "url": "https://docs.python.org/3.14/library/multiprocessing.html",
      "type": "official_docs"
    },
    {
      "id": "python-license-futures-3-14-8-20261001",
      "url": "https://docs.python.org/3.14/license.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Python concurrent.futures の投入制御と停止境界

## 問い・適用範囲

`max_workers` を小さくしたのに大量入力でメモリが増える、`Future.result(timeout=...)` が期限切れになったのに処理が続く、例外を捕捉した後も `with` から戻らない。この三つを、投入量・結果の消費・実行中の仕事・Executor の寿命に分けて考える。

本稿は同期 callable を thread/process pool に投入する処理が対象。`concurrent.futures.Future` と `asyncio.Future` / `Task` は別 API であり、既存の asyncio TaskGroup キャンセル記事の置換ではない。公式文書の契約、そこからの設計判断、ローカルで観測した事実を分けて記載する。

2026-10-01 UTC に3.14系ページの表示版 **3.14.8**、比較用の3.13系ページの表示版 **3.13.16** を確認した。これは実機にインストールした版や最新安定版の宣言ではない。[What's New](https://docs.python.org/3.14/whatsnew/3.14.html#concurrent-futures) は `buffersize` の導入を3.14、3.14の公開日を2025-10-07と記す。今回の選定は未収録だった実用上の穴を埋めるもので、2026-10-01に新設された機能とは扱わない。

## 公式契約: 投入制御は worker 数と異なる

[3.14 Executor.map](https://docs.python.org/3.14/library/concurrent.futures.html#concurrent.futures.Executor.map) で確認した要点:

- `buffersize=None` が既定。省略時は入力 iterable を遅延消費せず、直ちに収集する。generator を渡すだけではバックプレッシャーにならない
- `buffersize` は投入済みで結果がまだ yield されていない仕事を制限し、buffer が満ちると結果を返すまで入力の反復を止める。未開始の仕事だけの上限ではなく、完了したが未返却の結果も考慮する
- `map` は組み込み `map` と同様の入力順で値を取り出す。実行・完了順の保証ではない。処理の例外はその値を取り出す際に表面化する
- `ProcessPoolExecutor.map` は入力要素を `chunksize` ごとの chunk にまとめ、chunk を仕事として投入する。thread / interpreter pool では `chunksize` は効果を持たない
- `map(timeout=...)` の起点は最初の `map` 呼出し。各要素の処理開始や各 `next()` ごとに予算を再設定する API ではない

[3.13の同 API](https://docs.python.org/3.13/library/concurrent.futures.html#concurrent.futures.Executor.map) の署名には `buffersize` がなく、入力を直ちに収集する契約である。3.13以前向けのコードへ新引数だけを持ち込まない。

### この契約からの設計判断

以下は独自の設計案であり、標準ライブラリが全体の容量を保証するという意味ではない。

1. `max_workers` は同時に仕事を処理する worker 数、`buffersize` は結果消費へ連動した投入制御として、別々に設定する。worker を4にしただけで百万件の `submit` を有限の待ち行列に変えたつもりにならない
2. `buffersize` はバイト数・RSS・上流 DB cursor のプリフェッチ量・下流の蓄積量の上限ではない。入力1件の大きさ、処理中の一時領域、結果の大きさ、consumer が保存する量も別に測る。`list(executor.map(...))` は最終的に全結果を保持する
3. process pool では `buffersize=8, chunksize=100` を「未消費の入力は8件以内」と解釈しない。chunk の個数と入力要素数を区別する。ここで両引数の積を全メモリ量や、あらゆる瞬間の入力先読み件数の厳密な式とはしない
4. 最初の仕事だけ遅いと、後続の仕事が終わっても入力順の結果返却を待つ。この head-of-line blocking により、処理が終わった worker がいても新規投入を十分に進められない場合がある。buffer 増量はこの偏りを隠せても、順序待ち自体は解消しない
5. consumer が途中で `break` した、結果イテレータを保持したまま放置した、または例外を受けたという事実だけを、投入済みの副作用が全停止した証拠にしない。Executor の終了責任を caller に残す

## 公式契約: timeout・cancel・shutdown

3.13と3.14の公式文書にある共通契約を整理する。[Future](https://docs.python.org/3.13/library/concurrent.futures.html#future-objects)、[shutdown](https://docs.python.org/3.13/library/concurrent.futures.html#concurrent.futures.Executor.shutdown)、[ThreadPoolExecutor](https://docs.python.org/3.13/library/concurrent.futures.html#threadpoolexecutor) が根拠。

- `Future.result(timeout)` は結果の待機期限を超えると `TimeoutError`、取り消された Future なら `CancelledError`、仕事が失敗したならその例外を送出する
- `Future.cancel()` は実行中・実行済みの仕事を取り消せず `False` を返す。取消成功なら `True`。戻り値を確認せず「cancelを呼んだので止まった」と記録しない
- `shutdown(cancel_futures=True)` は未開始の Future を取り消す。実行中・完了済みは対象外。`cancel_futures` は3.9追加
- `shutdown(wait=False)` は呼出しを待たずに返すが、未完了の Future が終わる前にPythonプログラム全体が終了できるという契約ではない。`with Executor(...)` の退出は `wait=True` 相当
- thread pool 内で同じ pool の Future を待つと、worker 枯渇や相互待ちでデッドロックし得る。インタプリタ終了時の thread join は利用者が登録した `atexit` handler より先に行われる

したがって「待機を打ち切る」「未開始を取り消す」「実行中の仕事が戻る」「pool の資源が解放される」は個別の状態として観測する。`result` の `TimeoutError` は、外部送信や DB 更新が未実行だった証明ではない。timeout 後の再試行には、元の仕事が遅れて成功する経路も含めた重複防止が必要になる。この業務設計は独自の提案である。

### with から戻らない理由

たとえば `with ThreadPoolExecutor(...)` 内で `future.result(timeout=1)` の例外を外へ送出しても、退出処理が実行中の仕事を待てば、呼出し全体は1秒で終わらない。外側の `except TimeoutError` に到達する前に待機し得る。timeout を増減する前に、どの境界の待ち時間を測っているかを分ける。

`wait=False` は thread の強制停止手段ではない。また、`atexit` にだけ停止シグナルを書くと、pool の join より遅いという順序問題が起きる。正常時だけでなく、主処理の例外経路でも worker が戻れる条件を用意する。

## 実行中の仕事を止める設計

### ThreadPoolExecutor: 協調停止

[threading](https://docs.python.org/3.14/library/threading.html) は `Thread` を外部から停止・割り込みする API を提供しないと記し、正常な停止の通信に `Event` などを挙げる。[Event](https://docs.python.org/3.14/library/threading.html#event-objects) は `set()` でフラグを立て、`is_set()` で検査し、`wait(timeout)` でシグナルまたは期限を待てる。

これを使う独自の手順案:

1. caller は新規投入を止め、共有 `Event` に停止要求を出す
2. worker は処理の区切りでフラグと自分の期限を検査する。停止要求に気づける間隔を、処理単位とI/Oの timeout で設計する
3. 未開始の Future を取り消し、実行中の仕事は協調停止・後始末を経て戻るのを待つ
4. 未開始取消、処理途中停止、成功、失敗を区別して記録する。Event による途中終了を通常の戻り値で表現する場合、それは Future の cancelled 状態と同じではない

Event は任意のブロッキング I/O を自動で中断しない。たとえば外部ライブラリが期限なしの読み取りに入れば、次のフラグ検査まで戻れない。ネットワーク・DBクライアント自身の期限と、その期限が接続・読み取り・処理全体のどれを制御するかを別に確認する。

この共有 Event 案をそのまま process / interpreter pool へ横展開しない。プロセス間通信や interpreter の隔離は別契約であり、本稿では停止シグナルの具体的な転送方式を検証していない。

### ProcessPoolExecutor: pool 単位の強制終了

3.14では [terminate_workers / kill_workers](https://docs.python.org/3.14/library/concurrent.futures.html#concurrent.futures.ProcessPoolExecutor.terminate_workers) が追加された。生存 worker 全体に `Process.terminate()` / `Process.kill()` を試み、内部で Executor の shutdown も行う。呼出し後に同じ pool へ追加投入しない。これは個別 Future 一件の安全な取消 API ではない。

[multiprocessing の警告](https://docs.python.org/3.14/library/multiprocessing.html#multiprocessing.Process.terminate) では、終了 handler や `finally` が走らず、子孫プロセスも自動では終わらない。pipe / queue 破損や、保持中の lock に起因する他プロセスのデッドロックも起こり得る。

独自の判断基準として、強制終了を要件にするなら、破棄可能な単位へ仕事を分離し、外部副作用の整合性と資源の再生成まで含めて設計する。単に thread pool を process pool に置き換えても、安全なロールバックや厳密な終了 SLA が得られるわけではない。

## 入力順が不要な処理の選択肢

[wait / as_completed](https://docs.python.org/3.14/library/concurrent.futures.html#module-functions) は完了した Future を扱える。`wait(..., return_when=FIRST_COMPLETED)` はいずれかの完了・取消で戻り、`as_completed` は完了した Future を順次返す。ただし、大量の `submit` の後に `as_completed` を使っても投入量は抑えられない。

以下は3.13以前にも適用を検討できる有限集合の設計案で、再利用 module や検証済み実装の提供ではない。

- caller 側で最大N件の Future だけを持ち、`FIRST_COMPLETED` で回収する
- 完了を回収し、結果の保存先が受け入れた分だけ入力から補充する。失敗時に継続するか全体を止めるかを先に決める
- 入力IDを Future と対応付け、完了順の結果に元のIDを残す。後から入力順へ並べ直すなら、その reorder buffer も容量設計に含める
- pool 内の worker に「同じ pool へ子仕事を submit して同期的に待つ」役割を持たせず、依存の調整を caller 側へ置く

小さいNはメモリと未処理量を抑えやすい一方、結果保存の遅延や仕事のばらつきによって worker の稼働率を下げる。Nや `buffersize` は固定の万能値を置かず、throughput・待ち時間・最大RSS・停止完了までの時間を同時に見て決める。

## 検証したこと・受入試験の案

今回のローカル環境には CPython **3.12.14** があった。標準ライブラリだけの一時的な実験で、1 worker を Event によって実行中に保持し、次を確認した。worker の待機には5秒の試験用上限を設け、後始末で必ず Event を開放した。

- `result(timeout=0.01)` が `TimeoutError` を出した後も、対象 Future は running で cancelled ではなかった
- 実行中の `cancel()` は `False`、待ち行列に残した未開始 Future の `cancel()` は `True`。後者の `result()` は `CancelledError` になった
- 別の未開始 Future を残して `shutdown(wait=False, cancel_futures=True)` を呼ぶと、未開始だけが cancelled になり、実行中の Future は残った。Event 開放後、実行中の仕事は正常完了した

この実験は共通契約の限定確認であり、3.14の `buffersize`、process pool、負荷時メモリ量、OSをまたぐ終了動作を実測したものではない。3.14環境を新規導入しての試験は今回行っていない。

本番適用前に実施する試験案:

1. 3.14で入力 generator に消費カウンタを付け、consumer を止める。有限 `buffersize` と既定値で先読みとRSSを比較する。内部 queue の瞬間的な長さを公開契約と決めつけない
2. 先頭だけ遅い入力、途中の例外、大きい戻り値、遅い保存先を組み合わせ、入力順の待機と結果保持量を観測する
3. process pool の `chunksize=1` と大きな値を比較し、入力要素の先読み、待機時間、chunk内の失敗影響を対象版で確認する
4. worker の開始を Event で同期してから cancel と timeout を試す。短い `sleep` の競争だけで「未開始」と推定しない
5. `with` の内側の timeout と外側の例外捕捉までを別々に計測する。協調停止しない仕事を含む試験は、監視する別プロセスの時間制限下で行う
6. consumer の `break`、主処理の例外、保存先障害のそれぞれで、投入停止・未開始取消・実行中の後始末・外部副作用の記録を確認する

検索 eval は記事の発見可能性を確認するだけで、これら実行時保証の代わりにはならない。

## 出典・鮮度・未確認事項

`concurrent.futures` と What's New の3.14ページの更新表示は2026-10-01 10:41 UTC。ページの更新日を各機能の公開日へ読み替えない。公式文書の継続更新ページを参照し、CPython実装のcommit固定分析は行っていないため、内部キュー・先読みの正確な瞬間件数・イテレータ破棄時の実装依存の後始末は未確認とする。

[History and License](https://docs.python.org/3.14/license.html) により文書は PSF-2.0、文書内のコード例等は追加で0BSDと確認した。コードや例文の転載はなく、独自の日本語整理と試験案を保存する。全 source の取得日は2026-10-01 UTC。release_notes の30日TTLが最短なので明示期限は2026-10-31。Python 3.15、サードパーティ Executor、free-threaded build の性能比較、InterpreterPoolExecutor のデータ共有・拡張モジュール互換性は対象外。

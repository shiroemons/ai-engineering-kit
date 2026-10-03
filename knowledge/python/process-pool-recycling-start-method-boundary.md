---
{
  "id": "python-process-pool-recycling-start-method-boundary",
  "title": "Python ProcessPoolExecutor: worker再生成・起動方式・初期化の境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "CPython 3.14.8 (2026-09-30), 8e6e75d9102e39bed2a2b279203a396741180f12; gh-115634修正は3.14.7 (2026-08-05); 2026-10-03 UTC確認",
  "tags": [
    "research-domain:backend",
    "ProcessPoolExecutor",
    "multiprocessing",
    "max_tasks_per_child",
    "forkserver",
    "spawn",
    "mp_context",
    "initializer",
    "worker-recycling"
  ],
  "sources": [
    {
      "id": "python-futures-recycling-3148-20261003",
      "url": "https://docs.python.org/3.14/library/concurrent.futures.html",
      "type": "official_docs"
    },
    {
      "id": "python-multiprocessing-context-3148-20261003",
      "url": "https://docs.python.org/3.14/library/multiprocessing.html",
      "type": "official_docs"
    },
    {
      "id": "python-whatsnew-process-start-314-20261003",
      "url": "https://docs.python.org/3.14/whatsnew/3.14.html",
      "type": "release_notes"
    },
    {
      "id": "cpython-process-recycling-3148-20261003",
      "url": "https://github.com/python/cpython/tree/8e6e75d9102e39bed2a2b279203a396741180f12",
      "type": "github_repository_analysis"
    },
    {
      "id": "cpython-process-recycling-fix-314-20261003",
      "url": "https://github.com/python/cpython/commit/42f81fa7a6c523019907dfda733359e0e7602f62",
      "type": "github_repository_analysis"
    },
    {
      "id": "python-release-3147-recycling-20261003",
      "url": "https://www.python.org/downloads/release/python-3147/",
      "type": "release_notes"
    },
    {
      "id": "python-release-3148-recycling-20261003",
      "url": "https://www.python.org/downloads/release/python-3148/",
      "type": "release_notes"
    },
    {
      "id": "python-license-recycling-3148-20261003",
      "url": "https://docs.python.org/3.14/license.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# ProcessPoolExecutor の worker 再生成を有効にする前に

## 問い・適用範囲

長時間動く process pool のメモリ増加を抑えるために `max_tasks_per_child` を足したところ、初回起動が遅くなった、親で設定した値が見えない、何件か処理した後で止まった。この変更を「一定件数ごとの再起動」だけでレビューしてよいか。

結論は、**起動方式、worker単位の初期化、再生成後の進捗を一緒に検証する**ことである。特に `mp_context` を省略したまま再生成を有効にすると、OS既定とは別に `spawn` が選ばれる。小さな正常系だけでは、worker交代時の不具合も発見できない。

対象は標準ライブラリの `concurrent.futures.ProcessPoolExecutor`。既存の [投入量・timeout・shutdownの文書](futures-bounded-map-timeout-shutdown.md) が扱う待機期限や取消の説明は繰り返さず、workerの生成と交代へ絞る。`multiprocessing.Pool` の `maxtasksperchild`、サードパーティpool、Python 3.15は別契約として対象外にする。

API文書の表示版は3.14.8、実装・テストは `v3.14.8` の commit `8e6e75d9102e39bed2a2b279203a396741180f12` に固定した。[3.14.8公開日](https://www.python.org/downloads/release/python-3148/) は2026-09-30。以下の実装観察はこのsnapshotに限り、Pythonを実行して得た結果ではない。

## 公式契約: 再生成引数が起動方式を変える

[ProcessPoolExecutorのAPI](https://docs.python.org/3.14/library/concurrent.futures.html#concurrent.futures.ProcessPoolExecutor) によると、`max_tasks_per_child` は3.11追加で、workerが交代するまでのtask数を指定する。既定の `None` ではpoolの寿命までworkerを使う。数値を指定して `mp_context` を省略すると `spawn`、`fork` コンテキストとの併用は非対応。`initializer` は各workerの開始時に呼ばれ、失敗するとpoolは `BrokenProcessPool` になる。

[3.14の変更記録](https://docs.python.org/3.14/whatsnew/3.14.html#multiprocessing) では、macOS以外のUnixで既定が `fork` から `forkserver` に変わり、Windows/macOSの `spawn` は維持された。これは3.14の導入時の変更であり、2026-09-30に新設された機能ではない。

### 固定snapshotで確認した選択分岐

コンテキストは起動方式と関連する資源の生成APIを束ねるオブジェクトとして扱い、方式名だけの文字列とは区別する。

[process.pyのconstructor](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/concurrent/futures/process.py#L774-L800) と [context.py](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/multiprocessing/context.py#L323-L334) を読んだ結果:

| 指定 | 選ばれる方式・結果 |
|---|---|
| `mp_context=None`, `max_tasks_per_child=None` | `multiprocessing.get_context()` のコンテキスト |
| `mp_context=None`, 正の `max_tasks_per_child` | 明示的に `get_context("spawn")` を取得 |
| `mp_context` に `spawn` / `forkserver` のコンテキストを渡す | 渡されたコンテキストを使用。利用可能性は対象環境で確認 |
| `fork` コンテキストと正の `max_tasks_per_child` | constructorが `ValueError` |

この版のOS既定は、非Windowsで `HAVE_SEND_HANDLE` がありmacOSでなければ `forkserver`、それ以外は `spawn`。従って「すべてのPOSIXでforkserver」とも「Linuxなら引数に関係なくforkserver」とも読まない。アプリが先に設定したglobalなコンテキストと、個別Executorへ渡すコンテキストも区別する。

独自の設計判断として、再生成を導入する変更には、worker数・task上限だけでなく採用する `mp_context` と理由を含める。意図せず方式が変わっても動くという期待に任せず、性能測定と互換試験の条件を固定する。OS既定の変化を避けるためだけに `fork` へ戻す判断は、再生成との非互換を解決しない。

## 親で準備した状態をそのまま期待しない

[multiprocessingの制約](https://docs.python.org/3.14/library/multiprocessing.html#the-spawn-and-forkserver-start-methods) は、`spawn` / `forkserver` でのpicklabilityとmain moduleの安全なimportを要求し、子が見るglobal変数は親の `start()` 時点の値と一致しない場合があると記す。[コンテキストの注意](https://docs.python.org/3.14/library/multiprocessing.html#contexts-and-start-methods) では、`fork` コンテキストで作ったlockを `spawn` / `forkserver` の子へ渡せない例と、ライブラリがcallerからコンテキストを受け取る設計を示す。

ここからの独自の移行手順:

1. worker関数と初期化関数を、子がimportできる通常moduleへ置く。`__main__` ガードはpool作成などの起動処理を囲むために使い、子に必要な関数定義まで親専用の実行枝へ隠さない
2. 親で後から書き換えたglobal設定に依存せず、設定の値を `initargs` やtask引数で明示的に渡す。APIがpicklableを要求することと、接続済みDB clientや外部SDKの内部状態を意味的に共有してよいことは別の検討事項
3. worker内で必要な接続・cacheを初期化する場合、再生成で何度も初期化される前提にする。初期化を「pool全体で一回だけ実施する業務操作」の置き場所にしない
4. lock・queueなどは利用するコンテキストを起点に組み立て、どのコンテキストから作られたかをAPI境界で隠さない。ライブラリのimport時にglobalな `set_start_method()` を呼んでcaller全体の方針を決めない
5. 最初のworkerだけでなく、交代後のworkerでも設定値、依存module、接続先、初期化回数を確認する。親の一時的なmonkey patchやテストfixtureでだけ成功する経路を本番の成立条件にしない

[futuresの文書](https://docs.python.org/3.14/library/concurrent.futures.html#processpoolexecutor) はREPL内の関数やlambdaを期待どおり動くものとして扱わず、callable・引数・戻り値にpicklabilityを求める。通常scriptでのsmoke testは、この起動境界を通すために必要である。picklingの成功だけを外部資源の安全性の証明にはしない。

POSIXのfrozen executableでは `spawn` / `forkserver` が一般に利用できないという [公式の制約](https://docs.python.org/3.14/library/multiprocessing.html#contexts-and-start-methods) もある。配布物がPyInstaller等の実行形式なら、開発環境での通常script成功から互換性を推定せず、配布形式ごとに可否を確認する。

## task上限は入力件数・時間・メモリの上限ではない

固定snapshotの [_process_worker](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/concurrent/futures/process.py#L226-L272) はqueueから受け取るcall itemごとにcounterを進め、上限のcallの結果または例外を送ってから通常の交代経路へ進む。従って、ここでの再生成は実行中callを指定時間で中断する機構ではない。workerがcallから戻らなければ、その正常終了経路にも進めない。

同じ版の [map実装](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/concurrent/futures/process.py#L920-L954) は、入力を `chunksize` 個ずつまとめた `_process_chunk` のcallを送る。**`map` の一つのchunkが一つのworker task** として数えられ、入力要素ごとの回数ではない。これは公開APIから連想した厳密な性能保証ではなく、固定版の実装観察である。

たとえば全chunkが満杯で成功する条件なら、`max_tasks_per_child=10, chunksize=100` のworkerは、交代までに1000回の要素関数を実行し得る。「10行処理したら交代する」という想定にはならない。末尾の短いchunk、途中の例外、複数workerへの配分は別に考える。

独自の容量設計では、task上限に加えて一件の最大作業量と出力保持量を決める。RSS上限、入力総件数、wall-clock deadline、task間の完全な状態初期化は別々の要件である。上限を小さくすれば初期化や再importが頻発し、大きくすれば一つのworkerに状態が残る時間が長くなる。万能な閾値を置かず、交代時の遅延とメモリ推移を同じ測定で評価する。

## 3.14.7で直った交代時の停止を、版付きで確認する

[3.14系修正commit](https://github.com/python/cpython/commit/42f81fa7a6c523019907dfda733359e0e7602f62) は `42f81fa7a6c523019907dfda733359e0e7602f62`。2026-07-03に [PR #152927](https://github.com/python/cpython/pull/152927) で取り込まれた。gh-115634は、終了済みworkerが残したidle semaphoreのtokenにより、必要なreplacementの生成を見送ってしまう問題だった。修正では交代判断をそのtokenへ依存させず、残存process数から補充する。一般の新規submitでのidle worker再利用は別経路である。

[APIのversionchanged](https://docs.python.org/3.14/library/concurrent.futures.html#concurrent.futures.ProcessPoolExecutor) は修正を **3.14.7** と記す。[公開日](https://www.python.org/downloads/release/python-3147/) は2026-08-05。3.14.8 snapshotの [3.14.7 NEWS](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Misc/NEWS.d/3.14.7.rst) にも対応項目がある。ここで確認したのはこの修正の導入版と3.14.8内の実装であり、3.11以降の全patch版・vendor backportの影響範囲を実測したものではない。「3.14.7以降はprocess poolが一切止まらない」という保証にも広げない。

同じNEWSには、交代が `shutdown(wait=False)` の後に来た場合の未完了仕事の取り残し（gh-119592）や、Executorが回収済みでreplacementを作れない場合（gh-152967）の修正もある。固定版の [_replace_dead_worker](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/concurrent/futures/process.py#L408-L471) は、残るworker数・未完了仕事・強制終了中かを見て、進捗できない場合をfailureへ変える。これを「再生成は必ず成功する」「workerが一つ減れば直ちにすべて失敗する」と単純化しない。

運用上の独自の判断は、進捗停止を検出したら、task自身の停止、initializer失敗、worker死亡、再生成失敗を区別できる記録を残すこと。pool内部のprivate属性を書き換えるworkaroundを前提にせず、採用patch版の修正状況と同一条件の回帰試験を先に確かめる。

## レビュー済みのテストと、実施すべき受入試験

3.14.8の [test_process_pool.py](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/test/test_concurrent_futures/test_process_pool.py#L196-L255) を読んだ。既存テストには、再生成有効時に既定がspawnであること、fork併用の拒否、上限を超えたtaskでworkerが交代すること、`test_max_tasks_per_child_pending_tasks_gh115634` がある。後者はworker数/task上限/投入数として `(1, 2, 6)` と `(2, 2, 8)` を使い、交代中にもbacklogが残る経路の結果完了を確認する構成である。これは**テストを読んだ**記録であり、今回そのテストを実行したという意味ではない。

次は本番採用前の独自の受入試験案:

1. 実際に採用するPython patch版・OS・配布形式・コンテキストを記録し、通常scriptとして起動する。親とworkerで採用方式と初期化成功を記録する
2. 1 worker、task上限2に対して6件以上を投入し、最初の世代だけでなく複数回の交代後も全結果を回収できることを確認する。短いbatchだけで合格にしない
3. `spawn` と利用可能なら `forkserver` で同じ入力を処理し、親で変更した設定が明示的な引数なしに伝わることを要件にしていないか点検する
4. `chunksize=1` と大きなchunkを比較し、task数と要素関数の実行数を別々に数える。業務入力の処理量、worker初期化コスト、交代前後のRSSを対応付ける
5. initializer失敗と通常のtask例外を別々に起こす。後者を常にpool故障と扱わず、前者を一件だけの業務失敗として握りつぶさない
6. 再生成境界と終了処理が重なる試験は、監視用の外部プロセスに時間上限を設ける。対象pool自身の `result(timeout=...)` だけでは、退出まで確実に終わる試験にならない

検索evalが確認するのはこの記事の発見可能性だけである。今回利用できたPythonは3.12.14で、対象の3.14.8 interpreterは導入せず、Pythonの挙動試験は行っていない。OS別性能、第三者SDKのfork安全性、frozen executable、free-threaded buildも未検証である。

## 出典・ライセンス・取得上の制約

取得日はすべて2026-10-03 UTC。3.14.8 APIページの更新表示は2026-10-02 20:52 UTCであり、patch公開日とは分けた。API文書・What's New・公開日ページはnative webで本文を開いた。巨大な統合changelogは取得上限、固定GitHub blobページはwebのcache missになったため、版固定の実装・テスト・NEWS・LICENSEはGitHubのread-only file取得で内容を確認した。取得できなかったHTMLを読了したとは扱わない。

[History and License](https://docs.python.org/3.14/license.html) と両commitのLICENSEで、Pythonのソフトウェア・文書はPSF-2.0、文書内コードは追加で0BSDと確認した。`process.py` 冒頭はPSFへのContributor Agreement表記、確認した `context.py` とテスト冒頭には別ライセンスの宣言がなかった。python.orgのreleaseページは [サイトの権利表記](https://www.python.org/about/legal/) に基づき再利用ライセンスを `unknown` とし、日付等の帰属付き要約だけを使う。ソースコード、テストコード、例文のコピーはない。release_notesの30日TTLに合わせ、明示期限を2026-11-02とする。

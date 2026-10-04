---
{
  "id": "ruby-fork-hook-failure-process-boundary",
  "title": "Ruby 4.0.7 fork hook: 子の例外脱出・親の失敗・終了回収の境界",
  "kind": "knowledge",
  "technology": "ruby",
  "version": "CRuby 4.0.7 (2026-09-15) @ 229531a6cfbf07e3caef30dbac24a2a3f3fed482; compared with 4.0.6 @ 03b6d3f8898a28604fe6cb00eae3226b821168f4; Ruby 4.0 RDoc; 2026-10-04 UTC確認; runtime未実行",
  "tags": [
    "research-domain:backend",
    "ruby",
    "Process.fork",
    "Process._fork",
    "fork-hook",
    "child-process",
    "exception",
    "ruby_stop",
    "waitpid2",
    "at_exit"
  ],
  "sources": [
    {
      "id": "ruby-fork-process-api-4-0-20261004",
      "url": "https://docs.ruby-lang.org/en/4.0/Process.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-fork-release-4-0-7-20261004",
      "url": "https://github.com/ruby/ruby/releases/tag/v4.0.7",
      "type": "release_notes"
    },
    {
      "id": "ruby-fork-report-22243-20261004",
      "url": "https://bugs.ruby-lang.org/issues/22243",
      "type": "incident_report"
    },
    {
      "id": "ruby-fork-runtime-229531a6-20261004",
      "url": "https://github.com/ruby/ruby/tree/229531a6cfbf07e3caef30dbac24a2a3f3fed482",
      "type": "github_repository_analysis"
    },
    {
      "id": "ruby-fork-before-03b6d3f8-20261004",
      "url": "https://github.com/ruby/ruby/tree/03b6d3f8898a28604fe6cb00eae3226b821168f4",
      "type": "github_repository_analysis"
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

# Ruby 4.0.7 fork hook: 子の例外脱出・親の失敗・終了回収の境界

## 解く問いと採用判断

監視・接続管理ライブラリが `Process._fork` をoverrideする環境で、`fork { ... }` を呼んだ子が、blockを実行せず親向けのrescue loopへ戻ることがある。Rubyのpatch更新で何が直り、親が受けた例外や子の終了から何を判断できるか。

**CRuby 4.0.7のBug #22243修正は、overrideしたfork hookの実行・戻り値変換で例外が出たとき、実際にfork後の子ならVMの終了経路へ移す。親での例外は伝播する。** block形式だけの保護ではなく、共通呼出経路の変更である。親がPIDを受け取ること、子の初期化成功、子の終了、業務処理の成功は分けて確認する。

2026-09-15の[4.0.7 release](https://github.com/ruby/ruby/releases/tag/v4.0.7)と、[不具合報告](https://bugs.ruby-lang.org/issues/22243)、版固定実装を2026-10-04 UTCに再確認した。以下はAPI契約・固定sourceの観察・独自の設計判断を区別する。Ruby runtime試験は実行していない。

全knowledge/pattern/moduleの横断検索で `ruby fork`、`Process._fork`、`at_fork`、`fork exception` は既存該当なし。既存の[Ruby SIGINT延期](sigint-interrupt-mask-cleanup-boundary.md)は同一process内の非同期例外の配送、[Linux pidfd autokill](../linux/pidfd-autokill-reference-exit-boundary.md)はOS参照の解放と終了通知を扱う。本稿はfork前後のRuby呼出経路とhook失敗の境界であり、両者の再説明ではない。件数順候補よりも、最近の安定版修正と未収録の子process逸脱リスクを優先した。

## 1. 確認した版と根拠

| 対象 | 確認内容 | 適用判断 |
|---|---|---|
| CRuby 4.0.7、2026-09-15公開 | release notesがBug #22243を掲載。tagは `229531a6cfbf07e3caef30dbac24a2a3f3fed482` | 本稿の修正後実装・回帰testをこの版へ固定 |
| CRuby 4.0.6、2026-07-14公開 | tagは `03b6d3f8898a28604fe6cb00eae3226b821168f4`。overrideしたhookを直接呼ぶ旧実装 | 直前版との間で修正差分を確認 |
| Ruby 4.0 RDoc | `Process._fork` / `Process.fork` / wait系の契約 | 系列表示でpatch版・個別更新日は非表示。修正の導入判定はRDocだけで行わない |
| Ruby 3.4 / 3.3、別実装、vendor build | 本調査では同じ経路を比較していない | 4.0.7の確認を全Ruby系列や独自backportへ一般化しない |

公開日は公式ニュースの[4.0.7](https://www.ruby-lang.org/en/news/2026/09/15/ruby-4-0-7-released/)と[4.0.6](https://www.ruby-lang.org/en/news/2026/07/14/ruby-4-0-6-released/)で照合した。issueの相対表示から初回報告日を逆算しない。release一覧に不具合が載ることと、全利用環境の安全性が実証されることも別である。

## 2. public forkとinternal hookの戻り値を混同しない

[Process RDoc](https://docs.ruby-lang.org/en/4.0/Process.html#method-c-_fork)は `_fork` を監視ライブラリ等のhook用internal APIとし、通常コードから直接呼ばないよう説明する。`Kernel#fork`、`Process.fork`、`IO.popen("-")` がこの経路を利用する。`Process.daemon` は内部でOSのforkを使う場合があっても、このhookを通らない。

固定[process.c](https://github.com/ruby/ruby/blob/229531a6cfbf07e3caef30dbac24a2a3f3fed482/process.c)で区別できる戻り値は次の通り。

- internal `_fork` の `super` は、親では子のPID、子では整数0を返す
- publicなblockなし `Process.fork` は、親では子のPID、子ではnilを返す
- blockありpublic forkは、子でblockを実行し、正常なblock終了では終了へ進む。親はblockを実行しない

したがってhook内部で `if pid.nil?` を子判定に使うのは誤りである。0はRubyでtruthyなので `if pid` も親子の区別にならない。public戻り値のnil判定を、internal hookへそのまま持ち込まない。hookが正常終了する場合は元の整数を保つ必要があり、ログ呼出等を末尾に置いて戻り値を変えると変換エラーの原因になる。

## 3. 旧版でblock開始前に漏れた理由

[Bug #22243](https://bugs.ruby-lang.org/issues/22243)の報告は、`super` の後、子側のhookで例外を上げる場合を扱う。外側に広い `rescue StandardError` を持つloopがあると、子がそのrescueへ入り、親向けloopを続け得た。報告中のgem名はhook利用例であり、本調査で各gemの全版を再現したという意味ではない。

[4.0.6のprocess.c](https://github.com/ruby/ruby/blob/03b6d3f8898a28604fe6cb00eae3226b821168f4/process.c)では、`rb_f_fork` がまず `rb_call_proc__fork()` を呼び、その後に「子か」「blockがあるか」を調べる。block本体を囲む `rb_protect(rb_yield, ...)` は後段にあるため、前段のhook例外を捕まえられない。block内部にrescueやensureを書くだけでも、このblock開始前の例外には届かない。

これは「block本体からの例外処理が新たに追加された」という修正ではない。block本体には既存の保護があり、今回追加された保護はその手前のhook呼出である。

## 4. 4.0.7の保護範囲と親子の分岐

[4.0.7のprocess.c](https://github.com/ruby/ruby/blob/229531a6cfbf07e3caef30dbac24a2a3f3fed482/process.c)の `rb_call_proc__fork` を読むと、処理は次のように分かれる。

1. `_fork` が基本定義のままなら、従来どおり `proc_fork_pid()` へ進む
2. overrideされていれば、呼出前の実PIDを `getpid()` で保存する
3. `call_proc__fork_protected` の中でhookを呼び、`NUM2PIDT` による戻り値変換まで行う。この二つを `rb_protect` の対象にする
4. 保護した呼出が失敗し、現在PIDが保存PIDと違えば、forkされた子として `ruby_stop(state)` へ進む
5. PIDが同じなら `rb_jump_tag(state)` で元の呼出側へ失敗を伝播する

子の判定はhookの返した整数ではなく、実PIDの比較で行う。戻り値の変換も保護範囲なので、hookが子で不正な型を返す場合も対象に含まれる、というのが固定実装からの帰結である。ただし、この不正戻り値の子側ケースのruntime試験は本調査では行っていない。

| 失敗位置 | 固定実装から読める帰結 | 誤って推測しないこと |
|---|---|---|
| hookが実fork前にraise | 同一PIDなので呼出側へ伝播 | 全てのhook失敗でprocessが終了するわけではない |
| `super` 後の親側hookがraise | 親PIDは変わらず例外が伝播 | 例外が出たから子は存在しない、とは言えない |
| `super` 後の子側hookがraise | 子PIDの相違を検出し終了経路へ | 親が同じ例外をrescueできるわけではない |
| hookが正常に戻り、後からblock本体でraise | 既存のblock本体の保護で処理 | hook失敗と同じ観測点・同じ回帰testではない |
| hook内部で例外を握りつぶす、または無限待機する | 保護呼出から失敗として戻らない | この修正が初期化成功や時間制限を検査するわけではない |

**親側hookの例外は、実際のfork成功後にも起こり得る。** そこで無条件に再forkすると、先に作られた子が働く一方で追加の子を作る可能性がある。これは固定分岐から導いた設計上の注意であり、特定のserver製品で重複処理が観測されたという主張ではない。

## 5. blockなしforkとIO.popenにも届くが、万能なhookではない

issueの題名はblock形式を強調しているが、追加された `rb_call_proc__fork` の保護にはblock有無の条件がない。`rb_f_fork` はblockを調べる前にその共通関数を呼ぶため、**blockなしforkも同じhook失敗処理を通る**。正常なblockなしforkで子がnilを受け取って継続する契約は変えていない。

固定[io.c](https://github.com/ruby/ruby/blob/229531a6cfbf07e3caef30dbac24a2a3f3fed482/io.c)でも、`IO.popen("-")` のfork側が同じ関数を呼ぶ。ここでは子のpipe redirectより前にhookを実行するため、hookの出力をpopen後の業務出力と同じ経路だと決めつけない。通常の外部commandを起動するpopenの全経路に同一のhook契約がある、とは拡張しない。

`Process._fork` を直接呼ぶコードはこの共通wrapperを経由する契約ではない。RDocの「直接呼ばない」を守り、今回の保護を利用するためにinternal APIへ乗り換えない。`Process.daemon`や全C拡張の子生成まで一括で覆うとも考えない。

## 6. ruby_stopはexit!でも最大停止時間でもない

固定[eval.c](https://github.com/ruby/ruby/blob/229531a6cfbf07e3caef30dbac24a2a3f3fed482/eval.c)で `ruby_stop` は `ruby_cleanup` の結果を使って終了する。cleanupはteardown・end procedure・finalizer等のVM終了処理を通る。一方、固定 `process.c` の `exit!` は `_exit` を使い、[RDoc](https://docs.ruby-lang.org/en/4.0/Process.html#method-c-exit-21)もexit handlerを呼ばない契約を示す。

したがって今回の子終了を「exit!で即時終了」「at_exitは動かない」「一定時間で強制終了」と説明しない。終了handlerやschedulerの終了処理が待つ場合も考慮が必要で、cleanup経路があることは終了期限の証明ではない。終了コードも任意のhook・SystemExit・終了handlerを含む全条件で一定だとは断定しない。

外側のrescueへ戻らないことと、アプリが望む全ensure・DB切断・telemetry送信が完了することも同義ではない。block未開始ならblock内に置いた解放処理はまだ実行準備されていない。親から継承した状態を終了handlerが参照する可能性を踏まえ、終了処理は実PIDや資源の所有者を識別する設計にする、というのが本稿の提案である。

## 7. 採用・運用の設計判断

以下は公式の必須手順ではなく、確認した契約からの独自提案である。

1. **hookを棚卸しする**。runtimeのpatch版に加え、監視・接続管理のどのライブラリが `_fork` をoverrideし、どの順序で `super` を呼ぶか確認する。例外を全部握りつぶして起動成功に見せず、初期化に必須の失敗と任意の診断の失敗を設計時に分ける
2. **起動を段階で観測する**。親のfork戻り値は子PIDの取得であり、子のhookや初期化が終わった通知ではない。必要なら子がhookと初期化を通過した後にreadinessを返し、親はその待機期限を別に持つ
3. **親側例外から即再実行しない**。実fork前の失敗と、fork後に親のhookが失敗した場合を区別できる記録を持つ。作成済み子の所在・状態が不明なら、重複起動や業務副作用を確認するまで盲目的なretryをしない
4. **終了を回収して評価する**。親は返された子PIDを記録し、必要なら `Process.waitpid2(pid)` で当該PIDとstatusを受け取る。`Process.wait`が戻ることと業務成功は別。一般的なforkではzombieを避けるためwaitまたはdetachが必要で、このpatchが自動回収を追加したわけではない
5. **終了責務を一つにする**。frameworkが既に子をreapするなら別経路で競合してwaitしない。結果が重要な仕事では、単に終了へ無関心になる `detach` だけで完了判定を済ませない。process成功statusも外部への副作用commitを証明しない
6. **非対応環境とthreadを分ける**。fork非対応platformでは `Process.respond_to?(:fork)` 等を確認する。fork後の子に残るRuby threadは呼出threadだけというAPI制約は本修正で変わらない。別threadが保持していた接続やライブラリ状態を安全に再利用できるとの保証にはしない

## 8. 上流testと追加確認の範囲

固定[test/ruby/test_process.rb](https://github.com/ruby/ruby/blob/229531a6cfbf07e3caef30dbac24a2a3f3fed482/test/ruby/test_process.rb)の `test__fork_raisig_after_fork` は、子側hookでraiseし、block出力と外側rescue出力が出ないこと、stderrに例外が現れることを検査する。名前の `raisig` は上流にある綴りである。`Process.respond_to?(:_fork)` が有効な場合に定義される。

このtestは当該子のstatusを `waitpid2` で直接assertする形ではなく、全終了handlerの完了や最大停止時間を証明するものでもない。隣接する既存testはblockなしのhook順序、popenのhook利用、不正型を返すhookに対する親側TypeErrorを検査するが、これらすべてが今回追加されたわけではない。4.0.6には新しい子側raise回帰testがないことも確認した。

実環境へ適用するときは、隔離した子processと監督processで少なくとも次を追加確認する。

- 正常hookで、internalの子0とpublicの子nilが保たれ、blockは子でのみ実行される
- 子hookのraiseでblock未実行・外側rescue未到達・子の終了statusが観測できる
- blockなしforkと `IO.popen("-")` でも、子hook失敗時に親向け処理へ進まない
- 親hookが `super` 後にraiseしても、作成済み子の管理を失わず重複実行しない
- 子側の不正戻り値、終了handlerあり、hook無限待機を分けて試す。監督側の期限で回収した結果を正常cleanup成功と数えない

上記は試験案であり、今回実行した試験結果ではない。端末上の任意processへsignalを送らず、自分が作成して追跡している試験processだけを回収する。

## 9. 未確認事項・provenance

- 本調査環境にRuby runtimeがなく、CRuby 4.0.6 / 4.0.7の実測、上流test実行、Puma・Sidekiq・監視gem等との統合、macOS/Windows差、C拡張や終了handlerの競合stressは未検証
- blockなしforkとpopenへの修正範囲、親側例外後の子存在、戻り値変換の保護は固定sourceからの分析であり、全呼出形の独立した回帰test結果ではない
- GitHubのPR・commit・blobページにはnative web取得失敗があった。release・issue・RDocの本文はnative webで開き、tag解決・固定コード・COPYING・LEGALはGitHub connectorのreadで補完した
- 4.0.6 / 4.0.7の同一commitにあるCOPYING・LEGALを確認し、分析したcore sourceはRuby License OR BSD-2-Clauseとしてcatalogに記録した。release・issue・RDoc本文の個別ライセンスはunknownとして扱う。全記述は独自要約・分析であり、上流コードやreproducerの移植、module昇格は行っていない
- source type `release_notes` のTTL 30日に合わせ明示期限を2026-11-03とした。再調査時は版とhook実装を読み直し、取得日だけを延ばさない

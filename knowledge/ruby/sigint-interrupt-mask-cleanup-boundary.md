---
{
  "id": "ruby-sigint-interrupt-mask-cleanup-boundary",
  "title": "Ruby SIGINT masking: 3.4.11の修正と延期・custom trap・資源解放の境界",
  "kind": "knowledge",
  "technology": "ruby",
  "version": "CRuby 3.4.11 (2026-09-23) / 4.0.6 (2026-07-14) にBug #22133の修正を確認。3.4.10 / 4.0.5の固定sourceと比較。Ruby 3.4 RDoc、2026-10-04取得",
  "tags": [
    "research-domain:backend",
    "ruby",
    "SIGINT",
    "Thread.handle_interrupt",
    "SignalException",
    "Interrupt",
    "pending_interrupt",
    "graceful-shutdown",
    "reentrancy",
    "cleanup"
  ],
  "sources": [
    {
      "id": "ruby-sigint-thread-api-3-4-20261004",
      "url": "https://docs.ruby-lang.org/en/3.4/Thread.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-sigint-signal-api-3-4-20261004",
      "url": "https://docs.ruby-lang.org/en/3.4/Signal.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-sigint-trap-caveats-3-4-20261004",
      "url": "https://docs.ruby-lang.org/en/3.4/signals_rdoc.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-sigint-release-3-4-11-20261004",
      "url": "https://github.com/ruby/ruby/releases/tag/v3_4_11",
      "type": "release_notes"
    },
    {
      "id": "ruby-sigint-release-4-0-6-20261004",
      "url": "https://github.com/ruby/ruby/releases/tag/v4.0.6",
      "type": "release_notes"
    },
    {
      "id": "ruby-sigint-report-22133-20261004",
      "url": "https://bugs.ruby-lang.org/issues/22133",
      "type": "incident_report"
    },
    {
      "id": "ruby-sigint-runtime-592f1ffd-20261004",
      "url": "https://github.com/ruby/ruby/tree/592f1ffdb36153e8be83603ade3c2e9ab6138a77",
      "type": "github_repository_analysis"
    },
    {
      "id": "ruby-sigint-before-2b0b7728-20261004",
      "url": "https://github.com/ruby/ruby/blob/2b0b7728dc7f0561c35c3d8c4489945c94b783ad/signal.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "ruby-sigint-runtime-03b6d3f8-20261004",
      "url": "https://github.com/ruby/ruby/blob/03b6d3f8898a28604fe6cb00eae3226b821168f4/signal.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "ruby-sigint-before-64336ffd-20261004",
      "url": "https://github.com/ruby/ruby/blob/64336ffd0ee9e1f4c05891695a3d7b49cb709721/signal.c",
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

# Ruby SIGINT masking: 3.4.11の修正と延期・custom trap・資源解放の境界

## 問いと採用判断

`Thread.handle_interrupt(SignalException => :never)` で資源の取得・解放を囲んでいても、Ctrl-Cで内部状態が壊れることがあるのはなぜか。Rubyのpatch更新後、既存のshutdown設計をどこまで信用できるか。

**CRuby 3.4.11は、Ruby既定handlerのSIGINTがmaskを迂回する不具合を修正した。変更は非同期例外の配送経路であり、あらゆる終了を防ぐ仕組みではない。** 修正版でも、maskの対象class・適用thread・再許可する場所・custom trapの有無を別々に確認する。長時間の`:never`は終了を遅らせるため、cleanupの保護と停止期限を一つの設定で済ませない。

既存の[Fiber schedulerのhookと終了](fiber-scheduler-nonblocking-io.md)、[Socket.tcpの接続成功](socket-connect-success-timeout-boundary.md)とは異なり、本稿は「非同期例外をいつ実行中のRubyコードへ届けるか」を扱う。全knowledge/pattern/moduleと直近16件のknowledge commitを確認し、`handle_interrupt`・`ruby cancellation`の検索は既存該当なし、`SIGINT`はGitHub ActionsとTerraformのみだった。件数優先候補よりも、2026-09-23の3.4系修正と未収録の資源解放リスクを優先して選定した。

以下の「確認事実」は公式RDoc・release・固定sourceの読解、「設計判断」はそれを基にした独自提案である。全資料の取得日は2026-10-04 UTC。Ruby runtime試験は実行していない。

## 1. 版を分ける: 3.4.11の反映と4.0系の先行修正

| 対象 | 確認した根拠 | 判断できる範囲 |
|---|---|---|
| CRuby 3.4.11、2026-09-23公開 | [release](https://github.com/ruby/ruby/releases/tag/v3_4_11)がBug #22133を掲載。[tag SHA](https://github.com/ruby/ruby/tree/592f1ffdb36153e8be83603ade3c2e9ab6138a77)で実装と回帰testも確認 | 3.4.11に修正が含まれる |
| CRuby 3.4.10 | [signal.c](https://github.com/ruby/ruby/blob/2b0b7728dc7f0561c35c3d8c4489945c94b783ad/signal.c)の既定SIGINTが`rb_interrupt()`を直接呼ぶ | 比較した直前tagは旧経路 |
| CRuby 4.0.6、2026-07-14公開 | [release](https://github.com/ruby/ruby/releases/tag/v4.0.6)が同じBugを掲載。[signal.c](https://github.com/ruby/ruby/blob/03b6d3f8898a28604fe6cb00eae3226b821168f4/signal.c)も新経路 | 4.0系では4.0.6ですでに修正されている。4.0.7の新機能ではない |
| CRuby 4.0.5 | [signal.c](https://github.com/ruby/ruby/blob/64336ffd0ee9e1f4c05891695a3d7b49cb709721/signal.c)が直接`rb_interrupt()`を呼ぶ | 比較した直前tagは旧経路 |
| Ruby 3.3 / Windows / 別実装 | [issue](https://bugs.ruby-lang.org/issues/22133)は3.3のbackportをREQUIREDと表示。固定回帰testはWindowsを除外 | 3.3への修正済み宣言、Windowsの同一動作、JRuby/TruffleRubyへの一律適用はしない |

隣接tagの比較から、ここで確認したupstream 3.4/4.0系列の切替点は3.4.11 / 4.0.6となる。ただしdistribution独自backport、未調査の古い版、後続版すべての動作まで証明したものではない。API名の存在や`RUBY_VERSION`のmajor/minorだけを修正判定に使わず、配布元patchと実際のhandlerも確認する。公開日は公式ニュースの[3.4.11](https://www.ruby-lang.org/en/news/2026/09/23/ruby-3-4-11-released/)・[4.0.6](https://www.ruby-lang.org/en/news/2026/07/14/ruby-4-0-6-released/)でも照合した。

## 2. 修正が変更したのはdefault SIGINTの配送

[Bug #22133](https://bugs.ruby-lang.org/issues/22133)では、Async/Async::Containerのsignal処理を調べる中で、Ruby既定handlerのSIGINTが`:never`を通り抜けることが報告された。`Thread::Queue#pop`の待機中でも保護区間の内側に`Interrupt`が発生し得る。これは「処理を中断しない間に同期状態を整える」という前提を壊す問題である。

3.4.11の固定sourceでは、次のように責務が分かれる。

1. [signal.cのrb_signal_exec](https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/signal.c)はRuby既定SIGINTについて、直接例外を上げず`rb_threadptr_interrupt_raise(th)`を呼ぶ
2. [thread.cの同関数](https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/thread.c)はmain threadを対象とし、従来のno-messageな`Interrupt`を作る。main threadの生存を確認し、pending interrupt queueに入れて対象threadへ割込みを通知する
3. pending queueから取り出す際に、対象threadのmaskを照合する。`:never`なら配送を保留し、再び許可された時点でRuby例外として発生させる

したがって例外classが`SignalException`から別物へ変わった修正ではない。もともとの`Interrupt`を、maskが参照される経路に通す変更である。signalを受けた瞬間と、Rubyコードで例外が発生する瞬間は同じとは限らない。

## 3. mask契約: class・thread・timingを同時に確認する

[Thread RDoc](https://docs.ruby-lang.org/en/3.4/Thread.html#method-c-handle_interrupt)と3.4.11の`thread.c`を照合すると、判断は次のようになる。

- **classの一致**: 指定classの派生classも対象。固定[error.c](https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/error.c)では`Interrupt < SignalException < Exception`であり、`StandardError`は別の分岐である。`StandardError => :never`だけではSIGINTのInterruptを保護しない
- **適用thread**: `handle_interrupt`は呼出中の現在threadのmask stackに設定を積む。default SIGINTの配送先はmain thread。worker thread内のmaskを、process全体やmain threadを保護するglobal設定と解釈しない
- **`:never`**: 対象の非同期例外を延期する。破棄・無視する契約ではない。外側に別のmaskがあればその設定も効くため、常に最内blockの終了直後に必ず発火する、と単純化しない
- **`:immediate`**: 内側のblockで対象classの延期を解除できる。pendingがあれば、内側blockを実行し始める境界から例外が発生し得るものとしてcleanupを準備する
- **`:on_blocking`**: CRubyの説明ではGVLを解放して実行する操作が対象。任意の「遅い処理」やCPU loopに自動で期限を付ける機能ではなく、wall-clock timeoutにもならない
- **同期例外**: その場の通常の`raise`や処理失敗まで`:never`が封じると解釈しない。APIの対象は非同期interruptの配送時機である
- **kill/terminate**: RDocは`Exception`ではThreadのkill/terminate interruptを扱えず、すべてのinterrupt classを扱う例では`Object`を使うと説明する。ただし`Object => :never`を全終了耐性へ一般化してはいけない。これはRuby内部のinterrupt照合であり、SIGKILL・VM crash等を防ぐ契約ではない

[Thread.pending_interrupt?](https://docs.ruby-lang.org/en/3.4/Thread.html#method-c-pending_interrupt-3F)は延期された例外の有無を観測する。固定実装はqueueの空否・class該当性を調べるだけで、**読むだけではqueueを消費しない**。`false`を「この後もsignalが来ない」という予約保証にせず、`true`をcleanup完了の証拠にも使わない。

## 4. DEFAULT・SYSTEM_DEFAULT・custom trapは別経路

[Signal.trapの契約](https://docs.ruby-lang.org/en/3.4/Signal.html#method-c-trap)では、`DEFAULT` / `SIG_DFL`はRubyの既定handler、`SYSTEM_DEFAULT`はOSの既定handlerである。名前の近さから同一視しない。`IGNORE`はsignalを無視し、custom blockは登録コードを実行する。今回の修正対象はRuby既定handlerから作られるSIGINT例外である。

特に`Signal.trap('INT') { raise Interrupt }`を既定handlerと等価と扱わない。3.4.11の固定`signal.c`はcustom callbackを`signal_exec`で実行し、そのcallbackから出た例外を`EC_JUMP_TAG`で戻す。既定SIGINT用のpending queue経路とは別であり、**custom callback内のraiseをmaskが延期する保証へは拡張できない**。RDocにsignal trap未対応の注意が残っていても、default SIGINT修正と区別して読む必要がある。

また[signal callbackの注意事項](https://docs.ruby-lang.org/en/3.4/signals_rdoc.html)では、callbackをRuby VMの内部状態が安全な時点まで遅らせる一方、アプリ側の不変条件まではVMが判断できないとされる。thread-safeであることとreentrantであることは違い、`Mutex#lock` / `Mutex#synchronize` / それらを使う`Monitor`はtrap中で安全ではない。更新したRubyであっても、trap内へDB transaction・通常logger・pool返却などをまとめて移す理由にはならない。

独自の設計判断として、custom trapが必要なら、文書で安全な操作に挙げられる`Thread::Queue#push`などで停止要求を渡し、通常の実行経路で受付停止・drain・cleanupを行う。queueを使っただけでアプリ全体の停止が完了するわけではなく、消費側が停止要求を処理する条件と残作業の完了確認も設計する。frameworkが既にtrapを登録している場合は、上書き前にその契約を確認する。

## 5. 資源解放と停止期限を分離する設計判断

以下は公式APIの必須手順ではなく、上記契約からの独自提案である。

1. **実際のsignal経路を先に分類する**。terminal、service manager、test runnerからのSIGINT/SIGTERMが、Ruby DEFAULT・framework custom trap・OS SYSTEM_DEFAULTのどれに届くかを記録する。ライブラリが勝手にprocess全体のtrapを置き換える設計は避ける
2. **短い所有権遷移だけをmaskする**。対象classを絞った外側`:never`の中で資源の取得と所有者記録を行い、解放用`ensure`を準備する。中断可能な本処理は内側`:immediate`、解放は外側maskの内側で行う。ensureをmaskの外へ置くと、延期解除時に次のinterruptへ露出する区間が生じる
3. **取得失敗は別に処理する**。maskは同期例外や半端な初期化を成功に変えない。取得済み資源だけを解放できるようにし、処理完了・解放完了・終了例外を別々に記録する。cleanup自体が失敗した場合の原因も握りつぶさない
4. **`:never`の下で無期限に待たない**。SIGINTが実際に延期されるようになると、従来は偶然Ctrl-Cで抜けていた待機が継続し得る。外側maskの中でqueue待ちやI/Oを行うなら、解除条件と終了予算を別に決める。`:on_blocking`への置換だけではCPU loopの停止期限を満たせない
5. **shutdown要求と業務commitを混同しない**。interrupt受領後も完了させるローカル不変条件を定義する。DB commit・外部送信が成功したかはそのAPIの結果と再照合で判定し、maskを副作用のrollbackやexactly-once保証として使わない
6. **強制終了への別の回復経路を持つ**。SIGKILL、process crash、host喪失等に対する永続化・冪等性・再起動回復は本修正の外にある。mask範囲を`Object`まで広げるだけで運用上の安全性を得たと判定しない

## 6. 上流の回帰testが示すこと・示さないこと

3.4.11の[test_handle_interrupt_masks_sigint](https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/test/ruby/test_thread.rb)は子Rubyプロセス内で二つのQueueを使う。補助threadが待合せ後に自身のprocessへSIGINTを送り、解放用queueを進める。main threadの`:never`区間内で`Interrupt`をrescueしたかを記録し、外側で受けたことを示す`outer`と、内側では受けていない`false`を期待する。

重要な範囲制限がある。

- `mswin` / `mingw`では「SIGINT handling differs on Windows」を理由にomitする。Unix向け回帰testの存在をWindowsの保証にしない
- `outer false`の期待は、このdefault-handler経路の配送位置を検査するもの。DB cleanupの成功、全I/Oの中断安全性、最大停止時間はassertしていない
- testがrepositoryに存在することと、今回の環境でそのtestを実行したことは別。本調査ではRuby実行環境がなく、読解のみである

採用時の追加試験案は、対象OS/runtimeを固定し、必ず隔離した子processを使う。対象外processへsignalを送らない。default SIGINTを取得前・所有者記録後・本処理中・cleanup中に届け、例外が出た位置と解放回数を観測する。custom trap版、連続SIGINT、`:on_blocking`でのCPU/I/O差、main/workerのmask差も分ける。停止不能なケースは親process側の期限で回収し、回収された結果を正常cleanupとして数えない。

## 7. 適用限界・未確認事項・provenance

- CRubyの固定sourceと公開releaseの比較であり、Ruby 3.4.11 / 4.0.6のruntime試験、Windows試験、Async / Rails / Puma等との統合試験、連続signalやC拡張の競合stress試験は未実行
- 3.3の未反映表示は取得時点のissue情報である。独自distributionのbackport有無を断定しない。JRuby/TruffleRubyのsignal既定動作もこのCRuby修正から導かない
- `Thread.handle_interrupt`は停止時機を制御するAPIであり、任意処理のthread-safety、排他、transaction atomicity、processの生存、特定時間内の終了を保証しない
- RDocは3.4の系列表示で、patch版・個別公開日は示されない。版差の確定にはfull SHAのsourceを用いた。issueの初回公開日も相対表示から推測していない
- GitHub commitページのnative web取得は失敗したものがあり、tag解決・固定source・COPYING・LEGALはGitHub connectorのreadで補完した。公式release/RDoc/issueはnative webで本文を開いた
- 固定sourceは各commitのCOPYING / LEGALを確認し、Ruby License OR BSD-2-Clauseとして記録した。本稿は独自要約・分析で、上流コードの複製やmodule昇格は行わない。release/RDoc/issue本文の個別ライセンスはunknownとし、引用コードを移植しない
- source type `release_notes`のTTL 30日に合わせ、明示期限は2026-11-03。日付だけを更新せず、対象branchのrelease、実装、回帰条件を再確認する

---
{
  "id": "ruby-io-buffer-short-transfer-migration-boundary",
  "title": "Ruby 4.1 preview IO::Buffer: 引数順・short transfer・scheduler待機責務の移行境界",
  "kind": "knowledge",
  "technology": "ruby",
  "version": "Ruby 4.1.0-preview1 (2026-10-12) @ 2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1; compared with Ruby 4.0 RDoc (patch unspecified); verified 2026-10-23 UTC; Ruby runtime未実行",
  "tags": [
    "research-domain:backend",
    "ruby",
    "IO::Buffer",
    "Fiber::Scheduler",
    "io_read",
    "io_write",
    "io_pread",
    "io_pwrite",
    "short-transfer",
    "offset",
    "length",
    "EAGAIN",
    "migration"
  ],
  "sources": [
    {
      "id": "ruby-io-buffer-preview1-release-20261023",
      "url": "https://www.ruby-lang.org/en/news/2026/10/12/ruby-4-1-0-preview1-released/",
      "type": "release_notes"
    },
    {
      "id": "ruby-io-buffer-api-4-0-20261023",
      "url": "https://docs.ruby-lang.org/en/4.0/IO/Buffer.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-io-buffer-scheduler-api-4-0-20261023",
      "url": "https://docs.ruby-lang.org/en/4.0/Fiber/Scheduler.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-io-buffer-preview1-source-2d08b5f4-20261023",
      "url": "https://github.com/ruby/ruby/tree/2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Ruby 4.1 preview IO::Buffer: 引数順・short transfer・scheduler待機責務の移行境界

## 問いと結論

Ruby 4.0 向けの buffer adapter / Fiber scheduler を Ruby 4.1.0-preview1 で試すとき、引数の入替えだけで互換になるか。

**ならない。** `read` / `write` / `pread` / `pwrite` は `(length, offset)` から `(offset, length)` へ変わるだけでなく、length が最低転送量から最大転送量になり、short transfer をそのまま返す。さらに明示的な zero length は「一度試す」ではなく no-op になる。scheduler hook も同じ契約へ変わり、待機・再試行と完全転送の責務を呼出側で整理する必要がある。

この文書は移行のための原典読解であり、Ruby の実行検証ではない。既存の [Ruby 3.4 Fiber scheduler 概説](fiber-scheduler-nonblocking-io.md) は hook 群・設定・終了を扱うがシグネチャを対象外としている。本書は具体的な IO 引数と転送量の互換性に限定する。

## 適用版と根拠の境界

- [公式リリース発表](https://www.ruby-lang.org/en/news/2026/10/12/ruby-4-1-0-preview1-released/) は **2026-10-12 公開の Ruby 4.1.0-preview1**。安定版・最新版として推奨する資料ではない。発表は4メソッドの最大長・単一操作化と、対応する fiber scheduler hooks の experimental 扱い終了を明記する。
- 旧契約は [Ruby 4.0 IO::Buffer RDoc](https://docs.ruby-lang.org/en/4.0/IO/Buffer.html) と [Ruby 4.0 Fiber::Scheduler RDoc](https://docs.ruby-lang.org/en/4.0/Fiber/Scheduler.html)。patch版・ページ公開日は表示されていないため補わない。
- 新契約の細部は公式 tag **`v4.1.0-preview1`** を解決した commit **`2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1`** の source / RDoc コメント / tests を確認した。commit日時は2026-10-11 UTC、リリース公表日とは別である。`v4_1_0_preview1` と推測して参照しない。
- `/en/master/` は moving master であり、表示が Ruby 4.1 でも preview1 と同一snapshotとは限らない。本書の preview1 の断定は master RDoc ではなく固定commitによる。Storage / Slice 等の別変更を本テーマへ追加しない。
- 以下の「確認事実」は公式記載または固定sourceの読解、「設計案」は本書独自の推奨。2026-10-23 UTCに取得し、release notes の30日TTLに合わせ2026-11-22を再確認期限とする。

## 確認事実: 位置引数と転送量は別々に移行する

旧4.0 RDocとpreview1の `io_buffer.c` を比較した契約は次のとおり。`from` はファイル上の位置、`offset` はbuffer内の位置であり別の座標である。

| 操作 | Ruby 4.0 | Ruby 4.1.0-preview1 |
|---|---|---|
| read / write | `(io, [length, [offset]])` | `(io, [offset, [length]])` |
| pread / pwrite | `(io, from, [length, [offset]])` | `(io, from, [offset, [length]])` |
| length の意味 | 最低転送量。scheduler RDocは残容量までの上振れを説明する | 1操作で転送する最大量。正のshort resultも正常 |
| 省略時 | offsetは0、lengthはbuffer残容量 | offsetは0、lengthはbuffer残容量。ただし完全転送の約束はない |
| 明示length=0 | 1回のIO操作を要求する | 転送せず0を返す |

固定sourceの `io_buffer_extract_offset_length` では `nil` も省略扱いになる。`io_buffer_validate_range` は `offset + length` のoverflowとbuffer範囲を検査する。**zero lengthでも引数・buffer検証を省略できるわけではない。** IO変換や範囲・読込先の書込可否の確認は0の早期returnより前にある。有効なzero-length呼出しではscheduler hookや転送syscallへ到達しない。

位置引数の個数は変わらないため、旧コードは ArgumentError になるとは限らず、意図と異なる場所・量で成功し得る。例えば12-byte bufferに対する `buffer.read(io, 4)` は、旧版ではoffset0で最低4-byteという指定、新版ではoffset4で最大8-byteという指定になる。**arity検査だけでは移行漏れを検出できない。**

### 移行時に区別する3つの意図

以下は独自の呼出例であり、実行結果の報告ではない。

1. buffer位置2から最大5-byteを書きたい: preview1では `buffer.write(io, 2, 5)`。旧 `buffer.write(io, 5, 2)` の引数を交換しても、「最低5-byteまで進める」という旧意味は保存されない。完全転送が必要なら別途ループを置く。
2. buffer位置2から残容量の範囲で一度だけ読みたい: preview1では `buffer.read(io, 2)`、または最大量を明示する。旧 `buffer.read(io, 0, 2)` を単純交換して `buffer.read(io, 2, 0)` にすると何も読まない。
3. file位置100からbuffer位置2へ最大5-byteを読みたい: preview1では `buffer.pread(io, 100, 2, 5)`。`from` とbuffer offsetを交換しない。pread / pwrite はIOの現在位置を変更しないというsourceコメントと上流testを確認した。

引数を全省略した `buffer.read(io)` / `buffer.write(io)` も、preview1ではbuffer全体を一度に完了する保証がない。文字列の `IO#read` 等の公開API全体がこの引数順やshort-return契約へ変わった、という意味でもない。

## 確認事実: scheduler hook は「試行」と「待機」を分ける

preview1 の [scheduler.c](https://github.com/ruby/ruby/blob/2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1/scheduler.c) のRDocコメントと引数配列で確認したRuby側hookは次の4つ。

- `io_read(io, buffer, offset, length)`
- `io_write(io, buffer, offset, length)`
- `io_pread(io, buffer, from, offset, length)`
- `io_pwrite(io, buffer, from, offset, length)`

4.0のhookではlengthがoffsetより先で、最低量に達するための非ブロッキング試行と `io_wait` をhook内で組み合わせる説明だった。preview1では最大lengthまでの1回の非ブロッキング操作を行い、short resultはそのまま返す。IOが未準備なら **`-EAGAIN` を返し、呼出側が待機・retryを判断する**。zero lengthは0、エラー結果は負のerrnoという値の契約で、`nil` / `false` / `:wait_readable` を代用品としない。

固定 `io_buffer.c` はschedulerが結果を返した場合、その値を直接返す。IO hookが未実装なら `scheduler.c` は `RUBY_Qundef` を返し、buffer側はfallbackへ進む。したがって「hookがないため失敗する」と一律には言えないし、fallbackを常にノンブロッキングと見なしてもいけない。schedulerのないbuffer経路ではfdに対して `read` / `write` / `pread` / `pwrite` を行う。single-operationとはshort transferを満たすまで内部で反復しない契約であり、blocking fdが待たないという保証ではない。

[同commitのtoy scheduler](https://github.com/ruby/ruby/blob/2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1/test/fiber/scheduler.rb) の `IOBufferScheduler` はfdをnonblockにし、`Fiber.blocking` 内からbufferを呼ぶ形になっている。これは再度同じhookへ入らないための参考になるが、テスト用実装のsource読解であり、本番向けの採用推奨・性能保証ではない。

### 同時に確認すべき2つの境界

- **hooksのexperimental終了 ≠ IO::Buffer全体の安定化。** preview1 の `io_buffer.c` にはクラスとRuby/C interfaceがexperimentalである旨の警告・コメントが残る。さらにpreview releaseであるため、最終版にそのまま固定されるとは断定しない。
- **IO hookの任意性 ≠ schedulerのinstall条件。** preview1では `fiber_interrupt` が必須となり、`Fiber.set_scheduler` の検証で未実装なら拒否される。IO転送の修正後でも、既存schedulerがinstallできるかは別に確認する。

C拡張も [buffer header](https://github.com/ruby/ruby/blob/2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1/include/ruby/io/buffer.h) と [scheduler header](https://github.com/ruby/ruby/blob/2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1/include/ruby/fiber/scheduler.h) の対応する `offset, length` を確認する。両方とも `size_t` なので、順番の誤りを型エラーだけに頼って発見する設計は不十分である。Ruby側hookとC APIでは `from` / `buffer` の並びも同一ではないため、Rubyの引数配列を機械的にCへ転記しない。

## 設計案: 完全転送は明示的な上位操作にする

以下はbyte stream / 通常ファイルを対象とする独自設計案。datagramのメッセージ境界や端末固有のzero-byte挙動を一般化するものではない。

- adapter内部では `buffer_offset`、`file_offset`、`maximum`、`completed` を別の名前にする。Ruby版別の位置引数変換を一箇所へ閉じ込める。旧版の最低量と新版の最大量を、同じ意味の `length` として隠さない。
- 「最大N-byteを一度試す」と「N-byteを完全に読む/書く」を別の操作として扱う。完全転送では残量と実際の正の戻り値を使って進捗を更新する。例: 最大5を要求して2が返れば残量は3、次のbuffer位置は元の位置+2であり+5ではない。
- pread / pwrite の反復では **buffer offsetとfile offsetの両方** を `completed` 分進める。file位置が不変のまま反復すると同じ位置を再読込・上書きする。現在の `io.pos` が動かないことは、自動的に次のfile位置へ進むことを意味しない。
- 正のshort readをEOFと判定しない。対象がbyte stream / 通常ファイルで、正の残量を要求して0が返った場合の終了扱いと、明示length=0のno-opを区別する。write側の進捗0は無限retryせずno-progressとして停止・報告する方針を定める。
- `-EAGAIN` 等の一時的エラーでは進捗を増やさず、readiness待機後に残量だけretryする。待機とretryには共通のdeadline / cancellationを適用し、busy loopにしない。他の負errnoや例外を一律retryしない。
- 途中まで書いた後の失敗はrollbackではない。既に確定した `completed` を保持し、同じstreamへ元buffer全体を再送しない。再接続・業務要求の再送には別のprotocol設計が必要で、低水準byte数だけではその成否を決められない。

この設計案は4.0と4.1にまたがる完全な互換adapter実装ではない。特に旧版の上振れを残して互換とするか、新版の最大量へ制限するかはprotocolの意図に基づいて決める。version文字列の単純な辞書順比較や同じarityだけで対応版を判定しない。

## 上流testの観察と移行時の確認表

[固定commitの `test/ruby/test_io_buffer.rb`](https://github.com/ruby/ruby/blob/2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1/test/ruby/test_io_buffer.rb) では以下のassertionを読んだ。**上流にtestが存在するという証拠であり、この調査で成功を実測した証拠ではない。**

- `test_read_returns_short_result`: nonblocking pipeに3-byteを用意し、最大5-byteのreadが3を返すことをassertする。
- `test_read_with_offset`: offsetだけを指定したreadがそのbuffer位置へ書く。
- `test_zero_length_io`: 空bufferとbuffer末尾の有効範囲を含め、4操作のlength0が0を返すことをassertする。
- `test_pread_offset` / `test_pwrite_offset`: buffer/file位置と `io.tell` が変わらないことを確認する。
- [fiber側test](https://github.com/ruby/ruby/blob/2d08b5f42f2cbed4a4e6ecf8d62987a4f21f98c1/test/fiber/test_io_buffer.rb) はschedulerありの非対称なbuffer offset / lengthと、pread / pwrite の `from` を検査する。環境条件でskipする経路もある。

自作adapterの検証では以下を追加する（未実行のtest計画）。

| ケース | 検出したい移行漏れ |
|---|---|
| offset2・最大5、前後にsentinel bytes | offsetとlengthの取り違え、範囲外の変更 |
| 省略length、明示0、nil、末尾offset | デフォルトとno-opの混同、zero lengthで検証を省略する誤り |
| 5要求→2成功→3成功 | 要求量で進めて欠落する、short resultを失敗にする |
| short成功→EAGAIN→readiness→残量成功 | hook内部の旧待機責務、busy loop、転送済みbyteの重複 |
| 正の残量でread0 / write0 | EOFとno-opの混同、無限loop |
| pread / pwriteでfile100・buffer2から部分転送 | file offsetかbuffer offsetだけを進める誤り |
| 途中成功→非再試行エラー / cancellation | 成功済みprefixを失い全体を再送する誤り |
| schedulerなし・あり・IO hookなし | fallbackの混同、再帰hook、経路依存のsignature漏れ |
| `fiber_interrupt` 未実装のscheduler | transfer test前のinstall失敗を見落とす |

## 限界・provenance

- Ruby runtime試験は実行していない。preview1のビルド、上流test、自作adapter、実socketのbackpressure、Windows / macOSでの挙動、Asyncやio-event gemとの互換性・性能は未検証。source読解や検索evalの成功をruntime成功へ読み替えない。
- native webでreleaseと4.0 RDocを開いた。preview commitのGitHub/rawページ取得はcache missとなったため、公式repositoryのtagをGit cloneし、HEADをfull SHAで確認して読解した。取得失敗をsource不在やrelease未確認の意味へ拡大しない。
- 同commitの `COPYING` / `BSDL` / `LEGAL` と対象file headerを確認した。Ruby sourceはRuby LicenseまたはBSD-2-Clauseとして識別し、該当fileを別条件とするLEGAL記載は見つからなかった。ニュース本文のlicenseはunknown、RDoc本文固有のlicenseも独立には確定していないため、独自要約とAPI識別子だけを記録する。実装・上流testコードは転載していない。
- source snapshotの `NEWS.md` / `io_buffer.c` / `scheduler.c` / 関連header / testsを相互確認したが、全IO API・全C ABI・全schedulerの移行を監査したものではない。最終4.1 releaseへ適用する前には、そのreleaseに固定して再確認する。

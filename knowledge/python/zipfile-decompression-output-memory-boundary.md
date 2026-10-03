---
{
  "id": "python-zipfile-decompression-output-memory-boundary",
  "title": "Python 3.14.8 zipfile: 展開出力の上限・旧codec互換・資源予算の境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "CPython v3.14.8 (2026-09-30) @ 8e6e75d9102e39bed2a2b279203a396741180f12; Python 3.14.8 documentation verified 2026-10-03 UTC; runtime untested",
  "tags": [
    "research-domain:backend",
    "python",
    "zipfile",
    "ZipExtFile",
    "decompression",
    "max_length",
    "needs_input",
    "memory",
    "resource-budget",
    "CVE-2026-15310",
    "monkey-patching"
  ],
  "sources": [
    {
      "id": "python-zipfile-release-3148-20261003",
      "url": "https://www.python.org/downloads/release/python-3148/",
      "type": "release_notes"
    },
    {
      "id": "python-zipfile-api-3148-20261003",
      "url": "https://docs.python.org/3.14/library/zipfile.html",
      "type": "official_docs"
    },
    {
      "id": "python-bz2-output-bound-3148-20261003",
      "url": "https://docs.python.org/3.14/library/bz2.html",
      "type": "official_docs"
    },
    {
      "id": "python-lzma-memory-output-bound-3148-20261003",
      "url": "https://docs.python.org/3.14/library/lzma.html",
      "type": "official_docs"
    },
    {
      "id": "python-zstd-output-bound-3148-20261003",
      "url": "https://docs.python.org/3.14/library/compression.zstd.html",
      "type": "official_docs"
    },
    {
      "id": "cpython-zipfile-bounded-3148-8e6e75d9-20261003",
      "url": "https://github.com/python/cpython/tree/8e6e75d9102e39bed2a2b279203a396741180f12",
      "type": "github_repository_analysis"
    },
    {
      "id": "cpython-zipfile-bound-fix-31980e84-20261003",
      "url": "https://github.com/python/cpython/commit/31980e84b9a708424a0a1dfecde3fc991e313f89",
      "type": "github_repository_analysis"
    },
    {
      "id": "cpython-zipfile-compat-9d167992-20261003",
      "url": "https://github.com/python/cpython/commit/9d167992b59cf5e23c66b9ed742b13f5925f7d70",
      "type": "github_repository_analysis"
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

# Python 3.14.8 zipfile: 展開出力の上限・旧codec互換・資源予算の境界

## 問いと結論

利用者がアップロードした ZIP を `ZipFile.open(...).read(8192)` で少しずつ処理すれば、
一時的な大量メモリ消費と archive 全体の展開量を制限できるか。
Python を更新した後も、追加の圧縮方式を提供するライブラリを同じ条件で信用できるか。

結論は次の三段階になる。

1. **標準codecの修正を取り込む。** 2026-09-30公開の Python 3.14.8 は、
   bzip2 / LZMA / Zstandard の小分けreadでも1回の展開が無制限になり得た
   CVE-2026-15310 / gh-156002 の修正を収録する。
2. **実際のdecompressorを確認する。** 同版には旧third-party codecとの互換fallbackもある。
   読めるようになったことと出力制限が機能することは別であり、公式NEWSにも残存リスクが明記される。
3. **アプリケーションの予算を別に設ける。** `max_length` はdecoder呼出しの出力上限であり、
   RSS上限、全member合計、ディスク、処理時間、同時実行数の制限ではない。

対象は上流CPythonの **v3.14.8** と、その版を表示する公式文書である。
配布元が同じバージョン文字列へ独自patchを適用している場合、実体を追加確認する。
他のPython系列の最初の修正版や全影響版は本稿では確定していない。
既存のtarfile文書が扱うfilter・filesystemへの部分展開とは別に、
本稿はZIP読取りのdecoderと資源消費に焦点を置く。
根拠は [3.14.8 release](https://www.python.org/downloads/release/python-3148/)、
[版固定NEWS](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Misc/NEWS.d/3.14.8.rst)。

## 確認した変更と適用版

| 記録 | 確認した事実 | 運用上の意味 |
|---|---|---|
| 2026-09-03、`31980e84b9a708424a0a1dfecde3fc991e313f89` | 3.14 branchへ非DEFLATEの出力制限をbackport | `read(n)` の戻り値だけを小さくする対処では足りなかった |
| 2026-09-16、`9d167992b59cf5e23c66b9ed742b13f5925f7d70` | monkey-patched decompressorとの互換処理を追加 | 新しい引数と属性に対応しないcodecでは制限を失い得る |
| 2026-09-30、v3.14.8 | release pageがCVE修正を掲載し、release treeに両方の変更を確認 | 最初の修正diffだけでなく、配布snapshotの実装を読む |
| v3.14.8 tagの実体 | `8e6e75d9102e39bed2a2b279203a396741180f12` | 日々変わる3.14 branchと区別して検証する |

日時はrelease page、公式PR、commit metadataで確認した。
[出力制限patch](https://github.com/python/cpython/commit/31980e84b9a708424a0a1dfecde3fc991e313f89)、
[互換修正patch](https://github.com/python/cpython/commit/9d167992b59cf5e23c66b9ed742b13f5925f7d70)、
[release snapshot](https://github.com/python/cpython/tree/8e6e75d9102e39bed2a2b279203a396741180f12)を根拠とする。
Issueのopen/closed表示やPR名だけを修正版判定に使わない。

ZstandardのZIP対応はPython 3.14追加である。公式APIはmethod ID 20と93を読み、
書込みは93を使うと説明する。bzip2とLZMAは以前からある方式なので、
「Zstandardを使っていないから今回の確認は不要」とは判断できない。
[zipfile API](https://docs.python.org/3.14/library/zipfile.html)

## 修正前: 小さい戻り値の前に大きいallocationが起きる

公式patchの説明では、従来DEFLATEにはdecoderへ渡す出力制限があった一方、
bzip2 / LZMA / Zstandardでは `decompress(data)` を上限なしで呼んでいた。
その結果から後で `data[:self._left]` を切り出しても、
大きい展開結果を作った時点のallocationを取り消せない。

この問題は、callerが小さい正数の `read(n)` を使っている場合にも起こる。
「要求8192bytesだから、内部にも8192bytes以上は存在しない」という推論は成立しない。
ZIP全体を一度に読む誤用だけが対象だったわけではない。
ここでは攻撃用archive、巨大allocationの再現コード、被害規模の実測は作成しない。
[patchの問題説明と修正](https://github.com/python/cpython/commit/31980e84b9a708424a0a1dfecde3fc991e313f89)

## 修正後: 出力を制限し、残ったdecoder入力をdrainする

以下はv3.14.8の `Lib/zipfile/__init__.py` を読んだ実装上の観察であり、
private methodの将来互換APIを提案するものではない。

### 1. read(n)と内部_read1(n)を分ける

標準の非DEFLATE経路は、decoderの第2引数へ
`max(n, self.MIN_READ_SIZE)` を渡す。同snapshotの `MIN_READ_SIZE` は **4096** である。

- 公開 `read(100)` は最大100bytesを返すが、内部の `_read1(100)` では
  最大4096bytesの展開結果を作り、余剰をread bufferに保持し得る
- 大きい `n` を渡せば、その大きさも出力要求へ反映される。4096は全呼出しの固定上限ではない
- `ZipFile.read(name)` はmember全体を返すため、固定サイズstreamingの代わりにはならない
- `ZipExtFile.read()` の省略・負数・NoneはEOFまで読み集める経路である。
  patch後も、その結果を全てメモリに保持する契約は残る

### 2. 圧縮入力を読み切った時点と出力終了は一致しない

bzip2 / LZMA / Zstandardのincremental decoderは、出力を上限で止めると、
まだ処理できる入力を内部に保持し得る。

- `needs_input=False` なら、新しい圧縮bytesを追加せず、`b''` を渡して残りを取り出す
- `needs_input=True` なら、追加の圧縮入力が必要になる
- zipfileは `_compress_left <= 0` だけで終了とせず、decoderの `eof` や
  `needs_input` も使って判定する。その後にmetadata由来の `_left` とCRCを処理する

したがって、独自wrapperが出力制限だけを追加し、drainとEOFの扱いを維持しなければ、
途中で切れるなど別の不具合を招く。LZMA用のzipfile wrapperも
`max_length` を下位decoderへ渡し、headerを集めている間は
`needs_input=True` とする実装になっている。

この出力上限と空入力drainの意味は、
[BZ2Decompressor](https://docs.python.org/3.14/library/bz2.html#bz2.BZ2Decompressor)、
[LZMADecompressor](https://docs.python.org/3.14/library/lzma.html#lzma.LZMADecompressor)、
[ZstdDecompressor](https://docs.python.org/3.14/library/compression.zstd.html#compression.zstd.ZstdDecompressor)
の公開契約でも確認した。
実装根拠は [版固定zipfile](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/zipfile/__init__.py)。

## third-party互換fallbackは制限の保証ではない

一部のライブラリはprivateな `_get_decompressor()` をmonkey-patchして、
標準外の圧縮方式を追加する。v3.14.8には次の互換処理がある。

- `needs_input` 属性がないときは、`getattr(..., True)` により入力が必要とみなす
- 2引数の `decompress(data, bound)` が `TypeError` を送出すると、
  1引数の `decompress(data)` へfallbackする

公式の版固定NEWSは、`needs_input` と2引数 `decompress()` を備えないdecompressorが
CVE-2026-15310に対して脆弱なままであると明記する。
このwarningは「全てのthird-party libraryが脆弱」という判定ではない。
実際のcodec実装、patch方法、利用経路を個別に調べる必要がある。

**独自の運用判断:** import後の読取りが成功したこと、Pythonのバージョン番号、
あるいは属性が存在することだけを安全判定にしない。
追加codecを必須にしない取り込み経路なら、承認した標準方式だけを受け入れる設計を選べる。
必須の場合は上限を実際に守ること、drainで内容が欠落しないこと、例外時の動作を検証し、
確認できるまで未検証codecを信頼された処理経路へ入れない。
標準の圧縮方式名でも置換されていれば標準実装の保証をそのまま引き継げない。

また、この `except TypeError` は署名不一致だけを型として区別する検査ではない。
「TypeErrorだから出力制限を外しても安全」とアプリケーション側へ模倣しない。
根拠は [互換修正](https://github.com/python/cpython/commit/9d167992b59cf5e23c66b9ed742b13f5925f7d70)と
[release NEWSのLibrary節](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Misc/NEWS.d/3.14.8.rst)。

## アプリケーションが別に持つ資源予算

次は上記契約から導く**独自の設計案**であり、zipfileが自動的に提供する機能一覧ではない。

| 境界 | 対策案 | 混同しないもの |
|---|---|---|
| 圧縮archiveの受信 | uploadの圧縮bytes上限を入口に置く | 小さい圧縮サイズは小さい展開量を保証しない |
| metadata読取り | constructorも資源制限したworker内で行い、許容member数を確認する | `infolist()` 後の件数検査だけでは、それ以前のallocationを止めない |
| 1回のdecoder出力 | 修正版の標準codecと有限の正数 `read(n)` を使う | 戻り値サイズはdecoder内部メモリやRSSの上限ではない |
| 単一member・全member合計 | 実際に受け取ったbytesを数え、残予算超過を保存前に拒否する | headerの `file_size` は利用者側の運用予算ではない |
| 保存先 | jobごとの保存容量上限と途中出力の破棄方針を設ける | streamingでも全体のディスク使用量は増える |
| 実行時間・CPU | 隔離workerのdeadline・CPU制約と取消後の終了確認を設ける | chunk間の時刻検査だけでは1回のdecoder呼出しを強制停止できない |
| 同時実行 | worker数・待ち行列・tenant単位の予算を別に制限する | 1jobの上限だけではサービス全体の最大消費量は決まらない |

### metadata確認もconstructorの後になる

同snapshotの `ZipFile(..., "r")` はconstructorから `_RealGetContents()` を呼び、
central directoryを読み、各entryの `ZipInfo` を作る。
つまり、archiveを開いてから `len(infolist())` を検査しても、
constructor中のmetadataメモリ消費を遡って防ぐことはできない。
圧縮入力上限とworkerのメモリ制限は、member展開loopより前から適用する設計が必要になる。

`file_size` と合計値を使う事前拒否は有用だが、trusted manifestと同等には扱わない。
実際の出力bytesも計数する。予算を超えるchunkは公開先へ書き込まず、
jobを失敗として扱う。ここでの保存前判定も、既にdecoderが使った一時メモリまでは制限しない。

### decoder内部メモリとread結果は別物

`lzma.LZMADecompressor` の公開APIには、
返すbytesを制限する `max_length` とは別に、
decompressorが使うメモリを制限する `memlimit` がある。
v3.14.8のZIP用LZMA wrapperは `FORMAT_RAW` とfiltersを使って構築し、
この `memlimit` 引数を渡していない。
`ZipFile(..., memlimit=...)` という公開設定があると推測しない。

この違いからも、「read chunkを8KiBにしたのでprocessのRSSも8KiB程度」とは言えない。
詳細なcodec内部の最大メモリ量は本調査では測定していない。
根拠は [lzma API](https://docs.python.org/3.14/library/lzma.html#lzma.LZMADecompressor)、
[zipfile constructorとwrapper](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/zipfile/__init__.py)。

### 検査完了前の公開を避ける

公式文書は、資源不足や中断によって展開が失敗・不完全になることを明記する。
独自設計として、取り込み途中の成果物は非公開に保ち、
許容条件の検証と全体完了の後に公開する。
CRC不一致、未対応codec、deadline、予算超過を部分成功へ黙って変換しない。

`testzip()` も安価な事前承認APIではない。同snapshotでは全memberを読みCRCを確認するため、
実データの展開コストが発生する。その成功だけで資源予算の余裕を証明することはできず、
同じ制限下で実行する対象に含める。
[zipfileの注意点](https://docs.python.org/3.14/library/zipfile.html#decompression-pitfalls)、
[版固定testzip実装](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/zipfile/__init__.py)

## 回帰テストが確かめている範囲

公式 `Lib/test/test_zipfile/test_core.py` を静的に読んだ。

- `AbstractBoundedDecompressTests.test_read1_output_is_bounded` は、
  4MiBの展開内容を持つ小さなfixtureを作り、
  private `_read1(100)` の出力長が `MIN_READ_SIZE` 以下であることを検査する。
  STORED / DEFLATE / bzip2 / LZMA / Zstandardごとのclassがあり、
  optional codecのテストは利用可能性の条件付きである
- これは返却された展開bytesの長さを検査するテストであり、
  peak RSS、CPU上限、全archiveの許容量を測定するテストではない
- `MonkeypatchedDecompressorTests.test_roundtrip_monkeypatched_decompressor` は、
  旧1引数APIのcodecに対するread、read1、seek、巻戻し後の内容一致を確認する。
  これは互換性の回帰テストであり、旧codecに資源上限を与えたという証明ではない

運用側の追加試験案は、有限サイズのfixtureで各採用codecのchunk読取り、
drain後の内容一致、member上限とarchive合計上限のちょうど境界・1byte超過、
constructor段階の件数・入力上限、CRC失敗、中断時の未公開出力破棄を検査すること。
資源制約を保証するには、単に「エラーになった」だけでなく、
上限超過を検出した場所、worker終了、保存済み量も観測する。
これは提案であり、この調査で実施した試験結果ではない。
[版固定回帰テスト](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/test/test_zipfile/test_core.py)

## 検証・ライセンス・未確認事項

- native webでrelease、zipfile / bz2 / lzma / compression.zstdの公式API、
  2件の公式PRとcommit、Pythonのライセンス頁を開いた
- v3.14.8のtagをannotated tagから40桁commitへ解決し、
  read-only GitHub connectorで版固定の実装・テスト・NEWS・LICENSEを確認した。
  webのraw/blob取得はcache miss、統合changelogはサイズ上限で失敗したため、
  同じ版のファイルをconnectorで取得した。失敗を再確認日だけで埋めていない
- Pythonのソフトウェア・文書はPSF-2.0、文書中のコード例は追加で0BSD。
  本稿は独自要約で、ソースコードや攻撃fixtureを複製していない。
  release web page自体は再利用ライセンスを断定せずcatalogをunknownにした
- ローカルのPythonは **3.12.14** だった。対象 **3.14.8のruntime試験は実行していない**。
  上記は公式テストの読解であり、codec別実行成功・メモリ消費量・
  サービスのDoS耐性を検証したという主張ではない
- CVEサイトはJavaScript必須で詳細を取得できなかった。
  CVEと修正の対応はPython公式release / NEWSで確認した範囲に限定する。
  CVSS、攻撃条件の網羅、全系列の最初の修正版、配布元独自backportは未確認
- ライブラリが実際にmonkey-patchしているか、そのcodecが上限を守るかは
  利用アプリケーション固有の確認事項である
- 検索evalは知識文書を取り出せることの検査であり、Pythonの安全性テストではない


---
{
  "id": "python-tarfile-filter-patch-partial-extraction-boundary",
  "title": "Python tarfile: 3.14.8の修正範囲とdataフィルタ・部分展開の境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "CPython 3.14.8 / 3.13.16; 3.14未リリース修正 a4919937a4e1e69a0d178909c6f20557eca5d1d0; 共通契約の限定実測3.12.14",
  "tags": [
    "research-domain:backend",
    "tarfile",
    "extractall",
    "data_filter",
    "extraction-filter",
    "CVE-2026-19672",
    "CVE-2026-82049",
    "CVE-2026-87910",
    "partial-extraction",
    "link-fallback"
  ],
  "sources": [
    {
      "id": "python-tarfile-filter-3-14-8-20261002",
      "url": "https://docs.python.org/3.14/library/tarfile.html",
      "type": "official_docs"
    },
    {
      "id": "python-tarfile-filter-3-13-16-20261002",
      "url": "https://docs.python.org/3.13/library/tarfile.html",
      "type": "official_docs"
    },
    {
      "id": "python-release-3-14-8-tarfile-20261002",
      "url": "https://www.python.org/downloads/release/python-3148/",
      "type": "release_notes"
    },
    {
      "id": "python-release-3-13-16-tarfile-20261002",
      "url": "https://www.python.org/downloads/release/python-31316/",
      "type": "release_notes"
    },
    {
      "id": "python-tarfile-cve-19672-advisory-20261002",
      "url": "https://mail.python.org/archives/list/security-announce@python.org/thread/J2WT2ALRWEXQJOB3C7Q2HYWUXP3CINWO/",
      "type": "maintainer_article"
    },
    {
      "id": "python-tarfile-cve-82049-advisory-20261002",
      "url": "https://mail.python.org/archives/list/security-announce@python.org/thread/EFJWGAZJA56AKSBR2WHMHQZO7RRLZPRH/",
      "type": "maintainer_article"
    },
    {
      "id": "python-tarfile-cve-87910-advisory-20261002",
      "url": "https://mail.python.org/archives/list/security-announce@python.org/thread/57TBTLL2W6APMZR3A25B2YV7GL3EPTDJ/",
      "type": "maintainer_article"
    },
    {
      "id": "psf-cna-tarfile-87910-20261002",
      "url": "https://cveawg.mitre.org/api/cve/CVE-2026-87910",
      "type": "official_docs"
    },
    {
      "id": "cpython-tarfile-3-14-8-8e6e75d9-20261002",
      "url": "https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/tarfile.py",
      "type": "github_repository_analysis"
    },
    {
      "id": "cpython-tarfile-none-fix-a4919937-20261002",
      "url": "https://github.com/python/cpython/commit/a4919937a4e1e69a0d178909c6f20557eca5d1d0",
      "type": "github_repository_analysis"
    },
    {
      "id": "python-pep706-extraction-contract-20261002",
      "url": "https://peps.python.org/pep-0706/",
      "type": "official_docs"
    },
    {
      "id": "python-license-tarfile-3-14-8-20261002",
      "url": "https://docs.python.org/3.14/license.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# tarfile の更新済みランタイムと展開ポリシーを別々に確認する

## 問いと結論

外部から受け取ったデータtarをバックエンドで展開するとき、「Python 3.14なので既定の `data` が守る」「3.14.8へ更新したのでリンクも含めて検査済み」と判断してよいか。

**どちらも十分な受入条件ではない。** 2026-09-30公開の [3.14.8](https://www.python.org/downloads/release/python-3148/) は重要な修正を含むが、custom filterの `None` がリンク代替展開で無視される修正は、そのrelease tagには入っていない。さらに、フィルタの正常終了と「全件を受理した」「副作用を戻した」「資源使用量が有限」は別の条件である。

本稿はPython標準ライブラリの展開API、patchの境界、呼出し側の成功判定を扱う。攻撃アーカイブの作成手順や、本番システムの診断・変更は対象外。公式の契約、固定実装の観察、独自の運用設計を以下で区別する。版と取得日は2026-10-02 UTCに確認した。

## 直近の修正を一括りにしない

### CVE-2026-19672: 最終到達先だけでは途中の作成を表さない

[2026-08-19の担当者告知](https://mail.python.org/archives/list/security-announce@python.org/thread/J2WT2ALRWEXQJOB3C7Q2HYWUXP3CINWO/) が説明する問題は、途中で展開先を離れて戻る名前について、検証は解決後のパスを見ていた一方、中間ディレクトリの作成は元の名前を使っていた点にある。

影響はPOSIXでの**外側の空ディレクトリ作成**であり、この記事から任意の外部ファイル内容が書き換わるとは主張しない。告知では内容自体は展開先内に収まり、Windowsでは同じ現象は起こらず、secure randomized destinationはこの問題の成立条件を除くとされる。

[3.14.8のtar_filter](https://docs.python.org/3.14/library/tarfile.html#tarfile.tar_filter) と [3.13.16の同API](https://docs.python.org/3.13/library/tarfile.html#tarfile.tar_filter) は、member名の `..` 成分を正規化する変更をそれぞれのpatch版に明記する。したがって上流3.14系でこの修正を必要とする運用は3.14.8、3.13系では3.13.16を確認済みの修正版として扱える。ランダムな作業場所だけで他の不具合まで解決したとは扱わない。

### CVE-2026-82049: release一覧と影響版を分ける

[2026-09-14の告知](https://mail.python.org/archives/list/security-announce@python.org/thread/EFJWGAZJA56AKSBR2WHMHQZO7RRLZPRH/) はCPython 3.13以前を対象に、symlinkへのhardlinkを通じた展開先外ファイルの権限・mtime変更や、展開ツリーからの内容露出を説明する。[3.13.16の公開情報](https://www.python.org/downloads/release/python-31316/) はこの修正を含む。

3.14.8の公開情報にも関連変更が載るが、その一行から「3.14.0〜3.14.7の安定版もこのCVEに罹患」とは推定しない。後述の固定実装ではhardlinkの参照対象（`os.link` の第1引数）を `realpath` で解決する処理を確認した。ライブラリの変更が列挙されていることと、その系列が元々どの条件で影響を受けたかは別の問いである。

### CVE-2026-87910: 3.14.8に残るcustom filterの例外経路

[2026-09-11の告知](https://mail.python.org/archives/list/security-announce@python.org/thread/57TBTLL2W6APMZR3A25B2YV7GL3EPTDJ/) は、リンクを作れず内容を代替展開するときに、リンク位置の名前と元memberの名前で行うfilter呼出しのうち、一方の戻り値を捨てていたと説明する。`None` によるスキップを使うcustom filterに関係し、通常ファイルの全filter呼出しが無視される問題ではない。

- [PSF CNA record](https://cveawg.mitre.org/api/cve/CVE-2026-87910) の2026-10-02更新版は、3.14.0a1以上3.15.0未満をaffectedとしている。これは取得時点の登録範囲であり、今後の3.14 patchや配布元backportまで永続的に未修正とする宣言ではない
- [3.13.16のrelease情報](https://www.python.org/downloads/release/python-31316/) は87910を含む。上流3.13系で本稿の三件を確認する基準は3.13.16となる
- [3.14向け修正commit](https://github.com/python/cpython/commit/a4919937a4e1e69a0d178909c6f20557eca5d1d0) は、[PR 157307](https://github.com/python/cpython/pull/157307) で2026-10-01にmergeされた。9月30日の3.14.8公開より後であり、branchへのmergeだけではrelease済みといえない

よって「3.14.8以上なら87910も修正済み」という判定は使わない。対象配布物に修正が入ったかを確認する。未確認で、リンクを受け入れつつcustom filterの `None` に依存する用途なら、その経路の受入を保留するのが本稿の設計提案である。独自monkey patchをこの調査の成果として推奨・実装するものではない。

## 公開APIの契約: filterの選択とskipは業務上の成功ではない

[3.14文書](https://docs.python.org/3.14/library/tarfile.html#extraction-filters) と [3.13文書](https://docs.python.org/3.13/library/tarfile.html#tarfile.TarFile.extraction_filter) を比較した。

- `extractall(filter='data')` はフィルタを明示する。3.14は引数と `extraction_filter` がともに `None` ならdata、3.13はその条件で警告を出しfully_trustedへ戻る。3.13.16へpatch更新してもこの既定は変わらない
- `data` は展開先外リンクや特殊ファイルなどを制限するが、全リンクを拒否する指定ではない。内部に留まるリンクを不要とする業務なら別の制約が要る
- `TarFile.extraction_filter` 属性にはcallableまたは `None` を設定する。引数と違い文字列 `'data'` を代入する場所ではない
- custom filterは各memberの展開直前に呼ばれ、変更後 `TarInfo` を返せばそれを使い、`None` はスキップを意味する。APIの戻りが正常でも、アーカイブの全memberが採用されたとは限らない

[PEP 706](https://peps.python.org/pep-0706/#backporting-forward-compatibility) は機能が旧版へbackportされ得ると説明する。`hasattr(tarfile, 'data_filter')` は機能の有無を調べる手段であり、2026年の個別修正の有無を証明しない。未対応時にfully_trustedへ黙ってfallbackすると、同じアプリでも環境によって受入ポリシーが変わる。

### errorlevelと部分展開

[3.13のerrorlevel契約](https://docs.python.org/3.13/library/tarfile.html#tarfile.TarFile.errorlevel) では、既定の1はfatalな `FilterError` / `OSError` を送出し、2はnon-fatalな展開エラーも送出する。0はfilter拒否を含む該当エラーを抑制して継続するが、引数不正などすべての例外を消す設定ではない。

途中で例外になっても既に書いたファイルは残り得る。`extractall` には全体を自動rollbackする契約がない。`with TarFile` によりアーカイブを閉じたことを、展開先のcleanup完了と同じにしない。フィルタが返した `None`、抑制されたエラー、採用したmemberを区別せず、戻り値だけで取込完了を通知しない。

## 固定した実装で確かめた範囲

`v3.14.8` のpeeled tagは **8e6e75d9102e39bed2a2b279203a396741180f12**。その [Lib/tarfile.py](https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/tarfile.py) を読み、次を確認した。

1. `_get_filtered_attrs` は `..` を含むmember名を正規化してから到達先を検査する
2. `makelink_with_filter` のhardlink処理は参照対象を `os.path.realpath` で解決し、`os.link` の第1引数に渡す
3. fallbackで名前をリンク位置に変えた最初のfilter呼出しには戻り値の判定がなく、その後の元memberに対する戻り値だけが `None` 検査に使われる

対して **a4919937a4e1e69a0d178909c6f20557eca5d1d0** では最初の結果も保持し、`None` なら戻る。[同commitのテスト差分](https://github.com/python/cpython/commit/a4919937a4e1e69a0d178909c6f20557eca5d1d0) には `test_extract_filters_target_none` が追加されている。これは既知のCVEとrelease境界の確認であって、新しい脆弱性の発見や、全OSでの再現完了という主張ではない。

公開docsは更新され続ける。表示が3.14.8でも、tagから後の変更が説明に入り得るため、patch内のコード有無は固定tagで照合する。実装の内部関数名をアプリケーションから呼び出す提案でもない。

## 呼出し側の取込設計案

以下は資料を踏まえた独自の設計であり、tarfileが提供する一括トランザクションではない。[PEP 706の追加検証](https://peps.python.org/pep-0706/#hints-for-further-verification) が挙げる資源制限・名前検査・新しい作業場所を、業務上の公開条件に接続する。

1. 受信物をそのまま公開ディレクトリへ展開せず、処理ごとに新しい非公開のランダム作業ディレクトリを割り当てる。他の書き手が内容を差し替えられる場所を使わない
2. 許容する形式を先に絞る。普通のデータ集なら通常ファイルと必要なディレクトリだけを候補にし、hardlink・symlinkは要件がある場合だけ個別に試験する。権限やUNIX固有metadataの忠実な復元が目的のバックアップとは同じポリシーにしない
3. 標準のdata制限を維持した上で、業務上の名前・型・拡張子・重複名の扱いを決める。正規化後やcase-insensitiveな保存先で衝突する名前についても、採用順を偶然に任せない
4. 件数、1件と合計の展開量、名前長などの予算を決める。同時にOS側のCPU・メモリ・ディスク制限を設ける。圧縮入力の小ささやヘッダーの申告量だけを、実際の資源消費量の保証に使わない
5. 想定したmanifestと実際に受け入れた内容を照合し、スキップやエラーがあればその理由を記録する。厳密な取込なら `errorlevel=0` で不完全なbundleを黙認しない
6. 全体検証が終わった成果物だけを公開する。公開の原子性は保存先の仕組みで別に設計し、tarfileによる保証としない。失敗時は今回の非公開作業場所をcleanupし、再試行時も新しい場所を使う

`data` だけではDoSも、展開中に他者がディレクトリを変更する競合も防げない。ランタイム更新、取込制約、隔離、資源制限、公開前照合は互いの代替ではない。この設計案は全アーカイブ形式や第三者解凍ライブラリへの認証済みの安全策ではない。

## 実測と受入試験

ローカルのCPython **3.12.14** で、外へ出るパスやリンクを含まない通常ファイル3件の自作tarを使い、2件目だけcustom filterが `FilterError` を出す限定試験を行った。

- `errorlevel=1` は例外となり、1件目 `before.txt` は残り、2件目と3件目は作られなかった
- `errorlevel=0` は正常に戻り、拒否した2件目はなく、1件目と3件目 `after.txt` は存在した

各試験は別の一時ディレクトリ内で行い、終了時に破棄した。これは部分展開・skipの限定観察で、3.14.8、87910のfallback、OS差分、悪意ある入力の安全性を実測したものではない。上流テストは読んだだけで実行していない。

本番採用前の試験案:

- アプリが使う実際の配布物で、版・配布元・backport情報を記録する。表示版、関数の存在、修正commitのmergeを混同しない
- 通常取込に加え、途中のfilter拒否、意図した `None`、ディスク不足、処理期限切れで、未公開・cleanup・再試行の結果を確認する
- linkを許可する場合は、リンク非対応環境でのfallbackも対象にする。通常ファイルだけの成功をfallbackの検証に数えない
- 必須memberの欠落、重複名、保存先で衝突する名前、容量予算超過を別ケースにし、採否と観測記録を確かめる

検索evalはこの記事の発見可能性だけを確認する。これらの実行時試験や安全性レビューの代わりにはならない。

## 出典・鮮度・未確認事項

一次資料は本文のリンクとFront Matterのcatalogを参照。全取得日は2026-10-02 UTC。release_notesの30日TTLに合わせ期限を2026-11-01とした。PSF CNAは2026-10-02T00:35:07.049Z更新の87910レコードを読み、提供者 `PSF` を確認した。影響範囲の登録と実際の配布物は今後再照合する。

[公式ライセンス](https://docs.python.org/3.14/license.html) は文書をPSF-2.0、文書内コードを追加0BSDとする。PEP 706はCC0-1.0またはpublic domain。固定tagのLICENSEとtarfile.pyヘッダーも読み、後者はMITと確認した。告知・releaseページ・CNA本文の個別ライセンスはunknownとして原文やコードを転載せず、独自の日本語要約を保存した。コードの取込みやmodule昇格はない。

巨大な統合changelogはweb取得のサイズ上限で読めなかったため根拠に使わず、版別releaseページ、API文書、告知、固定sourceで照合した。固定raw sourceとCNA JSONは直接取得できたが、同URLをweb表示する経路は取得エラーとなった。3.14.8実行環境の新規導入、CVE再現、各OS・配布元patch・全旧版の検証、87910を含む次の3.14正式releaseの確認は未実施である。

---
{
  "id": "python-startup-start-pth-suppression-boundary",
  "title": "Python 3.15 .start移行: 起動順序・同名.pth抑制・失敗観測の境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "CPython 3.15.1, tag v3.15.1 dereferenced to c37e74cb6bcdadcd5f46f289498f6b3298ddee61; rolling Python 3.15.1 docs and PEP 829 verified 2026-10-24 UTC; source inspection, no 3.15 runtime execution",
  "tags": [
    "research-domain:backend",
    "python",
    "PEP-829",
    "startup",
    ".start",
    ".pth",
    "StartupState",
    "migration",
    "initialization",
    "observability"
  ],
  "sources": [
    {"id": "python-startup-release3151-20261024", "url": "https://www.python.org/downloads/release/python-3151/", "type": "release_notes"},
    {"id": "python-startup-whatsnew3151-20261024", "url": "https://docs.python.org/3.15/whatsnew/3.15.html", "type": "release_notes"},
    {"id": "python-startup-site3151-20261024", "url": "https://docs.python.org/3.15/library/site.html", "type": "official_docs"},
    {"id": "python-startup-pkgutil3151-20261024", "url": "https://docs.python.org/3.15/library/pkgutil.html", "type": "official_docs"},
    {"id": "python-startup-pep829-20261024", "url": "https://peps.python.org/pep-0829/", "type": "official_docs"},
    {"id": "python-startup-cpython3151-20261024", "url": "https://github.com/python/cpython/tree/c37e74cb6bcdadcd5f46f289498f6b3298ddee61", "type": "github_repository_analysis"}
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/backend.json"]
}
---

# Pythonの起動hookを「呼ばれたはず」で判定しない

**判断:** `.pth` の実行行を `.start` へ移すときは、同じ初期化関数の二重実行だけでなく、両方とも実行されない状態と、旧経路だけが残る状態を試験する。読み込めた空の `.start` でも同名 `.pth` の実行行は抑制される一方、壊れたUTF-8で読めなかった `.start` は、固定したCPython 3.15.1実装では抑制の登録に達しない。新hookが失敗したときの自動fallbackや、起動成功による初期化完了の保証ではない。

既存の[UTF-8移行](utf8-default-file-pipe-migration-boundary.md)はテキストI/Oのcodec契約、[lazy imports](lazy-imports-release-readiness-boundary.md)はmodule初期化の遅延を扱う。本稿の問いは、ユーザーコードの最初の行より前に動くpackage startup設定の移行である。以下では公開契約、固定sourceの観察、独自の受入条件を分ける。

## 1. 確認した版と将来計画を分ける

[3.15.1配布ページ][release]の公開日は2026-10-21。[What's New][new]はPython 3.15の公開日を2026-10-09とし、PEP 829のpackage startup configurationを導入機能として掲載する。`.start` は未公開案としてではなく、3.15系列の機能として扱う。ただし3.15.1の配布日を、この機能が初めて導入された日とはしない。

API文書の取得時表示は3.15.1。`/3.15/` は更新されるURLであり、固定release snapshotではない。実装の比較点は、GitHub APIでtag `v3.15.1` をdereferenceした **c37e74cb6bcdadcd5f46f289498f6b3298ddee61** に固定した。上流の `Lib/site.py`、`Lib/pkgutil.py`、`Lib/test/test_site.py`、`LICENSE` を読んだが、CPython 3.15をbuild/runしていない。

[PEP 829][pep]はFinal、作成日2026-03-31、最終更新表示2026-06-15 UTC。概要は3.15–3.17の移行期間、3.18–3.19の実行行の黙示無視、3.20以降の警告を述べるが、Specificationには「移行期間後に無視して警告」という省略した説明も残る。[site文書][site]の版別説明と区別し、まだ公開されていない3.18以降を実測済み挙動として書かない。将来版の採用時に改めて確認する。

## 2. 残すものと移すもの

`.pth` のpath拡張は残り、`import` に続く空白またはtabで始まる実行行を移行する。`.start` にPython文や引数付きの呼出し式を書くのではなく、import可能なmodule内の引数なし関数を `pkg.mod:callable` 形式で指す。[site文書][site]は、旧Python併存中は同じ関数を `.pth` からも呼び、新版では同名 `.start` を優先する移行方法を示す。

[pkgutilの公開契約][pkgutil]では、3.15追加の `strict=True` はcolonの左右を必須にする。`pkg.mod` だけ、`pkg.mod:`、引数を添えた式はこの形式ではない。ただし名前が構文上正しいことと、実際にimportできること、対象がcallableであること、初期化が成功することは別の判定である。

固定sourceの `_execute_start_entrypoints()` は `pkgutil.resolve_name(..., strict=True)` で解決し、その結果を引数なしで呼ぶ。戻り値を起動成否の判定に使う処理はない。成功・失敗をbooleanで返す設計にしても、`False` がPython全体の起動失敗へ変換されるとは期待しない。[実装][impl-entry]

PEPは `.start` にUTF-8と任意BOMを認める `utf-8-sig` を要求する。これはlocale encodingへのfallbackを持つ旧 `.pth` と同じ条件ではない。固定sourceもUTF-8 decode失敗時に、`.start` では戻り、`.pth` だけlocaleでのdecodeへ進む。[PEP][pep]、[実装][impl-read]

## 3. alphabeticalだけで実行順序を設計しない

通常起動の `site.main()` は一つの `StartupState` へsite directory群を集める。固定sourceの `process()` の呼出し順は次の通り。[実装][impl-process]

1. 蓄積した静的pathを `sys.path` へ反映する
2. 対応する `.start` が登録されていない `.pth` のimport行を実行する
3. `.start` のentry pointを実行する

したがって、`aaa.start` と `zzz.pth` という名前を付けても、前者が後者の実行行より先になるわけではない。`.pth` 内の実行行をpath行の途中へ置き、従来の逐次処理順に依存していた初期化も再点検する。「全pathが先」は、同じbatchに集めた静的拡張の説明であり、任意のhookが後で行う動的変更まで事前適用する約束ではない。

各directoryでは `.pth` 群と `.start` 群がそれぞれfilename順で読まれ、各file内のentry順が保たれる。全directoryのfilenameを横断して一つにsortする処理ではない。固定テスト `test_addsitedir_startup_state_preserves_site_relative_order` は、pathが `sitedir1, extdir1, sitedir2, extdir2` の相対順を保つことを確認対象にする。[実装][impl-discovery]、[上流テスト][tests-order]

`site.addsitedir()` をdirectoryごとに呼ぶ場合は、それぞれ新しいstateを作って即座に処理する。後から追加するdirectoryのpathが先のhookに見えるとは限らない。同じbatchが必要なら `StartupState.addsitedir()` で蓄積して最後に一度だけ `process()` を呼ぶ。[What's New][new]、[実装][impl-shim]

重複entry pointは削除されない。また `StartupState.process()` は冪等ではなく、同じinstanceで再実行すると実行行・entry pointが再度動く。pathの重複排除を、初期化の一回性へ読み替えない。[site文書][site]、[重複実行テスト][tests-duplicate]

## 4. 同名抑制の境界は「新hookの成功」ではない

固定実装の `_read_start_file()` は、読取に成功した後でfileの完全pathをentry point辞書へ登録する。空でも登録する。`_exec_imports()` は `.pth` の完全pathの拡張子を `.start` に置換し、同じbatchの辞書にあるかを調べる。package名や関数名の一致ではない。[実装][impl-start]、[抑制判定][impl-suppress]

| 状態 | 固定sourceから確認できる境界 |
|---|---|
| 同じdirectoryに同stemの、読める空 `.start` | `.pth` のimport行を抑制する。代わりの関数は一つもない |
| commentだけの、読める同名 `.start` | 空fileと同様に登録される |
| UTF-8として読めるがentry point構文が不正 | fileは登録され、`.pth` は抑制済み。構文失敗は実行段階で報告する |
| import先欠落やcallable実行時の通常例外 | 旧 `.pth` を復活させる処理はない |
| 不正UTF-8の同名 `.start` | 読取失敗で登録されない。抑制判定を満たさず、読めた `.pth` の実行行が残る |
| 別stemまたは別directoryの `.start` | 完全pathの対応がないため、その `.pth` の抑制条件にはならない |

空・comment-only・不正UTF-8の登録状態は、それぞれ上流テストで明示されている。空fileによる実際のimport抑制も専用テストがある。不正UTF-8と同名 `.pth` を組み合わせた最終結果は、読取テストと抑制関数を合わせた**source読解による推論**であり、この環境での実行観測ではない。[読取テスト][tests-read]、[空file抑制テスト][tests-empty]

`_read_pthstart_file()` にはstat/readの `OSError` やOS hidden属性で登録前に戻る分岐もある。これらも「成功した読取だけが登録される」という実装境界を示すが、permission・hidden属性・symlink・case差の全組合せを検証したとはしない。壊れたfileを故意に残して旧経路へ戻す運用は推奨しない。

独自の運用案: `.pth`、`.start`、対象moduleを一つの配布単位として検証する。空placeholderを先に配り、後から関数指定を埋める手順は、旧経路まで止める期間を作り得る。rollbackも `.start` だけでなく、その時点のPython版と旧hookの互換性を合わせて設計する。

## 5. stderrが空、exit codeが0でも初期化完了とは限らない

固定sourceの診断経路を分けて読む。

- fileのstat/read失敗や `.start` のUTF-8 decode失敗は `_trace()` 経由で、通常は `-v` のときだけ診断される。不正UTF-8の上流テストも、非verboseでstderrが空という期待を持つ。[読取実装][impl-read]、[読取テスト][tests-read]
- 一方、不正entry point構文は解決時の `ValueError`、importや属性解決の失敗は `Exception`、関数内の通常例外も `Exception` として `_print_error()` へ渡り、次のentryへ進む。こちらは `-v` がなくてもstderrへ出力する。[実装][impl-entry]
- legacy `.pth` の実行行も、各行を個別に `try` で囲み、通常例外を報告して次へ進む。移行期間のdeprecated通知自体は `_trace()` であり、通常のwarning filterを設定しただけで必ず見える診断ではない。[実装][impl-suppress]

「不正entry pointはparse時に黙って捨てる」とPEPの一般的なエラー説明だけから結論しない。固定実装は非空・非comment行をいったん受理し、構文検査を実行時へ遅らせている。上流の `test_impl_read_start_file_accepts_all_non_blank_lines` と `test_impl_execute_entrypoints_strict_syntax_rejection` がこの差を扱う。[テスト][tests-read]、[構文拒否テスト][tests-errors]

この継続処理はtransactionやrollbackではない。先のhookが登録した状態を後の失敗時に戻す保証もない。さらにcatch対象は `Exception` であり、`SystemExit` 等の `BaseException` 全体を必ず握りつぶして継続するとは書かない。独自の判断として、必須の登録・設定が揃ったことをアプリのreadinessで明示的に検査し、processが生きているだけを合格にしない。

## 6. 独自の移行試験と採用判断

以下は運用提案であり、上流のテストを実行した結果ではない。実際のwheelや配布物を使い、対象Pythonの新規processで毎回確認する。test suiteの先行importで偶然初期化された状態を流用しない。

| 試験 | 合格条件・観測対象 |
|---|---|
| 旧Pythonと採用する3.15 patch版 | 同じ初期化関数が期待回数だけ動く。path拡張と登録結果を別々に比較 |
| 正常な同名pair、別stem、別directory | 抑制対象を完全pathで記録し、二重実行や未実行を検出 |
| 空、comment-only、不正UTF-8、不正構文 | 期待する旧経路の有無を確認。stderrの有無だけで成功判定しない |
| module欠落、属性欠落、非callable、関数内例外 | file・entry・例外を追跡でき、必須初期化欠落時はreadiness不合格 |
| 複数directoryに分かれたpathとhook | batchと個別addsitedirで結果を比較し、暗黙の実行順依存を排除 |
| 重複entry、二度のprocess | 呼出し回数と副作用を測り、再入や冪等性をアプリ側で定義 |
| 通常起動、診断用 `-v`、隔離試験用 `-S` | verboseだけに出る失敗も収集。`-S` でstartup処理がない結果を正常配備と混同しない |
| 一部初期化後の失敗とrollback | 半端な登録状態でもtrafficを受けない。旧版へ戻す配布物も試験 |

`.start` は安全なsandboxではない。PEP自身も、entry point経由の任意コード実行は残り、allow/denyのglobal policyは将来の仕様に委ねるとする。監査しやすい表現への変更を、実行権限の縮小や未信頼packageの安全性と同一視しない。[PEP][pep]

`-S` はsite処理を自動実行しないため両形式に影響するが、後で明示的に `site.main()` を呼べばsite処理を起動できる。したがって「.startだけを止めるrollback flag」として採用しない。[site文書][site]

## 7. 検証範囲とprovenance

取得日は2026-10-24 UTC。一次資料本文をnative webで開き、release・PEP・APIの表示版と条件を確認した。固定GitHub blobのnative web取得は失敗したため、GitHub connectorによるtag参照・tag object・固定SHAのfile取得でsourceを読んだ。tag objectは `f379247960123edf69b596041054836115af6f5a`、そのcommitが前記SHAである。

調査環境のPythonは3.12.14。3.15実行、Windows/macOS、実際のpackaging backend、editable install、embedding、全上流suiteは未検証。source内のテスト名は存在と期待値の確認であって実行成功ではない。`._pth` はPEP 829対象外であり、似た名前でも本稿の移行規則を流用しない。[PEP][pep]

PEPはPublic DomainまたはCC0-1.0-Universal。Python文書はPSF-2.0で、文書内codeには追加0BSDが付く。固定repositoryのLICENSEもPSF-2.0と歴史的noticeを含む。配布告知自体のlicenseはunknownとして、公開日等の独自要約だけを採用した。外部コードの転載・module昇格はない。release_notesの30日TTLを踏まえ、明示期限は2026-11-23とする。

[release]: https://www.python.org/downloads/release/python-3151/
[new]: https://docs.python.org/3.15/whatsnew/3.15.html#pep-829-package-startup-configuration-files
[site]: https://docs.python.org/3.15/library/site.html
[pkgutil]: https://docs.python.org/3.15/library/pkgutil.html#pkgutil.resolve_name
[pep]: https://peps.python.org/pep-0829/
[impl-read]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/site.py#L180-L228
[impl-discovery]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/site.py#L333-L365
[impl-start]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/site.py#L442-L461
[impl-process]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/site.py#L463-L500
[impl-suppress]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/site.py#L502-L544
[impl-entry]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/site.py#L546-L577
[impl-shim]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/site.py#L595-L612
[tests-read]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/test/test_site.py#L1060-L1158
[tests-errors]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/test/test_site.py#L1289-L1324
[tests-duplicate]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/test/test_site.py#L1326-L1340
[tests-empty]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/test/test_site.py#L1379-L1392
[tests-order]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/test/test_site.py#L1536-L1592

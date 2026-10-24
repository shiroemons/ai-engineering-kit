---
{
  "id": "python-lazy-imports-release-readiness-boundary",
  "title": "Python 3.15 lazy imports: 初回利用・登録副作用・移行判定の境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "CPython 3.15.0 final released 2026-10-09; 3.15.1 released 2026-10-21, commit c37e74cb6bcdadcd5f46f289498f6b3298ddee61; rolling 3.15.1 docs verified 2026-10-24 UTC; source inspection, no 3.15 runtime execution",
  "tags": [
    "research-domain:backend",
    "python",
    "lazy-imports",
    "PEP-810",
    "import-time",
    "side-effects",
    "initialization",
    "migration",
    "__lazy_modules__",
    "readiness",
    "sys.lazy_modules",
    "maintenance-release"
  ],
  "sources": [
    {
      "id": "python-lazy-315rc3-release-20261024",
      "url": "https://www.python.org/downloads/release/python-3150rc3/",
      "type": "release_notes"
    },
    {
      "id": "python-lazy-3150-release-20261024",
      "url": "https://www.python.org/downloads/release/python-3150/",
      "type": "release_notes"
    },
    {
      "id": "python-lazy-3151-release-20261024",
      "url": "https://www.python.org/downloads/release/python-3151/",
      "type": "release_notes"
    },
    {
      "id": "python-lazy-language-3151-20261024",
      "url": "https://docs.python.org/3.15/reference/simple_stmts.html",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-sys-3151-20261024",
      "url": "https://docs.python.org/3.15/library/sys.html",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-types-3151-20261024",
      "url": "https://docs.python.org/3.15/library/types.html",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-commandline-3151-20261024",
      "url": "https://docs.python.org/3.15/using/cmdline.html",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-pep810-20261024",
      "url": "https://peps.python.org/pep-0810/",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-cpython-3151-20261024",
      "url": "https://github.com/python/cpython/tree/c37e74cb6bcdadcd5f46f289498f6b3298ddee61",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Python 3.15 lazy imports の移行判定

## 問いと結論

CLIやbackendの起動時間を減らすため `lazy import` を使うと、依存不足・plugin登録・設定読込はいつ失敗するのか。Pythonを更新すれば一律に有効になるのか。final公開後に何を再検証すべきか。

**2026-10-24 UTC時点で3.15.0 finalは公開済みであり、確認したmaintenance版は3.15.1である。** ただし安定版公開は、個別アプリの登録副作用・初回request・introspectionの互換性を証明しない。導入判断は「遅らせてもよい依存」と「readiness前に検証すべき依存」を分ける。`sys.lazy_modules` の件数や空集合を起動完了条件にしない。

既存のPython futures記事は実行queueとshutdown、.start記事は起動ファイル、本稿はmodule初期化・初回利用・release移行の契約を対象にする。以下は公式契約、固定sourceの観察、独自の設計案を区別して記す。新規機能の発表ではなく、rc2時点でstaleになった本文全体の再検証である。

## Release境界: 延期履歴から3.15.1へ

| 確認した版 | 実際の公開日 | 本稿での意味 |
|---|---|---|
| [3.15.0rc3](https://www.python.org/downloads/release/python-3150rc3/) | 2026-10-02 | lazy-import release blockersに伴うfinal延期を受けた追加preview。現在の本番推奨版ではない |
| [3.15.0 final](https://www.python.org/downloads/release/python-3150/) | 2026-10-09 | 安定版公開を実ページで確認。古い予定日2026-10-01やPEPのstatusから推測しない |
| [3.15.1](https://www.python.org/downloads/release/python-3151/) | 2026-10-21 | 3.15初回maintenance release。3.15.0ページにも置換先として表示され、本稿の固定source比較点とする |

旧版の記録は「2026-10-02の調査時点ではrc3公開が未確認で、rc2固定snapshotだけを読んだ」という履歴として扱う。今回rc3の実公開を確認できたが、rc3 runtimeそのものは未検証である。旧catalog recordsと既存evalは保持し、現在の本文は新しい取得日・source recordsへ切り替えた。

`/3.15/`文書は取得時に3.15.1を表示するrolling文書であり、今後も更新される。[PEP 810](https://peps.python.org/pep-0810/)の `Final` はPEPの状態で、Python配布版の公開証明ではない。実装観察は `v3.15.1` のdereference先 `c37e74cb6bcdadcd5f46f289498f6b3298ddee61` に固定した。再確認期限は2026-11-23とするが、それまで無変更であるという保証ではない。

## 公開契約: 遅れるのは探索・初期化・失敗

[Language Reference](https://docs.python.org/3.15/reference/simple_stmts.html#lazy-imports)と固定版の対応文書で確認した範囲:

1. module scopeの `lazy import package` / `lazy from package import member` はproxyを束縛し、名前の初回利用でmoduleをロードする。構文を置いた時点の起動成功は、対象依存のロード成功を証明しない。
2. `lazy from` は指定memberだけを部分実行する仕組みではない。最初の利用でmodule全体をロードし、そのとき利用した名前を解決する。他の名前のproxyは残り得る。moduleの初期化が終わっても、未使用memberの存在確認まで終わったとは限らない。
3. moduleロードの `ImportError` や `SyntaxError` は初回利用で発生する。エラーがimport文の周囲からrequest処理やsubcommandの呼出しへ移ることをテストする。
4. function・class body・try/except/finally内の `lazy` 構文、および `lazy from ... import *`、future importのlazy化は構文エラー。optional dependencyをtry内で通常importし、失敗時にfallbackする方式は維持できる。

明示的なlazy化はそのimportに対する選択であり、依存graph全体を再帰的にlazy化する指定ではない。通常importしかないアプリを3.15へ更新しただけで、全依存がlazyになるわけではない。global modeや各moduleのcompatibility指定は別途確認する。[PEPの後方互換性節](https://peps.python.org/pep-0810/#unchanged-semantics)では `__import__()` と `importlib.import_module()` の動的importはeagerのままである。

## 有効化の境界: normalは全面無効化ではない

[sys API](https://docs.python.org/3.15/library/sys.html#sys.set_lazy_imports)と[command-line reference](https://docs.python.org/3.15/using/cmdline.html)で確認したmodeは `normal` と `all`。

- `normal` は既定で、明示的なlazy指定を尊重する。`__lazy_modules__` の指定も有効であり、`normal` に戻すことを「全依存をeagerに戻すkill switch」として扱わない。
- `all` はlazy化可能なtop-level importを候補にする。function・class・try等の制約やstar importの制約まで解除しない。入口は `-X lazy_imports=all`、`PYTHON_LAZY_IMPORTS=all`、`sys.set_lazy_imports("all")`。
- `sys.set_lazy_imports_filter()` はimport元、絶対名へ解決したimport先、fromlistを受ける。`False` はその候補をeagerにする。libraryが呼出し元アプリ全体のmodeを変更することは公式に非推奨。
- filterは**lazy候補のimport文を実行した時点**で判定する。初回利用のたびに改めて判定するaccess-control hookではない。固定版 [Python/import.c](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Python/import.c) の `_PyImport_LazyImportModuleLevelObject` はfilter判定後にproxyを作る。filter自体の例外はこの入口で失敗になり、すべての失敗が初回利用へ移るとは限らない。
- mode/filterの後変更は、既存proxyの一括解決、moduleのunload、初期化副作用の巻戻しを提供しない。eager化が必要な対象は、そのimport文より前にpolicyを決める。

### PEPと配布版の差: noneをrollbackに配布しない

取得した[PEP 810のglobal control](https://peps.python.org/pep-0810/#global-lazy-imports-control)には依然 `none` がある。しかし3.15.1の [Python/sysmodule.c](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Python/sysmodule.c) の `sys_set_lazy_imports_impl` は `normal` / `all` 以外の文字列を `ValueError` にする。[固定版テスト](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/test/test_lazy_import/__init__.py) の `test_global_off_rejected` も `none` 拒否を期待する。

これはrc2限定の注意ではなく、確認した3.15.1でも継続する不一致である。[3.15.0b3のNEWS](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Misc/NEWS.d/3.15.0b3.rst)にも `none` 非対応化が記録されている。`sys.set_lazy_imports("none")` をrollback手順に採用しない。

独自の運用案として、初期導入では `all` を使わず、変更したimportを通常importへ戻し、compatibility指定も外してprocessを再起動できる単位にする。filterによるeager指定を使う場合も候補作成前に設定し、循環依存などeager時にだけ表れる問題を別途試験する。実行済みの外部登録を取り消す仕組みとは考えない。

## 旧版を併存させる __lazy_modules__

[compatibility契約](https://docs.python.org/3.15/reference/simple_stmts.html#compatibility-via-lazy-modules)では、module内の `__lazy_modules__` に完全修飾module名のcontainerを置く。旧Pythonでも構文として読める移行経路であり、旧版でlazy化を再現するpolyfillではない。

- 本稿独自の例 `__lazy_modules__ = {"reporting.render"}` は、そのmoduleを対象にした通常importを3.15でlazy候補にする。
- `from reporting.render import Renderer` の照合対象は `reporting.render`。member名 `Renderer` ではない。
- 相対importは絶対名に解決して照合する。`from .render import Renderer` に対し `".render"` を登録しない。
- 親packageを列挙しただけで全submoduleが対象になるとはしない。function・class・try/except/finally内のimportはこの指定の対象外。
- 旧Pythonでは通常importがeagerのまま。両版でparseできることと、初期化順・循環依存・障害発生時刻が同じであることは別。

固定版 [Python/ceval.c](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Python/ceval.c) の `check_lazy_import_compatibility` は絶対名を取得してcontainerの包含判定を行い、呼出し側はmodule-levelかを確認する。単なる命名上の推奨ではなく照合対象の違いである。

## 初期化・探索状態・失敗再試行

[PEPのreification説明](https://peps.python.org/pep-0810/#what-changes-at-reification-time-what-stays-the-same)と固定版source/testの観察:

- plugin登録だけを目的にimportし、束縛名を使わなければ登録が実行されない可能性がある。必要な登録はeagerに保つか、明示的な初期化関数を起動順序に含める。
- module探索にはreification時点の `sys.path` 等を使う。import宣言後にpathやインストール済み内容を変えても「既に選択済みだから無関係」と考えない。`test_sys_path_at_reification_time_is_used` は初回失敗後にpathを追加し、次の利用で解決するケースを持つ。
- `test_reification_retries_on_failure` は解決失敗後の再利用が再試行になり得ることを確認する上流テストである。業務上安全なretryは保証しない。module初期化が外部登録等を途中まで済ませた場合の冪等性はアプリ側の責務。
- [thread-safetyの説明](https://peps.python.org/pep-0810/#thread-safety-and-reification)は既存import-lock規律を使う。一方、初回利用を行うthreadで初期化が起こり得るため、main-thread専用処理・thread-local設定・実アプリのlock順序が自動的に安全になるとはしない。上流の同一moduleの並行reificationテストは存在を確認したのみで、本調査での実行結果はない。

### 例外のcauseだけでなくnotesも残す

旧rc2記事の解決処理参照先は `Python/import.c` だったが、3.15.1の `_PyImport_LoadLazyImportTstate` は [Objects/lazyimportobject.c](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Objects/lazyimportobject.c) にある。`lazy_import_add_exception_cause` は既存cause/contextや `raise ... from None` がある場合、そのchainを保持して宣言位置を `__notes__` に追加する。そうでない場合は宣言位置のtracebackを持つcauseを付ける経路がある。情報を付加できない場合まで保証する公開APIではない。

独自の運用案として、ログは最終例外の一行や `__cause__` だけに限定せず、traceback chain・notes・初回アクセス位置を保つ。例外を一括で `ImportError` に置換したり、module内部の障害まで「任意dependency未導入」として隠したりしない。

## Introspection境界: sys.lazy_modulesはreadinessではない

[types.LazyImportType](https://docs.python.org/3.15/library/types.html#types.LazyImportType) はproxyの型と `resolve()` を提供する。一方、`globals()` / moduleの `__dict__` を取得しても全proxyが解決されるわけではない。固定版の `test_globals_returns_lazy_proxy_when_accessed_from_function` や `test_globals_dict_access_returns_lazy_proxy_inline` は辞書経由でproxyを観察する経路を持つ。値を別の名前へ束縛して通常参照する経路は異なり得るため、dict経由ならその後も絶対にロードされないとも保証しない。

[sys.lazy_modulesの公開文書](https://docs.python.org/3.15/library/sys.html#sys.lazy_modules)は、このsetをdebugging/introspection用とし、各entryの状態を消費側が確認するよう求める。属性名などmoduleでない文字列や、既にアクセス済みの項目が混じり得る。`lazy from pathlib import Path` に対する `pathlib.Path` はその例である。

[rc3 NEWS](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Misc/NEWS.d/3.15.0rc3.rst)には、既ロードmoduleの追跡や `pkg.attr` entry除去の改善がある。それでも公開契約は厳密な未解決依存一覧を約束していない。したがって以下は本稿独自の設計判断とする:

- `len(sys.lazy_modules) == 0` をreadiness条件、残件数を必須依存の未検証件数にしない。
- set内の文字列をすべて `importlib.import_module()` に渡す「全件warm-up」は作らない。属性名をmodule名と誤認し、任意依存まで意図せず実行する。
- 必須driver・route・pluginの一覧をアプリが別に持ち、実際に必要な初期化関数や代表操作の成功をreadinessに結び付ける。proxyを扱う低levelの箇所は `LazyImportType` / `resolve()` の契約と対応frameworkを個別に試験する。

## rc2からの移行では実際の修正境界を試験する

次は固定した上流NEWSに記録された修正であり、本稿が再現実行した結果ではない。NEWS fragment内の作成日と配布版の公開日を混同しない。

| 配布版と一次根拠 | 確認した修正 | アプリ側で再確認する経路（独自設計） |
|---|---|---|
| [3.15.0rc3 NEWS](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Misc/NEWS.d/3.15.0rc3.rst)、gh-157757 | `lazy import a.b as c` が親の属性 `b` を拾うのではなくsubmoduleを束縛し、対応moduleがなければ `ModuleNotFoundError` にする | packageの同名属性とsubmodule、alias、module欠落の区別 |
| [3.15.1 NEWS](https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Misc/NEWS.d/3.15.1.rst)、gh-158824 | module自身のlazy import解決より先に `__getattr__` が呼ばれ、名前をshadowする問題 | lazy importを公開APIとして再exportし、fallbackの `__getattr__` も持つpackage |
| 同3.15.1 NEWS、gh-158820 / gh-159288 | shutdown中のfinalizerによるlazy宣言・解決のcrash、長いalias proxy chainのcrashを修正。後者は `RecursionError` へ | 終了処理とdeep dotted import。crash修正を無制限のchain対応とは解釈しない |
| 同3.15.1 NEWS、gh-158936 / gh-153888 | 解決不能lazy名を含むdataclass文字列annotation、lazy objectを使う `unittest.mock.patch` の `autospec` / `spec` / `spec_set` を修正 | annotationを実行時評価するframeworkとmock。通常importのtest成功だけで互換としない |

3.15.0 final公開でrc2の注意が全て不要になったわけでも、3.15.1の修正一覧がアプリ互換性を保証するわけでもない。採用するpatch版と実行形態を固定してテスト結果を残す。

## 採用判断と受入テスト（独自設計）

最初の候補は任意subcommandでしか使わず、import-time副作用に依存しない重いmodule。認証設定・route登録・必須driverはreadiness前に明示的に実行する。起動時間だけでなく初回requestの遅延・障害発生時刻も測る。

| 確認する境界 | 受入条件の例 |
|---|---|
| 未使用・初回利用・二回目 | 不要な初期化は行わず、初回機能が成功し、二回目で外部登録を重複させない |
| module欠落・member欠落・module内例外 | 必須依存は起動確認で検出。任意依存だけ限定fallbackへ移し、任意のImportErrorを一括で隠さない |
| plugin・route登録 | readiness前に登録が揃う。別testの先行importや順序に依存しない |
| normal / all / filter | normalは全面無効化でないこと、noneがValueErrorであること、filter設定前後のproxy作成時刻を区別 |
| 旧版と3.15.1 | `__lazy_modules__`を双方で実行し、eagerでだけ現れる循環依存も確認 |
| registry・serializer・sys.lazy_modules | raw namespaceのproxyと診断setを検証し、setの空・非空でreadinessを決めない |
| alias・__getattr__・annotations・mock | rc3/3.15.1 NEWSの修正対象を、アプリが使う経路で確認 |
| 同時初回アクセス・終了処理 | 使用thread構成と対象buildで初期化・lock・例外を確認し、終了時に初めて必須依存をロードしない |
| path変更と例外観測 | 探索状態の変更、失敗後再試行、既存cause/context/notesを記録できる |

テストは独立processで未ロード状態から始める。CLIのhelpが速くなっただけで全subcommandやbackendのready判定を合格にしない。`all` はアプリ所有者が互換試験を用意して導入し、libraryが暗黙に有効化しない。

## 検証範囲・未確認事項・provenance

- 2026-10-24 UTCにrc3/3.15.0/3.15.1公開ページ、Language Reference、sys/types/command-line文書、license、PEP 810をnative webで開いた。旧本文の構文、mode、compatibility、path、proxy、例外、移行判断を全て見直した。
- repositoryは `git ls-remote` の `v3.15.1^{}` とread-only cloneのHEADを照合し、`c37e74cb6bcdadcd5f46f289498f6b3298ddee61` に固定した。本文で挙げた実装・テスト・NEWS・対応するDoc・LICENSEを読んだ。GitHub/rawのnative web取得はcache miss、巨大なrolling changelogはサイズ制限で失敗したため、これらは固定cloneを代替根拠とした。
- repositoryのLICENSEはPSF License Version 2と歴史的license noticesを含む。公式docsはPSF-2.0、文書内codeには0BSDも付く。PEP 810はpublic domainまたはCC0-1.0。releaseページの転載licenseは未確認なので独自要約のみ。codeコピー、module昇格は行っていない。
- この調査環境のPythonは3.12.14。CPython 3.15.1をbuild/runしておらず、引用した上流テストの存在・期待値の確認を実行成功と扱わない。性能倍率、個別framework、free-threaded build、main-thread依存、全release blockerの修正網羅性は未検証。
- PEPに残る `none` と配布版契約の不一致は未解消として明示する。今後の版へ外挿せず、採用版変更時はsource・本文全体・検索evalを再確認する。検索evalは記事を取得できる検証であり、Python runtimeの互換性試験ではない。

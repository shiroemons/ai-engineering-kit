---
{
  "id": "python-lazy-imports-release-readiness-boundary",
  "title": "Python 3.15 lazy imports: 初回利用・登録副作用・移行判定の境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "CPython 3.15.0rc2 (435c9e5a798c99653e3ab64ce29baed0e4f3dfee); rolling 3.15 docs retrieved 2026-10-02; final postponed, rc3 not verified as released",
  "tags": [
    "research-domain:backend",
    "python",
    "lazy-imports",
    "PEP-810",
    "import-time",
    "side-effects",
    "initialization",
    "migration",
    "prerelease",
    "__lazy_modules__"
  ],
  "sources": [
    {
      "id": "python-lazy-315rc2-release-20261002",
      "url": "https://www.python.org/downloads/release/python-3150rc2/",
      "type": "release_notes"
    },
    {
      "id": "python-lazy-315-rc3-delay-20261002",
      "url": "https://discuss.python.org/t/python-3-15-following-tradition-lets-have-a-surprise-rc3/109313",
      "type": "maintainer_article"
    },
    {
      "id": "python-lazy-language-315rc2-20261002",
      "url": "https://docs.python.org/3.15/reference/simple_stmts.html",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-sys-315rc2-20261002",
      "url": "https://docs.python.org/3.15/library/sys.html",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-types-315rc2-20261002",
      "url": "https://docs.python.org/3.15/library/types.html",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-commandline-315rc2-20261002",
      "url": "https://docs.python.org/3.15/using/cmdline.html",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-pep810-20261002",
      "url": "https://peps.python.org/pep-0810/",
      "type": "official_docs"
    },
    {
      "id": "python-lazy-cpython-315rc2-20261002",
      "url": "https://github.com/python/cpython/tree/435c9e5a798c99653e3ab64ce29baed0e4f3dfee",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-10-09",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Python 3.15 lazy imports の移行判定

## 問いと現時点の結論

CLIやbackendの起動時間を減らすため `lazy import` を使うと、依存不足・plugin登録・設定読込はいつ失敗するのか。Pythonを更新すれば一律に有効になるのか。

導入判断は「importを遅らせてもよい依存」と「起動完了前に検証すべき依存」を分ける。既存のPython futures記事は実行queueとshutdown、本稿はmodule初期化と初回利用の契約を対象にする。以下の運用案・受入テストは本稿独自の設計判断であり、Pythonが提供する起動検証機能ではない。

重要なrelease境界:

- [3.15.0rc2公開ページ](https://www.python.org/downloads/release/python-3150rc2/)は2026-09-01公開のpreviewとし、本番利用を推奨していない
- [release managerの2026-10-01告知](https://discuss.python.org/t/python-3-15-following-tradition-lets-have-a-surprise-rc3/109313)は、lazy-import関連のrelease blockersにより3.15.0 finalを延期した。rc3は翌日予定、finalは約1週間後の見込みで、確定日は未告知。古い「2026-10-01 final予定」をリリース済みの根拠にしない
- 2026-10-02 UTCの調査時、`git ls-remote` の `v3.15*` tagはrc2までで、rc3/final公開は確認できなかった。これは今後の公開を否定するものではない。本稿はrc2の固定snapshotを移行調査の比較点にし、最新性・修正完了を保証しない
- `/3.15/`文書は取得時に3.15.0rc2を表示するが更新される文書である。[PEP 810](https://peps.python.org/pep-0810/)の `Final` もPEPの状態であり、Python 3.15.0 finalの配布完了とは別

公開直前の変更を含むため明示期限は1週間とする。rc3/final採用時はsource・release blockers・実アプリのテストを再確認する。

## 公開契約: 遅れるのはmodule初期化と失敗

[Language Reference](https://docs.python.org/3.15/reference/simple_stmts.html#lazy-imports)で確認した範囲:

1. module scopeの `lazy import package` / `lazy from package import member` はproxyを束縛し、名前の初回利用でmoduleをロードする。構文を置いた時点の起動成功は、その依存のロード成功を証明しない
2. `lazy from` は指定memberだけを部分実行する仕組みではない。最初の利用でmodule全体をロードし、そのとき使った名前だけを解決する。他の名前のproxyは残り得る
3. moduleロードの `ImportError` や `SyntaxError` は初回利用で発生する。エラーが「import文の周囲」から「request処理やsubcommandの呼出し」へ移ることをテストする
4. function・class body・try/except/finally内の `lazy` 構文、および `lazy from ... import *`、future importのlazy化は構文エラー。optional dependencyをtry内で通常importし、失敗時にfallbackする方式は維持できる

明示的なlazy化はそのimportに対する選択であり、依存graph全体を再帰的にlazy化する指定ではない。通常importしかないアプリを3.15へ更新したという理由だけで全面的な遅延を前提にしない。ただしglobal modeや各moduleのcompatibility指定を採用した場合は別である。

## 有効化の境界: normalは全面無効化ではない

[sys API](https://docs.python.org/3.15/library/sys.html#sys.set_lazy_imports)と[command-line reference](https://docs.python.org/3.15/using/cmdline.html)の取得時の指定値は `normal` と `all`。

- `normal` は既定で、明示的なlazy指定を尊重する。`__lazy_modules__` による指定も後述の通り有効。`normal` に戻すことを「全依存をeagerに戻すkill switch」として扱わない
- `all` はlazy化可能なtop-level importを対象にする。function・class・try等の制約まで解除しない。設定入口は `-X lazy_imports=all`、`PYTHON_LAZY_IMPORTS=all`、`sys.set_lazy_imports("all")`
- `sys.set_lazy_imports_filter()` はlazy候補ごとにimport元、絶対名へ解決したimport先、fromlistを受ける。`False` はそのimportをeagerにする。libraryが呼出し元アプリ全体のmodeを変更することは公式に非推奨
- filterやmodeを変更する操作を、すでに作ったproxyの全解決・moduleのunload・初期化副作用の巻戻しだとは解釈しない

### PEPと固定版実装の差: noneを配布しない

取得した[PEP 810のglobal control](https://peps.python.org/pep-0810/#global-lazy-imports-control)には `none` の記述が残る。一方、固定したCPython rc2の [Python/sysmodule.c](https://github.com/python/cpython/blob/435c9e5a798c99653e3ab64ce29baed0e4f3dfee/Python/sysmodule.c)の `sys_set_lazy_imports_impl` は `normal` / `all` 以外の文字列を `ValueError` にする。[rc2テスト](https://github.com/python/cpython/blob/435c9e5a798c99653e3ab64ce29baed0e4f3dfee/Lib/test/test_lazy_import/__init__.py)の `test_global_off_rejected` も `none` の拒否を期待する。

したがって `sys.set_lazy_imports("none")` をrollback手順に採用しない。この差はsource読解による確認で、rc3/finalの挙動を断定しない。まず `all` を使わず、変更したimportを通常importへ戻し、compatibility指定も外す、というアプリの変更単位で戻せる設計にする。

## 旧版を併存させる __lazy_modules__

[compatibility契約](https://docs.python.org/3.15/reference/simple_stmts.html#compatibility-via-lazy-modules)では、module内の `__lazy_modules__` に完全修飾module名のcontainerを置く。これは旧Pythonでも構文として読める移行経路であり、旧版でlazy化が再現されるpolyfillではない。

- 例として `__lazy_modules__ = {"reporting.render"}` は、そのmoduleを対象にした通常importを3.15でlazy候補にする
- `from reporting.render import Renderer` の照合対象は `reporting.render`。member名 `Renderer` ではない
- 相対importは絶対名に解決して照合する。`from .render import Renderer` に対し `".render"` を登録しない
- 親packageを列挙しただけで全submoduleが対象になるとはしない。function・class・try/except/finally内のimportはこの指定の対象外
- 旧Pythonでは通常importがeagerのままである。両版でparseできることと、初期化時刻・循環依存・障害発生時刻が同じであることは別

固定rc2の [Python/ceval.c](https://github.com/python/cpython/blob/435c9e5a798c99653e3ab64ce29baed0e4f3dfee/Python/ceval.c)は `check_lazy_import_compatibility` で絶対名を取得し、containerの包含判定を行う。上記は単なる命名上の推奨ではなく、照合対象の違いである。

## 初期化・探索状態・introspectionの落とし穴

[PEP 810のreificationと副作用の説明](https://peps.python.org/pep-0810/#reification)から、初回利用まで登録処理・探索・失敗が遅れる。以下の実装確認はrc2に限定する。

- import文をplugin登録のためだけに置き、束縛名を使わなければ登録が実行されない可能性がある。起動時に必要な登録はeagerに保つか、明示的な初期化関数を起動順序の一部にする
- moduleを探すときはreification時点の `sys.path` 等を使う。import宣言後の探索path変更を「すでに選択済みのmoduleには無関係」と考えない。固定テスト `test_sys_path_at_reification_time_is_used` は、最初の失敗後にpathを追加すると次の利用で解決できるケースを記録する
- 解決失敗後の次の利用は再試行になり得る。これは業務上安全なretryを保証しない。初期化が外部登録等を途中まで済ませていた場合の冪等性はアプリ側で決める
- `globals()` / moduleの `__dict__` を読むだけで全proxyが解決されるとは限らない。[types.LazyImportType](https://docs.python.org/3.15/library/types.html#types.LazyImportType)はproxyの型と `resolve()` を提供する。registryやserializerがnamespaceの値を直接列挙する経路を確認する
- 固定テスト `test_globals_returns_lazy_proxy_when_accessed_from_function` はfunction内の辞書経由でproxyを得るケースを持つ。ただし値を別の名前へ束縛し通常の名前参照で使う経路は異なり得る。dict経由という理由だけで、その後も絶対にロードされないとは保証しない

[Python/import.c](https://github.com/python/cpython/blob/435c9e5a798c99653e3ab64ce29baed0e4f3dfee/Python/import.c)の `_PyImport_LoadLazyImportTstate` も解決時にimport処理を呼び、失敗のexceptionにlazy宣言位置のcauseを付ける。ログでは最後の例外だけを短縮して保存せず、宣言位置と初回アクセス位置を調べられるようにする。並行初回利用の完全な正しさやrelease blockersの解消を、この一関数の読解から保証しない。

## 採用判断と受入テスト（独自設計）

最初の採用候補は、任意subcommandでしか使わず、import-time副作用への依存がない重いmodule。認証設定・route登録・必須driverなど起動検証が必要な経路は、起動完了判定までに明示的に実行する。起動時間だけでなく初回requestの遅延・失敗時刻を測る。

| 確認する境界 | 受入条件の例 |
|---|---|
| 通常起動と未使用依存 | 任意機能を使わない実行が成功し、不要な初期化を行わない |
| 初回利用と二回目 | 機能呼出しが成功し、同じ初期化を不用意に重複させない |
| module欠落・member欠落・module内例外 | 必須依存は起動確認で検出し、任意依存は限定したfallbackへ移る。任意のImportErrorを一括で隠さない |
| plugin・route登録 | readiness前に必要な登録が揃う。全test suiteの先行importによる偶然の成功を避け、新規processでも確認 |
| normal / all / filter | `all`採用前後を比較し、対象外依存とtry内の通常importが意図した時刻にロードされる |
| 旧版と3.15 | `__lazy_modules__`を使う場合も双方で実行し、eagerでのみ見える循環依存や登録漏れを確認 |
| introspection・同時初回アクセス | 使用frameworkのregistry・serializer・thread構成で結果と例外を確認。PEPの説明だけで互換としない |

テストは独立processで未ロード状態から始める。CLIのhelp表示が速くなっただけで、全subcommandやbackendのready判定が正しいとはしない。`all` はアプリ所有者が互換試験を用意して導入し、libraryが暗黙に有効化しない。

## 検証範囲・未確認事項・provenance

- 原文ページを開いて確認した公開契約、rc2固定sourceの観察、独自の運用案を上で区別した。repositoryは `v3.15.0rc2` のdereference先 `435c9e5a798c99653e3ab64ce29baed0e4f3dfee` に固定し、`git ls-remote` とclone後のHEADを照合した
- 取得したrepositoryの `LICENSE` はPSF License Version 2と歴史的license noticesを含む。docsのlicenseはPSF-2.0、文書内codeには0BSDも付く。PEP 810はpublic domainまたはCC0-1.0。releaseページ・延期告知の転載licenseは未確認のため、独自要約のみでcodeのコピーやmodule昇格はない
- GitHub/rawページはnative web取得に失敗したため、固定tagのread-only cloneを代替に使った。公開docs・PEP・release/延期告知のnative web取得は成功した
- この調査環境のPythonは3.12.14。CPython 3.15をbuild/runしていないため、上流テストの存在確認を実行成功とは扱わない。性能倍率、framework互換性、free-threaded実行、rc3のblocker修正内容、final版の契約は未検証
- finalの確定日・公開を予定から推測しない。再調査時は実際に採用するversionとSHAを固定し、この文書全体・source records・検索evalを更新する

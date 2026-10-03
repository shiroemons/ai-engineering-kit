---
{
  "id": "testing-pytest-importorskip-transitive-collection-boundary",
  "title": "pytest 9.1 importorskip: 任意依存・壊れたimport・収集件数の境界",
  "kind": "knowledge",
  "technology": "testing",
  "version": "pytest 9.1.0 (2026-06-13) default change; 9.1.1 commit cf470ec0bf7eb89cd97dd56df4859eae5db46447; stable docs and Python 3.14.8 exception docs checked 2026-10-03 UTC",
  "tags": [
    "research-domain:quality-operations",
    "testing",
    "pytest",
    "importorskip",
    "ModuleNotFoundError",
    "ImportError",
    "exc_type",
    "minversion",
    "collection",
    "optional-dependency",
    "transitive-dependency"
  ],
  "sources": [
    {
      "id": "pytest-importorskip-migration-20261003",
      "url": "https://docs.pytest.org/en/stable/deprecations.html#import-or-skip-import-error",
      "type": "official_docs"
    },
    {
      "id": "pytest-importorskip-api-20261003",
      "url": "https://docs.pytest.org/en/stable/reference/reference.html#pytest-importorskip",
      "type": "official_docs"
    },
    {
      "id": "pytest-importorskip-outcomes-9-1-1-20261003",
      "url": "https://github.com/pytest-dev/pytest/blob/cf470ec0bf7eb89cd97dd56df4859eae5db46447/src/_pytest/outcomes.py",
      "type": "github_repository_analysis"
    },
    {
      "id": "pytest-importorskip-tests-9-1-1-20261003",
      "url": "https://github.com/pytest-dev/pytest/blob/cf470ec0bf7eb89cd97dd56df4859eae5db46447/testing/test_runner.py",
      "type": "github_repository_analysis"
    },
    {
      "id": "pytest-release-9-1-dates-20261003",
      "url": "https://docs.pytest.org/en/stable/changelog.html#pytest-9-1-0-2026-06-13",
      "type": "release_notes"
    },
    {
      "id": "pytest-skip-scope-reporting-20261003",
      "url": "https://docs.pytest.org/en/stable/how-to/skipping.html#skipping-on-a-missing-import-dependency",
      "type": "official_docs"
    },
    {
      "id": "python-import-exceptions-3-14-20261003",
      "url": "https://docs.python.org/3.14/builtins/exceptions.html#ModuleNotFoundError",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# pytest 9.1 importorskip: 任意依存・壊れたimport・収集件数の境界

## 解く問いと適用範囲

optional dependency のテストを `pytest.importorskip` で省略するとき、未インストールと壊れた環境をどう区別し、CIが成功しても必要なテストが消える事態をどう検出するか。既存のVitest非同期assertion文書が扱うawaitとは別に、テスト収集・import・skipの契約を扱う。

[変更履歴](https://docs.pytest.org/en/stable/changelog.html#pytest-9-1-0-2026-06-13)では9.1.0は2026-06-13、9.1.1は2026-06-19に公開されている。本調査は10月の新機能発表ではなく、未収録だった9.1移行の品質上の意味を確認するもの。2026-10-03 UTCのstable文書に加えて、9.1.1 tagをcommit `cf470ec0bf7eb89cd97dd56df4859eae5db46447` へ解決し実装とテストを読んだ。後続版・任意pluginで同じ動作を保証しない。

## 確認した契約

### 1. 既定の例外型変更は、壊れたimportを可視化する

[移行文書](https://docs.pytest.org/en/stable/deprecations.html#import-or-skip-import-error)は8.2で `exc_type` が導入され、9.1から未指定時の捕捉対象が `ModuleNotFoundError` になったと記載する。従来の `ImportError` 捕捉では、パッケージが存在してもインストールやコンパイルの問題でimportできない場合をskipにできてしまった。

[API](https://docs.pytest.org/en/stable/reference/reference.html#pytest-importorskip)上、`exc_type=ImportError` の明示指定は引き続き可能である。しかし、CIを緑に戻すために一律に指定すると、その不具合を再びskipへ戻す。8.2以降との共有コードでは `exc_type=ModuleNotFoundError` を明記して意図を示せるが、それより前の版へこの引数を無条件に持ち込まない。

固定版の[upstreamテスト](https://github.com/pytest-dev/pytest/blob/cf470ec0bf7eb89cd97dd56df4859eae5db46447/testing/test_runner.py)には、見つからないmoduleを既定でskip、単なる `ImportError` を既定で伝播、明示的な `exc_type=ImportError` ではskip、という期待値がある。test本体で壊れたimportを行うintegration caseはfailed=1を期待する。収集時やfixture setup時に起きる失敗を、すべてtest本体のfailureと同じ報告分類だとはみなさない。

### 2. ModuleNotFoundErrorは「指定moduleだけがない」という証明ではない

[Python例外仕様](https://docs.python.org/3.14/builtins/exceptions.html#ModuleNotFoundError)では `ModuleNotFoundError` は `ImportError` のsubclassで、moduleが見つからない場合のほか `sys.modules` に `None` がある場合にも発生する。`ImportError.name` と `path` は診断情報であり、インストール済みdistributionの完全性証明ではない。

固定版の[実装](https://github.com/pytest-dev/pytest/blob/cf470ec0bf7eb89cd97dd56df4859eae5db46447/src/_pytest/outcomes.py)は `importlib.import_module(modname)` 全体を指定の例外型で囲み、捕捉した `exc.name` が要求した `modname` と一致するかを検査しない。このコードから導ける境界は次のとおり。

- `adapter` 自体がないため `ModuleNotFoundError` になる場合、既定でskipとなる
- 存在する `adapter` が内部で `codec_backend` をimportし、そちらがないため同じ例外型になる場合も、例外が伝播してくれば既定でskipとなる
- 存在する `adapter` が通常の `ImportError`、`RuntimeError` など別の例外を出す場合、既定の捕捉対象にはならない

後二つの区別は9.1への更新だけで解消しない。推移依存が欠けた環境まで「任意のadapterを入れていないので正常」と認定しない。上記は固定実装の読解による結論であり、各third-party importerやwheelでの実行結果ではない。

### 3. importの配置によって未実行の範囲が変わる

[skipガイド](https://docs.pytest.org/en/stable/how-to/skipping.html#skipping-on-a-missing-import-dependency)はmodule level、test内、setupでの使用を説明する。固定実装が作るskipはmodule levelを許可するため、module直下に置くとそのmoduleの収集全体へ作用する。固定版の `test_importorskip_module_level` は「収集0項目・module skip1件」を期待する。これを、そのファイルに書かれたテストがすべて実行済みという結果へ変換しない。

test内ならそのtest、fixtureであればそれを必要とするtestのsetupに影響する。無関係な必須テストとoptional adapterテストを同じmoduleに置いたまま、先頭の `importorskip` でまとめて省略しない。

`minversion` は[API](https://docs.pytest.org/en/stable/reference/reference.html#pytest-importorskip)が示すmoduleの `__version__` を使う。固定実装ではそれがない場合もskipとなる。distribution metadataやlockfileの版を比較するAPIではなく、import成功だけでも十分な機能確認ではない。

### 4. 独自reasonは診断を隠すことがある

固定実装の既定reasonには要求したmodule名と捕捉例外が入る。呼出側が `reason` を渡すとその文字列を用いるため、常に「optional dependency未導入」とだけ書くと、上記の推移依存欠落との違いが見えにくくなる。[skipガイド](https://docs.pytest.org/en/stable/how-to/skipping.html#skipping-on-a-missing-import-dependency)ではskip詳細は既定で省かれ、`-rs` などで表示できる。表示を増やすこと自体はskipを失敗に変更する設定ではない。

## 採用判断と回帰確認（独自の設計案）

次の二つのCI条件を別々に用意する。これはpytestの自動保証ではなく、上記の契約からの運用提案である。

1. 最小依存jobでは、本当にサポート対象外のoptional adapterだけがskipされることを、module名・理由・収集件数で確認する
2. 完全依存jobでは、必要なadapterと推移依存を含む環境で通常のimport確認を行い、必須のテストIDが収集・実行されたことを照合する。その確認までimportorskipで包まない
3. 各importorskipを、許容する不足、捕捉する例外型、影響するmodule/fixture/testに分類する。広い `ImportError` 捕捉を残すなら、その理由と期限を局所的に記録する
4. 独自の小さなfixture packageで以下のケースを分離し、採用するpytest・Python・pluginの組合せごとに観測する。実サービスを壊して試験しない

| 入力条件 | 9.1.1の既定処理から期待される境界 | jobで確かめること |
|---|---|---|
| 指定module未導入 | ModuleNotFoundErrorをskipへ変換 | 最小依存jobだけで許すか |
| 指定moduleは存在し推移依存欠落 | 同じ例外型ならskipへ変換 | 完全依存jobが不足を検出するか |
| 指定moduleが通常のImportErrorを送出 | skipへ変換せず伝播 | import破損を失敗として観測できるか |
| 明示exc_type=ImportError | 広い捕捉に戻る | 互換指定が欠陥を隠さないか |
| minversion不足・__version__なし | import後でもskip | 期待するadapter版のtestが実行されたか |
| module levelでskip | moduleの収集を中断 | 必須testが巻き込まれないか |

単にskip件数が減ったことやCI終了が成功したことを、coverageが保たれた根拠にしない。9.1移行前後のtest ID、収集数、skip理由、実行phaseを保存して比較する。予期せず消えた必須caseは再試行回数を増やすより先に環境と収集条件を調べる。

## 未確認事項と由来

この調査ではpytest本体・upstreamテストの実行、wheelのABI不整合、plugin独自skip、ユーザー環境のcoverage測定は未実施。上表は一次資料と固定実装に基づく期待であり、実機試験結果ではない。検索evalは文書の発見性だけを検証する。

固定blobのWeb取得はcache missだったためGitHub connectorの同一40桁refから本文を取得して確認した。[pytest MIT license](https://docs.pytest.org/en/stable/license.html)と同commitのLICENSEを確認済み。Python文書のfooterはPSF-2.0、例示コードは追加で0BSDと記載する。ここには実装を転載せず独自要約と独自の検証案だけを記録した。release notesのTTL30日が最短なので、再確認期限は2026-11-02とする。

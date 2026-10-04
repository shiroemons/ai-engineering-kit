---
{
  "id": "testing-hypothesis-seed-cache-replay-boundary",
  "title": "Hypothesis 6.168.2: seed固定・共有constants cache・回帰入力保存の境界",
  "kind": "knowledge",
  "technology": "testing",
  "version": "Hypothesis 6.168.2 fix (2026-09-27); implementation merge ff7e800971bacfabe4f498bc883c3fafc16c797f; API/tutorial docs rendered 6.168.3, checked 2026-10-03 UTC; pytest-xdist stable docs unversioned",
  "tags": [
    "research-domain:quality-operations",
    "testing",
    "Hypothesis",
    "property-based-testing",
    "seed",
    "constants-cache",
    "atomic-write",
    "pytest-xdist",
    "collection",
    "ExampleDatabase",
    "reproduce_failure",
    "regression"
  ],
  "sources": [
    {
      "id": "hypothesis-seed-cache-release-61682-20261003",
      "url": "https://hypothesis.readthedocs.io/en/latest/changelog.html#v6-168-2",
      "type": "release_notes"
    },
    {
      "id": "hypothesis-seed-cache-implementation-20261003",
      "url": "https://github.com/HypothesisWorks/hypothesis/blob/ff7e800971bacfabe4f498bc883c3fafc16c797f/hypothesis/src/hypothesis/internal/constants_ast.py",
      "type": "github_repository_analysis"
    },
    {
      "id": "hypothesis-seed-cache-upstream-test-20261003",
      "url": "https://github.com/HypothesisWorks/hypothesis/blob/ff7e800971bacfabe4f498bc883c3fafc16c797f/hypothesis/tests/cover/test_constants_ast.py",
      "type": "github_repository_analysis"
    },
    {
      "id": "hypothesis-seed-replay-api-20261003",
      "url": "https://hypothesis.readthedocs.io/en/latest/reference/api.html",
      "type": "official_docs"
    },
    {
      "id": "hypothesis-replay-regression-tutorial-20261003",
      "url": "https://hypothesis.readthedocs.io/en/latest/tutorial/replaying-failures.html",
      "type": "official_docs"
    },
    {
      "id": "hypothesis-flaky-external-state-20261003",
      "url": "https://hypothesis.readthedocs.io/en/latest/tutorial/flaky.html",
      "type": "official_docs"
    },
    {
      "id": "pytest-xdist-collection-consistency-20261003",
      "url": "https://pytest-xdist.readthedocs.io/en/stable/known-limitations.html",
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

# Hypothesis 6.168.2: seed固定・共有constants cache・回帰入力保存の境界

## 解く問いと適用版

同じseedで動かすproperty-based testingが並列CIだけで異なる入力を作るとき、乱数seed、生成に使うcache、失敗入力の保存をどう切り分けるか。結論は、seed固定を回帰入力の永続保証にせず、cache修正の適用と明示的な回帰caseを別々に確認すること。

[6.168.2変更履歴](https://hypothesis.readthedocs.io/en/latest/changelog.html#v6-168-2)は2026-09-27、共有する `.hypothesis` 配下のsource constants cacheを非原子的に書いていたため、並列readerが途中の内容を受け取り、固定seedでも生成データが変わる問題を修正したと記載する。pytest-xdistは影響する実行形態の例である。6.168.3は2026-09-28の別の統計処理性能修正であり、本件の導入版と混同しない。

実装は修正PR #4886のmerge commit `ff7e800971bacfabe4f498bc883c3fafc16c797f` に固定した。これはrelease tagを解決したSHAとの主張ではない。公開文書は2026-10-03 UTCに表示された6.168.3を確認した。既存のpytest importorskip文書が扱う依存不足によるskipとも、一般的なflaky retry文書とも異なり、生成入力の再現条件を扱う。

## 一次資料から確認した境界

### 1. 修正対象はsource constants cacheであり、失敗入力DBではない

固定した[constants_ast.py](https://github.com/HypothesisWorks/hypothesis/blob/ff7e800971bacfabe4f498bc883c3fafc16c797f/hypothesis/src/hypothesis/internal/constants_ast.py)はlocal moduleのsourceから定数を抽出し、`storage_directory("constants")` に保存する。keyはsource bytes由来のhashとlimit指定で決まる。修正は、同じdirectoryのtemporary fileへ書き終えてから `os.replace` する方式への変更である。

一方、[API Reference](https://hypothesis.readthedocs.io/en/latest/reference/api.html#hypothesis.settings.database)の `settings.database` は失敗test caseを保存・再利用するExampleDatabaseで、未設定時の通常の保存先は `.hypothesis/examples`。同じ親directoryにあっても用途が異なる。`database=None` は失敗入力の保存を止める設定であり、これだけでconstants cacheも無効になるという説明はない。

### 2. 原子的writeは、既存cacheの完全性検査を意味しない

同じ固定実装ではcacheを読んでASTとしてparseできると、その結果を先に返す。parse等で例外になった場合は元sourceから計算し直すが、構文上有効な空fileなら空の定数集合として処理できる。cache内容とsourceを突き合わせる完全性比較は、このread経路にない。

ここから導く実装読解上の結論は、更新後に新しく作るfileの部分読み取り対策と、既に残った「parse可能だが不完全な旧cache」の修復は別、ということ。6.168.2へ更新しただけで過去のcacheがすべて検証・修復されたとは判定できない。実環境で破損が残っていると断定するものでもない。

write経路の例外は捕捉され、計算済み定数を返す。したがってtestの正常終了はcache永続化成功の証明にもならない。これを任意filesystemでのdurability、crash recovery、例外時のtemporary file cleanupまで保証する契約へ広げない。

### 3. upstreamの回帰testが確かめる範囲にも限界がある

固定した[upstream test](https://github.com/HypothesisWorks/hypothesis/blob/ff7e800971bacfabe4f498bc883c3fafc16c797f/hypothesis/tests/cover/test_constants_ast.py)の `test_cache_file_is_written_via_a_temporary_file` は空の保存先でtemporary fileの作成を記録し、保存内容のround-trip、作成回数、成功後にtemporary pathが残らないことをassertする。

このtestの存在は、複数processが競合するtimingを再現した実験結果ではない。Windowsのfile lockやnetwork filesystem、`os.replace` 失敗時の片付け、既存のparse可能な不完全cacheの修復を網羅した試験とも読まない。本調査ではupstream testを実行していない。

### 4. seed・再現blob・明示入力の役割を分ける

[API](https://hypothesis.readthedocs.io/en/latest/reference/api.html#hypothesis.seed)のseed再現条件には、timing・hash randomization・外部状態など他の非決定性がないことが含まれる。`derandomize=True` もHypothesis、Python、test関数の更新をまたぐ入力列の永続保証ではない。`@reproduce_failure` は特定版の一時的な再現手段であり、別Hypothesis版での互換利用は保証されない。

[再現ガイド](https://hypothesis.readthedocs.io/en/latest/tutorial/replaying-failures.html#prefer-example-over-the-database-for-correctness)は、版更新やtest変更でExampleDatabaseの内容が使われなくなる場合があるため、必須入力を `@example` として明示することを推奨する。明示入力自体はshrinkされない。再現blobを恒久fixtureとして保存するのではなく、原因が分かった後に読める入力・期待値へ落とす。

ただし `@example` を書けば無条件で実行されるわけではない。[phase契約](https://hypothesis.readthedocs.io/en/latest/reference/api.html#hypothesis.settings.phases)では `Phase.explicit` が明示入力、`Phase.reuse` が過去の失敗入力を扱う。`Phase.explicit` を除外したprofileで回帰入力を走らせたと報告しない。現行の組込みci profileは `database=None` なので、directoryを保存した事実だけでCIが失敗入力DBを利用したとも言えない。

### 5. 並列収集の一致と、test内部の再現性は別の観測対象

[pytest-xdistの制約](https://pytest-xdist.readthedocs.io/en/stable/known-limitations.html#order-and-amount-of-test-must-be-consistent)ではworker間で収集testの順番と件数が一致する必要がある。unorderedな値からparametrizeすると順番差が問題になる。

独自の判断として、値集合が同じで順序だけ異なる場合はsortが候補になるが、生成された値自体が異なるならsortだけでは解決しない。収集時のtest ID不一致を、通常の `@given` 本体でのassertion失敗と同じログ分類にまとめない。

[flakyガイド](https://hypothesis.readthedocs.io/en/latest/tutorial/flaky.html)は、Hypothesisから見えるtest結果とdraw列の再現性を要求する。入力間でresetされないfile・DB・global state、thread schedulingなどは別の原因となる。cache修正版でなお不安定な場合、seedを追加するだけで `FlakyFailure` や `FlakyStrategyDefinition` を解消したと扱わない。

## CIでの切り分け手順（独自の設計案）

以下は上記の契約から組み立てた運用案であり、本調査で測定した再現率やupstreamの保証ではない。

1. 最初に失敗phaseを記録する。収集時ならworkerごとのtest IDの順序・件数・生成parameter、実行時なら失敗入力・例外・drawに影響するstateを保存する。単に「retryで通った」とだけ残さない
2. Hypothesis/Python/pytest/xdistの版、testのGit revision、seedまたはderandomize、実効profile、phases、database、worker数、OS、保存directoryの共有範囲を記録する。設定fileに書いた値と実際に有効な値を区別する
3. 6.168.2の修正を含む採用版か確認する。そのうえで、停止した隔離環境のcacheを保全して、同じrevision・設定で「既存constants cache」と「新しいconstants保存先」を比較する。共有directoryを実行中に削除しない。失敗入力DBと混同して `.hypothesis` 全体を無条件に消さない
4. 単一workerと複数worker、cold cacheとwarm cacheを別条件にする。どの条件でparameter列や収集IDが変わるかを見る。parallel runが一度通ったことをrace不存在の証拠にしない
5. 原因入力を再現できたら、要求仕様を表す明示的な回帰testまたは `@example` を追加する。重要caseの実行を通常の収集結果とphase設定で確認する。再現blobは調査中の同版環境に限定する
6. 探索の強さと回帰の固定を分ける。高速な決定的jobに加え、許容予算内で異なる入力を探索するjobを設計する場合も、既知の回帰入力は両者の偶然の生成に依存させない。固定seedのjobが繰り返し緑でも未探索入力への正しさは保証しない

調査用manifestにcache内容そのものを公開する必要はない。source由来の定数や失敗入力に秘密・個人情報が含まれ得る環境では、保存先のaccessとretentionを確認し、共有する診断を最小化する。これは本ライブラリの自動redaction機能との主張ではない。

## 判定例と避ける誤認

| 観測・設定 | そこから言えること | 追加で必要な確認 |
|---|---|---|
| seedが同じなのにworkerごとの収集IDが違う | 入力生成または収集順序に差がある | 版、constants cache、実際の値集合、外部状態を分離 |
| database=Noneでもconstants directoryがある | 失敗入力DBとsource constants cacheは別用途 | directoryの存在だけでDB利用・未利用を判定しない |
| 6.168.2へ更新後も旧cacheを復元している | 新write方式を採用していても旧fileを読める | 既存fileとfresh環境の比較、cacheの復元経路 |
| explicit caseを追加したがgenerateのみのprofile | 明示回帰caseを実行した証拠がない | Phase.explicitを含む実効設定と実行結果 |
| 再現blobを残してHypothesisを更新した | 同版再現の前提が変わった | 入力を明示的な回帰testへ移したか |
| upstreamのtemporary-file testがある | そのassertionが保護するwrite構造は分かる | 実運用の並行・filesystem条件は別途検証 |

## 未確認事項・出典の扱い

- wheelのinstall、Hypothesis/xdistの実行、race再現、OS別のfile operation、実際のcache破損率は未検証。本文の実装読解・公式契約・独自検証案を実機試験結果と混同しない
- 本調査の検索evalは発見性のみを確認し、seedの決定性やcache修復を実行検証しない
- replay tutorialには既存失敗の再利用を `Phase.explain` と書いた箇所があるが、同版API Referenceは `Phase.reuse` と明示する。本文のphase分類はAPIを採用し、この文書差を隠さない
- native Webで公式文書を開いて確認した。固定GitHub blobのWeb取得はcache missだったため、同じ40桁refをGitHub connectorで取得した。想定した旧命名release tagと旧 `hypothesis-python/` pathは404であり、release-tag検証済みとはしていない
- 固定実装・testのfile headerと[LICENSE.txt](https://github.com/HypothesisWorks/hypothesis/blob/ff7e800971bacfabe4f498bc883c3fafc16c797f/LICENSE.txt)はMPL-2.0。実装は転載していない。公開文書の独立したlicenseは確認できずcatalogではunknownとし、短い独自要約と帰属のみを記録した
- release notesのTTL30日が最短なので再確認期限は2026-11-02。以後の版・他backend・plugin固有挙動へ無条件に拡張しない

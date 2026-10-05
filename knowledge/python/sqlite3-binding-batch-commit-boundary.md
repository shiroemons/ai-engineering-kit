---
{
  "id": "python-sqlite3-binding-batch-commit-boundary",
  "title": "Python sqlite3: named binding移行・一括実行の部分成功・commit境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "Python 3.14.8 documentation / CPython v3.14.8 @ 8e6e75d9102e39bed2a2b279203a396741180f12 (2026-09-30); 3.13.16 docs comparison; limited local observations on CPython 3.12.14 + SQLite 3.53.1, not 3.14 runtime validation",
  "tags": [
    "research-domain:backend",
    "python",
    "sqlite3",
    "DB-API",
    "named-placeholders",
    "executemany",
    "autocommit",
    "RETURNING",
    "transaction-boundary"
  ],
  "sources": [
    {
      "id": "python-sqlite3-binding-api-3148-20261005",
      "url": "https://docs.python.org/3.14/library/sqlite3.html",
      "type": "official_docs"
    },
    {
      "id": "python-sqlite3-binding-api-31316-20261005",
      "url": "https://docs.python.org/3.13/library/sqlite3.html",
      "type": "official_docs"
    },
    {
      "id": "python-sqlite3-binding-whatsnew-314-20261005",
      "url": "https://docs.python.org/3.14/whatsnew/3.14.html",
      "type": "release_notes"
    },
    {
      "id": "python-sqlite3-binding-release-3148-20261005",
      "url": "https://www.python.org/downloads/release/python-3148/",
      "type": "release_notes"
    },
    {
      "id": "cpython-sqlite3-binding-3148-8e6e75d9-20261005",
      "url": "https://github.com/python/cpython/tree/8e6e75d9102e39bed2a2b279203a396741180f12",
      "type": "github_repository_analysis"
    },
    {
      "id": "python-sqlite3-binding-license-3148-20261005",
      "url": "https://docs.python.org/3.14/license.html",
      "type": "official_docs"
    },
    {
      "id": "pep249-batch-results-20261005",
      "url": "https://peps.python.org/pep-0249/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2026-11-04",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Python sqlite3: 入力拒否・反復完了・永続化を別々に判定する

**判断:** Python 3.14へ移行する際は、SQLのplaceholder表記と渡すPython型を一組で確認する。`executemany()` の例外はバッチ全体の未実行やrollbackを保証しない。入力検査、transactionの所有者、結果の回収、commit成功を分けて設計する。

本稿はPython標準 `sqlite3` のDB-API境界を扱う。既存の[SQLite WAL競合待ち](../sqlite/wal-busy-timeout.md)や[backup完了判定](../sqlite/backup-done-finish-publication-boundary.md)のengine側運用とは異なる。全corpusの検索では `executemany` の記録がなく、直近16件にも同じ論点がなかったため、未収録の実務上の穴を選んだ。2026年10月に新設された機能という主張ではない。

## 1. 適用版と移行理由

- [3.14 What's New][new] は、3.12以来非推奨だった「named placeholderにsequenceを渡す」呼出しを、3.14から `ProgrammingError` に変更したと記す。3.14系列の公開日は2025-10-07
- [比較用3.13.16 API][api13] はこの経路を `DeprecationWarning` と記す。警告が普段見えない環境で動作していたことは3.14での互換性を保証しない
- [今回開いた公式API][api] の表示版は3.14.8。[配布告知][release] の公開日は2026-09-30。確認対象のtagをGitHub APIで剥離し、commit `8e6e75d9102e39bed2a2b279203a396741180f12` を得た
- Python版と組込みSQLite版は別に記録する。What's Newは `sqlite3.version` / `version_info` の削除を説明し、実行中engineの版には `sqlite3.sqlite_version` / `sqlite_version_info` を使う

以下では公式契約、固定sourceの読解、古いPythonでの限定実測、独自の設計案を区別する。3.14.8のruntimeはこの環境で実行していない。

## 2. named placeholderと位置引数は交換できない

[APIのbinding説明][api]に沿って、SQLとデータ構造を対応させる。

| SQL側 | Python側 | 検査上の注意 |
|---|---|---|
| `?` というqmark | 要素数が一致するsequence | 個数が違えばProgrammingError |
| `:tenant` などnamed placeholder | 必須キーを全て持つdictまたはそのsubclass | 足りないキーは失敗し、余分なキーは無視される |
| `:1` のような数値名 | 名前として解釈される | PEP 249のnumeric位置指定だと思ってtupleを渡さない |

[PEP 249][pep]が例示するnumericの `:1` と、SQLiteの `?1` は区別する。[固定版のtest_dbapi.py][dbtest]には `SELECT ?2, ?1` へ `(1, 2)` を渡して `(2, 1)` を得る試験がある。これはindexed nameless parameterのsource上の確認であり、「数字を含むplaceholderは全て禁止」という説明は誤りになる。

[固定cursor.c][cursor]はsequenceをbindするとき、名前があり先頭が `?` でないparameterを拒否する。namedとqmarkが混在するSQLも、この拒否経路へ入り得る。driver間移植を考える設計案としては、plain `?` と明示的なnamedの二方式に揃え、独自の数値表記変換を増やさない。

また、余分なdictキーが無視される仕様は、入力の全fieldが保存された証拠にはならない。独自の入力schemaで必須・許可キーを検査し、SQLから抜けた列や綴り違いを別に検出する。tupleをSQL文字列へ展開してこの型検査を回避せず、値はbindingで渡す。

## 3. executemanyは全件事前検査でも一括commitでもない

[API][api]の `executemany(sql, parameters)` は、一つのDMLを各itemについて反復する。`execute()` と同じ暗黙transaction処理を使う。[固定cursor.c][cursor]の順序は、iteratorから一件取得、bind、SQLite実行、次の一件である。バッチ全体を先に検証する処理はない。

したがって、最初のdictが正常で二番目の必須キーが欠けていれば、二番目のbinding失敗より前に最初のDMLが実行済みとなり得る。3.14で途中itemがtupleになった場合の型拒否も、すでに処理したitemまで取り消す指示ではない。iteratorやadapterが途中で例外を出す経路も、入力生成の外部副作用までtransactionで巻き戻せるとは限らない。

error処理で文全体をresetすることとtransactionのrollbackは別である。固定sourceは失敗時に `rowcount = -1` を設定するため、失敗後のrowcountから「成功したprefixは0件」や「この件数だけを再送すればよい」とは判断できない。処理済み集合を再構成するなら、安定した業務キーとcommit範囲を利用する必要がある。

## 4. autocommitの三方式とwith conの責務

[API][api]と[固定connection.c][connection]に照らすと、3.14.8でも既定値は `LEGACY_TRANSACTION_CONTROL` である。`autocommit` 引数の追加は3.12で、既定値がすでにFalseへ変わったという意味ではない。

- `autocommit=False`: driverはtransactionを開いた状態に保ち、commit / rollback後に新しいtransactionを開始する。`in_transaction=True` が残ることだけで前回commitの失敗とはいえない
- `autocommit=True`: `Connection.commit()` と `rollback()` は何もしない。`with con` もtransactionの成功・失敗処理を行わない。明示SQLで `BEGIN` した場合でも、この属性のままPythonのcommit methodに終了を任せない
- legacy: `isolation_level` がNoneでなく、対象DMLを実行し、まだtransactionがなければ暗黙に開始する。`isolation_level=None` はこの暗黙開始を止める。`isolation_level` は他のautocommit方式を制御しない

`with con` は接続をcloseする構文ではなく、進行中transactionの成功・失敗を扱うconnection context managerである。入口で独立transactionやsavepointを作らない。[固定source][connection]の `__enter__` は接続自身を返し、`__exit__` は未捕捉例外の有無でcommit / rollbackを選ぶ。内側でも同じ接続を `with` で囲めば、自動的にネストrollback範囲ができるという契約ではない。

重要なのは例外を捕捉する場所である。全件失敗にしたい処理で `with con` の内側に `try/except ProgrammingError` を置いて正常終了すると、出口からは成功に見え、実行済みprefixをcommitし得る。例外を外へ伝えるか、明示的な失敗処理を所有者へ返す。commit自体の失敗も成功に書き換えず扱う。

## 5. RETURNING・lastrowid・rowcountは別の証拠

[API][api]は、`executemany` が生成した行を破棄し、`RETURNING` も対象になるとする。成功した呼出しであっても、後からfetchして各入力の生成IDを回収できる設計にしない。[PEP 249][pep]でもexecutemanyによるresult setの扱いは一般には未定義で、他driverへsqlite3の振る舞いを一般化できない。

さらに、API上の `lastrowid` は成功した `execute()` のINSERT / REPLACEで更新され、executemany / executescript後は更新されない。以前の値が残っていても今回の全件成功や最後の入力との対応を示さない。`rowcount` は行数でありID対応表ではなく、結果を返すstatementでは完了までfetchすることが更新条件になる。

独自の設計案: 各入力の結果が必要なら、同じtransaction内で一件ずつ `execute(... RETURNING ...)` し、その結果を最後まで回収して業務キーと対応付ける。その後のcommit成功まで外部へ確定結果として通知しない。単一の複数行statementへ組み替える場合も、入力順と戻り順の対応を無検証で仮定しない。返却行の破棄という仕様は、どのPython/SQLite組合せでもRETURNING付き一括実行が例外なく完走する保証ではない。

## 6. executescriptは失敗時の逃げ道にならない

[API][api]と[固定cursor.c][cursor]は、legacy方式でpending transactionがあると `executescript()` がscript開始前にCOMMITすると示す。それ以外の暗黙transaction管理は行わず、必要なSQLによる制御はscript自身の責務となる。[固定transaction試験][txtest]は、False / True / legacyを分け、legacyだけで事前COMMITが現れることを確認する。

このため「既存更新、その後にscriptを実行、script失敗を外側のwithがrollback」という形でも、事前commit済みの更新まで戻るとは限らない。script内に `BEGIN; ...; COMMIT;` を書いても、最後のCOMMITより前で失敗した時の終了処理は別に必要で、先の事前commitを取り消せるわけでもない。

設計案として、値を伴うバッチを文字列連結したscriptへ変換しない。schema scriptとアプリDMLのtransaction所有者を分離し、呼出し前のpending状態と終了処理を明記する。`executescript` は複数statement実行用であり、parameter引数付きexecuteの単なる短縮形ではない。

## 7. この環境での限定実測

実行したのは **CPython 3.12.14、SQLite 3.53.1** の組合せである。engineが返したsource-idは `2026-05-05 10:34:17 c88b22011a54b4f6fbd149e9f8e4de77658ce58143a1af0e3785e4e6475127e9`。各ケースを別processのin-memory databaseで試し、table作成は先にcommitした。現行3.14.8やファイル耐久性の実測ではない。

- 正常dictの後に必須キー不足dictを渡す二件のexecutemany: False / legacyでは例外をwithの外で捕捉すると残存0件、内で捕捉すると残存1件だった
- 同じ試験のTrue方式: 捕捉位置にかかわらず残存1件だった
- pending更新の後、insertと存在しないtable参照を含むscriptをwith内で実行: Falseでは残存0件、legacyではscript前とscript中の2件が残った
- True方式でSQLのBEGINとinsertの後にPythonのcommit methodを呼ぶと、`in_transaction` はTrueのままだった。後始末にはSQLのROLLBACKを使った
- RETURNINGの試行は公式記述と異なる結果だった。この配布buildでは一件のexecutemanyから行を取得でき、二件では一件処理後に **InterfaceErrorで失敗した**。対照のRETURNINGなし二件と、一件ずつexecuteしてfetchする方式は成功した。三つのautocommit方式で同じ傾向を確認したが、原因・影響版は不明で、3.14.8のupstream不具合とは断定しない。結果破棄の仕様をruntimeで検証できたとは数えない

上の残存件数は同じ接続からSELECTした観測であり、別接続からの可視性・fsync・電源断後の耐久性まで実証するものではない。3.14のnamed＋sequence拒否は公式文書と固定upstream試験の読解であり、3.12上の結果で代用しない。

## 8. 採用時の判断手順と受入条件

以下は独自の運用案で、公式の万能テンプレートではない。

1. SQL、placeholder方式、各itemのPython型、Python版、SQLite版を揃えて記録する。3.12/3.13のwarningもCIで確認する
2. 一つのbounded batchの必須キー・型・上限を検査する。巨大generatorの全件list化を無条件に行わず、分割するならcommit単位が変わることを利用側へ示す
3. transactionを所有する最外層を一つ決め、autocommit値を明示する。内部helperに無断のcommit・属性切替・executescriptをさせない
4. 「正常→欠落キー」「正常→誤った型」「入力generator失敗」「制約違反」を末尾だけでなく途中にも置き、例外型と残存集合を照合する
5. 例外の内側捕捉と外側捕捉、ネストしたwith、legacy / False / Trueで、期待するrollback範囲が本当に同じかを確認する
6. RETURNING行、lastrowid、rowcount、commit結果を別々に記録する。結果取得やDML反復の完了だけを永続化成功として返さない
7. script前のpending更新がどの時点で確定するかを試験する。False方式へ変更して既存の手動BEGINと衝突しないかも確認する
8. retryは入力拒否の修正後、以前のcommit範囲と業務キーを確かめて行う。型エラーを一時障害として盲目的に同じバッチへ再試行しない

## 9. 限界・provenance

競合時のbusy timeout、WALのdurability、trigger副作用、custom adapter、ORM、別DB driver、全vendor buildの互換性は検証していない。固定upstreamのtest sourceを読んだことと、そのsuiteを実行したことも区別する。文書・sourceの取得日は全て2026-10-05 UTC。release_notesの30日TTLを考慮して期限を2026-11-04にした。

[Pythonのlicense文書][license]と固定root LICENSEを確認した。Python文書はPSF-2.0、文書内codeは追加で0BSD。今回読んだpysqliteのC実装とtestにはZlib型の個別license headerがあり、rootの表示だけで上書きしない。PEP 249はPublic Domain。配布告知の個別licenseはunknownとして、日付等の独自要約だけを使用する。外部codeの転載・module昇格は行っていない。固定GitHub blobのnative web表示はInternal Errorとなったため、同じcommitの内容をread-only connectorで取得した。

[api]: https://docs.python.org/3.14/library/sqlite3.html
[api13]: https://docs.python.org/3.13/library/sqlite3.html
[new]: https://docs.python.org/3.14/whatsnew/3.14.html#sqlite3
[release]: https://www.python.org/downloads/release/python-3148/
[pep]: https://peps.python.org/pep-0249/
[license]: https://docs.python.org/3.14/license.html
[cursor]: https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Modules/_sqlite/cursor.c
[connection]: https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Modules/_sqlite/connection.c
[dbtest]: https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/test/test_sqlite3/test_dbapi.py
[txtest]: https://github.com/python/cpython/blob/8e6e75d9102e39bed2a2b279203a396741180f12/Lib/test/test_sqlite3/test_transactions.py

---
{
  "id": "python-frozendict-shallow-hash-cache-key-boundary",
  "title": "Python frozendict: 浅い不変性・hashability・cache key同値性の境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "Python 3.15.0 introduction (2026-10-09); Python 3.15.1 documentation (release 2026-10-21), PEP 814 Final; retrieved 2026-10-23 UTC; runtime not executed",
  "tags": [
    "research-domain:backend",
    "python",
    "frozendict",
    "PEP-814",
    "shallow-immutability",
    "hashability",
    "cache-key",
    "lru_cache",
    "MappingProxyType",
    "typed",
    "snapshot"
  ],
  "sources": [
    {
      "id": "python-frozendict-release3150-20261023",
      "url": "https://www.python.org/downloads/release/python-3150/",
      "type": "release_notes"
    },
    {
      "id": "python-frozendict-release3151-20261023",
      "url": "https://www.python.org/downloads/release/python-3151/",
      "type": "release_notes"
    },
    {
      "id": "python-frozendict-pep814-20261023",
      "url": "https://peps.python.org/pep-0814/",
      "type": "official_docs"
    },
    {
      "id": "python-frozendict-stdtypes3151-20261023",
      "url": "https://docs.python.org/3.15/builtins/stdtypes.html",
      "type": "official_docs"
    },
    {
      "id": "python-frozendict-types3151-20261023",
      "url": "https://docs.python.org/3.15/library/types.html",
      "type": "official_docs"
    },
    {
      "id": "python-frozendict-functools3151-20261023",
      "url": "https://docs.python.org/3.15/library/functools.html",
      "type": "official_docs"
    },
    {
      "id": "python-frozendict-datamodel3151-20261023",
      "url": "https://docs.python.org/3.15/reference/datamodel.html",
      "type": "official_docs"
    },
    {
      "id": "python-frozendict-license3151-20261023",
      "url": "https://docs.python.org/3.15/license.html",
      "type": "official_docs"
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

# Python frozendict: 凍らせたmappingは、そのまま安全なcache keyになるか

**判断:** 外側のmappingを変更できないこと、hashを計算できること、同じkeyなら計算結果も同じことは、別々の条件である。`frozendict` は前者を提供するが、可変な値・順序依存処理・入れ子の型区別・外部状態まで自動的に固定しない。

既存の[Python sentinel](sentinel-missing-pickle-identity-boundary.md)は未指定値とpickle同一性、[futures](futures-bounded-map-timeout-shutdown.md)は実行待ちと停止が対象。本稿はmappingのsnapshotとcache key設計を扱う。事前index後の `frozendict`・`MappingProxyType` は該当なし、`lru_cache` の既存結果は別技術のsampling設定であり、主題の重複はなかった。

## 1. 公開版と契約の範囲

[Python 3.15.0配布ページ][release0]は2026-10-09のstable releaseとしてPEP 814を掲載する。[3.15.1][release1]は2026-10-21公開の最初のmaintenance releaseであり、frozendictが初めて加わった版ではない。取得日は2026-10-23 UTC。

[組込み型文書][stdtypes]などの `/3.15/` ページは取得時に3.15.1を表示した。更新されるURLなので、その表示版を適用範囲とする。[PEP 814][pep]はFinal、作成2025-11-12、最終更新表示2026-02-21。本稿では公開API文書とPEPを調べ、CPython repositoryの実装分析は行っていない。

## 2. 外側の不変性と、値の不変性を分ける

[公開API][stdtypes]では、frozendictはdictのsubclassではなく、項目代入・削除やupdateなどの更新methodを持たない。keyとvalueのすべてがhashableならhashを計算できる。`|=` は既存objectを変更せず、新しいfrozendictを作る。

[PEPのConstruction・Copy節][pep]では、dictからの構築はO(n)の浅いcopyで、値のobjectを再帰的に凍結しない。listを値に持つmappingも作れるが、そのlistを含むfrozendictのhash計算は失敗する。`copy()` も浅いcopyで、CPythonでは同じfrozendictを返す。

次の区別は、これらの契約から導く**独自の設計判断**である。

| 目的 | 選択と条件 | 凍結だけでは解決しないこと |
|---|---|---|
| 呼出先による外側の項目変更を防ぐ | frozendictを渡す | 値のlistやcustom objectは別途制御する |
| 設定のsnapshotを保持する | 構築後に元dictの項目を差し替えても影響しない外側copy | 元と共有する入れ子objectの変更 |
| cache keyにする | hash可能かつ計算意味が固定された値へ正規化 | hash成功だけでは意味の不変性を証明できない |
| 更新可能なmappingのread-only viewを渡す | MappingProxyTypeを使う | snapshotではなく、元の変更が見える |

例えば設定の値にlistを使う場合、元dictのその項目を別listへ差し替える操作と、元のlist自体へ要素を追加する操作は異なる。浅いcopyが分離するのは前者の束縛であり、後者のobjectではない。deepcopyで所有権を分けることと、コピー後のlistを不変にすることも違う。

`|=` で新しい設定を生成しても、既に別componentへ渡した古い参照が新設定へ切り替わるわけではない。設定更新は明示的な参照交換・世代番号で扱い、in-place更新と混同しない。

## 3. MappingProxyTypeを代替snapshotにしない

[typesの現行契約][types]では、MappingProxyTypeは元mappingの変更を反映するdynamic viewである。また3.12以降の `hash(proxy)` は基底mappingのhashへ委譲する。したがって、通常の非hashableなdictを包むことと、hashableなmappingを包むことを分ける。

PEP 814の歴史的比較にはMappingProxyTypeを非hashableとする記述があるが、それを現行API全体の断言として転載しない。**read-onlyであること、snapshotであること、hashableであることを別項目で確認する**のが本稿の判断である。proxyの有無だけから、呼出し間で内容が一定だとは推論しない。

## 4. cache keyに必要なのは、計算結果の同値性

[PEPのHashing and Comparison節][pep]によれば、frozendictは挿入順を保持してiterateする一方、等価比較とhashは項目順序に依存しない。以下はこの契約と[functoolsのcache契約][functools]を組み合わせた**独自の失敗分析**である。

### 順序を読む関数には、順序を含むkeyを渡す

同じ項目を逆順に挿入した二つのfrozendictは等価でも、iterationで得る順序は異なり得る。計算が「最初の項目を選ぶ」「項目順で出力を組み立てる」なら、等価なkeyなのに期待結果が異なる。順序を無視するcacheへそのまま渡す設計は不適切である。

業務上順序に意味がなければ、計算自体も順序に依存させない。順序が意味を持つなら、`tuple(mapping.items())` のような順序を持つ表現を候補にする。ただしtupleの各要素もhashableであることと、入れ子の意味が不変であることは引き続き必要である。

### typed=Trueは、入れ子の型識別を保証しない

[lru_cache文書][functools]ではtypedが区別するのは関数の直接の引数型であり、その内容ではない。二つの引数がともにfrozendictなら、値が異なる型であることをtypedだけで区別できるとは限らない。型差が業務上の意味を変える場合は、型を明示した値表現やschemaをkeyに含め、互いに等価な値を一つにまとめてよいか決める。

また、keyword argumentsの順序によって別entryになる場合がある。単一のfrozendict引数へまとめる設計は、呼出形式の整理だけでなく、どの入力を同一視するかの変更でもある。移行前後のhit数だけで正当性を判定しない。

### hash成功と、意味の固定を区別する

[data model][datamodel]では、利用者定義classは既定でidentityに基づく比較・hashを持てる。object内部が可変でもhashが存在する場合がある。一方、値による比較を実装する可変objectでは、keyとして使っている間にhashが変わる設計はhash tableの契約に反する。

**そこからの推論:** 設定objectを値に入れてhashが成功しても、関数がそのobjectの後から変わる属性を読むなら古い結果を再利用し得る。cache keyには、計算に必要な固定scalar、固定tuple、検証済みの入れ子mapping、またはデータ世代を取り出す。単に `hash()` を試す検査は必要条件の一部であり、十分条件ではない。

processをまたぐcacheへPythonのhash整数を永続IDとして保存しない。[data model][datamodel]はstrとbytesのhashがprocess間で予測可能でないことを明記する。外部cacheにはschema版と明示的なcanonical serializationなどを別途設計し、frozendictのhashを互換形式とみなさない。これは本稿の推奨であり、Pythonが外部cache形式を提供するとの主張ではない。

## 5. 並行性・所有権・移行時の判断

[PEPのThread Safety節][pep]は、生成後の浅い不変性と、値が他threadに変更されないという条件を分ける。frozendict化を、入れ子objectへの書込み許可や、生成元を同時変更してよい根拠にしない。共有前に値の所有権と生成時の同期をアプリ側で決める。

[lru_cache文書][functools]ではcache構造のthread safetyと、同時miss時の一回だけの関数実行は別である。引数・戻り値はentryが除去されるまで保持される。したがって、frozendict導入でsingle-flightや副作用の一回実行を実現したと判断しない。大量の設定世代をkeyにする場合はmaxsize、保持object、失効方法を確認する。

またdict subclassではないため、`isinstance(value, dict)` を入口条件にしているAPIへ無条件に渡す移行は避ける。読取りだけの境界ではMapping protocolを受け入れられるか確認する。第三者の同名frozendict package、型checker、serializerが組込み型と同じ互換性を持つかは、本調査では未検証である。

## 6. 採用前の受入テスト案

以下は**独自の試験計画**であり、実行済みテスト結果ではない。

1. 元dictの項目差替えと、共有するlistへの追加を分け、外側snapshotと浅い不変性の差を確かめる。list値を含むhash失敗を握りつぶさない。
2. 同じ項目の逆順入力で、keyの等価性・iteration順・cache前後の結果を比較する。順序依存処理を含むならkey形式を変更する。
3. 入れ子の値が等価だが型が違う入力を使い、typed=Trueだけで業務上必要な区別が失われないか確認する。
4. identity-hashableな可変objectの属性変更を挟み、計算結果が変わるならそのobjectをそのままcache keyに含めない。
5. MappingProxyTypeの元mappingを更新し、snapshotが必要な経路へdynamic viewを渡していないことを確認する。
6. `|=` 前の参照を別componentに残し、設定更新通知と世代切替が明示的に機能するか確認する。
7. 同時miss、maxsize到達、cache_clear、process再起動を別条件にする。hashの数値一致をprocess間の正解判定に使わない。

## 7. 証拠・ライセンス・未確認事項

- 公開API、PEP、両releaseページ、licenseをnative webで開いた。検索結果のsnippetだけを根拠にはしていない。repository analysisを行っていないため、架空のcommit pinや実装検証済みの主張は置かない。
- [Python文書のlicense][license]はPSF-2.0、文書内codeには追加で0BSD。PEP 814はpublic domainまたはCC0-1.0。配布告知ページのlicenseは未確認。本稿は独自要約・設計案で、上流codeのコピーやmodule昇格はない。
- 調査環境はPython 3.12.14であり、Python 3.15.1のruntime実行、各OS・free-threaded build、serializer・型checker・第三者packageの互換性、構築やhashの性能測定は未実施。C API・hash algorithm詳細・再帰freezeの汎用実装は対象外。
- 本文のcache誤用・参照交換・外部cache形式は公開契約からの推論と独自提案である。特定frameworkの実測障害や、全入力の再帰変換が可能との保証ではない。

検索確認用のAND query: `frozendict 浅い不変性`、`frozendict typed=True`、`frozendict 順序依存`、`MappingProxyType snapshot hash`、`frozendict 永続ID`。

[release0]: https://www.python.org/downloads/release/python-3150/
[release1]: https://www.python.org/downloads/release/python-3151/
[pep]: https://peps.python.org/pep-0814/
[stdtypes]: https://docs.python.org/3.15/builtins/stdtypes.html#frozendict
[types]: https://docs.python.org/3.15/library/types.html#types.MappingProxyType
[functools]: https://docs.python.org/3.15/library/functools.html#functools.lru_cache
[datamodel]: https://docs.python.org/3.15/reference/datamodel.html#object.__hash__
[license]: https://docs.python.org/3.15/license.html

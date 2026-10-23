---
{
  "id": "python-sentinel-missing-pickle-identity-boundary",
  "title": "Python 3.15 sentinel: 未指定・真偽値・pickle再解決の同一性境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "Python 3.15.0 introduction (2026-10-09); CPython 3.15.1 source c37e74cb6bcdadcd5f46f289498f6b3298ddee61 (2026-10-21); rolling docs retrieved 2026-10-23; runtime/type checker not executed",
  "tags": [
    "research-domain:backend",
    "python",
    "sentinel",
    "PEP-661",
    "missing",
    "None",
    "identity",
    "pickle",
    "reload",
    "type-narrowing",
    "migration"
  ],
  "sources": [
    {
      "id": "python-sentinel-release3150-20261023",
      "url": "https://www.python.org/downloads/release/python-3150/",
      "type": "release_notes"
    },
    {
      "id": "python-sentinel-release3151-20261023",
      "url": "https://www.python.org/downloads/release/python-3151/",
      "type": "release_notes"
    },
    {
      "id": "python-sentinel-builtins3151-20261023",
      "url": "https://docs.python.org/3.15/builtins/functions.html",
      "type": "official_docs"
    },
    {
      "id": "python-sentinel-pep661-20261023",
      "url": "https://peps.python.org/pep-0661/",
      "type": "official_docs"
    },
    {
      "id": "python-sentinel-pickle3151-20261023",
      "url": "https://docs.python.org/3.15/library/pickle.html",
      "type": "official_docs"
    },
    {
      "id": "python-sentinel-importlib3151-20261023",
      "url": "https://docs.python.org/3.15/library/importlib.html",
      "type": "official_docs"
    },
    {
      "id": "python-sentinel-license3151-20261023",
      "url": "https://docs.python.org/3.15/license.html",
      "type": "official_docs"
    },
    {
      "id": "python-sentinel-cpython3151-20261023",
      "url": "https://github.com/python/cpython/tree/c37e74cb6bcdadcd5f46f289498f6b3298ddee61",
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

# Python 3.15 sentinel: 未指定を保存するとき、何を同一とみなすか

**判断:** `None` が有効な入力なら、未指定の印は専用の sentinel に分ける。ただし同名の生成・copy・pickle・module reload は同じ操作ではない。共有する印は一か所で定義して再利用し、processや配備をまたぐ場合は名前の解決先まで契約に含める。

既存の[Python futures](futures-bounded-map-timeout-shutdown.md)は実行待ちと停止、[lazy imports](lazy-imports-release-readiness-boundary.md)はmodule初期化時刻を扱う。本稿は関数引数と直列化の同一性が対象。事前の全件indexと `sentinel`、`pickle identity`、`sentinel pickle`、`PEP-661` の検索では、本主題を扱う既存knowledge・pattern・moduleはなかった。

## 1. 公開済みの版と草案を区別する

[3.15.0配布ページ][release0]は2026-10-09公開のstable releaseとしてsentinel追加を掲載する。[3.15.1配布ページ][release1]は2026-10-21公開。3.15.1で初めて導入された機能ではない。取得日2026-10-23 UTCに両ページを確認した。

[builtins文書][builtins]・pickle・importlib・licenseは取得時に3.15.1を表示したが、`/3.15/` は更新されるURLである。実装観察はv3.15.1のtagをdereferenceした `c37e74cb6bcdadcd5f46f289498f6b3298ddee61` に固定した。[PEP 661][pep]はFinal、作成2021-06-06、最終更新表示2026-05-19。PEPの状態だけで配布完了を判断していない。

## 2. 関数境界: 未指定と空の値を混ぜない

[builtinsの公開契約][builtins]では `sentinel(name, /, *, repr=None)` は固有objectを作る。nameは文字列、reprは表示の調整であり、同じ表示だから同じ印になるわけではない。sentinelはtruthy、比較は `is` を使う。浅いcopyとdeepcopyは元のobjectを返し、subclass化はできない。

ここからの業務ルールは**独自の設計案**である。部分更新APIの内部関数なら、`ABSENT = sentinel("ABSENT")` を専用moduleで一度だけ定義し、次の三状態を分ける。

| 入力 | アプリ側で決める意味 | 判定の例 |
|---|---|---|
| ABSENT | 項目を変更しない | `value is ABSENT` |
| None | 明示的に値を消す | `value is None` |
| 0 / False / 空文字 / 通常値 | 指定された値として検証する | 上の二分岐以外 |

`if not value` で未指定を探すと、有効な0やFalseを混ぜる。逆に `value or fallback` はtruthyなABSENTをfallbackに置き換えない。HTTPのnullと欠落を入口で区別し、sentinelをそのまま外部JSONへ渡さず、送信する項目の有無へ明示的に変換する。これはAPIの設計提案であり、PythonがPATCHやJSONの欠落規則を提供するという意味ではない。

[PEPの不採用案][pep]では、同名をprocess-global registryで共有する設計が取り除かれている。各componentで同じ文字列を渡し直す代わりに、定義元のobjectをimportする。旧草案の別module名や、名前から同一objectを取り出せるという説明を採用しない。

## 3. pickle境界: import可能な束縛が必要

[builtins文書][builtins]は、module globalならnameと束縛名、class属性ならqualified nameと属性の経路を一致させることをpickleの条件にする。例えばmodule直下のABSENTならnameは `"ABSENT"`、module直下のclass `Patch` の属性なら `"Patch.ABSENT"` である。localな印を作れたことは、その印をpickleできることを意味しない。

[pickleのreduce契約][pickle]では、`__reduce__()` が文字列を返す場合、module側の名前でobjectを探す。[固定実装の `sentinel_reduce`][impl]はnameを返し、reprは返さない。さらに `caller()` は生成を呼び出したmoduleを記録する。したがって表示を変えることと、保存後の復元先を変えることは別である。

**独自の運用判断:** queueやcacheにpickleを置くなら、送信側でdumpが成功するだけでなく、受信側の新規processで定義moduleをimportして読み戻す。復元されるのは受信側でその名前にあるobjectであり、別processのメモリアドレスの共有ではない。module移動・属性改名・古いworkerの併存を、データ形式変更として扱う。

| 失敗しやすい置き方 | 問題の境界 | 独自の対処 |
|---|---|---|
| 関数内で生成しlocal変数だけに保持 | 解決可能なmodule/nameがない | process間で使う印は安定した定義moduleへ置く |
| helper moduleのfactoryが作った印を呼出側だけに代入 | 記録されたmoduleと公開先が違う | factoryを挟まず定義元で直接生成するのを既定にする |
| 同じglobal名を再生成した印へ上書き | 古い印と現在の名前のobjectが違う | 長寿命queueの処理中に定義を差し替えない |
| reprだけを保存し、受信側で同名を再生成 | 表示文字列と同一性を混同 | 外部形式には明示的な状態値と復号規則を用意する |

`__module__` は書換え可能だが、文字列を書き換えるだけで正しい公開先が用意されるわけではない。固定版の[ `Lib/pickle.py` の `whichmodule` ][picklesrc]と[ C実装 ][cpickle]は、指定先から得たobjectと保存対象が一致しなければ拒否する。factory表の問題はこの実装と呼出module記録からの推論であり、全factoryが無条件に失敗するとの主張ではない。

[pickle文書][pickle]の信頼境界も変わらない。unpickleは任意code実行を伴い得るため、外部から受け取った未検証のbytesに使わない。sentinelが標準化されたことはpickleの安全性の強化ではない。

## 4. reloadと型検査は別々に判定する

[importlib.reloadの契約][reload]ではmodule-level codeが再実行される一方、外部にある古い参照は自動では再束縛されない。**組合せからの推論:** sentinelを再生成する定義をreloadすると、古い関数のdefault引数や `from ... import ...` が保持する印と、新しいmodule上の印が分かれ得る。copyで同一性が保たれることを、reload耐性や配備間の永続IDの保証へ拡張しない。

開発時のhot reloadを本番互換性の試験に使う場合は、新規processでの起動試験も分ける。queue消化中のreloadを避ける、schemaを版管理した外部表現へ変換する、という選択はアプリ側の運用案である。

[PEPのtyping節][pep]は、束縛名と文字列nameが一致する定義を認識し、sentinel自体をunionの要素にでき、`is` / `is not` でnarrowingする契約を示す。例えば `int | ABSENT` は `int | sentinel` と意味が違う。後者は特定の印を表す指定ではない。PEPはbackportが現行設計と完全一致しないことも明記している。

採用する型checkerの対応版は本調査で検証していない。3.15のruntimeがunionを作れることと、使用中のcheckerが正しく型を絞ることを同じ合格条件にしない。旧Python向けの既存sentinelやbackportを機械的に置換せず、公開APIのdefault・pickle経路・truthinessを個別に比較する。

## 5. 固定sourceで確認したことと受入テスト案

[上流 `test_builtin.py` ][tests]には、同名の二回生成が別objectになること、copy/deepcopy、truthiness、module/class属性の各pickle protocolでの同一性、localな印のPicklingError、union演算を確認するテストがある。**上流テストを読んだ記録であり、この環境で実行に成功した記録ではない。**

本番採用前の受入条件は次のように設計する。

1. 未指定・None・0・False・空文字の各入力で、更新しない／消す／値を採用する分岐を別々に確認する。
2. 共通moduleをimportする二componentが同じ印を使い、独立した二回生成を同一と判定しない。
3. 実際に使うcopy経路とpickle protocolで往復し、受信workerの新規processで定義元の印との `is` を確認する。
4. module欠落・名前変更・古い定義・local factoryを故障条件に含め、失敗をNoneへ置換して処理を継続しない。
5. reload前に保存したdefault引数とreload後の印を比較し、hot reloadを許可するならライフサイクルのルールを決める。
6. runtimeのテストと型checkerのnarrowingテストを分離する。旧版を併存させるなら旧版側の生成と復元も確認する。

これらは独自の試験計画であり、3.15への更新だけで満たされる保証ではない。

## 6. 調査範囲・ライセンス・未確認事項

- 固定SHAは `git ls-remote` のv3.15.1 dereference先と、read-only cloneのHEADで照合した。GitHub/rawのnative web取得は失敗したため、実装はgit取得で補完した。公開API・PEP・release・licenseページはnative webで開いた。
- [license文書][license]と固定repositoryのLICENSEでPSF-2.0を確認。文書内codeには追加で0BSD、PEPはpublic domainまたはCC0-1.0。配布告知ページ自体のlicenseは未確認なので独自要約だけに使った。上流codeのコピー、module昇格はない。
- 調査環境のPythonは3.12.14。CPython 3.15.1のbuild/run、各OS・free-threaded build、実queue serializer、第三者backport、型checkerの実動作は未確認。sentinel固有の性能優位や全framework互換性は主張しない。
- 本文のfactory/reloadの結果は公開契約と固定sourceからの推論である。実行済みと扱わず、利用アプリの上記受入条件で確認する。

[release0]: https://www.python.org/downloads/release/python-3150/
[release1]: https://www.python.org/downloads/release/python-3151/
[builtins]: https://docs.python.org/3.15/builtins/functions.html#sentinel
[pep]: https://peps.python.org/pep-0661/
[pickle]: https://docs.python.org/3.15/library/pickle.html
[reload]: https://docs.python.org/3.15/library/importlib.html#importlib.reload
[license]: https://docs.python.org/3.15/license.html
[impl]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Objects/sentinelobject.c
[tests]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/test/test_builtin.py
[picklesrc]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Lib/pickle.py
[cpickle]: https://github.com/python/cpython/blob/c37e74cb6bcdadcd5f46f289498f6b3298ddee61/Modules/_pickle.c

---
{
  "id": "python-abi3t-wheel-gil-acceptance-boundary",
  "title": "Python abi3t: wheelのABI互換性とGIL無効実行の受入境界",
  "kind": "knowledge",
  "technology": "python",
  "version": "CPython 3.15.1 C API documentation (release 2026-10-21); abi3t introduced in 3.15.0 (2026-10-09); PEP 803 Final; runtime/build tests not run",
  "tags": [
    "research-domain:backend",
    "python",
    "abi3t",
    "abi3",
    "Stable-ABI",
    "free-threading",
    "wheel",
    "Py_TARGET_ABI3T",
    "Py_mod_abi",
    "Py_mod_gil",
    "PyModExport",
    "binary-compatibility"
  ],
  "sources": [
    {
      "id": "python-abi3t-whatsnew3151-20261024",
      "url": "https://docs.python.org/3.15/whatsnew/3.15.html",
      "type": "release_notes"
    },
    {
      "id": "python-abi3t-release3151-20261024",
      "url": "https://www.python.org/downloads/release/python-3151/",
      "type": "release_notes"
    },
    {
      "id": "python-abi3t-stable3151-20261024",
      "url": "https://docs.python.org/3.15/c-api/stable.html",
      "type": "official_docs"
    },
    {
      "id": "python-abi3t-migration3151-20261024",
      "url": "https://docs.python.org/3.15/howto/abi3t-migration.html",
      "type": "official_docs"
    },
    {
      "id": "python-abi3t-module3151-20261024",
      "url": "https://docs.python.org/3.15/c-api/module.html",
      "type": "official_docs"
    },
    {
      "id": "python-abi3t-export3151-20261024",
      "url": "https://docs.python.org/3.15/c-api/extension-modules.html",
      "type": "official_docs"
    },
    {
      "id": "python-abi3t-object3151-20261024",
      "url": "https://docs.python.org/3.15/c-api/object.html",
      "type": "official_docs"
    },
    {
      "id": "python-abi3t-extensions3151-20261024",
      "url": "https://docs.python.org/3.15/howto/free-threading-extensions.html",
      "type": "official_docs"
    },
    {
      "id": "python-abi3t-runtime3151-20261024",
      "url": "https://docs.python.org/3.15/howto/free-threading-python.html",
      "type": "official_docs"
    },
    {
      "id": "python-abi3t-pep803-20261024",
      "url": "https://peps.python.org/pep-0803/",
      "type": "official_docs"
    },
    {
      "id": "python-abi3t-license3151-20261024",
      "url": "https://docs.python.org/3.15/license.html",
      "type": "official_docs"
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

# Python abi3t: wheelのABI互換性とGIL無効実行の受入境界

## 問いと採用判断

CPython 3.15の `abi3t` が使えるなら、既存の `abi3` wheelのタグを書き換えるだけでfree-threaded buildへ配布できるか。importできたことを、GILなしで安全に並行実行できる証拠にしてよいか。

**どちらも不可。ABIの互換性、installerが選ぶartifact、GILなしで動かす実装上の安全性は別に確認する。** `abi3t` は配布するbinary数を減らす選択肢であり、古いbinaryの変換、全OS共通化、競合状態の修正ではない。必要なC APIを使えない拡張や未対応のbuild toolでは、従来の `abi3` とfree-threaded版向けのversion-specific wheelを維持する判断も有効である。

本稿は新しいABIの配布・受入契約に絞る。Python一般のthread高速化、既存のProcessPoolExecutor・asyncio・lazy import資料の再説明ではない。以下では公式資料の事実と、それを使う独自のrelease判断・試験案を分ける。

## 1. 何がいつ使えるようになったか

[Python 3.15 What's New](https://docs.python.org/3.15/whatsnew/3.15.html#pep-803-stable-abi-for-free-threaded-builds)は、3.15の公開日を2026-10-09と記録し、free-threaded build向けStable ABIを新機能として説明する。本稿がAPI契約を確認した文書の表示版は3.15.1。[3.15.1 release](https://www.python.org/downloads/release/python-3151/)は2026-10-21公開であり、previewの計画を正式提供済みと推測したものではない。

[PEP 803](https://peps.python.org/pep-0803/)は確認時点でFinal、対象Pythonは3.15、最終更新表示は2026-10-08 20:05:26 UTC。PEPを設計背景として使い、実際のAPIの細部は3.15.1のC API文書を優先した。3.15.1で初めて導入された機能、あるいは3.14へ遡って利用できるABIとは扱わない。

## 2. wheelの互換範囲を先に固定する

[PEP 803の配布契約](https://peps.python.org/pep-0803/#the-abi3t-wheel-and-filename-tags)に基づく整理を示す。表はPython tagとABI tagだけを記し、実際のwheelにはplatform tagも必要である。「3.15以降」はStable ABIの約束であり、未公開の将来版を実行確認したという意味ではない。

| tagの組合せ | 通常build | free-threaded build | 下限・制限 |
|---|---|---|---|
| `cp315-cp315` | 3.15 | 対象外 | version-specific |
| `cp315-cp315t` | 対象外 | 3.15 | version-specific |
| `cp315-abi3` | 3.15以降 | 対象外 | 従来のStable ABI |
| `cp315-abi3t` | 配布対象にしない | 3.15以降 | `abi3t` 単独targetは可能だが非推奨 |
| `cp315-abi3.abi3t` | 3.15以降 | 3.15以降 | 両ABIを明示的にtargetにしたbuild |

`abi3.abi3t` は二つのABIへの対応を示すcompressed tag setである。`.so` のファイル名へそのまま連結するものではない。両ABI対応のextensionはLinux/macOS等では `.abi3t.so`、Windowsでは `.pyd` を使う。通常buildが `.abi3t.so` を検索できることと、`abi3t` 単独でtagしたwheelを通常build向けと宣言することも同一ではない。

**同じsourceを複数版へcompileできることと、一つのbinaryを複数版で使えることを分ける。** 3.15下限のartifactを `cp314` と名付けても3.14互換にはならない。3.13t/3.14tの利用者が残るなら、別のversion-specific artifactを残すかsupport policyを変更する。これはrelease設計上の判断である。

[C APIのplatform条件](https://docs.python.org/3.15/c-api/stable.html#platform-considerations)はOSやCPUだけでなくcompiler・下位library・compile optionにも依存する。従って「Stable ABIなので一つのwheelで全platformを賄う」「wheel数が必ず半分になる」という予測はしない。

## 3. targetを選ぶこととsourceを移植すること

### 3.1 buildの下限を固定し、tagと一致させる

[Stable ABIのcompile契約](https://docs.python.org/3.15/c-api/stable.html#compiling-for-stable-abi)では `Py_LIMITED_API` が `abi3`、`Py_TARGET_ABI3T` が `abi3t` のtargetを指定する。両方へ明示的に対応するなら、両macroを最低対応版に合わせて `Python.h` より前に設定する。3.15の値は `0x030f0000`。使用中のheader版を無条件に下限にするのではなく、配布したい下限を選ぶ。

同文書は、macro・filename・metadataを整合させるbuild toolの機構がある場合はそれを優先する。[What's New](https://docs.python.org/3.15/whatsnew/3.15.html#pep-803-stable-abi-for-free-threaded-builds)も、toolが `abi3t` を扱えなければfree-threaded向け `cp315t` を別にbuildする選択を示す。ここでは各toolの最低対応版や設定構文を検証しておらず、特定toolの導入手順は提案しない。

独自の受入条件として、CIの表示名が `abi3t` であるだけでは足りない。実際のcompile target、生成されたextensionのsuffix、wheelのtagを突き合わせる。既存binaryのrenameやtag編集だけを移植完了にしてはいけない。

### 3.2 object layoutへの依存を除く

[What's New](https://docs.python.org/3.15/whatsnew/3.15.html#pep-803-stable-abi-for-free-threaded-builds)は、`PyObject` をinstance structに埋め込む形から、負の `basicsize` と `PyObject_GetTypeData()` を使う形への移行を主要な変更として挙げる。[移行guide](https://docs.python.org/3.15/howto/abi3t-migration.html#pyobject-opaqueness)では `PyObject` / `PyVarObject` がopaqueとなり、そのsizeやmemberへの依存を取り除く必要がある。実行時のsizeからoffsetを推測して古いlayoutを再現する方法は採らない。

[Object Protocol](https://docs.python.org/3.15/c-api/object.html#c.PyObject_GetTypeData)によれば、`PyObject_GetTypeData(obj, cls)` の `cls` は負の `basicsize` で作った型であり、`obj` はそのinstanceでなければならない。この条件をPythonが自動検査するわけではない。

移行guideは、subclassを許す型で `Py_TYPE(obj)` をそのまま渡すと、別のsubclassに割り当てた領域を指し得ると注意する。独自の試験案として、直接instanceだけでなく `__slots__` 付きsubclassを使うケースも残す。compile成功と一回のobject生成だけでは、定義元の型を選べている証拠にならない。

また、guideは `tp_itemsize` 等を使うvariable-sizedな独自型を `abi3t` 3.15へ移植できないと明記する。対象の構造を使うなら、限定APIの制約を回避するcastを追加するのでなく、当面version-specific buildを維持する。これは全拡張が直ちに `abi3t` へ移るべきでない具体例である。

## 4. ABI検査とGIL利用宣言は独立している

### 4.1 新しいexport hookでは検査より前の実行に注意する

[extension exportの契約](https://docs.python.org/3.15/c-api/extension-modules.html#extension-export-hook)により、3.15では `PyModExport_<name>` が `PySlot` 配列を返す。配列はinterpreter終了まで有効かつ不変である必要があり、通常はstaticな配列を返し、動的な処理はcreate/exec側へ置く。

重要なのは実行順である。`Py_mod_gil` とsubinterpreter対応のslotはhookが戻った後に検査されるため、hook内でGILの存在を前提にしてはならない。C APIを呼ぶ必要がある場合には先にABIを検査するという契約上の注意もある。独自のreview観点として、旧 `PyInit_` の初期化処理を名前だけ変えてhookへ移す変更を避ける。

### 4.2 検査できるABI不一致を拒否しても、thread safetyは証明しない

[Module Objects](https://docs.python.org/3.15/c-api/module.html#c.Py_mod_abi)は二つのslotを別に定義している。

- `Py_mod_abi` は使うABIの情報を渡す。`PyABIInfo_VAR` で作った情報を利用でき、module作成時に検査される。旧 `PyModuleDef` から作るmoduleを除いて必須である。
- `Py_mod_gil` はGILへの依存を宣言する。未指定の既定は `Py_MOD_GIL_USED`。`Py_MOD_GIL_NOT_USED` はGILなしで安全に動くという拡張側の宣言であり、安全にする自動変換ではない。通常buildではこのslotは無視される。

[ABI checking](https://docs.python.org/3.15/c-api/stable.html#abi-checking)は簡易的な防御であり、platformの不一致や全ての不正組合せを検出するものではない。共有libraryをloadできた後の検査なので、installer側の適合性確認も必要である。

[free-threading用C API guide](https://docs.python.org/3.15/howto/free-threading-extensions.html)は、同時変更されるcontainerからのborrowed reference、lockingしないaccess macro、直接のstruct fieldアクセス等に注意を要求する。例えば共有listからの参照取得は、必要に応じてstrong referenceを返すAPIへ見直す。`PyDict_Next()` は自動でdictionaryをlockしないため、同時変更があり得る走査ではcritical sectionが必要になる。

これらはABI tagの問題ではなく並行実装の問題である。独自の判断として、警告を消す目的で `Py_MOD_GIL_NOT_USED` を追加したり、未確認の拡張をGIL強制無効化で通したりしない。global state、参照の寿命、外部native libraryを含めた安全性を確認してから宣言する。

### 4.3 free-threaded buildと実際のGIL状態を分けて観測する

[Python側のfree-threading文書](https://docs.python.org/3.15/howto/free-threading-python.html#identifying-free-threaded-python)は、build能力の確認を `sysconfig.get_config_var("Py_GIL_DISABLED")`、そのprocessでGILが現在有効かの確認を `sys._is_gil_enabled()` と分ける。free-threaded buildでも実行時にGILを有効にでき、未対応のC extensionをimportすると警告とともに有効化される場合がある。

[移行guide](https://docs.python.org/3.15/howto/abi3t-migration.html#avoid-build-time-conditionals)では、`abi3t` のcompile時には `Py_GIL_DISABLED` が常に定義される。これはそのbinaryを通常buildで動かす場合も含むため、Cの条件compileだけを現在のGIL状態と解釈しない。

従って、起動直後だけGIL状態を記録しても不十分である。独自の受入案として、実際の依存moduleをimportした後と、遅れて読み込むplugin経路を通った後にも確認する。「3.15tでimport成功」という結果に、GILなしの性能や正しさまで含めない。

## 5. 配布を切り替えるための受入試験案

以下は独自の試験計画であり、本稿の調査で実行済みの結果ではない。対象packageの通常buildとfree-threaded buildを分け、同じbuild成果物を試す。test中にsourceから別binaryがbuildされて成功した場合は、配布wheelの成功と数えない。

| 確認境界 | 入力・操作 | 合格条件として残す証拠 |
|---|---|---|
| artifactの識別 | wheelを通常手順でinstallし、extensionの実path・hash・tagを記録 | 意図したwheelとbinaryが選ばれ、別版やsource buildで代替されていない |
| Python下限 | 宣言した最小minorと、それ以降の実際にsupportするminorで試す | target下限とmetadataが一致。未対応の旧版にinstall可能と宣言していない |
| 両buildへの適用 | 同一platformの通常3.15系とfree-threaded3.15系で通常機能を実行 | 両方でimport・機能・例外・終了が正常。単なるloader成功で止めない |
| GIL状態 | 依存群とoptional pluginの読込み前後を観測 | build能力とruntime状態を別々に記録し、想定外のGIL再有効化を見逃さない |
| object layout | 独自型の生成・解放と、許可したsubclassを試す | defining classに対応したdataを扱い、subclass領域との混同がない |
| 並行更新 | 実際に共有するobjectやglobal stateへ複数threadからアクセス | 寿命・同期・例外処理の不変条件が保たれる。短時間の成功だけを一般保証としない |
| 既存利用者 | 保持する旧Python向けartifactを通常のinstallerで取得 | 新wheel追加で旧系列の配布経路を消していない |
| 性能・rollback | 同じ仕事量で旧artifactと比較し、戻すartifactも保存 | latency/throughput・memoryの許容範囲と、互換範囲を保つ戻し先が明確 |

[移行guideのtesting節](https://docs.python.org/3.15/howto/abi3t-migration.html#testing)も、対応する各Python版と両buildでの試験を求める。Stable ABIの成立は、Pythonの挙動変更や拡張自身のbugを排除しない。

採用を急がない条件も明示する。variable-sized typeを必要とする、code generatorやbuild toolが未対応、旧版へのsource互換を保つ費用が大きい、または性能要件を満たさない場合は、別wheelの継続と移行延期を選べる。ABIの新しさだけをrelease完了条件にしない。

## 6. 検証範囲・出典・未確認事項

- 一次資料の実ページを2026-10-24 UTCに開き、releaseの日付、3.15.1のAPI表示、PEP 803のFinal状態を確認した。`/3.15/` の文書は更新されるため取得日と表示版をsource catalogに残す。repository実装の分析は行っておらず、浮動branchを固定commitの代わりには使っていない。
- 本調査環境はPython 3.12.14。CPython 3.15.1のbuild/run、C extensionのcompile、wheelの生成・install、通常/free-threadedの比較、OS別試験、raceや性能測定は未実施。上の表は実行結果ではない。検索evalは本稿の発見可能性だけを確認する。
- 全てのinstaller/build toolがこのtagを正しく扱うとは確認していない。外部native libraryのABI、vendor版、PyPy等の別実装、将来minorの具体的動作も対象外。特定packageがGILなしで安全だとは結論しない。
- [History and License](https://docs.python.org/3.15/license.html)でPython文書はPSF-2.0、文書のcode例等には追加で0BSDが適用されることを確認。PEP 803はpublic domainまたはCC0-1.0-Universalの寛容な方と明記。Python.org release告知本文のopen licenseは未確認なので該当recordは `unknown` とした。独自の日本語要約と試験案のみを保存し、上流code・図・長文を転載せずmoduleにも昇格しない。
- release_notesのTTL 30日が最短なので、明示期限は2026-11-23。新しいmicro版やtoolの対応が出た際は本文と契約を再確認し、日付だけを延長しない。

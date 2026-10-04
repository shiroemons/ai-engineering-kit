---
{
  "id": "testing-coverage-cross-platform-combine-evidence-boundary",
  "title": "coverage.py 7.16: cross-platform combine と shard 完備・source 同一性の境界",
  "kind": "knowledge",
  "technology": "testing",
  "version": "coverage.py 7.16.2 (2026-09-27), 5643ae7c682bbd56ee1ac2b11556145ab8c2cab0; auto-combine 7.14.0 / keep-combined 7.15.0 / slash normalization 7.16.0; source review only",
  "tags": [
    "research-domain:quality-operations",
    "coverage.py",
    "combine",
    "cross-platform",
    "shard",
    "relative_files",
    "paths",
    "keep-combined",
    "fail_under",
    "source-revision",
    "parallel",
    "CI"
  ],
  "sources": [
    {
      "id": "coverage-combine-release-7162-20261004",
      "url": "https://coverage.readthedocs.io/en/7.16.2/changes.html",
      "type": "release_notes"
    },
    {
      "id": "coverage-combine-command-7162-20261004",
      "url": "https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_combine.rst",
      "type": "official_docs"
    },
    {
      "id": "coverage-report-command-7162-20261004",
      "url": "https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_reporting.rst",
      "type": "official_docs"
    },
    {
      "id": "coverage-path-config-7162-20261004",
      "url": "https://coverage.readthedocs.io/en/7.16.2/config.html",
      "type": "official_docs"
    },
    {
      "id": "coverage-combine-core-7162-20261004",
      "url": "https://github.com/coveragepy/coveragepy/tree/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage",
      "type": "github_repository_analysis"
    },
    {
      "id": "coverage-combine-tests-7162-20261004",
      "url": "https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/tests/test_data.py",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# coverage.py 7.16: cross-platform combine と shard 完備・source 同一性の境界

## 問いと結論

Linux・Windows、Python版、テスト shard ごとの coverage data を集めて、総合割合が基準を超えた。その結果だけで「今回の全必須jobが、同じsourceに対して測定を終えた」と判断できるか。

できない。coverage.py の combine は、source file name を対応付けた測定データの union である。全必須jobの到着、テストの成功、source revision の一致を照合するリリース判定ではない。7.16.0 の separator 正規化は cross-platform 集約を容易にするが、この境界を変えない。以下は公式契約、固定実装の観察、それらに基づく独自のCI運用案を区別して記す。[combine公式文書](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_combine.rst) / [固定実装](https://github.com/coveragepy/coveragepy/tree/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage)

対象は coverage.py 本体の CLI と data merge。pytest-cov、tox、CI artifact downloader が別途設ける収集・失敗判定は、この調査だけでは保証しない。

## 1. 何が変わったか

- 7.14.0（2026-05-10）: report などの reporting commands が、parallel data files を自動combineするようになった
- 7.15.0（2026-07-02）: 自動combine後に入力を残す `--keep-combined` が追加された。入力削除が既定であること自体は変わらない
- 7.16.0（2026-08-28）: combine時に slash / backslash をローカルOSの separator へ正規化するようになった。また、`[paths]` の置換がpath中の複数箇所へ及んでいた不具合を修正し、先頭の対象部分だけを置換するようになった
- 本文は7.16.2（2026-09-27公開）の固定commitで確認した。自動combineやseparator正規化を7.16.2の新機能とは扱わない

導入版と日付は [7.16.2のchange history](https://coverage.readthedocs.io/en/7.16.2/changes.html) による。7.16.2より後の未リリース項目は適用範囲に含めない。

## 2. path が一致することと source が同じこと

### 公式契約

`relative_files = true` は data file に相対pathを保存する。この方式では `source` を run のコマンドラインだけに置かず、設定ファイルへ置く必要がある。report側もsourceの基点を知る必要があるためである。

絶対pathが異なる環境では `[paths]` で同一source treeに対応するrootを列挙できる。各listの先頭はreport環境に実在するsource pathとする。複数listは順に試され、変換先が存在しない候補は次へ進む。実際の対応付けは `--debug=pathmap` で確認する。[設定reference](https://coverage.readthedocs.io/en/7.16.2/config.html)

### 固定実装の観察

7.16.2の `CoverageData.update()` はpath対応付け後にseparatorを正規化し、対応するfile/contextのline集合をunionし、arcを重複なく取り込む。upstreamの `test_combining_autofixes_slashes` も、`src/a.py` と `src\a.py` を同じfileとしてまとめるfixtureを持つ。これはテストを読んだ結果であり、この調査環境での実行結果ではない。[sqldata.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage/sqldata.py) / [test_data.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/tests/test_data.py)

この変換は、`C:\work\app\src\a.py` と `/agent/build/app/src/a.py` の異なるrootを無条件に同一化する機能ではない。separatorだけが違う場合と、checkout rootが違う場合を分ける。後者には適切な `[paths]` や相対path運用が必要になる。[files.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage/files.py) / [combine公式文書](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_combine.rst)

確認したdata schemaとmerge処理は、file path、context、line number、arc等を扱い、入力ごとのGit SHAやsource本文の一致を検証していない。data fileのhashによる重複skipも、source revisionを保証する仕組みではない。したがって「変換先が実在する」「2つのpathが1つになった」ことからsource同一性を推論しない。[sqldata.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage/sqldata.py) / [data.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage/data.py)

独自の判断例: 同じ `src/a.py` でも、旧commitの10行目と新commitの10行目が別の処理なら、旧測定のline numberを新sourceへ重ねることには意味のずれがある。path aliasを広くして無理に1行へ集約する前に、revisionと生成sourceの同一性を確認する。

## 3. 自動report と明示combine は同じ入力初期化ではない

公式report文書は、parallel data filesがあれば自動combineされ、入力は既定で削除されると説明する。`--keep-combined` はその入力を保持する指定であり、対象の限定や完全性検証を行う指定ではない。[report公式文書](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_reporting.rst)

固定 `cmdline.py` の分岐は次のとおり。

| CLI経路 | 既存のbase data file | parallel入力が0件の場合 |
|---|---|---|
| `coverage combine` | `--append` なしでは既存dataをloadしない | `strict=True` で失敗する |
| `coverage combine --append` | 先に既存dataをloadする | `strict=True` で失敗する |
| `coverage report` 等 | 先に既存dataをloadし、その後combineする | combineは `strict=False`。それだけでは失敗せず、後続reportのdata有無などによる |

このため、新規shardが届かなくても古い `.coverage` が残っていれば、それを使ってreportできる経路がある。「reportを実行した」だけでは今回の測定がある証拠にならない。明示combineの「既存combined fileを無視して作り直す」という説明を、reportの初期化へそのまま当てはめない。[cmdline.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage/cmdline.py) / [combine公式文書](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_combine.rst)

保存指定の名称も別である。

- 明示combineでraw入力を残す: `--keep`
- reportの自動combineでraw入力を残す: `--keep-combined`

入力を保持して再集約できても、入力集合が今回の実行だけであることやsource同一性は別途確認する。

## 4. 「combine成功」「ファイル数」「fail_under」が証明しないこと

### 一部の入力を読めなくても、他の入力で集約し得る

固定 `combine_parallel_data()` は、入力のreadで発生した `CoverageException` を警告にし、その入力を削除せず残す。他の入力は続けて処理する。`strict=True` が確認するのは、候補があることと、少なくとも1件をcombineできたことであり、「指定入力すべてが正常」ではない。良い入力と壊れた入力が混在すると、警告を伴って一部のみ集約する可能性がある。

ただし全ての不整合が警告になるわけではない。line coverageとbranch coverageの混在、同一pathに対するfile tracerの衝突は、`CoverageData.update()` のerrorとして扱われる。read失敗の継続とmerge不整合の失敗を一括りにしない。[data.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage/data.py) / [sqldata.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage/sqldata.py)

### 集約数は必須job数ではない

data内容が重複すればskipされ得る。一方、1つのjobが複数processのdataを出すこともある。したがって `Combined N files` のNを期待shard数へ直接対応させない。重複skipされた入力も `--keep` がなければ削除対象となる。raw artifactの監査・再解析が必要なら、report生成前から保持方針を決める。[data.py](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/coverage/data.py) / [combine公式文書](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_combine.rst)

### 割合の閾値は到着確認ではない

`fail_under` はreport対象の総合割合が指定値を下回ると終了code 2にする。小数閾値には `precision` も関係する。この契約には、必須OS・Python版・shardの一覧との照合はない。[設定reference](https://coverage.readthedocs.io/en/7.16.2/config.html) / [report公式文書](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_reporting.rst)

独自の反例: 必須のLinux・Windows両jobが、同じ100行のうち同じ90行を測定する設計だとする。Windowsのartifactが未到着でも、Linuxだけで90%となり `fail_under=90` を満たし得る。総合unionの90%は、Windows上でその90行を実行した証拠にならない。OSごとの網羅性を要求するなら、環境別の判定も残す。

## 5. 実務で採る境界（独自のCI運用案）

1. **期待集合を先に決める。** run ID、対象source SHA、OS、Python版、shard ID、必須／任意をmanifestへ記録する。retryがある場合は採用attemptの規則を決め、単に「届いたファイル全部」を採用しない
2. **各jobの完了とartifactを照合する。** テスト終了状態、upload完了、artifact内のdata存在を別々に確認する。必須jobの欠落、キャンセル、読取不能を、coverage割合とは独立に失敗させる
3. **測定条件を揃える。** source SHAと生成sourceの対応、coverage版、branch/line mode、source/include/omit、必要なcontextやpluginを記録する。OS別依存差など意図的な差はmanifestに残す
4. **今回専用の集約先を作る。** 古いbase data fileや前回のraw shardを含めない。承認した入力の明示file一覧を `coverage combine --keep` へ渡す。directory引数を使う場合も、そのdirectoryが今回の入力だけであることを先に確認する
5. **raw入力と出力の場所を分ける。** 明示入力を指定すると現在directoryは自動では検索されない。一方、後続reportはbase data fileのdirectoryで自動combineし得るため、report先へ未承認のdotted-suffix filesを混在させない
6. **集約後も証拠を照合する。** read警告・error、対象file一覧、pathmapの対応、環境別dataを確認してから総合割合を判定する。予期しない欠落sourceを `--ignore-errors` で通す運用にはしない
7. **判定を分けて保存する。** 「全必須job成功」「全必須artifact到着」「source/測定条件一致」「集約成功」「割合達成」をそれぞれ残す。総合reportとmanifest・raw artifactの組をレビュー可能にする

上記はcoverage.pyの機能追加ではない。既存のCI・artifact管理側へ置く検証案であり、このknowledge追加では実装していない。

### parallel mode だけで集約競合は防げない

公式文書は、独立runnerが同じworking directoryで並行動作する場合、runnerごとに別のbase data file名を使うよう求める。parallel modeは測定ファイルの名前を分けるが、共通base名に対するcombine/eraseまで排他しない。runner別 `COVERAGE_FILE` または専用directoryで隔離し、全runner終了後に最終集約を行う。全体用の共通prefixで、まだ実行中のrunnerのdataを先取りして集約しない。[combine公式文書](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/doc/commands/cmd_combine.rst)

## 6. 導入前に確認するケース

以下は受入試験案であり、実機で成功したという記録ではない。

| 入力条件 | 必要な確認 |
|---|---|
| separatorだけ異なる同一sourceの相対path | 7.16系で期待fileへunionされること。upstream fixtureとの対応も確認する |
| Windows driveとLinux checkout rootが異なる | separator正規化だけを期待せず、pathmapとreport側sourceの存在を検証する |
| 必須shardを1件欠落させる | 割合が閾値以上でもmanifest照合で失敗すること |
| 正常dataと読取不能dataを混ぜる | combineの一部継続を見落とさず、入力完全性の判定が失敗すること |
| 古い `.coverage` だけ残し、今回のparallel入力を0件にする | reportの成功を新規測定の証拠にしないこと |
| 同一pathだがsource SHAを変える | merge前のrevision照合で拒否すること |
| branch dataとline dataを混ぜる | merge errorを握りつぶさず失敗にすること |
| rawを保持して同じ入力を再集約する | 再現可能な入力集合と結果を確認すること。重複skip数をjob数と誤認しない |
| 複数runnerを同時に終了・reportさせる | base名・directoryの隔離と最終集約barrierを確認すること |

## 7. 出典・適用限界

- 取得日は2026-10-04 UTC。7.16.2 tagはcommit `5643ae7c682bbd56ee1ac2b11556145ab8c2cab0` へ解決されることをGitHubのtag/commit情報で確認した
- change historyとconfigは7.16.2版の実ページを開いた。combine/reportのversioned HTMLは取得失敗したため、7.16.2表示のrolling HTMLを読み、上記固定commitの公式RST文書で照合した。source catalogは固定RSTを参照する
- `LICENSE.txt`、`NOTICE.txt`、該当Python/RSTヘッダーでApache-2.0を確認した。本文は独自要約であり、実装コードの転載やmodule昇格は行っていない。[LICENSE](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/LICENSE.txt) / [NOTICE](https://github.com/coveragepy/coveragepy/blob/5643ae7c682bbd56ee1ac2b11556145ab8c2cab0/NOTICE.txt)
- 固定実装とupstream testを読んだ。coverage.pyを導入してLinux／Windowsでmergeを実行したわけではなく、pytest-cov・toxの版別統合、実CIのartifact収集競合、巨大dataの性能も未検証
- 追加する検索evalは、この文書へ到達できることを確認する。shard完備、coverage数値、OS互換性、データ保持のruntime試験を代替しない

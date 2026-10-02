---
{
  "id": "sqlite-stale-expression-index-reindex-migration",
  "title": "SQLite 3.53 の stale expression index: self-healing と REINDEX EXPRESSIONS の移行境界",
  "kind": "knowledge",
  "technology": "sqlite",
  "version": "SQLite 3.53.0 (released 2026-04-09): self-healing and REINDEX EXPRESSIONS; official contracts retrieved 2026-10-02; 3.52.0 was withdrawn",
  "tags": ["research-domain:data", "expression-index", "reindex", "self-healing", "migration", "generated-column", "integrity-check"],
  "sources": [
    {"id": "sqlite-353-expression-release-20261002", "url": "https://www.sqlite.org/releaselog/3_53_0.html", "type": "release_notes"},
    {"id": "sqlite-352-withdrawn-release-20261002", "url": "https://www.sqlite.org/releaselog/3_52_0.html", "type": "release_notes"},
    {"id": "sqlite-stale-expression-guide-20261002", "url": "https://www.sqlite.org/staleexpridx.html", "type": "official_docs"},
    {"id": "sqlite-reindex-expressions-contract-20261002", "url": "https://www.sqlite.org/lang_reindex.html", "type": "official_docs"},
    {"id": "sqlite-generated-column-index-boundary-20261002", "url": "https://www.sqlite.org/gencol.html", "type": "official_docs"},
    {"id": "sqlite-expression-index-contract-20261002", "url": "https://www.sqlite.org/expridx.html", "type": "official_docs"},
    {"id": "sqlite-integrity-check-index-contract-20261002", "url": "https://www.sqlite.org/pragma.html#pragma_integrity_check", "type": "official_docs"},
    {"id": "sqlite-documentation-public-domain-20261002", "url": "https://www.sqlite.org/copyright.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# SQLite 3.53 の stale expression index: self-healing と REINDEX EXPRESSIONS の移行境界

## 問いと対象

SQLite、OS、CPU、アプリ独自 SQL 関数を更新したあと、式の計算結果と既存 index key が一致し続けるか。3.53.0 の自動修復を理由に移行後の検査を省略できるか。

本書は expression index の整合性を扱う。[WAL checkpoint](wal-checkpoint-progress-reset-safety.md) の race 修正や [foreign key 検査](foreign-key-enforcement.md) とは別の問題である。公式資料を 2026-10-02 UTC に読み、機能導入版を記録した。最新 patch の推奨・全版の安全性判定ではない。

## 確認した変更: 導入版は 3.53.0

[3.53.0 release notes](https://www.sqlite.org/releaselog/3_53_0.html) は 2026-04-09 公開として、self-healing index と `REINDEX EXPRESSIONS` の追加、浮動小数点と文字列の変換処理の更新を記載している。後者では既定の有効桁数が 15 から 17 に変わった。これらは別々の変更点であり、有効桁数の変更だけから特定の index が stale になったとは判定できない。

[3.52.0 release notes](https://www.sqlite.org/releaselog/3_52_0.html) は、2026-03-06 の版を Withdrawn とし、予定機能を 3.53.0 へ移したと明記する。一方、[stale index guide](https://www.sqlite.org/staleexpridx.html) の変換処理の説明には「3.51 から 3.52」の記述が残る。本書ではその一節から 3.52.0 を正式導入版と推定せず、3.53.0 の release notes と各機能の明示版を採る。

## 公式契約: deterministic でも環境変更を棚卸しする

[Indexes On Expressions](https://www.sqlite.org/expridx.html) によれば、expression index は列に対する式、または VIRTUAL generated column を key にできる。式に使える関数は deterministic に限られ、アプリ独自関数は既定では対象外である。登録時の `SQLITE_DETERMINISTIC` は SQLite にその性質を伝える指定であり、関数の実装変更を検査する仕組みとは記述されていない。

[stale index guide](https://www.sqlite.org/staleexpridx.html) が説明する問題は、同じ行から再計算した式の値と index に保存済みの値が異なる状態である。CPU / OS、SQLite や外部ライブラリ、アプリ独自関数の変更が確認対象になる。明示的な CAST がなくても暗黙変換や JSON の `->>` による浮動小数点抽出が関係し得る。公式は稀な問題とし、検索範囲の境界で結果差があり得ると説明する。

これは query planner の式の一致条件とは別である。[expression index reference](https://www.sqlite.org/expridx.html) は、WHERE / ORDER BY の式が index 定義と一致する必要があり、数学的に同値でも自動変形しないと説明する。たとえば `x+y` と `y+x` の不一致による index 不使用を stale index と混同しない。

## 公式契約: self-healing は全件検査の完了通知ではない

[stale index guide の self-healing 節](https://www.sqlite.org/staleexpridx.html) は、3.53.0 以降、該当する行の UPDATE / DELETE 時に古い式 index を修復する場合があると説明する。旧版でエラーになった場面を救済するもので、開いた時点や SELECT だけで全 index を再構築する保証は示されていない。

`PRAGMA integrity_check` は stale entry を引き続き報告する。浮動小数点の小さなずれでは、破損という表現の代わりに imprecise floating-point value を示す診断になり得る。したがって「UPDATE が成功した」だけでは、未更新行を含めた検査結果の代わりにならない。この最後の判断は、修復契機から導いた運用上の推論である。

[PRAGMA reference](https://www.sqlite.org/pragma.html#pragma_integrity_check) では、`integrity_check` は欠落・余分な index entry などを検査し、問題がなければ `ok` の1行を返す。`quick_check` は index と table の内容一致および UNIQUE 検査を省略するため、今回の検査の代用にしない。全体検査と特定 table の部分検査も区別し、後者から database 全体の健全性を結論しない。

## 公式契約: REINDEX の対象と名前衝突

[REINDEX reference](https://www.sqlite.org/lang_reindex.html) を根拠に、実行範囲を分ける。

- `REINDEX;` は全 attached database の全 index を再構築する
- `REINDEX schema.index_name;` は指定 index が対象。table を指定した場合はその table の全 index が対象となる
- `REINDEX name;` は collation 名の一致を table / index 名より優先する。特定 object を狙う場合は schema を付け、曖昧性を避ける
- 3.53.0 で追加された `REINDEX EXPRESSIONS;` は expression index をまとめて再構築する
- ただし `expressions` という collation / table / index が存在すると、上記の名前解決規則に従う追加の再構築も行われる。式 index 以外は常に無変更、と一律に扱わない

stale index guide の一般説明は式 index がなければ no-op とするが、実行対象の厳密な判断には REINDEX reference の名前衝突の但し書きも含める。旧版で同じ SQL 文字列が全 expression index 一括という意味になるとは仮定しない。

## 設計判断: 移行時の確認手順

以下は一次資料から導いた運用案であり、SQLite の保証する手順や今回の実行試験結果ではない。

1. 実際のアプリ組込み SQLite の版、独自関数・collation・依存ライブラリ、CPU / OS を記録する。OS 付属 CLI の版だけでアプリも 3.53 対応と判定しない
2. schema と index 定義を棚卸しし、VIRTUAL generated column 上の index も対象へ含める。対応 CLI では `.indexes --expr` を補助に使えるが、これは SQL ではない。独自関数を使う database は、その関数を同じ実装で利用できる環境で調べる
3. 稼働中 database に直ちに一括 REINDEX を投げず、復元可能な検証用コピーで変更前後を比較する。読み書きの停止許容、対象 index 数、再構築時間・領域・ロック待ちは実測し、本書から無停止や所要時間を保証しない
4. 更新後に採用する関数・collation の実装を定めてから再構築する。対象を絞れる場合は schema-qualified な index 指定を検討する。一括指定なら attached database と `expressions` の名前衝突を先に確認する
5. 再構築後は対象 schema の `integrity_check` の全出力を保存し、検索境界値と UPDATE / DELETE のアプリ結果も確認する。`quick_check` の成功や診断文字列の抑制を修復の証拠にしない
6. 旧 runtime への rollback や異なる端末への database 配布が必要なら、その環境も同じ検証に含める。一度新環境で再構築したことから、異なる計算実装との往復互換性までは導けない

独自関数の挙動を意図的に変える場合は、単なる index の再構築で業務上の意味の変更も承認されたと扱わず、期待値を別途決める。

## STORED generated column という設計上の選択肢

[Generated Columns](https://www.sqlite.org/gencol.html) は、VIRTUAL を読込み時、STORED を行の書込み時に計算すると規定する。STORED 列上の index は通常の index であり、VIRTUAL 列上の index は expression index である。STORED は格納領域を使い、生成式は deterministic な scalar 関数などに制限される。

式 index の再計算と保存済み key のずれを避けたい設計では、STORED 列へ materialize して通常の index を作る案を比較できる。ただし `ALTER TABLE ADD COLUMN` で STORED 列は追加できない。生成列の対応は 3.31.0（2020-01-22）以降なので、古い reader との schema 互換性も確認する。

運用上の推論として、STORED 化は「既存の保存値が新しい関数の意味へ自動で全件更新される」という契約ではない。関数変更後の既存行の再計算・移行方針は別途必要であり、領域・移行費用・読書き比率を測って選ぶ。

## 検証案と未確認事項

今回追加する eval は本文の検索可能性だけを検査する。次の runtime 受入試験は未実行である。

- 旧版で構築した浮動小数点 / JSON 抽出 / 独自関数 index を、新 runtime で検査する
- 対象行の UPDATE / DELETE と、未更新行の SELECT / 整合性検査を区別する
- `expressions` の collation / object がある場合の一括 REINDEX の対象を確認する
- VIRTUAL 列 index を棚卸しから落とさず、STORED 移行の schema 制約を確認する
- アプリに組み込んだ SQLite と検査 CLI の版・関数実装が異なる場合を検出する

全 CPU / OS 間の数値互換性、vendor backport、binding ごとの診断変換、任意の破損や非 deterministic 関数の自動修復、停止時間・負荷限界は未確認。self-healing を一般的な corruption recovery として拡張解釈しない。外部コードのコピーや module 昇格は行っていない。

## 出典・日付・ライセンス

すべて 2026-10-02 UTC にページ本文を確認した。公開 / 更新日は取得日と区別する。

- [3.53.0 release notes](https://www.sqlite.org/releaselog/3_53_0.html): 公開版・日付 3.53.0 / 2026-04-09。新機能の導入境界
- [3.52.0 withdrawal](https://www.sqlite.org/releaselog/3_52_0.html): 見出し 2026-03-06、withdrawal 自体の日付は明示なし。廃止版の扱い
- [Stale Expression Indexes](https://www.sqlite.org/staleexpridx.html): 表示更新 2026-06-13 15:05:34Z。原因・診断・self-healing の条件
- [REINDEX](https://www.sqlite.org/lang_reindex.html): 表示更新 2026-03-21 13:08:51Z。対象と名前解決規則
- [Generated Columns](https://www.sqlite.org/gencol.html): 表示更新 2026-05-14 15:13:28Z。VIRTUAL / STORED と導入版
- [Indexes On Expressions](https://www.sqlite.org/expridx.html): ページ内の更新日表示なし。関数制約と式の一致条件
- [PRAGMA reference](https://www.sqlite.org/pragma.html#pragma_integrity_check): 表示更新 2026-06-04 01:35:31Z。integrity_check / quick_check の検査範囲
- [SQLite Copyright](https://www.sqlite.org/copyright.html): 表示更新 2026-01-12 12:33:24Z。SQLite のコードと文書の public domain 宣言を確認。本文は独自の要約・運用案であり、当 repository 自体のライセンスを決めるものではない

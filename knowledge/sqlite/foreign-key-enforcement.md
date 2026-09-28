---
{
  "id": "sqlite-foreign-key-enforcement",
  "title": "SQLite の外部キー施行: PRAGMA foreign_keys と DEFERRABLE 遅延制約・参照アクション・foreign_key_check",
  "kind": "knowledge",
  "technology": "sqlite",
  "version": "SQLite 3.6.19+ (foreign key support introduced 2009-10-14; foreignkeys page updated 2026-03-20)",
  "tags": ["sqlite", "foreign_keys", "foreign_key_check", "defer_foreign_keys", "DEFERRABLE", "CASCADE", "RESTRICT", "research-domain:data"],
  "sources": [
    {"id": "sqlite-foreign-keys-docs", "url": "https://www.sqlite.org/foreignkeys.html", "type": "official_docs"},
    {"id": "sqlite-pragma-foreign-keys-docs", "url": "https://www.sqlite.org/pragma.html#pragma_foreign_keys", "type": "official_docs"},
    {"id": "sqlite-create-table-foreign-key-docs", "url": "https://www.sqlite.org/lang_createtable.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active"
}
---

# SQLite の外部キー施行: PRAGMA foreign_keys と DEFERRABLE 遅延制約・参照アクション・foreign_key_check

外部キー制約を宣言しても、施行にはビルド条件と接続単位の `PRAGMA foreign_keys=ON` が要る。以下は公式文書で確認した契約だけを書く。

## 要点

- 外部キー施行には、`SQLITE_OMIT_FOREIGN_KEY` と `SQLITE_OMIT_TRIGGER` なしでビルドされていることに加え、接続ごとに `PRAGMA foreign_keys=ON` が必要である。[SQLite Foreign Key Support](https://www.sqlite.org/foreignkeys.html)
- `PRAGMA foreign_keys` の既定は 3.6.19 以降 `OFF` であり、コンパイル時オプション `SQLITE_DEFAULT_FOREIGN_KEYS` で上書きできる。[PRAGMA Statements](https://www.sqlite.org/pragma.html#pragma_foreign_keys)
- `PRAGMA foreign_keys` はトランザクション内では no-op である。施行の切替はトランザクション外で行う。[PRAGMA Statements](https://www.sqlite.org/pragma.html#pragma_foreign_keys)
- 即時制約は文ごとに検査され、`DEFERRABLE INITIALLY DEFERRED` の遅延制約は `COMMIT` 時に検査される。暗黙トランザクションでは遅延制約も即時制約と同様に振る舞う。[SQLite Foreign Key Support](https://www.sqlite.org/foreignkeys.html)
- `ON DELETE` / `ON UPDATE` の参照アクションは `NO ACTION`・`RESTRICT`・`SET NULL`・`SET DEFAULT`・`CASCADE` であり、文書に記載の意味と処理順序に従う。[SQLite Foreign Key Support](https://www.sqlite.org/foreignkeys.html)
- `MATCH` 句は構文解析されるが、常に `MATCH SIMPLE` として扱われる。[SQLite Foreign Key Support](https://www.sqlite.org/foreignkeys.html)
- `RESTRICT` の効果は遅延外部キーであっても即時に及ぶ。[SQLite Foreign Key Support](https://www.sqlite.org/foreignkeys.html)
- `PRAGMA foreign_key_check` は違反ごとに1行を返し、4列は子テーブル・rowid（`WITHOUT ROWID` では NULL）・親テーブル・`foreign_key_list` に対応する外部キー番号である。テーブル名を指定して対象を絞れる。[PRAGMA Statements](https://www.sqlite.org/pragma.html#pragma_foreign_keys)
- `PRAGMA integrity_check` は `FOREIGN KEY` エラーを見つけない。違反の検出には `PRAGMA foreign_key_check` を使う。[PRAGMA Statements](https://www.sqlite.org/pragma.html#pragma_foreign_keys)
- `PRAGMA defer_foreign_keys=ON` はすべての外部キーを最外の `COMMIT` まで遅延させる。既定は `OFF` で、`COMMIT` または `ROLLBACK` 時に自動でリセットされる。[PRAGMA Statements](https://www.sqlite.org/pragma.html#pragma_foreign_keys)
- `foreign-key-clause` の構文は `REFERENCES foreign-table [(columns)]` に `ON DELETE` / `ON UPDATE` の各アクション（`SET NULL`・`SET DEFAULT`・`CASCADE`・`RESTRICT`・`NO ACTION`）、`MATCH name`、`NOT DEFERRABLE` / `DEFERRABLE INITIALLY DEFERRED` / `DEFERRABLE INITIALLY IMMEDIATE` を組み合わせる。[CREATE TABLE](https://www.sqlite.org/lang_createtable.html)
- 外部キーの親キーは名前付き列でなければならず、rowid は使えない。[CREATE TABLE](https://www.sqlite.org/lang_createtable.html)

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する数値や構成を指すものではない。

- 接続を開いたらトランザクション開始前に `PRAGMA foreign_keys=ON;` を実行し、問い合わせで `ON` であることを確認してから本処理に進む。既定 `OFF` とトランザクション内 no-op の契約のため、設定の成否検査を省略しない。
- 親子をまたぐ複文更新では、制約を `DEFERRABLE INITIALLY DEFERRED` で宣言するか、`PRAGMA defer_foreign_keys=ON` で最外 `COMMIT` まで遅延させる。ただし `RESTRICT` は遅延下でも即時効果を持つため、削除・更新の阻止条件としては即時検査を前提に順序を組む。
- 移行や修復後の検証には `PRAGMA foreign_key_check;`（必要ならテーブル名付き）を使い、`integrity_check` の成功を外部キー健全性の根拠にしない。`WITHOUT ROWID` テーブルでは第2列が NULL になる契約を読み取り側で考慮する。
- 親キーには必ず名前付き列（主キーまたは一意制約のある列）を指定し、rowid への参照を書かない。

## 避ける使い方

- 外部キーを宣言しただけで施行されると見なす。ビルド時の omit 指定や接続単位の既定 `OFF` により、宣言があっても検査されない運用になる。
- トランザクション開始後に `PRAGMA foreign_keys` を切り替えて効果を期待する。文書上は no-op であり、施行状態は変わらない。
- `integrity_check` の成功をもって外部キー違反なしと判断する。文書上 `FOREIGN KEY` エラーは検出対象外であり、`foreign_key_check` を使わないと違反を見落とす。
- `MATCH` 句に `SIMPLE` 以外の意味を期待する。文書上は解析のみで常に `MATCH SIMPLE` 扱いである。
- 親キーに rowid を指定する。文書上は名前付き列だけが許され、rowid 参照は契約外である。
- 遅延制約下で `RESTRICT` の検査も `COMMIT` まで先送りされると見なす。文書上 `RESTRICT` は即時効果であり、遅延を前提にした文順序は阻止される。

## 適用版と本番での注意

- 外部キー対応は `SQLite 3.6.19`（2009-10-14）導入の契約で確認する。`foreignkeys` 頁の更新は 2026-03-20、`lang_createtable` 頁の更新は 2025-04-30 である。将来の最新とは扱わない。
- 本文の接続時設定・遅延選択・検証手順は設計判断であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。
- 未確認事項: 各参照アクションの詳細な処理順序、トリガーとの相互作用、WAL や並行書込下での検査タイミング。これらは今回の検証対象外であり、必要になれば公式文書の該当節を別途確認する。推測で順序や並行性を主張しない。

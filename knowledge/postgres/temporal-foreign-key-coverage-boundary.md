---
{
  "id": "postgres-temporal-foreign-key-coverage-boundary",
  "title": "PostgreSQL 18 temporal FK: PERIOD の全期間被覆と WITHOUT OVERLAPS の移行境界",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18 versioned manual verified 2026-10-03 UTC; WITHOUT OVERLAPS / PERIOD introduced in 18 (2025-09-25); runtime tests not executed",
  "tags": [
    "research-domain:data",
    "temporal",
    "period",
    "without-overlaps",
    "range",
    "multirange",
    "foreign-key",
    "coverage",
    "null",
    "gist",
    "migration",
    "deferrable"
  ],
  "sources": [
    {
      "id": "postgres-temporal-release-18-20261003",
      "url": "https://www.postgresql.org/docs/18/release-18.html",
      "type": "release_notes"
    },
    {
      "id": "postgres-temporal-create-table-18-20261003",
      "url": "https://www.postgresql.org/docs/18/sql-createtable.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-temporal-range-boundaries-18-20261003",
      "url": "https://www.postgresql.org/docs/18/rangetypes.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-temporal-btree-gist-18-20261003",
      "url": "https://www.postgresql.org/docs/18/btree-gist.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-temporal-alter-table-18-20261003",
      "url": "https://www.postgresql.org/docs/18/sql-altertable.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-temporal-set-constraints-18-20261003",
      "url": "https://www.postgresql.org/docs/18/sql-set-constraints.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-temporal-documentation-license-18-20261003",
      "url": "https://www.postgresql.org/docs/18/legalnotice.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# PostgreSQL 18 の temporal FK と全期間被覆

## 問いと採用判断

契約期間を分割して保存するとき、子の利用期間に「同じ顧客の親が一件ある」だけで十分か。親の期間を短縮・分割しても、子の全期間をカバーできることをデータベースで守れるか。

**判断:** 親の同一業務キーで期間が重ならないことは `WITHOUT OVERLAPS`、子の期間が同じキーの親行の集合で完全に覆われることは `PERIOD` 外部キーで表す。親を一行だけ探す存在確認、単なる overlap、最小開始日と最大終了日だけの確認では、この二つの条件を代替できない。

既存の[transaction再試行](transaction-isolation-retry.md)や[CONCURRENTLYによるmigration](zero-downtime-migration-concurrently.md)にはない、期間付き参照の意味と導入境界を扱う。新しいニュースの紹介ではなく、未収録だった18系のschema設計の判断材料である。

## 導入版と適用範囲

[PostgreSQL 18 release notes](https://www.postgresql.org/docs/18/release-18.html) は公開日2025-09-25で、PRIMARY KEY / UNIQUE の `WITHOUT OVERLAPS` と FOREIGN KEY の `PERIOD` を導入した。2026-10-03 UTCに18系のversioned manualを読み直した。17以前のDDLにこの構文を逆適用しない。

取得時の公式manualの版一覧は18をsupported、19をdevelopmentとして区別している。本稿は18系の文書契約を対象とし、19 previewの機能・将来のGA・個別patchでの実行一致を保証しない。新規導入は採用する配布元の保守patchと拡張対応を別途確認し、18.0初版を推奨する意味にしない。

## 確認した契約

### 1. WITHOUT OVERLAPS は業務キーの単純な一意性ではない

[CREATE TABLE: UNIQUE / PRIMARY KEY](https://www.postgresql.org/docs/18/sql-createtable.html#SQL-CREATETABLE-PARMS-UNIQUE) では、最後の列に `WITHOUT OVERLAPS` を指定し、その列の重複を等値でなくoverlapで検査する。同じ業務キーを持つ行は、期間が重ならなければ複数存在できる。最後の列はrangeまたはmultirangeであり、empty range / empty multirangeは許されない。PRIMARY KEYなら構成列はNOT NULLになる。

この制約はGiSTを使い、通常のUNIQUEとは異なりexclusion型の検査になる。`bigint` の業務キーをrangeと組み合わせる本稿の例では、[btree_gist](https://www.postgresql.org/docs/18/btree-gist.html) のint8用opclassを前提にする。btree_gistを入れただけで普通の `CREATE UNIQUE INDEX ... USING gist` がB-treeの一意indexと同じになるわけではない。

設計上の帰結: 単なる `UNIQUE (customer_id, valid_during)` はrange値全体の等値を禁止する条件であり、異なる値の部分重複を防ぐ設計として使わない。反対に、通常の `UNIQUE (customer_id)` を加えると同じ顧客の期間分割まで禁止してしまう。

### 2. PERIOD は複数の親行から全期間の被覆を判定する

[CREATE TABLE: FOREIGN KEY](https://www.postgresql.org/docs/18/sql-createtable.html#SQL-CREATETABLE-PARMS-REFERENCES) によれば、少なくとも一つある非PERIOD列は等値で一致させる。PERIOD列自体は等値一致ではなく、該当する親行のPERIOD値を合わせた集合が子の値を完全に覆うことを要求する。単一の親行が子を丸ごと包含する必要はない。

参照先は `WITHOUT OVERLAPS` を宣言したPRIMARY KEYまたはUNIQUE制約が必要で、外部キーの参照先キーは非遅延、すなわちnon-deferrableであることも確認する。既存の任意の `EXCLUDE` を、動作が似ているという理由だけでtemporal FKの参照先にできるとは扱わない。子側の列にPERIODを付け、参照列リストを明記する場合は対応する親列にもPERIODを付ける。参照列を省略したときは親PRIMARY KEYの最終列がWITHOUT OVERLAPSであることが必要になる。

### 3. NULL・empty・無限境界を別にする

同じFOREIGN KEY契約では既定の `MATCH SIMPLE` は外部キー列のどれかがNULLなら親一致を要求しない。必須参照なら業務キーとPERIOD列の両方へ `NOT NULL` を付ける。`MATCH FULL` でも全列NULLは一致検査の対象外なので、NOT NULLの代用にはならない。

[Range Types](https://www.postgresql.org/docs/18/rangetypes.html) はemptyを値を一つも含まない集合、境界省略を無限側の範囲として区別する。range constructorにNULLの境界を渡す場合もその側が無限になる。これはrange列全体のSQL NULLとは違う。アプリの未入力をそのままconstructorへ渡し、意図しない無期限契約を作らない。

親のWITHOUT OVERLAPSにはempty拒否が文書化されている。子の業務要件も「少なくとも一日」なら、後述のようにNOT NULLに加えて `CHECK (NOT isempty(...))` を明示する。本稿は、親のempty規則があらゆる子schemaにも自動適用されるという推測に依存しない。

### 4. 親期間の更新は子の自動分割・削除ではない

18系のCREATE TABLEはtemporal FKで `RESTRICT`、`CASCADE`、`SET NULL`、`SET DEFAULT` を未対応とし、既定の `NO ACTION` を使う。親期間の短縮を契機に、子の期間を自動で切り詰めたり二行へ分割したりする契約ではない。被覆を失う変更は制約検査で拒否されるものとして設計する。

子のFKを `DEFERRABLE` にする選択は、参照先キーを非遅延に保つ条件と区別する。[SET CONSTRAINTS](https://www.postgresql.org/docs/18/sql-set-constraints.html) は現在transaction内で検査を延期でき、DEFERREDからIMMEDIATEへの変更時には未検査の変更もその場で検査する。NOT NULLとCHECKはこの設定で遅延しない。

## 最小schemaと具体的な期待結果（独自例・未実行）

以下は新規の隔離した検証DB向けに作成した例であり、本番移行スクリプトではない。btree_gistが利用可能で、対象DBに承認された導入が済んでいることを前提とする。公式のコード例は転載していない。

```sql
CREATE TABLE plan_terms (
    customer_id bigint NOT NULL,
    valid_during daterange NOT NULL,
    CONSTRAINT plan_terms_temporal_pk
        PRIMARY KEY (customer_id, valid_during WITHOUT OVERLAPS)
);

CREATE TABLE entitlements (
    entitlement_id bigint PRIMARY KEY,
    customer_id bigint NOT NULL,
    required_during daterange NOT NULL,
    CONSTRAINT entitlements_nonempty
        CHECK (NOT isempty(required_during)),
    CONSTRAINT entitlements_coverage_fk
        FOREIGN KEY (customer_id, PERIOD required_during)
        REFERENCES plan_terms (customer_id, PERIOD valid_during)
        DEFERRABLE INITIALLY IMMEDIATE
);

INSERT INTO plan_terms VALUES
    (42, '[2026-01-01,2026-04-01)'::daterange),
    (42, '[2026-04-01,2026-07-01)'::daterange);

INSERT INTO entitlements VALUES
    (9001, 42, '[2026-02-01,2026-06-01)'::daterange);
```

公式契約から導く最後のINSERTの期待結果は成功である。親の二行を合成すると必要期間の全体を覆う。二行目は4月1日を含み、一行目は含まないため、その境目に重複も穴もない。**これは実機で観測したログではなく、採用環境で検証すべき期待値である。**

各否定例は独立した初期状態またはsavepointから試す。失敗したtransactionに次のSQLを続けて、すべて同じ失敗に見せない。

- 同じ親二行で子を `[2026-02-01,2026-07-02)` にすると拒否を期待する。7月1日の被覆がない。親に一件overlapがあることは全期間被覆の証明ではない
- 別fixtureで親二行目の開始を4月2日にすると、元の子INSERTも拒否を期待する。最小開始日 `min` と最大終了日 `max` は変わらないが、4月1日に穴がある
- customer_id=43の子に同じ期間を入れると、42の親が覆っていても拒否を期待する。別顧客の期間を全体集計へ混ぜない
- 元の親へ42の `[2026-03-31,2026-04-02)` を追加すると、親のWITHOUT OVERLAPSで拒否を期待する。これは子のFK違反とは別の不変条件である
- 子のcustomer_idまたはrequired_duringへSQL NULLを入れると、この例ではNOT NULLで拒否する。子のemptyは明示したCHECKで拒否する
- 子9001がある状態で二行目の親を削除し、そのまま終了すると被覆を失うため拒否を期待する。CASCADEで子が消えるという成功条件にしない

### daterange の境界は表示文字列だけで判断しない

Range Typesは `daterange` の正規形を下端を含み上端を含まない `[)` とする。たとえば「6月30日まで有効」を `[2026-01-01,2026-06-30)` と保存すると6月30日を含まない。画面の終了日とDB上端の対応を明示し、6月30日・7月1日の値そのものを試験に使う。

multirangeは離れた複数区間を一値にできる。PERIODの完全被覆は子の集合全体についての条件で、子multirangeの内側の空白まで新しく必要期間になるわけではない。業務上「中断のない一期間」を求めるなら、単一区間のrangeを選ぶ案が単純である。min/maxの外包区間へ置き換えて同じ意味としない。

## 分割更新とmigrationの組み立て方（独自の運用案）

1. **まず意味を固定する。** 顧客などの非PERIODキー、日付か時刻か、両端の包含、NULL、empty、無期限を定義する。親同士の重複と子の未被覆を別々に棚卸しする。事前SELECTの成功だけで並行writerの将来入力を保証しない
2. **親キーと子FKの遅延を分ける。** 一つの親期間を二行へ分割するなら、子FKだけを必要なtransactionでDEFERREDにし、旧親を除去して非重複の新親を作り、最後に同FKをIMMEDIATEへ戻して被覆を検査する構成を試す。親キーは非遅延なので、新親を先に重ねてから旧親を消す順序は拒否され得る。constraint名の衝突も避け、安易にALLを遅延しない
3. **transactionの成否で確定する。** 上記の途中で子の参照先が一時的に足りなくても、最後の被覆検査とCOMMITまで成功しない限り業務変更を確定しない。延期は整合性検査をなくす仕組みではなく、外部通知の確定保証でもない。並行更新・待機・deadlockは採用環境で別途試す
4. **B-treeのonline移行recipeを流用しない。** [ALTER TABLE](https://www.postgresql.org/docs/18/sql-altertable.html) の `ADD ... USING INDEX` は既定整列のB-treeを要求する。temporal keyのGiSTを先にCONCURRENTLY構築して同じ構文で取り付ける手順は、この18系の契約に合わない。`NOT VALID` の対象種別にもPRIMARY KEY/UNIQUEは含まれないので、親temporal keyの検査を後回しにできるとは説明しない
5. **DDLのlockを計画する。** 同資料ではADD制約の多くはACCESS EXCLUSIVE、ADD FOREIGN KEYは子・親の両tableにSHARE ROW EXCLUSIVEを取る。キー構築、既存行検査、待機時間を検証用コピーで測る。停止時間を許容できない既存大表では、この短いschema例だけを根拠に本番変更せず、別表への段階移行・切替を個別に設計する
6. **運用経路まで受入確認する。** ORMがPERIOD/非重複キーを正しく保存・再生成するか、schema diff、dump/restore、親の分割・短縮、子の延期検査を確認する。schemaが作れたことだけで全writerの動作や移行完了としない

btree_gistは通常B-treeより速いことを目的とする拡張ではない。DB制約で期間整合性を共有できる利点と、GiST維持・FK検査・期間変更の費用を比べる。大量の期間分割がある顧客、長い子期間、頻繁な親変更を含むworkloadで測り、一律の性能改善値を作らない。

## 未確認事項・出典・provenance

PostgreSQL serverを起動しておらず、SQL例、並行writer、分割transaction、migrationのlock時間、ORM、managed serviceでの実行は未検証。partitioned table、独自range/opclass、異なるrange型同士のFK、全patchの差、temporal FKをNOT VALIDで追加する詳細経路は今回の対象外である。一般のFK説明から未検証のtemporal migration全体を成功済みとしない。検索evalはこの資料の可発見性だけを確認する。

資料の取得日はすべて2026-10-03 UTC。release notesの公開日は2025-09-25で、その他18系manual各ページには個別の公開・更新日表示がない。取得日を仕様導入日と混同しない。repository実装の解析はしていないため、commit固定の実装証拠を持つという主張も行わない。

[18 Legal Notice](https://www.postgresql.org/docs/18/legalnotice.html) と[公式License](https://www.postgresql.org/about/licence/)で、softwareとdocumentationを対象とするPostgreSQL Licenseを確認した。原文・実装コードの転載なし。本文の運用判断とSQLはこの資料用の独自作成であり、module昇格やこのrepository全体のライセンス決定は行わない。

release_notesの30日TTLが最短なので再確認期限は2026-11-02。次回はPERIODの参照action、NULL、参照先条件、USING INDEXの制限まで読み直し、取得日だけを延長しない。

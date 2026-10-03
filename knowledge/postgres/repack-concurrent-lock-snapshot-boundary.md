---
{
  "id": "postgres-repack-concurrent-lock-snapshot-boundary",
  "title": "PostgreSQL 19beta4 REPACK CONCURRENTLY: 最終lock・古いsnapshot・追加容量の採用境界",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 19beta4 bundled documentation; REL_19_BETA4 at b73d13c32c834a2c8e1c60cb92f79530376cedf1; announced 2026-09-24; runtime behavior not tested",
  "tags": [
    "research-domain:data",
    "repack",
    "concurrently",
    "table-rewrite",
    "mvcc",
    "snapshot",
    "disk-space",
    "replication-slot",
    "preview"
  ],
  "sources": [
    {
      "id": "postgres-repack-beta4-command-20261003",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/ref/repack.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-repack-beta4-mvcc-20261003",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/mvcc.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-repack-beta4-config-20261003",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/config.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-repack-beta4-progress-20261003",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/monitoring.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-repack-beta4-release-20261003",
      "url": "https://www.postgresql.org/about/news/postgresql-19-beta-4-released-3386/",
      "type": "release_notes"
    },
    {
      "id": "postgres-repack-development-command-20261003",
      "url": "https://www.postgresql.org/docs/19/sql-repack.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-repack-vacuum18-comparison-20261003",
      "url": "https://www.postgresql.org/docs/18/sql-vacuum.html",
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

# REPACK CONCURRENTLY の採用境界

## 問い・結論・適用版

肥大化した PostgreSQL table の領域を OS に返すため、REPACK CONCURRENTLY を停止時間のない定例処理として導入できるか。採用判断では、最終lock、古いsnapshot、作業用disk容量、対象tableの条件を別々に評価する。CONCURRENTLYという名前だけで、read/writeの無停止やsnapshotの安全性を約束しない。

対象は PostgreSQL 19beta4 の同梱文書である。公式repositoryの tag REL_19_BETA4 は commit b73d13c32c834a2c8e1c60cb92f79530376cedf1 に固定し、commit message「Stamp 19beta4.」と2026-09-21のcommit日時を確認した。[公開告知](https://www.postgresql.org/about/news/postgresql-19-beta-4-released-3386/)の日時は2026-09-24であり、commit日と公開日を混同しない。同告知はREPACKのcrash、invalid index・materialized viewでの不正動作、権限・error報告の修正を挙げる。すべての不具合が解消したとは読み替えない。

[19公開manual](https://www.postgresql.org/docs/19/sql-repack.html)も2026-10-03 UTCに開いて照合したが、unsupported versionの表示がある。本書はpreviewの評価用で、19 GAの出荷保証や本番採用の推奨ではない。server実装やbinaryを実行しておらず、以下の「確認」は固定した同梱文書の契約確認を意味する。

既存のzero-downtime migration文書が扱うindex/constraintの段階的変更とは異なり、本書はtable全体のrewriteを対象とする。外部拡張のpg_repackを調査したものでもない。

## 先に通常VACUUMで足りるかを判断する

[PostgreSQL 18 VACUUM](https://www.postgresql.org/docs/18/sql-vacuum.html)では、通常のVACUUMは主にtable内部で再利用する空間を回収する。VACUUM FULLはtable全体を新しいfileへ書き直し、処理中のACCESS EXCLUSIVE lockと追加disk容量を必要とする。通常VACUUMでも末尾pageの切り詰めでは強いlockを必要とし得るので、「通常ならlockなし」とも扱わない。

独自の判断: 今後の更新で再利用できる空間を保持してよいなら、まず通常の保守と容量推移を調べる。OSへ容量を返す必要や物理配置を変える理由が明確になってからrewriteを選ぶ。空き容量が尽きかけた時点で、その場しのぎにREPACKを開始する設計にはしない。

## CONCURRENTLYでも最後の排他lockは残る

[19beta4 REPACK文書](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/ref/repack.sgml)では、新tableとindexを作成し、その間の変更をlogical decodingで捕捉して反映した後、fileを交換する。CONCURRENTLYはその交換に必要なACCESS EXCLUSIVE lockをなくすものではない。lock待ちの間にも大量の変更が増えると、取得後に残りを適用する時間も必要となる。

独自の運用判断: 所要時間を「コピー時間」「追随時間」「最終lock待ち・保持時間」に分けて見積もる。大規模更新や長いtransactionと重なる時間帯を避け、許容する停止時間を事前に決める。短いlockになるという一般説明から、特定workloadの上限msを推測しない。DDLの並行実行も失敗要因として扱い、schema deployとの同時実行を避ける。

## 対象tableと呼び出し方を事前に絞る

同じ[固定REPACK文書](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/ref/repack.sgml)の利用条件を、preflightで次のように点検する。

- tableへのMAINTAIN権限を持ち、invalid indexが残っていないこと。invalid indexを検出したら、用途を確認した修復・再作成を別手順で行う。REPACK成功のためだけにindexを削除しない
- CONCURRENTLYの対象はloggedなheap tableであること。materialized view、UNLOGGED、partitioned table、system catalog、TOAST、user_catalog_table指定などの禁止条件を除外する。partitioned parentに指定して全partitionを並行処理できるとは扱わない
- primary key または index-based replica identity を持つこと。primary keyも該当indexもないtableは不可であり、REPLICA IDENTITY FULLという設定名だけではこの条件を満たさない
- CONCURRENTLYではtableを明示し、transaction blockの外から発行する。migration frameworkが自動でBEGINする経路を確認する。ANALYZE optionにもtransaction・function等の追加制約があるため、実際の呼び出し経路で別途検査する

[19beta4 configuration](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/config.sgml)のmax_repack_replication_slotsはREPACK用slot数を制限し、既定は5、変更はserver start時である。slot枠不足を無制限な再試行で解決しようとせず、保守jobの同時数を制御する。値を変える計画には再起動の検討が必要で、session内の設定だけで増やせるとは扱わない。通常のmax_replication_slotsとの内部会計やWAL保持の全仕様は本調査で検証していない。

## 古いsnapshotではtableが空に見える場合がある

[19beta4 MVCC caveats](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/mvcc.sgml)は、REPACK CONCURRENTLYをMVCC-safeではないcommandに含める。rewriteのcommit以前に取得したsnapshotを使う並行transactionでは、rewrite後に対象tableが空に見える場合がある。

重要なのは対象tableへの先行アクセスである。command開始前からそのtableへアクセスしているtransactionなら、少なくともACCESS SHARE lockを保持し、rewrite側を待たせる。文書が警告するのは、対象tableをまだ参照していない古いsnapshotとの境界であり、同じtransactionが対象tableを何度も読んだだけで必ず内容が消える、という意味ではない。他tableとの見え方の不整合は起こり得る。

独自の評価例: REPEATABLE READで先に別tableを読み、snapshotを維持する。その間に別sessionで対象tableをrewriteし、完了後に最初のsessionから対象tableを初めて読む。この試験と、最初から対象tableを読んでrewriteを待たせる試験を分ける。集計・batch・長時間read transactionがある環境では、最終lock時間の測定だけで採用可としない。

## 空き容量の見積りと進捗の読み方

固定REPACK文書の資源説明に基づき、tableサイズをT、index合計をIとすると、sortなしの新file用空き容量は少なくともT+I、sequential scanとsortを使う場合のtemporary spaceは最大で2T+I程度になる。これは既存fileに加えて必要な作業用diskの話である。さらにCONCURRENTLYでは、コピー中に発生して直ちに適用できない変更のtemporary fileも増える。

独自の容量計画: T+Iや2T+Iを総使用量の厳密な上限にせず、継続DML、WAL、他jobの消費と安全余裕を別枠で確保する。必要量や増加率を見積もれないtableでは開始を見送る。個別環境のWAL倍率や一時fileのピーク値は実測が必要である。

[19beta4 monitoring](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/monitoring.sgml)のpg_stat_progress_repackにはphaseと各種counterがある。heap_blks_totalはseq scanning heap開始時点の値、heap_blks_scannedはそのscan段階で進む。catch-upではコピー中のDMLを処理し、heap_tuples_updated/deletedが進む。scan比率が100%でも、catch-up、交換、後片付けの完了を意味しない。

独自の監視案: phase滞在時間、保守sessionの成否、対象tableの応答時間、disk空き容量を組み合わせる。counterの増加だけから「残り何秒」や「新規変更より追随が速い」と断定せず、phaseごとに観測する。progress行が消えただけでは成功・失敗を区別できないため、commandの最終結果も保存する。

## 再試行と運用試験（独自の手順案）

以下は未実行の設計案で、PostgreSQLが提供する自動復旧保証ではない。

1. 開始前にbinary版、対象table、indexのvalid状態、権限、replica identity、slot枠、容量を記録する。許容する待機時間と中止基準を決め、DDL deployを重ねない
2. 制約違反、invalid index、容量不足、slot枠不足は、原因が変わるまで再試行しない。REPACKを通すために権限や永続設定を無条件に広げない
3. DDL競合や接続中断の後は、元の保守sessionの結果・稼働状態と対象schemaを照合する。同じ対象へ新jobを重ねず、再実行に必要な容量と条件をもう一度確認する
4. 成功時はtable/indexの状態、主要queryの結果と計画、容量の変化を確認する。clusteringを使った場合は適切なANALYZEも計画する。新しく挿入された行の順序や将来の更新で物理順が維持されるとは期待しない
5. 正常系だけでなく、長いsnapshot、最終lockを妨げるtransaction、書き込み集中、DDL競合、invalid index、materialized view、slot枠飽和を別々に試す。主キーなしでREPLICA IDENTITY FULLのみのcaseもpreflightの否定例に含める

serverのcancel・crash時の内部後片付け、slotのWAL保持上限との相互作用、extension index access method、managed service対応、RC/GAでの変更、性能と停止時間の実測は未確認。DB適合試験は実行していない。検索evalは文書が期待queryで取得できることを確認するものである。

## 出典・取得日・provenance

すべてのUTC取得日は2026-10-03。固定repository資料は19beta4の文書・COPYRIGHTの確認に限定し、実装を監査したとは扱わない。公開manualには個別更新日表示がなく、ニュースの公開日、tagのcommit日時、取得日を分けた。release_notesのTTL 30日に合わせ、再確認期限を2026-11-02とする。

[固定COPYRIGHT](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/COPYRIGHT)と[19 Legal Notice](https://www.postgresql.org/docs/19/legalnotice.html)はdocumentationも対象に含む。[公式License](https://www.postgresql.org/about/licence/)でPostgreSQL Licenseの名称を確認した。ニュース本文固有の再利用licenseは未確認なのでunknownを記録する。原文・実装コードを転載せず、出典を示した独自要約と設計案のみを追加し、moduleへ昇格しない。

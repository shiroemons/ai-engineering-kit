---
{
  "id": "postgres-sequence-refresh-switchover-boundary",
  "title": "PostgreSQL 19beta4 sequence同期: REFRESH完了・LSN盲点・switchoverの境界",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 19beta4 bundled documentation; REL_19_BETA4 at b73d13c32c834a2c8e1c60cb92f79530376cedf1; announced 2026-09-24; no runtime validation",
  "tags": [
    "research-domain:data",
    "postgres",
    "logical-replication",
    "sequence",
    "refresh-sequences",
    "switchover",
    "page-lsn",
    "preview"
  ],
  "sources": [
    {
      "id": "postgres-sequence-beta4-logical-contract-20261004",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/logical-replication.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-sequence-beta4-refresh-command-20261004",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/ref/alter_subscription.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-sequence-beta4-publication-scope-20261004",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/ref/create_publication.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-sequence-beta4-catalog-state-20261004",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/catalogs.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-sequence-beta4-worker-monitoring-20261004",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/monitoring.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-sequence-beta4-value-observation-20261004",
      "url": "https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/func/func-sequence.sgml",
      "type": "github_repository_analysis"
    },
    {
      "id": "postgres-sequence-preview-release-20261004",
      "url": "https://www.postgresql.org/about/news/postgresql-19-beta-4-released-3386/",
      "type": "release_notes"
    },
    {
      "id": "postgres-sequence-development-guide-20261004",
      "url": "https://www.postgresql.org/docs/19/logical-replication-sequences.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-sequence-v18-restrictions-20261004",
      "url": "https://www.postgresql.org/docs/18/logical-replication-restrictions.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# sequence同期とswitchoverの境界

## 問い・結論・適用版

PostgreSQL 19で「sequence replication」が使えるなら、logical replicationのsubscriberをそのまま書込み先へ切り替えてよいか。**継続的な採番変更は複製されないため、table追随、sequenceの対象集合、値の再同期、旧writerの停止を別々に確認する。** REFRESHコマンドの受付やLSN一致だけを切替許可にしない。本書の切替手順は公式契約から組み立てた独自の評価案であり、完成済みのHA手順や無損失failover保証ではない。

対象は19beta4同梱文書である。REL_19_BETA4をgit ls-remoteで確認し、commit [b73d13c32c834a2c8e1c60cb92f79530376cedf1](https://github.com/postgres/postgres/commit/b73d13c32c834a2c8e1c60cb92f79530376cedf1)の「Stamp 19beta4.」と2026-09-21T19:15:09Zを照合した。[公開告知](https://www.postgresql.org/about/news/postgresql-19-beta-4-released-3386/)は2026-09-24で、feature preview、beta期間中の仕様変更可能性を明記する。commit日時と公開日は別である。

2026-10-04 UTCに[公式homepage](https://www.postgresql.org/)も開き、19beta4のpreview表示とsupported release 18.6の掲載を確認した。[公開19manual](https://www.postgresql.org/docs/19/logical-replication-sequences.html)にはunsupported version表示がある。19のrelease notesに残る「2026-??-??」をGA公開日と解釈せず、本書はRC/GAの採用保証をしない。公開manualの個別更新日は表示されず、以下の主契約は固定したbeta4文書を正本にする。

既存のCDC output plugin許可リスト、REPACKのrewrite、WAITによるstandby読取りは本書の対象ではない。今回は「採番状態をlogical subscriberへ渡し、切替可能かを判定する」という未収録の問いに絞る。

## 18から変わったことと、変わらないこと

[18のRestrictions](https://www.postgresql.org/docs/18/logical-replication-restrictions.html)ではsequence dataは複製されず、serial/identity列の値がtableの一部として届いても、sequence自体は追随しない。[19beta4 logical replication文書](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/logical-replication.sgml)では、初回同期と明示的な再同期が追加される一方、incremental sequence changesは引き続き複製されない。最後に同期した後のpublisherのnextvalを、subscriberが自動追随するわけではない。

- 読取り専用subscriberでは、採番しない限りこの差が表面化しにくい
- table側に新しいIDの行があることは、subscriberの次の採番値が安全である証明にならない
- sequence同期にはpublisher 19以降が必要である。subscriberだけ19へ更新した18→19構成に、この再同期経路を適用しない
- schemaとDDLは複製されない。subscriber側のsequenceの存在と定義を、値の転送とは別に準備する

「19がsequenceを複製する」と「変更の継続配信はしない」は矛盾しない。ここで追加された機能は、指定した時点でsequence状態を同期する仕組みである。

## 対象集合の更新と値の更新を分ける

[19beta4 CREATE PUBLICATION](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/ref/create_publication.sgml)のFOR ALL SEQUENCESはdatabase内のpersistent sequenceを対象とし、将来作成されるものもpublicationの対象になる。temporaryとunlogged sequenceは含まれない。任意の1本だけをFOR SEQUENCEで指定できると推測しない。FOR ALL SEQUENCESの作成にはsuperuserが必要で、既存の限定的なtable publicationから権限・同期範囲を無条件に拡大しない。

[19beta4 ALTER SUBSCRIPTION](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/ref/alter_subscription.sgml)とlogical replication文書に基づく役割は次のとおり。

| 操作 | 対象集合 | sequence値 | 注意点 |
|---|---|---|---|
| CREATE SUBSCRIPTION、copy_data=true | subscriptionへ対象を登録 | 初期値を同期 | copy_dataの既定はtrue。falseで値のコピー完了とは判定しない |
| REFRESH PUBLICATION | 新規対象の追加・脱落対象の除外を反映 | copy_data=trueで新たに購読するsequenceを同期 | 既知のsequenceを再同期する操作ではない |
| REFRESH SEQUENCES | 現在subscriptionが知る集合を維持 | 既知の全sequenceを再同期 | 未登録sequenceを発見して追加する操作ではない |

publicationが将来sequenceを含むことと、subscriberが既にそのsequenceを知っていることは別である。対象が変わった場合はREFRESH PUBLICATIONを先に行い、必要なsequenceが登録されたことを確認してからREFRESH SEQUENCESで既知の値を更新する。コマンドを同じものの別名として使わない。

REFRESH PUBLICATIONはtransaction block内では実行できず、two_phase有効時にはcopy_data=falseでない限り追加制約がある。migration frameworkがBEGINで囲む場合は個別に検証する。このtransaction制約を、文書に記載のないREFRESH SEQUENCESへ自動的に拡張しない。

## commandの正常復帰と同期完了は別の観測点

[固定logical replication文書](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/logical-replication.sgml)では、上記subscriber commandの実行後にsequence synchronization workerが起動し、同期を終えて終了する。起動はmax_sync_workers_per_subscriptionに制限される。したがってcommand受付、worker起動、対象全件の同期は別段階として観測する。

定義不一致があるとworkerは対象を示すerrorをlogへ残して終了し、apply workerは成功するまで再起動を繰り返す。対応はsubscriber側のALTER SEQUENCEなどで意図した定義へ揃えることであり、単なる待機時間の延長では直らない。ALTER SUBSCRIPTION参照には不一致の「warnings」という案内も残るが、詳細章はworkerのerror・終了・再起動を明記する。軽い通知として無視せず、実機logと対象の状態を確認する。

[固定catalog文書](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/catalogs.sgml)では、pg_subscription_relのsequence状態はsrsubstateのi=initialize、r=readyである。srsublsnは最後の同期時にpublisherのsequence pageが持っていたLSNであり、tableの継続的なreplication progressと同じ意味ではない。

独自の完了判定案: 実行対象subscriptionを限定し、事前に作った期待sequence一覧とcatalogの実集合を照合する。対象漏れがなく、今回の同期後の状態・値を確認できたときだけ先へ進む。「返った行がすべてr」でも期待対象が0件・不足していれば成功にしない。rだけでは、その後publisherが進んでいないことも証明できない。

[固定monitoring文書](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/monitoring.sgml)に基づき、監視は次を分ける。

- pg_stat_subscription.worker_typeのsequence synchronizationは起動中のworker。relidとreceived_lsnはNULLであり、NULLを0件処理・異常値へ機械変換しない
- worker行が消えただけでは成功と失敗後の再起動待ちを区別できない。catalog、値、error logと突き合わせる
- pg_stat_subscription_stats.sync_seq_error_countはworkerでのerror回数。1workerが全sequenceを同期するため、増分1を「失敗sequenceが1本」と数えない
- counterの絶対値0を完了条件にせず、今回の操作前後の増分とstats_resetを読む。蓄積統計には反映遅延・transaction内cacheがあるため、同じ長時間transactionの固定表示をpolling結果と誤認しない

## page_lsn一致を「採番ずれなし」の証明にしない

固定logical replication文書は、subscriberのpg_subscription_rel.srsublsnとpublisherのpg_get_sequence_dataが返すpage_lsnの比較を案内する。ただし、sequenceはWAL記録をまとめるため、通常32値の同一block内で値が進んでもLSNの差に現れない場合がある。**LSN差は再同期の手掛かりになるが、LSN一致は値の一致を保証しない。** 通常32という説明を、全環境での固定的な許容ずれ件数や障害時の損失上限へ変換しない。

[固定Sequence Functions](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/doc/src/sgml/func/func-sequence.sgml)では、pg_get_sequence_dataのlast_valueはdiskに書かれた値であり、cacheを使う場合は最後に配布した番号より先にあることもある。is_calledとpage_lsnを併せて記録し、last_valueを「最後にcommitしたtable行のID」と同一視しない。

同関数はSELECT権限不足や不適切なrelation OIDなどでNULLの行を返す。観測不能を「変化なし」に正規化しない。また、nextvalは採番を進め、その値はtransactionをrollbackしても回収されない。setvalの変更もrollbackされないため、監視のためのSELECT nextvalや、試しにsetvalしてROLLBACKする手順を読取り専用検査と呼ばない。

以下は独自の読取り用SQL例であり、server実行済みではない。subscription名とschema-qualified sequence名を実環境の検証済み対象へ置き換える。publisher/subscriberの観測を1つのatomic snapshotにする例ではない。

```sql
-- subscriber: 期待一覧との照合に使う。行数0を成功と扱わない。
SELECT r.srrelid::regclass AS sequence_name,
       r.srsubstate, r.srsublsn
FROM pg_catalog.pg_subscription_rel AS r
JOIN pg_catalog.pg_subscription AS s ON s.oid = r.srsubid
JOIN pg_catalog.pg_class AS c ON c.oid = r.srrelid
WHERE s.subname = 'orders_sub' AND c.relkind = 'S'
ORDER BY sequence_name;

-- publisher: NULL、値とLSNの異なる意味を残して観測する。
SELECT * FROM pg_catalog.pg_get_sequence_data('app.order_number_seq'::regclass);

-- subscriber: 同期後に採番を進めず、値とis_calledを照合する。
SELECT last_value, is_called FROM app.order_number_seq;
```

## 計画switchoverの評価手順（独自の運用案）

以下は、旧publisherへ接続できる計画切替の検証項目である。sequence同期だけでDDL、table追随、writer排他、接続切替を完成させるものではない。

1. **範囲を固定する。** 両側のbinary版、publication、subscription、persistent sequenceの期待一覧、定義、利用権限、copy_data設定を記録する。tableを同期しているだけの構成を除外する
2. **旧writerを止める。** app、batch、直接SQL、nextvalだけを呼ぶ処理を含め、切替中の新しい採番と書込みを止める。未完了transactionの扱いと旧writerを再開させない仕組みを別途決める。tableのINSERT停止だけで採番が止まったとは見なさない
3. **tableと対象集合を確認する。** tableの最終変更がsubscriberへ適用済みであることを別の検証済み手順で確かめる。新規sequenceがある場合はREFRESH PUBLICATIONで登録を反映し、期待一覧との差分を解消する
4. **値を再同期して待つ。** REFRESH SEQUENCESを実行し、worker待ち・定義不一致・error再試行を観測する。受付成功の時点でroutingを変更しない。対象全件の状態と値を確認し、page_lsn一致だけには依存しない
5. **採番方針を点検して切り替える。** subscriberで独自に採番した履歴、手動setval、負のincrement、cycle、番号の外部持出し等がないか確認する。該当する場合は汎用のmax(id)+1式で安全とせず、利用範囲に合った衝突回避を別途設計する。旧writer停止が維持され、tableと採番状態の検証を通ってから新writerを開く

publisherが失われた非計画failoverでは、REFRESH SEQUENCESで現在値を取得できるとは限らない。直近の同期値やtableの最大値だけから、DB外へ既に渡された番号や未反映の採番まで復元したと主張しない。許容するデータ損失・番号再利用リスクと、復旧時の採番方法は障害対応計画の別項目である。

## 採用前の境界試験と未確認事項

以下は未実行の適合試験案である。正常な初期同期に加えて、差分だけで誤った成功判定を作らないことを検証する。

| case | 確認したい境界 |
|---|---|
| 初回copy_data=trueの後にpublisherでnextval | subscriberが自動追随する前提を置いていないこと |
| 同期後に1回だけnextvalしてLSN比較 | 同一WAL block内の値ずれをLSN一致で見落とさないこと |
| publicationへ新sequence、REFRESH SEQUENCESのみ | 未登録対象を「すべて同期済み」にしないこと |
| REFRESH PUBLICATIONのみで既知sequenceを再利用 | 既存値まで更新されたと誤認しないこと |
| copy_data=false、対象0件、temporary/unlogged | 値の未コピーや範囲外を成功扱いしないこと |
| definition mismatch、worker枠不足 | command受付と完了の間の待機・errorを識別できること |
| sequence SELECT権限不足、error counter増分 | NULLを一致、error回数を失敗sequence件数にしないこと |
| publisher 18 / subscriber 19 | tableの互換性からsequence同期まで可能と推測しないこと |
| 切替直前の旧writer再開 | 古い同期結果だけで新writerを許可しないこと |

PostgreSQL serverは起動しておらず、SQL例、全設定組合せ、crash時の内部状態、workerのtransaction境界・原子性、managed serviceでの利用可否、RC/GAでの仕様変化、停止時間・throughputは未検証。文書の契約を、すべてのsequenceが単一atomic snapshotで転送される保証へ拡張しない。検索evalは本資料の可発見性を検査し、DB適合試験を代替しない。

## 出典・取得日・provenance

UTC取得日はすべて2026-10-04。6件の固定repository資料は19beta4の文書解析に限定し、server実装コードを監査したものではない。公開19manualをnative webで照合し、18のRestrictionsとbeta4告知も実際に開いて版差と公開日を確認した。release_notesのTTL 30日に合わせ、再確認期限を2026-11-03とする。

[固定COPYRIGHT](https://github.com/postgres/postgres/blob/b73d13c32c834a2c8e1c60cb92f79530376cedf1/COPYRIGHT)はsoftwareとdocumentationを対象とし、[公式License](https://www.postgresql.org/about/licence/)と[19 Legal Notice](https://www.postgresql.org/docs/19/legalnotice.html)でPostgreSQL Licenseと範囲を照合した。告知本文固有の再利用licenseは未確認のためunknown。原文・実装コードの転載はせず、独自の日本語要約、SQL観測案、試験案だけを追加する。module昇格は行わない。

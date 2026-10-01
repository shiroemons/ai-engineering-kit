---
{
  "id": "postgres-output-plugin-allowlist-cdc-upgrade",
  "title": "PostgreSQL 18.6 / 17.11 の output_plugin_libraries: CDC と logical slot の移行確認",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18.6 / 17.11 (released 2026-08-13); configuration and slot monitoring details use PostgreSQL 18 manual retrieved 2026-10-01",
  "tags": [
    "research-domain:data",
    "postgres",
    "cdc",
    "logical-decoding",
    "logical-replication",
    "output_plugin_libraries",
    "allowlist",
    "pg_upgrade",
    "replication-slot",
    "migration"
  ],
  "sources": [
    {
      "id": "postgres-18-6-output-plugin-release-20261001",
      "url": "https://www.postgresql.org/docs/release/18.6/",
      "type": "release_notes"
    },
    {
      "id": "postgres-17-11-output-plugin-release-20261001",
      "url": "https://www.postgresql.org/docs/release/17.11/",
      "type": "release_notes"
    },
    {
      "id": "postgres-18-output-plugin-config-20261001",
      "url": "https://www.postgresql.org/docs/18/runtime-config-replication.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-18-logical-upgrade-20261001",
      "url": "https://www.postgresql.org/docs/18/logical-replication-upgrade.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-18-slot-state-20261001",
      "url": "https://www.postgresql.org/docs/18/view-pg-replication-slots.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-18-pgupgrade-preflight-20261001",
      "url": "https://www.postgresql.org/docs/18/pgupgrade.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-documentation-license-20261001",
      "url": "https://www.postgresql.org/docs/18/legalnotice.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# output_plugin_libraries 導入後の CDC と logical slot の移行確認

## 問いと今回の判断

minor update 後にアプリの通常 SQL は動くのに、第三者の logical decoding plugin を使う CDC (change data capture) が接続できない。plugin の再インストールや slot の作り直しに進む前に、どこを確認すべきか。

2026-08-13 公開の [18.6 release notes](https://www.postgresql.org/docs/release/18.6/) と [17.11 release notes](https://www.postgresql.org/docs/release/17.11/) には、CVE-2026-6471 の修正として output plugin の許可リスト追加がある。既存の schema migration・transaction retry とは別に、更新時の CDC 接続契約を扱う。

**判断:** バイナリ配置、plugin を信頼して許可する設定、slot の利用可能性、consumer の追随を別々に確認する。通常 SQL の疎通だけで更新完了にしない。これは公式契約から組み立てた運用案であり、本番実測の報告ではない。

## リリースで変わった契約

- 18.6 と17.11は `output_plugin_libraries` を新設し、既定で `pgoutput` と `test_decoding` だけを許可する。別の output plugin を必要とする構成では、更新後の設定にその plugin を明示する必要がある。
- 同じ major の18.X / 17.Xからの更新は dump/restore 不要だが、設定調整まで不要という意味ではない。
- 17以降の旧 cluster から logical slot を移す `pg_upgrade --check` では、新 cluster の許可リストに旧 slot の plugin が含まれていなければ失敗する。移行先設定を直してから再検査する。

根拠: [18.6](https://www.postgresql.org/docs/release/18.6/) / [17.11](https://www.postgresql.org/docs/release/17.11/) の Migration と最初の Changes 項目。この資料はその変更に限定し、release notes 内の他の修復・reindex 要件を置き換えない。

## PostgreSQL 18 の設定契約

[Replication configuration](https://www.postgresql.org/docs/18/runtime-config-replication.html#GUC-OUTPUT-PLUGIN-LIBRARIES) で確認した内容:

- 対象は `dynamic_library_path` にインストール済みで、logical output plugin として信頼する library の一覧。ファイルの配置だけでは許可にならない。
- 一覧外を使う logical decoding / replication request は拒否される。全 user が対象なので superuser に切り替えても、この制限は回避できない。
- comma-separated list の名前と client が指定する plugin 名は厳密一致する必要がある。大文字小文字や path の書き方の違いを同一視しない。
- 管理者が安全性を確認したうえで追加する。既存 slot の利用実績は、その library が全 REPLICATION user に安全だという証明ではない。
- 拒否時は `library ... may not be used as an output plugin` を含むエラーが手がかりになる。公式の復旧案は、安全な library を設定へ追加し configuration reload すること。これは更新済み server の設定反映の話であり、バイナリ更新全体が無停止になる保証ではない。

## 移行前の inventory

以下はこの資料用に作成した読み取り専用 SQL。PostgreSQL 18 の [pg_replication_slots](https://www.postgresql.org/docs/18/view-pg-replication-slots.html) の列を使う。古い major へそのまま持ち込まない。

```sql
SELECT slot_name, database, plugin, temporary, active,
       restart_lsn, confirmed_flush_lsn, wal_status,
       safe_wal_size, invalidation_reason
FROM pg_replication_slots
WHERE slot_type = 'logical'
ORDER BY database, slot_name;
```

- `plugin` は logical slot が使用する shared object の base name。physical slot では NULL。
- この view は現在存在する slot の一覧である。`temporary` な slot は永続化されず、error またはsession終了で消える。まだ作られていない slot や、拒否されて作成に成功しなかった利用経路は inventory に現れない。
- 運用案: 結果と CDC client の設定・起動 job・拒否ログを照合し、必要な名前と所有チームを整理する。slot の一覧を無審査で設定へ自動コピーしない。

## 実用的な rollout 手順

以下は独自の運用案。設定変更の承認や vendor 固有の手順は、運用環境側で確保する。

1. 更新前に logical slot と consumer、利用plugin、通常の追随位置を記録する。slot未作成の新規接続経路も client 設定で洗い出す。
2. 移行先に合う plugin binary の配置と信頼性を確認し、許可する名前を決める。既定名の利用も確認し、第三者名を追加する際に必要な既存名を落とさない。
3. 更新後に有効な許可リストと拒否ログを確認する。許可が欠けていれば必要最小限を追加して configuration reload 後に、その CDC 接続を再試行する。設定が存在しない旧版へ同じ変更を先行適用できるとは仮定しない。
4. 通常 SQL、CDC の再接続、テスト変更の consumer 到達をそれぞれ確認する。`active` は streaming 中という意味であり、それだけをconsumer処理成功の判定にしない。
5. `confirmed_flush_lsn` の進み方、consumer 側の処理位置、`wal_status`、ディスク空き容量を更新前の状態と比べる。確認のために slot を削除したり読み捨てたりせず、必要ならconsumer固有の復旧手順へ分岐する。

binary互換性と major upgrade の preflight は [pg_upgrade](https://www.postgresql.org/docs/18/pgupgrade.html) を参照する。`pg_upgrade` は minor update には不要。major upgrade では新 server の binary を使い、`--check` だけを先に実施する。shared object は新 server の binary に適合するものが必要で、許可名の追加はインストールの代わりにならない。

## major upgrade では追加条件を確認する

[Logical replication upgrade](https://www.postgresql.org/docs/18/logical-replication-upgrade.html#LOGICAL-REPLICATION-UPGRADE-PREPARE-PUBLISHER) では、旧 cluster が17.0以降のとき logical slot 移行をサポートし、それ以前の slot は移行対象にならない。

publisher 側には新 cluster の `wal_level = logical`、旧 slot 数以上の `max_replication_slots`、plugin の配置と許可、旧側からsubscriberへのtransaction / logical decoding messageの送信完了などの前提がある。allowlist が通っても、これらの前提が満たされたとは限らない。停止・再開順序は構成別の公式手順に従う。

取得時の同ページには「全 slot が usable」としながら `conflicting` の否定条件が整合しない文言がある。これをそのまま合否 SQL に転写しない。view の説明では `conflicting = true` は recovery conflict により無効化された状態である。本稿はこの不整合を解消したとは扱わず、対象版の `pg_upgrade --check` と公式文書の再確認を必須とする。

## 許可修正だけで復旧を断定しない

CDC が止まっている間の WAL 保持も調べる。[設定](https://www.postgresql.org/docs/18/runtime-config-replication.html#GUC-MAX-SLOT-WAL-KEEP-SIZE)では `max_slot_wal_keep_size = -1` が既定で、slot が WAL を無制限に保持し得る。有限値を設定しても、必要な WAL が消えればその slot で継続できないことがある。

[slot view](https://www.postgresql.org/docs/18/view-pg-replication-slots.html) の `wal_status = lost` は利用不能を示す。`unreserved` は即座に `lost` と同じ意味ではなく、保持を失って次のcheckpointで必要なファイルが削除され得る状態である。`safe_wal_size` の NULL は安全の保証ではなく、lost または保持上限が -1 の場合にも現れる。

運用案: 許可を直した後もslotがlost / invalidatedなら、再接続だけで回復する前提を捨て、consumer側の再同期・欠落確認へ切り分ける。slot削除やWAL保持上限の縮小を、permission errorの汎用的な修復として行わない。何秒まで待てるかは WAL 生成量・空き容量・consumer の追随速度で決め、固定の安全時間を作らない。

## 適用範囲・出典・未確認事項

- release notes の公開日は両方とも2026-08-13。その他の参照ページに個別の公開・更新日は表示されていない。UTC取得日はすべて2026-10-01。18系のversioned manualも更新され得るため、immutableな18.6ソースコードの調査とは扱わない。
- 17.11について確認したのは導入変更と既定値。本文の詳細な設定・列定義・監視例は18系manualに基づく。14〜16系へのbackport時点、managed databaseの設定公開・反映方式、第三者decoderの具体的互換版は未確認。
- pluginの安全性審査、実際のCDC再接続、server設定変更、SQLの実行、本番のWAL容量試験は実施していない。既存streamがreload中にいつ中断するか、vendor独自clientの再試行順序も本調査では確定しない。
- [PostgreSQL 18 Legal Notice](https://www.postgresql.org/docs/18/legalnotice.html) はsoftwareとdocumentationを許諾対象にしている。[公式Licenseページ](https://www.postgresql.org/about/licence/)で名称をPostgreSQL Licenseと確認した。原文・実装コードの転載はなく、上記SQLは列定義から作成した独自の観測例。本資料の配布ライセンスを別途宣言するものではない。
- release_notes のTTL 30日が最短のため、2026-10-31に再確認する。設定の文言、リリース適用版、slot view、major upgradeの不整合を読み直してから取得日を更新する。

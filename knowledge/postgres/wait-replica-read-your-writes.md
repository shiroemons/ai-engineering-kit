---
{
  "id": "postgres-wait-replica-read-your-writes",
  "title": "PostgreSQL 19 preview WAIT: replica readのcommit境界とsnapshot・timeline制約",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 19 development manual observed 2026-10-02; Beta 4 announced 2026-09-24; exact Beta 4 implementation parity unverified",
  "tags": [
    "research-domain:data",
    "replication",
    "read-your-writes",
    "wait",
    "lsn",
    "snapshot",
    "timeline",
    "preview"
  ],
  "sources": [
    {
      "id": "postgres-19-wait-command-20261002",
      "url": "https://www.postgresql.org/docs/19/sql-wait.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-19-wait-read-your-writes-20261002",
      "url": "https://www.postgresql.org/docs/19/warm-standby.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-19-wait-lsn-functions-20261002",
      "url": "https://www.postgresql.org/docs/19/functions-admin.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-19-wait-draft-release-20261002",
      "url": "https://www.postgresql.org/docs/19/release-19.html",
      "type": "release_notes"
    },
    {
      "id": "postgres-19-wait-beta4-release-20261002",
      "url": "https://www.postgresql.org/about/news/postgresql-19-beta-4-released-3386/",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# PostgreSQL 19 preview WAIT と replica read の境界

## 問いと適用版

primaryへの書き込み直後、非同期replicaで「まだ存在しない」と返さないために、何をどの接続で待つか。今回の対象は2026-10-02に取得したPostgreSQL 19 development manualである。既存のtransaction分離・CDC移行の記事にはない、replica読み取り前の待機契約を扱う。

[19 draft release notes](https://www.postgresql.org/docs/19/release-19.html)はWAITを新機能として載せるが、取得時点のrelease日は未確定の2026-??-??、編集基準表示は2026-09-14だった。[2026-09-24のBeta 4公開記事](https://www.postgresql.org/about/news/postgresql-19-beta-4-released-3386/)はWAITのdeadlock修正とisolation-level error説明の改善を列挙し、beta中は仕様が変わり得るとしている。

したがって以下はpreviewの採用検討用であり、19 stableの動作保証でも、Beta 4バイナリとの厳密な一致確認でもない。19 manual自体がunsupportedと表示されており、特定commitの実装分析はしていない。既存18系の記事へこの構文をそのまま適用しない。

## まずcommitを含むLSNを用意する

[Read-Your-Writes Consistency](https://www.postgresql.org/docs/19/warm-standby.html)の順序は、書き込みtransactionのcommit完了、primaryでLSN取得、standbyでそのLSNのreplay待機、成功後のreadである。待機先へ値を渡すのはapplicationまたはconnection poolerの役割となる。本資料の対象は同じWAL履歴を持つ物理replicationのstandbyであり、logical subscriberや別clusterへ数値だけ渡す設計には拡張しない。

LSN取得関数を名前の類似だけで入れ替えない。[System Administration Functions](https://www.postgresql.org/docs/19/functions-admin.html)では、pg_current_wal_insert_lsn()はWALの挿入末尾、pg_current_wal_lsn()は内部bufferからwriteされた位置、pg_current_wal_flush_lsn()はdurable storageへのflushが確認された位置を返す。insertとwriteとflushは別の進捗である。これらは観測関数で、読み取りにsuperuserを必要としない。

独自の設計判断: commit成功を確認してから、書き込み先と同じprimaryのpg_current_wal_insert_lsn()でtokenを取得する。commit前の値や、synchronous_commit=offで遅れ得るwrite/flushの値を、当該commitを含む境界と決めつけない。token取得までの間に別transactionのWALが増えても、その分を含む保守的な待機目標として扱い、transaction IDそのものとは呼ばない。これは分岐のない同じWAL履歴を前提とする。

## WAITで確認できること

以下は[WAIT command](https://www.postgresql.org/docs/19/sql-wait.html)の取得時点の契約を要約したもの。

- MODEの既定はstandby_replay。standby_writeはOSへのwrite、standby_flushはflushを待つため、どちらもread可能になるまでのreplayを保証しない。primary_flushはprimary用で、replica readの代用ではない
- TIMEOUT省略または0は無期限。0より大きく0.5ms以下も丸めで0になるので、短い待機のつもりで無期限化しない
- NO_THROWはtimeoutとnot in recoveryをstatusとして返す。successだけを到達確認とする。入力不正や開始前の条件違反など、その他のerrorまで握りつぶす設定ではない
- top-level commandとして実行し、function・procedure・DO blockへ埋め込まない。保持中のsnapshotがあると実行できない
- recovery中のstandby modeでは、未到達の目標を待つsessionが既にlockを持つと拒否される。READ COMMITTEDでも先行文のlockは残り得る。目標に到達済みならこのlock制約による待機拒否は生じないが、他の実行制約がなくなる意味ではない
- 比較はnumeric LSNだけでtimelineを識別しない。promotion時のnot in recoveryに加え、cascading standbyが別timelineへ追随した場合のsuccessも、期待した履歴の確認とは同一視しない

## 独自の運用設計: 待機からreadまでを一つの経路にする

1. 書き込み応答のcommit成否を確定する。成否不明ならWAIT成功を業務処理の成功判定に流用せず、業務IDによる照合など別の回復手順へ回す
2. primary由来のLSNに、applicationが管理するcluster識別とtopology世代を関連付ける。tokenがあるだけで別clusterやfailover後の履歴へ持ち越さない。世代管理は本資料の設計案であり、PostgreSQLが返す追加tokenの仕様ではない
3. 待機先replicaを選び、WAITから後続SELECTまでその接続に固定する。transaction poolingやread load balancerが途中で別replicaへ切り替えないことを確認する
4. WAITはtransaction外、または最初のsnapshot取得・lock取得文より前へ置く。ORMの存在確認SELECT、cursor、exported snapshotを挟む処理を別途検査する。待ってから必要なread transactionを組み立てる
5. 正の整数msで待機予算を与える。NO_THROWを使う経路はstatus別に分岐し、timeoutでは「存在しない」と返さず、許容する場合だけprimaryへのread fallbackを試す。fallbackでもtopologyの整合確認を省かない
6. not in recoveryや接続中断では接続役割を再判定する。無条件に同じtokenを何度も再投入する処理にしない。例外でtransactionが失敗状態ならrollbackしてから別の処理を行う

待機成功後のSELECTで得るのは、そのreadのsnapshotで見える状態である。後続の別transactionによる更新まで止める仕組みとして設計しない。LSN tokenがユーザー入力から渡される場合も、到達不能な遠い値で接続を占有しないよう、受理条件と待機上限をapplication側に設ける。

## 観測と既存機能との境界

[Recovery Information Functions](https://www.postgresql.org/docs/19/functions-admin.html)のpg_last_wal_receive_lsn()はstreamingで受信・同期済みの位置、pg_last_wal_replay_lsn()はreplay位置を返す。streaming未開始等ではreceive値がNULLになり、recovery完了後はこれらの値がその時点で固定される。観測値が増えないことだけから「待機対象と同じ履歴で追随中」とは推定しない。

[同期replication](https://www.postgresql.org/docs/19/warm-standby.html)ではremote_applyが、対象の同期standbyからreplay報告を受けるまでcommitを待たせる。一方、このread-your-writes設計はread経路で必要な進捗を確認する。どのstandbyが同期確認の対象かを無視して、remote_applyなら全read replicaへ即座に読ませてよいとは扱わない。

独自の監視案: WAITのstatus別件数、待機時間、fallback率、接続先、topology世代を記録する。receive/replayのLSN差と組み合わせ、timeout、役割変更、入力・前提条件errorを区別する。LSN差をそのまま残り秒数に換算する監視は作らない。

## 導入前の試験案と未確認事項

本調査ではPostgreSQL serverやreplicaを起動せず、以下は未実行の試験案である。検索evalは文書を取得できることだけを検査し、databaseの適合試験ではない。

- 正常系: commit後tokenを取得し、遅延replicaで待機後に対象行が読めるか。token取得前commitとread接続固定をログで確認する
- 境界系: 未到達token、TIMEOUT 1ms、NO_THROWの全status、malformed LSNで、statusと例外を混同しないか
- 接続系: ORMが先行SELECTを発行する場合、REPEATABLE READでsnapshot取得済みの場合、poolがread先を変える場合を別々に試す
- 障害系: WAL受信停止、recovery conflict、待機中promotion、上流promotion後のcascading standbyを試す。numeric LSN一致だけで履歴検証を通さないか確認する

個別beta・RC・GAでのoption追加時点、managed serviceの対応、driverの構文対応、SQLSTATEの具体値、timeline検証protocol、性能・容量の実測は未確認。採用するbinaryとpoolerで再検査するまで本番互換性を確定しない。

## 出典・日付・ライセンス

UTC取得日はすべて2026-10-02。manual各ページには個別更新日がなく、release notesの編集基準日とBeta 4告知の公開日を区別して記録した。release_notesのTTL 30日に合わせ2026-11-01を再確認期限とし、その際は開発版表示と本文全体を読み直す。

[19 Legal Notice](https://www.postgresql.org/docs/19/legalnotice.html)はsoftwareとdocumentationを対象とし、[公式Licenseページ](https://www.postgresql.org/about/licence/)で名称をPostgreSQL Licenseと確認した。ニュース本文固有の再利用ライセンスは未確認のため、そのcatalogはunknown。原文・実装コードを転載せず独自に要約し、コードのmodule昇格も行わない。sourceのライセンス記録は本リポジトリ自体の配布条件を定めるものではない。

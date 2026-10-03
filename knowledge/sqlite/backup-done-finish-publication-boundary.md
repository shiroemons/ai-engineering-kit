---
{
  "id": "sqlite-backup-done-finish-publication-boundary",
  "title": "SQLite live backup: SQLITE_DONE・finish成功・復元可能な成果物の公開境界",
  "kind": "knowledge",
  "technology": "sqlite",
  "version": "Online Backup API / VACUUM INTO unversioned official docs retrieved 2026-10-03 UTC; VACUUM INTO introduced in 3.27.0 (2019-02-07); limited runtime observations on local 3.46.1 build with source-id suffix alt1, not 3.53.4 validation",
  "tags": [
    "research-domain:data",
    "backup",
    "sqlite3_backup_step",
    "sqlite3_backup_finish",
    "SQLITE_DONE",
    "snapshot",
    "VACUUM INTO",
    "restore",
    "durability",
    "progress",
    "cancellation"
  ],
  "sources": [
    {
      "id": "sqlite-backup-c-api-20261003",
      "url": "https://www.sqlite.org/c3ref/backup_finish.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-backup-usage-20261003",
      "url": "https://www.sqlite.org/backup.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-backup-vacuum-into-20261003",
      "url": "https://www.sqlite.org/lang_vacuum.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-backup-corruption-20261003",
      "url": "https://www.sqlite.org/howtocorrupt.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-backup-restore-checks-20261003",
      "url": "https://www.sqlite.org/pragma.html",
      "type": "official_docs"
    },
    {
      "id": "sqlite-backup-vacuum-introduction-20261003",
      "url": "https://www.sqlite.org/releaselog/3_27_0.html",
      "type": "release_notes"
    },
    {
      "id": "sqlite-backup-documentation-license-20261003",
      "url": "https://www.sqlite.org/copyright.html",
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

# SQLite live backup の完了と公開を分ける

## 問いと判断

バックアップ処理の終了関数が成功したとき、出力ファイルを「復元できる最新世代」として公開してよいか。

**判断:** Online Backup APIでは `sqlite3_backup_step()` が `SQLITE_DONE` に到達したことを保存し、`sqlite3_backup_finish()` の結果とは別に判定する。途中取消しでもfinishが成功し得る。さらに、完成したdatabase、保存先での耐久性、検査済み世代の公開は別の条件として管理する。

本稿は未収録だったlive backupの成功判定を扱う。[WAL checkpoint](wal-checkpoint-progress-reset-safety.md)の完了条件や[busy_timeout](wal-busy-timeout.md)の接続初期化を言い換えるものではない。checkpointの成功から任意のファイルcopyの安全性は導けない。

## 適用版と確認の強さ

2026-10-03 UTCに公式の非versioned文書を確認した。[3.27.0 release notes](https://www.sqlite.org/releaselog/3_27_0.html)による `VACUUM INTO` の導入日は2019-02-07。ただし、導入版に現在の同期処理などすべての説明が既に存在したとは主張しない。

C APIの一部は、ローカルの `libsqlite3.so.0` が報告する3.46.1で限定実測した。source-idは `2024-08-13 09:16:08 c9c2ab54ba1f5f46360f1b4f35d849cd3f080e6fc2b6c60e91b16c63f69aalt1`。末尾がalt1のbuildであり、upstream無改変版や全vendorの挙動と同一とは扱わない。3.53.4のruntime、WAL下の競合、電源断を検証した結果ではない。

## 公式契約: finishは完成証明ではない

根拠は[C API referenceのstep / finish節](https://www.sqlite.org/c3ref/backup_finish.html)。

- `sqlite3_backup_init()` が成功したら、対応するfinishは必ず一度だけ呼ぶ。初期化時はsourceとdestinationを別接続にし、destinationには既存のread/write transactionを残さない。初期化失敗時のerror情報はdestination接続に格納される。
- stepの `SQLITE_OK` は途中のpage転送成功で、まだ未転送pageがある。全pageの転送が完了した返り値が `SQLITE_DONE`。負のpage数を渡すと残りすべてを一回で処理するが、失敗やlock競合まで消えるわけではない。
- DONE前にfinishするとdestination側の実行中write transactionはrollbackされる。それまでのstepにerrorがなければ、転送が未完了でもfinishは `SQLITE_OK` を返し得る。
- `SQLITE_IOERR_XXX`、`SQLITE_NOMEM`、`SQLITE_READONLY` は当該backupでの致命的失敗として扱い、同じstepを無限再送しない。`SQLITE_BUSY` / `SQLITE_LOCKED` は後で再試行できるが、原因の解消やジョブ全体の期限は別途必要。
- READONLYはファイル権限だけを意味しない。destinationがWAL、またはin-memoryで、sourceとのpage sizeが不一致でも起こり得る。権限変更で直そうとする前に、source/destinationの設定を確認する。

したがって本稿の成功判定案は「DONEを観測した」かつ「finishが成功した」。取消し、期限切れ、stepのfatal errorは独立した失敗理由として保持し、後続のcleanup成功で上書きしない。ファイルが存在することや、以前の世代が開けることも、この試行の完成証明にならない。

### ローカル観測: 同じfinish=OKでも内容が違う

独自の小さな再現で、1024-byte pageのsourceに200行のpayloadを入れ、既存destinationには `previous_backup` tableだけを作った。作成・転送・結果照会をすべて上記の同じC libraryで実施した。SQLや実装codeを出典からコピーしていない。

1. 1 pageだけstepした結果はOK、全426 page中425 pageが未転送。そこでfinishはOKだったが、destinationには `previous_backup` が残り、source tableへの置換は確定していなかった。
2. 別のdestinationへ全pageを転送するとstepはDONE、finishはOKとなり、source tableへの置換を確認した。
3. 別source接続でexclusive transactionを保持するとstepはBUSY。直後に中止したfinishもBUSYだった。
4. 初回step後、別接続で201行目をcommitしてから転送を完了すると、出力は201行となった。ジョブ呼出し時の200行に固定されていなかった。

3は現行C API文書の「BUSY / LOCKEDはfinishの戻り値に影響しない」という説明と一致しない。文書・旧版・ローカルbuild差の原因は未確認で、仕様変更時期や修正版は推測しない。BUSY後のfinishが必ずOKという前提を実装へ持ち込まず、実配布libraryでerror pathを検査する。上記の観測はDONEを別管理する必要性を補強するが、現行全版の適合テストではない。

## 公式契約: 再開始と進捗は単調ではない

[利用guideのFile and Database Connection Locking / progress節](https://www.sqlite.org/backup.html)と[C API](https://www.sqlite.org/c3ref/backup_finish.html)は、分割stepの間にsourceが更新される場合を扱う。

sourceのread lockはstepの読取り中だけであり、処理全体を通じてwriterを止める設計ではない。別接続・別processの更新が入るとbackupが自動再開始される。file-based sourceを同じsource接続で更新する経路は追随可能とguideにあるが、これをin-memoryも含む全構成へ一般化しない。guideは再開始が多ければ完了しない可能性を明示している。

`backup_remaining()` / `backup_pagecount()` は直前stepの観測値であり、呼出し時点のdatabaseを新たに測るものではない。sourceのサイズ変更や再開始をまたいでremainingの単調減少を期待しない。stepとこれらの進捗APIを別threadから同時呼出しして安全な値が得られるとも仮定しない。

guide冒頭のsnapshot説明だけを「ジョブ開始時刻に固定される」と読まず、途中更新と再開始の節も合わせて読む。完了したcopyは一貫したsnapshotとなるが、運用ログの開始timestampを厳密なcutoffとして表示する契約はここでは得られない。特定の業務時点を要求する場合は、snapshotの保持方式とwriterへの影響を別途設計する。

## 公式契約: destination接続を専有する

[C APIのConcurrent Usage of Database Handles節](https://www.sqlite.org/c3ref/backup_finish.html)では、成功したinitから対応するfinishまで、destination接続を他のAPIへ渡してはならない。誤用してもerrorが検出されるとは限らず、動作不良やmutex deadlockになり得る。

進捗callbackからdestinationをSELECTしたり、poolへ返してhealth checkを走らせたりしない。必要なpage size・journal modeなどの情報はinit前に取り、検査はfinish後に行う。shared cacheを使う場合は同じcacheへの他接続からのアクセスも避ける必要がある。backup対象はsource/destinationの指定schemaであり、複数attached database全体が一度に同一時点へ揃うと仮定しない。

## VACUUM INTOを選ぶ場合の完成条件

[VACUUM reference](https://www.sqlite.org/lang_vacuum.html)によると、`VACUUM INTO` は元databaseを変えず、一貫したlogical snapshotを別fileへ作る。空き領域や削除済み内容の痕跡を除いた小さいcopyに向く。Backup APIは比較上CPU消費が少なく、分割実行できる。backupの目的と、sourceへ許す負荷・中断方式から選ぶ。

- 出力先は未存在または空fileである必要があり、既存の非空backupへ同じ名前で上書きするcommandではない。新世代ごとに別の出力先を用意する。
- 実行中の電源断や予期しない終了では、出力fileが不完全・破損状態になり得る。fileの出現を成功通知にしない。
- 取得時の文書は、元databaseの `synchronous` が `NORMAL` または `FULL` のとき、書込み後に出力fileをfsync / FileFlushBuffersで同期すると明記している。「VACUUM INTOは同期しない」という一律の説明は採用しない。OS・filesystem・hardwareが正しく動くことが前提で、古い版すべてへこの詳細を逆適用しない。
- 同じ接続にopen transactionや未終了statementがあると失敗し得る。元databaseを変更しないことは、任意のtransaction内で実行できるという意味ではない。
- VACUUMは明示的な `INTEGER PRIMARY KEY` のないtableのROWIDを変え得る。再構築copyに対して暗黙ROWIDやfileのbyte列の完全一致を業務同一性の条件にしない。

出力fileの同期は、その後の遠隔upload、checksum検証、世代一覧の更新まで保証しない。独自の保存先へ移動した後のdurabilityはその保存先の契約で確認する。

## 設計案: 生成・検査・公開を一つの成功flagにしない

以下は上の根拠からの運用提案であり、SQLiteが提供するbackup schedulerや公開protocolではない。

1. **生成先を隔離する。** 世代IDを持つ候補へ出力し、直近の検証済み世代をその場で置換しない。使用libraryの版、source-id、対象schema、開始/終了時刻、方式、設定、step/finishの両結果を記録する。
2. **完了を判定する。** APIはDONEとfinish結果、VACUUM INTOはstatementの正常完了を確認する。分割APIには処理全体のdeadlineと取消し経路を設ける。page数・待機時間の固定値を公式推奨とせず、sourceの負荷と実測から決める。
3. **接続を終了して復元検査する。** コピー先への書込みが終わった後、隔離した復元環境でfileを開き、schemaや必要な拡張、業務上の整合性を確認する。[PRAGMA契約](https://www.sqlite.org/pragma.html#pragma_integrity_check)ではintegrity_checkが問題なしなら1行の `ok` を返すが、FOREIGN KEY違反は検出しない。[foreign_key_check](https://www.sqlite.org/pragma.html#pragma_foreign_key_check)も必要に応じて別実行する。形式上の正常と、要求する業務データがあることを混同しない。
4. **保存先で確認してから公開する。** 転送先の実体・checksum・復元手順を確認した世代だけを利用対象にする。失敗した候補のcleanup成功で前の良好世代を消さない。cleanupやretention自体の権限と要件は別管理する。
5. **復元時にlive fileを差し替えない。** [corruption guideの1.2 / 2.5](https://www.sqlite.org/howtocorrupt.html)は、transaction中の生copyや開いたdatabaseのunlink/renameを危険としている。復元先は接続を止めるか隔離し、旧journalと新databaseを混ぜない。APIで完成したbackupの公開と、稼働中sourceのfile置換を同じ操作として扱わない。

## 受入確認と残る限界

本番導入では「部分転送後の取消し」「BUSYからの期限切れ」「fatal error」「別writerによる再開始」「destinationへの誤アクセスをしない設計」「非空VACUUM出力先」「復元後の業務整合性」を試験対象にする。fileが残った、進捗が一度100%になった、終了処理が成功した、だけでは合格にしない。

今回の実測は前述4ケースのみ。実sourceの書込み負荷での完了時間、WAL / shared cache / in-memoryの全組合せ、bindingの例外変換、最新patchの実装、電源断、filesystem同期、遠隔保存先の保証、複数database間の原子的snapshotは未検証。検索evalは本稿の取得可能性を確認するもので、これらのruntime検証を代替しない。

## 出典・取得日・ライセンス

全7 sourceを2026-10-03 UTCに実ページで確認した。表示更新日はBackup guideが2025-11-13 07:12:58Z、VACUUMが2025-07-12 15:11:36Z、corruption guideが2026-04-13 10:54:51Z、PRAGMAが2026-06-04 01:35:31Z、copyrightが2026-01-12 12:33:24Z。C APIページの独立した公開/更新日は表示から確認できない。3.27.0の公開日は2019-02-07で、取得日を新規release日とは扱わない。

[公式copyright](https://www.sqlite.org/copyright.html)で配布codeとdocumentationのpublic domain表記を確認した。build script等の例外へ一般化しない。本稿は独自の日本語要約と明示した設計案・限定実測であり、OSS codeの転載・module昇格はない。release_notesのTTL 30日に合わせ、再確認期限は2026-11-02。日付だけを延長せず、API文書と実配布版の差も読み直す。

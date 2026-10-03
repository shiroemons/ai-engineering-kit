---
{
  "id": "postgres-returning-old-concurrent-trigger-boundary",
  "title": "PostgreSQL 18.6: RETURNING OLD の並行更新修正と監査差分の境界",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "OLD/NEW in RETURNING introduced in PostgreSQL 18 (2025-09-25); concurrent-update/BEFORE-trigger fix verified in 18.6 (2026-08-13); 18 manual reverified 2026-10-03 UTC",
  "tags": [
    "research-domain:data",
    "postgres",
    "RETURNING",
    "OLD",
    "NEW",
    "READ COMMITTED",
    "BEFORE UPDATE",
    "trigger",
    "audit",
    "concurrent-update",
    "commit"
  ],
  "sources": [
    {
      "id": "postgres-returning-old-fix-186-20261003",
      "url": "https://www.postgresql.org/docs/18/release-18-6.html",
      "type": "release_notes"
    },
    {
      "id": "postgres-returning-old-introduction-18-20261003",
      "url": "https://www.postgresql.org/docs/18/release-18.html",
      "type": "release_notes"
    },
    {
      "id": "postgres-returning-row-images-18-20261003",
      "url": "https://www.postgresql.org/docs/18/dml-returning.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-returning-update-output-18-20261003",
      "url": "https://www.postgresql.org/docs/18/sql-update.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-returning-read-committed-18-20261003",
      "url": "https://www.postgresql.org/docs/18/transaction-iso.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-returning-before-trigger-18-20261003",
      "url": "https://www.postgresql.org/docs/18/trigger-definition.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-returning-transaction-commit-18-20261003",
      "url": "https://www.postgresql.org/docs/18/tutorial-transactions.html",
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

# RETURNING OLD の「更新前」はどの時点か

## 問いと対象範囲

PostgreSQL の更新結果から監査差分を作るとき、保存された新値が正しければ `RETURNING OLD` も正しいと判断してよいか。18.6の修正は、この判断が成立しない並行実行条件を示す。

[PostgreSQL 18](https://www.postgresql.org/docs/18/release-18.html)は2025-09-25公開で、DMLの `RETURNING` に明示的な `OLD` / `NEW` を導入した。[18.6](https://www.postgresql.org/docs/18/release-18-6.html)は2026-08-13公開で、ここで扱う不具合の修正を確認した版である。18.5は公開されていない。10月の新機能や未確認の将来版として扱わない。

既存の[分離レベルと再試行](transaction-isolation-retry.md)はtransaction全体の契約を扱う。本稿は、更新APIが返す行の旧値・新値を監査や差分表示へ使う境界に絞る。GIN統計、REPACK、論理レプリケーションの設定は対象外。

## 確認できた修正と通常の契約

### 18.6 が直したのは並行更新後の旧値の取り違え

18.6リリースノートによると、`READ COMMITTED` で対象行が並行更新され、テーブルに `BEFORE UPDATE` triggerがある場合、`RETURNING` の `OLD` が古い行の値を返す不具合があった。trigger自身と更新後の行は正しい値を参照していた。修正後に期待する旧値は、競合相手の更新を反映した行の値である。

したがって、テーブルの最終値だけの検証では返却旧値の誤りを検出できない。この項目から、全UPDATEが誤った値を書き込んだ、全triggerのOLDが壊れていた、あるいは既存の監査履歴が更新で修復されるとは結論できない。

### OLD はアプリが先に SELECT した値とは限らない

[18系の分離レベル契約](https://www.postgresql.org/docs/18/transaction-iso.html#XACT-READ-COMMITTED)では、`READ COMMITTED` のUPDATEは開始時点で対象行を探すが、その行が他transactionに更新されていれば完了を待つ。相手がcommitした場合、更新済み行に対して `WHERE` を再評価し、まだ条件を満たせばその行を更新する。

つまり「事前SELECTの値」「UPDATE開始時点のsnapshotで見えた値」「自分が実際に置き換えた行の旧値」は区別する。返却されたOLDと事前画面の値が異なるだけで、修正後のサーバーが誤っているとは判断しない。相手がrollbackすれば元の行を使う条件になる。`REPEATABLE READ` では競合相手の実更新のcommitによりserialization failureとなる場合があり、同じ成功結果を期待する試験にはしない。

### 返却値・更新件数・triggerは別の観測項目

[RETURNINGの説明](https://www.postgresql.org/docs/18/dml-returning.html)と[UPDATE reference](https://www.postgresql.org/docs/18/sql-update.html)による。

- UPDATEの無修飾列名や `RETURNING *` は新値を返す。旧値は `OLD` または指定した別名で明示する。別名指定後は元のOLD/NEW名が隠れる
- triggerが行を加工する場合、RETURNINGに見える値にはその加工が反映される。アプリが送った値を、そのままDBの新値として扱わない
- RETURNINGは実際に更新された行が対象。更新件数は同値更新も含むため、返却一行だけで値の変化があったとは言えない
- `UPDATE 0` はエラーではない。条件不一致に加え、`BEFORE UPDATE` による抑止でも件数は減り得る

[triggerの契約](https://www.postgresql.org/docs/18/trigger-definition.html)では、row-level BEFORE triggerは変更したNEWを返せるほか、NULLを返してその行の操作を中止できる。statement-level triggerは更新ゼロ行でも発火し得る。よって「監査用triggerが呼ばれた」「対象行が更新された」「RETURNINGに一行あった」を同じ成功指標にしない。

## 小さな時系列で差分を確かめる（独自の試験設計）

以下は公式契約から導いた期待値であり、DBを動かした実測ではない。単一の通常テーブル、値を加工せずNEWを返すrow-level BEFORE UPDATE trigger、同じ主キーの一行を前提とする。

1. 初期値を7とする。接続Aが11へ更新し、commit前で止める
2. 接続BがREAD COMMITTEDで同じ行へ「現在値に3を加える」UPDATEを開始し、Aの完了待ちになったことを確認する
3. Aをcommitする。Bの条件は主キーだけなので引き続き一致する
4. Bの期待値は OLD=11、NEW=14、差分=3。開始時の7を旧値にすると差分は7となり、Aの変更までBの変更に含めてしまう
5. Bの返却旧値、新値、triggerが観測した値、保存値を別々に比較する。新値14だけをassertする試験では不足する

同じ順序でAをrollbackする場合、Bの期待値は OLD=7、NEW=10。どちらも「Bが開始する前にAが終わっていた」順序では待機経路の検証にならない。固定秒数のsleepだけに依存せず、テストハーネスで更新待機を確認してからAを解放する。

## 適用・監査の判断手順（独自の運用案）

1. アプリ、ORMが生成するSQL、DB関数についてOLDを返すUPDATEを探し、対象テーブルのBEFORE UPDATE triggerと実際の分離レベルを組み合わせて調べる。構文に対応する18系という情報だけで、修正済みと判断しない
2. 修正を確認した18.6への更新、または利用中配布元の同等修正の証跡を確認する。配布元によるbackportの有無は本稿では未確認。triggerを無効化する暫定策を標準手順にはしない
3. 保存値とRETURNINGを独立に検証する。既に外部へ出した監査差分に疑いがある場合は、当時の別の記録がある範囲で照合する。現在の行だけから過去の正確なOLDを復元できるとは約束しない
4. 明示的なtransaction blockでは返却行を取得しても後続の `ROLLBACK` で更新が取り消される。[Transactions](https://www.postgresql.org/docs/18/tutorial-transactions.html)の契約を踏まえ、監査の外部確定はCOMMIT成功と区別する。同じDBの監査行を更新と同一transactionに保存する案は、外部送信の保証とは分けて設計する

BEGIN省略時は通常各文がimplicit transactionとなるが、client libraryが自動的にtransactionを開始する場合もある。「RETURNINGの受信」だけで利用中ドライバーのcommit完了を判定せず、そのAPI契約も確認する。本稿は通知のexactly-onceや全変更のCDC取得を保証するものではない。

## 受け入れ試験で残す境界（未実行）

- 並行更新のcommit / rollback: 上の7→11→14の順序を固定し、OLDとNEWを個別に確認する
- WHERE再評価: version列等の条件が競合後に不一致となるケースで、返却ゼロ行を「行が存在しない」と即断しない
- NEWの加工: BEFORE triggerが値を正規化するケースで、入力値でなく加工後の新値を比較する
- NULLによる抑止: row-level BEFORE triggerが操作を中止するケースと、同値更新で一行返るケースを分ける
- 返却後のROLLBACK: 明示的transactionで結果を得てからrollbackし、確定監査として外部へ出さない制御を確認する
- REPEATABLE READ: READ COMMITTEDの成功期待値を使い回さず、競合時のtransaction全体の再試行を検証する

## 出典・ライセンス・限界

UTC取得日は2026-10-03。リリース日のある二資料以外の18系マニュアルは個別公開日が表示されていない。[PostgreSQL License](https://www.postgresql.org/about/licence/)がソフトウェアと文書を対象とすることも同日に確認した。既存sourceは変更せず、本稿から参照する7件のcatalogを追加。本文は独自の日本語要約と試験設計で、コード転載なし。release_notesの30日TTLに合わせた再確認期限は2026-11-02。

実装コミットの解析、各旧パッチでの発生範囲、DB実機の並行試験、ORM/driverごとの返却・commit順序、過去監査の修復は未実施。partition間の行移動、viewのINSTEAD OF trigger、MERGE、UPSERT、複雑な連鎖triggerへ上の単純な期待値を一般化しない。検索evalは本資料を見つけられることを検査し、DBの正しさの試験を代替しない。

---
{
  "id": "postgres-gin-reltuples-post-upgrade-analyze-boundary",
  "title": "PostgreSQL 18.6: 並列 GIN の reltuples 異常と更新後 ANALYZE の修復境界",
  "kind": "knowledge",
  "technology": "postgres",
  "version": "PostgreSQL 18.6 release 2026-08-13; parallel GIN introduced in 18 (2025-09-25); 18 manual reverified 2026-10-03 UTC",
  "tags": [
    "research-domain:data",
    "postgres",
    "GIN",
    "parallel-build",
    "reltuples",
    "ANALYZE",
    "autovacuum",
    "upgrade",
    "statistics",
    "NaN",
    "Infinity",
    "binary",
    "wraparound"
  ],
  "sources": [
    {
      "id": "postgres-gin-reltuples-release-186-20261003",
      "url": "https://www.postgresql.org/docs/18/release-18-6.html",
      "type": "release_notes"
    },
    {
      "id": "postgres-gin-parallel-release-18-20261003",
      "url": "https://www.postgresql.org/docs/18/release-18.html",
      "type": "release_notes"
    },
    {
      "id": "postgres-gin-reltuples-catalog-18-20261003",
      "url": "https://www.postgresql.org/docs/18/catalog-pg-class.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-gin-autovacuum-threshold-18-20261003",
      "url": "https://www.postgresql.org/docs/18/routine-vacuuming.html",
      "type": "official_docs"
    },
    {
      "id": "postgres-gin-repair-analyze-18-20261003",
      "url": "https://www.postgresql.org/docs/18/sql-analyze.html",
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

# PostgreSQL 18.6 の GIN 修正: バイナリ 更新と保存済み統計の修復

## 調査した問いと適用版

並列 GIN インデックス を作った PostgreSQL 18 のサーバを18.6へ更新すれば、autovacuum / autoanalyze の判定も自動で正常に戻るか。本稿は**更新で新しい異常を防ぐこと**と**既に残った `pg_class.reltuples` を直すこと**を分ける。

[18.6 リリースノート](https://www.postgresql.org/docs/18/release-18-6.html)の公開日は2026-08-13。2026-10-03に リリース一覧 と18系マニュアルを確認した時点で掲載されていた18系パッチであり、10月の新規リリースとは主張しない。18.5はリリース前の回帰不具合発見により公開されていない。[18 リリースノート](https://www.postgresql.org/docs/18/release-18.html)は2025-09-25公開で、GINの並列構築導入を明記する。

既存の[インデックス選択とEXPLAIN](index-explain.md)や[full-text searchのGIN選択](../search/postgres-fulltext-tsvector-gin-rank.md)は用途・実行計画が対象である。ここではパッチ適用後の保守判定の修復を扱い、GIN一般論、[REPACK](repack-concurrent-lock-snapshot-boundary.md)、他拡張のREINDEX要件を混ぜない。

## 確認できた契約

### 18.6 が修正する異常と残る作業

18.6 リリースノートの並列 GIN項目によると、ワーカーが処理行数として未初期化値を返し、**インデックス自身ではなく元テーブルの** `reltuples` が不正になる場合があった。InfinityやNaNも含む。これにより後続のautovacuum / autoanalyzeが処理対象と判定しなくなる場合があり、その場合は自然回復しない。手動 `ANALYZE`、または別インデックスの作成により値を正す必要がある。

同リリースの移行節はGIN インデックスを持つテーブルの `reltuples` 点検を案内する。18.Xからの更新にダンプ/リストアは不要だが、それは統計点検・修復が不要という意味ではない。すべてのGIN インデックスが壊れたという主張でも、すべてのテーブルのREINDEXを要求する項目でもない。バイナリ更新後も既存の異常値が残っていないか確認する。

### reltuples は厳密な行数ではない

[pg_class](https://www.postgresql.org/docs/18/catalog-pg-class.html)の `reltuples` はプランナー向けの推定生存行数で、`VACUUM` / `ANALYZE` / 一部DDLが更新する。未VACUUM・未ANALYZEのテーブルでは `-1` が行数不明を表す。したがって `-1` だけで今回の不具合と決め付けず、少量の推定誤差をデータ破損と扱わない。

異常の対象は統計値である。この資料だけからヒープの行やGIN内のエントリが失われたとは結論できない。有限値でも不自然な値になり得るため、NaN/Infinityだけの検査を完全な検出器としない。

### 自動処理の閾値と別経路

[Routine Vacuuming §24.1.6](https://www.postgresql.org/docs/18/routine-vacuuming.html#AUTOVACUUM)は通常のVACUUM閾値と挿入閾値が `reltuples` を使い、ANALYZEも行数に応じた閾値で判定すると説明する。

ただしトランザクションIDの経過数によるwraparound防止VACUUMは別の起動経路を持つ。**通常の閾値判定を妨げ得る**ことと、すべてのautovacuumが永久に停止することは同義ではない。wraparound処理を待ってよいという推奨でもない。

また、パーティション親はautovacuumの処理対象にならず、一時テーブルも自動処理できない。パーティションやセッション固有のテーブルでautoanalyzeが来ない原因を、今回の不具合だけに帰属させない。

### ANALYZE の完了を確認する条件

[ANALYZE reference](https://www.postgresql.org/docs/18/sql-analyze.html)による。

- 通常はテーブルの `MAINTAIN` 権限が必要。データベース所有者には追加の範囲があるが、権限不足のテーブルはスキップされる。コマンド終了だけで全対象が修復されたと判定しない
- `SKIP_LOCKED` は最初のリレーションのロック待ちを避けるため対象をスキップする。インデックスやパーティションの標本取得等では待つ場合もあり、完全な待機なし保証ではない
- テーブル指定は通常そのパーティション/継承子も処理し、`ONLY` は範囲を限定する。親のロック競合で `SKIP_LOCKED` が全パーティションをスキップする場合がある
- 大表ではサンプリングなので統計は近似で、再実行でもプランナーの推定値は多少変わり得る。

## 修復の判断手順（独自の運用案）

1. 実際のサーバーのパッチ版とGIN構築履歴を確認し、修正済みバイナリへの更新と保存済み統計の点検を別項目にする。古い未修正バイナリで統計だけ直して完了にしない
2. 各データベースでGINを持つテーブルを列挙する。`pg_index.indrelid` からテーブル、`indexrelid` からインデックス、インデックスの `pg_class.relam` から `pg_am.amname` をたどり、`gin` のテーブル側 `reltuples` を読む。複数GINを持つテーブルは重複排除し、スキーマ名も記録する
3. NaN/Infinity、非現実的な有限値、通常の `-1` を区別する。既知のデータ量や最近の変更量を踏まえて対象を決め、重い厳密な `COUNT(*)` を全テーブルへ無条件に追加しない
4. 承認された保守作業時間・権限で、異常が疑われる対象を明示した `ANALYZE` を実施する。単なる統計修復のためだけに新しいインデックスを増やすことは通常の第一選択にしない
5. スキップや警告を確認し、**修復前後のreltuples**を比較する。コマンド成功、推定値の妥当性、対象漏れの有無を別々に記録する。スキップされたテーブルは修復済みとせず再確認対象にする
6. その後の通常保守作業の実行を、テーブル別の設定と変更量を踏まえて確認する。直後にautoanalyzeが走らないだけで失敗とはしない。権限・ロック競合・autovacuum設定・パーティション親等の別原因も切り分ける

この手順は本番データベースへ実行した記録ではなく、一次資料から導いた運用手順案である。カタログへの直接UPDATEで数値を上書きする運用は提案しない。

## 避ける判断と受け入れ試験案（未実行）

- **パッチだけで完了**: バイナリ更新後にも既存の異常な `reltuples` が検出される試験データを用意し、点検・修復・再点検が別工程になっていることを確認する
- **すべてREINDEX**: テーブル統計の修復をGIN インデックス内容の破損修復と混同しない。18.6の別項目であるbtree_gist / ltreeの再構築条件は本稿の対象外
- **NaNだけ見れば十分**: Infinity、不自然な有限値、正常な `-1`、サンプリングによる差を区別する検査を用意する
- **ANALYZE終了なら全件完了**: 権限不足、`SKIP_LOCKED`、パーティションの範囲による対象漏れを検出する。空テーブルも一律に未修復と判断せず、修復前後の `reltuples` を確認する
- **autoanalyzeの不在は同じ原因**: パーティション親と一時テーブルを別に分類し、wraparound保護を含む各保守作業経路を混同しない

## 出典・ライセンスと未確認事項

UTC取得日2026-10-03。リリースノートの公開日は上記、18系マニュアルの個別公開・更新日は表示なし。公開[PostgreSQL License](https://www.postgresql.org/about/licence/)はソフトウェアと文書を対象とする。既存カタログを変更せず、本稿が参照する5つの新規記録を追加した。最短TTLはrelease_notesの30日なので再確認期限は2026-11-02。

リリースノートの該当コミットリンクはウェブツール取得が失敗したため、実装コミット解析の出典として採用していない。公式リリースノートとマニュアルを根拠にし、40桁コミット固定のリポジトリ分析を行ったとは主張しない。個々の構築条件・発生率・全旧パッチの再現、修復SQLの本番実行、性能やロック待ちの実測は未確認。検索評価は可発見性を検証し、データベースの異常再現試験を代替しない。

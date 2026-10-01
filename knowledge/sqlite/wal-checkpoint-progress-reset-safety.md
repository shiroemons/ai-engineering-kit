---
{
  "id": "sqlite-wal-checkpoint-progress-reset-safety",
  "title": "SQLite WAL checkpoint: 完了・再利用・切詰めの境界と WAL-reset 修正版",
  "kind": "knowledge",
  "technology": "sqlite",
  "version": "SQLite checkpoint contract retrieved 2026-10-01; NOOP added in 3.51.0; WAL-reset fix in 3.51.3 and documented backports",
  "tags": ["research-domain:data", "wal", "checkpoint", "passive", "restart", "truncate", "noop", "starvation", "wal-reset"],
  "sources": [
    {"id": "sqlite-wal-checkpoint-overview-2026-10-01", "url": "https://www.sqlite.org/wal.html", "type": "official_docs"},
    {"id": "sqlite-wal-checkpoint-v2-2026-10-01", "url": "https://www.sqlite.org/c3ref/wal_checkpoint_v2.html", "type": "official_docs"},
    {"id": "sqlite-wal-checkpoint-pragma-2026-10-01", "url": "https://www.sqlite.org/pragma.html#pragma_wal_checkpoint", "type": "official_docs"},
    {"id": "sqlite-release-3-51-3-wal-reset-2026-10-01", "url": "https://www.sqlite.org/releaselog/3_51_3.html", "type": "release_notes"}
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# SQLite WAL checkpoint: 完了・再利用・切詰めの境界と WAL-reset 修正版

## 問いと適用範囲

WAL が大きくなったとき、checkpoint の成功は「全 frame を転記した」「次の writer が再利用できる」「ファイルが小さくなった」のどれを保証するか。
接続初期化と競合待ちを扱う [WAL / busy_timeout](wal-busy-timeout.md) を補い、ここでは進捗の観測、checkpoint の mode、reader による停滞、修正版の境界を確認する。
API 契約は 2026-10-01 UTC に取得した公式文書、版差は明示された release notes に基づく。実行環境の SQLite が最新であるとは仮定しない。

## 公式契約: 戻り値を mode と一緒に読む

根拠は [sqlite3_wal_checkpoint_v2](https://www.sqlite.org/c3ref/wal_checkpoint_v2.html) と [wal_checkpoint PRAGMA](https://www.sqlite.org/pragma.html#pragma_wal_checkpoint)。以下は source の契約であり、本書の推奨値ではない。

- `PASSIVE` は reader / writer の終了を待たずに可能な frame だけ転記する。busy handler は呼ばない。未完でも返り得るため、`SQLITE_OK` だけでは全転記を保証しない。
- `FULL` は writer 不在と、reader が最新 snapshot を読む状態を待ち、全 frame を転記・同期する。待機には busy handler を使い、その間は新たな writer を抑えるが reader は進められる。
- `RESTART` は FULL に加えて、reader が WAL を使わなくなるまで待つ。成功すれば次の writer は WAL の先頭から再利用できる。成功した時点でファイル長がゼロとは限らない。
- `TRUNCATE` は RESTART に加え、成功して返る直前に WAL をゼロ byte に切り詰める。
- `NOOP` は転記せず進捗値だけを取得する。PASSIVE を「観測だけ」の代用品にすると実際に checkpoint が走る。

SQL の `PRAGMA main.wal_checkpoint(MODE)` は3整数の1行を返す。第1列は対応する C API の OK / BUSY に対応して 0 / 1、第2列は WAL の frame 数、第3列は転記済み frame 数を表す。第1列 0 のみを「完了」と解釈しない。
WAL がない場合などは第2・第3列が -1 になる。対象 schema を明示し、-1 を正常な空 backlog と同一視しない。

## 公式契約: 進捗値と競合の落とし穴

[C API](https://www.sqlite.org/c3ref/wal_checkpoint_v2.html) の出力引数・lock・エラー規定が根拠。

- `pnCkpt` は今回の呼出しだけで転記した数ではなく、以前の checkpoint 分も含む WAL 内の転記済み総 frame 数。呼出し回数をまたぐ単純加算はできない。
- 成功した TRUNCATE は `pnLog` と `pnCkpt` の両方が 0。これは未処理ゼロを表すが、過去の転記量がゼロだったという意味ではない。
- すべての mode は checkpoint lock を取る。同時 checkpoint の lock 競合では `SQLITE_BUSY` を返し、設定済みの busy handler も呼ばれない。NOOP も無競合の観測 API ではない。
- FULL / RESTART / TRUNCATE で writer や reader を待ち、busy handler が 0 を返すと、以後は PASSIVE 相当の可能な処理をして BUSY を返す。部分進捗があり得るので、「BUSY なら何も実行されていない」も誤り。
- C API の `zDb` が NULL または空文字なら全 attached WAL database を処理し、`pnLog` / `pnCkpt` は未定義。一つの database の監視値として使うなら名前を指定する。

## 公式契約: ファイル長と checkpoint starvation

[WAL overview](https://www.sqlite.org/wal.html) の Concurrency、The WAL File、Avoiding Excessively Large WAL Files が根拠。

reader はトランザクションの snapshot に必要な WAL を保持するため、長い read transaction や切れ目なく重なる reader は checkpoint の完了・再利用を妨げる。
通常は WAL の割当済み領域を再利用し、checkpoint ごとにファイルを切り詰めない。したがってディスク上の WAL byte 数だけでは未転記 frame 数を判断できない。
WAL は database の永続状態の一部であり、開いている database から `-wal` を手動で削除・分離すると、commit 済みデータの消失や破損につながり得る。

## 確認した版差: NOOP と WAL-reset 修正は別の変更

[3.51.3 release notes](https://www.sqlite.org/releaselog/3_51_3.html) は、過去の 3.51.0 の変更と当該 patch の変更を分けている。

- `NOOP` は 3.51.0（2025-11-04）で追加。3.51.3 が初出ではなく、古いライブラリで当然使えるとも扱わない。
- 3.51.3（2026-03-13）の当該変更は WAL-reset database corruption bug の修正。
- [WAL-reset 節](https://www.sqlite.org/wal.html#the_wal_reset_bug) は、3.7.0 から 3.51.2 に存在する可能性を示し、3.51.3 以降と backport 3.44.6 / 3.50.7 の修正を明記する。単純な数値比較で backport を未修正と判定しない。
- 対象は同じ WAL database を複数接続から扱い、別 thread / process で書込みや checkpoint の時機が重なる特殊な race。公式は稀な条件と説明する一方、修正版への更新を勧めている。
- WAL 文書は 2026-08-24 の更新として、特殊な test hook を使わない再現例が得られたと追記している。「通常の再現手段は存在しない」という古い説明だけを採用しない。

本書では更新の影響を判断するために修正境界を採用し、発生確率・独自再現例・全 vendor backport の網羅は主張しない。2026-10-01 に新機能が出たという意味でもない。

## 設計判断: 観測と領域回収を分ける

以下は上の契約から導いた運用案であり、SQLite が保証する運用手順や設定値ではない。

1. 実際にアプリへ組み込まれた SQLite の版と vendor の修正記録を確認する。OS の CLI の版だけで、別ライブラリを使うサービスの修正済み判定をしない。
2. 対象 database 名、mode、戻り値、log / checkpointed frame 数、WAL の byte 数、read transaction の継続時間を別々に記録する。NOOP 対応版では、監視の目的で転記したくない場合に NOOP を選ぶ。
3. 正常な非負 frame 数に限り、同一呼出しの `log - checkpointed` を未転記量の観測に使う。時刻間の差を処理 throughput と断定せず、reset や並行書込みで値が変わる前提にする。
4. backlog が持続するときは、長い transaction の保持箇所や reader の隙間を調べる。いきなり TRUNCATE を連打すると writer 待ちと checkpoint lock 競合を増やし得る。
5. 転記だけで足りるか、次の書込みで再利用したいか、今ファイルを縮めたいかを先に決めて mode を選ぶ。利用者の遅延許容を測り、FULL / RESTART / TRUNCATE の待ちと BUSY を扱う上限を設ける。
6. WAL が大きいことを理由に sidecar を削除しない。checkpoint 成功を「任意の live file copy が安全」という許可にも変換しない。バックアップは別の契約で設計する。

## 検証案と未確認事項

以下はアプリ実装の受入条件案であり、この調査で SQLite の並行処理や障害を実行した結果ではない。

- reader を開いたまま書込みし、PASSIVE が OK でも未転記 frame が残る場合を正しく表示する。
- reader 解放後に FULL と RESTART の到達条件を比較し、RESTART 後の byte 数をゼロと仮定しない。
- TRUNCATE 成功時は 0 / 0 を処理し、BUSY 時は部分進捗を許して次の試行方針を決める。
- 別接続の checkpoint と競合する場合、busy_timeout だけに依存せず即時 BUSY を扱う。
- NOOP 非対応版、非 WAL database の -1、attached database の指定漏れを監視の正常値に紛れ込ませない。
- 長期 reader が終了しない状態をタイムアウト付きで検証し、監視そのものが無期限の強制 checkpoint にならないことを確認する。

負荷別の最適 threshold、filesystem / VFS ごとの耐久性、言語 binding のエラー変換、vendor patch の内容、WAL-reset の実行再現は未確認。
コードのコピーや module 昇格は行っていない。検索 eval は文書の取得可能性を確認するもので、ここに挙げた runtime 検証の代わりにはならない。

## 出典・日付・ライセンス

全4 source の取得日は 2026-10-01 UTC。WAL ページの表示更新日は 2026-08-25 19:42:39Z、release notes の patch 公開日は 2026-03-13。PRAGMA ページの表示更新日は 2026-06-04 01:35:31Z。C API の独立した公開・更新日は今回の表示では確認していない。
[公式 copyright](https://www.sqlite.org/copyright.html) で SQLite の配布コードと文書が public domain と確認した。build script など別ライセンスの範囲まで同じと推測しない。本書は出典を示した独自の日本語要約。
release_notes の TTL 30日が最短のため再確認期限は 2026-10-31。再確認時は本文の mode・修正境界も照合し、日付のみを更新しない。

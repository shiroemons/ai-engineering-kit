---
{
  "id": "go-database-sql",
  "title": "Go database/sql の production 向け接続プールとトランザクション",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 (database/sql API)",
  "tags": [
    "database",
    "sql",
    "connection-pool",
    "transaction",
    "rollback",
    "context",
    "cancellation",
    "timeout",
    "driver",
    "isolation-level",
    "rows",
    "scan",
    "prepared-statement",
    "query",
    "errnorows"
  ],
  "sources": [
    {
      "id": "go-database-sql-docs",
      "url": "https://pkg.go.dev/database/sql",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-23",
  "expires_at": "2026-12-22",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/search.json"
  ]
}
---

# Go database/sql の production 向け接続プールとトランザクション

`database/sql` は SQL（および SQL 風）データベースの汎用インタフェースで、標準ライブラリに driver は含まれず、サードパーティの driver と組み合わせて使う。[公式 API](https://pkg.go.dev/database/sql)

## 要点

- `DB` は「0 個以上の underlying connection のプール」を表す handle で、複数 goroutine から並行利用できる。`sql.Open` は引数を検証するだけで接続を作らない場合があり、DSN の確認は `Ping` / `PingContext` で行う。`Open` は1回だけ呼び、`DB` は長命で共有するのが公式の前提。
- 接続の帰属: `Tx` は1本の接続に束縛され、`Commit` / `Rollback` 後にその接続は `DB` の idle connection pool へ戻る。idle プールの上限は `SetMaxIdleConns` で制御する。
- プール上限の契約: `SetMaxOpenConns` の既定は 0（無制限）。`SetMaxIdleConns` の既定は 2 で、公式は「将来のリリースで変わる可能性がある」と明記する。idle 上限が open 上限（正の値）より大きい場合は open 上限に合わせられ、`n <= 0` は idle を保持しない。`SetConnMaxLifetime`（go1.6+）は接続の再利用可能期間、`SetConnMaxIdleTime`（go1.15+）は idle 上限時間で、いずれも期限切れ接続を再利用前に遅延で閉じ、0 以下で無効になる。`DBStats` は接続を待った回数と総ブロック時間（`WaitCount` / `WaitDuration`）、各上限によるクローズ数（`MaxIdleClosed` / `MaxIdleTimeClosed` / `MaxLifetimeClosed`）を返す。
- context の範囲: `*Context` でないメソッド（`Query`・`Exec`・`Begin`・`Ping`・`Prepare` 等）は内部で `context.Background()` を使う。取消の効き方は driver 依存で、公式 overview は「context cancellation をサポートしない driver はクエリが完了するまで戻らない」と明記する。`PrepareContext`・`Tx.PrepareContext`・`Tx.StmtContext` の context は preparation のみに使われ、返った statement の実行は準備された側の context で行う。
- トランザクションの取消: `BeginTx` に渡した context は commit / rollback まで使われ、取消されると sql パッケージはトランザクションを rollback し、`Tx.Commit` はエラーを返す（`DB.BeginTx`・`Conn.BeginTx` とも同じ契約）。isolation の既定は driver 依存で、driver が非対応の level はエラーになり得る。`TxOptions` は nil 可。
- `Tx` は `Commit` か `Rollback` の呼び出しで終え、以降の操作はすべて `ErrTxDone` を返す。`Tx.Prepare` / `Tx.Stmt` で作った statement は commit / rollback の呼び出しで閉じられる。
- `Rows` は `Next` が false を返し後続の result set がなければ自動で閉じられ、以後は `Rows.Err` を確認すれば足りる。`Close` は冪等で `Rows.Err` には影響しない。書き込みを含む処理では公式 example が `rows.Close()` のエラーを確認する（auto-commit error が起きて rollback される場合があるため）。
- `QueryRow` / `QueryRowContext` は常に非 nil を返し、エラーは `Scan` まで遅延する。行が無ければ `Scan` は `ErrNoRows` を返す。`Row.Err` は `Scan` を呼ばずに query エラーの有無を確認できる。
- `Stmt` は並行利用可能。`Tx` や `Conn` で作った `Stmt` は1本の接続に永久に束縛され、その `Tx` / `Conn` が閉じると全ての操作がエラーになる。`DB` で作った `Stmt` は `DB` の寿命まで有効で、新しい接続で実行するときは自動でその接続に prepare し直す。`DB.Prepare` の `Stmt` は呼び出し側が `Close` する責務を持つ。
- `Conn` は単一 connection で、公式は「連続した単一 DB session が必要な場合以外は `DB` からの実行を優先」と明記する。`Conn.Close` でプールに戻し、以降の操作は `ErrConnDone`。`Close` は他の操作の完了を待つ。`DB.Conn(ctx)` は接続が得られるか context が取消されるまで待ち、同一 `Conn` 上の query は同一 session で実行される。`DB.Close` は新しい query の開始を止め、サーバ上で処理中の query が終わるのを待つ。
- エラー変数は `ErrNoRows`・`ErrTxDone`・`ErrConnDone`。`Result.LastInsertId` / `RowsAffected` は全データベース・全 driver がサポートするとは限らない。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する数値や構成を指すものではない。

- プロセスに `DB` を1つ作り、`SetMaxOpenConns` と `SetMaxIdleConns` を明示する。既定の idle 2 を本番の想定と読み替え、`DBStats` の `InUse`・`Idle`・`WaitCount`・`WaitDuration` で観測して上限を決める。
- 読み取りは `Request.Context()` を基点に timeout を付けた `*Context` メソッドで実行する。クライアント取消で止めてよい副作用のない処理も同じ基点を使う（公式 example の `/quick-action`・`/long-action`）。逆にクライアント取消で途切れたくない副作用のある処理は、公式 example の `/async-action` どおり `context.Background()` を基点に timeout を付ける。
- トランザクションは `BeginTx` で開始し、`defer tx.Rollback()` を先に置く（公式 example のコメント: 後で commit すれば rollback は無視される）。失敗は rollback、成功のみ commit。`BeginTx` の context が取消されると自動で rollback されるため、commit が必要な処理では取消されない context を基点にする。
- ヘルスチェックと起動確認には短い timeout の `PingContext` を使う。
- `Rows` は early return / break でも `Close` し、走査後に `rows.Err()` を確認する。書き込みを含む場合は `rows.Close()` の戻り値も確認する。
- 使い続ける query だけを `PrepareContext` し、`defer stmt.Close()` でサーバ側リソースを回収する。DB 由来の `Stmt` をトランザクション内で使うときは `Tx.Stmt` / `Tx.StmtContext` でトランザクション用に取り直す。

## 避ける使い方

- `sql.Open` の成功だけで疎通できたと考える。接続を作らない場合があり、確認は `PingContext`。
- production の実行経路で `Query` / `Exec` / `Begin` / `Prepare` 等の非 context メソッドを使う。内部は `context.Background()` で、呼び出し側の timeout と取消が効かない。
- context を渡せばどの driver も query を中断すると考える。公式は cancellation 非対応 driver はクエリ完了まで戻らないと明記する。
- リクエストごとに `Open` / `Close` する。`Open` は1回、`DB` は共有が前提。
- `Rows`・`Stmt`・`Conn` を閉じない。`Rows` の自動 Close は `Next` が false を返した場合に限られ、途中で抜ける走査では呼び出し側の `Close` が要る。`Stmt` はサーバ側リソースを使う。
- `Commit` も `Rollback` も呼ばずに `Tx` を放棄する。`Tx` はどちらかで終える契約で、以後の操作は `ErrTxDone` になる。
- `BeginTx` に渡した context が commit 前に取消されても commit できると考える。公式は取消時に自動 rollback され、`Commit` はエラーを返すと明記する。
- `LastInsertId` / `RowsAffected` を全環境で使えると信頼する。公式は対応が一様でないと明記する。
- `RawBytes` を次回の `Rows.Next` / `Scan` / `Close` の後も保持する。`RawBytes` は元データへの参照なので、保持するならコピーする。一方、`Scan` の宛先が `*[]byte` の場合は呼び出し側が所有するコピーが作られる。独自 `Scanner` に渡される `[]byte` も必要なら次の `Scan` 前にコピーする。
- isolation level を指定しただけで全 driver で同じ保証を得られると考える。非対応の level はエラーになり得る。既定レベルは driver 依存。
- `Conn` を通常経路に使う。公式は単一 session が必要な場合以外は `DB` 利用を優先と明記する。

## 適用版と本番での注意

- `Go 1.27.1 (database/sql API)` と pkg.go.dev の表示 `go1.27.1`（Published Sep 1, 2026）で確認する。将来の最新とは扱わない。
- 導入版: `*Context` メソッド・`BeginTx`・`TxOptions`・`IsolationLevel`・`Named` は go1.8、`Conn` は go1.9、`OpenDB` は go1.10、`SetConnMaxLifetime` は go1.6、`SetConnMaxIdleTime` は go1.15、`Null[T]` は go1.22、`ConvertAssign`（driver 向け）は go1.27。古い Go では該当 API が無い。
- driver ごとに差が出る項目は、cancellation のサポート、isolation level のサポート、`RowsAffected` / `LastInsertId`、placeholder の形式。標準ライブラリに driver は含まれない。
- 本文のプール上限の決め方と監視構成は設計判断であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。

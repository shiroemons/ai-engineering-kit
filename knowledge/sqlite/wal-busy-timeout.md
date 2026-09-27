---
{
  "id": "sqlite-wal-busy-timeout",
  "title": "SQLite の WAL モード切替と busy_timeout による競合待ちの契約",
  "kind": "knowledge",
  "technology": "sqlite",
  "version": "SQLite 3.7.0+ (WAL availability as documented)",
  "tags": ["sqlite", "wal", "journal_mode", "busy_timeout", "concurrency", "research-domain:data"],
  "sources": [
    {"id": "sqlite-pragma-docs", "url": "https://www.sqlite.org/pragma.html", "type": "official_docs"},
    {"id": "sqlite-c-busy-timeout-docs", "url": "https://www.sqlite.org/c3ref/busy_timeout.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active"
}
---

# SQLite の WAL モード切替と busy_timeout による競合待ちの契約

`PRAGMA journal_mode=WAL` で先行書き込みログを使う設定に切り替え、`PRAGMA busy_timeout` または `sqlite3_busy_timeout()` でロック競合時の待ち時間を決める。以下は公式文書で確認した契約だけを書く。

## 要点

- 既定の journal モードは `DELETE` である。WAL を使う場合は明示的に `PRAGMA journal_mode=WAL;` を実行する。[PRAGMA Statements](https://www.sqlite.org/pragma.html)
- WAL は write-ahead log を使う方式であり、設定は接続をまたいで永続する。一度切り替えれば接続ごとに設定し直す必要はない。[PRAGMA Statements](https://www.sqlite.org/pragma.html)
- WAL を使えるのは SQLite 3.7.0 以降である。それより古い版での動作は本書の対象外。[PRAGMA Statements](https://www.sqlite.org/pragma.html)
- `PRAGMA journal_mode` は成功時に新しいモードを1行で返し、失敗時には変更前のモードを返す。文の成功だけを見て切り替わったと判断せず、戻り行の値を読む。[PRAGMA Statements](https://www.sqlite.org/pragma.html)
- `PRAGMA busy_timeout` はミリ秒単位で busy timeout を問い合わせ・設定する。これは `sqlite3_busy_timeout()` の代替手段である。[PRAGMA Statements](https://www.sqlite.org/pragma.html)
- 1つの接続が持てる busy handler は1つだけである。`PRAGMA busy_timeout` の設定はその接続の既存 handler を上書きする。[PRAGMA Statements](https://www.sqlite.org/pragma.html)
- `sqlite3_busy_timeout()` の handler は、累積の sleep 時間が指定ミリ秒に達するまで繰り返し sleep する。累積が指定値以上になると handler は 0 を返し、`sqlite3_step()` は `SQLITE_BUSY` を呼び出し元に返す。[Set A Busy Timeout](https://www.sqlite.org/c3ref/busy_timeout.html)
- 引数が 0 以下の場合、すべての busy handler が無効になる。競合時は即座に `SQLITE_BUSY` が返る運用になる。[Set A Busy Timeout](https://www.sqlite.org/c3ref/busy_timeout.html)
- 1つの接続に設定できる busy handler は1つだけであり、`sqlite3_busy_timeout()` を呼ぶとそれ以前の `sqlite3_busy_handler()` 設定は消去される。[Set A Busy Timeout](https://www.sqlite.org/c3ref/busy_timeout.html)

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する数値や構成を指すものではない。

- WAL 切替は起動時の移行処理として1回実行し、戻り行が `wal` であることを確認してから本処理に進む。失敗時は prior mode が返る契約のため、戻り値検査を省略しない。
- busy_timeout は接続単位の handler であるため、接続を開くたびに設定する。プールや fork 後の再接続では設定が引き継がれない前提で扱う。
- 競合をリトライで吸収したい用途では timeout を正のミリ秒に設定し、即時失敗を検出したい用途でのみ 0 以下を使う。timeout 中は sleep の繰り返しであり、呼び出し側はその間 block される前提で設計する。
- 独自の `sqlite3_busy_handler()` と `sqlite3_busy_timeout()` / `PRAGMA busy_timeout` は1つの接続上で共存しない。どちらか一方に統一し、後からの設定が前を消すことを前提に初期化順序を固定する。

## 避ける使い方

- `PRAGMA journal_mode=WAL;` の実行成功だけで WAL 化を断定する。失敗時は prior mode が返るため、戻り行を見ないと `DELETE` のまま運用する恐れがある。
- 接続ごとの `busy_timeout` 設定を省略する。WAL 設定は永続するが busy handler は接続単位であり、別接続には引き継がれない。
- 引数 0 以下を「短い待ち」と解釈する。文書上は handler 無効化であり、競合時は即 `SQLITE_BUSY` になる。
- 独自 handler と timeout 系設定の併用を期待する。1接続1 handler の契約により後勝ちで上書きされる。

## 適用版と本番での注意

- `SQLite 3.7.0` 以降の WAL 提供契約で確認する。将来の最新とは扱わない。
- 本文の待機・初期化手順は設計判断であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。
- 未確認事項: `PRAGMA synchronous` の各段階の耐久性、WAL の読み書き並行度、checkpoint の条件と運用値。これらは今回の検証対象外であり、必要になれば公式文書の該当節を別途確認する。推測で耐久性や並行性を主張しない。

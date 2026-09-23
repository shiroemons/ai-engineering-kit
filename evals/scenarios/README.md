# 再利用シナリオ

最初のシナリオは context の検索からキャンセル可能な待機の検証までとする。

```sh
just check
go run ./cmd/kb search context --json
go test -race ./modules/go/contextwait
```

検索で knowledge・pattern・module を取得し、期限と出典を確認する。module テストは [JSON eval](../modules/contextwait.json) を読み、正常終了・nil・キャンセル・期限切れ・ゼロ・負値を検証する。待機中の中止と64呼び出しの並行テストも実行する。

将来 idempotency を追加する場合は、次の候補を eval 化する。これらは未実装であり、検証済みとは扱わない。

- 重複要求
- 同一キーと同一 payload
- 同一キーと異なる payload
- 並行要求
- timeout
- storage failure
- 失敗後の retry

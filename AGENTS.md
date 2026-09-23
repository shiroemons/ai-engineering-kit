# このリポジトリでの原則

- 新規実装前に `just index` と `go run ./cmd/kb search "検索語" --json` で knowledge・patterns・modules を横断検索する。
- stale を最新情報として扱わない。元の source と適用版を確認し、再調査の結果に基づいて取得日を更新する。
- 公式一次情報を優先し、事実と独自の設計案を区別する。
- metadata と source の必須項目は [契約](docs/metadata.md) に従う。
- module への追加前に用途・API・README・単体テスト・境界テスト・並行性・eval・provenance を確認する。
- `just test` と `just validate` を通し、コードと記録をレビューしてから昇格する。
- OSS 由来のコードはライセンスと provenance を確認する。不明なライセンスのコードを持ち込まない。
- 既存 module で用途を満たせるなら再利用し、重複 module を作らない。
- `rag/` は生成物。正本を編集し、`just index` で再生成する。
- OSS clone は `.workbench/repositories/` に限定し、不要になった clone だけを確認して削除する。
- 調査・昇格の手順は [運用手順](docs/workflows.md) を参照する。
- 依存は標準ライブラリを優先し、失敗を握りつぶさずテストする。
- Git のブランチ作成は事前確認する。コミットを依頼された場合だけ作成する。

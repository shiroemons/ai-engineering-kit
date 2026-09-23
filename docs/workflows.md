# 収集から再利用まで

知識とコードは、根拠・適用条件・検証結果を確認して追加する。実行に失敗したら修正し、該当する検証を通すまで昇格を止める。

## knowledge を追加する

1. `go run ./cmd/kb search "検索語" --json` で重複候補を探す。index がなければ `just index` を先に実行する。
2. 公式文書を読み、URL・表示版・取得日・ライセンスを `sources/catalog/<id>.json` に保存する。
3. [knowledge template](../examples/knowledge.template.md) を対象の技術ディレクトリへコピーし、拡張子を `.md` にする。
4. 要点・推奨方法・避ける使い方・版の差・本番での注意点を、確認できた範囲で書く。未確認事項を推測で埋めない。
5. metadata と検索 eval を追加し、`just validate`・`just test`・`just index` を実行する。
6. 実際の検索語で取得でき、参照リンクと適用範囲が正しいことを確認する。

`examples/` と `*.template.md` は検証・index の対象外。コピー後は sample の ID・日付・出典を実データに置き換える。

## OSS の設計を分析する

clone する URL と保存名を確認してから、次の各コマンドを個別に実行する。例の URL は調査対象に置き換える。

```sh
mkdir -p .workbench/repositories
git clone https://github.com/OWNER/REPO.git .workbench/repositories/REPO
git -C .workbench/repositories/REPO rev-parse HEAD
git -C .workbench/repositories/REPO status --short
```

得られた40桁の commit SHA を source に保存する。repository の LICENSE と該当ファイルのヘッダーを確認し、識別子・URL・例外を記録する。unknown のまま module へ持ち込まない。

architecture、責務分割、公開 API、エラー処理、retry、並行性、永続化、テスト、ログ・メトリクス、拡張点を読む。pattern には根拠のパスと「なぜそう設計するか」、代償を残す。未調査の項目は未確認と明示する。

コードのコピーや改変がある場合は、使用範囲と出典を provenance に記録する。分析後は knowledge/pattern、source、eval を保存して検証する。clone は再取得できることと未保存の作業がないことを確認し、不要な対象だけを削除する。

## module へ昇格する

Source → Research → Knowledge → Pattern → Prototype → Tests → Evals → Review → Module の順で根拠を積み上げる。各段階は次の成果物を残す。

| 段階 | 残すもの |
|---|---|
| Source | 出典とライセンス |
| Research | 問い・根拠・未確認事項 |
| Knowledge | 技術の適用条件 |
| Pattern | 判断理由とトレードオフ |
| Prototype | 再利用価値を確かめる試作 |
| Tests | 正常・異常・境界・並行性のテスト |
| Evals | 実行可能な利用シナリオ |
| Review | API・テスト・由来の確認者と日付 |
| Module | README・コード・module.json・provenance |

試作は `.workbench/` に置く。昇格候補を `modules/<technology>/<name>/` へ移し、[module template](../examples/module.template.md) に沿って説明する。`module.json` の tests は module 相対、evals は repository 相対のパスを指定する。JSON eval を Go テストが読み、期待する結果を照合する例は [contextwait](../modules/go/contextwait/README.md) を参照する。

`just check` が成功し、レビューで用途・API・並行性・provenance を確認できたものを module として採用する。`reviewed_by` と `reviewed_at` は実際の確認を記録する。形式検証だけでテストやレビューの完了とは扱わない。知識から必ずコードを作る必要はない。

## stale を更新する

`just freshness` で期限切れを探し、source を再取得して変更点を確認する。本文と適用版を見直し、source と文書の取得日、明示期限を更新する。内容を確認せず日付だけを延ばさない。`just check` と検索で結果を確認する。

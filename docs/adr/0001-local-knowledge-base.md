# ADR-001: Markdown を正本とするローカル検索

状態: 採用。判断日: 2026-09-23。

## 背景

個人と AI エージェントが知識を共有するには、根拠・期限・テストを追跡できる必要がある。初期段階では、検索基盤の運用費より内容の検証を優先する。

## 決定

本文は Markdown、metadata と設定は JSON にする。Go 標準ライブラリで CLI を実装し、追加の parser や DB を不要にする。JSON Front Matter は YAML より記述量が多いが、厳密に検証できる。

knowledge・patterns・modules は正本、rag は再生成するデータとする。検索は Backend インターフェースを介し、初期実装では AND 条件の部分一致を使う。後からローカル embedding、OpenAI Embeddings、Vector DB、hybrid search を追加する場合も、入力の検証と期限・信頼度の扱いを維持する。

Go 1.27 の [encoding/json/v2](https://go.dev/doc/go1.27) で重複キーや不正な UTF-8 を拒否する。未知の項目も拒否し、metadata の曖昧な解釈を防ぐ。出力は従来の JSON API で安定した順序を保つ。

Go 1.26 の [errors.AsType](https://go.dev/doc/go1.26) で包まれたエラーの型を確認する。[go fix](https://go.dev/doc/go1.26) を更新手順に組み込む。[testing/synctest](https://pkg.go.dev/testing/synctest) は待機テストを仮想時計で決定的にする。新 API は用途がある箇所に限って採用する。

## 影響

ネットワークなしで検証・検索できる。意味の近い語や日本語の語形変化は取得できず、文書量に応じて読み込みコストも増える。実測と検索 eval で不足を確認してから検索方式を増やす。配布ライセンスの選定は所有者の判断として残す。

---
{
  "id": "performance-k6-completion-exit-code-linger",
  "title": "k6 v2 の完了判定: Cloud abort の exit code 97 と v2.3 execution_result / linger",
  "kind": "knowledge",
  "technology": "performance",
  "version": "k6 v2.0.0 (2026-05-11) and v2.3.0 (2026-09-21); v2.3.x docs verified 2026-10-01",
  "tags": [
    "research-domain:quality-operations",
    "performance",
    "k6",
    "CI",
    "completion",
    "exit_code",
    "execution_result",
    "linger",
    "exit-on-running"
  ],
  "sources": [
    {
      "id": "k6-release-v2-0-0-completion-20261001",
      "url": "https://github.com/grafana/k6/releases/tag/v2.0.0",
      "type": "release_notes"
    },
    {
      "id": "k6-release-v2-3-0-status-20261001",
      "url": "https://github.com/grafana/k6/releases/tag/v2.3.0",
      "type": "release_notes"
    },
    {
      "id": "k6-options-completion-20261001",
      "url": "https://grafana.com/docs/k6/latest/using-k6/k6-options/reference/",
      "type": "official_docs"
    },
    {
      "id": "k6-cloud-commands-completion-20261001",
      "url": "https://grafana.com/docs/k6/latest/reference/cloud-commands/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# k6 v2 の完了判定: Cloud abort とプロセス終了を混同しない

## 問いと選定理由

負荷試験を CI の昇格条件にするとき、「CLI が返った」「プロセスが生きている」「試験が完了した」のどれを成功判定に使うべきか。v2.0.0 の Cloud abort 終了コード変更と、2026-09-21 公開の v2.3.0 に追加された `execution_result` は、開始受付を合格と誤認する監視を変える具体的な版差である。既存の [arrival-rate と threshold](k6-coordinated-omission-arrival-rate-latency.md) は負荷モデルと測定基準を扱う。本書は実行結果を回収する境界に限定する。

取得日は 2026-10-01 UTC。release 日は GitHub の release metadata で確認し、取得日と区別した。以下の事実と、それを組み合わせた運用提案を分ける。

## 確認した契約

### 1. Cloud abort の終了コードが v2 で変わった

[v2.0.0 release](https://github.com/grafana/k6/releases/tag/v2.0.0) は、Cloud 実行の system、limit、script error、user、timeout による abort を従来の `0` から `97` に変更した。threshold による abort は前後とも `99`、release の表にある Finished は `0` のままである。したがって「abort は全部 99」という分類も、旧版の 0 を新しい意味で遡及評価する運用も誤る。

これは Cloud の当該終了経路の比較表であり、全 k6 エラーコードの一覧ではない。認証失敗・起動失敗なども含めて 0/97/99 の三値しか存在しないとは扱わない。同 release では Cloud の stack 指定が必須になり、旧来の最初の stack へのフォールバックもなくなった。

### 2. Cloud の開始受付と最終結果は別

[Options reference](https://grafana.com/docs/k6/latest/using-k6/k6-options/reference/#exit-on-running) では、通常の `k6 cloud run` は finalized status まで待つ。`--exit-on-running` / `K6_EXIT_ON_RUNNING` は既定 `false` で、明示有効化すると `running` に達したところで CLI が先に終了し、試験は背後で継続する。

[Cloud commands](https://grafana.com/docs/k6/latest/reference/cloud-commands/) の `cloud run` は script と resources の archive をアップロードして Cloud 側で試験を実行する。`--local-execution` はローカル実行と Cloud への結果送信を選ぶ別モードである。`cloud upload` はアップロードだけで、実行しない。いずれも単にコマンド名に cloud があることから、同じ完了確認経路だとは推定しない。

### 3. v2.3.0 では linger 中も結果を読める

[v2.3.0 release](https://github.com/grafana/k6/releases/tag/v2.3.0) は、REST API `/v1/status` に `execution_result` を追加した。結果未確定なら `null`、確定後は test の `exit_code` を持つ。JSON の参照位置は `data.attributes.execution_result`。release の `exec.test.abort()` 例は `exit_code: 108` を示す。Cloud abort の `97` をローカル script abort の値として流用してはいけない。

同じ release が説明する問題は、`--linger` によって試験終了後もプロセスが残ると、OS の process exit status だけでは完了結果を回収できないこと。Options reference でも `linger` / `K6_LINGER` は既定 `false`、ローカル `k6 run` の試験完了後にプロセスを残す設定と定義される。生存監視を成功監視に置き換える情報ではない。

### 4. REST API は v2 で明示有効化が必要

[v2.0.0 release](https://github.com/grafana/k6/releases/tag/v2.0.0) と [Address option](https://grafana.com/docs/k6/latest/using-k6/k6-options/reference/#address) は、HTTP API server が既定で起動しなくなったことを示す。利用には `--address` または `K6_ADDRESS` を設定する。v1 の既定 localhost listener を想定したまま v2 の `/v1/status` を呼んでも、到達できるとは限らない。

## 実務での選択（独自の設計案）

1. リリース判定の CI では通常の Cloud final status 待ちを選び、k6 の実際の終了コードをジョブへ伝える。`97` は実行が中断した診断、`99` は threshold による中断の診断として記録し、その他の非ゼロも失敗として調べる。ログ転送や後処理が k6 の失敗コードを上書きしないようにする。
2. 開始だけを返す用途で `--exit-on-running` を選ぶなら、ジョブの成功ラベルを「開始確認」とし、別の認可済み経路で同一 test run の最終状態を回収するまで昇格を保留する。別 run の成功を取り違えない識別子と回収期限も設計する。具体的な Cloud polling API は本調査で検証していない。
3. ローカル診断で `--linger` が必要なら、v2.3.0 の版と明示した API address をセットで管理し、`execution_result` が `null` または欠落なら未判定とする。接続失敗も合格に変換しない。確定した非ゼロは診断情報として保存する。
4. API listener を不要に公開しないため、同一ホスト監視なら loopback を選ぶ。これは本書の安全側の構成提案であり、REST API の認証や外部公開の安全性を検証したという意味ではない。

## 移行時の境界確認

以下は実装済みテストではなく、採用先で用意する検証項目である。

- 旧版と v2 の Cloud user/timeout abort を同条件で比較し、旧 0 と新 97 を履歴上区別できるか
- threshold abort の 99 を保持し、未知の非ゼロを成功扱いしないか
- `--exit-on-running` を使った後に試験が失敗しても、開始ジョブだけでリリースしないか
- `cloud upload` の成功を「試験実行済み」と表示しないか
- v2.3 の linger で `execution_result: null`、成功 0、script abort 108、API 到達不能を別状態として扱えるか
- v2 に更新して listener が既定無効でも、監視接続失敗を試験成功へ補完しないか

## 適用範囲・ライセンス・未確認

- 対象は確認済み v2.0.0 と v2.3.0、および取得時に v2.3.x と表示された公式文書。v2.3.0 tag の対象 commit は `e0887846143ab176d4b5483c9d52cf3b3e009f1a`。v2.3 より前に `execution_result` があるとは保証しない。
- k6 repository の [LICENSE.md](https://github.com/grafana/k6/blob/v2.3.0/LICENSE.md) は AGPL-3.0。文書サイトでは open license を確定できず、catalog は unknown とした。本書は帰属付きの独自要約で、source code や例をコピーしていない。module 昇格はしない。
- k6 binary や Cloud の実試験は行っていない。ネットワーク断時の Cloud CLI 再接続、すべての終了コード、Cloud API による監視、v1 patch ごとの backport、listener の認証設定は未確認。
- release_notes の TTL 30日に合わせ 2026-10-31 を再確認期限とする。期限延長時は本文と版差を再取得し、旧 source record は上書きしない。

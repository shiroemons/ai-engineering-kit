---
{
  "id": "k6-coordinated-omission-arrival-rate-latency",
  "title": "coordinated omission を回避する k6 arrival-rate 負荷試験と百分位レイテンシの threshold 計測",
  "kind": "knowledge",
  "technology": "performance",
  "version": "k6 v2.3.0 / Grafana k6 v2.3.x docs (verified 2026-09-27)",
  "tags": [
    "research-domain:quality-operations",
    "performance",
    "k6",
    "load testing",
    "負荷試験",
    "coordinated omission",
    "arrival rate",
    "arrival-rate",
    "open model",
    "closed model",
    "percentile",
    "百分位",
    "latency",
    "レイテンシ",
    "threshold",
    "preAllocatedVUs",
    "maxVUs",
    "dropped_iterations",
    "p99"
  ],
  "sources": [
    {
      "id": "k6-docs-open-vs-closed-models",
      "url": "https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/open-vs-closed/",
      "type": "official_docs"
    },
    {
      "id": "k6-docs-constant-arrival-rate",
      "url": "https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/",
      "type": "official_docs"
    },
    {
      "id": "k6-docs-ramping-arrival-rate",
      "url": "https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ramping-arrival-rate/",
      "type": "official_docs"
    },
    {
      "id": "k6-docs-arrival-rate-vu-allocation",
      "url": "https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/arrival-rate-vu-allocation/",
      "type": "official_docs"
    },
    {
      "id": "k6-docs-thresholds",
      "url": "https://grafana.com/docs/k6/latest/using-k6/thresholds/",
      "type": "official_docs"
    },
    {
      "id": "k6-release-notes-v2-3-0",
      "url": "https://github.com/grafana/k6/releases/tag/v2.3.0",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-10-27",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/quality-operations.json"]
}
---

# coordinated omission を回避する k6 arrival-rate 負荷試験と百分位レイテンシの threshold 計測

「毎秒 N リクエストが届く本番負荷」を再現するには、負荷生成の iteration 開始を SUT の応答時間から切り離す必要がある。切り離さないと SUT が遅いほど負荷が下がり、レイテンシ計測が楽に見える（coordinated omission）。以下は Grafana k6 の公式ドキュメント（v2.3.x ライン）5件と k6 v2.3.0 の release notes（2026-09-27 に再取得して確認）に記載された事実と、それを組み立てる設計案を分けて書く。

## 要点（公式情報に記載された事実）

### closed model の到着レート減衰こそ coordinated omission（Open and closed models）

- closed model では次の iteration が前の iteration の完了後にしか始まらず、新規 iteration の開始レートが iteration 所要時間と密結合する。応答が遅いと iteration が長くなり新規 iteration の到着レートが下がり、速いと上がる。この問題は testing literature で *coordinated omission* と呼ばれる。[Open and closed models](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/open-vs-closed/)
- 公式例: `constant-vus` 1 VU で応答 6 秒の API を1分間のテストで叩くと、iteration 所要 6 秒のため到着は 1 本/6 秒となり、1分間で完了するのは 10 iterations にとどまる。[Open and closed models](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/open-vs-closed/)
- open model は iteration 開始を iteration 所要時間から切り離し、SUT の応答時間が負荷に影響しない。k6 は `constant-arrival-rate` と `ramping-arrival-rate` の2つの arrival-rate executor で open model を実装する。[Open and closed models](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/open-vs-closed/)
- 公式例: `rate: 1, timeUnit: '1s', duration: '1m', preAllocatedVUs: 20` の open model シナリオは、同じ応答 6 秒の API に対し 60 iterations を完了させ、1 iters/s を維持した。[Open and closed models](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/open-vs-closed/)

### constant-arrival-rate executor のオプション（Constant arrival rate）

- オプション: `duration`（必須。`gracefulStop` を除く総シナリオ時間）、`rate`（必須。各 `timeUnit` に開始する iteration 数）、`preAllocatedVUs`（必須。テスト開始前に割り当てる VU 数）、`timeUnit`（既定 `"1s"`）、`maxVUs`（未指定時は `preAllocatedVUs` と同値）。[Constant arrival rate](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/)
- この executor は VU がある限り指定レートで iteration を開始し続ける。iteration の開始は同時ではなく分数的に間隔を取られる（`rate: 10` / `timeUnit: '1s'` なら各 iteration は約 100ms ごとに開始される）。[Constant arrival rate](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/)
- iteration の末尾に `sleep()` を置かないこと。到着レートは `rate` と `timeUnit` が調整するため、iteration の終わりに sleep は不要と明記されている。[Constant arrival rate](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/)
- 公式例: `rate: 30, timeUnit: '1s', duration: '30s', preAllocatedVUs: 2, maxVUs: 50` で、k6 が必要に応じて最大 17 VU まで自動増分してレートを維持した。ただし `preAllocatedVUs` が低すぎると、リソースを継続的に割り当てる必要が生じ、目標レートでテストを維持できる時間が減ると注意されている。[Constant arrival rate](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/)

### ramping-arrival-rate executor のオプション（Ramping arrival rate）

- オプション: `stages`（必須。`{target, duration}` の配列）、`preAllocatedVUs`（必須）、`startRate`（既定 `0`）、`timeUnit`（既定 `"1s"`。`startRate` と各 stage の `target` に適用される期間で、シナリオ全体を通して固定であり stage ごとの変更は不可）、`maxVUs`（未指定時は `preAllocatedVUs` と同値）。[Ramping arrival rate](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ramping-arrival-rate/)
- open model executor として、到着レートを SUT の応答とは独立にランプし、テストに十分な割り当て VU がある間だけ iteration を開始する。k6 が VU 数を調整して stage の target を狙う。`gracefulStop` 中は新しい iteration を開始しない。[Ramping arrival rate](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ramping-arrival-rate/)
- 末尾 `sleep()` の非推奨は constant 版と同じ。公式例は `startRate: 300, timeUnit: '1m', preAllocatedVUs: 50` に4段の stages を置く構成。[Ramping arrival rate](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ramping-arrival-rate/)

### preAllocatedVUs と maxVUs の割り当て（Arrival-rate VU allocation）

- arrival-rate シナリオは開始前に `preAllocatedVUs` を初期化する。割り当てが不足して開始できない iteration ごとに k6 は `dropped_iterations` メトリクスを送出する。[Arrival-rate VU allocation](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/arrival-rate-vu-allocation/)
- 事前割当の目安式は `preAllocatedVUs = [median_iteration_duration * rate] + constant_for_variance`。ただしこれは理想式であり、実際には iteration 所要時間はテスト中に増える傾向があり、応答が遅くなり VU が足りなくなると k6 は iteration をドロップする。ローカルで試しながら `preAllocatedVUs` を増やすことが前提とされている。[Arrival-rate VU allocation](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/arrival-rate-vu-allocation/)
- `maxVUs` を設定すると k6 は (1) `preAllocatedVUs` を事前割当、(2) レート到達を試み、(3) 不足すれば VU を追加割当、(4) `maxVUs` に達するまで継続、という順に動く。しかし実行時の VU 割当は CPU・メモリコストを持ち、load generator を過負荷にして結果を歪める（skew）可能性があるため、文書は「almost all cases でやるべきことは、必要な VU を事前割当すること」と明記する。`maxVUs` が妥当なのは初回試験での割当量の探索、`preAllocatedVUs` への小さな予備、load generator を慎重にスケールする巨大な分散テスト、の3つに限られる。[Arrival-rate VU allocation](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/arrival-rate-vu-allocation/)
- Grafana Cloud のテストでは `preAllocatedVUs` が契約にカウントされ、`maxVUs` は `preAllocatedVUs` を上回る分として契約にカウントされる。[Arrival-rate VU allocation](https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/arrival-rate-vu-allocation/)

### thresholds による百分位レイテンシの合否判定（Thresholds）

- threshold はテストメトリクスに対する pass/fail 基準。式がテスト終了時点で false ならテスト全体が fail になり、k6 は非ゼロの exit code を返す。[Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/)
- 式の形式は `<aggregation_method> <operator> <value>`。Trend 型（レイテンシ等）で使える集計は `avg`、`min`、`max`、`med`、`p(N)`（N は 0.0〜100 の数値、例 `p(99.9)`）。値の単位はミリ秒。N が 0〜100 の範囲外や NaN はパースエラーになり、テストは実行前に停止する。[Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/)
- 公式例: `http_req_duration: ['p(90) < 400', 'p(95) < 800', 'p(99.9) < 2000']` で百分位ごとの上限を、`http_req_failed: ['rate<0.01']` でエラー率を合否にする。[Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/)
- 同一 metric に複数の式を置くときは1つのキーの配列にまとめる。同じ object key を繰り返して書くと、2つ目以降は**黙って無視される**。[Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/)
- タグ付きの sub-metric threshold は `'http_req_duration{type:API}': ['p(95)<500']` のように書く。[Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/)
- 長形式は `threshold`・`abortOnFail`・`delayAbortEval` を持つ。ただし k6 Cloud では threshold の評価が60秒ごとに行われるため、`abortOnFail` の停止は最大60秒遅れる。[Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/)
- `checks` は threshold と違い exit status に影響しない。checks のみではテストを失敗させられず、`checks: ['rate>0.9']` のように checks メトリクス自体へ threshold を付ける必要がある。[Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/)
- v2.3.0 追加の `--scenario` で scenario を選択すると、除外した scenario にタグされた threshold は警告付きでスキップされ（選択 scenario がそのタグでサンプルを出してもスキップは解除されない）、global threshold と他のタグフィルタは有効なまま残る。[Thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/) / [v2.3.0 release notes](https://github.com/grafana/k6/releases/tag/v2.3.0)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- 「毎秒 N リクエスト」という到着率を再現する目標なら、open model の arrival-rate executor を選ぶ。closed model（`constant-vus` や `shared-iterations` 等）は応答遅延で負荷が自動的に下がるため、遅延耐性を測る試験に向かない。スループット上限の探索など、負荷を「完了数」で律速したい目的なら closed model も選択肢になる。
- `preAllocatedVUs` は目安式（median iteration 所要時間 × rate + 余白）で概算し、ローカル試行で `dropped_iterations` が 0 になるまで増やす。`maxVUs` は予備として小さく留め、実行時増分に依存しない。ドキュメントが事前割当を最善とする理由（load generator の過負荷と結果の歪み）を前提に設計する。
- iteration の末尾には `sleep()` を置かない。レート調整は `rate` / `timeUnit` / `stages` に任せる。
- 百分位の合否は SLO と対にして threshold に落とす。例として `http_req_duration: ['p(95)<200', 'p(99)<500']` と `http_req_failed: ['rate<0.01']` を同じ `thresholds` オブジェクトに置き、CI では非ゼロ exit code を失敗条件にする。具体的な数値は本例の例示であり、対象サービスの SLO から決める。
- 結果は百分位と `dropped_iterations` を必ず併せて見る。dropped が出ていれば目標到着率を維持できていなくなり、その百分位は目標負荷下の分布ではない。ドキュメントも dropped をシステム性能劣化の兆候と位置づけている。
- 段階的な負荷（warm-up → 本試験 → 降下）は `ramping-arrival-rate` で組み、`timeUnit` を全 stage 共通で先に決める。stage ごとの単位変更はできない。
- `abortOnFail` は早期打ち切りの補助として使い、Cloud 実行では評価間隔60秒により即時停止しない前提で運用する。十分なサンプルを取ってから判定したい場合は `delayAbortEval` を併用する。

## 避ける使い方

- closed model で「応答遅延時のレイテンシ」を測る。SUT が遅くなるほど到着レートが下がり、計測対象の負荷そのものが減る（coordinated omission の典型）。
- `preAllocatedVUs` を削って `maxVUs` に頼る。実行時割当で load generator を過負荷化して結果が歪み、Cloud では契約も `maxVUs` を上回る分まで使われる。
- 同じ metric キーを繰り返して threshold を書く。2つ目以降が黙って捨てられ、意図した1つ目の条件しか評価されない。
- `checks` の結果だけで CI の合否を決める。checks は exit status に影響しないため、テストは成功扱いになる。
- arrival-rate executor の iteration 末尾に `sleep()` を置いてレートを調整する。executor 側のペacingと二重になり、到着率の制御を壊す。
- `dropped_iterations` を見ずに百分位を喜ぶ。開始できた iteration の分布しか見ておらず、目標到着率未達の可能性を排除できない。
- `p(N)` の N に 100 や NaN のような範囲外・不正値を入れる。パースエラーでテストは実行前に停止する。

## 適用版と本番での注意

- 適用版: k6 v2.3.0（release notes 2026-09-21、commit `e0887846143ab176d4b5483c9d52cf3b3e009f1a`）と、そのラインに対応する Grafana k6 docs latest（v2.3.x ドキュメント）。release notes 上、v1 はメンテナンス branch として扱われている。将来の版とは扱わない。
- 再確認期限: 公式 docs 5件は `official_docs`（TTL 90日）で 2026-12-26、release notes は `release_notes`（TTL 30日）で 2026-10-27。文書の期限は最短に合わせて 2026-10-27。`performance` は `config/freshness.json` に技術 TTL の定義がないため技術期限は適用されない。
- 未確認事項（本調査の範囲外として推測で埋めない）: Trend メトリクスの百分位の内部計算方式（サンプル蓄積構造と近似誤差）、`dropped_iterations` の summary 表示やアラート条件の詳細、OSS 実行時（非 Cloud）の threshold 評価間隔、k6 以外の負荷試験ツールでの coordinated omission 回避方法、`http_req_duration` がどのサンプル集合（`expected_response` 等）で百分位を算出するかの詳細。
- 本ドキュメントの threshold 数値と VU 見積もりは例示・設計案であり、公式が保証する数値ではない。`preAllocatedVUs` の正しい値は iteration コードと SUT の処理速度に依存するため、対象ワークロードで測って決める。

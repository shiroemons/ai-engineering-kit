---
{
  "id": "observability-prometheus-native-histogram-migration",
  "title": "Prometheus native histogram: 3.8/3.9 の明示設定と quantile 集約の移行",
  "kind": "knowledge",
  "technology": "observability",
  "version": "Prometheus 3.8.0 stabilization / 3.9 flag transition; rolling spec, query and config docs verified 2026-10-01",
  "tags": [
    "research-domain:quality-operations",
    "prometheus",
    "native-histogram",
    "classic-histogram",
    "histogram_quantile",
    "scrape_native_histograms",
    "migration"
  ],
  "sources": [
    {
      "id": "prometheus-native-histogram-spec-20261001",
      "url": "https://prometheus.io/docs/specs/native_histograms/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-histogram-functions-20261001",
      "url": "https://prometheus.io/docs/prometheus/latest/querying/functions/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-native-histogram-config-20261001",
      "url": "https://prometheus.io/docs/prometheus/latest/configuration/configuration/",
      "type": "official_docs"
    },
    {
      "id": "prometheus-native-stable-release-3-8-20261001",
      "url": "https://github.com/prometheus/prometheus/releases/tag/v3.8.0",
      "type": "release_notes"
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

# Prometheus native histogram への移行

## 調査した問いと確認した変更

native histogram を使う環境で、Prometheus の更新後にデータが消えたり fleet 全体の p95 が変わったりするのをどう防ぐか。負荷生成法や OpenTelemetry temporality ではなく、scrape 設定と PromQL の表現変更を扱う。

[3.8.0 release notes](https://github.com/prometheus/prometheus/releases/tag/v3.8.0) は Native Histograms の安定化を明記する。release title の日付は 2025-11-28、GitHub の公開は 2025-12-02。2026-10-01 時点の新リリースとして紹介するものではなく、既存 corpus にない破壊的な移行条件を再確認した。3.8 と 3.9 の契約、および取得時点の rolling docs を区別して読む。

## 公式資料で確認できたこと

### stable と自動収集は別の設定

[Native Histograms specification](https://prometheus.io/docs/specs/native_histograms/) と release notes では、3.8 で `scrape_native_histograms` が導入され、旧 `--enable-feature=native-histograms` は既定値を true にする移行効果だけを持つ。3.9 以降はその flag が no-op となり、native の scrape は明示設定が必要になる。remote write の native 送信は `send_native_histograms` を別に確認する。spec の v4 に関する既定値の記述を、v3 環境へ先取りして適用しない。

[Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/) の取得時点では global `scrape_native_histograms` は false が既定、job 側は global を継承する。`always_scrape_classic_histograms` は native と一緒に公開される classic 側などの追加収集を制御し、`convert_classic_histograms_to_nhcb` は classic を custom-bucket native histogram（NHCB）へ変換する別設定である。変換しても元データ以上に細かい観測値が復元されるわけではない。

同ページの `native_histogram_bucket_limit` は histogram ごとの正負 bucket 数の上限で、超過時は解像度を下げ、制限内に収められなければ scrape が失敗する。既定0は無制限。単にストレージ量を切り詰めるだけの無害な上限とは扱わない。

### quantile は histogram を集約してから求める

[Query functions](https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_quantile) のルールでは、classic は `_bucket` と `le` を使い、native は histogram sample 自体を使う。両者とも先に各系列の `rate` を求め、その後 `sum` し、最後に `histogram_quantile` を適用する。classic 集約では `le` を残す。`rate` より前に集約すると個別 counter の reset が隠れ得る。

以下は上記の契約に沿った独自の指標名・窓での式（PromQL 実行は未検証）。

- classic: `histogram_quantile(0.95, sum by (service, le) (rate(worker_request_seconds_bucket[5m])))`
- native: `histogram_quantile(0.95, sum by (service) (rate(worker_request_seconds[5m])))`

quantile は bucket 内の推定値である。classic / NHCB / zero bucket は線形補間、標準 exponential schema の非ゼロ bucket は指数補間を使う。`rate` の同じ range 内で float と native histogram が混在すると、その要素は warn-level annotation 付きで除外される。これは「観測0件」とは区別する。

## 移行の判断手順（独自の設計案）

1. まず対象 job の exporter、scrape、remote write、保存先、query frontend の対応版を列挙する。サーバ側 stable という表現だけで end-to-end 対応を確定しない
2. 3.8 の段階で対象 job に明示的な true/false を設定し、旧 flag を外した状態で欠損・error を確認してから3.9へ進む。global 継承に頼る場合も、実効設定を記録する
3. classic を比較用に保持するなら収集設定と記録ルールをセットで管理する。同じ観測を classic/native の両方で二重加算せず、別ルール名で件数・平均・分位値を並べる
4. p95 の単純平均を fleet p95 としない。負荷が異なる二つの instance の synthetic data を用意し、histogram 集約後の結果と個別 p95 の平均が違うことを確認する
5. bucket limit の縮小や NHCB 変換は精度の変更として扱う。SLO 閾値近くに観測が集中する場合の推定誤差と、scrape 失敗時の通知を同時に点検する
6. 移行終了はグラフが見えることだけで決めない。無負荷、restart、欠損、混在型の range を含む query の結果と annotations を確認し、rollback 時に classic の記録ルールが利用可能かも確認する

## 避ける使い方と未確認範囲

旧 flag だけを残して3.9でも有効だと考えること、classic の `le` を先に消すこと、annotation で落ちた系列を正常値ゼロと補完すること、変換だけで精度が増えたとみなすことを避ける。

この調査では exporter / remote storage / dashboard の実機互換試験、PromQL の実行、最適な bucket 数・誤差の許容量は未検証。検索 eval は文書への到達だけを確かめる。設定/APIの rolling page には固定の公開日がなく、将来の変更は再取得が必要。release_notes の TTL30日が最短なので期限は 2026-10-31、他の official_docs は90日である。

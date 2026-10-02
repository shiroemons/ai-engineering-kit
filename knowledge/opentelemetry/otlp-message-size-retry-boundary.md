---
{
  "id": "opentelemetry-otlp-message-size-retry-boundary",
  "title": "OTLP 1.11.0: メッセージサイズ上限と 413・RESOURCE_EXHAUSTED・partial_success の再送境界",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "OTLP Specification 1.11.0 / opentelemetry-proto v1.11.0 (published 2026-07-21), pinned to 790608c4d51e6ffc12210b541e8514cbed9e91a4; verified 2026-10-02 UTC",
  "tags": [
    "research-domain:quality-operations",
    "OTLP",
    "message-size",
    "compression",
    "RESOURCE_EXHAUSTED",
    "RetryInfo",
    "partial_success",
    "retry",
    "telemetry-loss"
  ],
  "sources": [
    {
      "id": "otlp-v111-spec-size-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/docs/specification.md",
      "type": "official_docs"
    },
    {
      "id": "otlp-v111-release-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-proto/releases/tag/v1.11.0",
      "type": "release_notes"
    },
    {
      "id": "otel-collector-resiliency-20261002",
      "url": "https://opentelemetry.io/docs/collector/resiliency/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# OTLP のメッセージサイズ上限と再送境界

## 問いと今回の追加理由

export 失敗時にキューと retry を増やしてよいのか。それとも送信前の payload 設計を変えるべきか。OTLP のサイズ制限、受理の可否、再送してよい条件を分けて判断する。

[opentelemetry-proto v1.11.0](https://github.com/open-telemetry/opentelemetry-proto/releases/tag/v1.11.0) は 2026-07-21 公開。HTTP body / gRPC message の request・response size 制限を仕様へ追記し、Retry-After が HTTP-date でもよいことを明確化した。これは**仕様文書の変更**であり、同日に全 SDK・Collector・backend の既定値が変わったという意味ではない。既存の [BatchSpanProcessor](batch-span-processor.md) は SDK 内の件数ベースの queue / batch と終了処理を扱い、本稿は exporter 以降の byte 数と配送失敗を補う。

## 確認した契約

以下の正本は [1.11.0 固定仕様](https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/docs/specification.md)。trace・metric・log は Stable、profiles は Development のため、ここでは前者を対象とする。MUST は必須、SHOULD / RECOMMENDED は推奨として区別する。

### 四つのサイズ境界

| 境界 | 規範上の制限 | 既定値についての記述 |
|---|---|---|
| client が request を作る | 圧縮前も制限する SHOULD。超過した request を送らない MUST NOT | HTTP・gRPC とも 64 MiB を推奨 |
| server が request を読む | 展開後も制限する MUST | 64 MiB 推奨。gRPC の典型的実装既定は 4 MiB と説明 |
| server が response を作る | 圧縮前も制限する MUST | 4 MiB を推奨 |
| client が response を読む | 展開後も制限する MUST。超過は非再送エラー | HTTP は 4 MiB 推奨。gRPC の典型的な 4 MiB は使用可 |

各上限の設定可能性は SHOULD。64 MiB は全実装共通の強制値でも相手との合意値でもない。根拠: [gRPC request / response](https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/docs/specification.md#otlpgrpc-request)、[HTTP request / response](https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/docs/specification.md#otlphttp-request)。

### 再送可否を error code だけで一括しない

| 観測した事象 | 1.11.0 の契約 |
|---|---|
| request が受信上限を超えた | HTTP は 413。gRPC は非再送の RESOURCE_EXHAUSTED |
| client が response の上限超過を検出 | 非再送。gRPC では同じ RESOURCE_EXHAUSTED が現れる |
| server の一時的な資源枯渇 | RESOURCE_EXHAUSTED は回復可能性を示す RetryInfo がある場合に限り再送可能と解釈する SHOULD。なければ非再送の SHOULD |
| HTTP エラー | 429 / 502 / 503 / 504 は再送の SHOULD。他の 4xx / 5xx は再送禁止の MUST NOT |
| partial_success が設定された応答 | HTTP 200 でも、その request は再送禁止の MUST NOT |
| HTTP 接続が応答なしで切れた | 同じ request を backoff 付きで再送する SHOULD。ただし重複の可能性が残る |

OTLP/gRPC の非再送エラーでは、同じ telemetry data を再送しない MUST NOT に加え、そのデータを破棄する MUST がある。HTTP は別途、再送許可リストにない 4xx / 5xx への request 再送を MUST NOT と定める。gRPC の明示的な MUST drop を transport 共通の文言として引用しない。サイズ超過として判明したものを、一般的な backpressure と読み替えない。HTTP 500 も OTLP の再送許可リストにはない。partial_success には拒否数が入り、拒否数ゼロで警告だけの場合もある。全成功と一部受理を HTTP status だけで区別しない。根拠: [gRPC failures](https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/docs/specification.md#failures)、[HTTP responses](https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/docs/specification.md#otlphttp-response)。

partial_success 応答が大きすぎる場合、server は意味を変えず診断文などを短縮する SHOULD。収まらなければ gRPC は非再送 RESOURCE_EXHAUSTED、HTTP は 500 で失敗させる MUST であり、データをどこまで受理したかは未規定である。したがって、応答失敗を「受理ゼロ」の証拠にもしない。

### queue は別の障害を吸収する

[Collector の Resiliency guide](https://opentelemetry.io/docs/collector/resiliency/) は、送信 queue の満杯、retry 期限、メモリ queue の再起動、永続 queue の disk 障害・容量不足を別々の喪失要因として挙げる。file_storage の WAL は再起動への対策であり、永続化後も配送が失敗し得る。queue_size / queue_capacity と送信失敗を監視する必要がある。

この情報と再送契約を組み合わせると、queue の拡大や WAL だけで恒久的なサイズ超過を直せるとはいえない。また OTLP の ACK は隣接 client/server の単位であり、多段経路全体の保存完了を保証しない。根拠: [OTLP の配送範囲と重複](https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/docs/specification.md#otlpgrpc)、[Duplicate Data](https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/docs/specification.md#duplicate-data)。

## 採用時の判断手順（独自の設計提案）

以下は公式設定例ではなく、上記契約から組み立てた運用案である。SDK / exporter / Collector の実装検証は未実施。

1. 各 hop の「送信側版・受信側版・transport・encoding・圧縮・request 上限・response 上限」を記録する。SDK → agent、agent → gateway、gateway → backend を別行にする。途中に proxy があれば、その制限と返す status も対象にする。仕様の 64 MiB を相手の実設定欄へ転記しない。
2. 送信前のサイズ予算を、実測した相手の許容値より小さく取る。件数だけの batch 上限では、巨大な属性や log body を含む1件を制約できない。実際の serializer で生成される圧縮前 bytes を測り、件数・bytes・単一 item の大きさを別々に監視する。平均的な span サイズだけで最大 request を保証しない。
3. 将来送るデータの batch 分割、不要な属性の抑制、生成側の長さ上限を検討する。変更で診断価値が失われないか確認する。すでに非再送と判定された payload を、queue から自動で拾って分割再送する仕組みを標準の retry として足さない。過去データの救済が必要なら、製品固有の重複処理と承認された手順を別途設計する。
4. retryable な一時障害だけに retry 時間・queue・永続化の予算を割り当てる。413 や判明したサイズ超過は設定・データ生成の問題として扱い、回数を増やすだけで回復させようとしない。RESOURCE_EXHAUSTED を一律 retry にする共通 interceptor は、発生箇所と RetryInfo を失わないよう点検する。
5. response 側も保護する。小さな request だから応答も小さいとは仮定しない。制限なしの response read や、長い診断文を無制限に蓄積する処理を避ける。障害調査ログには payload 全文ではなく、transport・制限値・実サイズ・status・拒否数・retry 判定を安全に残す。
6. 「送信試行済み」「隣の Collector に受理済み」「最終 backend で照合済み」を区別する。集計結果が改善した時点だけで欠落解消と判定しない。partial_success と非再送 drop は、接続エラーと別のアラート原因にする。

## 独自の反例で判断を確かめる

以下の数値は説明用であり実測ではない。

- request が圧縮前 8 MiB、圧縮後 1 MiB、receiver の展開後上限が 4 MiB なら、wire size が小さいことも仕様の 64 MiB 推奨値も受理の根拠にならない。sender の圧縮前サイズと receiver の実上限を合わせて見直す。
- 100件を送って partial_success の拒否数が3なら、HTTP 200 を100件成功へ変換しない。同じ100件の再送も行わない。拒否した個々の item を、件数だけから推測して復元しない。
- 応答が大きすぎて HTTP 500 になった場合、通常の Web API 用「5xx は全再送」方針を適用しない。受理したデータが存在するかもしれず、再送不能であることと、受理件数が不明なことを同時に記録する。
- request 上限を 4 MiB から 64 MiB に上げても、Collector の process memory を 64 MiB に抑えたことにはならない。並列 request・queue・serializer のコピー・デコード後の構造を含む実メモリを別に負荷試験する。

## 受け入れ試験案（すべて未実行）

採用予定の exporter / receiver / proxy の版と構成を固定し、隔離環境で小さな試験用上限を設定する。本番 backend への巨大 payload 投入は不要である。

- request の圧縮前サイズを configured limit の直下・同値・直上にする。HTTP と gRPC を分け、送信前検出と受信時検出、実送信回数、drop 記録を確認する。
- 圧縮後は制限未満、展開後は制限超過の payload を用い、gzip による回避が成立しないことを確かめる。境界値の serializer 差や request envelope の overhead も計測する。
- gRPC のサイズ超過 RESOURCE_EXHAUSTED と、一時枯渇の RetryInfo あり・なしを別 fixture にする。非再送が retry queue に戻らず、回復可能時は delay が尊重されることを確認する。
- HTTP 413 / 500 / 429 / 502 / 503 / 504 を返す stub を用意し、4xx と 5xx の一括分類がないことを検査する。retryable 応答の Retry-After は秒数と HTTP-date を分けて試す。
- response を圧縮前・展開後それぞれ上限超過にする。HTTP 200 の大きすぎる応答も含め、メモリ保護、非再送判定、body の読み過ぎ防止を確認する。
- partial_success の拒否数が正、拒否数ゼロで警告あり、全成功で field なしを別々に返す。request の再送回数と、受理数・拒否数の記録を照合する。
- receiver が受理した直後に接続を切る。HTTP の応答なし再送が起きても、backend の重複許容・集計への影響を測り、exactly-once と誤認しない。
- 一時的な downstream 停止と、恒久的なサイズ拒否を別々に注入する。queue の排出、retry 期限、再起動時の残存、最終 backend での照合まで確認する。

## 版・provenance・限界

- 仕様は v1.11.0 の full commit `790608c4d51e6ffc12210b541e8514cbed9e91a4` に固定。release API の published_at は `2026-07-21T13:54:02Z`。tag API と release commit の CHANGELOG でも対応を確認した。2026-10-02 UTC の live 仕様表示も 1.11.0 だったが、将来の最新性を主張しない。
- Collector resilience guide は版指定のない概説。表示最終変更日は 2026-01-14 で、見出し記法の変更。Collector の実装版・既定 queue 数・retry 設定の保証には使わない。
- 一次資料を独自に日本語で要約し、コード・設定例の転載はしていない。opentelemetry-proto は固定版 [LICENSE](https://github.com/open-telemetry/opentelemetry-proto/blob/790608c4d51e6ffc12210b541e8514cbed9e91a4/LICENSE) の Apache-2.0、Collector guide は OpenTelemetry website の [CC-BY-4.0](https://raw.githubusercontent.com/open-telemetry/opentelemetry.io/main/LICENSE) を確認した。ここでの表・反例・採用手順・試験案は独自の整理である。
- 未確認: 各言語 SDK / 各 Collector distribution の対応版と実既定値、設定 option 名、proxy の変換動作、server が受理済みデータを永続化するタイミング、backend の重複処理。規範の存在を実装済みの証拠にしない。
- 再確認期限は 2026-11-01。取得日 2026-10-02 に release_notes の30日 TTLを適用した最小期限であり、資料の日付だけを更新して延長しない。

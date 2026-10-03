---
{
  "id": "opentelemetry-collector-log-batch-oversize-survivor-boundary",
  "title": "Collector v0.162.0 の log 分割: 巨大 record の除去・後続保持・進捗なし終了",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "OpenTelemetry Collector v0.162.0 / v1.68.0 (2026-09-28), 62cdad2ea133239380b44d20d84eb26e114779b6; compared with v0.161.0 0bf928af5487d3c4e0b4174eabb7ba075c322517; source review only",
  "tags": [
    "research-domain:quality-operations",
    "Collector",
    "exporterhelper",
    "sending_queue",
    "batch",
    "oversized",
    "log record",
    "bytes",
    "resource",
    "scope",
    "data loss"
  ],
  "sources": [
    {
      "id": "otel-log-split-release-0162-20261003",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.162.0",
      "type": "release_notes"
    },
    {
      "id": "otel-log-split-config-0162-20261003",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/README.md",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-log-split-implementation-0162-20261003",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/logs_batch.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-log-split-tests-0162-20261003",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/logs_batch_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-log-split-dispatch-0162-20261003",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/partition_batcher.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-log-split-sizer-0162-20261003",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/sizer/logs_sizer.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-log-split-prior-0161-20261003",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/blob/0bf928af5487d3c4e0b4174eabb7ba075c322517/exporter/exporterhelper/internal/queuebatch/logs_batch.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-log-split-signal-scope-0162-20261003",
      "url": "https://github.com/open-telemetry/opentelemetry-collector/tree/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# 巨大な log と、その後ろの正常な log を別々に扱う

## 問い・変更日・既存知識との違い

Collector で1件の巨大な log が見つかったとき、同じ request の後続 record まで失うのか。2026-09-28 公開の [v0.162.0 / v1.68.0 release](https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.162.0) は、exporterhelper の batch 分割中に後続データを巻き込む不具合の修正を掲載し、適用は **logs のみ**と明記している。公開時刻は GitHub release API の 2026-09-28T14:09:31Z。ここでは release tag を解決した commit `62cdad2ea133239380b44d20d84eb26e114779b6` の実装を確認する。

既存の [OTLP メッセージサイズ・再送](otlp-message-size-retry-boundary.md) は、通信相手のサイズ拒否と retry 可否を扱う。本稿は送信前の Collector 内部で、分割結果にどの record が残るかを扱う。SDK の BatchSpanProcessor、ネットワークの 413、backend の保存成功とは別の段階である。

結論は「logs の巻き添えを減らす修正を採用し、捨てられた record と生き残った record を別々に検証する」。単一 record 自体を小さく変換する機能ではなく、すべての入力で巨大な1件だけを捨てるという無条件の保証でもない。

## どの設定がこの問題に関係するか

[固定版 exporterhelper README](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/README.md) では、sending queue の batching は既定で無効。`sending_queue.batch` を有効にし、`max_size` を正の値にすると batch splitting を使う。`max_size: 0` は上限なしであり、サイズ制限の導入ではない。正の `max_size` は `min_size` 以上にする。

今回の単一巨大 record の問題を評価する際は、`sending_queue.batch.sizer: bytes` を明示する。queue 本体の `sending_queue.sizer` / `queue_size` と、batch の `sizer` / `max_size` は別の制限である。queue を1000 request保持できても、1つの出力batchに巨大なrecordが入るとは限らない。

[LogsBytesSizer](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/sizer/logs_sizer.go) は `plog.ProtoMarshaler` を使う。一方 `items` は log record 数で、1件を1として数える。したがって body の文字数、body 単体のUTF-8長、圧縮後のHTTP転送量、件数上限を同じbytes予算とみなさない。ここで確認したのはhelper内部のprotobuf sizingであり、すべてのexporterが最終的に生成するwire形式の上限ではない。

## 固定実装で確認した保存・破棄の分岐

### 1. 旧版は空の抽出結果で残りを返さず終了した

比較対象の [v0.161.0 logs_batch.go](https://github.com/open-telemetry/opentelemetry-collector/blob/0bf928af5487d3c4e0b4174eabb7ba075c322517/exporter/exporterhelper/internal/queuebatch/logs_batch.go) は、上限に収まる部分を取り出す `extractLogs` が0 recordを返すと、そこでerrorと既に取り出したbatchだけを返す。未抽出の残りrequestは結果へ加わらない。

この経路では、先に取り出せた record は残っても、巨大なrecordに続く、単独なら収まるrecordまで結果から失われ得る。「drop件数が巨大record数より多い」「入力順で欠落が変わる」という問題を、backendの拒否だけで説明しない。

### 2. v0.162.0 は「今の残容量」と「空batchでも入らない」を分ける

[新しい logs_batch.go](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/logs_batch.go) の `extractScopeLogs` は、resourceとscopeのheader・attributesおよびprotobufの長さ情報の分を考慮して、空batch側の `maxRecordSize` と現在の `capacityLeft` を区別する。

- 単独のbatchにも入らないと判定したrecordは除去し、後ろを引き続き調べる
- 単独なら入るが現在のbatchの残容量に入らないrecordは、その場で捨てず、後の分割へ残す
- 抽出後はsize cacheを更新し、除去されたrecord数を計算する。recordを含む分割結果を返しつつ、dropがあればerrorも返す

bodyだけが小さくても、付随する巨大なresource属性やscope属性を含めると入らない。recordのbodyを分割・切り詰めたり、その属性を自動で短縮したりする経路ではない。resource/scopeを含む実際の構造で試験する必要がある。

### 3. error があることと、全batchを送らないことは同義ではない

[partition_batcher.go](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/partition_batcher.go) は、`MergeSplit` のerrorを警告として記録し、残ったrequest listがあれば後続処理を続ける。error分の完了通知と各分割結果の完了を別に数えて扱う。

したがってsplit警告を見て「全部失敗」と決めつけることも、backendに数件届いたことから「全部成功」と判断することもできない。本稿はこの呼出し先までの観察であり、receiverが返す最終ステータス、利用SDKの再送、保存先のdeduplicationまでは検証していない。

### 4. 進捗なしの防御終了は残る

新しい `split` でも `removedSize == 0` なら終了し、それまでの結果だけを返す。errorに含めるdrop数は、既に除去した分と残りrequest内のrecord数を合わせる。これは無限loopを避ける分岐であり、「正常なrecordを必ず探して全件救う」という契約ではない。

[同版の upstream tests](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch/logs_batch_test.go) の `TestMergeSplitLogsStopsWhenNoProgressIsPossible` は、resourceだけで `max_size` をちょうど使い切り、その下に短いrecordを2件加える。期待値は分割結果なし、進捗なしerror、drop数2である。単一recordのbodyが短いことは保持の十分条件にならない。

同じテストファイルは、recordを持たない巨大resourceを除いた後に正常なrecordを処理できるケースも確認している。この場合はrecord欠落がないためerrorを要求しない。空の枠組みを除いたことと、業務上のlogを失ったことを同じdrop件数にしない。

### 5. 同じreleaseの traces / metrics へ一般化しない

[同commitの queuebatch ディレクトリ](https://github.com/open-telemetry/opentelemetry-collector/tree/62cdad2ea133239380b44d20d84eb26e114779b6/exporter/exporterhelper/internal/queuebatch) にある `traces_batch.go` と `metrics_batch.go` は、抽出が0 span / 0 data pointなら既存の結果だけを返す早期終了を残している。releaseのlogs限定という記載と一致する。

よってv0.162.0への更新だけで「traceの巨大span」「metricの巨大data point」に同じ改善があるとは報告しない。ここで比較した旧版はv0.161.0に限り、すべての旧版、vendor独自patch、後続releaseの状態は確認していない。

## 採用・診断の手順（独自の運用提案）

1. 稼働しているCollector distributionと実際に含まれるexporterhelper版を確かめる。image名だけで修正を含むと判断せず、logs pipelineが当該batch分割経路を使うか確認する
2. 対象を `logs + batch有効 + 正のmax_size + bytes` として再現条件を記録する。変更前後でsizer・max_size・入力・resource/scope構造を固定し、設定変更による改善と実装修正の効果を分ける
3. syntheticな一意IDを各recordへ付け、巨大recordの前後で保持対象のID集合を比較する。送信されたbatch数やdrop警告の有無だけを合格条件にしない。入力順を先頭・中間・末尾へ変えると、以前の巻き添えを検出しやすい
4. giant recordの取り扱いを別途決める。不要な巨大属性の生成抑制、必要な診断内容を失わないサイズ設計、別保管への参照化などを利用側で検討する。修正による後続保持を、その巨大record自体の保存策として扱わない
5. 上限を外すとこのsplit経路を避けられるが、出力先の拒否を解決したことにはならない。`max_size: 0` や `items` への変更を、損失対策として無試験で採用しない
6. 更新完了の条件は「正常recordの保持」「意図したdropの可視化」「batch上限」「backend到達」を別々に置く。no-progress警告と単一recordのdrop警告も区別する。欠落済みの過去logが更新によって復元されるとは期待しない

## 受け入れケースと証拠の強さ

以下は採用先で行う検証案であり、Collector実機や通信試験の実行結果ではない。upstream testを読んだ事実と、自分の環境で合格した事実を区別する。

| ケース | 確認する結果 |
|---|---|
| 通常logだけで上限の前後を跨ぐ | 正常ID集合が維持され、出力batchが実効上限以下になる |
| 巨大1件を先頭・中間・末尾へ配置 | 巨大recordのdropを検出し、保持可能な前後のIDが残る |
| 巨大recordを複数・全件にする | drop件数と残存ID集合を照合し、全件巨大なら空結果を成功保存扱いしない |
| 別resource・別scopeにも正常recordを置く | 同じscope内のbodyだけの試験で見逃す巻き添えを検出する |
| recordなしの巨大resourceが先行する | 枠組みの除去後も正常recordが残り、recordのdropを誤計上しない |
| resourceだけで上限を使い切る | 進捗なしで停止し、recordが救われない結果を明示する |
| items / bytes、max_size=0を比較する | 合否の意味が変わることを記録し、通信先の上限は別試験にする |

upstreamの `TestMergeSplitLogsDropsOnlyOversizedRecord` は複数の位置と複数batchの後続を含み、出力サイズ上限と `ProtoMarshaler` による再計算値・cached sizeの一致もassertしている。本調査ではこのテストを実行しておらず、その存在だけから採用先の構成や性能を保証しない。

## 取得・ライセンス・未確認事項

取得は2026-10-03 UTC。release本文とtag版READMEはnative webで開いた。固定commitのGitHub HTML/raw表示はcache missになったため、同commitの公式raw URLをread-onlyで取得した。v0.162.0のannotated tagから上記commitへの参照、v0.161.0のcommitと公開日もGitHub RESTで照合した。取得できなかったHTMLを読了扱いにはしていない。

実装・テストのSPDX headerと同commitの[LICENSE](https://github.com/open-telemetry/opentelemetry-collector/blob/62cdad2ea133239380b44d20d84eb26e114779b6/LICENSE)はApache-2.0。release proseの個別ライセンスは未確認としてunknownを記録した。内容は帰属付きの独自要約で、コードの転載・改変・依存追加・module昇格はしていない。release_notesのTTL30日を最短期限として2026-11-02に再確認する。

未確認は、各distributionの組込み版、custom exporterの対応、実際の性能、圧縮・JSON出力・proxyを含むwire上限、最終ACKとretry/重複、persistent queue再起動との組合せ、no-progress後の複雑な混在入力、profilesへの適用である。検索evalは本稿への到達性だけを確かめ、データ保持のruntime試験を代替しない。

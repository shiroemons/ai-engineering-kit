---
{
  "id": "opentelemetry-hostmetrics-process-rc-dual-emission-boundary",
  "title": "OpenTelemetry hostmetrics process RC: 同名dual emission・収集guard・global gateの比較境界",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "Contrib v0.163.0 at df849b5403c041d5e799e7846392a6f18b0b55ae; process metadata sem_conv_version 1.43.0; v0.160.0 introduction and 2026-10-13 announcement verified 2026-10-23 UTC; runtime not tested",
  "tags": [
    "research-domain:quality-operations",
    "OpenTelemetry",
    "hostmetrics",
    "process",
    "semantic-conventions",
    "versioned-metrics",
    "dual-emission",
    "feature-gate",
    "global-registry",
    "migration",
    "name_override"
  ],
  "sources": [
    {
      "id": "otel-hostmetrics-process-announcement-20261023",
      "url": "https://opentelemetry.io/blog/2026/hostmetrics-process-scraper-rc/",
      "type": "maintainer_article"
    },
    {
      "id": "otel-hostmetrics-process-release-0160-20261023",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.160.0",
      "type": "release_notes"
    },
    {
      "id": "otel-hostmetrics-process-release-0163-20261023",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.163.0",
      "type": "release_notes"
    },
    {
      "id": "otel-hostmetrics-process-reference-0163-20261023",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/documentation.md",
      "type": "official_docs"
    },
    {
      "id": "otel-hostmetrics-process-implementation-0163-20261023",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# OpenTelemetry hostmetrics process RC と dual emission の比較境界

## 調査した問いと結論

process scraper の新しい semantic conventions を試すとき、旧 dashboard を維持しながら、新旧の同じ観測値を別々に比較できるか。**dual emission は「同じ metric の旧版と新版を常に2本出す」契約ではない**。同じ emitted name で属性が変わる metric は、RC 側の1本に新旧属性を持たせる。別名へ変わる metric は2本出る場合があるため、出力本数も集計方法も metric ごとに確認する。

さらに feature gate は Collector process 内の global registry に登録される。receiver instance や pipeline を2つ作っても、その gate の legacy / RC 比較群を隔離できない。ここを誤ると、「control も同じスキーマになった比較」「設定上は enabled なのにデータがない」「旧名と新名の二重加算」が起きる。

対象は hostmetrics receiver が生成する **process metrics**。既存の [k8sattributes の resource 属性移行](k8sattributes-v1-semconv-migration.md)、[metric temporality](metric-temporality-reset-conversion.md)、[filelog checkpoint](filelog-checkpoint-delivery-boundary.md) とは別の境界を扱う。アプリケーション SDK が生成した metrics の一括変換や、hostmetrics 全 scraper の stable 化は対象外。

## 新しさ・版・日付を分ける

- [Contrib v0.160.0 release][release160] は GitHub API の published_at が **2026-09-02T16:36:04Z**。process RC を選ぶ2つの gate と、receiver-level gate の追加を記載する。2026-10-13に初めて公開された機能、と日付を取り違えない。
- Dónal O’Sullivan による [2026-10-13の解説][announcement] は、versioned metrics を実装した経緯と移行時の衝突処理を説明している。公開日が新しい一次記事として調べ、本文の適用版は以下の固定実装で照合した。
- [Contrib v1.2.0/v0.163.0 release][release163] は **2026-10-14T06:53:39Z** 公開。tag の annotated object を辿り、commit **df849b5403c041d5e799e7846392a6f18b0b55ae** を確認した。本書の実装分析はこの commit に限定する。
- [metadata.yaml][metadata] の sem_conv_version は **1.43.0**、process scraper の metrics stability は beta、移行 gate は alpha。metric key の **@v1 は metadata 内の世代識別子**であり、Collector binary v1.0、SemConv 1.0、全 hostmetrics の stable を意味しない。
- gate metadata の from_version は scraper 側が v0.158.0、receiver 側が v0.160.0。これは登録 metadata の値であり、本書では配布版の提供開始を release v0.160.0 で確認した。値だけから v0.158.0 binary に同一機能があったとは推定しない。

v0.163.0 でも対象 gate が alpha であることを固定ファイルで確認した。将来の beta 昇格・既定値変更・削除版は予測しない。記事の RC 試行期間の方針を、確定した stable 公開日として運用計画へ入れない。

## 1. 設定 key と emitted name を同じものにしない

[生成 reference][reference] は、たとえば設定上の process.cpu.time@v1 の **Emitted Name を process.cpu.time** と記載する。generated_metrics.go の init も同じ名前を設定する。したがって `process.cpu.time@v1` が OTel 出力に現れないこと自体は欠落ではない。

これは exporter 前の OTel metric name の話である。Prometheus などへの翻訳後の名前・unit suffix・label の表記まで同じになるとは限らず、backend 側の名前は実際の exporter 経由で別途確認する。

[metadata][metadata] と [generated_metrics.go][builder] で確認した主要な差分は次の通り。表の「旧属性」は metadata 内部の記号でなく、OTel datapoint に出るキーを示す。

| OTel metric / 変更前 | RC 側の OTel 出力 | 注意点 |
|---|---|---|
| process.cpu.time / process.cpu.utilization の state | cpu.mode | user / system は同じだが、Linux の wait は iowait へ対応する |
| process.disk.io の direction | disk.io.direction | read / write の値を保持する |
| process.context_switches の type、unit {count} | process.context_switch.type、unit {context_switch} | metadata の context_switch_type は name_override により旧出力では type になる |
| process.paging.faults の type、unit {faults} | system.paging.fault.type、unit {fault} | metadata の paging_fault_type も旧出力では type になる |
| process.threads | process.thread.count、unit {thread} | @v1 を除いても metric 名自体が変わる |
| process.handles | process.windows.handle.count、unit {handle} | Windows 対象の metric |
| process.open_file_descriptors | process.unix.file_descriptor.count、unit {file_descriptor} | OS ごとの実際の出力を別途試験する |

Linux の state=wait と cpu.mode=iowait の対応は [process_scraper_linux.go][linux] の同じ Record 呼出しの引数で確認した。「属性名だけ置換して値の wait はそのまま」という query 移行では一致しない。

また、公開解説で用いられる process.owner → process.owner.name の例は一般的な衝突説明である。本 revision の [resource reference][reference] は process.owner を列挙しており、この process metric gate だけで resource 属性がその例の通り改名されるとは扱わない。

## 2. dual emission で何本になるか

以下は既存の metric が有効で、RC target を明示的に無効にしていない通常の移行経路。EmitV1 は scraper.process.EmitV1SystemConventions、DontEmitV0 は scraper.process.DontEmitV0SystemConventions を指す。

| EmitV1 | DontEmitV0 | 同名の CPU metric | 別名になる process.threads |
|---|---|---|---|
| false | false | 旧 metric、state のみ | process.threads のみ |
| true | false | RC metric 1本に cpu.mode と state | process.threads と process.thread.count |
| true | true | RC metric 1本、cpu.mode のみ | process.thread.count のみ |

根拠は [builder の衝突処理と Record メソッド][builder]、および [TestVersionedMetrics][builder-tests] の legacy_only / dual_emission / new_only ケース。CPU time / utilization、context switches、disk IO、paging faults は、新旧の emitted name が衝突し、属性が違うと legacy builder 側を無効にする。DontEmitV0 が false の間は RC datapoint に legacy 属性も渡す。

したがって、dual 中に state で絞った query と cpu.mode で絞った query が同じ値を返しても、独立した2種類の収集経路の一致を証明しない。**同じ1本を2つの属性名で参照しているだけ**の場合がある。一方、process.threads と process.thread.count を足すのは同じ測定値の二重加算になり得る。

type や unit も旧 consumer の互換性の一部である。同名の context switches は RC unit {context_switch} を持つ1本になり、旧 unit {count} のコピーが別に残るわけではない。今回の process CPU metric を「Gauge から UpDownCounter へ変更」とは主張しない。CPU utilization は両方 Gauge、CPU time は両方 cumulative monotonic Sum である。

EmitV1=false / DontEmitV0=true という「新側を出さず旧側も止める」組は移行手順から除外する。今回の静的調査だけで全 distro がその組を確実に起動時拒否すると保証しない。設定レビューと必須 metric の出力確認で防ぐ。

## 3. enabled と gate と実際の scrape は別条件

[generated_config.go][config] と [builder][builder] には次の条件がある。

1. EmitV1=true のとき、legacy metric が enabled で、RC target の enabled が利用者に明示されていなければ、target を自動で有効にする。optional metric を gate だけで全種類有効にする処理ではない。
2. RC target に enabled=false を明示していた場合は enabledSetByUser が立つため、上の自動有効化が行われない。生成テンプレートなどが全 target に false を埋めると、「省略」と違う。
3. RC target を enabled=true にしても EmitV1=false なら、builder は warning を出してその target を無効にする。設定が parse できたことを出力成功にしない。
4. scraper は builder と別に収集の guard を持つ。[scrapeAndAppendThreadsMetrics][scraper] は legacy の ProcessThreads.Enabled を見て、false なら値を読む前に return する。

4の具体的な静的反例は、旧 process.threads を無効のまま、新 process.thread.count@v1 だけを有効にする変更である。新 key が config に存在していても、固定実装のこの経路では scrape 呼出しに進まない。**legacy の収集設定を残し、gate で出力世代を切り替える**のを出発点にする。RC-only にしたいからと旧 enabled を一括 false にするのは避ける。

これは v0.163.0 の実装読解であり、全 optional metric・全 OS・将来版に一般化した実行結果ではない。特に process.cpu.utilization は既定で無効で、[reference][reference] は初回 scrape に datapoint を出さないと説明する。受入試験は1回の空結果で不合格にも合格にもせず、必要な metric の有効化と複数回の観測を確認する。

## 4. receiver を分けても gate の対照群にはならない

[hostmetrics factory.go][factory] は receiver.hostmetrics.EmitV1SystemConventions が有効なら、featuregate.GlobalRegistry().Set で scraper.process.EmitV1SystemConventions を true にする。DontEmitV0 側も同様。ログには当該 receiver だけでなく Collector process 全体の process scraper に影響することが明記される。

[factory test][factory-tests] は receiver gate によって scraper gate が有効になることを検査する。scraper gate 自身も [生成 gate 登録][gates] で global registry を使う。従って次のように切り分ける。

- hostmetrics/legacy と hostmetrics/rc の receiver ID を作り、pipeline を分けるだけでは feature gate の独立性は得られない。
- receiver-level gate が有効なら、scraper-level gate を false にしたつもりでも receiver 作成時に true にされる。rollout / rollback は **receiver と scraper の両方の gate** を棚卸しする。
- legacy-only と RC-only を比較する設計は、別 Collector process を比較単位にする。同じ binary revision・scrape 対象・周期・権限を揃え、結果は別 dataset または識別可能な保存先へ送る。

最後の項目は独自の運用提案であり、hostmetrics が統計的な A/B 比較を保証するという意味ではない。別 process が同一瞬間に同じ値を観測する保証はなく、PID の変化、process の寿命、CPU の差分計算、scrape 時刻差もある。2組の telemetry を本番集計へ無差別に流し、二重収集したものを性能変化と誤認しない。

receiver gate の説明にある「all scrapers」から、すべての system.* metrics がこの版で RC/stable に移行したと広げない。本書で確認した factory の委譲先と実装分析は process scraper である。

## 移行を決める手順（独自の運用提案）

1. **実体を記録する**: Collector / vendor distro の binary、image digest、起動引数、receiver-level と scraper-level の gate、metrics enabled の明示値、OS を残す。@v1 key の有無だけでは世代を判定しない。
2. **consumer の依存を列挙する**: metric 名、OTel unit、type、monotonicity、属性キーと値、resource の分割、exporter 後の名称を比較項目にする。query、recording rule、alert、dashboard、保存コストへどの項目が影響するかを対応付ける。
3. **比較と互換維持を分ける**: dual emission は旧 query を動かしながら新 query を準備する段階。独立した legacy / RC 比較が必要なら別 process を使い、同名2本が出るという仮定を捨てる。
4. **RC-only を別に試す**: dual 中に旧属性が残るためだけに通っていた旧 query を発見する。新 query が必須 process 群を拾い、名前が変わる metric を二重加算せず、必要な optional metric が欠けないことを確認する。
5. **戻し方を先に決める**: 今回の alpha gate が存在する binary では、receiver / scraper 両階層の上書きを含めて legacy 出力へ戻す候補設定を用意する。保存済みの RC データは gate を戻しても変換されないため、履歴 query の互換期間を別に持つ。

## 受入試験の観点と証拠の限界

以下は実行すべき試験案であり、この調査では未実施。

| 条件 | 観測すること | 防ぐ誤判定 |
|---|---|---|
| Linux CPU、dual | OTel 名に @v1 がなく、1本に state=wait と cpu.mode=iowait が対応する | suffix 不在を欠落、同じ値を独立比較と扱う |
| context switches、dual | unit {context_switch} と新属性、legacy の type 属性 | metadata の context_switch_type を出力キーだと思う |
| thread count、dual → RC-only | 別名の2本から新名のみに移る | 両名を足して2倍にする |
| RC target の enabled を省略 / false 明示 | 自動有効化の差と必須 metric の有無 | default false と明示 false を同一視する |
| legacy threads=false、RC target=true | 旧 guard が収集を止める条件を再現する | config の存在を実データと同一視する |
| receiver gate=true、scraper gate=false | global gate の最終状態と出力 | receiver/pipeline 名を隔離境界と扱う |
| CPU utilization の初回 / 2回目以降 | 差分計算に必要な観測後の出力 | 初回空を通常時の無監視と混同する |
| exporter / backend を通す | translated name、unit、labels、query と alert の結果 | OTel 出力の確認だけで consumer 互換を宣言する |

上流の TestVersionedMetrics は builder を直接呼び、固定値を record して emitted metrics を調べる。これは OS の値取得や legacy enabled の scrape guard、全 exporter / backend の動作を通した試験ではない。また、factory test の gate assertion は複数 process の隔離実験ではない。本書では **上流 test を読んだが実行していない**。追加する検索 eval は文書の発見性を検査するもので、Collector runtime 合格の証拠ではない。

## 未確認事項・provenance・再確認期限

- vendor distro、Helm chart、個別 backend の変換・同名衝突処理、OS 別の process 取得権限、process 数による性能・コスト、全 optional metric の組合せは未確認。Collector binary / コンテナは起動していない。
- receiver-level gate の将来の他 scraper への委譲、stable 化・既定変更・旧 gate の終了時期は未確認。記事公開日や名前の v1 だけから保証しない。
- native web で告知、v0.160.0 / v0.163.0 release 本文を開いた。固定 raw URL の一部は Cache miss だったため、GitHub connector で同じ40桁 commit の reference、metadata、実装、test を取得して照合した。存在しない processscraper/README.md からは根拠を採らず、実在する documentation.md と receiver README を確認した。
- repository は同 commit の [LICENSE][contrib-license] で Apache-2.0 を確認し、手書き実装の SPDX も照合した。generated files は同 repository の LICENSE に基づく。コードの転載・改変・module 昇格はしていない。
- 告知は Dónal O’Sullivan 著、記事更新 commit 83c4047e5396d66375137c7bd1952bd45285b4ef の [サイト LICENSE][site-license] は CC-BY-4.0。ここでは日本語で独自に要約し、固定実装の分析と運用提案を追加した。release 本文固有のライセンスは unknown とし、原文の長文・コードは転載していない。

取得日 **2026-10-23 UTC**。release_notes の TTL 30日が最短のため、再確認期限は **2026-11-22**。本書は確認済み revision の記録であり、後続 release を自動的に包含しない。

[announcement]: https://opentelemetry.io/blog/2026/hostmetrics-process-scraper-rc/
[release160]: https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.160.0
[release163]: https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.163.0
[reference]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/documentation.md
[metadata]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/metadata.yaml
[builder]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata/generated_metrics.go
[config]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata/generated_config.go
[builder-tests]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata/generated_metrics_test.go
[scraper]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/process_scraper.go
[linux]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/process_scraper_linux.go
[factory]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/factory.go
[factory-tests]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/factory_test.go
[gates]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata/generated_feature_gates.go
[contrib-license]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/df849b5403c041d5e799e7846392a6f18b0b55ae/LICENSE
[site-license]: https://github.com/open-telemetry/opentelemetry.io/blob/83c4047e5396d66375137c7bd1952bd45285b4ef/LICENSE

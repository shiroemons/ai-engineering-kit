---
{
  "id": "linux-cgroup-v2-cpu-throttling-ancestor-accounting",
  "title": "Linux cgroup v2 CPU throttling: cpu.stat・cpu.stat.local・祖先 quota と PSI の診断境界",
  "kind": "knowledge",
  "technology": "linux",
  "version": "Linux cgroup v2 / fair-class CPU bandwidth; live official docs verified 2026-10-03; source snapshot e767a4ea70a3992c37ed604157d32f0dfbf9b1e3 (Makefile 7.3.0-rc5); documentation clarification 171569f8 (2026-06-29), interface introduction 677ea015 (2023-07-13); runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "linux",
    "cgroup-v2",
    "cpu.stat",
    "cpu.stat.local",
    "cpu.max",
    "cpu.pressure",
    "CFS",
    "PSI",
    "throttled_usec",
    "ancestor-quota"
  ],
  "sources": [
    {
      "id": "linux-cgroup-v2-cpu-accounting-docs-20261003",
      "url": "https://docs.kernel.org/admin-guide/cgroup-v2.html",
      "type": "official_docs"
    },
    {
      "id": "linux-psi-cpu-observation-docs-20261003",
      "url": "https://docs.kernel.org/accounting/psi.html",
      "type": "official_docs"
    },
    {
      "id": "linux-cfs-bandwidth-hierarchy-docs-20261003",
      "url": "https://docs.kernel.org/scheduler/sched-bwc.html",
      "type": "official_docs"
    },
    {
      "id": "linux-cpu-stat-scope-clarification-171569f8-20261003",
      "url": "https://github.com/torvalds/linux/commit/171569f8ee6724a4113a0100fea6ff83d9b70c6a",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-cpu-stat-local-introduction-677ea015-20261003",
      "url": "https://github.com/torvalds/linux/commit/677ea015f231aa38b3972aa7be54ecd2637e99fd",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-cpu-stat-snapshot-e767a4ea-20261003",
      "url": "https://github.com/torvalds/linux/blob/e767a4ea70a3992c37ed604157d32f0dfbf9b1e3/kernel/sched/core.c",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Linux cgroup v2 CPU throttling: cpu.stat・cpu.stat.local・祖先 quota と PSI の診断境界

## 解く問いと結論

ホスト全体の CPU 使用率に余裕があり、子 cgroup の `cpu.max` が `max`、`cpu.stat` の `nr_throttled` も増えていないのに、サービスの実行が CPU 制限で止まることはあるか。そのとき、どの階層の何を観測すればよいか。

子の quota だけでは制限を否定できない。祖先の帯域予算を使い切ると子も実行を待たされる。診断では、使用量、制限を発動した階層、その階層から影響を受けた runqueue、PSI の停滞時間を分ける。`cpu.stat.local` の local は「この cgroup 自身が設定した quota だけ」という意味ではない。逆に、`cpu.stat` のすべてのキーが同じ階層集計になるわけでもない。[cgroup v2](https://docs.kernel.org/admin-guide/cgroup-v2.html#cpu-interface-files) / [階層的な CFS 帯域制御](https://docs.kernel.org/scheduler/sched-bwc.html#hierarchical-considerations)

既存の Docker メモリ上限、Kubernetes MemoryQoS、systemd sandboxing の文書とは別の CPU 診断上の問いである。以下の契約は upstream の確認範囲、後半の診断順序と判断例は独自の設計案として記録する。

## 調査版と変更の意味

2026-06-29 の [文書修正 171569f8](https://github.com/torvalds/linux/commit/171569f8ee6724a4113a0100fea6ff83d9b70c6a) は、`cpu.stat` と `cpu.stat.local` の集計範囲を明示した。変更ファイルは cgroup v2 文書だけで、CPU 制限方式を変更する実装差分ではない。インターフェースの導入自体は [2023-07-13 の 677ea015](https://github.com/torvalds/linux/commit/677ea015f231aa38b3972aa7be54ecd2637e99fd) に確認できる。「2026 年に初めて使えるようになった機能」とは扱わない。

現行説明は 2026-10-03 UTC に公式ページを開き、`e767a4ea70a3992c37ed604157d32f0dfbf9b1e3` の文書と実装を照合した。固定した [Makefile](https://github.com/torvalds/linux/blob/e767a4ea70a3992c37ed604157d32f0dfbf9b1e3/Makefile) の版は 7.3.0-rc5 であり、これは検証した開発 snapshot の識別である。安定版の最新性、当該 rc への更新推奨、全ディストリビューションでの利用可能性を示さない。導入 commit を最初に含む release、各 vendor の backport は未確認なので、対象ホストでファイル・キー・設定を確認する。

## 確認した契約

### 1. 制限元と使用量の集計を混同しない

公式の [CPU Interface Files](https://docs.kernel.org/admin-guide/cgroup-v2.html#cpu-interface-files) と [固定版文書](https://github.com/torvalds/linux/blob/e767a4ea70a3992c37ed604157d32f0dfbf9b1e3/Documentation/admin-guide/cgroup-v2.rst) では、次の範囲が区別される。

- `cpu.stat` の `usage_usec`、`user_usec`、`system_usec` は、その cgroup と子孫の使用量を含む。ファイルは CPU controller の有効・無効にかかわらず存在する
- `nr_periods`、`nr_throttled`、`throttled_usec`、`nr_bursts`、`burst_usec` は controller が有効な場合の追加項目で、fair-class の帯域統計である。この5項目は自身の帯域制限に対応し、祖先から受けた throttling を階層集計しない
- `cpu.stat.local` の `throttled_usec` は自身の runqueue が受けた throttling を表し、祖先の制限による停止も含み得る。controller が無効な場合はこのキーを報告しない。ファイルの存在と測定値の存在は別である

したがって、子の `nr_throttled` 差分がゼロであることは、祖先 quota による停止がゼロであることを証明しない。また、親子双方の `usage_usec` を無条件に足すと、子孫の使用量を二重計上する。観測対象が「制限元」か「影響先」かをメトリクス名だけで判断しない。

### 2. cpu.max は帯域の設定であり、PSI の閾値ではない

cgroup v2 の `cpu.max` はマイクロ秒単位の `MAX PERIOD` で、`max` はそのファイルでの帯域上限なしを表す。祖先の制限を解除する値ではない。CPU controller は階層的に資源を制限するため、子が上位の設定を緩めることはできない。[cgroup v2 の構造と cpu.max](https://docs.kernel.org/admin-guide/cgroup-v2.html)

CFS Bandwidth Control 文書は、自身の quota 枯渇と親の quota 枯渇を別の停止経路として説明する。後者では、子に実行予算が残っていても親の予算が補充されるまで実行できない。この説明を fair-class の帯域制御の背景として使う。同ページの Management 節にある `cpu.cfs_quota_us` などは cgroup v1 の名前なので、v2 の操作手順へコピーしない。同じ文書にある `throttled_time` はナノ秒、v2 の `throttled_usec` はマイクロ秒である。[CFS Bandwidth Control](https://docs.kernel.org/scheduler/sched-bwc.html)

ホスト使用率が低くても、制限された一部の workload の予算は尽き得る。逆に、quota に達したという情報だけでは、全ホストの CPU が足りないとも、各リクエストの遅延が quota だけで説明できるともいえない。

### 3. cpu.stat.local は単一の wall-clock 停止率ではない

固定 snapshot の [kernel/sched/core.c](https://github.com/torvalds/linux/blob/e767a4ea70a3992c37ed604157d32f0dfbf9b1e3/kernel/sched/core.c) を読むと、`cpu_extra_stat_show` は対象 task group の `cfs_bandwidth` 統計を表示し、`cpu_local_stat_show` は `throttled_time_self` を使う。後者は各 possible CPU の runqueue の累積時間を合計し、ナノ秒からマイクロ秒に変換する。これは固定実装の観察であり、全 kernel 版の内部実装を保証する記述ではない。

この合計を、経過した wall-clock 時間だけで割って「停止した時間の割合、上限100%」と決めつけない。同時に複数の runqueue が停止する場合を区別できないからである。`nr_throttled / nr_periods` も回数ベースの比であり、リクエストの遅延割合への変換式ではない。ここで不足しているのは単位変換ではなく、集計対象と分母の定義である。

さらに [導入 commit の説明](https://github.com/torvalds/linux/commit/677ea015f231aa38b3972aa7be54ecd2637e99fd) は、親の停止中ずっと runnable でなかった子には self-throttling time が現れない例を挙げる。祖先の設定が存在するだけで、全子に常に同じ停止時間が加算されるという読み方も避ける。

### 4. PSI は資源待ちの観測であり、CPU 使用率ではない

[PSI の Pressure interface](https://docs.kernel.org/accounting/psi.html#pressure-interface) は `some` を少なくとも一部の task が資源待ちになった時間、`full` を non-idle task が同時にすべて待った時間として扱う。`avg10`、`avg60`、`avg300` は各時間幅の傾向を示す百分率、`total` は累積マイクロ秒である。CPU を実行できた時間や quota の残量を表す値ではない。

特に system-wide の `/proc/pressure/cpu` の `full` は意味が定義されておらず、5.13 以降は互換性のためゼロを表示する。これを「ホストには CPU の問題がない」と読むことはできない。cgroup の `cpu.pressure` と system-wide の観測範囲も分ける。[PSI の cgroup2 interface](https://docs.kernel.org/accounting/psi.html#cgroup2-interface)

`cgroup.pressure=0` による PSI accounting の無効化はその cgroup 単位であり、子孫に継承されない。対象 cgroup の停止したカウンタを、子孫全体の停滞ゼロとして扱わない。[cgroup.pressure](https://docs.kernel.org/admin-guide/cgroup-v2.html#core-interface-files)

## 診断手順と判断例（独自の設計案）

以下は公式が指定する監視システムや閾値ではなく、上記の契約を取り違えないための手順である。対象 workload は fair-class とし、RT や BPF scheduler の独自動作まで一般化しない。

1. **対象と観測範囲を固定する。** 対象プロセス、cgroup パス、観測側の namespace、kernel 版、CPU controller が対象で利用可能かを記録する。コンテナ内から見える階層がホスト上の全祖先とは限らないので、祖先を読めない場合は「制限なし」ではなく「上位未確認」とする。対象の `cgroup.subtree_control` だけを見て、その cgroup 自身が制御対象かを即断しない
2. **変更前に同じ時間帯の差分を取る。** 子と読める祖先について `cpu.max`、帯域統計、子の `cpu.stat.local`、`cpu.pressure`、ホストの CPU 観測、サービスの遅延と待ち行列を保存する。累積値の大小より、同一測定区間の差分を優先する。cgroup の作り直し・PID 移動・収集間隔のずれは別イベントとして残す
3. **制限元を辿る。** 子の `cpu.stat` の帯域統計が増えず `cpu.stat.local` が増え、祖先の帯域統計も増えるなら、祖先 quota の調査を優先する。子だけの上限緩和では解消しない可能性がある。祖先と兄弟の workload 配置、どの設定管理が上限を所有しているかまで確認してから変更案を作る
4. **PSI を原因の単独判定に使わない。** `cpu.pressure some` が増えても、それだけで quota 枯渇とは分類しない。帯域統計に証拠がなければ通常の CPU 競合なども調べる。PSI と throttling の数値が同じになることや、全 kernel 条件で同時に増えることを前提にしない
5. **修正効果を workload で判定する。** 変更案は親 quota、配置、並列度などの仮説ごとに一つずつ試す。停止指標だけでなく p95/p99、処理量、兄弟 workload の遅延が改善したかを見る。権限や設定の所有者を確認せず cgroupfs を直接書き換えることを、この調査の手順とはしない

診断表に入れる状態は少なくとも次の3つに分けると誤報を減らせる。

- **値ゼロ:** キーを読み取れており、同一 cgroup の観測区間に増分がない
- **測定なし:** ファイル・キーの欠落、CPU controller/PSI の無効、取得失敗がある
- **上位未確認:** 観測側に祖先が見えず、外側の制限を否定できない

たとえば「子の cpu.max は max、子の nr_throttled は差分ゼロ、親の nr_throttled は増加」という観測に対し、子を CPU 制限なしと判定するのは早い。子の runnable な期間と local 統計、親の補充周期、アプリケーション遅延を突き合わせる。一方、親が制限に達しても子がその期間アイドルなら、子の性能障害を自動的に説明することにはならない。

## 避ける判断

- `cpu.stat` の使用量と帯域統計を同じ「子孫を全部含む値」として処理する
- `cpu.stat.local` を「祖先の影響を除外した値」と名付ける
- 子の `cpu.max=max`、または `nr_throttled=0` だけで quota 起因を否定する
- `throttled_usec` を wall-clock の停止率へ無条件に変換し、100% で丸めて情報を失う
- system-wide CPU `full=0`、無効な PSI、欠落したキーを正常の証拠として使う
- cgroup v1 の `throttled_time` と v2 の `throttled_usec` を同じ単位で保存する
- 2026 年の文書補足を、同年の kernel 動作変更や必須アップグレードと誤認する

## 出典・ライセンス・未確認事項

- 公式 HTML 3ページは native web で本文を確認した。cgroup v2 のヘッダーは October 2015、PSI は April 2018 であり、現在の改訂日とは解釈しない。CFS ページに公開・更新日表示は確認できなかった。HTML の表示から厳密な kernel build 版を推測せず、別途固定 source の Makefile で 7.3.0-rc5 を確認した
- 固定 commit の文書・実装・差分は GitHub API でも読み取った。一部の SHA 固定 URL は native web では cache miss だったため、その失敗を本文取得成功と扱っていない。変更の根拠は API で取得した同一 SHA の内容にある
- [`COPYING`](https://github.com/torvalds/linux/blob/e767a4ea70a3992c37ed604157d32f0dfbf9b1e3/COPYING) と [`license-rules`](https://github.com/torvalds/linux/blob/e767a4ea70a3992c37ed604157d32f0dfbf9b1e3/Documentation/process/license-rules.rst) を確認した。`kernel/sched/core.c` の SPDX は GPL-2.0-only。repository 全体の COPYING は GPL-2.0 WITH Linux-syscall-note を示す。HTML 文書単体の別ライセンスは確定していないため catalog では unknown とし、ここでは出典付きの独自要約だけを行う。kernel や PSI のサンプルコードは転載しない
- 本調査で実機の CPU 負荷、祖先 quota、PSI 増分を再現していない。kernel build、ベンチマーク、exporter ごとの集計変換、Docker/Kubernetes/systemd の設定から各 cgroup への写像も未検証。`cpu.stat.local` と PSI を等価に扱う変換式や汎用の警報閾値は提示しない
- 安定版ごとの可用性、CONFIG_CFS_BANDWIDTH/PSI の build 条件と起動設定の全組み合わせ、BPF scheduler、RT scheduling、CPU affinity/cpuset、すべての accounting 更新タイミングは未確認。実際の変更前には対象環境で確認する
- 取得日は 2026-10-03 UTC。公式文書・repository 分析の TTL 90 日に合わせ、再確認期限は 2027-01-01 とする。検索 eval は内容を検索できることの検査であり、kernel の動作検証ではない

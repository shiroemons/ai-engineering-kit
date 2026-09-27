---
{
  "id": "architecture-circuit-breaker-retry-budget",
  "title": "サーキットブレーカーと retry budget によるカスケード障害の防止: Envoy の circuit breaker 閾値・outlier detection ejection 条件と Google SRE の再試行予算",
  "kind": "knowledge",
  "technology": "architecture",
  "version": "Envoy docs latest（表示 1.40.0-dev、commit e0f4199bc8865a9dccd9d3be279900a050b2cb6b に固定）、Site Reliability Engineering (O'Reilly 2017) Chapter 21・Chapter 22",
  "tags": [
    "research-domain:api-distributed",
    "architecture",
    "circuit-breaker",
    "retry-budget",
    "outlier-detection",
    "ejection",
    "envoy",
    "cascading-failure",
    "retry",
    "overload",
    "backpressure",
    "load-shedding",
    "distributed-systems",
    "health-check",
    "google-sre"
  ],
  "sources": [
    {
      "id": "envoy-v3-circuit-breaker-proto",
      "url": "https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/cluster/v3/circuit_breaker.proto",
      "type": "official_docs"
    },
    {
      "id": "envoy-v3-outlier-detection-proto",
      "url": "https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/cluster/v3/outlier_detection.proto",
      "type": "official_docs"
    },
    {
      "id": "envoy-arch-circuit-breaking",
      "url": "https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/circuit_breaking",
      "type": "official_docs"
    },
    {
      "id": "envoy-arch-outlier-detection",
      "url": "https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier",
      "type": "official_docs"
    },
    {
      "id": "sre-book-handling-overload",
      "url": "https://sre.google/sre-book/handling-overload/",
      "type": "official_docs"
    },
    {
      "id": "sre-book-addressing-cascading-failures",
      "url": "https://sre.google/sre-book/addressing-cascading-failures/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# サーキットブレーカーと retry budget によるカスケード障害の防止: Envoy の circuit breaker 閾値・outlier detection ejection 条件と Google SRE の再試行予算

1つのサービスの過負荷が再試行の正のフィードバックで全体へ広がるカスケード障害を、接続・リクエスト上限のサーキットブレーカー、再試行に専用の予算（retry budget）、故障ホストを負荷均衡から外す outlier detection（ejection）でどう防ぐかを整理する。Envoy の v3 API proto（[Circuit breakers](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/cluster/v3/circuit_breaker.proto)、[Outlier detection](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/cluster/v3/outlier_detection.proto)）とアーキテクチャ概要（[Circuit breaking](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/circuit_breaking)、[Outlier detection](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier)）は commit `e0f4199bc8865a9dccd9d3be279900a050b2cb6b` の文書に固定して確認し、Google SRE 本は [Chapter 21 Handling Overload](https://sre.google/sre-book/handling-overload/) と [Chapter 22 Addressing Cascading Failures](https://sre.google/sre-book/addressing-cascading-failures/)（2017年、O'Reilly）を 2026-09-27 に取得して確認した。以下で「記載事実」と「独自の設計案」を明示的に分ける。

## 要点（公式文書の記載事実）

### カスケード障害は再試行の正のフィードバックで起きる（SRE 第22章）

- カスケード障害は positive feedback（正のフィードバック）によって成長する故障として定義され、最頻はサーバの過負荷だと述べる。
- 素朴な再試行ループの数値例: 上限 10,000 QPS のバックエンドに 10,100 QPS を供給し、1リクエスト最大10試行のキャップがあると、超過分100 QPSが再試行で 100 → 200 → 300 QPS と増殖し、最終的に meltdown する。キャップ（10試行）だけでは増殖を止められないことが示されている。
- 再試行は負荷分散にとって「新しいリクエストと区別されない」（第21章）。失敗したホストを避けるロードバランサは残ホストへ負荷を集中させ、雪崩（snowball）を悪化させうる（第22章）。
- 第22章の緩和策: randomized exponential backoff with jitter、1リクエストあたりの再試行上限、サーバ全体の retry budget（例: 1プロセス毎分60再試行まで、超過したらリクエストを失敗させる）、マルチレイヤ再試行の回避（3層×3再試行 = 4³ = 64試行）、再試行可能/不可能を区別するステータスコードと専用の overload 状態（クライアントが再試行せずに back off できるようにする）、deadline 伝播、load shedding / graceful degradation、キュー寸法はスレッドプールの50%未満、失敗するまで load testing。

### retry budget の2つの規定（SRE 第21章 / Envoy circuit_breaker.proto）

- SRE 第21章の予算は2層:
  - **1リクエスト単位**: 最大3試行まで。それ以上はエラーを上位へ bubble up させる。
  - **クライアント単位**: クライアントの retry/total 比率が10%を下回るときだけ再試行する予算。試行回数キャップだけ（予算なし）だと最悪の増長が約3×、予算ありでは約1.1×に抑えられると述べる。
- 再試行の判断材料として attempt-count のメタデータを使い、ヒストグラムで再試行が蔓延しているときはバックエンドが「overloaded; don't retry」を返せるようにする。拒否した層の**直上だけ**が再試行する（多層の再試行は組合せで爆発する）。タスクの大きな部分集合が過負荷なら再試行せずエラーを伝播する。
- クライアント側 adaptive throttling は直近2分の requests/accepts 履歴を使い、K 乗数は通常 2。拒否順序は per-customer quota と criticality tier（CRITICAL_PLUS / CRITICAL / SHEDDABLE_PLUS / SHEDDABLE）で決める。
- Envoy の `retry_budget`（`CircuitBreakers.Thresholds` 配下）:
  - proto の説明は「active requests の数に対する concurrent retries の上限」。**設定すると「retry budget will override any configured retry circuit breaker」**、つまり `max_retries` のサーキットブレーカーは使われない。
  - `budget_percent` 既定 **20%** — active requests + active pending requests の合計に対する同時再試行の上限（例: active 100・25% なら active retries は25）。
  - `budget_interval` 既定 **0ms** — このとき現在 active / pending のリクエストだけを計算に使う。非零を指定すると、開始したリクエストが完了してなくても `budget_interval` の間だけ計算に含め続ける（例: 同時開始10リクエスト + 100ms なら次100ms は10件すべてが計算対象、期間経過で失効）。
  - `min_retry_concurrency` 既定 **3** — 「The limit on the number of active retries may never go below this number」（予算計算が示す値が低くてもこれだけは下回らない）。
- Envoy アーキテクチャ概要は静的設定より retry budget を推奨し、静的設定を選ぶなら「it should aggressively circuit break retries」として、その理由を「retries for sporadic failures are allowed, but the overall retry volume cannot explode and cause large scale cascading failure」と説明する。

### Envoy circuit breaker の上限と挙動（circuit_breaker.proto / Circuit breaking 概要）

- Circuit breaking は「fully distributed (not coordinated)」で、アプリ個別にコードする代わりに Envoy が**ネットワークレベルで強制**する。「nearly always better to fail quickly and apply back pressure downstream as soon as possible」が前提。
- 上限は **cluster 毎・priority 毎**に設定・追跡される。同一 priority に複数 `Thresholds` があると先頭が使われ、priority ごとに未設定なら既定値。既定値は `max_connections=1024`・`max_pending_requests=1024`・`max_requests=1024`・`max_retries=3`（並列再試行の上限）。
- 上限に達して溢れたとき（overflow）の統計（アーキテクチャ概要の対応表）:
  - 最大接続 → `upstream_cx_overflow`（active/draining とも計上。上限に達していてもロードバランサで選ばれたホストには最低1接続は確保されるため、上限 + (エンドポイント数 × コネクションプール数) まで超過しうる。この上限は全 worker 総和に適用される）
  - 最大 pending リクエスト → `upstream_rq_pending_overflow`（接続プール待ちのキュー。非 HTTP トラフィックには接続数上限として適用）
  - 最大 active リクエスト → `upstream_rq_active_overflow`（非 HTTP には適用されない）
  - 最大 active 再試行 → `upstream_rq_retry_overflow`
  - 最大並列コネクションプール → `upstream_cx_pool_overflow`（コネクションプールはタイムアウトせず自動清掃もされない。`max_connection_pools` の既定は unlimited で、プールが大量に作られる cluster でのみ設定を勧める、と proto は述べる）
- **worker 線は上限を共有**するが「Since the implementation is eventually consistent, races between threads may allow limits to be potentially exceeded」— 厳密な上限ではない。
- 既定で有効で値は控えめ（cluster あたり1024接続など）。無効化は thresholds を最大許容値に設定して行う。HTTP リクエストでは circuit breaking により router filter が `x-envoy-overloaded` ヘッダを付与する。
- `track_remaining`（既定 false）は「上限に達するまでに残ったリソース数」を示す統計を出すが、**retry budget を `max_retries` の代わりに使うと残量は追跡されない**（proto の Note）。
- `per_host_thresholds`（ホスト毎の上限）は「currently only the `max_connections` field is supported」。未設定ならホスト毎の上限はない。

### Envoy outlier detection の ejection 条件（outlier_detection.proto / Outlier detection 概要）

- outlier detection は**パッシブなヘルスチェック**。cluster 設定の一部で、エラー・タイムアウト・リセットを報告する**フィルタに依存**する（現在 http router / tcp proxy / redis proxy / thrift proxy のみ対応）。アクティブヘルスチェックとは別物で、併用も単独も可。
- エラーは external（トランザクション固有、サーバが返した500など）と local origin（Envoy 側で発生、timeout・TCP reset・接続不能など）に分かれる。**既定（`split_external_local_origin_errors=false`）では両者が同一バケット**に入り `consecutive_5xx` などで比較される（例: timeout 2回 + 接続後の500 = 3件）。tcp proxy のように上位プロトコルを知らないフィルタは local origin のみ報告。非 HTTP フィルタのエラーは内部的に HTTP 5xx へマップされ、gRPC は `grpc-status` レスポンスヘッダからマップした HTTP status を使う（既定は HTTP status のみ理解し、5xx を error、他を success とする。cluster の matcher 設定で error 判定を再定義できる）。
- **ejection 手順（Ejection algorithm）**: (1) outlier 判定（連続5xxは inline、success rate は interval 毎）→ (2) 現在の ejected 数が `max_ejection_percent` 以下か確認、超過なら ejection しない → (3) `base_ejection_time` × そのホストの連続 ejection 回数だけ ejection（時間は `max_ejection_time` で頭打ち）。ejection とは不健康マークにしてロードバランシングから外すこと（panic シナリオを除く）→ (4) ejection 時間が経過すれば自動で復帰する。ヘルスな状態が続けば interval 毎のチェックで ejection time multiplier が減り、ejection time を最小値（`base_ejection_time`）まで下げるのにかかる目安は `max_ejection_time / base_ejection_time × interval` 秒と概要は述べる。
- 既定値（proto）: `interval=10s`（分析スイープ間隔）、`base_ejection_time=30s`、`max_ejection_time` 未指定時は 300s と base の大きい方、`max_ejection_percent=10%`、`max_ejection_time_jitter=0s`、`detect_degraded_hosts=false`（`x-envoy-degraded` を返すホストを degraded に。degraded は ejected ではなく回転内での優先度低下）、`always_eject_one_host=false`（有効なら `max_ejection_percent` にかかわらず最低1ホストは eject）。
- `enforcing_*` は「outlier status を検出したときに実際に ejection される% chance」。既定:
  - `consecutive_5xx=5`（0で無効化）+ `enforcing_consecutive_5xx=100` — **既定で実際に効く**
  - `consecutive_gateway_failure=5`（502/503/504、0で無効化）+ `enforcing_consecutive_gateway_failure=0` — **検出はされても既定では ejection されない**
  - `consecutive_local_origin_failure=5`（`split_external_local_origin_errors=true` のときのみ有効）+ `enforcing_consecutive_local_origin_failure=100`（同 split 時のみ）
  - success rate: `enforcing_success_rate=100`。ただし発火の前提条件として `success_rate_minimum_hosts=5`（十分な request volume を持つホスト数がこれに満たないならクラスタ全体で success rate 検出を実施しない）と `success_rate_request_volume=100`（interval 内の総リクエスト数がこれ未満ならそのホストは対象外）がある。ejection 閾値は `mean − (stdev × success_rate_stdev_factor)`、`success_rate_stdev_factor=1900` は1000で割って使われるので実質 mean − 1.9×stdev。
  - failure percentage: `failure_percentage_threshold=85`（ホストの失敗率が**この値以上**で ejection 対象）+ `enforcing_failure_percentage=0`（**既定では ejection されない**）、`failure_percentage_minimum_hosts=5`、`failure_percentage_request_volume=50`。
- 復帰: ejection 時間経過で自動復帰。アクティブヘルスチェックが設定されていれば成功1回で uneject し**全 outlier detection カウンタを消去**する（`successful_active_health_check_uneject_host=true` が既定）。ただしアクティブ HC が実トラフィックを検証していないと「pass するがトラフィックは失敗」のときに premature uneject しうると文書は注記する。
- **cluster は複数フィルタチェーンで共有される**: あるフィルタチェーンが outlier detection でホストを eject すると、その種別では eject しない他のフィルタチェーンも影響を受ける（概要の明記）。
- ejection イベントは `OutlierDetectionEvent` の protobuf dump として任意でログ出力できる（グローバル統計だけではどのホストがなぜ eject されたか分からない、と概要は述べる）。

## 推奨方法（独自の設計案。上記文書の規定ではない）

- **再試行の主防御は2層に絞る**: (a) Envoy 側は静的 `max_retries` ではなく `retry_budget` を設定する（概要の推奨どおり。`budget_percent` は active + pending に対する比率なので、自クラスタの正常時の in-flight 数をもとに試算する）。(b) アプリ側クライアントは SRE の2予算（1リクエスト3試行 + retry/total 10%未満）を入れる。Envoy の20%と SRE の10%は分母が違う（in-flight 数 vs 全リクエスト比）ので、同じ数値として扱わない。
- **再試行は拒否した層の直上だけ**に許可し、多層同時再試行を作らない（4³=64 の組合せ爆発を避ける）。overload を明示する専用応答を用意し、クライアントはそれを再試行せず back off する規約にする。
- **outlier detection は enforce の既定を前提に設計し直す**: `consecutive_gateway_failure` と failure percentage は既定 `enforcing_*=0`（検出のみ、ejection なし）なので、502/503/504 や高失敗率で eject したいなら `enforcing_*` を上げる判断を明示的に行う。逆に段階導入（ramp up）にはこの既定が使える。
- **ejection 率の上限と残容量をセットで監視する**: `max_ejection_percent` 既定10%で残ホストへの負荷移動は抑えるが、SRE が指摘する「エラーを返したサーバを避けると雪崩が悪化する」という観測と矛盾しない運用にする。残ホストの頭打ち（panic 閾値を含む）を確認してから上限を上げる。
- **可観測性**: overflow 統計（`upstream_cx_overflow` / `upstream_rq_pending_overflow` / `upstream_rq_active_overflow` / `upstream_rq_retry_overflow` / `upstream_cx_pool_overflow`）、`x-envoy-overloaded` ヘッダ、ejection イベントログを見る。retry budget 使用時は `track_remaining` が効かないため残量の代替指標として overflow 統計を使う。上限が eventually consistent なら余裕（headroom）を確保した設計にする。
- **小規模クラスタは success rate / failure percentage のゲートを満たせない**（hosts 5未満・volume 100/50 未満なら発火しない）ため、ホスト数が少ない構成では `consecutive_5xx` 系に頼るか、ゲート値を実トラフィックに合わせて下げる判断が要る。
- SRE の緩和策を併用する: randomized exponential backoff with jitter、deadline 伝播、load shedding / graceful degradation、キュー寸法をスレッドプールの50%未満に、失敗するまで load testing。

## 避ける使い方

- **複数レイヤで同時に再試行する構成**（第22章: 3層×3再試行 = 64試行）。
- jitter なしの固定待機・即時再試行の連打。再試行を試行回数キャップだけで管理し、全体比率の予算を入れない（最悪3×増の数値例と、Envoy の「retry volume cannot explode」という勧告に反する）。
- **retry budget 設定時に `track_remaining` の残量統計を前提にした監視**（proto が明記するように追跡されない）。
- `enforcing_consecutive_gateway_failure` / `enforcing_failure_percentage` の既定値のままで「502連続や失敗率85%で eject される」と想定する（既定0で ejection されない）。
- 小規模クラスタで success rate outlier detection が発火すると想定する（`success_rate_minimum_hosts=5`・`success_rate_request_volume=100` 未満なら実施されない）。
- circuit breaker の上限を厳密な値として扱う（worker 競合で超過しうる、cluster の最大接続は選択ホストへの最低1接続保証で上振れしうる）。協調的（global で合意される）なブレーカーと想定することも誤り（fully distributed）。
- ejection を「故障ホストの完全除去」と扱う（`max_ejection_percent` 既定10%で上限、panic 時は除外されない。ejection は時間経過で自動復帰し、カウンタはアクティブ HC 成功で消える）。
- `base_ejection_time=30s` を固定の除外時間と考える（実時間は base × 連続 ejection 回数、`max_ejection_time` 既定300s で頭打ち。継続故障ほど長くなる）。
- outlier detection をアクティブヘルスチェックの別名として扱う、またはフィルタ報告なしで動くと想定する（対応4フィルタに依存し、tcp proxy は local origin のみ報告）。
- 一部フィルタチェーンだけの ejection だと他チェーンに影響しないと想定する（cluster 共有で波及する、概要の明記）。
- タスクの大きな部分集合が過負荷のときに再試行し続ける（第21章: 再試行せずエラーを伝播する）。エラーを返したホストだけを避けるロードバランサで雪崩が収まると想定する（第22章: 悪化させうる）。

## 適用版と本番での注意

- 適用版: Envoy 文書は `latest`（表示 1.40.0-dev）だが内容は commit `e0f4199bc8865a9dccd9d3be279900a050b2cb6b` の文書に固定して確認した。URL は `/latest/` のため次回取得では版と差分を再確認する。SRE 本は2017年版の Chapter 21・22 で、文中の QPS 例（10,000/10,100）、retry budget 例（1プロセス毎分60再試行）、K=2 は**説明用の例示**であり一般的な数値保証ではない。
- 予算の定義差: Envoy `budget_percent` 既定20% は「active + pending requests に対する同時再試行の上限」、SRE の10% は「retry/total のリクエスト比率」。期間・分母が別物で、片方をもう片方に変換する公式はない。`budget_interval` を非零にすると計算対象が interval 中の開始リクエストへ拡大する（既定0ms は現在の active/pending のみ）。
- ejection 時間: `max_ejection_time` は未指定時「300s と `base_ejection_time` の大きい方」が適用される。`enforcing_*` は ejection される% chance で、検出と実行が分離している点が運用判断の分かれ目。
- per-host 上限は `max_connections` のみ対応（proto の "currently" 表記、以降の版で変わりうる）。`max_connection_pools` は既定 unlimited で、接続プールが無制約に増える機能（例: Original Src Listener Filter）がある cluster でのみ設定を検討する。
- 未確認範囲: retry budget の内部実装（統計窓の正確な挙動、`min_retry_concurrency` 以外の微調整）と outlier detection の統計処理の実装詳細は本調査の範囲外。SRE の adaptive throttling や criticality tier は原則の記述で、Envoy の設定項目への直接対応は文書にない。Envoy 以外のメッシュ（Istio 等）での既定値差は未確認。
- 再確認期限: 全 source が official_docs（TTL 90日）。technology（architecture）固有 TTL は設定されていない。2026-12-26 までに Envoy 文書（`/latest/` の移動に注意）と SRE 章を再取得して内容を確認する。

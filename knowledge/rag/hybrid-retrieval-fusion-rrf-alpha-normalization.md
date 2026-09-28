---
{
  "id": "rag-hybrid-retrieval-fusion-rrf-alpha-normalization",
  "title": "hybrid retrieval の結果融合: RRF (reciprocal rank fusion) の k と rank_window、alpha 重み付けとスコア正規化の適用条件",
  "kind": "knowledge",
  "technology": "rag",
  "version": "Elasticsearch Reference 現行 docs + Elasticsearch Guide 8.19 (frozen) + Weaviate docs (v1.36.7 / v1.38.6 記載) + Qdrant docs (v1.17.0 記載) を 2026-09-28 に確認",
  "tags": [
    "research-domain:ai-engineering",
    "rag",
    "hybrid-search",
    "hybrid retrieval",
    "retrieval",
    "reciprocal rank fusion",
    "rrf",
    "rank_constant",
    "rank_window_size",
    "alpha",
    "score normalization",
    "normalization",
    "relativeScoreFusion",
    "rankedFusion",
    "dbsf",
    "weighted RRF",
    "elasticsearch",
    "weaviate",
    "qdrant"
  ],
  "sources": [
    {
      "id": "elastic-rrf-docs-current",
      "url": "https://www.elastic.co/docs/reference/elasticsearch/rest-apis/reciprocal-rank-fusion",
      "type": "official_docs"
    },
    {
      "id": "elastic-rrf-8-19",
      "url": "https://www.elastic.co/guide/en/elasticsearch/reference/8.19/rrf.html",
      "type": "official_docs"
    },
    {
      "id": "weaviate-hybrid-search-concepts",
      "url": "https://weaviate.io/developers/weaviate/concepts/search/hybrid-search",
      "type": "official_docs"
    },
    {
      "id": "weaviate-hybrid-search-howto",
      "url": "https://weaviate.io/developers/weaviate/search/hybrid",
      "type": "official_docs"
    },
    {
      "id": "qdrant-hybrid-multistage-queries",
      "url": "https://qdrant.tech/documentation/search/hybrid-queries/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active"
}
---

# hybrid retrieval の結果融合: RRF (reciprocal rank fusion) の k と rank_window、alpha 重み付けとスコア正規化の適用条件

dense と sparse (BM25) の2経路を返す hybrid retrieval の結果集合を1本に融合するとき、rank ベースの RRF (reciprocal rank fusion) とスコア正規化ベースの合成を、どの条件下で選ぶかを3社の公式文書だけに絞って整理する。文書化された事実 (各節の「文書化された事実」) と本書の設計上の提案 (「推奨方法」) を区別する。確認した適用版は front matter の version を参照。

## 要点

### 文書化された事実: Elasticsearch の RRF は rank と定数 k だけで合成する

- [Elasticsearch Reference: Reciprocal rank fusion](https://www.elastic.co/docs/reference/elasticsearch/rest-apis/reciprocal-rank-fusion) (現行 docs、2026-09-28 確認) の式は `score = Σ 1.0 / (k + rank)`。`rank` は各 result set の中で **1 始まり**。同ページは RRF が「requires no tuning」で、different relevance indicators は互いに無関係でよいと明記する。つまり rank ベースゆえ、BM25 スコアと cosine 類似度のような異種スコアの正規化は不要という前提である。
- `rank_constant` は整数で **1 以上、既定 60**。値が大きいほど下位 (lower ranked) の文書が最終順位へ与える影響が増す。
- `rank_window_size` は各 child retriever の result set の大きさを決める。**`size` 以上かつ `size` ではなく `1` 以上**である必要があり、最終的な ranked set は search request の `size` に切り詰められる。現行 docs の既定は **`size` パラメータ**。
- knn の `k` と `rank_window_size` の関係: `k` が `rank_window_size` より大きければ結果は `rank_window_size` に切られる。`k` の方が小さければ結果は `k` の大きさになる。
- RRF retriever は **最低2本の child retrievers** を要求し、**各 child は同等の重み (equal weight)** で式に加算される。retriever 単位の重みを付与するパラメータは現行 docs に存在しない。
- ページネーションは `rank_window_size` を固定している間だけ一貫する。`from + size` が `rank_window_size` を超えると **0 件**を返す。`rank_window_size` を変えると同じ rank でも順序が変わり得ると明記されている。
- 非対応は `scroll`・`sort`・`rescore` (併用すると例外)。RRF は内部で1つの PIT (point in time) を作り sub-retriever 間で共有する。aggregation は `rank_window_size` の制限を受けず、全 sub-retriever の結果集合の和に対して動く。

### 文書化された事実: rank_window_size の既定値は 8.19 frozen と現行 docs で記述が異なる

- [Elasticsearch Guide [8.19]: Reciprocal rank fusion](https://www.elastic.co/guide/en/elasticsearch/reference/8.19/rrf.html) (「no longer updated」バナー付きの frozen docs、2026-09-28 確認) は式・rank の 1 始まり・`rank_constant` 既定60・同等重み・knn `k` の切詰め・ページネーション条件を現行 docs と同様に記載する。
- 差異は `rank_window_size` の既定で、8.19 ページは **「Defaults to 10」**、現行 docs は **「Defaults to the `size` parameter」**。どのリリースで変わったかは本書の検証範囲に含まれず未確認。

### 文書化された事実: Weaviate の alpha は vector 脚の重み、融合方式で正規化の要否が変わる

- [Weaviate concepts: Hybrid search](https://weaviate.io/developers/weaviate/concepts/search/hybrid-search) (2026-09-28 確認) によれば `alpha` は 0〜1 で vector 結果の重みを決め、`0` = keyword のみ、`1` = vector のみ。
- サーバ既定の `alpha` は **0.75** だが、これは **alpha を欠いたリクエストにだけ**適用される (GraphQL。gRPC で alpha を未指定にできるようになったのは v1.36.7 以降)。クライアントは一律に未指定にするわけではなく、環境によっては実質 keyword のみになり得る。同ページは「重み付けが重要なときは alpha を明示せよ」と明記する。
- `rankedFusion` (v1.23 まで既定) は `1/(RANK + 60)` で、**位置だけ**を見る RRF (k = 60) である。
- `relativeScoreFusion` (**v1.24 から既定**) は各脚のスコアを min-max で [0,1] に正規化 (最大=1、最小=0) してからスケール済みの和を取る。スコアの大きさの情報 (magnitude) を保持する。
- `autocut` は実際の similarity score を使って切詰め点を探すため **`relativeScoreFusion` を要求**され、ranking 位置だけの `rankedFusion` では使わないことと記載されている (how-to: [Hybrid search](https://weaviate.io/developers/weaviate/search/hybrid))。
- hybrid search の threshold は **vector 脚の `max vector distance` のみ**。BM25 脚にも合成後スコアにも閾値はなく、理由は BM25 スコアが正規化・bounded ではないため。融合は post-processing より先に走り、boost (v1.38) と MMR 多様性選択 (v1.38.6) は fused 候補プールに対して適用される。ranked search の `offset + limit` は `QUERY_MAXIMUM_RESULTS` (既定 10000) が上限。

### 文書化された事実: Qdrant は k・retriever 重み・正規化方式を選べる

- [Qdrant: Hybrid and Multi-Stage Queries](https://qdrant.tech/documentation/search/hybrid-queries/) (2026-09-28 確認) の RRF は `score(d) = Σ 1/(k + (r_d + 1)/w_r - 1)`。rank は **zero-based** (トップが `r_d = 0`)。`k` の既定は **2** (Elastic の 60 とは大きく異なる)、retriever 重み `w_r` の既定は 1 で、このとき式は `1/(k + r_d)` に簡約される。
- 明示的な `k` は **v1.16.0 以降**、weights 付き RRF (weighted RRF) は **v1.17.0 以降**。`weights` 配列の長さは prefetch 数と一致する必要があり、既定は同重み。
- weights の調整指針も文書化されている: eval set を **train/val に分割**して調整し、eval set がなければ **(1.0, 1.0) のまま**にする。測らない手調整は既定を上回る可能性が低いと明記されている。
- DBSF (Distribution-Based Score Fusion、**v1.11.0 以降**) は各 retriever の返却スコアを 3-sigma で正規化して足し合わせる: `(s - (mu - 3 sigma)) / (6 sigma)`。**[0,1] には切り枝なく**、全スコア同一または1件のみなら 0.5 を出す。統計量は prefetch の top-k から計算するため、top-k に強い外れ値が1つあると正規化が歪む。
- fusion の選択表が文書化されている: **eval set があれば weighted RRF**、**生スコアを信頼して eval set がなければ DBSF**、**どちらもなければ RRF (safe default)**。
- 同ページは「dense と sparse の生スコアへ固定の alpha を掛ける線形結合」を、正規化なしでは **unreliable (信頼できない)** と明記する。理由は dense (bounded) と BM25 (unbounded、query ごとにシフト) のスケールが異なり、生スコアの絶対値が大きい側に毎回支配されるためで、RRF は rank で、DBSF は分布正規化でこの問題を回避すると説明している。

## 推奨方法

以下は上記5ページからの**設計上のまとめ**であり、各社が定める実装構成そのものではない。

- **融合方式は eval set の有無で先に決める。** Qdrant の選択表を共通の出発点にする: eval set (queries と known-relevant docs) があれば train/val 分割で weighted RRF、生スコアの傾きを信じるなら DBSF、どちらもなければ rank ベースの RRF。Elastic の「requires no tuning」は rank 融合が正規化不要である根拠として扱い、そのまま「調整不要」と読み替えない (Qdrant は weights の調整を推奨している)。
- **alpha を必ず明示する。** Weaviate のサーバ既定 0.75 は alpha 欠落時のみで、クライアントによって実効値が違う。重みが結果に効く話では、既定に任せず 0〜1 の値をリクエストごとに指定する。
- **生スコアへ固定 alpha を掛けない。** dense と BM25 の raw score を正規化せずに線形和すると、query ごとに絶対値の大きい経路に支配される (Qdrant の明記した注意)。合成をスコアベースで行うなら min-max 正規化 (Weaviate `relativeScoreFusion`) や 3-sigma 分布正規化 (Qdrant DBSF) を通すか、rank ベース (RRF) に逃げる。
- **後続がスコアの閾値・cutoff を必要とするならスコアベース融合を選ぶ。** autocut のような実スコアを要する後処理は `relativeScoreFusion` 必須で、`rankedFusion` と併用しない (Weaviate 文書)。Weaviate hybrid に BM25・合成後スコアの閾値はないため、それが必要なら融合後の段で自前につくる。
- **Elastic の `rank_window_size` は「child 各自の候補窓」として設計する。** 最終 `size` より十分大きく (制約は `>= size`)、ページネーション中は固定を維持する。`from + size > rank_window_size` は0件になるため、深掘りページを見込むなら窓をそれに合わせる。
- **k は engine 固有の定数として扱い、横持ちしない。** 既定は Elastic 60 (rank 1 始まり)、Qdrant 2 (rank 0 始まり)、Weaviate `rankedFusion` は式中に 60 固定。同じ数値でも rank の始まりが違うため意味が異なる。移行時は eval をやり直す。
- **weights は eval があるときだけ動かす。** Qdrant の指針どおり train/val 分割で試し、無ければ同重み (1.0, 1.0)。Elastic の RRF には retriever 単位重みがそもそもないため、重み付けが要件なら Qdrant の weighted RRF かアプリ側合成を選ぶ。

## 避ける使い方

- **正規化していない dense/BM25 生スコアに固定の alpha を掛ける。** スケールが非 bounded かつ query ごとにシフトするため、生スコアの大きい側に支配される (Qdrant の明記した注意)。
- **`rankedFusion` で autocut を使う。** autocut は実 similarity score を必要とし、ranking 位置だけの融合では使わないことと文書に明記されている。
- **ページネーション中に `rank_window_size` を変える、または `from + size` を窓より大きくする。** 後者は0件、前者は同じ rank でも順序が変わり一貫性が壊れる (Elastic 文書)。
- **Elastic の RRF に retriever 単位の重みがある前提で設計する。** 現行 docs は各 child は equal weight と明記しており、重みパラメータは存在しない。
- **k=60 や k=2 を別の engine にそのまま持ち込む。** rank の始まり (1始まり / 0始まり) と式が違うため、同一の数値でも配点が異なる。
- **クライアント既定の alpha を確認せずに「0.75 で動いている」と考える。** 0.75 は alpha 未指定のリクエストにのみ効き、クライアントの既定は環境により得点を keyword 側へ寄せることがある。
- **DBSF の正規化結果を [0,1] として閾値を置く。** 3-sigma 正規化は切らず、範囲外の値は範囲外のまま出る。また top-k の外れ値1つが統計量を歪め得る。
- **`scroll` / `sort` / `rescore` を Elastic の RRF と組み合わせる。** 非対応で例外になる。RRF は内部 PIT を作るため、リクエスト側で PIT を渡さない (渡さないことが推奨と文書にある)。
- **8.19 の「既定 10」か現行の「既定 size」かを普遍的な仕様として断定する。** frozen docs と現行 docs で記述が異なり、切替リリースは未確認である。

## 適用版と本番での注意

- **Elastic**: 現行 docs (applies_to: Elastic Stack: Generally available) と Elasticsearch Guide 8.19 (frozen、更新停止バナー付き) を 2026-09-28 に確認。`rank_window_size` の既定は両者で記述が異なる (8.19: 10、現行: `size`)。**未確認**: どのリリースで既定が変わったか、将来 retriever 単位の重みが追加されるか。運用では既定に頼らず `rank_window_size` と `rank_constant` を明示するのが安全 (これは本書の設計提案)。`rank_window_size` を大きくすると relevance が上がる代わりに performance コストが増える点は文書化されている。内部 PIT の寿命や解放タイミングは本書の source に含まれないため未確認。
- **Weaviate**: concepts / how-to ページを 2026-09-28 に確認。`relativeScoreFusion` 既定は v1.24 から、`rankedFusion` 既定は v1.23 まで。gRPC での alpha 未指定対応は v1.36.7 以降、keyword operators は v1.31、boost は v1.38、MMR は v1.38.6。**未確認**: `rankedFusion` の式中 k=60 を変更できるか、`QUERY_MAXIMUM_RESULTS` を変えた場合の性能影響。
- **Qdrant**: Query API v1.10.0、DBSF v1.11.0、RRF の k 指定 v1.16.0、weighted RRF v1.17.0 をページ記載として確認。DBSF は v1.11.0 以降の機能であり、旧版では使えない。
- 本文の `expires_at` は 2026-12-27 (取得 2026-09-28 + `official_docs` TTL 90日)。技術 TTL (rag) は設定にないため、source type と明示期限で判定する。5件とも vendor 公式ドキュメントで `trust: official`、本文書の `trust` もこれに合わせた。
- **未確認**: 自 workload での k / alpha / weights の最適値。3社とも自データでの eval による選定を前提としており、本書の推奨方法はその手順の設計提案であって数値の保証ではない。 timeouts・cancellation・resource ownership・shutdown はこれらの融合ページの主題外で、source に含まれないため推測で埋めない。

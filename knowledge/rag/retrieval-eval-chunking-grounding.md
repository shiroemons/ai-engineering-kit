---
{
  "id": "rag-retrieval-eval-chunking-grounding",
  "title": "RAG の retrieval 評価: chunking 戦略と Context Precision による relevance grounding",
  "kind": "knowledge",
  "technology": "rag",
  "version": "Ragas stable Context Precision + Azure AI Search Chunk Documents (ms.date 2026-06-08) + Anthropic Contextual Retrieval (2024-09-19)",
  "tags": [
    "research-domain:ai-engineering",
    "rag",
    "retrieval",
    "chunking",
    "context-precision",
    "relevance",
    "grounding",
    "overlap",
    "reranking",
    "evals"
  ],
  "sources": [
    {
      "id": "ragas-context-precision-docs",
      "url": "https://docs.ragas.io/en/stable/concepts/metrics/available_metrics/context_precision/",
      "type": "official_docs"
    },
    {
      "id": "azure-ai-search-chunk-documents",
      "url": "https://learn.microsoft.com/en-us/azure/search/vector-search-how-to-chunk-documents",
      "type": "official_docs"
    },
    {
      "id": "anthropic-contextual-retrieval",
      "url": "https://www.anthropic.com/news/contextual-retrieval",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "maintainer",
  "status": "active"
}
---

# RAG の retrieval 評価: chunking 戦略と Context Precision による relevance grounding

retrieval の順序つき relevance を Context Precision で測り、chunking 戦略の変更前後を eval で比較する方法を、3件の一次情報だけに絞って整理する。文書に書かれた動作 (事実) と本書の推奨方法節 (設計上のまとめ) を区別する。

## 要点

### 文書化された事実: Context Precision は順序つき relevance の指標である

- [Ragas Context Precision](https://docs.ragas.io/en/stable/concepts/metrics/available_metrics/context_precision/) (stable channel): Context Precision@K は relevance 指示子 v_k で重み付けした mean precision@k である。順序に感応し、同ページの例では不適合 chunk が先頭に来ると 1.0 から 0.5 に低下する。
- 同ページの変種は、reference を使う `ContextPrecision`、reference なしで response を基準にする `ContextUtilization` (response-based)、LLM を使わない NonLLM 版と ID-based 版である。reference の有無で使う変種が変わる。
- 含意として、適合 chunk が下位にあっても先頭の不適合は強く罰せられる。retrieval の grounding 確認は「適合の有無」だけでなく「順序」を見る必要がある。

### 文書化された事実: chunking は token 上限のために必要で、戦略と初期値が文書化されている

- [Azure AI Search Chunk Documents](https://learn.microsoft.com/en-us/azure/search/vector-search-how-to-chunk-documents) (ms.date 2026-06-08): chunking の対応表は fixed-size、variable-size、semantic、custom の4方式である。chunking が必要な理由は embedding モデルや chat モデルの token 上限であり、例として text-embedding-3-small の 8191 tokens が挙げられている。
- 同ページの Text Split skill は `textSplitMode` に `pages` / `sentences` を取り、`maximumPageLength`・`pageOverlapLength`・`maximumPagesToTake` で長さと overlap を制御する。例の既定値は pages モードで 2000 / 500 chars である。
- 同ページの開始点の推奨は 512 tokens (約 2000 chars) に 25% の overlap (128 tokens) である。これは出発点であり、全 workload の最適値として文書化されているわけではない。

### 文書化された事実: Contextual Retrieval は chunk への context 前置きと eval 常時実行を報告している

- [Anthropic Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval) (published Sep 19, 2024): 各 chunk の前に 50-100 token の Claude 生成 context を前置きし、Contextual Embeddings と Contextual BM25 を組み合わせる。
- 同記事の報告値 (同記事の workload における top-20 retrieval failure) は、5.7% から embeddings 単独で 3.7% (-35%)、BM25 併用で 2.9% (-49%)、さらに Cohere reranking (top-150 を top-20 に絞る) で 1.9% (-67%) である。これらは同記事の実験条件での報告であり、普遍的な改善率ではない。
- 同記事が挙げる調整・考慮事項は chunk size、chunk boundary、chunk overlap、embedding model、top-K=20、そして eval を常時実行すること (always-run-evals) である。

## 推奨方法

以下は上記 3件からの設計上のまとめであり、公式が定める実装構成そのものではない。

- retrieval の eval を固定してから chunking を変える。Context Precision@K の K (例: 報告条件に合わせた top-K=20) と reference/response 変種の選択を固定し、chunking 変更の前後比較だけを動かす。K や変種を同時に変えると、順序効果と集合効果が分離できない。
- grounding 確認は2層にする。reference がある場合は `ContextPrecision` で順序つき relevance を測る。reference がない場合は `ContextUtilization` のような response 基準の確認を足す。どちらか一方だけでは、順序の悪化または回答との乖離のどちらかを見落とす。
- chunking の出発点は fixed-size の 512 tokens (約 2000 chars)・25% overlap (128 tokens) とし、`maximumPageLength`・`pageOverlapLength`・`textSplitMode` (pages/sentences) を1つずつ変える。semantic・custom への移行は、この baseline の Context Precision@K を上回った場合に限る。
- chunk の孤立が疑われる場合に context 前置き (50-100 token) を選択肢に入れる。Contextual Embeddings と Contextual BM25 の併用、top-150 から top-20 への reranking は、eval の改善と cost・latency の代償を組にして判断する。報告値の -35% / -49% / -67% は目標値ではなく、同記事の workload での観測値として扱う。
- always-run-evals を回す。chunk size・boundary・overlap・embedding model・top-K のどれを変えても、Context Precision@K を再測定してから採用する。

## 避ける使い方

- **適合の有無だけを見て順序を無視する。** Context Precision は v_k 重み付きの mean precision@k であり、先頭の不適合 (例: 1.0 から 0.5 への低下) を捉える指標である。順序なしの適合率では ranking の悪化を見落とす。
- **reference なしの条件で reference 必須の変種を使おうとする。** reference なしでは response 基準の `ContextUtilization` 側を使うという使い分けに反する。
- **token 上限の確認なしに chunk なしの全文投入を続ける。** embedding・chat モデルの token 上限 (例: text-embedding-3-small の 8191 tokens) を超える文書は chunking の対象であり、上限未確認のまま投入しない。
- **開始点の 512 tokens・25% overlap を最適値として固定する。** 文書化されているのは開始点であり、chunk size・boundary・overlap・embedding model・top-K は eval で調整する項目である。
- **`textSplitMode` と長さ・overlap の対応付けなしに値を変える。** `pages` / `sentences` と `maximumPageLength`・`pageOverlapLength`・`maximumPagesToTake` の組で意味が決まるため、単独の数値変更は再現性を壊す。
- **Anthropic 記事の改善率 (-35% / -49% / -67%) を自 workload の保証として扱う。** 同記事の top-20 retrieval failure の報告値であり、chunk・embedding・rerank (top-150 から top-20) の条件が異なる環境にはそのまま適用できない。
- **eval なしで context 前置き・BM25 併用・reranking を積む。** 記事が求める always-run-evals に反し、cost・latency の代償だけが残る。

## 適用版と本番での注意

- Ragas `Context Precision` は stable channel の現行ページとして確認した。NonLLM 版・ID-based 版の利用条件や版ごとの導入履歴は本書の inventory に含まれないため、実装前に対応節で再確認する。
- Azure AI Search `Chunk Documents` は ms.date 2026-06-08 の版として確認した。Text Split skill の既定値 (pages 2000/500 chars の例) や token 上限の例は改訂で変わり得るため、実装前に対応ページで再確認する。
- Anthropic `Contextual Retrieval` は 2024-09-19 公開の開発元記事 (`maintainer_article`) であり、API 契約ではなく実験報告と設計指針として扱う。本文書の `trust: maintainer` は 3件中の最低信頼度に合わせたものである。
- 本文は `expires_at` 2026-12-26 (取得 2026-09-27 + `official_docs` / `maintainer_article` TTL 90日)。技術 TTL (rag) の設定はないため、source type と明示期限で判定する。
- **未確認**: chunk tokenizer と 512 tokens・2000 chars 換算の厳密な対応、sentence モードでの最適な overlap 値、rerank ベンダーの一般化、context 前置きの cost・latency、Cohere reranking の版と parameters。本書の inventory には含まれないため、推測で埋めず未確認とする。
- **未確認**: タイムアウト範囲、キャンセル、リソース所有権、シャットダウン順序。本書の inventory には含まれないため、推測で埋めず未確認とする。

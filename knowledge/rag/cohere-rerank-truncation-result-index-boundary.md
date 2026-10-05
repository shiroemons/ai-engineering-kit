---
{
  "id": "rag-cohere-rerank-truncation-result-index-boundary",
  "title": "Cohere Rerank: 文書の評価範囲・切詰め・結果indexの境界",
  "kind": "knowledge",
  "technology": "rag",
  "version": "Cohere POST /v2/rerank vs /v1/rerank; rerank-v4.0-pro / rerank-v4.0-fast (release 2025-12-11), v3.5 comparison; rolling docs retrieved 2026-10-05 UTC; chunk default / 10000 boundary discrepancy unresolved",
  "tags": [
    "research-domain:ai-engineering",
    "rag",
    "cohere",
    "rerank",
    "truncation",
    "max_tokens_per_doc",
    "max_chunks_per_doc",
    "top_n",
    "relevance_score",
    "result-index",
    "input-coverage"
  ],
  "sources": [
    {
      "id": "cohere-rerank-v2-request-contract-20261005",
      "url": "https://docs.cohere.com/v2/reference/rerank",
      "type": "official_docs"
    },
    {
      "id": "cohere-rerank-v1-request-contract-20261005",
      "url": "https://docs.cohere.com/v1/reference/rerank",
      "type": "official_docs"
    },
    {
      "id": "cohere-rerank-model-context-docs-20261005",
      "url": "https://docs.cohere.com/docs/rerank",
      "type": "official_docs"
    },
    {
      "id": "cohere-rerank-best-practices-20261005",
      "url": "https://docs.cohere.com/docs/reranking-best-practices",
      "type": "official_docs"
    },
    {
      "id": "cohere-rerank-result-index-overview-20261005",
      "url": "https://docs.cohere.com/docs/rerank-overview",
      "type": "official_docs"
    },
    {
      "id": "cohere-rerank4-release-20251211-verified-20261005",
      "url": "https://docs.cohere.com/changelog/rerank-v4.0",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2026-11-04",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# Cohere Rerank の評価範囲と結果indexを分ける

## 問い・適用範囲

RAG の候補文書に正解があるのに、rerank を挟むと回答の根拠が消える。文書全体が採点されたのか、途中で切り詰められたのか、戻った index を別の候補へ結び付けたのかを区別できるだろうか。

本書は Cohere の API 入力から rerank 結果を生成用資料へ戻すまでを扱う。[v4.0 の公開告知](https://docs.cohere.com/changelog/rerank-v4.0)は **2025-12-11** 付で、`rerank-v4.0-pro` / `rerank-v4.0-fast` と32k contextを紹介する。取得日の新機能という主張ではなく、既存文書が未確認としていた **Cohere の版・parameters・評価される本文範囲**を補う調査である。

既存の[retrieval 評価](retrieval-eval-chunking-grounding.md)はchunkingと関連度指標、[hybrid融合](hybrid-retrieval-fusion-rrf-alpha-normalization.md)は複数検索経路の順位合成、[filtered retrieval](azure-filtered-retrieval-candidate-starvation.md)は検索時の候補枯渇を扱う。本書はそれらの後段で起こる入力損失に絞る。[引用座標](citation-evidence-coordinate-evaluation.md)の回答rendererや引用の支持判定も作り直さない。他provider・クラウド経由のAPIへ以下の契約を一般化しない。

## 一次資料で確認した契約

### 1. model context と API の文書上限は別の設定

[Model documentation](https://docs.cohere.com/docs/rerank)は、各文書についてqueryとdocumentのtoken合計をcontext limitへ数え、超過時に内部chunkへ分けると説明する。Rerankは与えられた検索結果を並べ替えるもので、元の検索に戻って未提供の文書を発見するAPIではない。

一方、[v2 reference](https://docs.cohere.com/v2/reference/rerank)は `max_tokens_per_doc` の既定を **4096** とし、長い文書を指定token数へ自動的に切り詰めると記載する。`documents` は文字列配列、`model` は必須。`top_n` は**返す結果数**を制限し、省略時は全結果を返す。1000文書以下は性能上の推奨であり、同ページの記述だけで厳密な受付上限とは呼ばない。

したがって「v4は32kだから、既定requestでも32kの本文を評価する」とは推論できない。modelの処理能力とendpointの入力制御を別項目で設定・検証する。内部chunkingの説明だけで、APIの切詰め後に失われた範囲まで採点されたとは扱わない。これは資料の突合せから導く運用上の判断であり、server内部の処理順序を実測した結果ではない。

### 2. v1 の chunk 上限を v2 の token 上限へ機械変換しない

[v1 reference](https://docs.cohere.com/v1/reference/rerank)には、文字列またはobjectの `documents`、評価するfield順を指定する `rank_fields`、元文書を返す `return_documents`、内部chunk数を制限する `max_chunks_per_doc` がある。返却本文は送った文書であり、採点に使われた最良chunkの抜粋という契約ではない。

v2 referenceの入力形状は別である。objectやv1専用fieldをそのまま移植するのではなく、評価対象のfieldだけを自前で文字列化する。[Overview](https://docs.cohere.com/docs/rerank-overview)は構造化データをYAML文字列にすることを推奨する。[Best practices](https://docs.cohere.com/docs/reranking-best-practices#structured-data-support)は切詰めがあるためkey順が重要だと注意している。fieldの選択・順序・直列化も評価条件に含める。

取得時点には次の不一致がある。**資料ごとの記述を残し、serverの既定値を断定しない。**

| 項目 | v1 reference | Best practices | 本書の扱い |
|---|---|---|---|
| `max_chunks_per_doc` 既定 | 10 | 1 | v1では意図した値を明示し、既定依存の回帰を避ける |
| 文書数×最大chunk数の境界 | 10000未満 | 10000超過でerror | ちょうど10000の可否を未確認とし、境界未満で設計する |

Best practicesの `max_chunks_per_doc` 説明を、v2が同fieldを受け付ける証明にしない。token数はquery長・model・serializationにも左右されるため、`max_chunks_per_doc × 4096` を等価なv2設定とみなさない。

### 3. 文書スコアと根拠spanを同一視しない

[Best practices の Document Chunking / Interpreting Results](https://docs.cohere.com/docs/reranking-best-practices)は、内部chunkのscoreを最大値で文書scoreへ集約する例を示す。また `relevance_score` はquery依存で、順位付けのための値であると説明する。数値をそのまま回答の正しさの確率へ読み替えない。

[Overview の返却例と説明](https://docs.cohere.com/docs/rerank-overview#example-with-texts)では、`index` は入力 `documents` の **0始まりの位置**である。出力配列内の順位とは別であり、document IDや原文のoffsetでもない。通常の結果の `index` / `relevance_score` だけでは、どの内部chunkが最高点だったかは取得できない。

## 独自の設計案: 評価範囲を失わないrerank adapter

以下は上記資料を踏まえたアプリ側の提案であり、Cohereの必須実装や追加API fieldではない。

### 送信前: 候補を固定してから評価用表現を作る

1. **検索候補とrerank入力を別に数える。** 検索候補に必要な文書がない、候補にはあるが評価用本文に根拠がない、評価済みだがtop_nで落ちる、という失敗を区別する。
2. **送る候補配列をattempt単位で固定する。** 同じ順序でローカルの対応表を持つ。例となる列はinput index、安定document ID、原文revision、評価用chunk ID、送信文字列の識別値。送信後に配列をsort・deduplicate・置換しない。
3. **評価用projectionを明示する。** タイトル、必要な条件、本文などのfield順を固定し、長大なmetadataが本文の前で予算を使い切らないようにする。全文objectを無条件にYAMLへ落とすのではなく、認可済みで必要なfieldを選ぶ。文字数をtoken数と呼ばない。
4. **入力budgetをendpointごとに持つ。** v2では `max_tokens_per_doc` を明示し、queryとmodel contextの余裕も確認する。採用tokenizerやreserved tokenまで検証できていない状態では、上限ぎりぎりの文字数換算で全文収容を保証しない。
5. **必要ならアプリ側でchunkを作る。** 長い親文書の後半が重要なら、根拠を含む単位へ事前分割し、それぞれを別の入力documentとして送る。親IDとchunk IDを両方保持し、どの範囲を採点したか追えるようにする。

事前chunkingは常に品質を改善するわけではない。断片化で条件が失われたり、同じ親文書の多くのchunkが上位を占有したりする。本書の提案では、親文書ごとの上限・重複処理とその適用位置を固定し、分割方式だけを比較する。内部chunkingのmax集約をアプリ側で無条件に再現すれば最適になる、という主張はしない。

### 受信後: indexから同じattemptの入力へ戻す

- `results` の各要素の `index` を、送信時に固定した配列へ引く。返却順の0番を入力の0番へ結ぶ `zip` 型の処理を使わない。
- indexの型・範囲を検査し、存在しない番号は不正結果として扱う。別attemptの入力配列や「最新の検索結果」で補完しない。外部serviceの応答が不完全なら、その状態を残す。
- 先にfilter・deduplicate・chunk展開した場合、その操作後の送信配列がindexの基準である。元の検索配列とは別の番号空間にする。
- 同じ本文が異なる版・権限・親資料に属する場合、本文文字列一致だけで対応を決めない。表示前の認可確認もscoreで代替しない。
- 高得点の親文書の先頭だけを生成モデルへ渡さない。根拠位置が不明なら、アプリで評価したchunkを使うか、親文書の根拠抽出を別工程にする。rerankが採点した範囲と生成に渡した範囲を同一と仮定しない。

### 小例: 順位の変更を文書の変更にしない

送信配列が `[契約Aのchunk, 仕様Bのchunk, 手順Cのchunk]` で、返却が `index: 2`, `index: 0` なら、生成に使う候補順はC、Aである。入力先頭2件のA、Bへscoreを貼ると、HTTP成功でも違う文書を採用する。

さらにCの評価用chunkが親文書の後半から切り出されているなら、そのscoreを親文書の先頭段落へ付け替えない。ここで必要なのは正しいchunkの選択であり、回答生成後のcitation offsetの修復ではない。

## 独自の受入れテスト案と運用判断

以下は未実施のfixture案。APIを呼んだ実測結果や、このKBの検索evalが検証する振る舞いではない。

| fixture | 確認すること |
|---|---|
| 唯一の正解文を長文の末尾へ移す | 既定request・明示budget・事前chunkingで比較し、末尾の根拠消失をmodel能力不足と即断しない |
| query長だけを増やす | 文書側の利用可能contextが同じとは扱わず、根拠を含む範囲の採点を確認する |
| YAMLのmetadataを長くし、field順を交換 | serializationの変化による評価範囲・順位の違いを検出する |
| 返却indexが `[2,0]` | 入力C、Aを返し、A、Bへscoreを誤接続しない |
| 送信前に候補をfilterする | response indexをfilter後の配列へ解決する |
| 同一本文・異なるrevisionの2候補 | 本文一致で統合せず、送ったIDとrevisionを維持する |
| `top_n` だけを小さくする | 候補不足、返却打切り、入力truncationを別区分で記録する |
| 最高点の根拠が親文書の後半 | 最終contextにもその根拠が残るかを確認する |
| API失敗・timeoutでbaseline順位へ戻す | fallbackを明示し、rerank成功として品質集計しない |
| v1からv2への移行 | request形状、budget、入力ID対応を先に検査し、その後で品質を比較する |

比較にはcorpus revision、候補集合、query、model ID、endpoint、serialization版、budget、top_n、生成側context予算を残す。全文を無期限にログへ複製することを前提にせず、既存の機密区分・保持期間内で再現に必要な対応を保持する。

品質は入力coverage、rerank後の順位、最終contextの根拠保持を分けて測る。例えば「検索候補には正解があるが、評価用projectionには正解文がない」群を独立させる。score thresholdを移行前後で固定して合否だけ比べると、入力範囲の変化とscore分布の変化が混ざる。まず順位と根拠保持を比較し、その後に業務上許容する誤採用・取りこぼしに基づいてthresholdを再評価する。

timeout時に元順位で続行するか、回答保留にするかは製品要件で決める。budgetを上げるだけのretryや文書数を増やすだけの対策を自動で繰り返さない。候補集合のどの段階で正解が失われたかを調べてから変更する。

## 出典・適用版・未確認事項

- 上記6件の一次資料を **2026-10-05 UTC** に開いて確認した。v4.0公開日は2025-12-11。他5件はrolling documentationで、独立した公開日・改訂日は確認できず取得日で識別した。v3.5はmodel比較の対象であり、当該APIの導入最低SDK版までは確定していない。
- API referenceとguideの既定値・受付境界の不一致は未解決。利用予定endpointと明示parametersで検証するまでは、どちらかを実挙動と断定しない。最大値の受理、tokenizer、特殊token、実際の切詰め位置と内部処理順序もAPI実測していない。
- model weights、server実装、SDK codeのrepository分析は行っていないためcommit pinはない。開いたWebページから再利用ライセンスを確認できず、catalogは `unknown`。リンク付きの独自要約と設計案のみで、公式コード・図の転載やmodule昇格は行わない。
- release_notesの30日TTLが最短なので明示期限は **2026-11-04 UTC**。この期限は再確認日であり、モデルの終了日ではない。
- 未確認: Bedrock等の別endpointの互換性、課金単位、実運用の遅延・throughput、model間のscore較正、top_n変更時の計算量、文書並列batch間のscore比較可能性、提案したfixtureの実行結果。公開モデルの広告上の品質差を自データの保証として使わない。
- 追加する `evals/knowledge/ai-engineering.json` のcaseは本書を検索できるかの検査だけである。RAG品質・truncation・API移行が合格したことを意味しない。

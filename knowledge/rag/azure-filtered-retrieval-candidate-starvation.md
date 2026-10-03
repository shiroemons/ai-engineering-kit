---
{
  "id": "rag-azure-filtered-retrieval-candidate-starvation",
  "title": "Azure AI Search の filtered RAG: filter適用位置・候補枯渇・exact基準の境界",
  "kind": "knowledge",
  "technology": "rag",
  "version": "Search Service REST 2026-04-01 (stable) / 2026-08-01-preview; rolling guides retrieved 2026-10-03 UTC; strictPostFilterのpreview表記とstable enumに不一致あり、service実測なし",
  "tags": [
    "research-domain:ai-engineering",
    "rag",
    "azure-ai-search",
    "filtered-retrieval",
    "preFilter",
    "postFilter",
    "strictPostFilter",
    "filterOverride",
    "candidate-starvation",
    "exhaustive",
    "recall"
  ],
  "sources": [
    {
      "id": "azure-search-vector-filters-20261003",
      "url": "https://learn.microsoft.com/en-us/azure/search/vector-search-filters",
      "type": "official_docs"
    },
    {
      "id": "azure-search-hybrid-filter-override-20261003",
      "url": "https://learn.microsoft.com/en-us/azure/search/hybrid-search-how-to-query",
      "type": "official_docs"
    },
    {
      "id": "azure-search-documents-stable-filter-20261003",
      "url": "https://learn.microsoft.com/en-us/rest/api/searchservice/documents/search-post?view=rest-searchservice-2026-04-01",
      "type": "official_docs"
    },
    {
      "id": "azure-search-documents-preview-filter-20261003",
      "url": "https://learn.microsoft.com/en-us/rest/api/searchservice/documents/search-post?preserve-view=true&view=rest-searchservice-2026-08-01-preview",
      "type": "official_docs"
    },
    {
      "id": "azure-search-vector-ranking-exact-20261003",
      "url": "https://learn.microsoft.com/en-us/azure/search/vector-search-ranking",
      "type": "official_docs"
    },
    {
      "id": "azure-search-security-filter-pattern-20261003",
      "url": "https://learn.microsoft.com/en-us/azure/search/search-security-trimming-for-azure-search",
      "type": "official_docs"
    },
    {
      "id": "azure-search-api-version-list-20261003",
      "url": "https://learn.microsoft.com/en-us/rest/api/searchservice/search-service-api-versions",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# Azure AI Search の filtered RAG: filter適用位置・候補枯渇・exact基準の境界

## 調査の問いと範囲

tenant・権限・カテゴリで対象を絞った RAG が「資料なし」と答えたとき、本当に該当文書がないのか、それとも vector retrieval の候補が filter 前に切られたのか。filter の適用位置、各検索経路の有効な条件、評価の正解集合を分けて判断する。

これは既存の [hybrid融合](hybrid-retrieval-fusion-rrf-alpha-normalization.md) や [chunking評価](retrieval-eval-chunking-grounding.md) の前段にある未収録の運用課題を扱う。2026年10月の新機能発表ではない。2026-10-03 UTC に公開一次文書を開き、現行の版差を照合した。以下の「確認した契約」は文書の事実、「設計提案」「計算例」は本書独自の整理である。Azure serviceへの送信、負荷測定、SDK実行はしていない。

## 確認した契約

### 1. filterの位置が違えば、同じ条件とkでも候補集合が変わる

[Vector filters guide](https://learn.microsoft.com/en-us/azure/search/vector-search-filters)（更新表示2026-04-27）は、filterを `filterable` な非vector fieldに適用し、HNSWをshardごとに探索すると説明する。

| mode | filterを適用する位置 | 結果への影響 |
|---|---|---|
| `preFilter` | 各shardのHNSW探索中 | 条件に合う候補を探してからglobal結果へ集約 |
| `postFilter` | 各shardの未filter local top-kの後 | local候補から落ちた適合文書は集約に来ない |
| `strictPostFilter` | 未filter global top-kの後 | global候補の部分集合だけになり、適合文書が存在しても0件になり得る |

同guideはpreFilterについて、適合文書が十分あればk件を返すと説明する。少数だけ通す条件では探索量・latencyが増え得る。両postfilterでkを増やすことは欠落の緩和策だが、filter後の件数を固定する契約ではない。strictPostFilterの目的はfacet操作で未filter結果との部分集合関係を保つことであり、RAG recallの強化ではない。

**件数充足とexact recallは別**である。[Vector relevance guide](https://learn.microsoft.com/en-us/azure/search/vector-search-ranking)（更新表示2026-01-21）はHNSWを近似探索、exhaustive KNNをexact nearest neighborsとし、後者をANN評価のground truthに使えると説明する。したがってpreFilterの件数説明から「同じ距離尺度のexact top-kを必ず回収する」とは推論しない。HNSWでindexしたfieldでも `exhaustive: true` を指定できるが、exhaustiveKnn専用fieldからHNSW探索へ逆に切り替えるためのgraphはない。

### 2. 2026年のstable / preview契約を混同しない

[API version一覧](https://learn.microsoft.com/en-us/rest/api/searchservice/search-service-api-versions)は、確認時点で `2026-04-01` をstable、`2026-08-01-preview` をpreviewとしている。API識別子の日付は、そのページの公開日を示すとは限らない。

| 確認対象 | strictPostFilter | vectorQueries.filterOverride |
|---|---|---|
| [2026-04-01 REST reference](https://learn.microsoft.com/en-us/rest/api/searchservice/documents/search-post?view=rest-searchservice-2026-04-01) | VectorFilterMode enumに掲載 | VectorizedQueryのproperty表に掲載なし |
| [2026-08-01-preview REST reference](https://learn.microsoft.com/en-us/rest/api/searchservice/documents/search-post?preserve-view=true&view=rest-searchservice-2026-08-01-preview) | enumに掲載 | propertyとして掲載 |
| filter / hybrid how-to | strictPostFilterにpreview表記 | filterOverrideにpreview表記 |

**文書間の不一致は未解決**。strictPostFilterのstable enum掲載だけでGAやSLA適用を断定しない。一方、how-toのpreview表記だけからstable APIで必ず拒否されるとも断定しない。how-toのラベル更新遅れ等は可能性にすぎず、原因・正式なGA時期・実serviceでの対応は未確認である。

設計提案: API版・service・indexを固定した互換性テストと、production採用に必要なサポート状態の確認を分ける。リクエストが200になることだけでは後者を証明できない。indexの歴史も必要で、filter guideは約2023-10-15より前のindexではpostFilterが既定、preFilterを使うにはindex再作成が必要とする。日時だけで対応可否を決めず、指定したmodeでのqueryを検証する。

### 3. filterOverrideは追加のAND条件ではない

[Hybrid query guide](https://learn.microsoft.com/en-us/azure/search/hybrid-search-how-to-query)（更新表示2026-08-06）は、vector用filterOverrideがtop-level filterを**置換**し、keyword側には影響しないと説明する。security trimmingなど必須の条件はtop-levelと各vector-levelの両方に含める必要がある。[preview REST reference](https://learn.microsoft.com/en-us/rest/api/searchservice/documents/search-post?preserve-view=true&view=rest-searchservice-2026-08-01-preview)も、vector-level filterが未指定ならtop-levelを使うと定義している。「global filterはvectorには一切効かない」という一般化は誤りである。

設計提案として、必須条件をA、keyword固有条件をB、各vector固有条件をC_iと表すなら、top-levelは `A AND B`、overrideを使う各vectorには `A AND C_i` を明示する。これは説明用の論理表記であり、コピーして送信するODataコードではない。overrideを省略したvectorは `A AND B` を継承する。Bも全経路必須なら各C_i側にも含める。複数vectorの一つだけAを落とすケースを必ず検査する。

[Security filter pattern](https://learn.microsoft.com/en-us/azure/search/search-security-trimming-for-azure-search)（更新表示2026-08-24）は、principalはfilter内の文字列であり、その文字列自体を使った認証・認可は行われないと明記する。本書のAは認証済みの利用者から信頼できる側で導出することを前提とした設計上の記号である。LLMやクライアントに任意のAを生成させれば権限制御が成立するという意味ではない。組込みACLの方式・認証設定・権限更新伝播は本書の対象外。

## 独自の計算例: 0件でも「資料なし」とは限らない

次はfilter位置だけの差を示す**手計算用モデル**であり、Azureの実測でもHNSWの近似結果の予測でもない。距離は小さいほど近く、tieなし、k=2、各shardが理想的な距離順を返すと仮定する。

| shard | 距離の昇順 | filterに適合するID |
|---|---|---|
| A | a1=1, a2=2, a3=3, a4=4 | a3, a4 |
| B | b1=5, b2=6, b3=7 | b1, b3 |

- filtered集合のexact top-2は `{a3, a4}`
- preFilterの位置で選別すれば、Aからa3/a4、Bからb1/b3が候補になり、global結果はa3/a4
- postFilterではAのlocal top-2が両方落ち、Bからb1だけ残る。結果1件は「適合文書が1件しかない」という意味ではない
- strictPostFilterではglobal top-2のa1/a2が両方落ち、結果0件。それでも適合文書は4件ある

このモデルでは最後に `top` を増やしても、失われたa3/a4は作り直せない。kの拡大やfilter位置の変更を検討する段階と、生成モデルに渡す件数を決める段階を分ける。許可外文書を追加して件数を埋めるfallbackは設計しない。

## 設計提案: 同じfiltered集合を分母に評価する

### 評価条件を固定する

1. query vector、embedding版、距離尺度、量子化・圧縮条件、index内容、filterの有効式を固定する。比較中に文書・権限・vectorが変わったqueryは別のrunにする
2. vector-only、1query・1field・1文書1vectorの単純な検証から始める。表示段階の打切りやページングを混ぜず、全返却候補を照合する。hybrid融合、semantic reranker、score threshold、multi-vector重複排除を同時に変えない
3. **同一filter**を使い、その適合集合F内のexhaustive KNNで正解集合Gを作る。HNSWのpreFilterでもexactの代用品にはしない。サービスで基準queryを実行する際はfilterとmodeが意図どおり適用されることを検証する
4. 評価kを固定し、`G = exact top-min(k, |F|)`、比較対象の返却ID集合をRとする。tieが境界にあればtie-aware照合等の規則を先に決める

### 本書で用いる診断指標

まずRがFの部分集合であることを確認する。条件外IDが返った場合は契約違反として評価を失敗にし、件数やrecallの高さで相殺しない。

- `filtered Recall@k = |R ∩ G| / |G|`。分母は無filterの全体top-kでも、返却された件数でもない
- `|F|=0` なら分母0なのでrecallは **N/A**。1.0へ置換せず、正解なしquery件数・正しく空になったかを別報告する
- `0<|F|<k` のときは分母を |F| とする。例えばk=10、適合3件をすべて回収した場合は3/3であり3/10ではない
- 候補充足率は、|F|>0かつ上記の単純なvector-only条件で `min(|R|,k) / min(|F|,k)`。0件率、返却件数、recallを併記し、k件返っただけで回収品質の合格とはしない
- |F|、`selected fraction = |F| / 全対象文書数`、shardへの適合文書の偏りごとに層別する。ここで選択性は「残る割合」であり除外率ではない。全queryの平均だけでは小さいtenantの欠落を隠す
- kを変える実験では各kで基準Gも作り直す。latencyのp95/p99、timeout・失敗率も記録し、成功した高速queryだけの平均にしない

これらの式と分母・集計規則は本書独自の評価設計であり、Azureの課金・SLA・サービス保証ではない。exact近傍へのrecallはembedding空間の近似誤差を測る。人間が必要とする根拠文書の関連性や回答の正確さは、別のラベル付き評価で確認する。

### hybridへ戻すときの確認

hybrid guideはkをvectorから融合へ供給する件数、topを最終response件数として区別する。semantic rankerには最大50件が入り、vectorと併用するときk=50を勧める。しかしfilter後に候補が減るため「k=50指定 = 50件の有効入力」という式にはならない。

設計提案: 各vector経路、keyword経路、融合後、生成モデルへ渡した段階のIDと件数を別々に確認する。keywordが文書を補って最終top件数を満たしても、vector経路の枯渇は残り得る。rerankingのパラメータ調整より先に、どの経路で正解IDが消えたかを調べる。モデル出力だけを採点すると、この違いを診断しにくい。

## 避ける判断と検証ケース

| 判断・変更 | 必要な検査 |
|---|---|
| 0件だから資料が存在しない | 同一filterの基準Gが非空か。postfilterの候補欠落か |
| k件返ったからexact recallは十分 | 件数と `R ∩ G` を別評価 |
| filterOverrideにカテゴリだけ足す | 継承ではなく置換としてAの残存を全vector経路で検査 |
| kを増やしたのにtop件数が同じだから無意味 | 融合前候補ID・recallを比較。表示件数だけで判定しない |
| 回収不足なのでtenant/権限filterを外す | 必須条件を維持したままmode・k・queryを検討。許可外IDは失敗 |
| stable APIなので全enumの提供状態も解決済み | strictPostFilterの文書間不一致とservice/index互換性を別確認 |
| migration後も既定modeは同じ | 旧index・再作成indexの両方でmodeを明示して検証 |

回帰シナリオには、Fが空、1件、k未満、k以上、適合IDが全体top-k外、特定shardに偏る、多経路の一つだけoverrideあり、全overrideあり、overrideなし、の組合せを用意する。上表の計算例は期待するfilter位置を説明するfixtureになるが、実serviceの近似rankをそのまま固定するテストにはしない。

## 出典・provenance・未確認事項

- 参照7件はMicrosoftの一次文書。本文内リンクとfront matterのcatalog IDに対応する。API referenceの公開日は表示を確認できず、API版と取得日を記録した。how-toの更新日は公開日とは区別した
- 文書の転載ライセンスは確定していないためcatalogは `license: unknown`。原文・サンプルコード・図の転載は行わず、独自に要約した。数値例、記号、評価式、チェック表は本書で作成した。repositoryコード分析はしていないためcommit SHAは空
- filterOverrideの初回導入版、strictPostFilterの正式なGA時期、文書不一致の理由、空文字overrideの扱い、region/SKU/SDKごとの対応は未確認。未知の版へ横展開しない
- 自データでの最適k、CPU/QPS改善、厳密なshard配置、index更新中のsnapshot整合性、ACL更新後の反映時間、実serviceの認証や権限設定は未検証。API受理・性能・認可を1回の検索成功で保証しない
- 期限は通常official_docsの90日より短い2026-11-02を明示した。preview境界と文書不一致を早めに再確認するためであり、原文の事実がその日に失効するという意味ではない

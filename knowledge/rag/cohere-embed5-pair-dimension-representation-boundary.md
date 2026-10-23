---
{
  "id": "rag-cohere-embed5-pair-dimension-representation-boundary",
  "title": "Cohere Embed 5: Pro/Fastの共有空間・次元・表現形式を分ける検索投入境界",
  "kind": "knowledge",
  "technology": "rag",
  "version": "Cohere embed-v5.0-pro / embed-v5.0-fast (release 2026-09-30); POST /v2/embed; Embed 4 default-dimension comparison; rolling docs retrieved 2026-10-05 UTC; live API untested",
  "tags": [
    "research-domain:ai-engineering",
    "rag",
    "Cohere",
    "Embed5",
    "shared-space",
    "encoder-pair",
    "input_type",
    "output_dimension",
    "embedding_types",
    "packed-binary",
    "Matryoshka",
    "truncate"
  ],
  "sources": [
    {
      "id": "cohere-embed5-shared-space-release-20261005",
      "url": "https://docs.cohere.com/changelog/embed-v5",
      "type": "release_notes"
    },
    {
      "id": "cohere-embed5-model-dimensions-20261005",
      "url": "https://docs.cohere.com/v2/docs/cohere-embed",
      "type": "official_docs"
    },
    {
      "id": "cohere-embed-v2-input-contract-20261005",
      "url": "https://docs.cohere.com/v2/reference/embed.md",
      "type": "official_docs"
    },
    {
      "id": "cohere-embed5-retrieval-input-roles-20261005",
      "url": "https://docs.cohere.com/v2/docs/semantic-search-embed",
      "type": "official_docs"
    },
    {
      "id": "cohere-embed5-binary-packing-20261005",
      "url": "https://docs.cohere.com/docs/embeddings",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2026-11-04",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/ai-engineering.json"]
}
---

# Embed 5の共有空間を検索投入の契約にする

## 問いと今回の変更

文書は高品質なencoder、問い合わせは低遅延なencoderで埋め込みたい。同じ次元の配列が返れば、既存のvector indexへそのまま送ってよいだろうか。

[2026-09-30の公式発表][release]は、`embed-v5.0-pro` と `embed-v5.0-fast` が同じembedding空間を共有し、一方で索引化したcorpusを他方で検索できると明示した。文書をPro、問い合わせをFastにする構成が開発元の推奨である。本書はこの具体的な新しい組合せを扱う。異なる任意のモデルを、配列長だけで互換とする話ではない。

既存の[Qdrant移行](qdrant-embedding-migration-cutover-consistency.md)はbackfill、同時更新、切替・rollbackの整合性を扱う。本書はその前段にあるencoderと出力表現の契約検査を対象にする。[Cohere Rerank](cohere-rerank-truncation-result-index-boundary.md)の候補再順位付けとも異なり、ここでは検索用vectorを生成する。

## 一次資料で確認した契約

### 1. 共有空間と既定次元は別の情報

[モデル一覧][models]では、Embed 5の両モデルが `256, 512, 768, 1024, 1536, 2048` 次元を提供し、既定値は2048である。比較対象の `embed-v4.0` は `256, 512, 1024, 1536` 次元で、既定値は1536。両世代とも入力上限は128k tokensと記載される。

公式発表の共有空間はPro/Fastの組に対する説明である。本調査ではEmbed 4と5の共有空間を保証する記述を確認していない。両方に1536があることを、その保証の代用にしない。

### 2. 検索する側と検索される側の入力用途

[Semantic Search guide][roles]は、文書を `input_type="search_document"`、問い合わせを `input_type="search_query"` として生成する。例では双方の `output_dimension` を1024、`embedding_types` を `["float"]` としている。これは入力用途の対応であり、検索語と文書に同じroleを付ける指定ではない。

同guideのサンプルはPro/Proを使う。Pro/Fastの互換性の根拠は[発表][release]であり、サンプルのモデル名を読み替えて実測済みと報告しない。

### 3. APIの件数・切詰め・表現指定

[Embed API v2 referenceの公式Markdown版][api]で確認した範囲は次のとおり。

- `input_type` はv3以降で必須。`output_dimension` はv4以降で指定できる
- `texts` と `inputs` はそれぞれ1回96要素まで。これは個々の文章のtoken上限ではない
- `max_tokens` は各入力の埋め込み対象token数の上限。`truncate` の既定は `END` で、`START` は先頭、`END` は末尾を捨てる。`NONE` はモデルの最大入力長を超える場合にエラーとする
- `embedding_types` は複数表現を要求でき、既定は `["float"]`。応答の各表現の配列について、text入力数と同じ数のembeddingを返すと記載される

`max_tokens` の明示値を超えた入力で `truncate="NONE"` を使う細部は、本調査ではlive APIで検証していない。error statusやtoken境界を推測した独自実装にはしない。

### 4. binaryの配列長は論理次元と一致しない

[Embeddings guide][packing]は、Matryoshkaの次元を `output_dimension` で選ぶこと、複数の出力表現を同時に取得できることを説明する。binaryではbit列をbyteへ詰めるため、1024次元なら配列は128 byteになる。これは128次元のfloat vectorとは別の表現である。

同ページの説明文には `embeddings_types` という表記もあるが、サンプルとAPI referenceのparameterは `embedding_types`。本書は後者を用いる。この表記差をSDKやサービスの障害と断定しない。

## 独自の設計案: 名前の一致より、許可する組合せを記録する

以下は本書の設計提案で、Cohereが指定する保存schemaではない。新しい検索経路には、次の項目をひとまとまりの設定として持たせる。

| 項目 | 検査したいこと |
|---|---|
| document model / query model | 組合せの根拠を明示できるか。単純なモデル名一致も、任意の名前の混在も条件にしない |
| document role / query role | 索引用と検索用の入力用途を取り違えていないか |
| logical dimensions | 明示設定と返却値、保存schema、query側が対応するか |
| representation / codec | float、整数、packed bit列を区別し、decoderと比較器を固定しているか |
| distance / score interpretation | 採用する検索engineの距離と順位方向を確認したか |
| preprocessing / truncation policy | 本文の抽出・正規化と、どの範囲を埋め込むかが記録されているか |
| validation evidence | 組合せを検証した資料取得日と、自社の品質・遅延試験を区別して残したか |

設定の識別子を、生成job、検索request、保存先schemaに結び付ける。識別子の一致だけで品質が証明されるわけではないが、違う設定を黙って混ぜる事故を入口で検出できる。

### 導入時の判断例

以下は上記資料から導く判断案であり、APIを実行した結果ではない。

| 状況 | 判断 |
|---|---|
| 検証済みのPro文書indexを残し、queryを同世代Fastへ変更 | 公式の共有空間を根拠に候補とする。次元・表現・入力用途を保ち、品質と遅延を比較する |
| 旧indexは1536次元、query側の新モデルで次元指定を省略 | 既定値の変更を検出し、検索へ送らない。モデル変更とschema変更を別々に見逃さない |
| Embed 4文書とEmbed 5 queryを同じ1536に揃える | 次元検査だけでは採用しない。今回確認した互換範囲外として隔離する |
| Pro/Fastに違う次元を明示 | 共有空間の説明だけで長さの違う配列を比較しない |
| queryを `classification` 用に生成 | 正常なAPI応答でも、検索経路の入力用途と不一致として検出する |
| 同じ長さのint8配列をfloatとして読み込む | 自動castで対応済みにしない。量子化表現に対応した検索方式を別に検証する |

サービスが未知のモデル名へfallbackした場合も、HTTP成功を理由に許可済みの組合せへ登録しない。検証済み経路へ戻すか、そのrequestを未対応として扱う。互換性の表はモデル名のprefix一致で自動拡張しない。

## 独自の設計案: shape検査を二段に分ける

外側は「何入力分か」、内側は「一つをどう符号化したか」である。text専用の取込adapterなら、送信したN件のID・本文版・順序を固定し、要求した各表現でN件のvectorを回収したか検査する。複数表現が返っても、同じ文書をその数だけ増やさない。

floatだけを採用する経路では、一つのvectorの要素数を明示した次元と照合し、非数や無限大などを保存前に拒否する。これは自社の入力検証案であり、providerがそのような値を返すという観測ではない。

packed-binary専用経路は、byte数と論理次元を別fieldで管理する。符号付き/符号なしのbyte、bitの展開順、検索engineのbinary用比較をfixtureで合わせる。配列が短いからといってゼロを足したり、通常のfloat用cosineへ渡したりしない。Matryoshka対応という説明だけを根拠に、手元で任意にsliceした値がAPIの指定次元と完全に同じになるとも仮定しない。まず公式parameterで生成する。

入力範囲の管理もvector形状から分離する。短縮しても返却次元が同じなら、shape検査は通ってしまう。全文を対象にする業務では、入力長の検査と分割方針を決め、切詰めを許可する経路ならその方針を索引の版に含める。長い文書の末尾にだけ重要な但書があるfixtureを置き、返却成功を全文反映の証拠にしない。

## 独自の評価案: 互換性と検索品質を別のゲートにする

最初のゲートは既知のencoder pair、role、shape、codec、検索先schemaの対応を検査する。ここに不一致があれば、関連文書が偶然1件見つかっても合格にしない。

次のゲートではcorpusと正解ラベル付きqueryを固定し、次元・表現・filter・kを揃えてPro/ProとPro/Fastを比較する。費用を含む必要があれば同じ負荷条件で測る。Fast/Fastなど別の候補は別runにする。正解文書のRecall@k、問い合わせ群別の退行、query生成のp95遅延、索引生成時間を別々に記録し、平均値だけで日本語や短い識別子の退行を隠さない。

共有空間の契約は、同一vector、同一スコア、同一順位、全用途での品質同等を意味するという記述ではない。検索しきい値を使う製品は、モデル・次元・表現を変えるたびに正例/負例で較正する。類似度スコアを回答の正しさの確率として表示しない。採用基準を結果を見る前に定め、公式の推奨を自社SLOの達成と取り違えない。

## 採用前のfixture案

以下は将来の実装で行うテスト案で、本リポジトリでのAPI実測ではない。

| fixture | 期待する検査 |
|---|---|
| 許可済みPro/Fast、同じ明示次元とfloat | 契約ゲートを通し、独立した品質試験へ進む |
| 1536次元の旧schemaへ既定値の新query | 不一致を検索前に検出する |
| 異なる世代を同じ次元へ設定 | モデル名を除いたshape検査では不十分と分かる |
| query側だけ入力用途を変更 | 入力roleの不一致を検出する |
| textを2件、出力表現を2種類要求 | 文書数は2件のまま。各表現の回収数を検査する |
| 1024論理次元のbinaryを128 byteで保存 | 論理次元を128へ書き換えない |
| packed bit列をfloat経路へ誤投入 | codecの不一致で停止し、paddingで通さない |
| vectorは正しい長さだが末尾の但書を切詰め | shape合格と全文反映を区別する |
| 未知のfallbackモデルが正常応答 | 許可したencoder pairの外として扱う |
| 平均recallは維持、日本語群だけ悪化 | 全体平均で覆わず、群別の採用基準で判断する |

## 版・取得方法・ライセンス・未確認事項

- 公式発表の公開日は2026-09-30。モデルIDは `embed-v5.0-pro` / `embed-v5.0-fast`。5件とも2026-10-05 UTC取得。発表以外のページに独立した公開日・固定revisionは表示されておらず、取得時点のrolling documentationとして扱う
- Release、モデル一覧、Semantic Search、Embeddingsの実ページはnative webで開いた。API referenceのHTMLは取得toolでサイズ上限エラーとなり、公式の `.md` URLを読み取った。取得できなかったHTMLだけを証拠にしていない
- 公開資料の本文でオープンライセンスを確認できず、catalogは `unknown`。コード・図・vector値・例文を転載せず、要約と独自の設計例に限定した。repository実装は解析していないためcommit固定の対象はない
- 未確認: live APIとSDKの動作、Pro/Fastの実測品質・速度、契約モデルIDの背後の不変snapshot、手動sliceと指定次元の数値同一性、量子化codecの全仕様、全cloud配備の同等性、画像・混合入力の詳細上限、全入力方式を横断する応答順序の保証
- 画像の件数・サイズについてguideとreferenceには説明範囲の差があるため、text専用の本書では共通の上限を推測しない。特にtextの96件上限を画像全体へ拡張しない
- release_notesの30日TTLが最短なので期限は2026-11-04。`evals/knowledge/ai-engineering.json`は本文を発見する検索evalであり、上記fixtureやCohere APIの互換性試験ではない

[release]: https://docs.cohere.com/changelog/embed-v5
[models]: https://docs.cohere.com/v2/docs/cohere-embed
[api]: https://docs.cohere.com/v2/reference/embed.md
[roles]: https://docs.cohere.com/v2/docs/semantic-search-embed
[packing]: https://docs.cohere.com/docs/embeddings

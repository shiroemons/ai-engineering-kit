---
{
  "id": "rag-qdrant-embedding-migration-cutover-consistency",
  "title": "Qdrant RAG の embedding 移行: backfill・query encoder・rollback の整合性境界",
  "kind": "knowledge",
  "technology": "rag",
  "version": "Qdrant migration guide tested with 1.19.0; named vector schema mutation 1.18+; v1.19.0 OpenAPI @ 74f3e85b9473c62560006c043e13737ce6b48412; rolling docs 2026-10-03 UTC; insert_only導入下限に資料不一致あり",
  "tags": [
    "research-domain:ai-engineering",
    "rag",
    "embedding",
    "qdrant",
    "model-migration",
    "named-vectors",
    "backfill",
    "dual-write",
    "cutover",
    "rollback",
    "query-encoder",
    "source-revision"
  ],
  "sources": [
    {
      "id": "qdrant-embedding-migration-guide-20261003",
      "url": "https://qdrant.tech/documentation/tutorials-operations/embedding-model-migration/",
      "type": "official_docs"
    },
    {
      "id": "qdrant-vector-schema-alias-docs-20261003",
      "url": "https://qdrant.tech/documentation/manage-data/collections/",
      "type": "official_docs"
    },
    {
      "id": "qdrant-point-upsert-vectors-docs-20261003",
      "url": "https://qdrant.tech/documentation/manage-data/points/",
      "type": "official_docs"
    },
    {
      "id": "qdrant-v119-openapi-74f3e85-20261003",
      "url": "https://raw.githubusercontent.com/qdrant/qdrant/74f3e85b9473c62560006c043e13737ce6b48412/docs/redoc/v1.19.x/openapi.json",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# Qdrant RAG の embedding 移行: backfill・query encoder・rollback の整合性境界

## 問いと対象

再埋め込みが一巡し、新しい vector が検索できれば RAG のモデル移行は完了だろうか。本書では Qdrant を使う dense retrieval を対象に、古い本文からの遅延書込み、query encoder と検索先の食違い、戻し先の陳腐化を防ぐ判断条件を整理する。移行ガイドが検証対象とする Qdrant 1.19.0 を基準にする。

これは未収録だった embedding 世代切替の実務上の穴を埋める調査であり、取得日に新機能が公開されたという主張ではない。既存の[検索品質評価](retrieval-eval-chunking-grounding.md)、[hybrid融合](hybrid-retrieval-fusion-rrf-alpha-normalization.md)、[引用根拠の座標](citation-evidence-coordinate-evaluation.md)とは、保存・検索に使う embedding の世代と原文 revision の対応を扱う点が異なる。他の vector database へ Qdrant の操作契約を一般化しない。

## 確認した契約

### 1. schema追加・値投入・検索切替は別の段階

[Collections の Update Vector Schema](https://qdrant.tech/documentation/manage-data/collections/#update-vector-schema)は、既存 collection への named vector 定義の追加・削除を v1.18.0 以降の操作として記載する。schema追加だけでは既存pointに値は入らない。検索要求は出せても、値の投入前には結果がない。schema削除と、特定pointのvector値の削除も別の操作である。

同じ文書では `points_count` と `indexed_vectors_count` は近似値であり、distinctな検索可能pointの正確な件数ではないと説明する。aliasについては、複数のalias actionを一つの要求に含めて原子的に適用できる。

### 2. 部分vector更新にupsertを代用しない

[Points の Named Vectors / Update Vectors](https://qdrant.tech/documentation/manage-data/points/#named-vectors)は、既存IDへの通常のupsertがpointを置換し、省略されたvectorはnullになると説明する。対して `update_vectors` は指定vectorだけを更新し、非指定vectorを保持する。対象pointはすべて存在する必要がある。

[Update Mode](https://qdrant.tech/documentation/manage-data/points/#update-mode)の `insert_only` は既存IDを更新せず、`update_only` は欠落IDを作成しない。Pointsページの導入版表示は v1.17.0 である。count APIで正確な件数を求めるには `exact: true` を指定する。

### 3. 移行ガイドの無停止範囲には前提がある

[公式移行ガイド](https://qdrant.tech/documentation/tutorials-operations/embedding-model-migration/)は、二つのcollectionを並行運用する方式と、既にnamed vectorsを使うcollectionでの1.18+の追加方式を示す。原文を再取得して再埋め込みする。後者のbackfillでは旧vectorとpayloadを残すため `update_vectors` を用い、既存pointの更新・削除を止め、実行中の操作も完了させてから開始する。検索と新規insertは続けられる。

二collection方式ではbackfillを `insert_only` にし、通常writerが先に作ったpointを上書きしない。削除・部分更新は停止または追加の処理が必要になる。検索切替ではモデルと宛先の両方を変える。切替後も観察期間にdual writesを続け、旧系を更新できる間に戻せる状態を保つ。

### 4. 固定APIで確認できる範囲

[Qdrant v1.19.0のOpenAPI](https://raw.githubusercontent.com/qdrant/qdrant/74f3e85b9473c62560006c043e13737ce6b48412/docs/redoc/v1.19.x/openapi.json)を、release tagから得た40桁SHAに固定して照合した。

- `PUT /collections/{collection_name}/vectors/{vector_name}` はnamed vector定義の作成
- `PUT /collections/{collection_name}/points/vectors` は指定vectorの更新で、非指定vectorを残す
- `UpdateMode` は `upsert` / `insert_only` / `update_only`
- `HasVectorCondition` は指定名のvectorを持つpointを選ぶ `has_vector` 条件

このschema照合は、storage実装の競合検証や分散transactionの証明ではない。

## 独自の設計案: 切替単位を一つの設定として扱う

以下は上記契約から導く本書の運用案であり、Qdrantが提供する自動移行機能ではない。

embedding世代は少なくとも「document encoderの識別子・query encoderの識別子・前処理とchunking版・次元・距離設定・物理collection名またはvector名」の組で記録する。非対称モデルを使う場合は、同一文字列のモデル名ではなく、対応が検証されたdocument/queryの組を固定する。

検索処理の開始時に世代設定を一度だけ読み、その要求内では同じ設定を使い続ける。例えば世代AならqueryをA用にencodeし、物理collection A、または `using=A` を選ぶ。途中で共有設定を再読込してBへ変えない。次元が同じという検査だけでは、この対応関係を検証したことにならない。

alias切替が原子的でも、アプリ側のquery encoder更新まで一つのtransactionになるとは資料から確認できない。旧workerがAでencodeした直後に共有aliasがBへ向けば、旧query vectorが新indexへ届く。対策案は、移行期間中はworkerの世代設定に物理collection名を含め、要求単位でA/AまたはB/Bへ固定すること。共有aliasを使う構成なら、旧世代要求のdrainなど、encoderと宛先の対応を守る別の切替手順を設計・検証する。

## 独自の設計案: 二つのbackfill方式の選び方

| 条件 | 選択と代償 |
|---|---|
| 既存collectionがunnamed vector、または他のcollection設定も変える | 二collection方式を候補にする。payload複製分の容量と二系統writerの障害処理が必要 |
| named vectorsを使用済み、1.18+、既存point更新・削除を一時停止できる | 一collectionの追加方式を候補にする。原文を安定させて新vectorだけを埋める |
| 更新・削除を止められず、変更ログ・競合制御もない | ガイドのループをそのまま本番投入しない。移行要件または書込み経路から見直す |
| 元の本文・前処理版を再現できない | ベクトルだけから新モデルへ移行したことにしない。原文回収または対象の隔離を先に行う |

新規insertだけを許可する場合、ID再利用や「insertのつもりのupsert」が既存pointを書き換えないことも入口で確認する。更新停止はメインAPIだけでなく、再試行queue、管理画面、バッチ、payload変更の経路を含む。

### 競合例A: vectorが存在しても本文revisionと一致しない

次の時系列は独自の説明例であり、Qdrantで再現実験した結果ではない。

1. backfillがpoint42の本文revision10を読む
2. 通常writerがrevision11の本文と、A/B両vectorを保存する
3. 遅れたbackfillがrevision10から作ったBだけを `update_vectors` する

非指定vectorとpayloadを保持する性質は、この競合を解決しない。payloadとAはrevision11でも、Bだけrevision10へ後退し得る。`has_vector=B` では存在を確認できても、この古さは分からない。同じモデルを使うこと、同じpoint IDであること、同じ処理を再試行できることは、異なるrevisionの完了順を保証しない。

まずガイドどおり既存pointの更新・削除停止とin-flight drainを選ぶ。停止なしの設計が必要なら、原文revisionを検査して書込みを一つの直列化境界に置く等の別設計が必要である。単にpayloadへrevision番号を追加するだけでは、vector書込みとそのrevisionの対応が原子的に守られるとは限らない。本書は未検証のCAS実装を推奨コードとして提示しない。

### 競合例B: insert_onlyは削除履歴の代わりにならない

二collectionで、backfillが旧pointを読み終えた後に削除イベントが届き、新collectionにはそのIDがまだない場合を考える。後からbackfillが `insert_only` を実行すれば、現在存在しないIDとして古いpointが入る余地がある。insert_onlyが防ぐのは「既存IDへの上書き」であり、削除済みIDの再挿入ではない。

書込み順や削除履歴の仕組みを別途検証できなければ、削除を停止する。tombstoneを設計する場合も、backfillと通常writerの両方がそれを尊重する必要がある。「deleteを両collectionへ送った」という記録だけで移行完了としない。

## 独自の設計案: 完了判定とrollback

backfillのcursor終端、APIの成功応答、検索結果が1件出たことは、いずれも単独では切替承認にしない。次の各条件を別々に確認する。

1. 対象集合: 移行対象ID、対象外理由、原文revisionを台帳化する。二collectionでは件数一致だけでなくIDの欠落・余分・削除済み混入を照合する
2. vector被覆: named方式では対象集合と `has_vector` を組み合わせて欠落を調べ、必要なcountはexactにする。比較中の新規insertで分母が動くなら、照合対象時点または変更ログの境界を明示する
3. 内容整合: 完了記録を `point ID + 原文revision + embedding世代` に結び付ける。保存vectorの存在検査と、正しい本文から生成した証拠を分ける
4. writer健全性: 一方の書込み失敗、未処理retry、失敗queueを残したまま「両系同期済み」にしない。途中再開用のcursorは、対応する書込みが確認できた範囲だけ進める
5. 品質・費用: 同じ評価query、同じ対象集合・filter・kでA/Bを比較する。top-kの重なりは変化量であり改善の証明ではない。正解ラベルに対する品質、用途別の退行、遅延と費用を採用条件にする
6. 配送・復帰: 旧workerや遅延要求を含めて世代の対応を確認し、観察期間は旧系の更新も維持する。復帰操作もencoderと宛先の組で戻す

rollback可能性は「旧vectorを削除していない」だけでは判断できない。旧系更新停止後に本文が変わると、同じcollection内でも旧vectorは本文に追随しない。新vectorだけの通常upsertなら旧vector自体が消える場合もある。二collectionなら旧系に新規・変更・削除が反映されなくなる。したがって旧系への書込みを止めた時刻を明示し、それ以後の変更を反映する手段なしに「即時で損失なく戻せる」と説明しない。

## 採用前に通す検証シナリオ

以下は移行実装の受入れ条件案であり、本リポジトリでQdrantへ実行した試験ではない。

| 注入する状況 | 期待する判定 |
|---|---|
| schema追加直後、B vectorは未投入 | B検索可能でも移行未完了。schema存在と被覆を区別 |
| 既存pointをBだけの通常upsertで更新 | A消失を検出。部分backfillに採用しない |
| revision10のbackfillがrevision11のwriter後に完了 | revision不一致を検出。存在件数の合格で隠さない |
| backfill読込み後、新collectionへの挿入前に削除 | 削除済みID再出現を検出。insert_onlyだけでは合格にしない |
| Aでquery encodeした要求の途中でaliasをBへ変更 | A/B混在を検出し、世代固定またはdrain手順を見直す |
| 全IDにBがあるが、一部が古い本文由来 | vector被覆合格と内容整合不合格を別表示 |
| 旧系更新を止めた後に本文更新、その後rollback | 変更欠落・古いvectorを検出し、無損失復帰とは報告しない |
| 1.16環境へinsert_onlyを導入しようとする | 下限版の資料不一致を解消するまで本書だけで利用可能と判断しない |

## 適用版・出典・未確認事項

- 公式移行ガイドの表示上の検証対象は1.19.0。Collectionsページはvector schema変更を1.18.0以降とする。本書は1.19.0での契約照合であり、1.18全patchの不具合有無や各SDK対応版を保証しない
- `insert_only` の導入下限は移行ガイドの説明が1.16以降、PointsのUpdate Mode節が1.17.0以降で一致しない。固定1.19.0 OpenAPIには存在するが、それだけで導入版は決められない。旧版採用時は対象版schemaとserver実測で再確認する
- rolling公式3ページは2026-10-03 UTCにnative webで本文を開いた。公開日・改訂日は取得本文に表示されず、取得日を公開日へ読み替えない。ガイドの要約とAPI形状の確認を、実行検証とは区別する
- repository分析は `74f3e85b9473c62560006c043e13737ce6b48412` のOpenAPIのみ。同SHAの[LICENSE](https://raw.githubusercontent.com/qdrant/qdrant/74f3e85b9473c62560006c043e13737ce6b48412/LICENSE)はApache-2.0。Web文書の再利用ライセンスは本文から確認できず `unknown` とした。コード・図の転載やmodule昇格は行わない
- 明示期限2027-01-01は公式文書・repository分析の90日TTLに合わせた再確認日であり、旧版の契約が失効する日ではない
- 未確認: 実際のQdrant起動・API呼出し、停止なし移行の競合制御実装、全SDKの操作原子性、レプリカ間の整合性設定、payload権限filterの移植、embedding生成費用、実データの品質・遅延。ベンダーの無停止チュートリアルを全更新方式の無停止保証として扱わない
- `evals/knowledge/ai-engineering.json` の追加項目は、この知識を検索できるかの検査だけである。上表の障害注入・RAG品質評価を実行して合格したという意味ではない

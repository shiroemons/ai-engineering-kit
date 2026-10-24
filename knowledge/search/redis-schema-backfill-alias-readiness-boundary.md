---
{
  "id": "search-redis-schema-backfill-alias-readiness-boundary",
  "title": "Redis Search の schema 変更：FT.ALTER の受理・backfill・alias 切替を分ける",
  "kind": "knowledge",
  "technology": "search",
  "version": "Redis 8.12-RC1 release notes (October 2026; prerelease); RediSearch static snapshot 0366684950574be979e1983df35b4f9284f1c1d8 (2026-09-16); current command docs checked 2026-10-24 UTC; production patch/backports and runtime unverified",
  "tags": [
    "research-domain:data",
    "redis",
    "FT.ALTER",
    "FT.INFO",
    "FT.ALIASUPDATE",
    "FT.ALIASLIST",
    "SKIPINITIALSCAN",
    "backfill",
    "schema",
    "WORKERS",
    "percent_indexed",
    "FieldSpec"
  ],
  "sources": [
    {
      "id": "redis-search-schema-release-812rc1-20261024",
      "url": "https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.12-release-notes/",
      "type": "release_notes"
    },
    {
      "id": "redis-search-schema-alter-20261024",
      "url": "https://redis.io/docs/latest/commands/ft.alter/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-schema-info-20261024",
      "url": "https://redis.io/docs/latest/commands/ft.info/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-schema-json-indexing-20261024",
      "url": "https://redis.io/docs/latest/develop/ai/search-and-query/indexing/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-schema-alias-update-20261024",
      "url": "https://redis.io/docs/latest/commands/ft.aliasupdate/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-schema-alias-list-20261024",
      "url": "https://redis.io/docs/latest/commands/ft.aliaslist/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-schema-drop-index-20261024",
      "url": "https://redis.io/docs/latest/commands/ft.dropindex/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-schema-fixed-implementation-20261024",
      "url": "https://github.com/RediSearch/RediSearch/commit/0366684950574be979e1983df35b4f9284f1c1d8",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# Redis Search の schema 変更：受理・backfill・alias 切替

## 解く問いと結論

本番検索を止めずに属性を追加・変更するとき、`FT.ALTER` の `OK`、`FT.INFO` の進捗、alias の付け替えを、いつ検索の切替条件に使えるか。

**schema 定義の受理、既存文書の backfill、並行 query の安全性、アプリケーションの切替完了は別々に判定する。** 新属性を定義しただけでは既存文書集合の完全性を証明せず、進捗値が 1 でも検索意味論の検査は省けない。追加だけなら `FT.ALTER` を検討し、型変更や独立した切替・rollback が必要なら別 index を準備して alias を動かす。後者は本稿の設計案であり、公式の無停止保証ではない。

既存の [TIMEOUT / 部分結果](redis-timeout-partial-results.md) と [LIMIT / cursor 完了条件](redis-limit-cursor-completeness-boundary.md) は問い合わせの終了を扱う。本稿は index 自体を新 schema へ移す readiness を扱う。

## 2026年10月の変更と適用版の境界

[8.12 release notes][release] は **8.12-RC1、October 2026** と表示され、pre-release は production に適さないとする。同節は `WORKERS>0` のとき、並行する `FT.ALTER SCHEMA ADD` による query crash を VECTOR、TAG、NUMERIC、GEO、ISMISSING、GEOSHAPE の各 node で修正したと記載する。単に新fieldをまだqueryで使わなければ安全、という説明にはなっていない。

固定 commit `0366684950574be979e1983df35b4f9284f1c1d8`（2026-09-16）の [実装差分][implementation] では、VECTOR query が parse 時に保持した `FieldSpec*` を evaluate 時にも使う経路を改めている。`FT.ALTER` による field 配列の再確保でその pointer が無効になり得るため、安定した `fieldIndex` を保存し、実行時の配列から取り直す。`src/vector_index.h` の pointer 削除と `src/vector_index.c` の再取得を静的確認した。これはこの問題の機構であり、あらゆる query / schema 操作の競合解消を証明するものではない。

本稿で確定したのは release-note の RC 表示と固定修正である。正式版の公開日、RC binary の入手可能性、stable backport、影響を受ける最初の版と各提供形態の patch 対応表は未確認。**「8.10 ならすべて危険」「8.12 へ上げれば本番移行完了」とは判定しない。** 稼働中の Redis / Search build と対象修正の対応を確認し、不明なまま query と同一 index の schema 変更を重ねない。RC への本番 upgrade や `WORKERS=0` への設定変更を一律の回避策にはしない。

## 公式 API 契約を分けて読む

### 1. SCHEMA ADD は定義の追加であり、既存集合の移行証明ではない

[FT.ALTER][alter] の公開構文は `SCHEMA ADD`。属性追加後の document 更新には新属性が反映される。`SKIPINITIALSCAN` を指定すると scan と indexing を行わない。既存fieldの型変更・削除を、この追加 API の契約として扱わない。

同文書には TEXT 属性数の制約もある。32 未満で作成した index は追加後の TEXT が32までに制限され得る。32を超える予定なら作成時の `MAXTEXTFIELDS` を確認する。新しいfieldをいくつでも後から追加できる、とは見積もらない。[alter]

固定実装 `src/module.c` の `AlterIndexInternalCommand` は、`SKIPINITIALSCAN` なら `initialScan=false` にし、field追加成功かつ `initialScan` のときだけ再走査の入口を呼ぶ。同じsnapshotの `src/indexes_scan.c:IndexSpec_ScanAndReindexForAlter` は、DB内にkeyがある場合に非同期走査をscheduleする。indexの対象keyがあるかを、この条件だけで判断しているわけではない。この固定snapshotの ALTER は `DiskDisabledCmd` で Redis Flex の disk経路を拒否し、再走査側も disk無効をassertするため、ここでの実装分析は non-disk の経路に限る。このguardをすべての提供版へ一般化しない。その後の `OK` を、全既存文書の再索引と受入検査が済んだという意味へ拡張しない。[implementation]

### 2. 既存文書と新規・更新文書には異なる時間軸がある

[JSON indexing guide][indexing] では、既存 JSON 文書の index 作成は background で進み、検索できるまで時間がかかる一方、新規・更新 JSON 文書は同期的に indexing されると説明する。この説明の対象は JSON 文書であり、分散 replica やすべての index 構成の read-after-write 保証まで一般化しない。

従って、新規文書を一件 `JSON.SET` し検索できたことは、**その前から存在した文書の backfill 完了を証明しない**。切替テストには「作成前から存在し、その後一度も更新されていない文書」を入れる。これは時間軸の違いに基づく本稿の検査案である。[indexing]

### 3. FT.INFO の進捗と失敗情報を別々に読む

[FT.INFO][info] は `indexing`、`percent_indexed`（1が100%）、`hash_indexing_failures` を返す。index全体と各fieldの `indexing failures`、`last indexing error`、`last indexing error key` も確認できる。

ただし、固定実装の `src/info/info_command.c` と `src/indexes_scanner.c:IndexesScanner_IndexedPercent` は progress を業務上の正解集合とは比較していない。活動中は `scannedKeys` をその時点の `DbSize` で割って上限1に収める。SCAN / AsyncScan の at-least-once 配送により重複keyが数えられ得るため、分子はDB内のkey数を超え得る。scan待機中は0、OOM中断は保存された走査数を使う。活動中・OOMの比率計算で `DbSize==0` なら0を返し、scanner・scan待機・OOM記録がいずれもなければ1を返す。この snapshot の **percent_indexed は走査進捗** であって、期待文書がすべて新属性で検索可能という比率ではない。[implementation]

この実装観測を全歴代版の計算式として扱わない。一方で `SKIPINITIALSCAN` と進捗表示だけを組み合わせても完全性の証明にならないという運用判断は、公開 API の scan省略契約とも整合する。`indexing=0` / `percent_indexed=1` を readiness の必要な観測の一つとして使っても、error情報・対象key集合・schemaに応じた検索結果を別に検査する。[alter][info]

### 4. alias の更新は routing の更新である

[FT.ALIASUPDATE][aliasupdate] は alias を新しい index に関連付け、以前の関連付けを外す。`OK` はこの操作の成功を示し、存在しない index は error になる。新indexのbackfill、query品質、旧queryのdrainを自動で待つ契約は記載されていない。

[FT.ALIASLIST][aliaslist] は **8.10.0以降の Redis Open Source / Search** と表示され、指定indexに付くaliasを返す。RESP2はarray、RESP3はsetで、順序を判定条件にしない。2026-10-24確認の互換表では Redis Software / Redis Cloud は非対応とされるため、Open Source版の手順をそのままmanaged環境の必須チェックにしない。実際の提供機能を確認し、利用できなければその環境で確認できる方法で切替先を検証する。

alias一覧が期待どおりでも、それはroutingの証拠であり、検索内容の正しさの証拠ではない。[aliasupdate][aliaslist]

## 実務上の切替手順（上記からの独自設計）

1. **変更対象を固定する。** 既存indexの定義、対象keyのprefix/filter、queryが使うfield、Redis / Searchの正確なbuild、同時実行queryとworker設定を記録する。追加だけか、型変更・名称変更・rollbackを伴う置換かを区別する。
2. **並行変更の安全性を先に判断する。** 稼働版の修正対応が確認できない場合は、同一indexへの `FT.ALTER` をlive queryと重ねない。新index方式や計画停止を比較する。新indexを作るだけであらゆる競合がなくなるとの保証はしない。
3. **backfillの方針を明示する。** 既存集合も検索対象なら、理由なく `SKIPINITIALSCAN` を付けない。意図的に省略する場合は別途の再投入・検証手順と未移行集合を管理し、進捗値を代用しない。
4. **readiness を複数の条件で判定する。** 初期走査の終了、失敗情報、indexの対象範囲、代表queryの期待結果を確認する。既存未更新・更新済み・欠損field・型不一致・対象外keyを含むfixtureを用意し、新旧indexを同じ目的のqueryで比較する。異なるschemaの総件数が等しいだけでは合格にしない。
5. **alias を切り替え、実際の読み出しを確認する。** 置換方式では新indexを直接検査してから `FT.ALIASUPDATE` を行い、alias経由のqueryと、利用可能なら `FT.ALIASLIST` で確認する。アプリが物理index名を直接参照していないかも調べる。旧indexはrollbackに必要な間残す。
6. **cleanup を別操作として扱う。** 旧queryやcursorの寿命、直接参照、rollback終了を確認してから旧indexを削除する。共有keyを使う新旧indexでは、旧indexだけを消す意図で `DD` を付けない。

[FT.DROPINDEX][dropindex] は既定では文書を残すが、`DD` は実際の document key も非同期に削除する。また初期scan中に `DD` を実行すると、まだindexされていないhashは残り得る。従って `DD` は「新indexに影響せず旧indexだけ掃除する」操作でも「prefix配下の全key削除」でもない。許可された対象と目的を別途確認する。

## 避ける判定と受入テスト

- `FT.ALTER` が `OK` なので直ちに新fieldへのqueryを全量切替する。
- `SKIPINITIALSCAN` を高速化フラグとして使い、既存未更新文書の欠落を見ない。
- `percent_indexed=1` または `num_docs` の一致だけでfield単位の意味論を承認する。
- 新文書一件の同期indexingを既存集合のbackfill完了と取り違える。
- alias切替成功を旧query終了・index互換性・データ完全性と同一視する。
- 同じkeyを参照する旧indexに `FT.DROPINDEX ... DD` を行う。

採用環境では、scan中のquery、新属性を持つ未更新文書、意図的な不正field、途中OOM、alias切替後の旧cursor、再起動・failover時の結果を検証する。これは実行を推奨する検査項目であり、本調査が成功を確認したruntime結果ではない。OOM時のprogress実装観測は上記の固定commitに限る。

## 出典・provenance・未確認事項

- 調査日は **2026-10-24 UTC**。release-noteの公開日表示はOctober 2026までで、command文書・guideに公開日表示はない。rolling文書を特定patchの完全な仕様snapshotとは扱わない。
- native webでrelease notes、各command文書、JSON indexing guideの実ページを開いた。GitHubの一部PR/commitページはweb取得がCache missとなったが、同じ公開repositoryのAPIでPR metadataと固定commit差分・対象sourceを読み直した。失敗したweb取得自体を証拠にしていない。
- release-noteに列挙された6種の修正と、VECTORの固定差分は根拠を分けた。固定commitの静的分析は tagged release全体の動作保証ではなく、stable backportの有無も検索結果の不在だけでは断定しない。
- docsのLICENSEはCC-BY-NC-SA-4.0と旧redis-doc由来のCC-BY-SA-4.0例外を確認。固定repositoryのLICENSE.txtと対象source headerはRSALv2 / SSPLv1 / AGPLv3の選択式。本文は独自要約・分析のみで、コード転載やmodule昇格はない。
- runtime再現、負荷試験、全shard・replicaでの進捗集約、managed環境の実提供機能、aliasのin-flight query/cursorへの詳細な影響、stable patch対応表は未確認。検索evalは文書の発見性を検査するもので、Redisの意味論を実行検証しない。
- release_notesのTTL30日が最短となるため再確認期限は **2026-11-23**。その際はRC/正式版・backport・提供形態と本文全体を再調査し、日付だけ延長しない。

[release]: https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.12-release-notes/
[implementation]: https://github.com/RediSearch/RediSearch/commit/0366684950574be979e1983df35b4f9284f1c1d8
[alter]: https://redis.io/docs/latest/commands/ft.alter/
[info]: https://redis.io/docs/latest/commands/ft.info/
[indexing]: https://redis.io/docs/latest/develop/ai/search-and-query/indexing/
[aliasupdate]: https://redis.io/docs/latest/commands/ft.aliasupdate/
[aliaslist]: https://redis.io/docs/latest/commands/ft.aliaslist/
[dropindex]: https://redis.io/docs/latest/commands/ft.dropindex/

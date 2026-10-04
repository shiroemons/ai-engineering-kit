---
{
  "id": "search-redis-limit-cursor-completeness-boundary",
  "title": "Redis Search 8.10 の LIMIT 修正とページ走査の完了条件",
  "kind": "knowledge",
  "technology": "search",
  "version": "Redis Open Source 8.10.0 GA (2026-07-29); Search v8.10.0 at 294c88bca92b3e686d336dc165bcf68512d91ac5, MOD-16767 fix f76a1bd4c3b5ef43cd96299184b2a18309068538; current docs verified 2026-10-04 UTC; runtime untested",
  "tags": [
    "research-domain:data",
    "redis",
    "FT.SEARCH",
    "LIMIT",
    "SORTBY",
    "pagination",
    "WITHCURSOR",
    "KNN",
    "RESP3",
    "MOD-16767",
    "snapshot"
  ],
  "sources": [
    {
      "id": "redis-search-pagination-release-810-20261004",
      "url": "https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.10-release-notes/",
      "type": "release_notes"
    },
    {
      "id": "redis-search-pagination-ga-date-20261004",
      "url": "https://github.com/redis/redis/releases/tag/8.10.0",
      "type": "release_notes"
    },
    {
      "id": "redis-search-pagination-command-20261004",
      "url": "https://redis.io/docs/latest/commands/ft.search/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-pagination-cursor-lifecycle-20261004",
      "url": "https://redis.io/docs/latest/develop/ai/search-and-query/advanced-concepts/aggregations/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-pagination-cursor-read-20261004",
      "url": "https://redis.io/docs/latest/commands/ft.cursor-read/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-pagination-expiration-20261004",
      "url": "https://redis.io/docs/latest/develop/ai/search-and-query/advanced-concepts/expiration/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-pagination-fix-commit-20261004",
      "url": "https://github.com/RediSearch/RediSearch/commit/f76a1bd4c3b5ef43cd96299184b2a18309068538",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-search-pagination-reply-source-20261004",
      "url": "https://github.com/RediSearch/RediSearch/blob/294c88bca92b3e686d336dc165bcf68512d91ac5/src/module.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-search-pagination-upstream-tests-20261004",
      "url": "https://github.com/RediSearch/RediSearch/blob/294c88bca92b3e686d336dc165bcf68512d91ac5/tests/pytests/test_resp3.py",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# Redis Search のページ件数と全件走査を分ける

## 問いと結論

`FT.SEARCH ... LIMIT 20 10` が10件を返すよう修正されたら、一覧を順番に最後まで読めば全件を一度ずつ得られるか。あるいは `FT.AGGREGATE ... WITHCURSOR` のcursorが0になるまで読めば、開始時点のexportを完成と判定できるか。

**ページの切り出し、順序の決定性、母集団の固定、読み出した内容の保存は別々の条件である。** Redis 8.10の修正は最終replyの窓を正す。内部のKNN候補収集をページ件数まで減らす修正でも、書込み・失効・再配置をまたぐsnapshotの提供でもない。検索画面と厳密な全件exportでは成功条件を分ける。

既存の[TIMEOUTと部分結果](redis-timeout-partial-results.md)は期限切れを扱う。本稿は期限内に返った検索でも残る、offset・candidate数・cursorの読み切りと完全性の問題を扱う。

## 版・日付・証拠の強さ

- Redis Open Source **8.10.0 GAは2026-07-29公開**。公開releaseの時刻は17:20:06 UTCである。[GA release][ga]
- 8.10系列release notesのRC1欄は、cluster RESP3で `FT.SEARCH` の `LIMIT` が過剰件数を返す `MOD-16767` の修正を記載する。GAで初めて直ったと書き替えず、8.10系列に含まれる修正として扱う。[release notes][release]
- 8.10向けbackportの固定commitは `f76a1bd4c3b5ef43cd96299184b2a18309068538`（2026-07-09 merge）。Searchのtag `v8.10.0` は `294c88bca92b3e686d336dc165bcf68512d91ac5` を指す。GitHubのcommit比較で前者が後者の祖先であることを確認し、tagのソースとtestも開いた。[修正差分][fix] [tag実装][impl] [tag tests][tests]
- API/運用契約は2026-10-04 UTC取得のunversioned公式文書による。cursor機能自体を8.10の新機能とは扱わない。たとえば `FT.CURSOR READ` のavailability表示はSearch 1.1.0である。[cursor command][read]
- 以下の実装観察は静的分析である。Redis server、cluster、上流testは実行していない。他release系列へのbackport時期、全affected版、Redis Cloud/Softwareのrollout時期も未確定。

## MOD-16767: 内部候補数を最終ページ数に流用しない

### offsetがあるRESP3の過剰返却

固定tagの `src/module.c` では `sendSearchResults` が、結果列の先頭ではなく要求されたoffsetからreplyを組み立てる。停止位置には `offset + limit` と実際の結果列長を使う。修正前のRESP3経路は0番目から走査していたため、offsetで捨てるべき候補もreplyへ混ざった。[修正差分][fix] [tag実装][impl]

したがって `LIMIT 20 10` の受入確認は「10件以下になった」だけでは足りない。期待する21〜30番目のID列が返ることまで比較する。これは本稿の試験設計上の判断であり、単にclientで余分な先頭10件を切り捨てれば直るという処方ではない。

### KNNではRESP2も対象になる

release notesの短い見出しはRESP3を挙げるが、修正差分と上流testはKNNのRESP2/RESP3両方を扱う。KNNの経路では、shardから候補を集めてmergeするために内部の `requestedResultsCount` をKに応じて調整する。最終replyまでその値を流用すると、Kがページ件数より大きい場合に過剰返却となる。修正後はその内部値を候補収集側に残し、最終replyの窓だけを元の `offset + limit` で制限する。[修正差分][fix] [tag実装][impl]

たとえば **KNN 20とLIMIT 0 10は別の数**である。Kは検索が求める近傍集合の大きさ、10は今回返すページ側の要求。KをHNSW等の内部探索量の上限とも扱わない。返答が10件になっても、内部候補・CPU・memoryも10件分に制限されたとはいえない。同じ理由で、UI都合でKを10に下げることを、この不具合の等価な修正として扱わない。

### 上流testで実際に照合しているもの

固定tagの `tests/pytests/test_resp3.py` には次のregressionがある。[tag tests][tests]

| test | 条件 | 比較する期待値 |
|---|---|---|
| `test_search_sortby_limit_offset` | cluster・RESP3、数値SORTABLEの30文書、昇順、`LIMIT 20 10` | 20〜29に対応するID列 |
| `test_search_knn_limit_offset_resp2` / `resp3` | cluster・各protocol、FLAT/FLOAT32/2次元/L2、KNN 20 | `LIMIT 0 10` は0〜9、`LIMIT 10 10` は10〜19のID列 |

KNN fixtureは距離の異なる単純なvectorを使い、indexの準備を待ってから照合する。これは期待する窓を明確にした上流の試験であり、HNSWのrecall、同距離tie、並行更新、失効、再配置まで検証したものとは読まない。

## LIMITの決定性とsnapshotは別

公式 `FT.SEARCH` は、sortingなしの `LIMIT` を非決定的とし、続くqueryで重複や欠落が起こり得ると説明する。対策としてunique fieldの `SORTBY`、またはaggregateの `WITHCURSOR` を挙げる。offsetは0始まり、既定窓は `0 10` である。[search command][search]

uniqueな順序を持たせることから、複数queryを通じて母集団が変わらないことは導けない。以下は仕組みを説明する独自の思考例であり、Redisで実行した結果ではない。

1. 昇順の集合が `A,B,C,D,E,F` の時点で `LIMIT 0 2` を読み、AとBを得る
2. 次のqueryより前にAを削除し、集合が `B,C,D,E,F` になる
3. 同じ昇順で `LIMIT 2 2` を評価すればDとEとなり、Cは訪れない

tieのない順序でもoffsetの位置は動く。途中に先行する要素を追加した場合は、既読要素が後のページへずれるケースもある。ユーザーが現在の一覧を閲覧する用途で許容できても、固定時点の全件exportに同じ保証を宣言しない。

また、検索の総ヒット数と実際にreplyへ載った文書数を分ける。DIALECT 4 / WITHOUTCOUNTのsorting最適化ではcountが全件数でない場合があり、SORTBYのWITHCOUNTは全結果を処理して正確なcountを得る代わりに追加の仕事をする。このcount取得も、別queryで読む全ページの内容を固定する操作ではない。[search command][search]

## WITHCURSORの読み切り条件と残る穴

### cursor ID 0を見ても、そのreplyの行は処理する

aggregateのcursorはserverにquery状態を保持して継続する。初回と `FT.CURSOR READ` のreplyにはbatchとcursor IDがあり、ID 0が読み切りを示す。ただし **最終batchとcursor ID 0は同じreplyに共存し得る**。0を先に見てreturnすると、最後の行を捨ててしまう。[Cursor API][cursor]

COUNTは一回のread件数を調整するもので、全exportの母集団件数ではない。`FT.CURSOR READ ... COUNT` は初回指定を上書きできる。[cursor command][read]

独自のconsumer設計としては、毎回「error確認 → batchの保存 → 保存成否確認 → cursor ID 0なら終了」の順に扱う。cursorの存在しないerrorを、正常な空batchやEOFへ変換しない。初回のaggregate replyも同じ処理に通す。

### 継続期限と応答喪失を完了扱いしない

Cursor APIのidle timeoutは既定300000msで、MAXIDLEの指定もこの300秒を超えられない。readするとidle時計はresetされ、期限を過ぎるとcursorは削除される。これはscan全体の制限時間とは異なる。中断時は `FT.CURSOR DEL` でserver資源を解放できる。[Cursor API][cursor]

`cursor not found` はcommand上のerrorである。[cursor command][read] 本資料から、応答喪失後の同一read再送が同じbatchを返す保証は確認できていない。したがって、切断・不明な応答・idle失効を成功へ潰さず、作業を未完了として保持する。再開時の世代・重複排除・再走査方針はapplication側で定義する必要がある。

### 通常完了でも固定時点の完全性は保証しない

Cursor APIは、cursor生存中の新しい更新が結果に含まれない場合と、load rebalancing（Atomic Slot Migration等）中に一部の結果が欠ける場合を明記している。Redis Softwareでは再配置が利用者の操作なしに起こる場合もある。従ってcursor ID 0は、利用者が意図した固定母集団との一致まで証明しない。[Cursor API][cursor]

失効には別の時間軸がある。Redis 8以降はqueryまたは **各cursor read開始時**にkey/fieldの有効性を判断する一方、実行中のactive expirationにより返却件数は減り得る。hash field expiration自体の導入は7.4である。この説明を、cursor作成時の全行・全fieldを最後まで保持するsnapshot契約へ拡張しない。[expiration][expiry]

## 採用判断と受入試験（独自案・未実行）

- 検索画面: 表示順にuniqueな基準を用意し、更新でページ間の結果が動く仕様を明示する。clientが想定するRESP版・返答型ごとにID列を確認する
- 長いbest-effort走査: cursorを候補にするが、最終batch、失効、再配置、consumer保存失敗を扱う。batchの件数だけを全件性の証拠にしない
- 厳密なexport: まず「どの時点のどの集合か」を決める。Redisのlive searchだけで要件を満たせないなら、変更されないデータ世代や別途固定したID集合・内容を使う。固定IDだけでは、後から読む値やTTLも固定されたことにはならない

導入先では次を別々に照合する。以下は実行済みチェックリストではない。

1. **返却窓**: 非zero offsetの通常検索をRESP3で、K>NのKNNをRESP2/RESP3で試す。件数・先頭末尾・ID全列を比較し、standaloneの成功だけでclusterを合格にしない
2. **順序**: 無sort、unique sort、重複するsort値を分ける。tieの期待順を推測せず、必要ならschema/問い合わせを設計し直す
3. **更新**: page間の追加・削除・sort値変更を行い、重複排除で欠落まで修復できるという誤解を防ぐ
4. **終了**: cursor ID 0と非空batch、正常な空集合、cursor not found、保存失敗を別ケースにする。最後のbatchを保存する前に公開しない
5. **失効**: key TTLとfield TTLを使い、cursor read間およびread実行中を分けて期待集合を定める。返却数減少を必ずしもLIMIT defectとは扱わない
6. **運用変更**: 対応する検証環境でslot移動・再配置を試す。IDの基準集合との照合なしに、例外が出なかったことだけで全件export成功を宣言しない
7. **観測**: server/Search版、cluster構成、RESP版、query/dialect、K、offset/count、返却ID、count metadata、cursor lifecycleと保存結果を分けて残す

## 限界・出典・再確認

本稿は公式reference、release履歴、固定commitの実装・testの独自要約である。実サーバー再現、性能測定、client別自動retry、暗黙のsort tie-break、cluster topology変更時の詳細な欠落条件は未検証。cursorが全件性を持たないからといって、常に欠落する、常に同じ更新を無視する、とも主張しない。

公式文書はRedis Ltd.の[documentation license](https://raw.githubusercontent.com/redis/docs/main/LICENSE)、ソースは固定tagの[LICENSE.txt](https://github.com/RediSearch/RediSearch/blob/294c88bca92b3e686d336dc165bcf68512d91ac5/LICENSE.txt)とmodule.c headerを確認した。原文・実装コード・fixtureの転載はしていない。GitHub release本文のライセンス適用は未確定であり、日付とGA区分の事実確認に限定した。release notesを含む再確認期限は2026-11-03。再取得時は返却窓、KNN、cursor、失効の各契約を再照合し、日付だけ延長しない。

[ga]: https://github.com/redis/redis/releases/tag/8.10.0
[release]: https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.10-release-notes/
[fix]: https://github.com/RediSearch/RediSearch/commit/f76a1bd4c3b5ef43cd96299184b2a18309068538
[impl]: https://github.com/RediSearch/RediSearch/blob/294c88bca92b3e686d336dc165bcf68512d91ac5/src/module.c
[tests]: https://github.com/RediSearch/RediSearch/blob/294c88bca92b3e686d336dc165bcf68512d91ac5/tests/pytests/test_resp3.py
[search]: https://redis.io/docs/latest/commands/ft.search/
[cursor]: https://redis.io/docs/latest/develop/ai/search-and-query/advanced-concepts/aggregations/
[read]: https://redis.io/docs/latest/commands/ft.cursor-read/
[expiry]: https://redis.io/docs/latest/develop/ai/search-and-query/advanced-concepts/expiration/

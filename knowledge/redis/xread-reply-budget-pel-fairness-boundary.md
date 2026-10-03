---
{
  "id": "redis-xread-reply-budget-pel-fairness-boundary",
  "title": "Redis 8.10 XREAD: 総件数・soft byte budget・PELとstream間公平性の境界",
  "kind": "knowledge",
  "technology": "redis",
  "version": "MAXCOUNT/MAXSIZEはRedis Open Source 8.10.0 GA（2026-07-29）導入。8.10.2 commit 498ecd0d6d007db11ddb3aea9428552598a78622 の実装・upstream testsと2026-10-03取得のcommand referenceを照合。実サーバー未実行。",
  "tags": [
    "research-domain:data",
    "redis",
    "streams",
    "XREAD",
    "XREADGROUP",
    "MAXCOUNT",
    "MAXSIZE",
    "PEL",
    "backpressure",
    "fairness",
    "soft-limit"
  ],
  "sources": [
    {
      "id": "redis-xread-reply-budget-command-20261003",
      "url": "https://redis.io/docs/latest/commands/xread/",
      "type": "official_docs"
    },
    {
      "id": "redis-xreadgroup-reply-budget-command-20261003",
      "url": "https://redis.io/docs/latest/commands/xreadgroup/",
      "type": "official_docs"
    },
    {
      "id": "redis-xread-budget-ga-release-20261003",
      "url": "https://github.com/redis/redis/releases/tag/8.10.0",
      "type": "release_notes"
    },
    {
      "id": "redis-xread-budget-patch-release-20261003",
      "url": "https://github.com/redis/redis/releases/tag/8.10.2",
      "type": "release_notes"
    },
    {
      "id": "redis-xread-budget-stream-implementation-20261003",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_stream.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-xread-budget-network-accounting-20261003",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/networking.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-xread-budget-upstream-tests-20261003",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/stream.tcl",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-xreadgroup-budget-upstream-tests-20261003",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/stream-cgroups.tcl",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# Redis XREAD のreply予算をconsumer設計へ接続する

## 問いと結論

多数のstreamを一度に読むconsumerで、`COUNT 100` を指定しただけで一回の取得を100件、あるいは一定メモリ以下に抑えたといえるか。Redis 8.10で追加された `MAXCOUNT` と `MAXSIZE` を使えば、stream間の公平性やpendingの総量も自動的に制御できるか。

結論は、総件数は `MAXCOUNT` で制限し、`MAXSIZE` はreplyを組み立てる途中のsoft byte budgetとして使うこと。厳密なwire bytes上限やRSS上限として扱わない。さらに、読む順番、未ACKの仕事量、応答消失後の回復はconsumer側で別々に設計する。

これはproducer追加時のIDMP重複抑止、RDB/AOFの保存、Search queryのtimeoutとは独立した、consumer一回分の読み込み予算の問いである。新規性は8.10の累積budget導入にあり、XREAD自体やconsumer groupを新機能として説明しない。

## 適用版と確認の範囲

- [8.10.0 GA release](https://github.com/redis/redis/releases/tag/8.10.0) の公開日時は2026-07-29 17:20:06 UTC。MAXCOUNT/MAXSIZE追加を明記し、両commandのHistoryも8.10.0導入で一致する
- 読解対象は [8.10.2 release](https://github.com/redis/redis/releases/tag/8.10.2)（2026-09-17 15:06:56 UTC）のcommit `498ecd0d6d007db11ddb3aea9428552598a78622`。機能の導入版8.10.0と、実際に採用するpatchを分ける。8.10.2にはSECURITY区分の修正があり、本稿は初版への固定・downgrade推奨ではない
- command referenceは2026-10-03 UTCに読んだ版固定でないページで、表示された公開日・更新日は確認できなかった。コード由来の観察を全patchや互換サービスへ無条件に広げない
- 実サーバー、client library、RESP2/RESP3のパケット計測、障害注入は実施していない。以下では公式command契約、固定commitの静的読解、独自の設計案を分ける

## 公式command契約

### COUNTとMAXCOUNTは制限する集合が違う

[ XREAD のOptional arguments / MAXCOUNT and MAXSIZE](https://redis.io/docs/latest/commands/xread/#the-maxcount-and-maxsize-options) によると、`COUNT C` はstreamごとの上限である。N個のstreamに十分なentryがあれば合計N×C件になりうる。`MAXCOUNT K` は一つのcommandに列挙した全streamの累積上限。COUNTを省略してMAXCOUNTだけでも指定できる。

MAXCOUNTは正の整数で、COUNTを併記する場合は `MAXCOUNT >= COUNT` が必要。たとえば既存の `COUNT 100` に総量20件を意図した `MAXCOUNT 20` を単純追加する構成は拒否される。COUNTを20以下へ下げるか、省略して総量だけを指定する。`MAXCOUNT 0` を無制限指定として送らない。

`MAXSIZE B` も正の整数で、単位はbytes。MAXCOUNTとの併用では先に適用条件を満たした側が追加のentry出力を止める。件数が小さくても一entryが大きければ小さなreplyにはならない。

### 「少なくとも一件」はデータがない場合の保証ではない

同じ[XREAD reference](https://redis.io/docs/latest/commands/xread/)は、巨大な一entryしかなくても進めるよう、そのentryがMAXSIZEを超えていても返すと説明する。一方、対象IDより後のentryがない、またはBLOCKがtimeoutになった場合のRESP2 nil / RESP3 nullも明記する。

したがって「少なくとも一件返す」はsize budgetによって最初の読み出し可能entryまで拒否しないという意味であり、空のstreamからentryを作る、timeoutをなくす、指定した件数まで待ってbatchを満たすという保証ではない。BLOCKの待ち時間とMAXCOUNTの件数を別の設定として扱う。

### XREADGROUPのbudgetとPEL

[XREADGROUPのMAXCOUNT/MAXSIZE節](https://redis.io/docs/latest/commands/xreadgroup/#the-maxcount-and-maxsize-options)では、新規の `>` 読み込み、consumer自身のhistory/PEL読み込み、blocking解除後にも両budgetを適用する。MAXSIZEによる停止判定は次のentryを届ける前で、停止対象の新規entryをconsumerのPELへ先に追加しない。

ここでPELのhistoryは既にpendingである。budgetで読み出さなかったentryがPELから消えるという意味ではない。同referenceは、実際に再配信したentryの配信時刻・配信回数の更新、処理後のXACK、クラッシュ後のpending回復を説明する。`NOACK` はPELへの記録を省く指定であり、MAXSIZEを厳しくしたり、アプリの処理完了を確認したりする指定ではない。

## 固定commitから確認した実装上の境界

### MAXSIZEは次entryの「予測サイズ」を検査しない

8.10.2の [streamReplyMaxsizeReached](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_stream.c#L1968-L1976) は、少なくとも一entryを出力済みで、既に累積した `net_output_bytes_curr_cmd` が閾値以上なら止める。[通常entryのloop](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_stream.c#L2197-L2261) はこの判定の後で一entryのID・field/valueを出力する。次のentryを足した後の全長を先に見積もる比較ではない。

この順序からの静的な帰結は、公式文書が例示する「最初の巨大entry」以外でもovershootが起こりうること。たとえばB=1,000のとき既出bytesが900なら次のentryの処理に入れる。そのentryが大きく、出力後に1,000を超えても途中で切り詰めず、次回の判定で止まる。これはアルゴリズムを説明する仮定例であり、900 bytesのfixtureを実測した結果ではない。

「最初の一entryだけ例外で、二件目以降は必ずB以下」「あと100 bytesしかないから100 bytesを超すentryはpendingへ残して送らない」という判定は、この実装とは一致しない。正確には、すでに閾値へ達しているなら次entryを送らず、まだ達していなければ次entryが閾値を跨ぎうる。

### reply生成の計数とメモリ・ネットワーク完了を分ける

[networking.cのreply追加処理](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/networking.c#L515-L591)では、buffer/listへreplyを追加する際にこのcounterを増やし、bulk stringの経路ではprefix・内容・末尾CRLFを計数する。value本体だけを足したpayload bytesではない。

したがってMAXSIZEの数値をclientが実際に受信・decode済みのbytes、TLS/TCPを含む全通信量、server/clientのresident memoryの上限と等置しない。containerのmemory limit、client parserの最大frame、アプリのqueue容量をBそのものに揃えて安全と判断する根拠にはならない。replyのentry集合と実際のwire bytes、decode後メモリを別々に測る必要がある。

[XREADの総量loop](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_stream.c#L2979-L2994) はcommandを処理し始めた時点の既出bytesをbaselineへ加える。同じ `MULTI/EXEC` 内で先行commandが出したreplyは、このXREADのMAXSIZEを先に使い切らない。逆に、MAXSIZEはEXEC全体やpipeline全体の予算ではない。

### 止めたentryと、replyを失ったentryは状態が違う

[新規entry loop](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_stream.c#L2197-L2307)はsize判定より後でgroupのlast-delivered IDとPELを更新する。budgetにより出力前に止めたentryを読み飛ばした扱いにはしない。[history loop](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_stream.c#L2345-L2385)も判定より後で対象を再出力し、存在するentryの配信回数・時刻を更新する。

一方、serverが出力とpendingへの登録を行った後に通信が切れれば、clientにreplyが届かなくてもpendingは残りうる。「応答なし」と「budgetのため未配信」を同じ状態にしない。MAXCOUNT/MAXSIZEの導入は応答消失後のpending照合を不要にしない。通常の `>` 再読だけで、既に自分へ配信済みの仕事も全て回復したことにはならない。

### 小さなreplyとstream間の公平性は別

公式XREADGROUPのOrdering guaranteesは、非blocking時のstream間の順序を引数順とする。固定commitの[XREAD総数処理](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_stream.c#L2988-L3130)も先頭から進み、MAXCOUNT残量に合わせてそのstreamの件数を縮め、累積上限に達すると後続streamへ進まない。

たとえば常に十分なentryがある4streamを、毎回同じ順番で `COUNT 20 MAXCOUNT 50` と読むと、size制約など他の停止条件がない場合は先頭側の20・20・10件で予算を使える。四番目にbacklogがあっても、この一回のreplyには現れない。これは件数制御から導いた説明例であり、block解除時の全event順序まで同じになるという主張ではない。

したがって、返らなかったstreamを空と判定しない。先頭streamが継続的にbudgetを使い切れば後続の遅延が増えうる。COUNTを残すだけで公平性が完成するわけではなく、読む順序・分割・stream別のlagを設計する。

## 採用手順：以下は独自の設計案

1. **予算を三層に分ける。** 一commandの総件数Kとsoft bytes Bに加え、consumer全体の処理中件数・decode後queue bytesを別に制限する。小さなreadを無制限に繰り返せば未ACK件数やin-flightは増える。処理枠が空くまで次のreadを出さない等、実際のworker容量と接続する
2. **最大entryを入力側で管理する。** 予算を小さくしても巨大entryを拒否できない。producerで業務上のpayload上限を設け、大きい物は別ストレージへの参照にする等を検討する。consumerが処理できない巨大entryを、Bを下げて同じ位置から再試行し続ける回復ループにしない
3. **偏りを観測してから分割する。** 全体throughputだけでなくstreamごとの最古未処理時刻、最後に取得できた時刻、pending量を見る。開始streamを巡回する、latency要件が違うstreamを別pollへ分ける、または少数streamずつ読む案を比較する。巡回時はキーとIDの対応を一緒に並べ替え、別streamのcursorを取り違えない
4. **XREADのcursorをstreamごとに保持する。** 取得したstreamの処理済み境界を更新し、budgetにより返らなかったstreamのcursorは維持する。再読ごとに `$` を指定すると読み出し間に増えたentryを逃しうる。受信しただけで永続checkpointを進めるかは、下流副作用の再実行設計と合わせる
5. **groupの回復を別経路にする。** XREADGROUPの新規readと、自分のpending再取得・他consumerからのclaimを区別する。外部副作用を終えてXACKするまでの失敗に備える。NOACKはその回復記録を省くため、メモリ節約だけを理由に採用しない
6. **対応版を明示的にgateする。** 8.10.0未満のserverや未対応clientに新引数を渡した際、例外を握り潰して無制限readへfallbackしない。移行中は旧版用のper-stream分割などを別経路として検証し、新方式と同じbytes保証があるとは説明しない
7. **終了条件を件数だけにしない。** 少量・null・特定streamの欠落を、全streamの処理完了の根拠にしない。snapshotを最後まで読む用途なら、対象streamごとの終了境界とcheckpointを別に確定する。更新が続くstreamを一回のbudgetつきreplyで読み切れたとは扱わない

## 検証すべき境界とupstream evidence

読んだ [stream.tcl](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/stream.tcl#L3113-L3193) は、引数エラー、MAXCOUNT総数、先頭streamからの取得、巨大entry、MULTI/EXEC内の独立予算を試験する。[stream-cgroups.tcl](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/stream-cgroups.tcl#L201-L268) は新規・PELの件数とblocking解除後の件数も試験する。これはupstream testの存在の確認であり、今回それらを実行して合格したという記録ではない。MAXSIZEの該当testは「無指定より件数が少ない」「少なくとも一件」等をassertし、全replyのwire bytesがB以下とはassertしていない。

導入先では次を追加する。下記も本稿では未実行の受け入れ試験案である。

- 空、通常、複数stream、先頭だけ高負荷、先頭が空で後続が高負荷を分け、stream別件数とlagを記録する。固定順と巡回順の両方を比較する
- MAXCOUNT/COUNTの等値、総量の方が小さい不正値、ゼロ、負数、MAXCOUNT単独を試す。エラー時に無制限readへ変わらないことを確認する
- MAXSIZEより大きいfirst-entryに加え、small-entryの後にlarge-entryが来るケースを置く。payload合計、RESP reply bytes、decode後のpeak memoryを混ぜずに計測する
- 新規readでbudget後のentryがpendingへ増えないこと、history readで未読のpendingが保持されることを確認する。配信済みentryをXACKする前の停止と、reply自体の消失も別ケースにする
- blocking解除後、通常の即時reply、RESP2、RESP3を比較する。CLAIMを使うなら拡張replyとpending優先の経路も含め、未使用ならその検証済みを装わない
- MULTI/EXEC内の先行する大きいreplyと、複数readを含むpipelineを試す。一readのbudgetがtransaction・pipeline・consumer全体の上限でないことを確認する
- budgetを有効にしてもproducer送信量が処理量を上回る状態を作り、backlogと未ACK量が監視・抑制できることを確かめる

## 限界・出典・再確認

- byte計数の全分岐、全RESP encoding、TLS overhead、output-buffer制限との競合、整数最大値、削除済みPEL entry、混在patch、Clusterやmanaged serviceのcross-slot動作は網羅していない。最悪のovershootを数式で厳密に保証したものではない
- cursor保存・XACK・claim・外部副作用を含めたexactly-once保証は提供しない。公平性対策もアプリ側の案で、Redisが提供するstarvation-free契約として引用しない
- 原資料のコード・図・長文は転載していない。docsのライセンスは[redis/docs LICENSE](https://github.com/redis/docs/blob/main/LICENSE)のCC-BY-NC-SA-4.0と旧文書部分のCC-BY-SA-4.0例外を確認したが、当該部分の切り分けは未特定。コードは固定commitの[LICENSE.txt](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/LICENSE.txt)とfile headerのRSALv2 / SSPLv1 / AGPLv3選択制を確認。release notes本文の個別条件は未特定としてcatalogへ記録した
- release_notesのTTLは30日なので再確認は2026-11-02。次回はcommand文書だけでなく停止判定・reply計数・PEL更新順を再照合し、取得日だけを延ばさない。検索evalは本書を発見できることの検証であり、Redis動作試験の代わりではない

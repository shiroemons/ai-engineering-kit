---
{
  "id": "redis-lmovem-batch-order-admission-boundary",
  "title": "Redis 8.10 LMOVEM: 一括選択・同一key順序・不足時replyと待機の境界",
  "kind": "knowledge",
  "technology": "redis",
  "version": "LMOVEM/BLMOVEM introduced in Redis 8.10.0 (2026-07-29); implementation and upstream tests pinned to 8.10.2 (2026-09-17) @ 498ecd0d6d007db11ddb3aea9428552598a78622; rolling docs compared 2026-10-04 UTC",
  "tags": [
    "research-domain:data",
    "redis",
    "LMOVEM",
    "BLMOVEM",
    "COUNT",
    "EXACTLY",
    "OBO",
    "BULK",
    "list",
    "atomicity",
    "blocking",
    "RESP2",
    "RESP3",
    "same-key",
    "batch-order"
  ],
  "sources": [
    {
      "id": "redis-lmovem-release-8-10-0-20261004",
      "url": "https://github.com/redis/redis/releases/tag/8.10.0",
      "type": "release_notes"
    },
    {
      "id": "redis-lmovem-release-8-10-2-20261004",
      "url": "https://github.com/redis/redis/releases/tag/8.10.2",
      "type": "release_notes"
    },
    {
      "id": "redis-lmovem-contract-20261004",
      "url": "https://redis.io/docs/latest/commands/lmovem/",
      "type": "official_docs"
    },
    {
      "id": "redis-lmovem-blocking-contract-20261004",
      "url": "https://redis.io/docs/latest/commands/blmovem/",
      "type": "official_docs"
    },
    {
      "id": "redis-lmovem-implementation-20261004",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_list.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-lmovem-upstream-tests-20261004",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/list-4.tcl",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-lmovem-blocked-implementation-20261004",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/blocked.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-lmovem-reply-implementation-20261004",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/networking.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-lmovem-cluster-contract-20261004",
      "url": "https://redis.io/docs/latest/operate/oss_and_stack/reference/cluster-spec/",
      "type": "official_docs"
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

# Redis LMOVEM の一括移動を安全に利用する

## 問い・適用版・判断

listの複数要素をまとめて移すとき、「同じkeyなら変更されない」「OBOならLMOVEをN回呼ぶのと同じ」「EXACTLYで待てば先に来たworkerへ必ずN件渡る」と考えてよいか。

**判断:** batchの選択、destination上の順序、件数不足の応答、待機解除を個別に確認する。特に同一keyと空sourceには、取得したrolling docsの一般的な説明とRedis 8.10.2の固定sourceに差がある。clientの型変換や後続処理は、対象patch版・RESP版で確認してから採用する。

[8.10.0 GA](https://github.com/redis/redis/releases/tag/8.10.0) は2026-07-29公開でLMOVEM/BLMOVEMを導入した。本稿の実装・test分析は、2026-09-17公開の [8.10.2](https://github.com/redis/redis/releases/tag/8.10.2)、commit `498ecd0d6d007db11ddb3aea9428552598a78622` に固定する。8.10.2で新しいbatch仕様を追加したという意味ではない。旧8.8への適用、全8.10.xでの同一性、managed serviceへの提供は保証しない。

既存の [XREAD reply budget](xread-reply-budget-pel-fairness-boundary.md) はStreamsの読取・PELを扱う。本書はlistからlistへの破壊的な一括移動に絞り、consumer groupのACKや再配送と混同しない。

## 件数の条件を先に決める

[LMOVEM契約](https://redis.io/docs/latest/commands/lmovem/) と [BLMOVEM契約](https://redis.io/docs/latest/commands/blmovem/) は、selectorとorderingを対にして指定する。LMOVEMの省略形も成功replyは1要素のarrayであり、LMOVEのscalar replyを期待したwrapperをそのまま流用しない。

固定sourceの `lmovemParseOptions` では、追加部分は `COUNT n OBO`、`COUNT n BULK`、`EXACTLY n OBO`、`EXACTLY n BULK` のいずれかに相当する3tokenである。nは正の整数で、0や負数、orderingの省略を受け付けない。以下のNは有効な正の値とする。

| 要求 | 非blocking LMOVEM | blocking BLMOVEM |
|---|---|---|
| 省略形 | 1要素あれば移す | 1要素以上になるまで待つ |
| COUNT N | 現在ある要素を最大N個移す | 空の間だけ待ち、1要素でも利用可能になれば最大N個移す |
| EXACTLY N | N個未満なら何も移さない | N個以上になるまで待ち、満たした時点でN個移す |

BLMOVEMのtimeoutは秒単位で、0は無期限である。COUNT 100は100個まで貯めるbatch形成timerではない。少量でも処理を進めたいworkerにはCOUNT、固定個数を一度に移せなければ進めない処理にはEXACTLYが候補になる。ただし、この選択は独自の運用案であり、後者が業務上のgroup IDを検証する機能を意味しない。list長がN以上という条件だけでは、N件が同じ取引・同じ顧客のgroupかは判断できない。

## 一括選択と順序: OBOを文字どおりのLMOVE loopにしない

[8.10.2のt_list.c](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_list.c#L1541-L1646) の `lmovemMoveAndReply` は、まず移すcount分を全件popして一時配列に保持する。その後に並べ替え、replyを組み立て、destinationへpushする。replyの順序はdestination上の左から右であり、popされた時系列とは限らない。

以下はその分岐から導いた独自の小例である。sourceは左から `[a,b,c,d]`、別keyのdestinationは `[x,y]`、移動数は2。実行ログではない。

| source端 → destination端 | OBOのreply / destination | BULKのreply / destination |
|---|---|---|
| LEFT → LEFT | `[b,a]` / `[b,a,x,y]` | `[a,b]` / `[a,b,x,y]` |
| LEFT → RIGHT | `[a,b]` / `[x,y,a,b]` | `[a,b]` / `[x,y,a,b]` |
| RIGHT → LEFT | `[c,d]` / `[c,d,x,y]` | `[c,d]` / `[c,d,x,y]` |
| RIGHT → RIGHT | `[d,c]` / `[x,y,d,c]` | `[c,d]` / `[x,y,c,d]` |

BULKは選択された要素の元の相対順を保つ。OBOはpop順でpushした場合のdestination順へ並べる。同じ端どうしでは両者に差が出るが、逆の端なら上の結果は一致する。

**同一keyではbatch選択が特に重要である。** `[a,b,c,d]` に `LEFT LEFT COUNT 2 OBO` を適用する固定実装の結果は `[b,a,c,d]` となる。先にaとbを取り出すためであり、「1回のLMOVEでaを左から取り、直ちに左へ戻す」を2回繰り返す手順とは違う。後者ならaを再び選び、listは変わらない。OBOという名前を理由に、client側の反復を意味的に等価なfallbackへしない。

COUNT Nでも移す数は開始時に求めたsource長以下である。同じkeyを巡回してN回分を必ず返す操作ではない。EXACTLY Nなら、その時点の長さがN未満の場合は同一keyでも不足となる。

## rolling docsとの相違を狭く扱う

2026-10-04取得の [LMOVEMのElement ordering](https://redis.io/docs/latest/commands/lmovem/#element-ordering) は、同一keyのBULKを一般的にno-opと説明する。しかし、固定実装では**同じkeyであることだけでは不変を保証しない**。

- 同一key・同じ端のBULKでは、選択した要素を元の位置へ同じ順序で戻すため、listの要素順は変わらない
- 同一key・逆の端ではrotationになる。独自例 `[a,b,c,d]` の `LEFT RIGHT COUNT 2 BULK` は `[c,d,a,b]` になる
- 上流 [list-4.tclの同一key test](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/list-4.tcl#L93-L98) も、初期値 `1 2 3 4 $large` にLEFT→RIGHT、COUNT 2、BULKを適用し、replyが `1 2`、最終listが `3 4 $large 1 2` であることをassertしている。これはsource読解に加えた上流の期待値で、今回実行した結果ではない

さらに同一key・同じ端で要素順が変わらない場合も、[samekey分岐](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_list.c#L1614-L1627) はpop/push通知、keyModified、dirty count更新を行う。したがって「要素順が不変」を「通知や書込み扱いもない」に拡張しない。入力検証のために破壊的commandを気軽に試す設計にも使わない。

別の相違は [Moving multiple elements](https://redis.io/docs/latest/commands/lmovem/#moving-multiple-elements) の空source説明である。ページはEXACTLY以外でempty arrayを返すと説明するが、[8.10.2の不足分岐](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_list.c#L1648-L1684) は次の通りである。

1. EXACTLYでsource長が不足すると `addReplyNullArray` を呼ぶ
2. 省略形またはCOUNTでも、移動数tomoveが0なら同じ `addReplyNullArray` を呼ぶ
3. [networking.cの実装](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/networking.c#L1279-L1285) はRESP2でnull array、RESP3でnullを出力する。空arrayとはwire上の型が異なる

上流testの `assert_equal {}` は、それだけでwire上の空arrayを証明しない。本稿のnull判定はTcl表現から推測せず、reply生成関数まで追った結果である。将来rolling docsまたはsourceが変わり得るので、文書が誤りだと全版へ一般化せず、8.10.2との相違として記録する。実serviceではRESP2/RESP3、clientのnil/null・空array変換を別々に試す。

## EXACTLY待機は要素の予約ではない

[blmovemGenericCommand](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_list.c#L1708-L1742) はEXACTLYの必要数とsource長を比較し、不足ならsourceへblockする。部分的に要素を別listへ移して確保する処理はない。N件に満たないlistも他のclientから見えており、別のpop/moveで消費され得る。

必要数が2以上なら `BLOCKED_LIST_NONEMPTY` を使うため、既に存在するlistの成長も再処理の契機になる。上流 [blocking tests](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/list-4.tcl#L176-L245) はRPUSHによる段階的な増加のほか、LINSERT、LMOVE、SORT STOREでも必要長に達したclientを起こす期待値を持つ。「空keyが作られた時だけ起きる」と理解しない。

[blocked.cの待機者処理](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/blocked.c#L622-L675) はkeyごとの待機者を順に再処理するが、再びblockしたclientがいても次の待機者へ進む。ここから導く運用上の注意は、**待機開始順と完了順は別**ということになる。大きいEXACTLY要求の前に小さい要求が絶対に完了しない、あるいは必要数がいつか必ず貯まる、とは保証できない。これは固定sourceに基づく推論で、飢餓の再現試験や待機時間の上限測定はしていない。

MULTI/EXECやLuaなど `CLIENT_DENY_BLOCKING` の文脈では、必要数が不足したBLMOVEMは待たずにnullを返す。従ってnullをすべて「timeout秒数を実際に待った」と記録するのも誤りになる。通常のblocking接続でも、serverから受信したtimeoutのnullと、client自身のread timeout・接続断は区別する。後二つでは移動が実行されたかをreplyだけで確認できない。

## 失敗・再試行・資源上限の設計（独自案）

次は確認した契約と固定sourceから導いた利用方針である。新commandが提供する追加保証ではない。

1. **型エラーと不足を区別する。** sourceが十分なら、destinationの型検査は全件popより前に行う。上流WRONGTYPE testもsource不変をassertする。一方、不足時の分岐はdestination型検査より前なので、null応答をdestinationが健全である証拠にしない。blocked中のdestination変更も受入試験に含める
2. **バッチ数とbytesを別に制限する。** 固定実装は移す全要素の参照配列を確保し、replyを組み立てる。COUNT/EXACTLYは要素数でありreply bytes上限ではない。巨大な単一要素でも大きな転送になるため、producer側のpayload上限、1回のN、client bufferと処理中batch数を合わせて設計する。性能やpeak memoryは今回測定していない
3. **移動と業務完了を別に管理する。** ready listからprocessing listへ移ったことは処理成功を意味しない。batchを取得するworkerの容量、完了したitemの除去、worker障害で残るitemの回収、同じpayloadを区別するitem IDを明示する。EXACTLYは取得件数の条件でありexactly-once処理ではない
4. **不明な結果を無条件に再送しない。** 応答喪失後に同じLMOVEMを再送すると次のbatchを取り得る。同一key rotationも再送でさらに回転する。必要ならrequest IDと結果照会を持つ別の設計を採り、処理先の冪等性と照合手順を検討する。LMOVEM自体にはrequest ID引数がない
5. **replication表現をACK保証にしない。** 成功したBLMOVEMは [固定実装](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_list.c#L1686-L1694) で、実際の移動件数を持つLMOVEM EXACTLYへ書き換えて伝播する。上流testもそのcommand列を確認する。これはreplicaで再びblockしないための決定的な表現であり、clientの再送を重複排除する仕組みでも、永続化完了の証明でもない
6. **Clusterのkey配置を先に決める。** [Cluster仕様](https://redis.io/docs/latest/operate/oss_and_stack/reference/cluster-spec/#implemented-subset) のmulti-key操作は同一hash slotが条件となる。例えば `ready:{batch-a}` と `processing:{batch-a}` のように同じhash tagで設計する。これはserver間でlistを自由に移すcommandではない。さらに非同期replicationではacknowledged write喪失窓があるため、原子的な一括操作とfailover時の耐久性を分けて評価する

## 受入試験と今回の限界

採用対象runtimeで、少なくとも次のcaseを独立に確認する。

- 別keyの4方向 × OBO/BULKで、replyと両listの最終値を確認する。replyだけを見ない
- 同一keyでは同じ端と逆の端、Nが長さ未満・同じ・超過するcaseを分ける
- 空sourceの省略形/COUNT/EXACTLYをRESP2・RESP3で確認し、clientがnullと空arrayをどう表現するか記録する
- COUNTは少数の投入で解除され、EXACTLYは不足中に何も移さず、必要長に達してから解除されることを確認する
- 複数workerで大小のEXACTLYを混在させ、後続の小batchが先に進み得る運用でも整合性を保てるかを確認する
- source/destinationのWRONGTYPE、blocked中のkey変更、有限timeout、MULTI/EXEC内の不足、返信直前後の接続断を分ける
- batch処理途中のworker終了、残存itemの照合と回収、同一ID再処理を実際のconsumerで検証する

今回、Redis runtimeは実行していない。`redis-server` がこの環境のPATHにないことを確認し、source/test読解を実測と表示しなかった。上流list-4.tclの実行、wire capture、Cluster、Lua実行、複数待機者の公平性、切断・failover、OOM、durability、client libraryとの互換性は未検証である。検索evalは本稿へ到達するための検査であり、これらの代替ではない。

## 出典・取得日・ライセンス

- 全sourceの取得日は2026-10-04 UTC。GA 8.10.0の公開日は2026-07-29、分析した8.10.2は2026-09-17で、GitHub release APIのpublished_atを確認した
- LMOVEM/BLMOVEM、8.10 release notes、8.10.2 release、Cluster仕様、導入PRをnative Webで開いた。固定sourceとtest、tagの完全SHA、公開日時はGitHub connectorでも確認した。rolling command referenceにはページ固有の公開日がない
- Redis sourceは完全commitの [LICENSE.txt](https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/LICENSE.txt) を確認し、Redis 8のRSALv2 / SSPLv1 / AGPLv3選択制をcatalogへ記録した。blocked.cとnetworking.cにはBSD3部分の注意書きがあり、list-4.tclには個別headerがない。特定の旧寄与部分へのlicense選択は判断していない
- Redis docsの [LICENSE](https://github.com/redis/docs/blob/main/LICENSE) はCC-BY-NC-SA-4.0と旧redis-doc由来部分のCC-BY-SA-4.0例外を示す。個別ページ内の例外範囲は未特定。GitHub release本文の個別ライセンス範囲はunknownとした
- 内容は独自の要約・分析・設計案であり、上流実装の転載やmoduleへの移植はしていない。source上の矛盾候補について上流へissueを送信したという事実もない

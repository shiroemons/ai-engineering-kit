---
{
  "id": "redis-preload-source-restart-boundary",
  "title": "Redis preload-file復元: legacy AOFのsource保持とpreloadなし再起動の境界",
  "kind": "knowledge",
  "technology": "redis",
  "version": "Redis 8.10.2 (2026-09-17) static comparison at 498ecd0d6d007db11ddb3aea9428552598a78622; upstream fix #15916 at 940a4d72fc7caaed6f853b65b39920ff3fb9ac1f (2026-10-08); October 2026 8.12-RC1 announcement, tag/date and stable backport unverified; docs retrieved 2026-10-24 UTC; runtime not tested",
  "tags": [
    "research-domain:data",
    "redis",
    "preload-file",
    "legacy-AOF",
    "restore",
    "source-preservation",
    "restart",
    "appendonlydir",
    "manifest",
    "BASE",
    "INCR",
    "8.12-RC1"
  ],
  "sources": [
    {
      "id": "redis-preload-contract-20261024",
      "url": "https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/",
      "type": "official_docs"
    },
    {
      "id": "redis-preload-release-8102-20261024",
      "url": "https://github.com/redis/redis/releases/tag/8.10.2",
      "type": "release_notes"
    },
    {
      "id": "redis-preload-announcement-812rc1-20261024",
      "url": "https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.12-release-notes/",
      "type": "release_notes"
    },
    {
      "id": "redis-preload-old-startup-20261024",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/server.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-preload-old-aof-20261024",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/aof.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-preload-fix-20261024",
      "url": "https://github.com/redis/redis/commit/940a4d72fc7caaed6f853b65b39920ff3fb9ac1f",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-preload-installation-20261024",
      "url": "https://github.com/redis/redis/blob/940a4d72fc7caaed6f853b65b39920ff3fb9ac1f/src/aof.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-preload-tests-20261024",
      "url": "https://github.com/redis/redis/blob/940a4d72fc7caaed6f853b65b39920ff3fb9ac1f/tests/integration/backup.tcl",
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

# Redis preload-file復元の成功条件を分ける

## 問いと判断

legacy AOFを`preload-file`で読み込み、メモリ上に期待するkeyが現れたら復元は完了か。唯一の復旧用sourceをRedisの稼働directoryへ置いて、そのまま起動しても安全か。

**判断: メモリへの読込、source保持、local AOFの組立て、preloadなし再起動の成功は別々に検査する。** Redisの2026-10-08の修正#15916は、条件が重なるとsourceを移動した後で削除してしまうstartup順序を変更した。fsync方針を強くするだけではこのpath処理の問題を解消しない。修正の存在と本番で使うbinaryへの収録確認も別である。

既存の[RDB/AOF persistence](rdb-aof-persistence-durability.md)は書込みdurability、fsyncとrewriteを扱う。本書は`preload-file`による復元結果を次回起動へ引き継ぐ境界に絞る。BACKUP作成の全state machine、Clusterの全node同時点snapshot、replica昇格の保証は扱わない。

## 版・日付・証拠の強さ

- [8.10.2 release][release]は2026-09-17 15:06:56 UTC公開。比較する固定sourceは`498ecd0d6d007db11ddb3aea9428552598a78622`であり、現在のlatestや全8.10.xの挙動とは主張しない。
- [修正commit][fix]は`940a4d72fc7caaed6f853b65b39920ff3fb9ac1f`、2026-10-08 13:08:40 UTCのupstream修正。修正差分・その時点の実装とtestを読み、実行した結果と区別する。
- [8.12-RC1の告知][rc]はOctober 2026の欄で#15916を列挙し、Release Candidateはproductionに不適と明記する。これは**告知の確認であり、download可能な8.12-RC1 tagや正確な公開日の検証ではない**。確認したGitHub release取得経路は404となったため、告知だけからtagの実体や収録SHAを補完していない。404をrelease不存在の証明にもしない。
- 2026-10-24 UTC時点で、本稿はstable backportを確認していない。RCへの本番upgradeを推奨せず、採用するstable版・vendor buildについて修正の収録とrestore試験を別途確認する。
- rollingな[公式復元文書][contract]は同日に再取得した。公開日・各説明の追加日は未確認。仕様説明、固定sourceの観察、下記の独自運用案を混同しない。

## preloadは既存databaseへの追加ではない

[公式文書のRestoring a backup][contract]によれば、startup-only設定`preload-file`は`aof:<path>`または`rdb:<path>`で指定する。AOF manifestならその参照fileも読み込む。指定があると通常の`appenddirname`と`dump.rdb`のロードをskipし、指定targetの欠落や読込失敗ではstartupを中止する。ロード後は設定されたpersistence modeで稼働する。

ここから「既存local AOFも読み、その上へbackupのkeyをmergeする」と解釈しない。固定[upstream tests][tests]も、あらかじめlocal AOF/RDBへ入れた識別keyがpreload結果へ混ざらないことを検査している。復元先に残っている業務データの保存を必要とするなら、preloadをその保存策としない。

また`appendonly no`で読めたことは、local AOFが構築された証拠ではない。[固定実装][installation]の`aofSetupAfterPreloadFile`はpreload未指定またはAOFがONでない場合に戻る。RDB設定を含む他のpersistence条件まで本書で保証するものではない。

## #15916が直す具体的な欠落経路

### 「8.10.2ならすべてのrestoreで欠落」と一般化しない

比較した[8.10.2 startup][old-startup]と[AOF実装][old-aof]には次の経路がある。説明用の`D`はRedisの`dir`、`appendonly.aof`と`appendonlydir`はこの例で用いる名前である。

1. sourceが`D/appendonly.aof`にあるlocal legacy AOFで、例えばDが`/data`なら`preload-file aof:/data/appendonly.aof`と`appendonly yes`を指定している。
2. preloadによりメモリへの読込は成功する。
3. startupは`loaded`かつ`AOF_ON`で`upgradeAofIfNeeded`を呼ぶ。local legacy fileが存在し、例えば`appendonlydir`がまだ無い場合、legacy upgrade条件に入る。
4. `aofUpgradePrepare`がmanifestを用意し、sourceを`D/appendonlydir/appendonly.aof`へrenameする。
5. 後段の`aofSetupAfterPreloadFile`は元のpreload pathをsourceとしてinstallationを進める。sourceとtargetのpathが異なるため、installerはtargetをunlinkしてからsourceへのhardlink、失敗時のcopyを試みる。
6. 元のsource pathは既に移動済みで存在しない。一方targetのfileは直前に削除したため、installationに失敗してexitし、唯一のdisk copyを失いうる。

これは固定sourceと[upstream修正説明][fix]に基づく静的な失敗経路であり、今回この環境で再現した障害ではない。`upgradeAofIfNeeded`にはdirectory不在以外のmanifest状態の分岐もあるが、すべてのpath・既存manifest・preload形式を同じ結果とみなさない。別directoryのsource、RDB、current manifestは条件を分けて評価する。

### 修正は順序の変更であり、全restoreのtransaction化ではない

[修正差分][fix]は、successful preloadの後に行っていたupgrade callを除き、`!loaded && AOF_ON`の通常AOF loadを残す。preload後のinstallationを`aofSetupAfterPreloadFile`に任せることで、先にsource pathを移す問題を避ける。通常のnon-preload startupのlegacy upgradeを全廃した修正ではない。

この差分から、disk full、permission error、途中crashのあらゆる地点で古いdatasetへrollbackできるとは結論しない。修正後のinstallerにもfile置換とcleanupがあり、起動途中のfailure atomicityは別途試験が必要である。

## 修正後にも残るsource・destinationの区別

[修正時点のaof.c][installation]の観察は次の通りである。

- standalone AOF/RDBはlocal manifestのBASEとして扱う。入力がmanifestならそのBASEとordered INCRを使用し、HISTORYは持ち越さない。
- installationのsourceとtargetのpathが一致すれば、そのfileをその場で利用する。異なる場合はtargetを置換し、hardlinkを優先して失敗時はcopyへ進む。directory entryのfsyncも行う。
- 新manifestで参照しないlocal directory内のnon-directory entryはcleanup対象になる。従って`appendonlydir`を無関係なarchiveの保管場所にしない。
- 既存の入力fileへ復元後のwriteを追記しないよう、fresh INCRを確保する。既に使われているfile名を避けてsequenceを進めるため、入力のbasenameが`.incr.aof`でも、その名前だけでlocal manifest内の役割を判断しない。
- manifestのpublishと、その後のlocal AOF利用が必要になる。source fileが残ったことだけでlocal datasetの復元可能性を判定しない。

hardlink成功を「別storageに独立backupができた」と読むことも避ける。同一fileを参照するlinkと独立保管copyは障害範囲が異なる。この区別と、元artifactを別に保全する方針は本書の運用判断である。

### current-local-manifestの再利用例外

`preloadFileIsCurrentAofManifest`が現在のlocal manifestと判定すると、`aofSetupAfterPreloadFile`はreplication offset関係の更新後に戻る。通常startup側がexisting INCRを開く経路であり、外部artifactをimportする場合と同じfresh INCR追加を必ず行うとはいえない。[固定実装][installation]

判定は`aof:/` prefix、`.manifest`拡張子と、構築したcurrent absolute pathとの文字列比較を用いる。任意のsymlink、別名、同じinodeであるだけのpathがすべて同一扱いになるとは保証しない。上流testが扱うのはexact absolute pathのcurrent local manifestであり、`aof_rewrites`が0であることを確認する。「rewriteなし」を「diskに一切変更なし」「任意のbackupがそのまま安全」と拡大しない。[upstream tests][tests]

## 上流testが区別している成功条件

以下は[固定backup.tcl][tests]を読んだ結果であり、本調査で実行したtest結果ではない。

| 対象 | 上流で照合するもの | そこからは保証できないもの |
|---|---|---|
| legacy AOF regression | sourceの存在、local BASEの存在、preload key、復元後write、preloadなし再起動で両key | 全payloadのdigest一致、全OS・filesystem、任意crash地点 |
| standalone AOFの名衝突 | `.1.incr.aof`という入力をBASEへ登録し、fresh INCRを別sequenceへ進める | basenameだけでfile役割を判定できること |
| 外部sealed manifest | local manifestへの入力参照の引継ぎ、fresh INCR、再起動後の元keyと新key | 全node共通snapshotやapplication invariant |
| current local manifest | local keyが読め、不要なrewriteが起きない | importと同じfresh INCR追加、全path aliasの同一判定 |
| appendonlydir内のRDB source | sourceを残し、不要な旧INCRを除去して新しいINCRへ置換 | directory内の無関係なfileも保存されること |

特にlegacy regressionは`appendfsync always`を設定している。ここで防いでいるのはfsyncの弱さではなく、sourceをrenameした後にinstallationが旧pathを使う問題である。永続化policyと復元file操作の順序を別に検証する。

## 復旧手順に組み込む判断（独自案）

次は資料から導く本書の提案であり、Redisが追加で提供するtransaction保証ではない。

1. **入力とlive directoryを分離する。** 元artifactを独立した場所へ保全し、復元はその作業copyと隔離されたtargetで行う。唯一のsourceをstartupの移動・cleanup対象に置かない。manifestの場合は参照するfile集合、順序、サイズやdigestも記録する。
2. **binaryの修正収録を確認する。** stable release/vendor buildのsourceまたは提供者のbackport情報を確認し、正確なpatch版で試す。8.12-RC1の告知やfixed commitの存在だけで採用binaryが修正済みとは判定しない。
3. **最初の起動では置換結果を検査する。** 期待するdatabase番号・key・type・TTL・代表値を確認する。意図しない旧local keyが混ざらないこと、sourceが保持されること、local manifestの各参照fileが存在することを別々に検査する。件数一致だけでは内容同一性の証明にならない。
4. **復元後のwriteを試し、preloadなし再起動を行う。** 隔離targetに識別可能なprobeを作り、次回起動では`preload-file`を外して、元データとprobeの両方を検査する。毎回同じpreload inputを読む試験では、local persistenceの組立てを検証できない。逆に古いpreloadを残したままの再起動は、その入力を再選択するため復元後のwriteを失う原因になりうる。
5. **cutoverとcleanupを別判定にする。** applicationの整合性検査と再起動試験を通すまで、本番trafficの切替や元artifactの処分を進めない。失敗時はtargetのログ・manifest・残存fileを保全し、同じdirectoryへの盲目的な再起動や修復で証拠を上書きしない。

## 未実行のruntime試験案と限界

受入環境では少なくとも次を分ける。これらは試験案であり、成功済みチェックリストではない。

- local legacy AOF、別directoryのstandalone AOF、外部manifest、current local manifest、RDBの各入力。
- `appendonly yes/no`、空target、既存local AOF/RDBがあるtarget、入力file名とINCR名の衝突。
- hardlink可能なfilesystemとcopy fallback、read-only source、destinationのpermission error、disk full、installation途中のcrash。
- source内容の保持、manifest参照の完全性、fresh INCRの追記先、preloadなし再起動後の元データ・probe・TTL。

Redis serverもupstream testも今回は実行していない。stable backport、最初のaffected release、managed serviceへの適用、Cluster/replicaの復旧整合性、large dataset性能、電源断時のfile-system durabilityは未確認である。検索evalは本書を取得できるかの検査であり、restore動作の試験ではない。

codeのライセンスは固定commitの`LICENSE.txt`と該当C file headerでRSALv2 / SSPLv1 / AGPLv3の選択制を確認し、server.cのBSD3由来部分への注記も確認した。本文は独自要約で、codeの移植は行っていない。公式web文書の再利用licenseは未確定のためcatalogにはunknownを記録した。native webでrelease notes・公式文書・修正差分を開き、固定fileの取得がcache missとなった箇所はGitHub connectorで同じSHAを指定して照合した。

[contract]: https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/
[release]: https://github.com/redis/redis/releases/tag/8.10.2
[rc]: https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.12-release-notes/
[old-startup]: https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/server.c
[old-aof]: https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/aof.c
[fix]: https://github.com/redis/redis/commit/940a4d72fc7caaed6f853b65b39920ff3fb9ac1f
[installation]: https://github.com/redis/redis/blob/940a4d72fc7caaed6f853b65b39920ff3fb9ac1f/src/aof.c
[tests]: https://github.com/redis/redis/blob/940a4d72fc7caaed6f853b65b39920ff3fb9ac1f/tests/integration/backup.tcl

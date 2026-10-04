---
{
  "id": "distributed-systems-etcd-rangestream-snapshot-completion",
  "title": "etcd 3.7 RangeStream: 部分結果・最終metadata・過去revisionを分けたsnapshot構築",
  "kind": "knowledge",
  "technology": "distributed-systems",
  "version": "RangeStream introduced in etcd v3.7.0 (2026-07-08); v3.7.2 @ 68c065e562994b89e333e77b039ad066f933c586 (2026-09-22); verified 2026-10-04 UTC",
  "tags": [
    "research-domain:api-distributed",
    "etcd",
    "RangeStream",
    "GetStream",
    "snapshot",
    "partial-result",
    "revision",
    "compaction",
    "stream-completion",
    "migration"
  ],
  "sources": [
    {
      "id": "etcd-37-rangestream-api-20261004",
      "url": "https://etcd.io/docs/v3.7/learning/api/",
      "type": "official_docs"
    },
    {
      "id": "etcd-37-rangestream-announcement-20261004",
      "url": "https://etcd.io/blog/2026/announcing-etcd-3.7/",
      "type": "release_notes"
    },
    {
      "id": "etcd-372-rangestream-release-20261004",
      "url": "https://github.com/etcd-io/etcd/releases/tag/v3.7.2",
      "type": "release_notes"
    },
    {
      "id": "etcd-372-rangestream-client-20261004",
      "url": "https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/client/v3/kv.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "etcd-372-rangestream-server-20261004",
      "url": "https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/server/etcdserver/v3_server.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "etcd-372-rangestream-mvcc-20261004",
      "url": "https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/server/storage/mvcc/kvstore_txn.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "etcd-372-rangestream-tests-20261004",
      "url": "https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/tests/common/kv_test.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# etcd 3.7 RangeStream: 部分結果・最終metadata・過去revisionを分けたsnapshot構築

## 問いと採用判断

大きなprefixの全件読み込みをunary RangeからRangeStreamへ置き換えるとき、数chunkを受け取れたことを「cache初期化が完了した」と扱ってよいか。**chunkは候補データとして処理し、streamの正常終了・最終metadata・要求範囲の充足を確認してから、完成したsnapshotとして公開する**。streamingは配送形態を変えるが、途中の集合を全件集合へ昇格させる保証ではない。

[SIG-Etcdのv3.7.0発表](https://etcd.io/blog/2026/announcing-etcd-3.7/)は2026-07-08公開で、RangeStreamをこのminor版の新機能とする。3.6以前の全応答bufferingに対し、大きな結果をchunk単位で渡せることが主な変更である。本稿は[現行v3.7 API文書](https://etcd.io/docs/v3.7/learning/api/#rangestream)と、[v3.7.2](https://github.com/etcd-io/etcd/releases/tag/v3.7.2)の固定実装を照合する。3.7.2で初めて導入されたとは主張しない。

既存の[etcd lease/fencing文書](etcd-lease-fencing-resource-boundary.md)が扱う所有権と外部資源の保護とは別の、**大きな読み取り集合の完成判定**が対象である。ここでいうsnapshotはapplicationの論理的なkey集合であり、etcdのdisaster-recovery backup fileではない。

## 公式契約: 最初のchunkには完了情報がない

v3.7 API文書のRangeStreamはRangeRequestを受け取り、RangeStreamResponseの中に部分的なRangeResponseを返す。各chunkのKvsは互いに重ならず、到着順に連結すると対応するRangeの結果になる。重要なのはfieldの出現時点である。

- Kvsは各chunkに分割される
- Header、More、Countは正常に完成するstreamの最終chunkだけに入る
- 中間chunkのこれらのfieldはzero valueであり、そこから全件数・最終revision・残件有無を判断しない
- server側の処理が途中で失敗すれば完成したmetadataは得られない。受信側でもtransport errorと正常終了を区別する

したがって、中間応答のMore=falseをEOFの代わりにしたり、Count=0を空prefixの証明にしたりする実装は誤りになる。最終chunkを受け取った後も、RPCの正常終了まで確認する。channelが閉じたことだけを見て、直前のerror要素を無視するのも同じ種類の欠落である。

これは読み取りの整合性と利用側の可視性を分ける話である。serverが一貫したrevisionから読んでいても、clientが前半だけで公開cacheを上書きすれば、利用者に不完全な集合を見せられる。後半に存在したはずのkeyを「削除済み」と誤認する処理は、特に完了判定へ依存する。

## 固定実装: stream内の同一revisionとheaderの意味

以下はetcd v3.7.2、commit 68c065e562994b89e333e77b039ad066f933c586の読解であり、すべての版への保証や実行試験結果ではない。

[serverのRangeStream/rangeStream](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/server/etcdserver/v3_server.go)は、Serializableを指定しない読み取りでLinearizableReadNotifyを経由する。その後、chunkごとに内部のRangeを実行する。

Revision=0で開始すると、最初のRangeが返したstore revisionを後続chunkの要求revisionへ固定する。このため、stream開始後の書き込みを次々に取り込むlive tailではない。WithSerializableを指定すればquorumに対して古いmember-local状態を読む可能性があり、固定revisionであることとfreshnessは別に評価する。

### 明示した過去revisionをheaderで上書きしない

明示的に正の過去revision Rを指定した場合は、要求のRが値を選ぶ時点として保持される。一方、[MVCCのrangeKeys](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/server/storage/mvcc/kvstore_txn.go)は要求revisionで値を選んでも、RangeResult.Revにはそのread時のcurRevを返す。[txn/range.go](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/server/etcdserver/txn/range.go)がこれをHeader.Revisionへ載せ、RangeStreamは最初のheader revisionを最終応答まで保持する。

従って、**明示した過去Rと、最終Header.Revisionを無条件に同一視しない**。例として、R=40のデータを読み、開始時点のstore revisionが70なら、snapshotが40時点なのにheaderが70を示し得る。そこからWatchを71で開始すると41〜70の対象更新を飛ばせる。この数値例は上記実装から導く独自の反例である。

設計案として、snapshotのdata revisionを別の状態値で持つ。Revision=0による成功した全件取得なら最終headerを使い、過去Rを明示した取得ならRを保持する。Watchのstart revisionはinclusiveなので、そのsnapshotの次に必要な変更はdata revision+1から読む。履歴が既にcompactedなら、欠けた変更が復元できたと扱わず新たに全件を取り直す。負のrevisionを指定したstreamの挙動は本稿の適用対象に含めない。

## Limit・More・Countはchunk sizingとは別の契約

同じserver実装では、要求Limitを全streamの返却上限totalLimitとして保持する。Limit=0は無制限として扱い、内部chunkは別のLimitで取得する。成功した最後の応答でMore=trueなら、**このRPCが正常終了しても、指定範囲には未返却のkeyが残っている**。

最終Countは、要求Limitで返したKvs数だけを表す値ではない。上限で打ち切った場合、実装は残りのrangeを同じ要求revisionで数え、返却数に加える。CountOnlyならKvsを返さずCountを返す別経路もある。全件cacheを作る用途では、streamのEOFだけでなく、More、Limit、CountOnly、KeysOnlyの要求条件も確認する。

内部のinitialStreamChunkLimitは10で、その後は直前応答のprotobuf sizeとMaxRequestBytesを比較して件数を増減する。これは固定実装の観察であり、**1chunkのbyte数の厳密な上限ではない**。値の大きさが不均一な場合を含め、clientが「常に最大N bytes」と仮定できる契約ではない。要求Limitを下げることも、chunkのbyte予算を指定することとは違う。

実務上の選択は次のとおりである。

- 全件初期化なら、処理できる総量・期限を別に制限しつつ、全範囲を最後まで取得する
- 意図的なページ化なら、固定したdata revisionとkey範囲を維持して残件を読む。ページ間で毎回latestを指定して単一snapshotと呼ばない
- 個別keyを逐次処理できる用途でも、未取得keyに対する削除・不在判定は全体が完成するまで保留する

上記は利用側の設計案である。RangeStreamにapplication用の永続resume tokenや、障害を越えた処理のexactly-once保証が追加されたという意味ではない。

## Go client: 開始成功・途中error・自分の処理中断を分ける

[client/v3/kv.go](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/client/v3/kv.go)のGetStreamは二つのerror経路を持つ。

1. RPCを開始できなければ、GetStream自身がerrorを返す
2. 開始後のRecvでio.EOF以外のerrorになれば、最後にRangeStreamResponseのErr()へerrorを載せた要素を送り、その後channelを閉じる。この要素のRangeResponseはnilになる

各要素についてErr()を先に確認し、その後にKvs等へアクセスする。開始時のerrorがnilだから全部成功したと扱わない。[TestKVGetStreamCompactedError](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/tests/integration/clientv3/kv_test.go)も、古いWithRevでGetStreamを開始した時点はnil errorでも、消費するとtyped ErrCompactedになる経路を検査している。このupstream testは今回実行していない。

GetStreamToGetResponseは各要素のerrorを確認し、proto.Mergeで単一のGetResponseへ集約する。通常のGetに近い結果を得るためのhelperだが、clientで全Kvsを保持するため、**merge helperを使えばclientの全件bufferが消えるわけではない**。逐次処理を選んでも、自作の無制限queueへchunkを積めばmemory問題は残る。

### cancelだけで受信goroutineが必ず解放されるとしない

同じv3.7.2実装は容量1のrespChへ無条件sendし、send時にcontext取消をselectしていない。利用側が途中で読むのをやめると、clientの受信goroutineがchannelへの送信待ちで残る経路がある。contextをcancelしただけでは、既にblockしたsendを必ず解除できるとは言えない。

独自の終了設計は、処理中断時に自分が所有するcontextをcancelし、以後の要素を適用せず、errorを含めchannelが閉じるまでdrainする責任を明確にすること。中断を成功へ変換しない。これは固定client実装のlifecycle上の注意であり、全SDKの挙動や将来版の修正状況は未確認である。

## 移行: 同じRangeRequest型でも完全な置換ではない

[v3rpcのcheckRangeStreamRequest](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/server/etcdserver/api/v3rpc/key.go)は、非既定orderingおよびrevision filtersをUnimplementedで拒否する。ここでrevision filtersとはMin/MaxModRevisionとMin/MaxCreateRevisionであり、snapshotを固定するRevision自体が禁止なのではない。[IsDefaultOrdering](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/server/etcdserver/txn/range.go)では、通常の既定順に加えKEY+ASCENDも既定扱いとなる。「sort fieldを一つでも指定したら必ず失敗」と一般化しない。

[etcd gRPC proxy](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/server/proxy/grpcproxy/kv.go)のRangeStreamはUnimplementedを返す。ここで指すのはetcd実装のgRPC proxyであり、すべての汎用L4/L7 proxyが同じ制約という主張ではない。

採用時には、clientのversionだけでなく、接続し得るserver/memberと経路の対応を調べる。[upstreamの共通KV test](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/tests/common/kv_test.go)も全memberをprobeし、非対応optionのcaseを除外している。これはそのtestの準備動作であり、mixed-version clusterのすべての互換性を保証する資料ではない。

独自のfallback方針として、非対応なら同じsort・filter・revision semanticsを保つunary Rangeへ戻すか、明示的に利用を停止する。errorを消すためにfilterを落とす、descendingをascendingへ変える、historical revisionをlatestへ変更するのは避ける。途中まで取得した旧attemptとfallback結果も連結しない。unaryへ戻ると全応答のmemory負荷が再発し得るため、最大規模・同時実行数・deadlineを別途決める。

## snapshotを公開するまでの設計案

ここからは本書の推奨構成で、etcdの追加保証ではない。

1. attemptごとの非公開generationを用意し、期待するcluster、key範囲、要求revision、Limit等を記録する
2. chunkを検査しながらそのgenerationへ保存する。公開中のcacheや不在key一覧はこの段階で置換しない
3. 開始時error・各要素Err()・自分の処理失敗・期限超過を記録する。失敗attemptをcompleteへ遷移させない
4. 正常終了と最終metadataを確認し、More=false、全件用途に適した要求条件、必要なdata revisionが揃ったときだけcompleteにする
5. generationへの保存も完了した後、公開先のpointer等を一度に切り替える。snapshot完成と、後続Watchが必要な位置から追従できたことは別の状態として扱う
6. 失敗なら旧generationを維持し、候補を破棄または隔離する。retryは新attemptとしてやり直す

大きい集合では一時diskや別namespaceへ候補を置くなど、streamingの利点を失わない保存先を選ぶ。公開切替を原子的にできるかは利用側storageの契約であり、RangeStreamが外部cacheのtransactionまで作るわけではない。

## 境界試験と未確認事項

以下は導入前の受入試験案で、今回の実測結果ではない。

- 中間chunkのMore=false、Count=0、Header未設定で処理を終わらないこと
- 複数chunk取得後の通信断・deadline・consumer側検証失敗で、未完成generationを公開しないこと
- 正常EOFでも最終More=trueのlimit付き要求を全件完了にしないこと
- 空prefixとCountOnlyの正常結果を、途中errorによる空結果と分けること
- R=40指定・開始時store revision70という条件で、data revisionをheaderへ置き換えずWatch接続位置を決めること
- 取得中のcompactionで失敗する可能性を扱い、部分結果の再利用を「取りこぼしなし」とみなさないこと
- sort/filter非対応、etcd gRPC proxy経由、異なるserver版を、安易なquery書き換えなしで処理すること
- consumerの途中中断でcancelとdrainを完了し、受信goroutineが残らないこと
- 不均一な巨大valueと遅いconsumerで、client総memory・一時領域・queue長・完了時間を測ること

実cluster、負荷試験、network/compaction fault injection、upstream testの実行は行っていない。chunk sizeの上限値、性能改善率、任意言語SDKの互換性、Kubernetesでの利用開始版、全mixed-version upgrade経路は未検証。検索evalはこの知識を見つけられることだけを検証する。

## 出典・日付・provenance

- v3.7 API実ページと2026-07-08のrelease発表を2026-10-04 UTCにnative webで開いた。APIページの表示更新日は2026-05-05であり、機能release日とは区別する
- v3.7.2 releaseを開き、GitHub APIの公開日時2026-09-22T21:22:39Z、annotated tagのdereference先commit 68c065e562994b89e333e77b039ad066f933c586を確認した。実装・testはこのcommitに固定して取得した
- etcd Authorsのwebsite文書は[固定LICENSE](https://github.com/etcd-io/website/blob/d20f8ae346da60ab6eb6f24f200753b328aa9fcc/LICENSE)で、例外を除く文書CC-BY-4.0、code/sample Apache-2.0と確認した。実装は[固定LICENSE](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/LICENSE)と各file headerでApache-2.0を確認。release本文固有のライセンスはunknownとした
- 本文は独自の日本語要約と設計案で、コードの転載・module化・性能保証はしない。共有source recordを書き換えず新規記録を追加し、release_notesの30日TTLに合わせ再確認期限を2026-11-03とした

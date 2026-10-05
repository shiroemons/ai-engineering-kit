---
{
  "id": "messaging-nats-fast-ingest-gap-count-completion-boundary",
  "title": "NATS fast-ingest: gap通知・count・保存済みprefixと終了判定の境界",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "NATS Server v2.15.0 @ eb763679aa3c24a40dcd3012aa046ad1996d851c; ADR-50 rev.11 / snapshot 7e6e7d70ed9db187083f63437d49bcf02146957c; backward-gap source fix b43b0d37a57113a98600225a5674a2ce97a6ebc4 dated 2026-10-02, containing release unverified",
  "tags": [
    "research-domain:api-distributed",
    "messaging",
    "nats",
    "jetstream",
    "fast-ingest",
    "batch-publish",
    "gap",
    "puback",
    "flow-control",
    "deduplication"
  ],
  "sources": [
    {
      "id": "nats-fast-214-announcement-20261005",
      "url": "https://nats.io/blog/nats-server-2.14-release/",
      "type": "maintainer_article"
    },
    {
      "id": "nats-fast-adr50-7e6e7d70-20261005",
      "url": "https://github.com/nats-io/nats-architecture-and-design/blob/7e6e7d70ed9db187083f63437d49bcf02146957c/adr/ADR-50.md",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-fast-server-stream-2150-20261005",
      "url": "https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/stream.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-fast-server-progress-2150-20261005",
      "url": "https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_batching.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-fast-server-tests-2150-20261005",
      "url": "https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_batching_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-fast-backward-gap-fix-b43b0d37-20261005",
      "url": "https://github.com/nats-io/nats-server/commit/b43b0d37a57113a98600225a5674a2ce97a6ebc4",
      "type": "github_repository_analysis"
    },
    {
      "id": "nats-server-2150-release-20261003",
      "url": "https://github.com/nats-io/nats-server/releases/tag/v2.15.0",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# NATS fast-ingest の終了を入力全件の成功と取り違えない

## 問いと採用判断

JetStream へ大量のmessageを送ったとき、最後の `PubAck` がerrorなしで返れば、最初に渡した全messageを保存したと言えるか。途中のgap通知から再送範囲を決めてよいか。

**fast-ingest は途中まで保存してから終了できる。`gap:fail` も全件rollbackではなく、`PubAck.Error == nil` も入力全件成功の証明にはならない。`count` を保存件数にせず、入力の最終位置・gap mode・重複排除・通知種別を合わせて判定する。** 全件一括の可視化が要件なら、boundedなatomic batchまたはアプリ側の完成manifestを検討する。

[2.14発表記事](https://nats.io/blog/nats-server-2.14-release/)は2026-04-30公開で、fast batchの導入を説明している。2.12のatomic batchはstaging後に一括保存する一方、fast-ingestはmessageごとに進め、flow controlで送信量を調整する。本稿は後者の完了判定に限定し、atomic batch全般の解説には広げない。

既存の[NATS保守文書](nats-jetstream-desired-state-evacuation.md)はserver退避・配置変更、[Kafka transaction](kafka-transaction-timeout-lso-recovery-boundary.md)はtransaction再開とoffset、[RabbitMQ confirms](publisher-confirms-mandatory-retry.md)はAMQPのpublish確認を扱う。今回の対象はNATS固有のfast-ingest control channelとbatch sequenceであり、これらのack契約を流用しない。

## 適用版を二層に分ける

| 根拠 | 確認した版・日付 | ここで保証の根拠にする範囲 |
|---|---|---|
| released server | [v2.15.0](https://github.com/nats-io/nats-server/releases/tag/v2.15.0)、2026-09-17公開、`eb763679aa3c24a40dcd3012aa046ad1996d851c` | 固定実装と同commitのupstream testに書かれた動作 |
| 承認済み設計文書 | [ADR-50](https://github.com/nats-io/nats-architecture-and-design/blob/7e6e7d70ed9db187083f63437d49bcf02146957c/adr/ADR-50.md)、revision表11は2026-09-30、snapshot commitは2026-10-02 | 現在読んだ設計意図。後から加わった文を旧releaseへ遡及させない |
| 追加のsource修正 | [backward-gap fix](https://github.com/nats-io/nats-server/commit/b43b0d37a57113a98600225a5674a2ce97a6ebc4)、2026-10-02 10:28:38 UTC | 後退・同一sequence再送時の通知変更。取り込まれた正式releaseは未確認 |

ADRはIETF標準ではなくNATS開発元の設計資料で、statusはApprovedである。revision表の日付だけでは最後の変更日を表せない。10月2日のADR差分は、後退gapの通知と、expected-header失敗を `BatchFlowErr` で送る点を明記した。これは「10月2日にfast-ingest自体を新設した」という話ではない。

以下の「保存済み」はackとstream進捗の意味で使う。電源断への永続性、retention後の現存、consumerによる業務完了までを含めない。とくに `PersistMode: async` は後述の別条件になる。

## 同じinboxに来る四種類を混ぜない

[2.15.0のresponse型とhandler](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/stream.go)は、次の情報を別々に持つ。

| message | 主なfield | 判定上の意味 |
|---|---|---|
| `BatchFlowAck` | `type:"ack"`, `seq`, `msgs` | `seq` はbatch内の進捗。`msgs` は以後何messageごとにackするか。stream sequenceではない |
| `BatchFlowGap` | `type:"gap"`, `last_seq`, `seq` | `last_seq` は次に期待したbatch位置。名前にlastとあっても最後の保存済み位置ではない |
| `BatchFlowErr` | `type:"err"`, `seq`, `error` | 当該batch位置での個別エラー。そこまでの保存範囲を表さない |
| 最終 `PubAck` | `stream`, `seq`, `batch`, `count`、または `error` | `seq` はstream位置、`count` は後述のbatch進捗。保存情報が返っても予定した全入力の成功とは限らない |

streamのopt-inは `AllowBatchPublish`（JSONは `allow_batched`）であり、atomic用の `AllowAtomicPublish` とは別である。本稿のackを待つ設計ではNoAckを有効にせず、対応server/clientを実際に組み合わせて確認する。

開始時の `BatchFlowAck seq=0` はflow設定の応答であり、第1messageの保存確認ではない。最初の応答を待たずに送り続けると、streamの機能未設定や低い初期flowを無視してしまう。1messageで即commitする形は最初からPubAckになり得るため、「初回は必ずtype=ack」というparserにも固定しない。

[ADRのMessage Gaps / Server Errors](https://github.com/nats-io/nats-architecture-and-design/blob/7e6e7d70ed9db187083f63437d49bcf02146957c/adr/ADR-50.md#message-gaps)では、gapとerrは情報通知で、失われ得る。gapは保存ackと順不同で届き得るため、受け取った `gap.seq` を保存済みwatermarkに昇格しない。errを受けてもそれ以前を一括未保存へ戻さず、最終PubAckやflow ackと別々に残す。

独自のparser設計としては、JSONの `seq` だけを共通型へ読み込まず、まず通知種別を識別し、batch ID・stream・接続世代に結び付ける。未知の型や不正な応答は成功に丸めない。

## `count` は新たに保存されたmessage数ではない

[batching実装](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_batching.go)の `fastBatchCommit` は、stream側の `sseq` とbatch側の `lseq` を別fieldへ出す。`count` は `lseq` から作るため、gapやdedupがあると新規保存件数と一致しない。単に `count == 入力件数` を成功条件にしても取りこぼしを検出できない。

次の数値は[固定版upstream test](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_batching_test.go)のassertionを読んだ結果である。本調査ではtestを実行していない。各例は独立した空streamから始めるtest条件で、一般の並行publisherへstream位置の数値をそのまま移さない。

| 入力・操作 | 最終PubAckの期待値 | 読み取れること |
|---|---|---|
| `gap:fail` でbatch seq 1の次を3にする | `seq=1, count=1` | 保存済み1をrollbackせず残して終了。欠けた2やgapを検出した3を保存したとは扱わない |
| `gap:ok` でbatch seq 1と3、その後EOBを4で送る | `seq=2, count=3` | 保存された2messageに対してcountは3。batch内の穴は埋まらない |
| `gap:fail` でbatch seq 2のexpected-header検査を失敗させる | err通知後に `seq=1, count=1, Error=nil` | 失敗通知と、保存済みprefixを伝えるerrorなしPubAckは両立する |
| 同じ検査失敗を `gap:ok` で許容しEOBを3で送る | `seq=1, count=2, Error=nil` | 失敗位置を含むbatch進捗と保存件数は違う |
| 同じ `Nats-Msg-Id` の2messageをbatch seq 1・2で送り、EOBを3で送る | `seq=1, count=2, Error=nil` | 2回の入力が1件にdedupされる。countが2でも新規2件保存ではない |

根拠となるtest名は `TestJetStreamFastBatchPublishGapDetection`、`TestJetStreamFastBatchPublishHeaderCheckError`、`TestJetStreamFastBatchPublishDuplicatesEobCommit`。これらはR1/R3を組み合わせて期待値を定義している。

EOBは最終messageを保存せずbatchを閉じる操作で、EOB自身のbatch sequenceを成功入力件数へ足さない。また `TestJetStreamFastBatchPublishHeaderCheckErrorOnCommit` は、最後のcommit message自体が検査失敗した場合にもprefixを伝えるPubAckが来ることを期待している。最終messageの成否と「batchが終端になった」を一つのbooleanにしない。

## 10月2日のbackward-gap変更をreleaseと混同しない

`gap:ok` は、前へ飛んだ位置の欠落を許容する設定である。同一batch sequenceの再送や巻き戻しを安全なretryにする設定ではない。

- **v2.15.0**: `TestJetStreamFastBatchPublishGapOkBackwardSeq` は、1→5と進んだ後に3を送ると、backward-gap通知を出さず直接PubAckで終了する期待を持つ。handlerも前方gapの場合だけ通知している
- **10月2日の修正commit**: `b43b0d37a57113a98600225a5674a2ce97a6ebc4` は前方／後方双方へ通知条件を広げ、同じ1→5→3で `last_seq=6, seq=3` のgap通知を出してからPubAckで終了させるtestへ変える。さらにfail/ok両mode、R1/R3、重複start・append・commit等のtestを追加する

[修正commitのdiff](https://github.com/nats-io/nats-server/commit/b43b0d37a57113a98600225a5674a2ce97a6ebc4)では、後方gapはlost messagesではなく重複または順序逆転としてコメントも分けた。この変更は[ADR変更246b2f17](https://github.com/nats-io/nats-architecture-and-design/commit/246b2f17f0ad35f3c2683148831bd9ff40632e2b)から明示的に参照されている。

ここからの移行設計は独自の提案である。

1. gap通知を先に受けない限り最終PubAckを処理できないstate machineにしない。旧版では直接終端に進む場合があり、新版でも情報通知を受信できたとは限らない
2. `last_seq=6, seq=3` を「6〜2のmessage欠損」へ変換しない。特にunsigned減算で `seq-last_seq` を巨大な欠損件数にしない。前方の欠落と後方の順序異常を別に分類する
3. 同じsequenceを再送して継続しようとせず、終端情報を回収して残件を照合する。機能フラグやclient更新だけでserverの修正を取り込んだことにしない
4. 配備するbinary/tagに修正が含まれるか別途確認する。ここでは修正版release番号を推測せず、v2.15.0が新しい通知順序を保証するとも記載しない

## 完了状態を保存する設計案

以下はprotocolに追加の保証を主張するものではなく、取り込み側の独自設計である。

- **入力台帳**: batch IDだけでなく、業務message ID、payload hash、予定した最終data sequence、gap mode、EOBかdata付きcommitかを保存する
- **進捗**: 最後に送った位置、flow ackのbatch位置、個別gap/err、最終PubAckのstream位置とcount、終了理由を別々に記録する
- **`gap:fail` の終端**: 予定末尾より短いprefixで終われば部分成功として扱う。予定末尾へ到達しても、重複IDの意味や永続化設定を含めた業務要件を別に照合する
- **`gap:ok` の終端**: `count` から完全な受理集合を復元しない。取りこぼしを許せない入力ならこのmodeを避けるか、業務ID単位の照合を別に用意する。gap/err通知を全て受信したという推定だけにも依存しない
- **応答喪失**: deadlineを超えたら「未保存」ではなく結果不明にする。最後の既知prefixと入力台帳を残し、保存済みmessageとの照合または同じ業務IDによる重複対策を経て再投入を判断する

`Nats-Msg-Id` の重複排除と、fast-ingestのbatch sequence重複は別物である。前者は同じ業務入力を再保存しないための判定、後者は転送順序の異常としてbatchを終わらせ得る。batch IDだけを業務上のexactly-onceキーだと仮定しない。dedup窓の長さ・保持・ID衝突時のpayload同一性を本稿は保証していないため、結果不明のbatch全体を無条件再送する根拠にしない。

全体の完成だけをconsumerへ見せたいなら、別途version付きmanifestを使い、全chunkの業務ID・hash・総数を照合したgenerationのみ公開する設計が考えられる。fast-ingestのPubAckは、この完成manifestや下流DB transactionまで一緒に確定する機能ではない。

## flow controlと永続化の別条件

[ADRのFlow Control](https://github.com/nats-io/nats-architecture-and-design/blob/7e6e7d70ed9db187083f63437d49bcf02146957c/adr/ADR-50.md#flow-control)が扱う `msgs` はack間隔であり、毎秒の送信許可数ではない。ackを受けた個数だけで待機を減らすと、中間ack喪失から戻れない。例えばack 30を失ってもack 40が来れば古い未受信ackをまとめて処理する。ただし `gap:ok` では、そのwatermarkまでの入力全件保存を意味しない。

pingは未受信flow情報を回収する手段である。[固定版Ping test](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/jetstream_batching_test.go)とADRは、最後に送ったdataのbatch sequenceをpingに使い、ping用にsequenceを増やさない契約を示す。勝手に増やすと、送っていないdataを欠けたものとして申告してしまう。pingは新しいdataの保存や業務retryではない。

[2.15.0のPersistModeType](https://github.com/nats-io/nats-server/blob/eb763679aa3c24a40dcd3012aa046ad1996d851c/server/stream.go)は、`PersistMode: async` ではflush完了より前にpublish ackが返ることがあり、hard kill時に未flushの書込みが失われ得ると明記する。同版のconfig検査はこのmodeをFileStorageのR1に限定し、replicated streamやatomic batchとの併用を拒否する。fast-ingestは条件を満たすstreamでこのmodeと併用できるため、**errorなしPubAckを物理的な永続化完了やfsync完了へ読み替えない**。default modeについても任意のstorage破損・quorum喪失まで無損失と一般化しない。

独自の運用制約として、clientの未確認message数・byte数・保存台帳サイズに上限を置く。batch長がprotocol上boundedでないことと、アプリが無制限にメモリへ蓄えてよいことは別である。throughputだけで採否を決めず、欠落の可否、部分成功の扱い、永続化要求を先に固定する。

## 採用前の検証項目

次は追加の受入れ試験案であり、実施済みの結果ではない。

1. 初期 `seq=0` だけを返して第1messageの保存が遅れる場合に、入力を完了扱いしない
2. seq 1→3、expected-header失敗、同じmessage IDを使い、上記の `seq/count` と入力台帳を比較する
3. 最終commit messageだけを拒否し、errとprefix PubAckの両方を回収する
4. 旧版と修正commitを含む対象版で1→5→3を試し、gapの有無にかかわらず終端になることを確認する
5. `last_seq=6, seq=3` を受け、欠損範囲生成のunderflowを検出する
6. 中間ack、gap通知、最終PubAckを個別に失わせ、結果不明と確定prefixを区別する
7. pingのsequenceを正しく維持し、leader交代とflow変更時に未確認bufferが無制限に増えないかを見る
8. 永続化mode、replica数、retentionを固定し、consumerの業務完了とpublish側の終端を別々に観測する

## 出典・provenance・未確認事項

- 取得日は2026-10-05 UTC。native webでADRのmain実ページ、2.14発表記事、v2.15.0 release、v2.15.0のraw `stream.go` を開いた。設計資料・release・実装を相互照合した
- release tagはGitHubのannotated tag object `4165d0d62508a8abe99feed39bb9e8ca4216aede` を経て完全SHAへ解決した。tagと固定commitの `stream.go` は取得byte列が一致することも確認した
- ADRの固定URL、一部source/test、修正commitのnative web取得はcache missになった。代わりにGitHub RESTと固定commitのraw UTF-8を取得し全文／差分を読んだ。失敗したweb取得だけを根拠にしていない
- 両repositoryの該当commitのLICENSEはApache-2.0、serverの対象file headerも同ライセンス。2.14発表記事とrelease本文の独立した再配布ライセンスは未確認のためunknown。コード・testの転載はせず、独自要約と出典へのリンクだけを記載した
- upstream testは読解のみで、NATS serverを起動したfault testは実行していない。ローカルで実施するKB検索evalは文書取得を検証するもので、broker動作の再現試験ではない
- 10月2日のfixを含む正式release、client/Orbit別の対応版、独自clientの相互運用、実負荷でのthroughput、物理storage障害後の無損失性、業務IDのreconciliation実装は未確認

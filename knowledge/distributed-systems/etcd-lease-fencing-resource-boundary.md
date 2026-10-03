---
{
  "id": "distributed-systems-etcd-lease-fencing-resource-boundary",
  "title": "etcd lease lock: IsOwner transaction と外部資源 fencing の境界",
  "kind": "knowledge",
  "technology": "distributed-systems",
  "version": "etcd v3.7 docs; Go client concurrency v3.7.2 (published 2026-09-22) @ 68c065e562994b89e333e77b039ad066f933c586; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:api-distributed",
    "distributed-systems",
    "etcd",
    "lease",
    "fencing",
    "IsOwner",
    "transaction",
    "stale-holder",
    "session",
    "snapshot-restore"
  ],
  "sources": [
    {
      "id": "etcd-37-lock-lease-fencing-20261003",
      "url": "https://etcd.io/docs/v3.7/learning/why/",
      "type": "official_docs"
    },
    {
      "id": "etcd-37-concurrency-contract-20261003",
      "url": "https://etcd.io/docs/v3.7/dev-guide/api_concurrency_reference_v3/",
      "type": "official_docs"
    },
    {
      "id": "etcd-37-api-guarantees-20261003",
      "url": "https://etcd.io/docs/v3.7/learning/api_guarantees/",
      "type": "official_docs"
    },
    {
      "id": "etcd-37-recovery-revision-20261003",
      "url": "https://etcd.io/docs/v3.7/op-guide/recovery/",
      "type": "official_docs"
    },
    {
      "id": "etcd-372-mutex-owner-20261003",
      "url": "https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/client/v3/concurrency/mutex.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "etcd-372-session-lifecycle-20261003",
      "url": "https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/client/v3/concurrency/session.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "etcd-372-release-20261003",
      "url": "https://github.com/etcd-io/etcd/releases/tag/v3.7.2",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# etcd lease lock: IsOwner transaction と外部資源 fencing の境界

## 問いと採用判断

lease付きlockを取得したworkerが長く停止し、別workerに所有権が移ったあとで再開したとき、古い書き込みをどこで拒否するべきか。**保護対象がetcd内なら所有権条件と更新を同じTxnへ入れ、etcd外なら資源自身に世代検証を実装する**。TTL・keepalive・直前の所有確認だけをデータ整合性の根拠にしない。

本稿は新機能紹介ではなく、未収録だった失効後の所有者（stale holder）の書き込み防止を扱う。[既存のRaft文書](raft-leader-election-log-replication.md)は複製合意とread経路、[outbox文書](../messaging/transactional-outbox.md)はDB更新とイベント生成の結合が対象。本稿の焦点は、合意で選んだworkerと実際に副作用を受理する資源の間に残る隙間である。

## 確認した公式契約

### leaseが切れても、旧workerは停止したとは限らない

[lock and leaseの注意](https://etcd.io/docs/v3.7/learning/why/#notes-on-the-usage-of-lock-and-lease)は、serverがleaseを取り消したのにclientがまだ所有していると認識する状況を明記する。leaseだけでは相互排他を保証せず、資源のversion validationと組み合わせる必要がある。etcd外の資源を守る場合、その資源のreplicaを含めて整合した検証が必要になる。

これから導く設計判断は、GC停止・実行休止・分断を「旧workerが永久に消えた」とみなさないこと。停止していたworkerが再開する場合に加え、既に送信した要求が遅れて届く場合も扱う。TTLを延ばす判断は再取得頻度・障害回復時間の調整であり、資源側の検証を省く証明にはならない。

### lock所有を表すkeyと、保護対象の更新を結び付ける

[v3.7 concurrency API](https://etcd.io/docs/v3.7/dev-guide/api_concurrency_reference_v3/)のLockは、所有期間に存在するunique keyを返す。このkeyをtransactionと組み合わせ、所有権を持つときだけetcdを更新できる。Unlock、leaseの期限切れ・revokeで所有は解放される。ownership keyを利用者が直接変更すると動作未定義になり得る。

同じ名前に対する同一leaseのLockは単一取得として扱われる。したがって、独立したworkerが同じsessionを共有し、各workerの取得成功を相互排他の証拠にするのは避ける。保護したい競合単位とsessionの共有範囲を対応させる。

### Watch・timeoutは現在の所有権を確定しない

[API guarantees](https://etcd.io/docs/v3.7/learning/api_guarantees/)はKV操作のatomicity・strict serializabilityを示す一方、Watchにlinearizabilityを保証せず、通知遅延に上限を設けない。失効通知が未到着であることを、有効なleaseの証明にしない。RPCのtimeoutや通信途絶では、操作の完了状態がclientから不確定になる場合もある。エラー受信だけで「更新されていない」と判断して無条件にやり直さない。

## 固定実装で確認したIsOwnerとsessionの境界

対象は[etcd v3.7.2](https://github.com/etcd-io/etcd/releases/tag/v3.7.2)のGo clientで、commit `68c065e562994b89e333e77b039ad066f933c586`へ固定した。以下は実装読解の観察であり、全言語clientの共通仕様や今回の実行試験結果ではない。

[mutex.go](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/client/v3/concurrency/mutex.go)では次を確認した。

- `tryAcquire`はprefixとsessionのlease IDからkeyを作り、存在しなければlease付きで作成する。既存なら再利用し、そのkeyのcreate revisionを`myRev`に保持する
- `Lock`は古いcreate revisionを持つ待機keyの削除を待ち、その後にも自分のkeyの存在を確認する。待機中にsessionが失効してkeyを失うと`ErrSessionExpired`となる経路がある
- `IsOwner()`は`CreateRevision(myKey) = myRev`という比較式を返す。呼ぶだけでserverへ問い合わせたり、以後の操作を保護したりするものではない
- `Header()`は取得時のresponse headerを返すAPIで、内部の所有判定は別に保持したkeyのcreate revisionを使う。任意の最新revisionやlease IDを所有世代と同一視しない

独自の利用方針は、**Lock/TryLockが成功してから**、`Txn.If(m.IsOwner()).Then(保護対象の更新).Commit()`という一つのetcd transactionへ条件を入れること。待機keyの存在だけを確認するIsOwnerを、取得成功前に「自分が先頭の所有者である」判定として使わない。RPC errorと、応答の`Succeeded`がfalseで条件不成立だった結果も別に処理する。先に空のTxnでIsOwnerを確認し、その後に別Putを送る設計には、再びcheck-then-writeの競合が残る。

[session.go](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/client/v3/concurrency/session.go)の`Done`/`Ctx`は、leaseがorphaned・expired、またはrefreshされなくなったときの通知を提供する。`Orphan`はrefreshを止め、`Close`はその後leaseのrevokeを試みる。`WithContext`のcontextがClose完了前に取り消されると、revokeせず期限切れに委ねる場合がある。

従って`Done`を監視して仕事を止めるのは有用だが、それだけでは送信済み要求を取り消せない。また、lock取得用contextのtimeoutと、成功後のsession lease寿命は同一ではない。終了通知、revoke成功、外部副作用の終了を別々に確認する。

## 外部資源へ適用するfencingの設計案

以下は公式の「資源側のversion validationが必要」という条件を具体化する本書の設計案で、etcdが外部DBやHTTP APIに自動実装する機能ではない。

### 受理境界に単調な世代を置く

1. lock取得が成功した世代を識別するfencing tokenを決める。etcdを使うなら取得したownership keyのcreate revisionを候補にする。単なるlease IDの数値比較、ローカル時計、処理直前に読んだ無関係な最新revisionへ置き換えない
2. 対象資源ごとに、受理済みの最大世代（high-water mark）を永続化する。tokenの検査・世代の更新・業務データの変更を、同じ直列化可能な受理処理として確定する。検査後に別APIで更新する構成は避ける
3. high-water markより古い世代の要求を拒否する。新しい所有者が業務開始の境界を明確にしたい場合、先に対象資源へ新世代を登録し、成功応答を受けてから処理を始める
4. 受理するすべての書き込み経路、replica、管理用更新を同じ条件に従わせる。無検証の迂回経路があるなら、その資源はまだfencingで保護されていない

tokenは認証情報ではない。信頼できる発行・検証経路を用意し、clientが好きな巨大整数を送れば新所有者になれる設計にしない。保護対象のAPIが条件付き更新を提供しない場合は、worker側の期限確認を足して安全と称さず、資源側の更新経路を変更する必要性を明記する。

### fencingが保証しないこと

独自の反例として、Aの世代が41、Bが42を取得した状況を考える。資源が42を受理した後で届く41は拒否できる。しかし、資源がまだ42を知らない間に届く41まで、high-water markだけで拒否できるわけではない。**lease失効の瞬間に旧要求を一律遮断する仕組みではなく、資源が受理した所有世代の順序を守る仕組み**として仕様化する。

同じ世代42で同じ業務要求を2回送れば、世代検証だけでは重複副作用を防げない。同一世代の複数操作を許すか、一操作だけかを決め、必要に応じてoperation IDのdeduplicationや業務versionのcompare-and-swapを追加する。fencing、冪等性、業務操作の順序保証を一つの機能として扱わない。応答喪失後の再試行にも同じ区別が必要になる。

## snapshot restoreで世代の前提を再確認する

[Disaster recovery](https://etcd.io/docs/v3.7/op-guide/recovery/#revision-difference)は、古いsnapshotへ戻すとclientから見たrevisionが後退し得ること、restoreが新しいlogical clusterを作ることを説明する。`--bump-revision`はsnapshotのrevisionへ指定値を加え、`--mark-compacted`は主にWatch利用者の古いcacheを無効化するために説明される。

この事実からの設計上の帰結は、revision由来tokenの単調性をrestore・cluster再作成を越えて無条件に仮定しないこと。外部資源に高い世代が残ると新workerが拒否され続けるし、外部資源側も古いhigh-water markへ戻せば旧要求を受理する危険がある。復旧時は旧writerを遮断し、資源側の世代・発行側の世代・バックアップ時点を照合してから再開する。

必要なら別に管理するepochをtokenへ含める設計を検討するが、cluster IDやランダムUUIDを辞書順で比較すれば安全という意味ではない。epoch切替自体の権限、順序、永続化、旧epoch拒否を受理側で成立させる。bump-revisionの存在だけでは外部資源の復旧手順まで検証済みにならない。

## 導入前の境界試験と未確認事項

以下は受入試験案であり、今回実行したテストではない。

- A取得後にAを停止し、lease失効・B取得・B更新を進めてからAを再開する。Aの無条件Putとの違いを確認し、所有条件付きTxnではAの更新が成立しないことを検査する
- 同一session・同一prefixの2workerを試し、独立した排他単位にはならないことを確かめる。異なるsessionとの結果を比較する
- IsOwnerの事前確認直後に失効させる。別Putが危険な対照系と、同じTxnに条件を入れた系を分ける
- 外部資源へ42を登録してから41を遅延配送する。さらに42未登録時の41、同じ42の重複operation ID、別replicaへの配送を試す
- keepalive停止、Closeのcontext取消、更新commit後の応答喪失を別々に起こし、停止通知と更新結果を取り違えない
- 古いsnapshotからの復旧で、外部high-water mark、epoch、旧writer遮断、cache再構築が揃うまで更新を再開しない

固定commitの[integration test](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/tests/integration/clientv3/concurrency/mutex_test.go)には待機sessionをCloseしてから先行lockを解放する試験があるが、本調査ではテスト実行・実cluster・pause/partition注入を行っていない。全client言語、外部DB固有のisolation/replication、throughput、運用TTLの適正値、restore中の全障害組合せは未検証。検索evalは発見性の検証に限る。

## 出典・版・ライセンス

- 公開Webのv3.7 concurrency、API guarantees、lock/lease注意、recovery、およびv3.7.2 releaseの実ページを2026-10-03 UTCに開いた。release APIでは公開日時2026-09-22T21:22:39Zを確認し、annotated tagをdereferenceして40桁commitを確定した
- v3.7文書の表示更新日は順に2025-06-03、2026-05-05、2026-07-10、2026-04-03。ページ更新日をfencingやIsOwnerの導入日とは扱わない。v3.7.2で新たにこの保証が導入されたとは主張しない
- etcd Authorsのwebsite文書は[固定LICENSE](https://github.com/etcd-io/website/blob/d20f8ae346da60ab6eb6f24f200753b328aa9fcc/LICENSE)により、例外を除く文書CC-BY-4.0、code/sample Apache-2.0と確認した。上記は原文の転載ではなく独自の日本語要約。実装は同commitの[LICENSE](https://github.com/etcd-io/etcd/blob/68c065e562994b89e333e77b039ad066f933c586/LICENSE)と各file headerでApache-2.0を確認。release本文の個別ライセンスはunknownとした
- releaseがリンクするCHANGELOG-3.7.mdとcontribの候補fileは固定tag/commitで取得できなかったため、変更履歴やexample実装の根拠にはしない。mutex/session/test本体はGitHub connectorで固定commitを取得した。コード転載・module昇格は行わない
- source recordは新規追加し、共有recordの取得日を延長していない。release_notesの30日TTLに合わせ、明示期限を2026-11-02とした

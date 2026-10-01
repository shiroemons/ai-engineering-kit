---
{
  "id": "redis-client-side-cache-invalidation-disconnect",
  "title": "Redis client-side caching: invalidation・NOLOOP・切断時のローカルキャッシュ保護",
  "kind": "knowledge",
  "technology": "redis",
  "version": "redis.io current unversioned docs（2026-10-01確認）; CLIENT TRACKING / CLIENT CACHING は Redis Open Source 6.0.0から、個別server/client版の実動作は未検証",
  "tags": [
    "research-domain:data",
    "client-side-caching",
    "client-tracking",
    "invalidation",
    "resp3",
    "noloop",
    "disconnect"
  ],
  "sources": [
    {
      "id": "redis-client-cache-reference-20261001",
      "url": "https://redis.io/docs/latest/develop/reference/client-side-caching/",
      "type": "official_docs"
    },
    {
      "id": "redis-client-tracking-command-20261001",
      "url": "https://redis.io/docs/latest/commands/client-tracking/",
      "type": "official_docs"
    },
    {
      "id": "redis-client-caching-command-20261001",
      "url": "https://redis.io/docs/latest/commands/client-caching/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-12-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# Redis client-side caching の invalidation と切断

## 問いと適用範囲

アプリ内のローカルキャッシュを Redis の tracking で無効化するとき、GET の返答が遅れたり通知接続が切れたりしても、古い値をキャッシュへ戻さないために何が必要か。サーバー側の maxmemory eviction の選択や RDB/AOF durability とは別の問いである。

2026-10-01 に現行の公式参照ページと二つの command reference を読み合わせた。`CLIENT TRACKING` / `CLIENT CACHING` は6.0.0からあるが、ページは固定版ではない。以下を「Redis 6.0.0の全patch・全clientで検証済み」とは扱わない。新規リリースの紹介ではなく、現在の文書にある注意点と不整合を含む適用確認である。

## 現行文書の契約

### tracking は connection と失効通知の仕組み

`CLIENT TRACKING` はその接続で有効になり、OFF または切断まで続く。通常 mode は read したキーを追跡する。同じ接続で返答と通知を扱えるのは RESP3。RESP2 は別の Pub/Sub 接続への `REDIRECT` を使う。RESP3 でも別接続への redirect は可能で、redirect 先が切れると tracking 元に `tracking-redir-broken` push が届く。[CLIENT TRACKING](https://redis.io/docs/latest/commands/client-tracking/)

tracking は Redis がアプリのキャッシュメモリを書き換える機能ではない。変更・期限切れ・サーバー eviction 等に対する通知を受けて、client が対応エントリを失効させる。二接続構成では通知が先、古い GET 返答が後という順序がありうる。公式 reference は読み込み中の印を先に作り、通知で印が消えた後には古い返答を格納しない方法を示す。単一接続ではその接続上の通知/返答の順序が分かる。[Client-side caching reference: implementation / race conditions](https://redis.io/docs/latest/develop/reference/client-side-caching/#avoiding-race-conditions)

### NOLOOP は「書いた値を永久に追跡」の意味ではない

`NOLOOP` は当該接続自身の変更に対する通知を抑制する。現在の command reference は、通常 tracking mode では変更時に追跡表からキーが外れ、NOLOOP で通知を省略してもその点は変わらないと明記する。将来の変更通知を受けるには、そのキーを再び read して追跡し直す。BCAST は prefix に対する通知で、read 済みキー一覧を保持する通常 mode と区別する。[CLIENT TRACKING: NOLOOP / BCAST](https://redis.io/docs/latest/commands/client-tracking/#optional-arguments)

### 切断・全消去を個別キー通知と分ける

通知用接続を失ったら local cache を全消去する。RESP2 / RESP3 のどちらでも通知経路へ定期的に PING し、応答が所定時間内に来なければ接続を閉じて cache を消去する、という対策が公式 reference にある。FLUSHDB/FLUSHALL の通知に出る `null` は、キー名の文字列として処理しない。[Client-side caching reference: losing connection / two connections](https://redis.io/docs/latest/develop/reference/client-side-caching/#what-to-do-when-losing-connection-with-the-server)

### OPTIN / OPTOUT の指示は次のcommandに作用する

`CLIENT CACHING YES` は OPTIN で、`CLIENT CACHING NO` は OPTOUT で、同じ接続の直後の command のキー追跡を制御する。キー名に永続的な cache 許可を付ける命令ではない。[CLIENT CACHING](https://redis.io/docs/latest/commands/client-caching/)

取得時点の reference の Opt-out 節は `CLIENT UNTRACKING key` と記載していたが、[CLIENT TRACKING の OPTOUT](https://redis.io/docs/latest/commands/client-tracking/#optional-arguments) と CLIENT CACHING の説明は上記の NO で一致している。UNTRACKING の専用公式ページは取得できず、存在・導入版を裏付けられなかった。その命令を推奨手順に採用せず、使用版で command/API を検証する事項として残す。これは不一致の観察であって、あらゆる版で命令が存在しないという断定ではない。

## 安全なclient設計（独自の提案）

まず、その言語clientが tracking と失効処理をどこまで実装するかを確認する。サーバーが RESP3 対応というだけで、アプリ側のキャッシュまで安全に更新されるとは判断しない。

- 読み込み開始時にキーごとの世代と通知接続の世代を控える。返答到着時にどちらかが変わっていたら格納しない。これは公式の読み込み中の印の考え方を、同時に複数の GET が走る実装へ拡張する案である
- cache を消す処理と格納可否の判定を、アプリ内でも競合しないよう同期する。単一接続の wire 順序が分かっても、別workerへ渡した callback の実行順まで自動では保証されない
- 切断を検出したら cache hit と格納を止め、古い世代の in-flight 返答を捨てる。再接続後は tracking/REDIRECT の再設定を確認してから、空の cache で運用を再開する
- pool では `CLIENT CACHING` と対象 read が同じ接続で連続して送られることを保証する。間に他リクエストの command が入る構成を避ける
- NOLOOP の適用範囲はアプリ全体ではなく接続である。write の結果をローカルに保持する設計では、将来の invalidation を受ける追跡が残るかまで確認する。最初の実装では独自の write-through 最適化を足す前に read/invalidate 経路を検証する
- 最大保存時間と容量上限を決める。TTL は通知経路が壊れたときの追加防護であり、検出した切断後も TTL まで古い値を使う理由にはしない
- 更新直後に絶対に古い値を許せない判定には、非同期通知だけを整合性の根拠にしない。業務上許される stale 時間を決められないなら、その読取りをローカルキャッシュの対象から外す案を検討する

この方針は強整合性・線形化可能性や可用性の保証ではない。ネットワーク断の検出には時間がかかり、要求に合う停止・fallback 方針はアプリ側で決める。

## 本番前の試験案（今回未実行）

1. 通常の変更通知、expire、サーバー eviction と FLUSH の全消去を分けて試す。通知回数とDB更新回数が一致するという前提を置かない
2. 二接続で GET 応答だけを遅らせ、先に invalidation を処理させる。古い返答が cache へ復活しないことを確認する
3. 通知用接続のみ切断し、data 接続が生きている場合も試す。PING timeout 後の全消去と、再接続前の返答が後から来る経路を確認する
4. 通常 mode + NOLOOP で自分の write 後、別clientから同じキーを更新する。再readによる追跡再開の有無を比較し、古い write-through 値を保持しないことを確認する
5. pool の高並行負荷で OPTIN/OPTOUT の直後commandが入れ替わらないかを確認する
6. cache 容量・hit率・invalidation処理待ち・通知接続の最終正常時刻・全消去回数を測り、性能向上と stale リスクを別々に判断する

## 版・出典・残る不確実性

Redis Cluster の node 間切替、Sentinel failover、proxy、client別の自動再接続と読み込み中制御は未確認。具体的な Redis patch 版や client library を起動した試験も未実施。ページの公開日と各注意書きの追加日は表示されず、「最近仕様が変わった」とは断定しない。

Redis Website and Documentation の [LICENSE](https://github.com/redis/docs/blob/main/LICENSE) は CC BY-NC-SA 4.0 を掲げ、旧 redis-doc 由来の一部には CC BY-SA 4.0 の扱いがある。当該部分ごとの例外適用は未特定のため、文書の文章・図・コードは転載せず、確認した動作の独自要約に限定する。既存の別source recordのライセンス表記は変更しない。公式文書 TTL 90日により2026-12-30に再確認する。

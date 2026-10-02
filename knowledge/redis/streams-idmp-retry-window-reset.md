---
{
  "id": "redis-streams-idmp-retry-window-reset",
  "title": "Redis Streams IDMP: 再送の重複抑止窓と XCFGSET による追跡消去の境界",
  "kind": "knowledge",
  "technology": "redis",
  "version": "IDMP / IDMPAUTO / XCFGSET は Redis Open Source 8.6.0 導入（8.6 公開 2026-02-10）; 2026-10-02 の unversioned command/reference を照合、個別patch・client実行は未検証",
  "tags": ["research-domain:data", "streams", "idmp", "idmpauto", "xcfgset", "deduplication", "retry", "producer"],
  "sources": [
    {"id": "redis-streams-idmp-reference-20261002", "url": "https://redis.io/docs/latest/develop/data-types/streams/idempotency/", "type": "official_docs"},
    {"id": "redis-xadd-idmp-command-20261002", "url": "https://redis.io/docs/latest/commands/xadd/", "type": "official_docs"},
    {"id": "redis-xcfgset-command-20261002", "url": "https://redis.io/docs/latest/commands/xcfgset/", "type": "official_docs"},
    {"id": "redis-xinfo-stream-idmp-20261002", "url": "https://redis.io/docs/latest/commands/xinfo-stream/", "type": "official_docs"},
    {"id": "redis-86-streams-announcement-20261002", "url": "https://redis.io/blog/announcing-redis-86-performance-improvements-streams/", "type": "release_notes"},
    {"id": "redis-xreadgroup-pending-boundary-20261002", "url": "https://redis.io/docs/latest/commands/xreadgroup/", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# Redis Streams IDMP の再送窓と設定変更

## 問いと適用範囲

`XADD` の送信後に応答が消失し、producer が「未送信」として再試行するとき、stream への二重追加をどう防ぐか。Redis 8.6 の IDMP はこの境界を扱う。2026-02-10 の [8.6 公開記事](https://redis.io/blog/announcing-redis-86-performance-improvements-streams/) と、2026-10-02 時点の公式 command/reference を読み合わせた。これは最新版の推奨や、8.6.0 初版へのダウングレード推奨ではない。

結論は、同じ論理イベントの識別子を再送時にも保持し、時間と件数の両方で追跡が残る範囲を設計すること。設定変更は重複抑止状態を失う操作として扱う。producer の追加抑止、consumer の処理完了、障害時のデータ保存は、それぞれ別の確認が必要である。

## 公式文書で確認した契約

### 1. stream entry ID と idempotent ID を混同しない

`XADD` の `IDMP producer-id idempotent-id` と `IDMPAUTO producer-id` は 8.6.0 で導入された。両方とも entry ID に自動生成の `*` を指定する必要がある。アプリが先に決める iid と、追加後に返る stream entry ID は別物である。追跡中の同じ `(pid, iid)` を再送すると、新たな entry を作らず元の entry ID を返す。[XADD: Optional arguments](https://redis.io/docs/latest/commands/xadd/#optional-arguments)

manual IDMP では内容の等価性を iid の代わりに照合しない。公式例でも同じ pid/iid の payload を変えて送ると、元の ID が返る。したがって成功応答を「今回の変更内容が反映された」と読んではいけない。これは更新 API ではない。[XADD: Idempotent message processing examples](https://redis.io/docs/latest/commands/xadd/#idempotent-message-processing-examples)

### 2. IDMPAUTO の同内容判定と業務上の同一イベントは違う

manual IDMP は producer が iid を与える。IDMPAUTO は field-value の内容から Redis が iid を計算する。どちらも producer の再起動後に同じ pid を使う必要があり、manual mode では同じイベントに同じ iid を再利用する。iid は pid 内で一意ならよく、別 pid には独立した追跡がある。[Idempotent message processing: modes / producer isolation](https://redis.io/docs/latest/develop/data-types/streams/idempotency/)

同じ値のセンサー観測が連続した、同額の注文が二件あった、といった場合には内容が等しくても別イベントである。開発元も、別メッセージの内容が一致しうるなら IDMPAUTO を避け manual IDMP を使うよう説明している。「同じ内容は一回だけ」という要件がないのに自動モードを選ばない。[8.6 公開記事: Introducing idempotent IDs](https://redis.io/blog/announcing-redis-86-performance-improvements-streams/#introducing-idempotent-ids)

### 3. 再送窓は DURATION と MAXSIZE の早い方で切れる

`XCFGSET` の `IDMP-DURATION` は 1–86,400 秒、既定 100 秒。`IDMP-MAXSIZE` は producer ごとの 1–10,000 iid、既定 100 件。capacity に達すると、その pid の古い iid は残り時間に関係なく失われる。DURATION だけを一日にしても、一日の再送を保証したことにはならない。設定対象の stream は既に存在する必要がある。[XCFGSET: arguments](https://redis.io/docs/latest/commands/xcfgset/#optional-arguments)

公式解説は、クラッシュ復旧から再送までの時間に DURATION を合わせ、応答受信から送信済み記録までの遅延である mark-delay と producer ごとの送信率に余裕を足して MAXSIZE を見積もる考え方を示す。[Idempotent message processing: Determine optimal configuration values](https://redis.io/docs/latest/develop/data-types/streams/idempotency/#determine-optimal-configuration-values)

### 4. XCFGSET を通常の起動処理に無条件で入れない

値を変える XCFGSET は stream 内の全 producer の IDMP map を消去する。保持期間を延ばす変更でも、既存の追跡情報をそのまま長寿命化する操作とは扱えない。[Idempotent message processing: Persistence](https://redis.io/docs/latest/develop/data-types/streams/idempotency/#persistence)

取得時点では文書間に粒度の差があった。解説と [8.6 公開記事](https://redis.io/blog/announcing-redis-86-performance-improvements-streams/) は「現在と異なる値を設定したとき」と限定するが、[XCFGSET: Behavior](https://redis.io/docs/latest/commands/xcfgset/#behavior) は呼び出し時に既存 map を消去すると記述する。同じ値の再設定が無害かはこの読み合わせだけで断定しない。実装と対象patchで確認するまで、接続し直すたびに設定を再適用する手順を採用しない。

### 5. 永続化される追跡と end-to-end exactly-once は別

解説は RDB/AOF に pid/iid と stream の IDMP 設定を保存し、再起動後も追跡が続くと説明している。これは保存された状態に関する契約であり、あらゆるクラッシュや failover で直前の書き込みが失われないという検証結果ではない。[Idempotent message processing: Persistence](https://redis.io/docs/latest/develop/data-types/streams/idempotency/#persistence)

consumer 側では `XREADGROUP` により渡された未確認メッセージが PEL に残り、履歴読み込みや claim により再び取得されうる。`XACK` は PEL から外す操作である。外部DBへの反映完了後、XACK 前に停止する境界は producer の IDMP では消えない。したがって「stream に一度追加」と「外部の副作用が一度だけ成功」を同じ保証にしない。[XREADGROUP: Differences / Usage example](https://redis.io/docs/latest/commands/xreadgroup/)

### 6. XINFO の観測対象を分ける

8.6.0 から `XINFO STREAM` に IDMP 用の情報が加わった。`idmp-duration` / `idmp-maxsize` は設定、`pids-tracked` / `iids-tracked` は現時点の追跡数、`iids-added` / `iids-duplicates` は追加・重複抑止に関する累積値である。MAXSIZE は pid ごとの制限で、iids-tracked は全 producer の合計なので、両者をそのまま比較して「上限超過」とは判定しない。[XINFO STREAM: IDMP fields / History](https://redis.io/docs/latest/commands/xinfo-stream/)

## 採用・運用の設計案（独自の提案）

以下は上記の契約から導いた設計判断であり、Redis が提供する追加保証ではない。

1. 送信元の durable outbox 等に stream key、安定した pid、iid、payload、送信状態を一緒に記録する。同じイベントの retry はその組を再利用する。再起動ごとのランダム pid や、試行ごとの新しい iid は重複抑止を回避してしまう。別イベントの iid 使い回しも防ぐ
2. 内容を後から変える必要があるなら、既存 iid の retry に混ぜず、新しい業務イベントとして識別する。manual IDMP の同一キー・異なる payload をアプリ側で検出し、黙って成功扱いしない
3. 最初の追加から最後に再送しうるまでの経過時間に、停止、復旧、backoff、再送待ちを含める。MAXSIZE は、最も古い未確定イベントを再送するまでに同じ pid で追加される後続 iid の最大数を収める。平均送信率だけでなく burst、pipelining、応答消失中にも後続送信を続ける場合を含める
4. 例として同じ pid が毎秒 200 件を送り続け、最古の未確定イベントの retry が 60 秒後なら、後続だけで約 12,000 件になる。この仮定では MAXSIZE の上限 10,000 件では足りない。数値を切り詰めて保証ありとせず、未確定の間は新規送信を止める、in-flight を制限する、または別の durable な重複排除を併用する
5. XCFGSET の値変更前には producer の新規送信を止め、未確定イベントを照合・解決する。単にプロセスを停止しただけでは、すでに Redis が受け付けた未確認イベントが残りうる。解決できないなら、変更後の再送が重複しうるものとして下流で照合できるまで切り替えない。新しい pid へ変更するだけでは解決しない
6. `iids-duplicates` の増分を producer retry 数と突き合わせる。抑止数がゼロでも、retry がなかったのか、追跡期間を越えたのか、pid/iid が変わったのかは単独では分からない。設定変更の記録、最古未確定イベントの年齢、同じ pid の後続送信数も見る
7. consumer の副作用にもイベント識別子を渡し、外部DBなら unique 制約と処理結果記録を同じ transaction に置く等、対象に合わせた再実行対策を設ける。IDMP の期限を越す手動 replay も、この層の照合対象にする

## 導入前の境界テスト案（未実行）

隔離した stream を使い、応答IDだけでなく entry数、payload、追跡数、下流の副作用数を確認する。本調査で server/client の実行はしていない。

- 応答消失を模擬して同じ pid/iid を再送し、追跡が有効な間は元のIDと一件の entry になること
- 同じ pid/iid に別 payload を与え、更新されないことを検出できること。同じ内容の別イベントには異なる iid を付け、二件とも追加されること
- IDMPAUTO では同じ内容を再送した場合の抑止を確認する。field 順序やシリアライズの違いまで同一扱いされるとは仮定しない
- DURATION 経過と MAXSIZE 超過を別々に起こし、保持窓の外では抑止を期待できないこと。両方の境界で再送時刻・件数を記録する
- 値変更 XCFGSET と同値 XCFGSET を別ケースにする。変更は追跡消去を確認し、同値のケースは対象patch固有の観察として記録する
- producer 再起動後も識別子が変わらないこと。Redis の復旧では実際の RDB/AOF 設定と復元点を記録し、保存されていた追跡の回復を確認する
- consumer の副作用完了直後・XACK 前に停止し、再取得後の副作用数を検査する。producer の entry数だけを合格基準にしない

## 限界・未確認事項と出典の扱い

- command/reference は版固定URLではない。8.6.0 の導入境界は確認したが、全patch、互換サービス、Redis Cloud/Software、個別clientの対応版を検証したものではない。XREADGROUP の後発オプションも8.6に逆適用しない
- XCFGSET の同値再設定の実装差、IDMPAUTO の field 順序・エンコード・衝突条件、entry 削除や trim と追跡の関係、key の削除・再作成・移動、backup restore、failover の各挙動は未検証である
- 一日より長い再送や、producer ごとに10,000件を超す未確定範囲を、この機能単独で保証できるとは扱わない。多数の pid の総メモリ費用も本調査では測定していない
- 性能数値を計測していないため、公式解説の小さな overhead 値を本番容量の保証として転記しない。検索evalは本書の発見可能性を検証するもので、Redis の障害試験ではない
- 文書のライセンスは [redis/docs LICENSE](https://github.com/redis/docs/blob/main/LICENSE) の CC-BY-NC-SA-4.0 と旧文書の部分的 CC-BY-SA-4.0 例外を確認した。例外の当該箇所への適用は未特定。公開記事単体のライセンスは未特定と記録した。本文は独自要約と設計案で、公式コード・図・長文の転載や module への移植はしていない

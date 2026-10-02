---
{
  "id": "redis-himport-session-safe-bulk-replacement",
  "title": "Redis 8.10 HIMPORT: 接続ローカル fieldset と全置換を踏まえた安全な bulk import",
  "kind": "knowledge",
  "technology": "redis",
  "version": "HIMPORT / compact hashes は Redis Open Source 8.10.0 導入（GA 2026-07-29）; 8.10.2 release（2026-09-17）と2026-10-02のunversioned command/referenceを照合、server/client実行は未検証",
  "tags": [
    "research-domain:data",
    "himport",
    "compact-hash",
    "fieldset",
    "bulk-import",
    "connection-pool",
    "retry",
    "migration"
  ],
  "sources": [
    {
      "id": "redis-himport-prepare-contract-20261002",
      "url": "https://redis.io/docs/latest/commands/himport-prepare/",
      "type": "official_docs"
    },
    {
      "id": "redis-himport-set-replacement-20261002",
      "url": "https://redis.io/docs/latest/commands/himport-set/",
      "type": "official_docs"
    },
    {
      "id": "redis-himport-discard-lifecycle-20261002",
      "url": "https://redis.io/docs/latest/commands/himport-discard/",
      "type": "official_docs"
    },
    {
      "id": "redis-hset-partial-update-reference-20261002",
      "url": "https://redis.io/docs/latest/commands/hset/",
      "type": "official_docs"
    },
    {
      "id": "redis-compact-hash-suitability-20261002",
      "url": "https://redis.io/docs/latest/develop/data-types/hashes/",
      "type": "official_docs"
    },
    {
      "id": "redis-himport-client-pool-article-20261002",
      "url": "https://redis.io/blog/efficient-bulk-hash-insertion-with-redis-810s-himport/",
      "type": "maintainer_article"
    },
    {
      "id": "redis-himport-ga-8100-release-20261002",
      "url": "https://github.com/redis/redis/releases/tag/8.10.0",
      "type": "release_notes"
    },
    {
      "id": "redis-himport-patch-8102-release-20261002",
      "url": "https://github.com/redis/redis/releases/tag/8.10.2",
      "type": "release_notes"
    },
    {
      "id": "redis-compact-hash-announcement-20261002",
      "url": "https://redis.io/blog/announcing-redis-810-compact-hash-jsonpath-extensions-performance-improvements-and-more/",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# HIMPORT を bulk import に採用するときの境界

## 問いと結論

同じ schema の大量の hash を Redis に取り込むとき、HSET を HIMPORT に置き換えるだけで安全か。結論は、完全なレコードを再構築する import 用途に限定し、物理接続と入力世代を固定すること。field の部分更新、接続poolの自由な乗り換え、応答消失後の無条件retryを同じ仕組みに含めない。

既存資料の Streams IDMP はイベント追加の重複抑止、本書は hash 全置換と接続状態の整合性を扱う。compact hash は記憶形式の最適化であり、import 全体の整合性を肩代わりする機能ではない。

## 版と日付を区別する

- [8.10.0 GA release](https://github.com/redis/redis/releases/tag/8.10.0) は2026-07-29公開で、HIMPORT と compact hashes を列挙する。[9月14日の告知記事](https://redis.io/blog/announcing-redis-810-compact-hash-jsonpath-extensions-performance-improvements-and-more/)の公開日を初回提供日として使わない
- [8.10.2 release](https://github.com/redis/redis/releases/tag/8.10.2) は2026-09-17公開、SECURITY区分の修正を含む。機能の導入版8.10.0と、導入先に選ぶpatchを分ける。本書は8.10.0初版を本番向けに推奨するものではない
- 以下のcommand/referenceは2026-10-02 UTCに取得した版固定でない文書である。各patchでの実測一致、互換サービス、各client版の対応は別途確認する。PREPARE/SETの取得時の対応表では Redis Software / Redis Cloud の Standard と Active-Active は未対応表示だったため、Open Sourceの導入版だけでmanaged環境の利用可否を決めない

## 確認した契約

### 1. fieldset は順序も含む接続状態

[HIMPORT PREPARE](https://redis.io/docs/latest/commands/himport-prepare/) はfield名の順序付き定義を現在の接続に登録する。同名の登録は前の定義を置換し、重複field名はエラーになる。切断または RESET で定義は失われ、他の接続からは見えない。

取得時の機械向けdescriptionには sorted set という表現があるが、本文と SET は宣言順の対応を明記する。呼び出し側はfieldだけを独立にsortせず、宣言した配列とvaluesを一組として保持する。内部の並べ替え方式は本書では推測しない。

### 2. SET の対象は指定fieldだけでなく key 全体

[HIMPORT SET](https://redis.io/docs/latest/commands/himport-set/) は準備済みfieldと同じ位置のvalueを対応させ、既存keyがあれば上書きする。値の数が合わない場合や、その接続に定義がない場合はエラー。成功は OK であり、追加field数ではない。

対して [HSET](https://redis.io/docs/latest/commands/hset/) は指定fieldを更新し、追加したfield数を整数で返す。この違いから、既存hashの一部だけを変更する呼び出しを機械的にHIMPORTへ置換してはいけない。たとえば全項目を持つ在庫レコードに「価格だけのfieldset」をimportする設計では、残りの項目を保持する契約にならない。key TTLやfield TTLの保存も、この置換契約から推測しない。

### 3. 再prepareと、データのretryは別に判断する

[2026-08-24の開発元記事](https://redis.io/blog/efficient-bulk-hash-insertion-with-redis-810s-himport/)は、pool内の各接続でprepareが必要と説明する。また同記事のredis-rbは、設定に応じて再接続時にclient registryの定義を再登録すると説明される。これはその記事のclient機能の説明であり、全client共通のserver保証ではない。

再登録で直るのはfield定義の不在である。送信済みレコードがどこまで反映されたか、別writerの新しい値が入っていないかは、それだけでは判定できない。

### 4. DISCARD はimportの取り消しではない

[HIMPORT DISCARD](https://redis.io/docs/latest/commands/himport-discard/) は現在の接続の定義を片付ける。作成済みhashは影響を受けず、定義を削除したら1、存在しなければ0を返す。失敗したbatchの後でDISCARDしても、投入済みデータをrollbackしたことにはならない。

### 5. compact hash の節約はschema次第

[Hashes: Compact hashes](https://redis.io/docs/latest/develop/data-types/hashes/#compact-hashes)は、安定したfield集合を多数keyで共有する場合に適すると説明する。動的なfield追加・削除、共有の少ない集合、非常に大きなhashでは利点が減る。現在文書では、一度compactになったkeyはfield変更後もcompactのまま別集合へ移り、plain hashへ戻らない。設定を戻すだけで全keyの元のencodingへrollbackできるとは扱わない。

[発表記事](https://redis.io/blog/announcing-redis-810-compact-hash-jsonpath-extensions-performance-improvements-and-more/)の最大50%メモリ削減・最大2倍loading throughputは採用判定の保証値にしない。ネットワーク量、record幅、schema分布が異なる自分のデータで測る。

## 安全な取り込み手順（独自の設計案）

以下は上記契約から導いた運用案であり、Redisが追加で提供するtransactionやexactly-onceの保証ではない。

1. **用途をgateする。** 入力が完全なrecordか、対象keyの全置換が許されるかを先に決める。部分更新、既存TTLの維持が不可欠な処理、共有keyspaceで複数writerが更新し続ける処理は、要件を再設計するまでHSETから移さない
2. **入力manifestを固定する。** snapshot世代、key、schema版、field順序、期待値のdigest、期待件数を保存する。schema名には版または順序込みfingerprintを付ける。新旧schemaで同じ名前を再利用して、同じfield数なのに意味だけ変わる状態を作らない
3. **接続を所有する。** import workerが借りた物理接続をprepareから終了確認まで保持し、同じ接続上の無関係な仕事による同名再定義を避ける。再接続時は新しい接続世代としてprepareからやり直す。Clusterのnode変更やredirectも、準備済み接続を引き継ぐと仮定しない
4. **未確認範囲を小さくする。** pipelineの件数・bytesに上限を置き、各replyをrecordに対応させて成功・拒否・不明を記録する。socketへの送信完了や最後のreply一個をbatch全体の成功としない。clientの自動retryが有効かを把握し、未知の処理済み範囲を黙って再送させない
5. **不明な書き込みは照合する。** 隔離した取り込み世代で同じ入力を所有しているなら、再接続後に内容を照合し、必要なrecordを再構築する。live keyを他writerと共有する場合、古いsnapshotの再送が新しい更新を潰しうるので停止して競合を解決する。「同じkeyだからidempotent」という説明では不十分である
6. **切替前に内容を確認する。** 件数だけでなくfield集合、値、業務上必須の項目、想定TTLをmanifestと照合する。旧世代は直ちに削除せず、切替先のread結果を確認できる復帰手順を別に持つ。更新が継続する元データなら、snapshot後の差分取り込みと切替境界も設計対象にする
7. **後片付けを分離する。** 確認済みbatchの後でfieldsetをDISCARDする。接続状態の解放と、データ世代の廃棄は別操作として記録する。性能比較では取り込み中のpeak memoryと通常更新時のlatencyも測り、定常メモリだけで合格にしない

この設計では再prepareの成功を「残りを安全に流してよい」という承認にしない。再開の判断材料は、入力世代と対象keyの所有権、最後に確認した結果、現データの照合結果である。

## 導入前の境界テスト案（未実行）

- 辞書順と異なるfield順序でprepareし、異なる識別しやすい値を投入する。field名と値の対応を読む。値の個数だけで正しさを判定しない
- 同じ接続の同名再定義を含む二つの仕事を交互に実行し、アプリ側の所有・命名規則が取り違えを防ぐことを確認する
- poolの接続Aだけprepareし、接続B、RESET後、再接続後の送信を分ける。clientがどこで再登録するかも記録する
- 既存keyに追加項目とTTLを設け、全record投入と部分record投入を比較する。保存が必要な情報の欠落を検出し、TTLの実際の扱いを対象patchで確定する
- 重複field名、値の過不足、未準備定義をそれぞれ試す。エラー後の対象keyと後続replyも検査し、失敗がbatch全部のrollbackだとは仮定しない
- 応答消失をbatch途中で起こす。別writerの更新がある場合と、隔離した同一snapshotの場合を分け、前者の古いretryを停止できるか確認する
- DISCARD後も作成済みレコードをreadできることと、終了後に不要な接続状態を抱えないことを別に検査する
- 共通schemaが多いデータと、ほぼkeyごとに異なるデータを比較する。投入速度だけでなくメモリ・通常read/write・復旧時間も採否基準にする

## 限界・未確認事項

- server/client実行、performance benchmark、切断注入、Cluster routing、failover、replica、RDB/AOF restore、旧版へのdowngradeは未検証。コマンドが公開されていることから実運用の無停止移行を保証しない
- key TTL、field expiration、異なる型の既存key、エラー時の変更有無、clientの自動retry範囲は採用先で確認する。compact hashの自動変換設定の具体値やRDB変換手順は本書の対象外
- 文書のsorted set表現と宣言順の説明を混同しない。内部実装のcommit分析はしていないため、metadata表現の由来や全patchの一致は未確認
- 検索evalは本書を発見できるかだけを検証する。上記の境界テストが成功したことを意味しない
- 文書ライセンスは [redis/docs LICENSE](https://github.com/redis/docs/blob/main/LICENSE) のCC-BY-NC-SA-4.0と旧文書由来のCC-BY-SA-4.0例外を確認した。blog/release本文の個別ライセンスは未特定として記録した。独自要約と設計案のみで、公式コード・図・長文の転載やmodule化は行っていない

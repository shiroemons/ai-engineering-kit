---
{
  "id": "redis-aof-persistence-durability",
  "title": "Redis の RDB/AOF 永続化 durability: appendfsync everysec / always の fsync 契約と multi-part AOF rewrite",
  "kind": "knowledge",
  "technology": "redis",
  "version": "redis.io persistence docs (unversioned latest 表示, 2026-09-28 取得) + redis/redis tag 8.10.2 redis.conf (2026-09-17 release) + redis/redis Releases (8.10.2 Latest, 2026-09-28 確認)",
  "tags": [
    "research-domain:data",
    "redis",
    "persistence",
    "AOF",
    "RDB",
    "durability",
    "appendfsync",
    "appendfsync everysec",
    "appendfsync always",
    "appendfsync no",
    "fsync",
    "multi-part AOF",
    "AOF rewrite",
    "BGREWRITEAOF",
    "group commit",
    "no-appendfsync-on-rewrite",
    "aof-load-truncated",
    "appendonly",
    "manifest",
    "data loss"
  ],
  "sources": [
    {
      "id": "redis-persistence-docs",
      "url": "https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/",
      "type": "official_docs"
    },
    {
      "id": "redis-conf-8-10-2-persistence",
      "url": "https://raw.githubusercontent.com/redis/redis/8.10.2/redis.conf",
      "type": "official_docs"
    },
    {
      "id": "redis-releases-8-10-2",
      "url": "https://github.com/redis/redis/releases",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-10-28",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# Redis の RDB/AOF 永続化 durability: appendfsync everysec / always の fsync 契約と multi-part AOF rewrite

Redis で「書き込みをどこまで失うか」を決めるのは `appendfsync` の fsync ポリシーと multi-part AOF の構成である。以下は redis.io の persistence 公式文書、redis/redis tag 8.10.2 の redis.conf 例、redis/redis Releases ページ（3件とも 2026-09-28 に確認、文書ページは unversioned な latest 表示）に記載された事実と、それを組み立てる設計案を分けて書く。fsync や throughput の実測値は本調査で確認していない。

## 要点（確認できた公式記載の事実）

### appendfsync 3種の fsync 契約

- `appendfsync always` は AOF log への write のたびに fsync する。公式文書はこれを "very slow but very safe" と表現する。複数クライアントや pipeline からの書き込みはバッチとして1回の write になり、reply の前に1回の fsync が行われる。並行書き込みが入っている group commit の状態では、fsync は1回に集約される。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)
- `appendfsync everysec` は毎秒1回 fsync し、その fsync は background thread が実行する。公式文書は災害時に1秒分の書き込みを失う可能性があると記載する。既定値であり公式の推奨対象である。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)
- `appendfsync no` は fsync を行わず OS に任せる。Linux では通常30秒ごとに flush するが、kernel tuning 次第で変わる。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)

### tag 8.10.2 redis.conf の既定値と欠落幅の説明

- AOF は既定で無効（`appendonly no`）。fsync の既定は `appendfsync everysec`。[redis.conf at tag 8.10.2](https://raw.githubusercontent.com/redis/redis/8.10.2/redis.conf)
- 同ファイルのコメントは、既定ポリシーで失われうる書き込みを「server power outage という dramatic event で1秒分。Redis process 自身が異常な場合は OS が正常なら1 write 分」と説明する。[redis.conf at tag 8.10.2](https://raw.githubusercontent.com/redis/redis/8.10.2/redis.conf)
- `no-appendfsync-on-rewrite` の既定は `no`。`yes` にすると BGSAVE / BGREWRITEAOF の実行中は fsync が停止し、durability は `appendfsync no` と同じになる（default Linux 設定で worst 30秒分の log 欠落）。コメントは既定の `no` が durability 面で最善と記載する。[redis.conf at tag 8.10.2](https://raw.githubusercontent.com/redis/redis/8.10.2/redis.conf)
- 自動 AOF rewrite の既定は `auto-aof-rewrite-percentage 100` と `auto-aof-rewrite-min-size 64mb`（`0%` で自動 rewrite 無効）。`aof-use-rdb-preamble yes`、`aof-timestamp-enabled no` が既定。RDB snapshot の既定 save 条件は `3600 1` / `300 100` / `60 10000`。[redis.conf at tag 8.10.2](https://raw.githubusercontent.com/redis/redis/8.10.2/redis.conf)

### multi-part AOF（Since Redis 7.0.0）

- `appenddirname` 配下に、base file（最大1個。rewrite 時点のスナップショットで RDB 形式か AOF 形式）、incremental file（複数可）、それらの読み込み順を管理する manifest が置かれる。命名例は `appendonly.aof.1.base.rdb` / `appendonly.aof.1.incr.aof` / `appendonly.aof.manifest`、既定の `appenddirname` は `appendonlydir`。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)・[redis.conf at tag 8.10.2](https://raw.githubusercontent.com/redis/redis/8.10.2/redis.conf)
- 7.0 以降の rewrite では、parent が新しい increment を書き続けたまま child が base を生成し、temp manifest を atomic swap する。失敗した rewrite は間隔を広げる再試行制限を持つ。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)
- Redis < 7.0 では rewrite 中の新規書き込みがメモリに buffer され、rewrite 完了時に二重書き込み・freeze しうると公式文書は記載する。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)

### 起動時の読み込みと壊れた AOF

- `aof-load-truncated` は既定で有効。末尾が切り詰められた AOF は最終コマンドを破棄してログを出し、load する。途中の corruption は既定では起動を拒否する。`aof-load-corrupt-tail-max-size` は既定 `0`（無効）。[redis.conf at tag 8.10.2](https://raw.githubusercontent.com/redis/redis/8.10.2/redis.conf)
- AOF と RDB が同時に有効な場合、restart（開始時）のデータは AOF をロードして使う。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)・[redis.conf at tag 8.10.2](https://raw.githubusercontent.com/redis/redis/8.10.2/redis.conf)

### AOF のバックアップと BACKUP コマンド

- 7.0 以降は、rewrite 中の appenddirname をそのままコピーしない手順が必要になる。公式文書は `auto-aof-rewrite-percentage 0` で自動 rewrite を止めて `INFO persistence` で rewrite 非実行を確認する方法と、hard link を使う方法を示す。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)
- Since Redis 8.10.0 の BACKUP コマンド族は multi-part AOF 互換の BASE / INCR / manifest を生成する。確認した Releases ページでは tag 8.10.2（2026-09-17 release）が Latest 表示であり、8.10.0 以降の機能を含む版数である。[Redis persistence](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)・[Releases](https://github.com/redis/redis/releases)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- durability 要件が未定義のサービスでは既定の `appendfsync everysec` を維持し、許容できる data loss を「power outage 時で最大1秒（Redis process 異常時は OS 正常なら1 write 分）」としてチームで文書化する。失うわけにはいかない書き込みがある場合だけ `appendfsync always` を検討し、公式が "very slow" と評価するコストを実ワークロードの throughput 測定で確認してから採用する。group commit で並行書き込み時に fsync が1回へ集約されても、単独コマンドの reply 前に fsync が入る契約は変わらない。
- `no-appendfsync-on-rewrite yes` は「rewrite 中の durability が `appendfsync no` 相当になりうる」設定として扱う。スループット目的で使う場合も、default Linux で worst 30秒分の log 欠落という窓と `BGSAVE` も対象になる点を許容条件として明示する。
- AOF バックアップは multi-part AOF 想定の手順を使う。Redis 8.10 以降なら BACKUP コマンド族が BASE / INCR / manifest を生成するため第一候補にし、7.x では rewrite 非実行を `INFO persistence` で確認してから copy（または hard link）する。
- Redis < 7.0 の rewrite 中バッファリング（二重書き込み・完了時 freeze）を踏まえ、multi-part AOF を使える版へのアップグレードを永続化構成の前提条件にする。
- AOF を有効化する運用では `appendonly no` が既定である点を踏まえ、有効化・無効化の変更を設定管理で明示し、AOF と RDB の同時有効時に restart が AOF を使うことを利用側の復旧手順に反映する。

## 避ける使い方

- `appendfsync always` を「everysec とほぼ同じ速さ」と期待する。公式文書は very slow と明記しており、実測なしで採用しない。
- `appendfsync everysec` を「書き込みを失わない保証」と説明する。公式文書は災害時に1秒分失う可能性を明記しており、redis.conf コメントも Redis process 異常時に OS 正常なら1 write 分失うと説明する。
- `no-appendfsync-on-rewrite yes` を「rewrite の最中だけの短い一時措置」として安易に設定する。失われうる窓は default Linux で worst 30秒分であり、対象は `BGREWRITEAOF` だけでなく `BGSAVE` も含む。
- `aof-load-truncated yes` を「corruption なら何でも起動できる」と読む。既定で救済されるのは末尾の切り詰めだけで、途中 corruption は既定で起動拒否、`aof-load-corrupt-tail-max-size` は既定 `0` で無効である。
- 7.0 以降の `appenddirname` を rewrite 実行中に素直に tar / cp する。公式文書は rewrite 中のコピー回避手順（自動 rewrite 停止 + `INFO persistence` 確認、または hard link）を示している。
- Redis < 7.0 の rewrite 挙動を 7.0 以降の multi-part AOF と同じとみなす。旧バージョンは rewrite 中の書き込みをメモリに buffer する点で挙動が異なる。

## 適用版と本番での注意

- 適用版: redis.io の persistence 公式文書（unversioned な latest 表示、2026-09-28 取得。pinned version ではない）、redis/redis tag 8.10.2 の redis.conf 例（2026-09-17 release。Releases ページで 8.10.2 が Latest 表示、tag commit `498ecd0d6d007db11ddb3aea9428552598a78622`、2026-09-28 確認）。将来の最新とは扱わない。
- 再確認期限: 3件中 Releases ページだけが `release_notes`（TTL 30 日）のため、文書の期限は 2026-10-28。`official_docs` 2件（persistence 文書・redis.conf）は TTL 90 日。redis に技術固有 TTL は設定されていない。
- 未確認事項（本調査の範囲外として推測で埋めない）: fsync / write の実測レイテンシと throughput、仮想化・クラウドディスクなどハードウェア構成による差、Linux 以外の OS の flush 間隔、`everysec` の background thread が fsync を完了できない場合の挙動、replica や replication 確度（`WAIT` 系）との相互作用、disk full 時の挙動、Redis Enterprise の永続化、BACKUP コマンド族の失敗時エラー契約。
- 本ドキュメントの推奨は設計案であり、単一事例・単一ベンチマークの一般化ではない。許容 data loss と throughput のバランスはワークロードとハードウェアに依存するため、対象構成で測定して決める。

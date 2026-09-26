---
{
  "id": "redis-eviction-policy-cache",
  "title": "Redis の maxmemory と eviction policy 選択 (cache 用)",
  "kind": "knowledge",
  "technology": "redis",
  "version": "Redis 7.4.0 redis.conf example + Redis official eviction-policy docs (retrieved 2026-09-26; not a latest-version claim)",
  "tags": ["research-domain:data", "redis", "maxmemory", "maxmemory-policy", "eviction", "noeviction", "allkeys-lru", "volatile-ttl", "expire", "cache"],
  "sources": [
    {"id": "redis-conf-7-4-maxmemory", "url": "https://raw.githubusercontent.com/redis/redis/7.4.0/redis.conf", "type": "official_docs"},
    {"id": "redis-eviction-policy-docs", "url": "https://redis.io/docs/latest/operate/rs/databases/memory-performance/eviction-policy/", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/data.json"]
}
---

# Redis の maxmemory と eviction policy 選択 (cache 用)

cache 用途で `maxmemory` 上限と `maxmemory-policy` を選ぶための確認済み契約だけを書く。個々の expire 系コマンドの振る舞いや LRU / LFU の内部サンプリング実装は本調査の検証範囲外であり、推測で埋めない。

## 要点（確認できた公式記載の事実）

- `maxmemory` は Redis のメモリ使用量上限を byte 単位で定める。上限に到達した後は `maxmemory-policy` に従って key を除去して空きを作る。[redis.conf at tag 7.4.0](https://raw.githubusercontent.com/redis/redis/7.4.0/redis.conf)
- policy が `noeviction` の場合、または除去可能な key が存在しない場合は、メモリを使う書き込み（例: `SET`、`LPUSH` 等）が error になる。一方 `GET` のような read-only コマンドは動き続ける。[redis.conf at tag 7.4.0](https://raw.githubusercontent.com/redis/redis/7.4.0/redis.conf)
- 確認した policy 名は `noeviction`、`allkeys-lru`、`allkeys-lfu`、`allkeys-random`、`volatile-lru`、`volatile-lfu`、`volatile-random`、`volatile-ttl` である。同ページには Enterprise 向けの LRM 系 variant の記載もあるが、本ドキュメントは OSS の 8 値だけを cache 選択の対象にする。[Eviction policy](https://redis.io/docs/latest/operate/rs/databases/memory-performance/eviction-policy/)
- `volatile-*`（`volatile-lru`、`volatile-lfu`、`volatile-random`、`volatile-ttl`）は expire が設定された key のみを除去対象にする。`volatile-ttl` は残り TTL (time to live) が最短の key を選ぶ。[Eviction policy](https://redis.io/docs/latest/operate/rs/databases/memory-performance/eviction-policy/)
- 同ページは standard の既定を `volatile-lru`、Active-Active の既定を `noeviction` と記載している。ただし OSS の `redis.conf` の既定は別途確認が必要であり、本ドキュメントは OSS 既定値を断定しない。[Eviction policy](https://redis.io/docs/latest/operate/rs/databases/memory-performance/eviction-policy/)・[redis.conf at tag 7.4.0](https://raw.githubusercontent.com/redis/redis/7.4.0/redis.conf)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- 純粋な cache（miss したら再取得・再計算できる）では `allkeys-lru` 系を第一候補にする。特定 key だけを残したい混在用途ではなく、全 key が除去候補でよい場合に `maxmemory` と組み合わせる。
- 永続データと cache を同インスタンスに混在させ、永続 key を eviction から守りたい場合だけ `volatile-*` を選ぶ。選ぶ前に、守りたい key 以外の全 cache key に expire が付く運用（書き込み経路での付与漏れ対策を含む）を用意する。expire なしの key が多いと `volatile-*` は除去候補を見つけられず書き込み error に寄る。
- 残り TTL が優先度の代理になる用途（短い TTL から消してよい）に限って `volatile-ttl` を選ぶ。アクセス頻度・最近性を優先する場合は `volatile-lru` / `volatile-lfu` 側で選ぶ。
- 書き込み error を許容できない重要書き込みがある構成では `noeviction` を cache 用の第一選択にしない。`noeviction` 下では上限到達後の書き込みが error になるため、使う場合は上限超過時のアプリケーション側の振る舞い（リトライ可否、欠落許容、read-only 継続の扱い）を先に決める。

## 避ける使い方

- expire を付けていないのに `volatile-lru` / `volatile-lfu` / `volatile-random` / `volatile-ttl` を選ぶ。除去対象が空になり、公式記載どおり書き込みが error になる。
- `noeviction` を「上限後は古いものから消える」と思い込んで cache に使う。確認した記載では `noeviction` 下の書き込みは error であり、自動除去は起きない。
- `volatile-ttl` を「アクセス頻度の低いものから消す」目的で選ぶ。確認した記載では選択基準は残り TTL の短さであり、アクセス頻度・最近性ではない。
- ページ記載の既定（standard `volatile-lru` / Active-Active `noeviction`）を OSS `redis.conf` の既定と同一視する。両者は別確認が必要と明記されているため、適用先の版の `redis.conf` を直接確認する。

## 適用版と本番での注意

- 適用版: `redis/redis` tag `7.4.0` の `redis.conf` 例と、2026-09-26 に確認した eviction-policy の公式 docs ページ（latest 表示）。将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90 日）で redis に技術固有 TTL がないため、2026-12-25 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: `EXPIRE` / `TTL` 系コマンドの個別契約、passive / active な期限切れのタイミング、LRU / LFU のサンプリング数や精度、error 応答の文字列、replica や cluster での eviction の伝播、Enterprise LRM variant の選択条件、timeout・cancel・shutdown 時の eviction の扱い。これらは該当版の該当節を別途確認する。
- 本ドキュメントの推奨構成は設計案であり、特定ベンチマークや単一事例の一般化ではない。ヒット率・書き込み error 率・レイテンシへの影響はワークロードと `maxmemory` 値に依存するため、対象構成で測定して採用する。

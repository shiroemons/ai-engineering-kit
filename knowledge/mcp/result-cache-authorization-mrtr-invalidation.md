---
{
  "id": "mcp-result-cache-authorization-mrtr-invalidation",
  "title": "MCP result caching: ttlMs・cacheScope・MRTR除外・通知失効の境界",
  "kind": "knowledge",
  "technology": "mcp",
  "version": "MCP specification 2026-07-28 revision: Caching / Schema Reference / Tools / Pagination / Subscriptions / Streamable HTTP; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:ai-engineering",
    "mcp",
    "result-cache",
    "ttlMs",
    "cacheScope",
    "authorization-context",
    "MRTR",
    "cache-invalidation",
    "pagination",
    "subscriptions/listen"
  ],
  "sources": [
    {
      "id": "mcp-result-cache-contract-20260728-verified-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/caching",
      "type": "official_docs"
    },
    {
      "id": "mcp-result-cache-schema-20260728-verified-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/schema",
      "type": "official_docs"
    },
    {
      "id": "mcp-result-cache-changelog-20260728-verified-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/changelog",
      "type": "official_docs"
    },
    {
      "id": "mcp-result-cache-tools-20260728-verified-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/server/tools",
      "type": "official_docs"
    },
    {
      "id": "mcp-result-cache-pagination-20260728-verified-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/pagination",
      "type": "official_docs"
    },
    {
      "id": "mcp-result-cache-subscriptions-20260728-verified-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/subscriptions",
      "type": "official_docs"
    },
    {
      "id": "mcp-result-cache-transport-20260728-verified-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# MCP result cache の再利用・共有・失効境界

## 問いと採用判断

MCP gateway が tool 一覧や resource を保存して再利用するとき、同じ URL・同じユーザーなら返してよいか。elicitation の回答を伴う再試行、認可の切替、通知切断を挟んでも同じ結果として扱えるか。

**method と結果に影響する params、認可コンテキスト、freshness を別々に判定し、MRTR の結果はキャッシュから除外する。** `cacheScope: "public"` は認証済み endpoint の結果でも共有可能という表明になるため、一覧に tenant 固有情報が混じる実装で安易に付けない。

本書は [2026-07-28 の変更一覧][changes]が導入した result caching の未整理だった適用境界を扱う。2026年9月・10月の新規リリースではない。全ページは `2026-07-28` の名前付き仕様を2026-10-03 UTCに読み直したもので、各段落の追加日や SDK の導入版は未特定。

[LLM prompt cache](../llm/prompt-caching-breakpoints-ttl-scope.md)は推論入力の再利用、[Claude の tool-list pinning](../agents/claude-inline-tools-mcp-list-pinning.md)は会話に渡す定義の固定である。ここで扱う MCP 応答キャッシュとは別層であり、どちらも remote tool の実行許可を与えない。[Tasks の ttlMs](tasks-extension-polling-cancellation-migration.md)も task の保有期限であり、本書の応答受信からの鮮度期間と混同しない。

## 確認した契約: キャッシュ対象を先に限定する

[Caching: Cacheable Results / Cache Key][caching]は次を規定する。

- `resultType: "complete"` のうち `server/discover`、`tools/list`、`prompts/list`、`resources/list`、`resources/templates/list`、`resources/read` の6種類には caching hints が必須
- `input_required` はキャッシュ不可で hints も持たない
- `inputResponses` または `requestState` を伴う MRTR 再試行から得た結果は **MUST NOT cache**。最終的な `complete` でも対象外
- method と結果に影響する params がキーになる。例えば `resources/read` の `uri`、list の `cursor` が違えば同じ結果を返してはならない

したがって `complete` だけで汎用 cache middleware を通す設計は不足する。`tools/call` や `prompts/get` は上記6種類に含まれず、この契約を根拠にキャッシュを有効にしない。

[Streamable HTTP][transport]では JSON-RPC 要求ごとに同じ MCP endpoint へ別々の POST を送る。従って `POST /mcp` という HTTP method と URL だけでは、異なる MCP method・resource URI・cursor を区別できない。本文の hints を通常の HTTP cache 設定へ機械的に移すだけで安全になる、という規定も確認していない。

## 確認した契約: private と public は実行権限ではない

[Schema Reference の ListToolsResult / ReadResourceResult / DiscoverResult][schema]では `ttlMs` と `cacheScope` が必須フィールドである。`public` はユーザー固有情報を含まず、認可コンテキストをまたぐ共有を許す。`private` の再利用は同じ authorization context に限定され、異なる access token は異なる cache が必要になる。

[Tools: Capabilities][tools]によると、`tools/list` は接続ごとの状態では変わってはならないが、要求で提示された認可によって集合が変わることは許される。また、集合が変わらなければ決定的な順序で返すことが SHOULD。**stateless は全ユーザーに同じ一覧を返す意味ではない。**

認証が必要な endpoint だから共有されない、という判断はしない。Caching の Security Considerations は認証済み endpoint の `public` 結果も別 token と共有され得るとし、primitive ごとのアクセス制御を別途必須としている。[Caching][caching]

## 確認した契約: ttlMs と通知は二つの失効条件

`ttlMs` はミリ秒の非負整数。正値は応答受信後の fresh 期間を示し、0 は即 stale。旧 server の欠落は0を仮定する SHOULD、負値も無視して0として扱う SHOULD であり、server は負値を返してはならない。fresh 判定は `now < t_received + ttlMs`。期限到来そのものを自動 polling の周期にはせず、次の利用時の再取得判断に使う。polling を選ぶ実装には jitter と backoff が必須である。[Caching: TTL / Freshness Calculation][caching]

TTL は期間中の不変保証ではない。関連通知を受けたら残り時間に関係なく即 stale になる。refetch エラー時の stale 利用は MAY だが、無条件に採用すべき推奨ではない。[Caching: Notifications][caching]

[Subscriptions][subscriptions]では `subscriptions/listen` の最初の通知が `notifications/subscriptions/acknowledged` であり、受諾された filter の subset が返る。要求した `toolsListChanged` が ack に無ければ、その通知を受ける前提にはできない。購読stream上の全通知は `io.modelcontextprotocol/subscriptionId` で元の購読に対応し、transport 切断で購読は終わる。HTTP の [Last-Event-ID による再開はない][transport]ため、再接続しただけで切断中の変更を回収したとは扱わない。

## 確認した契約: ページごとのfreshと全体snapshotを分ける

各ページは別々の受信時刻・TTL を持ち、同じ list request の全ページは同じ `cacheScope` を持つ必要がある。ページ間の snapshot 一貫性は保証されず、重複・欠落が起こり得る。cursor が無効になったら全ページを捨てて先頭から再取得する SHOULD である。一貫した一覧が必要な場合も先頭からの再取得が SHOULD とされるが、それを snapshot 保証の新設と読み替えない。[Caching: Pagination][caching]

[Pagination][pagination]では cursor は opaque であり、解析・改変してはならない。`nextCursor` の欠落が終了を示す一方、**空文字 `""` は有効な cursor** で、truthiness による終了判定は禁止される。固定ページサイズも仮定しない。

## 独自の設計案: 安全側のcache adapterにする

以下は仕様の実装案であり、MCP が指定する唯一のアルゴリズムではない。

1. **lookup 前に対象を絞る。** 対応 protocol revision と6メソッドの allowlist を確認し、MRTR のフィールドが存在する要求は lookup と格納をともに bypass する。空の `inputResponses` を false 相当として見落とす分岐を作らない。応答側では `complete` と method 固有schemaを再確認する
2. **server 境界をキーの外枠に置く。** 接続設定から確定する server 識別・protocol revision・MCP method・結果に影響する params を保持する。同じ文字列の resource URI でも別 server なら別物として扱う。結果に影響しないと検証できない `_meta` や拡張paramsを、hit率向上のために勝手に落とさない
3. **認可を別namespaceにする。** `private` では token の切替を新しい namespace とし、同じユーザー名だけで統合しない。namespace識別子をログ可能な不透明値にして、access token 自体をキー文字列・ログへ出さない。`public` の採用前には説明文、URI、schema、内部識別子まで含めて全ユーザー共通かを点検する
4. **返答と履歴の扱いを分ける。** 再利用対象は検証した result とcache metadataに限定する。過去のJSON-RPC envelopeをそのまま送り返して、元の request ID を新要求へ混入させない。モデルに渡す現在のtool一覧と、過去会話で提示済みの定義も別に持つ
5. **cache hitで期限を延長しない。** 受信時刻は実際の新しい応答を受けた時点として記録し、ローカル再利用を新受信扱いにしない。プロセス内は単調時計で経過を測り、最初の実装では再起動時に破棄して時計復元の曖昧さを避ける。数値の型、加算のoverflow、容量上限も入力境界で検査する
6. **失効とin-flight格納を同じ世代で制御する。** 一覧を読み始めた世代と、通知処理後の世代を比較し、途中で失効した古い返答はcacheへ格納しない。例えば read開始→list_changed→旧read完了の順でも、通知で消した一覧が復活しないようにする。HTTP応答と購読streamのcallback順まで保証されるとは仮定しない
7. **切断・未受諾通知の方針を明示する。** 通知の連続性を前提に使う一覧は切断時に失効させ、再購読のack後に再取得する。通知非対応ならTTLに基づくon-access再取得へ落とす。ack後の取得でも snapshot / exactly-once の保証にはならない。再試行回数・待ち時間の上限を設け、変更が続く場合は失敗を可視化する
8. **stale の用途を限定する。** 一時的な表示と実行直前の判断を分け、古いtool定義・resource情報を実行権限の根拠にしない。fresh一覧も権限失効の即時反映を保証しない。期限内の一覧があっても実行先では現在のアクセス制御を適用する。legacy の `cacheScope` 欠落をpublicと補完せず、共有不可・cache無効を初期方針にする。これは仕様に未記載の既定値を埋める保守的な設計判断である

通知を先に確立してから一覧を取得する方法も、世代チェックも、漏れなく同一時点のsnapshotを得る証明にはならない。完全な一貫性を要件にするなら、そのserverのversion付きresource等の別契約を確認し、存在しなければ保証できないと明示する。

## 受入試験案と観測点

下記は導入先で行う試験案であり、本調査では通信・SDK実行を行っていない。

- `server/discover` を含む対象6種類と、`tools/call` / `prompts/get` / `input_required` を分け、後者を同じcacheへ格納しない
- 同じ `uri` でも MRTR なし、`inputResponses` あり、`requestState` ありを区別し、再試行が `complete` になってもbypassする
- token A / B、同一ユーザーでのtoken更新、異なるserverの同一URI、異なるcursorでcacheの交差が起きない
- `ttlMs: 1500` の受信から1499msではfresh、1500msではstaleとなる境界を確認する。local hitで延命しない。0・欠落・負値・overflow相当の値も試す
- fresh期間中の関連通知、ackのsubset、未受諾filter、購読切断、古い世代の遅延返答をそれぞれ試す
- `nextCursor: ""` を次ページへ渡し、欠落だけを終了とする。ページ間で異なるTTLと矛盾するscope、invalid cursor、更新による重複・欠落を試す
- legacy のscope欠落やmodernの必須フィールド欠落を、許可範囲の拡大として修復しない
- cache hit数だけでなく、認可namespace・MRTRによるbypass、通知失効、cursor再開始、stale表示、schema不適合を理由別に観測する。本文・tokenをログに出す必要はない

## 出典・provenance・未確認事項

7件はいずれも公式の名前付き仕様ページで、規範語の強さは各本文に合わせた。front matter の `trust: primary-source` は独自の実装判断も含むためである。全sourceを2026-10-03に新規記録し、`official_docs` TTL 90日に合わせて2027-01-01を再確認期限とした。古い共有catalog記録の日付は延長していない。

[repository LICENSE][license]は新しい仕様貢献をApache-2.0、再許諾未同意の元貢献をMITのまま、とする移行状態を示す。通常documentationのCC-BY-4.0と仕様の扱いを区別し、当該各段落の著者別割当は未特定。本文は独自の日本語要約と設計案で、コード・図・長文の転載やmodule昇格は行わない。LICENSEは40桁commitで確認したが、web上の仕様本文とそのcommitの同一性は主張しない。

未確認なのは、具体的SDKのcache自動実装・対応版、gatewayが本文hintsを扱う方式、通知配送の遅延上限、認可変更の通知範囲、複数serverや複数拠点での実測挙動である。HTTPの `Age` / `Cache-Control` / `Vary` との完全な写像やETag再検証も確認していない。形式検証と検索evalの成功は、これらの通信・安全性の試験を代替しない。

[caching]: https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/caching
[schema]: https://modelcontextprotocol.io/specification/2026-07-28/schema
[changes]: https://modelcontextprotocol.io/specification/2026-07-28/changelog
[tools]: https://modelcontextprotocol.io/specification/2026-07-28/server/tools
[pagination]: https://modelcontextprotocol.io/specification/2026-07-28/server/utilities/pagination
[subscriptions]: https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/subscriptions
[transport]: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http
[license]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/3098fe94caa1b9e0afaaa6d30e040b61d5802471/LICENSE

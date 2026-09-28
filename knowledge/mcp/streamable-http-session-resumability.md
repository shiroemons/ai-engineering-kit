---
{
  "id": "mcp-streamable-http-session-resumability",
  "title": "MCP Streamable HTTP のセッション管理と resumability（2026-07-28 改訂で廃止）",
  "kind": "knowledge",
  "technology": "mcp",
  "version": "MCP specification 2026-07-28 revision（legacy の挙動は 2025-11-25 revision を確認）",
  "tags": [
    "research-domain:ai-engineering",
    "mcp",
    "streamable-http",
    "session",
    "resumability",
    "Mcp-Session-Id",
    "Last-Event-ID",
    "initialize",
    "handshake",
    "sse",
    "reconnection",
    "transport",
    "stateless",
    "SEP-2567",
    "SEP-2575"
  ],
  "sources": [
    {
      "id": "mcp-spec-streamable-http-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http",
      "type": "official_docs"
    },
    {
      "id": "mcp-spec-changelog-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/changelog.md",
      "type": "official_docs"
    },
    {
      "id": "mcp-spec-transports-2025-11-25",
      "url": "https://modelcontextprotocol.io/specification/2025-11-25/basic/transports",
      "type": "official_docs"
    },
    {
      "id": "mcp-spec-lifecycle-2025-11-25",
      "url": "https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle",
      "type": "official_docs"
    },
    {
      "id": "mcp-spec-versioning-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning",
      "type": "official_docs"
    },
    {
      "id": "mcp-spec-security-best-practices-2025-11-25",
      "url": "https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# MCP Streamable HTTP のセッション管理と resumability（2026-07-28 改訂で廃止）

MCP の Streamable HTTP transport は 2026-07-28 改訂で、protocol-level session（`Mcp-Session-Id`）と SSE イベントの resumability（`Last-Event-ID`）を削除した。本ドキュメントは「legacy（2025-11-25 以前）の session・再接続仕様の正確な内容」と「現行リビジョンでの扱い」を対で記録する。[公式 Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http) / [Changelog 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/changelog.md) / [Transports 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports) / [Lifecycle 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle) / [Versioning 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning) / [Security Best Practices 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices)

## 要点

### 現行リビジョン（2026-07-28）: session と resumability の削除（公式 Streamable HTTP / Changelog）

- Streamable HTTP ページの Info ボックスは「Revision 2026-07-28 changed the behavior of Streamable HTTP … Removal of the GET stream endpoint. Removal of protocol-level sessions.」と明記する。
- 旧リビジョン（2025-03-26〜2025-11-25）で使われた次の4機構は、**本 revision の一部ではない**（"None of these mechanisms are part of this revision"）。
  1. `Mcp-Session-Id` ヘッダによる session の指定
  2. HTTP DELETE による session の明示終了
  3. GET による独立 SSE ストリーム（ストリーム読み取り用 endpoint）
  4. `Last-Event-ID` による resumability（"Resumable SSE streams via `Last-Event-ID` are not supported."）
- 本 revision の server の対応は、旧クライアント向け互換挙動として次の通り。古いクライアントからの **GET / DELETE には 405**、`Mcp-Session-Id` は **ignore して mint / echo しない**、`Last-Event-ID` も **ignore**。
- **cancellation** は SSE レスポンスストリームの close で表現する。
- リクエストヘッダ `MCP-Protocol-Version` / `Mcp-Method` / `Mcp-Name` が必須。ヘッダ不一致は **400 + `-32020 HeaderMismatch`**、未知メソッドは **404 + `-32601`**。
- [Changelog](https://modelcontextprotocol.io/specification/2026-07-28/changelog.md) の Key Changes で session / resumability に直接効くのは、削除理由付きの3項目。
  1. **Key Changes #1: protocol-level sessions と `Mcp-Session-Id` ヘッダの削除（SEP-2567）**: list endpoints は接続毎に変化しない。cross-call 状態は server-minted handle を**通常の tool 引数で渡す**。
  2. **Key Changes #2: `initialize` / `notifications/initialized` handshake の削除（SEP-2575）**: version / capabilities を `_meta`（`io.modelcontextprotocol/protocolVersion`、`clientCapabilities`）で毎リクエスト送付し、不一致は `UnsupportedProtocolVersionError`。
  3. **Key Changes #9: `Last-Event-ID` と SSE event ID による resumability / redelivery の削除（SEP-2575）**: 切断された response stream は in-flight request を失い、**client は新規 request ID で再発行が MUST**。
- 併せて `server/discover` は servers MUST（Key Changes #3）、HTTP GET endpoint は削除され `resources/subscribe` / `unsubscribe` は `subscriptions/listen` に置換（#4）。Roots / Sampling / Logging と旧 HTTP+SSE transport は deprecated。

### legacy（2025-11-25）の session 管理（公式 Transports "Session Management"）

- server は initialization 時、`Mcp-Session-Id` を **HTTP レスポンス（InitializeResult を載せるもの）で付与してよい（MAY）**。値は可視 ASCII 0x21–0x7E、推奨は暗号学的に安全な値（UUID / JWT 等）。
- 付与された場合、client は**全後続リクエストで送付が MUST**。session を必須とする server は欠落時に **400 を返す SHOULD**。
- server はいつでも session を終了してよく（MAY）、**終了後は 404 を返す MUST**。404 を受けた client は、**session ID なしで新しい `InitializeRequest` を送る MUST**。
- client は不要な session を **HTTP DELETE で明示終了することが SHOULD**（server は 405 を返してよい MAY）。
- この revision では `MCP-Protocol-Version` ヘッダが必須で、欠落時は 2025-03-26 とみなす SHOULD。

### legacy（2025-11-25）の resumability と再接続ウィンドウ（公式 Transports "Resumability"）

- server は SSE event に **`id` を付けてよい（MAY）**: session 内でグローバル一意、per-stream の cursor として扱う。
- server は event ID 付きの空 data の **prime event を即時に送る**。connection を stream 終了前に close してよい（MAY）、`retry` フィールドを送ることが SHOULD。
- client は **GET + `Last-Event-ID` で再接続することが SHOULD**。replay は**同一 stream に限定**され、他の stream の再配信は MUST NOT。
- server は session 期限切れ時に SSE stream を終了してよい（MAY）。
- **数値の再接続ウィンドウ（reconnection window、例: 分単位の猶予）は本文に存在しない。** 2025-11-25 の Transports にも Security Best Practices にも「何分以内に再接続せよ」という値は書かれておらず、期限切れ時の stream 終了 MAY の記述だけが根拠になる。さらに現行 2026-07-28 では resumability 自体が削除されており、再接続ウィンドウの概念が事実上消えた。

### legacy（2025-11-25）の initialize handshake（公式 Lifecycle）

- 初期化は first interaction として定義される: client が `initialize` を送り **protocolVersion / capabilities / clientInfo** を交換、server が対応する capabilities と serverInfo を返す。成功後、client は **`notifications/initialized` を送信が MUST**。
- initialize 応答前は client は **ping 以外のリクエスト送信 SHOULD NOT**。`initialized` を受領するまで server も ping / logging 以外のリクエスト SHOULD NOT。
- Version Negotiation: client は支持版の最新を送り、server は同一版を返すか他版を返す。互換が無ければ client は **disconnect が SHOULD**。
- HTTP 使用時は後続の全リクエストに `MCP-Protocol-Version` ヘッダを送ることが MUST。
- shutdown に専用メッセージは無く、HTTP では**接続クローズで終了を示す**。

### era 判定と互換（公式 Versioning 2026-07-28 / Streamable HTTP）

- 現行に negotiation handshake は存在しない。**Modern = 2026-07-28 以降（per-request metadata）**、**Legacy = initialize handshake で session を確立する 2025-11-25 以前**と定義される。Modern↔Legacy の直接対話は互換マトリクス上で fails。
- version 不一致は **`-32022 UnsupportedProtocolVersionError`** に supported 一覧を添えて返すことが MUST、client は互換版で retry することが SHOULD。
- era 判定は server の属性として行う。**stdio は `server/discover` probe**、**HTTP は modern request を先に試し、4xx で modern error body でなければ `initialize` へ fallback**（Streamable HTTP ページの互換注記では「modern request → 400 の body を検査 → initialize へ fallback」の順）。
- version / capabilities のキャッシュは server process（stdio）/ origin（HTTP）の lifetime までが SHOULD。
- dual-era server は、modern `_meta` を stateless に処理しつつ `initialize` を legacy 意味論（HTTP では session scoping）で処理し、**同一 endpoint / プロセスでの併存が MAY**。

### session のセキュリティ（公式 Security Best Practices 2025-11-25）

- session は prompt injection の経路になり得る: session ID を持つ攻撃者が他サーバから event を enqueue し、resumable stream 経由で本物の client に届く恐れ。
- Mitigation として、servers は inbound request を MUST 検証、**session を認証に使ってはならない（MUST NOT）**、session ID は secure / non-deterministic（推測・連番を避け、rotating / expiring を推奨）、queue 保存時は `<user_id>:<session_id>` 形式で user-specific 情報と結合することが SHOULD。
- session 終了後の再接続手順としての数値ウィンドウ規定は存在しない（上記の通り）。session 管理自体は 2026-07-28 で廃止されたため、この節は legacy 実装の脅威評価にのみ残る。

## 推奨方法

以下は上記の公式要件からの**設計上のまとめ**であり、仕様が定める実装構成そのものではない。

- 新規実装は 2026-07-28 を前提に **stateless** で設計する。接続を跨ぐ状態は server-minted handle を通常の tool 引数で受け渡す（SEP-2567 の方向）。
- 通知の購読は `subscriptions/listen` を使い、**応答ストリームの再開はできない前提**で、切断された in-flight request を新規 request ID で再発行する再試行ロジックを client 側に持たせる（SEP-2575）。
- 古い SDK / プロキシと混在する HTTP 端点では、era 判定を「modern request → 4xx かつ modern error body でなければ initialize fallback」で実装し、stdio 経路では `server/discover` probe を使う。
- 古いクライアントへの応答は、仕様どおり GET / DELETE に 405 とし、`Mcp-Session-Id` / `Last-Event-ID` を echo しない。互換のために session 状態を残すと、削除された機構が復活して互換マトリクス（Modern↔Legacy fails）から外れる。
- 旧 session 機構を維持する場合も、**session を認証として扱わない**。認可は [mcp-authorization](authorization.md) の OAuth 2.1 フローに任せ、session ID は推測不能・期限付き・ユーザー識別子と結合して管理する。
- resumability が必要な要件は、プロトコルの再送に頼らず「リクエストを冪等に再発行できるか」で設計する。冪等性の一般論は [http-retry-idempotency](../http/retry-idempotency.md) を参照（本トピックの仕様外の設計判断として記録）。

## 避ける使い方

- **`Last-Event-ID` による stream 再開を現行サーバで前提にする**。2026-07-28 では "Resumable SSE streams via `Last-Event-ID` are not supported"、サーバはヘッダを無視する。
- **`Mcp-Session-Id` を送れば session が確立・維持されると仮定する**。現行サーバは mint も echo もしない。旧 server の 400（session 欠落）/ 404（session 終了）と、現行 server の 405（GET / DELETE）を混同すると era 判定を誤る。
- **切断後に同一 request ID で再送する**。現行では切断された response stream は in-flight request を失い、新規 request ID での再発行が MUST。
- **数値の再接続ウィンドウ（分単位）が仕様にあると想定して実装・監視を組む**。legacy にも現行にもそのような規定は無い。
- **session を認証として使う**。Security Best Practices の MUST NOT 違反。
- **旧 handshake を必須として modern server に `initialize` を投げ続ける**。現行では `initialize` は era fallback 経路としてのみ意味を持ち、version / capabilities は `_meta` の毎リクエスト送付が正。
- **`resources/subscribe` / GET stream / HTTP DELETE 終了を現行実装に残す**。`subscriptions/listen` へ置換済みで、GET / DELETE は 405 対象。
- **公式チュートリアルや旧 SDK の session コードを現行リビジョン用としてそのままコピーする**。同リポジトリの [mcp-authorization](authorization.md) でも、2026-07-28 チュートリアルのサンプルが `Mcp-Session-Id` を含み changelog と食い違う点を観察済み（サンプルの適用リビジョンは未確認）。

## 適用版と本番での注意

- **確認済みの版**: 2026-07-28 revision の Streamable HTTP / Changelog / Versioning、2025-11-25 revision の Transports / Lifecycle / Security Best Practices（6件とも 2026-09-28 に公開ページで確認。changelog の catalog 記録は既存の取得日 2026-09-26 をそのまま再利用）。本文の MUST / SHOULD / MAY は各リビジョンの normative 表記の引用で、「推奨方法」節の設計案と区別している。draft リビジョンは根拠に使っていない。
- **旧リビジョンの値を新規向けに持ち込まない**: `Mcp-Session-Id`・`Last-Event-ID`・GET stream・DELETE 終了・initialize handshake はすべて 2025-11-25 以前の仕様。旧版向け SDK と混在させない。
- **未確認**: 各公式 SDK（TypeScript / Python / Go / C#）が 2026-07-28 の session 廃止・handshake 廃止・`server/discover` MUST をどの版で実装したか。実装前に SDK の release notes を確認する。
- **未確認**: 古いロードバランサ / プロキシが GET ストリームや DELETE を前提としている場合の挙動、および dual-era server を経由する古いクライアントの実運用例。
- 本文の明示期限は 2026-12-25（official_docs TTL 90日。再利用した changelog source の取得日 2026-09-26 が拘束）。改訂追加時は Streamable HTTP の Info ボックスと Changelog を再確認する。

---
{
  "id": "mcp-authorization",
  "title": "MCP リモートサーバーの OAuth 2.1 認可フロー（仕様 2026-07-28）",
  "kind": "knowledge",
  "technology": "mcp",
  "version": "MCP specification 2026-07-28 revision",
  "tags": [
    "research-domain:ai-engineering",
    "oauth",
    "authorization",
    "authorization-flow",
    "pkce",
    "discovery",
    "rfc9728",
    "rfc8707",
    "client-registration",
    "scopes",
    "step-up",
    "token-validation",
    "401",
    "resource-server",
    "security"
  ],
  "sources": [
    {
      "id": "mcp-authorization-spec-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/index.md",
      "type": "official_docs"
    },
    {
      "id": "mcp-authorization-discovery-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery.md",
      "type": "official_docs"
    },
    {
      "id": "mcp-authorization-client-registration-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration.md",
      "type": "official_docs"
    },
    {
      "id": "mcp-authorization-security-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations.md",
      "type": "official_docs"
    },
    {
      "id": "mcp-spec-changelog-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/changelog.md",
      "type": "official_docs"
    },
    {
      "id": "mcp-authorization-tutorial-2026-07-28",
      "url": "https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/authorization.md",
      "type": "official_docs"
    },
    {
      "id": "mcp-spec-index-2026-07-28",
      "url": "https://modelcontextprotocol.io/specification/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# MCP リモートサーバーの OAuth 2.1 認可フロー（仕様 2026-07-28）

MCP の認可は HTTP ベース transport の伝送レベルで定義され、リモート MCP サーバー（resource server）を OAuth 2.1 で保護するための discovery・クライアント登録・authorization flow・トークン検証を規定する。[公式 Authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/index.md) / [Authorization Server Discovery](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery.md) / [Client Registration](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/client-registration.md) / [Authorization Security Considerations](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations.md)

## 要点

### 適用範囲（公式 Authorization "Protocol Requirements"）

- 認可は MCP 実装にとって **OPTIONAL**。HTTP ベース transport を使う実装はこの仕様に SHOULD conformance、**STDIO transport は SHOULD NOT で、環境変数から資格情報を取得する**。認可フローは HTTP 用であり、ローカル（STDIO）サーバーは [公式チュートリアル](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/authorization.md) も「環境ベースの資格情報や埋め込み資格情報を使う」と説明する。
- 役割は、MCP server = OAuth 2.1 resource server、MCP client = OAuth 2.1 client、authorization server（AS）はリソース所有者と連携してアクセストークンを発行する。AS は resource server と同居も分離もできる。
- 認可とは別に、[仕様冒頭の Security and Trust & Safety](https://modelcontextprotocol.io/specification/) は「**ホストはツールを起動する前に明示的なユーザー同意を得なければならない**」「tool annotations のようなツール挙動の説明は信頼できるサーバー以外から得たものなら untrusted として扱う」と原則を示す。MCP 自身はこれらをプロトコルレベルで強制できないと明記しており、transport レベルの認可とツール実行の同意は別レイヤーとして設計する。

### discovery（公式 Authorization Server Discovery）

- MCP server は **RFC 9728 Protected Resource Metadata の実装が MUST**。返す文書には `authorization_servers` を1件以上含める。複数の AS が listed されている場合は選択がクライアントの責務で（RFC 9728 §7.6）、**client identifier は AS 単位で独立**。ある AS で有効な資格情報が別 AS でも有効とは仮定しない（MUST NOT）。
- server は (1) 401 応答の `WWW-Authenticate` の `resource_metadata`、(2) well-known URI のどちらかを提供することが MUST。**client は両方を MUST サポート**し、ヘッダがあればそれを使い、なければ MCP エンドポイントのパス用 well-known → ルート用 well-known の順にフォールバックする。
- AS メタデータの discovery 既定 suffix は RFC 8414 の `oauth-authorization-server`（MCP 独自 suffix は無い）。path を持つ issuer（例 `https://auth.example.com/tenant1`）では「path 挿入の oauth-authorization-server → path 挿入の openid-configuration → issuer 直下の openid-configuration」の順、path 無しでは「oauth-authorization-server → openid-configuration」の順に **client が MUST で試す**。取得した文書の `issuer` は URL と完全一致で検証し（RFC 8414 §3.3 / OIDC Discovery §4.3）、不一致は使用禁止（`attacker.example` の文書が `honest.example` の issuer を名乗る例を MUST reject と明記）。

### クライアント登録（公式 Client Registration）

3方式があり、すべてに対応する client は次の優先順位に従うことを SHOULD とされる。

1. 事前登録済み client 情報があればそれを使う
2. AS が `client_id_metadata_document_supported` を出力していれば **Client ID Metadata Documents (CIMD)** を使う
3. AS が `registration_endpoint` を出力していれば **Dynamic Client Registration (DCR)** をフォールバックとして使う
4. それ以外はユーザーに入力を促す

- CIMD は HTTPS URL を `client_id` にする（パス構成必須、例 `https://example.com/client.json`）。文書には `client_id`・`client_name`・`redirect_uris` が必須で、`client_id` は文書 URL と完全一致させる。AS 側は redirect URI を文書と照合し、HTTP キャッシュヘッダに従ってキャッシュを SHOULD。
- **DCR（RFC 7591）は 2026-07-28 改訂で deprecated**（[changelog](https://modelcontextprotocol.io/specification/2026-07-28/changelog.md)、PR #2858）。新規実装は CIMD を使うこと。DCR を使う場合は OIDC の redirect URI 制約に備えて `application_type` を native / web から選ぶことが MUST。
- **資格情報は発行した AS の `issuer` でキー付けが MUST**。AS が protected resource metadata の変更で変わったら再登録が MUST、異なる AS への流用禁止。CIMD は自分の HTTPS URL を AS が都度取得する方式なので AS をまたいで portable。

### authorization flow の手順（公式 Authorization "Authorization Flow Steps"）

1. トークン無しの MCP リクエスト → server は **401 + `WWW-Authenticate`**（`resource_metadata`、必要なら `scope`）
2. Protected Resource Metadata を取得し、AS を選択
3. AS metadata discovery（上記の順序）で `issuer` を検証し、**検証済み issuer を PKCE code verifier と同じ per-request 記録に保存**
4. client registration（CIMD / pre-registration / DCR）
5. authorization request に **PKCE・`resource`・scope** を載せてブラウザを開く
6. callback の `iss` を検証してから token request（`code_verifier` + `resource`）
7. `Authorization: Bearer` で MCP リクエスト

- **PKCE は MUST**（OAuth 2.1 §7.5.2）、技術的に可能なら `S256` code challenge method が MUST。PKCE 対応確認は metadata の `code_challenge_methods_supported` で行い、**無ければ client は進めない（MUST refuse）**。OIDC Discovery 経由でも同フィールドの存在を MUST で検証し、AS 側は OIDC Discovery でこのフィールドを出さなければならない。
- **`resource`（RFC 8707 Resource Indicators）は authorization request と token request の両方が MUST**、値は対象 MCP サーバーの canonical URI（`https://mcp.example.com/mcp` のような絶対 URI、fragment 無し、trailing slash は原則付けない）。**AS がサポートしていても無くても送る**。server 側は受け取ったトークンが自分向けに発行された audience であることを MUST で検証する。
- **scope 選択**: 初回は 401 の `WWW-Authenticate` の `scope` を最優先、無ければ Protected Resource Metadata の `scopes_supported` を全部使う（`scopes_supported` が未定義なら `scope` パラメータ自体を省略）。challenge の scope 集合と `scopes_supported` の包含関係は **MUST NOT で仮定しない**。challenge が当該操作の権威であり、再認可時は challenge の scope をこれまで要求した scope に加えて送る。
- **`iss` 検証（RFC 9207、SEP-2468）**: AS は `iss` を SHOULD で返し、`authorization_response_iss_parameter_supported` が `true` なら metadata でその旨を出力する。client は token endpoint へ認可コードを渡す前に RFC 9207 §2.4 の検証が MUST。metadata が `true` で `iss` 欠落なら破棄、`iss` があれば記録済み issuer と**文字列比較で照合**（scheme/host の大小文字、デフォルトポート省略、trailing slash、percent-encoding の正規化は禁止）。不一致ならエラー応答の `error` 表示もしてはならない。将来の改訂で AS 側の `iss` は SHOULD → MUST に上げる予定と本文に明記されている。

### トークン利用・エラー（公式 Authorization "Access Token Usage" / "Error Handling"）

- アクセストークンは **全 HTTP リクエストの `Authorization: Bearer` ヘッダで送る（MUST）**、URI クエリ文字列への付与は禁止。
- server は OAuth 2.1 §5.2 に従って検証し、**自分向けに発行された audience を検証（MUST）**。無効・期限切れは 401。他資源向けトークンや他社トークンの受け取り・中継（token passthrough）は禁止で、上流 API へは MCP server 自身が OAuth client として別トークンを取得する。
- エラーは **401 = 認証要求または無効トークン、403 = scope 不足・権限不足、400 = 不正な認可リクエスト**。
- 実行中の scope 不足は 403 + `WWW-Authenticate`（`error="insufficient_scope"`、必要最小限の `scope`、`resource_metadata`）で返す。server は1回の challenge で当該操作に必要な scope をすべて提示し、逐次1つずつ要求しないことを SHOULD。
- **step-up authorization flow**: client は「これまで要求した scope の集合 ∪ challenge の scope」で再認可し、元リクエストを再試行する。**再試行は数回まで**で、超えたら恒久的な認可失敗として扱う（SHOULD でリトライ上限と upgrade 回数の追跡を要求）。user 代理の client は step-up を SHOULD、`client_credentials` の client は中断も可。
- **refresh token**: client は機密保持が MUST、`grant_types` に `refresh_token` を入れるのは SHOULD、発行は AS の裁量なので **MUST NOT で前提にしない**。MCP server（resource）は `WWW-Authenticate` や `scopes_supported` に `offline_access` を入れない SHOULD。

### セキュリティ要件（公式 Authorization Security Considerations）

- AS の全エンドポイントは HTTPS が MUST、redirect URI は `localhost` か HTTPS が MUST。client は登録済み redirect URI を使い、`state` の検証と不一致破棄を SHOULD。
- **mix-up attack** は記録済み issuer に対する `iss` 検証で防ぐ。**open redirect** は redirect URI の完全一致検証で防ぐ。
- **confused deputy**: 静的 client_id を使う MCP プロキシは、動的登録クライアントごとに**第三者 AS へ回す前のユーザー同意が MUST**。
- CIMD は AS が metadata document を fetch するため **SSRF リスク**を考慮し、`localhost` の redirect URI によるなりすましに備えて hostname を明示表示する。domain ベースの trust policy は AS 側で任意導入。
- token theft 対策として、AS は短命アクセストークンを SHOULD、public client の refresh token rotation は MUST。

### 版の差（公式 Key Changes 2026-07-28）

- 公開済み最新リビジョンは **2026-07-28**（`draft` は別扱いで、本文は draft を根拠にしない）。
- 2025-11-25 まで単一ページだった `basic/authorization` は、2026-07-28 で index / discovery / client-registration / security-considerations の4ページに分割された。
- 認可まわりの改訂: `iss` 検証の必須化（SEP-2468）、**DCR の deprecated 化**（PR #2858）、資格情報の AS 紐付け（SEP-2352）、DCR 時の `application_type` 必須（SEP-837）。
- 認可以外だが波及する大型変更: `initialize` ハンドシェイクとプロトコルレベル session（`Mcp-Session-Id`）の除去による stateless 化（SEP-2575）、HTTP GET エンドポイントと SSE 再送の廃止、ping / logging の削除、Roots・Sampling・Logging の deprecated 化（SEP-2577）。

### 観察: 公式チュートリアルと改訂の不整合（未解決）

[docs/2026-07-28 のチュートリアル](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/authorization.md) の TypeScript 実装例は `Mcp-Session-Id` ヘッダと `isInitializeRequest` によるセッション管理を含むが、同じ 2026-07-28 改訂の changelog は Streamable HTTP からのプロトコルレベル session（`Mcp-Session-Id`）の除去を明記している。**サンプルの適用リビジョンは未確認**で、そのままコピーすると現行改訂と食い違う可能性がある。一方で認可部分の手順（401 → PRM → トークン検証）自体は仕様の要件と矛盾しない（introspection による検証は公式チュートリアルが示す一例で、検証方式そのものは仕様が定めない）。

## 推奨方法

以下は上記の公式要件からの設計上のまとめであり、公式が定める実装構成そのものではない。

- リモート MCP サーバーを保護するなら、RFC 9728 の Protected Resource Metadata を公開し、401 + `WWW-Authenticate: Bearer resource_metadata=...` から始める（チュートリアルの pitfall にも「401 で `resource_metadata` を返す」が挙がる）。
- client 側は登録を「pre-registration → CIMD → DCR → 手動入力」の順に実装し、PKCE `S256` と `resource` を常に送る。DCR は既存 AS 対応のための後方互換経路として扱う。
- issuer は「検証済み AS metadata から記録した値」として比較対象を作る。取得元が未検証の issuer は `iss` 検証の前提を壊すため使わない。
- scope は challenge の集合を当該操作の正とし、step-up は「既存 scope ∪ challenge scope」+ 再試行上限で実装する。
- トークン検証は既製ライブラリに委ね、自作しない（チュートリアル pitfall の筆頭）。検証方式は introspection または JWKS のいずれかを選び、audience が自分の resource を含むかを必ず確認する。
- access token は短く、保存は暗号化し、`Authorization` ヘッダ・トークン・コードをログに出さない。application（client）の資格情報と resource server の資格情報を分ける。
- STDIO のローカルサーバーには OAuth フローを組まず、環境変数または埋め込み資格情報を使う（仕様が SHOULD NOT と示す範囲）。

## 避ける使い方

- **DCR を新規実装の既定にする**。2026-07-28 で deprecated。無認可 DCR を開け放つと誰でも client を登録できる。
- **audience 検証を省いて他リソース向けトークンを受け取る・中継する**（token passthrough）。仕様は server に audience 検証を MUST とし、禁止を明記。
- **`iss` を正規化してから比較する**（trailing slash 除去、ホスト小文字化、デフォルトポート省略など）。比較前の正規化は仕様が禁止する。
- **metadata の `issuer` と取得 URL が不一致のまま AS metadata を使う**。mix-up / 攻撃経路として MUST reject とされる。
- **`code_challenge_methods_supported` を確認せずに認可を進める**。PKCE 未確認で進めることは MUST refuse とされる。
- **scope を `scopes_supported` に固定して challenge を無視する**。包含関係は仮定できないと MUST NOT で明記されている。
- **アクセストークンをクエリ文字列に付ける**。仕様が禁止。
- **1つの client credentials を複数の AS で使い回す**。AS 変更時は再登録が MUST。
- **step-up を無制限にリトライする**。数回で恒久的失敗として扱うのが公式の指示。
- **プロキシで静的 client_id を使い、動的登録クライアントへの同意を省く**。confused deputy の MUST 違反。
- **チュートリアルのセッションコードをそのまま現行リビジョン用としてコピーする**。上記の不整合が未確認のため。

## 適用版と本番での注意

- `MCP specification 2026-07-28 revision` を modelcontextprotocol.io の公開ページ（2026-09-26 取得）で確認した。**draft リビジョンも同時に公開されており、draft の内容は本ドキュメントの根拠に使っていない。**将来の最新とは扱わない。
- 旧リビジョン（2025-11-25 / 2025-06-18 / 2025-03-26 / 2024-11-05）はページ構成も要件も異なる。2025-11-25 の認可ページ（2026-09-26 に再確認）では DCR は deprecated ではなく `MAY` の後方互換手段で、`iss` 検証・Refresh Tokens 節・step-up の scope 和集合要件・client 資格情報の AS 紐付け規定は無い（登録方式の優先順序の4段階だけは同じ形）。旧版向け SDK と混在させない。
- 本文は `expires_at` 2026-12-25（official_docs TTL 90日）。認可周りは改訂が速く、`iss` の SHOULD → MUST 升格が予告されているため、リビジョン追加時は再確認する。
- **未確認**: 各公式 SDK（TypeScript / Python / C# / Go）の CIMD・`iss` 検証・`resource` 送信の対応状況。実装前に対応表を公式 SDK ドキュメントで確認する必要がある。
- **未確認**: 本ドキュメントが触れていない Tasks / MCP Apps などの extension が認可 scope とどう相互作用するか（extensions は opt-in で独立ネゴシエーション）。
- 本文中の MUST / SHOULD は 2026-07-28 改訂の normative 表記をそのまま引用したもので、本リポジトリの推奨事項（「推奨方法」節）と区別している。

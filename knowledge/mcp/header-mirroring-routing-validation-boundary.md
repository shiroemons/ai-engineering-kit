---
{
  "id": "mcp-header-mirroring-routing-validation-boundary",
  "title": "MCP HTTP headers: x-mcp-header の抽出・符号化・body照合と routing の境界",
  "kind": "knowledge",
  "technology": "mcp",
  "version": "MCP specification 2026-07-28; SEP-2243 Final (created 2026-02-04) is historical; official pages verified 2026-10-03 UTC and checked against repository commit 3098fe94caa1b9e0afaaa6d30e040b61d5802471; SDK/runtime untested",
  "tags": [
    "research-domain:ai-engineering",
    "mcp",
    "streamable-http",
    "x-mcp-header",
    "Mcp-Name",
    "HeaderMismatch",
    "header-mirroring",
    "gateway",
    "routing",
    "base64"
  ],
  "sources": [
    {
      "id": "mcp-header-transport-20260728-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http",
      "type": "official_docs"
    },
    {
      "id": "mcp-header-tools-20260728-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/server/tools",
      "type": "official_docs"
    },
    {
      "id": "mcp-header-base-20260728-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/basic",
      "type": "official_docs"
    },
    {
      "id": "mcp-header-changelog-20260728-20261003",
      "url": "https://modelcontextprotocol.io/specification/2026-07-28/changelog",
      "type": "official_docs"
    },
    {
      "id": "mcp-header-sep2243-history-20261003",
      "url": "https://modelcontextprotocol.io/seps/2243-http-standardization",
      "type": "official_docs"
    },
    {
      "id": "mcp-header-repository-3098fe9-20261003",
      "url": "https://github.com/modelcontextprotocol/modelcontextprotocol/tree/3098fe94caa1b9e0afaaa6d30e040b61d5802471",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/ai-engineering.json"]
}
---

# MCP HTTP headers の routing と検証境界

## 問いと採用判断

MCP gateway が tenant・region・tool 名を HTTP header だけで見て振り分けるとき、実行する JSON-RPC body と同じ対象を見ていると、どこで確認するか。2026-07-28 改訂の `x-mcp-header` はその照合契約を持つが、header と body の一致だけで利用者の権限は証明されない。

本書は7月改訂の未収録の移行課題を扱う。10月に新しく導入された機能という主張ではない。[session 廃止](streamable-http-session-resumability.md)、[JSON Schema の一般的検証](tool-schema-json-value-ref-validation.md)、[result cache](result-cache-authorization-mrtr-invalidation.md) は既存文書を参照し、ここでは schema から header を作り、gateway と origin で意味を揃える経路に絞る。「確認した契約」は一次資料、「設計提案」「試験案」は独自の整理である。

採用判断: header routing を有効にする前に、同じ call から header と body を構築し、body を処理する側で照合し、認可は認証済み主体から別に判定する。schema が不正なら個別 tool の公開を止める。通信失敗を理由に照合なしの legacy 経路へ流す設計は避ける。

## 確認した契約: JSON Schema と header mapping は別の適合条件

[Tools の x-mcp-header 節][tools]では、server が annotation を付けるかは任意だが、Streamable HTTP client は対応しなければならない。annotation の値は `Mcp-Param-` の後ろに付く名前であり、空でない HTTP field-name token、制御文字なし、同じ inputSchema 内で大文字小文字を無視して一意であることが必要になる。

- 対象型は string・integer・boolean。`type: number` は不可で、integer の範囲は −9007199254740991〜9007199254740991
- root から `properties` だけをたどれる静的な property path に限る。nested object は可能だが、経路中の `items`、composition、条件分岐、`$ref` は不可
- annotation 制約に違反した tool は、HTTP client が `tools/list` の結果から除外する MUST。警告ログは SHOULD。他の正常な tool まで一括して使えなくする要件ではない
- stdio など他transportは annotation を無視してよい MAY。秘密・PII などを mirrored parameter に指定しないことは server 作者への SHOULD NOT

したがって、通常の JSON Schema validator が schema を受理しても header mapping の適合を証明しない。`allOf` や `$ref` が inputSchema 全体で禁止されたわけでもない。**禁止対象は annotation までの抽出経路**である。型・dialect の検証と、その経路の検証を分ける必要がある。[Tools][tools] / [Transport][transport]

## 確認した契約: header の有無と値の表現

以下は [Streamable HTTP の Request Metadata][transport] が規定する modern request の対応である。

| header | body の対応先 | 必須範囲 |
|---|---|---|
| `MCP-Protocol-Version` | `params._meta` 内の `io.modelcontextprotocol/protocolVersion` | modern request の POST |
| `Mcp-Method` | `method` | 全 request |
| `Mcp-Name` | `params.name` または `params.uri` | tools/call・prompts/get・resources/read の request |
| `Mcp-Param-{Name}` | annotation の exact property path にある call 引数 | 対象値があり、null でないとき |

`tools/list` にまで Mcp-Name を必須化しない。custom header は、値が欠落または null なら省略する。これは **null を inputSchema に適合させる規則ではない**。null の入力妥当性は別に検証する。false・0 を欠落と同じ扱いにする実装も不適切である。

値は string をそのまま、integer を10進表記、boolean を小文字の true/false に変換する。非ASCII・制御文字・前後空白など、安全な plain header にできない値は UTF-8 を Base64 化して `=?base64?...?=` で囲む。Mcp-Name も同じ規則で、元の ASCII 文字列自体がこの sentinel に一致するときも符号化する。比較前に復号し、header 名だけを case-insensitive に扱う。値まで一括で小文字化しない。integer の比較は数値として行う SHOULD であり、例えば 42.0 と 42 の字面の差だけで拒否しない。[Transport][transport]

設計上の含意: schema の default や別階層の同名propertyから値を補って header だけを生成せず、送信する実際の引数を確定してから exact path を読む。Base64 は暗号化ではないため、header ログへの露出を減らす対策にはならない。

## 確認した契約: エラーと歴史資料を読み分ける

[Base Protocol][base]では body の必須 `_meta` 欠落は HTTP 400 と `-32602`、未宣言の必要 capability は 400 と `-32021` になる。[Transport][transport]では、header の欠落・不正・body 不一致を `400 / -32020 HeaderMismatch` と区別する。未知の RPC method は `404 / -32601`、未対応protocol versionは `400 / -32022` である。複数の不備が同時にある場合の検証優先順は、この調査では確認していない。

body を処理する server は復号後の一致を検証する MUST。body を解析する intermediary もこの責務を負う。intermediary の拒否には適切な HTTP error が必要だが、JSON-RPC error body は必須ではない。認識しない Mcp-Param header を単に中継する intermediary は転送してそれ以外は無視する。header に基づく policy を適用する intermediary は、照合を要求するprotocol versionか確認し、旧版・version欠落を信用せず拒否する SHOULD である。[Transport][transport]

[SEP-2243][sep] は Final でも**履歴資料**である。現行仕様との差を埋めずにサンプルを移植しない。

| SEP に残る記述 | 2026-07-28 の実装で採る根拠 |
|---|---|
| HeaderMismatch の旧値 `-32001` | changelog・Base Protocol の `-32020`。SEP末尾も再割当を注記 |
| notification も Mcp-Method が必要、initialized の例 | 現行HTTP coreにclient→server notificationはなく、notification POST のheader要件は未定義 |
| schema を未取得なら custom header なしで送る注記 | 現行 Client Behavior は欠落を非適合とし、HeaderMismatch 後の tools/list 再取得・訂正再送を SHOULD とする |
| annotation を任意深さへ置く説明 | 現行 Tools/Transport の properties-only 制約で具体的な経路を判定 |

notification POST を受け付けた場合の 202/no body という輸送上の規則と、現行coreがその送信を定義するかは別である。SSEで届くserver通知に、request用のMcp-Methodを付ける要件を作り出さない。現行HTTPの取消はSSE response streamのcloseであり、stdioの `notifications/cancelled` をHTTP POSTへ機械的に転用しない。[Transport][transport] / [SEP][sep] / [Changelog][change]

## 設計提案: 不一致の検出と認可を順に組み立てる

以下は特定SDKの保証ではなく、本書の配備案である。

1. **catalog取込み時**: toolのschema検証に加え、annotationの型・名前・経路を検査する。正常toolの一覧と除外理由を分け、除外したtoolをLLMへ提示しない。該当schemaの識別子と取得時点をcallの診断情報へ結び付ける
2. **call構築時**: 引数を確定し、同じ値からbodyとheaderを生成する。annotationを解決するためだけにcompositionを展開したりnetworkの `$ref` を取得したりしない。引数を書き換えるmiddlewareがあるなら、照合対象も同じ最終値に揃える
3. **gateway通過時**: clientが主張したtenantはrouting用の入力と扱う。headerとbodyが両方tenant Bを指していても、主体AがBを操作できる証拠にはならない。認証済み主体に対するtenant許可集合・tool権限は別の認可処理で確認する。credentialをmirrored parameterにして認可の代わりにしない
4. **origin実行前**: 復号・型比較・input validation・認可を完了してから副作用へ進む。照合なしで実行した後にログだけを出しても、誤ったroutingや権限行使を防げない
5. **拒否後**: Mcp-Paramの不一致ならschemaを再取得し、どのmappingが変わったかを確認する。訂正できない状態で同じcallを無限再送しない。一般的な切断や502を「未実行」と同一視せず、外部操作の重複防止は既存の[冪等性設計](../http/retry-idempotency.md)で別に扱う

gateway がJSON-RPCを返さない400は、transportのera判定でlegacy fallbackの候補になり得る。しかしheader policyを必要とする配備で、そのfallbackを無条件に許すと同じ安全性を維持できない。legacyを許すなら、許可する経路とbody側の認可・検証を明示的に設計する。上記はfallbackの禁止をMCP全体のMUSTとして追加する主張ではない。

運用ログは原因分類・protocol version・schema識別子・相関IDを中心にする。tenant値や復号後の引数を無制限に記録する必要はない。header長と件数には配備ごとの予算を設ける。重複field-lineをlibraryが結合する前の扱いも決め、gatewayとoriginで最初／最後の値を異なって採用しないよう検証する。**具体的な上限値、重複headerの採用順、製品ごとの正規化は本書で確認したMCP契約ではない**。

## 受入試験案

以下は実行済みテストではなく、client・gateway・originを同時に検査するための独自ケースである。

| 入力・障害 | 確認する境界 |
|---|---|
| nested object の properties-only と、array items 内のannotation | 前者を抽出でき、後者はtool単位で除外できる |
| 異なるpropertyが Region / REGION を指定 | wire送信前にcase-insensitiveな名前衝突を検出 |
| type:number、integer上限超過 | 汎用schemaの受理だけでmirroringを許可しない |
| 欠落・null・false・0・空文字 | 欠落/nullの省略、false/0の送出、空文字の存在判定を分ける |
| 日本語・前後空白・CR/LF・sentinelに見えるASCII | trimや二重復号で値を変えず、元のbodyと比較 |
| headerのtool名とbodyのtool名が異なる | toolに到達する前に拒否し、originなら400/-32020 |
| schema変更後にcustom headerが欠落 | tools/list再取得でmappingを訂正し、無限retryを避ける |
| body必須meta欠落、必要capability欠落 | HeaderMismatchと混同せず、-32602/-32021を分類 |
| gatewayがJSON-RPCなしで拒否 | legacyへ自動的に迂回せず、配備方針で処理 |
| tenantのheader/bodyは一致するが主体には権限がない | 整合性検査通過後も認可で拒否 |
| HTTPでnotificationを送ろうとするadapter | SEPの古い例から現行header要件を推測しない |
| 同名headerを複数行で送る、proxyがheaderを欠落させる | 実際のHTTP stack全経路で曖昧さを検出 |

検索evalはこの文書の発見可能性の検査である。上のruntime試験、安全性、SDK適合性を実証するものではない。

## 出典・provenance・限界

- 一次仕様4ページとSEPを2026-10-03 UTCにnative webで開いた。版識別子は2026-07-28で、各仕様ページに独立した公開・更新日は表示されない。SEPの作成日は2026-02-04、状態はFinal。現在の規範は版付き仕様を優先する
- 固定[repository tree][repo]は `3098fe94caa1b9e0afaaa6d30e040b61d5802471`、commit日時は2026-10-01T12:55:32Z。公式connectorで同SHAの `docs/specification/2026-07-28/basic/transports/streamable-http.mdx`、`server/tools.mdx`、`basic/index.mdx`、`seps/2243-http-standardization.md`、LICENSEを読み、上記の差異を照合した。これは仕様文書の来歴確認であり、SDK実装解析ではない
- [固定LICENSE][license]は、新規仕様・codeがApache-2.0、未再許諾の過去のcontributionはMITに残る移行を明記する。CC-BY-4.0は仕様を除くdocumentationの区分である。本文は原文code・sampleの転載を行わない独自要約と設計案で、module昇格はない
- SDKの対応版、実serverのerror優先順、browser CORS、gatewayのheaderサイズ制限・重複処理・Base64 decoderの不正入力処理は未検証。HTTP stackで実測しないまま製品横断の互換性を保証しない
- 明示期限は取得日から90日後の2027-01-01。protocol改訂・SDK更新・gateway変更が先に起きた場合は、期限前でも再照合する

[transport]: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http
[tools]: https://modelcontextprotocol.io/specification/2026-07-28/server/tools
[base]: https://modelcontextprotocol.io/specification/2026-07-28/basic
[change]: https://modelcontextprotocol.io/specification/2026-07-28/changelog
[sep]: https://modelcontextprotocol.io/seps/2243-http-standardization
[repo]: https://github.com/modelcontextprotocol/modelcontextprotocol/tree/3098fe94caa1b9e0afaaa6d30e040b61d5802471
[license]: https://github.com/modelcontextprotocol/modelcontextprotocol/blob/3098fe94caa1b9e0afaaa6d30e040b61d5802471/LICENSE

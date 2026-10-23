---
{
  "id": "security-chips-partition-session-logout-boundary",
  "title": "CHIPS Partitioned cookies: site・ancestor・session失効とlogoutの境界",
  "kind": "knowledge",
  "technology": "security",
  "version": "Chrome CHIPS / cookies API docs verified 2026-10-23 UTC; HTML LS 2026-10-22; RFC6265bis draft-22 (2025-12-01); CHIPS draft-01 (2022-11-10) historical only; runtime not tested",
  "tags": [
    "research-domain:security",
    "CHIPS",
    "Partitioned",
    "cookie",
    "partitionKey",
    "hasCrossSiteAncestor",
    "SameSite",
    "__Host-",
    "session",
    "logout",
    "iframe"
  ],
  "sources": [
    {
      "id": "chrome-chips-partition-session-20261023",
      "url": "https://privacysandbox.google.com/cookies/chips",
      "type": "official_docs"
    },
    {
      "id": "chrome-cookie-partition-api-20261023",
      "url": "https://developer.chrome.com/docs/extensions/reference/api/cookies",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-cookie-site-origin-20261023",
      "url": "https://html.spec.whatwg.org/multipage/browsers.html",
      "type": "official_docs"
    },
    {
      "id": "ietf-cookie6265bis22-session-identity-20261023",
      "url": "https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-rfc6265bis-22",
      "type": "official_docs"
    },
    {
      "id": "chrome-cookie-attributes-embed-20261023",
      "url": "https://privacysandbox.google.com/cookies/basics/cookie-attributes?hl=en",
      "type": "official_docs"
    },
    {
      "id": "chrome-storage-partition-logout-20261023",
      "url": "https://privacysandbox.google.com/cookies/storage-partitioning",
      "type": "official_docs"
    },
    {
      "id": "ietf-chips01-historical-status-20261023",
      "url": "https://datatracker.ietf.org/doc/html/draft-cutler-httpbis-partitioned-cookies-01",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# CHIPS: cookie の格納区画と、アプリの session 境界を分ける

## 問いと採用判断

第三者 iframe の session cookie に `Partitioned` を追加すれば、埋め込み先ごとの認証・tenant 分離・logout が完成するか。答えは否である。CHIPS は browser 内の cookie の利用文脈を狭めるが、server 上の session の意味や失効対象を決めない。

本稿の採用判断は、**同じ埋め込み先でのみ継続すればよい状態に CHIPS を使い、partition の実効値と server session の終了を別々に試験する**、である。複数 site に共通のログイン cookie が必要な機能を、そのまま置換しない。

既存の [BFF](../oauth/bff-cookie-csrf-proxy-boundary.md) は token 保管と API CSRF、[COOP/COEP](coop-coep-credentialless-deployment-boundary.md) は読み込み・browsing context の隔離、[OIDC logout](../oidc/rpinitiated-frontchannel-backchannel-logout.md) は logout protocol を扱う。本稿は未収録の cookie partition identity と session lifecycle を対象にし、それらを再説明しない。調査前の index/search では `CHIPS` が0件、`Partitioned` は cookie 以外の2件だった。

## 一次資料で確認した境界

### 1. top-level site は origin や tenant ID ではない

[Chrome CHIPS 文書][chips]によれば、partitioned cookie は通常の host 側の範囲に加え、設定時の top-level site に結び付く。第三者 C を A 内で使って設定した cookie は、別 site B 内の C にも、C 自身を top-level として訪れた要求にも、そのまま送られない。新しい tab に C を開けば A 内の session が自動的に続く、という設計は成立しない。

[HTML §7.1.1–7.1.1.1][html]では、origin の同一性は scheme・host・port を比較する。一方、通常の site は scheme と registrable domain から作る。registrable domain がない host はその host を使う。したがって次の区別が必要になる。

| 比較する配置 | site と origin の関係 | 本稿の設計上の帰結 |
|---|---|---|
| `https://tenant-a.example.com` と `https://tenant-b.example.com` | 異なる origin でも同じ site になり得る | top-level subdomain 別というだけで tenant 分離を保証しない |
| `https://example.com:443` と `https://example.com:8443` | port が違えば origin は別だが site は同じ | port 別アプリの認証隔離を CHIPS に任せない |
| `http://example.com` と `https://example.com` | scheme が異なり site も別 | 開発用 HTTP の結果を HTTPS 本番の共有範囲へ一般化しない |
| A に埋め込まれた C と、B に埋め込まれた C | C の origin が同じでも top-level site は別 | 同じ endpoint だから同じ cookie が来るとは限らない |

表は定義から導いた判断例である。実際の送信には cookie 自身の host/path、他の属性、browser policy も関係する。「site が同じ」は送信の十分条件ではない。

### 2. Chrome の partition key は top-level URL 文字列だけで復元しない

[Chrome cookies API][chrome-api]の `CookiePartitionKey` は `topLevelSite` と `hasCrossSiteAncestor` を持つ。後者は Chrome 130+ の API field として記載される。これは field の利用可能版であり、CHIPS 全体や underlying 機能の導入版を意味しない。

[Storage Partitioning 文書][storage]は、A1 → B → A2 の nested iframe で、A1/A2 が同じ site でも途中の cross-site B を ancestor bit に反映する理由を説明する。同じ top-level site だけで first-party 文脈と同一視しない。この説明と cookie API の現行 field を照合し、診断時は frame chain も保存する。

拡張機能で cookie を検査する場合、cookies API は既定で unpartitioned cookie を対象にし、`partitionKey` を渡して対象を指定する。Chrome 132+ の `getPartitionKey` は frame の key を返す。拡張用 API を通常の web page から使えると説明したり、`partitionKey` 省略時の空結果を「cookie なし」と断定したりしない。通常/Incognito の `storeId` も別の軸である。[Chrome cookies API][chrome-api]

### 3. Partitioned・SameSite・__Host-・HttpOnly は別の制約

[Cookie attributes][attributes]が扱う `SameSite` は cross-site 送信条件、`HttpOnly` は script からの cookie 読み取り制限であり、partition の選択とは異なる。cross-site iframe 用には `SameSite=None` が必要な場合があるが、それ自体は partitioning を有効化しない。CHIPS は `Secure` を要求する。[CHIPS security design][chips]

基礎 cookie 仕様の [RFC6265bis draft-22 §4.1.3.2][cookies]では、`__Host-` に `Secure`、`Path=/`、`Domain` なしを要求する。これは cookie を特定 host に縛るが、port は区別しない。prefix が top-level site を origin 単位へ細分化するわけではない。

以下は server が不透明な session ID を発行する場合の独自の設定例であり、ブラウザ実行済みのコードではない。値は説明用で、credential として利用しない。

```http
Set-Cookie: __Host-embed_session=opaque-example; Path=/; Secure; HttpOnly; SameSite=None; Partitioned
```

この組合せでも、許可された文脈内の XSS による認証済み操作、server 側の tenant 認可、session replay を自動的に解決しない。BFF 用の `SameSite=Strict` 方針を全 cookie で一括して `None` に緩和せず、埋め込み専用 session と用途を分けるのが本稿の提案である。

### 4. 対応していない browser では安全側に失敗するとは限らない

[Chrome CHIPS 文書][chips]は、`Partitioned` 非対応でも third-party cookies が許可されていれば、`SameSite=None` の cookie が third-party 文脈へ送られ得ると明記する。したがって「設定できた」「同じ iframe で値を読めた」は partition 分離の確認にならない。

本稿の運用判断は、分離が必須な認証機能では unsupported fallback を暗黙に許可しないこと、である。対象 browser / version / mode ごとに A と B の双方を使って非共有を確認し、保証できない環境では認証付き embed を提供しない等の代替 UX を定義する。単なる機能検出をするために、実在ユーザーの ID を別 site へ渡さない。

### 5. server は Cookie header だけから partition を認証できない

[RFC6265bis draft-22 §4.2.2][cookies]の `Cookie` header は name/value を返し、設定時の属性を返さない。同名 cookie の並び順にも依存すべきでない。partition key が返却される追加の認証情報と考えない。

歴史的な [CHIPS draft-01 §2.4][historical]も、partition が違えば同じ name/domain/path を持つ cookie が共存できる設計を提案していた。移行中に partitioned/unpartitioned の同名 cookie が同時に存在する可能性は、server の parser と移行方針で扱う。先頭または末尾を無条件に採用する実装を安全性の根拠にしない。

本稿の設計提案は、旧方式と新方式を必要に応じて異なる cookie 名で区別し、曖昧な session 候補を拒否し、server が検証済みの tenant と session の tenant を照合すること、である。cookie 名を変えるだけでは browser の対応証明にならない。また、異なる partition に意図的に同じ credential を配ったり、backend が同じ ID へ無条件に結び付けたりすれば、アプリの認可・相関は browser の区画だけでは制限できない。

## Logout は三つの終了を区別する

以下は確認した契約から組み立てた独自の運用設計である。

1. **現在の embed を終了する。** 対象 session を server で無効化したうえで、その cookie を設定した埋め込み文脈から削除応答を返す。基礎仕様では削除時に path/domain を合わせる必要がある。[RFC6265bis §3.1][cookies] さらに CHIPS の partition を同じものとして扱うため、同じ属性・対応する top-level/ancestor 文脈を再現する。`Partitioned` を落とした削除や、C の独立 tab だけでの削除が A 内の cookie を除いたとは扱わない。
2. **他の site 内の embed を終了する。** A からの cookie 削除を B へ波及させる前提を置かない。[Chrome Storage Partitioning][storage]の `Clear-Site-Data` の説明も、一つの partition 内の消去としている。これを全 partition・全端末の logout API にしない。現在の browser/version で各 data type の結果を検証する。
3. **アカウント全体を終了する。** どの server session を対象にするかを先に定義し、対象 session 群の失効を server 側で行う。cookie が端末に残っていても、それで認証済み操作を再開できないことを終了条件にする。永続 cookie の期限や browser session の終了だけを頼らない。

例えば logout hub H に C の iframe を描いても、H 内の partition は A 内の C の session cookie と同じとは限らない。これは CHIPS の文脈分割からの帰結であり、iframe の HTTP 200 は A 内の logout 成功を証明しない。OIDC の front-channel/back-channel の protocol 選択・検証は既存文書を参照し、CHIPS で補えたとみなさない。

Chrome の storage partitioning は `BroadcastChannel` 等の通信にも影響する。[Storage Partitioning][storage] 同一 origin の全 embed に broadcast すれば logout が届く、とする実装は避ける。UI 通知が届くことと server の失効は別である。logout と並行して開始済みの request が古い session を再発行する場合も、失効後の credential が権限を復活させないことを検証する。

## 配備前の試験計画（未実行）

次のケースは browser integration test の提案であり、末尾の検索 eval の合格で代替しない。各試験では browser の正確な版、profile/mode、third-party cookie policy、top-level site、全 frame chain、cookie の partition 情報、実 request と server の session 判定を照合する。session 値自体はログへ保存しない。

| ケース | 操作 | 合否として確かめるもの |
|---|---|---|
| cross-site isolation | A 内の C で設定し、B 内の C と C の独立 tab を開く | A 専用の session が他の文脈へ流用されない |
| same-site tenants | sibling subdomain A1/A2 から同じ C を使う | browser 分割を tenant 認可に代用せず、server が不適切な tenant の操作を拒否する |
| ancestor bit | A → A と A → B → A を比較する | top-level site が同じでも key/送信が同じとは決めつけない |
| unsupported fallback | 非対応環境と third-party cookies 許可状態を含める | cookie の保存成功を隔離成功にせず、必要な代替 UX へ進む |
| duplicate migration | 旧 cookie と新 cookie を共存させる | parser の順番で別 session を採用しない |
| local logout | A 内で終了後、A を再訪し、B も再訪する | A の session 失効と B の扱いが製品仕様どおりである |
| global logout | 複数 partition の session を作って全体終了する | 残存 cookie 付きの要求も server で拒否される |
| logout race | 終了前に開始した応答を終了後に返す | 遅れて届く cookie で失効済み session が復活しない |

## 適用版・出典の位置付け・未確認事項

- 2026-10-23 UTC に一次資料の実ページを開いた。CHIPS の Chrome 114+ 対応は公式説明の記載であり、今回の実機検証ではない。Firefox・Safari・WebView の版別動作、private mode、企業 policy、拡張機能による例外を横断検証していない。全 browser で同じ安全性とは主張しない。
- CHIPS の個人提出 draft-01 は 2022-11-10 公開、2023-05-14 失効の archived Internet-Draft で、IETF が承認した標準ではない。新たに取得した日を付けても現行標準にはならない。歴史的な identity / clearing の意図を補足するためだけに参照した。
- RFC6265bis は固定した draft-22 を読む。公開日は 2025-12-01、本文の expiry は 2026-06-04 だが、確認時の Datatracker は active / RFC Ed Queue と表示した。公開済み RFC とも、2026-10-23 に新規公開された版とも扱わない。HTTPWG の日付付き latest preview は固定 draft と区別した。
- HTML は 2026-10-22 表示の Living Standard。Google の各文書の footer 日付は catalog に分けて記録した。古い demo flag、ブラウザ全体の third-party cookie 廃止予定、cookie quota の現行値は本稿の根拠にしない。
- 実 browser、framework の serializer/parser、reverse proxy、logout race、Storage Access API grant による経路差は未検証。header 例と試験表は独自案である。検索 eval は本稿の発見可能性のみを確認する。
- 原文の長文・コードを転載せず要約した。Google 文書の CC-BY-4.0 / sample Apache-2.0、WHATWG の [IPR policy §7.1.1](https://whatwg.org/ipr-policy)、IETF 各草案の copyright notice を確認した。draft / live docs を横断するため、通常90日 TTL より短い 2026-11-22 を再確認期限とする。

[chips]: https://privacysandbox.google.com/cookies/chips
[chrome-api]: https://developer.chrome.com/docs/extensions/reference/api/cookies
[html]: https://html.spec.whatwg.org/multipage/browsers.html#sites
[cookies]: https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-rfc6265bis-22
[attributes]: https://privacysandbox.google.com/cookies/basics/cookie-attributes?hl=en
[storage]: https://privacysandbox.google.com/cookies/storage-partitioning
[historical]: https://datatracker.ietf.org/doc/html/draft-cutler-httpbis-partitioned-cookies-01

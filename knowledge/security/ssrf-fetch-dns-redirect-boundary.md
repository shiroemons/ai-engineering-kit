---
{
  "id": "security-ssrf-fetch-dns-redirect-boundary",
  "title": "SSRF fetch 境界: OWASP 2026年9月の host allowlist 明確化と DNS・redirect・接続先の整合",
  "kind": "knowledge",
  "technology": "security",
  "version": "OWASP SSRF Cheat Sheet snapshot 2026-10-02; allowlist/parser clarification e2d422148bebd737e7d85e45347ae85ae5e878ba (2026-09-14); ASVS 5.0.0 release 5cf9b032440be53ce345ab3c130fda46ba1ce7a2 (2025-05-30)",
  "tags": ["research-domain:security", "ssrf", "fetch", "allowlist", "dns-rebinding", "redirect", "egress", "asvs", "url-parser", "tls"],
  "sources": [
    {"id": "owasp-ssrf-prevention-20261002", "url": "https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html", "type": "official_docs"},
    {"id": "owasp-ssrf-allowlist-clarification-20260914", "url": "https://github.com/OWASP/CheatSheetSeries/commit/e2d422148bebd737e7d85e45347ae85ae5e878ba", "type": "github_repository_analysis"},
    {"id": "owasp-asvs-v5-encoding-ssrf-20261002", "url": "https://raw.githubusercontent.com/OWASP/ASVS/5cf9b032440be53ce345ab3c130fda46ba1ce7a2/5.0/en/0x10-V1-Encoding-and-Sanitization.md", "type": "official_docs"},
    {"id": "owasp-asvs-v5-tls-client-20261002", "url": "https://raw.githubusercontent.com/OWASP/ASVS/5cf9b032440be53ce345ab3c130fda46ba1ce7a2/5.0/en/0x21-V12-Secure-Communication.md", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# SSRF fetch 境界: host allowlist と実際の接続先を一致させる

## 問いと適用範囲

Webhook 配信・外部画像取得・metadata fetch で、入力の host が許可済みでも、後段の URL parser、DNS 解決、redirect、proxy が別の宛先を選んだらどう防ぐか。対象はバックエンドの外向き HTTP(S) 通信であり、ブラウザの CORS や OAuth callback の完全一致検証を代用にはしない。

既存の [OAuth BFF](../oauth/bff-cookie-csrf-proxy-boundary.md) は token を付ける API proxy の host/path 制約、[MCP schema](../mcp/tool-schema-json-value-ref-validation.md) は external `$ref` を取得するかどうかを扱う。本稿は、それらにも共通する「許可した論理宛先と実際に到達するネットワーク宛先」の実装前レビューに絞る。汎用の安全な fetch 実装を提供・検証した文書ではない。

## 適用版と最近の変更

- OWASP Cheat Sheet の [2026-09-14 の変更][delta] は、host-only 入力を allowlist と照合してリクエストを再構築することと、サービス間の parser disagreement を拒否することを具体化した。URL から host を取り出す場合にも、元の path/query をそのまま次の処理へ渡さないという範囲の説明が加わった。
- 差分は commit `e2d422148bebd737e7d85e45347ae85ae5e878ba` に固定して確認した。これは新しい SSRF 防御プロトコルではなく既存ガイドの明確化である。DNS・redirect・network layer の助言がこの変更で初めて追加されたとは扱わない。
- 選定時に当該ファイルの直近5コミットを GitHub API で確認した。2026-09-29 の `1cde2c91c0a5fbb8a1c0f53fa5e45a418cf3a70c` はリンクの修正、8月の2件と6月の1件は表記・誤字修正であり、9月14日の実質的な補足を採った。web の履歴ページだけでは9月分が欠けていたため、表示順を最新性の根拠にしていない。
- ASVS は [5.0.0 の公開 release](https://github.com/OWASP/ASVS/releases/tag/v5.0.0_release)（2025-05-30）を使う。`v5.0.0_release` が指す `5cf9b032440be53ce345ab3c130fda46ba1ce7a2` の V1 / V12 を直接読み、移動する master や版番号だけの参照と区別した。

## 一次資料で確認した制約

### ASVS 5.0.0 の要件

[V1][asvs-v1] の `v5.0.0-1.3.6`（L2）は、別サービスの呼出しに使う不信なデータについて protocol・domain・path・port の allowlist と危険な文字の処理を検証対象とする。`v5.0.0-1.5.3`（L3）は、同じデータを読む複数の parser の解釈と文字 encoding の整合を求める。`v5.0.0-1.1.1`（L2）は期待する形式を一度だけ復号・正規化し、検証後に再び復号する流れを避ける要件である。

[V12][asvs-v12] の `v5.0.0-12.3.2`（L2）は TLS client にサーバー証明書の検証を求める。SSRF 対策のために IP へ接続を固定しても、この要件を解除する理由にはならない。

これらの個別要件を参照しただけで ASVS L2/L3 全体への適合を主張しない。特に、任意の公開 URL を受け付ける機能を単に denylist で制御しても `1.3.6` の allowlist 要件を満たしたとは判定しない。

### Cheat Sheet の既存ガイダンス

[SSRF Cheat Sheet][ssrf] は、宛先を特定できる場合と任意の外部宛先へ送る場合を分ける。前者では allowlist、後者では制約のある blocklist 防御を扱う。URL の解析成功そのものは宛先の許可にならない。

同資料は HTTP client の自動 redirect を無効にし、DNS pinning 対策で全 A + AAAA の解決先を検査すること、application layer と network layer の両方を制限することを説明する。private・loopback・IPv4/v6 link-local 等を考慮し、掲載された denylist は最低限の例であって完全な宛先判定表ではない。

以下の接続制御・TLS identity・再試行の設計は、これらの資料を組み合わせた本稿の推奨である。OWASP が特定の HTTP transport や proxy 実装を安全と認定している、という意味ではない。

## 設計提案: 入力から socket まで許可を引き継ぐ

### 1. 固定連携と公開 URL 取得を別のポリシーにする

固定連携は利用者に送信先 URL を選ばせず、連携先 ID から server 管理の `scheme / host / port / path` を選ぶ構成を優先する。可変部分は業務上必要なフィールドだけにし、選択した宛先の credentials も同じ設定に束縛する。

任意の公開 URL を扱う製品要件がある場合は、それを明示した別の fetch 機能として扱う。社内連携用の credentials・到達可能ネットワーク・HTTP client pool を共有しない。社内向け公開 IP や独自 route もあり得るため、「RFC1918 以外なら許可」という判定にはしない。どの宛先集合を許可するかは配備環境の所有者が決める。

### 2. parse と authorization を分ける

- 利用する parser と正規化方針を決め、その結果の構造化された値で policy を照合する。parse に成功した文字列を raw URL のまま別言語の worker へ渡して再解釈させない。
- host-only の契約なら、許可済みの設定項目から URL を構築する。入力に含まれていた userinfo・path・query を後から戻さない。完全 URL が本当に必要なら、その別契約で scheme・port・path 等を個別に審査する。
- 不要な userinfo、fragment、非 HTTP(S) scheme、曖昧な区切りや encoding は拒否する方針を推奨する。これは本稿の受入れ条件であり、すべての URL 標準が同じ文字列を不正と判定するという主張ではない。
- 複数 parser の host 解釈が違う入力は修復して通さず拒否する。前段だけで安全な文字列へ見せても、後段が異なる境界で読むなら照合が成立しない。
- IP literal は IP parser の結果で分類する。文字列の前方一致で private range を判定しない。IPv4-mapped IPv6、zone identifier、代替表記について、採用ライブラリが受け入れる範囲と比較時の形を固定し、未対応形式は拒否する。どのランタイムにも同じ canonicalization があるとは仮定しない。

### 3. DNS 検査を実際の接続に結び付ける

DNS rebinding / TOCTOU の重要点は検査回数ではなく、検査した値を接続時に使うことにある。「DNS を一度調べて許可 → 通常の HTTP client に元の hostname を渡す」だけでは、その client や proxy の再解決結果を制約できない。

推奨する invariant は、各新規接続がその接続用に審査した IP にしか到達しないことである。実装候補は、解決・分類・dial を一体化した transport、または同じ責任を負う egress proxy。単に resolver を信頼済みに変更するだけでは、攻撃者が管理する外部ドメインの応答変化を防げない。

- 全 A + AAAA を取得して分類し、公開・内部が混在する集合は保守的に拒否する。最初の1件だけで承認し、別のアドレスへ fallback させない。
- DNS timeout や解析不能を許可に変換しない。一方、片方の family にレコードが存在しない正当な NODATA と問い合わせ失敗は区別する。「A と AAAA が両方存在しなければ不正」という規則ではない。
- 検査済みアドレスから選択して接続し、暗黙の再解決を挟まない。retry・再接続・IPv4/IPv6 fallback もこの境界を通す。起動時の検査や登録時の URL 審査だけで後日の到達先を保証しない。
- proxy が最終宛先を名前解決するなら、制御点も proxy 側に必要になる。アプリが観測した接続先は proxy の IP であり、それだけでは最終宛先の検証にならない。

ここでの固定は接続ごとの制御であり、CDN の IP を恒久的に pin する運用とは違う。DNS TTL、接続 pool、policy 更新時の既存接続の廃棄、別 origin への接続共有をどう扱うかを実装の確認項目にする。

### 4. IP と TLS / HTTP identity を混同しない

接続先 IP を制御するときも、許可した論理 hostname を HTTPS の証明書検証名、必要な SNI、HTTP `Host` / `:authority` の根拠として保持する。これらを利用者に個別指定させず、同じ検証済み宛先から導く。SNI を送ること自体は証明書検証ではない。

URL 全体を IP 表記へ書き換え、証明書エラーが出たら検証を無効にする実装は採らない。IP 接続と hostname 検証を両立できるかは client/proxy の公式 API と実機テストで確認する。TLS が正常でも、許可していない内部宛先への接続を許してよいことにはならない。

### 5. redirect は新しい宛先決定として扱う

既定は自動 redirect 無効とし、3xx を最終結果または明示的なエラーとして扱う。製品要件で追従する場合は、次の `Location` を現在の URL に対して解決した後、構文・論理宛先・DNS・接続先を同じ順で再審査する。相対 `Location` も審査を飛ばす根拠にはしない。

最大 hop 数を決め、HTTPS から HTTP への降格や想定外の port を拒否する。次の要求の method・body・credentials をどうするかも明文化し、利用ライブラリが各 status で何を転送するか確認する。別 origin に元の `Authorization`、cookie、API key、署名済み header をそのまま引き継がない。これは本稿の安全側の設計案であり、OWASP が示した「追従を無効にする」構成と同等に検証済みという意味ではない。

### 6. egress と実行予算を最後の防壁にする

fetch worker から内部管理系・metadata service・不要な port への到達をネットワークでも制限する。環境変数の proxy 設定や sidecar が別経路を作る場合は、その経路も審査対象にする。アプリの denylist だけを最終防壁にしない。

接続 timeout だけでなく処理全体の期限、応答 byte 数、展開後の上限、並行数も予算化する。許可済みの公開宛先でも遅延や過大応答はあり得るためで、これらは SSRF の宛先制御とは別の resource 制限である。監査には policy ID、拒否理由、検証済み host、必要最小限の接続情報を残し、URL query や認証 header の秘密をログへ出さない。

## 実装時の受入れ試験案（未実行）

以下は本稿の設計を採用する場合の試験計画であり、検索 eval の成功で実施済みにはならない。

| 条件 | 観測したい結果 |
| --- | --- |
| 許可 host を通過した raw URL を別 parser が異なる host と読む | 後段へ転送する前に拒否。曖昧な入力を自動修復しない |
| host-only 入力に元 URL の query/path が残る | server 管理のリクエストに混入しない |
| A が許可済み、AAAA の一部が内部宛先 | mixed answer を拒否。IPv6 fallback でも送信しない |
| 検査時と接続時で DNS 応答を変える | 未審査 IP への packet / HTTP request が出ない |
| `Location` が内部宛先・別 port・別 origin を指す | 既定は追従せず、opt-in でも審査前に送信しない |
| 検査済み IP へ HTTPS 接続するが hostname の証明書が不一致 | 失敗。TLS 検証の無効化や IP 名への置換で継続しない |
| proxy、retry、connection pool を有効にする | 初回と同じ宛先制御が全経路で働き、policy 更新の扱いも再現可能 |
| 許可宛先が遅延・過大・圧縮応答を返す | 期限とサイズ予算で打ち切り、worker の資源を回収する |

防御試験は管理下の DNS・stub server・隔離ネットワークで行い、実在する第三者サービスや本番 metadata endpoint を試験先として使わない。例外が必要なら、対象連携だけに scoped policy を作り、汎用 fetch の全体許可へ広げない。

## 未確認事項と provenance

- 今回は既存 KB の `SSRF`、`DNS rebinding`、`redirect allowlist`、`URL validation` を検索し、上記2文書等を読んで重複を確認した。接続と名前解決の一貫した境界、9月14日の host-only 補足は独立した空白だった。
- 各 HTTP client の parser・dial hook・redirect・pool・proxy の具体的挙動、DNSSEC、NAT64 等の変換後の宛先判定、service mesh の構成は未検証。ライブラリ間でコピー可能な安全性保証や、CIDR の完全な denylist は提供しない。
- Cheat Sheet の公開ページは独立した版番号や本文更新日時を表示しないため、2026-10-02 UTC の取得内容として扱う。変更日時は commit API の UTC timestamp で確認した。ASVS の要件は release commit に固定した。明示期限は official_docs / repository analysis の90日 TTLに合わせた 2026-12-31。
- OWASP Cheat Sheet Series と OWASP ASVS の参照資料は CC BY-SA 4.0。各 repository の固定 commit にある LICENSE.md を確認した（[Cheat Sheet license](https://raw.githubusercontent.com/OWASP/CheatSheetSeries/e2d422148bebd737e7d85e45347ae85ae5e878ba/LICENSE.md)、[ASVS license](https://raw.githubusercontent.com/OWASP/ASVS/5cf9b032440be53ce345ab3c130fda46ba1ce7a2/LICENSE.md)）。本文は短い事実の要約と独自の設計・試験提案であり、コード・regex・図の転載、module 化はしていない。
- 取得時の失敗: `.../ASVS/releases/tag/v5.0.0` は取得できず、公式の `v5.0.0_release` を確認した。Cheat Sheet の固定 license は native web で cache miss だったため GitHub connector の read で確認した。取得失敗を資料の不存在や内容未確認の更新日延長に置き換えていない。

[ssrf]: https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html
[delta]: https://github.com/OWASP/CheatSheetSeries/commit/e2d422148bebd737e7d85e45347ae85ae5e878ba
[asvs-v1]: https://raw.githubusercontent.com/OWASP/ASVS/5cf9b032440be53ce345ab3c130fda46ba1ce7a2/5.0/en/0x10-V1-Encoding-and-Sanitization.md
[asvs-v12]: https://raw.githubusercontent.com/OWASP/ASVS/5cf9b032440be53ce345ab3c130fda46ba1ce7a2/5.0/en/0x21-V12-Secure-Communication.md

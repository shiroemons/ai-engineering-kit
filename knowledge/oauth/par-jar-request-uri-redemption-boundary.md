---
{
  "id": "oauth-par-jar-request-uri-redemption-boundary",
  "title": "OAuth PAR / JAR: request_uri の一回利用・client照合・認可確定の境界",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 9126 (September 2021); RFC 9101 (August 2021); RFC 9700 / BCP 240 (January 2025); FAPI 2.0 Security Profile Final (2025-02-22); verified 2026-10-04 UTC",
  "tags": ["research-domain:security", "oauth", "PAR", "JAR", "request_uri", "client-binding", "single-use", "parameter-tampering", "preload", "downgrade"],
  "sources": [
    {"id": "rfc9126-par-redemption-policy-20261004", "url": "https://www.rfc-editor.org/rfc/rfc9126.html", "type": "official_docs"},
    {"id": "rfc9101-jar-parameter-assembly-20261004", "url": "https://www.rfc-editor.org/rfc/rfc9101.html", "type": "official_docs"},
    {"id": "rfc9700-par-csrf-binding-20261004", "url": "https://www.rfc-editor.org/rfc/rfc9700.html", "type": "official_docs"},
    {"id": "fapi2-par-preload-single-use-final-20261004", "url": "https://openid.net/specs/fapi-security-profile-2_0-final.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# PAR / JAR: 認可要求の保存と、認可を一度だけ確定する処理を分ける

## 問いと採用判断

OAuth の認可要求を PAR endpoint に送って `request_uri` を得たら、ブラウザーから同じ URI が再到着しても安全なのか。署名付き JAR を併用すれば、client 認証や callback の対応付けを省けるのか。

本書の判断は、**要求内容の固定、client の照合、期限、認可の一回確定を別々に検査する**ことである。PAR は認可要求を AS に直接届ける経路、JAR は要求内容の署名等を扱う仕組みであり、同じスイッチとして実装しない。2026年の新変更とは主張せず、2021年の確定RFCと2025年のBCP・profileから、既存の [PKCE基礎](authorization-code-pkce-state-redirect.md) が未確認としていた認可開始の境界を補う。token交換や [DPoP](dpop-proof-binding-nonce-replay.md) の再説明は範囲外である。

## 確認した契約

### 1. PAR の受付成功と後続の認可は別段階

[RFC 9126 §2・§2.1][par] は HTTPS の POST と form body を使い、token endpoint と同じ client 認証規則を適用する。PAR body に `request_uri` を入れることは禁止される。認証資格情報を持たない public client を、この規定だけで confidential client に変えることはできない。

[§2.2・§4][par] の要点は、返された参照値を送信 client に結び付け、`expires_in` で寿命を示すこと。client は同じ値を一度だけ使う MUST、AS の one-time 処理は SHOULD で、ブラウザー refresh による重複を許す MAY がある。期限切れの拒否は MUST である。受付時の検査を認可時に省略できるのは、その検査結果に影響する要求・AS policy の変更がないと確認できる場合に限る。

したがって「PAR は成功したので、後から client の許可scopeを削除しても通してよい」「同じ URI は何回でも新しい認可に使える」という結論にはならない。

### 2. JAR の署名検査と client 認証を混ぜない

[JAR §4・§5・§6][jar] は認可パラメータを Request Object に収め、署名付き要求を処理するときは client に関連付いた鍵・許可された方式で検証する。外側の `client_id` と JWT 内の同名値は一致が MUST。認可パラメータは JWT 内の集合だけを使い、外側queryから補完・上書きする merge はしない。署名だけでは暗号化による機密性は得られない。

[PAR §3][par] で JAR を送る場合、form body の `request` 以外は client 認証に必要なパラメータに限り、認可パラメータを JWT 内に置く。認証済み client_id と Request Object の client_id が違えば拒否する。`client_assertion` と `request` は役割の異なる JWT であり、一方の検証成功で他方を検証済みにしない。

### 3. PAR 必須と署名必須は別の policy

`require_pushed_authorization_requests` は PAR 経由を要求する設定で、[RFC 9126 §4–§6][par] が定義する。`require_signed_request_object` は JAR の要件で、[RFC 9101 §10.5][jar] が downgrade 防止を定める。PAR対応endpointの存在だけを、全clientで署名必須という意味に読み替えない。署名が必須なら、PARでも通常のformパラメータへ自動fallbackしない。

同じ名前の `request_uri` でも、[JAR §2.2・§5.2][jar] には外部のRequest ObjectをHTTPSで取りに行く方式がある。AS自身が発行したPAR参照の内部取得を、未確認のURLへの一般的なfetchへ置き換える理由にはならない。

### 4. 一回利用を page load と同一視しない

[FAPI 2.0 Security Profile Final §5.3.2.2 Note 3][fapi] は、一回利用を課す場合の消費時点をページ読込ではなく認可時点に置くことを推奨する。先読み preload で正規ユーザーの参照値を無効化しないためである。これは RFC 9126 の refresh 許容と合わせて考える運用上の区別であり、「画面を開くたび新しいcodeを発行してよい」という許可ではない。

このprofileは confidential client が対象。認証付きPAR、S256 PKCEを要求し、`request_uri` の寿命を600秒未満に制限する。**600秒未満はFAPIの要件**で、一般のRFC 9126の一律上限ではない。本稿の一部を採用しただけでFAPI適合になるともいえない。

### 5. ブラウザーとの対応付けは残る

[RFC 9700 §2.1・§2.1.1・§4.7.1][bcp] では public client にPKCEが必須、confidential clientには推奨される。challengeを開始client・user agent・取引に安全に結び付ける必要がある。CSRF対策は必須だが、ASのPKCE対応を確認したclientはPKCEの保護に依存でき、OIDCにはnonceの選択肢もある。それらを使わない場合はuser agentに結び付いた一回限りのstateが必要になる。

従って「PARならPKCEやCSRF対策を削除できる」も「PKCEがあってもstateが常に仕様上必須」も正確ではない。application stateをstateに載せ、その完全性が必要なら、改ざん・入替えに対する保護を行う。

## 実装・運用の判断案（本リポジトリ独自の提案）

以下は特定DBの契約や完成した認可サーバー実装ではなく、既存OAuth実装を選定・設定する際のレビュー方針である。

1. **受理済み要求を不変にする**: issuer、client_id、検証済みパラメータ集合、期限、適用policy世代を一つの記録に関連付ける。ブラウザーから到着したscopeやredirect_uriでこの記録を書き換えない。PAR/JARの併用有無を明示して、形式から推測しない
2. **画面表示と認可確定を分ける**: 例えば「未確定」「確定済み」「期限切れ」の状態を持つ。preloadやrefreshのための読取りが認可結果の新規生成へ進まないようにする。画面再表示を許す設計では、同じ進行中処理への復帰に限定し、期限・client・ブラウザー関連付けを再確認する
3. **一回確定を原子的に守る**: 二つのAS instanceが同じ未確定記録を読んでも、確定権を得るのは一つにする。状態更新と認可結果の保存を別々の非原子的な操作にすると、停止・再実行時に二重発行を起こし得る。AS製品の耐障害性を試験し、単なるcache削除を十分な証拠にしない
4. **期限とpolicyを再検査する**: TTLによる物理削除の遅延に依存せず、処理時刻で有効性を判定する。受付後のclient無効化・scope縮小を反映する。検査の再利用はpolicy世代が一致すると確認できる場合に限定する
5. **失敗時に保護を外さない**: 期限切れや取消済みのflowから再開するときは、新しい認可試行として必要なPKCE・state等を作り直し、古いcallbackを現行試行へ取り込まない。通信失敗だけでplainな認可URLへ切り替えたり、署名必須を解除したりしない
6. **参照値を露出させない**: ログには生のrequest_uri、JWT、verifierを保存せず、理由コードとローカルな追跡IDを残す。診断項目を client不一致、署名失敗、期限切れ、policy変更、二重確定に分ける。短命なURIだから漏れてもよいとは扱わない

`request_uri` の寿命と、受付後に開始したユーザー認証・同意画面のセッション寿命は別に記録する。長いMFA操作の途中で何を失効させるか、期限到来後の同意POSTをどう扱うかはASの仕様・profileを確認して決める。本調査だけでは、その内部セッションの共通寿命やrefreshの安全な識別方法を確定していない。

## 受入れ試験案（未実行）

- **client入替え**: client Aの参照値に外側client Bを付ける。別clientの認可として処理されないことを確かめる
- **JWTと認証の不一致**: 認証済みA、署名対象のclient_id Bを用意する。署名が正しいだけで受理しない
- **外側query補完**: 署名対象にないscopeをqueryへ追加し、署名対象のscopeも別値で重複させる。いずれも検証済み集合を拡張・置換しない
- **preloadとrefresh**: 先読み後の正規操作が即座に無効化されない。refreshを許す設定でも、同じ要求から二つ目の独立した認可結果を作らない
- **並行確定**: 別instanceへ同時に最終操作を送り、単一instance試験では見えない競合を検査する
- **TTL削除遅延**: 記録を期限後も残し、参照の新規利用が拒否されることを確かめる。長時間MFAは別ケースとして製品仕様に沿って検査する
- **policy変更**: PAR成功後、認可前にscope許可を撤回する。古い受付結果だけで通らない
- **downgrade**: PAR必須・署名必須を独立に設定し、通常query要求と署名なしPARへの迂回を検査する
- **callback入替え**: 古いflowのstate・codeと新しいflowのverifierを混ぜ、同じclient_idという理由だけで受理されない
- **FAPI寿命境界**: FAPIを採用する構成では600秒ちょうどを許可値にしない。一般PARの適合性とは判定を分ける

検索evalは本文への到達性の検査だけであり、上記の相互運用・暗号・並行性試験を実行したものではない。

## 限界・版・provenance

- 2026-10-04 UTCにRFC 9126、RFC 9101、RFC 9700、FAPI Finalの実ページを開いた。既存catalogの日付を延長せず、今回の確認範囲を記録した別sourceを追加する。公式文書TTLは90日で、再確認日は2027-01-02
- RFC 9126のerrata取得はcache miss・429等で完了しなかった。未解決errataが存在しないとは確認していない。本文は取得できたRFC公開本文を根拠とする
- 製品別PAR/JAR対応、公開clientへの適用条件、client認証の鍵管理、分散storeの障害復旧、署名JWTの個別profile、JAR外部参照先の取得制限は未検証。特に署名者認証だけで継続的なclient資格情報の管理を置き換えない
- RFCのCopyright NoticeにあるBCP 78 / IETF Trust Legal Provisionsとコード部分の条件（RFC 9126/9101はSimplified BSD、RFC 9700はRevised BSD）、FAPI Appendix BのOIDF帰属・仕様策定/実装目的のcopyright licenseを確認した。IETF / OpenID Foundationを出典とする独自の日本語要約と設計案であり、団体の推奨・認証を表さない。コード・鍵例・図・長文の転載やmoduleへの昇格は行っていない

[par]: https://www.rfc-editor.org/rfc/rfc9126.html
[jar]: https://www.rfc-editor.org/rfc/rfc9101.html
[bcp]: https://www.rfc-editor.org/rfc/rfc9700.html
[fapi]: https://openid.net/specs/fapi-security-profile-2_0-final.html

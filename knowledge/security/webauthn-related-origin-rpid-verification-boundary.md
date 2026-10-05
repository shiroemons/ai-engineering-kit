---
{
  "id": "security-webauthn-related-origin-rpid-verification-boundary",
  "title": "WebAuthn related origins: RP ID共有・origin検証・既存passkey移行の境界",
  "kind": "knowledge",
  "technology": "security",
  "version": "WebAuthn Level 3 Recommendation 2026-08-25; Level 2 Recommendation 2021-04-08 comparison; passkeys.dev deployment guide updated 2026-02-05",
  "tags": [
    "research-domain:security",
    "webauthn",
    "passkey",
    "related-origins",
    "RP-ID",
    "origin",
    "topOrigin",
    "crossOrigin",
    "domain-migration",
    "Permissions-Policy"
  ],
  "sources": [
    {
      "id": "w3c-webauthn3-related-origin-verification-20261005",
      "url": "https://www.w3.org/TR/2026/REC-webauthn-3-20260825/",
      "type": "official_docs"
    },
    {
      "id": "w3c-webauthn2-client-data-comparison-20261005",
      "url": "https://www.w3.org/TR/2021/REC-webauthn-2-20210408/",
      "type": "official_docs"
    },
    {
      "id": "webdev-related-origin-rpid-server-origin-20261005",
      "url": "https://web.dev/articles/webauthn-related-origin-requests",
      "type": "maintainer_article"
    },
    {
      "id": "passkeysdev-related-origin-existing-migration-20261005",
      "url": "https://passkeys.dev/docs/advanced/related-origins/",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2027-01-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# WebAuthn related origins: RP ID共有・origin検証・既存passkey移行の境界

## 問いと結論

国別ドメインや別ブランドのサイトで同じpasskeyを利用するとき、`/.well-known/webauthn`にサイトを載せれば、サーバー側の検証も移行も完了したと考えてよいか。あるサイトを一覧から外すだけで、そのサイト経由の認証を止められるか。

本書の判断は、**クライアントがRP IDの利用を許す条件、RPサーバーが応答のoriginを受け入れる条件、既存credentialのRP IDを選ぶ条件を分離する**こと。Related Origin Requests（ROR）は共通のRP IDで新規登録・認証を行える範囲を広げるが、サーバーのorigin検証、アカウント対応付け、iframe配備方針を肩代わりしない。

対象は同一のアカウント基盤を使う複数のWeb origin。native appのorigin表現、Digital Asset Links、attestationの信頼判断は対象外とする。既存の[BE/BSとsignCount](webauthn-passkey-backup-flags-signcount.md)はcredential状態を扱う。本書は認証をどのサイトから受理するかという独立した配備判断を扱い、完全なWebAuthn検証器の実装手順にはしない。

## 確認した版と新規性

- [WebAuthn Level 3固定版][l3]は2026-08-25 W3C Recommendation。2026-10-05 UTCに現行TRと固定版の見出し・Statusを照合した。Statusには2026-05-26 Candidate Recommendation Snapshot以降の実質的変更なしとある。§18.1.1はrelated origins、topOrigin、cross-origin iframeでのcreateをLevel 2からの追加として列挙する
- [Level 2固定版][l2]は2021-04-08 Recommendation。CollectedClientDataにtopOriginはなく、Level 3と同じ情報がすべての旧応答に含まれるとは仮定できない
- [Googleの解説][google]はfooterが2024-08-22。本文のブラウザ対応説明や停止済みdemoを、2026-10-05時点での実機確認へ読み替えない。[passkeys.devの配備ガイド][deployment]はLast Updated 2026-02-05で、運営者をW3Cの関連CGとFIDO Allianceのmemberと表示する。後者も規範仕様ではない
- 調査前にknowledge・patterns・modules全体を検索し、`related origins`と`topOrigin`は該当なし、`webauthn origin`はBE/BS文書だけだった。最近16件の調査コミットにも本書の問いはない。新機能が今日出荷されたという報告ではなく、2026年の勧告に基づき未収録の実務上の隙間を埋める

## 仕様で確認した三つの境界

### 1. RP ID共有は呼出し元originの同一化ではない

[Level 3 §4・§6.1・§7.1–7.2][l3]で、RP IDはcredentialのスコープを表す。originと違ってschemeやportを含まず、作成したcredentialの寿命中に変わらない。RPはauthenticator dataの`rpIdHash`を期待RP IDのSHA-256と照合し、それとは別にclientDataJSONのoriginを確認する。

例えば、本書の仮想サービスがRP IDを`example.com`に統一し、`https://example.net`でも使うなら、認証器に対する期待RP IDは前者、そこで開始した処理の期待originは後者になる。[Google解説のStep 4][google]も、RP IDを変更しても呼出し元を表す期待originは変えないと説明する。serverの受信API URLをそのまま期待originにするとは限らない。

| 値 | 何を固定・照合するか | 代用できないもの |
|---|---|---|
| RP ID / rpIdHash | credentialのスコープ | 今回アクセスしたWeb origin |
| clientDataJSON.origin | WebAuthnを呼んだサイト | 最上位画面のorigin、アカウントの所有者 |
| clientDataJSON.topOrigin | 条件に応じて含まれる最上位画面 | 呼出し元originの検査、frame祖先すべての列挙 |
| credential ID / userHandle | 保存済みcredentialとユーザーの対応 | サイト名が似ていることによる自動account link |

§7.2は、開始時に利用者が既知ならその利用者のcredentialと照合し、userHandleが返れば一致を確認する。利用者が未知ならuserHandleを要求し、そのアカウントにcredentialが属することを確認する。RORを入れてもこの条件は残る。

### 2. well-knownはクライアントの追加許可を発見する場所

[§5.11–5.11.1][l3]の契約は次のとおり。

- 関連origin群で共通RP IDを選び、そのRP IDの`https://{RP ID}/.well-known/webauthn`に、origins配列を含むJSON objectを配信する。Content-Typeはapplication/json
- クライアントの取得はcredentialsとreferrerを送らずHTTPSを用い、追従するredirectもHTTPSに限定する。最終statusが200でない、Content-Typeが違う、JSON objectや配列の型が不正、取得失敗などではSecurityErrorになる
- 候補originはsame-originの条件で照合する。ドメイン文字列の部分一致やブランド名の一致ではない
- 対応クライアントには少なくとも5個のregistrable origin labelを扱う義務がある。仕様共通の「originを最大5行まで」という意味ではなく、上限はclient policyにも依存する

labelは、例えば`example.com`と`example.co.uk`ならどちらもexampleになる。§5.11.1は配列順に処理し、上限に達した後の未知labelを飛ばすが、既に数えたlabelの後続originまで一律に捨てる手順ではない。本書では配信前検査をoriginの行数だけで実装しない。上限を5とする対象では、六つ目の異なるlabelと、その後にある既出labelのoriginを別ケースとして試験する。

重要なのは[§5.1.3・§5.1.4.1][l3]の入口条件である。RP IDが呼出し元のeffective domainと一致するかregistrable domain suffixである通常経路では、related originsの取得による追加許可を必要としない。

**ここからの運用上の推論:** `login.example.com`からRP ID `example.com`を使う経路を止めたい場合、well-knownからそのoriginを削除しても、その削除だけでは通常経路を塞げない。well-knownはRP全体のサーバー側allowlistでも、credentialの失効台帳でもない。関連originからの経路についても、今回確認した契約だけで全クライアントのcache更新時間や即時停止を保証しない。

### 3. crossOriginはRP IDとのドメイン差ではなくframe祖先の情報

[§5.8.1][l3]のcrossOriginは、呼出し元と祖先が同一originかどうかに由来する。topOriginは最上位originで、crossOriginがtrueとなる場合に設定される任意memberである。

したがって、`https://example.net`のtop-level画面がRORによりRP ID `example.com`を使うだけなら、RP IDとの相違を理由にcrossOrigin=trueとはならない。一方、別originの最上位画面に埋め込まれた認証iframeでは、呼出し元originとtopOriginを別々に評価する。topOriginは直近の親iframeや全祖先の一覧ではないため、祖先すべての所有者をこれ一項目で検証したことにはならない。

[§7.1・§7.2][l3]は、crossOriginが存在してtrueならその埋込み利用をRPが想定していること、topOriginが存在すれば想定した最上位画面に対応することを確認する。`topOrigin`がないという一点だけで、最上位画面の本人性や非埋込みを証明した扱いにはしない。

§5.8.1.2の限定検証アルゴリズムにはrequireTopOriginという入力があり、trueにする場合の旧Level 2 serializationとの非互換が明記される。これはRPが使う全ライブラリに同名optionが必ずあるという保証ではない。

## 独自の配備設計

以下は仕様の要約から導いた本リポジトリの設計案であり、W3Cが定めたDBスキーマや必須運用ではない。

### 許可方針は要求を開始するときに選ぶ

RP ID、許す呼出し元origin、埋込みを許すか、許すtopOrigin、利用者の識別方法、ポリシー版を同じ認証経路の設定として管理する。サーバーは信頼する設定から期待値を選び、受信bodyに書かれたoriginや未検証のHostから期待値を作り直さない。

少数の管理サイトなら、scheme・host・portを含む明示origin一覧を優先する。これは[§13.4.9][l3]が認める手法の一つであり、完全一致だけが仕様上唯一の方法という意味ではない。§13.4.8–13.4.9は広いsubdomain許可とそこで動く不信なコードの危険も扱う。ユーザー投稿サイトや委託先サイトを「同じ親ドメインだから」と認証originへ自動追加しない。

origin検査が通っても、type、challenge、RP ID、credentialとアカウントの照合、必要なUP/UV、署名・attestation等、採用フローに必要な残りの検査を省略しない。JSONの意味検査用にparseしても、認証署名のhash対象は受信した元のclientDataJSON bytesであり、parse後に再serializeした別のbytesへ取り替えない。

### 非埋込みと承認した埋込みを別経路にする

- 非埋込みを前提とする経路では、crossOrigin=trueまたはtopOriginありを想定外として拒否する。RORの有無ではこの方針を変えない
- 埋込みを承認する経路では、呼出し元と最上位画面の組を明示する。本書の厳格な方針ではtopOriginの存在も要求し、欠落時には検証を弱めずtop-levelの認証画面へ誘導する。この欠落拒否は独自方針であり、§7の「存在すれば検査」を「全応答で必須」と言い換えたものではない
- [§5.9–5.10][l3]のPermissions Policyも別途満たす。publickey-credentials-createとpublickey-credentials-getは別featureで、既定はself。API呼出しが許可されたことをサーバーの最上位origin承認に置き換えない

埋込みUIの操作意図やclickjacking対策は別の確認を要する。最上位画面を許した事実やpasskeyの成功だけから、画面にある個別の購入・送金操作まで利用者が承認したと扱わない。

### 追加と撤回をそれぞれ試験する

新しいorigin追加前に、アカウント対応付けとサーバー期待値を配備し、well-known、クライアントの実応答、サーバー受理の順を通した試験を行う。配信JSONだけの成功をログイン完了にしない。

撤回時はサーバーのorigin許可を明示的に外し、各replicaへ反映されたことを確認する。併せてRORの配信一覧や関連画面を更新する。well-known取得がcacheから返る場合や、そもそも取得しないsuffix経路でも、サーバーが対象originの新しい応答を拒否することを確認する。

この措置で既に成立したアプリケーションsessionが終了したとは主張しない。必要ならsession終了、credential停止、進行中の認証要求の扱いを別の手順として決める。ポリシー撤回前に始まった処理も、最終受理時点の緊急拒否方針を再評価する設計を採るなら、その競合を試験に含める。

## 複数の既存RP IDから移行する

[§6.1][l3]のRP ID不変性から、既存passkeyのDB上のRP IDを新しい値に書き換えるだけでは移行できない。新規発行の共通RP IDと、既存credentialが持つ旧RP IDを区別する。

[passkeys.devのExisting Deployments][deployment]は、既存credentialのRP IDを把握し、identifier-firstのbackend lookupや元originへ戻るfederation flowを用意する方法を示す。本書では次を実装判断にする。

1. credentialごとのRP IDとアカウント対応を確認し、不明な旧レコードを新しいRP IDとして既定補完しない
2. 共通RP IDでの新規登録と、旧RP IDでの認証を並行して扱う。RORによる相互利用を選ぶ既存RP IDには、それぞれ必要なwell-known配信とserver側origin検証を整備する。一方向の追加で逆方向も許可されたと推測しない
3. capabilityのrelatedOriginsを確認してUXを分岐するが、未対応、API欠落、取得失敗、credential未発見をorigin検証の緩和で救済しない。利用可能な元originでの認証や既存のfederationを退避先にする
4. 新しいRP IDのcredentialが必要なら、そのRP IDで別途登録する。移行完了の判断は新規登録数だけでなく、旧RP ID利用者が再訪できること、対応外クライアントの退避、管理対象ドメインを維持できることから行う

[Googleの解説][google]も共有アカウント基盤を前提にしている。異なるサイトのメールアドレスが同じというだけでアカウントを自動統合する根拠にはならない。ドメインの廃止、買収、ブランド譲渡を伴う場合は、旧RP IDの維持と認証先の所有者変更を移行条件として評価する。

## 受入れ試験の具体例

以下は独自のテスト案で、今回ブラウザや実認証器で実行した結果ではない。検索evalは文書の発見性だけを確認する。

| 条件 | 確認したい結果 |
|---|---|
| RORの一覧にあり、rpIdHashも正しいがserverのorigin一覧にない | ブラウザの作成成功や署名成功だけでは受理しない |
| サブドメインをwell-knownから削除し、親domainのRP IDを指定 | suffix経路が残る前提でserver側拒否を確認する |
| JSONが404、HTMLの200、originsが文字列、HTTPSからHTTPへのredirect | 許可を取得したことにせず、UIと退避先が無限再試行にならない |
| maxLabels=5、六つ目が未知label、七つ目が既出label | 六つ目のskipと七つ目のsame-origin照合を別々に確認する |
| top-levelの関連originから共通RP IDを利用 | crossOrigin=trueを必須にしない |
| 承認したiframe originだがtopOriginが未承認 | 子originだけの照合で通さない |
| crossOrigin=trueでtopOrigin欠落 | 厳格な埋込み方針では拒否し、互換性低下を明示する |
| publickey-credentials-getだけ許可されたiframeでcreate | getの許可から登録も可能と推測しない |
| originは正しいがrpIdHashが別RP ID | originが似ていてもスコープ不一致を拒否する |
| 旧RP IDのcredentialしかない利用者が新サイトへ来る | DB書換えで救済せず、旧RP ID選択または認証退避へ進める |
| userHandleが別accountを指す有効な署名 | 保存credentialとaccountの対応が失敗し、sessionを発行しない |
| clientDataJSONに未知memberや異なるkey順がある | 仕様に沿って解析し、署名検証には元bytesを使う |
| 撤回前に開始し撤回後に到着した応答、旧JSONを使うクライアント | 採用した最終受理方針を全replicaで確認する |

## 限界・provenance

- 実際のブラウザbuild、OS、認証器、password manager、WebAuthnライブラリの互換性とcache挙動は未実測。標準のRecommendation化を全製品の実装完了としない
- 既存の同一origin認証、conditional UI、U2F appid拡張、native app連携、attestation・復旧本人確認の全条件は収録しない。特にappidのrpIdHash処理は別条件を持つため、本書の通常RP ID例を流用しない
- 取得日はすべて2026-10-05 UTC。2027-01-03の期限は90日後の再確認日で、仕様の失効日ではない
- Level 3のcopyright表示からW3C Software and Document License 2023、Level 2のdocument-use linkからW3C Document Licenseを確認した。Google記事footerは本文CC-BY-4.0・code samples Apache-2.0、passkeys.dev footerはCC-BY-NC-ND-4.0。本文は出典を示した独自の事実要約で、仕様の翻訳転載、図、コード、test vector、サンプル設定の移植は行っていない
- 保存・配備・撤回・試験の提案は本リポジトリの設計判断として分離した。公式情報だけの文書という信頼表示にはせず、trustをprimary-sourceとする

[l3]: https://www.w3.org/TR/2026/REC-webauthn-3-20260825/
[l2]: https://www.w3.org/TR/2021/REC-webauthn-2-20210408/
[google]: https://web.dev/articles/webauthn-related-origin-requests
[deployment]: https://passkeys.dev/docs/advanced/related-origins/

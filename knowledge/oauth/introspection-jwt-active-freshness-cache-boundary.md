---
{
  "id": "oauth-introspection-jwt-active-freshness-cache-boundary",
  "title": "OAuth JWT introspection: 応答iat・token期限・activeとcache失効の検証境界",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 9701 (January 2025); RFC 7662 (October 2015); RFC 9068 (October 2021); RFC 7519 (May 2015), selected claims only; verified 2026-10-04 UTC; RFC 9701 errata unverified",
  "tags": ["research-domain:security", "oauth", "introspection", "jwt", "active", "token_introspection", "iat", "expiration", "cache", "revocation", "cross-jwt-confusion", "resource-server"],
  "sources": [
    {"id": "rfc9701-introspection-response-boundary-20261004", "url": "https://datatracker.ietf.org/doc/rfc9701/", "type": "official_docs"},
    {"id": "rfc7662-introspection-cache-expiry-20261004", "url": "https://www.rfc-editor.org/rfc/rfc7662.html", "type": "official_docs"},
    {"id": "rfc9068-access-token-type-boundary-20261004", "url": "https://www.rfc-editor.org/rfc/rfc9068.html", "type": "official_docs"},
    {"id": "rfc7519-introspection-claim-clocks-20261004", "url": "https://www.rfc-editor.org/rfc/rfc7519.html", "type": "official_docs"},
    {"id": "rfc7662-errata-introspection-20261004", "url": "https://www.rfc-editor.org/errata/rfc7662", "type": "official_docs"},
    {"id": "rfc9068-errata-claim-profile-20261004", "url": "https://www.rfc-editor.org/errata/rfc9068", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# JWT introspection: 署名された状態照会の回答を、期限なしの利用許可にしない

## 問いと選定理由

resource server（RS）が署名付きintrospection responseを受け取り、署名と `active=true` を確認した。いつまで再利用できるか。応答JWTをそのままaccess tokenとして扱えるか。

**応答の真正性、照会対象tokenの状態、個々のAPI操作の許可は別判定である**。特にRFC 9701の外側JWTには `exp` を付けないことが推奨されているため、一般的なaccess-token validatorを流用するだけでは、正しい応答の拒否にも、古い回答の無期限再利用にもなり得る。

本稿は2026年の新機能という主張ではなく、[RFC 9701][signed]と既存のintrospection/cache契約を結ぶ未収録の実装判断を扱う。[Token Exchange](token-exchange-delegation-audience-revocation-boundary.md)は別tokenの発行と失効伝播、[ID Token検証](../oidc/id-token-validation.md)はRPの認証判断が対象。本稿はRSが受け取る状態照会の回答と、その局所cacheに限定する。

## 一次資料で確認した契約

### 外側は回答、内側は照会されたtoken

[RFC 9701 §4–5][signed]では、JWT応答を要求するAcceptと返却Content-Typeは `application/token-introspection+jwt`、JWTの `typ` は `token-introspection+jwt`。外側に必須の `iss` はASのissuer URL、`aud` は回答を受けるRS、`iat` は回答作成時刻である。対象tokenの情報は必須object `token_introspection` の内側に入る。

外側の `sub` と `exp` は **SHOULD NOT** であり、必須でも絶対禁止でもない。内側の `iat` はtoken発行時刻、`exp` はtoken期限で、いずれもRFC 7662ではOPTIONAL。たとえば外側iatが新しくなっても、内側expが延長されたとは解釈しない。[RFC 9701 §5][signed]・[RFC 7662 §2.2][base]

RFC 9701は、無効・期限切れ・失効済み・呼出RSの対象外なら、内側を `active=false` だけにするMUSTを定める。これは外側の必須claimsまで消す指示ではない。ASは呼出RSを識別・認証・認可し、どのtoken情報を渡せるか判定する。署名を追加すれば任意の呼出元へ情報を開示できるわけではない。[§3、§5][signed]

### cacheは署名とは別に古くなる

[RFC 7662 §2.2–2.3、§4][base]では、inactiveという照会結果は照会処理のエラーではない。`active` はbooleanであり、200応答だけを有効tokenの証拠にはできない。対象tokenの期限・失効・利用開始時刻・対象RSなど、適用される状態検査はASが行う。

同RFCはcacheの性能と安全性のtrade-offを要求する。cache利用中にtokenが失効する窓があり、応答に `exp` があれば、その時刻を越えてcacheしてはならない。RFC 9701ではこのtoken期限が内側に置かれる。署名は回答の保護であって、保存中もASの状態が変わらないという保証ではない。

`exp` がない場合も、仕様が無限cacheを安全と認めたことにはならない。RFC 7662はすべての配備で共通のcache秒数を指定していない。再照会頻度、失効反映の要求、AS停止時の扱いを配備側で決める必要がある。

### 同じ署名鍵・issuerでも、access tokenへ代用しない

[RFC 9701 §8.1][signed]は、introspection responseとaccess tokenで `iss` や `aud` が似ることによるcross-JWT confusionを扱う。JWT responseはaccess tokenの別表現ではない。`typ` とclaimsの階層を維持することが、取り違え防止の一部になる。

比較対象の[RFC 9068 §2、§4][access]はJWT形式のaccess token用profileであり、RSは `at+jwt` または `application/at+jwt` を要求し、他のtypを拒否するMUSTがある。`exp`・`sub` 等も必須となる。これはそのprofileに準拠するaccess tokenの規則で、すべてのopaque tokenやRFC 9701応答に適用する共通schemaではない。

同§4はaccess tokenのissuer・audience・署名・期限を検証し、認可claimsを他の状況情報と組み合わせて操作を判断する。したがって、一つの「AS署名が正しければvalid」という関数で応答JWTとAPI入力を共通承認しない。

### JWT一般のoptionalと個別profileの必須を分ける

[RFC 7519 §4.1、§4.1.3–4.1.6][jwt]は、使用するclaimsと必須性を個別applicationが定める前提である。一般仕様でoptionalだから、RFC 9701で必須の `iat` やRFC 9068で必須の `iss` を省略してよいわけではない。

含まれる `exp` はJWT自体の受入れ期限、`iat` は発行時刻で年齢を判断する材料である。外側expを持つ配備を扱う場合も、内側token期限と同じ変数へ上書きしない。RFC 9701に一律の最大response ageがある、あるいはiat単独が自動的に有効期間を決める、とは読まない。

## 導入時の判断案（本書独自の設計）

以下のschema分離・cache key・期限計算・障害方針は実装レビューの提案であり、RFCが指定する共通実装ではない。

### 1. 入力経路から検証profileを選ぶ

APIのaccess-token入力と、RSが開始したintrospection HTTP要求への回答を別の入口にする。未検証JWTの中身だけを見て、通りそうなvalidatorを順番に試すfallbackを作らない。RFC 9701応答用には、設定済みAS、期待するRS audience、許可algorithm、必要な署名・暗号化、外側claims、内側objectを検査する。

「外側expを必須にしない」は応答用profileだけの判断とする。access-token用のexp検査を全体設定で無効にしない。署名済みの内容を検証後に型付きobjectへ写し、`token_introspection` を平坦化して外側とmergeしない。外側audが回答受領者に一致しても、内側の権限から対象APIの操作許可を別に導く。

署名付き応答が必要な経路でplain JSONや未知Content-Typeが返ったら、互換性不一致として止める。自動的に認証を外した再照会や、署名を不要にする再試行へ変更しない。暗号化を採用した場合は、復号成功だけで署名検証を済ませたとみなさない。

### 2. 問合せと回答の対応をcacheへ持ち込む

cache keyは、提示tokenに対応する安全に扱うfingerprintに加え、AS、認証済みRS identity、tenant、照会結果に影響する追加context、policy世代を分離する。userの `sub` だけをkeyにすると、同じ利用者の別token・別権限・別RSの結果が混ざる。fingerprintも外部ログへ広く公開する識別子にはしない。

RFC 9701の必須claims集合には、要求した生tokenまたはそのhashのechoはない。これは仕様の項目からの観察であり、「署名によりこのHTTP要求のtokenとの対応まで自動検証できる」とは主張しない。RS内部で送信要求と受信回答の対応を保持し、その対応が確かな結果だけを同じcache keyへ保存する。任意の呼出元から持ち込まれた回答JWTをcacheへ直接注入するAPIは設けない。

### 3. 応答の鮮度とtoken期限を独立した上限にする

設計例では、再利用終了を次のうち最も早い時刻で決める。

- 最初の受信時刻 + 配備で定めたcache TTL
- 外側iat + 配備で定めた最大response age
- 内側expがあれば、そのtoken期限
- 外側expがある配備なら、そのJWT期限

最初の二つは本書の設計でありRFC既定値ではない。cache hitごとに受信時刻を更新するsliding TTLや、保存し直してageをゼロに戻す処理は避ける。内側expがなくてもlocal TTLは有限に保つ。逆に、業務上token期限を必須にするなら、RFCより厳しい配備profileであることを明記してASと合意する。

例として、応答作成12:00:00、受信12:00:05、最大age 20秒、cache TTL 30秒、token期限12:00:12なら、cache終了は12:00:12である。token期限が13:00なら、上限は12:00:20となる。これは計算例で、推奨秒数や失効検知SLAではない。AS側の状態反映遅延があれば、RSのcache時間だけでは失効から停止までの総時間を保証できない。

未来のiat、型不正、期限逆転は観測可能な検証失敗にする。時計ずれ予算とcache TTLを別々に設定し、RFC 7662のtoken expを越えるcache延長には使わない。経過時間はmonotonic clockで測り、wall clockの巻戻りで保存期限を延ばさない。外側と内側の値を記録する際も、応答時刻とtoken発行時刻が判別できる名前にする。

### 4. inactive・検証失敗・状態不明を区別する

有効な回答内のinactive、JWT検証失敗、RS認証失敗、ASのtimeout/5xxを別の内部結果にする。booleanの欠落や文字列 `"true"` をactiveへ変換しない。`active=true` でもscope不足、tenant不一致、対象objectの権限不足なら操作を許可しない。

この設計では、期限内かつ当該操作向けに承認済みのcacheだけを再利用し、有効な回答を得られなければ新しい許可を作らない。AS障害をinactiveと偽って永続negative cacheへ入れることも、last-known-activeを期限後に使い続けることも避ける。利用者へのエラー表現は別に設計し、上流の障害・認証失敗の詳細を不必要に漏らさない。

negative cacheを採用するなら期間と対象を独立に決める。問い合わせ過多の抑制と正規tokenが利用可能になった際の回復を試験する。RFC 7662はnegative cacheの万能な安全期間を保証していない。

### 5. 失効後の古い応答による再投入を防ぐ

管理下のcache無効化を実装する場合、無効化後に遅れて戻ったin-flightのactive回答がcacheを再作成しないよう、要求開始時のgenerationと現在generationを照合する。外側iatだけを全順序の識別子にせず、同一秒の回答や並行要求を考慮する。このgenerationはRS側の設計で、RFC 9701が定める失効通知protocolではない。

active hit、inactive、expired cache、署名/type/issuer/audience失敗、AS接続失敗、generation不一致を別々に計測する。token値・RS資格情報・回答全文・個人claimsを通常ログへ出さず、調査用保存が必要なら別の閲覧権限と保持期間を決める。

## 管理下で行う受入れ試験案（未実行）

| 条件 | 本書の設計で確認する結果 |
|---|---|
| 正しい応答で外側exp/subなし、必須claimsと内側objectあり | access-token用schemaを誤適用せず、応答profileと鮮度を検査する |
| 内側iatは古いが外側iatは新しい | token発行時刻を回答年齢に使わない |
| 署名は有効、外側audが別RS | 内側activeに関係なく回答を拒否する |
| active=falseだが外側iss/aud/iatは存在 | 正常なinactive回答として扱い、token用の詳細を要求しない |
| active欠落、null、文字列true | 有効なbooleanのtrueとして通さない |
| 同一AS鍵で署名した回答JWTをaccess token入力へ提示 | RFC 9068経路でtoken-introspection+jwtを拒否する |
| cache TTLより内側expが早い | token期限をcache上限にする |
| 内側expなしでcache hitを繰り返す | finite TTLと最大ageを延長しない |
| 同じsubの別token、または同じtokenで別RS/context | cache keyを分離し別の許可を流用しない |
| active回答のあと失効、再照会前にcacheを利用 | 残る失効反映窓を測り、即時失効と誤称しない |
| AS timeout、plain JSONへの予期しない変更、署名失敗 | 状態不明や不適合を成功へ変換しない |
| 無効化のあと古いin-flight回答が到着 | generationを確認し、cacheへ再投入しない |

## 版・errata・provenance・限界

- 4つのRFC本文を2026-10-04 UTCにnative webで開いた。RFC 9701は2025年1月のProposed Standard。RFC Editor HTMLの初回は429だったためIETF Datatracker全文とRFC Editor TXTで確認した。Datatrackerのlast updated 2026-05-20を新機能の公開日とは扱わない。
- [RFC 7662 errata][base-errata]では4764（Editorial、Verified 2024-01-17）と7607（Technical、Reported 2023-08-17）を確認した。前者は§3.1の登録対象の表記訂正、後者は§2.2の失効主体に関する提案である。Reportedを承認済みの改訂として扱わない。
- [RFC 9068 errata][access-errata]の8802（Technical、Reported 2026-03-04）はissの必須性変更を提案するが、未検証である。RFC 9068 §2.2のREQUIREDを弱める根拠にしない。RFC 9701のerrataは複数の公式入口が取得エラーとなり未確認。RFC 7519のerrata全件照合は実施していない。
- RFC本文の利用条件はBCP 78 / IETF Trust Legal Provisions。Code Componentsのnoticeは9701がRevised BSD、7662・9068・7519がSimplified BSD。errataページ固有のライセンスは確認できずunknownと記録した。本文は独自要約と設計案で、HTTP/JWT例や実装コードは転載していない。repository analysisとmodule昇格は行わない。
- 特定AS、gateway、JWT libraryのRFC 9701対応、暗号化互換性、実際のcache/失効遅延、時刻同期、tenant境界は未検証。DPoP/mTLS等のsender constraintや、操作ごとの業務認可をintrospectionだけで置き換える設計ではない。
- 検索evalは検索可能性の検査である。上記の暗号検証・障害注入・並行処理・実機試験を実施済みとはしない。採用時には必要な失効停止時間と障害時方針を決め、対象製品の版で確認する。

[signed]: https://datatracker.ietf.org/doc/rfc9701/
[base]: https://www.rfc-editor.org/rfc/rfc7662.html
[access]: https://www.rfc-editor.org/rfc/rfc9068.html
[jwt]: https://www.rfc-editor.org/rfc/rfc7519.html
[base-errata]: https://www.rfc-editor.org/errata/rfc7662
[access-errata]: https://www.rfc-editor.org/errata/rfc9068

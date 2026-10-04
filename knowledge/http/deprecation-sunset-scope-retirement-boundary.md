---
{
  "id": "http-deprecation-sunset-scope-retirement-boundary",
  "title": "HTTP APIの終了通知: Deprecation・Sunsetの日付型、scope、停止判断を分ける",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9745 (March 2025), RFC 8594 (May 2019), RFC 9651 (September 2024), RFC 9110/9111 (June 2022), RFC 8288 (October 2017); RFC 9745/8594 errata checked 2026-10-04 UTC",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "api-design",
    "deprecation",
    "sunset",
    "lifecycle",
    "structured-fields",
    "http-date",
    "scope",
    "migration"
  ],
  "sources": [
    {
      "id": "rfc9745-deprecation-lifecycle-20261004",
      "url": "https://www.rfc-editor.org/rfc/rfc9745.html",
      "type": "official_docs"
    },
    {
      "id": "rfc8594-sunset-lifecycle-20261004",
      "url": "https://www.rfc-editor.org/rfc/rfc8594.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9651-deprecation-date-20261004",
      "url": "https://www.rfc-editor.org/rfc/rfc9651.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9110-sunset-http-date-20261004",
      "url": "https://www.rfc-editor.org/rfc/rfc9110.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9111-sunset-cache-lifetime-20261004",
      "url": "https://www.rfc-editor.org/rfc/rfc9111.html",
      "type": "official_docs"
    },
    {
      "id": "rfc8288-deprecation-link-context-20261004",
      "url": "https://www.rfc-editor.org/rfc/rfc8288.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9745-deprecation-errata-20261004",
      "url": "https://errata.rfc-editor.org/search/?rfc_number=9745",
      "type": "official_docs"
    },
    {
      "id": "rfc8594-sunset-errata-20261004",
      "url": "https://errata.rfc-editor.org/search/?rfc_number=8594",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# HTTP APIの終了通知を停止・移行の実行条件にしてよいか

## 問いと採用判断

APIの応答に `Deprecation` と `Sunset` が付いた。クライアントはいつ移行すべきか。日時を過ぎたら通信を禁止し、保存データを削除し、リンク先へ認証情報付きで自動転送してよいか。

**判断:** 通知を依存先の移行計画に取り込む一方、運用停止・データ削除・接続先変更は別の実行条件にする。二種類の日付、通知対象のresource、policyの適用範囲、実際の可用性を分離して記録する。以下の「確認した契約」は一次資料の要約、「設計案」「試験案」は独自の提案である。

本稿は2025年3月のRFC 9745と2019年5月のRFC 8594による**既存の未収録領域**を補う。2026年10月の新機能とは扱わない。既存の[HTTPキャッシュ](caching-cache-control-conditional.md)は応答再利用、[再試行と冪等性](retry-idempotency.md)は再送可能性を扱う。本稿はAPIのlifecycle通知から移行判断を作る範囲に絞る。

## 確認した契約

### 1. DeprecationはDate型の任意通知

[RFC 9745 §2–5, §7](https://www.rfc-editor.org/rfc/rfc9745.html)の要点:

- 値はStructured Headerの**ItemかつDate**。過去の非推奨化と将来の予定を表せる
- 既定の対象は応答に関連するresource。API全体などへの拡張scopeは提供者が定義できるが、その規則を知らないconsumerには伝わらない
- 情報は任意のhintであり、欠けても動作できる必要がある。`rel="deprecation"` はpolicyや移行説明へのリンクで、日付通知より先に出すこともできる
- 併記したSunset時刻はDeprecation時刻より前であってはならない（MUST NOT）。逆転時は提供者に確認することがSHOULD
- §5は通知それ自体がresourceの意味・機能を変更しないとする。一方、§7は過去の非推奨日時なら従前と同じ挙動の継続を仮定しないことをMUSTとし、非機能特性への影響にも触れる。**「即座に利用不能」と「無期限の同等性保証」のどちらにも読み替えない**

### 2. Sunsetは停止予定のhintであり可用性保証ではない

[RFC 8594 §3–6, §8](https://www.rfc-editor.org/rfc/rfc8594.html)で確認した契約:

- 値は単一のHTTP-date。未来日時がSHOULDで、過去ならいつ利用不能になってもよい状態として捉えるのが安全
- 日時まで使える保証も、日時後に使えなくなる保証もない。終了後の4xx、3xx、接続不能などのどれになるかも決めない
- 通知日時は変更・撤回され得る。既定scopeは返答したresourceで、拡張scopeを知らないclientはその範囲を認識しない
- cacheのfreshnessとは別の意味を持つ。`rel="sunset"` はretirement policyへの情報リンク
- policyに基づく動作では真正性・正確性と対象scopeを確かめる。自動切替の方法・実行時期は本RFCの規定外

### 3. 同じ日付parserには渡さない

[RFC 9651 §2, §3.3.7, §4.2, §4.2.9](https://www.rfc-editor.org/rfc/rfc9651.html)のDateは、1970-01-01T00:00:00Zからの秒差を `@` に続けて表す。ミリ秒ではなく、うるう秒を数えない。RFC 8941だけに対応する旧parserはDateを扱えない。

Structured Fieldsの構文解析に成功しても、そのfieldの許可型に合うかは別の検査である。同名field lineはカンマ結合して解析し、parse failureではfield value全体を無視するかHTTP message全体をmalformedとして扱う。先頭の読めた日付だけを採用する仕様ではない。

Sunsetに使うHTTP-dateは、[RFC 9110 §5.6.7](https://www.rfc-editor.org/rfc/rfc9110.html#section-5.6.7)では送信時にIMF-fixdateをMUSTとし、末尾のzone表記を `GMT` と定める。時刻の意味はUTC。受信parserはIMF-fixdateと二つの旧形式を受理するMUSTがあるため、送信形式の限定を「旧形式は受信拒否」と取り違えない。

**資料間の例に注意:** RFC 9745 §4のSunset例には `UTC` が載っているが、HTTP-date生成の根拠は上の文法に置く。本稿の独自例は `GMT` を生成する。RFC 9745/8594の公式errata検索結果は取得時点で該当なしだったため、これを「修正済みerratum」とは記録しない。

### 4. Expiresとリンクcontextにも別の契約がある

[RFC 9111 §5.3](https://www.rfc-editor.org/rfc/rfc9111.html#section-5.3)の `Expires` は応答がstaleとみなされる日時で、元resourceが変更・消滅する日時を意味しない。したがって同じHTTP-dateの見た目でも、API終了時刻の欄にそのまま流用できない。

[RFC 8288 §3.1–3.2, §5](https://www.rfc-editor.org/rfc/rfc8288.html#section-3.2)ではLinkのtargetとcontextを分ける。`anchor` があればcontextを変更する。anchorを処理できない用途ではリンク全体を無視できるが、anchorだけ落としてリンクを使うことはMUST NOT。別resourceに対する第三者のassertionは無条件に信用できず、自動追跡やCookie等の漏洩リスクも考慮する。

**組合せからの判断:** policy URL、移行先API URL、通知対象resourceは別々に保持する。Linkのanchorを指定しただけでDeprecation/Sunset header自体のscopeがAPI全体に拡張されたとはみなさない。

## 実用上の設計案

### 提供側: 一つの設定から通知・説明・停止計画を検査する

次は構文説明用に作った独自例である。非推奨開始を2026-11-01 00:00:00 UTC、終了予定を2027-01-31 00:00:00 UTCと置いた。実APIの予定でも、実装試験済みの設定でもない。

```http
Deprecation: @1793491200
Sunset: Sun, 31 Jan 2027 00:00:00 GMT
Link: <https://api.example.test/policies/v1>; rel="deprecation"; type="text/html"
```

公開前のcheckでは、日付をinstantに変換して順序を比較する。文字列比較や「どちらも日付だから同じserializer」という処理にしない。gatewayのrouteにpolicy識別子を割り当て、対象が単一resource、API version、特定操作のどれなのかを説明ページと照合する。親pathへの通知を子pathすべてに広げる独自規則は、clientが知っている明示契約に限定する。

停止作業には別のgateを置く。担当owner、利用者への案内、後継との互換確認、rollout/rollback方法、許容できる残存利用量を揃える。header送信件数は、利用者が通知を読んで移行した証拠ではない。「旧版へのアクセスが直近ゼロ」も、休眠clientや月次jobまで存在しない証明にはならない。どの観測期間を採用したかを停止判断に残す。

### 消費側: 通知を依存先台帳に取り込み、実行は分離する

以下は推奨するローカル判断であり、RFCがclientにこの状態機械を要求しているわけではない。

| 観測 | 台帳・担当者への扱い | 自動的には行わないこと |
|---|---|---|
| 有効な未来Deprecation | 発効日と対象を記録し、移行調査に期限を付ける | 成功応答をエラーに変換する |
| 過去Deprecation | 現在の業務影響と後継の互換性を確認する | 日付だけでendpointを停止済みにする |
| 過去Sunsetでも通常呼出し成功 | 予定超過として再確認する | 無期限延長と判断する、保存データを削除する |
| SunsetがDeprecationより早い | inconsistent metadataとして提供者に確認する | 勝手に並べ替える、早い方へ統一する |
| Linkのみ | policyの発見として記録する | 非推奨開始日を現在時刻で補う |
| 通知がない・途中から欠落 | unknownとして扱い、既知の予定を要再確認にする | サポート継続や予定撤回を確定する |

記録には「受信resource、観測時刻、通知日時、取得したpolicy URL、確認した対象scope」を残す。tenantやdeploymentで予定が違う運用なら、それらの識別子も台帳keyに含める。raw token、個人情報入りURL、認証headerは記録対象にしない。

欠落・不正な通知を無効扱いにするSDK設計では、通常の業務応答と診断channelを分ける。Date以外のItemは受理せず、parse failureならfield全体を無視する選択を明示する。API成功を独自に失敗へ変える必然性はないが、HTTP層がmessage全体をmalformedとして処理した結果も握り潰さない。

policyの取得は業務requestのcritical pathから切り離し、サイズ・時間・redirect先を制限する。未確認のLink先へ業務APIのAuthorizationをそのまま付けない。リンクされた説明は移行先の認証・schema互換性の代わりにならないため、切替は通常の変更reviewに載せる。一般的なURL取得対策は既存の[SSRF境界](../security/ssrf-fetch-dns-redirect-boundary.md)も参照する。

## 境界を試す最小ケース（独自の試験案、未実行）

- **型検査:** `Deprecation: ?1` はStructured FieldsのBooleanとして読めてもDateではない。`true` や引用した日付も「非推奨=true」へ補正しない
- **単位検査:** サンプルの1793491200を2026-11-01 UTCに変換でき、ミリ秒へ誤解釈しない。`@1793491200.5` はDateの整数契約に合わない
- **重複field:** 二つのDeprecation行を結合した `@1793491200, @1801353600` を単一Itemとして採用しない。先勝ち・後勝ちで予定を選ばない
- **二種類の日付:** Sunsetの送信はGMT、HTTP-date受信は旧形式も対象にする。Deprecationの `@` をSunsetへ流用しない
- **日付順序:** 同一瞬間のDeprecationとSunsetは「より前」ではない。Sunsetが1秒前の例は矛盾として止め、境界を `<=` と `<` で取り違えない
- **scope分離:** `/v1/orders/42` の観測だけで `/v2/orders/42` や別tenantを非推奨にしない。home documentだけに出す通知は、明示したAPI全体scopeのclientでのみ拡張解釈する
- **リンクcontext:** anchor付きpolicyリンクを処理できないclientは全体を無視し、anchorだけ除いて現在resourceのpolicyとして登録しない
- **独立した時計:** Expiresが先に過ぎても移行完了扱いにしない。Sunset後の200、終了予定の延期、header一時欠落を入力して、削除・切替を発火させず再確認へ回す

## 陥りやすい誤りと適用限界

- 非推奨化を単一のbooleanに圧縮すると、予定日時・対象・未確認の区別が失われる
- HTTP status、cacheの再利用判断、lifecycle通知を同じ失敗flagにすると、必要な移行調査と正常業務処理が混線する
- 提供者の停止承認とconsumerの移行承認は別の作業である。これらのheaderはどちらの作業完了ackでもない
- 通知を認識しないclient、middleboxによるheader除去、browserでのheader可視性、実SDKのRFC 9651対応は別途試験する。今回これらの製品互換性や実ネットワークでの動作は確認していない
- JSONの特定fieldや個別query parameterの廃止契約は本稿の対象外。URI resource向けの通知だけから任意のschema変更を推論しない
- RFCの宣言から、契約上のSLA、最低告知日数、法的な保存・削除期限を導出しない。これらは提供者との個別契約の確認事項である

## 出典・版・ライセンスと再確認

2026-10-04 UTCに上記6 RFCをnative webで開き、該当節と著作権表記を確認した。版はRFC 9745（March 2025、Standards Track）、RFC 8594（May 2019、Informational）、RFC 9651（September 2024）、RFC 9110/9111（June 2022）、RFC 8288（October 2017）。公開日が月単位の資料を架空の日付で細分化していない。

[9745のerrata検索](https://errata.rfc-editor.org/search/?rfc_number=9745)と[8594のerrata検索](https://errata.rfc-editor.org/search/?rfc_number=8594)は、ともに取得時点で「No matching errata found」を返した。旧URL `www.rfc-editor.org/errata/rfc9745` と `/errata/rfc8594` の取得は失敗したため、各RFC infoページが指す公式の新ホストへ辿って確認した。後日のerrata不存在や他RFC全体のerrata監査を保証する記録ではない。

RFC本文はBCP 78 / IETF Trust Legal Provisionsの対象。抽出Code Componentsについて9745/9651/9110/9111はRevised BSD、8594/8288はSimplified BSDとの表記を確認した。ここでは短い原文識別子と独自の日本語要約・独自例のみを使い、RFC実装codeは取り込んでいない。errata検索ページ固有の明示licenseは未確認のためcatalogをunknownとし、検索結果の事実のみ記録する。

全sourceはofficial_docsでTTLは90日、技術httpに固有TTLはないため、明示的な再確認期限は2027-01-02。通知を停止作業へ昇格する前には、これより早くても実提供者のpolicyと予定、使用clientのparserを改めて確認する。

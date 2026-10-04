---
{
  "id": "http-ratelimit-quota-scheduling-cache-boundary",
  "title": "HTTP RateLimit 草案11: quotaの観測値・送信予算・cache・旧版移行を分ける",
  "kind": "knowledge",
  "technology": "http",
  "version": "draft-ietf-httpapi-ratelimit-headers-11 (2026-05-23, expires 2026-11-24); historical draft-07 (2023-06-24); RFC 9651, RFC 9110, RFC 9111, RFC 6585; verified 2026-10-04 UTC",
  "tags": ["research-domain:api-distributed", "http", "RateLimit", "RateLimit-Policy", "quota", "partition-key", "Retry-After", "current_age", "structured-fields", "draft-11", "admission-control"],
  "sources": [
    {"id": "http-ratelimit11-quota-scheduling-20261004", "url": "https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-11", "type": "official_docs"},
    {"id": "http-ratelimit07-dictionary-history-20261004", "url": "https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-07", "type": "official_docs"},
    {"id": "rfc9651-ratelimit-parsing-20261004", "url": "https://www.rfc-editor.org/rfc/rfc9651.html", "type": "official_docs"},
    {"id": "rfc9110-ratelimit-retry-after-20261004", "url": "https://www.rfc-editor.org/rfc/rfc9110.html", "type": "official_docs"},
    {"id": "rfc9111-ratelimit-cache-age-20261004", "url": "https://www.rfc-editor.org/rfc/rfc9111.html", "type": "official_docs"},
    {"id": "rfc6585-ratelimit-429-20261004", "url": "https://www.rfc-editor.org/rfc/rfc6585.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/api-distributed.json"]
}
---

# HTTP RateLimit 草案11: quotaの観測値・送信予算・cache・旧版移行を分ける

## 問いと採用判断

API応答に残量があれば、複数workerはその数だけ送ってよいか。窓の終了時刻に全量を補充してよいか。**RateLimitはserverが返した観測・助言として扱い、clientの送信予約とserverの受付判定を別に持つ。** 正の残量は受付保証ではなく、待機の終了も残量回復の約束ではない。

2026-10-04 UTCに[現行datatracker](https://datatracker.ietf.org/doc/draft-ietf-httpapi-ratelimit-headers/)と[固定draft-11](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-11)を開いた。公開日は2026-05-23、期限は2026-11-24で、Active Internet-Draft、WG Document、I-D Existsと表示される。確定RFCとして採用する記事ではない。直近の製品releaseを見つけたという主張でもなく、既存の[retryと冪等性](retry-idempotency.md)が扱わない「送信前のquota調整」を埋める調査である。

## 1. 旧Dictionaryと現在のListを同じ構文として扱わない

[draft-07 §3.1](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-07#section-3.1)はRateLimitをDictionaryとし、limit・remaining・resetを定義していた。2023-06-24公開、2023-12-26失効の旧版であり、現行契約の根拠にはしない。[draft-11の変更記録](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-11#name-changes)は、draft-07以後にpolicyを識別するList itemsへ再構成し、quota unitとpartition keyを追加したと記す。これらをdraft-11で初めて導入した機能とは断定しない。

現在の[§3–4](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-11#section-3)では、両fieldはStructured FieldsのListで、policyごとのItemにparameterを付ける。RateLimit-Policyは比較的安定した方針、RateLimitは変動するservice limitを表す。以下は確認した型と制約の要約である。

| 項目 | draft-11の契約 | 実装で分けること |
|---|---|---|
| RateLimit-Policyのpolicy識別子 | String。qは必須の非負Integer | policy名とquota値を混ぜない |
| qu | 任意のString、省略時requests。定義単位はrequests・content-bytes・concurrent-requests | byte枠や同時実行枠をリクエスト件数へ勝手に換算しない |
| w | 任意の正のInteger、秒。w=0は不可 | policyの時間幅と現在の残り時間は別 |
| RateLimitのr | 必須の非負Integer、対応policyの利用可能quota | r=0も有効。所有権tokenや予約済み枠ではない |
| t | 任意の非負Integer、秒。t=0は型上許される | tの省略をゼロに補完しない。除算と待機を別に検査 |
| pk | 任意のByte Sequence、quotaのpartition key | Stringや認証credentialとして扱わない |

両fieldはtrailerに置けない。複数field lineで送られる場合もある。[RFC 9651 §4.2](https://www.rfc-editor.org/rfc/rfc9651.html#section-4.2)は同名のfield lineを結合して解析し、§2.2は解析失敗時にfield全体を無視する扱いを説明する。draft-11 §7もmalformedなRateLimit fieldsを無視するMUSTを置く。文字列の単純splitや、壊れたmemberを除いて都合のよい残量だけ拾うparserを採用しない。これはHTTPメッセージ一般の構文違反を許容する指示ではない。

[RFC 9651 §3.3.1](https://www.rfc-editor.org/rfc/rfc9651.html#section-3.3.1)のIntegerは最大15桁の符号付き範囲である。任意精度のJSON数として受け入れればよいわけではなく、さらにq/r/t/w固有の符号・ゼロ条件を検証する。未知の拡張parameterは草案が許すので、既知parameterの型不正と未知parameterの存在を区別する。

## 2. 残量・時間・statusは三つの情報

[draft-11 §4.1.1–4.1.2、§6–8](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-11#section-4.1.1)によると、rが正でも次の要求の成功は保証されない。tはその時間内にrを超えて使わないためのeffective windowであり、全量の回復時刻ではない。次の応答でrが下がったりtが延びたりすることもある。全応答にfieldが付く保証、全policyの広告、特定のthrottling algorithmもない。

したがって200にr=0が付いても、その200の業務成功を429へ読み替えない。反対に残量が正でもエラーを成功扱いにしない。失敗応答がquotaを消費するかはservice固有で、同一応答のstatusだけからカウンターを戻す根拠にはならない。

同じ応答にRetry-Afterがある場合は、草案§7の**Retry-After precedence**が適用される。tを短い待機時間として優先しない。[RFC 9110 §10.2.3](https://www.rfc-editor.org/rfc/rfc9110.html#section-10.2.3)ではRetry-AfterはHTTP-dateまたは受信後に遅延する秒数で、tのIntegerとはparserを分ける。[RFC 6585 §4](https://www.rfc-editor.org/rfc/rfc6585.html#section-4)は429へのRetry-After付与をMAYとし、429のキャッシュ保存を禁止する。RateLimitの有無だけで再試行可否や副作用の冪等性は決まらない。

## 3. cacheの本文が新鮮でもquotaは現在値とは限らない

[草案§7.3](https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers-11#section-7.3)では、cacheから来た応答のRateLimit情報を無視するSHOULDがあり、その判定条件としてpositive current_ageを示す。cacheの本文は再利用できても、その生成時の残量で新しい送信枠を増やすべきではない。

current_ageを単純な「Ageヘッダーが正か」に置き換えない。[RFC 9111 §4.2.3](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.2.3)はDate、応答遅延、Age、保存後の経過時間を使って年齢を計算する。Ageがない場合はage_valueを0に置くが、current_age全体が0になるわけではない。よってAge: 0やAge欠落を、必ずorigin直送である証拠にしない。

独自の実装案として、HTTP層がcache由来と年齢を把握できるなら、その情報をquota処理へ渡す。受信APIから判別できない場合は「quotaの鮮度不明」を保持し、その応答で送信量を増やさない保守的な扱いにする。毎回no-cacheを付けてoriginへ到達しようとするのは、quota観測のために負荷を増やす別判断であり、本稿の既定案ではない。

## 4. 複数worker向けの送信管理（独自の設計案）

以下は草案の指定algorithmではなく、観測値を過剰な予約へ変えないためのclient設計である。

1. **適用先を固定する。** origin・既知の認証/route範囲・policy・pkの対応をprovider契約として管理する。pkはserverが選ぶpartition識別子で、全service共通の名前空間でも認可でもない。pk欠落から「全ユーザー共通枠」と推定しない。草案§6.1はuser・application・method・resource等の組合せを許すため、pathだけで同じ枠と決めない
2. **観測とローカル予約を別にする。** たとえばr=24を見た三つのworkerが各24件を予約すると72件になる。応答値をworkerごとの新規token補充イベントとして加算せず、対象範囲ごとに共有する送信管理へ渡す。ローカル予約もserverの予約ではなく、別clientやproxy再送の消費まで把握できない
3. **同時応答を単純上書きしない。** 後から届いた応答がserverで後に生成されたとは限らない。観測時刻・対象partition・進行中の予約を持ち、古い残量で急に並列度を増やさない。この草案にglobal sequenceや全consumer共通の残量合意があるとはみなさない
4. **複数の制約を単位別に満たす。** 同じ要求へ適用される時間枠、byte枠、concurrency枠を足して一つの整数にしない。既知の制約を個別に満たすようにし、広告されたpolicyだけが全制限だとも仮定しない。t省略やt=0をr/t計算へ通さず、provider契約と局所の安全上限へ戻す
5. **待機を有限にする。** Retry-Afterと実行deadline・cancelを両方扱う。待機がdeadlineを越えるなら延期や失敗として上位へ返す。待機上限でRetry-Afterを短縮して早く再送することとは違う。大きなrを理由に局所の並列数上限を外さず、窓の境界で全workerを一斉再開しない
6. **欠落と不正を無制限へ変換しない。** malformed fieldを無視した後も既存の局所上限は残す。field欠落だけで最後の観測を永久固定することも避け、情報の寿命とfallbackをserviceごとに決める。未知の単位をrequest数と読み替えて「使える」と判定しない

server側の受付制御は別途必要である。草案§8.1がいうように、fieldを送ってもclientの送信そのものは止まらない。clientの協力だけに依存せず、認証・容量保護・受付判定を実装する。partition keyに生の個人識別情報を載せたり、quotaログを無制限のmetric labelへ展開したりしない。後者は本稿の運用提案で、草案が特定のobservability方式を指定するものではない。

## 5. 導入前の受け入れ試験案（未実行）

| 入力・状況 | 確認する境界 |
|---|---|
| draft-07 Dictionary、draft-11 List、独自X-RateLimit形式 | providerと版を指定したparserへ分岐する。同名だから相互運用可能としない |
| w=0、t=0、r欠落、rが負数、15桁超、分割field lineの片側不正 | 型とfield固有制約を分離し、壊れたfieldの一部だけ採用しない |
| 200 + r=0、429 + 正のr、次の応答でt増加 | 元のHTTP結果を維持し、受付保証・全量補充へ変換しない |
| tより長いRetry-After、HTTP-dateのRetry-After、期限より長い待機 | precedenceを守り、deadline超過を早期再送で解消しない |
| positive current_ageの200、Age: 0だが保存経過あり | 本文再利用とquota採用を分離する |
| 同じpkの複数worker、別originで同じpk、応答順の逆転 | 残量の二重加算・別providerの混同・古い観測による加速を検知する |
| requestsとcontent-bytesの併記、concurrent-requests、pk/t欠落 | 単位・scope・時間の不明をゼロや無制限へ埋めない |
| 巨大な待機値、fieldの断続的欠落、局所queue満杯 | cancel、局所上限、延期/拒否の経路を保つ |

## 版・provenance・未確認事項

- 固定draft-11と旧draft-07、RFC 9651（2024年9月）、RFC 9110/9111（2022年6月）、RFC 6585（2012年4月）の実際の本文を2026-10-04 UTCに開いて確認した。旧版は移行差分の根拠に限る
- draft-11は作業中の仕様。provider、API gateway、client SDK、browserがこの構文を実装するかは未確認であり、広範な互換性を主張しない。draft-10→11だけの全差分、新しいproblem typeの登録完了、CORS経由のheader公開設定は本稿の範囲外
- 上表は試験案であり、wire test・負荷試験・SDK実行・障害注入は未実行。検索evalは文書へ到達できるかだけを検証する。実行可能なrate limiter実装は追加していない
- 各文書のCopyright NoticeでBCP 78 / IETF Trust Legal Provisionsを確認した。Code Componentsはdraft-07/11とRFC 9651/9110/9111がRevised BSD、RFC 6585は当時のSimplified BSD表記。RFC 9110/9111には2008-11-10以前の素材に関する追加制約のnoticeもある。本文は独自要約・設計案であり、コード・図・既存実装の転載なし
- official_docsのTTLは90日だが、進行中草案なので明示期限を2026-11-03へ短縮する。草案期限2026-11-24の前に版・状態・本文を再確認する。日付だけの更新で確定仕様扱いへ変更しない

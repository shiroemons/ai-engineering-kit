---
{
  "id": "security-http-framing-downgrade-smuggling-boundary",
  "title": "HTTP request smuggling の防御境界: body framing・HTTP/2変換・接続再利用",
  "kind": "knowledge",
  "technology": "security",
  "version": "RFC 9110 / RFC 9112 / RFC 9113 (June 2022); relevant errata status rechecked 2026-10-03 UTC",
  "tags": ["research-domain:security", "http", "request-smuggling", "framing", "content-length", "transfer-encoding", "http2", "downgrade", "gateway", "connection-pool"],
  "sources": [
    {"id": "rfc9112-request-framing-20261003", "url": "https://www.rfc-editor.org/rfc/rfc9112.html", "type": "official_docs"},
    {"id": "rfc9113-downgrade-validation-20261003", "url": "https://www.rfc-editor.org/rfc/rfc9113.html", "type": "official_docs"},
    {"id": "rfc9110-content-length-forwarding-20261003", "url": "https://www.rfc-editor.org/rfc/rfc9110.html", "type": "official_docs"},
    {"id": "rfc9112-framing-errata-20261003", "url": "https://errata.rfc-editor.org/search/?rfc_number=9112&presentation=records", "type": "official_docs"},
    {"id": "rfc9113-framing-errata-20261003", "url": "https://errata.rfc-editor.org/search/?rfc_number=9113&presentation=records", "type": "official_docs"},
    {"id": "rfc9110-content-length-errata-20261003", "url": "https://errata.rfc-editor.org/search/?rfc_number=9110&presentation=records", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# HTTP request smuggling: 普通のAPI要求の終端を全hopで一致させる

## 問い・選定理由・対象

CDN、gateway、backend が一つの要求の終端を異なって解釈するとき、入口で認証・認可した要求と backend が処理する要求をどう一致させるか。特に、外側が HTTP/2 でも内側で HTTP/1.1 に変換する経路を対象とする。

2026-10-03 UTC に検索した `Transfer-Encoding`、`framing`、`HTTP/2 downgrade` は既存文書に一致しなかった。既存の [CONNECT先行送信](../http/optimistic-connect-upgrade-rejection-boundary.md) は HTTP と tunnel の切替え、[SSRF](ssrf-fetch-dns-redirect-boundary.md) は宛先の選定を扱う。本稿は通常の HTTP 要求の body framing と変換後の接続再利用という、別の未収録の実務課題である。

これは2026年の新規防御仕様の紹介ではない。2022年6月の3 RFC と現在の errata を照合し、変わっていない契約を設計レビューへ落とす。対象は管理下の gateway を通るサイズ上限付きの書込みAPI。CONNECT、Upgrade、HTTP/3、製品別の脆弱性、攻撃用payloadは扱わない。

## 一次資料で確認した契約

以下の MUST / MAY 等は原文の規範強度を示す。後半の厳格な入口ポリシーとは区別する。

### HTTP/1.1: 優先順位と拒否後の接続

[RFC 9112 §6.1–6.3](https://www.rfc-editor.org/rfc/rfc9112.html#section-6.3):

| 入力 | 仕様上の扱い |
| --- | --- |
| `Transfer-Encoding` と `Content-Length` が共存 | 送信は禁止。受信serverは拒否またはTransfer-Encodingだけで処理できるが、どちらでも応答後の接続closeはMUST。中継を選ぶintermediaryは元のContent-Lengthを除去し、transfer codingを処理してから転送するMUST |
| 要求のtransfer codingの最終段がchunkedでない | serverは400応答と接続closeがMUST |
| Transfer-Encodingがなく、不正なContent-Length | 同一の有効な値だけからなるリストの例外を除き、回復不能なframing error。要求なら400応答と接続closeがMUST |
| 有効なContent-Lengthより短い受信で切断・timeout | 不完全なmessageとして接続closeがMUST |

転送時に片方のheaderを消すだけで、残ったbodyの符号化や境界も正しくなったとは限らない。変換後のmessageを一貫して生成する必要がある。

### Content-Length: 値を信用する前に契約を確定する

[RFC 9110 §8.6](https://www.rfc-editor.org/rfc/rfc9110.html#section-8.6) の Content-Length は非負の十進整数で表すoctet数。整数overflowや精度損失への対策はMUSTであり、既知の誤った値を付けたまま転送してはならない。構文不正値の転送も禁止されるが、同じ十進値だけのリストは、拒否または単一値への置換がMAYである。

したがって「重複Content-Lengthは標準が常に拒否を強制する」とは書けない。一方、任意の複数値から先頭だけを採用する許可でもない。本稿は互換性より境界の単純さを優先する入口ポリシーを別に選ぶ。

### HTTP/2: frameが有効でもmessageが有効とは限らない

[RFC 9113 §8.1.1・§8.2・§10.3](https://www.rfc-editor.org/rfc/rfc9113.html#section-8.1.1) は、content-lengthと実content長の不一致をmalformedとする。contentを持たない応答には別条件があるため、要求の検査をHEAD応答等へ機械的に流用しない。

- 検出したmalformed messageは転送禁止、`PROTOCOL_ERROR` のstream errorとして扱うMUST。これは一律にHTTP/2接続全体を閉じる規則ではない
- 値中のCR/LF/NUL、値の先頭・末尾のSP/HTAB等は不正。HTTP version変換前のfield検証はMUST。pseudo-headerからrequest lineを作る場合は、通常fieldの検査だけでは不十分
- HTTP/2の `Transfer-Encoding` / `Connection` 等はmalformed。別fieldである `TE` は要求で `trailers` のみ許容される

同§8.1.1は、全受信前に転送・処理を始めていて、後から不正を発見する場合も認めている。streamをresetしたことから、それ以前の副作用が取り消されたとは導けない。

## 設計判断: bounded bufferingを使う書込みAPI（独自提案）

以下は上の仕様を踏まえた本稿の配備案であり、RFCが全gatewayへ必須化した手順ではない。確認済みparserを持つHTTP実装を利用し、正規表現だけでframingを自作しない。

### 1. 入力と転送を別の状態にする

このAPIでは、header検証、上限内へのbody受信、message完了検証、backend送信、完了または失敗、という状態を分ける。bodyは最大サイズ・全体deadline・同時受信数を制限した一時領域に保存し、message完了までbackendへ送らない。最大値は製品要件と容量試験で決める。無制限にメモリへ全量を溜める実装は採らない。

この選択の利点は、遅れて判明する長さ不一致もbackend送信前に止められること。代償は追加の待ち時間、一時領域I/O、同時アップロード容量である。大容量uploadや双方向streamingへ、そのまま拡張できる方針ではない。

入力のoctet数と、JSONをparse・再整形した後の文字数を混同しない。検証対象のbodyと実際に送るbodyの対応を保持する。後段で内容を書き換える機能を追加するなら、framingの再生成と内容検証の責任もその変更に含める。

### 2. 曖昧な要求を入口で修復しない

この配備ではHTTP/1.1のTransfer-EncodingとContent-Lengthの共存、重複Content-Length、異なる長さ値、不正なfield区切りを拒否する。重複が同値でも拒否するのは本稿の厳格ポリシーであり、RFCの唯一の適合動作ではない。拒否率と正規clientへの影響を測り、互換性例外が必要なら特定経路だけで再審査する。

HTTP/2で不正なfieldを見つけた後に「HTTP/1.1なら読める形」へ修復して再送するfallbackは設けない。逆方向のHTTP/1.1→HTTP/2変換でhop固有fieldを除去することと、既にHTTP/2として届いた禁止fieldを黙って受理することを混同しない。

### 3. 変換の出力は構造化されたmessageから作る

全hopの受信protocol・送信protocol・parserの版と設定を台帳化する。browserからgatewayまでのALPNがh2でも、backend接続のprotocolは別に確認する。正常時だけでなく、retry先・障害時fallback・service mesh経由の経路も対象にする。

このbuffered profileからHTTP/1.1へ送る際は、完了済みbodyの正確なoctet数から単一のContent-Lengthを生成し、受信したraw header blockを再利用しない。method・path・authorityも対応する構文の構造化値からserializerに渡す。文字列連結でrequest lineを組み立てない。Content-Lengthのない正当なHTTP/2要求でも、body完了を検証できれば同じ方針で送出できる。

入力でのrouteと認可に使った値が、出力のrequest targetでも同じ意味を持つことを試験する。ただし、完全なURI正規化・host routingの規則やTLS identityは本稿の範囲外であり、framing検査だけで認可全体を保証しない。

### 4. 拒否対象のstreamと汚染し得る接続を区別する

buffered profileでは、不正要求をbackend接続へ書く前に拒否する。この場合、当該要求のために正常なbackend接続を破棄する必要はない。HTTP/1.1受信側のframing errorでは接続を終え、残余byteを次の業務要求として処理させない。

一方、既存のstreaming経路を残すなら、後から不整合が見つかった時点で何byteをどのbackend接続へ書いたかを把握する必要がある。本稿の推奨は、そのHTTP/1.1接続を再利用不可として終了し、残余データを別要求へ回さないこと。外側のHTTP/2 streamをresetするだけで、内側のHTTP/1.1接続が安全になったとは扱わない。

既に実行された業務処理の取消しやexactly-onceは別問題である。受信失敗から無条件に書込みを再試行せず、業務操作ID・実行記録・結果照会の契約を確認する。bufferingを省く変更には、この障害時意味論の再設計が必要になる。

### 5. 入口とbackendの両方を観測する

ログにはprotocol pair、拒否理由、route ID、接続再利用可否、試験用相関IDを残す。認証headerやraw bodyを一律に収集しない。HTTP statusだけで成功を判定せず、backendのhandler実行数と送信された要求境界を照合する。

## 管理下の受入れ試験案（未実行）

隔離環境のstub gateway/backendと無害なfixtureで行う。第三者サービスや本番環境への攻撃試験、具体的な攻撃payloadの作成は含めない。

| 条件 | 本稿のprofileで観測する結果 |
| --- | --- |
| 正常なHTTP/1.1、単一Content-Length、上限内body | 完了検証後にbackendへ1要求。出力長が実octet数と一致 |
| Transfer-EncodingとContent-Lengthの共存 | 入口で拒否・HTTP/1.1接続close。backend要求数は0 |
| 同値の重複Content-Length | 厳格ポリシーで拒否。RFCが全実装へ必須化した結果とは記録しない |
| 異なるContent-Length、数値overflow、受信途中timeout | 失敗し、完了扱いや後続要求への残余転用がない |
| HTTP/2のcontent-lengthとcontent長が不一致 | stream error。buffered profileではbackend要求数0 |
| HTTP/2 fieldの禁止文字・禁止Transfer-Encoding | 変換前に拒否。headerを修復したHTTP/1.1要求を生成しない |
| 正常なHTTP/2のTE: trailers | Transfer-Encodingと取り違えて一律拒否しない。別途定めたtrailer取扱いを確認 |
| 片方のHTTP/2 streamが不正、別streamは正常 | streamごとの結果を照合。接続全体closeを唯一の合格条件にしない |
| streaming例外でbackendへ一部送信後に不整合 | 対応するHTTP/1.1接続をpoolへ戻さず、副作用の有無は別途照会 |
| gateway更新、経路fallback、serializer変更 | 同じfixture群を再実行し、入口とbackendの要求数が一致 |

検索evalはこの文書への到達性だけを検証する。この表のprotocol試験、負荷試験、実製品の安全性検証は実行していない。

## 版・errata・由来・限界

- RFC 9110（STD 97）、RFC 9112（STD 99）、RFC 9113はすべて2022年6月公開。今回扱う普通の要求framingに新しい規範変更を確認した、という主張はしない。RFC 9112を更新したCONNECT先行送信の変更は既存の別文書で扱う
- [RFC 9112 errata](https://errata.rfc-editor.org/search/?rfc_number=9112&presentation=records) の7744・8284はVerifiedのEditorial。7633のchunk区切りをbare LFにも広げる提案は2023-11-07にRejected。提案文を現行の許容規則として採用しない
- [RFC 9113 errata](https://errata.rfc-editor.org/search/?rfc_number=9113&presentation=records) の7013は2022-07-06にVerifiedのEditorial。9175は2026-09-13報告、取得時点ではReportedで、OPTIONSのpath/query条件を扱う。本稿はこれを確定した改訂や新しいsmuggling防御として扱わない
- [RFC 9110 errata](https://errata.rfc-editor.org/search/?rfc_number=9110&presentation=records) の7870はContent-Lengthの転送に関する文言提案で、2024-05-14にRejected。本文の規範を提案文へ差し替えない
- 3 RFCのCopyright NoticeはBCP 78 / IETF Trust Legal Provisions、抽出Code ComponentsはRevised BSD。errataページ単体のライセンスは確認できずunknown。本稿は独自の日本語要約・設計案であり、コードやwire payloadを転載していない。repository分析をしていないためcommit SHAは該当しない
- 最初のerrata検索URL2件は取得エラーとなったが、RFC Editorのerrata入口からrecords表示を開いて確認した。本文取得やerrata確認の失敗を、変更なしの証拠にはしていない
- proxy製品・runtime別の修正版、既定parser動作、WAF検出精度、trailerの実装、resource limitの適正値は未検証。HTTP/2を全面有効化しただけで、すべてのsmugglingを解決したとは保証しない
- 取得日は2026-10-03 UTC。official_docsの90日TTLに合わせた再確認期限は2027-01-01。新しい版・errata・配備経路を確認してから更新する

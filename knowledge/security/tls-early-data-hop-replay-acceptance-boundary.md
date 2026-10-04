---
{
  "id": "security-tls-early-data-hop-replay-acceptance-boundary",
  "title": "TLS 1.3 0-RTT: Early-Dataのhop間伝播・425再試行・多拠点replayの受入れ境界",
  "kind": "knowledge",
  "technology": "security",
  "version": "RFC 9846 (July 2026, obsoletes RFC 8446); RFC 8470 (September 2018); RFC 8446 (August 2018) historical comparison; verified 2026-10-04 UTC; errata status unverified",
  "tags": [
    "research-domain:security",
    "tls",
    "0-rtt",
    "early-data",
    "replay",
    "425",
    "reverse-proxy",
    "retry",
    "forward-secrecy",
    "multi-region"
  ],
  "sources": [
    {
      "id": "rfc9846-zero-rtt-replay-20261004",
      "url": "https://datatracker.ietf.org/doc/rfc9846/",
      "type": "official_docs"
    },
    {
      "id": "rfc8470-early-data-hop-425-20261004",
      "url": "https://www.rfc-editor.org/rfc/rfc8470.html",
      "type": "official_docs"
    },
    {
      "id": "rfc8446-early-data-historical-20261004",
      "url": "https://www.rfc-editor.org/rfc/rfc8446.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# TLS 0-RTT: originのhandshake完了だけで受入れを決めない

## 問いと選定理由

CDNやreverse proxyでTLSを終端したあと、origin側のTLS handshakeが完了していれば、到着した要求を通常の1-RTT要求として処理してよいか。**上流hopのearly dataを示す `Early-Data` があれば、その履歴はoriginのhandshake完了では消えない**。本書はTLS設定の有効化から業務処理開始までの受入れ条件を扱う。

2026年7月の[RFC 9846][tls]はRFC 8446を置き換えたTLS 1.3の後方互換改訂であり、TLS 1.4ではない（§1.2）。ただし、0-RTTの接続間replayリスクは2018年の[旧仕様][old]にもあった。今回の価値は「新しく危険になった」という発見ではなく、現行TLS仕様と[HTTPのearly-data契約][http]をつなぎ、未収録だったmulti-hopの運用判断を残すことにある。署名検証や通常の業務retry台帳とは対象が異なる。

## 一次資料で確認した契約

### TLS層の受理回数と業務操作の回数は違う

[RFC 9846 §2.3、§8][tls]は、0-RTTに接続間のnon-replay保証がないことを明記する。最低限のMUSTは、同じ0-RTT handshakeを各server instanceが高々一度受理すること。これは全拠点を合算した高々一度を意味しない。共有state等でさらに抑えることはSHOULDであり、clientは実際の構成を一般には知らない。TLS上のreplayを抑えても、別接続でのapplication retryによる重複はアプリケーション側の課題として残る。

§8.2のClientHello recordingでは、記録windowが起動時刻と重なる間は0-RTTを拒否することがSHOULD。cacheが空になったことを未受信の証明にしない。Appendix F.5は、利用profileを定めずに0-RTTを使うこと、アプリケーションからの指示なしにTLS実装が有効化・拒否dataの自動再送を行うことを禁止する。

### 改訂で明確になったforward secrecyの前提

[旧RFC 8446 §2.3][old]はPSK由来の鍵で暗号化するためforward secretではないと説明していた。[RFC 9846 §2.3、§8.1][tls]は、protocol自体は保証せず、serverの保存・削除動作に依存し、その動作はprotocolでclientに伝わらない、と説明する。database参照型のsingle-use ticketとPSKの全コピー削除にはforward secrecyを得られる条件がある。**session resumptionや新RFCへの準拠だけで機密性の保証が増えたとは判断しない**。replay判定と鍵削除の保証も別である。

### HTTPでは上流hopの危険を保持する

[RFC 8470 §5.1][http]の `Early-Data: 1` は、以前のhopを含めearly dataで運ばれたことと425への対応を示す。途中のintermediaryは受信済みheaderを除去してはならない。現在のTLS状態が通常でも、header付きの要求が安全に処理できなければ425で拒否する必要がある。originが自分のhandshakeを待つだけでは解消しない。

有効値は `1` だけだが、**multipleまたはinvalidな値もserverは1と同等に扱うMUST**がある。たとえば `Early-Data: 0` を「earlyではない」と解釈しない。user agentはheaderを付けなくてもearly dataを送れるため、header欠落だけで最初のTLS終端における通常dataを証明できるわけでもない。

### 425の再試行主体を決める

[RFC 8470 §4][http]はearly dataを使うclientに425受信時のretryをMUSTとする。§5.2ではuser agentの**自動retry**がSHOULDで、そのretryをearly dataに載せることはMUST NOT。intermediaryが**受信した要求に既にEarly-Dataがある**なら425を上流へ返すMUSTがある。一方、自分がearly dataで受信したが受信headerはなかった場合、受信側接続のhandshake完了を待ってretryするMAYがある。どちらも単にorigin側接続が完了したことを条件にしない。

§4では、別の情報がなければclientはsafe methodをearly dataで送るMAYがあり、unsafeまたは安全性不明のmethodを送ることはMUST NOT。一方§3では、safe methodにも副作用を持つresourceがあり、originがresourceの明示情報を持たなければearly dataを拒否するかhandshake完了前の処理を避けるMUSTがある。したがってclient側の一般規則を、origin側のGET一括許可へ置き換えない。

§6.1では、gatewayはoriginがEarly-Dataを理解し425を正しく返すと分かっていなければ、early受信の要求を先行転送してはならない。§5.2はearly受信もheaderもない要求への425をSHOULD NOTとする。一般的な混雑・認証エラーをすべて425に変換する仕様ではない。

## 導入時の判断案（本書独自の設計）

以下は仕様の再掲や特定製品の保証ではなく、構成レビュー用の提案である。

### 1. hopごとの二つの情報を観測する

client → edge → regional gateway → originという経路図に、それぞれのTLS終端、0-RTT許可設定、受信TLSから得るearly-data情報、HTTP headerの入力・出力を記す。現在のhopがearlyかどうかと、上流から引き継いだEarly-Dataの有無を別項目にする。

「originのhandshake済み」という一つのbooleanで前者と後者を上書きしない。headerは本人性・認可の証明には使わず、危険を増やす入力として扱う。headerの欠落を安全の根拠にできるのは、前段が必要な情報を保存するという構成契約を検証した場合に限る。途中の汎用header sanitizationがEarly-Dataを削除していないかも見る。

### 2. 操作名で早期実行を許可する

初期状態は対象routeの早期実行を無効にし、公開された静的応答など、replay・順序逆転・費用増幅の影響を説明できる操作だけを個別審査する。GETでも、単発linkの消費、認証flowの進行、重い検索や外部呼出しを含むなら、method名だけでは採用しない。

審査票には「同じ要求を二回」「別regionで一回ずつ」「通常retryと重なる」「後続要求のあとへ遅延」という反例を残す。認証済みcookieやtokenがあることと、replayして安全なことを同一視しない。業務冪等キーがあっても、重複排除前の課金・外部アクセス・情報漏えいまで抑えるかは別途確認する。

### 3. 受入れ状態を業務副作用の手前に置く

受信、header解析、early-data判定、resource方針照合、通常処理開始を区切る。判定前にメール送信、状態更新、使い捨てtoken消費などを行ってから425を返す実装は採用しない。待機を選ぶ場合は、どの受信接続のhandshakeを待つのかを特定する。上流headerによる拒否をorigin側の待機にすり替えない。

前段の適合性が確認できない経路では、originだけに補正を押し付けず、その経路の入口で0-RTT無効化または確認済みの遅延処理を選ぶ。これは保守的な配備案であり、すべてのHTTP通信を常に遅らせる推奨ではない。

### 4. retryとrollbackを一つの経路で確認する

425を単なる一般的な5xx retry policyへ流さず、誰が受け取り、どの接続を待ち、early dataを使わず再送するかを確認する。intermediaryが上流から受けたheaderと自分のTLS検出を混ぜると、不要な再試行や無限425の原因を追えなくなる。試験ではheaderの各hopの値と、実際に業務処理へ入った回数を対で残す。

retry回数や待機予算に上限を設け、枯渇時は未確定として観測する。425を消して見かけの成功にしない。rollbackは新規early-data許可を止める経路を先に用意し、通常1-RTTへの回復も確認する。ticket / replay記録の再起動・region切替については、台帳が空の新instanceを即座に早期実行へ戻す条件を別に審査する。

## 管理下での受入れ試験案（未実行）

| 条件 | 本書の設計で確認する結果 |
|---|---|
| edgeでearly受信、origin側TLSは完了済み、上流Early-Dataあり | originの完了を理由に早期実行を許可しない |
| Early-Dataが0、未知値、重複field | 値をfalse扱いせず、1相当のリスク判定に入る |
| user agentにheaderがなく、最初のhopだけearly | TLS終端が検出し、必要なheader伝播を行う |
| gatewayが既存header付き要求を受信し、originが425 | gatewayが勝手に解消扱いにせず425を上流へ返す |
| gateway自身がearly受信、受信headerなし、originが425 | retryを採用するなら受信側handshake完了後に行う |
| 名前はGETだが単発tokenを消費するroute | method allowlistだけで早期実行へ入らない |
| 二つのregionで同じ要求、片方では通常retry | TLSのinstance単位受理回数と業務処理回数を別々に検査する |
| replay記録を持つprocessの再起動直後 | recording windowと起動時刻の重なりを方針に反映する |
| 425応答が続く・clientがretryを理解しない | 処理成功へ偽装せず、対応不備と予算枯渇を観測する |

実運用の計測では、0-RTT許可・拒否、上流Early-Dataあり、425、通常経路でのretry完了、業務重複拒否を区別する。通常ログへPSK、ticket本体、認証cookie、要求全文を残す必要はない。

## 適用版・由来・限界

- 3 RFCを2026-10-04 UTCにnative webで開き、対象節とCopyright Noticeを確認した。RFC 9846のRFC Editor HTMLは取得失敗だったが、IETF Datatracker全文とRFC Editor TXTで確認できた。公開月は2026年7月であり、Datatrackerのlast updated 2026-07-11を機能導入日とはしていない。
- RFC 9846は同じTLS 1.3の改訂。本稿のforward secrecy説明の明確化は旧§2.3との比較であり、multi-hopのEarly-Data / 425規則が2026年に新設されたという意味ではない。
- RFC 9846 / 8470のerrataページは複数の公式入口で取得エラーとなり、errata状態は未確認。「errataなし」「全訂正適用済み」とは主張しない。導入前には対象節のerrataを再確認する。
- 出典の文書はIETF Trust Legal Provisions / BCP 78。Code Componentsのnoticeは9846がRevised BSD、8446 / 8470がSimplified BSD。9846 / 8446には古い寄稿部分に関する制限もある。本文は独自要約と設計案で、RFCのコード・図を転載せず、暗号実装やmoduleへの昇格はしない。[IETF Trustの案内][license]も参照した。
- TLS library、CDN、proxy、browser、HTTP/3 stackの実機対応、同じheaderの正規化、retry実装、PSK全コピー削除、multi-region storeの一貫性、性能効果は未検証。HTTP/3固有のframe / transport制約やTLS 1.3の全変更点も対象外。
- 検索evalは文書を検索できるかの検査であり、上記障害注入・packet capture・暗号検証を実行した証拠ではない。採用する製品・版・設定が判明してから試験を具体化する。

[tls]: https://datatracker.ietf.org/doc/rfc9846/
[http]: https://www.rfc-editor.org/rfc/rfc8470.html
[old]: https://www.rfc-editor.org/rfc/rfc8446.html
[license]: https://trustee.ietf.org/documents/trust-legal-provisions/

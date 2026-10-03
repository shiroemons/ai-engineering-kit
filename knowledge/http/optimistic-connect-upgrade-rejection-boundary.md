---
{
  "id": "http-optimistic-connect-upgrade-rejection-boundary",
  "title": "HTTP/1.1 CONNECT の先行送信: RFC 9931 の拒否時close・407・CONNECT-UDP境界",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9931 (Proposed Standard, March 2026) updates RFC 9112 / RFC 9298; compared with RFC 9110 and RFC 6455; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "http1.1",
    "connect",
    "connect-udp",
    "upgrade",
    "optimistic-transmission",
    "connection-close",
    "proxy-authentication",
    "rfc9931"
  ],
  "sources": [
    {"id": "rfc9931-optimistic-connect-20261003", "url": "https://www.rfc-editor.org/info/rfc9931/", "type": "official_docs"},
    {"id": "rfc9110-connect-upgrade-semantics-20261003", "url": "https://www.rfc-editor.org/rfc/rfc9110.html", "type": "official_docs"},
    {"id": "rfc9112-close-request-boundary-20261003", "url": "https://www.rfc-editor.org/rfc/rfc9112.html", "type": "official_docs"},
    {"id": "rfc9298-udp-optimistic-baseline-20261003", "url": "https://www.rfc-editor.org/rfc/rfc9298.html", "type": "official_docs"},
    {"id": "rfc6455-handshake-wait-20261003", "url": "https://www.rfc-editor.org/rfc/rfc6455.html", "type": "official_docs"},
    {"id": "rfc9931-errata-status-20261003", "url": "https://errata.rfc-editor.org/search/?rfc_number=9931", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/api-distributed.json"]
}
---

# HTTP/1.1 CONNECT の先行送信と拒否時の接続境界

## 問いと選定理由

proxy が接続確立の遅延を減らすため、CONNECT / Upgrade の成功応答より先に利用アプリの payload を流してよいか。拒否後に同じ接続を使う認証・再接続処理はどう変わるか。

[RFC 9931](https://www.rfc-editor.org/info/rfc9931/) は2026年3月の Proposed Standard で、HTTP/1.1 と UDP proxying の既存仕様へ規範要件を追加した。本稿はこの未収録の変更を扱い、9月に公開された新機能とは扱わない。既存の [HTTP retry](retry-idempotency.md)、[Go HTTP client](../go/http-client.md)、[SSRF宛先制御](../security/ssrf-fetch-dns-redirect-boundary.md) と異なり、同じ byte 列を HTTP と tunnel のどちらとして解釈するかに絞る。

採用判断は、未信頼のアプリ入力を扱う HTTP/1.1 proxy client では原則として成功確認を待つこと。proxy server 側の拒否時closeも別に確認する。これは以下の規範を基にした本稿の安全側の設計案で、あらゆる protocol の先行送信を一律に禁止する新規標準ではない。

## 成功の境界を先に分ける

[RFC 9110 §7.8 / §9.3.6](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.3.6) の基礎契約:

- HTTP CONNECT の2xxは、response header section の直後から tunnel に切り替わることを示す。成功以外では tunnel はまだ成立していない。`100 Continue` は CONNECT 成功の代わりにならない
- Upgrade は相手が受け入れれば `101 Switching Protocols`。単なる `200 OK` を upgrade 成功として扱わない。server は Upgrade を無視して通常の HTTP 応答を返してよい
- Upgrade request を最後まで送ることは必要条件だが、それだけで相手の受諾が分かるわけではない。受信応答と、選ばれた protocol 固有の検証が別に必要になる

したがって「request write が成功した」「同じ宛先では前回成功した」「proxy への TLS 認証が成功した」を tunnel 成立イベントに置き換えない。ここで問題になる payload の信頼は、認証済み proxy client 自身の信頼とは別である。 RFC 9931 §4は、拒否後の接続がHTTPとして解釈されることで、利用アプリが選んだ先行payloadをproxy client自身の追加HTTP要求と取り違えるrequest smugglingを説明する。接続単位の認証があっても、この解釈の不一致は解消されない。

## RFC 9931 が追加した契約

[§6.3 / §8](https://www.rfc-editor.org/info/rfc9931/) の要約を、送信側と受信側に分ける。

| 対象 | 規範要件・許容条件 |
| --- | --- |
| 未信頼 TCP client の代理で HTTP/1.1 CONNECT を送る proxy client | payload 転送を2xxまで待つ、または request に `Connection: close` を付ける、の少なくとも一方が MUST。両方でもよい |
| HTTP/1.1 CONNECT を拒否する proxy server | `close` の有無によらず、同じ接続上の後続 request を処理せず underlying connection を閉じることが MUST |
| 上記 server の性能上の例外 | 未信頼 payload の転送を2xxまで待つと分かっている client に限り、この拒否時closeの緩和は MAY。単に client が `Connection: close` を付けるだけでは、この例外の条件にならない |
| CONNECT-UDP の optimistic transmission | HTTP/1.x の先行 UDP packet 送信は MUST NOT。HTTP/2以降では MAY が残る |

TCP CONNECT の二択を CONNECT-UDP へ流用しない。後者には `Connection: close` を付けたら先行送信できる、という例外はない。

[RFC 9298 §3.2–3.5 / §5](https://www.rfc-editor.org/rfc/rfc9298.html#section-5) の元の本文は、応答前の UDP packet 送信を HTTP version で限定していなかった。RFC 9931 はこの許可を狭めた。UDP proxying は HTTP/1.1 では `GET` + `Upgrade: connect-udp` と101応答、HTTP/2・HTTP/3では Extended CONNECT と2xx応答を使うため、名前が似ていても通常の TCP CONNECT と同じ分岐にはできない。HTTP/2以降で先行送信できても、拒否や buffer しない受信者による packet drop があり、配送保証にはならない。

## 拒否を全部同じclose規則にしない

RFC 9931 §5 は、一般の Upgrade 拒否について接続を閉じる一律の要求を置かない。HTTP/1.1 のまま継続できる場合がある。これに対し §8 は **CONNECT 拒否** に上記の防御を要求し、特に `407 Proxy Authentication Required` で接続確立が遅くなり得ると説明する。server の例外判定では User-Agent と vendor 文書による適合確認が示される。

独自の運用案として、例外を入れる前に対象 client の版・設定・vendor の根拠と packet capture による確認を記録する。User-Agent 文字列だけを認証済みの実装証明として扱わず、未確認 client にまで全体のclose方針を緩めない。client の更新や protocol fallback 後は確認をやり直す。

[RFC 9112 §9.6](https://www.rfc-editor.org/rfc/rfc9112.html#section-9.6) は、`Connection: close` を受けた server に、応答後の接続終了と後続 request の非処理を要求する。単に応答 header にcloseを書いて、同じ socket の parser が既に読み込んだ後続要求を実行する実装では境界が守られない。同節は即時 TCP close で最後の応答を読めなくなる reset 問題も説明する。安全な終了は「後続を実行しないこと」と「拒否応答を受け取れること」を分けて試験する。

[WebSocket RFC 6455 §4.1](https://www.rfc-editor.org/rfc/rfc6455.html#section-4.1) は既に、opening handshake 後の追加送信を応答まで待つことを MUST とし、101に加えて Upgrade / Connection / Sec-WebSocket-Accept 等の応答検証を要求している。これは RFC 9931 で初めて導入された制約ではなく、TCP CONNECT の `close` 二択で置き換えるものでもない。

## 実装・移行の設計案

以下は標準の追加義務ではなく、独自の設計・試験方針である。 待機と407再認証の手順はTCP CONNECTを対象とする。Upgradeの成功判定は101とprotocol固有の検証に読み替え、CONNECT-UDPのHTTP/1.x先行送信禁止は維持する。

### 1. 待機と転送を別状態にする

- 「HTTP handshake 待機」「tunnel 成立」「失敗・終了」を明示する。利用アプリからの読み込み開始と、上流への payload 書き込み許可を同じイベントにしない
- 待機中の入力は bounded buffer に置くか、backpressure で読み取りを止める。byte数・待機時間・同時handshake数を別々に制限し、2xxを待つ修正が無制限メモリ消費を作らないようにする
- 2xx の response header section を完全に受信するまで転送を解放しない。ヘッダー途中の切断・不正応答・timeout・cancel は失敗へ進める。先行 byte を送ることで timeout を回避する fallback は設けない
- 応答待ちの所有者を一つにし、受信 buffer の余剰 byte を次の protocol へ一度だけ引き渡す。別goroutineが同じsocketをHTTP parserとtunnel readerの両方で同時に消費する設計を避ける

### 2. 407 と再試行の費用を可視化する

拒否時closeを受けた client は、古い接続を pool へ戻したり、407への再認証を同じsocketへ書き続けたりしない。再接続が必要な認証フローでは、接続確立数、認証round trip、全体deadlineを測る。407の再発を無制限retryにせず、credential不備とtransport failureを分ける。新しい接続の認証とtunnel成立を確認する前に、保持payloadを送らない。

「以前より接続数が増えた」だけを理由に server のcloseを無効にしない。まず待機型clientを確認し、必要なら仕様上の例外を限定的に使う。HTTP/2・HTTP/3への移行を検討する場合も、このHTTP/1.1の問題がなくなることを、すべてのrequest smugglingや認可問題の解決と一般化しない。

### 3. version は接続ごとに判断する

手前がHTTP/2でも、gatewayの次のhopがHTTP/1.1なら、そのhopの送信側・受信側の役割を再評価する。前段のversionや過去のALPN結果だけで全経路を承認しない。特にHTTP/2からHTTP/1.1へfallbackするCONNECT-UDPは、送信形式だけでなく先行送信の可否も変える必要がある。これは版ごとの契約から導く設計上の帰結で、個別gatewayの対応を確認した結果ではない。

## 受け入れ試験案（未実行）

管理下のstub proxy・client・隔離ネットワークを使い、第三者サービスや本番宛先への攻撃試験にはしない。下表のserver試験は、2xx待機が確認済みのclientに対する例外を無効にした既定構成を対象にする。

| 条件 | 観測する結果 |
| --- | --- |
| clientへ入力を渡し、CONNECTの2xx応答を遅らせる | 待機型clientから上流へのpayloadは0 byte。2xxヘッダー完了後にだけ転送 |
| 2xx status lineの後、header sectionの途中で切断 | tunnel成立扱いにせず、保留payloadを送信しない |
| CONNECTを403・407・502・504で拒否し、同じ受信bufferに無害な後続request fixtureを置く | 後続request handlerの実行回数は0、接続は閉じる。close headerの有無を両方試す |
| 407でclose後、認証をやり直す | 新規接続でhandshake。古いsocketを再利用せず、deadline内で停止可能 |
| 未対応Upgradeを通常のHTTP応答で拒否する | TCP CONNECT拒否専用のルールを誤適用しない。新protocolへの送信も始めない |
| CONNECT-UDPがHTTP/2からHTTP/1.1へfallbackする | GET/Upgradeと101確認へ切り替え、先行UDP packetは送らない |
| TCP CONNECTのclose方式を選ぶclient | 拒否応答後に後続HTTP要求を実行させない。server例外の「2xxを待つclient」と混同しない |
| 入力が待機buffer上限を超える、または応答が返らない | backpressureか明示失敗。timeout・cancelでbufferとsocketを回収 |
| User-Agentが既知名でもclientの版・設定を確認できない | 運用案ではclose例外を適用しない |
| WebSocketの101で必要な応答検証が失敗する | handshake失敗。101だけで送信可能にしない |

## 版・provenance・未確認事項

- 2026-10-03 UTCに一次資料の本文を開いた。RFC 9931は2026年3月、RFC 9110 / RFC 9112は2022年6月、RFC 9298は2022年8月、RFC 6455は2011年12月の文書。RFC 9931の更新先は9112と9298であり、9110を置換したとは記録しない
- [RFC 9931 errata検索](https://errata.rfc-editor.org/search/?rfc_number=9931) は取得時に一致するerrataなしと表示した。将来の訂正がないことの保証ではない。他の参照RFCの全errataは本調査では網羅確認していない
- RFC 9931の `/rfc/rfc9931.html` は429で取得失敗したため、公式infoページの全文と [IETF Datatracker掲載本文](https://datatracker.ietf.org/doc/html/rfc9931) を照合した。旧errata URL2件も取得失敗したが、上記の現行検索URLで確認できた
- RFCのCopyright NoticeでIETF Trust条項を確認。Code Componentsの表記はRFC 6455がSimplified BSD、他4文書がRevised BSD。errataサイトの本文ライセンスはunknownとする。すべて独自要約で、コード・payload例・図の転載はない
- proxy製品、SDK、OS、browserごとの修正版・既定設定・実機動作・性能は未検証。TLS-first payloadの例外一般化、TLS 0-RTT、HTTP/2・HTTP/3固有の別攻撃、CONNECT-IPの実装詳細は扱わない
- 検索evalは本文への到達性だけを確認する。上表は実行済みnetwork testではない。全sourceがofficial_docsの90日TTLなので、明示期限は2027-01-01とする

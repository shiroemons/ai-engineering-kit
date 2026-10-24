---
{
  "id": "javascript-page-lifecycle-final-delivery-boundary",
  "title": "Page lifecycle: visibilitychange・bfcache・sendBeaconと最終送信の保存境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "Beacon CRD 2022-08-03; HTML public Living Standard 2026-10-22; Fetch source 59afe349913d56898c947576216f1c2579308be7 (2026-10-19); Chrome155 maintainer statement 2026-10-16; verified 2026-10-24 UTC; browser runtime unverified",
  "tags": [
    "research-domain:frontend",
    "page-lifecycle",
    "visibilitychange",
    "hidden",
    "pagehide",
    "pageshow",
    "persisted",
    "bfcache",
    "beforeunload",
    "unload",
    "sendBeacon",
    "keepalive",
    "inflightKeepaliveBytes",
    "acknowledgment",
    "outbox",
    "exactly-once"
  ],
  "sources": [
    {
      "id": "w3c-beacon-queue-delivery-boundary-20261024",
      "url": "https://www.w3.org/TR/2022/CRD-beacon-20220803/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-page-visibility-20261024",
      "url": "https://html.spec.whatwg.org/multipage/interaction.html#page-visibility",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-page-transition-persisted-20261024",
      "url": "https://html.spec.whatwg.org/multipage/nav-history-apis.html#the-pagetransitionevent-interface",
      "type": "official_docs"
    },
    {
      "id": "whatwg-fetch-keepalive-lifecycle-20261024",
      "url": "https://fetch.spec.whatwg.org/#http-network-or-cache-fetch",
      "type": "official_docs"
    },
    {
      "id": "whatwg-fetch-keepalive-source-59afe349-20261024",
      "url": "https://github.com/whatwg/fetch/blob/59afe349913d56898c947576216f1c2579308be7/fetch.bs",
      "type": "github_repository_analysis"
    },
    {
      "id": "chrome-page-lifecycle-delivery-20261024",
      "url": "https://developer.chrome.com/docs/web-platform/page-lifecycle-api",
      "type": "maintainer_article"
    },
    {
      "id": "chrome155-unload-default-change-20261024",
      "url": "https://developer.chrome.com/blog/unload-deprecated-in-chrome",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2027-01-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Page lifecycle の最終送信を保存完了と混同しない

## 問いと結論

タブを閉じるときにログや編集中のデータを必ず送れるか。`sendBeacon()` が `true` を返せば「同期済み」にしてよいか。

**終了直前の送信は最後の機会を増やす仕組みであり、確実な保存の境界ではない。** 編集中のcheckpoint、UAへのキュー投入、サーバーの受領・確定を分ける。`visibilitychange` の `hidden` を早めのcheckpointに使い、`pagehide` を補助にしても、終了通知やcallbackの実行を保証できるわけではない。

対象はページからの小さなtelemetryと保存要求の設計判断。既存の[fetch cancellation](fetch-abort-cancellation.md)は取消理由、[IndexedDB](indexeddb-transaction-completion-durability-boundary.md)はローカルtransaction完了を扱う。本稿はその外側の「ページが停止する前後に何を観測でき、どこまで成功と言えるか」を扱う。Service Workerの同期、サーバーの冪等実装、認証方式の実装は対象外。

## 事実: hidden は終了宣言ではない

[HTML Page visibility][visibility] は可視性を `visible` / `hidden` とし、状態変更時に `document` へ `visibilitychange` を発火する。backgroundタブや最小化は可視性の例であり、アプリケーションの業務完了を表す値ではない。

[Chromeのlifecycle解説][lifecycle]は、特にmobileで `beforeunload` / `pagehide` / `unload` が来ない終了があり、hiddenへの遷移が最後に観測できる機会になり得ると説明する。frozen中はtimerやfetch callbackなどのtaskが停止し、discard自体の通知も期待できない。これはChrome開発者の運用指針であり、全browserのイベント列を規定する仕様ではない。

**独自の採用方針:**

- 編集内容は操作中に小分けで保存する。hidden handlerで初めてdatabaseを開き、巨大payloadを生成し、token更新を待つ設計を避ける
- `document` の `visibilitychange` でhiddenを確認し、準備済みの小さなbatchを送信候補にする。通常のtab切替でも起きるので、logoutや業務sessionの不可逆な終了処理には直結させない
- `window` の `pagehide` を補助に使う場合も、先のhiddenと同じbatchを扱う。二つのhandlerを独立に実装して同じイベントを二重計上しない。一定の発火順序に依存しない
- `await` を置いてもブラウザーがそのPromiseを終了待ちの障壁にしてくれるとは扱わない。保存表示やキュー削除に必要な処理は、次回の起動・復帰でも再開できる形にする

## beforeunload と unload の用途を絞る

[Chromeの推奨][lifecycle]は `beforeunload` を未保存編集の警告に限定し、保存後にlistenerを解除すること。保存のため常時登録する終了hookではない。警告できたことも保存できたことも同義ではない。

直近の版差として、[Chrome公式の2026-10-16記事][unload]は **Chrome 155でunloadの既定不発火への移行が完了**したと報告する。同じ記事にPermissions Policy / enterpriseによるopt-outと、iOS版ChromeがWebKitを使う例外がある。「unloadが全環境から削除された」「例外設定を戻せば確実に保存できる」とは読まない。古いブラウザーで発火したという経験も新しい配信契約にはならない。

## 事実: sendBeacon の true はキュー受付

[W3C Beacon CRD 2022-08-03 §3][beacon]の契約を分解する。

| 観測 | 分かること | 分からないこと |
|---|---|---|
| `sendBeacon(url, data) === true` | UAが転送用キューへ受け付けた | 通信成功、HTTP status、サーバー確定 |
| `false` | その呼出しはキューへ受け付けられなかった | 次回なら必ず送れること |
| 例外 | URLやbody等の前提が成立しなかった | 単なる混雑やquotaだけが原因であること |
| アプリの受領確認 acknowledgment | そのAPIが定義した結果を確認した | APIが約束していない永続性や後続処理 |

Beaconは `POST` を使い、response callbackがない。methodや任意headerを指定するAPIではなく、offline保管やbackground同期を提供するものでもない。`true` は非同期転送の成否を知らせないので、送信元の唯一の編集コピーを消す根拠にはしない。

**独自の区別:** 損失を許容する計測なら「受付後にメモリー上のbatchを捨てる」方針も選べる。ただし意味はbest-effortであり、「配信済み」ではない。保存が必要な編集なら、ローカル保存済み・送信受付済み・サーバー確認済みを別状態にし、確認前のoutboxを残す。HTTP 2xxを受け取った場合も、それが入力受付なのか永続commitなのかはendpointの契約で決める。

## keepalive は無制限の退避路ではない

[Fetchの現行本文][fetch]と[固定commitのfetch.bs][fetch-source]で、`keepalive` は要求がenvironment settings objectより長く生きることを可能にするflagであることを確認した。ページのJavaScriptが生き続ける保証ではない。

HTTP-network-or-cache fetchは、今回のbody長 `contentLength` と、同じfetch group内の未完了keepalive要求のbody長合計 `inflightKeepaliveBytes` を足した値が **64 KiB（65,536 bytes）を超える**とnetwork errorにする。これは一要求ごとに独立した64 KiB枠でも、圧縮後のwireサイズでもない。body抽出では `ReadableStream` と `keepalive: true` の組合せが `TypeError` になる。

Beaconもkeepaliveを使い、[同じin-flight quotaを共有する][beacon]。従って `sendBeacon()` のfalseを受けて同じ大きなbodyを `fetch(..., { keepalive: true })` に渡しても、quotaを回避する方法にはならない。

**独自の送信設計:**

1. 通常稼働中にflushし、終了時までログをためない。上限ぎりぎりではなく、同じページの他のSDKが使う分も考えて小さいbatchにする
2. JSON文字列の文字数ではなく符号化後のbodyサイズを見積もる。FormDataのfield値だけを足す等、encodingの付加分を無視した計算を採用しない
3. 大量batchを同時発行して分割したつもりにならない。未完了分は合算される。受付拒否なら復帰後の通常経路へ残す
4. method/headerや応答が必要ならfetchを選ぶが、callbackが実行されない可能性を設計から消さない。Beaconの受付直後に別fetchも送るような無条件の二重送信を避ける

また、BeaconはCORS免除ではない。[処理モデル][beacon]ではcontent typeがsafelistedでなければCORS経路になる。cross-originへ `application/json` のBlobを送る場合はpreflightを含む許可が必要で、trueはその成功確認ではない。独自の推奨は、許可された同一originの収集endpointや既存の認証・CORS契約を活用し、終了時だけ別の送信形式を発明しないこと。

## bfcache: pagehide は一度きりの破棄とは限らない

[HTML PageTransitionEvent][transition]では、`pagehide.persisted === true` はページが再利用され得ることを表す。後で本当に復元される保証ではない。一方 `pageshow.persisted === true` は新規loadではない復帰を識別する。

**独自の復帰設計:**

- pagehideで停止したpollingや接続を復帰時に再確立し、サーバー側revision・認証状態を必要に応じ再確認する。古い画面が戻っただけで最新状態とみなさない
- ページ生涯に一度だけtrueにする `finalSent` は、bfcache復帰後の編集を取りこぼし得る。UIの活動区間、変更revision、送信batchを別の識別子で管理する
- hiddenとpagehideの重複呼出しは、同一区間・同一batchの「試行抑止」で抑える。このフラグを配信確認フラグに兼用しない。復帰時に試行可能にしても、再送する同じ操作のIDは変えない
- 既にサーバーが処理したが応答を観測できなかった状態を「未処理」に断定しない。再送するなら安定した操作IDを使い、サーバー側で重複とrevision競合を扱う

この案は重複処理を抑える設計であり、browserからサーバーまでの **exactly-once** 保証ではない。ローカルコピーの消失、容量不足、ユーザーが二度と戻らない場合まで、outboxと再試行だけで救えるとは約束しない。

## 採用前の境界試験

以下は提案であり実行済み結果ではない。handlerのconsoleログだけでなく、受信側の操作ID・確定結果と復帰後のoutboxを照合する。

| 条件 | 合格条件 |
|---|---|
| tab切替→visibleへ戻る | hiddenを終業とせず編集を継続できる。同じbatchを二重計上しない |
| hiddenとpagehideが両方通知される | 順序に依存せず同一区間の重複試行を抑える |
| mobileでbackground化後にプロセス終了 | 終了eventなしでも、直前までに確定したcheckpointから回復できる範囲を説明できる |
| Beaconがtrueの後、offline・server失敗 | UIをサーバー保存済みにせず、必要な未確認データを残す |
| false・不正URL・keepalive stream | 受付拒否と例外を分け、同じ要求の即時loopを作らない |
| 他の未完了送信と合計64 KiB超過 | 一件が小さくても失敗を扱える。fetch切替をquota回避としない |
| cross-origin JSONでpreflight拒否 | trueを受領確認と誤認せず、設定不備を通常稼働中に検出する |
| bfcache復帰後の新しい編集 | pageshow.persistedを記録し、listenerやtimerを二重登録せず新batchを送れる |
| server確定後に応答だけ失う | 同じ操作IDの再送で副作用を重複させず、古いrevisionで新編集を上書きしない |

ブラウザー/OSごとの実操作、通常navigation、bfcacheの実際の命中・不命中、offline、終了の条件を分けて記録する。合成eventのdispatchだけでは、freeze・discardや終了時のネットワーク継続を再現したことにならない。

## 版・出典・未確認事項

2026-10-24 UTCに一次資料を開いた。Beaconは2022-08-03 Candidate Recommendation Draftで、最終勧告ではない。HTMLは2026-10-22、Fetchは2026-10-19更新のliving本文。新機能紹介ではなく、保存と送信の境界を整理した。

WHATWGのcommit snapshot URLはnative webから取得できなかった。FetchはGitHub connectorで `59afe349913d56898c947576216f1c2579308be7` の `fetch.bs` とLICENSEを読み、本文のquota・body抽出契約を照合した。HTMLは取得時点の公開本文を根拠とし、固定sourceを読めたとは主張しない。Chrome lifecycle解説は2023-12-01公開・2026-10-16更新、unload移行記事は2026-10-16公開・更新。

WHATWG本文はCC-BY-4.0、source codeへの組込み部分はBSD-3-Clause。Beaconはその版がリンクする[W3C Software and Document License 2015][w3c-license]、Chrome両記事は本文CC-BY-4.0 / code Apache-2.0。本文・試験案は独自整理で、外部コードを転載していない。

既存索引で `sendBeacon` / `pagehide` / `visibilitychange` を横断検索し該当なしを確認した。browser runtime、実配信率、Chrome155と旧版の比較、Safari/Firefox、通信断や強制終了の実機試験は未実行。検索evalや文書検証は、配信・永続化・browser挙動の検証ではない。

[visibility]: https://html.spec.whatwg.org/multipage/interaction.html#page-visibility
[transition]: https://html.spec.whatwg.org/multipage/nav-history-apis.html#the-pagetransitionevent-interface
[beacon]: https://www.w3.org/TR/2022/CRD-beacon-20220803/
[fetch]: https://fetch.spec.whatwg.org/#http-network-or-cache-fetch
[fetch-source]: https://github.com/whatwg/fetch/blob/59afe349913d56898c947576216f1c2579308be7/fetch.bs
[lifecycle]: https://developer.chrome.com/docs/web-platform/page-lifecycle-api
[unload]: https://developer.chrome.com/blog/unload-deprecated-in-chrome
[w3c-license]: https://www.w3.org/copyright/software-license-2015/

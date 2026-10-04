---
{
  "id": "security-coop-coep-credentialless-deployment-boundary",
  "title": "COOP/COEP: credentialless の資格情報・Report-Only・実配信の隔離境界",
  "kind": "knowledge",
  "technology": "security",
  "version": "WHATWG HTML Living Standard 2026-10-04表示; Fetch 2026-09-21表示 / snapshot 357bd98924d94b81fbe8608192a2ee1f123b82f4; Chrome iframe credentialless説明 2023-01-12; 2026-10-04 UTC再確認",
  "tags": ["research-domain:security", "COOP", "COEP", "CORP", "credentialless", "cross-origin-isolation", "report-only", "popup", "iframe", "deployment"],
  "sources": [
    {"id": "whatwg-html-coop-coep-policy-20261004", "url": "https://html.spec.whatwg.org/multipage/browsers.html", "type": "official_docs"},
    {"id": "whatwg-html-isolated-capability-20261004", "url": "https://html.spec.whatwg.org/multipage/webappapis.html", "type": "official_docs"},
    {"id": "whatwg-fetch-credentialless-snapshot-20261004", "url": "https://fetch.spec.whatwg.org/commit-snapshots/357bd98924d94b81fbe8608192a2ee1f123b82f4/", "type": "official_docs"},
    {"id": "webdev-coop-coep-deployment-20261004", "url": "https://web.dev/articles/coop-coep", "type": "maintainer_article"},
    {"id": "chrome-iframe-credentialless-partition-20261004", "url": "https://developer.chrome.com/blog/iframe-credentialless", "type": "maintainer_article"}
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# COOP/COEP: 表示できたことと、期待した資格情報・隔離を別々に確認する

## 問いと採用判断

WebAssemblyの並列処理などに必要なcross-origin isolationを導入するとき、COEPを`credentialless`にすれば第三者resourceを従来どおり使えるのか。Report-Onlyで違反が出なければ、そのままenforceへ切り替えてよいのか。

本稿の判断は、**隔離の成立、resourceの読み込み、認証状態に合う内容の三つを配備条件にする**、である。読み込みを許可する代わりにcookieを送らなくなる経路では、HTTP成功も画面表示も、以前と同じ機能の成功とは限らない。

これは新しいreleaseの紹介ではなく、security領域の未収録だった配備上の問いを扱う。全27件のsecurity文書の対象と関連記述を確認し、KBで`COOP`、`COEP`、`credentialless`、`crossOriginIsolated`、`cross-origin isolation`は該当なしだった。既存の[SRI](sri-policy-admission-digest-boundary.md)は内容digest、[Trusted Types](trusted-types-csp-report-only-default-policy.md)はDOM sink、[BFF](../oauth/bff-cookie-csrf-proxy-boundary.md)は認証済みAPIを扱う。それらの防御を置換しない。

## 一次資料で確認した契約

### COOPはwindow間、COEPは読み込み条件を変える

[HTML §7.1.3–7.1.4][html-policy]のCOOP `same-origin`は、originやpolicyが合わないpopupとのbrowsing context groupを分離する。相手が画面に残っていてもopenerからは閉じたように見える場合がある。隔離用の組合せはCOOP `same-origin`と、COEP `require-corp`または`credentialless`。内部値`same-origin-plus-COEP`は直接送るheader値ではない。

COOP/COEPのheader処理はsecure contextを前提とし、non-secure contextでは既定の`unsafe-none`を返す。したがって送信headerの存在だけで隔離成立を判定しない。[HTML §7.1.3.1・§7.1.4.1][html-policy]

`same-origin-allow-popups`は同じ隔離条件ではない。popup互換のために値を変えた結果を、`same-origin`と同等の成功にしない。[HTMLのcrossOriginIsolated定義][html-capability]は、実効capabilityがheaderだけでなく`cross-origin-isolated` featureにも依存するとする。

### credentiallessはCORSや明示CORPを取り消さない

[Fetch §2.2.5・§4.6][fetch]では、COEP `credentialless`の資格情報制限は主にcross-originの`no-cors`要求に適用される。同一origin判定にはredirect-taintも関係する。名前から全要求を`credentials: omit`へ変更する契約とは読めない。

[Fetch §3.7・§4.10][fetch]を合わせると、次の区別になる。

| 要求・応答の条件 | 判断する境界 |
|---|---|
| `no-cors`、COEP `require-corp`、cross-origin、CORPなし | COEPにより拒否される |
| `no-cors`、COEP `credentialless`、資格情報なし、CORPなし | CORP欠落だけでは拒否しない。他の検査は残る |
| 同条件だが応答が明示CORP `same-origin` | cross-originの利用は拒否。credentiallessは所有者の制約を上書きしない |
| `cors`、`credentials: include` | CORSの照合が必要。応答の許可originは`*`では足りず、資格情報許可も必要 |

CORPは応答の利用制約であり、CORSによる応答共有の許可とは別である。CORPを付けるだけでJavaScriptからcross-originのbodyを読めるようにはならない。また、この表はブラウザが読む条件であり、サーバーの業務認可や通信の副作用を取り消す仕組みではない。

### Report-Onlyでは資格情報の変化を再現できない

[HTML][html-policy]はenforce値とreport-only値を別々に保持する。[Fetchの資格情報許可判定][fetch]が読むのはenforce側の値である。したがって、**enforce側が`unsafe-none`のままReport-Onlyだけを`credentialless`にしても、cookieを取り除く動作の試験にはならない**。これは両手順を突き合わせた帰結である。

Report-Onlyは違反候補の観測に使えるが、enforce後に匿名版の画像・動画・script等が返ることによる機能差までは合格判定できない。[web.devの導入記事][deployment]も監視とenforceを別段階にする。本稿ではその手順に、実際に資格情報を減らした経路の機能試験を加える。

### headerの重複とiframe属性を別の問題にする

[HTML §7.1.4.1][html-policy]はCOEPを単一のStructured Field tokenとして解析する。たとえば`require-corp, require-corp`は同じ値を二度強化する表現ではなく、`unsafe-none`へ戻る例として明記されている。CDNとアプリの両方が追加したheaderを、設定ファイルの片方だけで検査しない。

[Chrome開発者記事][iframe]の`<iframe credentialless>`は別機構である。既存のcookie jar・storageから分けた一時領域を使い、COEP未対応の第三者frameを埋め込む。領域はtop-level documentとframeのoriginに対応し、top-level documentがunloadされると消える。同記事はChrome 110から既定有効とするが、これは2023年の記事で確認した導入情報であり、2026年の全browser対応表ではない。

**COEP headerの`credentialless`を送るだけで、iframe属性の一時storageまで付くとは扱わない**。逆にiframe属性を足して表示を回復させても、通常のログイン状態を保持できた証拠にはならない。

## 配備の判断案（本稿独自の提案）

### 1. 「表示するresource」と「認証済みresource」を先に分ける

候補ページから、必要なscript・画像・動画・worker・frame・popupを列挙する。各項目にrequest mode、資格情報の必要性、配信owner、期待する認証状態、失敗時の代替を記録する。単にURLが第三者かどうかで`credentialless`適合を決めない。

- 公開asset: cookieなしでも同じ用途を満たすか、応答内容まで確認する
- ログイン依存asset: 匿名版に差し替わっても見た目が成立するケースを試す。利用者のavatar、個人向け動画、権限付きdownloadの状態を確認する
- JavaScriptでbodyを読むAPI: modeとCORSを明示し、CORP追加で解決したことにしない
- 第三者frame: 通常の埋め込みに必要なpolicyと、一時storageを許容するiframe credentiallessを別案として比較する
- OAuth・決済等のpopup: 実際の完了通知と復帰まで確認する。windowが開いたことを接続試験の成功にしない

隔離機能を使わない画面まで一斉に変更する必要があるかも検討する。たとえば認証完了後に隔離された作業画面へ遷移する設計を候補にできるが、session引継ぎの安全性と利用体験は個別に設計する。popup障害を直すためのCOOP緩和を、隔離成立の修正と混同しない。

### 2. 設定ではなく最終応答と実効状態を判定する

headerを生成するownerを決め、アプリ、reverse proxy、CDNで追加が重ならないようにする。通常画面だけでなく、認証後、エラー応答、旧build、cache hit、rollback後のブラウザ受信値を観測する。headerが二本あるか、途中で結合されているかも証跡に含める。

隔離機能の初期化前に`self.crossOriginIsolated`を確認し、必要なWindow・worker・frameごとに結果を記録する。top-levelがtrueであることをすべての子環境へ拡張しない。featureの制限や未対応環境でfalseなら、隔離を要しない実装への退避または明確な利用不可表示を選ぶ。フラグを偽装したり例外だけを握り潰したりしない。

受入れ条件は「headerがある」ではなく、想定環境でのcapability、必要resourceの成功、認証状態に合う内容、popup完了、復旧の再現性を組にする。隔離機能の開始率と業務操作の完了率を別指標にすると、表示だけ成功する劣化を見つけやすい。

### 3. Report-Onlyの次に限定したenforce試験を置く

監視で影響候補を集めた後、管理下の試験用アカウントと配信先でenforceを有効にし、匿名応答の意味を確かめる。cookieを送る旧経路と送らない新経路を比較するが、cookie値・token・個人化bodyを通常ログへ出さない。ログにはresource分類、build、browser版、認証状態の期待と判定だけを残す。

「違反ゼロ」だけでは移行しない。既知の違反を作ったときに報告が到達すること、popupと遅延ロードを試したこと、資格情報の変化を観測したことを別々の条件にする。Report-Onlyへ戻せば失われたログイン状態やpopup参照まで復元される、という復旧想定も避ける。再読込みや再認証が必要かを試験しておく。

第三者resourceを取り込めない場合、無条件に自サーバー経由へproxy化する変更はこの導入に含めない。proxyは別の信頼境界を作るため、必要なら[SSRFの文書](ssrf-fetch-dns-redirect-boundary.md)を起点に別途レビューする。

## 管理下で行う反例試験（未実行）

| ケース | 試験で分けて観測すること |
|---|---|
| Report-Only `credentialless`のみからenforceへ移る | cookie送信の有無、返った内容、表示、認証済み機能を別々に確認 |
| cross-originの`no-cors`画像にCORPなし | `require-corp`と`credentialless`の違い。他の制約を固定 |
| 同じ画像へ明示CORP `same-origin`を追加 | 資格情報を除いてもresource ownerの制約が残る |
| `cors` APIに`credentials: include`、許可originは`*` | credentialless設定をCORS失敗の救済にしない |
| CDNとoriginがCOEPを二重追加 | 最終headerの結合と`crossOriginIsolated`を確認 |
| `same-origin`から`same-origin-allow-popups`へ変更 | popup互換の回復と隔離capabilityの維持を同一結果にしない |
| header credentiallessのみとiframe属性ありを比較 | ログイン状態、storageの継続性、top-level再読込み後の差 |
| popupを開いて認証完了まで進む | 見た目、opener参照、アプリの完了通知、復帰を別判定 |
| childで`cross-origin-isolated` featureを許可しない | 親のtrueを子のcapabilityとして誤採用しない |
| 同一origin URLから別originへredirect | URL一覧の最初のhostだけで資格情報条件を推定しない |

検索evalは発見可能性だけを検査する。この表のブラウザ・HTTP・認証・storage試験を実行済みとはしない。

## 版・出典・未確認事項

- HTMLの2ページはLast Updated **2026-10-04**。FetchはLast Updated **2026-09-21**、取得したLiving Standardが示すsnapshotは`357bd98924d94b81fbe8608192a2ee1f123b82f4`。表示更新日を各機能の導入日とは扱わない
- Fetchの正規root URLはnative webでtimeoutになった。検索から開いた[同サイトのquery付きURL](https://fetch.spec.whatwg.org/?trk=public_post_comment-text)と、そこが示した公式snapshotの本文・日付を確認した。catalogは再現可能なsnapshotを参照する。snapshotは歴史的参照であり、将来の実装判断では[Living Standard](https://fetch.spec.whatwg.org/)を再取得する
- web.dev記事はPublished 2020-04-13、本文の更新表示・更新履歴は2022-06-21、footerは2020-04-13で一致しない。Chrome iframe記事のfooterは2023-01-12。古い記事のbrowser support記述を現在の対応状況へ延長していない
- 全5件を2026-10-04 UTCに実際に開いた。HTML・FetchはWHATWG文書の[IPR Policy §7.1.1](https://whatwg.org/ipr-policy)でCC-BY-4.0、source codeへの組込み部分はBSD-3-Clauseの区分を確認。web.dev / Chrome記事は各footerのCC-BY-4.0、code samples Apache-2.0を確認。独自の日本語要約と設計案であり、原文コード・画像・長文の転載はない
- 新しいsource recordだけを追加し、既存recordの取得日は延長していない。公式仕様の版固定参照であり、browser実装のrepository analysisやOSSコードの移植は行っていない
- 実ブラウザ、WPT、CDN、service worker、Reporting API収集は未実行。browser/OSごとの対応版、一時storageの製品別挙動、企業policyや拡張機能の影響、各OAuth/決済SDKとの互換性は未確認
- Document-Isolation-Policy等の別方式、試験段階のCOOP値、全worker型、全frame navigationの仕様網羅は範囲外。COOP/COEPの成立はXSS耐性、第三者scriptの安全、サーバー認可、完全なside-channel防御の証明ではない

[html-policy]: https://html.spec.whatwg.org/multipage/browsers.html
[html-capability]: https://html.spec.whatwg.org/multipage/webappapis.html#dom-crossoriginisolated
[fetch]: https://fetch.spec.whatwg.org/commit-snapshots/357bd98924d94b81fbe8608192a2ee1f123b82f4/
[deployment]: https://web.dev/articles/coop-coep
[iframe]: https://developer.chrome.com/blog/iframe-credentialless

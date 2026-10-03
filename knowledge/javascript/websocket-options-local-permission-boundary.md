---
{
  "id": "javascript-websocket-options-local-permission-boundary",
  "title": "Chrome 154 WebSocket options: local接続・許可・標準化の境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "Chrome 154 stable 2026-09-22; WHATWG live standard 2026-03-15 + WICG draft 2026-08-07 + unmerged proposals pinned at 7e677cfb410744d833d3261a3cbf46d98ff30925 / 332b714a7b9097fefc2fba899bc54be80f9d8f88; verified 2026-10-03, runtime untested",
  "tags": [
    "research-domain:frontend",
    "javascript",
    "WebSocket",
    "WebSocketInit",
    "protocols",
    "targetAddressSpace",
    "local",
    "loopback",
    "permission",
    "mixed-content",
    "Chrome154",
    "standardization"
  ],
  "sources": [
    {
      "id": "chrome154-websocket-options-release-20261003",
      "url": "https://developer.chrome.com/release-notes/154",
      "type": "release_notes"
    },
    {
      "id": "whatwg-websocket-live-interface-20261003",
      "url": "https://websockets.spec.whatwg.org/",
      "type": "official_docs"
    },
    {
      "id": "wicg-lna-draft-websocket-boundary-20261003",
      "url": "https://wicg.github.io/local-network-access/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-websocket-options-proposal-source-20261003",
      "url": "https://github.com/christhompson/websockets/blob/7e677cfb410744d833d3261a3cbf46d98ff30925/index.bs",
      "type": "github_repository_analysis"
    },
    {
      "id": "wicg-lna-websocket-target-proposal-source-20261003",
      "url": "https://github.com/WICG/local-network-access/blob/332b714a7b9097fefc2fba899bc54be80f9d8f88/index.bs",
      "type": "github_repository_analysis"
    },
    {
      "id": "chrome-lna-historical-websocket-limit-20261003",
      "url": "https://developer.chrome.com/blog/local-network-access",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Chrome 154 WebSocket options: local接続・許可・標準化の境界

## 調査する問いと適用版

HTTPS の管理画面からローカル機器やデスクトップ常駐サービスへ WebSocket 接続するとき、`targetAddressSpace` を付ければ接続できるのか。Chrome 154 が追加した constructor options と、ブラウザー許可・接続先・標準化状況を分けて判断する。

[Chrome 154 release notes](https://developer.chrome.com/release-notes/154) は2026-09-22の stable release として `WebSocketInit` と `targetAddressSpace` を記載する。取得日は2026-10-03 UTC。これは当該 Chrome の公開説明であり、全ブラウザー対応や実測結果ではない。既存の fetch cancellation 文書とは異なり、接続開始前の API 引数と local network permission を対象とする。

## 確認した変更と資料のずれ

### 実装の公開と標準の取り込みを分ける

- Chrome 154 は constructor 第2引数に options dictionary を受け取り、`protocols` をそのメンバーに置けると説明する。将来の options の拡張点でもある。[公開説明](https://developer.chrome.com/release-notes/154#add_options_bag_to_websocket_constructor)
- 同日取得した [WHATWG Living Standard](https://websockets.spec.whatwg.org/#interface-definition) は2026-03-15更新表示で、第2引数は文字列または文字列の列であり、`WebSocketInit` はまだない。履歴の再現点は [9879e5c snapshot](https://websockets.spec.whatwg.org/commit-snapshots/9879e5cf0d66c66af6990e4c75f72dda794e1b87/) に固定した。snapshot は現行標準を置換する規範として扱わない。
- [WHATWG PR #76](https://github.com/whatwg/websockets/pull/76) と [WICG PR #125](https://github.com/WICG/local-network-access/pull/125) は取得時ともに open / 未mergeだった。Chrome の導入を「Web標準への取り込み完了」と言い換えない。提案の読み取りにはそれぞれ `7e677cfb410744d833d3261a3cbf46d98ff30925` と `332b714a7b9097fefc2fba899bc54be80f9d8f88` を固定する。

### protocols の型は無条件に一般化しない

Chrome release notes の例は dictionary の `protocols` に単一文字列を置く。一方、取得した [PR #76 の IDL](https://github.com/christhompson/websockets/blob/7e677cfb410744d833d3261a3cbf46d98ff30925/index.bs) はそのメンバーを `sequence<DOMString>`、初期値を空列としている。資料間の差があるため「文字列と配列が全実装で同じ」とは結論しない。これは資料比較による観察であり、Chrome の不具合を再現した報告ではない。

従来の第2引数の文字列・配列と、dictionary 内のメンバーの型は別の契約である。既存の subprotocol 専用呼出しを一律に options 形式へ変える必要はない。options を使う配備では、配列形式を候補にして対象実装で検証する、というのが本稿の設計判断。TypeScript の型定義を通るだけではブラウザーの対応を確認したことにならない。

## local 接続を許す条件

[Chrome 154 の説明](https://developer.chrome.com/release-notes/154#support_targetaddressspace_option_for_websockets) は、公開 hostname 宛ての接続を `local` または `loopback` として扱う annotation を追加したとする。HTTPS を提供できない local server への mixed content 例外は、利用者が local network permission を許可し、hostname が local IP address に解決される場合に限る。単に文字列を付けるだけで公開インターネット向けの `ws:` が許可されるわけではない。

[WICG の提案](https://github.com/WICG/local-network-access/blob/332b714a7b9097fefc2fba899bc54be80f9d8f88/index.bs) では `WebSocketInit.targetAddressSpace` を handshake の request へ渡す。`public` は特別な指定をせず、`local` と `loopback` は対応する空間を指定する案である。未mergeの提案であることを保ち、細部を Chrome 実装全体の検証済み算法と同一視しない。

[2026-08-07表示の LNA draft](https://wicg.github.io/local-network-access/) は permission を secure context に制限し、対象空間に一致しない実接続先は失敗させる設計を説明する。ただし同 draft の WebSocket 節には options を置く場所がないという旧説明が残る。ここから Chrome 154 の機能不存在を推論せず、release notes・取得版・未取り込みの提案を並記する。

[Chrome の2025年 LNA記事](https://developer.chrome.com/blog/local-network-access) に残る「WebSockets はまだ対象外」という注記は Chrome 138 の初期 milestone の説明である。Chrome 154 の現在の回避策として使わない。同記事は旧 PNA preflight 方式から permission 方式への変更も説明している。古い試行版向けヘッダーやブラウザーフラグで新しい許可モデルを無効化する手順へ置換しない。

## 接続成功・失敗の扱い

[WHATWG WebSocket](https://websockets.spec.whatwg.org/) は constructor と非同期の handshake を分ける。オブジェクトが作れただけで接続済みとはせず、`open` と選択済みの `protocol` を確認する。subprotocol の不一致、CSP、証明書、DNS、permission、到達性は別々の原因候補である。`error` や異常 close の観測から「LNA拒否」と一意に決めない。仕様は失敗理由の詳細な区別をスクリプトへ公開しない。

これは `fetch()` のように Promise を `catch` すれば全失敗を捕捉できる API ではない。同期の引数例外と `error` / `close` の経路を分ける。互換性確認のために constructor を試す行為も接続処理を開始し得るため、無関係な機器への probe を feature detection として行わない。

## 配備判断と検証表（独自の設計案）

1. 通常の公開 `wss:` と local 機器向け接続を別の利用経路にする。local 機能は、接続対象と理由を画面に示し、利用者の操作から開始する。LNA許可は機器のアプリ認証や接続先の信頼性を保証しないので、接続後のアプリ独自認証を省略しない。
2. options 対応、指定空間、実IP、permission、subprotocol を別のテスト軸にする。`local` と `loopback` の境界を混ぜず、公開IPに解決されたケースも失敗として検証する。未対応ブラウザーで同じ体験になるとは推測しない。
3. 許可の未決定・許可・拒否・撤回、常駐サービス停止、DNSの変化、証明書エラーを試す。拒否を自動再試行ループにせず、説明と再接続操作を出す。全失敗を解消する目的で annotation を削除したり、より広い接続先へ黙って切り替えたりしない。
4. constructor の同期例外、handshake 成立、`open`、選択された `protocol`、サーバー側で認証済みになった時点を区別して記録する。ログに機器の秘密情報や認証トークンを残さない。
5. 検証に合格したブラウザー版・OS・配備方式を記録し、非対応環境には機能制限を説明する。管理ポリシーや安全設定の変更を一般的な導入前提にしない。

## 限界・出典・未確認事項

- 実機の Chrome 154、他ブラウザー、enterprise policy、iframe/worker からの許可要求は未検証。WPTやChromiumソースによる実装の追跡も行っていない。特定のエラー型、permission UI、全プラットフォームの動作を保証しない。
- `protocols` の単一文字列の扱いは release notes と提案IDLで資料差がある。実装テストなしに片方を誤りと断定せず、採用前の確認項目として残す。
- 固定した2つの GitHub source は native web では cache miss になったが、GitHub読み取りconnectorで指定commitの `index.bs` を取得した。PR状態も同connectorで再確認した。
- Chrome記事本文の CC-BY-4.0、WHATWG本文の CC-BY-4.0 とソース組込み部分の BSD-3-Clause、公開WICG draftの W3C Software and Document License 2023 をページで確認した。未merge branchの個別ライセンスは未確認のため catalog は `unknown`。独自日本語要約のみでコード転載・module昇格なし。
- 付属 eval は文書の検索到達性を検証するもので、ブラウザー接続の相互運用試験ではない。

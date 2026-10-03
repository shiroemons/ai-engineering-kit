---
{
  "id": "css-responsive-iframe-opt-in-resize-boundary",
  "title": "Chrome 154 responsive iframe: opt-in・origin許可・明示更新の境界",
  "kind": "knowledge",
  "technology": "css",
  "version": "Chrome 154 stable 2026-09-22; CSS Sizing4 ED 2026-09-06; unmerged HTML proposal 8fd9d8995c50145b4eda0c18382aa81880f877ee; WPT 2beef90fd2b8d267ebed384b0edc7f59f309f991; verified 2026-10-03, runtime untested",
  "tags": [
    "research-domain:frontend",
    "frame-sizing",
    "iframe",
    "responsive-embedded-sizing",
    "allow-origins",
    "requestResize",
    "intrinsic-size",
    "progressive-enhancement",
    "layout-shift",
    "Chrome154"
  ],
  "sources": [
    {
      "id": "chrome154-responsive-iframe-release-20261003",
      "url": "https://developer.chrome.com/release-notes/154",
      "type": "release_notes"
    },
    {
      "id": "chrome-responsive-iframe-article-20261003",
      "url": "https://developer.chrome.com/blog/responsive-iframes",
      "type": "maintainer_article"
    },
    {
      "id": "css-sizing4-responsive-iframe-draft-20261003",
      "url": "https://drafts.csswg.org/css-sizing-4/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-responsive-iframe-proposal-20261003",
      "url": "https://github.com/kojiishi/html/commit/8fd9d8995c50145b4eda0c18382aa81880f877ee",
      "type": "github_repository_analysis"
    },
    {
      "id": "wpt-responsive-iframe-contract-20261003",
      "url": "https://github.com/web-platform-tests/wpt/blob/2beef90fd2b8d267ebed384b0edc7f59f309f991/css/css-sizing/responsive-iframe/responsive-iframe-allow-origins.html",
      "type": "github_repository_analysis"
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

# Chrome 154 responsive iframe: opt-in・origin許可・明示更新の境界

## 問いと採用判断

高さが変わるコメント欄やフォームを iframe で提供するとき、親子間の寸法通知を `frame-sizing` だけで置き換えられるか。答えは、親の CSS、子の文書初期化、寸法公開先の許可、動的更新の通知を協調させた場合に限る。常時自動追従や、どのサイトからでも子を測定できる API として導入しない。

[Chrome 154 release notes][release] は2026-09-22の stable release で導入を記載する。取得は2026-10-03 UTC。以下はその公開説明と仕様案・テストを比較した記録であり、実機確認やブラウザー間相互運用性の認定ではない。既存の CSS containment 文書の描画省略とは異なり、別文書の寸法を親へ公開する条件を扱う。

## 親の指定と子の許可は別々に必要

[Chrome チームの記事][article] は、親の iframe に `frame-sizing: content-height`、子の初期 HTML の head に `name="responsive-embedded-sizing"` と `content="allow-origins=…"` を持つ meta を配置する構成を示す。子を編集できない第三者 iframe へ親の CSS だけを追加しても、この構成にはならない。

[CSS Sizing4 §5.3][draft] が定義するのは internal layout intrinsic size の利用である。`auto` はその寸法を利用しない。`content-width` / `content-height` は物理的な一軸、`content-inline-size` / `content-block-size` は論理的な一軸を選び、他方は通常の規則に従う。論理軸の writing mode は子文書ではなく iframe 要素側から決める。

したがって、これは author CSS の固定寸法をすべて上書きする指定ではない。採用時は既存の固定 `height` と min/max 制約も棚卸しする。縦に伸ばすウィジェットなら幅を親側で決め、対象ブラウザーで高さを `auto` に切り替える、というのが本稿の設計判断。`max-height` を残す場合は、長い内容への到達方法を別途確認する。

### allow-origins は埋め込み許可そのものではない

Chrome 記事は、`allow-origins=https://publisher.example` によって寸法を利用できる親 origin を限定し、複数 origin を空白で列挙できるとする。`*` は広く許す例であり、第三者ウィジェットには必要な公開先だけを指定することを勧めている。CSP `frame-ancestors` が担う「誰がこのページを埋め込めるか」と、寸法の公開先は分けて管理する。[記事の cross-origin 節][article]

[固定した WPT][wpt] には重要な負例がある。同一 origin のテストでも、content 欠落、空値、prefix のない `*`、`alloworigins=*`、大文字の `ALLOW-ORIGINS=*` は内容高400pxへの追従を期待せず、既定高150pxを期待する。cross-origin の親を許すには親 origin が照合対象であり、子自身だけを許すケースは非追従を期待する。150/400はこの fixture の数値であり、全製品の既定寸法を定める値ではない。テストの期待値を読んだ結果で、実行成功の報告ではない。

## 初期化時点には資料間の差がある

2026-10-03に読んだ資料を一つの確定仕様として混ぜない。

- CSS Sizing4 Editor's Draft 2026-09-06 は、初期 parse 中に meta を見つけるか body が開かれるかで flag を確定し、Document の生存中は変更しないと記述する。一方、例は content のない bare meta で、allow-origins の照合を記述していない。[§5.3.1][draft]
- Chrome 記事2026-09-16は `allow-origins=` を含めた meta を示し、子の読み込み後の動的挿入では有効化できないと説明する。[article]
- [WHATWG HTML PR #12444][proposal-pr] は取得時 open / 未merge。解析した実体は commit `8fd9d8995c50145b4eda0c18382aa81880f877ee` の差分で、meta の挿入・name/content の変更時に判定し、未確定 flag を Document の reveal 時に false にする案である。body 開始を締切とする CSS 草案とは同一でない。[固定差分][proposal]

同提案は小文字の `allow-origins=` prefix に続く source list を読み、container document の origin を照合する。flag が確定済みなら再判定せず、一文書に同名 meta を複数置かない要件も追加している。これは未merge提案の分析であり、Chrome 154の全挙動を実装ソースから証明したものではない。

実務上は、子の初期 head に許可先を明示した meta を一つだけ出力する。クライアント側の mount 後挿入、meta 削除による許可撤回、body 開始直後の際どい有効化に依存しない。この構成は資料のずれを避ける設計判断であり、「あらゆる動的挿入が全版で禁止」と一般化する根拠ではない。

## 動的更新は requestResize で明示する

[CSS 草案][draft] は `DOMContentLoaded` 後の最初の layout と `Window` の `load` で内部寸法を更新し、その後の content/style/layout 変更では自動更新しないとする。子が追加コメントや展開内容を DOM に反映した後で `window.requestResize()` を呼ぶ。Chrome 記事も継続的な layout 監視を行わず、変更をまとめてから呼ぶモデルを説明する。[article]

[WPT の動的更新用 resource][wpt-update] は load 後の animation frame で要素の寸法を変更し、その後に requestResize を呼ぶ。これを「必ず requestAnimationFrame 内で呼ぶ」という一般要件にはしない。フレームワークでは state setter が返った時点と DOM 更新完了を同一視せず、実際の反映後に通知するのが本稿の設計判断である。

`requestResize()` の存在確認だけでは呼出し資格を証明できない。CSS 草案では top-level document、iframe 以外の埋め込み、opt-in flag が未確定/false の場合に `NotAllowedError` を投げる。メソッドは保留中の style/layout を適用して寸法を更新するが、返り値は `undefined` であり、親が採用した高さを返す完了通知ではない。[§5.3.2][draft] [WPT error cases][wpt-errors] も親 window と opt-in のない子からの例外を期待する。

親の `frame-sizing` が `auto` のままなら内部寸法は利用されない。したがって例外なしの呼出しを「画面の高さが変わった」という証拠にせず、親での最終寸法と内容到達性を確認する。

### resize loop と ICB を区別する

CSS 草案は opt-in した子の最初の layout で initial containing block の寸法を記憶し、後続 layout にも使う locked embedded ICB size を定義する。目的は、子が viewport より少し大きくなるたびに親が伸び続けるような循環の抑制である。[draft]

この草案には、frame-sizing の off/on でその記憶を忘れさせるかという未解決の問いが残る。CSS の切替を正式なリセット操作として利用しない。viewport 基準の長さや条件付きレイアウトを使うウィジェットでは、親幅の変更と内容変更を別々に試験する。常時監視する独自 observer を重ねて無条件に通知し続けるより、内容を変更する処理の所有者が更新単位を決める方が検証しやすい、というのが設計上の判断である。

## 移行と回帰確認

以下は公式必須テストの一覧ではなく、本稿が提案する導入時の確認項目である。

1. 親の CSS support と子の requestResize support を別々に確認する。さらに子の初期 meta、実際の親 origin、配布されたテンプレートを確認する。`@supports` が真でも子の許可までは保証しない。
2. opt-in なし・非対応ブラウザー・許可しない親でも、固定高とスクロール等で内容に到達できる fallback を維持する。既存の寸法通知と新方式が同時に高さを書き換えないよう、親子で採用方式を揃える。
3. 初回表示、load 後の追加・削除・折りたたみ、画像等の遅延到着、親幅の変更を試す。減少方向も含め、内容が隠れないことと更新が循環しないことを測る。
4. 許可originの一致・不一致、prefix 欠落/誤字、初期metaの欠落、読み込み後の挿入を負例にする。認証や機微情報を含む子では、寸法公開を必要な相手へ絞る。
5. 初期表示から最終寸法までの layout shift と操作対象の移動を測る。Chrome 記事は特に first viewport での Core Web Vitals への悪影響を注意している。固定高を外せたことを性能改善と自動判定しない。[article]

## 取得・ライセンス・限界

- Chrome release notes と記事、CSSWG draft の実ページを開き、表示日とライセンスを確認した。Google文書は本文 CC-BY-4.0 / 例コード Apache-2.0、CSS草案はリンク先の W3C Software and Document License 2023。ここでは独自の日本語要約を作り、例コードや仕様本文は転載していない。
- HTML提案は上記40桁commitの差分と LICENSE、WPTは `2beef90fd2b8d267ebed384b0edc7f59f309f991` の対象テスト・resource・LICENSE.md を読んだ。HTMLは CC-BY-4.0（ソースコード組込み部分は BSD-3-Clause）、WPTは BSD-3-Clause。WPT commit は2026-08-27、HTML提案commitは2026-09-10であり、取得日時と混同しない。
- GitHubの一部ページはweb取得でtimeout/cache miss、Gitilesのファイル取得も失敗したため、同じ固定commitをGitHub read-only connectorで確認した。HTMLの大きな source ファイルの範囲取得は空応答だったため、固定commitの差分を根拠にした。未取得内容を補完した主張はない。
- ブラウザー実機、WPT実行、Chromium154の実装コード、Safari/Firefoxの最新対応、sandbox/opaque origin・多段iframeの挙動は未検証。草案の細部や古いWPTの期待値だけで製品挙動を保証しない。取得時点の提案状態と公開記事を再確認してから配布対象へ適用する。
- expires_at は release_notes の30日TTLに合わせ2026-11-02。再確認では対応版に加え、HTML提案のmerge状況、flag確定時点、allow-origins構文とCSS草案の整合を調べる。

[release]: https://developer.chrome.com/release-notes/154
[article]: https://developer.chrome.com/blog/responsive-iframes
[draft]: https://drafts.csswg.org/css-sizing-4/#responsive-iframes
[proposal-pr]: https://github.com/whatwg/html/pull/12444
[proposal]: https://github.com/kojiishi/html/commit/8fd9d8995c50145b4eda0c18382aa81880f877ee
[wpt]: https://github.com/web-platform-tests/wpt/blob/2beef90fd2b8d267ebed384b0edc7f59f309f991/css/css-sizing/responsive-iframe/responsive-iframe-allow-origins.html
[wpt-update]: https://github.com/web-platform-tests/wpt/blob/2beef90fd2b8d267ebed384b0edc7f59f309f991/css/css-sizing/responsive-iframe/resources/iframe-contents-request-resize.html
[wpt-errors]: https://github.com/web-platform-tests/wpt/blob/2beef90fd2b8d267ebed384b0edc7f59f309f991/css/css-sizing/responsive-iframe/responsive-iframe-request-resize-error.html

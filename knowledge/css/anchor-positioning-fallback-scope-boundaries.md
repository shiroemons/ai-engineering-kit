---
{
  "id": "css-anchor-positioning-fallback-scope-boundaries",
  "title": "CSS Anchor Positioning の fallback・anchor-scope と意味関係の境界",
  "kind": "knowledge",
  "technology": "css",
  "version": "W3C WD 2026-03-27; editor-hosted September 2026 snapshot; Safari 27.0 release 2026-09-17; MDN references retrieved 2026-10-02",
  "tags": [
    "research-domain:frontend",
    "anchor-positioning",
    "anchor-scope",
    "position-anchor",
    "position-area",
    "position-try-fallbacks",
    "position-try-order",
    "position-visibility",
    "fallback",
    "accessibility"
  ],
  "sources": [
    {
      "id": "w3c-css-anchor-position-wd-20260327-verified-20261002",
      "url": "https://www.w3.org/TR/2026/WD-css-anchor-position-1-20260327/",
      "type": "official_docs"
    },
    {
      "id": "csswg-anchor-position-september-snapshot-20261002",
      "url": "https://drafts.csswg.org/css-anchor-position-1/",
      "type": "official_docs"
    },
    {
      "id": "webkit-safari27-anchor-release-20261002",
      "url": "https://webkit.org/blog/18325/webkit-features-for-safari-27-0/",
      "type": "release_notes"
    },
    {
      "id": "mdn-position-anchor-reference-20261002",
      "url": "https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/position-anchor",
      "type": "official_docs"
    },
    {
      "id": "mdn-anchor-scope-reference-20261002",
      "url": "https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/anchor-scope",
      "type": "official_docs"
    },
    {
      "id": "mdn-anchor-fallback-guide-20261002",
      "url": "https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Anchor_positioning/Try_options_hiding",
      "type": "official_docs"
    },
    {
      "id": "chrome-anchor-syntax-migration-20261002",
      "url": "https://developer.chrome.com/blog/anchor-positioning-api",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# CSS Anchor Positioning の fallback・anchor-scope と意味関係の境界

## 解決する問い

繰り返し表示する部品の補足パネルを正しい起点へ結び付け、画面端でも操作可能に保つには何を指定し、何を別途実装・検証するか。対象は位置決め、名前の隔離、overflow 時の候補選択であり、dialog や popover の開閉 API 全体は扱わない。

結論は、参照先・配置候補・意味関係を別々に設計すること。CSS の配置成功だけを、正しい読み上げやキーボード操作の成功と扱わない。以下の「確認した契約」は出典の要約、「推奨判断」はこの資料の独自の設計提案である。

## 確認した契約

### 1. position-anchor の初期値を古い解説から推測しない

`position-anchor` は default anchor を指定する。明示的な参照は起点の `anchor-name` と参照側の同名指定で結ぶ。`absolute` / `fixed` な配置要素に、`position-area` または `anchor()` などによる配置も必要になる。

現在参照した値定義では、初期値は `normal`。`position-area: none` なら `none` として、それ以外なら `auto` として振る舞う。`auto` は利用可能な implicit anchor を使い、`none` は default anchor を持たない。[MDN position-anchor](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/position-anchor)

Safari 27.0 の開発元発表は、この初期値変更を出荷したことと、アンカー配置を使わない要素まで以前の `auto` の影響を受ける問題を理由として記録している。同版は transform-aware な追従も追加した。[WebKit Safari 27.0](https://webkit.org/blog/18325/webkit-features-for-safari-27-0/)

### 2. anchor-scope は明示名の境界

`anchor-scope` の初期値 `none` は隔離を行わない。`all` または名前の列挙で subtree の名前参照を制限できる。implicit anchor association はこの隔離の対象外である。[MDN anchor-scope](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/anchor-scope)

同名候補の選択を「常に DOM の最後」と単純化しない。公開 WD の検索手順は条件を満たす nearest ancestor を先に、なければ条件を満たす tree order の最後を選ぶ。`contain` による layout/style containment だけでは名前を隔離しない。[W3C WD §2](https://www.w3.org/TR/2026/WD-css-anchor-position-1-20260327/)

### 3. position-try-fallbacks の候補と組合せ

- `flip-block, flip-inline` は別々の候補。両方の反転が必要なら `flip-block flip-inline` という一候補も列挙する
- `position-area` の値や `@position-try` の名前も候補にできる
- `position-try-order` は利用可能な空間を基準に候補を優先できる。`position-try` は order と fallbacks の shorthand である

これは候補を指定する仕組みであり、あらゆる画面に収まる保証ではない。[MDN fallback guide](https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Anchor_positioning/Try_options_hiding)

公開 WD の overflow 判定は配置要素の margin box と inset-modified containing block を比較する。子孫だけのはみ出しはこの判定に含めない。`@position-try` で設定できるのは inset・margin・size・self-alignment・`position-anchor`・`position-area` の限定されたプロパティで、`!important` を付けた宣言は無効となる。[W3C WD §6.4–6.5](https://www.w3.org/TR/2026/WD-css-anchor-position-1-20260327/)

### 4. fallback の履歴と資料間の差

公開 WD は選ばれた候補を再び overflow するまで保持する。ただし配置・包含ブロック・fallback 関連宣言などの変更では履歴を解除する規則もある。すべての候補が不適合なら current styles を返し、以前に成功した配置が残る場合がある。MDN guide の「元の位置へ戻る」という説明を履歴に依存しない一般則として採用しない。[W3C WD §6](https://www.w3.org/TR/2026/WD-css-anchor-position-1-20260327/) / [MDN の該当説明](https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Anchor_positioning/Try_options_hiding#predefined_fallback_options)

### 5. position-visibility の新旧名と accessibility bindings

編集者側の取得本文は `anchor-valid` / `anchor-visible` という単数形を定義する。前者は使用している default anchor の解決失敗、後者は存在する default anchor の不可視・途中の要素による完全な clipping を条件とする。`no-overflow` は候補適用後も収まらない場合の非表示条件。古い `anchors-valid` / `anchors-visible` は許容可能な legacy alias であり、必須対応ではない。[CSSWG 取得本文 §6.6](https://drafts.csswg.org/css-anchor-position-1/)

Safari 27.0 は単数形と default anchor に絞った判定を出荷し、旧複数形を一時的に支えると説明している。他ブラウザや将来版にも同じ alias があるとは一般化しない。[WebKit の変更説明](https://webkit.org/blog/18325/webkit-features-for-safari-27-0/)

Anchor Positioning 自体は要素間の意味関係や accessibility bindings を作らない。必要な関係は適切な markup で表現する。[W3C WD §7](https://www.w3.org/TR/2026/WD-css-anchor-position-1-20260327/)

## 推奨判断と確認手順

以下は独自の設計提案であり、ブラウザ実測結果ではない。

1. **部品を二つ以上並べて参照先を試す**。起点とパネルを含む部品ルートに対象名の `anchor-scope` を置く。portal でパネルを別 subtree に移す設計なら同じ隔離に残ると仮定せず、配置先か命名方針を再検討する
2. **意図する default anchor を明示する**。名前で結ぶなら `position-anchor: --部品名`、implicit anchor を `anchor()` で使うなら `auto` の必要性を確認する。`normal` と以前の初期値の違いを migration テストに入れる
3. **候補は優先順位と角の条件から設計する**。上下だけでなく四隅、RTL、縦書きを確認する。本文が長い場合は候補を増やすだけで済ませず、サイズ上限、折り返し、スクロール可能な内容領域を検討する
4. **一度成功した配置からの変化を試す**。初回表示だけでなく、スクロール、resize、文字拡大、翻訳による長文化、起点の削除、部品の追加を連続して行う。全候補不適合時にも重要な操作へ到達できるかを確認する
5. **非表示を業務上の判断と混同しない**。`position-visibility: no-overflow` で隠してよいのは、情報・操作の代替経路を確保した場合とする。エラーや必須アクションが消える設計は避ける。clipping、非表示、再表示の間の focus 所在も確認する
6. **意味と操作を独立にレビューする**。アクセシブルな名前・説明・状態の通知、DOM の読み順、Tab 移動、開閉後の focus を確認する。CSS を無効化した状態や非対応環境でも目的を完了できる構成を基準にする
7. **構文検出と実動作確認を分ける**。必要な値ごとに `@supports` を入口として段階的に適用し、実際のサポート対象ブラウザで再利用部品と overflow の試験を行う。既存の配置方法を外す判断はその結果で行う

## 版差・適用範囲・provenance

- 公開仕様の固定根拠は W3C Working Draft 27 March 2026。Recommendation として確定した仕様とは扱わない
- 2026-10-02 の編集者側 URL は「W3C Working Draft, 6 September 2026」と表示したが、そこからリンクされた [2026-09-06 の TR URL](https://www.w3.org/TR/2026/WD-css-anchor-position-1-20260906/) は 404 だった。`/TR/css-anchor-position-1/` は今回の取得では 3月版を返した。9月本文は取得時点の編集者側資料として区別し、9月 TR の公開を確認済みとしない
- Chrome 記事は公開・更新表示が 2024-05-10 で、本文は Chrome 125 当時のもの。追記は `inset-area` → `position-area`、`position-try-options` → `position-try-fallbacks` が Chrome 129 で出荷されたと説明する。旧名のサンプルをそのまま現行契約とみなさない。[Chrome migration note](https://developer.chrome.com/blog/anchor-positioning-api)
- MDN 本文更新日は position-anchor が 2026-05-11、anchor-scope が 2026-07-26、fallback guide が 2026-09-06。取得時の Baseline 表示は position-anchor が 2026年9月、anchor-scope が同1月だった。表示月はプロパティ単位の観測で、記事更新日や全構文の対応日とは区別する。「Anchor Positioning 全体が8月に完全対応」という主張は今回の一次資料から確立していない
- ライセンスは W3C Software and Document License 2023、MDN 文書は Mozilla Contributors の CC-BY-SA-2.5-or-later、Chrome 記事本文は CC-BY-4.0。WebKit 記事の再利用ライセンスは取得ページから特定できず `unknown` とした。本文は技術的事実の独自要約と設計判断であり、原文・図・サンプルコードは移植していない。[W3C license](https://www.w3.org/copyright/software-license-2023/) / [MDN license](https://developer.mozilla.org/en-US/docs/MDN/Writing_guidelines/Attrib_copyright_license)
- 取得日はすべて 2026-10-02 UTC。release_notes の30日 TTL を含むため明示期限は 2026-11-01。取得失敗した9月 TR、各値の対応状況、fallback 履歴の説明差を次回も再確認する

## 未確認事項

ブラウザ実機、スクリーンリーダー、WebView、印刷、複数の Shadow DOM、全書字方向の組合せは未試験。transform-aware な挙動は Safari 27.0 の出荷説明を確認しただけで他エンジンの同等性を確認していない。検索 eval は本資料の発見可能性を検査するもので、描画や accessibility の適合性試験ではない。

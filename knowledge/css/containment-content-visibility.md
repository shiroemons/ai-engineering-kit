---
{
  "id": "css-containment-content-visibility",
  "title": "CSS containment contain and content-visibility rendering skip scope and accessibility caveats",
  "kind": "knowledge",
  "technology": "css",
  "version": "CSS Containment L2 Editor's Draft 2026-06-23 + W3C WD 2022-09-17 + web.dev content-visibility 2025-09-23 + MDN content-visibility 2026-07-26; retrieved 2026-09-27",
  "tags": [
    "research-domain:frontend",
    "contain",
    "content-visibility",
    "containment",
    "size-containment",
    "layout-containment",
    "paint-containment",
    "style-containment",
    "contain-intrinsic-size",
    "rendering",
    "hit-testing",
    "relevance",
    "accessibility",
    "aria-hidden"
  ],
  "sources": [
    {
      "id": "css-contain-2-ed-2026-06-23",
      "url": "https://drafts.csswg.org/css-contain-2/",
      "type": "official_docs"
    },
    {
      "id": "css-contain-2-wd-2022-09-17",
      "url": "https://www.w3.org/TR/css-contain-2/",
      "type": "official_docs"
    },
    {
      "id": "webdev-content-visibility-2025-09-23",
      "url": "https://web.dev/articles/content-visibility",
      "type": "maintainer_article"
    },
    {
      "id": "mdn-content-visibility-docs",
      "url": "https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/content-visibility",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "maintainer",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# CSS containment contain and content-visibility rendering skip scope and accessibility caveats

`contain` の4種の分離範囲と、`content-visibility: hidden / auto` の描画スキップ条件・検索/フォーカス可否・アクセシビリティ上の注意点。[CSS Containment L2 Editor's Draft](https://drafts.csswg.org/css-contain-2/) / [W3C WD 2022-09-17](https://www.w3.org/TR/css-contain-2/) / [web.dev content-visibility](https://web.dev/articles/content-visibility) / [MDN content-visibility](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/content-visibility)

## 要点

以下は公式一次情報の記述であり、推奨構成そのものではない。

### contain の値と各分離の範囲 (CSS Containment L2 Editor's Draft + W3C WD)

- `contain: strict` は `size layout paint style` と等価、`contain: content` は `layout paint style` と等価である。
- `size` は要素を内容がないかのようにサイズ決定する (size-as-if-empty)。内容量に依存したサイズ拡大をその要素の外に伝播させないための分離である。
- `layout` は独立した整形文脈 (independent formatting context) を確立し、子孫の `absolute` / `fixed` に対する包含ブロックとなり、stacking context を形成する。
- `paint` は描画を overflow clip edge でクリップし、stacking context を形成する。
- `style` はカウンタ (counters) とクォート (quotes) の作用範囲をその要素の子树に閉じる。
- 初期値は `contain: none`、`content-visibility: visible` であり、いずれもアニメーション不可 (not animatable) である。
- `size` / `layout` / `style` / `paint` は、`display: contents` / `display: none` の要素や、table・ruby・非アトミックな inline ボックスなど、適用対象外の条件では効果を持たない (no-effect cases)。

### content-visibility のスキップ条件と relevance (Editor's Draft + W3C WD + MDN)

- `content-visibility: hidden` は内容の描画をスキップし、その内容は find-in-page で見つからず、フォーカス可能・選択可能であってはならない (must not be findable / focusable / selectable)。
- `content-visibility: auto` は関連性がない (not relevant) 場合にのみ内容をスキップする。関連性がある場合は描画され、find-in-page・tab 順序・フォーカス・選択の対象であり続けなければならない。
- 関連性 (relevance) は、画面との交差 (on-screen intersection)、フォーカス、選択 (selection)、top-layer を含む。いずれかに該当すれば関連性がある。
- スキップされた内容は4種すべての containment が適用され、描画されず (no paint)、hit-testing の対象にもならない。
- MDN の formal definition は、初期値 `visible`、size containment が適用可能な要素に適用、非継承 (non-inherited)、discrete なアニメーション型、`visible` が全区間で適用される補間動作 (visible-throughout) と定義する。
- スキップ時の描画延期の制約は W3C WD の §4.4 Restrictions と §4.5 Accessibility Implications が規定する。

### 画面外の描画停止と placeholder (web.dev)

- `content-visibility: auto` は `layout` / `style` / `paint` を付与し、画面外では size containment が加わる。描画と hit-testing は停止し、viewport 近傍で描画が再開する。
- 画面外のサイズ推定には `contain-intrinsic-size` の placeholder (例 `1000px`) を指定する。`auto` キーワードは最後に描画されたサイズを記憶し、スクロールバーのずれを抑える。
- `content-visibility: hidden` は `display: none` と異なり描画状態を保持する。表示再開のための状態破棄を伴わない点が違いである。
- Chrome 85+ で利用可能であり、2025-09-15 に Baseline newly available となった。

### アクセシビリティ上の注意 (web.dev + MDN)

- `content-visibility: auto` で画面外スキップ中の要素は DOM と accessibility tree に残る。意図的に隠しているランドマークなどには `aria-hidden="true"` を併用する (web.dev の記述)。
- MDN の Accessibility 節も、画面外の `auto` 要素は DOM と accessibility tree に残ること、描画されていない `display: none` / `visibility: hidden` の子孫も現れることを明記し、`aria-hidden="true"` の併用を求める。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が定める実装構成そのものではない。

- 長い一覧や折りたたみ直下など、画面外に大量の DOM を持つ区画にだけ `content-visibility: auto` を付ける。ページ全体やフォーカスが頻繁に移動する容器への一括指定は避け、relevance による再描画の単位で付与する。
- `auto` と併せて `contain-intrinsic-size` の placeholder を必ず指定する。最終描画サイズの記憶 (`auto` キーワード) だけに頼らず、初回描画前のスクロールバーずれを見積もった固定値で補う。

```css
.offscreen-section {
  content-visibility: auto;
  contain-intrinsic-size: 1000px;
}
```

- `display: none` の代わりに状態保持目的で隠す場合だけ `content-visibility: hidden` を使う。検索・フォーカス・選択の対象外になることを前提にし、再表示時にフォーカス回復が必要なら表示切り替え側で管理する。
- `contain: strict` / `contain: content` の単独利用は、レイアウト分離が目的の部品 (独立したカード、埋め込みウィジェット枠) に限定する。`size` を含む場合は内容量に応じた高さ変化が外に伝わらなくなるため、高さ可変の本文容器には付けない。
- 意図的な非表示 (ランドマークの無効化、未展開パネルの秘匿) では `content-visibility` だけに頼らず `aria-hidden="true"` を併用する。画面外スキップと支援技術への公開可否を別々に制御する。

## 避ける使い方

- **`content-visibility: auto` の画面外要素が accessibility tree から消えるという想定**。DOM と accessibility tree に残るため、意図的非表示のつもりが支援技術には公開される。秘匿が目的なら `aria-hidden="true"` を併用する。
- **`content-visibility: hidden` の内容が find-in-page や Tab 順序に入るという想定**。仕様上は見つからず・フォーカス不可・選択不可である。検索導線やキーボード導線に含める設計には使わない。
- **placeholder なしの `auto` でスクロール位置が安定するという想定**。画面外サイズが 0 扱いに近づきスクロールバーがずれる。`contain-intrinsic-size` の見積もりを省かない。
- **`display: none` と `hidden` が同じ状態破棄をするという想定**。`hidden` は描画状態を保持する。状態破棄によるメモリ解放を期待して使わない。
- **`contain: size` を含む指定を高さ可変の本文容器に付ける**。内容量に応じたサイズ変化が外に伝わらず、はみ出しや重なりの原因になる。分離の単位とサイズ可変の単位を分ける。
- **table・ruby・非アトミック inline・`display: contents` / `none` に containment を期待する**。仕様上の no-effect 条件であり、分離が効かない。対象要素の表示型を先に確認する。

## 適用版と本番での注意

- `CSS Containment Module Level 2 Editor's Draft 23 June 2026` と `W3C Working Draft 17 September 2022` で値定義・スキップ意味・no-effect 条件・§4.4 / §4.5 の規定を、`web.dev content-visibility` (published 2020-08-05、updated 2025-09-23、Baseline newly available 2025-09-15) と `MDN content-visibility` (last modified 2026-07-26) で実務上の placeholder と accessibility tree の扱いを、それぞれ 2026-09-27 取得の内容で確認した。将来の最新とは扱わない。
- 本文は `expires_at` 2026-12-26 (official_docs / maintainer_article TTL 90日)。Editor's Draft は継続改訂のため、期限到来時に値定義と relevance 条件の改訂を再確認する。
- **未確認**: 各ブラウザの `content-visibility` 対応版の完全な対応表 (Chrome 85+ と Baseline 到達以外は今回の根拠に含めない)、`contain-intrinsic-size` の詳細構文の版差、スクリーンリーダーごとの `aria-hidden` 併用時の読み上げ実測。本文は仕様と Chrome チーム記事・MDN の記述のみを根拠にし、実機測定は含まない。
- 本文の付与単位・placeholder 見積もり・`aria-hidden` 併用の判断は設計判断であり、公式仕様の契約そのものではない。境界は各節の書き分けに従う。

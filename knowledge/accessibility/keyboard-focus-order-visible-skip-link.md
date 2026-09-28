---
{
  "id": "accessibility-keyboard-focus-order-visible-skip-link",
  "title": "Accessibility keyboard focus order, visible focus indicator, and skip-link scope per WCAG 2.2",
  "kind": "knowledge",
  "technology": "accessibility",
  "version": "WCAG 2.2 W3C Recommendation 12 December 2024 + Understanding 2.4.1/2.4.3/2.4.7/2.4.13; retrieved 2026-09-28",
  "tags": [
    "research-domain:frontend",
    "keyboard",
    "focus-order",
    "focus-visible",
    "focus-appearance",
    "skip-link",
    "bypass-blocks",
    "tabindex",
    "focus-indicator",
    "dialog",
    "landmarks",
    "accessibility",
    "wcag"
  ],
  "sources": [
    {
      "id": "wcag22-rec-2024-12",
      "url": "https://www.w3.org/TR/WCAG22/",
      "type": "official_docs"
    },
    {
      "id": "wcag22-understanding-focus-order",
      "url": "https://www.w3.org/WAI/WCAG22/Understanding/focus-order.html",
      "type": "official_docs"
    },
    {
      "id": "wcag22-understanding-focus-visible",
      "url": "https://www.w3.org/WAI/WCAG22/Understanding/focus-visible.html",
      "type": "official_docs"
    },
    {
      "id": "wcag22-understanding-bypass-blocks",
      "url": "https://www.w3.org/WAI/WCAG22/Understanding/bypass-blocks.html",
      "type": "official_docs"
    },
    {
      "id": "wcag22-understanding-focus-appearance",
      "url": "https://www.w3.org/WAI/WCAG22/Understanding/focus-appearance.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Accessibility keyboard focus order, visible focus indicator, and skip-link scope per WCAG 2.2

キーボードの focus order (フォーカス順序)・visible focus indicator (可視フォーカス表示)・skip-link (スキップリンク) の適用範囲を WCAG 2.2 で使う方法。[WCAG 2.2 Recommendation](https://www.w3.org/TR/WCAG22/) / [Understanding 2.4.3 Focus Order](https://www.w3.org/WAI/WCAG22/Understanding/focus-order.html) / [Understanding 2.4.7 Focus Visible](https://www.w3.org/WAI/WCAG22/Understanding/focus-visible.html) / [Understanding 2.4.1 Bypass Blocks](https://www.w3.org/WAI/WCAG22/Understanding/bypass-blocks.html) / [Understanding 2.4.13 Focus Appearance](https://www.w3.org/WAI/WCAG22/Understanding/focus-appearance.html)

## 要点

以下は公式一次情報の記述であり、推奨構成そのものではない。

### 規範の範囲 (WCAG 2.2 Recommendation 12 December 2024)

- WCAG 2.2 は WCAG 2.1 を拡張する勧告であり、適合レベル A / AA / AAA を持つ。
- 本書の対象は 2.4.1 Bypass Blocks (Level A)、2.4.3 Focus Order (Level A)、2.4.7 Focus Visible (Level AA)、2.4.13 Focus Appearance (Level AAA) である。
- WCAG 2.2 で追加された SC には 2.4.11 / 2.4.12 Focus Not Obscured と 2.4.13 Focus Appearance が含まれる。本書は 2.4.11 / 2.4.12 の判定条件の詳細には立ち入らず、存在のみを範囲として示す。

### 2.4.3 Focus Order: 意味を保つ順序 (Level A)

- ページを逐次ナビゲートでき、かつその順序が意味や操作に影響する場合、フォーカス可能な構成要素は意味と操作性を保つ順序でフォーカスを受け取る。
- DOM 順と視覚順の一致が基本であり、対応する十分な手法として C27 (CSS による順序付け) と G59 (視覚的提示に従う順序での要素配置) が挙げられる。
- トリガーで開く dialog はトリガーの直後に挿入する構成が対応例であり、SCR26 (動的コンテンツを DOM 順序へ挿入) と H102 (論理タブ順序から外れたコンテンツの削除) が挙げられる。
- 失敗例として F44 (正の `tabindex` によるタブ順序の並べ替えで意味や操作性が損なわれる) と F85 (非隣接の dialog や menu が正しい順序にない) が挙げられる。

### 2.4.7 Focus Visible: 常に見える表示の存在 (Level AA)

- キーボードで操作できるすべての UI について、フォーカス表示が見える操作モードが存在し、その表示は時間制限で消えないことが求められる。
- 十分な手法として G149 (フォーカス時にユーザエージェント既定の強調を変えない)、C15 (CSS によるフォーカス表示の変更)、G165 (キーボードフォーカスのハイライト表示)、G195 (作成者が提供する視覚的に明確なフォーカス表示)、C40 (二色のフォーカス表示)、C45 (`:focus-visible` による関連性のないポインタ入力時のフォーカス表示抑制) が挙げられる。
- 失敗例として F55 (フォーカス時に可視フォーカスを除去する) と F78 (代替なしに `outline:none` や `border:none` でフォーカス表示を除去する) が挙げられる。
- Understanding は 1.4.11 Non-text Contrast と 2.4.13 Focus Appearance を関連 SC として示す。

### 2.4.1 Bypass Blocks: 繰り返しブロックの迂回 (Level A)

- 複数のページ (set of pages) で繰り返されるブロックを迂回する手段が求められる。skip-link の適用範囲は、繰り返される navigation・header・filters のようなブロックであり、単発ページ内の一回限りの見出しではない。
- 十分な手法として G1 (本文へ直接移動する skip-link の追加)、G123 (ブロックの末尾へ直接移動するリンクの追加)、G124 (コンテンツ領域の先頭へのリンク追加) が挙げられる。
- skip-link 以外の代替として ARIA landmark による迂回 (ARIA11: landmark でリージョンを特定) と見出し群による迂回 (H69: 各セクション先頭に見出しを置く) が挙げられる。

### 2.4.13 Focus Appearance: 表示時の面積とコントラスト (Level AAA)

- フォーカス表示が見える場合、非フォーカス時の構成要素またはサブコンポーネントの周囲 (perimeter) に相当する、2 CSS px 以上の太さの面積を持ち、同一ピクセル群の focused 状態と unfocused 状態の変化が 3:1 以上のコントラスト比を持つことが求められる。
- 例外は、表示がユーザエージェントにより決定され作成者が調整できない場合、または作成者がフォーカス表示も背景も変更していない場合に限られる。
- Understanding は、1.4.11 Non-text Contrast の隣接コントラスト (adjacent-contrast) とは異なり、同一ピクセルの状態間変化 (change-of-contrast) を測る点を区別し、C40 の二色表示を許容例として示す。

## 推奨方法

以下は上記の公式要件からの設計上のまとめであり、公式が定める実装構成そのものではない。

- DOM 順を視覚順と一致させ (C27/G59)、正の `tabindex` で順序を上書きしない。トリガーで開く dialog や menu はトリガー直後に DOM 挿入し (SCR26)、閉じたらトリガーへフォーカスを戻す。F44 と F85 の構成を避ける。
- ページ先頭に本文直行の skip-link (G1) を置き、長い繰り返しブロックには末尾直行 (G123) や領域先頭リンク (G124) を足す。単発の見出し移動と混同せず、繰り返される navigation・header・filters の迂回として範囲を決める。landmark (ARIA11) と見出し (H69) は skip-link の代替ではなく併用する公開構造として整える。

```html
<a class="skip-link" href="#main">Skip to main content</a>
<header><!-- repeated navigation --></header>
<main id="main" tabindex="-1">
  <!-- page body -->
</main>
```

```css
.skip-link {
  position: absolute;
  left: -9999px;
}
.skip-link:focus-visible {
  left: 0;
}
:focus-visible {
  outline: 3px solid currentColor;
  outline-offset: 2px;
}
```

- フォーカス表示は UA 既定を残すか (G149)、CSS で明確に置き換える (C15/G165/G195)。`outline:none` にする場合は代替表示を必ず用意し (F78 の回避)、ポインタ操作時のみ抑制したい場合は `:focus-visible` を使う (C45)。表示は時間で消さない。
- AAA を狙う場合のみ、2 CSS px 以上の周囲面積と 3:1 の状態間コントラストを満たす二色表示 (C40) を検討する。AA の 2.4.7 だけが要件の製品では、面積と 3:1 の厳密値を必須としない。

## 避ける使い方

- **正の `tabindex` でタブ順序を並べ替える**。F44 の失敗例であり、意味や操作性が損なわれる。順序変更は DOM 順の修正で行う。
- **トリガーから離れた位置に dialog や menu を置く**。F85 の失敗例であり、順序が正しくないと判定される。トリガー直後への挿入 (SCR26/H102) を省かない。
- **フォーカス時に可視表示を除去する (F55)**。フォーカス表示が見えるモードがなくなる。UA 既定の除去は代替表示とセットで行う。
- **`outline:none` を代替なしで使う (F78)**。キーボード利用者が現在位置を失う。C15/G165/G195/C40 のいずれかで置き換える。
- **単発の見出し移動だけを 2.4.1 の迂回手段とする**。2.4.1 は複数ページで繰り返されるブロックの迂回が要件であり、繰り返される navigation・header・filters の範囲を外さない。
- **AAA の 2.4.13 を AA 要件と混同する**。2 CSS px の周囲面積と 3:1 の状態間変化は Level AAA の条件であり、2.4.7 (Level AA) の合否に持ち込まない。

## 適用版と本番での注意

- `WCAG 2.2 W3C Recommendation 12 December 2024` の規範 SC 配置と、`Understanding 2.4.1 / 2.4.3 / 2.4.7 / 2.4.13` の各ページの記述 (十分な手法・失敗例・例外・関連 SC) を 2026-09-28 取得の内容で確認した。将来の最新とは扱わない。
- 本文は `expires_at` 2026-12-27 (official_docs TTL 90日)。WCAG 3.x への移行や Understanding の改訂時は、SC 文言と手法番号 (C27/G59/SCR26/H102/G149/C15/G165/G195/C40/C45/G1/G123/G124/ARIA11/H69、失敗例 F44/F85/F55/F78) の付け替えを再確認する。
- **未確認**: 各ブラウザの `:focus-visible` 対応版の完全な対応表、スクリーンリーダーごとのフォーカス順序と landmark・見出し迂回の読み上げ実測、2.4.11 / 2.4.12 の判定条件の詳細。本文は WCAG 2.2 の契約のみを根拠にし、実機測定は含まない。
- 本文の DOM 配置・skip-link 配置・フォーカス表示の具体値は設計判断であり、公式の達成手法の例そのものではない。境界は各節の書き分けに従う。

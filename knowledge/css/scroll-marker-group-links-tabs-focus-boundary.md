---
{
  "id": "css-scroll-marker-group-links-tabs-focus-boundary",
  "title": "CSS scroll-marker-group の links / tabs と focus・非アクティブ内容の境界",
  "kind": "knowledge",
  "technology": "css",
  "version": "Chrome 154 stable 2026-09-22; CSS Overflow 5 Editor Draft 2026-08-04; historical FPWD 2024-12-17 and Chrome 135 article 2025-03-20; retrieved 2026-10-02",
  "tags": [
    "research-domain:frontend",
    "scroll-marker-group",
    "scroll-marker",
    "scroll-target-group",
    "links",
    "tabs",
    "carousel",
    "focus",
    "accessibility-tree",
    "progressive-enhancement",
    "migration"
  ],
  "sources": [
    {
      "id": "chrome154-scroll-marker-modes-release-20261002",
      "url": "https://developer.chrome.com/release-notes/154",
      "type": "release_notes"
    },
    {
      "id": "csswg-overflow5-marker-modes-ed-20260804-verified-20261002",
      "url": "https://drafts.csswg.org/css-overflow-5/",
      "type": "official_docs"
    },
    {
      "id": "w3c-overflow5-fpwd-20241217-verified-20261002",
      "url": "https://www.w3.org/TR/2024/WD-css-overflow-5-20241217/",
      "type": "official_docs"
    },
    {
      "id": "chrome-carousel-135-article-20250320-verified-20261002",
      "url": "https://developer.chrome.com/blog/carousels-with-css",
      "type": "maintainer_article"
    },
    {
      "id": "mdn-scroll-marker-group-reference-20261002",
      "url": "https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/scroll-marker-group",
      "type": "official_docs"
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

# CSS scroll-marker-group の links / tabs と focus・非アクティブ内容の境界

## 解決する問い

CSS だけでスクロール位置の操作部品を作るとき、リンク式のナビゲーションとタブ式の切替をどう選ぶか。旧 carousel サンプルを引き継いだ際、見た目が同じでも Tab 移動や読み上げ対象が変わる危険をどう検出するか。

結論は、配置の `before` / `after` と操作モデルの `links` / `tabs` を別に決め、目的の mode を明示すること。特に複数カードを同時に読む一覧では、Tab 回数を減らす理由だけで `tabs` を採用しない。以下の確認事項は出典の要約、判断と試験手順は独自の設計提案である。

## Chrome 154 で確認した変更

2026-09-22 公開の stable release notes は `scroll-marker-group` の二つの mode を出荷したと記録する。ここでいう「既定 links」は **mode 省略時** の扱いであり、プロパティ自体の初期値 `none` を変更したという意味ではない。

| mode | group / marker の role | キーボードの入口 | 内容への影響 |
| --- | --- | --- | --- |
| `links` | `navigation` / `link` | 各 marker が順次 Tab 対象 | 元の内容要素の role を変更しない |
| `tabs` | `tablist` / `tab` | 原則 active marker が Tab 対象、矢印キーで移動 | marker の起点は `tabpanel`、inactive な内容は accessibility tree から隠れる |

`tabs` の起動後は marker に focus が残る。次の Tab で選択中の内容へ進む設計である。非アクティブ内容を隠す契約から、画面上でも必ず消える、DOM から削除される、という結論は出さない。[Chrome 154 release notes](https://developer.chrome.com/release-notes/154) / [CSSWG Editor Draft §3.1.6](https://drafts.csswg.org/css-overflow-5/)

## 実装・テストで必要な仕様上の境界

取得した CSSWG Editor Draft は 2026-08-04 表示の作業中の仕様である。以下はその記述であり、全ブラウザの実測結果ではない。

1. **生成条件**: group 指定に加え、`::scroll-marker` の `content` が `none` 以外であることが必要。所属先は最も近い ancestor scroll container の group
2. **links の起動**: marker の focus を外し、起点を `sequential focus navigation starting point` にする。release notes の「focus が移る」という要約を、対象への `focus()` 呼出しと同一視しない
3. **tabs の次の Tab**: 対象が focusable ならその対象へ、そうでなければ対象を起点に次の focusable 要素へ進む。inactive panel は通常の Tab 巡回対象にならない
4. **focus の観測**: marker に focus があるときの `activeElement` は関連 scroll container。`:focus` / `:focus-visible` は疑似要素側に適用され、group と container の集約した状態には `:focus-within` を使う

[CSSWG Editor Draft §3.1.5、§3.1.9–10、§3.3](https://drafts.csswg.org/css-overflow-5/)

既存の HTML `<a>` をグループ化する `scroll-target-group` と、疑似要素を生成する `scroll-marker-group` も区別する。existing anchors を残す方式に切り替えても、生成 marker の tabs 契約が自動的に移植されるとは考えない。[MDN scroll-marker-group](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/scroll-marker-group)

## 旧資料を読む際の migration 境界

- **2024-12-17 の FPWD**: 文法は `none | before | after`。group は暗黙の focusgroup と説明され、現在の `links` / `tabs` の分岐はない。2026-10-02 の取得でも undated TR URL はこの版を返した。公開 TR と Editor Draft を同じ本文として扱わない。[固定版 W3C FPWD](https://www.w3.org/TR/2024/WD-css-overflow-5-20241217/)
- **Chrome 135 時代の記事（2025-03-20）**: mode を付けない `after` の例とともに focusgroup / tablist を説明している。この説明を Chrome 154 の省略時 `links` にそのまま適用しない。今回確認したのは資料間の差であり、Chrome 135 から153までの全版での実測や変更履歴ではない。[Adam Argyle, Carousels with CSS](https://developer.chrome.com/blog/carousels-with-css)
- **MDN（最終更新表示 2026-07-26）**: 取得本文は formal syntax に `links | tabs` を含むが、値の解説は `before` / `after` / `none` だけである。Limited availability・experimental の表示もある。新 mode の説明は CSSWG と release notes に戻って確認する。[Mozilla Contributors, scroll-marker-group](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/scroll-marker-group)

この差からの独自の推奨は、既存コードの `scroll-marker-group: before` / `after` を棚卸しし、期待する操作モデルを先に言葉で決めること。すべてに機械的に `tabs` を追記して旧見た目を維持する変更は避ける。

## 選び方と段階的な移行手順

以下は独自の設計判断であり、仕様が定める唯一の構成ではない。

1. **全文・一覧として読めることが重要なら links を第一候補にする**。記事の節移動、複数の商品を比較する横スクロール一覧など、非選択の内容も引き続き参照する用途が該当する。大量の marker で Tab 回数が増えるなら、章単位に減らす、普通の目次リンクを使う、一覧の構成を見直すことも比較する
2. **一つの panel を選んで操作するなら tabs を候補にする**。選択中だけが読み上げ・順次移動の対象になることを画面の説明と一致させる。隣のカードが大きく見えるレイアウトでは、そのカードのボタンが読み上げ対象外になる可能性を検証し、必要なら links や通常の一覧へ戻す
3. **mode と配置をともに明示する**。例えば links を内容後方に置く方針なら `after links`、tabs を内容前方に置く方針なら `before tabs` と記録する。CSS で上下を入れ替える変更時は、視覚順だけでなく移動順もレビューする
4. **fallback は情報と操作を先に残す**。新 mode を使わない環境でも内容を読め、操作できる通常のリストやスクロール領域を基礎にする。`@supports (scroll-marker-group: before tabs)` のように必要な値を検出し、単に `before` が通ることを tabs 対応とみなさない。構文検出は accessibility tree と keyboard の動作試験の代用ではない
5. **既存 JavaScript との担当重複を解く**。矢印キー、roving tabindex、選択状態、`aria-hidden`、`inert` の更新処理を列挙する。新 mode と旧コードが別々に active panel を決める構成は避け、各サポート経路で一つの仕組みに担当を寄せる。ライブラリ全体を無条件に削除するのではなく、フォームや更新通知など残る責務を確かめる
6. **見える丸印だけで受け入れない**。marker の意味を理解できるラベル、現在位置とキーボード focus の区別、拡大時の操作領域を確認する。自動生成される role の存在だけで部品全体のアクセシビリティ適合を宣言しない

## 受入試験案

これは今後実行するためのチェックリストであり、この調査で通過した実機テスト一覧ではない。

| 条件 | 確認する観測点 |
| --- | --- |
| `links`、focusable な対象と非 focusable な見出しを混在 | marker の順次移動、起動後の開始位置、次の Tab、読み上げの行き先を別々に記録 |
| `tabs`、各 panel 内にリンクと入力欄 | group 内の矢印移動、active marker への再入場、次の Tab、inactive 内容の到達範囲 |
| 同時に複数カードが見える画面 | 見える操作と accessibility tree に存在する操作の差が用途上許容できるか |
| キーボード操作後にタッチやホイールでスクロール | 現在位置の表示、focus の所在、読み上げ対象が食い違わないか |
| active panel の削除、並べ替え、空一覧 | 入口が失われず残る内容・外側の操作へ到達できるか |
| 入れ子の scroll container、長い翻訳、狭い画面 | marker の所属、内容の読み順、欠けた操作や二重スクロールがないか |
| CSS enhancement 無効、mode 非対応、旧ライブラリ有効 | fallback で情報が消えないか、二重の marker・focus trap・二重選択更新がないか |
| 実際の browser / OS / screen reader の組合せ | role、ラベル、Tab、矢印、読上げ対象、focus indicator を操作記録とともに保存 |

非 focusable な見出しに対して `activeElement === target` を一律の期待値にしない。自動試験で `activeElement` だけを assert して終えず、目視の focus indicator とアクセシビリティ情報も確認する。検索 eval はこの資料を発見できることだけを検査する。

## 出典の版・ライセンス・未確認事項

- 取得日はすべて **2026-10-02 UTC**。release_notes の30日 TTL を含むため、明示期限を **2026-11-01** とする
- 現行契約の参照は CSSWG の **Editor Draft、4 August 2026**。W3C Recommendation ではなく、再確認時には本文と版表示の両方を読み直す。旧 FPWD と Chrome 135 記事は歴史的な比較のために取得した
- W3C 資料は [W3C Software and Document License 2023](https://www.w3.org/copyright/software-license-2023/)。Chrome の2ページはページ末尾に本文 CC-BY-4.0、コード例 Apache-2.0 と表示する。MDN 文書は Mozilla Contributors の [CC-BY-SA-2.5-or-later](https://developer.mozilla.org/en-US/docs/MDN/Writing_guidelines/Attrib_copyright_license)。本文は技術的事実の独自要約・比較と独自の設計判断であり、原文・図・コード例の移植はない
- release notes からリンクされた [ChromeStatus entry](https://chromestatus.com/feature/5109685301673984) は取得ツールで本文0行、[Chromium tracking issue](https://issues.chromium.org/issues/425931511) は取得エラーだった。rollout の詳細や実装変更点の根拠には使っていない。MDN の互換表も今回の本文取得にはデータが現れず、Firefox・Safari・WebView の対応版一覧を確立していない
- 各 OS の実ブラウザ、支援技術、Shadow DOM、印刷、動的更新、RTL・縦書きは未試験。URL fragment / history の細部、marker の名前計算、全キー割当て、inactive 内容内で既に focus を持つ場合の挙動もこの資料では保証しない

---
{
  "id": "javascript-resizeobserver-box-delivery-convergence-boundary",
  "title": "ResizeObserver: 観測box・配送延期・loop errorと収束の境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "Resize Observer Level 1 Editor's Draft displayed 2022-11-22; HTML Living Standard updated 2026-10-04; MDN updated 2025-11-07; web.dev updated 2023-12-12; verified 2026-10-05 UTC",
  "tags": [
    "research-domain:frontend",
    "ResizeObserver",
    "content-box",
    "border-box",
    "device-pixel-content-box",
    "loop-error",
    "requestAnimationFrame",
    "writing-mode",
    "layout",
    "convergence"
  ],
  "sources": [
    {
      "id": "csswg-resize-observer-box-delivery-20261005",
      "url": "https://drafts.csswg.org/resize-observer/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-resize-rendering-loop-20261005",
      "url": "https://html.spec.whatwg.org/multipage/webappapis.html",
      "type": "official_docs"
    },
    {
      "id": "mdn-resize-observer-errors-20261005",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/ResizeObserver",
      "type": "official_docs"
    },
    {
      "id": "mdn-resize-observer-size-axes-20261005",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/ResizeObserverSize",
      "type": "official_docs"
    },
    {
      "id": "webdev-resize-observer-delivery-performance-20261005",
      "url": "https://web.dev/articles/resize-observer",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2027-01-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# ResizeObserver の観測box・配送延期・収束の境界

## 問い・適用範囲

`ResizeObserver loop completed with undelivered notifications.` が出たら無限loopなのか、`requestAnimationFrame` に移せば修理完了なのか、`borderBoxSize` を読むだけでborderの変更も監視できるのか。callbackの回数だけでなく、何を測り、どの変更で通知され、書き換え後に寸法が収束するかを分ける。

対象はブラウザーの `ResizeObserver`。既存の [event loop と描画順序](event-loop-microtask-rendering-order.md) の一般的な順序説明を繰り返すのではなく、観測対象と再配送の判断を扱う。[Fragment refs](../react/fragment-ref-dom-boundaries.md) のobserver登録先や、[responsive iframe](../css/responsive-iframe-opt-in-resize-boundary.md) の文書間寸法公開とは別の問題である。

2026-10-05 UTC に [CSSWG Resize Observer Level 1 Editor's Draft][spec] を開いた。表示日は **2022-11-22** で、work in progressでありW3Cの最終勧告ではない。[HTML Living Standard][html] の更新表示は **2026-10-04**。この古くからあるAPIの未収録の実用上の穴を埋める調査であり、2026年の新機能とは扱わない。

## 観測boxとcallbackが返す寸法は同じ選択ではない

[CSSWG §2.1 / §2.3][spec] の契約:

- `observe(target)` の既定は `content-box`。`border-box` はpaddingとborderを含む領域、`device-pixel-content-box` はCSS transform適用前の内容領域を整数device pixelsで観測する
- `box` option が選ぶのは、どの寸法の変化を通知条件にするか。entryに返すbox寸法の種類を一つだけに絞るoptionではない
- 同じobserverで同じtargetへもう一度 `observe()` すると、以前の登録を外して新しい設定に置き換える。contentとborderの両方を独立して観測したい場合は複数observerを使う

ここからの具体的な設計判断として、既定のcontent観測で `entry.borderBoxSize` を読むだけではborder変更の監視へ切り替わらない。内容寸法が変わらずborderだけ変わる構成なら、必要なのは `{ box: "border-box" }` の選択である。一方、border-box一定でpaddingが変わりcontentが縮む構成では、border観測だけでcontent変化を拾えると決めつけない。

[MDN ResizeObserver][mdn] はcontentをpadding・borderを除く領域と説明する。CSSへの書き戻し側でも、測定したboxと `box-sizing` による寸法の意味を揃える。たとえばcontent幅に増分を足した値をborder-boxの `width` へ入れる設計は、padding・borderを二重に扱う危険がある。測定値と設定値が数値上等しいだけでは同じ領域を表していない。

### 論理軸とdevice pixels

[MDN ResizeObserverSize][size] によると `inlineSize` / `blockSize` はwriting modeの論理軸。横書きではinlineが幅、blockが高さだが、縦書きでは対応が入れ替わる。`inlineSize` を常に物理的な `width` として代入しない。contentかborderかは、値を取り出したentryのプロパティで区別する。

CSSWGはdevice pixel寸法の算出にブラウザー固有のsubpixel snappingがあるとし、CSS pixel寸法×`devicePixelRatio` の丸めだけでは正確に復元できないと説明する。[§2.1][spec] したがってcanvas等ではCSS上の表示領域とbacking storeの単位を分ける。device pixel値をCSSのpx値としてそのまま設定しない。代替としてDPRを掛ける経路を置くなら、正確な同値変換ではなく近似だと扱う。

## 配送loopが止まる条件とアプリが収束する条件

[HTML の update the rendering][html] では、rAFのcallbackを実行した後、style/layoutを計算し、activeなresize観測を集めて配送する。配送があればstyle/layout計算へ戻る。最後にskippedな観測が残ればresize loop errorを報告する。この処理はpaintの完了通知ではない。

[CSSWG §3.4.1 / §3.4.5][spec] は、現在のdepthより深いtargetを配送候補にし、それ以外の変化をskippedへ入れる。配送後のdepthは、その回で配送した最も浅いtargetのdepthになる。depth計算はflattened DOM treeを使う。

これから導ける注意点:

- 一つの描画更新の中でcallbackは複数回呼ばれ得る。「1フレーム1callback」という契約ではない
- 自分自身や既に配送した浅いancestorを再び変えても、その回にすべての変更を配送しきるとは限らない
- これはcallbackの登録順や10回・100回の固定回数制限による収束判定ではない。DOMの深さと変化した対象に依存する

[MDN の Observation Errors][mdn] は、条件を満たさない通知が次のpaintへ延期され、Windowにerror eventが出ると説明する。この保護はブラウザーのlockupを避けるもので、アプリの無限増加や振動を解決するものではない。逆に、一回のerrorだけでは無限loopと断定できない。数回の配送へ分かれて最終寸法が安定する場合もあるが、途中の崩れが複数フレームに見える可能性がある。

たとえば「観測するたびに幅を10px増やす」規則には停止条件がない。毎回の描画更新を終えられても、次の更新でまた幅を増やせる。errorが消えたかだけでなく、callback回数・書込み回数・最終寸法の安定を別々に観測する。

## rAFへの延期を修理完了と扱わない

MDNは意図的に継続するresizeを次の描画機会へ送る方法と、期待寸法を記録して同じ寸法なら再設定しない方法を分けて示す。[mdn] Googleの [web.dev記事][article] も、同じフレームに処理できない変更は次のフレームへ延期されること、callbackでの過剰なlayout作業が応答性を悪化させることを説明する。

以下は独自の設計案であり、特定ライブラリの検証済み修正コードではない。

1. まず依存関係を切る。親の寸法を読み子を変える場合も、子の変更が親のintrinsic sizeを変えて戻ってこないか確認する。観測対象と書込み対象を別要素にしただけで循環が消えるとは限らない
2. 外部の要件から目標寸法を求め、既に目標を満たすときは書き込まない。前回測定値に無条件の増分を足す方式を、rAFで包むだけにしない
3. 期待値は観測box・CSSのbox・論理軸・単位を揃えて比較する。min/max制約、rounding、scrollbarの出現で設定値と実測値が一致しない場合を用意する。必要な許容差やhysteresisは製品の寸法精度に合わせて定義する
4. rAFを使うなら、複数entry・複数callbackから同じtargetへ古い書込みを積み上げず、保留する最新の目標値をまとめる。延期したentryの寸法は、その後も永遠に最新という保証ではない
5. observerを解除する際には、アプリが別に予約した書込みも無効化する。componentの世代や対象IDを確認し、解除済みの旧targetへの遅延書込みを防ぐ
6. error reportを丸ごと無視する前に、単発の延期か継続的な振動かを測る。他のJavaScript errorまで抑制して見えなくしない

rAFは次の描画機会に結びつく予定であり、一定ミリ秒後の実行を保証するタイマーではない。[HTML][html] はvisibilityや描画可能性などで描画更新を除外できる。バックグラウンドでも必ず1フレーム以内に完了する業務処理を、この通知だけへ依存させない。

## 測定範囲と寿命の落とし穴

- `contentRect` を画面上の位置・transform後の矩形として使わない。CSSWGのcontent rectは内容寸法とpadding起点のoffsetであり、CSS transformsだけの変更はresize観測を発火させない。[§3.3.1][spec]
- `display:none` への切替やDOMからの除去も観測対象の寸法へ影響する。callbackが来たことを可視化完了やユーザーが見た証拠にしない
- 非置換inline要素のcontent rectは空になる。inlineの文字列を測る意図で任意のspanへ登録し、block要素と同じ通知を期待しない
- box sizeが配列なのは、複数fragmentへの拡張を想定した形式。確認したLevel 1草案は先頭columnに対応する1件と記し、MDNもmulti-columnでは先頭columnの寸法と説明する。配列を受け取っただけで全columnの合計や全fragmentを得たと主張しない。[size]
- CSS layout boxを持たないSVG要素ではbounding boxの扱いになる。HTML boxのpaddingを含めた計算をSVGへ一律に適用しない。[§3.3.1][spec]

[CSSWGのlifetime節][spec] では、JavaScriptの参照がなく、かつ観測targetもないという両条件が揃うまでobserverが生存する。変数へ `null` を代入するだけを解除手順にしない。共有observerなら対象一つの `unobserve(target)`、そのobserverを使い終えるなら `disconnect()` を使う。独自設計として、共有observerの一利用者が全targetを消さないよう、登録と解除の所有者を明確にする。

## 導入判断・受入試験

見た目の条件分岐だけで済む要件ではCSS側で表現できるかを先に検討し、JSで寸法に応じて外部描画や処理を行う必要があるところへobserverを限定する。これは本稿の設計判断で、すべてをobserverへ移す勧告ではない。

以下は本番適用前の試験案。今回、ブラウザーruntime試験やWPTは実行していない。

1. 内容だけ、paddingだけ、borderだけを別々に変更する。boxごとに何がcallbackを発火させるか、返された別boxの値と観測条件を混同していないか確認する
2. horizontalとvertical writing modeで同じcomponentを使い、inline/blockの入替えと設定側のCSS propertyを確認する
3. fractional CSS pixels、DPRやzoomの変更、canvasのCSS寸法とbacking store寸法で単位・丸めを確認する。`device-pixel-content-box` の対応をAPI本体の存在だけで判断しない
4. 同じtargetを再登録し、box設定が置き換わることを確認する。二つのobserverを使う場合も最終書込み責任者を一つにする
5. 自己増加、親子相互依存、閾値をまたぐscrollbar出現を再現し、errorの有無と寸法の収束を別の期待値にする
6. 同期書込みとrAF延期を比較し、最終寸法、見える途中状態、callback数、INP等の応答性を計測する。error件数だけを合格条件にしない
7. mount / unmount、DOM除去、再挿入、`display:none`、transformのみの変更、非置換inline、multi-columnを負例として確認する
8. 解除直前に書込みを予約し、旧targetへ書かないことと、共有observerの他targetが観測を継続することを確認する

検索evalはこの記事の発見可能性だけを確かめ、ブラウザー互換性や上記の実行結果を保証しない。

## 出典の位置づけ・鮮度・未確認事項

- CSSWGは2022-11-22表示のEditor’s Draft。HTMLは2026-10-04更新のLiving Standard。どちらも2026-10-05に実ページを再確認し、取得日と仕様の表示日を分けた
- MDNのResizeObserver / ResizeObserverSizeは2025-11-07更新表示。Google web.dev記事は2023-12-12更新表示。記事の古い対応ブラウザー記述を、現時点の全subfeature互換表として採用していない
- 規範的な処理モデルはCSSWG / WHATWGを根拠にし、MDNの解説とGoogleの性能上の助言を補助に使った。Google記事を公式API契約と同一視しない
- 資料の一部には旧説明が残る。web.devの「contentRectだけを見る」という記述を全box optionに一般化せず、現在のCSSWGにある観測box選択を優先した。MDN Sizeのプロパティ説明にborder boxとある箇所も、entryのcontent / border / device-pixelの違いをなくす意味には採らない
- W3Cはリンク先のSoftware and Document License 2023、HTMLはCC-BY-4.0、MDN本文はCC-BY-SA-2.5、Google本文はCC-BY-4.0 / コード例Apache-2.0。原文・サンプルコードの転載を避け、独自の日本語要約と設計・試験案を記録した
- browser engineの固定commit解析、各ブラウザー最新版のsubfeature対応、Shadow DOMの複雑な相互作用、headlessでの描画機会、実機の性能は未確認。草案だけで実装の完全一致を主張しない
- 全sourceの取得日は2026-10-05 UTC。official_docs / maintainer_articleの90日TTLに合わせ、明示期限は2027-01-03

[spec]: https://drafts.csswg.org/resize-observer/
[html]: https://html.spec.whatwg.org/multipage/webappapis.html
[mdn]: https://developer.mozilla.org/en-US/docs/Web/API/ResizeObserver
[size]: https://developer.mozilla.org/en-US/docs/Web/API/ResizeObserverSize
[article]: https://web.dev/articles/resize-observer

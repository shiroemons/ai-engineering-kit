---
{
  "id": "css-scroll-anchoring-manual-correction-boundary",
  "title": "Safari 27のscroll anchoring: 手動補正・除外階層・suppressionの境界",
  "kind": "knowledge",
  "technology": "css",
  "version": "Safari 27.0 release article 2026-09-17; CSS Scroll Anchoring Level 1 Editor's Draft 2026-09-30; MDN overview modified 2025-11-07; verified 2026-10-23 UTC, browser runtime untested",
  "tags": [
    "research-domain:frontend",
    "scroll-anchoring",
    "overflow-anchor",
    "Safari27",
    "scrollTop",
    "suppression-window",
    "nested-scroll-container",
    "virtual-list",
    "prepend",
    "manual-correction"
  ],
  "sources": [
    {
      "id": "webkit-safari27-scroll-anchoring-release-20261023",
      "url": "https://webkit.org/blog/18325/webkit-features-for-safari-27-0/",
      "type": "release_notes"
    },
    {
      "id": "csswg-scroll-anchoring-selection-suppression-20261023",
      "url": "https://drafts.csswg.org/css-scroll-anchoring-1/",
      "type": "official_docs"
    },
    {
      "id": "mdn-scroll-anchoring-overview-20261023",
      "url": "https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Scroll_anchoring/Overview",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Safari 27のscroll anchoring: 手動補正・除外階層・suppressionの境界

## 問いと今回の変更

過去ログをリスト先頭へ追加する `prepend` 処理や遅延画像の到着後、アプリが `scrollTop` を補正している。Safari更新後もその処理を無条件に続けてよいか。判断の中心は「値が何px変わったか」ではなく、「同じ表示位置を誰が維持するか」である。

[WebKitのSafari 27.0出荷記事][release]は2026-09-17公開で、scroll anchoringの対応、既定の有効化、`overflow-anchor: none` によるopt-outを案内している。アプリが新しいAPIを呼ばなくても挙動が変わり得る更新として、既存のスクロール補正を見直す根拠になる。旧版の全対応状況やWKWebViewへの適用をこの記事だけで確定しない。

規範上の説明には、2026-10-23 UTCに開いた[CSS Scroll Anchoring Level 1][spec]の **Editor's Draft 2026-09-30** を用いた。これはwork in progressで、Safariが草案の細部をすべて実装したという出荷保証ではない。[MDN概要][mdn]の表示更新日は2025-11-07。以下では出荷情報、草案の処理モデル、独自の移行判断を分ける。

既存の[CSS containment](containment-content-visibility.md)は描画を省略する範囲、[ResizeObserver](../javascript/resizeobserver-box-delivery-convergence-boundary.md)は寸法通知の配送を扱う。本稿は、その変更を受けたスクロール位置の所有と補正を扱う。要素を別要素へ配置する[CSS anchor positioning](anchor-positioning-fallback-scope-boundaries.md)とも別機能である。

## 保持するのは絶対scrollTopではない

[草案§2][spec]は、scrolling box内で選んだanchor nodeの移動量に応じて、block方向のscroll offsetを調整するモデルを定める。対象nodeの見える位置が保たれていても、`scrollTop` の数値は変わり得る。反対に数値が不変でも、ユーザーが読んでいた行は動き得る。

ここからの試験設計として、通常の横書き・縦スクロールのリストでは、固定した行IDと、その行のscrollportに対する相対位置を観測点にする。たとえば先頭へ120px相当を追加した場合、「scrollTopが増えないこと」を成功条件にしない。読んでいた行が同じ位置に残ること、操作中のfocusが別項目へ移らないことを確認する。120pxは説明用の値で、ブラウザーの保証値ではない。

この仕組みを「常に末尾へ追従するchat機能」とみなさない。本稿では、読書位置を保つ状態と、新着を追う状態をアプリの別状態として管理することを勧める。上を読み返している利用者を、新着のたびに最下部へ戻す設計とは分ける。

## overflow-anchorの除外単位を間違えない

[草案§3][spec]では初期値が `auto`、非継承で、`none` は候補からの除外を表す。重要なのは次の二つを分けることである。

- 同じscrolling boxについて、除外したsubtree内の普通の子へ `auto` を置いても、祖先による除外を解除しない
- 独立したnested scroll containerは、自身のscrolling boxについて原則として別に有効化される。そのcontainerにも `none` を指定すれば、その内部の自動補正を無効にできる

後者は草案の注記に明示されている。[MDN概要][mdn]の「除外領域の子で再有効化できない」という短い説明を、入れ子のscrolling boxまで永久に無効と読むのは広すぎる。外側の補正対象になることと、内側が自分の内容を補正することは別である。ただし、この草案上の区別を全ブラウザーで実測したわけではない。

独自の設計判断として、次のように対象を選ぶ。

1. 広告など不安定な部分をanchor候補にしたくないだけなら、その部分を除外する。別の安定した候補によるページ全体の補正は残す
2. 仮想リストが自分で位置維持を所有するなら、そのリストの実際のscroll containerへ `none` を指定する。挿入する行だけを除外しても、既存行による自動補正は止められない
3. ページと内部のログpaneを同じ一つのスクローラーとして扱わない。どの要素のscroll offsetを書いているか、内部paneにも補正処理があるかを棚卸しする
4. 原因調査のための全要素一括opt-outを、そのまま恒久対策にしない。無関係な本文領域の位置安定まで失う可能性を別に評価する

「CSSでは継承されないのだから子のautoで戻る」という推論も、「bodyでnoneにしたからすべての入れ子paneが止まる」という推論も避ける。判定の単位は、単なるCSS継承ではなく、対象scrolling boxの候補探索である。

## suppressionはサイズ変更全般の禁止ではない

[草案§2.2][spec]は、補正をsuppression window内でまとめ、指定条件があれば抑止するモデルを示す。anchorからscrollable elementまでの経路上で、寸法、余白、position、transformなどの **computed value** が変わる場合は、その条件に含まれる。経路外でも、absolute positioningへ入る／外れるposition変更には別の条件がある。

この区別から、次を診断仮説として使える。

- 「画像が読み込まれてlayoutが変わった」と「anchorの祖先のheight宣言を変更した」を同じfixtureに混ぜない。前者だけの試験と後者も含む試験を比較する
- anchor候補が存在するケースと、scroll originにいるケースを分ける。初期位置だけで試して補正が起きないと判断しない
- virtual listの高さ調整や位置移動を外したときだけ症状が消えるなら、単に処理が速くなったのか、suppression条件が変わったのかを切り分ける

これは特定のcomputed style変更で全ブラウザーが同じ結果を返したという報告ではない。また、すべてのlayout変化をsuppressionと断定しない。画像や先行コンテンツの変化によるずれの軽減が、そもそもの機能の目的である。

草案はscroll位置に依存する操作の直前にwindowが終わる場合を挙げ、`getBoundingClientRect()` を例示する。したがって独自の計測でも、各DOM書込みの間に座標読取りを増やした版と増やさない版で条件が同じとは限らない。測定用コードを足して症状が消えても、直ったとは決めない。任意のsleepや「rAF一回で必ず完了」を公式の同期契約として使わない。

## 手動補正は一つの所有者に集める

以下は公式APIの使い方の規定ではなく、上述の契約を踏まえたアプリ設計案である。

### 自動補正に任せる領域

記事本文や単純な一覧では、まずnative anchoringを維持し、無条件の高さ差加算を外した構成を試す。画像等の領域予約も別に続ける。すべてのずれをブラウザーに修復させる前提にはしない。

DOM更新後の現在のscrollTopに追加高さを加算する処理は、その前にnative補正が済んでいれば同じ移動を二回数える可能性がある。一方、書込み時点やsuppressionによって結果は変わるため、「常に二倍ずれる」とは主張しない。native有効／無効と手動補正有効／無効を独立して切り替え、どちらが位置を書き換えたかを比較する。

### アプリが所有する領域

virtual listやページ単位の履歴復元など、アプリが基準を決める場合は、そのscroll containerのnative補正を明示的に無効化する案を検討する。状態管理には単なる全体高ではなく、基準行ID、基準位置、更新世代を使う。

- 更新前にユーザーが見ていた行と位置を記録し、commit後に同じ行が存在するか確認する
- 下側への追加、画像の遅延到着、行削除が同時に起こるなら、scrollHeight差をすべて「先頭追加分」とみなさない
- 計測後に別の操作や新しい更新が入った場合、古い世代の補正を適用しない。ユーザーの移動を古い位置へ巻き戻さない
- 基準行が削除された場合の代替行や末尾追従は、UIの要件として明示する。任意の最初の行を暗黙の代替にしない

実際の補正式はwriting mode、逆向きレイアウト、scroll範囲の端、同時更新の設計による。ここでは汎用の未検証コードとして提供しない。scrollHeight差分式を別の式へ置き換えるだけで、更新競合や基準消失まで解決したとはしない。

### scroll eventを入力の証拠にしない

[草案§2.2][spec]ではanchoringの補正もscroll eventを発生させる。[MDN概要][mdn]も、scroll listenerが追加の補正を想定していない場合の問題を説明する。

このため、「scroll eventが来たので利用者が過去ログを読み始めた」とだけ判定して追従状態を切り替えない。アプリが行った更新、位置保持、ユーザーのナビゲーション意図を別に扱う。同じlistenerから常に高さ変更とscroll書込みを行う構成は、フィードバックの有無を調べる。イベント回数をそのまま利用者操作数やエンゲージメント数にしない。

## 回帰確認の判定点

以下は未実行の独自テスト計画で、WebKitの公式テストや実機結果ではない。

1. **先頭追加**: 一覧途中の固定行を基準に、上側だけへ内容を追加する。nativeのみ、手動のみ、両方、両方なしの4構成で、行の相対位置とoffsetを別々に記録する
2. **除外階層**: 普通の子のauto、入れ子scroll containerのauto、入れ子scroll containerにもnoneを置く3構成を比較する。外側と内側のoffsetを同時に記録する
3. **suppression**: 先行行の挿入だけの場合と、同じ更新で基準行の祖先のcomputed heightを変える場合を分ける。テスト中に不要なstyle変更を混ぜない
4. **ユーザー意図**: 更新待機中に手動スクロールしてから完了させる。遅れて到着した補正で旧位置へ戻されないこと、過去ログ閲覧中に新着で末尾へ飛ばないことを確認する
5. **候補消失と遅延資源**: 基準行削除、画像到着、文字サイズ変更を個別に発生させる。行IDが残っている場合と失われた場合を同じ成功条件にしない
6. **イベント依存**: 手動入力なしのDOM更新でもscroll eventが来る構成を含め、analyticsや追従状態が誤更新されないことを確認する
7. **配布条件**: Safari 27.0と実際の旧対応版、利用者が使う他エンジンで実行する。CSSの構文検出が成功しただけで細部の一致や回帰不在を判定しない

単に「何px以下なら成功」という閾値だけでなく、基準にした行、観測時点、入れ子のどのoffsetか、手動補正の実行回数をテスト出力へ残す。失敗後に全画面opt-outで隠すより、位置維持の所有者を特定する材料になる。

## 出典・ライセンス・限界

- WebKit出荷記事とライセンス案内を開いた。[Licensing WebKit][webkit-license]はソフトウェアのLGPL/BSDを説明するが、記事本文の利用許諾まで確認できないためcatalogはunknown。文章・画像・サンプルコードは転載せず、短い事実の独自要約だけを用いた
- CSSWG草案の表紙日付、§2〜3、リンクされた[W3C Software and Document License 2023][w3c-license]を確認した。最終勧告や特定ブラウザーの実装証拠とは扱っていない。アルゴリズム全文や仕様コードは転載しない
- MDN概要と[利用許諾ページ][mdn-license]を確認した。文書はCC-BY-SA-2.5以降。診断上の説明を参照し、文章やサンプルをコピーしていない。概要の短い除外説明は、scrolling boxの区別を省いて一般化しない
- ブラウザー実機、WPT、WebKit実装commit、Safari patch版ごとの修正、WKWebView、shadow DOM、scroll snap、縦書き／逆向きレイアウトの組合せは未検証。release articleと草案の両方を読んだことは、これらの相互運用性の証明ではない
- Safari release notesへのApple Developerリンクは本文が取得できず、細部を根拠として追加していない。出荷根拠は実際に取得できたWebKit記事に限定する
- retrieved_atは2026-10-23 UTC、expires_atはrelease_notesの30日TTLに合わせ2026-11-22。再確認時はSafariの出荷情報と、草案の除外階層・suppression条件・実際の対象環境を見直す

[release]: https://webkit.org/blog/18325/webkit-features-for-safari-27-0/#scroll-anchoring
[spec]: https://drafts.csswg.org/css-scroll-anchoring-1/
[mdn]: https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Scroll_anchoring/Overview
[webkit-license]: https://webkit.org/licensing-webkit/
[w3c-license]: https://www.w3.org/copyright/software-license-2023/
[mdn-license]: https://developer.mozilla.org/en-US/docs/MDN/Writing_guidelines/Attrib_copyright_license

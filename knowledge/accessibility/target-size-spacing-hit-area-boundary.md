---
{
  "id": "accessibility-target-size-spacing-hit-area-boundary",
  "title": "WCAG 2.2 target size: 実クリック領域・spacing円・例外の判定境界",
  "kind": "knowledge",
  "technology": "accessibility",
  "version": "WCAG 2.2 Recommendation 2024-12-12; Understanding 2.5.8 updated 2026-05-11 / 2.5.5 updated 2026-09-06; C42 updated 2025-12-02; retrieved 2026-10-03 UTC; browser runtime untested",
  "tags": [
    "research-domain:frontend",
    "wcag22",
    "target-size",
    "pointer-input",
    "spacing",
    "hit-area",
    "bounding-box",
    "inline",
    "css-pixel"
  ],
  "sources": [
    {
      "id": "wcag22-target-size-rec-20241212-20261003",
      "url": "https://www.w3.org/TR/2024/REC-WCAG22-20241212/",
      "type": "official_docs"
    },
    {
      "id": "wcag22-target-size-understanding-20261003",
      "url": "https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html",
      "type": "official_docs"
    },
    {
      "id": "wcag22-target-spacing-c42-20261003",
      "url": "https://www.w3.org/WAI/WCAG22/Techniques/css/C42.html",
      "type": "official_docs"
    },
    {
      "id": "wcag22-target-enhanced-comparison-20261003",
      "url": "https://www.w3.org/WAI/WCAG22/Understanding/target-size-enhanced.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# WCAG 2.2 target size: 実クリック領域・spacing円・例外の判定境界

## 問いと結論

密なツールバーや一覧の操作ボタンで、「CSS の幅・高さが24px」「周囲に4pxの余白」「外側のコンテナが24px」のいずれかを確認すれば、SC 2.5.8 を満たすのか。

**描画されたアイコン、実際のクリック領域、spacing判定用の仮想円を分ける。** サイズで満たすか、個別の例外で満たすかを操作対象ごとに記録する。小さなtarget同士だけでなく、大きなtargetの縁もspacing判定の相手になる。

これは未収録だったポインター操作の耐久的な設計判断を補う調査。2026年にSCが新設されたという報告ではない。既存のkeyboard focus文書が扱うフォーカス順序・視認性と、今回の対象サイズは別に確認する。

## 規範と補足資料を分ける

[固定版 WCAG 2.2 Recommendation 2024-12-12](https://www.w3.org/TR/2024/REC-WCAG22-20241212/#target-size-minimum) の **SC 2.5.8はLevel AA**。ポインター入力のtargetを原則24×24 CSS pixels以上とし、次の例外を認める。

- Spacing: 小targetのbounding box中心に置く直径24 CSS pxの円が、他targetまたは他の小targetの判定円と交差しない
- Equivalent: 同一ページ内に、このSCを満たし同じ機能を実現する別コントロールがある
- Inline: 文中にある、または非targetテキストのline-heightでサイズが制約される
- User Agent Control: UAがサイズを決め、作者が変更していない
- Essential: 特定の表示が情報に不可欠、または法的に要求される

規範のtargetはポインター操作を受け付ける表示領域である。別の動作を行うtargetと重なった部分は、サイズへ二重計上しない。同じ動作・同じページを開く場合には注記の例外がある。単位はCSS pixelであり、スクリーンショットの物理pixel数をそのまま代入しない。

[Understanding 2.5.8](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html) と [Technique C42](https://www.w3.org/WAI/WCAG22/Techniques/css/C42.html) は**informativeな説明・達成手法**。適合の規範はSC本文であり、特定のCSS例の採用は必須ではない。以下では公式の解釈と、そこから作る独自の計算・実装提案を区別する。

## サイズを満たすことと、spacingで満たすこと

### bounding boxだけではサイズ要件の証明にならない

Understandingの「Size requirement」は、実targetの内部に**ページの水平・垂直軸に沿う24×24の正方形が完全に収まるか**で説明する。直径24pxの円形targetは24×24のbounding boxを持つが、その正方形を内包しないためundersizedとなる。ただしspacing例外で満たす余地はある。

spacing判定では、非矩形targetもbounding boxの中心に直径24pxの円を置く。凹形状では、その中心が実targetの外にあっても同じ手順となる。サイズ判定の「内側の正方形」と、spacing判定の「外へはみ出し得る円」を交換しない。ページzoomで利用者に拡大させることも、この要件を満たす根拠にはならない。[Understanding 2.5.8](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html)

設計上は、DOM要素の寸法を候補抽出に使い、形状や重なりがある候補では実クリック領域を別途確認する。見た目が円形でも、実targetが矩形ならその矩形を扱う。逆に、見た目だけ大きい装飾を有効な操作領域へ足さない。これはtarget定義から導くレビュー方針であり、特定ブラウザーのhit testingを測定した結果ではない。

### C42はコンテナを押せる領域へ変える手法ではない

C42は、小さなページ送りリンクを少なくとも24×24のcontainer内に中心配置して、判定円のための余白を確保する手法を示す。containerの面積そのものをリンクのtargetとして数える説明ではない。同手法のテストは、判定円と「他のtarget本体」「他の小targetの円」の両方を調べる。[C42](https://www.w3.org/WAI/WCAG22/Techniques/css/C42.html)

独自の実装方針として、外側のliへmin-widthを付けただけで完了にせず、最終的な配置・折返し・隣接操作を測る。実クリック領域を拡大する設計を選ぶなら、押下で同じ操作が成立する範囲を確かめる。余白の確保とhit areaの拡張は、受け入れ条件を分けて管理する。

## 独自の幾何計算: 一律のgapを決めない

以下はSCのspacing条件とC42の二種類の交差検査から導いた**理想図形の計算**。単位はCSS px。矩形は軸平行、同じ高さの中心線上に配置され、他target・clip・transform・重なりはないと仮定する。他の例外は使わない。

- 同じ16×16の小targetを横に並べ、縁間のgapをgとする。中心距離は16+g。半径12の判定円二つに内部の重なりがない境界は中心距離24なので、g=8が境界となる。g=4では中心距離20となり不足する
- 16×16の小targetの右に十分大きな矩形targetがある場合、小target中心から大targetの左端までの距離は8+g。大targetには小target用の仮想円を置かず、実領域との交差を調べる。半径12の円が入り込まない境界はg=4。g=3では1px入り込む
- 24px幅のcontainerを二つ接して置いても、左container内の16px targetを右寄せ、右container内の16px targetを左寄せすると中心距離16になる。containerの宣言サイズが揃っていることは、中心配置やspacing達成の証明ではない
- 実targetが真円なら、24×24正方形を内包する直径は24√2、約33.94以上となる。この数値は上の内包条件の幾何計算であり、「すべての丸いUIは34px必須」という別の規範ではない。spacing例外や実targetが矩形である場合を除外していない

境界値での接触と内部の重なりは分ける。Understandingの20px角＋4px gap、および16px角と大targetの4px gapの合格例を参照し、計算では接するだけの境界を許容した。本番レイアウトでは小数座標や縮小を考慮し、ちょうどの値への依存を避ける余裕を設計側で決める。余裕の量をWCAGが一律に指定しているとは扱わない。

この計算から、全ボタン共通の「4px gapならAA合格」も「必ず8px gapが必要」も導けない。左右だけでなく、上下の行や重なって表示される別操作も判定対象として列挙する。

## 例外と達成レベルを誤用しない

次は規範の条件を実装レビューへ落とす独自の問いである。

- Inline: CSSがdisplay:inlineかどうかだけで判定せず、文中または非target文字のline-heightによる制約を説明できるか。ナビゲーションを横一列にしただけで自動免除しない
- Equivalent: 代替操作の所在、実現する機能、そのSCの合格根拠を示せるか。「キーボードなら操作できる」だけで、同一ページの適合する別コントロールという条件を置き換えない
- User Agent Control: native要素という名前だけで免除せず、そのサイズを作者が変更したかを調べる
- Essential: 密度を下げたくないという好みと、情報を伝えるために必要な位置・表示を分け、理由を記録する

[Understanding 2.5.5](https://www.w3.org/WAI/WCAG22/Understanding/target-size-enhanced.html) が説明する **Target Size (Enhanced)はLevel AAAで44×44**。こちらにはspacing例外がないため、24px基準の余白による合格を44px要件へ流用しない。同ページは頻繁に使う操作や取り消しにくい操作で、さらに大きなtargetを検討するよう助言している。

そこで本資料の設計提案は、重要操作を最小値ぎりぎりに揃えるより、対象ユーザー・利用頻度・誤操作時の損失に応じて操作領域を広げること。これは一項目の最低条件を超える製品判断であり、44pxボタンを置けばページ全体がAAAになるという意味ではない。

## 検証結果と製品で残る確認

2026-10-03 UTC、独自に作成した標準ライブラリのみの幾何fixtureで次の8条件を計算し、期待値との一致を確認した。外部コードは取得・実行していない。

| 理想図形の条件 | 確認した期待値 |
|---|---|
| 実領域24×24の矩形 | 内包する正方形のサイズ条件を満たす |
| 実領域が直径24の円 | 正方形の内包条件を満たさない |
| 実領域が直径34の円 | 正方形の内包条件を満たす |
| 16px角二つ、gap=7 | spacing円の内部が重なる |
| 16px角二つ、gap=8 | spacing円が接する境界で内部は重ならない |
| 16px角と大矩形、gap=3 | spacing円が大矩形内へ入り込む |
| 16px角と大矩形、gap=4 | spacing円が大矩形に接する境界 |
| 24px container二つで小targetを互いの側へ寄せる | 中心距離16でspacing円が重なる |

これは**数値計算の確認だけ**であり、ブラウザーの座標取得、CSS適用、イベント配送、WCAG適合の自動判定器を実装・検証したものではない。

製品への適用時には、次の未実行のテスト計画を使う。

1. 状態と操作を棚卸しする。通常表示に加えて、メニュー展開・並替え・選択中・エラー表示で現れる小操作を記録する
2. 各targetへ、サイズで達成するか、どの例外に依存するかを一つずつ付ける。例外の根拠がなければ寸法・配置を見直す
3. 実際に狙った位置から操作し、意図した機能が発動する範囲を確認する。重なった削除アイコンと行全体のリンクなど、別機能の領域を分離する
4. viewport変更、文字拡大、翻訳によるラベル長の変化、折返しの後も再測定する。これは製品の回帰チェックであり、特定のzoom倍率を本SCが指定するという主張ではない
5. keyboard focus、名前・役割、誤操作からの回復は別に確認し、target sizeの結果で代用しない

検索evalには「正方形とbounding box」「小target同士と大target」「containerの中心配置」「例外とAA/AAA」の判断を呼び出せる8ケースを追加した。検索で文書が見つかること、幾何fixtureが通ること、実際のUIが適合することは異なる証拠である。

## 版・出典・未確認事項

- 規範は固定URLのWCAG 2.2 Recommendation **2024-12-12**
- Understanding 2.5.8の表示更新日は **2026-05-11**、C42は **2025-12-02**、Understanding 2.5.5は **2026-09-06**
- いずれも **2026-10-03 UTC** に本文を開いて確認した。表示更新日は個々の説明が導入された日という主張ではない
- C42の拡張子なしURLは初回取得でHTTP 429となり、同じW3Cの `.html` URLで本文を取得できた。取得に失敗したGitHubの変更commitは根拠へ採用していない
- 規範のライセンスは [W3C Document License 2023](https://www.w3.org/copyright/document-license-2023/)、WAI補足3資料のフッターは [W3C Software and Document License 2023](https://www.w3.org/copyright/software-license-2023/) を示す。本文は原文・コードの転載ではなく事実の独自日本語要約と分析であり、公式訳ではない
- 複雑なSVGの実hit area、各ブラウザーの丸角・clip・transform処理、支援技術とタッチ補正の挙動は未検証。実サービスの適合性、法的義務、全SCの充足も判定していない

再確認期限はofficial_docsの90日TTLに合わせ **2027-01-01**。その際は規範の例外、Supporting documentsのサイズ解釈・C42の手順、製品の操作領域を読み直す。取得日のみを延ばさない。

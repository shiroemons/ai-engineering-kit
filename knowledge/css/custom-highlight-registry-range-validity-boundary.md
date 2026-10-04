---
{
  "id": "css-custom-highlight-registry-range-validity-boundary",
  "title": "CSS Custom Highlight: 重なり順・Range共有・更新後の本文一致の境界",
  "kind": "knowledge",
  "technology": "css",
  "version": "Safari Technology Preview 253 (2026-09-23); CSS Highlight Level 1 Editor’s Draft 2026-09-10; DOM/Web IDL/Infra Living Standards retrieved 2026-10-04 UTC; browser fixture not executed",
  "tags": [
    "research-domain:frontend",
    "CSS.highlights",
    "Highlight",
    "HighlightRegistry",
    "StaticRange",
    "Range",
    "priority",
    "registration-order",
    "same-range",
    "document-revision",
    "accessibility",
    "Safari-TP253"
  ],
  "sources": [
    {
      "id": "webkit-tp253-highlight-fixes-20261004",
      "url": "https://webkit.org/blog/18357/release-notes-for-safari-technology-preview-253/",
      "type": "release_notes"
    },
    {
      "id": "csswg-custom-highlight-contract-20261004",
      "url": "https://drafts.csswg.org/css-highlight-api-1/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-dom-highlight-range-20261004",
      "url": "https://dom.spec.whatwg.org/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-webidl-highlight-maplike-20261004",
      "url": "https://webidl.spec.whatwg.org/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-infra-highlight-order-20261004",
      "url": "https://infra.spec.whatwg.org/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# CSS Custom Highlight の重なりと範囲の整合性

## 問いと今回の変更

検索結果・現在選択中の結果・レビュー注釈を同じ本文に重ねる UI で、強調色が見えることを「正しい箇所を指している」と判断してよいか。本文が再描画される場合、highlight の登録順と Range の追従だけでは不十分である。

[Safari Technology Preview 253 の release notes](https://webkit.org/blog/18357/release-notes-for-safari-technology-preview-253/)（2026-09-23）は、同じ Range を共有する第二の custom highlight が描画されない問題（185173794）と、wrapper の GC 後に CSS.highlights が登録順でなく hash 順に列挙される問題（185754041）の修正を記録する。これは TP253 の変更根拠であり、Safari stable の修正版や、影響する全旧版を示すものではない。

本書では、この変更を契機に未収録だった custom highlight の範囲管理と重なり順を扱う。CSS の anchor positioning や scroll-marker の focus 制御とは別の API である。RAG の source 座標・引用評価は[引用の保存と評価](../rag/citation-evidence-coordinate-evaluation.md)に委ね、ここでは既にある本文をブラウザー DOM のどこへ描画するかに絞る。

## 確認した仕様契約

以下は取得した一次資料の要約であり、後半の設計案とは区別する。

### Highlight と登録名・描画順

[CSS Highlight Level 1](https://drafts.csswg.org/css-highlight-api-1/) は作業中の Editor’s Draft である。取得本文の §3–5 は次を定める。

- Highlight は AbstractRange の集合で、CSS.highlights は document に対応する名前付き registry。登録と ::highlight(name) のスタイルを組み合わせる。DOM wrapper は追加されない
- 一つの Highlight 内の重複範囲は和集合として描画される。別々の Highlight の重なりは priority が大きい方を上にし、同値なら registry の挿入順で後のものを上にする。built-in highlight の overlay は custom より上にある
- 同じ Highlight を複数名で登録すると名前ごとに別スタイルで描画される。同じ Range を二つの Highlight が共有することとは異なる
- registry・範囲・priority の変更による repaint は非同期。無効な StaticRange や、境界 node が対象 document に属さない範囲は描画で無視する

したがって、スタイルシートの記述順だけで別 highlight 間の上下関係を説明したり、set が返った瞬間を描画完了としたりしない。

### 上書き set と再登録を分ける

[Web IDL の maplike set](https://webidl.spec.whatwg.org/#es-map-set) は [Infra の ordered map](https://infra.spec.whatwg.org/#ordered-map) の値設定を使う。既存 key があれば値だけを更新し、新規 key なら末尾へ追加する。delete は entry を取り除く。この組合せから、同じ名前の上書き set は順序を更新しないが、delete→set は末尾に移すと導ける。

例として、独立した Highlight A と B を first→second の順に登録し、priority を同値にする。first の値を別の Highlight へ上書きしても、keys の順序は first, second のまま。first を削除してから登録すると second, first になる。「最後に set を呼んだものが常に上」という契約ではない。これは標準の処理から導いた例で、今回のブラウザー実測結果ではない。

### StaticRange の valid は本文一致の検査ではない

[DOM §5](https://dom.spec.whatwg.org/#interface-staticrange) では、Range は DOM 変更に応じて境界を更新する live range、StaticRange は境界を自動更新しない範囲である。StaticRange の valid 条件は、両端が同じ node tree 内にあり、各 offset が node の長さ以内で、開始が終了以前にあること。検索語や元の文字列との一致は条件に含まれない。

[DOM §4.10 の replace data](https://dom.spec.whatwg.org/#concept-cd-replace) と data setter を併せると、次の反例を導ける。

1. 同じ Text node の内容が cat dog、対象が offset [0,3) であるとする
2. node.data に owl dog を代入して全文を置き換える
3. StaticRange の [0,3) は構造上 valid なまま、現在の文字は owl になる
4. 同じ初期範囲の live Range は、この全文置換では両端が0となり collapsed になる

「静的範囲が無効ならブラウザーが描画しない」と「意味的に古い範囲も自動で消える」は違う。また、live Range の自動更新を、検索語や annotation の同一性の維持とみなせない。これは上記の操作に対する仕様上の導出であり、全ての DOM 更新が collapse するという主張ではない。

### type の意味付けと操作 UI を分ける

[CSS Highlight §4.2.6](https://drafts.csswg.org/css-highlight-api-1/) の type は highlight・spelling-error・grammar-error。UA は可能な範囲で意味を accessibility API に公開することが推奨される。したがって「custom highlight は支援技術へ一切公開されない」とは扱わない。

一方、検索結果の件数表示、次の結果への操作、注釈を開くボタンはこの type だけでは定義しきれない。後述する代替 UI は本書の設計提案であり、全スクリーンリーダーの実装結果を述べたものではない。

## 独自の設計案: 検索状態から描画を再構成する

### 1. 描画用オブジェクトを検索結果の正本にしない

アプリの結果データに document ID、document revision、query generation、match ID と該当本文の位置を保持する。Range オブジェクトそのものを永続的な結果 ID としない。DOM の再利用・差し替え・仮想化で node と業務上の文章の対応が変わるためである。

worker 等で検索する場合は、開始時の revision と query generation を返却結果に含める。現在値と違う応答は採用しない。abort が成功したかどうかに依存せず、遅れて届いた結果の登録をこの照合で防ぐ。更新中に古い強調を残す場合も「更新待ち」と表示し、確定済みの結果として操作させない方針を決める。

### 2. 本文一致・DOM対応・装飾を順に検査する

表示対象の revision を確定し、検索結果を現行 Text node 群に対応付け、開始・終了と対象文字列を確認してから登録する。UTF-16 offset と外部の文字位置の単位差、Unicode 正規化、空白の折り畳み、複数 Text node をまたぐ一致を変換層の契約に含める。絵文字や結合文字の途中に表示境界を置かない検査も別途設ける。

単に new StaticRange が例外を投げなかったことや、CSS.highlights.has(name) が true であることを採用条件としない。重要なのは、利用者が今見ている文書の同じ箇所に一致を再確認できること。node が接続されていても、別の文章に再利用された可能性を revision と内容で排除する。

### 3. Range の方式を更新の所有者で選ぶ

- アプリが文書更新を一元管理し、描画単位で範囲を再生成するなら StaticRange を候補にする。文書更新時に古い集合を破棄する責任も同じ層に置く
- DOM 編集に追従させたい場合は live Range を候補にするが、追従後も意味的な一致を再検証する。画面上に位置が残るだけでは十分でない
- 外部コードも DOM を書き換える構成では、revision の更新漏れを避ける仕組みを決める。MutationObserver の通知だけを、全てのモデル状態が確定したという合図にしない

これは選択方針の提案であり、StaticRange の方が常に高速という測定結果ではない。大量範囲・頻繁な更新・仮想化を含め、実際の対象文書で計測する。

### 4. 描画優先度と名前を所有する

例えば検索一致に priority=10、現在の結果に priority=20 を割り当て、別々の Highlight にする。数値はアプリの規約であり標準値ではない。CSS の詳細度や偶然の登録時刻で上下関係を制御しない。一つの Highlight に多数の Range を入れた場合、個々の一致に別 priority を持たせる設計にはならない。

同じ Range を共有すると、その Range の変更も共有される。二つの layer が独立に境界を編集したいなら、独立した範囲を生成する。逆に「通常の検索一致」と「現在の一致」を同一座標へ追従させたい場合は、共有の意図を明記する。TP253 の修正があるからといって共有設計を無条件に禁止する必要はない。

registry はコンポーネント専用ではないため、画面・機能ごとの名前を予約する。他機能が使う CSS.highlights.clear() を cleanup に使わず、自分の名前だけを解除する。非同期の古い cleanup が新しい登録を消さないよう、CSS.highlights.get(name) が自分の登録した Highlight と同一である場合だけ delete する方法を検討する。

### 5. 装飾がなくても結果を利用できるようにする

検索件数、現在位置、次／前の結果へ進む操作、必要な注釈本文を通常の HTML UI でも提供する。色の上下関係だけで annotation の種類や現在選択を伝えない。検索一致を spelling-error と偽って通知させる使い方を避ける。

API 存在確認は開始条件に過ぎない。同一 Range の複数 layer、本文更新、selection との重なりが必要なら、それぞれ対象ブラウザーで確認する。任意の pointer 用 API の有無からキーボード操作の完成を推測しない。Safari TP の修正確認を理由に、利用者の stable Safari や組込み WebView の fallback を直ちに削除しない。

## 受入れ試験案と観測する証拠

以下は未実測の試験案。アプリの結果データ、registry の構造、画面の見た目、支援技術の操作を分けて判定する。

| ケース | 確認する証拠 |
|---|---|
| 同じ Range を二つの Highlight が共有 | 両方の set に範囲があり、異なる priority のスタイルが所期の順に重なる。仕様・TP修正記事だけで合格にしない |
| 同値 priority で first→second→first上書き | keys は first, second。delete→set の場合のみ second, first。registry順序と見た目を別に記録 |
| 静的範囲の cat→owl 置換 | valid な座標のまま本文が違うことを検出し、検索結果として再採用しない |
| live Range と全文置換 | [0,3) が [0,0) に変化する上記例を確認。別の挿入・分割操作はそれぞれ独立した期待値にする |
| query A の遅い応答が query B 後に到着 | query generation と document revision が不一致なら登録されない |
| 仮想化で node を再利用・文書を切替 | 古い match ID が新しい本文へ装飾されず、新 revision に対して範囲を作り直す |
| 古い所有者の cleanup と登録更新が競合 | identity guard が新所有者の Highlight を保持し、他機能の名前も残る |
| native selection・色変更・強調なし | 文字が読め、通常の結果一覧から同じ操作を完了できる |
| スクリーンリーダーとキーボード | type の実際の公開状況と、件数・現在位置・移動操作を別々に確認する |

GC 後の列挙順は Safari TP253 の記事で修正を確認したのみ。強制 GC 手段を前提に通常の Web API 試験へ再現性があると扱わず、長時間利用やメモリー圧を含む再現環境を別途用意する。

## 版・検証限界・provenance

- UTC 取得日はいずれも2026-10-04。CSS Highlight は Editor’s Draft 表示2026-09-10、DOM は Last Updated 2026-10-04、Web IDL は同2026-10-02、Infra は同2026-07-17。これらの更新表示を該当機能の導入日とは解釈しない
- TP253 記事は2026-09-23公開。release_notes の30日 TTL に合わせ expires_at は2026-11-03。stable Safari への backport・出荷版、Firefox/WebViewを含む互換表、全 API の Baseline は未確認
- この調査では Chromium 154.0.8037.57（Debian）を検出し、独自 HTML fixture の headless 起動を試したが、process_singleton の socket() が Operation not permitted となり起動できなかった。したがって **ブラウザー fixture は未実行**。Range/registry の期待値は仕様上の導出であり、描画・GC・accessibility の実測はない
- TP 記事がリンクする二つの WebKit commit ページは取得エラーだった。コード差分の解析・commit 固定・修正の原因推定は行っていない。記事に明記された変更だけを根拠とする
- CSSWG 文書の著作権者は W3C、[Software and Document License 2023](https://www.w3.org/copyright/software-license-2023/) を確認。DOM/Web IDL/Infra の本文は各ページの Intellectual property rights にある WHATWG の CC-BY-4.0、source code へ取り込む部分には BSD-3-Clause の表示を確認した
- WebKit 記事は再利用ライセンスを特定できず unknown。[Licensing WebKit](https://webkit.org/licensing-webkit/) はソフトウェアの LGPL/BSD を説明しており、記事本文へ一般化しない。本書は事実の日本語による独自要約と明示した設計・試験案であり、出典のコード・図・長文は転載していない。module 昇格は行わない
- 本書の検索 eval は発見可能性の検査である。入力変換・描画正確性・スクリーンリーダー適合性や性能を証明するものではない

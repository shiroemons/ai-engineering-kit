---
{
  "id": "react-viewtransition-commit-router-motion-boundary",
  "title": "React 19.3 ViewTransition: commit・router復元・motion設定の境界",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 19.3（2026-09-09公開）; v19.3表示のViewTransition・addTransitionType・startTransition・Suspense参照を2026-10-03確認。ブラウザー実機未検証",
  "tags": [
    "research-domain:frontend",
    "react",
    "ViewTransition",
    "startTransition",
    "addTransitionType",
    "Suspense",
    "popstate",
    "useLayoutEffect",
    "prefers-reduced-motion"
  ],
  "sources": [
    {
      "id": "react-193-viewtransition-release-20261003",
      "url": "https://react.dev/blog/2026/09/09/react-19-3",
      "type": "release_notes"
    },
    {
      "id": "react-viewtransition-reference-20261003",
      "url": "https://react.dev/reference/react/ViewTransition",
      "type": "official_docs"
    },
    {
      "id": "react-addtransitiontype-reference-20261003",
      "url": "https://react.dev/reference/react/addTransitionType",
      "type": "official_docs"
    },
    {
      "id": "react-starttransition-viewtransition-reference-20261003",
      "url": "https://react.dev/reference/react/startTransition",
      "type": "official_docs"
    },
    {
      "id": "react-suspense-viewtransition-reference-20261003",
      "url": "https://react.dev/reference/react/Suspense",
      "type": "official_docs"
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

# React 19.3 ViewTransition: commit・router復元・motion設定の境界

## 調査する問いと適用版

画面を `<ViewTransition>` で囲んだ後、なぜアニメーションが省略されるのか。戻る操作、非同期更新、読み込み表示、動きを減らす設定を壊さず導入する条件を整理する。既存の batching 文書は描画優先度、Fragment ref 文書は DOM 操作、browser 文書は SSR 回避を扱う。本稿は視覚遷移の成立と省略の境界が対象。

React チームの [19.3 公開記事](https://react.dev/blog/2026/09/09/react-19-3) は2026-09-09に ViewTransition の安定版化を発表した。取得した参照ページも v19.3 表示。以前の実験版例をそのまま適用せず、以下を当該版の文書上の契約として扱う。最新パッチ版や全ブラウザー対応を保証するものではない。

## 確認した契約

### 更新とアニメーションの入口

- 通常の即時 `setState` はアニメーションを起動しない。`startTransition` による更新、Suspense の reveal、`useDeferredValue` が入口になる。DOM向けの機能であり React Native 共通の挙動ではない。[公開記事](https://react.dev/blog/2026/09/09/react-19-3#view-transitions)
- `startTransition` の関数自体は直ちに実行される。`await` 後や `setTimeout` 内の state 更新まで自動的に Transition と見なさず、該当更新をもう一度 `startTransition` で囲む。controlled text input の値には使えない。pending 表示は戻り値を待つのではなく `useTransition` の役割。[startTransition](https://react.dev/reference/react/startTransition#caveats)
- 待機画面を出すたびに animate すると、読み込み済み UI の再表示まで遅く感じる。公開記事は fallback を即時表示し、fallback から完成内容への更新だけに animation を絞る `update="auto"` / `default="none"` の組み合わせを紹介している。[Suspense の表示方針](https://react.dev/blog/2026/09/09/react-19-3#animating-fallbacks-images-and-fonts-with-suspense)

### 同じ名前・同じ commit の意味

- `name` は shared element 用。通常は自動名を使い、明示名はアプリ全体で同時に重ならないようにする。share は同じ Transition で旧treeから消える側と新treeに現れる側の対。間に Suspense fallback の表示を挟むと、その後の登場とは対にならない。[共有要素](https://react.dev/reference/react/ViewTransition#animating-a-shared-element)
- `addTransitionType` は遷移理由を加えるが、type は各 commit 後に reset される。fallback の commit に付いた理由は後続 reveal へ残らない。複数 Transition をまとめるとtypeも集まり、複数一致のclassは結合される。一致したtypeの値に `none` があれば無効化が優先する。[type の寿命と選択](https://react.dev/reference/react/addTransitionType)

### router と既存アニメーション

React が `startViewTransition` を管理するため、同じ画面で独自に起動して競合させない。実行中に届いた複数更新はまとめられ、中間画面すべての animation を保証しない。途中の `flushSync` でも省略され得る。legacy `popstate` 由来の戻る操作は scroll/form 復元を優先して animation を省略する。Navigation APIを使うrouterで、NavigationがReactを待つ設計なら `useLayoutEffect` で待機解除する。`useEffect` では相互待ちになる。[内部処理](https://react.dev/reference/react/ViewTransition#how-does-viewtransition-work)・[router 統合](https://react.dev/reference/react/ViewTransition#building-view-transition-enabled-routers)

### 読み込みと利用者設定

ViewTransition の update で Suspense の reveal を animate する際、表示対象の画像と内容が新たに導入する font を待つ。ただしtimeout付きであり、無期限に全resource成功を保証しない。通常の Suspense はそれらを自動で待たず、画像に `onLoad` を付けると当該画像は待機対象から外れる。境界の内外どちらに ViewTransition を置くかで、fallback交換を一つのupdateにするか、個別のexit/enterにするかも変わる。[Suspense](https://react.dev/reference/react/Suspense)

React は `prefers-reduced-motion` に合わせて自動で animation を無効化しない。CSS media query等で抑制する責任を持つ。[利用者のmotion設定](https://react.dev/reference/react/ViewTransition#always-check-prefers-reduced-motion)

## 実務での判断と確認項目

以下は上記から導く独自の導入・テスト方針で、実測結果や公式の必須手順ではない。

1. アニメーションを見せたい画面更新と即時に反映すべき入力を分ける。決済成功・保存完了などの状態は演出の有無に依存させない。
2. 初回導入は一つの遷移に限定し、既存routerが `popstate` か Navigation APIか、独自 `startViewTransition` を呼んでいないか棚卸しする。戻る操作を華やかにするため復元機構だけを外さない。
3. 通常・低速resource・キャッシュ済み・連打のそれぞれで最終画面とscroll位置を確認する。中間animationが省略されても、表示状態の正しさを別に検証する。
4. type のテストは遷移開始、fallback commit、後続revealを別に記録する。最初の「次へ」という理由が最後まで残る前提で方向を固定しない。
5. 同じ商品を一覧と詳細で同時表示する構成は明示nameの衝突検査を加える。動きを減らす設定でも操作・完了表示が成立することを確認する。

## 未確認事項と出典の扱い

- React19.3アプリ、router、ブラウザーでの実行・性能・支援技術テストは未実施。animationやresource待機の時間値、画面上の安定性、router製品ごとの対応版は断定しない。
- 検索evalは本稿の発見性を検証するもので、実際のanimation挙動のテストではない。
- 出典は React チームの公式記事・参照文書（CC-BY-4.0）。本稿は帰属を示した独自日本語要約と設計上の提案で、サンプルコードの転載はない。取得日は全件2026-10-03 UTC、固定公開日を持たない参照ページは取得時点の内容として記録した。

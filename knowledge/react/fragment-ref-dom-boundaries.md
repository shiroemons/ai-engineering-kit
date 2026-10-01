---
{
  "id": "react-fragment-ref-dom-boundaries",
  "title": "React 19.3 Fragment refs: wrapper-free な DOM 操作と focus・observer の対象境界",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 19.3 (Fragment refs stable 2026-09-09); live major-19 reference retrieved 2026-10-01, not patch-pinned",
  "tags": [
    "research-domain:frontend",
    "react",
    "Fragment",
    "FragmentInstance",
    "focus",
    "focusLast",
    "observeUsing",
    "unobserveUsing",
    "first-level",
    "depth-first",
    "cleanup",
    "Activity"
  ],
  "sources": [
    {
      "id": "react-193-fragment-refs-release-2026-10-01",
      "url": "https://react.dev/blog/2026/09/09/react-19-3",
      "type": "release_notes"
    },
    {
      "id": "react-fragment-dom-reference-2026-10-01",
      "url": "https://react.dev/reference/react/Fragment",
      "type": "official_docs"
    },
    {
      "id": "react-versions-reference-2026-10-01",
      "url": "https://react.dev/versions",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# React 19.3 Fragment refs の DOM 境界

## 問いと今回の変更

子コンポーネントに ref の受け渡しを追加できず、機械的な wrapper も layout を崩す場合、複数の DOM node に focus・イベント・可視性の振る舞いをどう付けるか。

[React 19.3 の発表](https://react.dev/blog/2026/09/09/react-19-3)（2026-09-09）は Fragment refs の stable 化を明記する。ref を受けない部品や複数の兄弟要素にも、DOM 構造を変えず限定された操作を加えられる。対象はこの追加 API の採用判断であり、既存の automatic batching や外部 store の hydration 契約とは別の問いである。

## 確認した契約（公式 reference の要約）

根拠は [`Fragment` reference](https://react.dev/reference/react/Fragment)。以下は公式記載であり、後述の設計案・試験案とは区別する。

- ref は明示的な `Fragment` に渡す。短縮記法 `<>...</>` には渡せない。ref の値は DOM Element そのものではなく `FragmentInstance`。
- `addEventListener` / `removeEventListener`、`observeUsing`、`getClientRects` の対象は first-level host children。別の DOM 要素の内部まで直接対象を広げない。
- `focus` は入れ子を depth-first に探し最初の focus 可能要素へ移る。`focusLast` は最後を探す。`blur` は現在の focus が内部にある場合だけ解除する。
- `observeUsing` は `IntersectionObserver` / `ResizeObserver` を受け取り、`unobserveUsing` は同じ observer を指定して解除する。text node は観測せず、text-only の場合は開発時に警告する。
- hidden な `Activity` ツリーには追加イベント listener を適用せず、visible になると適用する。
- `scrollIntoView` は boolean のみを受け、options object はエラーになる。true/省略は最初の子を上端へ、false は最後の子を下端へ寄せる。空なら近い sibling/parent が代替対象になる。

## 対象境界から決める実装方針（独自提案）

1. まず wrapper の目的を分類する。単なる ref の取り付け場所なら Fragment refs を検討する。見出し・領域・リスト・フォーム等の意味や CSS の配置単位を表している要素は、DOM を減らす目的だけで取り除かない。
2. adapter の公開 API を必要な振る舞いに絞る。「DOM Element と同じもの」を返す契約にすると、利用者が任意の属性や DOM メソッドを期待してしまう。focus と visibility を使うだけなら、その2つの操作境界を説明する。
3. 可視性の判定単位を先に決める。兄弟カードのうち1枚でも見えれば全体を visible とするのか、全カードの状態を個別に返すのかを仕様化する。複数の observer entry を1つの boolean に集約する規則はアプリの責任として扱う。
4. 観測対象と focus 対象を同じ配列だと仮定しない。たとえば first-level の section 内に button がある構成では、section を観測することと button へ focus を移すことを別の期待値にする。
5. ref の接続期間に合わせて登録と cleanup を対にする。listener の関数・capture 条件と observer instance を保持し、解除時に別物を生成しない。登録方式は採用コードベースの callback ref / Effect の lifecycle と整合させる。
6. shared observer を使う場合、adapter 自身の購読解除と observer 全体の破棄を分ける。他の adapter と共有している observer を1つの部品の cleanup で止めない設計にする。
7. 一般的な Element 用の scrolling helper に FragmentInstance をそのまま渡さない。smooth 等の options object を必ず送る helper は契約が異なる。必要なスクロール効果を満たす対象と API を別途選ぶ。
8. focus 移動のタイミングと理由を利用者の操作に結び付ける。Fragment があるだけで focus trap、元の要素への復帰、Tab 順、キーボード操作が完成したと扱わない。これらは別の UX 要件としてレビューする。

## 移行時の試験案（未実装・未実測）

以下は採用側での受け入れ条件案であり、本調査が実行したブラウザテストではない。

- wrapper-free layout: 旧 adapter と新 adapter で grid/flex の子、CSS selector、画面構成が意図どおりか確認する。削除してよい wrapper だけを対象にする。
- first-level / depth-first: 直接の兄弟と DOM wrapper 内の子を混在させ、observer の対象と focus の到達先を別々に検証する。
- focusLast / blur: 入れ子の末尾への移動、外部の focus を誤って解除しないこと、focus 可能な子がない画面でのアプリ側の扱いを確認する。
- listener cleanup: mount/unmount、依存変更、再接続を繰り返して、古い handler の呼出や同じ操作の二重処理が残らないことを確認する。
- observer cleanup: 複数 adapter が同じ observer を共有するケースで、一方を外した後も他方の通知を扱えることを確認する。
- text-only / empty: 文字列だけと子がない状態を用意する。開発警告、スクロール代替対象、アプリが期待する空表示を確認し、全ケースで要素があると仮定しない。
- Activity: hidden と visible を切り替え、非表示時の listener 対象外と再表示後の操作を確認する。単に CSS で隠すケースと混同しない。
- scrolling: boolean の両方向と options object を渡してしまう誤用を分けて確認する。特定ブラウザの見た目やタイミングまで reference の保証だとは扱わない。

## 適用版・限界・provenance

- stable 化の根拠は React 19.3 の2026-09-09公開記事。今回開いた reference は v19.3 表示だが、[React Versions](https://react.dev/versions) は major 内で docs を更新し minor / patch 別を提供しないと説明する。特定 patch の実装を固定した解析ではない。
- reference と versions ページの公開日・更新日は明記されていない。UTC 取得日は2026-10-01。release_notes の30日 TTL に合わせ明示期限を2026-10-31とする。
- React 19.2以前の実験チャンネル、React Native、portal / Shadow DOM の詳細、TypeScript 型パッケージの対応版、SSR/hydration と framework 統合、性能差、ブラウザ間差は未検証。API が stable であることからこれらの互換性まで推測しない。
- 既存の ref adapter や DOM utility を置換する場合、利用している全メソッドを棚卸しする。本稿は限定 API の契約を扱い、任意の Element 操作を代替できるとは主張しない。
- react.dev の [README](https://github.com/reactjs/react.dev/blob/main/README.md) と [LICENSE-DOCS.md](https://github.com/reactjs/react.dev/blob/main/LICENSE-DOCS.md) を開き、文書の CC-BY-4.0 を確認した。出典の著作者は Meta Platforms, Inc. と contributor。日本語で独自要約し、設計・試験案を追加した。コードの転載・module 昇格は行っていない。
- reference の対象境界や React major 表示が変わったとき、または adapter に新しい DOM 操作を増やすときは再確認する。

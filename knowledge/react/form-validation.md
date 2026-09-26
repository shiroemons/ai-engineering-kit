---
{
  "id": "react-form-validation",
  "title": "React フォーム入力検証とアクセシブルなエラー表示",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 19 reference + WAI Forms Tutorial + WCAG 2.2 Technique ARIA21 (2026-09-26 確認)",
  "tags": [
    "research-domain:frontend",
    "form",
    "form-validation",
    "validation",
    "error",
    "error-display",
    "error-summary",
    "aria-invalid",
    "aria-describedby",
    "live-region",
    "useActionState",
    "FormData",
    "onSubmit",
    "accessibility"
  ],
  "sources": [
    {
      "id": "react-form-docs",
      "url": "https://react.dev/reference/react-dom/components/form",
      "type": "official_docs"
    },
    {
      "id": "react-use-action-state-docs",
      "url": "https://react.dev/reference/react/useActionState",
      "type": "official_docs"
    },
    {
      "id": "wai-forms-validation-tutorial",
      "url": "https://www.w3.org/WAI/tutorials/forms/validation/",
      "type": "official_docs"
    },
    {
      "id": "wai-forms-notifications-tutorial",
      "url": "https://www.w3.org/WAI/tutorials/forms/notifications/",
      "type": "official_docs"
    },
    {
      "id": "wcag-aria21-technique",
      "url": "https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA21",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# React フォーム入力検証とアクセシブルなエラー表示

React でフォーム入力を検証し、エラーを誰にでも伝わる形で表示する方法。[React `<form>` 公式](https://react.dev/reference/react-dom/components/form) / [React `useActionState` 公式](https://react.dev/reference/react/useActionState) / [WAI Validating Input](https://www.w3.org/WAI/tutorials/forms/validation/) / [WAI User Notification](https://www.w3.org/WAI/tutorials/forms/notifications/) / [WCAG 2.2 Technique ARIA21](https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA21)

## 要点

### 送信モデル (React `<form>` 公式)

- `onSubmit` ハンドラで送信を扱う場合は `e.preventDefault()` でブラウザ既定の再読み込みを抑止し、`new FormData(e.target)` で各欄を `name` ごとに読む。この読み方は入力を uncontrolled に保つ。入力を state で制御する controlled な欄は、送信時に `FormData` ではなくその state から読む。
- 関数を `action` prop に渡すと送信は Transition の中で実行され、`e.preventDefault()` は不要になる。`action` 関数が成功するとフォーム内の全 uncontrolled フィールドがリセットされる。`method` prop の値にかかわらず HTTP メソッドは POST になる。
- `<form>` を Error Boundary で包むと、`action` 関数が throw した送信時エラーの fallback を表示できる。Server Function を `action` に渡し `useActionState` で状態を読む構成にすると、JavaScript バンドル読み込み前の progressive enhancement でも送信エラーを表示できる。

### 検証エラーの扱い (React `useActionState` 公式)

- バックエンドが返す「数量が足りない」のような既知の validation エラーは、`reducerAction` の state として返して UI に inline 表示する。`undefined is not a function` のような未知のエラーは throw し、Error Boundary に伝えて表示する。
- `reducerAction` は `(prevState, formData)` の順で受け取る (`useActionState` なしの `action(formData)` と引数順が違う)。`dispatchAction` は action prop 経由で渡すか、`startTransition` の中で呼ぶ。throw した `dispatchAction` より後に queue された呼び出しは skip されるため、catch してエラー state を返す書き方が推奨される。

### 検証の層 (WAI Validating Input)

- HTML 標準の `required`、email・url・number・range・date・time などの input types、`pattern` による正規表現形式指定をまず使う。ブラウザがバリデーションと入力補助 (date picker など) を担う。`label` 側にも "(required)" のような表示を残し、支援技術や旧ブラウザへの伝達を冗長化する。
- 入力形式には寛容であること。電話番号の区切り違いを受け付ける、国により数字以外を含む郵便番号に `type="number"` を強いない、などが例示される。
- client-side validation だけでは安全は確保できない。迂回や改変が可能なため、server-side でも必ず検証する。

### エラーのアクセシブルな通知 (WAI User Notification + ARIA21)

- 送信失敗時は error summary (エラー一覧) をフォームの前に置く。各項目は対象欄の label 参照・簡潔な説明・修正方法・欄への in-page link を含め、動的表示の場合は容器に `role="alert"` を付ける。見出し (`<h1>` の "3 Errors" など) や `<title>` での通知と組み合わせられる。
- 各欄の error message は `aria-describedby` で欄とプログラム的に関連付ける。検証で失敗した欄には `aria-invalid="true"` を付ける。`aria-invalid` は検証を行う前に `"true"` にしてはならず、`"false"` は属性なしと同等 (ARIA21 の記述とテスト手順)。
- エラーがある送信後は、最初のエラー欄にフォーカスを移すと便利 (WAI の記述)。
- 入力中の即時通知は `aria-live="polite"` の live region に入れ、読み上げを割り込ませずキー入力ごとに読ませない。フォーカス移動時 (blur) の通知は `aria-live="assertive"` で先に読ませる。
- 上記は WCAG 3.3.1 Error Identification (Level A) と 3.3.3 Error Suggestion (Level AA) に対応する手法として整理されている。Technique は達成手法の例であり、WCAG 適合に必須ではない。

## 推奨方法

以下は上記の公式要件からの設計上のまとめであり、公式が定める実装構成そのものではない。

- 検証は「HTML 標準属性 → 送信時の React 側チェック → server-side」の順に層化し、各層のエラー表示先 (inline / error summary) を決める。
- 既知の検証エラーは `useActionState` の state として `{ fieldErrors, formError }` の形で返し、inline の `aria-describedby` 関連付けと error summary の両方から参照する。未知の例外だけを Error Boundary に逃がす。
- `aria-invalid` と `aria-describedby` の付け外しは検証結果の state から一方向に導出し、検証前の欄に `aria-invalid="true"` が残らないようにする。
- 送信ボタンは `useFormStatus` の `pending` で多重送信を抑止する (React `<form>` 公式の pending state の例)。

## 避ける使い方

- **client-side validation だけで済ませる**。迂回可能であり、server-side 検証が必須と WAI が明記している。
- **検証前に `aria-invalid="true"` を付ける**。ARIA21 が禁止し、テスト手順でも検証対象にしている。
- **エラーメッセージを色やアイコンのみで示す**。WAI はメッセージと視覚手がかりの併用と、修正方法の説明を求める。
- **入力中の live region に `assertive` を使う**。キー入力ごとの割り込み読み上げになり、WAI は `polite` を指定している。
- **`useActionState` の既知検証エラーを throw で Error Boundary に逃がす**。公式は既知エラーは state 返却、未知エラーは throw と使い分けている。
- **controlled 入力を `FormData` から読む**。公式は controlled な値は state から読むとし、`FormData` は uncontrolled 向きと説明している。

## 適用版と本番での注意

- React の `<form>` 関数 action / `useActionState` の記述を react.dev の公開ページ (2026-09-26 取得) で確認した。ページ自体に版ラベルはなく、Sandpack の例は react 19.0.0 系を指定する。旧版 React での可否は未確認で、利用版の対応表を別途確認する必要がある。
- WAI Forms Tutorial の validation ページは "Updated 27 July 2019"、notifications ページは "Updated 03 June 2022"、ARIA21 は "Updated 27 April 2026" の表示を同日確認した。Technique は WCAG 2.2 達成手法の例であり、規範要件そのものではない。
- 本文は `expires_at` 2026-12-25 (official_docs TTL 90日)。React 19 系の docs 改訂と WCAG 3.x への移行時は再確認する。
- **未確認**: 各ブラウザの Constraint Validation バブル文言・表示言語の差、スクリーンリーダーごとの `aria-describedby` と `role="alert"` 読み上げ順序の実測。導入前に利用環境での実機確認が必要。
- **未確認**: React Server Function を使わない SPA 構成での `useActionState` とバリデーション state の progressive enhancement 可否。本文は公式の Server Function 前提の例のみを根拠にしている。

---
{
  "id": "react-automatic-batching-transitions",
  "title": "React 18/19 の automatic batching 範囲と flushSync・useTransition・useDeferredValue の使い分け",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 19.3.0 (npm latest, 2026-09-28 確認) / automatic batching・useTransition・useDeferredValue は React 18.0.0 導入、Actions は React 19 導入 / react.dev reference は unversioned current (2026-09-28 取得)",
  "tags": [
    "research-domain:frontend",
    "react",
    "automatic-batching",
    "batching",
    "flushSync",
    "useTransition",
    "startTransition",
    "useDeferredValue",
    "isPending",
    "transition",
    "actions",
    "react-18",
    "react-19",
    "createRoot",
    "concurrent rendering"
  ],
  "sources": [
    {
      "id": "react-v18-release-blog-2022-03-29",
      "url": "https://react.dev/blog/2022/03/29/react-v18",
      "type": "release_notes"
    },
    {
      "id": "react-v19-release-blog-2024-12-05",
      "url": "https://react.dev/blog/2024/12/05/react-19",
      "type": "release_notes"
    },
    {
      "id": "react-flushsync-docs",
      "url": "https://react.dev/reference/react-dom/flushSync",
      "type": "official_docs"
    },
    {
      "id": "react-use-transition-docs",
      "url": "https://react.dev/reference/react/useTransition",
      "type": "official_docs"
    },
    {
      "id": "react-use-deferred-value-docs",
      "url": "https://react.dev/reference/react/useDeferredValue",
      "type": "official_docs"
    },
    {
      "id": "npm-react-latest-manifest",
      "url": "https://registry.npmjs.org/react/latest",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-10-28",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# React 18/19 の automatic batching 範囲と flushSync・useTransition・useDeferredValue の使い分け

React 18 で自動化された state 更新の batching がどこまで効くか、同期的に flush する `flushSync`、非同期で割り込ませる `useTransition` / `startTransition`、追従レンダーを遅らせる `useDeferredValue` をどう使い分けるか。[React v18 リリース投稿](https://react.dev/blog/2022/03/29/react-v18) / [React v19 リリース投稿](https://react.dev/blog/2024/12/05/react-19) / [`flushSync` reference](https://react.dev/reference/react-dom/flushSync) / [`useTransition` reference](https://react.dev/reference/react/useTransition) / [`useDeferredValue` reference](https://react.dev/reference/react/useDeferredValue) / [npm registry `react` latest](https://registry.npmjs.org/react/latest)。以下は「要点」が公式記載の事実、「推奨方法」がそれからの設計案。

## 要点（公式文書に記載された事実）

### automatic batching の適用範囲 (React v18 リリース投稿)

- batching は「複数の state 更新を1回の再レンダーに束ねる」処理で、性能のためのもの。
- React 18 以前は React event handlers 内の更新だけが batch された。promises、`setTimeout`、native event handlers、その他のイベント内の更新は React に既定では batch されない。
- React 18 の automatic batching ではこれらが自動で batch される。公式の `setTimeout` 例では、以前は state 更新ごとに render が2回走り、以後は末尾で1回だけ re-render する。
- React 18 の新機能は `createRoot` / `hydrateRoot` なしでは動作しない（`ReactDOM.render` / `ReactDOM.hydrate` の代替として両 API を案内し「New features in React 18 don't work without it」と明記）。

### flushSync による同期 flush (`flushSync` reference)

- `flushSync(callback)` (`react-dom`) は callback 内の pending work と更新を同期的に flush し、次の行までに DOM が更新済みになる。戻り値は `undefined`。
- caveat（公式記載）: 必要に応じて callback 外の更新（例: pending な click 更新）を先に flush することがある。pending Effects を実行してその更新を同期的に適用することがある。suspend した場合は Suspense の fallback が再表示され得る。著しく性能を害し得るため「last resort」。
- render 中、`useLayoutEffect` / `useEffect`、class component の lifecycle 内で呼ぶと noop となり `flushSync was called from inside a lifecycle method` と警告される。event handler 内での呼出は安全と公式が明記し、Effect からどうしても使う場合は `queueMicrotask` への延期が代替例。
- `beforeprint` の例: `flushSync` を使わないと印刷ダイアログには `isPrinting` が "no" と表示される。React が既定で更新を非同期に batch するため、ダイアログ表示時点では state が更新前だから。

### useTransition / startTransition (React v18 リリース投稿 + `useTransition` reference)

- 両方 React 18 で追加。changelog は `startTransition` を「pending feedback のない `useTransition` 版」と説明する。`useTransition` は `[isPending, startTransition]` の正確に2要素の配列を返す。
- update は urgent（クリック・キー入力等の直接操作）と non-urgent（画面遷移等）に区分される。`startTransition` 内の更新は non-urgent として扱われ、urgent 更新に割り込まれ、中断された描画作業は破棄されて最新更新だけが描画される。
- `startTransition` のコールバック（React 19 では "Action"）は遅延せず即時実行される（`1, 2, 3` の順に print される公式 Troubleshooting）。Action の呼び出し中に**同期的に**予約された state 更新だけが Transition 扱いになる。
- Action 内の `setTimeout` で行う state 更新は Transition 扱いにならない。`await` の後に行う state 更新も Transition 扱いにならず、さらに `startTransition` で包む必要がある。公式は「async context のスコープが失われる JS の制限により、`AsyncContext` が使えるようになるまで修正する既知の制限」と明記する。
- Transition は中断可能だが、controlled text input を制御する state には使えない（入力への応答は同期的であるべきため）。
- 進行中の複数の Transition は現在 React がまとめて batch する。公式は「将来のリリースで解消され得る制限」と記載する。
- Action が throw または rejected Promise を返すと、`useTransition` 由来のエラーは Error Boundary に表示される。component に関連付けられない standalone の `startTransition` は `isPending` を持たず、そのエラーは Error Boundary で扱えない。
- `await` を挟んだ連続した Action は応答が out-of-order になり得る（既知の制限）。公式は順序保証を `useActionState` や `<form>` action 等の組み込み抽象に任せるか、自分で queuing / abort を実装すると案内する。

### React 19 の Actions (React v19 リリース投稿)

- React 19 で `startTransition` に async 関数を渡す Actions が導入された。async transition は `isPending` を即 `true` にし、pending はリクエスト開始時に始まり、最終の state update が commit されたら自動でリセットされる。pending state、エラー、optimistic updates、フォームの管理を自動化する。
- `useActionState`、`useOptimistic`、`react-dom` の `useFormStatus`、`<form>` の action prop に関数を渡す form Actions が導入された。
- `useDeferredValue` に `initialValue` オプションが追加された。提供すると初回レンダーでは `initialValue` を返し、その後に deferredValue で background re-render をスケジュールする。
- React 19 は Canary チャンネルの Server Components の機能一式を包含する。

### useDeferredValue (`useDeferredValue` reference)

- signature は `useDeferredValue(value, initialValue?)`。`initialValue` は省略可で、省略時は初回レンダーで defer されない（前バージョンの value が存在しないため）。
- 更新時、React はまず旧 value での再レンダー（返り値は旧値）を行い、その後に新 value での background re-render をスケジュールする。background re-render は中断可能で、`value` の更新が来れば最初からやり直す（比較は `Object.is`）。
- 固定の delay はなく、元の re-render が終わるやいなや background re-render を開始する。イベント（キー入力等）による更新が優先され、background re-render を割り込む。
- Transition の中の更新では、`useDeferredValue` は常に新しい value を返し、deferred render を作らない（既に defer されているため）。
- Suspense と統合しており、background update が suspend しても fallback は表示されず、旧い deferred value を表示し続ける。
- それ単体では追加の network request を防がない。キー入力ごとに request は発生し、defer されるのは表示だけである。
- background re-render は画面に commit されるまで Effects を発火しない。
- 描画の最適化として使う場合、子コンポーネントは `memo` で包む必要がある（公式 Pitfall）。render の中に作った新規 object を渡すと毎回別物になり、不要な background re-render が起きる。渡す値は primitive か、render の外で作った object にする。

## 推奨方法

以下は上記の公式記載からの設計案であり、公式が定める実装構成そのものではない。

- render 回数の削減は automatic batching に任せる。テストは「state 更新ごとに1回 render」を固定期待せず、React 18 以前の前提（event handler 外は毎回 render）を確認していた箇所を更新する。
- 同期的な DOM 更新が必要なのは browser API 統合のような特殊ケース（`beforeprint` 等）だけと割り切り、event handler 内の `flushSync` に限定する。Effect 内からは `queueMicrotask` へ延期するか、event handler へ移す（公式の代替案）。
- タブ切替・ナビゲーション等の画面遷移は `useTransition` と `isPending` の視覚表示で包み、遅い下位ツリーの入力追従は `useDeferredValue` + 子の `memo` で扱う。入力そのものの state は同期のままにする。
- 順序保証が必要な送信（フォームや連続するリクエスト）は自前で `startTransition` + async に頼らず、React 19 の `useActionState` / form Actions へ預ける。
- `await` 後の state 更新は `startTransition` で再ラップして Transition 扱いにする。
- 遅れが気になる導出値には `query !== deferredQuery` のような比較で stale 表示（減光など）を添える（公式例のパターン）。

## 避ける使い方

- **controlled text input の state を Transition で更新する**。公式が明示的に不可とし、代替として2つの state か `useDeferredValue` を挙げる。
- **Action 内の `setTimeout` で setState して Transition とみなす**。コールバックは即時実行され、呼び出し中に同期予約された更新だけが対象。
- **`await` 後の setState を再ラップせず Transition と信じる**。公式が既知の制限として再ラップを要求している。
- **render / `useLayoutEffect` / `useEffect` / class lifecycle 内で `flushSync`**。noop になり警告されるだけ。
- **`useDeferredValue` を最適化として使うのに子を `memo` で包まない**。親の再レンダーで子が毎回再描画され、効果が消える。
- **`useDeferredValue` で network request が減ると期待する**。公式は「単体では防がない」と明記。
- **render の中で作った新規 object を `useDeferredValue` に渡す**。毎回変わるので background re-render が連発する。
- **複数の進行中 Transition が個別に pending 管理される前提で組む**。現在はまとめて batch される（既知の制限）。
- **standalone `startTransition` のエラーを Error Boundary で捕捉できる前提にする**。component と紐づかないため扱えない。
- **自前の async Action だけで応答順序を保証する**。out-of-order は既知の制限で、順序は組み込み抽象か自前の queuing / abort で担保する。

## 適用版と本番での注意

- npm registry の `react` latest は **19.3.0**（2026-09-28 取得、license MIT）。automatic batching と `useTransition` / `startTransition` / `useDeferredValue` は React 18.0.0（2022-03-29 投稿）導入、Actions と `useDeferredValue(initialValue)` は React 19（2024-12-05 投稿）導入。
- reference 3ページは版ラベルのない unversioned な現行 react.dev ページ（2026-09-28 取得）で、記載内容は React 19.x の挙動として読む。React 18.x での差（`initialValue` なし以外）は未確認。
- 明示期限は 2026-10-28。最短の期限は2つのリリース投稿（`release_notes`、TTL 30日）によるもので、reference ページと npm manifest は `official_docs`（TTL 90日）。React の新リリース時と reference 改訂時に再確認する。
- **未確認**: React 17 以前（`ReactDOM.render` の legacy mode）での実挙動、dev / production の差、`<StrictMode>` 下の差。公式投稿の説明に基づく historic な記載のみ。
- **未確認**: render 回数の実測。`setTimeout` 例の「2回→1回」は公式コード内の説明であり、本リポジトリでの再現計測は行っていない。
- **未確認**: scheduler の内部優先度や React 19.3.0 固有の挙動差。reference ページは版非依存で書かれている。

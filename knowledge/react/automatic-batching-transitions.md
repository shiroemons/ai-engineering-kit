---
{
  "id": "react-automatic-batching-transitions",
  "title": "React 18/19 の automatic batching と React 19.3 Transition 独立描画・async Action の境界",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 18.0.0 introduction (2022-03-29), React 19 Actions (2024-12-05), React 19.3 release (2026-09-09); v19.3.0 source commit 1d34f91dfde6bba84d08b683aaba164c7194dacb; live major-19 references retrieved 2026-10-02 UTC",
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
    "concurrent rendering",
    "parallel-transitions",
    "entanglement",
    "async-action",
    "19.3",
    "same-event",
    "update-queue"
  ],
  "sources": [
    {
      "id": "react-18-batching-release-2026-10-02",
      "url": "https://react.dev/blog/2022/03/29/react-v18",
      "type": "release_notes"
    },
    {
      "id": "react-19-actions-release-2026-10-02",
      "url": "https://react.dev/blog/2024/12/05/react-19",
      "type": "release_notes"
    },
    {
      "id": "react-193-browser-release-2026-10-02",
      "url": "https://react.dev/blog/2026/09/09/react-19-3",
      "type": "release_notes"
    },
    {
      "id": "react-flushsync-reference-2026-10-02",
      "url": "https://react.dev/reference/react-dom/flushSync",
      "type": "official_docs"
    },
    {
      "id": "react-use-transition-reference-2026-10-02",
      "url": "https://react.dev/reference/react/useTransition",
      "type": "official_docs"
    },
    {
      "id": "react-start-transition-reference-2026-10-02",
      "url": "https://react.dev/reference/react/startTransition",
      "type": "official_docs"
    },
    {
      "id": "react-use-deferred-value-reference-2026-10-02",
      "url": "https://react.dev/reference/react/useDeferredValue",
      "type": "official_docs"
    },
    {
      "id": "react-versions-browser-reference-2026-10-02",
      "url": "https://react.dev/versions",
      "type": "official_docs"
    },
    {
      "id": "react-193-transition-scheduler-source-2026-10-02",
      "url": "https://github.com/react/react/tree/1d34f91dfde6bba84d08b683aaba164c7194dacb",
      "type": "github_repository_analysis"
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

# automatic batching と React 19.3 Transition の独立描画

## 問いと訂正の理由

更新のまとめ方を利用して入力の応答性を保つとき、`automatic batching`、`flushSync`、`useTransition`、`useDeferredValue` はどの責務を持つか。特に、遅い処理Aが別の更新Bを待たせるかを、React 19.3 でどう判断するか。

本稿の旧版は適用版を19.3.0としながら、進行中の Transition は常にまとめられるという注意を一律に適用していた。[2026-09-09公開の React 19.3 発表](https://react.dev/blog/2026/09/09/react-19-3)は、無関係な Transition が遅い Transition に巻き込まれないよう独立した render を行う変更を明記する。今回は新しい紹介文を増やす代わりに、この版境界を訂正し、本文全体の出典を再確認した。

結論は「19.3で全部が独立する」でもない。同じ event の lane 割当、同じ update queue の結合、並行する async Action の entanglement は固定した19.3.0 sourceにも残る。描画の独立性と、非同期処理の完了・順序を区別する。独立した render という変更だけから、各 `useTransition` の `isPending` が完全に独立するという保証は導かない。

## 確認した公開契約

以下は公式文書の要約。後述の実装観察と独自の設計案とは区別する。

### React 18 で導入された automatic batching

[React 18.0.0 発表（2022-03-29）](https://react.dev/blog/2022/03/29/react-v18)は、複数の state 更新をまとめて再描画する範囲を拡張したと説明する。React event handlers に限らず、promises、`setTimeout`、native event handlers 内などの更新も既定で batch される。React 18移行時には `createRoot` / `hydrateRoot` が新機能を使う入口となる。

同記事は urgent な入力への反応と non-urgent な表示更新を区別し、Transition の描画作業が入力に割り込まれ得ることも説明する。これは「別々の更新がどこまで相互に待つか」という19.3の変更と異なる軸であり、19.3で automatic batching 全体が廃止されたとは読まない。

### startTransition、useTransition、async Action

[`startTransition` reference](https://react.dev/reference/react/startTransition)で確認した範囲:

- callback は直ちに実行され、その呼出中に同期予約された state 更新が Transition になる。`setTimeout` 内で後から行う更新は自動的に引き継がない。
- async Action の `await` は完了待ちに含められるが、`await` 後の state 更新を Transition とするには追加の `startTransition` が必要。完了待ちと更新の分類は別の契約。
- controlled text input 自体の更新には使えない。standalone の関数は `isPending` を返さず、callback の throw / rejected Promise は `reportError` へ報告される。

[`useTransition` reference](https://react.dev/reference/react/useTransition)で確認した範囲:

- `[isPending, startTransition]` を返す。pending は最初の開始で true となり、関係する Actions が完了し最終状態が表示されるまで続く。
- この Hook の Action エラーは、Hook を呼ぶコンポーネントを包んだ Error Boundary に表示できる。standalone のエラー経路と区別する。
- async Action 内のリクエストは応答が out-of-order になり得る。`useActionState` / `<form>` action 等の上位抽象が案内され、自作する場合は queuing / abort 等の順序制御が必要とされる。

[React 19 発表（2024-12-05）](https://react.dev/blog/2024/12/05/react-19)は async 関数を使う Transitions、Actions による pending・エラー・optimistic updates の管理と、`useActionState` / `useFormStatus` / form Actions を説明する。これらの導入を、19.3で新規追加されたものとは扱わない。

### flushSync は同期 DOM 連携の例外

[`flushSync` reference](https://react.dev/reference/react-dom/flushSync)によると、callback 内の更新を同期反映し、戻った時点の DOM を外部 API が使えるようにする。必要なら callback 外の pending 更新や Effects も処理し、suspend によって fallback が再表示され得る。範囲を callback だけと仮定しない。

同ページの `beforeprint` / `isPrinting` 例は印刷前の同期反映を示す。render、`useEffect` / `useLayoutEffect`、class lifecycle 内からの呼出は noop と警告の対象。通常は event handler に移し、難しい場合の `queueMicrotask` は追加の同期 render を生み性能面でも最後の手段とされる。Transition の待ちを一般的に解消する用途ではない。

### useDeferredValue は表示の追従を遅らせる

[`useDeferredValue` reference](https://react.dev/reference/react/useDeferredValue)で再確認した契約:

- `initialValue` の省略時は初回から渡した値を返す。指定時はそれを先に使って background re-render を予約する。この引数は[React 19](https://react.dev/blog/2024/12/05/react-19)で導入された。
- 更新値は `Object.is` で比較され、旧値を使う render の後に新値の中断可能な再描画を試みる。固定 delay はなく、新しい入力が来れば再試行できる。Transition 内では既に遅延されているため新しい値を返す。
- background render が Suspense で止まる場合は旧値を維持する。Effects は画面へ commit するまで発火しない。これは network request の削減機能ではない。
- primitive 値か安定した object を使う。render ごとに新しい object を作ると余計な background re-render が起きる。公式の遅いリスト最適化例は `memo` で同じ props の再描画を省く構成であり、Hookを追加するだけで計算そのものが高速化するわけではない。

## React 19.3 の境界を固定 source で確認する

### live reference と release の説明差

UTC 2026-10-02取得の `useTransition` / `startTransition` reference には、複数の進行中 Transition をまとめるという注意が残る。一方、19.3 release は無関係な Transition の独立描画を明記する。この注意を19.3のすべての render に適用することも、releaseを根拠にすべての async Action が独立したと拡張することも避ける。

[React Versions](https://react.dev/versions)によれば文書は major 内で更新され、minor / patch 別 reference は公開されない。ここでは19.3の公開変更を release に、残る結合条件を下記の immutable source に分けて根拠付ける。live reference の一文だけで版差を決めない。

### 19.3.0 の実装観察（公開 API の追加保証ではない）

対象は [v19.3.0 tag が指す commit 1d34f91dfde6bba84d08b683aaba164c7194dacb](https://github.com/react/react/tree/1d34f91dfde6bba84d08b683aaba164c7194dacb)。GitHub APIで tag と40桁 commit の対応を確認した。次はコードを読んだ観察であり、本リポジトリで React の runtime test を実行した結果ではない。

1. [`ReactFeatureFlags.js`](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/shared/ReactFeatureFlags.js)の `enableParallelTransitions` は true。[`ReactFiberLane.js`](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberLane.js)の Transition update lane 選択は、この条件下で `getHighestPriorityLane` を返す。すべての Transition update lanes を一括選択する分岐とは異なる。
2. 同じ `ReactFiberLane.js` には `getEntangledLanes` / `markRootEntangled` が残る。独立描画を有効にしても、結合済みの lane を加える仕組みは取り除かれていない。
3. [`ReactFiberRootScheduler.js`](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberRootScheduler.js)の `requestTransitionLane` は、同一 event の Transition に同じ lane を使う。これはscheduler内部の割当単位であり、あらゆるJavaScriptイベント境界を本稿で定義し直すものではない。async scope が存在する場合はその lane を再利用する。「別の startTransition 呼出なら必ず別 render」とは解釈できない。
4. [`ReactFiberHooks.js`](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberHooks.js)の `entangleTransitionUpdate` は、同じ update queue に残っている Transition lanes と新しい lane を結合する。関数を2回呼ぶだけで共有 state の意味や中間状態の制約が消えるわけではない。
5. [`ReactFiberAsyncAction.js`](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberAsyncAction.js)は、並行する async Actions の pending count、共有 lane、完了通知を管理する。`AsyncContext` のような対応付けなしには独立した Action を区別できないため結合する旨の説明と処理が残る。

したがって、同一 queue / event / async scope の条件を調べずに、19.3で一律に「全部待つ」または「絶対に待たない」と結論しない。lane の値や内部関数をアプリから利用する提案でもない。

## 実装を選ぶ判断（独自提案）

- 入力値と遅くてもよい結果表示を分ける。入力は同期的に更新し、結果側の state を扱えるなら Transition、prop や Hook の戻り値に追従させるなら `useDeferredValue` を選ぶ。
- 遅いAと無関係なBを19.3で独立させたいときは、まず state とデータの依存を調べる。同じ queue を共有している処理や、完了を待つ async Action を、見た目の別パネルという理由だけで独立とみなさない。
- 同時完了が製品要件なら、旧来の render batching を取引や整合性の代わりにしない。アプリ側で必要な結果を揃えてから公開する状態遷移を設計する。
- 各操作の送信完了を示したい場合、単一の `isPending` を全ネットワーク処理の台帳にしない。操作ID・成功/失敗・キャンセルを持つアプリの状態と、表示の pending を区別する。
- `await` 後の state 更新を再ラップすることと、古い応答を採用しないことを別々にレビューする。Transition化だけで request ordering の問題が解消したとみなさない。
- 古い結果を見せ続ける画面では、`query !== deferredQuery` 等から更新待ちを表示する。検索回数や帯域を減らす必要があれば、表示の defer とは別に request 制御を設計する。
- `flushSync` はブラウザ連携の同期要件を確認して局所化する。Aの待ちがBへ波及する理由を調べずに投入すると、入力やSuspenseの見え方を悪化させるおそれがある。

## 採用側で追加する試験案（未実装・未実測）

本稿の検索 eval は知識の取得を検証するだけで、以下の React 実行時の振る舞いを証明しない。

1. 独立した state / データを持つ2つの表示を用意し、異なる操作でAを遅く、Bを先に解決する。19.3でBがAの完了まで巻き込まれないケースを、採用 framework と実際の構成で確認する。
2. 同じ試験を同一 event、同じ update queue、重なる async Actions へ変える。別々の完了が必ず得られるという期待値を機械的に使い回さない。
3. async Action の `await` 前後と `setTimeout` の更新を区別し、入力の同期反映、pending の終了、Error Boundary / `reportError` の観測先を確認する。
4. リクエストを逆順に完了させ、後から返った古い結果が現在の選択を上書きしないことをアプリ側の期待値で検証する。
5. `useDeferredValue` の初回 `initialValue`、primitive / object identity、`memo`、Suspenseによる旧表示の保持を個別に試す。network request数とrender数を別々に計測する。
6. `beforeprint` 連携がある場合だけ `isPrinting` の反映と印刷後の復帰を試す。通常の画面操作に `flushSync` を広げた効果と解釈しない。

## 適用版・取得日・ライセンス・限界

- 取得日はすべて2026-10-02 UTC。導入日の根拠は18.0.0の2022-03-29、19の2024-12-05、19.3の2026-09-09公開記事。v19.3.0 の日付は Versions でも確認した。
- runtime source は上記の40桁 commit（commit日時2026-09-09T13:21:53Z）だけを解析した。react.dev reference はv19.3表示だが固定patchの文書ではなく、各referenceの公開日・更新日は明記されていない。
- 19.2以前の全patchや独自rendererへ一律の結論を広げない。React Native、SSR / hydration、framework独自のdata cacheやAction統合、開発/production差、StrictMode下のrender回数、性能の実測は未確認。
- upstreamのテストやReactのruntime testは今回実行していない。本稿の結論はrelease、公開reference、限定したsource観察であり、すべての依存関係・スケジューラ経路の証明ではない。
- GitHubの一部blobページはnative web取得で失敗したため、同一commitのファイルをGitHub connectorで取得した。`LICENSE-DOCS.md`のHTML取得失敗は公式raw URLで補った。未取得ページの内容を推測して埋めていない。
- react.dev の[README](https://raw.githubusercontent.com/reactjs/react.dev/main/README.md)と[LICENSE-DOCS.md](https://raw.githubusercontent.com/reactjs/react.dev/main/LICENSE-DOCS.md)を開き、文書のCC-BY-4.0を確認した。著作者はMeta Platforms, Inc.とcontributor。日本語で独自に再構成し、設計・試験案を追加した。
- runtime source は[固定commitのLICENSE](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/LICENSE)と解析対象のヘッダーでMITを確認した。コードの転載・改変・module昇格はない。
- 明示期限は2026-11-01。release_notesの30日TTLに合わせた再確認期限であり、古いリリースの歴史的事実がその日に変わるという意味ではない。新しいReact版、referenceの記述修正、アプリのAction構成変更時にも再確認する。

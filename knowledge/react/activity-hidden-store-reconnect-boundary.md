---
{
  "id": "react-activity-hidden-store-reconnect-boundary",
  "title": "React 19.3 Activity: 非表示中の購読解除と再表示時 snapshot 修復の境界",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 19.3.0; tag resolves to 1d34f91dfde6bba84d08b683aaba164c7194dacb (2026-09-09); Activity introduced in React 19.2 (2025-10-01); live major-19 reference retrieved 2026-10-03 UTC",
  "tags": [
    "research-domain:frontend",
    "react",
    "Activity",
    "hidden",
    "useSyncExternalStore",
    "snapshot",
    "reconnect",
    "cleanup",
    "memo",
    "updateStoreInstance"
  ],
  "sources": [
    {"id": "react-activity-reference-20261003", "url": "https://react.dev/reference/react/Activity", "type": "official_docs"},
    {"id": "react-193-activity-store-release-20261003", "url": "https://react.dev/blog/2026/09/09/react-19-3", "type": "release_notes"},
    {"id": "react-192-activity-introduction-20261003", "url": "https://react.dev/blog/2025/10/01/react-19-2", "type": "release_notes"},
    {"id": "react-193-activity-store-implementation-20261003", "url": "https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberHooks.js", "type": "github_repository_analysis"},
    {"id": "react-usesyncexternalstore-reference-2026-10-01", "url": "https://react.dev/reference/react/useSyncExternalStore", "type": "official_docs"},
    {"id": "react-versions-reference-2026-10-01", "url": "https://react.dev/versions", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/frontend.json"]
}
---

# React Activity の非表示・購読解除・再表示

## 問いと採用判断

タブや詳細ペインをアンマウントせず保持したいとき、`Activity` を使えば購読や動画も安全に停止し、再表示時に外部 store の最新値へ戻れるのか。

採用の条件は、保持したい UI state と、非表示中も必要な処理を分離できること。Activity は Effect を停止する一方で UI state と DOM を保持する。React 19.3 は外部 store の再接続時に古い snapshot が残る2経路を修正したが、アプリ固有の cleanup や、受信しなかったイベントの再生まで提供するものではない。

以下では公式の公開契約、固定実装での観察、独自の採用・試験案を区別する。既存の [snapshot・SSR hydration 文書](usesyncexternalstore-snapshot-hydration.md) は adapter の基本契約、[Fragment refs 文書](fragment-ref-dom-boundaries.md) は DOM 操作対象が主題。本稿は hidden 期間を挟む副作用の所有者と再接続の回帰条件を扱う。

## 版の根拠: Activity 導入と再接続修正を分ける

- [React 19.2 の発表](https://react.dev/blog/2025/10/01/react-19-2)（2025-10-01）が Activity の導入を記載する。`hidden` / `visible` による優先度と Effect の切り替えは、この導入時からの機能。
- [React 19.3 の changelog](https://react.dev/blog/2026/09/09/react-19-3#changelog)（2026-09-09）は、Activity が hidden の間の store mutation を `useSyncExternalStore` が取り逃す不具合を、修正 [#36947](https://github.com/react/react/pull/36947) として明記する。修正 PR の merge 日だけから出荷版を推定していない。
- GitHub API で `v19.3.0` を `1d34f91dfde6bba84d08b683aaba164c7194dacb` に解決し、同じ commit の実装と回帰テストに修正があることも確認した。19.2 系の全 patch に対する有無や backport 状況は調べていないので、「19.2 全版が未修正」とは断定しない。

## 公開契約: 非表示は処理全体の凍結ではない

根拠は [`Activity` reference](https://react.dev/reference/react/Activity)。`mode` は `visible` / `hidden` で、省略時は `visible`。

| 対象 | hidden と再表示の契約 |
|---|---|
| 子の UI と state | hidden は要素を `display: none` にして隠し、state を保持する。単なる条件付き unmount とは異なる |
| 子の Effect | hidden 時に cleanup し、visible へ戻ると再作成する。UI の購読解除は正常な挙動 |
| 子の props 更新 | hidden でも低い優先度で再 render する。処理が永久停止するという契約ではない |
| 初回から hidden | Effect を mount せず低優先度で pre-render する。事前にデータを読むのは Suspense 対応の source であり、Effect 内 fetch は検出されない |
| DOM 自身の副作用 | DOM を破棄しないため、動画などは非表示にしただけでは止まらない。必要な停止処理を cleanup に置く |

公式の動画例は、非表示と同時に停止させる目的で `useLayoutEffect` の cleanup を用いる。通常の `useEffect` では Suspense の再 suspend や View Transition によって処理が遅れる場合がある、と説明している。一方、text-only の hidden Activity は、隠す対象の DOM element がないため DOM 出力自体がない。「どの子も同一 DOM node を保持する」という一般化はしない。

## 19.3.0 の実装観察: reconnect で何を修復するか

対象は固定 commit の [`ReactFiberHooks.js` L1739–1913](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberHooks.js#L1739-L1913)。以下の内部関数名・flag は公開 API ではなく、この版の修正を説明するための観察である。

`updateSyncExternalStore` は render 中に snapshot を読み、subscription の Effect を用意する。さらに `updateStoreInstance` の Effect を、値が変わらない render でも Effect list へ残す。変更判定が偽なら `HookHasEffect` を付けないので、通常 commit では不要な実行を避けつつ、passive Effects の reconnect 時にはその検査を実行できる。

再接続側の `updateStoreInstance` は、render が使った値を登録し直した後、現在の `getSnapshot` と比較する。差があれば `forceStoreRerender` を使って更新を要求する。ここで回復するのは現在値との不一致であり、hidden 中の各通知を保存・順番どおり再送する機構ではない。

### 回帰条件1: reveal の render 後、再購読前に変わる

[同じ tag のテスト L355–414](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/__tests__/useSyncExternalStore-test.js#L355-L414) は次の経路を確認している。

1. visible の subtree が store の revision 1 を読み、購読者が1つある
2. hidden にすると購読者が0になる
3. visible に戻す render はまだ revision 1 を読む。その commit の wrapper の layout Effect が store を revision 2 にする
4. 再接続後の再検査によって revision 2 へ更新され、最終出力と購読数1を確認する

この条件では「再表示の render で `getSnapshot` を読むから十分」という説明が成立しない。読み取りと再購読の間に変更が入るからである。テストが初回読み取りと修復後の読み取りを両方記録することは、ブラウザーで古い表示が一度も paint されないという保証ではない。

### 回帰条件2: hidden 中の変更と memo bailout

[テスト L416–478](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/__tests__/useSyncExternalStore-test.js#L416-L478) は、単純な hide / show 試験で漏れやすい準備手順を含む。

1. `React.memo` の subscriber を visible で mount する
2. store は変えず、label prop だけを一度変更して再 render する。この段階が、修正前に snapshot 再検査用の Effect が list からなくなる状況を作る
3. hidden にして購読を解除し、その後 store を変更する。購読者がいないため、この変更による更新通知は届かない
4. 同じ label のまま visible に戻す。memo bailout で subscriber 自体の render が省略されても、再接続の検査によって新しい store 値が最終出力に現れる

テストのコメントは React Compiler による memoization も想定するが、試験本体は `React.memo` と `ReactNoop` を使う。この資料の存在を、特定 Compiler 版・ブラウザー・store 製品の実測結果へ読み替えない。

## 実装を選ぶ判断と落とし穴（独自提案）

1. **UI 購読とデータ生産を別々に配置する。** 非表示タブだけの subscription は停止させる。非表示中も必要な通知受信・共有同期・バックグラウンド処理は、寿命を管理できるサービスや Activity の外側に置く。UI が最新 snapshot を読めても、データ生産側まで停止していれば、受信していない更新は生成されない。
2. **最新値で足りるか、イベント履歴が必要かを決める。** 最終ステータスの表示なら snapshot で復元できる。全変更の監査、順序依存の処理、必須通知には、別の永続ログ・sequence・再取得手段を用意する。Activity の state 保持をイベント配送保証として使わない。
3. **cleanup を可視性 flag の Effect 本体へ追い出さない。** hidden 後にその subtree の Effect が mount して停止処理をしてくれる設計は避ける。取得した subscription、timer、player 等の所有者が返す cleanup に解除を置き、再接続時に再作成できる形にする。
4. **DOM の保存と停止を個別に判定する。** video の時刻や入力内容を残したくても、音声・埋め込み側の処理まで継続してよいとは限らない。公式が注意する video / audio / iframe を棚卸しし、停止 API や親子の協調手段を対象ごとに確認する。cross-origin iframe 内を親の Effect が無条件に制御できるとは仮定しない。
5. **snapshot adapter の基本契約を守る。** [`useSyncExternalStore` reference](https://react.dev/reference/react/useSyncExternalStore) の immutable snapshot と未変更時の同値性は、この修正後も必要。内部の比較は `Object.is` に基づくため、同じ object を破壊的更新して返す adapter が自動修復されるとは考えない。
6. **pre-render の効果を測ってから保持範囲を広げる。** 次に開く可能性の高い少数ペインから導入する。保持した DOM/state、低優先度の再 render、事前ロードの通信は無料ではない。メモリ使用量、復帰時間、無駄なロードを計測し、保持数や破棄条件をアプリ側で決める。
7. **回避策による state 破棄を意図的に選ぶ。** 修正を確認できない版で問題が再現する場合、影響ペインだけ条件付き unmount に戻す選択肢はあるが、入力・展開状態を失う。単に毎回 key を変えて「直った」とせず、保存したい値を外へ移すか、修正版への更新を評価する。

## 採用側の受入試験案

次は未実装・未実行の試験案。上記の upstream テストを読み取った事実と区別する。

| 場面 | 判定したいこと |
|---|---|
| visible → hidden → visible | 入力・展開状態を保持し、UI の subscription は解除・再作成され、重複しない |
| props-only 更新後に hidden | store を変えず一度 props を変更した後に隠す。hidden 中の mutation と同じ props での reveal を組み合わせ、memo bailout の回帰を拾う |
| reveal commit の layout Effect | render 後・再購読前の store 更新を作り、別のユーザー操作なしで最終表示が追従する |
| 初回から hidden | Effect fetch が未実行であることと、Suspense 対応データの事前取得を区別して観測する |
| 動画の再生中に hidden | 音声を停止し、意図した再開位置は保つ。通常 Effect と layout cleanup の必要性を実画面で確認する |
| shared store と複数タブ | 1つの UI 購読解除が、別の visible subscriber や継続すべきデータ生産を止めない |
| hidden 中に複数イベント | 最終 snapshot と履歴処理の期待値を分け、失われた通知が自動 replay される前提を置かない |
| StrictMode / repeated reveal | cleanup と再作成を繰り返しても listener・timer・player が累積しない。生涯一回という Effect 呼出数に依存しない |

## 適用限界・取得・provenance

- 取得日は2026-10-03 UTC。対象は React DOM の採用判断と native `useSyncExternalStore` の19.3.0実装。shim のテストではないことを upstream ファイルの冒頭でも確認した。
- `Activity` と `useSyncExternalStore` の live reference には公開・更新日の明記を確認できない。[React Versions](https://react.dev/versions) は major 内で文書を更新し、minor / patch 別の文書を提供しない方針を記載する。API reference のすべてを19.3.0の固定コードと同一視しない。
- 既存 catalog の `useSyncExternalStore` と versions は、今回もページを開いて上記の契約が適用可能と確認したうえで無変更で再利用する。新規 source だけ取得日を記録し、既存 source の期限は延ばさない。最短 TTL は新規 release notes の30日なので、本稿の期限は2026-11-02。
- React Native、third-party store adapter、Compiler の生成物、SSR の streaming / hydration、ブラウザーの paint 順、実負荷での速度・メモリは未検証。hidden の scheduling を完了時刻・CPU使用量の上限保証へ広げない。
- React 19.3 の別の Activity 修正（portal、title、hidden error 等）の詳細は対象外。外部 store 修正の確認だけで Activity 移行全体の安全性を保証しない。
- react.dev 本文は Meta Platforms, Inc. と contributor による CC-BY-4.0。公開 [README](https://github.com/reactjs/react.dev/blob/main/README.md) と [LICENSE-DOCS.md](https://github.com/reactjs/react.dev/blob/main/LICENSE-DOCS.md) を確認した。React 実装・テストは固定 commit の [LICENSE](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/LICENSE) と対象2ファイルの header で MIT を確認した。
- 本稿は出典付きの日本語の独自要約・実装観察・試験提案。公式コード・テストの転載、実行、module 昇格は行っていない。

---
{
  "id": "react-useeffectevent-committed-values-resync-boundary",
  "title": "React 19.3 useEffectEvent: committed値・再同期依存・イベント時点の境界",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 19.3.0 @ 1d34f91dfde6bba84d08b683aaba164c7194dacb (2026-09-09); useEffectEvent introduced in 19.2 (2025-10-01); live major-19 docs retrieved 2026-10-03 UTC",
  "tags": ["research-domain:frontend", "react", "useEffectEvent", "latest committed", "forwardRef", "memo", "Context", "dependencies", "stale closure", "cleanup"],
  "sources": [
    {"id": "react-useeffectevent-reference-20261003", "url": "https://react.dev/reference/react/useEffectEvent", "type": "official_docs"},
    {"id": "react-separating-effect-events-20261003", "url": "https://react.dev/learn/separating-events-from-effects", "type": "official_docs"},
    {"id": "react-193-effectevent-release-20261003", "url": "https://react.dev/blog/2026/09/09/react-19-3", "type": "release_notes"},
    {"id": "react-192-effectevent-introduction-20261003", "url": "https://react.dev/blog/2025/10/01/react-19-2", "type": "release_notes"},
    {"id": "react-193-effectevent-implementation-20261003", "url": "https://github.com/react/react/tree/1d34f91dfde6bba84d08b683aaba164c7194dacb", "type": "github_repository_analysis"},
    {"id": "react-versions-reference-2026-10-01", "url": "https://react.dev/versions", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/frontend.json"]
}
---

# useEffectEvent と再同期・時点の設計

## 問いと結論

接続・タイマー・DOM listener を毎回作り直さず、呼出し時の props / state を読むには、どの値を Effect の dependencies に残すべきか。`useEffectEvent` を使っても `memo` / `forwardRef` 内で古い Context を読む問題を、アプリの stale closure とどう切り分けるか。

**リソースを再同期させる値は Effect の依存に残し、そこから発生する通知の付随情報だけを Effect Event に移す。** さらに「呼出し時の最新値」と「元イベントを識別する値」を分ける。React 19.3 には wrapper component の更新反映に関する修正があるため、依存配列だけでなく採用版と wrapper を含む回帰試験を確認する。

本稿は公開契約、固定ソースでの観察、独自の設計・試験案を区別する。既存の [Activity と外部 store の再接続](activity-hidden-store-reconnect-boundary.md) が扱う購読停止・snapshot修復とは異なり、ここでは**既存の購読が保持する callback の参照先**を扱う。

## 版と修正の根拠

- [React 19.2 発表](https://react.dev/blog/2025/10/01/react-19-2#useeffectevent)（2025-10-01）が `useEffectEvent` の導入を記載する。19.3 の新設 API ではない。導入記事は Effect Event を理解する `eslint-plugin-react-hooks` への更新も求める。
- [React 19.3 changelog](https://react.dev/blog/2026/09/09/react-19-3#changelog)（2026-09-09）は、`forwardRef` / `memo` で最新値を読む修正を [#34831](https://github.com/react/react/pull/34831) として明記する。PR が main へ入った日と stable release に含まれた根拠を混同しない。
- GitHub の tag API で `v19.3.0` を `1d34f91dfde6bba84d08b683aaba164c7194dacb` に解決し、同じ SHA の実装・テストを読んだ。19.2 系全 patch の backport 有無は未調査であり、「すべての19.2で再現する」「初回修正版が必ず19.3」とまでは断定しない。

## 公開契約: latest committed と再同期を分ける

[`useEffectEvent` reference](https://react.dev/reference/react/useEffectEvent) は、返された関数が呼出し時の **latest committed values** を読むと説明する。進行中で未 commit の render の値まで公開するという意味ではない。また、値の変更そのものが Effect Event を実行するわけではない。

用途を次の3つに分ける。

| 値の役割 | 配置の判断 | 例 |
|---|---|---|
| 外部リソースを再作成・再同期する条件 | Effect 本体で読み、dependencies に残す | 接続先・部屋ID・タイマー間隔 |
| そのリソースから通知されたときにだけ使う最新の表示設定 | Effect Event 内で読む | 通知の theme・muted |
| 今回の出来事を識別する値 | 元の Effect / callback から引数で渡す | 遅延通知の対象ID・訪問したURL |

これは [公式の解説](https://react.dev/learn/separating-events-from-effects) の接続・訪問記録例を基にした分類である。実際に何が再同期条件かはアプリの契約次第であり、「props はすべて依存から外せる」という規則ではない。

### 呼出し場所と関数 identity

- Hook 自体は component / custom Hook のトップレベルで宣言する。返した Effect Event は、その場の `useEffect`・`useLayoutEffect`・`useInsertionEffect` や他の Effect Event から使う。Effect が設定した timer / 外部 listener からの呼出しも公式例にある。
- render 中の呼出し、JSX のクリック処理用 callback としての転用、他 component / Hook への Effect Event の受け渡しは公開契約外。一般のUI操作用関数が必要なら通常のイベントハンドラーを使う。
- 返り値の関数 identity は render ごとに変わる。Effect dependencies に含めると余計な再実行の原因になる。最新値を読めることと、参照が安定していることは別の性質である。

根拠は reference の Reference / Caveats / Troubleshooting。禁止をすべて実行時例外が検出すると期待せず、対応する Hooks lint を有効にする。

## 19.3.0 固定実装の観察

[`ReactFiberHooks.js` L2728–2779](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberHooks.js#L2728-L2779) では、mount 時に callback を持つ内部 ref を作る。update 時は同じ ref と次の callback を update queue の event payload に置き、返す wrapper 関数は改めて作る。wrapper は呼出し時に内部 ref の実装を呼ぶ。

[`ReactFiberCommitWork.js`](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberCommitWork.js#L507-L522) は、commit の処理で event payload の実装を反映する。`FunctionComponent` だけでなく `ForwardRef` / `SimpleMemoComponent` も対象に含まれる。元の [#34831 の固定 commit](https://github.com/react/react/commit/93d4458fdc054929e54fb25017d237ed85415533) が追加したのは、この component 種別の処理経路である。したがって、component が新しい Context で render できているのに、既に登録した Effect Event だけが古い値を読むケースを、単なる「memo が更新を止めた」と判断しない。

この SHA には `enableEffectEventMutationPhase` に応じた [別の commit 経路](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/ReactFiberCommitWork.js#L2087-L2109) もある。ここで必要な観察は「render 中に登録済み callback の実装を即時上書きせず、commit の処理で反映する」点まで。内部 flag や全 Effect 間の細かな順序をアプリの公開 API として利用しない。

### upstream のテストが確かめていること

同じ SHA の [`useEffectEvent-test.js`](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/packages/react-reconciler/src/__tests__/useEffectEvent-test.js) を確認した。

1. L947–983 は、Effect Event 自体を依存にした Effect が render 更新で再実行される例を試験する。テスト本文が明記するとおり、アプリ向け推奨コードではない。
2. L985–1032 は、一度だけ Effect で登録した関数が、再登録なしに commit 後の新しい値を読むことを確認する。この試験は通常の mount / update であり、全ての suspend / 中断経路を網羅する証拠にはしない。
3. L1191–1267 は、`React.memo` と `React.forwardRef` のそれぞれで Context を変更し、初回 Effect で保持した関数が新しい Context を読むことを確認する。見た目の再 render と登録済み callback の結果を別々に検査している。

本調査はソース閲読であり、ReactNoop の upstream 試験を実行した結果ではない。テスト内部が関数を外へ取り出す構造を、公開 API の受け渡し制約を緩める根拠にも使わない。

## 遅延処理: 「最新値」だけでは正しい記録にならない

[Separating Events from Effects](https://react.dev/learn/separating-events-from-effects#reading-latest-props-and-state-with-effect-events) は、訪問URLを `visitedUrl` 引数に、カート件数を Effect Event 内の読み取りにする例を示す。特に `setTimeout` で遅延すると、元の訪問URLと、実行時点のURLが異なることがある。

たとえばページAの訪問を予約したあとページBへ移動した場合、Aという出来事を記録するなら元のURLを引数として保持する。URLまで Effect Event の内部から読むと、通知がBの出来事に置き換わり得る。一方、送信時のカート件数を知りたい設計なら、その値を最新のものとして読むのは意図どおりである。

以下は独自の設計上の推奨。

- **記録する時点を項目ごとに決める。** 訪問時の件数も必要なら、URLだけでなく件数も元イベントの payload に含める。「常に最新」を全項目に適用しない。
- **キャンセル方針を別に設ける。** 全訪問を残す記録と、現在ページだけの一時通知では、画面変更後の遅延処理を残すかが違う。Effect Event は古い通信・timer を自動で取り消す機構ではない。
- **cleanup は取得したリソースに対応させる。** 部屋Aの接続を Effect が作ったなら、cleanup もその接続を閉じる。cleanup に必要なIDを無条件に最新値へ置き換えて部屋Bを操作しない。再同期条件と後始末を一組で設計する。
- **外部データの鮮度と混同しない。** latest committed は React の render 値についての契約。ネットワーク再取得、イベント順序、配送重複排除や全履歴の保存を提供しない。

## custom Hook への導入と移行手順（独自提案）

1. 既存 Effect が所有するリソースと、作り直す必要のある入力を列挙する。すでに正しい依存配列で問題なく動く Effect を一律に置換しない。
2. 通知の非 reactive な部分だけを切り出す。`roomId` や `delay` を隠して lint を黙らせる修正は採用しない。
3. custom Hook に抽出するなら、通常の callback を引数で受け、その Hook 内で Effect Event を宣言する。外側で作った Effect Event を渡さない。公式 `useInterval` 例でも `delay` は Effect の依存に残る。
4. 実際の `react` / renderer と Hooks lint の版・設定を lockfile で確認する。導入記事の `@latest` を再現可能な版指定の代用にしない。本稿は lint の最小対応 patch や全presetの互換性を検証していない。
5. 変更前後でリソース生成・cleanup 回数と callback が観測した値を記録する。表示が正しいだけでは、外部 listener の stale closure を検出できない。

## 採用側の受入試験案

以下は未実装・未実行の試験案である。検索 eval はこれらの動作を検証しない。

| 操作 | 判定したい結果 |
|---|---|
| theme / muted だけ変更して外部イベントを発火 | 接続は作り直さず、新しい通知設定で処理する |
| roomId / delay を変更 | 必要な cleanup と再設定が行われ、非 reactive 化で同期条件を失っていない |
| `memo` / `forwardRef` 内の Context を変更 | render 結果と、初回登録した listener の観測値の双方が新しい値になる |
| 遅延通知を待つ間に画面AからBへ移動 | 元イベントIDと実行時設定が、設計した時点の組合せになる |
| 中断可能な更新を commit 前に止め、既存 listener を発火 | 未 commit の候補値を見せない。再開・commit 後に新しい値へ変わることも確認する |
| cleanup / remount を反復 | listener / timer が増殖せず、解除対象が登録時のリソースと一致する |
| 禁止された渡し方を lint fixture に入れる | JSX / 他Hookへの Effect Event の受け渡しや依存配列への追加を CI が検出する |

## 適用限界・provenance

- 取得日は2026-10-03 UTC。live reference / guide に公開・更新日の表示は確認できない。[React Versions](https://react.dev/versions) は major 内で文書を更新し、minor / patch 別には公開しないと明記する。live 文書全体を19.3.0の固定ソースと同一視しない。
- `versions` catalog は今回も本文を開いて適用可能と確認し、無変更で再利用する。他の source は本稿で確認した内容を新規 immutable record に記録する。release notes の30日TTLに合わせ、期限は2026-11-02。
- 実ブラウザー、React Native、SSR / hydration、第三者の custom renderer、Compiler 出力、性能は未検証。Activity と Effect Event の細かな内部順序の差は対象外であり、wrapper 修正の確認だけで全 lifecycle の正しさを保証しない。
- GitHub PR は native web open が失敗したため GitHub connector で metadata / diff を確認し、tag・固定ファイルも connector で照合した。reference、guide、両release、versions、文書licenseの本文は native web で開いた。
- react.dev 本文は Meta Platforms, Inc. / contributors による CC-BY-4.0。[README](https://github.com/reactjs/react.dev/blob/main/README.md) と [LICENSE-DOCS.md](https://github.com/reactjs/react.dev/blob/main/LICENSE-DOCS.md) を確認した。実装とテストは固定 SHA の [LICENSE](https://github.com/react/react/blob/1d34f91dfde6bba84d08b683aaba164c7194dacb/LICENSE) および対象ファイルの header で MIT を確認した。
- 本稿は出典付きの独自要約・実装観察・設計提案。外部コードの転載・組込み、module昇格は行っていない。

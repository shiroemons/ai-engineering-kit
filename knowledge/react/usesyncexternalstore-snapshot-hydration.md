---
{
  "id": "react-usesyncexternalstore-snapshot-hydration",
  "title": "React useSyncExternalStore の snapshot キャッシュ・購読 identity・SSR hydration 契約",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 19.3 表示の react.dev live reference（major 19 内で更新、patch 固定なし、2026-10-01 確認） / Hook 導入は React 18.0（2022-03-29）",
  "tags": [
    "research-domain:frontend",
    "react",
    "useSyncExternalStore",
    "getSnapshot",
    "getServerSnapshot",
    "subscribe",
    "snapshot",
    "hydration",
    "SSR",
    "immutable"
  ],
  "sources": [
    {
      "id": "react-usesyncexternalstore-reference-2026-10-01",
      "url": "https://react.dev/reference/react/useSyncExternalStore",
      "type": "official_docs"
    },
    {
      "id": "react-hydrateroot-reference-2026-10-01",
      "url": "https://react.dev/reference/react-dom/client/hydrateRoot",
      "type": "official_docs"
    },
    {
      "id": "react-versions-reference-2026-10-01",
      "url": "https://react.dev/versions",
      "type": "official_docs"
    },
    {
      "id": "react-v18-external-store-release-2026-10-01",
      "url": "https://react.dev/blog/2022/03/29/react-v18",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/frontend.json"]
}
---

# React useSyncExternalStore の snapshot キャッシュ・購読 identity・SSR hydration 契約

## 問いと適用範囲

外部 store の変更を React に接続するとき、無限再描画、不要な再購読、初期 HTML の不一致をどう分けて防ぐか。
対象は既存の非 React store や browser API の adapter。通常の画面 state を外部 store に移す提案ではない。
以下の「確認した契約」は公式記載の要約、「設計判断」「試験案」はその契約からの独自提案で、実測結果ではない。

## 確認した契約（公式情報）

### 読み取りと通知を分ける

- `useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot?)` は現在の snapshot を返す。`subscribe(callback)` は変更通知を登録し、解除用の cleanup 関数を返す。
- 変更通知後に React は `getSnapshot` を読み直す。未変更なら同じ値を返し、前回と `Object.is` で異なるとき再描画する。
- snapshot は immutable である必要がある。mutable store では変更時に immutable snapshot を作り、未変更時は前回のキャッシュを返す。
- 毎回新しい object を返すと無限ループにつながる。関数自体を安定させるだけでなく、戻り値の同一性を守る必要がある。

根拠: [`useSyncExternalStore` の Parameters・Caveats・Troubleshooting](https://react.dev/reference/react/useSyncExternalStore)。

### subscribe identity は購読の境界

- 再描画で別の `subscribe` 関数を渡すと React は再購読する。依存しない関数はコンポーネント外へ出せる。
- 引数に依存する購読には `useCallback` を使い、その引数が変化したときに再購読する方法が公式に示される。

根拠: [My subscribe function gets called after every re-render](https://react.dev/reference/react/useSyncExternalStore#my-subscribe-function-gets-called-after-every-re-render)。

### SSR の初期データを hydration まで再現する

- `getServerSnapshot` はサーバーで HTML を作るときと、クライアントの hydration 中に使用される。
- サーバーと初回クライアントでは同じデータを返す必要がある。事前ロードした store 内容は serialize してクライアントへ渡す方法が示される。
- 第3引数を省いたコンポーネントのサーバー描画はエラーになる。意味のある初期値がなければ、意図した client-only 描画境界が必要になる。
- `hydrateRoot` はサーバーと同じ描画結果を要求する。不一致を自動修復できるとは限らず、属性差分の修復は保証されない。
- `suppressHydrationWarning` は1段だけの escape hatch で、対象の不一致テキストを React が修復するものではない。

根拠: [Adding support for server rendering](https://react.dev/reference/react/useSyncExternalStore#adding-support-for-server-rendering)、[`hydrateRoot` の Caveats・不一致抑制](https://react.dev/reference/react-dom/client/hydrateRoot)。

### Transition は外部 store の非同期化保証ではない

- Transition の DOM 反映前に snapshot が変わっていた場合、React は blocking update としてやり直し、画面内の store 版を揃える。
- 外部 store の変更は non-blocking Transition として扱えないため、この値から suspend する描画は公式が非推奨としている。

根拠: [`useSyncExternalStore` の Caveats](https://react.dev/reference/react/useSyncExternalStore#caveats)。

## 設計判断（独自提案）

1. adapter に「現在の読み取り」「変更通知」「初期 HTML 用の固定データ」の3責務を持たせ、画面は custom Hook を通して利用する。
2. immutable store は既存の安定した値を返す。mutable store は revision や更新イベントを境に snapshot を再生成し、更新検出方法を store ごとに明記する。
3. キャッシュには観測対象の全変更を反映する。参照だけを使い回して内部配列を書き換える方法では、過去の snapshot まで変化するので、変更した枝も切り離す。
4. selector で object を組み立てる場合も未変更時の同一性を検査する。毎回の全量 JSON 化・深いコピーを既定にせず、必要なフィールドと更新境界を先に決める。
5. `subscribe` が store instance や識別子を閉じ込める場合、それらを依存として扱う。空の依存配列で古い購読先を固定する「最適化」は避ける。
6. サーバー初期データと、hydration 後に変わり得る live store を概念上分ける。HTML 生成後に通知が到着しても初回 hydration が読む種データは上書きしない。
7. SSR では request ごとにデータの所有範囲を決め、別ユーザーの初期 snapshot を共有キャッシュへ混ぜない。転送形式と埋め込みの安全性は採用 framework の手順で別途確認する。
8. browser-only な値は、サーバーでも説明可能な初期表示を決めるか client-only にする。無関係な定数を入れて不一致警告だけを消す判断はしない。
9. 巨大な store 更新を `startTransition` で包めば応答性が保証されるとは考えず、snapshot 作成コストと影響する購読者を測定する。

## adapter の試験案（未実装）

- snapshot caching: store を変更せず2回読むと `Object.is` が真。観測対象を更新すると偽になり、その後の未変更読み取りは再び真になる。
- immutable 履歴: 以前に取得した snapshot を保持し、store を更新してもその内容が変わらないことを確認する。
- subscribe cleanup: 登録した callback に通知が届き、解除後には届かない。購読先の切り替え後には古い store のイベントが残らない。
- subscribe identity: 無関係な再描画で購読の作り直しを増やさず、意図した依存変更では切り替わる。生涯1回という呼出回数には固定しない。
- SSR hydration: サーバー snapshot を A とし、hydrate 前に live store が B になったケースでも初期 HTML と hydration のデータは A で一致し、その後 B を表示できるか確認する。
- hydration mismatch: 意図的に異なる初期データを渡し、警告・`onRecoverableError` 等の観測で発見できるか確認する。特定のエラー文面や発火回数は固定しない。
- request isolation: 同時に異なるユーザーの HTML を生成し、初期 payload と画面に他方の値が混入しないかを確認する。

## 適用版・限界・再確認条件

- [React Versions](https://react.dev/versions) と今回開いた reference は v19.3 表示。公式は minor・patch ごとの docs を公開せず major 内で更新するため、本文を特定 patch の実装保証として扱わない。
- [React v18.0 の投稿](https://react.dev/blog/2022/03/29/react-v18) は2022-03-29公開で、この Hook の導入と外部 store ライブラリ向けの用途を記載する。今回の調査は既存契約の整理であり、新機能発表ではない。
- reference と versions ページには公開日・更新日の明記を確認できない。取得日は2026-10-01 UTC。release notes の30日 TTL が最短のため、期限は2026-10-31とする。
- React 18 archive、React Native、store 製品固有の selector 最適化、streaming framework の具体的転送手順、実ランタイムでの挙動は未検証。
- hydration の「同じデータ」は転送後の内容を揃える契約として扱い、サーバーとブラウザーの object 参照を直接共有できるという意味には解釈しない。
- ライセンスは react.dev の [README](https://github.com/reactjs/react.dev/blob/main/README.md) と [LICENSE-DOCS.md](https://github.com/reactjs/react.dev/blob/main/LICENSE-DOCS.md) で CC-BY-4.0 を確認。著作者は Meta Platforms, Inc. と各 contributor。日本語で独自に要約し設計案を追加、コードは転載していない。
- 最初に確認した旧想定の `LICENSE` URL は404であり根拠に使っていない。上記の正規ファイルで確認を完了した。
- docs の major 表示変更、store 実装変更、SSR 経路追加の際は snapshot・購読・転送データの3契約を再検証する。

---
{
  "id": "react-browser-suspense-bailout-boundary",
  "title": "React 19.3 browser(): client-only SSR の Suspense 境界と onBrowserBailout の監視",
  "kind": "knowledge",
  "technology": "react",
  "version": "React 19.3 browser() introduced 2026-09-09; live major-19 browser/server references retrieved 2026-10-02, not patch-pinned",
  "tags": [
    "research-domain:frontend",
    "react",
    "browser",
    "client-only",
    "SSR",
    "Suspense",
    "onBrowserBailout",
    "onRecoverableError",
    "abort",
    "reason",
    "hydration"
  ],
  "sources": [
    {
      "id": "react-193-browser-release-2026-10-02",
      "url": "https://react.dev/blog/2026/09/09/react-19-3",
      "type": "release_notes"
    },
    {
      "id": "react-browser-bailout-reference-2026-10-02",
      "url": "https://react.dev/reference/react-dom/browser",
      "type": "official_docs"
    },
    {
      "id": "react-pipeable-browser-bailout-reference-2026-10-02",
      "url": "https://react.dev/reference/react-dom/server/renderToPipeableStream",
      "type": "official_docs"
    },
    {
      "id": "react-readable-browser-bailout-reference-2026-10-02",
      "url": "https://react.dev/reference/react-dom/server/renderToReadableStream",
      "type": "official_docs"
    },
    {
      "id": "react-versions-browser-reference-2026-10-02",
      "url": "https://react.dev/versions",
      "type": "official_docs"
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

# React 19.3 browser(): client-only SSR の Suspense 境界と onBrowserBailout の監視

## 問いと今回の変更

ブラウザー専用の部品を SSR から外すとき、どこまでを fallback にし、意図した描画の委譲と本当の障害をどう観測するか。
[React 19.3 の発表](https://react.dev/blog/2026/09/09/react-19-3#browser)（2026-09-09）は、Effect で mounted state を更新する方法や `typeof window` 分岐に代わる `browser()` を追加した。意味のある HTML をサーバーで生成できる部品は引き続き SSR し、localStorage や端末のタイムゾーンに依存する部分だけを対象にする用途である。

既存の外部 store 文書は `getServerSnapshot` の一致を扱う。本稿は SSR を意図的に中断してクライアントへ委ねる新 API と、その監視経路に限定する。「確認した契約」は公式記載の要約、「採用判断」「試験案」は独自提案で、実測結果ではない。

## 確認した契約（公式情報）

### use(browser()) と Suspense の境界

- `browser` は `react-dom`、`use` は `react` から読み込む。`use(browser())` はサーバー上の当該部品の描画を止め、最も近い祖先の `Suspense` fallback を HTML に残す。ブラウザーでは `undefined` を返し、通常の描画を続ける。
- `browser()` の単独呼出しにはこの効果がない。戻り値を `use` に渡し、`throw browser()` にはしない。
- 呼出し元は Client Component である必要があり、Server Component からは呼べない。Server Components を既定にする framework では `'use client'` のファイルか子の Client Component へ置く。
- SSR 中に祖先の `Suspense` がないと server render 自体が失敗する。
- `use` の規則に従い条件分岐内や early return 後にも置ける。初期データがあれば SSR を続け、なければ `use(browser())` へ進む形が公式例にある。

根拠: [`browser` Reference / Caveats](https://react.dev/reference/react-dom/browser#browser)、[React 19.3 の条件付き SSR 例](https://react.dev/blog/2026/09/09/react-19-3#browser)。

### bailout と障害の callback を分ける

- 正常に Suspense fallback へ委譲した場合は、サーバー renderer の `onError` と `hydrateRoot` の `onRecoverableError` を呼ばない。観測用には `onBrowserBailout` を使う。
- `onBrowserBailout(error, errorInfo)` は説明用の `Error` と `componentStack` を受け取る。`browser(reason)` の reason は `error.cause` に入る。reason 関数はサーバーが値に遭遇するたび評価され、ブラウザーでは実行されない。reason は HTML に serialize されない。
- `renderToPipeableStream` と `renderToReadableStream` はどちらもこの callback を持ち、未設定時には bailout に対して何もしない。
- Suspense がなく回復できないときは通常の error callback 経路になる。Node.js の shell 失敗には `onShellError` があり、Web Streams の shell 失敗では返却 Promise が reject する。

根拠: [`browser` の reporting](https://react.dev/reference/react-dom/browser#reporting-browser-only-rendering-on-the-server)、[`renderToPipeableStream` options](https://react.dev/reference/react-dom/server/renderToPipeableStream#parameters)、[`renderToReadableStream` options / returns](https://react.dev/reference/react-dom/server/renderToReadableStream#parameters)。

### 待機中の server render を browser へ委ねる

- Node.js の `renderToPipeableStream` は `abort` を返す。`abort(browser(reason))` とすると、未完了の Suspense 境界を fallback に残し、続きはブラウザーで描画する。
- この abort reason でも `onError` / `onRecoverableError` ではなく、回復した各 Suspense 境界を `onBrowserBailout` へ報告する。
- `AbortSignal` を受け取る server API では `AbortController.abort(browser(reason))` の形を使う。`renderToReadableStream` は options の `signal` を受け付ける。

根拠: [`browser` の abort 用法](https://react.dev/reference/react-dom/browser#aborting-pending-server-rendering-for-the-browser)、[`renderToPipeableStream` returns](https://react.dev/reference/react-dom/server/renderToPipeableStream#returns)、[`renderToReadableStream` signal](https://react.dev/reference/react-dom/server/renderToReadableStream#parameters)。

## 採用判断と落とし穴（独自提案）

1. まず初期 HTML に必要な情報を決める。サーバーが初期値を提供できるなら、その値で描画する経路を残す。警告を消す目的でページ全体を client-only にする判断は避ける。
2. 専用 API が必要な小さな部品の外側に Suspense を置く。fallback の文言・寸法・操作可能な周辺要素を決め、部品の準備待ちでページの主要情報まで消えない境界にする。
3. browser-only な読み取りは `use(browser())` より後の描画経路に置く。モジュール import 時点の browser API 参照まで、この描画中の呼出しが安全にすると想定しない。依存ライブラリの import 安全性は別に検証する。
4. 監視には「予定した client-only」「待機期限による委譲」「本当の描画障害」を別の分類で持つ。`onError` の件数が減っただけで SSR が改善したと判断せず、`onBrowserBailout` とクライアント側の表示完了を併せて見る。
5. `reason` は分類可能な短い理由にする。HTML に含まれなくても監視先へ送る情報になり得るため、個人情報や認証情報を入れない。関数を一度だけ実行される副作用の置き場にしない。
6. `abort(browser())` は待機予算を使い切ったときの明示的な運用方針として扱う。すべての例外をこの経路へ置き換えて、実障害の error callback を隠す運用は避ける。タイムアウト値はアプリ側で決め、公式例の値を性能保証と解釈しない。
7. `'use client'` の境界指定だけで SSR 不実行になると思わず、どの経路で `use(browser())` を評価するか確認する。framework が server renderer を包む場合、bailout callback と abort reason を利用できるか先に調べる。

## 採用時の試験案（未実装）

- **Suspense fallback**: browser-only 部品の初期 HTML に fallback があり、hydration 後に本体へ切り替わる。周辺の SSR コンテンツは初期表示に残るか。
- **missing Suspense**: 祖先境界を外す異常系で server render が失敗し、通常の error 経路に到達するか。正常な bailout と同じ扱いにしない。
- **conditional initialData**: データ有無の両方を試し、値がある経路だけを SSR できるか。値の有無と truthy / falsy を混同しない。
- **callback separation**: 正常な bailout で `onBrowserBailout`、本当の例外で通常の error callback を観測する。`onRecoverableError` だけを監視して完了としない。
- **reason cause**: `error.cause` と `componentStack` を受け取れるか、理由が HTML に漏れていないか確認する。reason 関数の単一実行回数を固定期待しない。
- **abort boundary**: 遅い Suspense 境界を複数用意し、`abort(browser())` 後の fallback と browser 描画、回復した境界の callback を確かめる。Node.js と Web Streams を採用する場合は各経路を別に試す。
- **client completion**: JavaScript が遅延・失敗した場合の fallback と、ブラウザーの本体描画で実例外が起きた場合を確認する。サーバーの委譲成功だけで利用者の表示成功を判定しない。

## 適用版・限界・再確認条件

- `browser()` の追加は React 19.3 の2026-09-09発表で確認。今回の取得日は2026-10-02 UTCで、reference は v19.3 表示だった。
- [React Versions](https://react.dev/versions) によれば docs は major 内で更新され、minor / patch 別には公開されない。reference の `reason`・callback・abort 詳細を特定 patch の実装検証結果とは扱わない。各 reference の公開日・更新日は明記を確認できなかった。
- React 19.2以前の実験版、framework 固有の SSR 設定、型定義パッケージ、実ランタイムの callback 回数・例外文面、SEO・性能効果、hydration 後のエラー復旧は未検証。取得した契約を越えて互換性や数値を保証しない。
- release notes の TTL 30日が最短のため明示期限は2026-11-01。React 更新、SSR runtime 変更、監視 callback の接続変更時に再確認する。
- 文書は Meta Platforms, Inc. と react.dev contributors の記載を日本語で独自要約し、独自の設計判断を追加した。コード転載なし。[README](https://github.com/reactjs/react.dev/blob/main/README.md) と [LICENSE-DOCS.md](https://github.com/reactjs/react.dev/blob/main/LICENSE-DOCS.md) で本文の CC-BY-4.0 を2026-10-02に確認した。

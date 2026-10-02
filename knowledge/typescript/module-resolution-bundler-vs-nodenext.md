---
{
  "id": "typescript-module-resolution-bundler-vs-nodenext",
  "title": "TypeScript module resolution の版移行: bundler/commonjs と nodenext の exports 境界",
  "kind": "knowledge",
  "technology": "typescript",
  "version": "TypeScript 6.0 announcement (2026-03-23) + v6.0.3 source at 050880ce59e30b356b686bd3144efe24f875ebc8; historical 4.7/5.0 release notes; live Handbook/TSConfig and Node.js v26.10.0 docs verified 2026-10-02; compiler runtime not tested",
  "tags": [
    "research-domain:frontend",
    "typescript",
    "module-resolution",
    "moduleResolution",
    "bundler",
    "nodenext",
    "node16",
    "package-exports",
    "package.json",
    "exports",
    "conditional-exports",
    "tsconfig",
    "esm",
    "commonjs",
    "customConditions",
    "typescript-6",
    "default-options",
    "hypothetical-emit",
    "commonjs-migration",
    "source-document-conflict"
  ],
  "sources": [
    {
      "id": "typescript-modules-reference-20261002",
      "url": "https://www.typescriptlang.org/docs/handbook/modules/reference.html",
      "type": "official_docs"
    },
    {
      "id": "typescript-tsconfig-module-options-20261002",
      "url": "https://www.typescriptlang.org/tsconfig/",
      "type": "official_docs"
    },
    {
      "id": "typescript-47-modules-release-20261002",
      "url": "https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-7.html",
      "type": "release_notes"
    },
    {
      "id": "typescript-50-bundler-release-20261002",
      "url": "https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html",
      "type": "release_notes"
    },
    {
      "id": "nodejs-26100-package-exports-20261002",
      "url": "https://nodejs.org/api/packages.html",
      "type": "official_docs"
    },
    {
      "id": "typescript-60-announcement-modules-20261002",
      "url": "https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/",
      "type": "release_notes"
    },
    {
      "id": "typescript-603-module-resolution-source-20261002",
      "url": "https://github.com/microsoft/TypeScript/tree/050880ce59e30b356b686bd3144efe24f875ebc8",
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

# TypeScript module resolution の版移行: bundler/commonjs と nodenext の exports 境界

## 問いと結論

`moduleResolution: bundler` と `nodenext` を、出力形式・package.json exports・依存パッケージの条件分岐まで含めてどう選ぶか。特に TypeScript 6.0 移行で、古い「bundler は esnext/preserve のみ」「commonjs なら node10 が既定」という説明をそのまま使えるかを扱う。

**6.0 は bundler + commonjs を追加している。6.0.3 のソースでは commonjs の既定解決も bundler である。** ただし許可される組み合わせと、実際の処理系に合う組み合わせは別問題になる。条件分岐はソースの `import` という見た目だけで判断せず、想定される emit と対象ファイルの形式を見る。[6.0 発表](https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/) / [6.0.3 ソース](https://github.com/microsoft/TypeScript/tree/050880ce59e30b356b686bd3144efe24f875ebc8)

## 確認した事実: 版を混ぜない

### 6.0 の追加とライブ文書の不整合

- 2026-03-23 の 6.0 発表は `module: commonjs` と `moduleResolution: bundler` の組み合わせを追加し、`moduleResolution: node` / `node10` を非推奨と説明する。Node.js 直接実行は nodenext、バンドラ利用は bundler への移行を案内している。6.0 の非推奨設定を `ignoreDeprecations: "6.0"` で一時的に抑止できても、7.0 での継続利用を保証しない。[6.0 発表](https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/)
- 取得した [Modules Reference](https://www.typescriptlang.org/docs/handbook/modules/reference.html) は更新日を 2026-09-28 と表示する一方、bundler の組み合わせを esnext/preserve に限定する文章が残る。この更新日だけを根拠に 6.0 の完全な契約とは扱わない。
- [TSConfig Reference](https://www.typescriptlang.org/tsconfig/) にも CommonJS → Node10、Node16/Node18/Node20 → Node16、NodeNext → NodeNext、Preserve → Bundler、その他 → Classic という旧既定値表が残る。これは下記の 6.0.3 実装と一致しない。ページ間の不整合を確認したのであり、ローカル compiler 実行で障害を再現したという意味ではない。

### 6.0.3 に固定した実装確認

公式 tag v6.0.3 の commit `050880ce59e30b356b686bd3144efe24f875ebc8` を使った静的分析である。将来版全体の保証ではない。

1. [`program.ts`](https://github.com/microsoft/TypeScript/blob/050880ce59e30b356b686bd3144efe24f875ebc8/src/compiler/program.ts#L4369) の検査と [`utilities.ts`](https://github.com/microsoft/TypeScript/blob/050880ce59e30b356b686bd3144efe24f875ebc8/src/compiler/utilities.ts#L9326) の判定から、bundler の組み合わせには preserve、commonjs、非 Node 系の ES2015〜ESNext module 値が含まれる。esnext/preserve だけという限定は不正確である。
2. [`utilities.ts` の computed options](https://github.com/microsoft/TypeScript/blob/050880ce59e30b356b686bd3144efe24f875ebc8/src/compiler/utilities.ts#L9077) は、`moduleResolution` の明示値を先に採用する。省略時は NodeNext → NodeNext、Node16/Node18/Node20 → Node16、None/AMD/UMD/System → Classic、それ以外 → Bundler。したがって commonjs、preserve、ES 系 module の既定は Bundler となる。非推奨の module 値を計算する分岐が残ることは、その設定の新規採用を勧める根拠にならない。
3. `module` 自体の省略値も同じファイルでは `target` から計算される。6.0 発表の「module の既定は esnext」という要約も、全 target に無条件に当てはめない。移行時は両方の値を明示し、使用版の実効設定を確認する。

## 条件分岐: 一致する条件の集合とキーの順番

以下は [Modules Reference](https://www.typescriptlang.org/docs/handbook/modules/reference.html)、[Node.js Packages](https://nodejs.org/api/packages.html)、および固定版の [`getConditions`](https://github.com/microsoft/TypeScript/blob/050880ce59e30b356b686bd3144efe24f875ebc8/src/compiler/moduleNameResolver.ts#L755) から整理した事実である。

- `types`、`node`、`import` / `require` は「この順番で必ず優先されるリスト」ではない。通常の型解決では TypeScript が一致可能な条件を用意し、実際の優先順位は package.json の conditional exports オブジェクトのキー挿入順で決まる。`default` は常に一致するフォールバックとして最後に置く。`types` 条件を先頭に置く指針は [4.7 release notes](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-7.html) でも確認できる。
- nodenext はファイル形式と想定 emit により import/require 側を選ぶ。CommonJS と判定される `.ts` の静的 `import` でも、想定出力が `require` なら require 条件になる。nodenext の ESM では `./foo.js` のような相対 import の完全拡張子が必要になる。
- bundler + preserve では ESM import と `import = require` をそれぞれの形式で扱う。`noEmit` でも hypothetical emit が解決に影響するため、型検査だけだから module は無関係とはいえない。
- 6.0.3 の upstream [`bundlerCommonJS.ts`](https://github.com/microsoft/TypeScript/blob/050880ce59e30b356b686bd3144efe24f875ebc8/tests/cases/conformance/moduleResolution/bundler/bundlerCommonJS.ts) と [trace baseline](https://github.com/microsoft/TypeScript/blob/050880ce59e30b356b686bd3144efe24f875ebc8/tests/baselines/reference/bundlerCommonJS.trace.json) では、bundler + commonjs の `.ts` import が require 専用 export を解決する。一方、同じ設定の `.mts` は import 条件になり、そのパッケージの解決に失敗する期待値になっている。これは公開テストの入力・期待値を読んだ結果であり、今回実行したテスト結果ではない。
- bundler は通常の組み込み条件として `node` を足さない。`customConditions` は既存の条件に追加する設定であり、並べた順番で export のキー順を上書きするものではない。`node16` / `nodenext` / `bundler` で使える。[TSConfig Reference](https://www.typescriptlang.org/tsconfig/)

## exports・拡張子・宣言ファイルの境界

- bundler は TypeScript 5.0 で導入された。Vite・esbuild・swc・Webpack・Parcel などの hybrid lookup 向けで、拡張子省略やディレクトリ解決と package.json imports/exports を扱う。`resolvePackageJsonExports` と `resolvePackageJsonImports` は node16/nodenext/bundler で既定 true。設定で無効化した場合まで同じ契約とは扱わない。[5.0 release notes](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html)
- node16/nodenext は 4.7 で導入された Node の dual-format を扱うモードである。`.mts` / `.cts`、`.d.mts` / `.d.cts`、package.json の `type` が形式判断に関わる。ESM 用と CJS 用の entrypoint には、それぞれの形式に対応する宣言ファイルを用意する。内容が同じという理由だけで一つの宣言に統合できるとは限らない。[4.7 release notes](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-7.html)
- exports が存在すると、公開キーやパターンで一致しない package subpath は解決対象から外れる。Node.js では `ERR_PACKAGE_PATH_NOT_EXPORTED` になる。これは package の公開境界であって、絶対パス等に対する強いセキュリティ境界ではない。exports は対応処理系で main より優先し、ターゲットには `./` 始まりやパストラバーサル禁止などの制約がある。[Node.js Packages](https://nodejs.org/api/packages.html)

## 推奨する選び方と移行手順

以下は検証済みの事実に基づく独自の設計推奨であり、全プロジェクトに必須の設定ではない。

1. **バンドラが生の TypeScript を読むアプリ**: bundler + preserve、またはバンドラの入力規則に合う esnext を起点にする。最終成果物が CommonJS という理由だけで TypeScript 側も commonjs に変えず、依存解決より前に実際に何へ変換されるかを確認する。
2. **先に CommonJS 化してからバンドラが解決する構成**: 6.0 の bundler + commonjs は候補になる。ただし `.mts` を含む場合や import/require 別 entrypoint の型が異なる依存を重点的に検証する。
3. **Node.js が直接読むアプリ・非バンドラ利用者に配布するライブラリ**: 実行先に合う Node 系 module / moduleResolution を使う。例えば両方を nodenext に揃える。npm 公開で bundler のみを使うと利用者側の解決失敗を隠しうるという 5.0 の警告を、無条件の bundler 禁止へ言い換えない。配布物を実際の利用方法で検証する。
4. **node10 からの移行**: compiler 版と tsconfig 継承元を固定してから module/moduleResolution を明示する。パッケージの root と subpath、ESM import と CJS require、ソースと公開 declaration を別々に確認する。TSConfig の `traceResolution` で選んだファイルと条件を記録し、ランタイムやバンドラが選ぶ実装と照合する。
5. **customConditions**: 開発用・ブラウザ用条件を追加したら、バンドラや実行時にも同じ意図の条件が成立するか確認する。型検査だけ別 entrypoint を読む構成を避ける。

### 避ける使い方

- 公式ページの更新日やナビゲーションの版表示だけを見て、本文の全オプション説明を 6.0 対応とみなす
- import という構文だけで import 条件を選ぶと考え、module・拡張子・package.json `type` を無視する
- 型解決の条件集合を優先順位リストと混同し、`default` を先頭に置いた exports をそのまま使う
- Node.js の ESM 実行を想定するのに bundler の拡張子省略がそのまま通ると期待する
- `ignoreDeprecations` を、非推奨設定の恒久互換性やランタイム互換性の保証として使う

## 適用版・証拠・未確認事項

- 取得日: **2026-10-02 UTC**。6.0 発表の公開日は 2026-03-23。Handbook と 4.7/5.0 ページは 2026-09-28 の更新表示を確認したが、4.7/5.0 は導入履歴としてのみ用いた。Node.js 文書の表示版は **v26.10.0**。ライブ文書は patch 固定ではない。
- 実装確認は **v6.0.3 / 050880ce59e30b356b686bd3144efe24f875ebc8** に固定。全称的な最新仕様の断言を避け、ページ記述との相違とソース読解を明示した。
- **実行していない**: TypeScript compiler のプローブ、upstream test suite、Vite/esbuild/Webpack 等のビルド、Node.js 実行、生成 declaration の利用者テスト。検索・metadata 検証が通っても compiler の振る舞いを証明するものではない。特定 runtime のエラー全文やすべての許容オプション組み合わせも実測していない。
- 調査途中の実装コミットと raw URL の web 取得には一部失敗した。最終的に固定した 6.0.3 のソース・baseline・LICENSE.txt は GitHub の読み取り API で取得した。公開ライセンスは固定 TypeScript repository の **Apache-2.0** を確認。その他のページの再利用ライセンスは未確認として catalog に `unknown` を記録し、本文は独自の要約のみ、コードの転載・module 昇格は行っていない。
- 明示期限は **2026-11-01**。参照する release_notes の TTL 30 日が最短となる。参照資料の該当節と文書全体を再点検して新しい catalog record を作成し、以前の record は変更していない。

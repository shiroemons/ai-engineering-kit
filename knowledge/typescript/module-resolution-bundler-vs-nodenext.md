---
{
  "id": "typescript-module-resolution-bundler-vs-nodenext",
  "title": "TypeScript の module resolution モード比較: `bundler` と `nodenext` の package.json `exports` 解決",
  "kind": "knowledge",
  "technology": "typescript",
  "version": "TypeScript 6.0 Handbook Modules Reference (last updated 2026-09-28) + TSConfig Reference moduleResolution + TS 4.7/5.0 release notes + Node.js v26.10.0 Modules: Packages; retrieved 2026-09-29",
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
    "customConditions"
  ],
  "sources": [
    {
      "id": "typescript-handbook-modules-reference-docs",
      "url": "https://www.typescriptlang.org/docs/handbook/modules/reference.html",
      "type": "official_docs"
    },
    {
      "id": "typescript-tsconfig-reference-docs",
      "url": "https://www.typescriptlang.org/tsconfig",
      "type": "official_docs"
    },
    {
      "id": "typescript-4-7-release-notes",
      "url": "https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-7.html",
      "type": "release_notes"
    },
    {
      "id": "typescript-5-0-release-notes",
      "url": "https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html",
      "type": "release_notes"
    },
    {
      "id": "nodejs-packages-docs",
      "url": "https://nodejs.org/api/packages.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-29",
  "expires_at": "2026-10-29",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# TypeScript の module resolution モード比較: `bundler` と `nodenext` の package.json `exports` 解決

`moduleResolution` の `bundler` と `nodenext` が package.json `exports` をどう解決するか、ペア制約・条件マッチ・拡張子要求の違いを公式一次情報で比較する。[Handbook Modules Reference](https://www.typescriptlang.org/docs/handbook/modules/reference.html) / [TSConfig Reference](https://www.typescriptlang.org/tsconfig) / [TS 4.7 Release Notes](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-7.html) / [TS 5.0 Release Notes](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html) / [Node.js Modules: Packages](https://nodejs.org/api/packages.html)

## 要点

以下は公式一次情報の記述であり、推奨構成そのものではない。

### 役割と `module` とのペア制約 (Handbook Modules Reference / TSConfig Reference)

- `moduleResolution` はモジュールスペシフィアの解決方法を制御する。公式は「ランタイム/バンドラに合わせて設定すべき」と記載する。
- `--moduleResolution node16|nodenext` は `--module node16|node18|node20|nodenext` とのペアが必須。
- `moduleResolution: bundler` は `--module esnext` または `preserve` とペア必須で、`--allowSyntheticDefaultImports` を示唆すると Handbook は記載する。
- TSConfig Reference の `moduleResolution` 既定値は `module` に依存する: `CommonJS` なら `node10`、`node16|node18|node20` なら `node16`、`nodenext` なら `nodenext`、`preserve` なら `bundler`、その他は `classic`。
- `node16`/`nodenext` は Node.js v12 以降の解決動作を反映する。TSConfig Reference では、この2モードが Node.js v12 以降の ESM/CJS 双方向アルゴリズムを、出力側の `import`/`require` に応じて選択すると説明されている。

### package.json `exports` の解決 (Handbook Modules Reference / TSConfig Reference)

- `bundler` と `nodenext` はどちらも、`resolvePackageJsonExports` が有効なら package.json `exports` を Node.js 仕様に従って解決する。`bundler` も `node16`/`nodenext` と同様に package.json `"imports"` / `"exports"` をサポートする (TSConfig Reference)。
- 条件マッチ: `types` と `default` は常にマッチする。追加条件は `customConditions` で指定する。`customConditions` は `node16`/`nodenext`/`bundler` でのみ有効 (Released 5.0、TSConfig Reference)。
- `nodenext` の条件の考慮順は、import 文脈で `types, node, import`、require 文脈で `types, node, require` (Handbook Modules Reference)。
- `bundler` は `types` に加えて、構文により `import` または `require` を使用する (Handbook Modules Reference)。つまり条件付き `exports` の `import`/`require` 分岐は、ソースが import 文か require 呼び出し文かで決まる。
- `exports` が存在すると、未列挙サブパスの解決は遮断される。Handbook も Node.js も同様に記載しており、Node.js では encapsulation と呼ばれ、未導出サブパスは `ERR_PACKAGE_PATH_NOT_EXPORTED` になる (Node.js Modules: Packages)。
- `resolvePackageJsonExports` / `resolvePackageJsonImports` は `node16`/`nodenext`/`bundler` で既定 `true` (TS 5.0 Release Notes)。

### 機能差: 拡張子省略とディレクトリ解決 (Handbook Modules Reference / TSConfig Reference / TS 4.7 Release Notes)

- 拡張子省略した相対パスとディレクトリ解決は、`bundler` では import 文脈・require 文脈とも可能。`nodenext` では import 文脈では不可、require 文脈でのみ可能 (Handbook Modules Reference)。
- TSConfig Reference も、`bundler` は Node.js 解決モードと異なり相対パスのファイル拡張子を「決して要求しない」と明記する。
- `nodenext` 側の背景として、TS 4.7 Release Notes は ESM では相対 import に完全な拡張子 (`./foo.js`) が必要と記載する。

### Node.js `exports` の仕様 (Node.js Modules: Packages)

- `"exports"` は `"main"` の現代的代替で、複数エントリと条件付き解決を提供する。`exports` と `main` が共存する場合は、対応バージョンの Node.js では `exports` が優先される。
- 条件オブジェクトではキー順が有意で、具体的→一般的の順に定義すべき。`import` と `require` は相互排他、`default` は常に最後。`"types"` 条件は typing system 用で、常に先頭に含めるべき (community conditions)。
- 導入履歴: subpath exports は v12.7.0、conditional exports は v13.2.0/v12.16.0、subpath patterns (`*` は文字列置換) は v14.13.0/v12.20.0 で追加。
- エクスポートターゲットは `./` 始まりの相対 URL で、パストラバーサルは禁止。

### 導入経緯 (TS 4.7 / TS 5.0 Release Notes)

- `module: node16` / `nodenext` は TS 4.7 で導入。package.json `"type"` と `.mts` / `.cts` / `.d.mts` / `.d.cts` 拡張子でモジュール形式を検出する。
- `exports` / `imports` / セルフレレンスは import conditions で解決され、import 文脈では `import` フィールド、CommonJS 文脈では `require` フィールドを参照する。型宣言には `"types"` 条件を追加でき、`"types"` は `"exports"` の先頭に置くべき。
- CJS エントリと ESM エントリは、それぞれ別の宣言ファイルが必要。
- `--moduleResolution bundler` は TS 5.0 で導入 (実装 PR #51669)。node16/nodenext は ESM の厳格な拡張子要求など制約が多いため、Vite・esbuild・swc・Webpack・Parcel 等のハイブリッド解決を行うバンドラ向けに新設された。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が定める実装構成そのものではない。

- バンドラだけで完結するアプリ (Vite / esbuild / Webpack 等) は `moduleResolution: "bundler"` を選び、ペア制約に従って `module` を `esnext` または `preserve` にする。
- Node.js で直接実行するコード、および npm 公開ライブラリは `moduleResolution: "nodenext"` + `module: "nodenext"` を選ぶ。TS 5.0 Release Notes が「npm 公開ライブラリでは `bundler` は非バンドラ利用ユーザーとの互換性問題を隠しうるため `node16`/`nodenext` の方が適切」と明記しているため、公式記載に基づく判断になる。
- `exports` は `"types"` 条件を先頭に置き、具体的→一般的の順、`default` は最後に並べる (Node.js / TS 4.7 の記載)。
- CJS と ESM の両方のエントリを公開するなら、それぞれ別の宣言ファイルを用意する (TS 4.7)。
- `nodenext` で ESM import を書くときは `./foo.js` のように完全な拡張子を付ける。拡張子省略が必要なリポジトリでは `bundler` 側を選ぶか、コード側を揃える。

```json
// アプリ (バンドラ完結) と Node.js/npm ライブラリの設定骨格
{
  "compilerOptions": {
    "moduleResolution": "bundler",
    "module": "esnext"
  }
}
```

```json
{
  "compilerOptions": {
    "moduleResolution": "nodenext",
    "module": "nodenext"
  }
}
```

## 避ける使い方

- **npm 公開ライブラリで `bundler` を使う**。TS 5.0 Release Notes が、非バンドラ利用ユーザーとの互換性問題を隠しうると明記している。
- **ペア制約に反する組み合わせ**。`nodenext` + `module: commonjs` や `bundler` + `module: node16` は公式のペア必須に反する。既定値は `module` から導出されるため、明示時に不整合を作らない。
- **`nodenext` で拡張子省略した相対 import を期待する**。ESM 文脈では不可で、require 文脈のみ可。ESM では `./foo.js` が必須 (TS 4.7)。
- **`exports` 追加後に未列挙サブパスが通ると思う**。`exports` は未列挙サブパスを遮断し、Node.js では `ERR_PACKAGE_PATH_NOT_EXPORTED` になる。
- **条件オブジェクトの順序を雑にする**。キー順が有意で、`default` を先頭に置くと具体条件が死ぬ。`types` は先頭、`import`/`require` は相互排他として扱う。
- **`customConditions` を `bundler`/`nodenext`/`node16` 以外で使う**。この3モードでしか有効でない (TSConfig Reference、Released 5.0)。

## 適用版と本番での注意

- Handbook Modules Reference のページ表示 `Last updated: 2026-09-28` (TypeScript 6.0 時点の記載)、TSConfig Reference は `moduleResolution` エントリ、Node.js は `v26.10.0` Documentation、release notes は TS 4.7 / TS 5.0 の各ページを 2026-09-29 に取得して確認した。将来の最新とは扱わない。
- 明示期限は 2026-10-29。根拠5件のうち TS 4.7 / TS 5.0 release notes は source type 上 `release_notes`（TTL 30日）で最短、`official_docs` 3件（Handbook・TSConfig・Node.js）は 2026-12-28 が相当。`typescript` は `config/freshness.json` に技術 TTL の定義がないため技術期限は適用されない。release notes は過去の出来事の記録だが、契約上は30日で再取得して内容を確認する。
- **未確認**: Vite / esbuild / Webpack / Parcel 各版の実際の解決実装と TypeScript 側の型解決の一致具合、`node10` モードの詳細仕様、特定 TypeScript バージョンでのエラー文言。本文は Handbook・TSConfig・release notes・Node.js 文書の記載のみを根拠にし、実測は含まない。
- 本節の設定選択方針 (バンドラ完結なら bundler、Node.js 実行と npm 公開は nodenext) は公式記載を根拠にした設計推奨であり、公式が定める必須構成そのものではない。

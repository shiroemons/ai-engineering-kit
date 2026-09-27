---
{
  "id": "typescript-discriminated-union-narrowing-exhaustive-never",
  "title": "TypeScript discriminated-union narrowing with exhaustive never checks under strict mode",
  "kind": "knowledge",
  "technology": "typescript",
  "version": "TypeScript 6.0 Handbook v2 Narrowing + Everyday Types (last updated Sep 22 2026) + TSConfig strictNullChecks reference; retrieved 2026-09-27",
  "tags": [
    "research-domain:frontend",
    "typescript",
    "discriminated-union",
    "narrowing",
    "never",
    "exhaustive",
    "exhaustiveness",
    "strictNullChecks",
    "kind",
    "switch",
    "literal-types"
  ],
  "sources": [
    {
      "id": "typescript-handbook-narrowing-docs",
      "url": "https://www.typescriptlang.org/docs/handbook/2/narrowing.html",
      "type": "official_docs"
    },
    {
      "id": "typescript-tsconfig-strictnullchecks-docs",
      "url": "https://www.typescriptlang.org/tsconfig/strictNullChecks.html",
      "type": "official_docs"
    },
    {
      "id": "typescript-handbook-everyday-types-docs",
      "url": "https://www.typescriptlang.org/docs/handbook/2/everyday-types.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# TypeScript discriminated-union narrowing with exhaustive never checks under strict mode

`kind` のような共通リテラル判別子で union を絞り込み、`never` への代入で未処理分岐をコンパイル時に検出する方法。[Handbook Narrowing](https://www.typescriptlang.org/docs/handbook/2/narrowing.html) / [TSConfig strictNullChecks](https://www.typescriptlang.org/tsconfig/strictNullChecks.html) / [Handbook Everyday Types](https://www.typescriptlang.org/docs/handbook/2/everyday-types.html)

## 要点

以下は公式一次情報の記述であり、推奨構成そのものではない。

### 弱い Shape 符号化では絞り込めない (Handbook Narrowing)

- 判別子を `kind: "circle" | "square"` の union リテラルにし、他欄を optional にした符号化では、`kind` の検査だけでは `radius` の有無は伝わらない。

```ts
interface Shape {
  kind: "circle" | "square";
  radius?: number;
  sideLength?: number;
}
```

- `if (shape.kind === "circle")` の内側でも `shape.radius` は `number | undefined` のまま残る。`strictNullChecks` 有効時は `Math.PI * shape.radius ** 2` が `'shape.radius' is possibly 'undefined'.18048` になる。
- `!` の non-null assertion (`shape.radius!`) で誤差は消えるが、公式は「コードを移動すると壊れやすい」とし、符号化の改善を促している。
- Everyday Types の記述では、`strictNullChecks` off の場合 null/undefined の値も通常どおり読め、あらゆる型のプロパティに代入できる。逆に on の場合は使用前に絞り込みが必要になる。これが `18048` が出る前提条件である。

### discriminated union の符号化と絞り込み (Handbook Narrowing)

- 各 union メンバに共通プロパティ `kind` を異なるリテラル型で持たせ、不足欄は required のまま分離する。公式はこの形を *discriminated union*、共通プロパティを *discriminant* と呼ぶ。

```ts
interface Circle {
  kind: "circle";
  radius: number;
}
interface Square {
  kind: "square";
  sideLength: number;
}
type Shape = Circle | Square;
```

- この符号化では `shape.radius` の直接参照が `Property 'radius' does not exist on type 'Shape'. Property 'radius' does not exist on type 'Square'.2339` になる。公式の説明では、optional 符号化の `18048` と異なり、この `2339` は `strictNullChecks` の設定にかかわらず発生する。
- `if (shape.kind === "circle")` または `switch (shape.kind)` の `"circle"` 分岐では `shape` が `Circle` に絞り込まれる。`"square"` 分岐では `Square` に絞り込まれる。`!` は不要になる。

```ts
function getArea(shape: Shape) {
  switch (shape.kind) {
    case "circle":
      return Math.PI * shape.radius ** 2; // (parameter) shape: Circle
    case "square":
      return shape.sideLength ** 2;       // (parameter) shape: Square
  }
}
```

### never と exhaustiveness checking (Handbook Narrowing)

- 絞り込みで union の選択肢がすべて除去された状態を表すのが `never` 型である。
- `never` はあらゆる型に代入できるが、`never` 自体以外から `never` への代入はできない。この性質を利用し、`switch` の `default` で残余を `never` に代入すると網羅性検査になる。

```ts
type Shape = Circle | Square;
function getArea(shape: Shape) {
  switch (shape.kind) {
    case "circle":
      return Math.PI * shape.radius ** 2;
    case "square":
      return shape.sideLength ** 2;
    default:
      const _exhaustiveCheck: never = shape;
      return _exhaustiveCheck;
  }
}
```

- 全分岐を処理済みなら `default` の代入は通る。新しいメンバを追加すると未処理分岐が残り、コンパイル誤差になる。

```ts
interface Triangle {
  kind: "triangle";
  sideLength: number;
}
type Shape = Circle | Square | Triangle;
// 上の getArea の default で:
// Type 'Triangle' is not assignable to type 'never'.2322
```

### Everyday Types の前提: union とリテラル推論 (Handbook Everyday Types)

- `string | number` のような union 値は、全メンバに有効な操作だけが絞り込みなしで許される。`typeof id === "string"` の分岐で `string` 側だけのメソッドが使えるようになる。
- リテラル型は `"circle"` のような特定値の型であり、union (`"left" | "right" | "center"`) と組み合わせて既知値集合を表す。`boolean` 自体が `true | false` の別名である。
- `const req = { url: "...", method: "GET" }` の `method` は `string` に拡大解釈されるため、`"GET" | "POST"` を要求する関数にはそのまま渡せない。対策は該当箇所への `as "GET"` の assertion か、全体への `as const` である。`as const` は全プロパティをリテラル型にする。
- `null` / `undefined` の `strictNullChecks` on/off の振る舞いは Everyday Types の `null and undefined` 節の記述であり、上記の `18048` / `2339` の違いと対で理解する。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が定める実装構成そのものではない。

- 排他的な状態やメッセージ種別は、optional 欄を並べた単一 interface ではなく、共通 `kind` リテラルを持つ interface の union で符号化する。各メンバ固有の欄は required にする。
- 分岐は `switch (shape.kind)` で全 `kind` を列挙し、各 `case` で絞り込まれた型だけを使う。`!` による沈黙化は使わない。
- `switch` の末尾に `default: const _exhaustiveCheck: never = shape` を置き、将来のメンバ追加を `2322` で検出する。`_exhaustiveCheck` を返すか throw するかは呼び出し側の契約で決める。
- `strictNullChecks: true` を前提にする。off では optional 符号化の欠落 (`18048`) が検出されなくなるため、網羅性検査の前提が崩れる。
- リテラル判別子を作るときは `as const` や明示リテラル型で拡大解釈を止め、`"rect"` のような綴り誤りを union 外比較の `2367` で検出できる形に保つ。

```ts
// 推奨する符号化と網羅性検査の骨格
type Shape = Circle | Square;

function getArea(shape: Shape): number {
  switch (shape.kind) {
    case "circle":
      return Math.PI * shape.radius ** 2;
    case "square":
      return shape.sideLength ** 2;
    default:
      const _exhaustiveCheck: never = shape;
      return _exhaustiveCheck;
  }
}
```

## 避ける使い方

- **optional 欄を並べた単一 `Shape` に `kind` 検査だけでアクセスする**。`radius` が `undefined` のまま残り、`strictNullChecks` 有効時は `18048` になる。`!` で消しても移動時に壊れやすいと公式が指摘している。
- **union メンバ外のプロパティを絞り込み前に読む**。`shape.radius` は `Square` に存在しないため `2339` になる。`kind` の分岐内で読む。
- **`default` の `never` 検査を省く**。`Triangle` のような追加メンバが黙って素通りする。`2322` で検出する構成を残す。
- **`strictNullChecks` off で網羅性を主張する**。off では null/undefined の読み取りが通常どおり通るため、`18048` 側の欠落検出が働かない。
- **判別子なしの `typeof` / truthiness だけで union を絞ったつもりになる**。公式の `printAll` の例にあるとおり、empty string のような falsy 値の扱いを誤る。判別子 `kind` の等価検査を使う。

## 適用版と本番での注意

- `Handbook v2 Narrowing` と `Handbook v2 Everyday Types` の各ページ表示 `Last updated: Sep 22, 2026`、`What's New` 一覧に `TypeScript 6.0` を含む構成を 2026-09-27 取得の内容で確認した。将来の最新とは扱わない。
- 本文は `expires_at` 2026-12-26 (official_docs TTL 90日)。Handbook 改訂時は `kind` 絞り込み・`never` 網羅性・誤差番号 (`18048` / `2339` / `2322` / `2367`) の記載を再確認する。
- **未確認**: 利用中の TypeScript compiler 版ごとの誤差文言の差、利用プロジェクトの `tsconfig.json` の `strict` / `strictNullChecks` 実設定値、bundler や test runner 側の型検査適用範囲。本文は Handbook と TSConfig 参照の契約のみを根拠にし、特定版の実測は含まない。
- 本文の符号化方針・`default` の扱い・`strictNullChecks: true` 前提は設計判断であり、公式 API 契約そのものではない。境界は各節の書き分けに従う。

---
{
  "id": "javascript-iterator-includes-consumption-close-boundary",
  "title": "Iterator.prototype.includes の Chrome 154 導入: skippedElements・消費・close 失敗の境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "Chrome 154 stable (2026-09-22); TC39 Stage 3 draft (2026-08-08) @ 63a7f4c95d84240f57a5cf79beca0eb2cd8911f3; V8 unflag snapshot @ deae9ab1cc95243bbc82663d2588caedd587a032 (2026-08-25); retrieved 2026-10-02 UTC",
  "tags": [
    "research-domain:frontend",
    "Iterator.prototype.includes",
    "skippedElements",
    "SameValueZero",
    "IteratorClose",
    "Chrome154",
    "Stage3",
    "consumption",
    "TypeError",
    "RangeError",
    "Infinity"
  ],
  "sources": [
    {
      "id": "chrome154-iterator-includes-release-20261002",
      "url": "https://developer.chrome.com/release-notes/154",
      "type": "release_notes"
    },
    {
      "id": "tc39-iterator-includes-draft-20261002",
      "url": "https://tc39.es/proposal-iterator-includes/",
      "type": "official_docs"
    },
    {
      "id": "tc39-iterator-includes-source-63a7f4c-20261002",
      "url": "https://github.com/tc39/proposal-iterator-includes/tree/63a7f4c95d84240f57a5cf79beca0eb2cd8911f3",
      "type": "github_repository_analysis"
    },
    {
      "id": "v8-iterator-includes-unflag-deae9ab-20261002",
      "url": "https://github.com/v8/v8/tree/deae9ab1cc95243bbc82663d2588caedd587a032",
      "type": "github_repository_analysis"
    },
    {
      "id": "ecma262-2026-iterator-abstract-20261002",
      "url": "https://tc39.es/ecma262/2026/multipage/abstract-operations.html",
      "type": "official_docs"
    },
    {
      "id": "ecma262-2026-array-includes-20261002",
      "url": "https://tc39.es/ecma262/2026/multipage/indexed-collections.html#sec-array.prototype.includes",
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

# Iterator.prototype.includes の Chrome 154 導入: skippedElements・消費・close 失敗の境界

## 問いと結論

配列を経由せず iterator の値を探すために `Iterator.prototype.includes()` へ置き換えると、入力検証と後始末はどう変わるか。結論は、**配列の非破壊的な照会ではなく、現在位置から iterator を消費する同期処理として扱う**ことである。第2引数と close の例外まで含めて移行条件を決める。

[Chrome 154 release notes](https://developer.chrome.com/release-notes/154) は 2026-09-22 の stable 導入を明記する。一方、[TC39 公開仕様](https://tc39.es/proposal-iterator-includes/) は取得時に Stage 3 Draft / 2026-08-08。Chrome の出荷、提案の段階、Firefox・Safari・Node.js 等の対応は別の事実であり、全 runtime 対応や完成済み ECMAScript 機能とは断定しない。

## 確認した契約

### 第2引数は fromIndex ではなく skippedElements

規範形は `includes(searchElement, skippedElements)`。`undefined` は 0 と扱い、他の値を Number に暗黙変換しない。`skippedElements` は呼出し時点から読み飛ばす個数であり、元データの絶対添字でも末尾基準の offset でもない。

| 第2引数 | 提案仕様の結果 |
|---|---|
| 省略、`undefined`、`0`、`-0` | 読み飛ばしなし |
| `1` など 0 以上 `Number.MAX_SAFE_INTEGER` 以下の整数 Number | その個数を読み飛ばしてから比較。途中で done なら false |
| `"1"`、`null`、`1n`、`NaN`、`1.5`、`-0.5`、数値化可能な object | `TypeError` |
| `-1`、`-Infinity`、有限の `Number.MAX_SAFE_INTEGER` 超 | `RangeError` |
| `Infinity` | 受理されるが、全要素を読み飛ばす。有限入力は最後まで消費して `false` |

引数不正時も iterator の `return` を調べて close する。検証は `GetIteratorDirect` による `next` の取得より前なので、無効な個数の呼出しを「何も起こらない入力エラー」とは扱えない。[固定 spec.emu](https://github.com/tc39/proposal-iterator-includes/blob/63a7f4c95d84240f57a5cf79beca0eb2cd8911f3/spec.emu)

対照的に [Array.prototype.includes](https://tc39.es/ecma262/2026/multipage/indexed-collections.html#sec-array.prototype.includes) の `fromIndex` は整数化され、負値は長さを使って末尾から計算する。空配列は第2引数の変換前に `false`、`Infinity` も探索せず `false` になる。したがって配列向けの offset をそのまま渡す置換は互換ではない。

### SameValueZero と消費

- 比較は `SameValueZero`。`NaN` 同士と `+0` / `-0` は一致し、Number と String は型変換して比較しない。object は同じ参照である必要があり、構造の深い比較ではない。
- `GetIteratorDirect` は `this.next` を直接使う。単なる iterable に対して `[Symbol.iterator]()` を自動実行する API ではない。
- 読み飛ばし中にも `next()` と値の取得が起こる。前に手動で読んだ要素は戻らず、結果が `false` になるまでに入力を尽くす。
- 正常な `done` なら `false`。一致すると close して `true` を返す経路へ入る。`IteratorClose` は `return` に通知する契約で、すべての custom iterator を強制的に使用不能にする機構ではない。`return` がない iterator もあるため、成功後の再利用可否を一般化しない。

根拠: [ECMAScript 2026 abstract operations](https://tc39.es/ecma262/2026/multipage/abstract-operations.html) の SameValueZero / SameValueNonNumber、GetIteratorDirect、IteratorStepValue と [提案仕様](https://tc39.es/proposal-iterator-includes/)。

### close の有無と、返るエラーは別に判断する

[IteratorClose](https://tc39.es/ecma262/2026/multipage/abstract-operations.html#sec-iteratorclose) と固定した [V8 includes 実装](https://github.com/v8/v8/blob/deae9ab1cc95243bbc82663d2588caedd587a032/src/builtins/iterator-helpers.tq) を照合した。

| 終了経路 | close と呼出し元の結果 |
|---|---|
| 値が一致 | `return` があれば呼ぶ。成功すれば `true`。getter / 呼出しが throw すればその例外、戻り値が object でなければ `TypeError` |
| skippedElements が不正 | close を試みるが、close 側の失敗より元の `TypeError` / `RangeError` を優先 |
| `next()` が throw | 例外を伝播し、この処理から追加の close はしない |
| 正常に `done` | `false`。追加で `return()` を呼ぶ終了方法ではない |

V8 は一致時に `IteratorClose`、引数不正時に `IteratorCloseOnException` を使い分ける。[同じ SHA の iterator.tq](https://github.com/v8/v8/blob/deae9ab1cc95243bbc82663d2588caedd587a032/src/builtins/iterator.tq) にこの例外優先順位が現れる。[mjsunit テスト](https://github.com/v8/v8/blob/deae9ab1cc95243bbc82663d2588caedd587a032/test/mjsunit/harmony/iterator-includes.js) も、引数検証で `next` getter を触らず close すること、`next` 失敗では `return` を触らないことを確認対象にしている。これはソース閲読であり、本調査でテストを実行したという意味ではない。

## 規範仕様と proposal repo の試作を取り違えない

2026-08-08 の固定 proposal SHA では、[src/index.ts](https://github.com/tc39/proposal-iterator-includes/blob/63a7f4c95d84240f57a5cf79beca0eb2cd8911f3/src/index.ts) が成功時にも `return()` の例外を握りつぶし、[test/index.mjs](https://github.com/tc39/proposal-iterator-includes/blob/63a7f4c95d84240f57a5cf79beca0eb2cd8911f3/test/index.mjs) はその状況で `true` を期待する。これは同じ commit の spec.emu が要求する `IteratorClose(..., NormalCompletion(true))` と異なる。

本書はここで **proposal 内の試作と仕様の不一致** を記録し、成功時の cleanup 失敗は規範手順と上記 V8 実装を根拠に扱う。試作をそのまま polyfill にしたり、そのテストを全実装の正解にしたりしない。取得した proposal の package.json に license 欄はなく、LICENSE は 404 だったため license は `unknown`。`copyright: false` は利用許諾と解釈せず、コードの転載・組込みはしない。

## 移行判断と確認項目

以下は公式契約から導いた独自の設計上の推奨である。

1. 既に配列があり、後でも同じ内容を読むなら配列の `includes()` を維持する。iterator 化のためだけに offset・例外・所有権を複雑にしない。
2. 消費を許せる同期 iterator に対し、SameValueZero の存在判定を一度行う用途で採用する。複数候補の確認に同じ iterator を繰り返し使わず、再生成可能な入力か有限の snapshot を設計する。
3. UI 入力から個数を受ける場合、有限・非負・安全な整数というアプリ独自の上限を呼出し前に検証する。API が受理する `Infinity` を「即時 false」や実行量制限として使わない。一致しない無限入力では終わらず、有限でも高コスト入力は長時間占有し得るため、処理件数の境界を別途設ける。
4. feature detection は `typeof globalThis.Iterator?.prototype?.includes === "function"` を最低条件にする。ただし関数の存在だけでは古い polyfill の引数検証・close 契約まで確認できない。
5. 有限の制御可能な iterator を使い、正常一致、欠損、NaN、型違い、負整数、分数、上限超、Infinity の全消費、close throw、非 object の close 結果、next throw を対象 runtime で確認する。検証時は `next` / `return` の回数と例外 identity も記録し、boolean だけのテストにしない。

`includes` は同期 iterator の契約であり、ここから async iterator や Promise を yield する入力の await 動作を導かない。cleanup を必要とする入力は「includes がどんな失敗でも自動解放する」とせず、所有者が終了経路を明示する。

## 根拠の範囲・未確認事項

- Chrome の導入根拠は 154 release notes（公開・更新 2026-09-22）、提案は Stage 3 Draft（2026-08-08）。V8 は [unflag commit](https://github.com/v8/v8/commit/deae9ab1cc95243bbc82663d2588caedd587a032)（2026-08-25 UTC）に固定して読んだ。V8 の任意の main SHA を Chrome 154 の各 patch binary と同一視していない。
- Firefox・Safari・Node.js の初回対応版、端末ごとの Chrome rollout、型定義・bundler・polyfill の対応は未確認。実ブラウザ・d8 の実行、性能測定、無限 iterator の実行はしていない。
- proposal と V8 の GitHub ページは web 取得に失敗したため、固定 SHA の本文を GitHub connector で取得して照合した。公開仕様・Chrome release notes・ECMAScript 2026 の各本文は web で開いた。ChromeStatus は本文が取得できず対応範囲の根拠に含めない。
- Google 文書の footer は本文 CC-BY-4.0 / sample Apache-2.0、V8 は当該ファイルの header と root LICENSE で BSD-3-Clause を確認。[ECMAScript 2026 の権利表示](https://tc39.es/ecma262/2026/multipage/copyright-and-software-license.html) も確認した。本書は独自要約であり、外部コードの複製はない。
- 取得日は 2026-10-02 UTC。release_notes の 30 日 TTL に合わせ期限を 2026-11-01 とする。検索 eval は本書の発見性を確認するもので、JavaScript の動作試験ではない。

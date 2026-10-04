---
{
  "id": "javascript-explicit-resource-lifetime-async-disposal-boundary",
  "title": "JavaScript explicit resource management: usingの寿命・非同期解放・完了待機の境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "Draft ECMA-262 / October 3, 2026 (ECMAScript 2027 display, not a published 2027 edition); TypeScript 5.2 introduction notes; original fixture on Node v24.19.0 / V8 13.6.233.17-node.51, verified 2026-10-04 UTC",
  "tags": [
    "research-domain:frontend",
    "explicit-resource-management",
    "using",
    "await-using",
    "Symbol.dispose",
    "Symbol.asyncDispose",
    "DisposableStack",
    "AsyncDisposableStack",
    "SuppressedError",
    "ownership",
    "cleanup",
    "lifetime"
  ],
  "sources": [
    {
      "id": "ecma2027-disposable-operations-20261004",
      "url": "https://tc39.es/ecma262/multipage/abstract-operations.html#sec-disposeresources",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-using-scope-20261004",
      "url": "https://tc39.es/ecma262/multipage/ecmascript-language-statements-and-declarations.html#sec-block-runtime-semantics-evaluation",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-disposable-stacks-20261004",
      "url": "https://tc39.es/ecma262/multipage/control-abstraction-objects.html#sec-disposablestack-objects",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-suppressed-error-fields-20261004",
      "url": "https://tc39.es/ecma262/multipage/fundamental-objects.html#sec-suppressederror",
      "type": "official_docs"
    },
    {
      "id": "typescript52-resource-management-introduction-20261004",
      "url": "https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html#using-declarations-and-explicit-resource-management",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/frontend.json"]
}
---

# JavaScript explicit resource management: usingの寿命・非同期解放・完了待機の境界

## 問いと結論

`using` を導入すれば、非同期の利用処理と後片付けが全部終わるまで資源を保持できるか。`AsyncDisposableStack.disposed` や、もう一度呼んだ `disposeAsync()` で終了を判定してよいか。

**資源の登録、利用処理の完了、解放処理の完了は別々に管理する。** `using` は登録したスコープの退出時に解放を実行する。`await using` が待つのは選択された非同期解放であり、初期化式や未 await の利用処理を自動的に待つ宣言ではない。さらに `Symbol.dispose` への fallback が返す Promise は待たれない。stack の `disposed` は受付終了を示す状態であり、非同期解放の成功証明ではない。

本稿は新しいリリースの紹介ではなく、未収録だった所有権と完了判定の設計判断を扱う。[fetch cancellation](fetch-abort-cancellation.md) の操作中断や [Iterator close](iterator-includes-consumption-close-boundary.md) の消費終了とは異なり、スコープに登録した資源をいつ誰が解放し、その失敗をどこで観測するかが主題である。

## 規範契約: 登録時に解放方法を決める

[ECMA-262 §7.5 AddDisposableResource / CreateDisposableResource / GetDisposeMethod](https://tc39.es/ecma262/multipage/abstract-operations.html#sec-adddisposableresource) と [§14.2 Block evaluation・§14.3 declarations](https://tc39.es/ecma262/multipage/ecmascript-language-statements-and-declarations.html#sec-block-runtime-semantics-evaluation) で確認した。

- `using` は同期解放の `Symbol.dispose` を選ぶ。`await using` は `Symbol.asyncDispose` を先に探し、それがない場合だけ `Symbol.dispose` を使う
- 解放関数は登録時に記録され、退出時に同じプロパティを引き直すわけではない。登録後にメソッドを差し替えると、既存登録と新規登録が異なる関数を保持し得る
- `null` / `undefined` は許可される。その他の値に必要な解放方法がない、または解放プロパティが呼び出せない場合は登録時に `TypeError` になり得る。既に登録済みの資源は、例外による退出でも解放対象になる
- スコープ退出時の `DisposeResources` は登録の逆順、すなわち LIFO で処理する。途中の解放が失敗しても残りの解放を試みる。非同期解放は順に待つため、複数の cleanup が勝手に並列化されるわけではない

これは GC によるオブジェクト回収の通知でも、外部 API の「閉じる」「取り消す」「保存する」を同じ意味へ統一する仕組みでもない。どの処理を解放メソッドに入れるかは、その資源の契約として別に定義する。

## await は取得・利用・解放の三か所で判断する

### 1. `await using` は取得 Promise を自動展開しない

取得関数が資源を解決値とする Promise を返すなら、宣言の右辺にも `await` が必要である。`await using resource = acquire()` と `await using resource = await acquire()` は異なる。通常の Promise 自体には解放プロトコルがないため、前者は資源を待つ前に `TypeError` となる。以下の右辺 `await` は取得完了、宣言側の `await using` は退出時の解放完了に対応する。

### 2. `return Promise` で利用中の資源を逃がさない

以下は本稿独自の使用例であり、ライブラリ実装の転載ではない。`acquire` は呼出側へ所有権を渡す `AsyncDisposable` を返し、`consume` は資源を使うすべての非同期処理を自身の Promise に含めることを前提とする。

```js
async function consumeOwned(acquire, consume) {
  await using resource = await acquire();
  return await consume(resource);
}
```

`return await` はここでは冗長な装飾ではない。`return consume(resource)` にすると、利用 Promise が完了する前に関数スコープから退出し、解放を開始し得る。`void consume(resource)`、timer、イベント callback へ参照を渡してから即座に退出する場合も、宣言だけでは利用期間を延ばせない。これは規範的なスコープ終了規則からの導出であり、後述の gate を使う Node fixture でも差を観測した。

上の形でも、`consume` が内部で fire-and-forget の処理を始めて早期に解決するなら不十分である。独自の設計判断として、利用完了を表す Promise の範囲を API 契約に含める。解放後も画面・接続・購読を使う必要があるなら、短いイベント handler のスコープから長寿命の owner へ所有権を移す。

### 3. sync fallback の Promise は待たない

[GetDisposeMethod の同期 fallback](https://tc39.es/ecma262/multipage/abstract-operations.html#sec-getdisposemethod) は、`Symbol.dispose` の呼出結果を解放用 Promise の解決値として引き継がず、`undefined` で解決する。従って `async [Symbol.dispose]()` のように同期プロトコルから Promise を返しても、`await using` に変えるだけで待機されるようにはならない。

非同期完了を待つ必要がある資源には `Symbol.asyncDispose` を実装する。既存の `closeAsync()` などを束ねるなら、`AsyncDisposableStack.defer(() => closeAsync())` のように非同期 callback の返り値を解放契約へ渡す設計を選ぶ。同期名のメソッドが内部で非同期処理を始めるだけの wrapper は避ける。

## 解放例外は元の失敗を消さずに読む

本文処理が `bodyError` を投げ、後から取得した B の解放が `bError`、先に取得した A の解放が `aError` を投げた場合、LIFO で B、A の両方を試みる。`DisposeResources` の規則から、最終値は概念的に次の入れ子になる。

```text
SuppressedError(
  error = aError,
  suppressed = SuppressedError(error = bError, suppressed = bodyError)
)
```

`error` は新たに発生した解放失敗、`suppressed` はそれまでの失敗である。本文が正常で解放が一つだけ失敗した場合は、その例外がそのまま伝わる。成功する予定だった `return` も解放失敗に置き換わり得る。

[SuppressedError constructor](https://tc39.es/ecma262/multipage/fundamental-objects.html#sec-suppressederror) の `error` と `suppressed` は non-enumerable である。オブジェクトの spread や単純な `JSON.stringify` だけに任せるログでは、重要な失敗枝が残らないことがある。独自の推奨として、通常の Error 情報に加え両フィールドを明示的に扱い、任意の throw 値、循環参照、深さ上限、秘密情報の除去を考慮したロガーで記録する。

解放失敗の捕捉は、本文中のネットワーク送信や保存を巻き戻す保証にはならない。`SuppressedError` が来たから処理全体を無条件に再実行するのではなく、業務処理と解放のどちらが完了したかを調べる。解放エラーを黙って捨てることも、元の body error だけを保存して終えることも避ける。

## stack の責務: 登録をまとめ、所有者を移す

[DisposableStack §27.3 / AsyncDisposableStack §27.4](https://tc39.es/ecma262/multipage/control-abstraction-objects.html#sec-disposablestack-objects) と [TypeScript 5.2 の導入説明](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html#using-declarations-and-explicit-resource-management) を照合した。

- `use(value)` は解放プロトコルを持つ値を登録する。`adopt(value, callback)` は値を引数に渡す独自 cleanup、`defer(callback)` は値を持たない cleanup の登録に使う
- 取得後すぐに登録すると、後続の取得や初期化で失敗しても先行資源を回収できる。必要な依存関係に合う逆順になるよう、取得と登録の順序を設計する
- `move()` は登録リストを新しい stack へ移し、元を空にして disposed にする。元のスコープの cleanup では移したリストを解放しない。元への追加や再度の move は `ReferenceError` となる
- stack 自身の再 dispose は処理を繰り返さない。しかし、同じ資源を別の stack にも登録した場合や、資源の生の `dispose` を手動でも呼んだ場合まで一回性を保証するものではない

独自の所有権判断として、`move` が移すのは cleanup の責任であり、資源オブジェクトへの別名参照を無効化する型システムではない、と扱う。移譲元が資源を使い続けたり、移譲先を失って誰も dispose しなかったりする問題は残る。組み立て成功時だけ `stack.move()` を返す factory は使えるが、返された stack を確実に終える新しい owner が必要である。

## disposed=true と二回目の disposeAsync は join ではない

[AsyncDisposableStack.prototype.disposeAsync](https://tc39.es/ecma262/multipage/control-abstraction-objects.html#sec-asyncdisposablestack.prototype.disposeasync) は最初に stack の状態を disposed にし、その後で `DisposeResources` に進む。従って、非同期 cleanup の待機中でも `disposed` は true になる。また disposed 状態での再呼出しは、最初の呼出しの完了 Promise を共有する操作ではない。

この規則から、次の判定は避ける。

- `stack.disposed === true` だけで、保存・切断・flush が完了したと判断する
- 別の場所ですでに開始された解放に合流するつもりで、もう一度 `await stack.disposeAsync()` を呼ぶ
- 解放の失敗後に `disposed` が true であることを根拠に成功を報告する

最初の解放を開始する唯一の owner が、その Promise を保存して共有する設計にする。次は独自の最小例で、作成した stack に対するすべての終了要求がこの関数を通ることを前提とする。

```js
function createCloseOnce(stack) {
  let closing;
  return function closeOnce() {
    return closing ??= Promise.resolve().then(() => stack.disposeAsync());
  };
}
```

`closing` の解決が成功、拒否が失敗を表す。失敗した Promise も保持するため、後続呼出しで失敗を成功へ置き換えない。microtask で解放を開始する前に `closing` を代入するので、cleanup が同期的に再入しても同じ Promise を返せる。直接 `closing ??= stack.disposeAsync()` とすると、右辺の cleanup が先に始まり、代入前の再入で別の Promise を返す窓ができる。

この例は解放開始をmicrotaskまで遅らせる設計である。cleanup自身が `closeOnce()` のPromiseを await / returnすると自己待機になるため禁止する。既に直接 `disposeAsync()` した stack を渡した場合や、別経路から直接閉じる場合を修復する wrapper ではない。

## 版と配備判断

[TC39 表紙](https://tc39.es/ecma262/multipage/) は **Draft ECMA-262 / October 3, 2026、ECMAScript 2027** と表示される。これは参照した live draft の表示版であり、2027年版の刊行や全ブラウザーの対応を意味しない。個別 engine の出荷版一覧は本稿では確定しない。

TypeScript 5.2 の release notes は `using` / `await using` と `Disposable` / `AsyncDisposable` の導入資料である。同ページは `target`、`lib`、必要に応じた runtime polyfill を別々に説明する。これを根拠に、型が通ることと配備先に必要な Symbol・stack・SuppressedError があることを分けて確認する。5.2当時の「新しい」「未対応が多い」という説明を2026年の対応状況へ読み替えない。ページの Last updated は2026-09-28だが、機能の導入日ではない。

独自の配備チェックとして、production build が残す構文、compiler helper と polyfill の組合せ、対象ブラウザーでの protocol 選択、拒否伝播を検査する。構文が未対応なら同じ bundle 内の feature detection より前に parse が失敗し得る。型定義の追加だけで構文や built-in を補えるとは扱わない。

## 実行観測と未確認事項

2026-10-04 UTC に、本稿用の独自 fixture を **Node v24.19.0 / V8 13.6.233.17-node.51、native ESM、polyfillなし** で実行した。gate を明示的に解放して順序を検査し、実時間の sleep に依存させていない。11ケースが通過した。

| 確認ケース | 観測した結果 |
|---|---|
| 登録後の解放メソッド差替え | 元の関数が呼ばれた |
| body失敗と二つの解放失敗 | B→A、外側aError・内側bError・元bodyError、非列挙フィールドを確認 |
| 二つ目の登録失敗 | 先行資源を解放してからTypeErrorを捕捉 |
| await using のsync fallback | scope-finishedが返されたPromiseの完了より先 |
| asyncDisposeの待機 | 解放完了後にscope-finished |
| Promiseをそのまま初期化値に渡す | TypeError |
| return Promise / return await | 前者だけ利用時に資源が既にdisposed |
| moveと重複dispose | 元では解放せず、移譲先で一回だけ実行 |
| 二回目のdisposeAsync | 初回cleanup待機中に解決、disposedもtrue |
| 非同期stackのLIFO | 後から登録したcleanup完了後に先行cleanup |
| closeOnceの同期再入と失敗 | microtask開始前にPromiseを保存し、再入でも同一Promise、解放拒否後も同じ拒否を保持 |

この観測は Node 一版の合成資源に限定される。ブラウザー実機、DOM・stream・外部接続との統合、TypeScript 5.2または現行 compiler の生成コード、polyfill、UI framework の lifecycle、process異常終了時の実行は未検証である。検索 eval は知識の検索到達性だけを検査し、この runtime fixture の代わりではない。

## 出典・provenance

- 5件の参照資料を2026-10-04 UTCに native web で開き、本文の算法・API説明を確認した。ECMA-262のlive draftはcommit固定のrepository解析ではないため、catalogのcommit_shaは空とした
- ECMA本文は [Ecma alternative copyright notice and copyright license](https://tc39.es/ecma262/multipage/copyright-and-software-license.html)、同文書内のsoftwareはBSD-3-Clause。本文は要点の独自要約であり、算法・サンプルを転載していない
- TypeScript文書の本文ライセンスは [TypeScript-Website LICENSE](https://github.com/microsoft/TypeScript-Website/blob/v2/LICENSE) のCC-BY-4.0を確認した。Microsoftの導入説明を出典として明記し、原文のコードはコピーしていない
- 刊行済み第17版の別URLは取得できなかったため、その版への収録時期や刊行日を本稿の根拠にしない。参照版を確認できたlive draftに限定した
- release notesを含むため再確認期限は30日後の **2026-11-03**。過去の導入事実がその日に変わるという意味ではない

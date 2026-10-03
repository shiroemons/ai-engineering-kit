---
{
  "id": "javascript-top-level-await-import-evaluation-boundary",
  "title": "Safari 27のtop-level await修正: 並行import・評価失敗・namespace初期化の境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "Safari 27.0 release article 2026-09-17; WebKit engineering article 2026-09-02; Draft ECMA-262 / 2026-10-03 (ECMAScript 2027); Node v24.19.0 Linux probes; Safari runtime untested",
  "tags": [
    "research-domain:frontend",
    "top-level-await",
    "dynamic-import",
    "module-evaluation",
    "Safari27",
    "TopLevelCapability",
    "Object.keys",
    "namespace",
    "TDZ",
    "retry"
  ],
  "sources": [
    {
      "id": "webkit-safari27-module-loader-release-20261003",
      "url": "https://webkit.org/blog/18325/webkit-features-for-safari-27-0/",
      "type": "release_notes"
    },
    {
      "id": "webkit-safari27-top-level-await-article-20261003",
      "url": "https://webkit.org/blog/18227/fixing-top-level-await-in-safari/",
      "type": "maintainer_article"
    },
    {
      "id": "ecma2027-dynamic-import-20261003",
      "url": "https://tc39.es/ecma262/multipage/ecmascript-language-expressions.html#sec-import-calls",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-module-evaluation-20261003",
      "url": "https://tc39.es/ecma262/multipage/ecmascript-language-scripts-and-modules.html#sec-cyclic-module-records-evaluate",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-namespace-binding-20261003",
      "url": "https://tc39.es/ecma262/multipage/ordinary-and-exotic-objects-behaviours.html#sec-module-namespace-exotic-objects",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-namespace-enumeration-20261003",
      "url": "https://tc39.es/ecma262/multipage/abstract-operations.html#sec-enumerableownproperties",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-namespace-object-keys-20261003",
      "url": "https://tc39.es/ecma262/multipage/fundamental-objects.html#sec-object.keys",
      "type": "official_docs"
    },
    {
      "id": "ecma2027-module-environment-20261003",
      "url": "https://tc39.es/ecma262/multipage/executable-code-and-execution-contexts.html#sec-module-environment-records-getbindingvalue-n-s",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Safari 27のtop-level await修正: 並行import・評価失敗・namespace初期化の境界

## 問いと採用判断

同じ機能を複数の画面から遅延ロードするとき、`await import()` の終了を初期化完了として扱えるか。通常の ECMAScript module では評価完了を待つ契約がある。Safari 27.0 は、この境界を破っていた旧 module loader の問題を修正した。ただし、循環依存の未初期化参照や、初期化処理そのものの失敗まで解消する修正ではない。

[Safari 27.0 の出荷記事][release] は2026-09-17に公開され、ESM loader の再実装と top-level await の順序・初期化不具合修正を記載している。2026-09-02の[技術記事][article]はまだ beta / Technology Preview 251 を案内しているため、その時点の説明を出荷済みの根拠に使わない。取得日は2026-10-03 UTC。本稿は新構文の紹介ではなく、既存アプリの loader workaround を外す際の判断を扱う。

## 修正対象は二回目以降の早すぎる完了

WebKit の技術記事では、同一 module を並行して dynamic import し、その module が top-level await で停止している間に、旧 loader が後続の import を先に完了させる例が示される。その namespace に対する `Object.keys()` で、まだ初期化されていない export に触れて例外が起きる。[article]

重要なのは「すべての import は呼出し順に完了する」という一般則ではなく、まだ評価中の module を評価済みとして公開しないこと。異なる依存グラフの完了順をこの例のログ順から決めない。本稿の回帰試験ではログの並びだけより、初期化ゲートが閉じている間の完了数と、解放後の export の利用可能性を主な判定点にする。

## 呼出しPromiseと評価Promiseを区別する

[ECMA-262 の Import Calls][import]では、`EvaluateImportCall` が呼出しごとに Promise を作る。`ContinueDynamicImport` は依存ロード、link、`Evaluate()` と進み、評価 Promise が fulfilled になった経路で namespace を使って呼出し側 Promise を resolve する。load/link/evaluate の失敗は対応する rejection 経路へ流れる。

一方、[Cyclic Module Record の Evaluate][evaluate] は `[[TopLevelCapability]]` の Promise を保存し、同じ評価単位への後続呼出しに再利用する。`InnerModuleEvaluation` も evaluating-async / evaluated の record を最初から実行し直さない。この記述は同一 Module Record と、その強連結成分を扱う契約であり、似た URL なら別 realm や別 host でもすべて同じ module になるという意味ではない。

したがって、本稿の設計上の判断は次の通り。

- `import()` の Promise identity が異なることを二重初期化の証拠にしない。評価回数や公開される値を観測する
- ready の判定にファイルの取得完了、namespace の存在、一定時間の sleep を使わない。評価を待つ処理の後から機能を利用する
- 同じ module を要求する利用者が増えても、未初期化の値を使ってよいとはしない。アプリ側に共通のロード窓口を設ける場合も、この契約を保つ

top-level await は待っている module とそれに依存する評価を遅らせるが、それに依存しない sibling まで一律に直列化しないというのが WebKit の説明である。[article] 起動画面の必須データか、操作後に取得すればよいデータかを依存関係で分ける。ネットワーク待ちを共通基盤の top-level に置くと影響する画面が増える、というのはこの性質からの設計上の推論である。

## 評価失敗後のimportは業務データの再試行ではない

[Evaluate と AsyncModuleExecutionRejected][evaluate] は評価エラーを record に残し、評価 Promise と待機中の親へ失敗を伝える。同じ Module Record を再利用する限り、再度 `import()` しても初期化本文を新しく実行する契約にはならない。ここで述べているのは評価エラーであり、取得失敗・解決失敗の再試行方針全般を同一視しない。

これを受けた本稿の推奨は、一時的な API 障害から再試行したい処理を、module の評価成功そのものから分離すること。module は操作関数を公開し、その呼出しが loading / success / error を返す構成なら、アプリの再試行回数や UI を設計しやすい。逆に、その初期化なしでは利用不能な固定依存には top-level await を使い、起動失敗を明示的に扱う余地がある。

無限に import を繰り返す、ReferenceError を握りつぶす、時間待ちで準備済みとみなす、という復旧方法は採らない。URL を変えて別ロードを狙う手法も、この修正の適用確認にはならない。URL 解決・module identity・再実行副作用を別途確認しないまま本番の復旧策にしない。

## Object.keysは初期化前でも安全な名前一覧ではない

namespace は通常のデータ object と同じ経路で読まれるとは限らない。次の仕様を合わせると、名前だけを知りたい処理でも binding の読み出しに到達することが分かる。

1. [Object.keys][keys] は `EnumerableOwnProperties` の key モードを呼ぶ
2. [EnumerableOwnProperties][enumeration] は各 String key の enumerable 判定に `[[GetOwnProperty]]` を使う
3. [module namespace の GetOwnProperty][namespace] は、その export の `[[Get]]` を呼んで descriptor の値を作る
4. [Module Environment Record の GetBindingValue][binding] は未初期化 binding に対して `ReferenceError` を投げる

これが、上記の不具合で Object.keys が例外を表面化させる仕様上の経路である。一方、[Object.getOwnPropertyNames][names] は `[[OwnPropertyKeys]]` から String key を取り出す。名前の一覧が取得できたことは、値の初期化完了を証明しない。

この違いは修正後にも意味がある。静的 import の循環依存の途中で namespace を読む状況と、完了した dynamic import の後で読む状況を区別する。前者の未初期化参照は依然として正当な TDZ エラーになり得る。Object.keys を別の列挙 API に変えて症状を消しても、その後に値を使う処理の依存順が正しくなったとは限らない。

## 最小の回帰試験と実測範囲

以下は本稿が作成した試験設計で、WebKit のサンプルコードや公式テストの転載ではない。

1. 初回評価回数を数える module を用意し、外から解放する Promise を top-level で待たせる。待機開始の通知を受けてから観測するため、任意の固定時間に依存しない
2. 同じ解決先を二回 import する。ゲート解放前はどちらも完了せず、評価開始が一回であることを確認する
3. ゲートを解放し、両方の export 値と namespace の同一性を確認する。後続の import でも評価回数が増えないことを確かめる。fixture には callable な `then` export など別の Promise 解決動作を招く要素を入れない
4. 別 module で評価時の await を拒否させる。二つの要求と後続の要求が失敗し、評価回数が増えないことを確認する
5. 静的な二 module の cycle を作り、一方の const 初期化前に他方から namespace を調べる。名前取得の成功と Object.keys の ReferenceError を別々に観測する
6. Safari の対象版と旧対応版、さらに実際の production build で上記を比較する。bundle 後に native ESM / top-level await が残るかを確認し、Node の成功で Safari の結果を代用しない

2026-10-03、Linux の既存 Node v24.19.0 で1〜5に相当する独自 fixture を実行した。二つの import Promise は別 object、ゲート解放前の完了数0、評価回数1、解放後の namespace は同一で export 値も一致した。評価拒否側は後続要求を含め同じエラーで失敗し評価回数1。cycle 側は名前取得に成功し、Object.keys は ReferenceError になった。これはこの fixture の観測であり、Safari 27 の実装検証や全循環グラフの正しさの証明ではない。

## 出典・ライセンス・未確認事項

- WebKit の二記事は実ページと公開日を確認した。サイトの [Licensing WebKit][webkit-license] はソフトウェアの LGPL / BSD を説明するが、記事本文へそのまま適用できる根拠を確認できなかったため catalog は unknown。記事コード・文章・画像は転載していない
- ECMA-262 は表紙に `Draft ECMA-262 / October 3, 2026`、名称に ECMAScript 2027 と表示された編集版を確認した。刊行済みの2027年版や Safari が草案の全機能に対応するという意味ではない。章ごとの実ページと [Copyright & Software License][ecma-license] を読み、本文の Ecma alternative copyright notice、ソフトウェアの BSD-3-Clause を区別した。仕様アルゴリズムをコードとしてコピーせず、独自の日本語要約と検証判断を記載した
- Safari 実機、WebKit の実装commit、旧版への backport、Bun、WKWebView、bundler ごとの生成物、module identity の host 別詳細、ネットワーク失敗からの回復は未検証。出荷記事だけで対象利用者全員へ修正が届いたとは判定しない
- 単一ページ版 ECMA-262 と記事からリンクされた GitHub PR は web 取得に失敗した。規範の確認には取得できた multipage 版を使い、未取得の PR 実装内容は根拠にしていない
- expires_at は release_notes の30日TTLに合わせ2026-11-02。次回は配布対象 Safari 版、既存 workaround の再現条件、production build の保持する module 境界を再確認する

[release]: https://webkit.org/blog/18325/webkit-features-for-safari-27-0/
[article]: https://webkit.org/blog/18227/fixing-top-level-await-in-safari/
[import]: https://tc39.es/ecma262/multipage/ecmascript-language-expressions.html#sec-import-calls
[evaluate]: https://tc39.es/ecma262/multipage/ecmascript-language-scripts-and-modules.html#sec-cyclic-module-records-evaluate
[namespace]: https://tc39.es/ecma262/multipage/ordinary-and-exotic-objects-behaviours.html#sec-module-namespace-exotic-objects
[enumeration]: https://tc39.es/ecma262/multipage/abstract-operations.html#sec-enumerableownproperties
[keys]: https://tc39.es/ecma262/multipage/fundamental-objects.html#sec-object.keys
[names]: https://tc39.es/ecma262/multipage/fundamental-objects.html#sec-object.getownpropertynames
[binding]: https://tc39.es/ecma262/multipage/executable-code-and-execution-contexts.html#sec-module-environment-records-getbindingvalue-n-s
[webkit-license]: https://webkit.org/licensing-webkit/
[ecma-license]: https://tc39.es/ecma262/multipage/copyright-and-software-license.html

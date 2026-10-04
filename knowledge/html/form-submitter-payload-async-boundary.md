---
{
  "id": "html-form-submitter-payload-async-boundary",
  "title": "HTML フォーム送信: submitter・FormData snapshot と非同期承認の境界",
  "kind": "knowledge",
  "technology": "html",
  "version": "HTML source 4c5586afa4fc6f2feeebcd25be1dca017cb51298 (2026-10-04); XHR source c3476b46f024cfc988f47d8d82118f3bcba562e9 (2026-08-18); live DOM/React v19.3/MDN checked 2026-10-04; browser runtime unverified",
  "tags": [
    "research-domain:frontend",
    "form",
    "requestSubmit",
    "submitter",
    "FormData",
    "formdata",
    "disabled",
    "snapshot",
    "async-validation",
    "implicit-submission",
    "reentrancy"
  ],
  "sources": [
    {
      "id": "whatwg-html-form-submit-20261004",
      "url": "https://html.spec.whatwg.org/multipage/forms.html#dom-form-requestsubmit",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-form-entry-list-20261004",
      "url": "https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#form-submission-algorithm",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-form-source-4c5586af-20261004",
      "url": "https://github.com/whatwg/html/blob/4c5586afa4fc6f2feeebcd25be1dca017cb51298/source",
      "type": "github_repository_analysis"
    },
    {
      "id": "whatwg-xhr-formdata-constructor-20261004",
      "url": "https://xhr.spec.whatwg.org/#dom-formdata",
      "type": "official_docs"
    },
    {
      "id": "whatwg-xhr-formdata-source-c3476b46-20261004",
      "url": "https://github.com/whatwg/xhr/blob/c3476b46f024cfc988f47d8d82118f3bcba562e9/xhr.bs",
      "type": "github_repository_analysis"
    },
    {
      "id": "whatwg-dom-form-dispatch-20261004",
      "url": "https://dom.spec.whatwg.org/#concept-event-dispatch",
      "type": "official_docs"
    },
    {
      "id": "mdn-requestsubmit-form-intent-20261004",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/HTMLFormElement/requestSubmit",
      "type": "official_docs"
    },
    {
      "id": "mdn-formdata-constructor-intent-20261004",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/FormData/FormData",
      "type": "official_docs"
    },
    {
      "id": "mdn-formdata-event-bubbling-discrepancy-20261004",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/HTMLFormElement/formdata_event",
      "type": "official_docs"
    },
    {
      "id": "mdn-submit-event-default-button-20261004",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/HTMLFormElement/submit_event",
      "type": "official_docs"
    },
    {
      "id": "react-form-state-dom-contract-20261004",
      "url": "https://react.dev/reference/react-dom/components/form",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# HTML フォーム送信: submitter・FormData snapshot と非同期承認の境界

## 問いと結論

「下書き保存」と「公開」が同じフォームにあり、二重送信防止と非同期確認を追加したとき、選ばれた操作・検証済みの値・実際の payload は一致するか。

**submitter、DOM の読取り時点、送信を続行する判断を別々に保持する。** ボタンを無効化してから payload を作ると操作名まで欠落し、`await` 後の `requestSubmit()` は最初の操作の再開にはならない。`formdata` は最終確認用の非同期フックでもない。

これは新 API の紹介ではなく、既存の HTML 送信契約の未整理だった実務的な空白を扱う。既存の [React フォーム検証](../react/form-validation.md) が扱うエラー表示や Actions の構成とは分け、native HTML の送信開始・データ採取・再入を対象とする。本文の「独自判断」は公式の追加要件ではない。

## 1. submitter を選ぶ契約

[HTML form methods][forms] と [MDN requestSubmit][request] で確認した。

| 開始方法 | 検証と submit | 操作ボタン |
|---|---|---|
| 利用者が submit ボタンを操作 | 通常は constraint validation 後に submit | `event.submitter` にそのボタン |
| `form.requestSubmit(button)` | 同じ送信算法へ入り、必要な検証後に submit | 指定ボタンの name/value と form* 属性を利用 |
| `form.requestSubmit()` / `requestSubmit(null)` | 検証後に submit | form 自身が内部 submitter。公開 `event.submitter` は null |
| `form.submit()` | interactive validation と submit event を省略 | ボタンを選ばず、ボタンの操作値も採らない |
| `new FormData(form, button)` | 検証・submit・通信を開始しない | データ採取に使うボタンだけ指定 |

`novalidate` / `formnovalidate` による検証省略、sandbox や非接続など送信算法の中止条件は別にある。`requestSubmit()` の返り値は undefined であり、成功・通信完了の証拠にはならない。`submit()` も entry list 構築を行うので、submit event がないことから formdata もないとは推論しない。

指定した引数が submit button でなければ TypeError、その form owner が対象 form でなければ NotFoundError。form 属性で外部から関連付いたボタンも owner が合えば対象となる。見た目の子孫関係だけで判定しない。これは [XHR の FormData 第2引数][xhr] にもある検査である。

### Enter と引数なし requestSubmit は同じではない

[implicit submission][submit-event] では、default button はその form を owner とする tree order 最初の submit button。ブラウザーが implicit submission を行う場合、有効な default button に click を発生させる。先頭が disabled だからといって次のボタンを選び直す契約ではない。Enter が常に implicit submission を起こすかは入力・端末の慣習に依存する。

一方、引数なし requestSubmit は default button を探さない。独自判断として、保存・公開の区別が必要な経路ではボタンを明示するか、null submitter の意味をアプリ側で定義する。null を無条件に「公開」と解釈しない。

## 2. FormData は現在の DOM の採取結果

[HTML entry list][entries] と [MDN constructor][constructor] が根拠である。フォームに関連付く submittable controls を tree order で調べ、disabled、未選択 checkbox/radio、submitter 以外の button などを除く。通常の入力には空でない name が必要。image input と form-associated custom element には別の構築規則があるため、「全要素が name/value の一組」とは一般化しない。

次の結果は契約からの導出であり、ブラウザー実測ではない。

- `new FormData(form)` はクリックされたボタンを自動発見しない。`new FormData(form, event.submitter)` と同じとは限らない
- `button.disabled = true` の後では、そのボタンを第2引数に指定しても通常の name/value は入らない。disabled を拒否する constructor 例外があるわけではない
- `fieldset.disabled = true` は子の採取にも影響する。ただし最初の legend の子孫には例外がある。fieldset を完全な入力ロックとみなさない
- `requestSubmit(disabledButton)` の開始検査も disabled 自体では拒否しない。click や implicit submission の無効化と混同しない。型と owner が適合し、他の中止条件がなければ submit は発生し得るが操作値は欠落し得る
- required が空でも FormData constructor 自体は validation をしない。採取できたことは妥当性の証明にならない
- 同名の複数値は残る。`getAll(name)` と、単一値を取る `get(name)` を用途で分ける。独自判断として、単一の intent を契約にするなら件数も検査する

独自判断として、自前送信では**同期的に payload を作り、その後に送信中 UI を無効化する**。native navigation を続行する場合、submit handler 内の無効化はその後の標準採取に影響するため、同じ手順を無条件には移植できない。

### React の推奨と DOM の採取条件

[React v19.3 の form reference][react] は、controlled input の値は state から読むように勧めている。この推奨は実際に記載されている。一方、HTML entry list の除外条件に controlled / uncontrolled の区別はなく、DOM 上にある named enabled control は通常どおり採取対象である。

独自判断として、アプリが正とする state と送信時 DOM snapshot のどちらを扱うかを先に決める。「controlled なので FormData には入らない」という技術制約に読み替えない。また、React の関数 action が持つ pending・Transition・uncontrolled field reset を、native addEventListener の async handler が自動で得るとは考えない。

## 3. formdata は同期的な加工点

固定 [HTML source][html-source] の送信算法は、通常の検証、cancelable な submit、entry list 構築、formdata、その後の送信方法処理という順序を持つ。submit を取消すと、その送信経路の自動的な entry list 構築へは進まない。ただし handler が自分で new FormData を呼べば、その constructor の formdata は発生する。

[MDN formdata][formdata] は constructor でも発生することと non-cancelable を説明する。したがって通知回数を送信回数・成功回数として数えない。加工 handler から fetch する設計では、単なる preview 用 FormData 作成でも送ってしまう危険がある。

**資料の不一致:** 2026-07-28 更新の同 MDN ページは bubble しないとするが、2026-10-04 に取得した live HTML と固定 source は bubbles=true を指定する。本稿は規範算法を根拠に扱う。実装間の比較結果は未確認であり、独自判断として加工 listener は当該 form に直接置き、委譲動作は対象環境で別途試す。

### 非同期変更と clone

固定 source は formdata の dispatch 後に entry list を clone して返す。[XHR 固定 source][xhr-source] はその list を constructor が作る FormData に設定する。よって event.formData と返された FormData は同じオブジェクトではなく、イベント中の同期加工は反映されても、dispatch と clone が終わった後の変更は返された list に反映されない。要素内 File の深い複製まで意味する記述ではない。

[DOM dispatch][dom] は listener の返した Promise の完了を待つ仕組みではない。特にネットワーク待ちの後で event.formData に承認 token を追加する方式に頼らない。同期加工だけを formdata へ置き、非同期処理は所有する payload に対して明示的に完了させる、というのが本稿の独自判断である。

### 再入ガード

固定 HTML / XHR 算法から次を導ける。

- 同じ form の formdata handler 内から new FormData(form) を呼ぶと、構築中 guard が null を返し、constructor が InvalidStateError を投げる。event.formData 自体を加工する
- 構築中の同じ form で requestSubmit や submit を呼んでも送信算法は早期終了する
- submit handler 中の同期 requestSubmit は firing submission events guard で終了する。これはアプリの二重送信防止の代替ではない
- 元の submit dispatch が終了した後（例: 後続 task で完了した通信待ち）には guard は解除され、新しい requestSubmit は新しい検証と submit を開始し得る。await を通過したことだけでは dispatch の終了を証明しない

## 4. 非同期確認を加えるときの独自設計

以下は上記契約に基づく設計案である。

1. 自前送信なら submit handler の最初に preventDefault を呼ぶ。await 後まで取消を遅らせない。後からの取消で既に選択された既定動作を巻き戻せるとは限らない
2. submitter の identity・intent と FormData snapshot を、無効化や await の前に採取する。formdata による同期加工後の payload も検査対象にする
3. 「採取時の内容を確認する」のか「送信直前の最新内容を確認する」のかを決める。前者は確認した snapshot を送る。後者は input revision と送信試行 ID を持ち、変更されたら再確認する
4. 初回 submitter を保持して requestSubmit(submitter) で再開する方式でも、DOM・name/value・form* 属性・owner の変更を検出する。要素参照を保存しただけでは意味は固定されない。消えたボタンを適当な別ボタンへ置換しない
5. native 再送に一回限りの通過フラグを使うなら、その同期 requestSubmit の呼出し単位で消費・解除する。validation failure で submit が発生しなかった場合にフラグが残る設計を避ける
6. fetch へ置換する場合、FormData は formaction / formmethod / formenctype / formtarget の実行を代行しない。異なる送信先や method を許すなら明示的に解決する。対応しない構成を黙って固定 endpoint へ送らない
7. in-flight 状態は UI の disabled と別に持つ。通信失敗後の再試行やサーバー側の重複排除は別契約。ブラウザーで一回だけ handler が動いても、業務処理が exactly-once とはならない

## 5. 独自の境界 fixture

次はネットワークを使わない JS fixture。空の通常 HTML ページの module script などで実行する。フォームを document に接続し、発生した submit は同期取消する。**ここでは構文検査だけを実施し、ブラウザーでの合格を報告していない。** 本番コードへそのまま組み込むための送信 wrapper ではない。

```js
async function probeFormBoundary() {
  const host = document.createElement("div");
  const form = document.createElement("form");
  const field = document.createElement("input");
  field.name = "title";
  field.required = true;
  field.value = "draft";
  const button = document.createElement("button");
  button.type = "submit";
  button.name = "intent";
  button.value = "publish";
  form.append(field, button);
  host.append(form);
  document.body.append(host);
  const results = [];
  const check = (name, actual, expected) => {
    if (!Object.is(actual, expected)) throw new Error(name);
    results.push(name);
  };
  const events = [];
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    events.push({
      submitter: event.submitter,
      data: new FormData(form, event.submitter),
    });
  });
  form.addEventListener("invalid", (event) => event.preventDefault(), true);
  try {
    check("omitted submitter", new FormData(form).has("intent"), false);
    check("explicit submitter", new FormData(form, button).get("intent"), "publish");
    button.disabled = true;
    check("disabled payload", new FormData(form, button).has("intent"), false);
    form.requestSubmit(button);
    check("disabled request event", events.length, 1);
    check("disabled request identity", events.at(-1).submitter, button);
    check("disabled request payload", events.at(-1).data.has("intent"), false);
    button.disabled = false;
    form.requestSubmit();
    check("no default lookup", events.at(-1).submitter, null);
    field.value = "";
    check("constructor skips validation", new FormData(form).get("title"), "");
    const before = events.length;
    form.requestSubmit(button);
    check("invalid blocks submit", events.length, before);
    field.value = "draft";
    let nestedError;
    let eventData;
    let eventBubbles;
    let eventCancelable;
    form.addEventListener("formdata", (event) => {
      eventData = event.formData;
      eventBubbles = event.bubbles;
      eventCancelable = event.cancelable;
      eventData.set("sync", "yes");
      try { new FormData(form); } catch (error) { nestedError = error.name; }
      queueMicrotask(() => eventData.set("late", "yes"));
    }, { once: true });
    const captured = new FormData(form);
    await Promise.resolve();
    check("nested construction", nestedError, "InvalidStateError");
    check("synchronous augmentation", captured.get("sync"), "yes");
    check("list clone", captured === eventData, false);
    check("late event mutation", eventData.get("late"), "yes");
    check("captured list unchanged", captured.has("late"), false);
    check("normative bubbles", eventBubbles, true);
    check("non-cancelable", eventCancelable, false);
    return results;
  } finally {
    host.remove();
  }
}
```

追加の受入ケースは、外部 form owner、別 form のボタンの NotFoundError、type=button の TypeError、disabled fieldset の legend 例外、formnovalidate、submit 中の再入、確認中のボタン交換・入力変更、同名 intent の重複、Enter の default button、sandbox 内の中止を分ける。native navigation とサーバー受領の試験は、この送信取消 fixture の合格とは別に行う。

## 確認範囲・版・利用条件

- HTML は 2026-10-04 表示の live 2ページを開き、source commit `4c5586afa4fc6f2feeebcd25be1dca017cb51298` の該当算法と照合した。commit は 2026-10-04T12:38:28Z。source は Git blob `1286cf2f87000e82ec573794caa8081de5c68917` を取得した
- XHR は 2026-08-18 表示の live §4 と、同日11:14:44Zの commit `c3476b46f024cfc988f47d8d82118f3bcba562e9` の xhr.bs を照合した。表示した日付は form 機能の導入日ではない
- 固定版の rendered commit snapshot URL は取得失敗したため、読めた live 本文と固定 raw source を使った。ブラウザー実装のソースや WPT の合格結果は解析していない
- MDN requestSubmit は 2025-06-23、FormData constructor は 2026-08-12、formdata event は 2026-07-28、submit event は 2026-09-02 更新表示。requestSubmit の September 2022、FormData 全体の July 2015 という Baseline 表示を、FormData 第2引数の全対応版の証明に流用しない
- Chromium CLI は `154.0.8037.57`。headless probe は起動時の socket permission エラーで失敗した。Node `v24.19.0` の `node --check` は JS 構文のみを検査し、DOM / form / validation を検証しない。Firefox、Safari、支援技術、React での動作比較は未確認
- WHATWG の [HTML LICENSE](https://github.com/whatwg/html/blob/4c5586afa4fc6f2feeebcd25be1dca017cb51298/LICENSE) と [XHR LICENSE](https://github.com/whatwg/xhr/blob/c3476b46f024cfc988f47d8d82118f3bcba562e9/LICENSE) は本文 CC-BY-4.0、source code に取り込む部分は BSD-3-Clause。MDN は Mozilla Contributors、本文 CC-BY-SA-2.5-or-later。React 文書は [LICENSE-DOCS.md](https://github.com/reactjs/react.dev/blob/main/LICENSE-DOCS.md) の CC-BY-4.0 を確認した
- 本文は出典付きの独自再構成、fixture は独自作成。転載・module 昇格なし。全 source の UTC 取得日は 2026-10-04。最短 TTL 90日で期限は 2027-01-02。検索 eval は到達性だけを検査し、フォーム挙動の検証には代えない

[forms]: https://html.spec.whatwg.org/multipage/forms.html#dom-form-requestsubmit
[entries]: https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#constructing-the-form-data-set
[html-source]: https://github.com/whatwg/html/blob/4c5586afa4fc6f2feeebcd25be1dca017cb51298/source#L64492-L65206
[xhr]: https://xhr.spec.whatwg.org/#dom-formdata
[xhr-source]: https://github.com/whatwg/xhr/blob/c3476b46f024cfc988f47d8d82118f3bcba562e9/xhr.bs
[dom]: https://dom.spec.whatwg.org/#concept-event-dispatch
[request]: https://developer.mozilla.org/en-US/docs/Web/API/HTMLFormElement/requestSubmit
[constructor]: https://developer.mozilla.org/en-US/docs/Web/API/FormData/FormData
[formdata]: https://developer.mozilla.org/en-US/docs/Web/API/HTMLFormElement/formdata_event
[submit-event]: https://developer.mozilla.org/en-US/docs/Web/API/HTMLFormElement/submit_event
[react]: https://react.dev/reference/react-dom/components/form

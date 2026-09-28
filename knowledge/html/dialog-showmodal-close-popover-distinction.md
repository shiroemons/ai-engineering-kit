---
{
  "id": "html-dialog-showmodal-close-popover-distinction",
  "title": "HTML dialog showModal close semantics and popover API distinction",
  "kind": "knowledge",
  "technology": "html",
  "version": "WHATWG HTML Living Standard 4.11.4 The dialog element + 6.12 The popover attribute (Last Updated 25 September 2026); retrieved 2026-09-28",
  "tags": [
    "research-domain:frontend",
    "dialog",
    "show",
    "showModal",
    "close",
    "requestClose",
    "returnValue",
    "closedby",
    "closerequest",
    "open",
    "top-layer",
    "backdrop",
    "inert",
    "cancel",
    "popover",
    "showPopover",
    "hidePopover",
    "togglePopover",
    "popovertarget",
    "popovertargetaction",
    "light-dismiss"
  ],
  "sources": [
    {
      "id": "whatwg-html-dialog-element",
      "url": "https://html.spec.whatwg.org/multipage/interactive-elements.html#the-dialog-element",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-popover-attribute",
      "url": "https://html.spec.whatwg.org/multipage/popover.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# HTML dialog showModal close semantics and popover API distinction

`dialog` の `show()` / `showModal()` / `close()` / `requestClose()` と `closedby` の契約を、`popover` 属性の `showPopover()` / `hidePopover()` / `togglePopover()` と区別して使う方法。[WHATWG HTML 4.11.4 The dialog element](https://html.spec.whatwg.org/multipage/interactive-elements.html#the-dialog-element) / [WHATWG HTML 6.12 The popover attribute](https://html.spec.whatwg.org/multipage/popover.html)

## 要点

以下は公式一次情報の記述であり、推奨構成そのものではない。

### dialog の表示方式 (HTML 4.11.4)

- `show()` は modeless で開く。`showModal()` は modal で開き、dialog を top-layer に置き、外側を inert にし、`::backdrop` で背面を装飾できる。
- いずれの表示も `open` 属性の付与を伴う。`open` の有無が表示状態の DOM 上の印であり、API 呼び出しと属性の手動操作は同じではない。
- `closedby` は `any` / `closerequest` / `none` を取る。`Auto` 指定時は `showModal()` で開けば `closerequest`、そうでなければ `none` として扱う。

### dialog の終了とイベント順序 (HTML 4.11.4)

- `close(returnValue)` は `returnValue` を設定して dialog を閉じる。
- `requestClose(returnValue)` は `returnValue` を設定した上で、cancel 可能 (cancelable) な `cancel` イベントを発火させる。`cancel` が prevent されなければ `close` イベントが続く。
- `open` 属性を手動で除去しても `close` イベントは発火しない。modal で開いた dialog の `open` を手動で外すと、画面からは消えても modal による blocking が残る。
- `returnValue` は `close()` / `requestClose()` に渡した値が保持される。dialog の結果 (どのボタンで確定したか等) を呼び出し側に返すための置き場所である。

### popover の表示方式 (HTML 6.12)

- `popover=auto` / `manual` / `hint` のいずれかを付けた任意の要素が popover になる。表示制御は `showPopover()` / `hidePopover()` / `togglePopover()` で行い、宣言的な起動には `popovertarget` と `popovertargetaction=toggle/show/hide` を使う。
- `auto` は他の `auto` popover を閉じ、light-dismiss (外側クリックなどによる暗黙の終了) と close-request (明示的な閉要求) に対応する。`manual` はいずれにも対応しない。
- `hint` は低優先の sibling stack として扱う。`auto` と `manual` の中間の軽い注釈用途であり、`auto` の排他スタックとは別に整理する。
- popover 妥当性検査 (check-popover-validity) は、対象が modal dialog である場合や、既に表示中なのに `showPopover()` する・既に非表示なのに `hidePopover()` する等の状態不一致の場合に `InvalidStateError` を投げる。

### dialog と popover の境界

- `dialog` の modal 管理 (top-layer + inert + `::backdrop` + `closedby`) と、`popover` の `auto` / `manual` / `hint` 管理 (排他スタック + light-dismiss + close-request) は別の仕組みである。`showModal()` した `dialog` を `showPopover()` の対象にしてはならない。
- `dialog` の終了は `close()` / `requestClose()` と `cancel` → `close` イベントで追い、`popover` の終了は `hidePopover()` / `togglePopover()` と `popovertargetaction` で追う。`returnValue` を持つのは `dialog` 側である。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が定める実装構成そのものではない。

- 確定・破棄の判断が要る操作 (削除確認、未保存変更の破棄確認) には `showModal()` + `requestClose(returnValue)` を使う。外側操作や Esc による終了を `cancel` で捕捉し、`close` 時に `returnValue` で分岐する。

```html
<dialog id="confirm" closedby="closerequest">
  <form method="dialog">
    <p>Delete this item?</p>
    <button value="cancel">Cancel</button>
    <button value="delete">Delete</button>
  </form>
</dialog>
<script>
  const dialog = document.getElementById("confirm");
  dialog.addEventListener("cancel", () => {
    // Esc や close 要求の入口。必要なら preventDefault() で継続する。
  });
  dialog.addEventListener("close", () => {
    if (dialog.returnValue === "delete") {
      // 削除を実行する。
    }
  });
  dialog.showModal();
  // 終了は dialog.close("delete") または dialog.requestClose("delete") で行う。
  // dialog.removeAttribute("open") では閉じない。
</script>
```

- 非 modal の補足表示 (ツールチップ的な注釈、入力補助の候補枠) には `popover=auto` または `hint` と `showPopover()` / `popovertarget` を使う。`dialog` の `showModal()` は持ち込まず、inert や `::backdrop` が要るかで選び分ける。
- `popovertargetaction` は既定 `toggle` のままにせず、開くボタンは `show`、閉じるボタンは `hide` と明示する。toggle の二重発火による表示・非表示の往復を避ける。
- `closedby` は既定 (`Auto`) に頼らず明示する。`showModal()` で close 要求を受け付ける設計なら `closerequest`、外側操作で閉じては困る確定 dialog なら `none` と書く。`any` は light-dismiss 相当の終了も許す指定であり、誤操作で消せないかを確認してから選ぶ。

## 避ける使い方

- **`dialog.removeAttribute("open")` で閉じたことにする**。`close` イベントが発火せず、modal の blocking が残る。終了は必ず `close()` または `requestClose()` で行う。
- **`requestClose()` を `close()` と同じ即時終了として扱う**。`requestClose()` は `cancel` を先に発火し、prevent されれば閉じない。未保存確認で止める設計なら `cancel` の購読を省かない。
- **modal の `dialog` を `showPopover()` / `popovertarget` の対象にする**。妥当性検査で `InvalidStateError` になる組み合わせであり、top-layer の modal 管理と popover の排他スタックを混ぜない。
- **`manual` popover に light-dismiss や close-request の自動終了を期待する**。`manual` はどちらにも対応しない仕様であり、外側クリックで閉じる導線が要るなら `auto` を選ぶ。
- **`returnValue` を popover 側に期待する**。結果返却の置き場所は `dialog` の契約であり、`hidePopover()` / `togglePopover()` に同等の値受け渡しはない。
- **状態を確認せず `showPopover()` / `hidePopover()` を繰り返す**。表示中への `showPopover()` や非表示への `hidePopover()` は `InvalidStateError` になる。呼び出し前に表示状態を確認するか、`togglePopover()` の往復条件を整理する。

## 適用版と本番での注意

- `WHATWG HTML Living Standard 4.11.4 The dialog element` と `6.12 The popover attribute` の `Last Updated 25 September 2026` 表示の内容を 2026-09-28 取得で確認した。将来の最新とは扱わない。
- 本文は `expires_at` 2026-12-27 (official_docs TTL 90日)。Living Standard は継続改訂のため、期限到来時に `closedby` の既定扱いと popover の妥当性検査条件の改訂を再確認する。
- **未確認**: 各ブラウザの `dialog` / `popover` / `closedby` / `popovertarget` 対応版の完全な対応表、スクリーンリーダーごとの modal の inert とフォーカス移動の実測、`::backdrop` の UA スタイル差。本文は WHATWG の契約のみを根拠にし、実機測定は含まない。
- 本文の使い分け (modal 確認に `dialog`、補足表示に `popover`) は設計判断であり、公式仕様の契約そのものではない。境界は各節の書き分けに従う。

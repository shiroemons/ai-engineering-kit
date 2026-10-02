---
{
  "id": "html-dialog-showmodal-close-popover-distinction",
  "title": "HTML dialog・popover の終了境界: requestClose の取消と Chrome 154 light dismiss",
  "kind": "knowledge",
  "technology": "html",
  "version": "WHATWG HTML source 0cd32204c6d9408be0a42cb15c86145e21deab9d (2026-10-02); Chrome 154 stable (2026-09-22); HTML PR #11536 open/unmerged at retrieval; browser runtime unverified",
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
    "light-dismiss",
    "beforetoggle",
    "no-op",
    "stale-result",
    "Chrome154",
    "click",
    "pointerdown",
    "pointerup",
    "reentrancy"
  ],
  "sources": [
    {
      "id": "whatwg-html-dialog-lifecycle-20261002",
      "url": "https://html.spec.whatwg.org/multipage/interactive-elements.html#the-dialog-element",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-popover-lifecycle-20261002",
      "url": "https://html.spec.whatwg.org/multipage/popover.html",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-dialog-popover-source-0cd32204-20261002",
      "url": "https://github.com/whatwg/html/tree/0cd32204c6d9408be0a42cb15c86145e21deab9d",
      "type": "github_repository_analysis"
    },
    {
      "id": "chrome154-dialog-popover-click-dismiss-20261002",
      "url": "https://developer.chrome.com/release-notes/154",
      "type": "release_notes"
    },
    {
      "id": "whatwg-html-click-dismiss-proposal-782a96da-20261002",
      "url": "https://github.com/josepharhar/html/tree/782a96da0106027d0778c7ae9def1da024362d22",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-10-16",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# HTML dialog・popover の終了境界: requestClose の取消と Chrome 154 light dismiss

## 問いと結論

閉じる要求を取り消したのに確認結果が更新されたと扱ってよいか。表示済みの popover をもう一度開くのは例外か。Chrome の light-dismiss 更新を全ブラウザの契約として扱えるか。

**要求、実際の終了、業務上の承認を分ける。** `requestClose(value)` を `cancel` で取り消せば、通常は `open` と公開 `returnValue` は変わらない。popover の通常の反復呼出しは no-op であり、全てを `InvalidStateError` と説明するのは誤りである。Chrome 154 の変更は出荷済み実装の説明であり、取得時に未統合の仕様提案とは区別する。

既存文書の `requestClose` 更新順序と popover 状態不一致の記述を訂正し、再オープン時に前回結果が残る境界を補った。基本的な訂正を Chrome 154 で初めて導入された機能とは扱わない。

## 根拠と適用範囲

- 規範: [HTML dialog](https://html.spec.whatwg.org/multipage/interactive-elements.html#the-dialog-element) と [popover](https://html.spec.whatwg.org/multipage/popover.html)。両ページの表示は Last Updated 2 October 2026。既存の knowledge 本文全体を再点検した。
- 固定根拠: [WHATWG HTML source](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/source)、commit `0cd32204c6d9408be0a42cb15c86145e21deab9d`、commit 日時 2026-10-02 15:54:17 UTC。dialog と popover の該当アルゴリズムを比較した。
- 出荷情報: [Chrome 154 release notes](https://developer.chrome.com/release-notes/154)、stable 日・ページ更新日ともに 2026-09-22。
- 提案: [WHATWG HTML PR #11536](https://github.com/whatwg/html/pull/11536) は 2025-08-05 作成。2026-10-02 21:35 UTC の GitHub API 確認では `state=open` / `merged=false`。解析 head は `782a96da0106027d0778c7ae9def1da024362d22` (2026-06-01) であり、main へ統合済みという意味ではない。

以下の「契約」は仕様読解、「設計判断」は独自の提案、「期待値」は未実行の試験仕様である。ブラウザ実測とは区別する。

## dialog の契約: cancel 前に結果を確定しない

### 表示方式と closedby

`show()` は modeless、`showModal()` は modal として開く。後者は top-layer と modal blocking を使い、外側を inert にし、`::backdrop` で背面を装飾できる。`open` は表示状態の印だが、属性操作だけで API のライフサイクルを代替できない。

`closedby` の `any` は外側操作と close request、`closerequest` は close request、`none` はユーザー操作による自動終了を制御する。属性省略・不正値は内部の Auto 状態となり、modal なら `closerequest`、そうでなければ `none` 相当である。Auto は独立した有効キーワードとして追加する値ではない。

**`closedby="none"` はプログラムによる終了禁止ではない。** `requestClose()` はこの属性を無視して close watcher を一時的に有効にする。`close()` も直接の終了 API であり、未保存ガードは `closedby` だけでは実装できない。[dialog の固定 source](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/source#L66566-L66730)

### requestClose(value) の値は内部の保留値

[request-to-close と close の固定手順](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/source#L67073-L67170) から、イベントハンドラが別の状態変更を起こさない通常経路は次になる。

1. 開いていて、接続済みかつ fully active な文書にあることを確認する。条件外では return する。
2. 引数を内部の `request close return value` に置く。この値は公開プロパティ `dialog.returnValue` と別である。
3. close watcher を通して `cancel` を発火する。`requestClose()` 経路の `cancel` は取消可能であり、`preventDefault()` すると終了へ進まない。
4. 終了が許可されたら close 手順に進み、`open` の除去、modal 状態の解除などを行う。非 null の結果があれば、この段階で公開 `returnValue` を更新する。
5. `close` イベントの発火は user interaction task source にキューされる。`requestClose()` から戻った時点の状態と、後で配送される通知を混同しない。

したがって、`cancel` 内で `dialog.returnValue` を読むと今回渡した値ではなく以前の値である。取り消した要求を承認済みとして記録してはいけない。ハンドラ自身が `returnValue` を代入したり `close()` を呼んだ場合は別の状態変更になるため、この保証の前提から外す。

### 取消後も内部の保留値は残る: native close request への持ち越し

公開値が不変であることは、内部の保留値が取り消し時に消えることを意味しない。固定 source では `requestClose("approved")` が先に内部値を保存し、[watcher の取消分岐](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/source#L89713-L89795) はその値をリセットせず return する。後で [dialog の closeAction](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/source#L66970-L67004) が走ると、その時点の内部値を close 手順へ渡す。内部値の null 化は実際の close 手順にある。

**source からの導出（実機未再現）:** 承認値を渡した要求を一度取り消し、その後新しい `requestClose()` で上書きせず、Esc などの native close request を受理すると、以前の内部保留値が公開 `returnValue` に反映され得る。`cancel` で止めた承認を、後の Esc によって承認済みへ変えてはならない。公開値を空文字にするだけでは、この内部値を消したことにはならない。

**設計判断:** 承認導線は独立した業務ガードを通った明示操作で `close("approved")` を呼び、`requestClose()` には未承認・取消用の値だけを渡す。全ての終了を requestClose に集約する場合も、承認可否は内部保留値に頼らず、今回の明示操作に結び付いた別の状態で判定する。下の独自例では、取消を経由する経路に承認値を渡さない。

### returnValue はセッションごとに自動初期化されない

| 操作 | 確認した通常経路 |
|---|---|
| `close("approved")` | `cancel` を通らず閉じ、結果を更新する |
| `requestClose("approved")` を取消 | 開いたままで、公開結果は以前の値のまま |
| 引数なしの `close()` / 許可された `requestClose()` | 結果を新しい文字列へ上書きしない |
| 次回の `show()` / `showModal()` | `returnValue` を空文字へ戻す処理はない |
| 閉じた dialog に `close(value)` | 冒頭の open 検査で return し、結果を更新しない |

このため、以前 `"approved"` を返した dialog を再利用し、次回は結果引数なしで閉じると、過去の承認が残り得る。**設計判断: 開く直前に `returnValue = ""` を設定し、今回の操作対象・表示回と結果を対応付ける。** `close` 通知の受信だけで削除や決済の承認が成立したとは扱わない。`close` は非同期通知なので、結果消費前に再オープンしない所有権も決める。

`open` 属性の手動除去も避ける。確認した仕様の注意書きでは `close` 通知がなく、modal で開いた文書の blocking が残り、通常の `close()` でも閉じられなくなる。単に非表示にすることと終了することは異なる。[HTML の注意書き](https://html.spec.whatwg.org/multipage/interactive-elements.html#note-dialog-remove-open-attribute)

## popover の契約: 状態不一致・不正状態・再入を区別する

### 通常の反復は no-op

[check popover validity](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/source#L93306-L93360) は「例外または boolean」を返す。状態不一致の `false` を `InvalidStateError` と読み替えない。

| 呼出し状況 | 固定 source の分岐 |
|---|---|
| 表示済みへ `showPopover()` を完了後もう一度呼ぶ | 状態不一致で false、呼出し元が return する no-op |
| 非表示へ `hidePopover()` を完了後もう一度呼ぶ | 同様に no-op |
| `popover` 属性がない対象 | 先に `NotSupportedError` |
| 開こうとする非接続要素、非 active 文書、modal dialog、fullscreen 要素 | 妥当性検査の該当分岐で `InvalidStateError` |
| `beforetoggle` 中など、同じ文書で別の popover を表示・非表示処理中に `showPopover()` | 表示側の再入ガードで `InvalidStateError` |

順序も契約の一部である。表示側の再入ガードは状態検査より先にあるので、**完了後の反復が no-op でも、処理中の再帰呼出しまで安全ではない。** `isConnected` や `:popover-open` を事前に確認するだけでは、イベントハンドラが途中で DOM を変更する経路を防げない。[show popover](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/source#L92497-L92579)

### 例外がなくても表示に成功したとは限らない

開くときの `beforetoggle` は取消可能であり、取消時は正常 return して非表示のままである。閉じるときの `beforetoggle` は取消可能にされていない。未保存確認を popover の閉じる `beforetoggle` で阻止する構成にはしない。

`showPopover()` / `hidePopover()` は成功 boolean を返さない。現在状態は `:popover-open` などで確認する。`togglePopover()` は処理後の開閉状態を返し、引数なしなら反転、`togglePopover(true)` / `togglePopover(false)` なら要求状態を指定できる。force 指定も入力妥当性検査や再入制限を免除しない。

`toggle` はキューされ、同じ要素の連続変更は集約される。呼出し回数と `toggle` イベント数の一対一対応を前提にして利用回数や承認操作を数えない。[show/hide と toggle の固定 source](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/source#L92740-L93043)

### auto は「常に一つだけ」ではない

- `auto` は light-dismiss と close request に対応し、開くと他の auto を閉じるが、有効な祖先 popover は残す。DOM の入れ子に加えて起動元の関係も祖先判定に使われる。
- `hint` も light-dismiss と close request に対応する。他の hint を整理し、auto を閉じないよう扱う。単なる manual と auto の中間ではなく、別の hint stack と祖先関係を持つ。
- `manual` は外側操作・close request による自動終了に対応せず、終了導線をアプリケーションが所有する。

宣言的な `popovertarget` と `popovertargetaction=toggle/show/hide` を使う場合も同じ区別が必要である。toggle ボタンには既定の `toggle` が自然であり、固定の「開く」「閉じる」導線では `show` / `hide` を選ぶ。全ての toggle を禁止する必要はない。[popover 属性と祖先規則](https://html.spec.whatwg.org/multipage/popover.html)

modal dialog の代わりに popover を使っても外側は自動的に modal inert にはならない。また、modal として開いた `dialog` を `showPopover()` で重ねて開けず、表示中の popover を `showModal()` することも不正状態になる。用途に応じて表示方式を一つ選ぶ。

## Chrome 154 light dismiss: 出荷説明と仕様統合を分ける

[Chrome 154](https://developer.chrome.com/release-notes/154) は、popover と dialog の light dismiss を `pointerdown` / `pointerup` の組み合わせによる判定から `click` ベースへ変更し、touch scroll と右クリックによる意図しない終了を防ぐと説明する。これは Chrome 154 stable の変更であり、全エンジンの同一実装を示す資料ではない。

一方、2026-10-02 の固定 HTML main はまだ pointer イベントを扱う。[PR #11536](https://github.com/whatwg/html/pull/11536) も取得時点で open / unmerged である。さらに [提案 head の source](https://github.com/josepharhar/html/blob/782a96da0106027d0778c7ae9def1da024362d22/source#L67084-L67159) は、click ベースへの統合でも押下・解放の target と座標を受け取り、押下先と解放先の dialog が一致するかを確認する。単純な `document.onclick` と包含判定だけへ置き換える実装モデルではない。

以下は独自の移行判断である。

- `closedby` と popover mode により誰が終了を管理するかを先に決め、独自の外側 `pointerup` ハンドラとネイティブ light dismiss を重ねない。
- ラッパーの終了理由判定を低レベル pointer event の発生順に結び付けない。dialog は `cancel` / `close`、popover は表示状態と toggle 通知を使い分ける。
- touch scroll、右クリック、内側から外側への drag、入れ子 popover、`::backdrop` を個別に回帰確認する。存在する API 名だけを調べても、この挙動差は検出できない。
- Chrome 154 の説明だけを根拠に Firefox・Safari の互換パッチを削除しない。合成 `dispatchEvent()` を実ユーザー入力の light-dismiss 検証と同一視しない。

## 独自例: 表示回ごとの結果を消費する

次は仕様の転載ではなく、結果初期化・内部保留値・取消の境界を示す独自例である。業務上の削除処理は実行せず、結果を表示する。`showModal` と `requestClose` がある対象ブラウザを前提とする。

```html
<button id="open-confirm" type="button">確認を開く</button>
<dialog id="confirm" closedby="closerequest" aria-labelledby="confirm-title">
  <h2 id="confirm-title">今回の操作を確認</h2>
  <label><input id="keep-open" type="checkbox">承認を保留し、取消可能な終了要求を拒否</label>
  <button id="approve" type="button">承認</button>
  <button id="dismiss" type="button" autofocus>戻る</button>
</dialog>
<p id="result" role="status"></p>
<script>
  const dialog = document.getElementById("confirm");
  const opener = document.getElementById("open-confirm");
  const keepOpen = document.getElementById("keep-open");
  const result = document.getElementById("result");

  opener.addEventListener("click", () => {
    if (dialog.open || opener.disabled) return;
    dialog.returnValue = "";
    keepOpen.checked = false;
    dialog.showModal();
    // close 通知を消費するまで、この導線では再オープンしない。
    opener.disabled = dialog.open;
  });
  dialog.addEventListener("cancel", (event) => {
    if (keepOpen.checked) event.preventDefault();
  });
  document.getElementById("approve").addEventListener("click", () => {
    // close は cancel を通らないので、承認用ガードをここで必ず検査する。
    if (keepOpen.checked) {
      result.textContent = "保留中のため承認しません";
      return;
    }
    dialog.close("approved");
  });
  document.getElementById("dismiss").addEventListener("click", () => {
    dialog.requestClose("cancelled");
  });
  dialog.addEventListener("close", () => {
    const value = dialog.returnValue;
    dialog.returnValue = "";
    result.textContent = value === "approved" ? "今回の承認を受信" : "今回は未承認";
    opener.disabled = false;
    opener.focus();
  });
</script>
```

実アプリでは表示対象 ID・未保存状態・二重実行防止を別に持つ。閉じた通知はサーバー処理の成功を意味しない。非同期の未保存確認が必要なら、`cancel` の同期ハンドラ内で先に `preventDefault()` し、その後に別の確認 UI を進める。Promise の完了だけで既に進んだ終了を取り消せるとは考えない。

## 試験すべき境界と未確認事項

次は source から導いた期待値であり、ブラウザでの合格報告ではない。

| 入力・状況 | 確認する期待値 |
|---|---|
| `returnValue="old"` で `requestClose("new")` を取消 | `cancel` 中も呼出し後も公開値は old、開いたまま、close 通知なし |
| 承認値の requestClose を取消後、保留解除して native close request | 固定 source では内部値の持ち越しがあり得る。独自例は承認値を requestClose に渡さない |
| `closedby="none"` で requestClose を許可 | cancel を経由して閉じる。none をアプリ終了の禁止と誤解しない |
| 承認後に再表示し、引数なしで終了 | 無初期化なら過去結果が残る。独自例は今回は未承認と表示する |
| showPopover を完了後に反復、hidePopover を反復 | 通常の no-op。重複イベントを期待しない |
| popover 属性なし・非接続・modal と popover の混在 | 属性なしの NotSupportedError と InvalidStateError を区別する |
| 開く beforetoggle を取消 | 例外なしでも表示されない |
| beforetoggle 中から別の showPopover | 固定 main の再入ガードを確認する |
| auto popover の子を開く | 有効な祖先は開いたまま |
| 連続 show/hide と toggle 計数 | 集約を許容し、呼出し一回につき通知一回を要求しない |
| 外側 click・touch scroll・右クリック・drag | Chrome 154 の説明と実機を照合し、他エンジンは別に測る |

取得時の Chromium CLI 表示は `151.0.7922.173`。headless 起動は socket 制限で失敗し、cloud browser でのローカル `file://` 試験ページも URL policy により開けなかったため、実行を中止した。上の HTML 例、Chrome 154 実機、Firefox・Safari、touch 入力、スクリーンリーダー、フォーカス復元と UA style の実測は未確認である。構文・metadata・検索検証はブラウザ挙動の検証を代替しない。

固定 commit の rendered snapshot は取得できず、live の公式2ページと固定 raw source を照合した。raw source は Git blob SHA `05319af4659e15aafb3f5137362b7d9561db8812`、提案 head は `dc59a2184845c9ca9f83de30014d628a29d07e1a` とそれぞれ一致することを確認した。PR 状態は取得時点の記録であり、公開時・適用時には再確認する。期限は未統合提案の追跡を考慮して 2026-10-16 とした。

## 出典の利用条件

[固定 WHATWG LICENSE](https://github.com/whatwg/html/blob/0cd32204c6d9408be0a42cb15c86145e21deab9d/LICENSE) の権利者は WHATWG (Apple, Google, Mozilla, Microsoft)。仕様本文は CC-BY-4.0、ソースコードへ組み込まれる部分は BSD-3-Clause である。旧 catalog の CC0 表記を今回の根拠として再利用せず、新しい記録を追加した。Chrome ページの本文は CC-BY-4.0、例示コードは Apache-2.0。PR 会話の再利用ライセンスは未確認で、状態と提案内容を独自に要約した。本文は出典付きの日本語再構成と独自設計であり、仕様や実装コードの転載・module 昇格は行っていない。

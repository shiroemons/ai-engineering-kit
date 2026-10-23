---
{
  "id": "javascript-beforeinput-ime-edit-ownership-boundary",
  "title": "beforeinput: IME・synthetic event・Reactをまたぐ編集責務の境界",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "Input Events Level 2 WD 2026-10-15 compared with WD 2025-08-14; UI Events WD 2026-02-21; React docs v19.3; verified 2026-10-23 UTC; browser and IME runtime unverified",
  "tags": [
    "research-domain:frontend",
    "beforeinput",
    "input",
    "IME",
    "composition",
    "cancelable",
    "inputType",
    "targetRanges",
    "dispatchEvent",
    "React",
    "editing ownership"
  ],
  "sources": [
    {
      "id": "w3c-inputevents2-20261015-edit-intent-20261023",
      "url": "https://www.w3.org/TR/2026/WD-input-events-2-20261015/",
      "type": "official_docs"
    },
    {
      "id": "w3c-inputevents2-20250814-edit-baseline-20261023",
      "url": "https://www.w3.org/TR/2025/WD-input-events-2-20250814/",
      "type": "official_docs"
    },
    {
      "id": "w3c-uievents20260221-composition-order-20261023",
      "url": "https://www.w3.org/TR/2026/WD-uievents-20260221/",
      "type": "official_docs"
    },
    {
      "id": "mdn-beforeinput-missing-cancelability-20261023",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/Element/beforeinput_event",
      "type": "official_docs"
    },
    {
      "id": "mdn-preventdefault-cancelable-passive-20261023",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/Event/preventDefault",
      "type": "official_docs"
    },
    {
      "id": "mdn-dispatchevent-result-not-edit-20261023",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/EventTarget/dispatchEvent",
      "type": "official_docs"
    },
    {
      "id": "mdn-input-programmatic-value-boundary-20261023",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/Element/input_event",
      "type": "official_docs"
    },
    {
      "id": "react193-common-beforeinput-abstraction-20261023",
      "url": "https://react.dev/reference/react-dom/components/common",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2027-01-21",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# beforeinputだけを編集処理の正本にしない

## 問いと適用範囲

入力マスク、候補補完、リッチテキストeditorで、`beforeinput` を捕捉して独自modelを書き換えれば、IME変換・自動補完・React経由の入力まで一貫して管理できるか。

**結論として採用する独自設計は、意図の受信、取消し、実際の変更、modelへの反映を別の段階として扱うこと。** 一つの編集をbrowserとapplicationの両方が適用しない。取消不能や通知の欠落も想定した経路を持ち、event配送の成功を編集成功の代用にしない。

対象はtext input、textarea、contenteditableの編集eventであり、フォームのsubmitter/payload、HTML sanitization、IME実装、共同編集algorithm全体ではない。既存の[フォーム送信文書](../html/form-submitter-payload-async-boundary.md)が扱う送信境界とは別問題である。

## 確認した版と証拠の強さ

- Input Events Level 2は[2026-10-15 Working Draft](https://www.w3.org/TR/2026/WD-input-events-2-20261015/)を確認した。最終勧告ではなく、文書自身もtest suiteとimplementation reportを作成中としている。
- [2025-08-14 Working Draft](https://www.w3.org/TR/2025/WD-input-events-2-20250814/)を比較用に読んだ。古い記述を現在の実装状況とみなさない。
- UI Eventsは[2026-02-21 Working Draft](https://www.w3.org/TR/2026/WD-uievents-20260221/)。以下ではInput Eventsとの相違を残し、都合のよい単一順序へ統合しない。
- Reactは取得時の公式referenceに表示されたv19.3。特定patchの実装sourceを分析したわけではなく、過去版・Canaryを含む一律の動作保証ではない。
- 取得日はすべて2026-10-23 UTC。規範の記述、referenceの注意、独自の運用提案、実測結果を分ける。実ブラウザー試験は後述の理由で未実行。

## 1. 取消しを試みたことと、編集を止めたことは違う

[2025年版 §6.1.2](https://www.w3.org/TR/2025/WD-input-events-2-20250814/#interface-InputEvent)の表では、`insertText` は取消可能だが、IMEの現在の変換文字列を置き換える `insertCompositionText` は取消不能である。同じ表に名前が載っていても、全browserが全inputTypeを実装する保証ではない。

[MDN beforeinput](https://developer.mozilla.org/en-US/docs/Web/API/Element/beforeinput_event)は、autocomplete、spell correction、password manager、IMEなどで、beforeinput自体が発生しない場合や取消不能の場合があると注意する。具体的な条件はbrowser/OSに依存する。必要に応じてinput側で変更を取り込み、beforeinputで処理できなかった変更へ対処する設計が必要となる。

[preventDefaultの契約](https://developer.mozilla.org/en-US/docs/Web/API/Event/preventDefault)では、`cancelable=false` のeventやpassive listener内での呼出しは取消しを成立させない。

以上からの独自の判断規則:

1. `inputType` の文字列だけで「取消し済み」と判断しない。受け取ったeventの `cancelable` と、自分が取消しを行えるlistener設定を確認する。
2. applicationが所有する編集を選ぶなら、同期handler内で取消しを成立させてからmodel operationを一度だけ適用する。`await` の後で検証結果が出ることを、元の編集を止める仕組みにしない。
3. 取消不能なら、独自operationを先行適用してbrowserにも同じ編集をさせない。browser側の変更を観測し、差分または変更後の値としてmodelへ反映する。
4. composition中の仮の文字列を毎回「不正」として差し戻す設計は避ける。入力中のdraftと、保存・送信時に受理する値を分離し、確定後の検証結果を利用者へ示す。

4はIME仕様の義務ではなく、本番入力を壊しにくくする設計案である。入力制限、Undo、selection保持まで含め、対象言語とIMEで確認する。

## 2. compositionupdateの前後順を、両仕様共通の保証にしない

取得した二つの一次資料には、次の**文書間不一致**がある。

| 資料 | 記載されているcomposition中の並び |
|---|---|
| Input Events Level 2 2026-10-15 §7 | compositionupdate → beforeinput → DOM更新 → input |
| UI Events 2026-02-21 §3.6.6 | beforeinput → compositionupdate → DOM更新 → input |

[Input Events §7](https://www.w3.org/TR/2026/WD-input-events-2-20261015/#input-event-order-during-composition)、[UI Events §3.6.6](https://www.w3.org/TR/2026/WD-uievents-20260221/#events-composition-input-events)を照合した差であり、browserで観測した差ではない。どちらかに反するbrowser一覧や、この不一致が解消したという結論は得ていない。

[UI EventsのisComposing](https://www.w3.org/TR/2026/WD-uievents-20260221/#dom-inputevent-iscomposing)は、compositionstartと対応するcompositionendの間の入力かを表す。保存成功や文字列の妥当性を示すflagではない。

独自設計では、compositionupdateが先に必ず到着する前提でbeforeinputの状態を決めない。composition lifecycleと受信event自身の情報を別々に記録し、未知の並びでは破壊的な変換を保留して現在のDOM/modelを照合する。`compositionend` は送信許可として扱わず、確定内容を検証する契機にとどめる。Enter送信の全browser共通workaroundや、旧来のkeyCode値による互換処理は本調査では推奨・検証していない。

## 3. targetRangesの「空」と、編集対象がないことを分ける

[従来の方法表](https://www.w3.org/TR/2025/WD-input-events-2-20250814/#interface-InputEvent)では、input/textareaとhistoryUndo/historyRedoのtarget rangesは空配列である。したがって、通常のtext controlで `getTargetRanges().length === 0` となっても、selectionや編集意図がないとは判断できない。また `data` は追加する文字列などであり、変更後の全文を表すわけではない。削除等ではnullとなり得る。

2026年版の重要な明確化は、**browserが作るtrusted eventの表と、scriptが初期化するeventの範囲を分けたこと**である。scriptが指定した `targetRanges` をUAはhostやinputTypeに基づいて作り直さず、未指定なら空のままとする。synthetic inputも初期化された範囲を返すと記述する。[2026年版 §6.1.2–6.2](https://www.w3.org/TR/2026/WD-input-events-2-20261015/#interface-InputEvent)

`InputEventInit.targetRanges` 自体は2025年版にも存在していた。**2026年にAPIが新設された、全browserが新挙動へ切り替わった、という意味ではない。** synthetic eventへtrusted event用の表を無条件適用するtest expectationを見直す材料である。

独自設計として、contenteditableのrangeを非同期処理に渡す場合は、DOMの範囲をそのまま永続的な編集位置とみなさず、model上の位置とrevisionに変換する。対応するeditorの外側や既に置換されたnodeを指す入力は受理しない。rangeの寿命そのものは[既存のStaticRange文書](../css/custom-highlight-registry-range-validity-boundary.md)で扱っており、ここで再定義しない。

## 4. synthetic eventの配送結果は編集完了ではない

[dispatchEvent reference](https://developer.mozilla.org/en-US/docs/Web/API/EventTarget/dispatchEvent)が示す戻り値は取消しの有無であり、`true` はmodel更新成功の証明ではない。listenerは同期的に呼ばれるが、listener内の例外がdispatchEvent呼出し元へ通常の例外として戻るわけでもない。

2026年版には、拡張機能によるsynthetic beforeinputをeditorが処理する互換性上の助言もある。ただしこれは非規範のnoteであり、`isTrusted=false` の入力をすべて無条件に受理する許可と解釈しない。[Input Events §6.2](https://www.w3.org/TR/2026/WD-input-events-2-20261015/#event-definitions)

独自の設計上、二つの方向の誤りを避ける:

- 全synthetic eventを捨てるだけでは、統合対象の補正機能やeditor toolbarとの連携を失い得る。一方、知らないeventを文字列名だけでmodel commandとして扱うのも避ける。
- 内部commandの成功は、専用関数の戻り値や明示的なoperation結果で受け取る。`dispatchEvent(...) === true`、`defaultPrevented`、`isTrusted` のいずれも、業務上の承認や編集完了のackに代用しない。

許容するinputType、payload、対象editor、range、revisionを受信側で検証し、単一のcommand経路へ変換することは独自案である。eventの発生源判定だけでアプリケーションの権限制御を完了できるとは主張しない。

## 5. ReactのonBeforeInputとnative beforeinputは同じ入口ではない

[React common reference](https://react.dev/reference/react-dom/components/common)は、`onBeforeInput` がnative beforeinputを直接使わず、他eventからpolyfillを試みると説明している。React eventと `nativeEvent` の具体的な対応は公開APIの保証ではない。同referenceのInputEvent handlerの追加propertyとして列挙されるのは `data` である。

そのため、TypeScriptで `e.nativeEvent as InputEvent` とassertしても、nativeのinputType・isComposing・getTargetRangesが揃うことは証明できない、というのがここでの設計判断である。型assertionをfeature detectionやbrowser試験の代わりにしない。

nativeの範囲情報を必要とするeditorなら、DOM refにnative listenerを取り付けるadapterを検討する。ただし以下を一つの責務として設計する:

- refの差し替え/unmountに合わせて解除する。React referenceにはref cleanupの仕組みがあるが、取り付けだけで終えない。
- 同じ編集をReact synthetic handlerとnative listenerの両方からmodelへ適用しない。
- DOMの編集権をbrowser/editorへ渡す領域と、Reactが描画する領域を決める。`suppressContentEditableWarning` は警告抑止であり、両者の競合を解消するalgorithmではない。

これらは独自のadapter設計であり、そのまま投入できる汎用IME対応componentの実装を提供するものではない。

## 6. inputによる補完にも境界がある

[MDN input](https://developer.mozilla.org/en-US/docs/Web/API/Element/input_event)によれば、text controlの利用者編集を通知するinputは、単なる `element.value = ...`、form reset、history navigationでの値復元などでは自動的に発生しない。

従って「beforeinputで処理しなかった変更は必ずinputで全て回収する」と範囲を広げすぎない。自分のprogrammatic writeには、model更新とDOM更新を一体で管理する明示的な入口を用意する。resetや復元を扱うなら、その経路でも実値とmodelを照合する。

非同期の候補取得や検証を併用する場合は、独自設計として `editor identity + revision + request generation` を結果に紐付ける。返却が遅れた結果を、取得時と異なるselectionや新しいcompositionへ適用しない。通信を中止したかどうかと、適用直前の世代確認は分ける。

## 7. 本番へ入れる前の検証表

以下は**提案する試験であり、今回成功したbrowser試験の一覧ではない**。

| ケース | 分離して確かめること |
|---|---|
| 取消可能な通常入力を独自処理 | browser側の変更とmodel operationが二重適用されない |
| 取消不能なcomposition更新 | 文字欠落や二重挿入がなく、変換候補とcaretが保たれる |
| beforeinputなしの補完・訂正 | 実際のinput/DOMを起点にmodelが追従する |
| compositionupdate順序の異なる記録 | 特定順序に依存して未初期化状態や破壊的変換を起こさない |
| input/textareaの空range | 空配列だけを理由に正常な編集を捨てない |
| synthetic eventに指定/未指定のrange | 受信payloadと採用した仕様版の期待値を分ける |
| handler例外とdispatchEvent戻り値 | 配送結果をcommand成功として記録しない |
| React/native両方のlistener | 一操作につきmodelへの適用が一回で、解除後に旧nodeへ書かない |
| value代入・reset・復元 | input通知がない経路でも、明示した同期方針が働く |
| 変換中に古い非同期結果が到着 | identity/revision/generation不一致で古い結果を採用しない |

実機記録にはbrowserの正確な版、OS、IME名・版、言語、text control/contenteditableの別、native/syntheticの別を残す。入力順だけでなく、DOM、model、selection、Undoの結果を比較する。通常キー、変換確定/取消し、再変換、貼付け、音声入力、自動補完は同一経路とは仮定しない。試験ログに実際のpasswordや個人の入力内容を残さず、専用fixtureを使う。

## 実施した確認と未確認事項

- 許可されたfrontend既存文書と直近16件のknowledge commitを点検し、`just index` 後に `beforeinput`、`insertCompositionText`、`isComposing`、`targetRanges` を検索した。執筆前はいずれも0件だった。
- 上記の公式仕様とreferenceを実際に開いて照合した。二つのWorking Draftの順序不一致を残し、sourceごとに日付とlicenseを記録した。仕様・sample codeの転載はしていない。
- Chromium 155.0.8059.79 / Debian GNU/Linux 13のheadless起動を試みた。既定profileの作成失敗後、書込み可能な一時profileで再試行したが、`process_singleton_posix.cc` の `socket() failed: Operation not permitted` でabortした。event試験に到達していない。sandbox制約の迂回は行っていない。
- DOM Standardはnative web取得でtimeoutとなったため、本稿ではその未取得本文を根拠に追加のalgorithm保証をしていない。
- browser runtime、OSの実IME、React実装、Undo/selectionの完全な保持、支援技術との組合せは未検証。synthetic eventのunit testだけでは実IMEの順序や取消し可否を検証できない。
- 検索evalは本稿を正しい語で取得できるかを確認するもので、入力動作のconformance testではない。

出典のライセンスはW3C Software and Document License 2023、MDN documentationのCC-BY-SA-2.5-or-later（Mozilla Contributors）、React documentationのCC-BY-4.0を確認した。本文は事実の独自整理と明示した設計案であり、上流の文章・コードのコピーやmodule昇格は行っていない。

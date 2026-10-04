---
{
  "id": "accessibility-streaming-live-region-announcement-boundary",
  "title": "Streaming live region: DOM更新・aria-busy・読み上げ完了を分ける",
  "kind": "knowledge",
  "technology": "accessibility",
  "version": "WAI-ARIA 1.2 Recommendation 2023-06-06; WCAG22 Understanding 4.1.3 updated 2026-05-11 / ARIA22 2026-01-12 / ARIA23 2026-06-01; Safari 27.2 beta 20625.2.4 released 2026-09-16; retrieved 2026-10-04 UTC; AT runtime untested",
  "tags": [
    "research-domain:frontend",
    "accessibility",
    "aria-live",
    "aria-busy",
    "aria-atomic",
    "aria-relevant",
    "status",
    "log",
    "streaming",
    "VoiceOver",
    "announcement",
    "focus",
    "beta"
  ],
  "sources": [
    {
      "id": "apple-safari272-beta-live-region-20261004",
      "url": "https://developer.apple.com/documentation/safari-release-notes/safari-27_2-release-notes",
      "type": "release_notes"
    },
    {
      "id": "wai-aria12-live-region-contract-20261004",
      "url": "https://www.w3.org/TR/2023/REC-wai-aria-1.2-20230606/",
      "type": "official_docs"
    },
    {
      "id": "wcag22-status-messages-understanding-20261004",
      "url": "https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html",
      "type": "official_docs"
    },
    {
      "id": "wcag22-aria22-status-20261004",
      "url": "https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA22",
      "type": "official_docs"
    },
    {
      "id": "wcag22-aria23-log-20261004",
      "url": "https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA23",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Streaming live region: DOM更新・aria-busy・読み上げ完了を分ける

## 問いと結論

チャットや生成回答をtokenごとに表示するとき、`aria-live="polite"`を付ければ差分だけが一度ずつ読まれるのか。`aria-busy="false"`になったら、回答を読み終えたとして次の画面へ進めてよいのか。

**受信完了、DOMへの反映、支援技術への通知、利用者の確認を別の状態にする。** live regionは読み上げの配信確認ではない。長文の全tokenを通知対象にする前に、短い状態通知で知らせる方式か、完成した発言を順に追加する方式かを決める。

既存の[Reactフォーム検証](../react/form-validation.md)は入力エラーの表示を扱う。本稿は、streaming表示の再描画、通知単位、再送・取消後の古い通知、読み上げの観測限界に対象を絞る。新しいARIA仕様が追加されたという主張ではない。

## 今回確認した変更と適用範囲

[Apple Safari 27.2 Beta Release Notes](https://developer.apple.com/documentation/safari-release-notes/safari-27_2-release-notes)は、**2026-09-16公開、27.2 beta、build 20625.2.4**と表示されている。Accessibilityのissue **186766347**は、streaming中の`aria-live`領域で、appendではなく再描画を行う場合にVoiceOverが既に通知したテキストを繰り返す問題の修正を記す。

これは2026-10-04時点に確認した**betaの修正告知**である。この資料だけから安定版への出荷、旧版すべての再現、全OS・全スクリーンリーダーでの修正を断定できない。内部アルゴリズムや修正commitも今回特定していない。実務上の変化は、同じ最終テキストになる実装でもDOM更新方法を変えて回帰確認する具体的な根拠が得られたことにある。

## 一次資料で確認した契約

### WAI-ARIA 1.2の規範

以下は[2023-06-06固定Recommendation](https://www.w3.org/TR/2023/REC-wai-aria-1.2-20230606/)の要約であり、本稿の設計案と区別する。

- [`aria-busy`](https://www.w3.org/TR/2023/REC-wai-aria-1.2-20230606/#aria-busy): 更新中に`true`を指定できる。ATはその間の変更を保留し、`false`時にまとめて扱ってもよい（MAY）。必ず保留・一括発話するという契約ではない
- [`aria-live`](https://www.w3.org/TR/2023/REC-wai-aria-1.2-20230606/#aria-live): `polite`は適切な機会の通知を勧める優先度で、利用者・AT・ブラウザーが変更できる。`assertive`は保留中の通知を消し得る。`off`でも領域にfocusがある場合は通知の対象になり得る
- [`aria-atomic`](https://www.w3.org/TR/2023/REC-wai-aria-1.2-20230606/#aria-atomic): `true`は領域全体、明示`false`は変更nodeを提示対象とする。`false`は文字列の未読suffixを追跡する指定ではない
- [`aria-relevant`](https://www.w3.org/TR/2023/REC-wai-aria-1.2-20230606/#aria-relevant): 既定値は`additions text`。ATへの提案で、対象とした全種類の変更を必ず提示する要件ではない。業務イベントの重複排除とは別である

したがって`aria-busy=false`を**playback完了・ack**に変換したり、DOM mutationの回数から発話回数を算出したりする設計は、この契約では正当化できない。これは上の規範の裁量から導く設計上の結論である。

### 状態通知と順序付き履歴の手法

[ARIA22（2026-01-12更新）](https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA22)は、短い状態通知に`role="status"`を使う。既定は`polite`かつatomicだが、環境差があるため全文を通知したい容器には`aria-atomic="true"`を明示することを勧める。また、通知発生前から容器にroleがあることをテストする。

[ARIA23（2026-06-01更新）](https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA23)は、順序のあるチャット履歴などの末尾への追加に`role="log"`を使う。通知は`polite`、atomicは`false`で、例にはATとブラウザーの組合せによる制限が明記されている。Techniqueは達成手法の例であり、WCAG適合にその実装だけが必須という意味ではない。

[Understanding 4.1.3（2026-05-11更新）](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html)は、検索結果の本文と「検索中」「結果あり」の状態通知を分ける。focusを移すcontext変更はこの達成基準のstatus messageとは別扱いである。待機表示を削除するだけでは終了が伝わらない場合があり、過剰な通知を避ける利用者テストも求めている。生成回答の本文をすべてstatusに入れる判断を、この達成基準だけから導かない。

## 推奨設計: 通知単位を先に選ぶ

以下は一次資料を踏まえた**独自の設計案**で、AppleやW3Cが認定した実装ではない。

### A. 長文は本文として残し、短いstatusで状態を伝える

長い説明・コード・表を含む回答は、まずこの方式を検討する。

1. 通常の見出し・本文・リストとして回答を構成し、streaming表示全体をlive regionにしない。利用者が手動で読み返せる構造を保つ
2. 独立した短い`role="status" aria-atomic="true"`の容器を初期表示から用意する。「回答を生成中」「回答の生成が完了」「生成を停止」のように、状態遷移時だけ内容を更新する
3. 状態の数値だけを書き換える場合も、何の進捗かが分かる文脈を同じ小さな容器に残す。回答全文や全会話をatomicな容器へ入れない
4. 完了時に入力欄のfocusを奪わない。「回答へ移動」など利用者が明示操作できる導線を別途設ける。移動時の通知と完了通知が重複しないか実機で確認する
5. 失敗・取消・途中までの回答を成功と同じ文言にしない。spinnerの消去だけで済ませず、可視の終端状態を残す

これは本文の読み上げを抑制する保証ではない。利用者が本文へ移動すれば自分の操作で読める必要があり、`aria-live="off"`を付けるだけで全状況の無音化はできない。

### B. 短い発言単位なら、完成したnodeをlogへ追加する

会話の逐次通知を利用者が必要とする場合は、視覚的なtoken更新と通知用の発言単位を分ける。

- 名前が分かる安定した`role="log"`容器を用意し、完成した発言nodeを末尾へ追加する。通知単位を一発言か確定した段落にするかは、内容の長さ・利用者テストで決める。ネットワークchunk境界をそのまま意味の区切りにしない
- nodeを追加するときには発話者と確定テキストを揃える。空nodeを先に追加し、後から全文を何度も置換する経路とは別のテストケースにする
- 同じ発言IDの受信再送で新nodeを作らない。表示の重複排除用IDと、同じ発言の訂正を識別するrevisionを分ける。このアプリ側の一度だけの追加は、AT側のexactly-once通知を意味しない
- log全体の`innerHTML`置換や、既存nodeを別nodeとして作り直す更新を避ける方針にする。フレームワークのrender呼出し回数ではなく、実際のnode再利用・置換を観察する
- `aria-relevant="additions"`だけに絞るなら、既存発言の文字修正は意図した通知対象から外れる。重要な訂正は「訂正: …」という別の確定項目にするか、`text`を含めて対象環境で確認する
- 本文をlogで通知する方式と、同じ全文を別statusにコピーする方式を重ねない。完成時の短いstatusとlog発言を併用する必要がある場合も、二重通知を試験する
- 古い履歴のprepend、仮想化による再挿入、並べ替えは新着末尾追加と別経路にする。履歴閲覧は必要だが、それを新着発言として再通知しないよう設計・試験する

append方式を選ぶのは意味のある更新単位を制御しやすくするためであり、Safari betaの修正告知から全環境での成功を保証するものではない。

## aria-busyを使うなら更新責務を限定する

複数nodeの変更を一まとまりとして提示したいときにだけ、対象容器の更新開始前に`true`、最後のDOM更新後に`false`を設定する。通信全体が終わるまでページ全体をbusyにする実装は避け、利用者に必要な操作・状態通知まで同じ容器へ巻き込まない。

- 正常終了だけでなく例外・取消・timeoutでもbusyを解除できる終端処理を持つ
- busyを解除する前に、成功・失敗・途中結果のどれを残すかを確定する。解除は成功フラグではない
- `true → 更新 → false`を同じtask内で実行するだけで、一回の発話が観測できるとは仮定しない。固定の待機時間を挿入しても、ATの発話完了確認にはならない
- 前のrequestの遅いcallbackが、次のrequestのbusy状態やstatusを書き換えないよう、現在のrequest IDと終端状態を照合する
- busyの解除を待って自動遷移したり、確認ボタンを自動実行したりしない。重要な意思決定には、表示された内容を再確認できる画面と明示操作を使う

たとえば取消直後に古いstreamのcompleteが届いた場合は、`cancelled`から`completed`へ上書きしない。単に`finally`で「完了」と通知する実装では、通信の後始末と業務上の成功が混ざる。busy解除は共通cleanupでも、終端メッセージは判定済みの状態から作る。

## 受入れ試験をDOMと実際の通知に分ける

以下は今後の実装向けの試験計画で、今回の実行結果ではない。

| ケース | アプリ／DOMで確認すること | ATを併用して確認すること |
|---|---|---|
| tokenを細かく受信 | 通知nodeは選んだ意味単位でだけ更新される | 文字ごとの割込みや既読部分の繰返しがないか |
| 完成nodeのappendと全領域の再描画 | 最終textが同じでもmutationの種類を記録する | Safari issue 186766347の症状を別々に再現・比較する |
| 一発言の再送と訂正 | 同一ID再送は重複追加せず、revision変更は意図どおり扱う | 新規・訂正の意味が区別できるか |
| 取消直後の遅い完了 | request IDと終端ガードが古い更新を拒否する | 「停止」の後に誤った「完了」が通知されないか |
| busy中の例外／timeout | busyが残留せず、途中結果と失敗を保持する | 解除後に必要な状態を把握できるか |
| 入力中・履歴を読んでいる途中 | 通常完了時のfocusを動かさない | 入力や読み返しが不必要に中断されないか |
| 通知中に利用者が移動・中断 | DOMを消さず、あとから本文を確認できる | 中断後も重要情報へ自力で戻れるか |
| alertとpoliteの競合 | 緊急でない進捗をassertiveへ昇格させない | 優先通知で消えた内容を履歴から確認できるか |
| 履歴prepend・仮想化・再mount | 既読履歴の再挿入を新着処理に混ぜない | 全履歴を再び読み始めないか |

対象OS、ブラウザーの正確なversion/build、ATのversion、操作mode、発話設定、更新頻度を記録する。Safari 27.2 beta + VoiceOverの確認と、利用者が実際に使う安定版の確認は別の結果として残す。他のブラウザーやATを列挙しただけでは対応検証にならない。

DOM assertionやaccessibility tree snapshotは、role・内容・focus・busy解除の証拠にはなるが、音声や点字で利用者に何が届いたかの証拠にはしない。発話の省略・割込みが設定や意図した利用者操作によるものか、不具合によるものかも区別する。通知を必ず再生させようとして利用者の中断を無視する設計は採らない。

## 検証範囲・未確認事項

- 実際に確認したのはApple公式ページの表示と、上記W3C一次資料の本文・日付・ライセンス。WAIのTechniqueとUnderstandingは補足であり、規範と分けた
- Appleのnative web取得はJavaScript要求のshellだった。公式Markdownへの取得も失敗したため、cloud browserでrender後の本文を読み、beta表記・build・修正内容を確認した
- VoiceOverを用いたstreaming回帰テスト、点字出力、他ATの発話queue、各安定版へのbackportは未確認。今回のcloud browserは資料閲覧であり、Safari/VoiceOver動作検証ではない
- 実行可能な製品実装、任意のフレームワークに共通する最適なflush間隔、全通知のexactly-once配信は提供・証明していない。`ariaNotify`など別APIへの移行も本稿の対象外
- 文中の実装・試験方針は独自提案。ソースコードのコピーはない。Apple資料は再利用ライセンスを特定できず`unknown`として原文を転載しない。W3C資料は明示されたSoftware and Document License（仕様2015版、WAIページ2023版）を記録した

---
{
  "id": "accessibility-authentication-password-manager-otp-paste",
  "title": "WCAG 2.2 の accessible authentication: password manager と OTP 貼り付けを認証全体で保つ",
  "kind": "knowledge",
  "technology": "accessibility",
  "version": "WCAG 2.2 Recommendation 2024-12-12 SC 3.3.8/3.3.9; Understanding 2026-09-06; H100/F109; HTML Living Standard 2026-10-01",
  "tags": [
    "research-domain:frontend",
    "wcag22",
    "accessible-authentication",
    "password-manager",
    "autocomplete",
    "otp",
    "mfa",
    "paste",
    "account-recovery",
    "cognitive-function-test"
  ],
  "sources": [
    {
      "id": "wcag22-authentication-rec-20241212-20261001",
      "url": "https://www.w3.org/TR/2024/REC-WCAG22-20241212/",
      "type": "official_docs"
    },
    {
      "id": "wcag22-understanding-authentication-20261001",
      "url": "https://www.w3.org/WAI/WCAG22/Understanding/accessible-authentication-minimum.html",
      "type": "official_docs"
    },
    {
      "id": "wcag22-technique-h100-authentication-20261001",
      "url": "https://www.w3.org/WAI/WCAG22/Techniques/html/H100",
      "type": "official_docs"
    },
    {
      "id": "wcag22-failure-f109-authentication-20261001",
      "url": "https://www.w3.org/WAI/WCAG22/Techniques/failures/F109",
      "type": "official_docs"
    },
    {
      "id": "whatwg-autocomplete-authentication-20261001",
      "url": "https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#autofilling-form-controls:-the-autocomplete-attribute",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-12-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# WCAG 2.2 の accessible authentication: password manager と OTP 貼り付けを認証全体で保つ

## 問いと結論

「パスワード欄に貼り付けを許可すれば、OTP を含むログインは accessible authentication といえるか」を扱う。結論は、最初のフォームだけでは判断できない。**認証経路の各段階で、記憶・転記などの認知機能テストへの対応を確認する**。保存済みパスワードを使えても、次の画面でコードの手入力を強制する設計は別に調べる必要がある。

未収録だった WCAG 2.2 SC 3.3.8 の実装上の境界を補う調査であり、2026年にこの基準が新設されたという報告ではない。バックエンドの認証強度や秘密情報の保存方式は対象外とする。

## 確認した公式の契約

### 1. SC 3.3.8 はパスワード自体の禁止ではない

[WCAG 2.2 Recommendation の SC 3.3.8](https://www.w3.org/TR/2024/REC-WCAG22-20241212/#accessible-authentication-minimum) は Level AA。認証のある段階で認知機能テストを要求する場合、次のいずれかがその段階で提供されることを条件とする。

- Alternative: 認知機能テストに依存しない別の認証方法
- Mechanism: テストを完了するための補助手段
- Object Recognition: 対象物を認識するテスト
- Personal Content: 利用者が当該サイトに提供した非テキストコンテンツを識別するテスト

規範の注記は、password manager による入力支援と copy and paste を補助手段の例として挙げる。したがって、パスワードを残したことだけで不適合、あるいは passkey の導入だけが唯一の達成方法、とはならない。

[SC 3.3.9](https://www.w3.org/TR/2024/REC-WCAG22-20241212/#accessible-authentication-enhanced) は Level AAA で、上記のうち Alternative と Mechanism のみを条件とする。AA の対象物認識・個人的コンテンツの例外を AAA に流用しない。この一項目の確認は、ページ全体の AA/AAA 適合宣言ではない。

### 2. MFA とアカウント回復を確認範囲から落とさない

[Understanding 3.3.8](https://www.w3.org/WAI/WCAG22/Understanding/accessible-authentication-minimum.html) は、既存利用者の認証を対象とし、最初のユーザー名作成やアカウント開始はこの SC の対象外と説明する。一方、multi-factor authentication (MFA) の各段階、および account recovery のために代替情報で本人認証する処理にも認知負荷への対応が必要である。

同ページは、音声 CAPTCHA を聞いて文字列を転記させる方法を、認知機能テストのない Alternative としては扱えないと説明する。「視覚以外でも提示した」という確認と、「記憶・転記を要求しない」という確認は分ける。

### 3. H100 と F109 が示す入力の境界

[H100](https://www.w3.org/WAI/WCAG22/Techniques/html/H100) は、適切な accessible name を持つ認証入力を使い、ブラウザ・password manager の補完と利用者の貼り付けを阻害しない達成手法。メールとパスワードだけでログインする場合の確認項目として、入力の名前と貼り付け可否を挙げる。Techniques は規範を満たす方法の例であり、その HTML サンプルへの完全一致が必須という意味ではない。

[F109](https://www.w3.org/WAI/WCAG22/Techniques/failures/F109) は、元の文字列を別の形式で再入力させ、適合する代替認証もない場合を失敗として扱う。代表例は特定位置のパスワード文字だけを選ばせる方式、または一桁ずつ入力させる OTP 欄。**分割 OTP 欄そのものを一律に禁止する説明ではない**。全文を最初の欄へ一度に貼り付けると残りへ自動展開される構成は、同ページの失敗例から除外されている。F109 の判定手順は、元の形式で全文を入力できるか、できなければ適合する代替認証があるかを確認する。

### 4. autocomplete の値と保証範囲

[HTML Standard 4.10.19.7](https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#autofilling-form-controls:-the-autocomplete-attribute) の用途を区別する。

- `username`: アカウント識別に用いるユーザー名
- `current-password`: そのアカウントの現在のパスワード
- `new-password`: 作成・変更する新しいパスワード
- `one-time-code`: 本人確認に用いる一回限りのコード

`autocomplete` はユーザーエージェント (UA) へのヒントである。属性が存在することと、特定端末で目的のコード候補が表示されて認証を完了できることは別の確認事項となる。個別ブラウザの対応版や SMS 自動取得能力をこの仕様表から推定しない。

## 推奨する実装判断

ここからは上の契約を踏まえた独自の設計案であり、WCAG が要求する唯一の実装や、実機で確認済みの挙動ではない。

1. **画面ではなく認証経路を棚卸しする。** 通常ログイン、追加認証、再認証、回復のそれぞれについて「利用者が何を覚えるか・何を写すか・どの支援を使えるか」を記録する。最短の成功経路だけでレビューを終えない。
2. **標準入力を基準実装にする。** 名前と入力目的を明確にし、保存済み資格情報の補完と貼り付けが処理中に消されないことを確認する。ログインに `new-password` を流用しない。既存フォームの制御状態・入力マスク・イベント処理もレビュー対象に含める。
3. **OTP はまず単一のテキスト欄で扱う。** 数字に見えるコードも文字列として保持し、先頭ゼロを含むテスト値で確認する。見た目の区切りのためだけに一桁入力を強制しない。分割 UI が必要なら、一括貼り付け後の全桁保持と訂正操作を受け入れ条件にする。
4. **補完候補が出ない環境でも入口を残す。** 貼り付けを維持し、利用できない認証器に行き止まりが生じる場合は別の適合する経路を設計する。別経路の本人確認強度は担当者と検討し、アクセシビリティ対応という理由で本人確認を省略しない。
5. **入力の加工は契約を決めてから行う。** コードの区切り文字を除去するなら発行側と受理側の形式を先に揃える。パスワードの空白除去・文字置換・大文字小文字変換を同じ処理へまとめない。表示上の便利さで元の資格情報を壊さない。
6. **実装ログに生の秘密を残さない。** 不具合調査では実際のパスワードや OTP の代わりにダミー値・入力経路・対象版・失敗段階を記録する。貼り付け文字列の収集を「動作確認」として追加しない。

HTML の属性確認だけで完成扱いにせず、フォームの状態管理、外部 identity provider の遷移、認証器の選択、回復までを一連の利用手順として検証する。

## 受け入れテスト案

以下は本資料の設計提案を検証するための**未実行のテスト計画**。SC の根拠と製品独自の堅牢性チェックを併記しており、検索 eval の成功とは区別する。

| 条件 | 操作・確認する結果 | 根拠・目的 |
|---|---|---|
| 保存済み資格情報 | 対象 password manager から入力し、送信直前まで値が保たれる | H100 の補完非阻害を製品環境で確認 |
| 外部保管したダミー資格情報 | ユーザー名・パスワードへ貼り付けて完了できる | H100 の accessible name と paste 確認 |
| 単一 OTP 欄 | `012345` を一括貼り付けし、先頭ゼロを含む全文が送られる | 製品独自の文字列保持チェック |
| 分割 OTP 欄 | 最初の欄への一回の貼り付けで全桁が展開され、訂正できる | F109 の例外境界＋訂正操作の独自チェック |
| 補完候補が現れない端末 | 利用可能な貼り付け経路または適合する代替認証で完了できる | ヒントの存在を完了能力と混同しない |
| 認証器が使えない回復経路 | 回復時の本人認証を最後までたどり、必須の記憶・転記を点検する | Understanding の回復認証の範囲 |
| 文字起こしが必要な音声 CAPTCHA | 音声切替だけを Alternative の達成証拠にしない | Understanding の認知機能テスト境界 |
| 特定文字の入力要求 | 全文入力と適合する代替認証の双方がない状態を検出する | F109 の失敗条件 |

対象ブラウザ、OS、支援技術、password manager、認証画面の版をテスト記録へ残す。単なる DOM への値代入テストは、利用者による実際の貼り付け・補完の再現とは区別する。

## 避ける判断と適用限界

- 最初のログインフォームだけの合格を、後続 MFA や回復フローへ自動的に拡張する
- `autocomplete="one-time-code"` があれば、全端末で OTP を取得できると見なす
- 六つの欄という見た目だけで F109 違反と決める、または最初の一桁が貼れれば成功と見なす
- 3.3.8 と別の基準である 3.3.7 Redundant Entry の例外を、認証の認知機能テスト全般への免除として流用する
- この文書からパスワード保存の暗号設計、認証強度、法的適合性、WCAG 全項目の合格を導く

本調査ではブラウザ・スクリーンリーダー・password manager・実サービスの runtime は未検証。UI サンプルコードの移植や module 昇格は行っていない。WebAuthn のプロトコル要件、SMS 配信方式、CAPTCHA ベンダーごとの適合性も未確認。

## 出典・日付・再確認

取得日はすべて **2026-10-01 UTC**。規範は固定 URL の WCAG 2.2 Recommendation **2024-12-12** を使用した。Supporting documents の表示更新日は Understanding 3.3.8 が **2026-09-06**、H100 が **2026-01-12**、F109 が **2026-04-27**。HTML の参照ページは **Last Updated 1 October 2026** と表示していた。これらは確認した版・表示日であり、当該日に特定の要件が追加されたという変更履歴の主張ではない。

規範は W3C Document License 2023、今回開いた WAI の3ページはフッターから W3C Software and Document License 2023 を案内していた。WHATWG の仕様本文は CC BY 4.0。ライセンス識別は [W3C Document License](https://www.w3.org/copyright/document-license-2023/)、[W3C Software and Document License](https://www.w3.org/copyright/software-license-2023/)、[WAI 使用案内](https://www.w3.org/WAI/about/using-wai-material/)、[WHATWG Intellectual property rights](https://html.spec.whatwg.org/multipage/acknowledgements.html#ipr) で確認した。本文は各出典の事実に基づく独自の日本語要約と設計上の考察であり、公式訳・公式認証ではない。原文やコード例の転載はしていない。

再確認期限は official_docs の90日 TTL に合わせ **2026-12-30**。再調査時は 3.3.8 と 3.3.9 の条件差、H100/F109 の条件、対象 UA の入力経路を読み直す。Supporting documents の更新日だけを取得日に転記しない。

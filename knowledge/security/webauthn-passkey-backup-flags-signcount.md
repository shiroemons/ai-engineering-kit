---
{
  "id": "security-webauthn-passkey-backup-flags-signcount",
  "title": "WebAuthn Level 3 の passkey: BE/BS と signCount を分けて検証する",
  "kind": "knowledge",
  "technology": "security",
  "version": "WebAuthn Level 3 W3C Recommendation 2026-08-25; Level 2 Recommendation 2021-04-08 comparison; FIDO consumer deployment white paper May 2024",
  "tags": ["research-domain:security", "webauthn", "passkey", "BE", "BS", "backupEligible", "backupState", "signCount", "attestation", "account-recovery"],
  "sources": [
    {"id": "w3c-webauthn3-rec-20260825-backup-20261002", "url": "https://www.w3.org/TR/2026/REC-webauthn-3-20260825/", "type": "official_docs"},
    {"id": "w3c-webauthn2-rec-20210408-flags-20261002", "url": "https://www.w3.org/TR/2021/REC-webauthn-2-20210408/", "type": "official_docs"},
    {"id": "fido-synced-passkey-consumer-recovery-202405-20261002", "url": "https://fidoalliance.org/wp-content/uploads/2024/05/Synced-Passkey-Deployment_-Emerging-Practices-for-Consumer-Use-Cases_2024-May-31.pdf", "type": "maintainer_article"}
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# WebAuthn Level 3 の passkey: BE/BS と signCount を分けて検証する

## 問いと実務上の結論

同期可能な passkey を受け入れる RP（Relying Party）が、登録情報をどう保存し、認証時のフラグ変化や署名カウンターをどう判定するかを扱う。OAuth のトークン検証やパスワード保存とは別の、credential 単位の検証・復旧設計である。

本書の設計判断は、バックアップ可否、観測時点のバックアップ状態、カウンター異常、認証器への信頼を別項目として扱うこと。`BS=0` を直ちに侵害と判定したり、`BE=0` だけでハードウェア保護を保証したりしない。対象は BE/BS を利用して RP の復旧案内・認証ポリシーを判断する実装であり、WebAuthn 検証器全体の代用品ではない。

## 確認した版と差分

- [Level 3 の固定公開版](https://www.w3.org/TR/2026/REC-webauthn-3-20260825/) は **2026-08-25 W3C Recommendation**。2026-10-02 に現行 TR URL と固定版の見出し・Status を照合した。Candidate Recommendation や Editor's Draft と記載しない。
- [Level 2 の §6.1](https://www.w3.org/TR/2021/REC-webauthn-2-20210408/#sctn-authenticator-data) では flags の bit 3〜5 が予約領域 RFU2 だった。Level 3 の §6.1 と §18.1.1 は bit 3 を BE、bit 4 を BS として定義する。旧実装の予約ビット前提を見直す必要があるというのが、本書の移行上の判断である。
- FIDO の consumer deployment white paper は本文表紙が **May 2024**。過去の展開事例をまとめた解説であり、2026 年時点のブラウザ対応表や規範仕様としては使わない。本書は §2.1〜2.2 の復旧上の論点だけを参照する。

## 仕様で確認した契約

### BE と BS は異なる寿命を持つ

[Level 3 §6.1・§6.1.3](https://www.w3.org/TR/2026/REC-webauthn-3-20260825/#sctn-credential-backup) によると、`BE`（Backup Eligibility、bit 3）は生成時に決まり変更禁止、`BS`（Backup State、bit 4）は変化し得る。意味の組合せは次のとおり。

- `BE=0, BS=0`: single-device credential
- `BE=1, BS=0`: multi-device credential だが、現在はバックアップされていない
- `BE=1, BS=1`: multi-device credential で、現在バックアップされている
- `BE=0, BS=1`: 許されない組合せ

バックアップ状態が不確か、またはバックアップされた credential に問題が疑われるとき、認証器は BS を立てるべきではない（SHOULD NOT、§6.1）。したがって BS が落ちたことだけから、利用者の操作や漏えいを原因として断定できない。

### RP の照合条件と永続化の境界

[§7.1](https://www.w3.org/TR/2026/REC-webauthn-3-20260825/#sctn-registering-a-new-credential) と [§7.2](https://www.w3.org/TR/2026/REC-webauthn-3-20260825/#sctn-verifying-assertion) は、登録時・認証時とも `BE=0` なら `BS=0` を確認する。登録時の credential record には `backupEligible`、`backupState`、`signCount` を保存する。

**固定版 §7.2 step 19 の条件に注意する。** 保存済み BE との一致確認は、credential の backup state を RP の業務ロジックまたはポリシーに使う場合の手順に含まれる。これを無条件の文面として引用しない。一方、認証器側の BE 不変規則はその条件とは独立している。本書の対象実装ではこの条件を満たすため、保存済み BE と今回の BE を照合する。

認証署名は `authenticatorData` と `clientDataJSON` の SHA-256 ハッシュを連結したデータに対して検証する。新しい状態は検証を経て保存し、RP 独自の追加セキュリティ検査がある場合も、その成功後まで更新を遅らせることが推奨される。フラグの抽出・比較と、DB 更新やユーザー向け案内の実行を同一視しない。

### signCount はクローン検出の補助情報

[§6.1.1](https://www.w3.org/TR/2026/REC-webauthn-3-20260825/#sctn-sign-counter) と §7.2 step 22 の要点は次のとおり。

- カウンター非実装の認証器では値が常にゼロになり得る
- 保存値か今回値のどちらかが非ゼロなら、今回値が保存値より増えたかを比較する
- 増加しない値はクローンの証明ではない。認証器の故障や、生成順と RP での処理順の逆転も候補になる
- 異常時に認証を失敗させるか、保存値を更新するかは RP 固有の判断になる

この比較の発動条件は BE の値ではない。`BE=1` を理由に非ゼロカウンターをすべて無視する規則も、「同期 passkey は必ずゼロ」という保証も、この検証手順からは導けない。

## フラグから推測しすぎない

[§6.1](https://www.w3.org/TR/2026/REC-webauthn-3-20260825/#sctn-authenticator-data) は、authenticator data の信頼を、RP が評価した認証器のセキュリティ特性に結び付ける。[§13.4.4](https://www.w3.org/TR/2026/REC-webauthn-3-20260825/#sctn-attestation-limitations) は attestation で得られる保証にも限界があることを説明する。

これを踏まえた本書の判断は以下である。

- `BE=0` は仕様上の single-device という区分として保存する。「秘密鍵が耐タンパー装置から絶対に取り出せない」「会社管理端末である」「以前と同じ物理端末である」という別の主張に変換しない
- 署名が正しいことと、認証器の製造元・保護方式を信頼できることを分ける。特定ハードウェアの保証が必要なら、受け入れる attestation、信頼アンカー、失効情報、対象認証器の要件を別途定義・検証する
- `BS=1` は端末台数、バックアップ先の一覧、独立した復旧手段の個数ではない。状態が将来も変わらない保証として UI に表示しない
- サーバーの credential 一件を物理端末一台に対応付けた端末管理台帳に流用しない。表示名や登録ブラウザの情報だけで、現在秘密鍵を利用できる端末を列挙したつもりにならない

## 独自の保存・復旧設計案

以下は本リポジトリの提案であり、仕様が定める DB スキーマや必須 UX ではない。

1. credential ごとに、登録時の BE、最後に検証して受理した BS、カウンター値、観測日時、使用したポリシー版を管理する。アカウント全体の一つの `isSynced` に潰さない
2. 旧レコードに BE/BS を保存していなかった場合は、未確認状態を保持する。列追加の既定値 false を「登録時 BE=0 だった証拠」にしない。保存済み原資料の有無を確認し、判定根拠がなければ移行方針に沿って再登録などを案内する。未知と明示的なゼロを区別する
3. 認証要求の利用者・credential 対応付け、challenge、origin、RP ID、必要な UP/UV、署名等の検証を省略しない。未検証の BS を使ってパスワードを消したり復旧設定を変更したりしない
4. 保存 BE が既知の本書対象フローでは、不一致を状態更新で上書きして隠さず検証失敗として扱う。BS の `0→1` と `1→0` は合法な状態変化として扱い、BE の変化と別のイベントにする
5. `BS=1→0` では追加の認証手段・復旧経路を確認する案内を用意する。`BS=0→1` でも、パスワード削除は独立した利用者確認・復旧評価を伴う操作とする。フラグ一つで自動移行しない
6. カウンター異常では、並行リクエストの処理順、他のリスク情報、対象 credential の特性を記録して、追加認証・拒否・継続の方針を決める。同期 credential 全般の拒否や、全アカウントの一括失効を既定にしない

[FIDO white paper §2.1〜2.2](https://fidoalliance.org/wp-content/uploads/2024/05/Synced-Passkey-Deployment_-Emerging-Practices-for-Consumer-Use-Cases_2024-May-31.pdf#page=6) も、同期済みであっても provider に入れなくなることや互換性のない端末への移行で復元できなくなる可能性を挙げる。そこで本書では、同じ provider に依存する複数の利用端末を、そのまま独立した復旧手段の複数所持とは数えない。

### 並行処理で保存値を壊さないための判断

独自案として、credential 単位のロックまたは楽観的競合制御で「保存値の読取・ポリシー評価・受理結果の更新」を整合させる。署名が有効でも、二つの応答が生成順と逆に届く可能性は残るため、DB の排他だけでカウンター異常の原因を確定できない。

非増加カウンターを受け入れる方針なら、保存値を維持するか更新するかを明文化する。異常扱いしておきながら無条件に小さい値を書き戻す実装を避ける。BS は単調増加値ではないので OR 集約や最大値保存を使わず、どの受理済み観測を「最後」とするか決める。これは同期 provider 全体の状態を復元する仕組みではない。

## 検証時の具体例

以下は実装前レビュー用の独自テスト案。検索 eval は文書の発見性だけを検査し、これらの認証動作を実行しない。

| 入力・状況 | 本書の方針で確認する結果 |
|---|---|
| 登録・認証の `BE=0, BS=1` | 不正な組合せとして拒否し、状態を保存しない |
| 保存 BE=1、今回 BE=0 | backup state ポリシーを使うフローの一致検証が失敗し、登録値を書き換えない |
| 保存 BE=1、BS が 1→0 | BE 不一致扱いにせず、成功した検証後に状態と復旧案内を更新する |
| カウンターが 0→0 | 非増加という理由だけでは拒否しない。他の検証はすべて実施する |
| カウンターが 0→3、4→6 | 増加として扱う。固定の +1 は要求しない |
| カウンターが 4→4、4→0 | どちらかが非ゼロなので異常評価へ進み、クローン確定と表示しない |
| BE=1、カウンターが 4→4 | BE を理由にカウンター評価を迂回しない |
| BS=1 だが署名・challenge が不正 | 登録情報、復旧状態、パスワードを変更しない |
| 並行応答が 8、7 の順で到着 | race condition を想定した分岐・保存競合・監査記録を確認する |
| 移行前レコードの BE が未保存 | false と同一視してポリシー保証を付けない |

## 限界・未確認事項・provenance

- 実際のブラウザ、OS、同期 provider、認証器、言語別 WebAuthn ライブラリの挙動は未検証。規範の公開と製品の実装完了を混同しない
- すべての errata・未解決 Issue、CTAP 実装、credential exchange、attestation メタデータ運用、特定の保証レベルへの適合性は未確認。BE/BS だけでこれらの要件を満たしたとは判断しない
- 対象を BE/BS とカウンターの判断境界に絞っている。完全な登録・認証器選択・conditional UI・復旧本人確認の手順は、対象版仕様と利用ライブラリを別途確認する
- 全 source は 2026-10-02 UTC に本文を取得・確認した。期限 2026-12-31 は参照条件を再確認するための 90 日 TTL であり、標準そのものの失効日ではない
- 出典のコード・図・長文・実装を移植していない。本文・保存方式・数値例・テスト案は独自に構成した日本語の要約と設計提案であり、公式仕様の翻訳版ではない
- Level 3 は著作権表示から [W3C Software and Document License 2023](https://www.w3.org/copyright/software-license-2023/) を確認。Level 2 は document-use 表示から [W3C Document License](https://www.w3.org/copyright/document-license-2023/) への転送を確認した。FIDO PDF は © 2024 FIDO Alliance / All rights reserved と表示され、再利用ライセンスは確認できないため独自要約だけに用いた

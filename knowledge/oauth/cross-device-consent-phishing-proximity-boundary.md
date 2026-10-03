---
{
  "id": "oauth-cross-device-consent-phishing-proximity-boundary",
  "title": "OAuth cross-device: RFC 10027の同意phishing・近接性・device grant採用境界",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 10027 / BCP 247 (August 2026); RFC 8628 (August 2019); FIDO CTAP 2.2 Proposed Standard 2025-07-14; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:security",
    "oauth",
    "cross-device",
    "RFC10027",
    "BCP247",
    "device-grant",
    "CDCP",
    "proximity",
    "user_code",
    "verification_uri_complete",
    "FIDO-CDA",
    "fallback"
  ],
  "sources": [
    {
      "id": "rfc10027-cross-device-risk-proximity-20261003",
      "url": "https://www.rfc-editor.org/rfc/rfc10027.html",
      "type": "official_docs"
    },
    {
      "id": "rfc8628-device-grant-confirmation-lifecycle-20261003",
      "url": "https://www.rfc-editor.org/rfc/rfc8628.html",
      "type": "official_docs"
    },
    {
      "id": "fido-ctap22-hybrid-proximity-20250714-20261003",
      "url": "https://fidoalliance.org/specs/fido-v2.2-ps-20250714/fido-client-to-authenticator-protocol-v2.2-ps-20250714.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# OAuth cross-device の同意と端末間の信頼境界

## 問いと今回の更新の意味

TV、共有端末、CLIの画面にあるQRコードをスマートフォンで読み取り、正規の認可サーバーでMFAを完了したら、意図した端末にだけ権限を渡せるか。ここで見直すべきなのは、本人確認の強さと、権限を受け取る端末への結び付けの違いである。

[RFC 10027 / BCP 247](https://www.rfc-editor.org/rfc/rfc10027.html) は2026年8月公開のBest Current Practice。RFC 8628に以前から存在したremote phishingの注意を踏まえ、cross-device全般のリスク評価、対策の限界、プロトコル選択を整理している。新しいgrant parameterや、RFC 8628を置換するwire protocolの導入と解釈しない。本稿は、既存の[PKCE](authorization-code-pkce-state-redirect.md)、[DPoP](dpop-proof-binding-nonce-replay.md)、[passkeyのBE/BS](../security/webauthn-passkey-backup-flags-signcount.md)に未収録だった、端末間で同意の文脈が入れ替わる問題を扱う。

## 一次資料で確認した契約

### 1. 正規サイトのMFAでも同意先の誤認は残る

[RFC 10027 §1.1](https://www.rfc-editor.org/rfc/rfc10027.html#section-1.1) のCDCP（Cross-Device Consent Phishing）は、認証情報を盗むことを必須としない。利用者が正規のAuthorization Deviceで承認しても、その要求を始めたConsumption Deviceが攻撃者側なら、そちらに権限が渡り得る。MFA成功を端末間channelの認証成功と読み替えない。

[§2](https://www.rfc-editor.org/rfc/rfc10027.html#section-2) の規範の強さは次のとおり。

- 導入前のrisk assessmentと、評価したリスクに対応する§6.1のmitigation選択はMUST
- 十分に低減できなければcross-deviceを避けるのはSHOULD
- 可能ならproximityを対策に含めるのはSHOULD。すべての実装に特定の位置情報APIを強制するMUSTではない

### 2. QR化しても利用者の確認手順は残る

[RFC 8628 §3.2–3.4](https://www.rfc-editor.org/rfc/rfc8628.html#section-3.2) では、clientのtoken要求に使う`device_code`と、人が扱う`user_code`を分ける。`verification_uri_complete`は後者相当の情報を含む省入力経路である。[§3.3.1](https://www.rfc-editor.org/rfc/rfc8628.html#section-3.3.1)では、これをQR等で表示してもclientの`user_code`表示はMUST。ASでのコード表示と一致確認はSHOULDで、許可・拒否の選択も残る。

[§5.4](https://www.rfc-editor.org/rfc/rfc8628.html#section-5.4)は、利用者が手元の端末を認可していると確認することを重視する。コードが一致しただけでその端末の物理的な所持を暗号学的に証明した、とは本稿では判定しない。

[§3.1・§3.5](https://www.rfc-editor.org/rfc/rfc8628.html#section-3.5)では、開始は利用者操作を受けるSHOULDであり、`authorization_pending`と`slow_down`以外のエラーではpolling停止がMUST。`expired_token`からの再開始はMAYだが、利用者操作を待つSHOULDがある。再試行機能を、拒否・期限切れのたびに新しい同意要求を自動発行する実装へ広げない。

### 3. 防御の効果を混ぜない

[RFC 10027 §6.1](https://www.rfc-editor.org/rfc/rfc10027.html#section-6.1)は各対策に限界を付けている。

| 対策 | 残る境界 |
|---|---|
| 短命QR・user code | 露出時間を狭めるが、その有効期間内の誘導を排除しない（§6.1.2） |
| sender-constrained token | tokenの持ち出しを難しくするが、Consumption Deviceを支配する者が鍵を利用できる場合は残る（§6.1.12） |
| 同一network・IP由来の近接推定 | VPN、携帯回線、位置精度などに左右され、AS単体が距離を測定する保証ではない（§6.1.1） |
| 承認画面・教育 | 文脈と拒否方法を改善するが、単独での十分な対策とはしない（§6.1.13–6.1.14） |

たとえばDPoPを追加する案では、「別の鍵で盗用されたtokenを拒否する」テストと、「最初から攻撃者の鍵に結び付いた認可要求へ利用者が同意する」テストを分ける。後者までDPoP導入だけで合格とする評価項目は作らない。これは上記の境界を具体化した本稿のテスト設計である。

### 4. FIDO CDAのQRは、任意のOAuth QRと同じ契約ではない

[CTAP 2.2 §11.5](https://fidoalliance.org/specs/fido-v2.2-ps-20250714/fido-client-to-authenticator-protocol-v2.2-ps-20250714.html)では、hybrid transportはBLEによる近接性と、network tunnelによるCTAPメッセージ輸送を分ける。[§11.5.1](https://fidoalliance.org/specs/fido-v2.2-ps-20250714/fido-client-to-authenticator-protocol-v2.2-ps-20250714.html)のQRはpublic keyとshared secretを運ぶ。単にASのverification URLをQR化するdevice grantとは別の仕組みである。

参照したのは2025-07-14のProposed Standard固定版。ブラウザやOSの実装完了を保証する資料として使わない。FIDO CDAは認証方式であり、OAuthのscopeや対象APIの認可判断を省く根拠にもしない。

## 採用判断と移行案（独自の提案）

以下は本稿の配備案であり、RFCが一律に指定するDB構造、数値上限、製品設定ではない。

### A. 「便利だからdevice grant」から始めない

機能単位に、受け取る権限、端末の能力、ネットワーク条件、復旧方法を記録する。たとえば共有表示端末で限定資料を映す機能と、同じ製品の管理者がcredentialを発行する機能は、同じQRログインの採用可否にまとめない。

[RFC 10027 §6.2](https://www.rfc-editor.org/rfc/rfc10027.html#section-6.2)の選択指針は、利用可能ならFIDO CDA、適用条件を満たせばCIBA、他方式が端末・システム制約で使えない場合に追加対策付きdevice grantを検討する方向である。特に§6.2.1.5はdevice grantをsame-device用途に使わず、機微・高価値・業務上重要な資源では避けるよう述べる。§6.2.3.5のFIDO推奨は、端末が対応し、Consumption Deviceに適切なFIDO credentialがないcross-device認証の条件付きSHOULDである。

この指針に沿う本稿のレビュー手順は次のとおり。

1. 同一端末の通常ブラウザ認可で用が足りるかを先に確認する。QRの有無を機能要件そのものにしない
2. FIDO CDAを使う案は、対象端末・スマートフォン・ブラウザ/OSの組と、BLEおよびネットワーク接続を実機で確認する。OAuth認可が必要ならauthorization code + PKCEとの接続を別に検証する
3. CIBAを使う案は、利用者identifierの取得とASから認可端末への連絡手段を確認する。通知へ変更しただけで予期しない承認要求が消えるとは判定しない
4. device grantを残す機能は、代替方式が使えない具体的理由、許可する低リスクのscope、開始可能な端末の信頼根拠、残余リスクの判断者を記録する
5. いずれも要件を満たせないなら、その端末で当該機能を開始させない経路を用意する。FIDO失敗時に無条件のdevice grantへ切り替えない

### B. 発行前・同意時・発行後を別のゲートにする

発行前は、client_idの申告だけを端末登録済みの証明にせず、採用した信頼方式をAS側で検証する。近接性を採用する配備では、何を観測し、誰が検証したかを記録する。単なる「同じIP」という値をBLEに基づく確認と同じ保証区分へ入れない。携帯回線などで判定が不明なら、理由を示して別の許可済み経路へ案内し、黙って検査を外さない。

同意時は、何を、どの利用先へ、どの範囲で許可するのかをASが把握する情報から表示する。clientの自由入力表示名だけで「会社管理端末」と装飾しない。「ログインの確認」という曖昧な文言で新端末への権限付与を隠さない。拒否を目立つ操作として残し、コードを読んだ時点とgrantを成立させる時点を分ける。

発行後は、その端末とgrantを利用者が確認・停止できる導線を用意する案とする。短命access tokenだけで停止要件を満たしたとせず、refresh token、利用先session、RSの失効確認方式まで含めて停止遅延を測る。侵害時にアカウント全体しか止められない構成と、該当grantを絞って止められる構成の運用負担も比較する。

### C. 状態遷移と監査を実装要件にする

自社ASを実装する場合の案として、要求ごとにpending、approved、denied、expired等の状態と一意な相関IDを持たせ、同意結果の遷移を競合制御する。同じ承認の二重処理が新たなgrantを増やさないこと、拒否後に遅れて届いたUI操作が状態を復活させないことを確認する。これはコード入力の一度きり化とは別であり、入力ミスや正規のpollingを攻撃と誤判定しないようにする。

監査には、採用方式、policy版、信頼判定、拒否/期限切れ理由、発行されたgrantとの相関を残す設計とする。device_code、QR全体、access/refresh tokenをログへ保存する方法は採用しない。位置情報を利用する場合は、精度や保管期間を必要最小限にし、位置確認への同意とOAuthの権限付与を同一操作として扱わない。

## 導入レビュー用の反例（未実行）

| ケース | 確認する結果 |
|---|---|
| 正規ASのMFAは成功したが、要求を始めたのは未登録端末 | MFA成功だけで端末の信頼ゲートを通さない |
| 攻撃者側の鍵にboundなtokenが発行可能な試験構成 | DPoPの所持証明成功をCDCP防止の成功条件へ数えない |
| verification_uri_completeのQRだけを表示しuser_codeを隠す | RFC 8628のclient表示MUSTに反するため修正する |
| 短命コードを有効期間内に別の表示場所へ転送する | TTLだけで転送元の信頼性を確定しない |
| 正規のTVはWi-Fi、スマートフォンは携帯回線 | 同一IP不一致を説明し、許可済み代替経路か拒否に進む |
| BLE不可・FIDO CDA失敗 | 無条件fallbackで当初の保証を弱めない |
| access_denied / expired_tokenと遅延承認が競合 | pollingが停止し、拒否・期限切れから勝手に新規要求を開始しない |
| 同じ承認イベントを二重配送 | grant成立を冪等にし、監査の相関を保つ |
| 利用者が該当grantを停止した直後 | 複数RS・sessionを含む実際の停止時間を測定する |

これらは設計・統合試験案で、実機試験結果ではない。付属の検索evalは文書の発見性を検査するだけで、CDCP耐性やプロトコル適合性を認定しない。

## 適用範囲・未確認事項・由来

- 本稿の中心はOAuth cross-deviceの採用と同意先の境界。CIBA全プロファイル、デジタルcredential提示、session transferの詳細、BLE relayの定量評価は対象外
- 特定AS、SDK、CLI、ブラウザ、OS、ハードウェアでの対応状況・既定値・fallback挙動は未検証。FIDO CDAを全攻撃の完全防止と表現しない
- RFC 10027本文とRFC EditorのinfoページでBCP 247・2026年8月を照合した。errataへの公式リンクは取得エラーとなり、全errataの有無は確認できなかった。未確認を「errataなし」と記録しない
- 全3 sourceの本文を2026-10-03 UTCに確認した。2027-01-01の期限は公式文書90日TTLによる再確認日で、RFCやCTAP自体の有効期限ではない
- RFC本文の著作権表示はIETF Trust / BCP 78。Code Componentsの表示はRFC 10027がRevised BSD、RFC 8628がSimplified BSDであり、一括して同じ表記にしない
- CTAP固定版はFIDO Alliance ©2025 / All Rights Reserved。再利用ライセンスを確認できず、catalogはunknownとして独自要約だけに用いた。出典のコード・図・長文は移植せず、本文・配備判断・テスト案を日本語で独自に構成した

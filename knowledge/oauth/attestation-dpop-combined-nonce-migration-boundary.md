---
{
  "id": "oauth-attestation-dpop-combined-nonce-migration-boundary",
  "title": "OAuth client attestation Draft 11: DPoP combined mode のnonce移行・鍵一致・再試行境界",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "draft-ietf-oauth-attestation-based-client-auth-11 (2026-09-03, work in progress), compared with -10 (2026-07-06); RFC 9449 (2023-09); RFC 7638 (2015-09); verified 2026-10-04 UTC",
  "tags": ["research-domain:security", "oauth", "client-attestation", "DPoP", "combined-mode", "nonce", "challenge", "key-binding", "JWK-thumbprint", "draft-11"],
  "sources": [
    {"id": "oauth-attestation-combined-draft11-20261004", "url": "https://datatracker.ietf.org/doc/html/draft-ietf-oauth-attestation-based-client-auth-11", "type": "official_docs"},
    {"id": "oauth-attestation-prior-challenge-draft10-20261004", "url": "https://datatracker.ietf.org/doc/html/draft-ietf-oauth-attestation-based-client-auth-10", "type": "official_docs"},
    {"id": "oauth-attestation-draft-status-20261004", "url": "https://datatracker.ietf.org/doc/draft-ietf-oauth-attestation-based-client-auth/", "type": "official_docs"},
    {"id": "rfc9449-attestation-dpop-context-20261004", "url": "https://www.rfc-editor.org/rfc/rfc9449.html", "type": "official_docs"},
    {"id": "rfc7638-attestation-key-equality-20261004", "url": "https://www.rfc-editor.org/rfc/rfc7638.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# OAuth client attestationとDPoPを一つのproofへまとめる境界

## 問いと採用判断

client instanceを保証するattestationと、tokenの持ち出しを制限するDPoPを併用するとき、proofを一つに減らしてよい条件は何か。また旧draftのchallenge処理を残したまま更新すると、どこで相互運用が崩れるか。

本稿は、試験導入済みのclient/AS/RSを更新する際に、**採用draft、proof mode、鍵、freshnessの経路を一組で固定する**方針を示す。通常のDPoP導入案内は[別文書](dpop-proof-binding-nonce-replay.md)にある。本稿の追加価値は、attestation用の鍵とDPoP用の鍵を結合する条件、および2026年9月のdraft更新によるchallenge経路の変更である。

## 版と実際に変わった契約

[Draft 11][draft11]は2026-09-03公開、期限表示は2027-03-07。2026-10-04に開いた[Datatracker][status]はActive Internet-Draft、WG stateはIn WG Last Call、IESG stateはI-D Existsだった。**RFC承認済みではない**。本稿の2026-11-03は再調査日であり、draft自身の失効日とは別である。

[Draft 10 §6.1・§7.3][draft10]（2026-07-06）は、challenge endpoint等で得たattestation challengeをcombined DPoP proofのnonceへ照合する構成だった。[Draft 11 §5.2・§6・§7.4とAppendix A][draft11]は、combined modeのserver-provided freshnessをDPoPの経路へ分離した。旧版の取得値をそのまま新しいnonce storeへ流す実装は、更新対象になる。

| 処理 | Draft 11で使用する経路 |
|---|---|
| 専用Client Attestation PoP JWT | `OAuth-Client-Attestation-Challenge`等の値を`challenge` claimへ入れる |
| DPoP combined mode | `DPoP-Nonce`の値をDPoPの`nonce` claimへ入れる |
| challenge endpointが両値を返す場合 | JSONの`attestation_challenge`とheaderの`DPoP-Nonce`を別々に扱う |
| combined modeで期待nonceが不正・欠落 | `use_dpop_nonce`と新しい`DPoP-Nonce`を使用する |

この変更はDPoP自体が新しくなったという意味ではない。RFC 9449のnonce機構をattestation側のモード選択に正しく接続する変更である。

## 検証する三つの関係

### 1. attesterへの信頼と、instance鍵の所持を別々に検証する

[Draft 11 §7.1–7.3][draft11]では、attestationは既知で信頼するClient Attesterで検証し、combined proofの公開鍵は検証済みattestationの`cnf.jwk`と一致させる。署名成功だけで任意のattesterを採用しない。

modeは送信headerの組にも影響する。専用の`OAuth-Client-Attestation-PoP`があれば通常modeとして検証し、DPoPを併送するなら独立に検証する。この構成では両用途の鍵一致は要求されない。一方combined modeは専用PoP headerを使わず、一つのDPoP proofで所持を証明する。ASが`attest_jwt_client_auth_dpop`非対応なら、そのcombined requestを拒否する。[§5.2・§7][draft11]

これは「DPoP対応ASなら自動的にclient attestationにも対応」という契約ではない。本稿では、通常modeとcombined modeを、同じ認証成功フラグへ潰さず記録する設計を採る。

### 2. 公開鍵の一致はJSON文字列の一致ではない

[RFC 7638 §3・§3.2][thumbprint]のJWK thumbprintは、鍵を表す必須memberだけを決められた順序・表現でhashする。たとえば対象のEC公開鍵では`crv`、`kty`、`x`、`y`を使い、任意の`kid`や`alg`を含めたJSON全体のhashではない。これによりoptional memberや並び順が違っても同じ鍵を識別できる。[Draft 11 §7.3][draft11]も、比較はthumbprint等で行い、JWK全体のcanonical representation比較を意味しないと補足する。

本稿の設計では、JOSEライブラリで鍵型・必須値を検証してから、対応したthumbprint処理へ渡す。`kid`一致だけで通す、未知の鍵型をECとして読む、optional memberを無視したことを署名algorithmの許可と混同する、といった短絡を避ける。鍵同一性、algorithm policy、署名検証は別の判定である。

### 3. tokenと実際のHTTP requestの結合も残る

[RFC 9449 §4.3・§6–7][dpop]に従うDPoP検証は、combined化しても省略しない。resource requestではaccess tokenの鍵結合と`ath`の照合も必要になる。attestationとproofの鍵が一致しても、別token・別method・別URIのproofを受理する理由にはならない。

[RFC 9449 §10–10.1][dpop]の`dpop_jkt`は鍵thumbprintを伝える値で、単独では秘密鍵の所持proofではない。combined modeに実際のDPoP proofが必要なことと整合する。`dpop_jkt`だけの要求を、client attestationのPoPまで検証した成功として記録しない。

## 移行と失敗処理（本稿独自の配備案）

以下は仕様の新しいMUSTではなく、Draft 10実装を含む試験環境を安全に更新するための設計である。

### A. 接続先ごとに互換性を決める

client/AS/RSそれぞれについて、実装版、採用draft、通常modeかcombined modeか、許可algorithm、信頼するattester、鍵の管理主体を台帳にする。metadataで見える機能対応と、ローカルに採用した仕様revisionを分ける。DPoPの対応表示だけでdraftの一致を推測せず、相手実装の版と試験結果を使う。

更新時にすべてのheaderを同時に付けて「通った方を使う」実装は避ける。専用PoP headerの追加はmode分岐を変えるため、combined modeの鍵一致試験を実際には通していない可能性がある。正常な二proof構成を選ぶなら、その理由と独立した鍵検証を明示する。

旧版対応が必要なら、対象接続先と終了日を決めた互換処理に閉じ込める。nonce不一致を見て自動的に旧challenge値へfallbackする方式は採らない。今回確認したmetadataに、draft番号を自動交渉する仕組みがあるとは主張しない。

### B. 更新する対象をエラーごとに分ける

| 観測 | この配備で行う処理 | 避ける処理 |
|---|---|---|
| `use_dpop_nonce`と新nonce | 発行元に対応するnonceを保存し、新しいDPoP proofで限定再試行 | 古いJWTそのものの再送、attestation challengeの代入 |
| `use_attestation_challenge` | 通常modeの専用PoP処理へ限定。combinedとして送ったなら契約不一致を調べる | すべてを同じnonceエラーに変換する |
| `use_fresh_attestation` | attesterからの再取得が必要かを判断し、attestationの鮮度を更新する | 新しい`jti`だけで古いattestationの鮮度まで回復した扱いにする |
| 鍵不一致・未対応mode | 認証失敗として止め、設定・鍵generation・相手の対応版を確認する | 鍵一致検査やattestationを外して再試行する |

Draft 11 §7.4はproofのnonceとattestation自身の鮮度のエラーを区別している。[RFC 9449 §8–9][dpop]ではnonceは発行サーバーごとに扱う。そこで本稿では、ASとRS、通常modeのchallengeとDPoP nonceを別の保存領域にし、任意の一つの`latest_nonce`変数へ集約しない。

再試行には回数・時間の上限を置き、失敗理由を残す。nonce取得に成功したことを業務処理の成功にしない。特に書込みAPIでは、新proofの生成と業務上の二重実行防止は別の責務にする。エラー応答のない通信切断をnonce challengeと同じ自動再試行へ流さない。

### C. 鍵とattestationを整合した単位で切り替える

本稿の案では、利用中の鍵handle、対応attestation、mode、適用policy版を一つの設定generationにまとめる。鍵だけを先に差し替えたrequestが旧attestationと混ざらないよう、更新中requestの寿命を管理する。

[Draft 11 §10.6][draft11]はinstance鍵のrotation protocolを定義せず、新しい鍵には新しいattestationの取得が必要とする。そこから、既発行tokenやrefresh tokenが新しい鍵へ自動移動できるとは導かない。移行時には旧鍵が必要なtokenの残存期間と破棄方針を別途確認する。秘密鍵、proof JWT、attestation全体、tokenを監査ログへ出さず、検証結果とgenerationの相関を残す。

### D. client認証成功とAPI保護成立を分ける

[RFC 9449 §5][dpop]では、ASが返すaccess tokenの`token_type`を確認する必要があり、DPoP保護が必要なclientはDPoP以外の応答を破棄する。combined client認証が通った事実だけで、返されたaccess tokenも期待どおりsender-constrainedだとは決めない。

本稿ではtoken受理、RS側の鍵結合、業務認可をそれぞれ観測する。attestationの採用はユーザーの同意や個々のデータへのアクセス権限を置き換えるものとして扱わない。

## 導入時の反例（未実行の試験案）

| ケース | 確認する境界 |
|---|---|
| challenge応答に異なる`attestation_challenge`と`DPoP-Nonce` | combined proofは後者を`nonce`へ使い、値を混同しない |
| combined requestへ専用PoP headerを追加 | 通常modeへの分岐を認識し、combined試験成功として集計しない |
| attestation鍵A、DPoP鍵B、両署名は正しい | combined modeでは鍵不一致を拒否する |
| 同じ鍵で`kid`・member順・空白のみ異なる | JWK文字列一致に依存せず、鍵同一性を正しく判定する |
| `dpop_jkt`だけでDPoP headerなし | combined PoP成立としない |
| 正しい新nonceと古いattestation | nonce処理成功だけでattestationの鮮度チェックを省かない |
| 更新中に鍵generationだけが新しくなる | 旧attestationと混在した送信を防ぐか、失敗として検知する |
| AS nonceをRSへ流用 | RS発行のnonceと混同せず、誤った発行元を拒否する |
| client認証成功だが`token_type=Bearer` | DPoP必須policyでは利用開始しない |

検索evalは文書が検索できることだけを確認する。この表の暗号検証・通信・相互運用を実行した結果ではない。

## 適用範囲・未確認事項・provenance

- Draft 11の一般形を対象にする。特定wallet profile、OSのplatform attestation、ハードウェア鍵の品質、clientとattester間の発行手順は未検証。仕様名にattestationがあるだけで端末全体の安全を保証しない
- 製品・SDKのDraft 11対応、実際のnonce競合、proxy/header上限、鍵rotation時のrefresh token継続性、相手profileによるclaim変更は未確認。運用開始前の接続試験が必要
- 基本DPoPの詳細なreplay store設計やOAuth grant lifecycleは既存文書へ分離し、本稿では新しいcombined境界に限った。失効伝播・業務のexactly-once保証は提供しない
- 全5件をnative webで2026-10-04 UTCに開き、本文・版・日付を確認した。Draft 10/11とRFCのcopyright noticeはIETF Trust / BCP 78を確認。Datatracker状態ページ固有の再利用ライセンスは未確認でcatalogをunknownとした
- 独自の日本語要約・比較・配備案であり、原文コード・JWT例・鍵・図をコピーしていない。repository analysisは行っていないためcommit固定は該当しない。既存source recordの日付を延長せず、今回の確認範囲の新recordを追加した

[draft11]: https://datatracker.ietf.org/doc/html/draft-ietf-oauth-attestation-based-client-auth-11
[draft10]: https://datatracker.ietf.org/doc/html/draft-ietf-oauth-attestation-based-client-auth-10
[status]: https://datatracker.ietf.org/doc/draft-ietf-oauth-attestation-based-client-auth/
[dpop]: https://www.rfc-editor.org/rfc/rfc9449.html
[thumbprint]: https://www.rfc-editor.org/rfc/rfc7638.html

---
{
  "id": "oauth-private-key-jwt-audience-replay-grant-boundary",
  "title": "OAuth private_key_jwt: audience移行・一回利用・grant再試行の境界",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 7521/7523 (May 2015); OIDC Core 1.0 errata set 2 (2023-12-15); FAPI 2.0 Final (2025-02-22); RFC 8725 (February 2020); draft-ietf-oauth-rfc7523bis-11 compared as work in progress; status checked 2026-10-23 UTC",
  "tags": [
    "research-domain:security",
    "oauth",
    "private_key_jwt",
    "client_assertion",
    "audience",
    "issuer",
    "replay",
    "jti",
    "client-authentication+jwt",
    "jwt-bearer",
    "retry",
    "FAPI"
  ],
  "sources": [
    {
      "id": "rfc7523-client-assertion-profile-20261023",
      "url": "https://www.rfc-editor.org/rfc/rfc7523.html",
      "type": "official_docs"
    },
    {
      "id": "rfc7521-client-assertion-binding-20261023",
      "url": "https://www.rfc-editor.org/rfc/rfc7521.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-core-private-key-jwt-errata2-20261023",
      "url": "https://openid.net/specs/openid-connect-core-1_0.html",
      "type": "official_docs"
    },
    {
      "id": "fapi2-client-assertion-issuer-audience-20261023",
      "url": "https://openid.net/specs/fapi-security-profile-2_0-final.html",
      "type": "official_docs"
    },
    {
      "id": "rfc8725-client-assertion-type-separation-20261023",
      "url": "https://www.rfc-editor.org/rfc/rfc8725.html",
      "type": "official_docs"
    },
    {
      "id": "oauth-rfc7523bis11-audience-migration-20261023",
      "url": "https://datatracker.ietf.org/doc/html/draft-ietf-oauth-rfc7523bis-11",
      "type": "official_docs"
    },
    {
      "id": "oauth-rfc7523bis-status-20261023",
      "url": "https://datatracker.ietf.org/doc/draft-ietf-oauth-rfc7523bis/",
      "type": "official_docs"
    },
    {
      "id": "rfc7523-errata-status-20261023",
      "url": "https://errata.rfc-editor.org/search/?rfc_number=7523",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# private_key_jwt: 署名成功・認証の一回利用・grant処理を分ける

## 問いと採用判断

秘密鍵を送らないclient認証へ移行したとき、正しい署名のJWTなら任意のAS endpointで使え、通信失敗時にもそのまま再送できるのか。判断に必要なのは、**採用profile、意図したAS、assertionの消費状態、grantの処理結果**の四つである。

本書は`private_key_jwt`を使うclientとauthorization server（AS）の境界を扱う。[ID Tokenの検証](../oidc/id-token-validation.md)、[PAR/JARのrequest_uri確定](par-jar-request-uri-redemption-boundary.md)、[DPoP proof](dpop-proof-binding-nonce-replay.md)とは検証対象が違う。既存文書の`client_assertion`とRequest Objectの区別を前提に、未収録だったaudienceのprofile差と認証JWTの一回利用へ絞る。

## 確認した仕様上の契約

### 1. client authenticationはgrantそのものではない

[RFC 7523 §1–2][jwt]の二つのbindingを分ける。

| 目的 | 送信するparameter | 何の証拠として処理するか |
|---|---|---|
| JWTでclientを認証 | `client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer` と `client_assertion` | clientの認証。`grant_type`には別途選択したgrantを指定する |
| JWTをauthorization grantとして交換 | `grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer` と `assertion` | grantの検証。client認証が必要かは別の方針で決める |

[RFC 7521 §4.2–4.2.1][framework]では、認証assertionとは別に`client_id` parameterを送るなら同じclientを指す必要がある。複数のclient認証方式を同時に使った場合は`invalid_client`になる。したがって、本書ではBasic認証とassertionを同時送信して「通る方を採用」する互換処理を設けない。

JWT grantの失敗は`invalid_grant`、client JWTの失敗は`invalid_client`という区別がある。[RFC 7523 §3.1–3.2][jwt] これは一つのエラー応答から、相手の内部で起きたすべての処理を確定できるという意味ではない。

### 2. RFC 7523の任意要件をOIDCのprivate_key_jwtへ持ち込まない

- [RFC 7523 §3・§6][jwt]では`jti`とreplay protectionは任意。`exp`は必須で、許容する時計ずれを考慮して期限を検証する。最大寿命や一回利用条件は相互運用上の合意事項でもある。
- [OIDC Core §9][core]の`private_key_jwt`では、`iss`と`sub`は両方とも認証対象の`client_id`。`aud`・`jti`・`exp`はREQUIRED、`iat`はOPTIONAL。再利用条件を当事者間で交渉した例外を除き、一回利用が要求される。

つまり、RFC 7523だけで許容される実装を、そのままOIDC `private_key_jwt`適合と呼ばない。JWTに`jti`があることと、再利用が拒否されることも別である。例外的な再利用の交渉方式はCoreの規定範囲外であり、通信timeoutを暗黙の交渉とみなす根拠にはならない。

### 3. audienceは「送信先URLなら何でもよい」ではない

| 適用する文書 | 確認したaudience条件 |
|---|---|
| RFC 7523 §3・§5 | ASを識別する値が必要。token endpoint URLはMAYであり、具体値は合意する。profileで別段の定めがなければ単純文字列比較 |
| OIDC Core §9 | ASが意図したaudienceか検証する。token endpoint URLはSHOULDであり、唯一の普遍的な必須値ではない |
| FAPI 2.0 Final §5.3.2.1・§5.3.3.1 | client認証assertionのaudはAS issuer identifierの文字列。配列中の要素として送らない |

出典: [RFC 7523][jwt]、[OIDC Core][core]、[FAPI 2.0][fapi]。FAPIの制約は公開済みFinalの要件であり、次節のdraftを採用した場合だけの条件ではない。例えばissuerが`https://as.example/tenant-a`、token endpointが`https://as.example/tenant-a/token`なら、FAPIでは前者を文字列で使う。両値を配列に並べる互換策はFAPI条件を満たさない。

### 4. pending updateを発行済みRFCと混同しない

2026-10-23に開いた[Datatracker状態][status]は`draft-ietf-oauth-rfc7523bis-11`をRFC Ed Queue、RFC Editor statusを`blocked: Final Approval Pending`と表示した。状態レコードのLast updatedは2026-10-19、Latest revisionは2026-04-28。一方、[版固定の本文][bis]のヘッダーはPublished 2026-03-26、Expires 2026-09-27である。これらを同じ種類の日付として上書きせず、**未発行の更新候補**として比較する。

[Draft 11 §1・§4–6][bis]で確認した重要点は次の通り。

- client authenticationのaudを唯一のAS issuer identifierへ制限し、token endpoint URLを使わない方向の更新。JWT bearer grantには同じ一律制約を適用していない。
- `typ=client-authentication+jwt`等を推奨するが、`typ`欠落だけを理由に拒否することは非推奨。主たる防御はaudience条件の強化である。
- PARのclient認証にもaudience条件を適用する。旧clientが新条件を強制するASと接続できなくなる移行上の非対称性を扱う。

同draftはmetadata中のendpoint値をaudienceとして別endpointへ送る場合の曖昧さを背景に挙げる。本書は、署名に`typ`を追加するだけの修正をaudience対策の完了とは扱わない。反対に、draftの`typ`推奨を理由に、正しいaudienceを使う既存clientを一律拒否する変更も正当化しない。

## 配備・実装の判断案（本書独自の設計）

以下は仕様の追加MUSTではなく、上記の契約を製品選定・接続試験へ落とすための案である。実装コードや稼働中の設定は本調査では変更していない。

### A. profileと検証器を受信JWTから選ばない

接続先ごとに、信頼するAS issuer、client_id、採用profileと版、認証方式、許可algorithm、audienceの値と型、assertionの寿命、一回利用方針を記録する。新規に両端を管理する構成ではissuer audienceへの統一を選び、既存接続先には対応確認と移行計画を置く。未知の相手にendpoint audienceとissuer audienceを順に試す自動fallbackは採らない。

ASは未検証の`iss`や`kid`を、限定された登録情報の検索候補として扱うにとどめる。JWT中のURLから新しい鍵取得先を信頼したり、受信した`alg`だけで許可方式を広げたりしない。[RFC 8725 §3.1・§3.8–3.12][bcp]はalgorithm・鍵とissuerの関連、受信claimの扱い、用途ごとに誤ったJWTを排除する検証規則を要求・推奨している。

本案ではclient認証、JWT grant、ID Token、Request Objectの検証入口を分け、必要なclaim・値・鍵の用途を明示する。`typ`の有無だけで入口を切り替えない。同じ署名鍵を共有する設計では、別用途JWTが必要claimまで満たさないかを試験し、区別できないなら鍵や受理条件を分離する。一般的な`typ=JWT`は用途分離の証明ではない。

### B. 一回利用の記録は共有し、認証成功前に原子的に確定する

本案は交渉済みの再利用例外がないOIDC構成を対象にする。

1. サイズ・構造・登録client・署名・algorithm・audience・時刻・必要claimを検証する。未検証JWTの任意の`jti`を無制限にstoreへ投入しない。
2. client認証を成功としてgrant処理へ進める直前に、利用済み印を原子的に作る。対象は例えば`(ASの認証領域, 検証済みclient_id, jti)`。同じassertionを受理し得るPAR/token endpointや複数replicaで同じ判定領域を使う。
3. read後にwriteする二段階判定ではなく、未使用の場合だけ一つの処理が確定できる操作を使う。`kid`やinstance名を識別子に足して、鍵rotationや別replicaへの配送で同じ`jti`を再利用可能にしない。
4. 利用済み印は、そのassertionが実際に受理され得る終了時刻まで保持する。許容leewayを採用する構成ではそれも含める。TTL削除のタイミングをJWT期限検証の代用にしない。
5. store障害・書込み結果不明・有効期間内のevictionで一回利用を確認できなければ、この方式での認証を止める。単一instanceのローカルcacheへ黙ってfallbackしない。

認証後にgrantが失敗したり応答が失われたりしても、利用済み印を消して「未使用」へ戻さないのが本案の方針である。仕様はこの保存モデル・分散アルゴリズム・障害回復手順を定めていない。製品のreplay storeがどこまで共有され、failover後も維持されるかを別途検証する。

[RFC 7521 §8.2][framework]の盗難assertionの脅威は、一回利用の保存だけでは消えない。攻撃者が正当clientより先に利用する競争も残る。本案ではTLS、assertionの非記録化、短い許容寿命を組み合わせ、`jti`だけで漏えい対策が完了したとは評価しない。

### C. 新しいassertionとgrantの再実行許可は別

| 観測 | 本案での扱い | 避ける判断 |
|---|---|---|
| audience・鍵・profile不一致の`invalid_client` | 設定と相手の契約を確認して停止。修正後は新しいassertionを生成 | 署名検証・audience制約を外す、別認証方式へdowngrade |
| 送信前の失敗で、要求が相手へ届いていないと確認できる | 有効なgrantであることを確認し、新しい`jti`で再送を準備 | 長時間cacheしたassertionを共有して再利用 |
| 送信後に応答なし | 認証の消費状態とgrantの処理結果をどちらも未確定として扱う | 同じJWTなら安全な再送、新しいJWTなら元grantも未使用、と推測 |
| 新しいassertionは受理されたが`invalid_grant` | grant固有の期限・使用済み・失効・結合条件を調べる | 認証成功を元のtoken発行成功・復元成功として報告 |

再試行用に新しい認証JWTを作れても、authorization codeやrefresh tokenの状態は巻き戻らない。[code/PKCE](authorization-code-pkce-state-redirect.md)や[refresh token rotation](refresh-token-rotation-reuse-detection.md)の契約を別に確認し、必要なら新しい認可手順へ戻る。client credentials等の別grantでも、新しいtoken発行が元の応答と同じになる保証を本書は与えない。

### D. 観測は認証と発行を別々に残す

診断項目は、profile版、ASの内部識別子、audience不一致、型不一致、署名失敗、replay競合、store不確定、grant失敗に分ける。秘密鍵・assertion全文・tokenをログへ出さず、ローカルな追跡IDで相関する。認証用`jti`を業務上のidempotency keyへ転用しない。

`private_key_jwt`の採用だけでaccess tokenがsender-constrainedになるとも判定しない。FAPIはclient認証とMTLS/DPoPによるaccess token保護を別要件としている。[FAPI §5.3.2.1・§5.3.3.1][fapi] 本案でも「client認証成功」「grant処理成功」「発行tokenの保護条件」を別々の確認点にする。

## 管理下で行う受入れ試験案（未実行）

| ケース | 確認する結果 |
|---|---|
| RFC 7523だけを想定した`jti`欠落JWTをOIDC private_key_jwtへ入力 | REQUIRED claim欠落を拒否する |
| 正しいassertionと異なる`client_id` parameter | client入替えとして拒否する |
| Basicとclient_assertionを併送 | 一つだけが成功しても多重client認証として拒否する |
| FAPIのissuerを単一要素配列で送る | 値が同じでも配列を拒否する |
| issuer audienceへ移行後、token endpoint URLだけを送る | 移行済みpolicyが広がらず拒否する |
| Draft 11方針で正しいaudience、explicit typなし | 欠落だけを理由に拒否しない。別の不正は通常どおり検証 |
| 同じ鍵のRequest ObjectやJWT grantをclient_assertionへ差し替える | 用途違いを拒否し、署名成功だけで通さない |
| 同じassertionを二つのreplicaへ同時送信 | 認証の消費確定は一方だけ |
| 同じjtiをPAR後のtoken endpoint、または別kidで再送 | 本案の共有領域では一回利用をすり抜けない |
| 利用済み印のeviction・store障害・write結果不明 | 未使用とみなして認証を継続しない |
| 認証消費後にgrant処理を失敗させる | 利用済み印は残る。再試行は新JWTとgrant固有の判断へ分離 |
| token応答を失わせ、新しいassertionで同じcodeを送る | 新しいjtiがcodeの再利用を許す根拠にならない |

検索evalは本文への到達性を確かめるだけで、上記の暗号検証・相互運用・分散競合を実行した結果ではない。

## 版・provenance・未確認事項

- 全8ページを2026-10-23 UTCにnative webで開いた。RFC 7523の[errata検索][errata]は該当なしを表示した。他RFCのerrata一覧や将来の改訂まで不存在を確認したわけではない。
- draftの状態は変わり得るため本書の再確認期限を2026-11-22とする。これはdraft本文の失効日でも、RFC刊行予定日でもない。未発行候補の記述を既存RFCへ遡及適用しない。
- 製品・SDKごとのaudience互換性、issuer取得時の信頼設定、client登録と鍵管理、replay storeの耐障害性、token応答喪失時の製品固有の復旧は未検証。全FAPI要件の適合性評価でもない。
- RFC 7521/7523/8725のCopyright NoticeはIETF Trust / BCP 78とCode ComponentsのSimplified BSD条件、Draft 11はRevised BSD条件を確認。Core Appendix C / FAPI Appendix BはOIDFへの帰属を伴う仕様策定・実装目的の利用条件を確認した。状態・errata検索ページ固有の再利用licenseは未確認でcatalogをunknownとする。
- IETFとOpenID Foundationを出典とする独自の日本語要約・設計案であり、団体の推奨や認証を意味しない。原文コード・鍵・JWT例・図・長文は転載せず、repository analysisやmodule昇格も行っていないためcommit固定は該当しない。既存catalogを変更せず新しい取得記録を追加する。

[jwt]: https://www.rfc-editor.org/rfc/rfc7523.html
[framework]: https://www.rfc-editor.org/rfc/rfc7521.html
[core]: https://openid.net/specs/openid-connect-core-1_0.html
[fapi]: https://openid.net/specs/fapi-security-profile-2_0-final.html
[bcp]: https://www.rfc-editor.org/rfc/rfc8725.html
[bis]: https://datatracker.ietf.org/doc/html/draft-ietf-oauth-rfc7523bis-11
[status]: https://datatracker.ietf.org/doc/draft-ietf-oauth-rfc7523bis/
[errata]: https://errata.rfc-editor.org/search/?rfc_number=7523

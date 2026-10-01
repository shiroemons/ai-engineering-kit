---
{
  "id": "oauth-dpop-proof-binding-nonce-replay",
  "title": "OAuth DPoP: access token の鍵結合、nonce と replay 拒否の境界",
  "kind": "knowledge",
  "technology": "oauth",
  "version": "RFC 9449 (2023-09) + RFC 9700 / BCP 240 (2025-01); verified 2026-10-01",
  "tags": [
    "research-domain:security",
    "DPoP",
    "sender-constrained",
    "ath",
    "cnf",
    "jkt",
    "nonce",
    "replay"
  ],
  "sources": [
    {
      "id": "rfc9449-dpop-proof-nonce-20261001",
      "url": "https://www.rfc-editor.org/rfc/rfc9449.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9700-access-token-replay-20261001",
      "url": "https://www.rfc-editor.org/rfc/rfc9700.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-12-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# OAuth DPoP の鍵結合・nonce・replay

## 調査した問い

盗まれた access token を別の HTTP request で使わせないために、resource server は何を照合し、再送をどこまで拒否するべきか。既存の [refresh token rotation](refresh-token-rotation-reuse-detection.md) が対象外としていた sender-constrained の具体を扱う。新しい仕様変更の発見ではなく、RFC 9449 と Security BCP の既存契約を 2026-10-01 に確認した記録である。

## 確認できた契約

### 採用判断と認可の境界

[RFC 9700 §2.2.1](https://www.rfc-editor.org/rfc/rfc9700.html#section-2.2.1) は access token の sender-constraining を SHOULD とし、DPoP と mutual TLS を例示する。[§2.3](https://www.rfc-editor.org/rfc/rfc9700.html#section-2.3) と [§4.10.2](https://www.rfc-editor.org/rfc/rfc9700.html#section-4.10.2) では audience・resource・action の制限も SHOULD とする。RS は各 request の対象・権限を検証するので、鍵を持つことだけで全 API を許可するものではない。§4.10.1 の攻撃モデルでは token と鍵を同時に使える攻撃者への防御には限界がある。

### Proof と token を別々に検証して結合する

[RFC 9449 §4.2–4.3、§6–7](https://www.rfc-editor.org/rfc/rfc9449.html#section-4.3) の確認項目は次の通り。

- DPoP header は単一の正しい JWT。`typ=dpop+jwt`、許可した非対称 `alg`、公開 `jwk` による署名を照合する。秘密鍵を含む JWK は不可
- `htm` は実メソッド、`htu` は query/fragment を除いた URI と照合。時刻の許容窓と、要求済みなら `nonce` も検証する
- resource request では `ath`（access token の ASCII 表現の SHA-256 を base64url 化）と、JWT access token または introspection 応答の `cnf.jkt` による鍵結合を検証する。token 自体の有効性・権限の確認は別に残る

### nonce は使い捨て proof の代わりにならない

[RFC 9449 §7.3、§8–9、§11.1–11.4](https://www.rfc-editor.org/rfc/rfc9449.html#section-11.1) によれば、proof の受理を有限の有効期間に限定することは MUST、秒〜分程度の短い期間は望ましい選択とされる。`jti` を受理期間中に記録すれば同一 proof の再利用を拒否できるが、全サーバへの共有状態は自動では提供されない。nonce の複数回利用は可能で、nonce の有効期間にわたる `jti` の追跡と重複拒否を組み合わせる。

nonce challenge は AS が `400 use_dpop_nonce`、RS が `401` と `WWW-Authenticate: DPoP` を使い、いずれも `DPoP-Nonce` を返す。nonce は発行サーバごとに区別し、要求後に `nonce` claim を省略した proof は受け付けない。再試行では新しい proof を生成する。DPoP/Bearer 両対応 RS は鍵結合 token の Bearer 利用を拒否する（§7.2）。HTTPS は必須で、body の完全性や XSS 防御を DPoP だけに期待しない。

## 実装へ落とす判断（独自の設計案）

以下は RFC の追加要件ではなく、導入レビューのための設計案である。

1. token validator と proof validator を分け、最後に同一 request について結合を確認する。署名が通っただけで認可へ進む経路を作らない
2. reverse proxy 配下では外向き URI の復元規則を固定し、信頼してよい proxy と header を限定する。`htu` 不一致を回避するために比較を無効化せず、port・scheme・path の正規化を fixture で検証する
3. replay store は原子的な「未登録なら登録」を使い、URI・鍵の識別・`jti` を区別する。複数 replica への同時送信を受けた際の保証、障害時の fail-closed 方針、保存容量と期限を決める。RFC に一律の保存秒数があるとは扱わない
4. nonce の保存は発行元単位にし、AS と RS を共用しない。nonce challenge の連続回数に上限を設け、通常の認証失敗・期限切れと区別する
5. business idempotency key と proof の `jti` は別物として扱う。同じ業務リクエストの安全な再試行でも proof は更新し、業務の二重実行対策は API 側で保持する

## レビューで試す境界（テスト案、未実行）

- 正しい token と別鍵の proof、正しい鍵と別 token の `ath`、method/path 不一致が拒否されるか
- 期限内の同一 `jti` を別 replica へ同時送信しても採用した replay 方針が守られるか
- 古い nonce、別 RS の nonce、nonce 要求後の省略、DPoP-bound token の Bearer 提示を区別できるか
- proxy 経由と直結で外向き URI が一致し、正常な再試行は新 proof で成功するか

## 適用範囲・未確認

RFC 9449（2023年9月）と RFC 9700（2025年1月）に限定する。言語別 SDK、認可サービスの既定値、CORS 設定、PAR/code binding、proof 許容窓の具体値は未検証。本稿の検索 eval は文書発見用で、OAuth 実装の適合試験ではない。再確認期限は official_docs の90日で 2026-12-30。

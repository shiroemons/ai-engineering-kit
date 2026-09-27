---
{
  "id": "oidc-rpinitiated-logout-session-termination",
  "title": "OIDC RP-Initiated Logout with front-channel and back-channel logout session termination",
  "kind": "knowledge",
  "technology": "oidc",
  "version": "RP-Initiated Logout Final (2022-09-12) + Front-Channel Logout Final (2022-09-12) + Back-Channel Logout Final incorporating errata set 1 (2023-12-15)",
  "tags": ["research-domain:security", "oidc", "logout", "rp-initiated", "end_session_endpoint", "id_token_hint", "post_logout_redirect_uri", "frontchannel_logout_uri", "backchannel_logout_uri", "logout_token", "sid", "session", "termination"],
  "sources": [{"id": "oidc-rpinitiated-logout-1-0", "url": "https://openid.net/specs/openid-connect-rpinitiated-1_0.html", "type": "official_docs"}, {"id": "oidc-frontchannel-logout-1-0", "url": "https://openid.net/specs/openid-connect-frontchannel-1_0.html", "type": "official_docs"}, {"id": "oidc-backchannel-logout-1-0", "url": "https://openid.net/specs/openid-connect-backchannel-1_0.html", "type": "official_docs"}],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# OIDC RP-Initiated Logout with front-channel and back-channel logout session termination

Relying Party (RP) 起点で OpenID Provider (OP) の session を終了し、同じ OP session を使う他 RP の session 終了まで波及させる手順。RP-Initiated Logout の `end_session_endpoint` への redirect、Front-Channel Logout の iframe 描画、Back-Channel Logout の `logout_token` 直接配送の3仕様で構成する。

## 要点 (文書化された事実)

### RP-Initiated Logout: logout request と OP 側検証 ([RP-Initiated](https://openid.net/specs/openid-connect-rpinitiated-1_0.html))

- RP は User Agent を OP の `end_session_endpoint` へ redirect する。parameter は `id_token_hint` (以前発行された ID Token。RECOMMENDED で付与)、`post_logout_redirect_uri`、`client_id`、`state` である。
- OP の Logout Endpoint は GET と POST の両方を MUST で support する。
- OP は `id_token_hint` の issuer が自 OP であること、および `client_id` が `id_token_hint` の audience (`aud`) と一致することを MUST で validate する。
- OP は `post_logout_redirect_uri` が当該 client の登録済み `post_logout_redirect_uris` と exactly match しない限り redirect してはならない (MUST NOT)。
- OP は post-logout redirection の前に、その OP session で login 中の他 RP へ logout を notify する (Front-Channel / Back-Channel の機構を使う)。

### Front-Channel Logout: iframe による session clear ([Front-Channel](https://openid.net/specs/openid-connect-frontchannel-1_0.html))

- OP は登録済み `frontchannel_logout_uri` を持つ各 RP について、当該 URI を iframe で render する。query として `iss` と `sid` (Session ID) を付与してよい (MAY) が、付与する場合は両方とも付け、付けない場合は両方とも付けない (both-or-neither)。
- RP は iframe への GET を受けたら当該 session の cookie・local storage を clear する。応答には `Cache-Control: no-store` を返す。
- 登録・Discovery 項目は `frontchannel_logout_uri`、`frontchannel_logout_session_required`、および OP 側の `frontchannel_logout_supported` / `frontchannel_logout_session_supported` である。`sid` は session を識別する Session ID claim として定義される。

### Back-Channel Logout: logout_token の直接配送と検証 ([Back-Channel](https://openid.net/specs/openid-connect-backchannel-1_0.html))

- OP は登録済み `backchannel_logout_uri` へ signed Logout Token を form-encoded の `logout_token` parameter で直接 POST する (User Agent を経由しない server-to-server 通信)。
- Logout Token は `iss`・`aud`・`iat`・`exp`・`jti`・`events` (値 `http://schemas.openid.net/event/backchannel-logout` を含む) が REQUIRED、`sub`・`sid` が OPTIONAL (ただし少なくとも一方の存在が必要)、`nonce` が PROHIBITED である。
- RP は仕様の 11-step validation を MUST で実行する。成功時は HTTP 200 または 204、失敗時は 400 を返す。
- 登録・Discovery 項目は RP 側の `backchannel_logout_uri`・`backchannel_logout_session_required`、および OP 側の `backchannel_logout_supported` / `backchannel_logout_session_supported` である。

## 推奨方法 (上記からの設計上のまとめ)

- logout 開始時は `id_token_hint` を付けて `end_session_endpoint` へ送り、`post_logout_redirect_uri` を使う場合は登録値との完全一致を事前に確認する。`state` で post-logout 後の RP 側状態を維持する。
- 他 RP の session 終了が必要なら、RP-Initiated だけでなく Front-Channel と Back-Channel のどちらを各 RP が support するかを登録情報で把握する。server-to-server で確実に届けたい RP には Back-Channel の `backchannel_logout_uri` を登録し、browser session の cookie・storage 清掃が必要な RP には Front-Channel の `frontchannel_logout_uri` を登録する。
- Back-Channel の受信側は Logout Token の 11-step validation を既製 JWT 検証に載せ、成功時のみ session を破棄して 200/204 を返し、検証失敗時は session を残して 400 を返す。`nonce` 付き token や `sub`・`sid` の両欠如は拒否する。
- Front-Channel の受信側は iframe GET で session 識別 (`sid`・`iss`、または自 cookie) から対象 session のみを破棄し、`Cache-Control: no-store` で応答する。

## 避ける使い方

- `id_token_hint` なしで logout し、OP が要求する issuer・audience 照合を回避する前提で作ること。検証は MUST である。
- 未登録または完全一致しない `post_logout_redirect_uri` への redirect を期待すること。OP は MUST NOT で redirect しない。
- Front-Channel で `iss` のみ・`sid` のみの query を送受信すること。仕様は両方またはなし (both-or-neither) と定める。
- Back-Channel の Logout Token に `nonce` を付与・期待すること。PROHIBITED である。
- Back-Channel で `sub` も `sid` もない Logout Token を受け入れること。少なくとも一方の存在が必要である。
- Back-Channel の検証失敗時に 200/204 を返すこと。失敗時は 400 が規定である。
- Front-Channel の logout 応答を cache 可能にすること。`Cache-Control: no-store` が規定である。

## 適用版と本番での注意

- `RP-Initiated Logout Final (2022-09-12)`、`Front-Channel Logout Final (2022-09-12)`、`Back-Channel Logout Final incorporating errata set 1 (2023-12-15)` の公開ページ (2026-09-27 取得) で確認した。将来も最新とは扱わない。
- 本文は `expires_at` 2026-12-26 (official_docs TTL 90 日)。
- **未確認**: 各 OP・ライブラリが `end_session_endpoint`、`frontchannel_logout_supported` / `backchannel_logout_supported`、11-step validation の各 step をどこまで実装しているか、iframe が third-party cookie 制限下で動作するか、OP の session 管理方式 (sid 発行・保持期間)。実装前に対応製品の公式文書で確認する必要がある。
- 本文中の MUST/MUST NOT/RECOMMENDED/MAY は仕様の normative 表記の引用であり、本リポジトリの推奨事項 (「推奨方法」節) と区別している。

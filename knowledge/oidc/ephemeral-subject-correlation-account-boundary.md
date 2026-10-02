---
{
  "id": "oidc-ephemeral-subject-correlation-account-boundary",
  "title": "OIDC ephemeral subject の境界: Draft 03 の再訪相関・アカウント紐付け・UserInfo照合",
  "kind": "knowledge",
  "technology": "oidc",
  "version": "OpenID Connect Ephemeral Subject Identifier 1.0 Draft 03 (2026-07-10), final-vote notice 2026-09-02; compared with Core 1.0 errata set 2 (2023-12-15); verified 2026-10-02 UTC",
  "tags": [
    "research-domain:security",
    "oidc",
    "ephemeral",
    "subject_type",
    "subject_types_supported",
    "pairwise",
    "public",
    "sub",
    "iss",
    "UserInfo",
    "account-linking",
    "privacy",
    "Draft-03"
  ],
  "sources": [
    {
      "id": "oidc-ephemeral-subject-draft03-20261002",
      "url": "https://openid.net/specs/openid-connect-ephemeral-subject-identifier-1_0-03.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-core-subject-stability-errata2-20261002",
      "url": "https://openid.net/specs/openid-connect-core-1_0.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-ephemeral-final-vote-notice-20260902-20261002",
      "url": "https://openid.net/notice-of-vote-for-proposed-openid-connect-ephemeral-subject-identifier-1-0-final-specification/",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-10-16",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# OIDC ephemeral subject とアカウントの継続性

## 問いと結論

OP が同じ利用者へ毎回異なる `sub` を返すとき、RP は従来の会員データに自動で紐付けてよいか。**ephemeral を交渉した認証では、過去の訪問と同一人物であることを `sub` の継続性に依存して判断しない**。通常の会員ログインと、訪問をまたいだ相関を抑える属性確認は、保存モデルから分ける。本書は後者の試験導入に向けた境界を整理するもので、一般提供や互換性を保証しない。[Draft 03 §1・§4][ephemeral]

## 版と最近の動き

- 読んだ仕様は 2026-07-10 公開の **Draft 03**。版固定URLと可変URLの双方がこの表題・日付であり、Warning は最終標準ではないと明記していた。2026-10-02 UTC の取得時点で確認した版として記録する。[仕様][ephemeral]
- 2026-09-02 の公式告知は、Final Specification の投票期間を9月16〜30日と案内する。投票期間が過ぎたことから可決や正式発行を推測しない。本調査では承認結果を確認できていない。[投票告知][vote]
- 比較対象は Core 1.0 incorporating errata set 2（2023-12-15）。既存の [ID Token validation](id-token-validation.md) の代替ではなく、検証後の識別子の扱いを対象にする。

## 確認した契約

### public・pairwise との違い

Core §8 の `public` は複数clientに同じ `sub` を提示する。`pairwise` は相関の範囲を狭めるが、§8.1 では Sector Identifier ごとに決定的な値を計算する。したがって pairwise も、同一の相関範囲内で再訪を結び付ける用途を保つ。複数RPが同じ sector を共有する構成もある。[Core §8・§8.1][core]

Core §5.7 は通常の識別に `iss` と `sub` の組を使い、`email` や `preferred_username` の一意性を保証しない。ephemeral 拡張を採るRPで、この通常の安定性をそのまま新しい認証要求間へ広げない。[Core §5.7][core] / [Draft §4][ephemeral]

### Draft 03 が変える単位

Draft §4 は認証要求ごとに異なる `sub` を定義する。§1 の認証セッション内の一定性と、別の認証要求での再生成を混同しない。OP は値を再利用してはならず（MUST NOT）、推測成功確率を `2^-128` 以下にする（MUST）。`2^-160` 以下の目標と、想定する期間・規模で十分小さい衝突確率は SHOULD の要件である。長い文字列であるだけでは必要な乱数品質の根拠にならない。[Draft §1・§4・§7][ephemeral]

OP は Discovery の `subject_types_supported` に `ephemeral` を公開し、RP は登録の `subject_type` などで選択する。単にランダムに見える `sub` を観測しただけで ephemeral 対応と判定しない。[Draft §5・§6][ephemeral]

同じ認証に対応する UserInfo の `sub` は、ID Token の `sub` と完全一致を確認する。一致しなければ UserInfo の値を使わない。この照合を「毎回変わる仕様だから」と省略する根拠はない。[Core §5.3.2][core]

## 実装時の設計判断（本書の提案）

1. client 登録に選択した subject type を残し、RP の認証完了処理で通常の会員ログインと ephemeral 用の短期レコードを分ける。トークンの文字列形式を推測する分岐は使わない。
2. 継続的な注文履歴や権限を持つ既存会員へ、メール一致だけで自動リンクしない。継続利用が必要なら、別途設計・認可した明示的な紐付け手順を用意し、その相関がプライバシー目的に与える影響を説明する。
3. 一時レコードには終了条件と保存期限を設ける。認証成功を理由に永続アカウントを毎回新規作成すると、同じ人の登録が増え、アカウント削除や認可の追跡が複雑になる。
4. `sub` を変えても、固定のcookie、メール、端末識別子、詳細な行動ログまで非相関になるわけではない。本書の脅威モデルでは、RPが併せて保持する識別情報も別に点検する。仕様だけでシステム全体の匿名性を達成したとは判定しない。
5. 既存の署名・issuer・audience・時刻・nonce 検証を維持したまま導入する。未知の subject type を無言で public に読み替える実装は避ける。

## 確認する境界ケース（提案、実行結果ではない）

- 同じアカウントの独立した認証要求で `sub` が変わっても、以前の会員権限を継承しない
- ID Token と対応する UserInfo の `sub` 不一致を拒否する
- 別 issuer の同じ文字列 `sub` を同じ主体に混同しない
- 登録で未交渉の type や provider の非対応を、意図したエラーとして扱う
- プロセス再起動や複数発行ノードで値が再利用されないこと、乱数源・衝突予算をOP側で検証する
- メールなどの補助claimが同じでも自動リンクせず、不要な相関ログを残さない

このリポジトリに追加する eval は検索到達性の検証であり、上記のOIDC適合性試験ではない。

## 制約・未確認事項

- Final 投票結果、製品・ライブラリの対応状況、実際のOP/RP間接続は未確認。仕様策定中なので再確認期限を2026-10-16と短くした。正式採用前には承認状態・版差分・実装の対応を再調査する。
- refresh、再認証、logout、複数タブにわたる具体的なセッション寿命を、この短い拡張仕様だけから一律に決めない。採用するOPとプロファイルで別途確認する。
- 仕様は OpenID Foundation の Notices を確認した。投票告知の再利用ライセンスは不明。独自要約だけを記録し、コード・仕様本文の転載やmodule昇格は行っていない。

[ephemeral]: https://openid.net/specs/openid-connect-ephemeral-subject-identifier-1_0-03.html
[core]: https://openid.net/specs/openid-connect-core-1_0.html
[vote]: https://openid.net/notice-of-vote-for-proposed-openid-connect-ephemeral-subject-identifier-1-0-final-specification/

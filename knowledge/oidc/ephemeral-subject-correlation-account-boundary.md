---
{
  "id": "oidc-ephemeral-subject-correlation-account-boundary",
  "title": "OIDC ephemeral subject 1.0 Final: 再訪相関・refresh・アカウント紐付けの境界",
  "kind": "knowledge",
  "technology": "oidc",
  "version": "OpenID Connect Ephemeral Subject Identifier 1.0 Final (2026-10-07), approval announcement 2026-10-08; compared with Draft 03 (2026-07-10) and Core 1.0 errata set 2 (2023-12-15); verified 2026-10-24 UTC",
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
    "refresh",
    "account-linking",
    "privacy",
    "Final"
  ],
  "sources": [
    {
      "id": "oidc-ephemeral-subject-final-20261007-20261024",
      "url": "https://openid.net/specs/openid-connect-ephemeral-subject-identifier-1_0-final.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-ephemeral-subject-draft03-comparison-20261024",
      "url": "https://openid.net/specs/openid-connect-ephemeral-subject-identifier-1_0-03.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-core-subject-refresh-errata2-20261024",
      "url": "https://openid.net/specs/openid-connect-core-1_0.html",
      "type": "official_docs"
    },
    {
      "id": "oidc-ephemeral-final-approval-20261008-20261024",
      "url": "https://openid.net/openid-connect-ephemeral-subject-identifier-1-0-final-specification-approved/",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2027-01-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# OIDC ephemeral subject とアカウントの継続性

## 問いと結論

OP が同じ利用者へ認証要求ごとに異なる `sub` を返すとき、RP は従来の会員データへ自動で紐付けてよいか。**ephemeral を交渉した認証では、独立した再訪を `sub` の継続性で同一人物と判断しない**。継続的な会員ログインと、訪問間の相関を抑える属性確認は、保存モデルから分ける。[Final §1・§4][ephemeral]

2026-10-07 公開版は **1.0 Final** であり、翌日の公式告知で承認結果も確認できた。以前の「Draft 03・Final 投票結果未確認」という導入判断は更新する。ただし、正式仕様の成立と、採用するOP/SDKの対応・移行の安全性は別である。また、**認証要求ごとの変更は、UserInfo呼び出しや token refresh のたびに `sub` を変える指示ではない**。[承認告知][approval] / [Core §5.3.2・§12.2][core]

## 版の変更と変わらない契約

| 確認対象 | 日付・状態 | 今回の判断 |
|---|---|---|
| [版固定の Draft 03][draft] | 2026-07-10、Warning に未確定仕様と記載 | 過去の比較対象。現在の適用版としては使わない |
| [版固定の Final][ephemeral] | 2026-10-07、Status: Final | 現在の基準。可変URLの古い検索結果や投票予定だけで状態を判断しない |
| [承認告知][approval] | 2026-10-08 | OIDF会員によるFinal承認を明記。投票終了日から可決を推測したものではない |
| [Core 1.0 errata set 2][core] | 2023-12-15 | 識別子・UserInfo・refresh の既存契約を再確認 |

Draft 03 と Final の §1・§4〜8 を対照した範囲では、認証要求単位、値の非再利用・推測困難性、Discovery/Registration、衝突リスクと非相関の動作契約に変更は確認しなかった。Final からDraftの Warning と Document History が外れたことを、新たなwire parameterや乱数要件の導入と取り違えない。今回の実務上の変化は、確定した版を採用基準にできることと、既存の保留理由を実装確認へ移せることである。[Draft 03][draft] / [Final][ephemeral]

既存の [ID Token validation](id-token-validation.md) を置き換える文書ではない。検証後の識別子と保存先を扱い、登録全般は [RP Metadata Choices](rp-metadata-choices-registration-runtime-boundary.md) と切り分ける。

## 確認した契約

### public・pairwise・ephemeral の相関範囲

Core §8 の `public` は複数clientに同じ `sub` を提示する。`pairwise` は相関の範囲を狭めるが、§8.1 では Sector Identifier ごとに決定的な値を計算する。複数RPが同じ sector を共有する構成もあり、pairwise を「同じRPへの再訪も識別できない方式」とは扱わない。[Core §8・§8.1][core]

Core §5.7 の通常の識別キーは `iss` と `sub` の組であり、`email` や `preferred_username` の一意性・継続性は保証されない。ephemeral 拡張では、issuer の区別を捨てるのではなく、この組が独立した認証要求をまたいで同じ人物を指すという前提を外す。[Core §5.7][core] / [Final §4][ephemeral]

### 新しい認証要求と同じ認証内の応答を区別する

- **独立した認証要求**: Final §4 は要求ごとに異なる `sub` を定義する。§1 の認証セッション内の一定性と区別し、別の要求で同じ利用者だったことを値から再発見しない。[Final §1・§4][ephemeral]
- **対応するUserInfo**: UserInfo の `sub` はID Tokenの値との完全一致を確認する（MUST）。不一致ならUserInfoの値を使わない（MUST NOT）。ephemeral を理由に照合を省略しない。[Core §5.3.2][core]
- **token refresh**: Core §12.2 のrefresh成功応答はID Tokenを含まない場合がある。含む場合は、その `iss`・`sub`・`aud` を元の認証時と同じにする（MUST）。Final はこの節の上書きを定義していない。「短命だから毎回新しい `sub`」という発行処理をrefreshへ流用しない。[Core §12.2][core] / [Final §1・§4][ephemeral]

最後の点は「ephemeralの全実装がrefreshに対応する」という主張ではない。refreshを提供する構成で維持するCoreの境界であり、発行可否・寿命・セッション終了条件はOPと採用プロファイルで別途確認する。

### OPの生成責務と交渉

Final §4 でOPは値を再利用してはならず（MUST NOT）、推測成功確率を `2^-128` 以下にする（MUST）。`2^-160` 以下の目標と、想定する期間・規模で十分小さい衝突確率は SHOULD である。§7 は、衝突が別ユーザーのローカルレコードを混同し、誤認可や情報漏えいへつながることを述べる。文字列の長さだけでなく、暗号学的乱数源・実効entropy・再起動や複数プロセスでの再現性を点検する。[Final §4・§7][ephemeral]

OPは Discovery の `subject_types_supported` に `ephemeral` を公開し、RPは登録の `subject_type` または別の手段で選択する。ランダムに見える `sub` の観測は交渉の代用にならない。[Final §5・§6][ephemeral] 本書の設計ではOPの対応宣言と当該clientの採用済み設定を別に確認し、非対応や未知のtypeを無言でpublicへ読み替えない。

## 導入・移行時の設計判断（本書独自の提案）

以下は仕様が定めるDB構造や製品対応表ではない。

1. **用途を分ける**: 継続的な注文履歴・会員権限のログインを一括でephemeralへ切り替えない。短期の属性確認を別client登録または明確な処理経路に分離し、選択したsubject typeと設定版を認証処理に結び付ける。単なるFinalラベル更新を、既存会員の移行実施と扱わない。
2. **保存単位を決める**: 短期レコードに `iss`・今回の `sub`・認証処理との対応・保存期限を持たせる。新しい認証ごとに永続会員を自動作成しない。一方、同じ認証に対応するUserInfoやrefreshを別会員として保存しない。
3. **自動リンクを止める**: メール一致だけで既存会員へ紐付けない。継続利用が必要なら、別途設計・認可した明示的な紐付け手順を用意し、その相関がプライバシー目的に与える影響を説明する。
4. **混在期間を定義する**: 古いpublic/pairwise認証に基づくセッションと新しいephemeral認証を区別する。設定変更後のcallbackやrefreshを、現在のグローバル設定だけで再解釈しない。終了・再認証・rollbackの扱いを決め、以前の会員権限を暗黙に引き継がせない。
5. **識別子以外も点検する**: 固定cookie、メール、端末識別子、詳細な行動ログや新旧 `sub` の対応表は相関を残し得る。不要なclaim取得とログ保存を減らし、仕様の採用だけでシステム全体の匿名性を達成したとは判定しない。
6. **検証器を維持する**: 既存の署名・issuer・audience・時刻・nonce検証を弱めない。導入完了の根拠を「Finalになった」「登録要求が送れた」だけにせず、下記の接続試験結果と設定を残す。

## 確認する境界ケース（提案、実行結果ではない）

- 同じアカウントの独立した認証要求で `sub` が変わっても、以前の会員権限を継承しない
- ID Tokenと対応するUserInfoの `sub` 不一致を拒否する
- refresh対応構成では、ID Token省略と、元の `iss`・`sub`・`aud` を維持したID Tokenの双方を扱う。`sub` が変わったrefresh応答は正常な独立再訪として受け入れない
- 別issuerの同じ文字列 `sub` を同じ主体に混同しない
- 登録で未交渉のtypeやproviderの非対応を、意図したエラーとして扱う
- OP側では、プロセス再起動・複数発行ノード・snapshot復元などの運用条件で値が再利用されない設計と乱数源・衝突予算を確認する。少数回の値の比較だけで非再利用を証明したとしない
- メールなどの補助claimが同じでも自動リンクせず、不要な相関ログを残さない
- 設定変更前に始めた認証や既存refreshが、新設定の経路へ誤って混入しない

このリポジトリのevalは検索到達性の検証であり、上記のOIDC適合性・乱数品質・接続試験ではない。

## 制約・provenance

- Final承認は確認済み。製品・SDKの対応版、実際のOP/RP間接続、保存データの移行は未確認であり、仕様確定を一般提供・互換性の保証には使わない。
- Core §12.2 のrefresh時の識別子維持は確認したが、refresh発行可否、再認証、logout、複数タブをまたぐセッション寿命はこの拡張だけから一律に決めない。Self-Issued OPとの組合せやISO/IEC 27551の非相関レベルの適合性も検証していない。
- 2026-10-24 UTCにFinal・旧Draft・Core・承認告知を開き、本文の既存主張も再確認した。旧catalogは変更せず、今回の版・取得日・確認範囲を新規recordに記録した。Final確認後の再確認期限は、使用するsource typeの90日TTLに合わせ2027-01-22とした。
- OpenID Foundationを出典とする独自の日本語要約と設計・試験提案である。Final / Draft Appendix B、Core Appendix Cの仕様著作権ライセンス（帰属表示、仕様策定・実装目的）を確認した。承認告知は再利用許諾を確認できずlicenseをunknownとした。コード・長文・図の転載やmodule昇格は行わず、OIDFの推奨・認証を受けた文書とも表示しない。

[ephemeral]: https://openid.net/specs/openid-connect-ephemeral-subject-identifier-1_0-final.html
[draft]: https://openid.net/specs/openid-connect-ephemeral-subject-identifier-1_0-03.html
[core]: https://openid.net/specs/openid-connect-core-1_0.html
[approval]: https://openid.net/openid-connect-ephemeral-subject-identifier-1-0-final-specification-approved/

---
{
  "id": "npm-trusted-publishing-stage-dist-tag-permissions",
  "title": "npm trusted publishing: stage-only・2FA承認・dist-tag の権限境界",
  "kind": "knowledge",
  "technology": "supply-chain-security",
  "version": "npm CLI 12.2.0 release 2026-09-30; OIDC dist-tags: 11.21.0+/12.2.0+; registry changes 2026-09-03 and 2026-09-30; live docs verified 2026-10-02",
  "tags": [
    "research-domain:security",
    "npm",
    "trusted-publishing",
    "staged-publishing",
    "oidc",
    "dist-tag",
    "stage-only",
    "proof-of-presence",
    "2fa",
    "additive-permissions"
  ],
  "sources": [
    {
      "id": "github-npm-dist-tag-oidc-release-20261002",
      "url": "https://github.blog/changelog/2026-09-30-opt-in-dist-tag-permissions-for-npm-trusted-publishing/",
      "type": "release_notes"
    },
    {
      "id": "github-npm-multiple-trust-release-20261002",
      "url": "https://github.blog/changelog/2026-09-03-multiple-trusted-publishing-configurations-for-npm/",
      "type": "release_notes"
    },
    {
      "id": "npm-trusted-publishers-dist-tag-20261002",
      "url": "https://docs.npmjs.com/trusted-publishers/",
      "type": "official_docs"
    },
    {
      "id": "npm-staged-publishing-overview-20261002",
      "url": "https://docs.npmjs.com/staged-publishing/",
      "type": "official_docs"
    },
    {
      "id": "npm-stage-cli-v12-20261002",
      "url": "https://docs.npmjs.com/cli/v12/commands/npm-stage/",
      "type": "official_docs"
    },
    {
      "id": "npm-dist-tag-cli-v12-20261002",
      "url": "https://docs.npmjs.com/cli/v12/commands/npm-dist-tag/",
      "type": "official_docs"
    },
    {
      "id": "npm-cli-12-2-0-changelog-20261002",
      "url": "https://docs.npmjs.com/cli/v12/using-npm/changelog/",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# npm trusted publishing: stage-only・2FA承認・dist-tag の権限境界

## 問いと採用判断

CI を OIDC trusted publishing に移し、`npm stage publish` だけを使えば、CI が侵害されても利用者に配布される版を変えられないのか。答えは、**新規版の公開承認と、既存版を指す distribution tag の変更を別々に制限する必要がある**、である。2026-09-30 に OIDC の dist-tag 操作が追加されたため、stage-only という説明だけでは CI の配布への影響を判定できない。

以下の「確認した契約」は公式資料の要約、「運用設計」は本書の判断である。既存の [npm lock/integrity](npm-lock-integrity-scoped-registry-ci.md) は取得物の一致、[artifact attestation](github-artifact-attestation-slsa-verify.md) は来歴の検証を扱う。本書は npm registry が受け付ける操作と公開判断の境界を扱い、それらの検証方式は再説明しない。

## 確認した契約

### 1. 2026-09-30: dist-tag は独立した opt-in

[GitHub の変更告知](https://github.blog/changelog/2026-09-30-opt-in-dist-tag-permissions-for-npm-trusted-publishing/) は、OIDC の短命資格情報で dist-tag を管理できるようになったと明記する。`Allow npm dist-tag` は新規・既存の設定とも既定 off。direct publish の許可とは独立しており、stage-only の設定にも付与できる。従来の token によるタグ操作は継続する。

[npm trusted publishers の対応版](https://docs.npmjs.com/trusted-publishers/#managing-dist-tags-with-trusted-publishing) は v11 系で 11.21.0 以降、v12 系で 12.2.0 以降。[CLI changelog](https://docs.npmjs.com/cli/v12/using-npm/changelog/) も 12.2.0 の公開日を 2026-09-30、機能追加を dist-tag OIDC authentication としている。CLI の更新だけで既存の registry 設定へタグ権限が追加されるわけではない。

[dist-tag reference](https://docs.npmjs.com/cli/v12/commands/npm-dist-tag/) では、tag は version の別名であり、add・rm・ls は対応付けの追加、削除、列挙を行う。バージョンもタグも指定しない `npm install <pkg>` は `latest` を選ぶ。したがって、タグ変更を単なる表示用 metadata として扱わない。既存版へのルーティング変更は新しい tarball の公開とは異なる操作である。

### 2. 複数設定は additive / OR。狭い設定を足しても既存権限は狭まらない

[2026-09-03 の変更告知](https://github.blog/changelog/2026-09-03-multiple-trusted-publishing-configurations-for-npm/) によると、各 trusted publisher 設定は repository・workflow・environment 条件を持ち、互いに独立した追加的な許可として評価される。publish / stage は OIDC token がいずれか一つの設定に一致すれば認可される。設定同士は制限し合わず、評価順序も保証されない。2026-09-30 の告知も、dist-tag を許可した設定のいずれか一つとの一致でタグ操作を認可するとしている。

[npm の現行ガイド](https://docs.npmjs.com/trusted-publishers/) では一つの package に最大10設定。2026-09-03 より後に作成した設定は stage が自動的に許可され、direct publish を追加選択できる。以前の設定が自動的に stage-only へ縮小されたとは扱わない。同ガイドは旧設定の既存動作を変えないことも明記する。

具体例は運用上の推論である。実行が設定 A と B の両方に一致し、A が direct publish を許可、B が stage-only なら、B を後から追加しても A による公開を抑止できない。dist-tag でも同様に、重なる設定の一つに許可が残れば、別の設定で off にしただけでは拒否条件にならない。

### 3. stage は保留への登録、approve は公開操作

[staged publishing overview](https://docs.npmjs.com/staged-publishing/) の前提は npm 11.15.0 以降と Node 22.14.0 以降。これは staged publishing 機能の下限であり、上記の OIDC dist-tag 対応版とは別である。採用する CLI 自体の Node 対応範囲も別途満たす必要がある。

`npm stage publish` は承認待ちへ登録し、その時点では 2FA を要求しない。公開には maintainer の review と approve が必要で、CLI と npmjs.com のどちらから承認しても 2FA を要求する。新規 package では `0.0.0-stage` の placeholder が公開されるが、承認前の対象版と内容は公開されない。したがって新規 package の staging を「外部状態も公開情報も一切作らない dry-run」と解釈してはならない。

[2026-09-03 の告知](https://github.blog/changelog/2026-09-03-multiple-trusted-publishing-configurations-for-npm/) では、staged package の malware scanning が終わるまで承認できず、Web の approve ボタンは scan 完了後に利用可能になる。scan 完了は人の承認を代行する状態ではない。

### 4. stage-id・version・tag を一緒に確認する

[npm-stage reference](https://docs.npmjs.com/cli/v12/commands/npm-stage/) の重要な境界は次の通り。

- staged と published は package 内の semver 一意制約を共有する。staging 中の版と同じ版を通常 publish することはできない。一方、別の版の通常 publish は pending stage の存在だけでは禁止されない
- stage に記録した tag は immutable。修正には既存 stage を reject して再登録する必要があり、承認時だけ別 tag に差し替える契約ではない
- `--tag` を省略すると既定は `latest`。prerelease や最新 semver より古い版では明示的な tag が必要で、未指定なら CLI が失敗する
- `approve` は公開、`reject` は staged package の永久除去で、ともに 2FA を要求する。`list`・`view`・`download` は同 reference の表では 2FA 不要

2FA 不要は匿名アクセスや OIDC 利用可能を意味しない。[trusted publishers ガイド](https://docs.npmjs.com/trusted-publishers/#managing-dist-tags-with-trusted-publishing) は `stage list/view/approve/reject` に OIDC token を使えず、対話認証が必要と説明する。よって「stage 権限があれば CI から自動承認できる」という権限拡張は成立しない。

## 運用設計（上記の事実からの独自提案）

### package 単位で三つの操作を棚卸しする

1. **候補の登録**: ビルド用 CI は stage を担当し、対象 package・version・stage-id・予定 tag とビルドの参照を記録する
2. **公開の判断**: maintainer がその stage-id の内容を確認して承認する。再ビルドしたローカル成果物だけを見て、別の staged tarball に承認を与えない。二人承認が必要なら組織側の追加手順を設計する。2FA はそれだけで別人による four-eyes review を保証しない
3. **配布先タグの変更**: `latest` / `next` を動かす automation は独立した release 権限として扱う。単なるビルド job に dist-tag を追加しない。必要な場合は承認済み version の許可リストと変更前後の対応付けを確認する

「CI 侵害時にも人の確認まで新規版を公開させない」が要件なら、全 trusted publisher の direct publish 許可と既存 token 経路を調べる。さらに「既存版への誘導も変えさせない」なら dist-tag も調べる。設定一件だけ、または同名 workflow だけの確認では OR 評価を見落とす。

package の従来 token 制限と OIDC 設定は別の制御である。[npm ガイド](https://docs.npmjs.com/trusted-publishers/#recommended-restrict-token-access-when-using-trusted-publishers) は従来 token を禁止しても trusted publishers は動作すると明記する。切替時には新経路を確認してから不要な旧経路を閉じる方針を採り、障害時に広い token を黙って復活させる設計は避ける。本調査では権限や設定を変更していない。

### 失敗を公開への自動 fallback にしない

- stage 成功をリリース完了として通知しない。承認待ち、scan 待ち、公開済みを別の状態で記録する
- 同一 version の衝突時は既存 stage-id と内容を確認する。とりあえず通常 publish に変更して保留を迂回しない
- tag を誤った場合は、その stage を承認しない。reject と再登録は既存 review を引き継ぐものとせず、対象を改めて確認する
- dist-tag の権限エラーを direct publish の追加許可で直さない。必要な操作とそれを許可する一致設定を調べる
- `npm whoami` の成功・失敗を trusted publishing の認可テストにしない。公式ガイドも判定用途ではないとする。導入試験は影響を理解した検証用 package と操作別の期待結果で計画する

この構成でも、人が悪意ある候補を誤承認することや、許可されたタグ変更 automation の侵害までは解消しない。scan・2FA・来歴・内容レビューは異なる問いに答える制御であり、どれか一つを「安全な package」の証明として置き換えない。

## 版差・資料の食い違いと未確認事項

- `npm-stage` の表示版は 12.2.0、本文の最終編集表示は 2026-09-24。その Trust Relationship Permissions 節には短命 token の利用先を publish / stage publish に限る古い説明が残る。本書は 2026-09-30 の明示的な追加告知、同日版の CLI changelog、更新後の trusted publishers の専用節を根拠に dist-tag 対応を記載する。表示 CLI 版だけで全段落の更新を推定しない
- trusted publishers の「対話認証」と npm-stage の「2FA 不要」の表は同じ概念ではない。read 系操作ごとの正確な認証 UX、`stage download` の OIDC 可否は本調査では実機未確認
- npm service の契約と live CLI reference の調査であり、npm 実装を commit 固定で解析していない。npm 11 系と12系の全挙動同一性、各 provider の実際の OIDC claims、runner 別の成功率は未確認
- stage の保持期限、scan の所要時間・検出率、承認と権限削除が競合した場合の結果、複数承認者や同時 tag 更新の競合は未確認。無期限保持、完全な malware 検出、compare-and-swap、exactly-once な公開を仮定しない
- registry 操作、ログイン、token 発行、権限設定、package の公開・承認・削除は一切実行していない。検索 eval は発見可能性だけを検査し、上記運用案の安全性や runtime の結果を証明しない

## 出典・日付・provenance

全資料の取得日は **2026-10-02 UTC**。GitHub Changelog 2件の公開日は 2026-09-03 と 2026-09-30。npm CLI changelog は 12.2.0 / 2026-09-30。npm の service guides は last edited 2026-09-29（staged publishing）、2026-09-30（trusted publishers）。CLI reference の表示は 12.2.0 で、最終編集表示は npm-stage が 2026-09-24、npm-dist-tag が 2025-10-04。これらはリリース日・文書編集日・取得日の別記録である。

npm/documentation の content に対する [CC-BY-4.0 の区分](https://raw.githubusercontent.com/npm/documentation/main/.reuse/dep5) と [LICENSE](https://raw.githubusercontent.com/npm/documentation/main/LICENSE)、CLI 文書の出所 npm/cli の [Artistic-2.0 LICENSE](https://raw.githubusercontent.com/npm/cli/latest/LICENSE) を確認した。v12.2.0 固定の LICENSE URL は取得に失敗したため、確認したのは latest 側のライセンスであり、版固定の法的検証とはしていない。GitHub Blog は著作権表示を確認したが再利用ライセンスを特定できず catalog では unknown。全体を独自の日本語要約として作成し、コード・workflow・長い原文は転載していない。release_notes の TTL 30日に合わせ、再確認期限を **2026-11-01** とする。

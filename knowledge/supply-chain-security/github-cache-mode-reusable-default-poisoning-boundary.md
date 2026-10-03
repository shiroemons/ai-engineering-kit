---
{
  "id": "github-cache-mode-reusable-default-poisoning-boundary",
  "title": "GitHub cache-mode の境界: 低信頼 trigger の既定値・reusable workflow の明示上限・cache poisoning",
  "kind": "knowledge",
  "technology": "supply-chain-security",
  "version": "github.com cache-mode GA 2026-09-10 (all plans); GitHub Docs live references verified 2026-10-03 UTC, no fixed page version; runner/action minimum versions and GHES availability unverified",
  "tags": [
    "research-domain:security",
    "supply-chain-security",
    "github-actions",
    "cache-mode",
    "cache-poisoning",
    "reusable-workflow",
    "least-privilege",
    "pull_request_target",
    "write-only",
    "scoped-cache-tokens"
  ],
  "sources": [
    {
      "id": "github-cache-mode-ga-20260910-20261003",
      "url": "https://github.blog/changelog/2026-09-10-control-github-actions-cache-access-with-cache-mode/",
      "type": "release_notes"
    },
    {
      "id": "github-cache-mode-syntax-20261003",
      "url": "https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax",
      "type": "official_docs"
    },
    {
      "id": "github-cache-mode-defaults-scope-20261003",
      "url": "https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching",
      "type": "official_docs"
    },
    {
      "id": "github-cache-mode-reusable-cap-20261003",
      "url": "https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows",
      "type": "official_docs"
    },
    {
      "id": "github-cache-mode-token-separation-20261003",
      "url": "https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# GitHub cache-mode: 既定 read と呼出先への明示上限を分ける

## 問いと適用版

低信頼イベントから動く CI を read-only にしたつもりでも、再利用先が cache を保存できるのはどんな場合か。対象はビルド入力を介した cache poisoning と reusable workflow への権限委譲である。

[2026-09-10 の公式告知][ga]で、`cache-mode` が github.com の全 plan に GA となったことを確認した。[構文][syntax]・[cache reference][cache]・[再利用手順][reuse]・[再利用 reference][config]は2026-10-03 UTC に実際に開いた版番号・公開日の表示がない live 文書である。GHES や特定 runner/action の対応版には一般化しない。

実務上の設計判断は、**低信頼 caller には明示的な `read` または `none` を置き、再利用先にもその契約を合わせる**こと。イベント由来の暗黙 read は呼出先の上限と同義ではない。[既存の OIDC / GITHUB_TOKEN 文書](../github-actions/oidc-cloud-id-token-least-privilege.md)と[artifact attestation 文書](github-artifact-attestation-slsa-verify.md)に対し、本稿は「cache を誰が書き、誰が後で実行するか」を補う。

## 1. 公式契約: mode は read/save の能力

[Workflow syntax][syntax]では、scoped cache tokens がアクセスを制限する。job の `cache-mode` は workflow の設定をその job について上書きする。

| mode | restore | save |
| --- | --- | --- |
| `read` | 可 | 不可 |
| `write` | 可 | 可 |
| `write-only` | 不可 | 可 |
| `none` | 不可 | 不可 |

`write-only` は `read` より単純に「強い」設定ではない。両者は非包含で、別の能力を持つ。通常 job の上書きと、次節の reusable caller の上限を区別する。

許可されない操作は skip され、restore の skip は cache miss、save の skip は未保存となる。これだけでは step/job/run は失敗しない。`cache-mode` による skip は informational message、従来の低信頼 read-only 制限で save を試みた場合は cache reference が warning と説明している。ログの強度を同一と断定せず、成功終了を保存成功の証拠にしない。[構文][syntax]・[cache reference][cache]

## 2. 公式契約: 既定値を明示 write が上書きする

省略時は trigger によって `read` または `write` になる。既定 branch scope では、`push` などの指定された trigger は書込可能で、`pull_request_target`・`issue_comment`・`workflow_run` などは既定 read-only である。しかし明示的な `write` / `write-only` は、この低信頼既定値を上書きする。GA 告知はその場合の warning annotation も説明している。[cache reference][cache]・[GA告知][ga]

`pull_request` はこの既定 branch 制限の対象とは別で、作成 cache が `refs/pull/.../merge` に限定される。base branch や別 PR からは復元できない一方、fork を含む PR は base/default branch の cache を読める。したがって「fork は cache を一切書けない」「read-only なら中身を秘密にできる」とは読めない。[cache reference][cache]

## 3. 公式契約: reusable workflow は明示上限を検査する

[再利用手順][reuse]は、呼出 job に直接指定した、または caller workflow から継承した**明示 mode**が callee の要求上限になると定める。呼出 job がどちらも持たなければ、低信頼イベントの暗黙 `read` からでも callee が明示 `write` を要求できる。これは明示 `read` の突破ではない。

以下は公式例と上の能力表を組み合わせたレビュー用整理であり、実機試験結果ではない。

| caller の状態 | callee の明示要求 | 判断 |
| --- | --- | --- |
| 低信頼、明示指定なし | `write` | 既定 read だけでは防げない |
| 明示 `read` | `write` | 開始前 validation error |
| 明示 `write-only` | `read` | restore を過大要求し、開始前 validation error |
| 明示 `write` | `read` | 能力の縮小。上限内 |
| 明示 `none` | `write-only` | save が上限外 |

過大要求は「実行中に権限だけ縮めて続行」ではなく run が開始しない契約である。子側の明示 `write` を残したまま親に `read` を足す移行は、機能停止を伴い得る。[再利用手順][reuse]

呼出 job では `cache-mode` と `permissions` が別 key としてサポートされる。`GITHUB_TOKEN` は callee が caller より権限を増やせないが、その説明を cache の暗黙 default に流用しない。`permissions: read-all` だけを cache 保存禁止の設定として扱わず、cache は `cache-mode` の契約で確認する。[再利用 reference][config]・[構文][syntax]

## 4. 防げることと残る危険

[cache reference][cache]は cache 内容が署名・検証されないこと、復元物が後続の実行ファイルを変え得ることを警告している。read-only consumer でも、既に汚染された入力を安全にはしない。

ここからの独自の設計判断は次のとおり。

- cache key に lockfile hash があっても、それだけを producer の認証や内容審査にしない。cache hit は依存関係・ビルド結果の安全性判定から分離する
- 高権限 release job は、producer と経路を説明できない cache を復元しない。必要なら `none` にして性能より入力の分離を優先する
- `write-only` は writer 自身の復元を防ぐ選択肢だが、信頼できない入力から作った cache を後続 consumer に渡す危険は残る。mode 名だけで安全な producer と判定しない
- cache path に秘密情報を入れない。読み手の制限と書き手の制限は別にレビューする

## 5. 導入レビューと試験案（未実行）

1. workflow/job ごとに trigger、明示 mode、復元 scope、書込 producer、後続 consumer を一覧化する。`actions/cache` だけでなく setup action 内の cache 利用も調べる
2. 低信頼 caller に明示 `read` / `none` を置く。同じ変更で callee の要求を照合し、開始前エラーを意図せず導入しない
3. cache 更新が必要なら、信頼した `push` job に限定して検討する。trigger 名だけで trust を決めず、その job が処理する checkout・入力・実行コードを審査する
4. `ACTIONS_CACHE_MODE`、skip の記録、cache の実際の生成有無を確認する。環境変数は観測値として扱い、mode 変更の代わりに手で上書きする運用を設計しない
5. 非公開データや実 credential を使わない検証環境で、次の境界を試す

| 試験 | 確認すること |
| --- | --- |
| caller に指定なし / 指定 read の比較 | callee の write 要求が通る条件と開始前拒否を分ける |
| workflow read + 通常 job write | 通常 job override と reusable cap の違い |
| read または none で cache 操作 | skip と run 成功が両立し、保存成功と誤認しない |
| write-only writer | restore しないことと、作成物の信頼性を別に確認 |
| PR merge ref と default branch | 許可 mode と可視 scope を別々に確認 |
| release consumer の none | build が cache miss でも正しく成立するか |

## 限界・来歴・再確認

- 本稿は公式文書の原著要約と明示した設計案であり、GitHub Actions 上の実行試験、攻撃再現、特定 account の有効値確認は行っていない
- runner、`actions/cache`、`@actions/cache`、setup action の最小対応版や第三者 cache backend への適用は未確認。cache service の契約と、各 client のログ/UI の実装差を分けて導入前に確かめる
- `none` を付けるだけで過去の cache を除去したり、他の job のアクセスを失効させたりするとは主張しない。既存 cache の監査・削除・credential 失効は別作業である
- GitHub Docs は [github/docs LICENSE](https://raw.githubusercontent.com/github/docs/main/LICENSE) の CC-BY-4.0 を取得日に確認。GitHub, Inc. の文書を出典とし、本文の構成・説明・試験案は本稿で作成した。Changelog の開放ライセンスは未確認のため `unknown` と記録し、コードや長文を転載していない。repository 実装分析は行っておらず、固定 commit の実装保証はしない
- release notes の30日 TTL が最短となるため再確認期限は2026-11-02。live 文書の将来変更、GHES 適用、未実施試験を含めて再調査し、日付だけ延長しない
- 検索 eval は `evals/knowledge/security.json` に追加した。検索到達性の検証であり、上表の Actions 実行試験とは別である

[ga]: https://github.blog/changelog/2026-09-10-control-github-actions-cache-access-with-cache-mode/
[syntax]: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
[cache]: https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching
[reuse]: https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows
[config]: https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations

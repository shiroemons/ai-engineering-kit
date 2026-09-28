---
{
  "id": "github-actions-oidc-cloud-id-token-least-privilege",
  "title": "GitHub Actions からクラウドへの OIDC: id-token 権限と最小権限 GITHUB_TOKEN スコープ",
  "kind": "knowledge",
  "technology": "github-actions",
  "version": "GitHub Docs 3ページ (current pages, verified 2026-09-28)",
  "tags": ["research-domain:infrastructure", "github-actions", "OIDC", "id-token", "GITHUB_TOKEN", "permissions", "least-privilege", "configure-aws-credentials", "sts.amazonaws.com", "token.actions.githubusercontent.com"],
  "sources": [{"id": "github-docs-oidc-cloud-providers", "url": "https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers", "type": "official_docs"}, {"id": "github-docs-automatic-token-authentication", "url": "https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication", "type": "official_docs"}, {"id": "github-docs-oidc-aws", "url": "https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services", "type": "official_docs"}],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active"
}
---

# GitHub Actions からクラウドへの OIDC: id-token 権限と最小権限 GITHUB_TOKEN スコープ

長期クレデンシャルを workflow に保存せず、クラウド側の trust で受け取れる OIDC トークンを使うための権限設定である。以下は3つの公式ページ (いずれも2026-09-28に確認した版ラベルのない現行ページ) に書かれた事実と、それを組み立てる設計案を分けて書く。

## 要点（公式文書に記載された事実）

### OIDC トークンの取得には `id-token: write` が要る

- OIDC JWT を作成するには job または workflow に `permissions: id-token: write` が必要である。[Configuring OpenID Connect in cloud providers](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers)
- `id-token: write` の設定は OIDC トークンの要求・取得 (requesting/fetching) のみを許可し、リソースへの書き込みを許可するものではない。[Configuring OpenID Connect in cloud providers](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers)
- AWS の workflow 例は OIDC 用の `id-token: write` に加え、checkout 用の `contents: read` を付けている。[Configuring OpenID Connect in Amazon Web Services](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services)

### トークン交換の経路は公式 action か手動フロー

- クラウドへの交換は公式 action を使う方法と、`ACTIONS_ID_TOKEN_REQUEST_TOKEN` / `ACTIONS_ID_TOKEN_REQUEST_URL` を使う手動フローがある。[Configuring OpenID Connect in cloud providers](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers)
- AWS の例は `aws-actions/configure-aws-credentials` を使い、OIDC provider URL は `https://token.actions.githubusercontent.com`、audience は `sts.amazonaws.com` である。[Configuring OpenID Connect in Amazon Web Services](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services)

### クラウド側 trust には少なくとも1つの条件が要る

- クラウドプロバイダー側の trust は少なくとも1つの条件 (at least one condition) を定義しなければならない。[Configuring OpenID Connect in cloud providers](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers)
- AWS の IAM trust は `token.actions.githubusercontent.com:sub` と `token.actions.githubusercontent.com:aud` を評価し、`StringEquals` / `StringLike` の例が示される。例には `repo:ORG@ID/REPO@ID` 形式の不変 (immutable) な参照が含まれる。[Configuring OpenID Connect in Amazon Web Services](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services)

### `GITHUB_TOKEN` の権限は `permissions` キーで絞る

- `permissions` キーは workflow 全体または単一 job の `GITHUB_TOKEN` を変更する。[Use GITHUB_TOKEN for authentication in workflows](https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication)
- 指針は必要最小限のアクセス (least required access / minimum permissions) を付与することである。[Use GITHUB_TOKEN for authentication in workflows](https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication)
- action は明示的に引き渡さなくても `github.token` コンテキストへアクセスできる。[Use GITHUB_TOKEN for authentication in workflows](https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication)
- job レベルの例として `contents: read` と `issues: write` の組み合わせが示される。[Use GITHUB_TOKEN for authentication in workflows](https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- OIDC を使う job だけに `permissions: id-token: write` を付け、workflow 頂点には付けない。`id-token: write` は取得権限であり書き込み権限ではないが、付与範囲をデプロイ job に閉じることで、他 job が OIDC トークンを取得できない形にする。
- checkout が要る job は `contents: read` を併記する (AWS の公式例と同じ組み合わせ)。checkout が不要な job には `contents: read` を付けない。
- `GITHUB_TOKEN` の残りのスコープは job ごとに必要なものだけ書く (`contents: read` + 必要な `issues: write` 等の形)。action が `github.token` に暗黙に触れることを前提に、頂点の既定権限は広げず job レベルで明示する。
- クラウド側 trust には少なくとも1つの条件を入れ、AWS では `sub` と `aud` を評価する。`StringLike` のワイルドカードは使える範囲を狭くし、固定できる対象は `repo:ORG@ID/REPO@ID` 形式の不変参照を選ぶ。
- 交換手段は公式 action (`aws-actions/configure-aws-credentials` 等) を既定とし、`ACTIONS_ID_TOKEN_REQUEST_TOKEN` / `ACTIONS_ID_TOKEN_REQUEST_URL` の手動フローは公式 action が無いプロバイダーや独自交換が必要な場合に限る。

## 避ける使い方

- `id-token: write` を「リソースへの書き込み許可」と読んで OIDC 用 job から外す。文書は要求・取得のみの許可と明記し、JWT 作成にはこの権限が必要である。[Configuring OpenID Connect in cloud providers](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers)
- workflow 頂点に広い `permissions` を置いたまま job レベルで絞らない。`permissions` キーは workflow 全体にも単一 job にも適用でき、指針は最小権限である。[Use GITHUB_TOKEN for authentication in workflows](https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication)
- `github.token` は明示的に渡さない限り使われないと考える。文書は明示的な引き渡しがなくても action が `github.token` コンテキストへアクセスできると述べる。[Use GITHUB_TOKEN for authentication in workflows](https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication)
- クラウド側 trust を条件なしで作る。文書は少なくとも1つの条件を要求する。[Configuring OpenID Connect in cloud providers](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-cloud-providers)
- provider URL や audience を推測で変える。AWS の公式例は `https://token.actions.githubusercontent.com` と `sts.amazonaws.com` である。[Configuring OpenID Connect in Amazon Web Services](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services)

## 適用版と本番での注意

- 適用版: 3ページはいずれも版ラベルのない現行ページで、2026-09-28に確認した内容に基づく。将来の最新とは扱わない。
- 再確認期限: source がいずれも `official_docs` (TTL 90日) のため、文書の取得日 + 90日の2026-12-27に再取得して内容を確認する。文書の `expires_at` も同日に設定した。
- 未確認事項 (推測で埋めない): 上記3ページ以外への OIDC claim (`sub` の `environment` / `ref` / `pull_request` 等の書式)、各クラウド (Azure/GCP) 固有の trust 書式、`permissions` 未指定時の既定スコープの現在値、手動フローでの audience 指定方法と有効期限。本調査では確認していない。
- 本ドキュメントの推奨構成は設計案であり、特定リポジトリの workload (デプロイ対象、checkout 要否、利用 action) に対するもの。公式の記載事実 (`id-token: write` の意味、trust の条件要求、provider URL と audience、`permissions` の適用範囲と最小権限指針) とは本文中で区別してある。

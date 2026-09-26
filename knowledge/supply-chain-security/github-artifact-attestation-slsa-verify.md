---
{
  "id": "github-artifact-attestation-slsa-verify",
  "title": "GitHub artifact attestation と SLSA provenance によるビルド成果物・コンテナイメージの来歴検証 (gh attestation verify)",
  "kind": "knowledge",
  "technology": "supply-chain-security",
  "version": "SLSA Build Provenance v1.2 (Status: Approved) / GitHub Docs・GitHub CLI manual 2026-09-26 取得時点の current ページ (ページ版ラベルなし)",
  "tags": ["research-domain:security", "supply-chain-security", "artifact-attestation", "provenance", "slsa", "gh-attestation-verify", "attestation verify", "github-actions", "sigstore", "container-image", "oci", "offline", "trusted-root", "bundle", "signer-workflow", "predicate-type", "build-provenance", "provenance verification"],
  "sources": [
    {"id": "gh-attestation-verify-cli-manual", "url": "https://cli.github.com/manual/gh_attestation_verify", "type": "official_docs"},
    {"id": "github-docs-use-artifact-attestations", "url": "https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations", "type": "official_docs"},
    {"id": "github-docs-artifact-attestations-concepts", "url": "https://docs.github.com/en/actions/concepts/security/artifact-attestations", "type": "official_docs"},
    {"id": "github-docs-verify-attestations-offline", "url": "https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/verify-attestations-offline", "type": "official_docs"},
    {"id": "slsa-build-provenance-v1-2", "url": "https://slsa.dev/spec/v1.2/build-provenance", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# GitHub artifact attestation と SLSA provenance によるビルド成果物・コンテナイメージの来歴検証 (gh attestation verify)

GitHub Actions の artifact attestation は、ビルド成果物に workflow・repository・commit SHA 等を含む署名済み claim を付け、`gh attestation verify` で来歴 (provenance) を検証する。既定の predicate は SLSA Build Provenance (`https://slsa.dev/provenance/v1`) である。以下は公式文書から確認した事実 (要点) と、そこからの本リポジトリの設計案 (推奨方法) を区別して記載する。

## 要点 (文書化された事実)

### attestation の生成 (GitHub Actions)

- 生成 action は `actions/attest@v4`。workflow permissions は `id-token: write`、`contents: read`、`attestations: write`。コンテナイメージの場合はさらに `packages: write` と `push-to-registry: true` が必要 ([use-artifact-attestations](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations))。
- `subject-name` はタグを含まない完全修飾名、`subject-digest` は `sha256:HEX_DIGEST` 形式。
- 既定 predicate は SLSA。SBOM 等の非既定 predicate は `--predicate-type` で指定する (例: `https://spdx.dev/Document/v2.3`。`attest` action は SPDX と CycloneDX に対応)。
- 署名対象は release / binary / package。test build や個別ファイルは署名対象にしないのが文書化された guidance ([artifact attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations))。
- attestation は GitHub Actions の OIDC token に由来する workflow link、repository / organization / environment / commit SHA / triggering event を含む署名済み claim であり、生成には Sigstore を使う (同上)。
- SLSA レベル: 「Artifact attestations by itself provides SLSA v1.0 Build Level 2」。reusable workflow が文書化された SLSA v1.0 Build Level 3 の経路 (同上)。
- public リポジトリは Sigstore Public Good Instance (公開・不変の transparency log) を使う。private リポジトリは GitHub の Sigstore instance を使い、transparency log はなく、federation は GitHub Actions のみ (同上)。
- **警告 (文書)**: attestation は成果物が secure である保証ではない。source と build instructions へのリンクを提供するのみで、policy / risk の判定は consumer の責任 (同上)。

### 検証 (`gh attestation verify`)

([GitHub CLI manual](https://cli.github.com/manual/gh_attestation_verify)、[use-artifact-attestations](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations))

- 構文: `gh attestation verify [<file-path> | oci://<image-uri>] [--owner | --repo]`
- 既定の `--predicate-type` は `https://slsa.dev/provenance/v1`。
- identity は attestation 証明書の SourceRepository / SourceRepositoryOwner / SubjectAlternativeName に対して強制される。
- `--owner` か `--repo` が最低限必須。`--signer-workflow` / `--cert-identity` が推奨される。reusable workflow の attestation は `--signer-workflow` か `--signer-repo` が必須。
- バイナリの検証: `gh attestation verify PATH -R OWNER/REPO`。
- container image の検証: `oci://` 付きの完全修飾イメージ名 (FQDN) と registry ログイン (`docker login ghcr.io`)。attestation をレジストリから取得するオプションは `--bundle-from-oci`。
- online と offline の検証は分離されている。offline 側は `--bundle` / `--custom-trusted-root` を使う。
- `--format=json` の出力で改竄不可能な (non-manipulable) のは `signature.certificate` と `verifiedTimestamps` のみ。`statement.predicate` は workflow を制御する攻撃者に偽装可能で、緩和策は trusted reusable-workflow builder。
- predicate 内容の確認: `--format json --jq '.[].verificationResult.statement.predicate'`。
- その他の確認済み flag: `--cert-oidc-issuer` 既定 `https://token.actions.githubusercontent.com`、`--deny-self-hosted-runners`、`--digest-alg` {sha256|sha512}、`--limit` 既定 30、`--no-public-good`。

### offline 検証フロー

([verify-attestations-offline](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/verify-attestations-offline))

1. `gh attestation download <file> -R owner/repo` — `sha256:….jsonl` の bundle を書き出す (以前の内容を上書きする)。
2. `gh attestation trusted-root > trusted_root.jsonl`
3. `gh attestation verify <file> -R owner/repo --bundle sha256:….jsonl --custom-trusted-root trusted_root.jsonl`

`trusted_root.jsonl` には組み込みの有効期限がない。生成前に署名された material は検証され続け、生成後の署名は Sigstore instance が鍵素材をローテーションするまで検証できる (ローテーションは "typically happens a few times per year")。生成以降の revocation は検知できないため、新しい署名素材を取り込むときに trusted root を再生成することが best practice として文書化されている。public リポジトリは Sigstore public good instance、private リポジトリは GitHub の Sigstore instance を使う。

### SLSA Build Provenance predicate (v1.2)

([SLSA Specification v1.2 Build: Provenance](https://slsa.dev/spec/v1.2/build-provenance)、Status: Approved。v1.0 の provenance ページは Retired)

- in-toto attestation framework 内の predicate type 文字列は `https://slsa.dev/provenance/v1`。URL bar の文字列ではなくこの文字列を常に使う。
- Parsing rules: consumer は unrecognized fields を MUST で無視する。`predicateType` URI は major version を持ち、backward-incompatible な変更でのみ変わる。minor version は backwards compatible で monotonic であり URI は変えない。unset / null / empty は MUST で等価に解釈する。
- schema: `predicate` は `buildDefinition` (`buildType`, `externalParameters`, `internalParameters`, `resolvedDependencies`) と `runDetails` (`builder.id` / `builderDependencies` / `version`, `metadata.invocationId` / `startedOn` / `finishedOn`, `byproducts`) からなる。
- `externalParameters` は untrusted。MUST で include し、下游で MUST で検証される。Build L3 で MUST complete (それ未満の level では best effort が MAY)。verifier は `externalParameters` 内の unrecognized / unexpected fields を SHOULD で拒否する。
- `internalParameters` は検証不要 (need not be verified)。
- Build L1 で REQUIRED のフィールド: `buildDefinition`、`runDetails`、`buildType`、`externalParameters`、`builder`。
- consumer は特定の signer–builder pair のみ MUST で受け入れる (GitHub は "GitHub Actions" builder 用に署名できるが "Google Cloud Build" builder 用には署名できない)。
- `builder.id` は SLSA Build level の sole determiner と意図されている。
- ResourceDescriptor の digest keys に `sha256`、`sha512`、`gitCommit` が含まれる。
- v1.2 の change history は provenance build-model 図の明確化のみで、schema は v1.0 から不変。

## 推奨方法 (上記からの設計上のまとめ)

以下は公式文書の事実ではなく、本リポジトリでの設計案である。

- 検証コマンドは `-R OWNER/REPO` (または `--owner`/`--repo`) に加え、可能なら `--signer-workflow` か `--cert-identity` を固定する。identity 照合は証明書の SourceRepository 等のフィールドベースであり、CLI manual が `--signer-workflow` / `--cert-identity` を推奨している通り、`--owner`/`--repo` だけでは sign 側の絞り込みにならない。
- `statement.predicate` を無条件に信じない。workflow を制御できる攻撃者は predicate を偽装できるため、`externalParameters` は下游で必ず検証し、unexpected fields は拒否する。predicate の信頼性を上げる手段は trusted reusable-workflow builder に置く。
- container image はタグではなく digest と結び付け、検証対象を `oci://` FQDN で指定する (`subject-name` はタグなし完全修飾名、`subject-digest` は `sha256:` 形式が生成側の契約)。
- air-gapped 環境では download した bundle と trusted root をセットで管理し、新しい署名素材を取り込むたびに `gh attestation trusted-root` を再生成する (revocation が検知できないため)。
- private リポジトリでは transparency log が存在しない前提で監査方針を組み立てる (検証は GitHub の Sigstore instance と GitHub Actions federation に限定される)。
- artifact attestation は SLSA v1.0 Build Level 2 相当と割り切り、Level 3 を要件にする用途では reusable workflow 化を前提にする。

## 避ける使い方

- `statement.predicate` の信頼。CLI manual が明示する通り workflow 制御者に偽装可能で、改竄不可能なのは `signature.certificate` と `verifiedTimestamps` だけである。
- identity 絞り込みなしの検証。`--owner`/`--repo` は必須だが、`--signer-workflow` / `--cert-identity` を落とすと signer 側の制約が緩む (reusable workflow の attestation は `--signer-workflow` か `--signer-repo` が必須)。
- `predicateType` を URL bar からコピーする行為。仕様は「Always use the above string for `predicateType` rather than what is in the URL bar」と明示している。
- attestation を「成果物は secure」と解釈する運用。公式文書は明確に非保証と宣言し、policy / risk 判定は consumer 責任としている。
- test build や個別ファイルへの署名。guidance は release / binary / package に限定している。
- private リポジトリで transparency log を前提にした監査設計。GitHub の Sigstore instance には transparency log がない。
- `trusted_root.jsonl` を使い回して新しい署名素材を取り込む運用。生成以降の revocation は検知できない。
- `--format=json` の全フィールドを対等に扱う運用。JSON 出力内の `statement.predicate` は偽装され得る。
- SLSA の signer–builder pair 規則の無視。特定 signer が別 builder の provenance に署名する組み合わせは MUST で受け入れてはならない。

## 適用版と本番での注意

- 2026-09-26 に GitHub CLI manual (`gh attestation verify`)、GitHub Docs 3 ページ (use-artifact-attestations / artifact-attestations / verify-attestations-offline)、SLSA Specification v1.2 の Build: Provenance ページを取得して確認した。いずれも取得時点の current ページで、ページ上の版ラベルはない (SLSA のみ v1.2 / Approved 明記)。将来も最新とは扱わない。
- 本文の `expires_at` は 2026-12-25 (official_docs TTL 90 日)。
- **未確認**: 各 flag の GitHub CLI 最小対応バージョン、`actions/attest@v4` 以外の major version の動作、GitHub Actions が生成する `externalParameters` の実値と `buildType` URI の実体、in-toto Statement 上位フィールド (`_type`/`subject`) の詳細、`gh attestation` の将来の flag 追加。実装前に該当公式ページを再取得して確認する。
- 本文の MUST/SHOULD/MAY は SLSA 仕様の normative 表記の引用であり、「推奨方法」節は本リポジトリの設計案と区別している。

---
{
  "id": "github-actions-lockfile-reusable-verification-boundary",
  "title": "GitHub Actions lockfile preview: reusable workflow 呼出しと検証成功の境界",
  "kind": "knowledge",
  "technology": "github-actions",
  "version": "public preview 2026-10-14; maintainer correction 2026-10-20; gh-actions-lock main 306099fd71536793d6f1bab556ab318dcd8cb975 (2026-10-22), release tagではない; live Docs verified 2026-10-23 UTC; CLI/backend未実行",
  "tags": [
    "research-domain:infrastructure",
    "github-actions",
    "actions.lock",
    "gh-actions-lock",
    "reusable-workflow",
    "jobs.uses",
    "verify",
    "verify-local",
    "SHA-pinning",
    "Dependabot"
  ],
  "sources": [
    {
      "id": "github-lockfile-preview-20261023",
      "url": "https://github.blog/changelog/2026-10-14-github-actions-workflow-lockfiles-in-public-preview/",
      "type": "release_notes"
    },
    {
      "id": "github-lockfile-concept-20261023",
      "url": "https://docs.github.com/en/actions/concepts/security/workflow-dependency-lockfiles",
      "type": "official_docs"
    },
    {
      "id": "github-lockfile-adoption-20261023",
      "url": "https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies",
      "type": "official_docs"
    },
    {
      "id": "github-lockfile-enforcement-20261023",
      "url": "https://docs.github.com/en/actions/how-tos/administer/enforce-workflow-lockfiles",
      "type": "official_docs"
    },
    {
      "id": "github-lockfile-cli-snapshot-20261023",
      "url": "https://github.com/github/gh-actions-lock/tree/306099fd71536793d6f1bab556ab318dcd8cb975",
      "type": "github_repository_analysis"
    },
    {
      "id": "github-lockfile-reusable-correction-20261023",
      "url": "https://github.com/github/gh-actions-lock/issues/179#issuecomment-6294039032",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# GitHub Actions lockfile preview: reusable workflow 呼出しと検証成功の境界

## 問いと結論

GitHub Actions の依存を `actions.lock` に移すとき、すべての `uses:` を tag に戻し、`gh actions-lock --verify` が成功すれば固定済みと判断してよいか。

**いいえ。2026-10-23 に確認した CLI snapshot では、他 repository の reusable workflow を呼ぶ job-level `jobs.<id>.uses` は lock 対象外である。** step-level action の固定、reusable workflow の呼出し先、呼出し先内部の依存、container image を分けて棚卸しする。CLI が検査する集合の成功を、実行される全依存の固定完了へ広げない。

本書は[起動ポリシーの既存文書](execution-policy-workflow-path-boundary.md)とは異なり、実行を許可する actor/event ではなく **実行コードの依存を何で固定し、何を検証できたか**を扱う。リポジトリの workflow、権限、保護設定を変更する手順の実施記録ではない。

## 適用時点と資料間の不一致

- [公式告知][release]は **2026-10-14 の public preview**。同日の編集注記で GitHub Enterprise Cloud、data residency付きEnterprise Cloud、Team、Free を対象として列挙した。GAとは書かない。
- [concept Docs][concept]には reusable workflow も固定するという説明が残り、[導入Docs][adopt]は共有workflow repositoryを先にonboardするよう案内する。しかし、[2026-10-20のmaintainer訂正][correction]はこの説明を誤りと認めている。短期案として述べたCLIによる任意SHA pinも、そのコメントだけで提供済みとは判断できない。
- 実装の確認点は `gh-actions-lock` main commit `306099fd71536793d6f1bab556ab318dcd8cb975`（2026-10-22 16:18:34 UTC）。[固定README/コード][snapshot]を読み、release tagや全配布binaryの振る舞いには一般化しない。
- 導入・強制設定Docsの利用条件にはEnterprise Cloud表記が残る。訂正済み告知の対象planと、各accountで利用できる管理機能を同一視しない。とくに[強制設定Docs][enforce]の **Evaluate はEnterprise限定**であり、全planで試せると案内しない。GHESは固定READMEの対応対象外。

これは「officialなら常に同じ契約」という事例ではない。告知日、live Docs、maintainerの訂正日、固定CLI snapshotをそれぞれ証跡に残す。

## 1. lockfile が記録するものと実際のcoverage

[concept Docs][concept]上の `actions.lock` は `.github/workflows/` に置く生成物で、workflowごとの依存集合と、参照先commit・source repositoryの数値identityを保持する。onboard済みworkflowが使う対象actionはentryが必要になり、実行とallowed actions policyの照合にはlocked commitが使われる。固定しても、そのcommitの内容が安全だと審査したことにはならない。

確認した境界は次のとおり。

| 依存の種類 | この調査で確認した扱い | 移行時の判断 |
| --- | --- | --- |
| step-level remote action とそのaction依存 | lockfileの対象 | entryと実行commitの差分をレビュー |
| 他repoの job-level reusable workflow 呼出し | 固定CLIではlock対象外 | 既存のfull-length SHA pinを外さない |
| reusable workflow内部のaction | caller側lockfileへ包括されると考えない | callee側の導入とレビューを別に確認 |
| container / service / Docker action のimage | 固定READMEではlockfile対象外 | image digest管理を別に残す |

表の後半は[固定READMEと実装][snapshot]に基づく。calleeをonboardしただけで、callerの可変tagからそのcalleeへ安全に到達できる保証は得られない。逆にcallerのSHA pinだけでも、callee内の可変action/imageまで自動固定されない。これは依存を別々に管理するための設計判断である。

### 固定コードから確認した「分類」と「固定対象」の違い

[internal/uses/uses.go][classify] の `Classify` は、正しい位置にあるremote reusable referenceを `ReusableWorkflow` に分類する。[internal/workflowfile/workflowfile.go][extract] の `ExtractActionRefs` は `Action` だけをlocking用 `Refs` に追加し、`ReusableWorkflow` は追加しない。

同snapshotの [TestExtractActionRefs_ReusableWorkflowOnlyAtJobLevel][test] は、job-level呼出しをaction集合から外し、stepへ誤配置したreusable参照をmalformedとして扱う。つまり **syntaxとして認識できることと、依存としてpinすることは別**である。[固定コード][snapshot]

Issue #179にはv0.2.0で外部reusable呼出ししかないworkflowが空の依存集合となり、`--verify` が通る再現報告がある。これは上流報告であり、本調査による実行結果ではない。後続のmaintainer訂正と固定コードの読解が、そのcoverageの不足を裏付ける。[訂正と背景][correction]

## 2. verify の成功を三つに分ける

| 確認 | 得られる証拠 | それだけでは言えないこと |
| --- | --- | --- |
| `--verify-local` | offlineのcoverage検査 | upstream provenanceの再検証、対象外依存の固定 |
| `--verify` | networkを使う検証。固定READMEはprovenance確認を含むと説明 | 全 `uses:` とimageがlockされた、依存コードが安全 |
| GitHub上のworkflow実行 | 実際のaccount・runner・policyで受理された結果 | 検査対象外の可変参照まで固定されている |

前二者の説明は[固定README][snapshot]と[強制設定Docs][enforce]を照合した。offline checkをonline verificationの代用にせず、online成功も対象外のreusable呼出しを救済するものとはしない。新しいCLIでcoverageが変わった場合は、このsnapshotの制限を流用せず再確認する。

なお固定READMEは、floating refのlockfileを同じ履歴内の古いcommitへ戻す変更を検出できないとも注意する。古いcommitが存在し検証可能であることと、更新の方向が妥当であることは別である。[固定README][snapshot]

## 3. 更新と段階導入で見落としやすい点

[導入Docs][adopt]では、generatorはlockfileだけでなくworkflow参照も変更し、partial versionを具体的なtagへ狭める場合がある。lockfileだけでなく差分全体を読む。またonboard後のallowlistはlocked SHAで照合されるため、tagだけを許可した既存entryが合わなくなる場合がある。解決のために広いwildcardへ機械的に緩めるのではなく、必要なcommitと管理方針を別途レビューする。

Dependabotの更新は、既にonboardされたworkflowの対応lock entryを同じPRへ更新する。一方、**新しいworkflowの初回onboardingは代行しない**。設定が存在するだけで未導入workflowが減るとは限らない。[強制設定Docs][enforce]

通常のgenerator実行では、`main` やpartial versionの既存pinを無条件に追従させない。移動を受け入れる際はupstream履歴を調べてから `--relock` を使う。元commitが到達不能な場合に使う `--accept-moved` は、異常を調べず定期処理へ追加するものではない。[導入Docs][adopt]

`Require lockfile` の対象へ未導入workflowを一度に入れるとrunを止め得る。まず対象を限定し、生成・レビュー・継続更新が成立することを確認してから強制する。Evaluateが使えないaccountで「評価モードで確認済み」と記録しない。[強制設定Docs][enforce]

## 4. 独自の導入判断と境界試験案（未実行）

1. caller YAMLからstep action、job-level reusable call、local action、imageを別列で列挙し、各々の固定手段を記録する。
2. このsnapshotではremote reusable callの既存SHA pinを維持する。callee側のaction lockとimage digestも別に確認し、片方で代用しない。
3. isolated branchでgeneratorを試す際、CLI版/commitを記録し、workflow書換えとlockfileのcommit・identity差分を一緒にレビューする。
4. offline coverage、online provenance、実際のrun結果を別の証跡として保存する。未確認のpolicy範囲やcallを成功欄へ入れない。
5. 導入対象を広げる前に、新規workflow、Dependabot更新、moved tag、callee変更の各経路を確認する。

最小の境界試験は、remote reusable呼出しだけのworkflowで「検証成功なのにentryなし」を検出できるか、同じ参照をstepへ置いた誤構文が別の失敗になるか、未導入workflow追加がDependabotだけではonboardされないか、image tagがlockfileで固定されたと誤判定されないか、である。これは試験仕様であって、本調査でGitHubにworkflowを作成・実行した結果ではない。

## 未確認事項・出典とライセンス

- 実際のCLI実行、各planの管理画面、runner/backend側の全拒否条件、将来のpreview仕様、全release binary間の比較は未検証。
- live Docsの本文はnative webで確認したが、固定SHAのGitHub web表示は取得失敗した。repository分析はpublic Git cloneで同SHAを確認し、README・関連コード・test・LICENSEを直接読んだ。
- GitHub Docsのライセンスは[github/docs LICENSE](https://github.com/github/docs/blob/main/LICENSE)でCC-BY-4.0、CLI repositoryは固定commitのLICENSEでMITを確認。告知とissue commentの再配布ライセンスはunknownとして扱う。本文は出典付きの独自要約・分析であり、上流コードや長い文章は転載していない。

[release]: https://github.blog/changelog/2026-10-14-github-actions-workflow-lockfiles-in-public-preview/
[concept]: https://docs.github.com/en/actions/concepts/security/workflow-dependency-lockfiles
[adopt]: https://docs.github.com/en/actions/how-tos/secure-your-work/lock-workflow-dependencies
[enforce]: https://docs.github.com/en/actions/how-tos/administer/enforce-workflow-lockfiles
[snapshot]: https://github.com/github/gh-actions-lock/tree/306099fd71536793d6f1bab556ab318dcd8cb975
[correction]: https://github.com/github/gh-actions-lock/issues/179#issuecomment-6294039032
[classify]: https://github.com/github/gh-actions-lock/blob/306099fd71536793d6f1bab556ab318dcd8cb975/internal/uses/uses.go
[extract]: https://github.com/github/gh-actions-lock/blob/306099fd71536793d6f1bab556ab318dcd8cb975/internal/workflowfile/workflowfile.go
[test]: https://github.com/github/gh-actions-lock/blob/306099fd71536793d6f1bab556ab318dcd8cb975/internal/workflowfile/workflowfile_test.go

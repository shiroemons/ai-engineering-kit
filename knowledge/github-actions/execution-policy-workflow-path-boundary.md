---
{
  "id": "github-actions-execution-policy-workflow-path-boundary",
  "title": "GitHub Actions 実行ポリシー: workflow_path の省略・空配列と pull_request_target の段階適用",
  "kind": "knowledge",
  "technology": "github-actions",
  "version": "github.com workflow execution protections GA 2026-09-17; live GitHub Docs verified 2026-10-03 UTC; REST examples use 2026-03-10; qualified default pull_request_target enforcement scheduled 2026-11-02, GHES unverified",
  "tags": [
    "research-domain:infrastructure",
    "github-actions",
    "workflow-execution",
    "workflow_path",
    "actor-rules",
    "event-rules",
    "evaluate",
    "has_parents",
    "pull_request_target",
    "policy-as-code"
  ],
  "sources": [
    {
      "id": "github-actions-execution-ga-20260917-20261003",
      "url": "https://github.blog/changelog/2026-09-17-workflow-execution-protections-in-github-actions-generally-available/",
      "type": "release_notes"
    },
    {
      "id": "github-actions-execution-concept-20261003",
      "url": "https://docs.github.com/en/actions/concepts/about-actions-policies",
      "type": "official_docs"
    },
    {
      "id": "github-actions-execution-admin-20261003",
      "url": "https://docs.github.com/en/actions/how-tos/administer/control-workflow-execution",
      "type": "official_docs"
    },
    {
      "id": "github-actions-execution-rest-20261003",
      "url": "https://docs.github.com/en/rest/actions/policies",
      "type": "official_docs"
    },
    {
      "id": "github-actions-execution-pr-target-20261003",
      "url": "https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-10-30",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# GitHub Actions 実行ポリシー: 対象の選択と起動の許可を分ける

## 問い・結論・適用版

特定の deploy workflow だけ起動者を絞り、残りの CI を動かしたまま運用するには、何を確認する必要があるか。API の条件を省略することと空配列を送ること、既定ポリシーの将来の強制適用と自分で有効化済みの制限を区別する。

結論は、**作成と更新の意図を別に持ち、workflow path・actor・event・適用階層を確認してから active にする**こと。update 要求から `workflow_path` を省くことを「全 workflow へ変更」と扱う同期処理は誤る。許可リストへの追加だけで、他の階層の制限や実行コードの危険性が解消されるとも判断しない。

[公式告知][ga]の公開日は2026-09-17。GA では workflow file targeting、Insights、REST API が加わった。本稿は2026-10-03 UTC に実際に開いた github.com 向け公式資料に基づく。Docs に固定版・公開日の表示はなく、[REST reference][rest]の例にある API header は `2026-03-10` である。この API 版が機能の初出とは主張しない。GHES の対応版は未確認。

[既存の cache-mode 文書](../supply-chain-security/github-cache-mode-reusable-default-poisoning-boundary.md)は cache の read/save 能力を扱う。本稿の範囲は **workflow を起動できる条件**であり、cache mode の再説明や変更は行わない。

## 1. 公式契約: actor と event の許可は同時に必要

[Actions policies の概念][concept]では、enterprise・organization・repository の各階層で起動を管理する。actor rule は起動者、event rule は起動イベントの allowlist であり、強制適用中の制限に反する run はエラーになる。コードへの write access と CI の起動権を分けられるため、共同開発者であることだけを許可の根拠にしない。

[設定手順][admin]は各階層の保護を重ねる設計を示す。actor と event の両方を制限した場合、許可された actor でも許可されていない event からは起動できない。したがって以下は独自の運用判断である。

- deploy 用と一般 CI 用で対象を明確にし、ひとつの大きな例外リストに集約しない
- 下位での許可を上位制限の解除として設計しない。制限を置いた account と実際の対象を記録する
- workflow YAML の trigger 設定だけをレビューして、外側の起動ポリシーを確認済みとしない

利用範囲は全 public repository と、GitHub Team / GitHub Enterprise の private repository。管理者が新規ポリシーで選ぶ Evaluate は手順上 GitHub Enterprise Cloud に限定される。これと、後述する GitHub 提供の default policy が evaluate で配布される話を混同しない。[設定手順][admin]

GitHub 機能の組込み処理には例外があるが、その identity を利用して自分で作った workflow まで一律免除ではない。例えば `dependabot[bot]` で起動する独自 workflow は、必要な allowed actor として扱う。[設定手順][admin]

## 2. 公式契約: workflow_path の省略・空配列・~ALL

以下は [REST reference][rest]の organization / repository policy を読んだ整理である。新規 repository 条件について、JSON schema に収まることだけでは受理を保証しない。

| 操作・値 | 確認した意味 |
| --- | --- |
| create で `workflow_path` を省略 | 明示条件を保存せず、全 workflow が対象 |
| update で `workflow_path` を省略 | 既存 targeting を保持。解除ではない |
| `include` が空配列、`exclude` に pattern あり | 除外に一致しない全 workflow が対象 |
| `include` が `~ALL` のみ | 全 workflow を含める指定。除外は別に評価 |
| `exclude` に一致 | その条件は不成立 |

`include` と `exclude` はともに配列が必要。`~ALL` は include 内で他の pattern と併用できず、exclude には使えない。新規・変更する repository の workflow 条件は、include または exclude の少なくとも一方に pattern が必要で、両方空配列は server-side validation に反する。古い保存値を表現できる schema と、新しい要求が受理される条件は別である。[REST reference][rest]

organization の `conditions` は `repository_name` / `repository_id` / `repository_property` のいずれかと、任意の `workflow_path` を組み合わせる。一方 repository policy の conditions は空、または workflow_path を持つ形である。organization 用 payload を repository 用へ無変更で流用しない。[REST reference][rest]

ここからの独自の設計案として、policy-as-code の入力で「変更しない」「特定 path を対象にする」「全体を対象にする」を別の操作として表現する。serializer が欠落値を空配列に置換したり、設定ファイルから key を消しただけで remote が初期状態へ戻ったと判定したりしない。応答を再読込し、差分を意図と照合する。

一覧の `has_parents` は既定 true で、適用される上位 policy を含む。ページングもあるため、1ページまたはローカル階層だけの一覧を全有効制限とは見なさない。調査時の List/Get も REST 文書では該当 scope の Administration write 権限を要求している。read-only の HTTP 操作であることと token の必要権限を分け、監査目的だけで無断の権限拡張をしない。[REST reference][rest]

## 3. 2026-11-02 は特定 default policy の将来の強制適用日

2026-10-03 時点の [pull_request_target の公式説明][prtarget]は、次を区別している。

- 対象は、適用される Actions event policy がまだない public repository。private / internal repository は対象外で、既存の適用可能な event policy は置換しない
- GitHub 提供の default policy は現在 evaluate。対象 event を使う workflow が動き続けても、その観測だけでは今後も許可される根拠にならない
- 2026-11-02 の自動強制適用について、告知は **GA 前から default pull_request_target policy を使っていた affected repository** と条件を付けている。全 repository が同日一律に切り替わるとは書かない

[GA 告知][ga]と[安全利用の説明][prtarget]は、この対象条件と期日で一致する。一般的な概要ページの短い期日だけで対象集合を拡張しない。手動設定済みの active policy による現在の拒否と、default policy の予定変更も別件である。

移行の選択肢は、不要なら `pull_request` 等への変更を検討する、必要性と安全性を確認した workflow だけ適用可能な event policy で明示許可する、または既定の拒否を維持すること。期日を理由に全 workflow の `pull_request_target` をまとめて許可する設計にはしない。[安全利用の説明][prtarget]

## 4. 起動許可は fork コードの信頼確認を代行しない

`pull_request_target` は base repository の token と secrets にアクセスする高信頼の実行文脈を持つ。fork のコードを checkout しただけで直ちに実行したことにはならないが、その後の build・test・依存関係や設定の処理で攻撃者のコードが実行され得る。`workflow_run` 等から取得した artifact を実行する経路にも同じ種類の危険がある。[安全利用の説明][prtarget]

本稿の設計判断として、例外の承認対象は「event 名」だけでなく、workflow path、起動者、受け取る入力、実行されるコード、必要権限の組にする。承認済み actor が起動する場合でも入力由来の信頼性を別に確認する。起動制限が通った事実を、checkout 先や artifact 内容の安全性の証明として扱わない。

cache-mode、GITHUB_TOKEN の最小権限、runner 分離は引き続き独立したレビュー項目である。既定起動制限の opt-out を、これら全体の安全確認が済んだという意味にしない。

## 5. 導入手順と境界試験案（独自案・未実行）

1. workflow ごとに path、trigger、通常の actor、必要権限、所有者を記録する。fork 対応・定期実行・手動起動など、発生頻度の低い経路も対象にする
2. 適用される上位 policy と既定 policy を取得し、対象条件と enforcement を控える。取得できなかった階層は「制限なし」ではなく未確認として扱う
3. path の境界を限定した候補を作り、利用できる plan では evaluate と Policy insights で影響を観察する。insights は active の実拒否と evaluate の仮想拒否を分けて示す。[設定手順][admin]
4. active へ進める前に、許可される経路・拒否される経路・まだ観測していない経路をレビューする。「一定期間エラーがなかった」だけでは、未実行 workflow の到達性は分からない
5. API 変更後の値、実行結果、insights を照合する。事故時は原因となる path・actor・event・階層を特定し、関係ない workflow まで広げる blanket allow を復旧手順にしない

以下は本番でポリシーを変更せずに準備できる試験仕様である。実際の GitHub 設定変更・API mutation・Actions run は本調査で行っていない。

| 境界試験 | 合格時に残す証跡 |
| --- | --- |
| create と update の省略比較 | 初期値と、既存 target が残る更新結果を別に保存 |
| include/exclude の serializer | 空・欠落・全体指定を意図せず相互変換しない入力検査 |
| 許可 actor + 不許可 event | actor 許可だけで通ったと誤判定しない結果 |
| 下位 allow + 上位 restriction | 制限元を追跡できる policy / insights の組 |
| GitHub 機能の独自 workflow | bot identity の扱いを組込み処理と区別した確認 |
| fork PR と release artifact | 起動成功の検査とは別の入力信頼性レビュー |
| 低頻度の予定 job | 未観測であること、検証予定、業務上の停止許容範囲 |

## 限界・来歴・再確認

- github.com の公式文書の読解であり、特定 account の plan、配布済み既定 policy、実効 enforcement は確認していない。個別 repository の移行時刻や GA 後作成 repository の具体的な rollout は断定しない
- enterprise API の request schema、glob の細かい照合規則、reusable workflow の actor 解決、policy 更新と実行開始の競合時の原子性は未確認。organization / repository の契約をそのまま流用しない
- API の version header は閲覧した例の値である。GHES、旧 API 版、runner / action の最小版に対する互換性を保証しない
- GitHub, Inc. の Docs は [github/docs LICENSE][license] の CC-BY-4.0 を取得日に確認した。出典を明記した原著要約で、構成・設計案・試験案は本稿で作成した。Changelog の開放ライセンスは確認できず catalog は unknown。コードや長文の転載はない
- 明示期限は2026-10-30。release notes の取得日+30日より早く、予定された2026-11-02 の前に対象・期日・live 文書を再確認するための期限である。将来の enforcement 完了を確認した記録ではない
- `evals/knowledge/infrastructure.json` の追加 eval は検索到達性を確認するもの。上記のサービス実行試験とは区別する

[ga]: https://github.blog/changelog/2026-09-17-workflow-execution-protections-in-github-actions-generally-available/
[concept]: https://docs.github.com/en/actions/concepts/about-actions-policies
[admin]: https://docs.github.com/en/actions/how-tos/administer/control-workflow-execution
[rest]: https://docs.github.com/en/rest/actions/policies
[prtarget]: https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target
[license]: https://raw.githubusercontent.com/github/docs/main/LICENSE

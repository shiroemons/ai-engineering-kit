---
{
  "id": "github-actions-blob-egress-inventory-boundary",
  "title": "GitHub Actions blob宛先追加: Meta一覧・egress許可・証跡取得の境界",
  "kind": "knowledge",
  "technology": "github-actions",
  "version": "GitHub.com change announced 2026-10-22; live Meta API and GitHub Docs checked 2026-10-24 UTC; GHES/data-residency environments and firewall runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "github-actions",
    "blob",
    "egress",
    "allowlist",
    "meta",
    "job-summary",
    "logs",
    "CNAME"
  ],
  "sources": [
    {
      "id": "github-actions-blob-expansion-20261022",
      "url": "https://github.blog/changelog/2026-10-22-meta-api-endpoints-updated-for-actions-blob-storage/",
      "type": "release_notes"
    },
    {
      "id": "github-meta-contract-20261024",
      "url": "https://docs.github.com/en/rest/meta/meta?apiVersion=2026-03-10",
      "type": "official_docs"
    },
    {
      "id": "github-meta-blob-observation-20261024",
      "url": "https://api.github.com/meta",
      "type": "official_docs"
    },
    {
      "id": "github-ip-list-limits-20261024",
      "url": "https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses",
      "type": "official_docs"
    },
    {
      "id": "github-selfhosted-egress-functions-20261024",
      "url": "https://docs.github.com/en/actions/reference/runners/self-hosted-runners",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Actions blob宛先追加: jobの起動成功を証跡取得の成功と分ける

## 問いと今回の差分

仮定の障害例として、閉域のself-hosted runnerでjobが起動し、GitHub APIにも接続できるのに、job summaryやlogの取得だけが途切れる。Meta APIの`actions`をfirewallへ登録済みなら、blob通信も網羅したと言えるか。

[2026-10-22の公式発表][release]は、github.comのActions blob storageに次のhostを追加したと説明する。いずれも両端を含む。

- `productionresultssa30.blob.core.windows.net`〜`productionresultssa49.blob.core.windows.net`
- `productionresultssa100.blob.core.windows.net`〜`productionresultssa199.blob.core.windows.net`

個別hostを列挙している環境では、この追加を反映しないとjob summary/logに支障が出る可能性がある。一方、既に`productionresultssa*.blob.core.windows.net`を許可しているなら、**今回の追加だけを理由とする変更は不要**。これを、Actionsの全通信要件を満たす保証や、より広いwildcardを一律に許可すべきという勧告へ拡大しない。GHESやdata residency環境への同一適用は未確認である。

## 一覧の種類を混ぜない

[Meta契約][meta-doc]ではIPとdomainの情報が返るが、Docs上の応答は例示であり、実際の値はAPIへ問い合わせる必要がある。[2026-10-24 UTCの公開API応答][meta-live]を読み取ったところ、`domains.actions`に上の追加120hostが全件存在した。これは当日の一覧の観測であり、疎通試験ではない。

同じ応答のtop-level `actions`はCIDR配列で、`domains.actions`はhost名の配列だった。[IP説明][ip]は、`actions`/`actions_macos`をGitHub-hosted runnerのIP範囲として説明し、MetaのIP一覧自体も網羅的でないと注意している。**runnerの送信元IPと、runnerがアクセスするblob宛先domainを同じallowlistとして扱わない**。

API snapshotから得た一覧は取得時刻付きのinventoryとして保存し、前回との差分を確認する。GitHubはIP変更があり得るため、IPで許可する場合の定期監視を求め、IP単位の許可を推奨していない。API失敗・欠落field・型変更を空の許可リストとして適用しない、という運用はここからの独自提案である。

## self-hosted runnerの機能別egress

[runner reference][runner]はoutbound HTTPSのport443を求め、通信先を機能別に分けている。基本動作の`github.com`/`api.github.com`/`*.actions.githubusercontent.com`と、summary・log・workflow artifact・cacheの転送先`results-receiver.actions.githubusercontent.com`/`*.blob.core.windows.net`は別に記載される。基本動作やcheckoutの成功だけでは、後者の経路まで確認できない。

同資料はCNAMEを使うdomainがあること、firewallによってはCNAME先を再帰的に許可する必要があり、その参照先は将来変わり得ると説明する。現在のIPを一度名前解決して固定するだけの構成を、domain更新へ追従できる構成と同一視しない。

organization/enterprise側のIP allowlistを有効にしている場合にrunnerのIPを登録する要件は、runner側egressの宛先許可とは別である。どちらか一方を設定したことを、逆向きの許可まで済ませた証拠にしない。

## 実務上の判断と確認手順

以下は公式仕様から導く独自の運用案であり、GitHubが提供する自動修復や可用性保証ではない。

1. 影響を切り分ける。対象がgithub.comか、送信元IPの許可か宛先domainの許可か、個別host列挙かwildcardかを記録する。今回発表のhost範囲だけで既存リスト全体を置換しない。
2. live Metaと機能別Docsの差分をレビューする。今回の120件は追加差分であり、全Actions宛先の完全なinventoryではない。Docs例示をそのまま本番設定へ転記しない。
3. ネットワーク管理者の承認のもとで、必要な追加先だけを扱う。接続失敗を理由にTLS検証や全egress制限を無効化しない。wildcardの対応可否・DNS/CNAME追跡・proxy経由の挙動は利用製品ごとに別途確認する。
4. 代表runnerと通常のproxy/DNS経路で試験する。jobの受理、テスト終了、summary表示、log取得、利用するartifact/cacheの往復を別々に観測する。最後の2機能は一般的なDocs要件の検証であり、10月22日の発表が各機能の障害を報告したという意味ではない。
5. 失敗時は時刻・run/job ID・宛先host・DNS/TLS/HTTP段階を残す。認証付きURLやquery中のtokenは共有しない。logが見えないことだけで、job本体が未実行と断定せず、外部へ副作用を起こすjobを無条件再実行しない。

| 観測 | 言えること | 言えないこと |
|---|---|---|
| 新hostがMetaにある | 当日の公開inventoryへ掲載済み | 自組織のfirewallへ反映済み |
| runnerがonlineでjobを開始 | 基本通信と割当経路が動作 | summary/log/blob経路も疎通済み |
| narrow wildcardが既に許可済み | 発表された追加hostは同patternに一致 | 全Actions機能や全CNAME先まで許可済み |
| job終了statusは成功だがlogを取得できない | 実行結果と証跡取得に別の観測がある | job未実行、または再実行が安全 |

## 適用範囲・未確認事項

- host追加の発表日は2026-10-22、本文とlive APIの再取得日は2026-10-24 UTC。段階的な展開時刻、個別組織への影響開始時刻、deadlineの公表は今回の資料から確認できない。
- 本調査は公開資料と読取専用のMeta取得だけ。firewall・runner・organization設定は変更していない。実際の疎通、proxyやTLS inspection製品、GHES、data residency環境は未検証。
- API応答のSHA-256は`eef96b429e4f5a6adbc448d3407fdea9ee943920d50e5ae496c23108b49571f6`。追加2区間を生成して`domains.actions`との差集合が空であることを確認した。応答全体や実運用設定の永続保証ではない。
- 既存のretention文書は証跡の保持期間・API検索範囲、execution-policy文書はworkflow起動権を扱う。本書は**ネットワーク宛先inventoryと証跡転送経路**の別問題を扱い、保持期限や起動権の説明を重複させない。
- GitHub Docsは[CC-BY-4.0のLICENSE][license]を確認し、独自に要約した。BlogとAPIデータの転載許諾は未確認としてcatalogを`unknown`とし、コード・応答全体・文章の大量転載はしていない。module化は対象外。

[release]: https://github.blog/changelog/2026-10-22-meta-api-endpoints-updated-for-actions-blob-storage/
[meta-doc]: https://docs.github.com/en/rest/meta/meta?apiVersion=2026-03-10
[meta-live]: https://api.github.com/meta
[ip]: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses
[runner]: https://docs.github.com/en/actions/reference/runners/self-hosted-runners
[license]: https://github.com/github/docs/blob/main/LICENSE

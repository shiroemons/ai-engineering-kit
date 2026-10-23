---
{
  "id": "testing-pact-environment-version-deployment-gate",
  "title": "Pact Broker 配備gate: 共存provider版・pending結果・deployment台帳の整合性",
  "kind": "knowledge",
  "technology": "testing",
  "version": "Pact Broker v2.120.0 environment/cvpv fix (2026-07-16); v2.122.0 @ 818e7bc48967f602fe3aca95c6138072e7e78307 (2026-10-01); official rolling guides checked 2026-10-23 UTC; Broker/CLI runtime untested",
  "tags": [
    "research-domain:quality-operations",
    "testing",
    "pact",
    "contract-testing",
    "can-i-deploy",
    "cvpv",
    "pending-pacts",
    "deployment",
    "release",
    "verification"
  ],
  "sources": [
    {
      "id": "pact-broker-can-i-deploy-contract-20261023",
      "url": "https://docs.pact.io/pact_broker/can_i_deploy",
      "type": "official_docs"
    },
    {
      "id": "pact-broker-deployment-lifecycle-20261023",
      "url": "https://docs.pact.io/pact_broker/recording_deployments_and_releases",
      "type": "official_docs"
    },
    {
      "id": "pact-broker-pending-build-result-20261023",
      "url": "https://docs.pact.io/pact_broker/advanced_topics/pending_pacts",
      "type": "official_docs"
    },
    {
      "id": "pact-broker-verification-publication-20261023",
      "url": "https://docs.pact.io/pact_broker/advanced_topics/provider_verification_results",
      "type": "official_docs"
    },
    {
      "id": "pact-broker-matrix-selector-doc-drift-20261023",
      "url": "https://docs.pact.io/pact_broker/advanced_topics/matrix_selectors",
      "type": "official_docs"
    },
    {
      "id": "pact-broker-environment-cvpv-v2120-release-20261023",
      "url": "https://github.com/pact-foundation/pact_broker/releases/tag/v2.120.0",
      "type": "release_notes"
    },
    {
      "id": "pact-broker-environment-gate-v2122-code-20261023",
      "url": "https://github.com/pact-foundation/pact_broker/tree/818e7bc48967f602fe3aca95c6138072e7e78307",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# 契約テストの成功を、共存する全版との配備互換性へ拡大しない

## 問いと適用範囲

providerのCIが緑、または `can-i-deploy` が成功したとき、実環境で使われている相手の全バージョンと互換なのか。Pactでは「何を検証したか」「どの版を照会したか」「何が配備中と登録されているか」が別々の情報であり、どれかが欠けると緑の意味が変わる。

本稿は契約テストから配備判断へ渡す証拠の契約を扱う。Pact Broker v2.120.0の共存版取り落とし修正を、v2.122.0の固定commitで確認した。[既存のtest scope](test-scope-doubles-hermetic-flaky.md)はテストの分類、[canaryの昇格判定](../sre/canary-promotion-evidence-population-boundary.md)は配備後の観測十分性を扱う。本稿は配備前のconsumer/provider組合せと台帳の正確性であり、新たなtechnology分類は追加しない。

## 最初に分ける四つの識別子

[公式の結果公開手順][publication]と[can-i-deploy手順][gate]によると、consumerが生成したpact、検証したprovider application version、配備候補のapplication version、照会先environmentを関連付けて判断する。providerは正しい版を設定してverification結果の公開を有効にする必要がある。

実務上は次を別欄で残す設計を勧める。これは本稿の証拠管理案で、Brokerの新しいAPIではない。

- consumer artifactと、その生成元application version・pact内容
- 検証したprovider artifactとprovider application version・branch
- 成否を公開したverificationの参照先と、その照会結果
- 実際に配備するartifact、候補版、対象environment、照会時刻

`main` は本番配備済み版の別名ではない。候補を動く `--latest` に任せるより、配備artifactと対応する `--version` を指定し、`--to-environment` を明示する。targetを省いたCLI照会は相手の全体の最新の版を対象とし、本番台帳への照会へ自動変換されない。[gate]

同じpact内容に対する既存verificationの再利用は公式に説明されている。「今回のconsumer buildの後にprovider testが必ず再実行されたこと」を必須条件にするのも誤りである。新旧consumer versionが同じ契約を参照し得ることと、誤ったprovider版の結果を流用してよいことは別問題になる。[publication]

## v2.120.0の修正: 共存版を最新1版へ畳み込まない

### releaseと実装を照合した結果

[2026-07-16公開のv2.120.0 release][release]には、environment向け `can-i-deploy` を `cvpv` に変更する修正が含まれる。変更が存在することだけで全配備の安全を主張せず、次の経路を[v2.122.0固定source][code]で読んだ。

- `lib/pact_broker/matrix/parse_can_i_deploy_query.rb`: 既定の `latestby` は `cvp`。Stringのenvironmentを受ける経路だけ `cvpv` に上書きする。tagの `to` は別経路で、`cvp` と `latest: true` が残る
- `lib/pact_broker/api/resources/can_i_deploy_pacticipant_version_by_branch_to_environment.rb`: branchの最新候補からenvironmentへ照会する経路も `cvpv` を使う
- `lib/pact_broker/matrix/repository.rb`: `apply_latestby` は `cvp` と `cvpv` で異なるgrouping keyを選ぶ。前者はconsumer versionとprovider、後者はconsumer versionとprovider versionを区別する

意味の差は「最新結果を使うか」だけではなく、「異なるprovider版を同じグループへ入れるか」である。共存する旧provider版の失敗を、新provider版の成功で置き換えてはいけない。

### 独自の反例

候補consumer C7がprovider P40とは不適合、P41とは適合し、P40とP41が両方とも対象environmentでsupported releaseとして登録されているとする。

- P41の成功1行だけを見る判断は誤り。C7→P40とC7→P41の2組を残し、全体を不適合として扱う
- P40とP41が異なる永続application-instanceへ配備されている場合も、片方の成功では足りない
- 一方、同じinstanceへの配備がP40をP41で本当に置換し、P40が対象から外れた場合、過去の失敗を永久にgateへ残す必要はない

これは数値・名称を変えた説明用モデルで、実測ではない。根拠は `spec/features/can_i_deploy_spec.rb` のHTTP経路の回帰ケースである。複数release・複数instanceの一部失敗ではmatrixを2行とし `summary.deployable` をfalseにする。両方成功なら2行でtrue、同一instanceの置換後は1行となる。HTTP成功レスポンスを受け取ることと、配備可否がtrueであることは別である。[code]

### 公式ページにも古い例が残る

取得した[Matrix selectors頁][selectors]は最終更新2023-08-31と表示され、environmentに `latestby: "cvp"` を組み合わせる例が残っていた。これはv2.120.0のrelease記載と、v2.122.0のenvironment経路の実装に一致しない。

したがって、この例をそのまま独自のMatrix API wrapperへ写さない。採用版の `can-i-deploy` 経路と、その実際のquery optionsを確認する。修正は「全Matrix queryの既定値をcvpvへ変更した」という意味ではない。tagのみ、対象なし、独自queryを同じ修正範囲だと推測しない。本稿は影響を受け始めた最初の旧版や各CLI実装の送信形式までは確定していない。

## provider buildの緑とpendingの失敗は両立する

[Pending pacts頁][pending]によると、pendingはpact内容・公開済みverification・provider branchから動的に決まる。pendingなpactのmismatchはprovider verification taskを失敗終了させないことがあるが、結果公開が有効ならBrokerへは失敗として記録される。consumerがその新契約を使って配備できる、という許可ではない。

たとえば本番consumer C6との契約は適合し、未配備C7の新契約だけ不適合でも、pendingによりproviderの通常buildは進められる。その緑をC7の配備証拠へ転用しない。candidate C7と対象environmentを改めて照会する。`enablePending` は結果を成功へ変える設定ではなく、verification publicationを省略してよい設定でもない。[pending]

同頁はprovider変更時の通常workflowと、特定pact URLを渡すcontract-change webhookのworkflowを区別している。ここではpendingの一般原則だけを採用し、古い言語別対応表を2026年の全clientの既定動作として扱わない。

## 台帳の更新が早すぎても遅すぎても、照会対象が変わる

[deployment / releaseの公式契約][lifecycle]では、deploymentは同じapplication・environment・application-instanceの前版を置換する。一方、releaseは既存releaseを終了せず、support終了を別操作で記録する。匿名instanceも独立した値なので、未指定から名前付きへ移すと両方が残り得る。

rolling途中は旧版がまだ応答し得る。公式手順は旧版が残らず配備が完了した最後に `record-deployment` を呼ぶ。rollback時は戻した版を再登録する。releaseのsupport終了とdeploymentの終了は独立である。[lifecycle]

この契約から、次の運用を提案する。

1. 本番で共存するものが置換型なのか、配布後も利用可能なrelease型なのかを先に決める。モバイル旧版を単に最新版の記録で消すモデルにしない
2. rolling開始時に新版だけを登録して「旧版はもういない」と宣言しない。先の例でP41を先行登録すると、P40へ当たるC7を誤って進める台帳になり得る
3. 恒久的な顧客別配備などでinstance名を導入するときは、既存の匿名instanceと実配備を照合する。通すために古い行を消すのではなく、実際に置換・撤去されたかを確認する
4. 配備は終わったがBroker登録に失敗したら、「配備失敗」か「全工程成功」へ丸めず、台帳不一致として止めて照合・修復する。戻していないのに旧版へのrollback登録をしてはいけない
5. `record-support-ended` は失敗行を隠すための掃除ではなく、実際のsupport方針変更として扱う。今も支援すべき利用者をgate対象から外さない

照会後に他の配備が環境を変える競合も残る。確認した契約は、実環境の変更権を予約する分散lockではない。関連する配備を調整し、照会後に前提の版集合が変わったら再照会する設計を勧める。ただし再照会だけで原子的な配備を保証できるとはしない。

## HTTP成功・false・null・空集合を分ける

v2.122.0の `DeploymentStatusSummary#deployable?` とspecを読むと、結果は二値だけではない。[code]

- 考慮対象として指定した版が存在しない場合はfalseになる
- 考慮対象行のsuccessがnil、または必要なintegrationの行が不足する場合はnilになる。JSONではnullとして扱う。unknownがあると、別行の失敗より先にnilが返る経路もあるため、理由とcountsを併読する
- 上記がなければ、考慮対象行すべてのsuccessで判断する。必要な依存が欠けていない空集合ではtrueになり得る。specにはconsumerがいないproviderの配備を許可するケースがある
- ignored rowは考慮対象と分けられる。specには未検証行や失敗行をignoreすると、残りの成功だけでtrueとなるケースがある

独自のCI wrapperはHTTP応答成功だけで通さず、期待するJSON形状とbooleanの `summary.deployable === true` を要求する。`summary.deployable !== false` ではnullを通すので使わない。これはJavaScriptで示した独自の判定方針であり、Pact CLIの実装を転載したものではない。

さらに、trueは「現実の依存を漏れなく登録済み」の証明ではない。必要なintegration一覧・対象版集合・ignoreの有無を別途照合する。空matrixを一律エラーにすることも、0件を無条件の全件成功にすることも避け、consumer未配備が妥当なprovider初回配備と、登録漏れを区別する。dry-runは公式に成功exitとなるため、本番gateへそのまま持ち込まない。[gate]

## 導入先で行う受け入れ試験（未実行）

| fixture / 操作 | 確認する結果 |
|---|---|
| C7対P40が失敗、P41が成功。両版をsupported releaseとして登録 | 共存2版がmatrixに残り、gateを通さない |
| 同じ2版を異なるapplication-instanceへ登録 | 新版の成功が旧版の失敗を覆い隠さない |
| 同一instanceをP40からP41へ実際に置換 | 台帳も置換され、旧版が不要な対象として残らない |
| environment照会と同名tag照会を比較 | CLIが実際に送る対象とquery optionsの違いを記録する |
| pending C7契約は失敗、配備済みC6契約は成功 | provider buildの結果とC7のcan-i-deployを別判定する |
| provider版を誤設定、結果公開を無効化、未検証版を選択 | 想定する組合せの証拠不足を検出する |
| HTTP成功でdeployable=false / null / trueを返すfixture | falseとnullは止める。JSON不正・フィールド欠落も通さない |
| 空matrix、ignoreあり、未登録consumer | 0件や対象削減を想定した受け入れ範囲と照合する |
| rolling途中、完了後の登録失敗、rollback完了 | 実配備とBroker台帳がずれた状態を正常完了としない |
| 匿名instanceから名前付きへ移行 | 旧匿名記録の残存を意図した共存と区別する |

これらは実行手順の設計であり、今回のBroker・CLI・DBを起動した検証ではない。検索evalは文書を引けるかだけを確かめる。upstream specの静的確認を、この環境でのruntime合格や本番システムの受け入れ完了と書き換えない。

## 版・取得日・ライセンスと残る限界

- v2.120.0の公開日はGitHub release APIの2026-07-16T11:01:02Z。tagのcommitは `2393f531f4ad461df786c0316c61cd54349e1f07`。environment/cvpv修正が収録された版として扱う
- コード解析はv2.122.0の `818e7bc48967f602fe3aca95c6138072e7e78307` に固定。CHANGELOGとrelease APIの公開日は2026-10-01。これをあらゆるclientや将来版への保証にしない
- 公式頁の表示更新日: Can I Deployは2022-10-05、deployment/releaseとverification publicationは2023-01-26、pendingは2023-07-24、Matrix selectorsは2023-08-31。取得日はすべて2026-10-23 UTC。古い更新日や2020/2021年の対応表を、新機能の公開日・現在の実装対応表と混同しない
- deployment/release頁に書かれた最低要件はBroker 2.81.0、Ruby CLI 1.47.0、Docker CLI 0.47.0.0。これは環境資源を使える最低版であって、2026年の共存版修正を含む保証ではない。採用clientの実版とBrokerの実版を両方確認する
- コードは固定commitの `LICENSE.txt` でMITを確認。release記載は同repositoryのCHANGELOGにもある。公式ウェブ頁の再利用licenseは確定できずcatalogはunknown。原文の長文翻訳、コード、図のコピーはせず、帰属付きの独自要約と設計案のみを記した。module化は行わない
- native webでは一部GitHub blob / Changelog / CLIリンクを取得できなかった。release本文と公式解説は開き、実装・spec・LICENSE・CHANGELOGはGitHub connectorで固定commitから取得した。読めなかったCLI本文を根拠に未確認flagやexit code一覧を補っていない
- pactに表現していない業務挙動、実トラフィックの経路、性能、認可、deployment同士の直列化、各言語SDKの現行pending対応、独自wrapperの動作は未検証。契約テストによる互換性照会を総合的な本番安全証明にはしない
- release_notesのTTL 30日が最短なので、再確認期限は2026-11-22。修正の歴史が失効する日ではなく、採用版・資料の差を再点検する日である

[gate]: https://docs.pact.io/pact_broker/can_i_deploy
[lifecycle]: https://docs.pact.io/pact_broker/recording_deployments_and_releases
[pending]: https://docs.pact.io/pact_broker/advanced_topics/pending_pacts
[publication]: https://docs.pact.io/pact_broker/advanced_topics/provider_verification_results
[selectors]: https://docs.pact.io/pact_broker/advanced_topics/matrix_selectors
[release]: https://github.com/pact-foundation/pact_broker/releases/tag/v2.120.0
[code]: https://github.com/pact-foundation/pact_broker/tree/818e7bc48967f602fe3aca95c6138072e7e78307

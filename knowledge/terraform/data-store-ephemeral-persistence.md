---
{
  "id": "terraform-data-store-ephemeral-persistence",
  "title": "Terraform 1.16 terraform_data.store: ephemeral入力の永続化とversion更新の境界",
  "kind": "knowledge",
  "technology": "terraform",
  "version": "Terraform v1.16.0導入（2026-08-26）/ Docs v1.16.x / builtin実装 v1.16.4 commit 6ee8af215a535cfca08bf729ff1017d4221d6667（2026-09-23 release）; 2026-10-02 UTC取得、実行未検証",
  "tags": [
    "research-domain:infrastructure",
    "terraform",
    "terraform_data",
    "store",
    "ephemeral",
    "write-only",
    "sensitive_output",
    "state",
    "version",
    "plan-apply"
  ],
  "sources": [
    {
      "id": "terraform-data-store-docs-20261002",
      "url": "https://developer.hashicorp.com/terraform/language/resources/terraform-data",
      "type": "official_docs"
    },
    {
      "id": "terraform-sensitive-storage-docs-20261002",
      "url": "https://developer.hashicorp.com/terraform/language/manage-sensitive-data",
      "type": "official_docs"
    },
    {
      "id": "terraform-write-only-storage-docs-20261002",
      "url": "https://developer.hashicorp.com/terraform/language/manage-sensitive-data/write-only",
      "type": "official_docs"
    },
    {
      "id": "terraform-v1160-store-release-20261002",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.16.0",
      "type": "release_notes"
    },
    {
      "id": "terraform-v1164-store-release-20261002",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.16.4",
      "type": "release_notes"
    },
    {
      "id": "terraform-data-store-implementation-v1164-20261002",
      "url": "https://github.com/hashicorp/terraform/blob/6ee8af215a535cfca08bf729ff1017d4221d6667/internal/builtin/providers/terraform/resource_data.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Terraform 1.16 terraform_data.store の保存境界

## 問いと対象

一時的な値を受け取るwrite-only引数なら、その値はstateに残らないと判断してよいか。terraform_data.storeでは、入力経路と保存先を分けて読む必要がある。store.inputはephemeral入力を受け取れるが、applyで得た値をstore.outputまたはstore.sensitive_outputへ保存するための機能である。state非保存を要求する値の中継先として無条件に選ばない。

本稿はTerraformの組込みresourceにおける保存・更新・plan/applyの契約に絞る。既存の[state lockingとdrift](state-locking-drift-import.md)、[リソース置換順序](replacement-ordering-create-before-destroy.md)とは問いを分ける。secret管理全般の設計や特定cloud providerの設定手順は対象外とする。

## 適用版と日付

- [v1.16.0 release](https://github.com/hashicorp/terraform/releases/tag/v1.16.0)は2026-08-26公開で、terraform_dataのstore block追加を告知した。既存のterraform_data利用可否とstore対応可否を混同しない
- [v1.16.4 release](https://github.com/hashicorp/terraform/releases/tag/v1.16.4)は2026-09-23公開。今回の実装観察はこのreleaseのcommit 6ee8af215a535cfca08bf729ff1017d4221d6667に固定した
- 公式referenceの版表示は取得時点でv1.16.x、v1.17.xはbeta。以下のDocs取得日は2026-10-02 UTCであり、Docs各ページの公開日ではない。公開ページには明示的な公開日がなく、release日を代用していない
- release_notesのTTL 30日を含むため再確認期限は2026-11-01。v1.17以降やOpenTofuへは一般化しない

## 確認した公開契約

### 1. storeの入口と出口を区別する

[terraform_data reference](https://developer.hashicorp.com/terraform/language/resources/terraform-data)に基づく整理:

| 対象 | 契約 | 保存判断への意味 |
|---|---|---|
| 組込みprovider | terraform.io/builtin/terraformを使い、別providerの設定は不要 | 外部providerを導入していないことは非保存の根拠にならない |
| resource直下のinput | 通常のstate保存用引数 | store.inputとは別のfield |
| store.input | 任意型を受け付けるoptional・write-only引数。ephemeralも利用できる | write-onlyは入口の属性 |
| store.output / store.sensitive_output | apply時の入力から値を取り、stateへ保存する。両出力は排他的 | 入力がephemeralだったという来歴だけでは保存を防げない |
| store.sensitive | trueならsensitive_output側を選ぶ | 保存先の表示上の扱いを選ぶ |
| store.version | 指定時はversionも変わらなければ入力変更を保存出力に反映しない | 更新の契機を入力値と切り離せる |
| store.replace | trueならstore出力の変更をresource置換として扱う | 値更新とresource identity更新は別判断 |

このtableはfieldの役割を示すもので、任意型に対する全境界値の実行確認ではない。storeという名前から外部secret managerのような暗号化・権限制御を推測しない。

### 2. sensitive・ephemeral・write-onlyは異なる契約

[Manage sensitive data](https://developer.hashicorp.com/terraform/language/manage-sensitive-data)では、sensitiveはCLI/UI上の伏字、ephemeralはその値をstate/planに保存しない仕組みとして説明される。sensitiveだけのvariable/outputはstate/planに残り、terraform output -json / -rawでは平文で読める。local stateは平文であり、remote backendの保存時暗号化はbackendの契約による。

同資料の最低版は、sensitive variable/outputが0.15、ephemeral variable/child module output/resourceが1.10、managed resourceのwrite-onlyが1.11である。これらの導入版とstoreの1.16を混ぜない。root module outputにはephemeralを付けられないため、非保存値をroot output経由で渡す設計にも制限がある。

[write-only guide](https://developer.hashicorp.com/terraform/language/manage-sensitive-data/write-only)は、write-onlyが通常値とephemeral値の両方を受け付け、version引数の解釈はproviderごとに異なると説明する。したがって「すべてのwrite-only値はどの出口にも残らない」という読み方はできない。terraform_data.storeについては、専用referenceにある保存出力の契約を優先して読む。

## 固定実装から確認した更新動作

以下は[Terraform v1.16.4のresource_data.go](https://github.com/hashicorp/terraform/blob/6ee8af215a535cfca08bf729ff1017d4221d6667/internal/builtin/providers/terraform/resource_data.go)を読んだ観察である。公開APIの将来保証ではなく、Terraformを実行して得た結果でもない。v1.16.0の同ファイルとバイト列が一致することも確認した。

- dataStoreResourceSchemaではstore.inputとstore.versionはDynamicPseudoType、sensitiveとreplaceはbool。versionを単調増加する整数だけに制限する実装ではない。比較はRawEqualsであり、型も含めた値の一致を見る
- planDataStoreResourceChangeはversionが非nullなら前のversionと比較する。同じversionでは、新しいinputと保存出力を比較する分岐へ進まない。versionが未設定ならinputと既存output / sensitive_outputの比較で更新を判断する
- sensitiveとreplaceはplan時に既知でなければ診断errorになる。sensitive変更時は保存済み値を出力間で移す処理があり、sensitive=trueへの変更を値の消去として扱えない
- planの返却前にstore.inputはnullへ落とす。更新対象のstore出力はapplyDataStoreResourceChangeで、そのapplyのconfigにあるstore.inputから埋められる。plan時の一時入力値をそのまま復元して使う経路ではない

更新判断の読み方を整理すると、同じversionのまま入力だけを差し替えてもローテーション成立とはいえない。ただしversionは入力変更に伴う出力更新を制御するもので、state全体の凍結ではない。上記のsensitive切替やstore block自体の削除などを抑える保証として使わない。逆にversion未指定で毎回変わる一時値を渡せば、保存済み値との差分が更新候補になる。versionが変わる分岐では入力値の直接比較をせず再捕捉を計画するため、replace=trueを使う場合は同じ値を再供給する予定でも置換計画を確認する。

## 採用判断と運用案

以下は上記契約からの独自の設計提案である。

### 保存を認める場合

値をTerraformのresource lifecycleに合わせて保持したい場合にはstoreが候補になる。採用前に、保存先を「一時入力→write-onlyの入口→state内の保存出力」と記述し、stateへの保存を明示的に認める。機密値ならsensitiveを表示抑制として設定したうえで、backendの暗号化、state読取主体、backup、CI artifactへの流出経路を別々に確認する。

versionには非機密の世代番号や公開revisionを用い、変更PRに「何の値をいつ再捕捉するか」を記すことを勧める。実装が任意型を許すことと、versionに秘密そのものや不必要な複合objectを入れることは別である。世代変更、入力の供給、下流の反映を一つの運用単位として扱う。

replaceは保存値を変える必要性だけでは決めない。terraform_dataのidentity変更がprovisionerや既存の置換連動設定へ及ぼす影響を調べ、値の更新だけで足りるなら置換を追加しない。無停止切替の可否は別途検証する。

### state非保存が必要な場合

storeを通す構成は要求と合わない。対応するwrite-only引数へephemeral値を直接渡す構成を検討する。将来のrunでも同じ値が必要なら、write-only guideで示される「外部secret storeへ保存し、ephemeralで読み直す」という構成上の選択肢がある。ただし特定providerの対応resource・最低版・外部保存結果を本稿は保証しない。write-only入力が本当に最後に消費されるまで、他のoutputや通常resource属性への複製も確認する。

### planとapplyを分ける場合

レビュー対象を「planで見えた秘密の実値」として扱わず、取得元、世代、保存許可、反映対象をレビューする。apply時には入力を再供給できるようにする。失効するtokenや毎回生成する値を扱う場合、plan時の値とapply時の値が同じであることを必要条件にしない設計か、同一値を安全に再取得する設計かを決める。saved planに入力そのものが保持されるという前提では組み立てない。

## 避けたい誤読

- store.inputがwrite-onlyであることを、store.sensitive_outputまで非保存である根拠にする
- sensitive=trueをstate暗号化や過去snapshotの消去とみなす
- inputを変更しただけで、固定versionの保存値も更新されたと報告する
- `_wo_version`という慣例を全provider共通の世代管理仕様として扱う
- ephemeralの導入版1.10だけを満たせばstoreも使えると判断する
- plan/apply間で一時値が変わらないことや、replace=trueによる切替が無停止であることを未検証のまま保証する

## 導入時に行う検証案（未実施）

実際のsecretやcloud resourceを使わない隔離環境で、無害なfixture値A/Bと固定CLI版を使う。次は本稿で実行したテストではない。

1. 初回apply後、store.inputと保存出力を機械可読stateで別々に確認する。sensitive=trueの表示が伏字でも、state側には値があることを確認する
2. version固定でAからBへ入力だけ変更し、保存値がAのままかを検査する。version変更後はそのapplyで供給したBが保存されるかを検査する
3. version未指定で入力を変更した場合、出力更新とresource IDを比較する。replace=trueのケースは別fixtureで、置換計画と下流への影響を観測する
4. 同じ値でsensitiveを切り替え、出力fieldの移動と平文取得経路を確認する。切替前のstate snapshotを安全に保持するテストなら、過去値が消えたとは判定しない
5. saved planを使いplan/applyで異なる無害な入力を供給する。適用時の入力調達、実際に保存される値、失敗時の再試行を検査する
6. string以外を採用する場合は、予定するobject/list/nullと型変更を追加する。any typeというschemaから未知値やnullの全組合せまで推測しない

## 未確認事項とprovenance

Terraform CLI、HCP Terraform、Terraform Enterprise、provider pluginを実行した検証はない。特定backendでのsnapshot保持・削除、旧CLIへのdowngrade、保存値と外部システムの整合性、各providerのwrite-only version解釈は未確認である。本文の検証案は実施結果とは区別する。

公式Docsはnative webで本文を開いた。Docsソースの[web-unified-docs LICENSE](https://github.com/hashicorp/web-unified-docs/blob/main/LICENSE)はBUSL-1.1を示す。Terraform固定commitの実装とroot LICENSEも確認した。固定raw sourceのnative web取得はcache missとなったため、同じ公開raw URLのread-only HTTP取得で補完した。source catalogにはcommitと取得経路を記録している。原文・実装コードの転載、moduleへのコード移植は行っておらず、本文は出典付きの独自要約と設計提案である。

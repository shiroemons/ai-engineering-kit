---
{
  "id": "terraform-action-failure-taint-destroy-retry-boundary",
  "title": "Terraform 1.16.5 action_trigger: taint・destroy継続・失敗後の再実行の境界",
  "kind": "knowledge",
  "technology": "terraform",
  "version": "Terraform v1.16.0でon_failure/destroy events導入 (2026-08-26); v1.16.5 @ ef47237fd0e03d6e93bdc510b526287f06e94c03を確認、release見出し2026-09-30/公開2026-10-02; Docs v1.16.x、2026-10-04 UTC取得; runtime未検証",
  "tags": [
    "research-domain:infrastructure",
    "terraform",
    "action_trigger",
    "on_failure",
    "halt",
    "taint",
    "continue",
    "before_destroy",
    "after_destroy",
    "after_create",
    "retry",
    "DestroyMode",
    "NoOp"
  ],
  "sources": [
    {
      "id": "terraform-action-failure-release1160-20261004",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.16.0",
      "type": "release_notes"
    },
    {
      "id": "terraform-action-failure-release1165-20261004",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.16.5",
      "type": "release_notes"
    },
    {
      "id": "terraform-action-failure-invoke-docs-20261004",
      "url": "https://developer.hashicorp.com/terraform/language/invoke-actions",
      "type": "official_docs"
    },
    {
      "id": "terraform-action-failure-lifecycle-docs-20261004",
      "url": "https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle",
      "type": "official_docs"
    },
    {
      "id": "terraform-action-failure-runtime1165-20261004",
      "url": "https://github.com/hashicorp/terraform/tree/ef47237fd0e03d6e93bdc510b526287f06e94c03",
      "type": "github_repository_analysis"
    },
    {
      "id": "terraform-action-failure-apply-tests1165-20261004",
      "url": "https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/context_apply_action_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "terraform-action-failure-plan-tests1165-20261004",
      "url": "https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/context_plan_actions_test.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Terraform 1.16.5 action_trigger: taint・destroy継続・失敗後の再実行の境界

## 問いと結論

DB削除前のバックアップや、作成後の初期化を `action_trigger` に置いたとき、`on_failure = halt` は削除を必ず止め、`taint` は失敗した操作を必ず再実行するのか。

**どちらも一般的な保証にはならない。** v1.16.5の固定実装では、通常のcreate/updateに対するtaintは新規作成後の `after_create` に限定される。さらに**full destroy modeでは、呼び出したdestroy actionのエラーを警告に変換する**。通常の置換を行うapplyと、`terraform destroy` / destroy-mode planのapplyを同じ失敗経路として扱わない。

以下は公式説明、固定commitの静的読解、独自の運用案を分けたもの。Terraformやクラウドproviderを動かして確かめた結果ではない。既存の[置換順序](replacement-ordering-create-before-destroy.md)と[destroy=falseによる管理解除](forget-replacement-deposed-boundary.md)を補完し、ここでは外部actionの失敗処理だけを扱う。

## 適用版と確認した変更

- [v1.16.0 release](https://github.com/hashicorp/terraform/releases/tag/v1.16.0)の2026-08-26見出しに、`on_failure` の `halt` / `taint` / `continue`、`before_destroy` / `after_destroy`、Stacksでの `caller` 対応、`-invoke` と `-target` の組合せが記載される。既存action機構全体や通常workspaceのcallerがこの版で初登場したという意味ではない
- 実装はv1.16.5の `ef47237fd0e03d6e93bdc510b526287f06e94c03` に固定。[v1.16.5 release](https://github.com/hashicorp/terraform/releases/tag/v1.16.5)の見出しは2026-09-30、GitHub releaseの公開時刻は2026-10-02 13:04 UTC。両者を同じ日付に丸めない
- 同releaseはtainted instanceの不正statusを観測した際のcrash修正を含む。これは `on_failure` の新設ではなく、作成失敗後のstate処理を含む修正である。以下の詳細を古い1.16 patchへ無検証で外挿しない
- 公式文書は2026-10-04に表示されたv1.16.x。[Invoke an action](https://developer.hashicorp.com/terraform/language/invoke-actions)の一般説明より細かい条件は、下記固定sourceに基づく。v1.17 beta、HCP/Enterpriseの実行環境、個々のproviderについて同じ挙動を実証したものではない

## 公式説明から分かる基本契約

[Invoke an action](https://developer.hashicorp.com/terraform/language/invoke-actions)によればactionはproviderが実装する外部操作であり、`action` を宣言するだけで任意のAPIを呼べるわけではない。CLIによる個別呼出しまたはresourceのlifecycleから起動する。

[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)は `actions` を順序付きリストとし、eventと任意のconditionに従って実行すると説明する。失敗時の概要は次の通り。

| on_failure | 公開説明の概要 | 読み落としてはいけない点 |
|---|---|---|
| `halt`（省略時） | エラーを報告してapplyを停止、resourceをtaintしない | 作成済み実体や既に成功した外部操作を元に戻す指示ではない |
| `taint` | エラーを報告して停止、taintedとして次回置換へ | 固定実装では新規作成後に限定。更新失敗全般の再実行スイッチではない |
| `continue` | 警告として記録しapplyを継続 | actionの成功証明ではなく、エラーを処理継続可能なseverityへ変える指定 |

表右列の具体的な根拠とdestroy例外は次節以降。公式文書の「actionはresource stateを変更しない」という概説を、失敗ポリシーによるtaintまで起こらないという意味には広げない。

## create/update: taintはafter_create限定

固定版の [invokeActions](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_abstract_instance.go#L3379-L3427) は、action呼出しエラーに対し以下を行う。

- `halt`: errorを返し、それ以降のこの呼出し列を進めない
- `taint`: errorを返すと同時に、eventが `AfterCreate` の場合だけtaint要求を返す
- `continue`: 呼出しdiagnosticsをWarningへ変換し、ループを進める

[taintInstanceState](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_apply_instance.go#L469-L489)にも、apply対象が `plans.Create` である場合だけstateをtaintedにする条件がある。置換は処理中にcreate/deleteへ分解されるため、「新規resource addressでしか使えない」という意味ではなく、置換中の新しい実体の作成にも関わる。

したがって、`after_update` に `on_failure = taint` を付けても、この分岐が更新済みresourceをtaintして次回作り直すわけではない。`before_create` の失敗はそもそもproviderの作成呼出しへ進む前に停止する。`taint` の名称だけで全eventを置換対象にすると解釈しない。

### continueが吸収しない失敗

同じ `invokeActions` は、action呼出し前にtriggerのconditionを評価する。**condition評価エラーはhard errorとして戻る**ため、`on_failure = continue` でも警告化されない。`condition = false` による正常なskipと、式評価の失敗は別である。

`planActionTriggers` / `planActionTrigger` の通常plan経路でも、conditionやproviderの `PlanAction` が返したエラーを、`continue` だからという理由では警告化しない。`continue` は構文検証やplan失敗を無視する万能指定ではない。orphaned instanceの特殊経路は後述する。

## destroy: haltを削除前の安全装置にしない

### 通常applyとfull destroyの分岐

固定版 [context_apply.go](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/context_apply.go#L375-L390) は、planの `UIMode == plans.DestroyMode` を `walkDestroy` に対応させる。

[invokeDestroyActions](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_abstract_instance.go#L3347-L3377)では、**`walkDestroy` または明示的continue**のどちらかでaction呼出しのdiagnosticsを警告に変える。

| 経路 | destroy actionの呼出しが失敗した場合 |
|---|---|
| 通常apply中の置換等、`walkDestroy` 以外、`halt` / `taint` | actionのerrorが残る。before_destroyなら、そのノードはresource削除呼出しの前に戻る |
| 通常apply中、`continue` | actionのerrorをWarningへ変換し、そのaction失敗では止めない |
| full destroy mode、`halt` / `taint` / `continue` | actionのerrorをWarningへ変換。halt指定も削除前バックアップ成功の強制条件にならない |

この表は**actionの呼出しエラー**に限定する。provider自身の削除エラー、state保存エラー、別の検証失敗まで無視してdestroyを成功扱いにする、という意味ではない。[node_resource_destroy.go](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_destroy.go#L112-L205)はresource削除とstate更新のdiagnosticsも扱う。full destroyが必ず成功するという保証はない。

`after_destroy` の発火だけを外部資産の削除完了証跡にも使わない。同ファイルのcurrent instance経路は、削除applyが返したdiagnosticsを保持してstateを確定させた後、状態書込が成功すればafter_destroyの呼出しへ進む。削除失敗が既にある場合でも、そのevent名自体が削除成功を証明するわけではない。一方、[deposed instance経路](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_destroy_deposed.go#L316-L331)はstate書込後に蓄積errorを検査して戻るため、この細部を全削除経路共通とはしない。

### destroy actionの入力には別の制限がある

[node_resource_validate.go](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_validate.go#L104-L120)は、destroy eventを持つtriggerでの `condition` を拒否する。`condition` にバックアップ確認式を書けばfull destroyを止められる、という構成は前提から誤る。

また [planActionTrigger](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_abstract_instance.go#L3268-L3304) はdestroy actionのconfigについて、plan時点で全値が既知であることを要求し、ephemeral値を拒否する。destroyの `caller` は削除前の値を使い、apply中はplan済みconfigを復号して呼ぶ。削除後に新しい値を取り直す仕組みだと期待しない。

### 構成から消した場合も必達ではない

`resource` / moduleの設定を削る操作では、actionを評価するのに必要な構成も失われ得る。[node_resource_plan_orphan.go](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_plan_orphan.go#L230-L252)はorphaned instanceのaction plan失敗を警告に変え、計画できなかったactionを呼ばないことを通知する。

これは「設定を消せば全部のdestroy actionが常にskipされる」と断定するものではない。**実体削除の進行と後処理actionの必達は別**であり、必要なaction/provider構成が残るかを個別に確認する必要がある。

## 失敗後にapplyを繰り返すだけでは復旧にならない

[通常resourceのapply処理](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/node_resource_apply_instance.go#L255-L343)は、`plans.NoOp` ならaction呼出しに入る前に戻る。resourceのstate書込はafter-actionより前に行われる。

この制御順からの**限定的な推論**: resource作成が成功しafter_create actionだけが `halt` で失敗したとき、resourceをtaintせず状態が保存され、次回planがそのresourceをNoOpとするなら、同じafter_createは再発火しない。`after_update` の失敗後も、更新済みresourceが次回NoOpになればaction失敗だけを理由に再試行する仕組みにはならない。driftや別の変更があれば別eventが発生し得るため、常に再実行されないという意味ではない。

- `halt` はtransaction rollbackではない。失敗するまでに外部側で行われた処理の取消しを、このコード経路は実施しない
- `taint` はaction単体のretryではなく、該当する新規作成実体を次回置換へ導く指定。初期化失敗のたびにresource再作成が必要かを判断する
- `continue` でapplyが先へ進んでも、外部処理が完了したとは限らない。actionの業務結果を別に観測する
- [公式CLI手順](https://developer.hashicorp.com/terraform/language/invoke-actions)には `-invoke` による個別呼出しがある。ただしproviderの冪等性、対象識別、callerが必要な場合の呼出し元選択、過去の部分成功を確認してから再実行を設計する。StacksはローカルCLIから直接invokeできない

## 上流テストが示す範囲

読んだのは固定SHAのテストコードであり、**上流testを実行して合格させたわけではない**。

- [TestContextApply_actions の on_failure modes](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/context_apply_action_test.go#L976-L1102): 3つのresourceのafter_create actionを失敗させ、taint側はObjectTainted、haltとcontinue側はObjectReady、taint/haltに依存するresourceは未作成、continue側の依存resourceは作成済み、diagnosticsは2 errorsと1 warningとするassertionがある
- 同[applyテストの after destroy fixture](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/context_apply_action_test.go#L1304-L1338)では、旧callerを参照するafter_destroy構成で呼出しを確認する（値そのものの個別assertionではない）。旧値を選ぶ根拠は前述のplanActionTriggerの静的読解に置く。上記の3 mode比較だけでfull destroy失敗時の全組合せや全providerの挙動を実証したとはしない
- [plan/validationテスト](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/internal/terraform/context_plan_actions_test.go)には `no ephemeral in destroy`、`destroy action must be known`、`destroy cannot use condition`、`complex orphaned module instances` がある。最後のcaseはactionを計画できない理由を警告にし、planにerrorがないことを検査する

## 運用判断と受入試験（独自の設計案）

以下は上記sourceから導いた提案であり、Terraformが提供する追加保証ではない。

1. **必須バックアップを削除actionの成功だけに依存させない。** 削除権限を行使する前にバックアップID、対象世代、完了、復元可能性を別工程で確認し、承認済みplanと結び付ける。full destroyを試験せず通常置換の成功だけで導入しない
2. **失敗対応をresource状態と外部操作状態に分ける。** 「resourceは作成済み／更新済み、actionは失敗または結果不明」という状態を運用で表せるようにする。applyの終了コードだけで外部ジョブ成功を数えない
3. **taintは再作成費用を承認できる初期化に限定する。** 次回planの置換対象・新旧の共存・削除副作用を確認する。更新後の通知失敗を補うために無条件でresourceを置換する設計にしない
4. **個別retryは外部operation IDで管理する。** 重複実行の抑止や完了照会がprovider/API側にあるかを確認し、不明なら「未実行」へ戻さず「結果不明」で保留する
5. **continueはbest-effort用途に明示的に選ぶ。** 業務上必須の初期化を失敗したまま下流resourceへ進めてよいか、警告収集先と再処理担当を決める

最低限の受入試験は、after_create失敗での3 mode、after_update+taint、continue+condition評価失敗、通常置換とfull destroyでのbefore_destroy失敗、削除provider失敗とafter_destroy観測、unknown/ephemeral入力、module instance除去、失敗後のNoOp再applyを別々にする。並行する他resourceの状態も保存し、巻戻し済みと推定しない。これらは実施すべき試験項目であって、本調査の実測結果ではない。

## 限界・出典・再確認

- native webで公式releaseとv1.16.xの文書を開き、固定commitのsourceはread-only GitHub connectorで読んだ。固定sourceページのnative web取得はcache missだったため、web取得成功とは記録しない
- [固定版LICENSE](https://github.com/hashicorp/terraform/blob/ef47237fd0e03d6e93bdc510b526287f06e94c03/LICENSE)とsource SPDXでBUSL-1.1を確認。公式Web文書の再配布ライセンスはunknownとし、コード・本文のコピーやmodule昇格は行わない
- Terraform実機、上流unit test、任意providerの外部副作用、再実行冪等性、HCP/Enterpriseの表示・制御、クラッシュ直後の全state回復経路は未検証
- source catalogは既存記録を変更せず2026-10-04取得の新規IDで保存。release_notes TTL 30日を含むため再確認期限は2026-11-03。日付だけを延ばさず、この失敗分岐と該当testを再読する

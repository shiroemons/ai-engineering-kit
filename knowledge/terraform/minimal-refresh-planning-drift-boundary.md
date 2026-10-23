---
{
  "id": "terraform-minimal-refresh-planning-drift-boundary",
  "title": "Terraform 1.17 minimal-refresh: 初期plan・依存非伝播・drift確認の境界",
  "kind": "knowledge",
  "technology": "terraform",
  "version": "Terraform v1.17.0（2026-10-21 release）, 018a99c83c5dcc6d87bcdc4637b1e8da572a7042; Docs v1.17.xを2026-10-23 UTC確認; runtime未検証",
  "tags": [
    "research-domain:infrastructure",
    "terraform",
    "minimal-refresh",
    "preplan",
    "replan",
    "NoOp",
    "drift",
    "for_each",
    "schema upgrade",
    "identity schema",
    "provider upgrade",
    "refresh-only",
    "detailed-exitcode"
  ],
  "sources": [
    {
      "id": "terraform-minimal-refresh-release-1170-20261023",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.17.0",
      "type": "release_notes"
    },
    {
      "id": "terraform-minimal-refresh-plan-docs-20261023",
      "url": "https://developer.hashicorp.com/terraform/cli/commands/plan",
      "type": "official_docs"
    },
    {
      "id": "terraform-minimal-refresh-core-1170-20261023",
      "url": "https://github.com/hashicorp/terraform/tree/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/terraform",
      "type": "github_repository_analysis"
    },
    {
      "id": "terraform-minimal-refresh-core-tests-1170-20261023",
      "url": "https://github.com/hashicorp/terraform/blob/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/terraform/context_plan_minimal_refresh_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "terraform-minimal-refresh-cli-1170-20261023",
      "url": "https://github.com/hashicorp/terraform/tree/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/command/arguments",
      "type": "github_repository_analysis"
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

# 変更のない資源を読まないことと、driftがないことを分ける

## 問いと適用範囲

大量のresourceを持つ構成で、Terraform 1.17の `-minimal-refresh` を通常planの代わりにしてよいか。答えは、**変更レビューの待ち時間を減らす候補にはなるが、未変更資源まで観測した証明にはならない**。高速な結果と最新の観測を同じCI判定にまとめない。

[v1.17.0 release](https://github.com/hashicorp/terraform/releases/tag/v1.17.0)（2026-10-21）で導入を確認した。対象はその安定版と取得時点のDocs v1.17.x。release日と文書取得日2026-10-23 UTCは別で、live docsの公開日は不明。旧版、OpenTofu、全providerへの同一挙動を主張しない。

既存の[state locking・drift・import](state-locking-drift-import.md)は前提として参照し、本稿は新オプションのrefresh選別と導入判定だけを扱う。新しいTerraform本体やproviderを本番へ導入した記録ではない。

## 公開契約: preplanはファイル差分ではない

[plan referenceのPlanning Options](https://developer.hashicorp.com/terraform/cli/commands/plan#planning-options)で確認した契約は次のとおり。

- instanceごとに構成とprior stateから初期planを作る。変更がなければrefreshを省略し、変更があればrefresh後のstateでreplanする。
- あるinstanceをrefreshしても、それに依存するinstanceが自動的にrefreshされるわけではない。それぞれの判定は独立する。
- 未変更と判定された資源の外部変更は見落とし得る。結果は不完全・不正確になり得るため、No changesをdrift不在と読まない。
- resource schema版の変更はrefreshの例外条件。ただしprovider upgrade時は `-minimal-refresh` と `-refresh=false` を避けるよう公式が勧告する。
- `-refresh-only` および `-refresh=false` との併用は不可。`-detailed-exitcode` の0は空のdiff、1はerror、2は非空のdiffを示すだけである。

ここでいうpreplanは本稿の呼称であり、別のCLIコマンドではない。初期planとreplanは1回のplanning内部の処理で、利用者が `terraform refresh` を別途実行するという説明ではない。

## 固定実装の観察: NoOpの判定前後

実装はcommit `018a99c83c5dcc6d87bcdc4637b1e8da572a7042` に固定して読んだ。以下は静的なコード観察であり、実環境での再現結果ではない。

[managedResourceExecute](https://github.com/hashicorp/terraform/blob/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/terraform/node_resource_plan_instance.go#L342-L427)では、既存stateがあり、import中でなく、resource dataのupgradeもない場合に初期planの分岐へ入る。NoOpならその結果を報告してreturnし、そうでなければrefreshを経て[もう一度plan](https://github.com/hashicorp/terraform/blob/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/terraform/node_resource_plan_instance.go#L473-L518)する。

[readResourceInstanceState](https://github.com/hashicorp/terraform/blob/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/terraform/node_resource_abstract.go#L557-L599)のupgrade通知は、state schemaとidentity schemaのupgradeの論理和である。**providerのパッケージ版番号が変わったかを検査する条件ではない**。そのため、このガードがあることをprovider更新時の通常refreshを省く理由にしない。

初期planにも処理コストがあるので、remote read削減率から総所要時間の改善率を直接推定しない、というのが本稿の設計判断である。すべてのprovider通信を止めるoffline modeとしても扱わない。

## テストが区別している境界

固定版の[Coreテスト](https://github.com/hashicorp/terraform/blob/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/terraform/context_plan_minimal_refresh_test.go)を読み、次のassertionを確認した。テスト名の共通接頭辞は `TestContext2Plan_minimal_refresh_`。今回は実行していない。

| suffix | 読み取れた境界 |
|---|---|
| for_each | 同じresource blockでも変更したkeyだけrefreshし、NoOpのkeyは省く |
| provider_update_will_refresh | 構成引数が同じでもproviderが計画差分を生成するとrefreshする。provider版更新のテストではない |
| schema_upgrade_will_refresh / identity_upgrade_will_refresh | upgrade後の最終actionがNoOpでもreadを要求する |
| ignore_changes_noop | 設定値が異なってもignore_changesでNoOpならreadを省く |
| force_replace / replace_triggered_by | 置換が計画されるinstanceはrefresh対象になる |
| destroy / orphans / deposed | これらの削除経路ではrefreshを省略するケースをassertする |

「依存非伝播」は依存関係を無視するという意味ではない。特にreplace_triggered_byでB自身に置換が生じればBもrefreshされる。Aをrefreshしたという事実だけでBをrefreshすることとは区別する。

## 禁止組合せと、許可されても用途を分ける組合せ

[CLI引数検査](https://github.com/hashicorp/terraform/blob/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/command/arguments/extended.go#L240-L253)はnormal/destroy以外のmodeを拒否し、Refresh=falseとの併用も拒否する。[parse test](https://github.com/hashicorp/terraform/blob/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/internal/command/arguments/plan_test.go#L99-L139)ではrefresh-only併用の診断が `Incompatible plan mode options`、refresh=false併用が `Incompatible refresh options` になることをassertしている。

したがって `-destroy -minimal-refresh` を「禁止」とは書けない。一方、前節の削除経路は通常の変更資源に対するpreplan説明のまま一般化できない。本稿の導入提案ではdestroy、orphan整理、deposed資源の復旧を最初の試行対象から外す。これは運用上の慎重な選択であり、Terraformの禁止事項ではない。

## 導入判定: 速い確認と最終確認を別レーンにする

以下は上記の根拠から組み立てた独自の運用案で、HashiCorpの必須手順ではない。

1. **対象を限定する。** まずproviderとCoreの版が固定され、帯域外変更の扱いが決まっている非本番の通常変更を選ぶ。新オプションを全workspace共通の環境変数やwrapperに一括追加しない。どのジョブが有効にしたか記録する。
2. **速いplanを「変更候補の早期レビュー」と表示する。** No changesやexit 0を受けても、drift確認済みというチェック名を付けない。errorを変更なしへ変換しない。
3. **最終適用の根拠は通常refreshのplanで取り直す。** 初期試行では高速planの出力をそのまま承認済み成果物として流用しない。通常planとの差をレビューし、その最終planと承認対象を対応付ける。新旧両planを作っただけで同じ結果だったと判断せず、instance actionと差分を確認する。
4. **provider upgradeを別の検査条件にする。** lockfileやprovider制約の変更を見つけたら高速レーンを止める。state/identity schemaの番号が変わらない更新も同じ扱いにする。upgradeガードを網羅的な互換性証明としない。
5. **drift確認の経路を残す。** 最後に通常refreshを伴う確認が成功した日時と対象を別に追跡する。高速planの成功でその日時を上書きしない。障害対応や手動変更があった場合は通常確認へ戻す。
6. **撤退条件を先に置く。** providerエラー、想定外のaction差分、削減効果の不足があればオプションを外す。異常を無視して採用を続けず、通常planで再評価する。lockや権限の緩和を高速化の代替策にしない。

## 受入れ試験案と観測項目（未実行）

本稿が提案する検証シナリオであり、成功済みの証拠ではない。破壊可能な隔離環境と許可された観測手段で実施する。

- **未変更資源のdrift:** 構成とstateをそろえた後、管理対象の非本番属性だけを帯域外変更する。高速planのNoOpと通常planの観測差を比較し、NoOpでもread省略があり得ることを確認する。帯域外変更は本番で試さない。
- **per-instance:** for_eachの2つのkeyのうち一方だけ変更する。resource block単位のread数だけでなく、instance address別に読み取りを追う。
- **依存先:** Aの変更とBのNoOpを分離できる構成、およびBにreplace_triggered_byがある構成を別々に試す。「Aを読んだ」と「B自身にactionがある」を混ぜない。
- **upgradeと禁止引数:** 選んだproviderの実際のschema/identity移行を確認する。移行が発生しない試験でガードを検証したとしない。refresh-only/refresh=falseとの併用はerrorとして扱い、通常planへ黙って置換しない。
- **性能と判断品質:** plan所要時間、managed resourceのread回数、providerエラー、通常planとの差、最後の通常確認からの経過を記録する。API readが減っただけで正確性や総時間の改善を宣言しない。

同じ前提で比較するため、Core/provider版、構成、入力、開始state、帯域外変更の時刻をそろえる。結果が違ったときに、minimal-refreshの影響と試行間の環境変化を切り分けられる情報を残す。

## 検証範囲・出典の扱い

- native webでreleaseとv1.17.xの公式plan文書を開き、固定commitの実装・テストはread-only GitHub connectorで照合した。固定GitHub/rawページのweb取得はcache missとなったため、取得成功したと装っていない。
- 同一commitの[LICENSE](https://github.com/hashicorp/terraform/blob/018a99c83c5dcc6d87bcdc4637b1e8da572a7042/LICENSE)と対象GoファイルのSPDXでBUSL-1.1を確認した。コードの転載・改変・module化はしていない。公式Web文書の配布ライセンスは未確定のため、契約を独自要約した。
- Terraformのbuild、上流テスト、実providerでのplan/apply、性能測定は未実行。HCP Terraform / Terraform Enterpriseのremote executionでの受入れ状況、data sourceやephemeral resourceの全経路、全providerの内部API通信数も未確認。本稿の採用案はそれらの保証ではない。

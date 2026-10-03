---
{
  "id": "terraform-forget-replacement-deposed-boundary",
  "title": "Terraform 1.16.3 destroy=false: forget置換・deposed状態・管理解除の境界",
  "kind": "knowledge",
  "technology": "terraform",
  "version": "Terraform v1.16.0 resource-level destroy=false導入 (2026-08-26); v1.16.3修正 (2026-09-16), 42121cd5ca5e1ab1d37e286540ce11acf70e290a; Docs v1.16.xを2026-10-03 UTC確認; runtime未検証",
  "tags": [
    "research-domain:infrastructure",
    "terraform",
    "destroy=false",
    "create_before_destroy",
    "ForgetThenCreate",
    "CreateThenForget",
    "deposed",
    "replace_triggered_by",
    "prevent_destroy",
    "removed",
    "plan-json",
    "ownership"
  ],
  "sources": [
    {
      "id": "terraform-forget-resource-release-1160-20261003",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.16.0",
      "type": "release_notes"
    },
    {
      "id": "terraform-forget-cbd-release-1163-20261003",
      "url": "https://github.com/hashicorp/terraform/releases/tag/v1.16.3",
      "type": "release_notes"
    },
    {
      "id": "terraform-forget-lifecycle-docs-20261003",
      "url": "https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle",
      "type": "official_docs"
    },
    {
      "id": "terraform-forget-removed-docs-20261003",
      "url": "https://developer.hashicorp.com/terraform/language/block/removed",
      "type": "official_docs"
    },
    {
      "id": "terraform-forget-state-remove-docs-20261003",
      "url": "https://developer.hashicorp.com/terraform/language/state/remove",
      "type": "official_docs"
    },
    {
      "id": "terraform-forget-cbd-implementation-1163-20261003",
      "url": "https://github.com/hashicorp/terraform/tree/42121cd5ca5e1ab1d37e286540ce11acf70e290a",
      "type": "github_repository_analysis"
    },
    {
      "id": "terraform-forget-cbd-tests-1163-20261003",
      "url": "https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/terraform/context_apply2_test.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Terraform の「壊さず置き換える」を、管理解除としてレビューする

## 問い・適用範囲

resourceの `lifecycle { destroy = false }` を使えば、置換を止められるか。`create_before_destroy` と併用したapply失敗を、providerの不具合と即断してよいか。planのdestroy件数がゼロなら、既存資源の運用に変化はないか。

結論は、**destroy=falseは旧オブジェクトを残したままTerraformの管理から外す指定であり、置換禁止ではない**。新しいオブジェクトの作成、旧オブジェクトのforget、現在stateとdeposed状態の整合を分けて判断する。1.16.3にはこの組合せのCore修正があるため、設定の可否だけでなくpatch版も確認する。

既存の[置換順序と依存伝播](replacement-ordering-create-before-destroy.md)を前提に、本稿は未収録だったforget置換の実装・回帰テスト・plan検査に絞る。state locking、drift、storeへのephemeral値保存は繰り返さない。実環境の資源削除やstate編集を行う手順書ではなく、変更レビューの判断材料である。

## 導入版と修正版を分ける

- [v1.16.0 release](https://github.com/hashicorp/terraform/releases/tag/v1.16.0)（2026-08-26）は、**resource block内のlifecycle**でdestroy=falseを扱えるようになったことを記載する。removed blockのdestroy=falseがこの版で初めて追加された、という意味ではない
- [v1.16.3 release](https://github.com/hashicorp/terraform/releases/tag/v1.16.3)（2026-09-16）は、destroy=falseとcreate_before_destroyの組合せの修正を#39169として収録する。機能が存在する1.16.0と、この修正を含む1.16.3を同じ対応状態として扱わない
- 以下の実装とテストはv1.16.3のcommit `42121cd5ca5e1ab1d37e286540ce11acf70e290a` に固定した。これは検証対象snapshotであり、1.16.3を取得時点の最新版として推奨する記述ではない
- 公式Docsは取得時点でv1.16.x表示。公開日は明示されず、取得日2026-10-03 UTCとrelease日を混同しない。全旧patch版の不具合再現、1.17 beta、OpenTofuへの一般化はしていない

## 公開契約: 保護・管理解除・構成削除は別操作

[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)と[removed reference](https://developer.hashicorp.com/terraform/language/block/removed)の契約を整理する。

| 意図・指定 | 判断すること |
|---|---|
| prevent_destroy=true | 破壊を含むplanを拒否する。構成に残っていることが前提 |
| destroy=false | 実体を破壊せずstateから外す。旧実体の管理継続を保証する指定ではない |
| create_before_destroy=true | 置換側を先に作る順序。依存先にも伝播し得る |
| removed blockのdestroy=false | 対象の管理を終了し、実体を残す |
| removed blockでdestroyを既定のまま扱う | 既定はtrue。stateから外すだけという意図を満たさない |

同じlifecycle referenceは、create_before_destroy以外のresource lifecycleルールをstateに明示保存しないと説明する。**resource blockごと消しても、以前のdestroy=falseがstateに残って資源を保護する、と期待しない**。管理だけを終える変更は[公式の管理解除手順](https://developer.hashicorp.com/terraform/language/state/remove)に沿ってremovedへ置換し、そこでdestroy=falseを明示してplanを確認する。

管理解除後にTerraformへ戻すにはimportが必要になる。同手順のfromには、複数instanceのうち一つを示す `[1]` のようなinstance keyを含められないと記載されている。単一instanceだけの移管要件を、resource全体のremoved blockへ機械的に変換しない。

## 固定版の実装観察: deleteをforgetへ変えても置換は残る

ここからは公開APIの将来保証ではなく、上記commitを読んだ観察である。

### 1. 二つのforget置換とdeposed key

[置換planの生成](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/terraform/node_resource_abstract_instance.go)は、通常のDeleteThenCreateをForgetThenCreateへ、CreateThenDeleteをCreateThenForgetへ変換する。taintedな既存オブジェクトの置換も、forget指定と作成順序を組み合わせて選ぶ。[Action.IsReplace](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/plans/action.go)は、どちらも置換として判定する。

| plan action | state上の順序 | 旧実体 |
|---|---|---|
| ForgetThenCreate | 旧オブジェクトをforgetしてから新しいものを作る | 削除を要求せず、管理対象から外す |
| CreateThenForget | 新しいものを作ってから旧オブジェクトをforgetする | 削除を要求せず、管理対象から外す |

ここでのforgetは「新しいものが旧実体と同一になる」操作ではない。外部サービス側の一意名、quota、課金、残存データは別途確認する必要がある。特にForgetThenCreateでも旧実体は消えないので、順序だけ変えて同名衝突を解消できるとは限らない。これは契約からの運用上の推論である。

[DiffTransformer](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/terraform/transform_diff.go)は、createが先のdelete/forget置換でdeposed keyを事前割当し、旧オブジェクトを示すkeyと後段の除去対象を一致させる。[NodeForgetDeposedResourceInstanceObject](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/terraform/node_resource_destroy_deposed.go)はcreate-before-destroyとして自身を報告し、対象keyのdeposed stateをforgetする。deposedは置換途中の旧状態を区別するための内部識別であって、旧実体がすでに破壊されたという印ではない。

### 2. prevent_destroyを管理継続のguardにしない

固定版の[checkPreventDestroy](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/terraform/node_resource_abstract_instance.go)はDelete、DeleteThenCreate、CreateThenDeleteを検査対象にし、ForgetThenCreateとCreateThenForgetをその条件に含めない。

従ってdestroy=falseとprevent_destroyを併用しても、prevent_destroyがforget置換を拒否して「同じ実体を引き続き管理する」ことを守ってくれる、と設計しない。管理解除も承認対象にするなら、設定名に頼らずplan actionで別に検査する。これはこの実装観察を使った独自のレビュー方針である。

### 3. 依存先のforget置換も連鎖の入口になる

[EvaluateReplaceTriggeredBy](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/terraform/eval_context_builtin.go)では、resourceまたはinstance全体への参照について、Updateとdelete系置換だけでなくForgetThenCreate/CreateThenForgetも置換契機となる。属性参照はこのaction判定を通ったうえで、対象属性のbefore/after値を比較する。

単なるForgetだけはこの対象一覧にない。**「forgetを含む置換」と「管理解除だけ」を一括して同じtriggerにしない**。独自のアップグレード確認では、修正後に下流のreplace_triggered_byが意図どおり発火するか、影響資源が増えていないかを再planで調べる。下流資源にも自動的にdestroy=falseが伝播する、という意味ではない。

## plan JSON・人間向け集計の確認

固定版の[JSON変換](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/command/jsonplan/plan.go)は、Forgetを `["forget"]`、ForgetThenCreateを `["forget","create"]`、CreateThenForgetを `["create","forget"]` に変換する。配列の順序も意味を持つ。[人間向け集計](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/command/jsonformat/plan.go)では、forget置換はaddへ加算され、destroy件数へは加算されない。

このため **0 to destroyは、管理対象や参照先が変わらない証拠ではない**。独自のplan検査では次を別々に数える。

1. deleteを含むaction: 実体削除のレビュー対象
2. forgetを含むaction: 管理解除と旧実体の責任者・保存先のレビュー対象
3. createを含むaction: 新実体の作成、名前衝突、quota、費用のレビュー対象
4. 置換actionの順序: 作成失敗時に旧実体がどのstate状態にあるかを、対象版・providerで検証する対象

deleteだけを危険操作として抽出する既存CIは、forgetを無変更扱いしないよう見直す。未知のactionを無視して承認するより、未対応として検査を止める方針を選ぶ。これはTerraform内蔵のpolicyではなく、利用側の設計案である。

## 回帰テストから分かること・分からないこと

固定snapshotの[context_apply2_test.go](https://github.com/hashicorp/terraform/blob/42121cd5ca5e1ab1d37e286540ce11acf70e290a/internal/terraform/context_apply2_test.go)を読んで確認したassertion:

- TestContext2Apply_forget_createBeforeDestroyはexplicitとpropagatedの両条件を扱う。taintedな旧状態からCreateThenForgetを計画し、providerにdestroyを要求しないこと、create呼出しが1回であること、apply後にcurrent objectがありdeposed stateが残らないことを確認する
- TestContext2Apply_forget_createBeforeDestroyInvalidResultは、初回作成後に属性変更でproviderがRequiresReplaceを返す経路を扱う。二回のapplyでcreate合計2回、destroy要求なし、診断エラーなしを確認する
- TestContext2Apply_forget_replaceはCBDを明示しないforget置換でForgetThenCreateを期待し、forgetの警告、destroy要求なし、最終stateにdeposedがないことを確認する

これは**テストを読んだ結果であり、本調査でupstream testやTerraform実行を行った結果ではない**。mock providerの成功経路から、全cloud providerの原子性、quota衝突時の復旧、強制終了後のstate整合、旧資源のアクセス権や課金停止を保証しない。deposed stateが空になることも、旧実体が消えた証拠にはならない。

## 変更前後の確認手順（独自の設計案）

- CLI patch版・provider版・対象workspaceを記録する。resource-level destroy=falseを受理することと#39169修正を含むことを別項目で確認する
- directなCBD指定だけでなく、依存元からの伝播を含めたplanを読む。旧実体・新実体のIDと、管理を離れた旧実体の引継ぎ責任者を残す
- 修正済みの採用版でplanを作り直し、forget系action、replace_triggered_byの下流変更、残存費用を再レビューする。古いplan表示や一行のdestroy集計だけで承認を引き継がない
- 試験ではtainted置換、属性変更によるRequiresReplace、explicit/propagated CBD、CBDなしのforget置換、作成失敗を分ける。成功時だけでなく途中失敗後のcurrent/deposed状態も観察する
- 失敗を消すためにdestroy=falseを外す、stateを手編集する、旧実体を削除する、といった操作を自動的なworkaroundにしない。修正版で残った計画と実体を照合してから、別途承認された復旧を行う

## 出典・ライセンス・未確認事項

一次資料はrelease 2ページ、公式Docs 3ページをnative webで開いた。実装・テスト・LICENSEは上記40桁SHAに固定してread-only GitHub connectorで取得した。native webの固定commit/一部file表示はcache miss、state/removeのEdit this pageは取得失敗だったが、本文と固定ソースは別経路で確認できた。

Terraform固定版のLICENSEとaction.goのSPDXはBUSL-1.1。Docsはリンク先web-unified-docsの[LICENSE](https://github.com/hashicorp/web-unified-docs/blob/main/LICENSE)でBUSL-1.1を確認した。本稿は独自要約で、コードやテストを転載・module化していない。

未確認は実機再現、全旧版の影響範囲、HCP/Terraform Enterpriseでの差、失敗途中の復旧保証、個別providerの副作用。検索evalは文書を見つけられることの検証であり、インフラの動作検証ではない。release_notesのTTLを含む再確認期限は2026-11-02 UTC。

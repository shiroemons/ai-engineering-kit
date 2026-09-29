---
{
  "id": "terraform-replacement-ordering-create-before-destroy",
  "title": "Terraform のリソース置換順序: 既定の destroy-then-create と create_before_destroy の依存伝播",
  "kind": "knowledge",
  "technology": "terraform",
  "version": "Terraform Docs v1.16.x（lifecycle reference / Configure a resource、2026-09-29 確認）",
  "tags": [
    "research-domain:infrastructure",
    "terraform",
    "create_before_destroy",
    "resource replacement",
    "replacement ordering",
    "dependency propagation",
    "depends_on",
    "replace_triggered_by",
    "prevent_destroy",
    "provisioner",
    "in parallel",
    "timeouts"
  ],
  "sources": [
    {
      "id": "terraform-lifecycle-meta-arguments-docs",
      "url": "https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle",
      "type": "official_docs"
    },
    {
      "id": "terraform-configure-resource-docs",
      "url": "https://developer.hashicorp.com/terraform/language/resources/configure",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-29",
  "expires_at": "2026-12-28",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/infrastructure.json"]
}
---

# Terraform のリソース置換順序: 既定の destroy-then-create と create_before_destroy の依存伝播

1つのリソースを差し替えるとき「先に壊してから作る」のか「先に作ってから壊す」のか、その選択が依存リソースへどう波及するかを、HashiCorp 公式2ページ（[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle) と [Configure a resource](https://developer.hashicorp.com/terraform/language/resources/configure)、いずれも v1.16.x を 2026-09-29 に原文確認）の**記載事実**としてまとめる。公式に書かれていない挙動は「未確認」として残し、設計判断は「推奨方法」に分けて記す。

## 要点（公式文書に確認した事実）

### apply が行う操作と置換の既定順序

- `terraform apply` は次の操作を行う: (1) state に実体がない構成上のリソースの作成、(2) state にあるが構成から消えたリソースの破壊、(3) 引数が変わったリソースの in-place 更新、(4) **remote API の制約により in-place 更新できなくなった引数変更に対する destroy と再作成**、(5) apply 中に実行するよう設定した action の呼び出し。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
- **既定では「Terraform は既存オブジェクトを破壊してから、新しい設定引数を持つ置換オブジェクトを作成する」**（destroy-then-create）。`create_before_destroy` を使うと、**既存リソースを破壊する前に置換リソースを作成する**よう指示できる。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
- `create_before_destroy` は**選択的（opt-in）の挙動**である。多くの remote object type は名前の一意性などの制約を持ち、新旧オブジェクトの同時存在に適応させる必要がある。一部のリソース型はオブジェクト名にランダムサフィックスを付ける特別なオプションを提供するが、**Terraform CLI はそうした機能を自動で有効化できない**ため、そのリソース型で `create_before_destroy` を使う前に各リソース型の制約を理解する必要がある。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)

### 依存リソースへの伝播（置換順序が広がる理由）

- **Terraform は `create_before_destroy` をすべてのリソース依存へ伝播・適用する。** 例として、リソース A に `create_before_destroy` が有効で A がリソース B に依存している場合、Terraform は**リソース B にも暗黙的に `create_before_destroy` を有効化し、state ファイルに保存する**。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
- その結果、**リソース B で `create_before_destroy` を `false` に上書きすることはできない**。それは依存グラフにおける依存の循環（dependency cycles）を意味するため。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)

### destroy 時 provisioner と state 記録

- リソースが **`destroy` 操作中に実行される provisioner を含む場合、`create_before_destroy` を `true` にするとその provisioner は実行されない。**[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
- **`create_before_destroy` を除き、Terraform はリソースの `lifecycle` ルールを state に明示的に記録しない。** したがって構成からリソースを削除すると、`prevent_destroy` が有効でも apply 中に実インフラが破壊される。**`create_before_destroy` は state に明示記録される唯一の lifecycle ルール**である。`precondition` / `postcondition` はチェックの結果を state に記録するが、チェックの内容は記録しない。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)

### lifecycle は依存グラフの構築に影響する

- **すべての lifecycle 設定は、Terraform が依存グラフを構築・走査する方法に影響する。** 処理は任意の式評価より前に行われるため、**リテラル値しか使用できない**。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)

### 依存順序と並行実行（Configure a resource）

- **Terraform は通常、リソースを依存順（dependency order）で更新し、可能な場合は並行（in parallel）に更新する。**[Configure a resource](https://developer.hashicorp.com/terraform/language/resources/configure)
- 暗黙依存は `resource` ブロック内の**式が他オブジェクトを参照していることから導かれる**ため、通常は手動で依存を指定する必要はない。Terraform が暗黙に判断できない場合（アクセスポリシーのように「ポリシーが先に必要」といった隠れた依存）、順序を制御するには **`depends_on` メタ引数**で上流リソースを明示する。[Configure a resource](https://developer.hashicorp.com/terraform/language/resources/configure)
- **`replace_triggered_by`** は独立したリソース間に依存を追加し、**参照したリソースまたは属性の変更時に親リソースを置換させる**。参照先が複数インスタンスのリソースなら、いずれかのインスタンスの更新・置換計画で置換が発火し、単一インスタンス参照ならその更新・置換計画で、単一属性参照なら属性値の変更で発火する。式で参照できるのは managed resources のみで、local value や input variable は独自の planned action を持たないため直接使えない（`terraform_data` を経由する方法が文書に示される）。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle) / [Configure a resource](https://developer.hashicorp.com/terraform/language/resources/configure)
- **`timeouts` は Terraform 全体ではなくリソース型ごとの設定。** 一部のリソース型は `timeouts` 引数を支持し、Terraform が特定の操作を待つ時間を指定してタイムアウトエラーで終了する。**各リソース型が対象操作の集合を決め、扱いも provider ごとに異なる。** 規約は `timeouts` 子ブロックに操作ごとのネスト引数を置き、値は `"60m"` / `"10s"` / `"2h"` のような文字列期間。文書の例は `aws_db_instance` の `create = "60m"` と `delete = "2h"`。[Configure a resource](https://developer.hashicorp.com/terraform/language/resources/configure)

### 置換を守る・抑える関連の lifecycle ルール

- **`prevent_destroy = true`** は、リソースに関連するインフラオブジェクトを破壊する計画を Terraform が拒否しエラーを返す。公式は「再作成に高コストなオブジェクト（DB インスタンス、ストレージなど）の**誤置換を防ぐ**保護」と位置付ける。一方で、有効化するといくつかの構成変更が適用できなくなり、**オブジェクト作成後は `terraform destroy` も動かなくなる**ため控えめに使うよう指示される。構成からリソースを削除した場合は防えない。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
- **`ignore_changes`** は作成計画では該当引数を考慮し、**更新計画では無視する**。`all` キーワードで全属性を無視でき、その場合 Terraform はリモートオブジェクトの作成と破壊は行うが、更新を提案することはない。`ignore_changes` 自身や他のメタ引数には適用できない。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
- **`precondition`** はリソース作成前に評価され、リソース構成引数の評価**より前**に評価される（引数評価エラーより先に効くことがある）。`count` / `for_each` の評価後に評価されるため、条件に `count.index` / `each.key` が使える。**`postcondition`** は作成後の評価で、失敗すると**このリソースに依存する下流リソースの変更を妨げる**。両者は `resource` / `data` / `ephemeral` ブロックで使える。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)
- **`destroy = false`** を設定すると、実インフラを破壊せずにリソースを state から削除できる。[lifecycle reference](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- **置換は「既定で destroy → create」と前提して plan を読む。** 既定順序では旧オブジェクトが消えてから新オブジェクトができ、その間に依存リソースが参照先を失う時間帯があり得る。plan 確認では当該リソースが replace として表示され、先に destroy される順序になっていないかを確認する。
- **`create_before_destroy` は「そのリソースだけでなく、依存先も新旧同時存在できる」ことを確認してから付ける。** 伝播により、A に付けると依存する B も state 保存込みで有効化される。B の名前一意性・同時作成制約が未確認のまま A だけ有効化すると、想定していなかったリソースでも新旧同時作成の挙動になる。名前衝突を避ける手段（ランダムサフィックス等）がそのリソース型に存在するかを provider 側のドキュメントで確認する。
- **destroy 時 provisioner に後始末を頼っているリソースでは `create_before_destroy` を有効化しない。** 有効化すると destroy 時 provisioner は実行されない。後始末が必須なら、置換順序の選択を諦めるか、後始末を provisioner 以外の経路（別リソースや外部ジョブ）に移す。
- **独立したリソースだが同時に差し替えるべき対象には `replace_triggered_by` を使う。** 両者が同じ変更で置換されることが要件なら、値の参照による暗黙依存が生まれない組でも `replace_triggered_by` で依存を明示的に足す。逆に、独立に運用してよいリソースへ乱用すると不要な置換が連鎖する。
- **隠れた依存は `depends_on` で明示し、並行が止まっていないか plan の順序で確認する。** 依存順・並行は既定の挙動であり、意図した順序で動く保証は plan 表示での確認による。
- **置換に時間がかかるリソース型には、その型が `timeouts` を支持する場合のみ create / delete を設定する。** 対象操作と単位は provider 文書で確認し、Terraform 全体の設定と混同しない。

## 避ける使い方

- **「置換は無停止」と考える。** 既定は destroy-then-create で、無停止化は opt-in の `create_before_destroy` による。
- **リソース型の制約を確認せず `create_before_destroy` を付ける。** 公式は新旧同時存在に必要な各リソース型の制約理解を前提条件として明記している。制約不明のまま有効化すると、同時作成が許されない名前一意性の制約に抵触する危険がある（失敗時の具体的なエラー形状は未確認）。
- **依存側 B に `create_before_destroy = false` と書く。** 依存の循環を意味するため上書き不可。
- **`create_before_destroy = true` のまま destroy 時 provisioner の動作を期待する。** 実行されない。
- **`prevent_destroy` を付けていれば設定から消しても安全と読む。** lifecycle ルールは `create_before_destroy` 以外 state に明示記録されず、構成からの削除は実インフラの破壊を招く。
- **`prevent_destroy` を付けたまま日常的に `terraform destroy` や置換を前提とする変更を行う。** 作成後は destroy が動かず、いくつかの変更が適用不能になる。
- **`lifecycle` に変数参照などの式を書く。** lifecycle は依存グラフ構築に影響し、リテラル値のみ許可される。
- **`timeouts` を Terraform 全体の設定と誤解する。** 対象操作と挙動は resource type / provider 毎に異なる。
- **`replace_triggered_by` に local value や input variable を直接書く。** planned action を持たず、文書はリソースアドレスのみを認める。

## 適用版と本番での注意

- 適用版: Terraform Docs **v1.16.x**（lifecycle reference と Configure a resource の2ページを 2026-09-29 に developer.hashicorp.com で全文確認。各ページの "Edit this page on GitHub" は `content/terraform/v1.16.x/docs/language/meta-arguments/lifecycle.mdx` と `content/terraform/v1.16.x/docs/language/resources/configure.mdx` を指す）。版選択には v1.17.x が beta、v1.16.x が latest と表示されており、v1.16.x は現行 stable だが将来の最新と同一視しない。
- 再確認期限: 2件とも `official_docs`（TTL 90日）で取得日の 2026-09-29 から **2026-12-28**。terraform は `config/freshness.json` の technologies に含まれないため技術 TTL は適用されず、文書の明示期限も同日を基準とする。
- 未確認事項（推測で埋めない）: リソースごとに「in-place 更新可能か」を Terraform / provider が判定する根拠（remote API 制約の内訳）、置換が plan / apply 出力上でどう表示されるか、module・stack の境界を越えた `create_before_destroy` 伝播の実挙動、HCP Terraform や Terraform Enterprise 上での差、`action_trigger` など本題外の lifecycle ルールの詳細、`destroy = false` と state 削除手順の併用時の挙動、`timeouts` の既定値。これらは該当ページの該当節か provider 側ドキュメントを別途確認する。
- 本ドキュメントの推奨方法は設計案であり、単一の障害事例やベンチマークの一般化ではない。名前制約の扱いや timeouts の値はワークロードと provider ごとに検証して決める。

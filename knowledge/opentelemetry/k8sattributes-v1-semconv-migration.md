---
{
  "id": "opentelemetry-k8sattributes-v1-semconv-migration",
  "title": "OpenTelemetry k8sattributes v1: stable 化に伴う属性名・型の移行と dual emission",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "k8sattributesprocessor v1.0.0 / Collector Contrib v0.161.0 (published 2026-09-15); Semantic Conventions 1.42.0; verified 2026-10-02",
  "tags": [
    "research-domain:quality-operations",
    "OpenTelemetry",
    "k8sattributes",
    "semantic conventions",
    "dual emission",
    "migration",
    "resource attributes",
    "stability"
  ],
  "sources": [
    {
      "id": "otel-k8sattributes-v1-release-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.161.0",
      "type": "release_notes"
    },
    {
      "id": "otel-k8sattributes-v1-readme-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/processor/k8sattributesprocessor/README.md",
      "type": "official_docs"
    },
    {
      "id": "otel-k8sattributes-v1-generated-reference-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/processor/k8sattributesprocessor/documentation.md",
      "type": "official_docs"
    },
    {
      "id": "otel-k8sattributes-v1-announcement-20261002",
      "url": "https://opentelemetry.io/blog/2026/k8s-attributes-processor-v1/",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# OpenTelemetry k8sattributes v1 と属性スキーマの移行

## 調査した問い

Kubernetes attributes processor が stable になったので Collector を更新してよいか。アップグレード前後で、label を使うフィルタ、ダッシュボード、アラート、コンテナイメージタグの抽出が同じ意味を保つかを判断する。対象は processor が追加する resource attributes のスキーマ移行であり、既存の metrics temporality・trace sampling・BatchSpanProcessor の文書とは分ける。

## 何がいつ変わったか

[Contrib v1.0.0/v0.161.0 release](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.161.0) は GitHub API 上で **2026-09-15T15:05:47Z** 公開。logs・metrics・traces の stable 昇格と同時に、次の gate を alpha から beta へ進め、既定で有効にした。

- `processor.k8sattributes.EmitV1K8sConventions`: stable 側の属性を出す
- `processor.k8sattributes.DontEmitV0K8sConventions`: legacy 側の属性を出さない

つまり既定動作は **新スキーマのみ**。stable という告知を「従来の出力名が無変更」という意味で読まない。[2026-09-16の告知](https://opentelemetry.io/blog/2026/k8s-attributes-processor-v1/) が v1.0.0 とするのはこの component であり、Contrib 全体の全 component が stable になったという宣言ではない。同記事は Kubernetes semantic conventions が v1.42.0 で stable となり、今回の昇格の前提になったことを説明している。

以下の reference は Contrib tag v0.161.0 が指す commit `3f8455d8038a985398861171e5310bc9b4e988b2` に固定した。後続の Collector 配布版・vendor distro・Helm chart の内容を、この番号から推定しない。

## 属性名だけでなく型を確認する

[固定版 README の Semantic Conventions Compatibility](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/processor/k8sattributesprocessor/README.md#semantic-conventions-compatibility) が示す差分は次の通り。

| 従来 | stable 側 |
|---|---|
| `container.image.tag` | `container.image.tags` |
| `k8s.pod.labels.<key>` | `k8s.pod.label.<key>` |
| `k8s.pod.annotations.<key>` | `k8s.pod.annotation.<key>` |
| `k8s.node.labels.<key>` | `k8s.node.label.<key>` |
| `k8s.node.annotations.<key>` | `k8s.node.annotation.<key>` |
| `k8s.namespace.labels.<key>` | `k8s.namespace.label.<key>` |
| `k8s.namespace.annotations.<key>` | `k8s.namespace.annotation.<key>` |

label / annotation は複数形から単数形へ移るが、image tag は逆に複数形になる。機械的に末尾の `s` を除く置換では移行できない。

[生成 reference の Resource Attributes](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/processor/k8sattributesprocessor/documentation.md#resource-attributes) は `container.image.tag` を **Str**、`container.image.tags` を **Slice** とする。後者には `container.id` または `k8s.container.name` が必要との説明がある。下流の処理でキーだけ変え、引き続き文字列比較を行う案は型境界を見落とす。配列から何を検索・表示するかは backend 側の仕様と合わせて決める。

この一覧はこの processor のスキーマ変更の根拠である。別の SDK や receiver がすでに付けた任意の属性、独自の `tag_name`、exporter が変換した backend 内の名称まで、同じフラグで一括移行できるとは確認していない。

## dual emission と戻し方

[release の推奨](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.161.0) は移行期間に dual emission を使うこと。指定は次の通り。

```text
--feature-gates=-processor.k8sattributes.DontEmitV0K8sConventions,processor.k8sattributes.EmitV1K8sConventions
```

`DontEmitV0` を無効にする符号 `-` を落とすと、「旧属性を残す」意図と逆になる。両 gate が既定で有効であることを踏まえ、起動引数の最終形を確認する。

[README](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/processor/k8sattributesprocessor/README.md#semantic-conventions-compatibility) は v1.x.x の一定期間、gate を beta に保ち、旧スキーマへ戻せるようにすると説明する。ただし終了日や削除版は明記されていない。永続的な互換設定として扱わない。release は旧名が必要なら両 gate を無効にするか stable 名へ移すよう案内している。旧スキーマのみへ戻す設定案は以下であり、実際に使う distro での起動・出力確認が必要。

```text
--feature-gates=-processor.k8sattributes.DontEmitV0K8sConventions,-processor.k8sattributes.EmitV1K8sConventions
```

後者は gate の意味から組み立てた設定案で、今回実行していない。attribute の dual emission だけを根拠に、backend の二重計上やコスト増が一定倍率になるとは主張しない。

## stable と Pod association の成功は別に確かめる

schema の切替と、telemetry が正しい Pod に紐づくことは別の確認事項である。[README の Configuration](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/processor/k8sattributesprocessor/README.md#configuration) は、既定の association が接続元 IP に依存すること、proxy 等を通る場合は別の rule が必要になることを説明する。`connection` を使う場合、この情報を除く batching / tail sampling より前に processor を置く必要がある。

また [生成 reference](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/processor/k8sattributesprocessor/documentation.md#internal-telemetry) の `otelcol.k8s.pod.association` は `status` に `success` / `error` を持つが、その metric の成熟度は **Development**。新しい出力スキーマの stable、signal の stable、内部監視 metric の成熟度を同じ保証にしない。profile signal も [README の status](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/processor/k8sattributesprocessor/README.md) では Development のままである。

## 移行手順（独自の運用提案）

1. **依存を列挙する**: 現在の binary / image、feature gates、抽出設定を保存し、旧名を読む routing、filter、transform、保存先の index、dashboard、alert を検索する。表示だけでなく、telemetry を落とす条件や転送先を選ぶ条件を先に点検する
2. **同じ入力で出力を比較する**: 小さな canary に既知の Pod label / annotation と container 情報を与え、変更前・dual emission・stable の resource 属性を比較する。collector の正常起動だけを合格条件にしない
3. **consumer を先に対応させる**: 移行期間には旧名・新名の両方を受け付ける条件を用意し、どちらを優先するかを定める。同じ事象を両方の条件が拾う集計は二重計上しないか確認する。image tags は配列として実際の exporter / backend を通す
4. **新スキーマのみへ絞る**: 読み手の移行と出力比較が済んだ対象から既定動作へ戻す。旧属性の利用が残っていないこと、alert の空結果・誤検知が増えていないことを確認する
5. **戻す境界を決める**: gate だけ戻すのか image も戻すのかを事前に分ける。保存済み telemetry の過去データは collector のフラグ変更だけで書き換わらないため、履歴を読む query の互換期間も決める

これはこのリポジトリ向けの設計案で、公式の一律の rollout 手順ではない。具体的な保存先や負荷に対する性能・コスト測定は行っていない。

## 受入試験案と失敗の切り分け

- **旧名依存**: 旧 `k8s.pod.labels.team` のみを見る条件にテスト入力を流し、dual emission 中と stable のみの状態を区別できるか確認する。空結果を「対象 Pod が存在しない」と即断しない
- **型の差**: `container.image.tags` を受けた後のルール、index、検索、画面表示を確認する。型変換エラーを無視して文字列へ丸めない
- **新旧混在**: collector の一部だけ更新した状況を用意し、同一 workload を過不足なく集計できるか調べる。移行前後の query を単純加算しない
- **association 不成立**: proxy 経由や pipeline 順序の変更を別の条件として検証する。新旧どちらの名前も現れない場合、rename だけを原因と決めつけない
- **rollback**: 戻し用の起動引数と旧名を使う consumer の組を canary で検証する。runtime の feature gate が存在しない場合に暗黙の成功としない

これらの動作試験は未実施。追加する検索 eval は、この知識が検索で見つかることを確認するものであり、Collector の実動作や rollout 成功を保証しない。

## 未確認事項・出典・鮮度

- 実環境の Kubernetes 版、vendor distro への収録日、後続 patch の差分、Helm chart の既定引数、各 exporter / backend の配列処理と命名変換は未確認
- beta gate の終了日・最終サポート版は参照資料に明記がなく、将来版まで rollback 可能とは扱わない
- README の一部設定例には旧 `container.image.tag` が残る。例の全文を一括転載せず、互換節・生成 reference と照合した範囲だけを使った。旧 `extract.metadata` 指定がすべての gate の組でどう扱われるかは runtime 未検証
- release と告知は native web で本文を開いた。README は tag 指定の raw 本文を native web で開き、GitHub connector で commit 固定版を再確認。commit 指定 raw URL と生成 reference の native web 取得は Cache miss だったため、後者も connector の同 commit で補った
- repository 文書の [LICENSE](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/3f8455d8038a985398861171e5310bc9b4e988b2/LICENSE) は Apache-2.0。告知サイトの [LICENSE](https://raw.githubusercontent.com/open-telemetry/opentelemetry.io/main/LICENSE) は CC-BY-4.0。release 本文固有のライセンスは未確認で、独自要約のみとしコードや長文は転載していない

取得日は **2026-10-02 UTC**。README / 生成 reference に独立した公開日表示はないため、tag と commit で適用版を限定した。release_notes の30日 TTL が最短なので再確認期限は **2026-11-01**。この文書は v0.161.0 の変更を検証した記録であり、現在の最新 release との同一性は主張しない。

---
{
  "id": "opentelemetry-js-config-failfast-noop-boundary",
  "title": "OpenTelemetry JS 0.222.0: 設定の厳格化と計装なしで起動する境界",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "@opentelemetry/sdk-node 0.222.0 / experimental/v0.222.0 (2026-08-31); fixed upstream 0b72a81636fa476e8f1f1afd2ae0c90a1362194c; verified 2026-10-02, runtime untested",
  "tags": [
    "research-domain:quality-operations",
    "opentelemetry",
    "startNodeSDK",
    "NodeSDK",
    "NOOP_SDK",
    "OTEL_NODE_RESOURCE_DETECTORS",
    "configuration",
    "fail-fast"
  ],
  "sources": [
    {
      "id": "otel-js-config-release-02220-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-js/releases/tag/experimental%2Fv0.222.0",
      "type": "release_notes"
    },
    {
      "id": "otel-js-config-start-02220-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/src/start.ts",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-js-config-resource-02220-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/src/create-from-config.ts",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-js-config-env-02220-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/configuration/src/EnvironmentConfigFactory.ts",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-js-config-factory-02220-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/configuration/src/ConfigFactory.ts",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-js-config-readme-02220-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/README.md",
      "type": "github_repository_analysis"
    },
    {
      "id": "otel-js-config-tests-02220-20261002",
      "url": "https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/test/start.test.ts",
      "type": "github_repository_analysis"
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

# OpenTelemetry JS の設定失敗と計装なし起動

## 問いと結論

`@opentelemetry/sdk-node` を 0.222.0 へ更新したとき、設定の「fail-fast」はアプリケーションの起動失敗を意味するか。**実験的な `startNodeSDK()` では、設定読込や SDK 部品の作成に失敗すると診断を出して `NOOP_SDK` を返す。呼出しが戻ったことや `shutdown()` が成功したことだけでは計装の成立を確認できない。**

[2026-08-31 のリリース](https://github.com/open-telemetry/opentelemetry-js/releases/tag/experimental%2Fv0.222.0)は、propagator・MeterProvider・TracerProvider・Resource を設定から作る際の厳格化を破壊的変更として列挙している。設定ファイルを使わない `startNodeSDK()` でも `OTEL_NODE_RESOURCE_DETECTORS=all` は影響例に挙げられる。一方、この変更は `new NodeSDK()` 利用者には影響しないとの説明がある。入口を混同せず、使用版と起動方法を一緒に調べる。

## 対象と版の境界

- 対象は experimental/v0.222.0 と、同タグの commit `0b72a81636fa476e8f1f1afd2ae0c90a1362194c`。0.222.0 より前の細部や将来版の修正を推測しない
- `sdk-node` は [同版 README](https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/README.md)でも実験的パッケージであり、新版が破壊的変更を含み得る
- `new NodeSDK(...); sdk.start()` と `startNodeSDK(...)` は同義の表記ではない。前者用の設定説明を後者へ機械的に移してはいけない
- 本稿は起動時の設定境界を扱う。OTLP 送信後の再試行や tail sampling は[送信上限](otlp-message-size-retry-boundary.md)・[遅着 span](tail-sampling-late-span-decision-cache.md)の既存文書を参照する

## 固定実装から確認した動作

### 1. 設定ファイルなしでも同じ作成経路へ入る

[`ConfigFactory.ts`](https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/configuration/src/ConfigFactory.ts)は `OTEL_CONFIG_FILE` に値があればファイル用 factory、そうでなければ環境変数用 factory を選ぶ。「YAML を使っていないので設定変更の対象外」とは判定できない。

[`EnvironmentConfigFactory.ts`](https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/configuration/src/EnvironmentConfigFactory.ts)は `OTEL_NODE_RESOURCE_DETECTORS=all` を複数 detector のモデルへ展開し、その先頭に `container` を含める。`serviceinstance` はモデル上の `service` へ対応付けられる。環境変数名とモデル中の名前は必ずしも一致しない。

### 2. 設定モデルが表せても SDK が実装するとは限らない

[`createResourceFromConfig`](https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/src/create-from-config.ts)のこの版の detector 分岐は `host`・`process`・`service` を処理し、未知の名前は例外とする。`container` は別リポジトリにある detector のため未対応、とコード内に説明がある。したがって上記の `all` 展開は部品作成の例外経路へ至る。これはリリースの警告と固定実装の両方で確認した結論であり、実行環境での再現結果ではない。

また、同ファイルの `checkConfigUse` は未処理の property に警告を出す処理である。「あらゆる未知フィールドが例外になる」「設定モデルを解析できれば全機能が有効になる」のどちらも正しくない。部品種別の未対応と property の未処理を分けて確認する。

### 3. 内部の例外とプロセス終了を区別する

[`start.ts`](https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/src/start.ts)では、設定モデルの読込失敗と `create(...)` の失敗を別々に捕捉し、診断後に no-op オブジェクトを返す。global provider の登録は部品作成成功後に行う。返り値の契約は `shutdown` 関数であり、公開の成功フラグはない。no-op 側にも即座に完了する `shutdown()` がある。

従って、これらの失敗を検出するために `try/catch` だけを置く、返り値が truthy なら成功とする、終了処理が解決したら計装済みとする、という判定には穴がある。ただし任意の初期化例外をすべて捕捉するという保証には拡張しない。instrumentation の登録は部品作成の try 範囲の前にあり、「no-op を返したら副作用も資源も完全にゼロ」とも断言しない。

### 4. upstream test の確認範囲

[固定版の起動テスト](https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/test/start.test.ts)には、不正な設定形式、不存在のファイル、未知の LogRecordProcessor について、診断と `NOOP_SDK` を確認したうえで `shutdown()` を呼ぶ例がある。これは上流テストの読解であり、本調査でそのテストを実行したという意味ではない。正常応答の HTTP サービスが存在することと、trace・metric・log の受信確認は別の検証事項になる。

## 実務での移行判断

以下は一次資料の機構に基づく独自の運用案であり、SDK が自動で行う保証ではない。

1. lockfile の実版、起動入口、設定ファイルパス、detector 指定を変更前に記録する。秘密値を含む環境変数全体や exporter header をログへ出さない
2. `startNodeSDK` と `all` の組合せなら更新前に修正候補を評価する。サポートされる必要最小の detector への変更、または既存の `NodeSDK` 初期化の維持を検討し、resource 属性と各 signal が維持されることを試験する。単に `all` を削除して成功扱いにしない
3. canary で既知の trace・metric・log を必要な signal ごとに送る。生成、export、収集先への到着を分け、service.name や resource の値、sampling 設定も比較する
4. 起動時診断を独立したログ経路で確認する。計装が壊れたことを、その計装からしか送れない監視に依存させない
5. 事前に許容した方針に沿って、計装が成立しない場合はデプロイを止めるか、計装欠損を明示した縮退にする。アプリ本体の可用性と必須の観測証跡を両方評価し、SDK の no-op 動作を暗黙の承認にしない

回帰試験の候補は、正常な最小設定、`all`、未知 detector、不存在ファイル、未知 processor、意図した `OTEL_SDK_DISABLED` の各経路である。戻り値だけでなく、診断・受信側の実データ・期待する無効化の理由を突き合わせる。ここで挙げた候補は本調査では未実行である。

## 制約・未確認事項・由来

- 本番アプリ、Node.js 実行、collector 接続、速度・欠損率の測定は未実施。検索 eval は文書の検索性を確認するもので、SDK 動作テストではない
- README の detector 説明は class 用の説明と実験的設定経路で差がある。この固定版ではコードの対応分岐と実際の入口を確認し、README の `all` 説明を全入口共通の対応表にしない
- v3 の予定日や別パッケージの版番号から、この問題が解決したとは判断しない。更新候補ごとにリリースと実装を再確認する
- 固定したコードは [package LICENSE](https://github.com/open-telemetry/opentelemetry-js/blob/0b72a81636fa476e8f1f1afd2ae0c90a1362194c/experimental/packages/opentelemetry-sdk-node/LICENSE) とヘッダーで Apache-2.0 を確認した。コードは転載していない。release prose の独立したライセンスは未確認のため、独自要約のみを保存した
- 一部固定コードの native web 取得は cache miss だったため、同一 SHA の raw HTTPS 本文を直接取得して確認した。未取得を確認済みに置き換えていない。取得日はいずれも 2026-10-02 UTC

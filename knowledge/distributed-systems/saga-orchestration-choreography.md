---
{
  "id": "distributed-systems-saga-orchestration-choreography",
  "title": "Saga による分散トランザクション: orchestration と choreography の選択、compensating transaction 設計、isolation 欠如への対処",
  "kind": "knowledge",
  "technology": "distributed-systems",
  "version": "Azure Architecture Center Saga Design Pattern (ms.date 2025-02-25, page updated 2026-06-03)、AWS Prescriptive Guidance Saga pattern (current, 版表記なし)、microservices.io Pattern: Saga by Chris Richardson (版・日付表記なし)",
  "tags": [
    "research-domain:api-distributed",
    "distributed-systems",
    "saga",
    "orchestration",
    "choreography",
    "compensating-transaction",
    "compensation",
    "compensable",
    "pivot",
    "retryable",
    "orchestrator",
    "domain-events",
    "isolation",
    "lost-update",
    "idempotent-consumer",
    "transactional-outbox",
    "event-sourcing"
  ],
  "sources": [
    {
      "id": "azure-architecture-saga-pattern",
      "url": "https://learn.microsoft.com/en-us/azure/architecture/patterns/saga",
      "type": "architecture_pattern"
    },
    {
      "id": "aws-prescriptive-guidance-saga-pattern",
      "url": "https://docs.aws.amazon.com/prescriptive-guidance/latest/modernization-data-persistence/saga-pattern.html",
      "type": "architecture_pattern"
    },
    {
      "id": "microservices-io-saga",
      "url": "https://microservices.io/patterns/data/saga.html",
      "type": "architecture_pattern"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2027-03-26",
  "trust": "maintainer",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# Saga による分散トランザクション: orchestration と choreography の選択、compensating transaction 設計、isolation 欠如への対処

複数サービスにまたがる更新を単一の ACID トランザクションにできないとき、各サービスのローカルトランザクションを連続させ、失敗したら compensating transaction で前段を取り消す Saga の定義、choreography と orchestration の2協調方式、isolation 欠如の代償を、3件のパターン文書で整理する。Azure の [Saga Design Pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/saga)（ms.date 2025-02-25、ページ更新 2026-06-03）、AWS の [Saga pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/modernization-data-persistence/saga-pattern.html)（Prescriptive Guidance、current）、microservices.io の [Pattern: Saga](https://microservices.io/patterns/data/saga.html)（Chris Richardson、版・日付表記なし）を 2026-09-27 に取得して確認した。以下で「記載事実」「設計案」を明示的に分ける。

## 要点（パターン文書の記載事実）

### Saga の定義: ローカルトランザクションの連続と補償

- 3文書は一致して、Saga を各サービスが自分のデータベースだけを更新するローカルトランザクションの連続（sequence）と定義する。各ステップは自サービス DB を更新し、次のステップを event または message で起動する。全体を束ねる分散 ACID トランザクションは使わない。
- どこかのステップが失敗したら、それまでに成功した前段を compensating transaction（補償トランザクション）で取り消す。自動で rollback されるのではなく、取り消しのためのトランザクションを明示的に設計・実装する。
- microservices.io は例として注文（order）とクレジット予約（credit reservation）の流れを示す。各サービスが DB 更新と message/event 発行を1ステップとして実行する。
- AWS は Saga を、トランザクション毎の event と成功パス・失敗パスで複数マイクロサービスの処理を調整する方式と説明する。適用条件として、サービス間を密結合にせず整合性を取りたい場合、長時間にわたるトランザクション、rollback が必要になる処理を挙げる。

### 2つの協調方式: choreography と orchestration

- choreography は中央の controller なしに event 駆動で進める方式。各サービスが domain event を受けて自分のローカルトランザクションを実行し、次の event を出す。microservices.io はこれを domain event による協調と説明する。
- orchestration は中央の orchestrator が全体の状態を保持し、各参加サービスへ command を送って駆動する方式。失敗時には orchestrator が補償の実行を駆動する。Azure は orchestrator が状態を保持して補償を駆動する点を明記する。
- Azure は両方式の benefits と drawbacks を対比する。AWS はサービス数が増えるほど debug が難しくなると警告する。方式の選択は追跡可能性と結合のトレードオフになる（詳細な優劣の評価は各文書の対比表を参照する。本書は文書の記載を超える一般化をしない）。

### トランザクションの分類: compensable / pivot / retryable

- Azure は Saga を構成する各ローカルトランザクションを compensable、pivot、retryable に分類する考え方を示す。補償設計では、取り消し可能な段階と、失敗後に再試行で前進させる段階を区別して配置する。
- 本調査で確認した範囲では、各分類の厳密な定義文の引用までは行わない。実務上は「どの段階までなら補償で戻せるか、どの段階から先は戻せないか、どこで再試行するか」を設計書に明示する前提として扱う。

### 代償1: 自動 rollback がない

- microservices.io は drawback として、Saga には自動 rollback がないことを明記する。失敗パスの compensating transaction は開発者が1件ずつ用意する。AWS も compensating transaction を明示的に組む programming model が必要だと述べる。
- 補償自体の失敗（補償の再試行、部分的に残る副作用）への備えも設計に含める必要がある。3文書はいずれも補償の用意を Saga 採用の前提条件として扱う。

### 代償2: サービス横断の isolation がない

- Azure と microservices.io はともに、Saga にはサービスをまたぐ isolation がないことを明記する。Azure は起こりうる読み取り異常として lost update、dirty read（dirty な読み取り）、nonrepeatable read を挙げ、対処（countermeasures）が必要だと述べる。microservices.io も isolation なしへの対処要求を drawback に挙げる。
- 本調査で確認した個別の対処パターンの網羅は行わない。文書が求める方向性は、未確定の中間状態を他処理に見せない工夫、冪等な consumer、監視の組み合わせである。

### 信頼できる発行: Transactional Outbox または Event Sourcing

- microservices.io は、DB 更新と message/event 発行を確実に両立させるには Transactional Outbox または Event Sourcing が必要だと述べる。DB 更新だけ commit されて発行が抜ける、またはその逆の dual write を避けるための要求である。
- 発行側の具体的な実装（polling publisher、CDC / transaction log tailing、at-least-once での重複排除）は本書の範囲外とし、同一 domain の `messaging-transactional-outbox` に譲る。

### 運用の要求: 冪等と監視、debug 困難への備え

- Azure は冪等（idempotence）と監視（monitoring）の必要性を述べる。event の再送・再実行が起きても副作用が重複しない受け止めと、Saga 全体の進行状態を見える化することが前提になる。
- AWS はサービス数が増えるほど debug が困難になると警告する。失敗パスを含めた event の追跡手段なしに規模を増やさないことが含意される。

## 推奨方法（独自の設計案。上記文書の規定ではない）

- Saga を選ぶのは、AWS が挙げる条件に当てはまるときに絞る。サービス間を密結合にせず整合性を取りたい、処理が長時間にわたる、失敗時に rollback（補償）が必要になる。短時間で単一 DB に閉じる処理を Saga にしない。
- 協調方式は追跡と結合で選ぶ。参加サービスが少なく event の流れが追えるうちは choreography、参加数が増える・失敗時の補償順序を中央で制御したい・全体の進行状態を持ちたい場合は orchestration を既定にする。方式の混在（Saga の一部だけ orchestration 化など）は文書に規定がないため、混在させるなら境界と状態の持ち方を設計書に明示する。
- 補償は各ステップと対にして設計する。成功パスと失敗パスを同じ設計図に書き、補償の順序（原則として実行の逆順）、補償自体が失敗したときの再試行方針、補償不能な段階（pivot 相当）の位置を明示する。Azure の compensable / pivot / retryable の分類を、そのまま設計書の見出しに使う。
- isolation 欠如は前提として受け入れ、中間状態の扱いを決める。未確定の予約・仮押さえを他処理が確定済みとして読まない規約（状態の明示、読み取り側の条件指定）、再送・再実行に対する consumer の冪等（イベント ID による重複排除、同一ローカルトランザクションでの処理済み記録）を必須要件にする。
- 発行の信頼性は outbox 側に寄せる。各ステップの DB 更新と event 発行を同一トランザクションで outbox 行に書き、relay がブローカへ送る構成にする（詳細は `messaging-transactional-outbox` の推奨方法に従う）。
- 可観測性を最初から入れる。Saga ID・ステップ番号・成功/失敗パスを event に付与し、orchestration なら orchestrator の保持状態、choreography なら event 連鎖を追える相関 ID で辿れるようにする。AWS の debug 困難の警告に対する直接の備えである。

## 避ける使い方

- 単一 DB の ACID トランザクションや 2PC が使える構成で Saga を選ぶこと（適用条件に反する）。
- compensating transaction なしに Saga を始めること（自動 rollback はない、と2文書が明記）。
- 補償の失敗時動作を決めないこと（補償自体の再試行・残存副作用の扱いが未定義になる）。
- どの段階まで戻せるか（pivot 相当の位置）を曖昧にすること（分類の考え方を示す Azure の前提に反する）。
- サービス横断の isolation がある前提で読み書きを設計すること（lost update / dirty read / nonrepeatable read が起きうる、と Azure が明記）。
- 非冪等な consumer で event 駆動の Saga を組むこと（再送・再実行時の重複副作用。冪等と監視は Azure の要求）。
- DB 更新と発行の dual write を各サービスコードの逐次呼び出しで済ませること（Transactional Outbox または Event Sourcing が必要、と microservices.io が明記）。
- 失敗パス・追跡手段なしに choreography の参加サービスを増やすこと（サービス数に伴う debug 困難、と AWS が警告）。
- Azure の benefits/drawbacks 対比を読まずに「orchestration が常に優れる」「choreography が常に単純」と決めつけること（文書は両者の対比として論じており、一方向の一般化は記載を超える）。

## 適用版と本番での注意

- 適用版: Azure Architecture Center の Saga ページ（ms.date 2025-02-25、ページ更新 2026-06-03）、AWS Prescriptive Guidance の Saga ページ（current、版表記なし）、microservices.io の Saga ページ（版・日付表記なし、footer は Copyright © 2026 Chris Richardson）。いずれも 2026-09-27 取得。
- 保証の範囲: 3文書が保証するのは Saga の構造（ローカルトランザクションの連続、event/message による起動、補償による取り消し）と代償（自動 rollback なし、isolation なし）までである。具体的な補償の冪等実装、再試行間隔、タイムアウト、Saga 全体の完了期限（timeout scope）は文書に規定がない（未確認）。必要なら各実行基盤の文書で別途確認する。
- 方式の代償: choreography と orchestration の具体的な benefits/drawbacks の文言は Azure の対比表を参照する。本書は inventory で確認した骨子（controller の有無、orchestrator の状態保持と補償駆動）を超える優劣の断定をしない。
- 関連知識: 発行の信頼性パターンは `messaging-transactional-outbox`（polling publisher と CDC の選択、at-least-once での重複排除）を参照する。本書は Saga 側の要求（Outbox または Event Sourcing が必要）のみを扱う。
- 未確認範囲: compensable / pivot / retryable の厳密な定義文の引用、isolation 対処の個別パターンの網羅、特定オーケストレーション製品（Temporal、Camunda 等）の Saga 実装差は本調査の範囲外。
- 再確認期限: 全 source が architecture_pattern（TTL 180日）。technology（distributed-systems）固有 TTL は設定されていない。2027-03-26 までに3文書を再取得して内容を確認する。

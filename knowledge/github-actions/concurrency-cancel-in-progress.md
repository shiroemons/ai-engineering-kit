---
{
  "id": "github-actions-concurrency-cancel-in-progress",
  "title": "GitHub Actions の concurrency group と cancel-in-progress: workflow/job スコープ、pending キュー、キャンセル保証の限界",
  "kind": "knowledge",
  "technology": "github-actions",
  "version": "GitHub Docs (workflow syntax / control concurrency / workflow cancellation, unversioned current pages) と GitHub Changelog 2026-05-07、2026-09-27 取得",
  "tags": [
    "research-domain:infrastructure",
    "github-actions",
    "concurrency",
    "concurrency group",
    "cancel-in-progress",
    "queue max",
    "pending",
    "workflow",
    "job",
    "github.workflow",
    "SIGINT",
    "cancelled()",
    "cancellation timeout",
    "if always"
  ],
  "sources": [
    {
      "id": "github-docs-workflow-syntax-concurrency",
      "url": "https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax",
      "type": "official_docs"
    },
    {
      "id": "github-docs-control-workflow-concurrency",
      "url": "https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs",
      "type": "official_docs"
    },
    {
      "id": "github-changelog-concurrency-queue-max-2026-05-07",
      "url": "https://github.blog/changelog/2026-05-07-github-actions-concurrency-groups-now-allow-larger-queues",
      "type": "release_notes"
    },
    {
      "id": "github-docs-workflow-cancellation",
      "url": "https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-10-27",
  "trust": "official",
  "status": "active"
}
---

# GitHub Actions の concurrency group と cancel-in-progress: workflow/job スコープ、pending キュー、キャンセル保証の限界

concurrency group は「同時にどれだけ進めるか」ではなく「group ごとに実行は1つ、残りは pending として扱う」制御であり、既定の挙動は実行中の置換ではなく pending の置換である。以下は4つの公式ページ（いずれも2026-09-27に確認した unversioned な現行ページ）と2026-05-07の公式 Changelog に基づく記載事実と、それを組み立てる設計案を分けて書く。

## 要点（公式文書に記載された事実）

### スコープと group の作り方

- `concurrency` は workflow レベルと `jobs.<job_id>.concurrency` の両方で指定できる。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- 許可される式のコンテキストがスコープで異なる。workflow レベルの group 式は `github`・`inputs`・`vars` コンテキストのみ。job レベルはさらに `needs`・`strategy`・`matrix` を含められる。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax) [Control the concurrency of workflows and jobs](https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs)
- group 名は大文字小文字を区別しない (case insensitive)。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- 同一リポジトリ内で group 名は workflow をまたいで一意でなければならない。`github.workflow` を group 名に含めないと、他の workflow の run も同じ group に入り、意図せずキャンセル対象になる。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- 同 group の run は「各 run が待機を開始した時刻」順 (FIFO) で処理される。dispatch 時刻ではなく待機開始時刻が基準で、文書は順序を保証しないと明記する。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)

### pending の扱いと cancel-in-progress

- 1つの concurrency group につき実行できるのは workflow/job が最大1つ。別の run が進行中新規 run は `pending` 状態になる。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- 既定 (`queue` 未指定、`cancel-in-progress` 未指定) では、group に既に pending run がある状態で新しい run が入ると、既存の pending run がキャンセルされて新しい run に置き換わる。pending の置換であり実行中 run の中断ではない。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax) [Control the concurrency](https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs)
- `cancel-in-progress: true` を付けると、進行中の run もキャンセル対象になる。値は条件式で指定でき、文書の例では `!contains(github.ref, 'release/')` により release 系ブランチでは実行中を止めない。[Control the concurrency](https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs)
- group 名の例として `github.head_ref || github.run_id` が示される。PR では head_ref でまとめ、イベントに head_ref が無い場合 (schedule 等) は run_id で group を分離する。[Control the concurrency](https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs)

### `queue` プロパティ (2026-05-07 の変更)

- `concurrency.queue` は `single` (既定) と `max` の2値を取る。`single` は従来どおり pending は最大1件で置換される。`max` は pending を最大100件まで保持し、100件を超えて入る run はキャンセルされる。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax) [Control the concurrency](https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs)
- `queue: max` は `cancel-in-progress` が `false` または未指定のときだけ指定できる。`queue: max` と `cancel-in-progress: true` の併用は workflow の検証エラーになる。[Control the concurrency](https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs) [Changelog 2026-05-07](https://github.blog/changelog/2026-05-07-github-actions-concurrency-groups-now-allow-larger-queues)
- 変更前は1つの進行中 run と1つのみ pending を保持し、追加で入る run が pending を置換キャンセルしていた。複数 pending を直列処理できる `queue: max` はこの Changelog で追加された機能で、公式の機能追加日は2026-05-07。[Changelog 2026-05-07](https://github.blog/changelog/2026-05-07-github-actions-concurrency-groups-now-allow-larger-queues)
- `queue: max` の pending も直列 (sequentially) に処理される。[Changelog 2026-05-07](https://github.blog/changelog/2026-05-07-github-actions-concurrency-groups-now-allow-larger-queues)

### キャンセルの実際の動作 (cancel-in-progress は無条件の保証ではない)

- キャンセル時にサーバは実行中 job の `if` を再評価する。`if: always()` の job はキャンセルされず、条件なしの `if` は `if: success()` と同等として扱われる。[Workflow cancellation](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)
- キャンセルされず残った job では、未完了の step の `if` が `cancelled()` 表現で再評価される。[Workflow cancellation](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)
- step の停止手順は、runner が step の entry process に SIGINT (Ctrl-C) を送り、7500 ms 後に SIGTERM/Ctrl-Break、さらに2500 ms 待った後にプロセスツリーを kill する。entry process は JavaScript action が node、container action が docker、`run` ステップが bash/cmd/pwd。[Workflow cancellation](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)
- キャンセルされてから5分の cancellation timeout が経過すると、サーバはキャンセル対象として残っている残りの job/step を強制終了する。[Workflow cancellation](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- group 名には既定で `github.workflow` を含める。特定の workflow だけをまとめたい場合が大半で、含めないと他 workflow の run まで同じ group に束ねられ cancel 対象になる。
- PR の CI は `github.head_ref || github.run_id` を group 名にし、PR 間の重複実行を抑えつつ、head_ref を持たないイベントでは group を分けられるようにする。デプロイなど「最新でなければ意味がない」処理には `cancel-in-progress: true` を付け、release 系ブランチには条件式で免除を残す。
- 「1回でも実行されなければ漏れる」処理 (全件必要なデータ投入、通知、後始末) では既定の pending 置換を避け、`queue: max` で直列キュー化する。`cancel-in-progress` と併用できないため、実行中の中断が不要な workflow に限る。
- キャンセルは best-effort とみなし、長期 step は SIGINT に応答できるようにする。終了時の後始末が必要な job は `if: always()` を付けて残し、その job 内では step の `if` を `cancelled()` で分岐させる。
- `group` と並行して job 間の依存は `needs` で表現し、concurrency に依存関係の表現を期待しない。job レベルの group 式で使える `needs` は「実行順の制御」ではなく「group 名の生成素材」である。

## 避ける使い方

- `github.workflow` を含めない group 名を複数 workflow で共有する。他の workflow の run までキャンセル対象になる点を公式は明記する。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- `queue: max` と `cancel-in-progress: true` を併用する。workflow 検証エラーになる。[Control the concurrency](https://docs.github.com/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/control-the-concurrency-of-workflows-and-jobs)
- 既定の挙動を「古い run が実行中にキャンセルされる」と理解する。既定で置換キャンセルされるのは pending run であり、実行中の中断は `cancel-in-progress: true` を付けた場合。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- dispatch 順に実行されると考える。基準は待機開始時刻で、文書は順序を保証しないと明記する。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)
- `cancel-in-progress: true` を「指定した run は確実に即座に死ぬ」と読む。`if: always()` の job は生き残り、残った step の `if` は `cancelled()` で再評価され、プロセスの終了には SIGINT→7500 ms→SIGTERM→2500 ms→kill と段階があり、5分の cancellation timeout で最後に強制終了する。[Workflow cancellation](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)
- pending は無制限に溜まる (または100件あれば十分) と考える。`queue: single` なら1件、`queue: max` でも100件を超えたら以降はキャンセルされる。[Workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)

## 適用版と本番での注意

- 適用版: GitHub Docs の Workflow syntax / Control the concurrency / Workflow cancellation の3ページはいずれも版ラベルのない現行ページで、2026-09-27に確認した内容に基づく。将来の最新とは扱わない。`queue: max` は2026-05-07の Changelog で追加された機能で、それ以前の GHES など旧環境では存在しない可能性を排除できない (本調査では GHES への提供状況は未確認)。
- 再確認期限: 本文書の source のうち Changelog が `release_notes` (TTL 30日) のため、2026-10-27 に再取得して内容を確認する。3つの公式ドキュメント (TTL 90日) は2026-12-26が相当だが、有効期限は最も短い source に引きずられる。
- 未確認事項 (推測で埋めない): `queue: max` の pending が repo または org の run 数制限にどう影響するか、pending 中の run の再評価 (イベント再送や PR 更新) の具体的な優先順位、GHES での対応バージョン、`cancelled()` 再評価時の env/outputs の保持範囲。
- 本ドキュメントの推奨構成は設計案であり、特定リポジトリの workload (dispatch 頻度、step 長、release 分岐) に対するもの。公式の記載事実 (スコープ、pending 置換、queue 上限、キャンセル手順) とは本文中で区別してある。

---
{
  "id": "github-actions-node24-runtime-migration",
  "title": "GitHub Actions の Node 24 移行: action runtime・アプリの Node.js・runner の版を分ける",
  "kind": "knowledge",
  "technology": "github-actions",
  "version": "github.com / GitHub with Data Residency: Node 20 action runtime removal 2026-09-23; Node 24 default migration from 2026-06-16; GitHub Docs unversioned pages verified 2026-10-01; GHES未確認",
  "tags": [
    "research-domain:infrastructure",
    "github-actions",
    "node24",
    "node20",
    "action runtime",
    "runs.using",
    "setup-node",
    "node-version",
    "self-hosted",
    "migration"
  ],
  "sources": [
    {
      "id": "github-changelog-node20-removal-20260923-r20261001",
      "url": "https://github.blog/changelog/2026-09-23-node-20-is-no-longer-available-in-github-actions/",
      "type": "release_notes"
    },
    {
      "id": "github-changelog-node24-transition-r20261001",
      "url": "https://github.blog/changelog/2025-09-19-deprecation-of-node-20-on-github-actions-runners/",
      "type": "release_notes"
    },
    {
      "id": "github-docs-action-metadata-node24-r20261001",
      "url": "https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax",
      "type": "official_docs"
    },
    {
      "id": "github-docs-nodejs-project-runtime-r20261001",
      "url": "https://docs.github.com/en/actions/tutorials/build-and-test-code/nodejs",
      "type": "official_docs"
    },
    {
      "id": "github-docs-action-version-ref-r20261001",
      "url": "https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# GitHub Actions の Node 24 移行: action runtime・アプリの Node.js・runner の版を分ける

## 問いと今回の変更

2026-09-23 の最終告知後、JavaScript action の Node 20 警告に対して何を更新すべきか。対象は github.com と GitHub with Data Residency。GitHub は action の実行を Node 24 へ移し、Node 20 と暫定 opt-out を利用できなくしたと公表した。アプリケーションをビルド・テストする Node.js の選択とは別に点検する。[最終告知](https://github.blog/changelog/2026-09-23-node-20-is-no-longer-available-in-github-actions/)

この文書は公開告知の変更点を扱う。一般的な「最新 action に更新する」という知識の追加ではなく、移行猶予の終了により旧対処が使えなくなった差分を記録する。実際の runner 上で互換性試験はしていない。

## 確認した移行段階

| 時点・資料 | 確認した状態 | 読み方 |
| --- | --- | --- |
| 2025-09-19 公開の旧告知 | 当時の runner v2.328.0 は Node 20 / Node 24 に対応し、既定は Node 20。先行確認には `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` | 過去の移行準備。v2.328.0 を現在の運用最低版とは扱わない |
| 旧告知の 2026-05-19 改訂 | Node 24 への既定切替開始は 2026-06-16。`ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION=true` による一時的な Node 20 継続を案内 | 切替開始日であり、全 runner の同時切替完了を意味しない |
| 旧告知の 2026-08-25 改訂 | Node 20 廃止予定日を 2026-09-23 に更新 | 古い転載・過去の予定日をそのまま使わない |
| 2026-09-23 最終告知 | Node 20 と一時 opt-out が利用不可、JavaScript actions は Node 24 を使用 | 2026-10-01 の判断ではこの確定告知を優先する |

根拠: [改訂を含む旧告知](https://github.blog/changelog/2025-09-19-deprecation-of-node-20-on-github-actions-runners/)、[最終告知](https://github.blog/changelog/2026-09-23-node-20-is-no-longer-available-in-github-actions/)。旧告知には古い将来形の文章も残るため、本文の一文だけで現在の状態を決めない。

## 混同しやすい三つの設定

### 1. JavaScript action 自身の runtime

`action.yml` / `action.yaml` の `runs.using` は action のコードを実行する runtime を指定する。`runs.main` に加え、存在する `runs.pre` と `runs.post` もその runtime を使う。action 作者は `runs.using: node24` へ変更し、対応する新しい版を公開するよう最終告知で案内されている。[Metadata syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/metadata-syntax#runs-for-javascript-actions)、[最終告知](https://github.blog/changelog/2026-09-23-node-20-is-no-longer-available-in-github-actions/)

取得時点の metadata reference には `node20` と `node24` が両方列挙されていた。これは観測した文書間の差であり、Node 20 の実行継続を保証する根拠にはしない。また「古い `runs.using: node20` が必ず構文エラーになる」とも、この調査からは断定できない。構文の記載とサービスで利用可能な runtime を分けて読む。

### 2. プロジェクトを実行する Node.js

`actions/setup-node` の `node-version` は指定版の Node.js バイナリを `PATH` に追加し、残りの job で利用するための入力。matrix を通じてアプリの複数 Node.js 版をビルド・テストできる。[Building and testing Node.js](https://docs.github.com/en/actions/tutorials/build-and-test-code/nodejs#specifying-the-nodejs-version)

以上からの整理として、`node-version` を 24 に変えるだけで参照しているすべての action の metadata が更新されるわけではない。逆に Node 24 対応 action を採用した事実だけで、アプリの `node-version` も 24 に変わったとは判断しない。`run: node --version` は後続コマンド側の確認には使えるが、それだけで action 自身の runtime の証明にはならない。

### 3. action の参照版と runner 環境

workflow の `uses` は実行する action とその ref / SHA を選ぶ。公式文書は、公開済み版の commit SHA 固定を安定性・安全性の観点で推奨し、major tag の更新方針は作者次第、default branch 参照は破壊的変更を受ける可能性があると説明する。[Workflow syntax: uses](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idstepsuses)

Node 24 対応を確認した action の参照先へ更新する必要がある。SHA 固定している場合も、旧 SHA のままにしておけば互換性が自動で得られるとは考えない。一方、SHA が固定するのは action の版であり、ホストサービスの廃止日を延長する設定ではない。これは資料を組み合わせた運用上の判断である。

self-hosted runner では OS / architecture も点検する。最終告知は macOS 13.4 以下との非互換、ARM32 の公式非サポート、および該当環境の self-hosted runner サポート終了を明記する。単に action の参照を更新するだけでこの環境制約は解消しない。[最終告知](https://github.blog/changelog/2026-09-23-node-20-is-no-longer-available-in-github-actions/)

## 移行の実務判断（独自の設計案）

以下は公式手順の引用ではなく、確認した契約に基づく点検案。

1. 失敗した step を特定し、`uses` の action 実行なのか、`run` のアプリ処理なのかを区別する。最初からアプリの Node.js major を変えると、別の破壊的変更まで混ざる。
2. workflow と利用する composite action / reusable workflow の呼出し先を棚卸しする。直接見えている `uses` だけで確認を終えず、実際に呼ばれる JavaScript action の固定版と `runs.using` を確認する。
3. action 利用者は Node 24 対応が確認できる公開版を選び、入力・出力・権限・cache などの関連する版差もレビューする。固定 SHA を使う場合は、レビュー済み公開版に対応する SHA を採用する。特定 action の最新 major 番号を、この文書で一律に指定しない。
4. action 作者は metadata の変更だけで完了とせず、依存ライブラリ、配布 bundle、通常経路とエラー経路、pre/main/post の必要な経路を Node 24 環境で試験する。利用者が選べる版として公開し、対応 runner 条件を明記する。
5. self-hosted runner は agent の版、OS、architecture を別々に記録し、使用する action の要求条件と現在の runner サポート条件を再確認する。旧移行告知に出た runner 番号を長期的な固定下限にしない。
6. 旧 opt-out を回復策にせず、対応 action と対応環境で検証する。検証記録には action ref、runner 環境、アプリの Node.js 版、通常処理と後始末の成否を残す。

受入基準の例は、固定した呼出し先が確認済みであり、対象 runner で action の初期化・主処理・後始末が成功し、アプリのサポート対象 matrix も成功すること。これは提案であり、本調査で達成した実行結果ではない。

## 限界・再確認

- GHES の適用版・適用時期は未確認。github.com / Data Residency の廃止日を GHES にそのまま移さない。
- 各 action の最新公開版、現在の self-hosted runner 最低許容版、OS 別の完全な依存条件、古い metadata に対する runner 内部の選択実装は未確認。個別採用時に確認する。
- 今回の廃止は action runtime の話であり、プロジェクトの Node 20 を安全・サポート中と認定するものではない。アプリ側の lifecycle 判断は別途必要。
- GitHub Docs の三ページは版ラベル・公開更新日のない現行ページとして 2026-10-01 UTC に確認した。将来の最新版保証はしない。
- Changelog は `release_notes` の TTL 30日を適用するため、再確認期限は 2026-10-31。取得日だけを延長せず、最終告知と文書間の差も再読する。
- GitHub Docs の説明は出典を付けて日本語で再構成した。[github/docs のライセンス説明](https://github.com/github/docs#license) と [LICENSE](https://github.com/github/docs/blob/main/LICENSE) で文書の CC-BY-4.0 を確認。Changelog はページ上にオープンライセンス表示がなく `unknown` と記録し、コード・長文の転載はしていない。

---
{
  "id": "rails-active-job-retry-discard-handler-budget",
  "title": "Rails Active Job の retry_on / discard_on: handler 優先順・試行回数・失敗終端",
  "kind": "knowledge",
  "technology": "rails",
  "version": "Ruby on Rails 8.1.4（2026-09-24公開、2026-10-01確認）; Active Job @ c3466ea00d7121798e3aa3144ffdf7174b81d8cb",
  "tags": [
    "research-domain:backend",
    "active-job",
    "retry_on",
    "discard_on",
    "after_discard",
    "retry-budget",
    "jitter"
  ],
  "sources": [
    {
      "id": "rails-activejob-exceptions-api-8-1-4-20261001",
      "url": "https://api.rubyonrails.org/classes/ActiveJob/Exceptions/ClassMethods.html",
      "type": "official_docs"
    },
    {
      "id": "rails-activejob-basics-8-1-4-20261001",
      "url": "https://guides.rubyonrails.org/active_job_basics.html",
      "type": "official_docs"
    },
    {
      "id": "rails-config-retry-jitter-8-1-4-20261001",
      "url": "https://guides.rubyonrails.org/configuring.html#config-active-job-retry-jitter",
      "type": "official_docs"
    },
    {
      "id": "rails-activejob-exceptions-c3466ea0-20261001",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activejob/lib/active_job/exceptions.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-release-8-1-4-20261001",
      "url": "https://rubyonrails.org/2026/9/24/Rails-Version-8-1-4-has-been-released",
      "type": "release_notes"
    },
    {
      "id": "rails-activejob-changelog-8-1-4-20261001",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activejob/CHANGELOG.md",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-01",
  "expires_at": "2026-10-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Rails Active Job の retry_on / discard_on

## 問い・調査時点

「5回まで retry」と書いた job がそれ以上実行される理由、同じ例外に複数 handler が合うときの選択、再試行を使い切った後の監視方法を整理する。2026-10-01 に API/Guides の表示 v8.1.4 を確認し、実装は `c3466ea00d7121798e3aa3144ffdf7174b81d8cb` に固定した。tag の剥離先と `RAILS_VERSION` も照合した。

[8.1.4 の公開日は2026-09-24](https://rubyonrails.org/2026/9/24/Rails-Version-8-1-4-has-been-released)。本稿はこの時点で使える契約の調査であり、以下の API が8.1.4で新設されたとは主張しない。queue adapter の再送保証、Active Job Continuations、DB transaction の commit 境界は別の問題として扱う。

## 8.1.4で今回確認した関連修正

[Active Job CHANGELOG @ c3466ea0](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activejob/CHANGELOG.md) の8.1.4節には、`enqueue_after_transaction_commit` によって enqueue が遅延した場合に、`retry_job` の `wait_until`・`queue`・`priority` を保持する修正が記載される。該当構成を使うチーム向けの設計案として、upgrade時には再試行先のqueue・priority・予定時刻がcommit後にも保たれることを回帰試験に加える。commit境界全般や全adapterの動作をこの一件の修正から保証しない。

同じ CHANGELOG の8.1.0節では `retry_on` / `discard_on` への `report:` 追加が確認できる。従来からの試行契約と、この版系列で追加された報告機能を分けて扱う。

## 公式 API で確認した契約

- `retry_on` / `discard_on` の選択は宣言の下から上、その後にクラス階層を上へ探索する。`exception.is_a?(klass)` に合う最初の handler が対象。「最も具体的な例外型が自動的に勝つ」という規則ではない
- `retry_on` の `attempts` は初回を含み、既定は5。`attempts: :unlimited` もある。`wait` の既定指定は3秒で、数値・Duration・Proc・`:polynomially_longer` を使える
- 予算を使い切り block がなければ例外は backend へ伝わり、その backend が追加の retry や失敗保管を行う場合がある。block があれば、代わりにその処理へ渡す
- `discard_on` は合致した例外について Active Job の再試行を行わない。対象が消えて処理の意味がなくなった場合などに使う。`report: true` は破棄前に error reporter へ報告する

根拠: [ActiveJob::Exceptions::ClassMethods](https://api.rubyonrails.org/classes/ActiveJob/Exceptions/ClassMethods.html)。Active Job 層は設定なしに失敗 job を再試行しないが、backend の独立した再試行まで否定した文ではない。GlobalID で渡したレコードが enqueue 後・perform 前に削除されると `ActiveJob::DeserializationError` になる。[Active Job Basics: Retrying / Missing Records](https://guides.rubyonrails.org/active_job_basics.html#retrying-or-discarding-failed-jobs)

## 固定した8.1.4実装から読める境界

以下は [exceptions.rb @ c3466ea0](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activejob/lib/active_job/exceptions.rb) の分岐を読んだ観察で、すべての旧版や将来版への一般化ではない。

### 試行予算は一つの job 全体の絶対上限ではない

同じ `retry_on` 宣言へ複数の例外を渡すと、その宣言の例外集合に対する `exception_executions` カウンタを共有する。キーは `exceptions.to_s` なので、異なる例外集合の宣言は通常別カウンタになる。同じ例外リストを再宣言した場合まで独立という意味ではない。古い形式で保存された job には全体の `executions` へフォールバックする分岐もある。

したがって「一つの宣言に `attempts: 5`」は、同じ handler が選ばれ続ける通常の経路なら初回＋最大4回の再 enqueue。別 handler への移行や backend による再配信まで含む総実行数を5と断定できない。

### exhaustion と after_discard は監視上の意味が違う

`retry_on` の通常の再 enqueue では `after_discard` を呼ばない。予算終了時は exhaustion block の後、または元例外を再 raise する前に呼ぶ。`discard_on` は個別 block の後に呼ぶ。よって `after_discard` が動いたことだけでは「backend も含め二度と実行されない」と証明できない。

個別 block が先に例外を投げれば、その後の `after_discard` へ到達しない経路がある。`after_discard` の各 callback が投げた StandardError は集められ、残りの callback も試みた後、最後の例外が raise される。通知・記録処理自身の失敗を、元の job 例外と区別して観測する必要がある。

`retry_on(report: true)` の報告呼出しは、実装では再試行する分岐の中にある。これだけを「すべての終端失敗を一度だけ報告する仕組み」と扱わない。

### jitter と待機時間

`:polynomially_longer` は実行回数の4乗を基礎にした多項式 backoff で、指数 backoff という名前に読み替えない。固定秒数/Duration と多項式には jitter を加算するが、Proc の戻り値にはこの実装から追加 jitter を掛けない。

`config.active_job.retry_jitter` は `config.load_defaults` の対象版に依存し、original は0.0、6.1以降は0.15。[Configuring Rails: retry_jitter](https://guides.rubyonrails.org/configuring.html#config-active-job-retry-jitter)。API説明の15%だけを見て、独立利用した Active Job や旧設定を持つ upgrade 済みアプリにも同じ実効値があると推定しない。job に指定した `jitter` と起動後の設定を確認する。予定の待機時間は queue の混雑まで含む実行開始 SLA ではない。

## 実装方針（独自の設計案）

1. 一過性の接続障害、入力不備、期限切れなどを分類し、回復の見込みがある例外だけ retry する。広い例外の handler を追加するときは既存の具体的 handler の探索順を確認する
2. Active Job と backend の両方の retry 設定を一枚の運用記録に書き、最長経過時間と終端保管先を決める。handlerごとの `attempts` を全体予算の代わりにしない
3. 外部副作用の後に例外が起こる試験を入れ、同じ job が再実行されても業務結果が重複しない識別子・状態遷移を用意する。retry handler 自体はこの冪等性を提供しない
4. exhaustion block で例外を処理し終えるなら、誰が未完了の業務を拾うかまで決める。通知だけして正常終了すると、その後の失敗保管が行われるという前提は置けない
5. 削除済みレコードの `DeserializationError` は、再生成不要な仕事なら discard 候補。ただし常に破棄してよいかは業務上の意味で決める。監査や復旧が必要な仕事を単に消さない
6. error reporter、`after_discard`、backend の失敗記録を別々に観測し、job ID と試行回数で相関する。通知回数を処理回数とみなさない

## 受入試験の案（今回未実行）

- 同じ handler だけで失敗し続ける job に `attempts: 1` と `attempts: 5` を指定し、再 enqueue 回数と最終例外を確認する
- 同一宣言の二種類の例外を交互に発生させ、共有予算を確認する。別宣言へ分けた場合も比較する
- 親 job の広い例外と子 job の具体的例外、同じクラス内の宣言順を組み合わせ、意図した handler が選ばれるか確認する
- exhaustion block なし/正常終了/例外発生、および `after_discard` callback の失敗を分けてテストする
- `jitter: 0` で多項式の基礎待機時間を確認し、Proc に別途 jitter が追加されないことを対象版で確認する
- 本番と同じ adapter で、再 enqueue 後・業務副作用後の worker 障害と再配信を試す。検索 eval や test adapter の成功だけを配送保証にしない

## 未確認事項・出典と鮮度

Sidekiq・Solid Queue など各 backend の再試行回数、失敗保管期間、exactly-once の保証は今回調べていない。8.1.4の遅延enqueue修正が個別アプリで期待どおりに働くかは実行検証していない。Ruby実行による挙動テストは未実施で、固定した実装と公式文書の読み合わせに基づく。

API と Active Job 実装の MIT、Guides の CC BY-SA 4.0 を確認した。release記事のライセンスは未確認。コード・例文は転載せず、独自の説明と試験案だけを保存する。公開日表示のない資料は取得日2026-10-01を記録し、release source の TTL 30日が最短のため2026-10-31に再確認する。

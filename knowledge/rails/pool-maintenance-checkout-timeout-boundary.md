---
{
  "id": "rails-pool-maintenance-checkout-timeout-boundary",
  "title": "Rails 8.1.4 connection pool: maintenance 待機と checkout_timeout の境界",
  "kind": "knowledge",
  "technology": "rails",
  "version": "Rails v8.1.4 (2026-09-24), Active Record @ c3466ea00d7121798e3aa3144ffdf7174b81d8cb; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:backend",
    "rails",
    "activerecord",
    "connection-pool",
    "checkout_timeout",
    "keepalive",
    "maintenance",
    "timeout-budget",
    "backpressure"
  ],
  "sources": [
    {
      "id": "rails-release-pool-maintenance-8-1-4-20261003",
      "url": "https://github.com/rails/rails/releases/tag/v8.1.4",
      "type": "release_notes"
    },
    {
      "id": "rails-pool-api-8-1-4-20261003",
      "url": "https://api.rubyonrails.org/v8.1.4/classes/ActiveRecord/ConnectionAdapters/ConnectionPool.html",
      "type": "official_docs"
    },
    {
      "id": "rails-pool-maintenance-c3466ea0-20261003",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/connection_pool.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-pool-queue-clock-c3466ea0-20261003",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/connection_pool/queue.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-keepalive-config-c3466ea0-20261003",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/database_configurations/hash_config.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-pool-maintenance-tests-c3466ea0-20261003",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/test/cases/connection_pool_test.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-pool-reaper-c3466ea0-20261003",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/connection_pool/reaper.rb",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Rails 8.1.4 connection pool: maintenance 待機と checkout_timeout の境界

## 問いと結論

DB接続の取得が設定した `checkout_timeout` より大幅に遅いとき、pool不足だけを疑ってよいか。Rails 8.1.4 は background maintenance の後ろで待つ経路の固定100秒を修正した。ただし、maintenance優先の取得順は変えていない。poolに増設余地があっても待機し、そこでtimeoutになり得る。修正を「request全体・SQL・接続確立を同じ期限で中断する仕組み」と扱わない。

[8.1.4 release](https://github.com/rails/rails/releases/tag/v8.1.4) は2026-09-24公開。keepalive pingなどが止まるとcallerの設定より長く待つ問題と、`database.yml` の `keepalive: false` が既定600秒へ戻る問題を修正したと記載する。以下は公式API、固定commitの実装観察、独自の運用案を分ける。一般的なtransaction・MySQL read-back lockは対象外。

## 何の時間を設定するか（公式API）

[v8.1.4 ConnectionPool](https://api.rubyonrails.org/v8.1.4/classes/ActiveRecord/ConnectionAdapters/ConnectionPool.html) が示す意味は次のとおり。

- `checkout_timeout`: 利用可能な接続を待つ秒数。既定5秒
- `keepalive`: idle接続にkeepalive確認を行う間隔。既定600秒
- `idle_timeout`: 未使用接続をpool内に保持する秒数。既定300秒。0はidleによる切断をしない設定

keepaliveの間隔と、ping自体のネットワーク処理の上限時間は別物である。`keepalive: 10` を「接続取得やDB応答を10秒で打ち切る」と読まない。後述の実装もmaintenance作業そのものへ `checkout_timeout` を渡していない。

## 修正後もmaintenanceの返却を優先する（固定版の実装観察）

[ConnectionPoolの実装](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/connection_pool.rb) の `acquire_connection` / `try_to_queue_for_background_connection` を確認した。内部methodは将来互換を保証するpublic APIではない。

1. 通常の取得では、即座に取れるavailable接続、background maintenance接続の返却待ち、新しい接続の作成、の順に試す
2. maintenance待ちは `@maintaining > 0` で、poolのlock内でも `@maintaining > @available.num_waiting` のときに選ばれる。maintenance中の本数より既に待っている人数が少ない場合、返却される接続を待つ
3. この経路はv8.1.4で `@available.poll(checkout_timeout)` を呼ぶ。public `checkout` は既定のpool設定、またはcallerが渡した引数を `acquire_connection` へ渡す。reap後の再試行経路も同じ引数を使う
4. maintenance待ちで `ActiveRecord::ConnectionTimeoutError` になれば、pool情報を設定してraiseする。その例外を握りつぶして新規作成へfallbackする経路ではない
5. どの候補も使えなければreapし、候補を再確認し、最後は通常のavailable queueで待機する。maintenanceが存在するだけで全callerを必ず同じ待機へ送るわけではない

したがって、`connections < max_connections` だけでは即時取得できると判断できない。maintenance待ちの条件を満たせば、新規作成より先にそこで待つ。pool増設でこの取得順そのものは変わらない。これらは固定版の制御フローからの帰結であり、特定アプリでtimeoutが発生する頻度は測定していない。

## checkout_timeoutはend-to-end deadlineではない（実装からの限界）

[Queueの実装](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/connection_pool/queue.rb) は `poll` でpoolのlockを取ってから `wait_poll` に進む。`wait_poll` 内で `CLOCK_MONOTONIC` を起点にし、condition waitへ残時間を渡す。queueが空のままelapsedがtimeout以上になると例外にし、待機者数はensureで戻す。

このclockの範囲と [checkoutの処理](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/connection_pool.rb) から、次の境界を区別する。

- lock取得から、接続の準備、取得後の処理、SQL完了までを包む単一deadlineではない。queue内の待機計測より前のlock取得時間は、そのclockに含まれない
- `try_to_checkout_new_connection` や取得後の `checkout_and_verify` に、このqueue待機用の残り時間を渡す設計ではない。アプリのrequest全体をこの設定だけで時間制限できるとはいえない
- pinned connectionの別経路はlockと `verify!` を使う。通常queueの修正をあらゆる内部checkout経路へ一般化しない
- この変更は、停止したmaintenance自体をcancelする処理ではない。`sequential_maintenance` は作業を呼び、戻る・例外になるとensureで接続を返す。待っているcallerのtimeoutと、maintenanceの終了は別である
- OS schedulingやlock再取得を含め、設定値ちょうどのwall-clock時刻に必ず戻るというreal-time保証ではない。queueのループはwake後に接続の有無を先に確かめるため、厳密な全経路deadlineの証拠として使わない

独自の設計案として、requestの時間予算、pool待機、adapter/driverの接続・read/write timeout、DBのstatement実行制限を分けて設計する。具体的なoption名とcancel後の接続再利用可否はadapter・driver・DB別の調査が必要であり、本稿では検証していない。無条件にRubyの非同期例外で強制中断する対策は、この調査から推奨できない。

## keepalive:falseの修正で変わる範囲（固定版の設定解析）

[HashConfig#keepalive](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/database_configurations/hash_config.rb) のv8.1.4分岐は次のとおり。

| 設定値 | 解析結果 | 意味 |
|---|---|---|
| boolean `false` | `nil` | keepalive無効 |
| 未指定・`nil`・boolean `true` | `600.0` | 既定間隔 |
| `to_f` 後に正の数となる値 | その値 | 指定間隔 |
| `0`・負数など `to_f` 後に正でない値 | `nil` | keepalive無効 |

`ConnectionPool#keep_alive` はthresholdがnilなら何もしない。`nil`を設定ファイルに書けば無効になる、という解釈は誤りである。HashConfigがnilを600秒へ解決することと、その後のpool内部でnilが無効を表すことを混同しない。環境変数由来の文字列やYAMLのquotingを含む全入力を検証したわけではないため、意図するboolean・数値の型と解決後の設定を確かめる。

無効化してもbackground maintenance全停止ではない。[Reaper](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/connection_pool/reaper.rb) はreap、flush、prepopulate、retire_old_connections、keep_alive、preconnectを順に呼ぶ。keepalive無効化だけで接続維持の全処理や待機がなくなるとはいえない。接続を長く保持する必要性と、DB・中間network機器のidle切断条件を確認して選ぶ、というのが独自の運用案である。

## upstreamの回帰testから確認できること

[connection_pool_test.rb](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/test/cases/connection_pool_test.rb) を読解した。

- `test_checkout_queues_behind_maintenance_connections` は `max_connections: 3` のpoolに実接続2本を用意し、一方をmaintenance中にする。別threadのcheckoutが待機し、maintenance解放後に同じ接続を受け取ること、第三の接続を作らないことをassertする
- `test_checkout_queued_behind_maintenance_respects_checkout_timeout` は `checkout_timeout: 0.1`、`reaping_frequency: nil`、`async: false` でmaintenanceをEvent待ちに固定し、checkout側が `ConnectionTimeoutError` になることを確認する。`join(1)` は旧100秒待ちならtestが長くhangしないための上限である
- 後者を「厳密に0.1秒以内で完了することを計測したtest」と要約しない。回帰testのassertは例外と1秒のjoin上限であり、全adapter・全scheduler・全network故障の時間保証ではない

これはupstreamのtestコードを読んだ結果で、この環境でRuby・DBを起動して実行していない。KB検索evalは記録の発見性を検査するもので、並行性や実際のtimeoutの検証を代替しない。

## 移行と運用のチェックリスト（独自案）

1. Rails/Active Record、adapter、driver、DBの実版を保存し、8.1.4の修正が導入されたartifactか確認する。旧版全体の影響範囲や他seriesへのbackportはここでは未確認
2. 「全接続がアプリに貸出中」と「返却可能だった接続がmaintenance中」を分けて再現する。poolに空き枠があるcaseも含める
3. maintenance作業の開始・解除をbarrierで制御し、通常成功、停止したままのtimeout、解除後の次回取得を別caseで確かめる。sleepだけで競合したとみなさない
4. pool設定の値に加え、public `checkout` に個別timeoutを渡すcall siteを調べる。queueが採用したtimeoutとrequest全体のelapsedを別々に記録する
5. `ConnectionTimeoutError` を即時無制限retryしない。残ったrequest予算とDB障害時の負荷を考慮し、入口での拒否・待機数の制限・限定的retryを選ぶ
6. `keepalive: false` を使う箇所はboolean型と実効値を確認し、無効化後のidle切断・初回クエリの遅延・再接続も検証する。timeout回避だけを理由に全環境で無効化しない
7. `pool.stat` のconnections/busy/idle/waitingとcheckout elapsedを合わせて見る。固定版statにmaintenance専用項目はないため、その値だけで待機理由を断定しない。必要なmaintenance計測の追加方法はアプリで別途設計する
8. 手動checkoutにはcheckinの所有責任を残す。`release_connection` は手動checkoutの返却手段ではない。既存leaseがある `with_connection` も、そのblockが必ず新規取得・返却をするとは限らない。timeout修正と接続リークの対処を混同しない

## 出典・版・ライセンス・未確認事項

- 取得日は全sourceで2026-10-03 UTC。release公開日は2026-09-24。APIページ固有の公開日は表示されていない。以後のreleaseを未調査のまま「最新版」とは呼ばない
- 実装・testはv8.1.4のcommit `c3466ea00d7121798e3aa3144ffdf7174b81d8cb` に固定した。実装の観察を将来版のpublic API保証として扱わない
- 同commitの [activerecord/MIT-LICENSE](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/MIT-LICENSE) をGitHub connectorで確認した。コード・testを転載しておらず、moduleへの昇格も行っていない。release本文と版別APIページの個別ライセンス表示は直接確定できず、catalogではunknownとし独自要約に限定した
- releaseと版別ConnectionPool APIはWebで実際に開いた。固定commitの一部raw/blob、版別HashConfig API・APIトップのWeb取得は失敗した。固定版の実装・test・licenseはGitHub connectorで読解し、取得失敗したページを読めたと扱っていない
- network故障、各DB/driverのtimeout、async executorとfiber schedulerの組合せ、実アプリのrequest期限、Ruby別の時間精度は未検証。今回の限定的なqueue修正からend-to-end latencyの改善量を数値で主張しない

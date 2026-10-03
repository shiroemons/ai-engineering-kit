---
{
  "id": "rails-dynamic-route-removal-dispatch-boundary",
  "title": "Rails main の動的 route 廃止: 描画時 ArgumentError・MissingController・起動 warning の移行境界",
  "kind": "knowledge",
  "technology": "rails",
  "version": "Rails main 8.2.0.alpha at 14d6d42b21be8527dda803ad3e8c790471c2521e (merged 2026-09-30); compared with v8.1.4 at c3466ea00d7121798e3aa3144ffdf7174b81d8cb",
  "tags": [
    "research-domain:backend",
    "rails",
    "routing",
    "dynamic-segment",
    "controller",
    "action",
    "ArgumentError",
    "MissingController",
    "eager_load",
    "migration"
  ],
  "sources": [
    {
      "id": "rails-routing-news-20261002",
      "url": "https://rubyonrails.org/2026/10/2/this-week-in-rails",
      "type": "maintainer_article"
    },
    {
      "id": "rails-routing-pr58893-20261003",
      "url": "https://github.com/rails/rails/pull/58893",
      "type": "maintainer_article"
    },
    {
      "id": "rails-routing-merge-14d6d42b-20261003",
      "url": "https://github.com/rails/rails/commit/14d6d42b21be8527dda803ad3e8c790471c2521e",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-routing-version-14d6d42b-20261003",
      "url": "https://github.com/rails/rails/blob/14d6d42b21be8527dda803ad3e8c790471c2521e/RAILS_VERSION",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-routing-stable-8-1-4-20261003",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/actionpack/lib/action_dispatch/routing/route_set.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-routing-release-8-1-4-20261003",
      "url": "https://github.com/rails/rails/releases/tag/v8.1.4",
      "type": "release_notes"
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

# Rails main の動的 route 廃止と dispatch 境界

## 問いと結論

古い catch-all route を残したまま Rails の開発版へ進めるか。2026-09-30 に main へ入った変更では、URL から controller / action を選ぶ route は描画時に `ArgumentError` となる。HTTP request を受けたときの404だけを試す移行では足りない。route定義、controller class宛てのaction指定、middlewareの `Request#controller_class`、起動ログを別々に点検する。

[2026-10-02の公式週報](https://rubyonrails.org/2026/10/2/this-week-in-rails)が紹介した [PR #58893](https://github.com/rails/rails/pull/58893) を起点に調べた。これは Rails 8.1.4 のリリース変更ではない。既存の [connection poolの待機](pool-maintenance-checkout-timeout-boundary.md) や [MySQL read-back lock](create-or-find-mysql-shared-lock-boundary.md) と異なり、routeを読み込めるか・どのendpointへdispatchするかの互換性を扱う。

## 適用版: 古い警告の「9.0」と main の実装を分ける

- [merge commit](https://github.com/rails/rails/commit/14d6d42b21be8527dda803ad3e8c790471c2521e) は `14d6d42b21be8527dda803ad3e8c790471c2521e`。作成日時は2026-09-30 17:47:30 UTC。ここを本稿の実装観察の固定点とする
- 同commitの [RAILS_VERSION](https://github.com/rails/rails/blob/14d6d42b21be8527dda803ad3e8c790471c2521e/RAILS_VERSION) は `8.2.0.alpha`。この表示だけから8.2正式版の公開や最終仕様を保証しない
- 比較対象の [v8.1.4 release](https://github.com/rails/rails/releases/tag/v8.1.4) は2026-09-24公開。[同版の RouteSet#add_route](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/actionpack/lib/action_dispatch/routing/route_set.rb) は動的segmentを含むrouteを追加した後、非推奨警告を出す経路を持つ。その予告文は「Rails 9.0」だが、確認したmain snapshotではすでに削除されている

したがって「警告に9.0とあるから8.2系の検証は不要」と決めない。一方、mainの変更を根拠に「8.1.4へ更新すると必ずrouteを読めなくなる」とも言わない。実際に導入するgem/tag/commitを固定して比較する。一般公開guideやedgeguideは取得時点で当該節が表示されず、本文の挙動証明は固定commitとPRを優先した。取得日は2026-10-03 UTC。

## route描画時に拒否するもの（固定commitの観察）

[Mapper#normalize_options! / check_part](https://github.com/rails/rails/blob/14d6d42b21be8527dda803ad3e8c790471c2521e/actionpack/lib/action_dispatch/routing/mapper.rb) と [Action Pack CHANGELOG](https://github.com/rails/rails/blob/14d6d42b21be8527dda803ad3e8c790471c2521e/actionpack/CHANGELOG.md) に基づく。

1. `:controller` または `:action` がpathの動的segmentにあれば `ArgumentError`。両方そろっている場合に限定しない。controllerを固定して `"/reports/:action"` だけ残す形や、actionを固定して `"/:controller"` を残す形も対象
2. 判定は `to:` の種類を処理する前に行う。Rack endpointへ送るrouteでも、業務データとして `:action` というsegment名を使っていた場合は移行対象になる。通常のデータなら `:operation` 等の別名を検討し、参照するコードも合わせて変更する
3. controller classを `to:` に渡す場合、解決されたactionが必要になる。たとえば `get "reports/show", to: ReportsController, action: "show"` のように指定する。classを渡すだけでpath名や旧来の動的actionから補われると期待しない
4. controller / action の指定値が `Regexp` なら `check_part` が拒否する。これは `:id` など通常のpath parameterへの正規表現制約をすべて禁止する変更ではない
5. この拒否分岐は `config.load_defaults` を読んでいない。古いframework defaultsを維持するだけで動的routeを復活できる、という前提で移行しない

非推奨warningをsilenceしても、描画時の例外は消えない。`ArgumentError` をまとめて握りつぶして起動を続ける方法は、必要なrouteが欠けた状態を正常と扱うため避ける。以上は固定sourceの分岐の読解であり、独自routing拡張すべてへの互換保証ではない。

## request側: controller不在を仮の404 endpointと扱わない

[Request#controller_class](https://github.com/rails/rails/blob/14d6d42b21be8527dda803ad3e8c790471c2521e/actionpack/lib/action_dispatch/http/request.rb) は `path_parameters[:controller]` を使う。controller path parameterがない場合、以前のplaceholderを返す経路がなくなり `ActionDispatch::MissingController` をraiseする。CHANGELOGはrouting前のrequestやRack endpoint宛てrequestを例に挙げる。

運用上の推奨は、middleware・監視・ログ処理が「どのrequestにもcontroller classがある」と仮定していないか確認すること。query stringの `controller` を存在確認の代用にしない。controllerを必要とする処理をcontrollerへroutingされた後に置くか、controllerを持たないendpointの扱いを明示する。すべての `NameError` を404へ変換する設計に拡張しない。固定実装は対象controllerの解決失敗以外の `NameError` を再raiseする。

controller classが見つかることはactionが存在すること、認証済みであること、認可されたことの証明ではない。この変更を認可の代用にしない。

## 起動時のmissing-controller warningは別の境界

同mergeの [RoutesReloader](https://github.com/rails/rails/blob/14d6d42b21be8527dda803ad3e8c790471c2521e/railties/lib/rails/application/routes_reloader.rb) は、eager loading時にrouteを読み終えた後、missing controllerのmessageを `Rails.logger.warn` へ送る。[RouteSet#missing_controller_messages](https://github.com/rails/rails/blob/14d6d42b21be8527dda803ad3e8c790471c2521e/actionpack/lib/action_dispatch/routing/route_set.rb) と合わせると次の範囲になる。

- dynamic segmentによる描画時 `ArgumentError` と、明示routeが存在しないcontrollerを指すときのwarningは別の失敗
- `config.eager_load` が無効の開発環境だけで起動確認しても、同じwarning確認にはならない。loggerがない場合、この警告出力処理は戻る
- controllerにdispatchするrouteが対象で、Rack endpointや `StaticDispatcher` のcontroller class routeを同じ方法では調べない
- controllerを解決する確認であり、全actionの実在・全requestの成功を検査する処理ではない
- warningは自動的な起動失敗ではない。upstream testもmissing controllerのあるeager-load起動で例外が出ないことを確認してからログを照合する

したがって「起動終了コードが0なら全routeが正しい」「ログが静かなら全endpointが検証済み」とは結論しない。CIで警告を失敗扱いにするなら、それはアプリ側の明示的な運用ルールとして実装・試験する。

## 実務の移行順序（独自の設計案）

1. **現行版で棚卸しする。** `config/routes.rb`、分割routes、engine、test helperを調べる。`:controller` / `:action` の文字列検索で候補を集めるが、静的optionの `controller:` / `action:` まで一律削除しない。`bin/rails routes` の出力と利用中URLを比較する
2. **必要な公開面を決める。** catch-allをcontrollerの全public actionの自動公開へ置き換えず、意図したpath・HTTP method・controller・actionを列挙する。既存URLの維持が必要なら、そのURLを静的に定義する。たとえば旧 `/reports/export` を単純に `/reports/:id` へ置き換えると別の意味になり得る
3. **認識と生成を別々に試験する。** inbound routingだけでなく `url_for`、named route helper、メール内URL、optional id / format、namespaceを確認する。routeの順番やhelper名も固定し、不要なmethodと非公開actionへ到達しないnegative testを加える
4. **class宛てとRack宛てを分ける。** controller classにはactionを解決できる定義を与える。Rack routeの業務用segment名を変えた場合はrequest parameter利用側も試験する
5. **middlewareと起動を点検する。** controller不在のrequestを `MissingController` で意図せず500にしないか確認する。production相当のeager-load設定でmissing-controllerログを読み、個々のrequest/action testも行う
6. **2つの版で比較する。** 明示routeへの置換を先に現行安定版で確認し、その同じroute定義を導入予定commitで試す。失敗時はreview済みのroute変更と依存版の組合せへ戻す。削除済みの動的dispatchをmonkey patchで戻すことを既定のrollback手段にしない

## 確認したtestと未確認事項

[固定merge diff](https://github.com/rails/rails/commit/14d6d42b21be8527dda803ad3e8c790471c2521e) のtestを読んだ。`test_dynamic_controller_segments_are_not_supported` と `test_dynamic_action_segments_are_not_supported` は描画時の例外を期待する。RequestControllerClassのtestはcontrollerの正常解決、controller parameter不在、存在しないcontrollerを区別する。Railtiesのapplication routing testはeager loadingの有無とwarning、既存controllerへのrequest成功を分けている。これはupstream testの意図の読解であり、この調査でRailsのtestを実行したものではない。

実アプリ・engine・middlewareでの起動とHTTP応答、Ruby/gemの組合せ、release後の変更、全route DSLの組合せは未検証。正式8.2の公開日や最終removal仕様も未確認。検索evalは知識の発見性だけを検証し、Railsの互換性や本番可用性を保証しない。

## 出典とライセンス

Webで公式週報、PR本文、8.1.4 releaseを開き、GitHub connectorで上記の固定sourceとtest差分を読んだ。raw WebのCHANGELOG取得は失敗したためconnectorを用いた。main commitのroot / Action Pack / Railties、および比較対象8.1.4のAction PackのMIT-LICENSEを確認した。週報・PRコメント・releaseページはlicenseを推測せずunknownとして独自要約のみを記録。upstreamの実装・testは転載せず、短いroute例と移行案は本稿の説明用に作成した。

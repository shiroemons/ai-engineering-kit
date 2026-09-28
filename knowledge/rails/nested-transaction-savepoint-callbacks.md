---
{
  "id": "rails-nested-transaction-savepoint-callbacks",
  "title": "Rails Active Record のネスト・トランザクション: requires_new (savepoint) のロールバック範囲と after_commit / after_rollback の発火条件",
  "kind": "knowledge",
  "technology": "rails",
  "version": "Ruby on Rails 8.1.4 (API / Guides 表示 v8.1.4、activerecord @ c3466ea00d7121798e3aa3144ffdf7174b81d8cb)",
  "tags": [
    "research-domain:backend",
    "rails",
    "activerecord",
    "transaction",
    "nested-transaction",
    "savepoint",
    "requires_new",
    "rollback",
    "activerecord-rollback",
    "after_commit",
    "after_rollback",
    "callback",
    "joinable",
    "isolation",
    "mysql"
  ],
  "sources": [
    {
      "id": "rails-api-databasestatements-8-1-4",
      "url": "https://api.rubyonrails.org/classes/ActiveRecord/ConnectionAdapters/DatabaseStatements.html",
      "type": "official_docs"
    },
    {
      "id": "rails-api-transactions-classmethods-8-1-4",
      "url": "https://api.rubyonrails.org/classes/ActiveRecord/Transactions/ClassMethods.html",
      "type": "official_docs"
    },
    {
      "id": "rails-guides-active-record-callbacks-8-1-4",
      "url": "https://guides.rubyonrails.org/active_record_callbacks.html",
      "type": "official_docs"
    },
    {
      "id": "rails-impl-abstract-transaction-c3466ea0",
      "url": "https://raw.githubusercontent.com/rails/rails/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/transaction.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-impl-transaction-c3466ea0",
      "url": "https://raw.githubusercontent.com/rails/rails/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/transaction.rb",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-11-27",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/backend.json"]
}
---

# Rails Active Record のネスト・トランザクション: requires_new (savepoint) のロールバック範囲と after_commit / after_rollback の発火条件

Active Record のネストした `transaction` はデフォルトでは savepoint を作らず親トランザクションに参加するため、部分ロールバックは `requires_new: true` のときだけ起きる。`after_commit` は最外トランザクションのコミット後、`after_rollback` は「トランザクションまたは savepoint」のロールバック直後に発火する。以下は Ruby on Rails 8.1.4 の公式 API / Guides に書かれた事実、同じ 8.1.4 の実装（commit `c3466ea00d7121798e3aa3144ffdf7174b81d8cb`）で観察した挙動、そして独自の設計案を分けて記録する。

## 要点

### ネスト時のロールバック範囲（公式 API に記載された事実）

- ネストの `transaction` はデフォルトで全 SQL が親トランザクションの一部になる。[`DatabaseStatements#transaction`](https://api.rubyonrails.org/classes/ActiveRecord/ConnectionAdapters/DatabaseStatements.html) は、現在のトランザクションが joinable なら新規トランザクションを作らずブロックをそのまま実行すると説明し、`requires_new: true` の場合にのみブロックがデータベースの savepoint によるサブトランザクションで包まれるとする。
- [`Transactions::ClassMethods`](https://api.rubyonrails.org/classes/ActiveRecord/Transactions/ClassMethods.html) の Nested transactions 節も、ネストの `transaction` はデフォルトで全 SQL が親トランザクションの一部であり、`requires_new: true` で真のサブトランザクション（savepoint）を要求すると記載する。問題が起きたら親はロールバックせず、データベースが savepoint 開始点までロールバックする。
- `ActiveRecord::Rollback` は transaction ブロック内で捕捉されて再 raise されない（その他の例外は再 raise される）。ClassMethods の記載では、ネスト内で `ActiveRecord::Rollback` を raise しても親には伝播せず実トランザクションはコミットされる。公式の例では、ネスト内で `ActiveRecord::Rollback` を raise しても ROLLBACK されず両レコードがコミットされるが、`requires_new: true` なら親を巻き込まずサブトランザクション開始点までロールバックし「first」のみ残る。
- 本物のネストトランザクションをサポートするのは MS-SQL のみで、それ以外のデータベースは savepoint でエミュレートされる（同 API 記載）。
- 同 API の既知の制約: MySQL では savepoint 生成ブロック内で DDL（CREATE TABLE / TRUNCATE など）を実行すると savepoint が自動解放され、RELEASE SAVEPOINT がデータベースエラーになる。`isolation:` をネスト（savepoint）トランザクション内で指定すると `TransactionIsolationError` になる。

### after_commit / after_rollback の発火条件（公式 API / Guides に記載された事実）

- `after_commit` はトランザクションコミット直後、`after_rollback` は「トランザクションまたは savepoint がロールバックされた直後」に、そのトランザクション内で保存・削除された全てのレコードに対して呼ばれる（[ClassMethods](https://api.rubyonrails.org/classes/ActiveRecord/Transactions/ClassMethods.html) の Callbacks 節）。
- トランザクション完了時に、そのトランザクション内で create / update / destroy された全モデルに対して `after_commit` か `after_rollback` のどちらかが呼ばれる。[`after_save` との違い](https://guides.rubyonrails.org/active_record_callbacks.html)は、DB 変更がそれぞれコミット・ロールバックされた後まで実行されない点である。
- `save` / `destroy` は自動的にトランザクションで包まれ、`after_commit` がコミット後にのみ発火する唯一のコールバックだと ClassMethods に明記される。
- コールバックはフィルタ単位で重複排除され、`on:` オプションで発火対象（create / update / destroy）を限定できる。`after_create_commit` / `after_update_commit` / `after_destroy_commit` / `after_save_commit` は内部で `after_commit` にエイリアスされるため、同じメソッド名で複数の別名を定義すると重複排除されて最後の定義だけが残る。
- 同一トランザクションで同一レコードを複数回保存した場合、コールバックはそのレコードの最初の変更オブジェクトにのみ発火する（Guides §11.1）。
- `after_commit` 内で例外が起きてもデータは既に永続化しておりロールバックはされず、例外は伝播して残りの `after_commit` / `after_rollback` は実行されない。ガイドはコールバック内で rescue すべきと記載する。また `after_commit` / `after_rollback` 内で実行されるコード自体はトランザクションで囲まれない。
- トランザクション系コールバックの定義順実行は Rails 7.1 の既定（以前は逆順で、`config.active_record.run_after_transaction_callbacks_in_order_defined = false` で逆順に戻せる。Guides §11.3）。

### 実装で観察した挙動（activerecord @ `c3466ea00d7121798e3aa3144ffdf7174b81d8cb`、ドキュメント記載とは区別）

- [`SavepointTransaction#rollback`](https://raw.githubusercontent.com/rails/rails/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract/transaction.rb) は `rollback_to_savepoint` を実行した後 `rollback_records` を走らせる。つまり savepoint ロールバック時点でレコードの `rolledback!`（＝`after_rollback` の発火）が行われる。
- `SavepointTransaction#commit` は `release_savepoint` のみで、`TransactionManager#begin_transaction` は `run_commit_callbacks = !current_transaction.joinable?` でトランザクションを作る。親が joinable な savepoint では `commit_records` が `run_commit_callbacks == false` の分岐に入り、レコードを `connection.add_transaction_record` で親トランザクションへ移送する。`@callbacks` も同様に親へ `append_callbacks` で移送されるため、`after_commit` は最外の実トランザクションのコミットまで遅延する。
- `SavepointTransaction#full_rollback?` は false。savepoint のロールバックは親の実トランザクションを巻き戻さない。
- [`ActiveRecord::Transaction`](https://raw.githubusercontent.com/rails/rails/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/transaction.rb): `after_commit` はトランザクションが完全にコミットされた後に呼ばれる。現在のトランザクションがコミットされると親へコールバックが移送され、ロールバック時は破棄され、これが最外トランザクションに達するまで繰り返される。`after_rollback` はネストの連鎖が全て正常コミットされた場合は一度も呼ばれない。コールバックが例外を投げてもトランザクションはコミット済みのままで維持される。

これらを合わせると、デフォルトのネストには savepoint が無いためネスト単位のロールバックは存在せず、`ActiveRecord::Rollback` を投げてもロールバックを理由に `after_rollback` が発火することはない。`requires_new: true` の savepoint をロールバックした時点でその savepoint 内のレコードの `after_rollback` が発火し、savepoint がコミットしても `after_commit` はその時点では発火せず、最外トランザクションのコミットまで待つ。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する構成を指すものではない。ガイドが `after_commit` の用途として挙げるのは、データベーストランザクションの一部ではない外部システムとの連携である。

- デフォルトは親トランザクションへの参加と決め、切り離したい処理にだけ `requires_new: true` を付ける（設計判断: ロールバック範囲をコードで明示するため）。
- 外側のトランザクション全体を止めたいときは `ActiveRecord::Rollback` ではなく例外を送出する。`ActiveRecord::Rollback` はネスト内で捕捉されて伝播せず、その他の例外は再 raise される（公式 API 記載からの運用上の解釈）。
- 外部への副作用（メール送信、ジョブ投入、外部 API 呼び出し）は `after_commit` に置き、ロールバック時にだけ走らせたい後始末は `after_rollback` に分離する。
- `after_commit` / `after_rollback` 内では例外を rescue して扱い、残りのコールバックを止めない（Guides の記載に同じ）。
- コールバックの発火回数は「トランザクション単位・フィルタ単位・最初の変更オブジェクト単位」で数える前提で設計し、保存回数に比例した発火を期待しない（公式記載からの設計上の含意）。

## 避ける使い方

- ネスト内で `ActiveRecord::Rollback` を raise すれば親トランザクションも巻き戻ると信じる。デフォルトのネストではネスト内で捕捉され、親はコミットされる。
- `requires_new: true` を付けずに savepoint による部分ロールバックを期待する。デフォルトは savepoint を一切作らない。
- `after_commit` を「保存のたびに毎回呼ばれる `after_save` の延長」とする。同一トランザクションで同一レコードを複数回保存すると最初の変更オブジェクトにのみ発火し、フィルタ単位の重複排除も効く。
- `after_create_commit` と `after_update_commit` に同じメソッド名を使う。内部エイリアスで重複排除され最後の定義だけが残る（両方で使いたい場合は `after_save_commit`）。
- `after_commit` 内の例外でロールバックされると考える。データは永続化済みで、例外は伝播して残りのコールバックだけが止まる。
- `after_commit` / `after_rollback` 内をトランザクションだと考える。その中で実行されるコード自体はトランザクションで囲まれない。
- ネストの savepoint トランザクション内で `isolation:` を指定する（`TransactionIsolationError`）。
- MySQL で savepoint 生成ブロック内で DDL を実行する（savepoint 自動解放により RELEASE SAVEPOINT がデータベースエラーになる）。

## 適用版と本番での注意

- Ruby on Rails 8.1.4 で確認した。API / Guides は v8.1.4 の表示、実装は 8.1.4 リリース準備コミット `c3466ea00d7121798e3aa3144ffdf7174b81d8cb`（2026-09-24、`Preparing for 8.1.4 release`）のソース。将来の最新とは扱わない。
- トランザクション系コールバックの実行順は Rails 7.1 で既定が定義順に変わった。それ以前の版では逆順だった点はバージョンをまたぐ移行時に注意する。
- 本物のネストトランザクションは MS-SQL のみ、他は savepoint エミュレーションという前提で DB 依存の制約（MySQL の DDL、`isolation:` 指定）が効く。
- 「実装で観察した挙動」節は `c3466ea00d7121798e3aa3144ffdf7174b81d8cb` のソースコード読解であり、公式 API / Guides の契約記述とは区別する。振る舞いの契約に必要な場合は公式文書の節を正とする。
- savepoint を作ることの性能影響や、joinable を変えるその他のオプションの挙動は本調査の範囲外で未検証。
- 本文の「推奨方法」節は独自の設計案であり、公式契約との境界は各節の書き出しで区別している。

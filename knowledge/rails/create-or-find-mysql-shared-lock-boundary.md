---
{
  "id": "rails-create-or-find-mysql-shared-lock-boundary",
  "title": "Rails 8.1.4 create_or_find_by: MySQL の共有 read-back lock と後続更新の境界",
  "kind": "knowledge",
  "technology": "rails",
  "version": "Rails v8.1.4 (2026-09-24), activerecord @ c3466ea00d7121798e3aa3144ffdf7174b81d8cb; MySQL 8.4 locking-read contract; retrieved 2026-10-02 UTC",
  "tags": [
    "research-domain:backend",
    "rails",
    "activerecord",
    "create_or_find_by",
    "find_or_create_by",
    "mysql",
    "unique-constraint",
    "shared-lock",
    "deadlock",
    "repeatable-read"
  ],
  "sources": [
    {
      "id": "rails-release-create-find-lock-8-1-4-20261002",
      "url": "https://github.com/rails/rails/releases/tag/v8.1.4",
      "type": "release_notes"
    },
    {
      "id": "rails-relation-create-find-c3466ea0-20261002",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/relation.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-mysql-create-find-lock-c3466ea0-20261002",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract_mysql_adapter.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-create-find-regression-c3466ea0-20261002",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/test/cases/relations_test.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "mysql-locking-reads-8-4-20261002",
      "url": "https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Rails 8.1.4 create_or_find_by: MySQL の共有 read-back lock と後続更新の境界

## 問いと結論

同じ業務キーを複数 worker が同時に作成するとき、Rails 8.1.4 の MySQL deadlock 修正で何を安全とみなせるか。結論は「重複 INSERT 後に既存行を読み直す特定の競合は改善するが、取得した行の更新まで排他に実行する契約ではない」。DB unique 制約、検索属性、validation、取得後の更新を別々に設計する。

[8.1.4 release](https://github.com/rails/rails/releases/tag/v8.1.4) は2026-09-24公開。Active Record が MySQL transaction 内で重複 INSERT 後に行う read-back を共有lockへ変更し、REPEATABLE READ の可視性を維持しつつ、競合する共有lockから排他lockへの昇格を避けたと記載する。対象は `find_or_create_by` / `create_or_find_by` と両方の bang 版。この文書では公式の説明、固定commitの実装観察、独自の設計案を分ける。

## 確認した呼出し経路（8.1.4の実装観察）

以下は [Relation のコメントと実装](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/relation.rb) を読んだ結果。内部method名は将来の互換性を保証するAPIではない。

1. `find_or_create_by` は先に `find_by` を行う。既存行が見つかれば、その結果をそのまま返し、重複 INSERT 後のlock経路には入らない。見つからなければ `create_or_find_by` に委譲する。bang 版も同じ構造。
2. `create_or_find_by` は `transaction(requires_new: true)` 内で `create` を試す。通常の作成成功と unique 違反時の既存行取得は異なる経路になる。validation等で作成が不成立の場合も、既存行へ必ずフォールバックするわけではない。
3. `ActiveRecord::RecordNotUnique` を捕捉した後、`connection.transaction_open?` が真なら（開いている transaction があれば） `rewhere(attributes)` した relation に `connection.create_or_find_by_lock` を適用し `take!` で読み直す。開いた transaction がなければ `find_by!(attributes)` に進む。常にlocking readになるわけではない。
4. [AbstractMysqlAdapter](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/abstract_mysql_adapter.rb) の `create_or_find_by_lock` は、MariaDB または MySQL 8.0.1 未満では `LOCK IN SHARE MODE`、それ以外では `FOR SHARE` を選ぶ。コメントは、重複 INSERT が得た共有record lockを昇格させず、REPEATABLE READでcurrent readを行う意図を明示する。

最後の分岐はRails内部の互換SQL選択であり、MySQL/MariaDB全版のサポート一覧や動作確認結果ではない。`create_or_find_by_lock` は `nodoc` の内部methodなので、アプリ設定として直接呼ぶ・monkey patchする推奨ではない。

## 共有lockで変わること、残ること（公式契約とその含意）

[MySQL 8.4 Locking Reads](https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html) では、FOR SHARE中も他sessionの読取りは可能だが、対象行の変更は制限される。FOR UPDATEは更新用の排他lockを取る。これらのlockはtransactionのcommit/rollbackで解放される。同manualは、複数sessionが共有lockで同じcounterを読んだ後に更新するとdeadlockになり得ると説明する。

これをRailsの変更へ適用すると、複数呼出しが同じ行を取得できることと、その後の業務更新を直列化できることは別である（独自の設計上の含意）。read-back成功後に両者が同じ行を更新すれば、後続更新でlock昇格競合が再び起き得る。修正の説明を「MySQL transactionのdeadlockを全面的に解消」「find-or-createから更新まで一人ずつ実行」と読み替えない。

特に、取得後に単に `.lock!` を足せば全競合を解消できるという設計にはしない。既に共有lockを持つ複数transactionが同じ排他lockを要求する状況を、その追加処理自体が作り得る。必要な業務排他は、重複作成を始める前の順序・更新方式・transaction単位の再試行可能性まで含めて設計する。これは推奨設計であり、この調査で特定の排他方式を実装・検証したわけではない。

## unique 制約と検索属性の境界（公式APIコメントの事実）

[固定版 Relation](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/relation.rb) は次の制限を記載する。

- DB側に関連列の unique 制約が必要。`find_or_create_by` の最初のSELECTとINSERTは不可分ではなく、制約がなければ重複行を作り得る。アプリの先行検索だけを唯一性の根拠にしない。
- INSERTが衝突した列がattributesの一部だけの場合、その後の検索条件すべてに一致する行がなく `ActiveRecord::RecordNotFound` になることがある。制約違反した行が必ず返るAPIではない。
- INSERT失敗後、読取りまでの間に他clientがDELETEすれば `INSERT → DELETE → SELECT` の競合は残る。新しい共有lockが存在するから、すべての呼出しが必ず既存行を返すとはいえない。
- unique列の uniqueness validation が先に失敗すると、DBの `RecordNotUnique` を経由せず、既存行取得に進まない。非bang版はvalidation失敗時に未永続化objectを返し得る。bang版は無効なrecordの作成で例外となる。共有lock修正はvalidation失敗を成功へ変換しない。

### 呼出しをどう組み立てるか（独自の設計案）

- まず業務上の同一性を決める。例として `(account_id, external_key)` を一意にするなら、その組合せに対応するDB制約と検索条件を揃える。更新可能な `display_name` まで同一性の判定に混ぜない。部分index、NULL、照合順序、式indexを使う場合はこの単純な例の外で別途検証する。
- 初回だけ埋める属性は、検索キーとは別に作成block等で設定する設計を検討する。これは競合時に既存行を更新する操作ではない。既存行に新しい名前が反映されたと誤認しない。
- `persisted?` や発生例外を確認してから後段の処理へ進む。既存モデルのvalidationを一括削除するという移行ではなく、validationを必要とするUI経路と競合作成経路が何を返すかを決める。
- 行が返ったことを「このworkerが新規作成した証拠」や「副作用を一度だけ実行してよい権利」として使わない。メール、課金等の副作用の一度性は、このメソッドの戻り値だけでは証明できない。
- 新規作成が多いか、既存行の取得が多いかも選択条件にする。どちらのメソッドを選んでもDB制約は必要で、常にcreate-firstへ置換すれば正しくなるという移行ではない。

## 移行・検証のチェックリスト（独自案）

1. Rails/Active Record、adapter、DBの実版と分離levelを記録する。Rails 8.1.4で修正を確認した。以前の全版の影響範囲、他seriesへのbackportの有無は未調査なので、版の大小だけで断定しない。
2. 本番と同じ unique 制約で、独立したconnectionを使う複数workerの競合を再現する。単一connectionで直列に2回呼ぶだけのtestでは、このread-back競合を検証できない。
3. 「winnerが先にcommitし、二つのloserが重複INSERTで失敗してから同時に読み直す」条件を作る。最初のSELECTで未存在を読み終えた古いsnapshot、REPEATABLE READ / READ COMMITTED、外側transactionとsavepointの有無を分ける。
4. 新規1件・同じIDの取得だけでなく、取得後の同一行更新も別caseにする。共有read-backで改善したcaseと、後続更新で依然競合するcaseを分離して計測する。
5. 非一意属性の不一致、uniqueness validationによる早期失敗、読取り前DELETEを別caseにし、`RecordNotFound`・validation例外・未永続化objectを成功扱いしていないかを確認する。
6. transactionの再試行を設計するなら、再実行可能な業務範囲、上限、外部副作用を先に決める。無制限retryや、DB transactionだけを巻き戻して既に行った副作用を繰り返す構成は避ける。

### upstreamテストで確認できた範囲

[8.1.4 relations_test](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/test/cases/relations_test.rb) のコードには、Mysql2Adapter / TrilogyAdapter用の `three_concurrent_creates` がある。winnerのcommitを待つ二つのthreadと、重複INSERT後のbarrierで競合を合わせる。`find_or_create_by` とbang版について `repeatable_read` / `read_committed` のcase、非bang版のsavepoint caseを持ち、既存行と同じ結果・最終行数1件をassertする。savepoint caseでは後続の別キー作成も確認する。

これはupstreamテストの読解結果であり、この環境でRuby・MySQLを起動して実行した結果ではない。全4メソッド×全分離level×全adapter×同一行の後続更新を網羅したと主張しない。アプリ独自callbackやschemaを含む結果は未検証。

## 出典・版・ライセンス・未確認事項

- Rails releaseの公開日は2026-09-24。取得日は全sourceとも2026-10-02 UTC。release本文は個別ライセンス適用を確定していないため、事実を独自の文章で要約した。
- 実装・コメント・testはRails v8.1.4に対応するcommit `c3466ea00d7121798e3aa3144ffdf7174b81d8cb` へ固定。同commitの [activerecord/MIT-LICENSE](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/MIT-LICENSE) を確認した。コードは転載せず、moduleへの昇格も行わない。
- MySQLは8.4 manualの当該節を確認し、prefaceは8.4.11 LTSと表示していた。ページ固有の公開日は表示されていない。[Oracleの文書利用条件](https://dev.mysql.com/doc/refman/8.4/en/preface.html#legalnotice) はGPLではないため、SQL例・原文の転載をせず、限定的な事実要約だけを使用した。旧MySQL/MariaDB版へmanualの記述を一般化しない。
- GitHubの一部raw/blob URLはWeb取得で失敗したため、固定commitの実装とライセンスはGitHub connectorで取得した。公開releaseページ、APIページ、MySQL本文はWebで実際に開いた。取得失敗を内容確認済みとして扱っていない。
- 性能差、deadlock発生率、第三者adapter、別DB、認可scope、部分unique制約、アプリの再試行実装は未検証。一般的なsavepoint/callback契約は既存文書で扱っており、この文書は重複作成後のread-backと後続更新の判断に限定する。

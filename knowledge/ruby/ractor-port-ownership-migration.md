---
{
  "id": "ruby-ractor-port-ownership-migration",
  "title": "Ruby 4.0 Ractor::Port 移行: 受信所有者・終了通知・copy/move とバックプレッシャー",
  "kind": "knowledge",
  "technology": "ruby",
  "version": "Ruby 4.0 RDoc (2026-10-02確認、patch版非表示); 変更導入 Ruby 4.0.0 (2025-12-25); 4.0.7公開 (2026-09-15) のみ別途確認",
  "tags": [
    "research-domain:backend",
    "ruby",
    "Ractor",
    "Ractor::Port",
    "migration",
    "ownership",
    "Ractor.select",
    "Ractor#value",
    "Ractor::ClosedError",
    "Ractor::MovedError",
    "Ractor::RemoteError",
    "backpressure",
    "experimental"
  ],
  "sources": [
    {
      "id": "ruby-ractor-release-4-0-0-20261002",
      "url": "https://www.ruby-lang.org/en/news/2025/12/25/ruby-4-0-0-released/",
      "type": "release_notes"
    },
    {
      "id": "ruby-ractor-release-4-0-7-20261002",
      "url": "https://www.ruby-lang.org/en/news/2026/09/15/ruby-4-0-7-released/",
      "type": "release_notes"
    },
    {
      "id": "ruby-ractor-port-4-0-docs-20261002",
      "url": "https://docs.ruby-lang.org/en/4.0/Ractor/Port.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-ractor-4-0-docs-20261002",
      "url": "https://docs.ruby-lang.org/en/4.0/Ractor.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-ractor-design-4-0-docs-20261002",
      "url": "https://docs.ruby-lang.org/en/4.0/language/ractor_md.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-copying-4-0-20261002",
      "url": "https://docs.ruby-lang.org/en/4.0/COPYING.html",
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

# Ruby 4.0 Ractor::Port 移行: 受信所有者・終了通知・copy/move とバックプレッシャー

## 解く問いと適用範囲

Ruby 3.x の Ractor worker pool を 4.0 系へ移すとき、逐次結果の受信、worker 終了、データの所有者をどう分けるか。既存の Fiber scheduler 文書が扱う非ブロッキング IO とは別の、Ractor 間メッセージ通信の互換性と終了処理を扱う。

対象は 2026-10-02 UTC に読んだ Ruby 4.0 の公式 RDoc。変更の導入は [Ruby 4.0.0 の発表](https://www.ruby-lang.org/en/news/2025/12/25/ruby-4-0-0-released/)（2025-12-25）で確認した。これは新着リリースの紹介ではなく、未収録だった破壊的変更の整理である。[Ruby 4.0.7](https://www.ruby-lang.org/en/news/2026/09/15/ruby-4-0-7-released/) は 2026-09-15 公開の bugfix release と確認したが、RDoc の URL は patch 版を表示せず、4.0.7 実行時の全挙動を検証したという意味ではない。

## 確認した変更と API 契約

### 1. 旧 API を機械的に置換しない

4.0.0 の発表は `Ractor.yield`、`Ractor#take`、`Ractor#close_incoming`、`Ractor#close_outgoing` の削除を記載している。新しい `Ractor::Port` が継続的なメッセージ交換を担い、`Ractor#join` / `Ractor#value` は Ractor の終了待ちに使う。`Ractor.select` の引数は Ractor または Port となり、Ractor を渡した場合は終了を待つ。このため、結果を何度も yield していた処理の take を value に置換するだけでは、途中結果を受信する設計にならない。

4.0.0 の発表は Ractor を experimental としており、取得した 4.0 RDoc の `Ractor.new` にも experimental 警告が残る。安定 API 化や性能向上の保証とは扱わない。

### 2. Port の受信所有者と close

[Port API](https://docs.ruby-lang.org/en/4.0/Ractor/Port.html) で確認した契約:

- `Ractor::Port.new` でその Port を作成した Ractor だけが、`receive` と `close` を実行できる。他の Ractor へ Port を渡しても受信権・閉鎖権は移らない
- `send`（別名 `<<`）は受信者の準備を待たず戻る。`receive` は開いている queue が空なら呼出元 Thread を待機させる
- close 後の send は `Ractor::ClosedError`。close 済みでも未受信メッセージは取り出せ、空になった後の receive が同例外になる

[Ractor API](https://docs.ruby-lang.org/en/4.0/Ractor.html) では、各 Ractor の `default_port` に `Ractor#send` と `Ractor.receive` が委譲される。`Ractor#close` も default port を閉じるため、その Ractor 自身から呼ぶ必要がある。親から worker の受信口を自由に close できる契約ではない。

[Ractor 設計文書](https://docs.ruby-lang.org/en/4.0/language/ractor_md.html) は incoming queue を無制限とし、Ractor 終了時にはその Ractor の Port が自動で閉じると説明する。send の成功は処理完了や消費済みの確認にはならない。

### 3. copy / move / shareable を別々に選ぶ

[Port API](https://docs.ruby-lang.org/en/4.0/Ractor/Port.html) の `send(obj, move: false)` は、shareable な値なら参照を送り、非 shareable な部分は既定で deep clone する。[設計文書](https://docs.ruby-lang.org/en/4.0/language/ractor_md.html) では `move: true` が所有者を移し、送信元からの再アクセスは `Ractor::MovedError` になる。copy や move に対応しないオブジェクトもあるため、任意の gem オブジェクトを送れるとは限らない。

[Ractor API](https://docs.ruby-lang.org/en/4.0/Ractor.html) に従い、外側だけの freeze を shareable の判定に使わない。入れ子に非 shareable な値があれば `Ractor.shareable?` は false になり得る。`Ractor.make_shareable` は参照先も freeze するため、元の設定オブジェクトを引き続き変更したいなら `copy: true` の必要性を検討する。`Ractor.new` の block は外側のローカル変数を捕捉できず、引数として明示的に渡した値にも共有・コピー規則が適用される。

### 4. メッセージと終了値を混同しない

[Ractor API](https://docs.ruby-lang.org/en/4.0/Ractor.html) の `Ractor.select` は Port を待った場合 `[port, message]`、Ractor を待った場合 `[ractor, termination_value]` を返す。`join` は終了まで待って Ractor 自体を返し、`value` は終了値を取得する。失敗終了は呼出側に伝わる。終了値を受け取れる Ractor は最大一つであり、複数の監督 Ractor が各々 value を受け取る設計にはしない。

[設計文書](https://docs.ruby-lang.org/en/4.0/language/ractor_md.html) の例外経路は `Ractor::RemoteError` の `cause` と `ractor` を示す。同文書のスレッド安全性の説明は Ractor 間の境界についてであり、一つの Ractor 内で複数 Thread が可変データを扱う場合の同期は別途必要になる。

## 実務での移行判断（独自の設計案）

以下は上記契約から導くアプリケーション側の方針であり、Ruby が提供する worker pool、配送保証、キャンセル API ではない。

1. 結果を集約する Ractor が result Port を作り、worker にその送信先を渡す。job ID と結果状態をメッセージに含め、業務結果と worker の終了を別イベントとして集計する
2. 無制限 queue に投入し続けず、同時実行中の job 数に上限を置く。完了 ACK を受けるたび次を投入するなど、バックプレッシャーを送信側で実装する。worker 死亡時にも slot が解放または失敗確定する経路を用意する
3. 停止要求はアプリケーションの stop メッセージとして送る。worker 自身が受信口の close と残仕事の扱いを決め、集約側は必要な結果を drain してから join/value で終端を確認する。先に終了通知を受けた場合にも未回収の job を成功扱いしない
4. 不変設定は事前に shareable 判定する。可変 job は copy を出発点とし、大きい payload の move は再アクセス不要という所有権設計が成立するときに限る。move 後のログ出力や再送に備える情報は別の小さな識別子として持つ
5. 結果 Port と worker Ractor を一緒に select する監督 loop では、先頭要素の種類で処理を分ける。終了済み worker は監視対象から外し、業務結果の数と終了イベントの数を合算しない

`closed?` を事前に見ても send 完了までの状態変化を防げないため、送信側は ClosedError 自体を処理する。Port はプロセス内通信なので、再試行、永続化、重複排除、障害後の復旧は別の要件として設計する。

## 避ける使い方と検証項目

- `take` を `value` に一括置換して無限 worker loop の結果を待つこと。途中結果の Port と終了確認を分ける
- 親が worker の default Port を close すること。作成者の制約を検証する
- close を「既存の全メッセージが消える」と解釈すること。キュー有りでの drain、空になった後の ClosedError をテストする
- move 後の元 payload を inspect・再送すること。小さなログ用 ID を分離する
- send が戻る速さだけで負荷試験を評価すること。未完了 job 数、結果待ち時間、RSS、worker 異常終了を合わせて測る

移行テストでは、通常の結果受信、空の Port の待機、非所有者 receive/close、閉鎖済み send、途中例外、終了値の別 Ractor からの取得、入れ子 freeze、move 後アクセスを対象にする。これらは提案する実行時テストであり、本調査で実行済みという意味ではない。

## 出典・鮮度・未確認事項

- 一次資料は上記 5 ページを実際に開いて確認した。RDoc 3 ページは Ruby 4.0 表示で公開日と patch 版を表示しない。発表日は 4.0.0 が 2025-12-25、4.0.7 が 2026-09-15。すべての取得日は 2026-10-02 UTC
- RDoc のライセンスは同じ 4.0 文書の [COPYING](https://docs.ruby-lang.org/en/4.0/COPYING.html) で Ruby License / 2-clause BSDL の選択肢を確認し、LEGAL の例外一覧に Ractor 文書の個別項目は見つからなかった。ニュース記事本文へのライセンス適用範囲は確認できず catalog は unknown とした。いずれも原文・サンプルコードを転載せず独自に要約し、module には昇格しない
- `release_notes` の TTL 30 日が最短なので明示期限は 2026-11-01。再確認なしに取得日だけを更新しない
- Ruby 4.0.x の実行、gem / C extension ごとの対応、patch 間の全差分、性能、select の公平性・大規模時の上限、クラッシュ耐性は未検証。実運用の runtime と依存版で確認する。RDoc の仕様確認と検索 eval の成功は、実行時互換性の証明ではない

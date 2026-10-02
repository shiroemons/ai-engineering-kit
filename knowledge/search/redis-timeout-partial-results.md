---
{
  "id": "search-redis-timeout-partial-results",
  "title": "Redis Search 8.10 の TIMEOUT と RETURN_STRICT：部分結果・foreground cap・移行判定",
  "kind": "knowledge",
  "technology": "search",
  "version": "Redis Open Source 8.10.0 GA release notes（July 2026）と2026-10-02取得のcurrent検索文書; 8.8履歴を比較、初出patchと実環境の既定値は未確定",
  "tags": [
    "research-domain:data",
    "redis",
    "search",
    "TIMEOUT",
    "RETURN_STRICT",
    "partial-results",
    "search-workers",
    "foreground-cap",
    "migration"
  ],
  "sources": [
    {
      "id": "redis-search-timeout-release-8-10-20261002",
      "url": "https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.10-release-notes/",
      "type": "release_notes"
    },
    {
      "id": "redis-search-timeout-configuration-20261002",
      "url": "https://redis.io/docs/latest/develop/ai/search-and-query/administration/configuration/",
      "type": "official_docs"
    },
    {
      "id": "redis-ft-search-timeout-command-20261002",
      "url": "https://redis.io/docs/latest/commands/ft.search/",
      "type": "official_docs"
    },
    {
      "id": "redis-ft-aggregate-timeout-command-20261002",
      "url": "https://redis.io/docs/latest/commands/ft.aggregate/",
      "type": "official_docs"
    },
    {
      "id": "redis-ft-hybrid-timeout-command-20261002",
      "url": "https://redis.io/docs/latest/commands/ft.hybrid/",
      "type": "official_docs"
    },
    {
      "id": "redis-search-timeout-history-8-8-20261002",
      "url": "https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.8-release-notes/",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# Redis Search 8.10 の TIMEOUT と RETURN_STRICT

## 解く問いと結論

検索が期限に達したとき、返った行を完全な検索結果として扱ってよいか。`TIMEOUT` を指定すればアプリケーションの応答時間まで保証されるか。

`RETURN_STRICT` は期限の適用を強める部分結果モードであり、完全結果を保証するモードではない。全件性が必要な処理では timeout を明示的な失敗として扱う設計が必要になる。期限予算、失敗時の返答、実行threadの3つを別々に確認する。

本稿の新規性は2026年7月GAの8.10で整理された検索の実行境界にある。既存のPostgreSQL全文検索・trigram索引の文書を置き換えず、Redis Streamsの配送・重複排除とも別の問いを扱う。「8.10で同名機能が初めて実装された」とは断定しない。

## 確認した公式契約

### 1. TIMEOUTの値と返答方針は別

- `FT.SEARCH` の `TIMEOUT` はミリ秒で、moduleのtimeoutをそのクエリについて上書きする。閲覧した構文にクエリ単位の `ON_TIMEOUT` はない。設定名 `search-on-timeout` を、そのまま検索コマンドの引数へ挿入しない。[FT.SEARCH](https://redis.io/docs/latest/commands/ft.search/)
- `FT.AGGREGATE` にも同じ単位の上書き指定がある。`WITHCURSOR` が存在することと、処理全体がtimeoutから免除されることは同義ではない。[FT.AGGREGATE](https://redis.io/docs/latest/commands/ft.aggregate/)
- `FT.HYBRID` も実行用のミリ秒指定を持つ。全文とvectorの検索に加え `LOAD` などの後処理を持つため、単純な `FT.SEARCH` の測定だけで移行可否を決めない。[FT.HYBRID](https://redis.io/docs/latest/commands/ft.hybrid/)

8.10のrelease notesは、`FAIL` を期限超過時のエラー、既定の `RETURN` を後処理の期限適用が厳格でない部分結果、`RETURN_STRICT` を後処理まで期限を適用する部分結果として説明する。`RETURN_STRICT` の利用条件は `search-workers > 0`。`FAIL` もworkerを使う場合はpreemptiveな期限適用になると記載する。[8.10 release notes](https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.10-release-notes/)

したがって `STRICT` という名前から完全性を推論できない。これは上記契約からの設計上の帰結であり、Redisがアプリケーションの用途別方針を指定しているわけではない。

### 2. 既定値とforeground capを混同しない

設定文書の契約は次のとおり。[Configuration parameters](https://redis.io/docs/latest/develop/ai/search-and-query/administration/configuration/)

- `search-timeout` は個別指定がない場合の既定値であり、上限ではない。query parsingはその計時に含まれない
- `search-workers = 0` では `search-_max-foreground-timeout-limit` が実効timeoutを制限する。既定は `60000` ms。超過指定をエラーで拒否せずcapする
- このcapは通常無制限を意味する `TIMEOUT 0` にもかかる。RESP3では `MaxTimeoutCapped` warningが返る
- capの設定自体を `0` にすると無効。workerが正数の場合もこのcapは無効
- 対象として `FT.SEARCH`、`FT.AGGREGATE`、`FT.CURSOR READ`、`FT.HYBRID` が列挙されている

つまり数値 `0` は、query側では無制限指定、cap側では上限制御の無効化、worker側ではforeground実行という別の意味を持つ。設定を一括でゼロにする移行は安全な共通手順にならない。

## 適用版と文書の不一致

以下は資料同士を照合した観察であり、実装試験の結果ではない。

1. 8.10のrelease notesは `RETURN_STRICT` をnewとして説明し、設定文書も8.10以降と記す。一方、2026年5月の8.8履歴には、既に同名policyでcoordinatorの部分結果と `SORTBY` が誤る不具合 `MOD-13617` の修正がある。最初の導入版・提供形態の差は未確定。8.10より前に存在しないとは書かない。[8.8 release notes](https://redis.io/docs/latest/operate/oss_and_stack/stack-with-enterprise/release-notes/redisce/redisos-8.8-release-notes/)
2. 8.10 release notesはworker正数を既定として説明するが、閲覧した設定文書の `search-workers` 既定値は `0`。配布物や構成による差なのか、文書の更新差なのかは未確認。実効値を取得するまでどちらも導入先の既定値と決めない
3. 8.10の修正一覧には、local `FT.HYBRID` がstrict指定でも期限を無視する問題 `MOD-16492` がある。同じpolicy名を受け付けることだけでは、目的の実行経路まで検証済みとはならない
4. 8.8履歴には `FT.CURSOR READ` のtimeoutと `FAIL` がcoordinator/shardで適用されない問題の修正もある。cursor利用時は最初のaggregateと継続readを別々に試験する

8.10のGA表記は月まで確認した。current URLの内容は変更され得るため、ここで扱う契約は2026-10-02取得時点の文書による。Redis Cloud/Softwareでの提供開始日、混在version構成、個々のpatchの実装はこの調査では確認していない。

## 採用判断：以下は独自の設計案

### 用途ごとに成功の意味を決める

- 候補提示・検索サジェスト：利用者が途中結果を受け入れられるなら、部分結果モードを検討する。期限と返却件数だけでなく、期待した候補の欠落率も評価する
- 請求対象の抽出・全件export・集計の確定：部分結果を完全な母集団として後工程に渡さない。timeoutを失敗にする方針と、未完了を再試行できる作業単位を設計する。`FAIL` を選んでもindexの鮮度や検索モデル自体の完全性が保証されるわけではない
- RAGの根拠検索：部分結果を空集合や「該当資料なし」と同一視しない。必要な根拠が揃わなければ回答を保留するか、絞り込んだ再検索へ移る条件を別途定義する

### 変更前に確認するもの

1. server/Searchのversion、standalone/cluster、clientのRESP版、実際の4設定値を記録する。`search-timeout`、`search-on-timeout`、`search-workers`、`search-_max-foreground-timeout-limit` が対象。未対応名や未対応policyを無視して成功扱いしない
2. アプリケーションが組み立てる `TIMEOUT` の全経路を洗う。共通既定値だけ変更しても個別指定が残れば予算は統一されない
3. 返却件数と完了判定を分ける。clientがtimeoutやwarningをどの型・フィールドで公開するか確認し、部分結果の判別方法を受け入れ試験に入れる。ここでは未検証のwireフィールド名を発明しない
4. 検索処理の予算と、接続・待ち行列・転送・client処理を含む端から端のdeadlineを分けて測る。server設定だけをAPIの応答SLAとして公約しない

### 移行試験の最小セット

- 通常完了、期限超過、該当なしを別ケースにする。いずれも空配列に潰さない
- 後処理が重い `LOAD` / `GROUPBY` / `REDUCE`、および利用している `FT.HYBRID` の経路で、modeごとの返答と実測時間を比較する
- foreground構成で個別の正数timeout、`TIMEOUT 0`、cap未満・超過を試す。設定が受理されたことと、意図した実効予算になったことを別々に確認する
- clusterでは遅いshardを含め、部分結果と通常結果の集合・集計を比較する。混在versionの移行中に同じ振る舞いを仮定しない
- `FT.CURSOR READ` を使う場合、継続読み出し・中断・後始末を含める。cursorがあるから全件を得られた、とは判定しない
- 再試行は回数・全体deadlineを制限する。期限切れの重いクエリを無条件に即再送して負荷を増幅させない

## 落とし穴と未検証範囲

`RETURN_STRICT` を厳密な集計、`TIMEOUT 0` を無条件の無制限、設定反映を実効性の証明、クエリの返答を端から端の時間保証と読み替えることが主な落とし穴である。

実サーバーを起動した負荷試験、worker既定値の実装追跡、timeout warningのclient別decode、cluster coordinatorの停止精度、cursor全体のdeadline合成は未実施。本文の試験項目は検証計画であり、成功した実験結果ではない。原資料のコード・図・長文を転載せず、契約の独自要約と設計上の推論を記録した。参照文書のライセンス確認先と例外は各source catalogに残す。

release notesを含むため再確認期限は2026-11-01（30日）。更新時はversion境界・worker条件・cap・文書不一致を再照合し、日付だけを延長しない。

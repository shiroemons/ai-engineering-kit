---
{
  "id": "observability-alertmanager-inhibition-source-lifecycle-boundary",
  "title": "Alertmanager 0.34.1: 複数 source の解消・GC と inhibition 継続の境界",
  "kind": "knowledge",
  "technology": "observability",
  "version": "Alertmanager v0.34.1 (2026-09-17), commit 73c6bfe7393929211294c1954f30d8ed78e4d0ad; configuration contract verified 2026-10-03",
  "tags": [
    "research-domain:quality-operations",
    "alertmanager",
    "prometheus",
    "inhibition",
    "equal",
    "source-lifecycle",
    "garbage-collection",
    "two-sided-match",
    "group_wait"
  ],
  "sources": [
    {
      "id": "alertmanager-inhibition-release-0-34-1-20261003",
      "url": "https://github.com/prometheus/alertmanager/releases/tag/v0.34.1",
      "type": "release_notes"
    },
    {
      "id": "alertmanager-inhibition-config-0-34-1-20261003",
      "url": "https://github.com/prometheus/alertmanager/blob/73c6bfe7393929211294c1954f30d8ed78e4d0ad/docs/configuration.md",
      "type": "official_docs"
    },
    {
      "id": "alertmanager-inhibition-implementation-0-34-1-20261003",
      "url": "https://github.com/prometheus/alertmanager/tree/73c6bfe7393929211294c1954f30d8ed78e4d0ad/inhibit",
      "type": "github_repository_analysis"
    },
    {
      "id": "alertmanager-inhibition-tests-0-34-1-20261003",
      "url": "https://github.com/prometheus/alertmanager/blob/73c6bfe7393929211294c1954f30d8ed78e4d0ad/inhibit/inhibit_test.go",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/quality-operations.json"
  ]
}
---

# Alertmanager inhibition を source の集合とライフサイクルで検証する

## 調査した問いと変更の位置づけ

上位障害の alert が残っているのに、配下の警告通知が突然再開する場合、inhibition の設定ミスと実装上の解除をどう切り分けるか。特に同じ `equal` labels を共有する複数 source のうち、1件だけが解消・GC された場面を扱う。

[0.34.1 release][release] は **2026-09-17** の公開版として、inhibition が一部の条件で不適切に解除される問題の修正を記載し、#5542・#5449・#5559 を列挙する。取得日は 2026-10-03。以下の実装観察は tag を解決した commit `73c6bfe7393929211294c1954f30d8ed78e4d0ad` に固定する。現行版一般の完全性や、すべての旧版が同じ不具合を持つことは主張しない。

今回の新しさは複数 source と cache/index のライフサイクルを扱う修正である。`equal` における欠落 label の意味や self-inhibition 回避は、0.34.1 で新設された仕様として紹介しない。既存の SLO／burn-rate 文書が扱う「いつ alert を発火させるか」とは別に、発火後の通知抑止を調べる。

## 公式の設定契約

[固定版 configuration][config] の `inhibit_rule` では、target matcher に一致する alert に対し、source matcher に一致する alert が存在し、`equal` に指定したすべての label 値が一致すると抑止が成立する。severity の大小を自動推論する機能ではなく、依存関係を matcher と label で表現する。

- `equal` の label が欠落している場合と値が空文字の場合は同値である。両側で `equal` の全 label が欠落していれば、他の条件が一致する rule は適用される。「cluster を指定したから cluster 欠落データは安全に除外される」とは考えない
- source と target の両側に一致する alert を two-sided match と呼ぶ。この alert は、自分自身を含む別の two-sided alert からは抑止されない。公式は、両側に同時一致しない matcher 設計を推奨する
- two-sided の例外は「両側に一致した target は絶対に抑止されない」という意味ではない。source 側だけに一致する別 alert があれば、その source による抑止は成立し得る。この区別は固定版の `Mutes` と `hasEqual` でも確認できる。[実装][implementation]

## 0.34.1 の実装で確認した保持・解除の境界

以下は [cache.go][cache] と [inhibit.go][inhibit] の静的観察であり、将来版の内部 API 契約ではない。

### 1. equal-key は単一 source ではなく集合を指す

cache は source alert 本体を fingerprint ごとに持ち、`equal` labels の fingerprint をキーとする index に `FingerprintSet` を保存する。`find` は該当集合を探索し、評価時点で `ResolvedAt(now)` の候補と、追加の条件に合わない候補を除外する。有効な候補が1つでも残れば、その fingerprint を返す。

したがって source が2件あるケースの正しい判断単位は、「最後に見た1件の状態」ではなく「有効な候補の有無」である。1件が解消しただけで、同じ equal-key の残りの候補まで消えたものとして扱ってはいけない。

### 2. GC は解消した候補だけを取り除く

`gc` は解消済み alert を本体 map から取り除き、その fingerprint だけを index の集合から削除する。集合が空になった場合に限り equal-key 自体を消す。`set` と `gc` は本体と index を同じ `RWMutex` の write lock 下で変更し、`find` は read lock 下で参照する。

この実装から確認できるのは **1プロセス内の cache/index の整合性** である。単一 lock があることを、HA replica 間の同期、入力の到着順、receiver 配信の原子性まで保証する根拠にしない。

### 3. 抑止根拠として返る1件は全候補の一覧ではない

`find` は最初に有効な source を見つけると返り、`Mutes` も適用できる rule を見つけた時点で true を返す。抑止結果に現れる fingerprint だけから、ほかの有効 source が存在しないとは判断できない。根拠 fingerprint の切り替わりと、抑止の解除を別の事象として観測する。

### 4. upstream のテストが区別しているケース

[固定版 inhibit_test.go][tests] を読み、次の期待値を確認した。テストをこの調査環境で実行したという意味ではない。

- `TestInhibitRuleIndexSurvivesGC`: 同じ equal labels に active と resolved の source がある場合、GC 前後とも active source が target を抑止する。最後の active も解消して GC された場合、探索結果と index が空になる
- `TestInhibitRuleTwoSidedDoesNotShadow`: source-only と two-sided の候補が同じ equal labels を持つ場合、two-sided を除外しても source-only が見つかる
- `TestInhibitRuleGCCallbackDoesNotRemoveRefreshedSameFingerprintSourceAlert`: 同じ fingerprint の古い解消済み source を active な値に更新した後、GC が更新後の source を失わせない

## 運用で使う判断手順（独自の設計案）

以下は資料に基づく独自の確認手順であり、公式の運用保証や実測結果ではない。

### 設定と時系列を同じ fixture にする

検証用に `cluster=west`、`service=orders` を equal labels とする。source S1 と S2 は severity=critical、target T は severity=warning とし、source/target の役割が重ならない rule から始める。S1/S2 は equal labels 以外の label で識別する。これは本書独自の仮想例である。

1. T だけを入力した状態では、この rule による抑止は成立しないことを確認する
2. S1 と S2 を両方 active にすると T が抑止されることを確認する
3. S1 だけを解消させても S2 が残る間は抑止が続くことを確認する。ここを今回の最重要回帰ケースにする
4. S1 の除去を挟んでも S2 が保持されることを確認し、同じ試験を source の投入順を逆にして行う。内部 GC を直接呼べない black-box 試験では、観測した抑止継続と GC 経路を実際に検証できたかを分けて記録する
5. S2 も解消した後、この rule 以外の抑止要因を取り除いた状態で T が抑止されなくなることを確認する。source の1件解消と最後の1件の解消を別ケースにする
6. 同じ source fingerprint の EndsAt 更新、解消と再発火、同時入力を加える。通知の最終到達だけでなく、受け付けた source 状態、抑止状態、receiver への送信を別々に記録する

追加の負例は、cluster 不一致、cluster の両側欠落、片側欠落／片側空文字、source-only と two-sided の共存にする。欠落を業務上許さない場合、label 付与の契約と非空を要求する matcher を双方で確認する。値が違えば不成立、欠落同士なら同値という契約を、upgrade で修正される不具合と混同しない。

### 到着待ちと内部の誤解除を分離する

[configuration の route][config] は、新規 group の最初の通知まで待つ `group_wait` を説明する。短すぎると inhibition source の到着前に target の初回通知が送られることがあり、長すぎると重要通知も遅れる。0.34.1 の cache 修正は、まだ到着していない source を先読みする保証にはならない。

検証では「source が存在したのに抑止が失われた」と「source の到着が初回通知に間に合わなかった」を分ける。後者を全体の group_wait 延長だけで隠さず、source/target の生成時刻、Alertmanager への到着時刻、必要な検知期限を比較する。値の最適解はこの調査からは決められない。

また、この rule の inhibition 解除だけで即時配送を期待しない。通知の有無には route、silence、通知タイミング、receiver の成否も関わる。入力 alert の firing／resolved と、通知を抑止した状態を別欄に保存し、「通知なし」を障害復旧の証拠にしない。

### 更新前後の差分を判定する

旧版で source の一部解消に伴って配下の通知が増え、0.34.1 でそれが止まった場合、まず残存 source と rule の適用を調べる。通知件数が減ったことだけを配信故障と判定しない。一方、抑止されたくない別 cluster／service の対照ケースが引き続き通知可能かも確認し、欠落 label による過剰抑止を同時に検出する。

## 避ける使い方・未確認事項

- 0.34.1 へ更新しただけで inhibition 設計、到着順、全 receiver の到達性も検証済みとする
- equal labels の異なる source を用意して、同じ equal-key の複数候補・GC 問題を試験したとする
- 抑止根拠の fingerprint 1件を、抑止できる全 source の一覧と解釈する
- two-sided target を常に非抑止と断定したり、source-only の対照を省略したりする
- 上流のテスト本文を読んだこと、KB の検索 eval が通ったことを、Alertmanager 実機試験の成功と書く

本調査は v0.34.1 の契約・固定コード・upstream テストの読解までで、旧版全範囲の再現、上流テスト実行、実トラフィック replay、HA rolling upgrade、性能劣化の測定、実機の通知配送は未実施。#5449 の PR ページは web 取得が失敗したため、内容をそのページから読めたとは扱わず、release に含まれる記載と固定版の回帰テストで確認できる範囲に限った。cache.go とテストの web 表示も Cache miss のため GitHub read connector で同じ full commit の本文を確認した。

実装・固定文書は [Apache-2.0 LICENSE][license] を確認し、コードやテストを転載せず独自に要約した。release notes 本文の独立した license 表示は未確認（catalog は unknown）。公開日は release の日付と区別して、文書独自の公開日は不明とする。release_notes の TTL30日が最短なので、再確認期限は 2026-11-02。

[release]: https://github.com/prometheus/alertmanager/releases/tag/v0.34.1
[config]: https://github.com/prometheus/alertmanager/blob/73c6bfe7393929211294c1954f30d8ed78e4d0ad/docs/configuration.md
[implementation]: https://github.com/prometheus/alertmanager/tree/73c6bfe7393929211294c1954f30d8ed78e4d0ad/inhibit
[cache]: https://github.com/prometheus/alertmanager/blob/73c6bfe7393929211294c1954f30d8ed78e4d0ad/inhibit/cache.go
[inhibit]: https://github.com/prometheus/alertmanager/blob/73c6bfe7393929211294c1954f30d8ed78e4d0ad/inhibit/inhibit.go
[tests]: https://github.com/prometheus/alertmanager/blob/73c6bfe7393929211294c1954f30d8ed78e4d0ad/inhibit/inhibit_test.go
[license]: https://github.com/prometheus/alertmanager/blob/73c6bfe7393929211294c1954f30d8ed78e4d0ad/LICENSE

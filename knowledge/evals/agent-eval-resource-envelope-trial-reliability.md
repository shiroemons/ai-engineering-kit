---
{
  "id": "evals-agent-resource-envelope-trial-reliability",
  "title": "Agent eval の比較条件: resource envelope・試行分離・pass@k/pass^k を揃える",
  "kind": "knowledge",
  "technology": "evals",
  "version": "Anthropic agent eval 設計記事 2026-01-09 / infrastructure noise 実測記事 2026-02-05（Terminal-Bench 2.0）; τ-bench arXiv:2406.12045v1 2024-06-17; 2026-10-02確認",
  "tags": [
    "research-domain:ai-engineering",
    "evals",
    "agents",
    "resource-envelope",
    "infrastructure-noise",
    "trial-isolation",
    "pass-at-k",
    "pass-hat-k",
    "outcome",
    "reproducibility"
  ],
  "sources": [
    {
      "id": "anthropic-agent-evals-design-20260109-verified-20261002",
      "url": "https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents",
      "type": "maintainer_article"
    },
    {
      "id": "anthropic-agent-evals-infrastructure-noise-20260205-verified-20261002",
      "url": "https://www.anthropic.com/engineering/infrastructure-noise",
      "type": "maintainer_article"
    },
    {
      "id": "tau-bench-trial-reliability-v1-20261002",
      "url": "https://arxiv.org/html/2406.12045v1",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/ai-engineering.json"]
}
---

# Agent eval の比較条件: resource envelope・試行分離・pass@k/pass^k を揃える

## 問いと適用範囲

モデルを交換したら成功率が上がった。その差はモデルの改善か、RAM の増量、再試行回数、前回の作業ファイルが残った結果か。本書は agent eval の比較単位を揃え、単発の成功と繰り返しの信頼性を読み分けるための記録・判定案を示す。

2026年2月の agent 評価基盤の実測報告を中心に、未収録だった比較条件の穴を埋める調査である。直近の API 仕様変更を発見したという主張ではない。LLM judge 自体の較正は[既存の採点方式文書](llm-as-judge-grading-bias-mitigation.md)、一般のテスト分離は[hermetic testing](../testing/test-scope-doubles-hermetic-flaky.md)を参照する。本書では agent が環境を変更することと、複数試行の集計が相互作用する点に絞る。

## 一次資料で確認できた事実

### 1. 回答文・実行結果・試行を同じものとして扱わない

[Anthropic の評価設計記事](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)（2026-01-09）は、入力と成功条件を持つ task、その一回の試みである trial、実行記録 transcript、終了時の環境状態 outcome を区別する。評価対象の agent はモデルと agent harness の組合せである。同記事は、試行間に残ったファイル・キャッシュ・Git 履歴などが成功率を上振れさせたり、共通の資源不足が失敗を相関させたりする問題を報告している。

### 2. 資源の余裕は、単なる安定化にも追加の解法にもなる

[Anthropic の infrastructure noise 記事](https://www.anthropic.com/engineering/infrastructure-noise)（2026-02-05）は、同じ Claude モデル・harness・task 集合で Terminal-Bench 2.0 の資源条件を変更した実験を報告する。厳格な 1x 条件から 3x 条件では基盤エラー率が 5.8% から 2.1% に下がった一方、成功率の変化は統計的に明確ではなかった（p=0.40）。上限なしの条件では、1x に対して成功率が6パーセントポイント高くなった（p<0.01）。記事は保証量と上限を別々に報告し、余裕の倍率を実測で較正するよう提案している。

これは当該構成の観測であり、「RAMを3倍にすれば公平」「上限なしで評価すれば正しい」という一般則ではない。記事自身も倍率は benchmark と task 分布に依存すると留保する。とくに OOM の減少と、重い依存関係や計算を使えるようになったことによる解法の変化を区別する必要がある。

### 3. pass@k と pass^k は異なる質問に答える

[τ-bench 論文 §3](https://arxiv.org/html/2406.12045v1#S3)（arXiv:2406.12045v1、2024-06-17）は、同一 task の独立同分布（i.i.d.）の試行について、少なくとも一回成功する pass@k と、全 k 回成功する pass^k を区別する。task ごとに n 回中 c 回成功したとき、次を各 task で計算して平均する不偏推定量を示す。

- pass@k: `1 - C(n-c, k) / C(n, k)`
- pass^k: `C(c, k) / C(n, k)`

`C(a,b)` は組合せの数で、`a < b` なら0とする。ここでは整数 `1 <= k <= n` が必要である。同論文の報酬判定は終了時の DB と回答を使うが、終状態の一致だけでは途中の方針違反を検出しきれないという限界も明記する。

## 独自の設計案: 比較用の記録を一つの単位にする

以下は本書の提案であり、上記の論文や製品が要求する設定形式ではない。resource envelope は agent に与えた資源・時間の範囲を指す本書内の呼び方である。

| 記録する項目 | 最低限残す内容 | 揃えないと混ざる差 |
|---|---|---|
| task の版 | task 集合の固定版、初期状態、成功条件、grader の版 | 問題の難易度や採点規則の変更 |
| agent の版 | モデル識別子と取得可能な snapshot、prompt、harness、tool の版 | モデルと実装の変更 |
| resource envelope | CPU の割当と制限方法、RAM保証量と上限、ディスク、実行基盤 | 計算可能な解法や基盤障害 |
| 打切り予算 | 壁時計時間、tool timeout、turn/token 上限、内部 retry 上限 | 一回の trial で許す仕事量 |
| 実行条件 | 同時実行数、開始時刻、ネットワーク・外部 API 条件 | 競合や時間帯による遅延 |
| 集計条件 | task ごとの試行数 n、成功数 c、k、欠測・除外・再実行規則 | 成功率の分母と再試行効果 |

モデル名の alias しか得られなければ、snapshot を確認したことにせず、呼出日時と未固定であることを記録する。CPU制限とメモリ上限を同じ「超過なら kill」として記録しない。実際の制限方法・観測結果を別々に残す。

比較したい一要因だけを変更する。たとえばモデル A/B を比べる本試験の途中で B だけ RAM を増やさない。RAM不足を調べる実験は別の条件として識別し、採用する条件を確定したあとで両モデルを同条件で再測定する。

## 独自の設計案: trial の実行と失敗の扱い

1. 各 trial に一意の ID を付け、同じ初期状態から開始する。作業ディレクトリだけでなく、DB、外部 sandbox、キャッシュ、既存の成果物・Git履歴の扱いを固定する。前回の解答を渡すなら「試行分離済み」ではなく、その情報を使う別の agent 構成として評価する。
2. 完了メッセージを成功フラグにせず、grader が成果物や backend 状態を確認する。加えて、利用者への説明や必須の手順など、その task の成功条件に必要な性質を個別に確認する。終状態だけで足りる task と足りない task を区別する。
3. 失敗は少なくとも「目標未達」「予算超過」「基盤エラー」「採点不能」に分け、根拠となるイベント・終了コード・観測状態を付ける。原因未確認の timeout を自動で基盤エラーへ付け替えない。
4. 全件に同じ再実行規則を適用する。後から成功した試行だけを残す運用は禁止する。元の trial と再実行した trial の対応、回数、費用を保持する。
5. モデル比較の実行順を片寄らせない。同じ task の A/B を時間的に近く実行する、順番を交替するなどの計画を事前に決める。それでも外部 API や共有ホスト由来の相関が残る可能性を記録する。

本番相当の end-to-end 成功率では、決めた予算内に成果が返らなかった試行も失敗として分母へ残す。一方で原因分析用に「基盤エラーを除く集計」を出す場合は、補助指標と明記して除外件数と規則を併記する。二つの集計を入れ替えて改善を主張しない。採点不能は成功にせず、未解決件数を明示して比較の確定を保留する。

## 数値例: 同じ成功率から異なる結論を作らない

以下の数値は出典の実測ではなく、本書の計算例である。

同じ task で真の一回成功確率が p=0.8、各試行が独立なら、k=3 の「一回以上成功」は `1 - (1-0.8)^3 = 0.992`、「全回成功」は `0.8^3 = 0.512` となる。99.2% は「毎回ほぼ失敗しない」という意味ではない。また、正解を識別する手段や複数試行の費用を考慮しない pass@k を、そのまま本番の成功率にはできない。

有限標本の計算例として、n=5、c=3、k=2 なら、前述の組合せによる推定値は pass@2=0.9、pass^2=0.3 である。`(c/n)^2 = 0.36` は同じ推定式ではない。k>n は未定義として扱い、成功確率0と表示しない。

task A/B の真の成功確率がそれぞれ0.9と0.1で、二つを同じ重みで評価するなら、pass^2 は `(0.9^2 + 0.1^2)/2 = 0.41` である。先に平均した0.5を二乗した0.25とは一致しない。taskごとの値を計算してから集約し、難易度が異なる問題を一つの確率に潰さない。試行数が違うときは、task等重みと trial等重みのどちらを測るのかも宣言する。

## 採用前に通す検証シナリオ

以下は将来の評価基盤向けの受入れ条件案であり、本リポジトリで agent を実行した結果ではない。

| 入力・状況 | 期待する観測・判定 |
|---|---|
| agent が「保存済み」と返すが成果物が初期状態のまま | outcome 判定は失敗。発言だけでは成功にしない |
| 成果物は正しいが、task が禁止した途中の変更を行った | 該当する制約の検査で失敗。終状態の一致で上書きしない |
| 前 trial の成果物や履歴だけで解ける状態が残る | isolation 検査で検出し、汚染した結果を比較用の成功として採用しない |
| B のみメモリ上限を変更 | 記録差分で検出。モデルだけを変えた A/B 比較とは表示しない |
| OOM 後の再実行だけ成功 | 元の失敗と追加試行を保存。元の trial を成功に書き換えない |
| n=5、c=3、k=2 の集計 | pass@2=0.9、pass^2=0.3。k=6 は算出不可 |
| task別に試行数が異なる | 重み付けを明示。無断で task 平均から trial 平均に変えない |
| 共通の API 障害で複数 trial が同時失敗 | 共通障害のラベルを残し、i.i.d. の式だけで信頼性を保証しない |

## 版・provenance・未確認事項

- 基盤の数値は2026-02-05公開の Terminal-Bench 2.0 実験報告に限定する。正確なモデル snapshot、全 task の生データ、実験環境一式を本調査で再現したわけではない。他モデル・別 benchmark への倍率や効果量の外挿はしない。
- 指標の定義は τ-bench の arXiv:2406.12045v1（2024-06-17）に固定する。τ2-bench や現行実装の挙動を調べたという主張ではない。n 回の試行が同一条件・独立でない場合、式の数値をそのまま i.i.d. の確率推定として解釈しない。
- Anthropic記事2件には取得した本文でオープンライセンスを確認できず、catalog の `license` は `unknown` とした。τ-bench論文は arXiv の license 表示とリンク先で CC BY 4.0 を確認した。コード、図、データセット、試験問題を転載せず、短い事実要約と独自の設計・計算例だけを記載する。
- 3資料を native web で2026-10-02 UTCに開いて確認した。`maintainer_article` の90日 TTL に合わせて期限は2026-12-31とした。論文の公開年を取得年へ置き換えていない。
- 未確認: agent の実行、benchmark の再現、各クラウドの resource enforcement、統計的検出力、相関を考慮した信頼区間、実運用費用。本書は測定規約の案であり、モデル選定の実測結論ではない。
- `evals/knowledge/ai-engineering.json` で実行するのは本書を見つける検索 eval のみ。上表の振る舞い検証や、成功率の数値実験が通ったことを意味しない。

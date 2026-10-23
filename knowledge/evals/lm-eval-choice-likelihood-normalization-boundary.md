---
{
  "id": "evals-lm-eval-choice-likelihood-normalization-boundary",
  "title": "lm-eval 0.4.13: 選択肢尤度・文字数正規化・生成正解を分ける",
  "kind": "knowledge",
  "technology": "evals",
  "version": "lm_eval 0.4.13; tag v0.4.13 published 2026-08-31T20:42:30Z; commit ddd67220430a2470529f25fd5c05a576ca1057a0; ConfigurableTask default multiple_choice and HFLM causal scoring",
  "tags": [
    "research-domain:ai-engineering",
    "evals",
    "lm-evaluation-harness",
    "multiple-choice",
    "loglikelihood",
    "acc_norm",
    "acc_bytes",
    "acc_mutual_info",
    "tokenization",
    "reproducibility"
  ],
  "sources": [
    {
      "id": "lmeval-choice-release-0-4-13-20261023",
      "url": "https://github.com/EleutherAI/lm-evaluation-harness/releases/tag/v0.4.13",
      "type": "release_notes"
    },
    {
      "id": "lmeval-choice-task-guide-ddd6722-20261023",
      "url": "https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/docs/task_guide.md",
      "type": "official_docs"
    },
    {
      "id": "lmeval-choice-scoring-ddd6722-20261023",
      "url": "https://github.com/EleutherAI/lm-evaluation-harness/tree/ddd67220430a2470529f25fd5c05a576ca1057a0",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/ai-engineering.json"]
}
---

# lm-eval の選択式評価で、何を「正解率」と呼ぶか

## 問いと採用判断

同じモデル・問題集で `acc` と `acc_norm` が違う。どちらかが誤計算なのか。さらに、選択式では正解でも、実際に文章を生成させると違う答えになる。この差をモデル品質の改善・悪化と決める前に、**採点する文字列、候補の順位付け、生成の判定**を分離する。

対象は **lm-evaluation-harness / lm_eval 0.4.13** の `ConfigurableTask` 既定の `multiple_choice` 処理で、選択肢をcontinuationに置く `multiple_input=false`、正答が一つの場合に限定する。独自の `process_results`、選択肢を入力側へ置くtask、複数正答、自由生成の抽出・採点を同じ契約だと扱わない。新規APIの紹介ではなく、既存の尤度評価を再現可能に使うための調査である。

結論は、比較前に採用metricと採点文字列を固定し、結果を見て都合のよいmetricへ替えないこと。`acc_norm` は「token数で割った正解率」でも「偶然正解を補正した正解率」でもない。

## 版・根拠・ライセンス

- [v0.4.13 release](https://github.com/EleutherAI/lm-evaluation-harness/releases/tag/v0.4.13) は2026-08-31公開。GitHub release APIの `published_at` は **2026-08-31T20:42:30Z**。tagをdereferenceしたcommitは **ddd67220430a2470529f25fd5c05a576ca1057a0** で、同commitの [pyproject.toml](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/pyproject.toml) の版も0.4.13。最新版一般を保証する表記ではない。
- [固定版task guide](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/docs/task_guide.md) はtask YAMLとcode commitを再現条件として扱い、`doc_to_choice`、`doc_to_target`、`output_type`、delimiter、metric設定を説明している。文書単体の公開日は未表示であり、固定commitの内容として採用した。
- 実装根拠は同commitの [task.py](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/lm_eval/api/task.py)、[model.py](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/lm_eval/api/model.py)、[huggingface.py](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/lm_eval/models/huggingface.py)、[metrics.py](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/lm_eval/api/metrics.py)。以下の実装観察はこの版の該当経路に限る。
- root [LICENSE.md](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/LICENSE.md) とpackage metadataでMITを確認した。metrics.pyには別途Apache由来のexact-match関数節があり、本書はその関数を分析・転載していない。release proseのライセンスはunknown。ソースコード・benchmark問題をコピーせず、独自要約と自作の数値例を記す。

## 確認した契約: 同じ正解率でも順位付けが異なる

各選択肢の文字列を `c_i`、その条件付きloglikelihoodを `L_i` とする。固定版 `ConfigurableTask.process_results` は次の最大値を取る候補indexを正答indexと比較し、一問ごとに0/1を返す。該当metricの既定aggregationは `mean` である。

| metric | 順位付けに使う値 | 混同しないもの |
|---|---|---|
| `acc` | `L_i` | 文章を生成して正解を抽出した結果ではない |
| `acc_norm` | `L_i / len(c_i)` | Python文字列の長さであり、token数・UTF-8 byte数ではない |
| `acc_bytes` | `L_i / len(c_i.encode("utf-8"))` | Unicode文字数とは非ASCIIで異なり得る |
| `acc_mutual_info` | 条件付きloglikelihoodから空context側のloglikelihoodを引いた値 | 長さによる除算ではない |

`acc_norm` が平均するのは問題ごとの正誤であって、割り算後のloglikelihoodそのものではない。分母は `doc_to_choice` の返す文字列から計算される。別に付加する `target_delimiter` の長さはその分母に含まれない。Pythonの文字列長は見た目の一文字を常に一つと数えるものではなく、結合文字を含む表示の長さと同一視しない。

[metrics.pyの登録](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/lm_eval/api/metrics.py) にあるmetric関数名だけを読んでも、選択式の採点全体は分からない。この版の上記metric関数はpass-throughであり、順位付けと正誤の計算本体は `task.py` にある。custom processorを指定したtaskには、この既定計算を自動適用したと主張しない。

### 自作の数値例: 文字数とbyte数で勝者が入れ替わる

候補 `ab` と `あ` に、仮の条件付きloglikelihood `-3` と `-2` を与える。これはモデル実測ではなく、式を確認するための入力である。

- `acc` の順位: `-2 > -3` なので `あ`。
- `acc_norm` の順位: 文字列長は2と1なので、`-1.5 > -2` となり `ab`。
- `acc_bytes` の順位: UTF-8の長さは2と3なので、約 `-0.667 > -1.5` となり `あ`。

これだけで特定metricが多言語評価に優れるとは言えない。何を補正したいか、既存benchmarkがどのmetricを採用したかを先に決める必要がある。

反対に `doc_to_choice` が `A/B/C/D` のような同じ長さのASCIIラベルなら、同じ有限loglikelihood列に対する `acc`・`acc_norm`・`acc_bytes` の順位は同じになる。問題文に表示した選択肢本文の長短は、ラベルだけを採点するこの分母には現れない。**ラベル採点と回答本文採点は別の評価条件**であり、`acc_norm` を選ぶだけで選択肢位置・ラベルの偏りを除去できるとはしない。

## 確認した契約: 候補内の最良回答はgreedy生成の一致ではない

[HFLMの `_loglikelihood_tokens`](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/lm_eval/models/huggingface.py) は、continuationの各tokenに対応するlog probabilityを取り出して合計する。文脈tokenやpaddingまで含む平均値を返す処理ではない。一方、返り値の `is_greedy` は各位置のargmax tokenとcontinuation tokenの一致を確認する。

固定版task側の `multiple_choice` における `exact_match` は正答候補の `is_greedy` を使う。`acc` は提示候補の相対順位を見るため、全候補がgreedy出力とは異なっていても、その中の正答が最大なら1になり得る。逆に、greedyは各stepの局所選択であり、全長の系列確率が最大になる候補探索の別名ではない。

したがって、自由に応答させたときの指示遵守、回答抽出、停止条件を調べたいなら、別の生成評価として明示する。選択式の `acc=1` を「本番で必ずその答えを生成する」と読み替えない。`loglikelihood` output typeの `acc` と `multiple_choice` output typeの `acc` も、名前だけで同じ採点処理だと扱わない。

## 確認した契約: 空白・chat template・空contextも採点条件

[taskのrequest構築](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/lm_eval/api/task.py) は通常、contextと `target_delimiter + choice` の組を作る。task guideのdelimiter既定値は半角空白。ただし `apply_chat_template` の経路では `gen_prefix` とその末尾空白によってdelimiterが空になるため、設定値だけで実際のcontinuationを推定しない。

[TemplateLM._encode_pair](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/lm_eval/api/model.py) はcontext末尾の空白をcontinuation側へ移す。causal経路では結合した文字列をtokenizeし、contextのtoken数を使ってcontinuation部分を切り分ける。各部分を単独tokenizeして単純連結したものと常に同じとは仮定しない。これは共有実装の観察であり、すべての外部API backendの同一動作保証ではない。

`acc_mutual_info` を有効にすると、候補数Kに対して通常のK件に加えて、空contextで各候補を採点するK件をrequestへ追加する。返却列は先に条件付きK件、後に空context側K件として分割される。request数の倍増は確認できるが、cache・batchingがあるため実時間や料金が厳密に2倍になる保証ではない。

この空contextは、TemplateLM経路ではprefix tokenを条件にする。HFLMのprefix選択も実装・設定に依存するため、`P(choice)` を「tokenを一切与えない、モデル非依存の普遍的事前確率」と説明しない。同じdelimiter、tokenizer、prefix設定で条件付き・空context側の対応を維持する必要がある。

自作例として、条件付き値が `[-1,-2]`、空context側が `[-0.2,-3]` なら、差は `[-0.8,1]`。`acc` と `acc_mutual_info` は異なる候補を選ぶ。固定版の [tests/test_metrics.py](https://github.com/EleutherAI/lm-evaluation-harness/blob/ddd67220430a2470529f25fd5c05a576ca1057a0/tests/test_metrics.py) にも条件付き・空contextの分割と勝者が変わる回帰ケースがある。本調査はそのテストの読解までで、実行していない。

## 実装観察: 長い候補では見えるcontext自体が変わり得る

固定版HFLMのcausal尤度計算は、continuation token数が `max_length` 以下であること等をassertし、結合列が長すぎると左側を切って最後の `max_length + 1` tokenを使う。その後、最後のtokenを入力から外して次token予測を採点する。この経路の切り詰めを、生成経路で使う `truncation` 引数だけで無効にしたと思い込まない。

ここから導く注意は、**候補のtoken長が違うと、長い候補のために残せるcontextが短くなり得る**こと。最初の質問・few-shot例が全候補で同じ範囲だけ見えているとは限らない。これは切り詰め式からの推論であり、特定モデルで精度低下を測定した主張ではない。比較用データでは入力長の上限を事前に検査し、overflow件数と方針を記録する。

## 独自の設計案: 比較を固定する最小manifest

以下は本書の運用案であり、lm-evalの標準保存形式や完全再現保証ではない。

1. **何を測るかを先に決める。** ラベル尤度、回答本文尤度、自由生成のどれか、主metricと補助metric、正答indexの対応を固定する。`acc_norm` が高かったから過去の `acc` と比較する、といった事後変更をしない。
2. **版を独立に保存する。** harness commit、task YAMLとcustom processor、dataset revision・split・対象ID、model weights revision、tokenizer artifact、chat templateの内容・引数、依存package版を別々に識別する。model名だけでtokenizer/template/version artifactsが固定されたとはしない。
3. **実際の採点入力を残す。** 機密性に配慮したfixtureでrendered context、continuation、delimiter、文字数、UTF-8 byte数、token ID列と長さを照合する。few-shot件数・選択例・順序・seed、prefix/BOS/EOSも記録する。
4. **事前に不正・境界入力を止める。** 空候補、重複候補、不正なgold index、非有限loglikelihood、長さ上限超過を検出する。全件を安全に処理するvalidatorが既定で備わるとは主張せず、運用側の受入れ検査とする。同点は候補順とtie方針も保存する。
5. **変更後は入力差と採点差を分離する。** 同じ固定loglikelihood列でmetric実装を比較し、次に同じfixtureでtoken化・context保持を比較する。その後にモデルを呼ぶ。最終平均だけ一致しても、sampleごとの勝者が入れ替わっていないか確認する。

[v0.4.13 release notes](https://github.com/EleutherAI/lm-evaluation-harness/releases/tag/v0.4.13) 自体も、few-shotへの評価document混入などの修正により既報値が変わり得ると記載する。harnessを更新して数字が変わったことだけではモデルの改善を意味しない。過去版の全差分調査や、汚染がすべて解消されたとの保証は本書の対象外である。

## 導入先での受入れケース

以下は将来の評価pipelineに対する確認案。検索evalの合格や本書の算術確認は、これらのモデル実行を代替しない。

| 入力・変更 | 確認すべき結果 |
|---|---|
| `ab` / `あ`、loglikelihood `-3` / `-2` | accとacc_bytesは後者、acc_normは前者を選ぶ |
| 同長ASCIIラベル `A/B/C/D` | 同じ有限スコア列なら三つの長さ系metricの順位が一致する |
| 正答が候補内最大だが `is_greedy=false` | accと正答候補のexact_matchを同一視しない |
| 条件付き `[-1,-2]`、空context側 `[-0.2,-3]` | mutual-infoの差は `[-0.8,1]`。対応順の取り違えを検出する |
| target_delimiterだけ変更 | 分母はchoice文字列のまま。実際のcontinuationとtoken列の差を確認する |
| templateやgen_prefixを変更 | render済み文字列の変更を採点条件の変更として検出する |
| 長い候補でcontext上限を超える | 候補別のcontext欠落を検出し、無断で通常評価に混ぜない |
| 空候補・重複候補・不正gold・NaN | 運用側の事前検査で止め、比較できる正常結果として採用しない |

## 限界と再確認期限

- 固定版sourceと公式文書の静的照合、自作数値例のPython標準機能による算術確認まで。lm-evalの依存導入、公式テスト実行、モデル推論、tokenizer別・GPU別の再現実験は未実施。
- `multiple_input`、複数正答、task固有processor、非HFLM backend、seq2seq、multimodal、分散集約、外部API料金・cacheの正しさは検証範囲外。char/byte正規化が公平性や精度を改善する実測結果は提供しない。
- 日本語の例は文字列長とUTF-8長の差を示すもの。実モデルのtoken数・token境界やUnicode正規化の同値性を検証した例ではない。
- 取得日は **2026-10-23 UTC**。release notesのTTL 30日を含むため明示期限は **2026-11-22**。期限は採点定義が自動変更される日ではなく、対象版・根拠・運用条件を再確認する日である。

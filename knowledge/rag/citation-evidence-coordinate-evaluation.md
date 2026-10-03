---
{
  "id": "rag-citation-evidence-coordinate-evaluation",
  "title": "RAG の引用付き回答: source版・引用座標・根拠の支持と回答の網羅性を検証する",
  "kind": "knowledge",
  "technology": "rag",
  "version": "Claude Messages API Citations / Search results / Streaming rolling guides (2026-10-03 UTC確認、ページ版・公開日表記なし); ALCE EMNLP 2023, DOI 10.18653/v1/2023.emnlp-main.398 (2023-12)",
  "tags": [
    "research-domain:ai-engineering",
    "rag",
    "citations",
    "source-version",
    "document_index",
    "search_result_index",
    "citation-recall",
    "citation-precision",
    "entailment",
    "answer-coverage"
  ],
  "sources": [
    {
      "id": "claude-document-citation-coordinates-20261003",
      "url": "https://platform.claude.com/docs/en/build-with-claude/citations",
      "type": "official_docs"
    },
    {
      "id": "claude-search-result-citation-mapping-20261003",
      "url": "https://platform.claude.com/docs/en/build-with-claude/search-results",
      "type": "official_docs"
    },
    {
      "id": "claude-citation-stream-assembly-20261003",
      "url": "https://platform.claude.com/docs/en/build-with-claude/streaming",
      "type": "official_docs"
    },
    {
      "id": "alce-citation-quality-emnlp2023-20261003",
      "url": "https://aclanthology.org/2023.emnlp-main.398.pdf",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# RAG の引用付き回答を検証可能にする

## 問いと適用範囲

回答に引用リンクが付いていても、違う版の資料を開く、本文の別の箇所を強調する、数値の半分だけを裏付ける、といった失敗が残る。検索結果の relevance を測るだけで、利用者が回答の根拠を検証できるだろうか。

本書は未収録だった**引用の保存・表示・評価という durable な設計課題**を扱う。新しい release の発見ではない。Claude Messages API を具体例にし、保存形式と受入れテストを提案する。以下の座標契約は Claude のものであり、他 provider の annotation に一般化しない。

検索段階の順位・chunking は[retrieval 評価](retrieval-eval-chunking-grounding.md)、候補の不足は[filtered retrieval](azure-filtered-retrieval-candidate-starvation.md)、judge の較正は[LLM-as-a-judge](../evals/llm-as-judge-grading-bias-mitigation.md)を参照する。本書は、検索後に生成された回答と、その回答に結び付けられた evidence を対象とする。

## 一次資料で確認した契約

### document と search_result の座標を分ける

[Claude Citations guide](https://platform.claude.com/docs/en/build-with-claude/citations#citation-indices) は、`document_index` を全 messages の document block を並べた0始まりの位置として定義する。引用の型ごとの位置は次のとおりで、**終端はすべて exclusive** である。

| 引用 type | 座標 fields | 起点・単位 |
|---|---|---|
| `char_location` | `start_char_index`, `end_char_index` | 0始まり・文字 |
| `page_location` | `start_page_number`, `end_page_number` | 1始まり・ページ |
| `content_block_location` | `start_block_index`, `end_block_index` | 0始まり・custom content 内の block |

plain text / PDF は文単位へ分割され、custom content は追加分割されない。引用可能なのは document の `source` 内の本文で、`title` / `context` は対象外。画像引用には対応せず、抽出可能な text がない scanned PDF は引用できない。guide の保証は、提供した資料への有効な pointer とそこから抽出した `cited_text` である。この保証を、全主張の支持・元資料の正しさの保証へ拡張しない。

[Search results guide](https://platform.claude.com/docs/en/build-with-claude/search-results#citation-fields) の `search_result_location` は別の型である。`search_result_index` は全 messages と tool results に現れる search_result の順番を数える。`source` は URL に限らず内部識別子でもよい。`start_block_index` / `end_block_index` は0始まり・半開区間で、引用は block 全体を対象とし、`cited_text` は該当 block 群の連結となる。細かく引用したい場合は入力 block を分ける。

### streaming と形式の組合せ

[Citations guide の streaming 節](https://platform.claude.com/docs/en/build-with-claude/citations#streaming-support) は `content_block_delta` の `citations_delta` が現在の text block の引用リストへ1件を追加すると説明する。[Streaming guide](https://platform.claude.com/docs/en/build-with-claude/streaming#event-types) では、その `index` は最終 `Message.content` 配列の位置であり、block の開始・delta・終了の後、通常の stream は `message_stop` で終わる。stream 内に `error` が来る場合もあり、未知の event type への配慮も必要となる。

取得時点の Citations guide は、user-provided document / search_result の citations と `output_config.format`（旧 `output_format` を含む）の併用を400とする。引用付き prose を生成する経路と、独自 JSON schema の出力経路を同一と仮定しない。

### ALCE が測る citation quality

[ALCE 論文](https://aclanthology.org/2023.emnlp-main.398.pdf)（Gao et al., EMNLP 2023, §3.3）では、statement ごとに「引用が存在し、引用 passages の結合が statement を含意する」場合を1として平均するのが citation recall である。citation precision は各引用の関連性を測る。statement の recall=1 が前提で、単独では支持せず、除去しても残りの引用で支持できる引用を irrelevant とする。最小の引用集合を要求しているわけではない。

同論文 §3.4 は、本文を抜き出して引用するだけでも citation score が高くなり得る一方、質問への回答としての correctness / coverage が不足し得ることを示す。Limitations と Appendix E は、NLI が partial support を適切に扱えず、関連する引用まで低く採点する問題を明示する。これは2023年の評価方法の説明であり、現行モデルの品質を実測した結果ではない。

## 独自の設計案: request に対応した不変の source manifest

以下は本書の設計案で、Claude の必須保存形式ではない。回答、送信した資料、引用座標を同じ生成 attempt に束ねる。`document_index: 0` や URL 単独を永続 ID にしない。

| 保存するもの | 例となる項目 | 目的 |
|---|---|---|
| 生成 attempt | ローカル一意ID、provider response ID、モデル、開始時刻、完了状態 | 再試行や続行で結果を混ぜない |
| request の順序対応 | 型の名前空間、index、送信元 message/tool_result の位置 | 応答時の0番をその request の0番へ戻す |
| source の同一性 | corpus ID、原文の版、取得時刻、元ファイル/本文の hash | URLが同じでも変わった内容を識別する |
| 送信本文 | 実際に送った text/block 列、抽出器・正規化・chunker の版 | 引用座標が指す文字列を再現する |
| 元資料への対応 | chunk ID、原文中の位置、PDF page 対応、変換履歴 | 送信断片から原文へ戻す |
| 生成回答 | 順序付き content blocks と各引用、最終表示版 | 主張と evidence の対応を保つ |

hash は同じデータかを調べるための識別情報であり、内容の正しさ・出所の信用・表示権限を証明しない。保存先は既存の機密区分と保持期間に従い、権限を再確認してから資料を表示する。再表示のために新しく無期限保管することを前提にしない。

とくに次の変換を区別する。

1. **切り出した連続範囲**: 元文と chunk の文字列表現・座標単位が同一なら、local offset に切出し開始位置を足せる。検証済みの対応だけを使う。
2. **正規化・OCR・表の直列化**: 改行除去、Unicode正規化、列順変更は座標を変える。変換後本文を保存し、元資料への対応表を持つ。変換後の offset を直接元ファイルへ足さない。
3. **生成された補足 context**: retrieval 用に生成した説明を原文 chunk に前置きすると、その説明自体が引用され得る。生成補足と原文を分離して保存し、補足部分を元資料からの直接引用として見せない。
4. **重複・overlap chunk**: 同じ文が複数 chunk にあっても、回答が実際に引用した版・chunk から解決する。表示だけの重複排除では、元の全対応を残す。

URL は読者向けの入口として保持し、回答生成時の snapshot と現在の公開ページを区別して表示する。最新版を再取得できても、過去の回答を支持していた snapshot を黙って置換しない。

## 独自の設計案: 引用を壊さない renderer

- `type` で座標を dispatch する。document と search_result の0番を共通の配列へ流し込まない。raw citation object を保存し、UI用に整形したものから元情報を復元しようとしない。
- 全 text を連結して引用だけを末尾に集める前に、content block ごとの text と citations の対応を保存する。表示上まとめる場合も、どの主張にどの根拠が付いていたか追える中間表現を持つ。1 block を1つの原子的主張とは仮定しない。
- 範囲・型・manifest の存在をまず検査する。既知の block 単位なら `0 <= start < end <= block数`、PDFなら `1 <= start < end <= page数+1` を確認する。存在しない index を「いちばん近い資料」へ補正しない。
- 文字座標の Unicode 単位は、今回の guide から byte / Unicode scalar / UTF-16 code unit / grapheme のどれかまで確定できなかった。Go の byte slice や JavaScript の UTF-16 slice をそのまま契約とみなさない。日本語・補助平面文字・結合文字を含む fixture で provider と採用 SDK の実挙動を別途確認し、未確定なら誤った位置の highlight を出さず、`cited_text` と資料の対応を表示する。
- `cited_text` を全文の文字列検索だけで配置しない。同文が2回登場する場合、文字列一致だけでは位置を決められない。PDF ではページを開けることと、ページ内の正確な矩形を得ることも別である。
- `title`、`source`、引用本文は外部入力として escape する。内部 ID は認可付き resolver で解決し、任意の文字列をクリック可能 URL にしない。

### streaming の状態を保存する

実装案は、attempt ごと・response content の `index` ごとに text と citations を別々に蓄積する。text_delta だけを読む renderer では引用を落とす。`content_block_stop` はその block の受信終了であり、回答全体の完了と同じフラグにしない。

本文が届いてから citation が届くまでの表示は「生成中」として扱う。途中の `error` や切断で終わった場合は partial を残し、引用未着を「出典なしと確定」あるいは「引用検証済み」に変えない。正常な stream の終端を確認した後も、停止理由と task の要件を照合して完成品か判断する。`message_stop` だけで回答品質の合格にはしない。

新しい request で続行・再生成したら別 attempt にする。古い途中回答へ新しい引用の index を継ぎ足さない。未知の citation type は raw 情報を残して未対応として扱い、既知の char_location に読み替えない。

## 独自の評価案: 支持・網羅性・正しさを別に測る

評価対象の corpus snapshot、request manifest、生成回答、採点規則を固定する。retrieval の Recall@k が高いことも、引用 pointer が有効なことも、以下の検査を省く理由にならない。

| 層 | 検査すること | 合格しても残る問題 |
|---|---|---|
| 決定的な参照検査 | source版、型、index、範囲、表示対象が一致するか | 選んだ箇所が主張を支持するとは限らない |
| 根拠による支持 | 主張の数値・単位・否定・時点・条件を引用資料だけで導けるか | 元資料自体の誤りは残る |
| 回答の網羅性 | 問いに必要な項目・重要な例外を答えているか | 書いた内容の支持や正しさとは別問題 |
| 正しさ・source品質 | 信頼できる正解/レビュー基準と整合し、適切な版を使うか | citation の表示不具合は別途残り得る |

ALCE 型の citation recall と、正解項目を分母にした answer coverage を別名で記録する。後者は本書の製品評価案であり、ALCE の citation recall そのものではない。採点前に statement の分割規則、引用不要とする文、正解項目、部分点、集計単位を決める。主張を短く削った結果の支持率上昇を、質問への回答改善と取り違えない。

意味の検査はまず人手ラベルで較正し、自動 judge の不一致を保持する。数値、比較、複数資料の組合せ、例外条件を別の検証群にする。evidence が長くて judge の入力上限を超えたときは、末尾を黙って捨てて不支持とせず「採点不能」を残す。statement 0件や citation 0件で分母が0なら、機械的に100%へ埋めない。拒否/回答保留は別の結果区分とし、coverage の扱いも事前に定める。

### 独自の小例

原資料Aは「標準プランの保持期間は30日」、Bは「監査ログの保持期間は90日」とする。回答が「標準プランと監査ログはいずれも90日」と書き、Bを引用した場合、pointer は正しくても主張全体は不支持になる。

回答を「監査ログは90日」だけに変えれば、その一文の支持は成立する。しかし質問が両方の期間を求めていたなら、正解2項目中1項目しか答えておらず、本書の answer coverage は1/2である。生成した1文だけを分母にする citation recall と混ぜない。

## 採用前の受入れテスト案

以下は将来の RAG 実装の fixture 案であり、実際の API を呼んだ試験結果ではない。

| fixture | 確認する期待結果 |
|---|---|
| 前の message にdocument A、次にdocument B | Bのdocument_indexを直近messageだけで数え直さない |
| document 0とsearch_result 0を同時に利用 | 別の名前空間で正しいsourceへ解決する |
| custom block列[A,B,C]で範囲[1,2) | Bだけを対象とし、Cまで含めない |
| PDFが3ページ、引用が[3,4) | 第3ページだけ。end=4を架空の第4ページとして扱わない |
| 同じURLで原文v1からv2へ更新 | v1回答の座標をv2本文へ当てない |
| 同じ文章が同一sourceに2回登場 | 最初の文字列一致へ置き換えない |
| 絵文字・結合文字・CRLF正規化 | 未確認の座標単位や変換を検出し、誤highlightを避ける |
| citationの到着前にstreamが切断 | partialとして保存し、引用確定済みにしない |
| responseのblock 0とblock 1が別資料を引用 | 受信indexで対応し、引用一覧の到着順だけで割り当てない |
| 保持期間の独自小例 | pointer合格、全体支持は失敗。短縮回答はcoverage 1/2 |
| 無関係な引用を追加 | 引用数の増加を品質改善にしない |
| 根拠に関連する部分支持をjudgeが誤判定 | 自動判定を人手ラベルと照合し、誤差として記録する |
| citationsとoutput_config.formatを同時指定 | 取得時点の400契約を検出し、別経路へ設計を修正する |

## 版・ライセンス・未確認事項

- Claude の3 guide は2026-10-03 UTCに実ページを開いた rolling documentation で、ページの固定版・公開日は表記されていない。例中のモデル名を API全体の固定snapshotとは扱わない。本書の説明対象は取得時点の形状で、各cloud実装の完全な同等性を試験していない。
- ALCE は2023年12月のEMNLP出版版（pp. 6465–6488、DOI 10.18653/v1/2023.emnlp-main.398）を参照した。[出版情報ページ](https://aclanthology.org/2023.emnlp-main.398/)で公開月とCC BY 4.0を確認した。repository実装は解析していないためcommit固定の対象はない。
- Claude guideの本文でオープンライセンスを確認できず、catalogはunknown。資料のコード・図・データセット・設問は取り込まず、契約の要約と独自の設計例を記載した。独自案を含むため文書のtrustはprimary-source。
- 未確認: live API/SDKでの座標のUnicode単位、PDF抽出器差、引用品質・遅延の実測、stream復旧時の各SDK挙動、日本語NLIの精度、他providerへの移植性。sourceを固定することで、誤った原文が正しくなるわけではない。
- 取得日からofficial_docs / maintainer_articleの90日TTLで期限を2027-01-01とした。`evals/knowledge/ai-engineering.json`で実行するのは本文を見つける検索evalであり、上表の受入れテストや引用品質の実証ではない。

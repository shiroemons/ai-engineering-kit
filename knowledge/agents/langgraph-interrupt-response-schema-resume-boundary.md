---
{
  "id": "agents-langgraph-interrupt-response-schema-resume-boundary",
  "title": "LangGraph 1.2.12 interrupt response_schema: 表示・型検証・resume永続化の境界",
  "kind": "knowledge",
  "technology": "agents",
  "version": "Python LangGraph 1.2.12 (2026-09-21); released commit 49cce0ca852be4cfb567a1cbe0e511ff325a1682; unversioned docs checked 2026-10-02 UTC",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "langgraph",
    "interrupt",
    "response_schema",
    "resume",
    "human-in-the-loop",
    "checkpoint",
    "validation",
    "idempotency"
  ],
  "sources": [
    {
      "id": "langgraph-interrupt-release-1-2-12-20261002",
      "url": "https://github.com/langchain-ai/langgraph/releases/tag/1.2.12",
      "type": "release_notes"
    },
    {
      "id": "langgraph-interrupt-docs-20261002",
      "url": "https://docs.langchain.com/oss/python/langgraph/interrupts",
      "type": "official_docs"
    },
    {
      "id": "langgraph-interrupt-repository-49cce0c-20261002",
      "url": "https://github.com/langchain-ai/langgraph/tree/49cce0ca852be4cfb567a1cbe0e511ff325a1682",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/ai-engineering.json"
  ]
}
---

# LangGraph interrupt response_schema の入力境界

## 問いと採用判断

agent が承認待ちで停止した後、人の回答を型付きフォームで集めれば、その回答を安全に実行へ渡せるか。Python LangGraph **1.2.12** は `interrupt(response_schema=...)` を追加したが、同じ JSON Schema を画面に表示できても、引数が辞書か Python の型かで runtime validation が異なる。

採用判断は、フォーム表示、回答の型検証、承認者の権限、外部操作の重複防止を別々に確認すること。前二者は以下の版付き実装を根拠とし、後二者の構成は本書の設計案として区別する。LLM の structured outputs と異なり、ここで検証する対象は停止した graph へ戻す人・client の resume 値である。

## 版と出典

- [1.2.12 release](https://github.com/langchain-ai/langgraph/releases/tag/1.2.12) の公開は **2026-09-21T14:43:40Z**。release notes は `response_schema` 追加を1.2.11からの変更として記載している。GitHub release APIでも日時と対象commitを照合した。「最新全般」の保証ではなく、このリリースを適用対象にする。
- [公式 Interrupts 文書](https://docs.langchain.com/oss/python/langgraph/interrupts) は機能に `langgraph>=1.2.12` が必要と明示する。文書自体の公開日・固定版は表示されていないため、2026-10-02 UTC取得時点の記述として扱う。
- 実装分析は **49cce0ca852be4cfb567a1cbe0e511ff325a1682** に固定した。[types.py](https://github.com/langchain-ai/langgraph/blob/49cce0ca852be4cfb567a1cbe0e511ff325a1682/libs/langgraph/langgraph/types.py)、[test_interruption.py](https://github.com/langchain-ai/langgraph/blob/49cce0ca852be4cfb567a1cbe0e511ff325a1682/libs/langgraph/tests/test_interruption.py)、[pyproject.toml](https://github.com/langchain-ai/langgraph/blob/49cce0ca852be4cfb567a1cbe0e511ff325a1682/libs/langgraph/pyproject.toml) を確認し、パッケージ版1.2.12と照合した。
- 固定commitの[LICENSE](https://github.com/langchain-ai/langgraph/blob/49cce0ca852be4cfb567a1cbe0e511ff325a1682/LICENSE) はMIT。release proseとrendered websiteへの適用ライセンスは未確定としてcatalogにunknownを記録した。本文は独自要約であり、ソースコードは転載していない。

## 確認した契約: schema の渡し方で保証が変わる

公式文書と `types.py` の `interrupt` 実装を照合した結果は次のとおり。

| response_schema に渡すもの | clientに渡る情報 | 再開時の検証・返り値 |
|---|---|---|
| 省略 / None | schemaなし | resume 値をschema検証せず返す |
| 生の JSON Schema dict | 辞書をそのまま公開 | 辞書に対するvalidationは行わず、resume値をそのまま返す |
| Pydantic model class | TypeAdapterで生成したJSON Schema | 検証済みmodel instanceを返す |
| typing_extensions.TypedDict | 型から生成したJSON Schema | 検証済みdictionaryを返す |
| dataclass | 型から生成したJSON Schema | 検証済みdataclass instanceを返す |

`Interrupt.response_schema` は質問などを入れる `Interrupt.value` とは別フィールドである。したがって、質問payloadの中にschemaらしい辞書を置くことと、この引数を指定することも同じではない。既存呼び出しで引数を省略しても、回答の自動検証が新たに有効になるわけではない。

型指定では **Pydanticのcoercion規則** が適用される。型が付いたことだけで、型変換禁止や未知フィールド拒否まで保証されたとは扱わない。固定commitの `test_interrupt_response_schema` は追加の `extra` field を渡し、None / raw dictでは保持され、テスト内のmodel / TypedDict / dataclassでは除かれる期待値を持つ。これはそのテストで使った型定義の結果であり、全Pydantic設定の共通仕様ではない。

## 確認した契約: 不正回答は訂正して再開できる

型付き回答が不正なら `pydantic.ValidationError` が発生する。公式文書は、同じ `thread_id` に訂正した `Command(resume=...)` を渡して再試行できると説明している。「再開要求を送れた」ことを「回答が採用された」ことと同一視しない。

固定commitで確認した実装上の要点は以下。これはコード読解であり、DB障害を含むend-to-end実験の結果ではない。

- Python型指定では `TypeAdapter` を作る。`None` / dictではadapterを作らない
- 過去のresume値を読む経路でも `validate_python` を呼び、検証が通ってから `RESUME` write を送る
- 複数回答を持つ経路で送るのは、そのinterruptが消費した位置までの `scratchpad.resume[:idx+1]`。後続interruptの未検証回答まで先に永続化しない
- 新しい回答の経路でも、検証してから元の値をresume listへ追加する。返す値は検証済みobjectだが、resume履歴へ追加する値はraw valueである

[同版の回帰テスト](https://github.com/langchain-ai/langgraph/blob/49cce0ca852be4cfb567a1cbe0e511ff325a1682/libs/langgraph/tests/test_interruption.py) は、直接resumeとinterrupt IDをキーにしたID-map resumeの双方で、不正な `approved` の後に有効な回答を入れ直す期待動作を持つ。さらに、同一nodeの一つ目のinterruptを通過した後、二つ目で不正回答が発生しても訂正できるケースがある。本調査ではテストを読んだだけで実行していない。

この範囲から「不正値はどのログ・外部保存先にも残らない」とは言えない。検証対象はinterruptが消費するresume履歴の書き込み順であり、入力ログ、独自checkpointer、外部処理までの監査ではない。

## 確認した契約: resume はnodeの先頭から再実行する

公式文書は、checkpointerとthread IDを必要条件とし、再開時には停止したnodeの先頭から処理をやり直すと説明する。interrupt直前の命令位置へそのまま復帰する仕組みではない。したがって、`interrupt` より前の副作用も再実行される。複数interruptのresume listはtaskごとに持ち、node内では呼び出し順で対応するため、途中のinterruptを条件によって飛ばしたり順序を変えたりしない。

固定commitのコードは、再実行された `interrupt` に渡された型からadapterを構築する。これは観察できた実装であり、停止時に表示したschemaと将来の配備後のschemaを自動で同一に固定する保証は、この関数から確認できない。停止中のgraphをまたぐschema変更を、単なるフォーム修正として扱わない。

## 独自の設計案: 承認入力と実行を分ける

以下は上の契約を使うアプリケーション側の提案であり、LangGraphが提供する認可機能の記述ではない。

1. **表示と検証を明示する。** Python型をsource of truthにできる場合は型指定を使う。raw JSON Schemaを採用するなら、回答を実行へ渡す前のserver側validatorと失敗時の応答を別途設ける。UIが正しい入力だけを送る前提にしない
2. **承認の意味を絞る。** 承認booleanだけでなく、対象操作のID・版、対象リソース、承認者、期限をアプリ側で照合する。型検証の成功は認証・認可の成功ではない。未知fieldとcoercionを許容するかも承認ポリシーとして決める
3. **停止中の契約を固定する。** 回答schemaと操作内容のversion / fingerprintを保存し、再開時に一致を確かめる。異なる配備で契約が変わったら、新しい説明と回答を求める。このfingerprint照合は本書の追加案であり、response_schemaの標準機能ではない
4. **訂正と実行を分ける。** ValidationErrorは入力訂正へ戻し、外部操作の自動retryに流さない。`thread_id` とpending interrupt IDを維持し、並列branchの回答はID-mapで対応付ける
5. **副作用は承認後に置き、別途冪等化する。** 承認前のnodeには読み取り・表示準備を中心に置く。承認後の外部操作にも操作IDによる重複排除や結果照会を設ける。配置変更だけで障害再実行を含むexactly-onceになったとは主張しない

## 導入時に自分の環境で確認する項目

| ケース | 確認する結果 |
|---|---|
| raw dict と Python型で同じフォームを表示 | 前者をruntime validation済みと誤認していない |
| 不正なboolean / 未知field / 文字列からのcoercion | 自分の型・validator設定で許可/拒否が意図どおり |
| 不正値の後に訂正、直接resumeとID-map resume | 同一threadで回復し、外部操作はまだ行われない |
| 同じnodeの二つ目のinterruptだけ不正 | 先行回答は維持され、後続の不正値を訂正できる |
| 停止中にschemaや操作内容が変わる | アプリ独自のversion照合が古い回答を検出する |
| 再開を繰り返す / 外部操作直後に失敗 | 承認前の副作用と承認後の重複実行を検出できる |

これらは提案する実行検証であり、このリポジトリに追加する検索evalの合格とは別である。

## 限界と再確認期限

- 対象はPython LangGraph 1.2.12。JavaScript版の `responseSchema`、LangGraph Serverの全serialization経路、Studioの配備版ごとの対応は未検証
- 固定commitのsourceと回帰テストの静的確認まで。LangGraphのインストール、checkpointer backend別テスト、障害注入、認可実装の検証は行っていない
- `response_schema` の追加は外部操作の原子性、認可、期限、重複排除を追加するものとして評価していない。これらはアプリで別途検証する
- 取得日はすべて **2026-10-02 UTC**。release notesのTTL 30日を含むため、明示期限は **2026-11-01**。期限はリリース日付の真偽が変わる日ではなく、適用版・文書・実装の再照合日である

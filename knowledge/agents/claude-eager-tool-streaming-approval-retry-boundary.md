---
{
  "id": "agents-claude-eager-tool-streaming-approval-retry-boundary",
  "title": "Claude tool streaming: Python SDK 1.9.0 の eager実行・承認延期・再送境界",
  "kind": "knowledge",
  "technology": "agents",
  "version": "anthropic Python SDK 1.9.0 (2026-09-28), commit a7285e919ab79998d9380b3b57f6315b7860b8d8; unversioned Claude API guides checked 2026-10-02 UTC",
  "tags": [
    "research-domain:ai-engineering",
    "agents",
    "claude",
    "streaming",
    "eager_input_streaming",
    "run_tools_eagerly",
    "defer_tool_call",
    "partial_json",
    "approval",
    "idempotency",
    "failure-handling"
  ],
  "sources": [
    {
      "id": "anthropic-python-eager-tools-release-1-9-0-20261002",
      "url": "https://github.com/anthropics/anthropic-sdk-python/releases/tag/v1.9.0",
      "type": "release_notes"
    },
    {
      "id": "anthropic-python-eager-tools-a7285e9-20261002",
      "url": "https://github.com/anthropics/anthropic-sdk-python/tree/a7285e919ab79998d9380b3b57f6315b7860b8d8",
      "type": "github_repository_analysis"
    },
    {
      "id": "claude-fine-grained-input-streaming-20261002",
      "url": "https://platform.claude.com/docs/en/agents-and-tools/tool-use/fine-grained-tool-streaming",
      "type": "official_docs"
    },
    {
      "id": "claude-streaming-completion-recovery-20261002",
      "url": "https://platform.claude.com/docs/en/build-with-claude/streaming",
      "type": "official_docs"
    },
    {
      "id": "claude-tool-reference-streaming-scope-20261002",
      "url": "https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-reference",
      "type": "official_docs"
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

# Claude tool streaming の受信・実行・承認境界

## 問いと採用判断

tool引数を少しずつ表示する仕組みに、返信完了前のtool実行を追加したとき、人の承認や失敗時の再送をどこで止めるか。

Python SDK **1.9.0** の `run_tools_eagerly` は実行時刻を変える。APIの `eager_input_streaming` は入力の配送粒度を変える。両者を同じ「streaming有効化」として扱わない。副作用のあるcallを一律にeager実行するより、重複しても安全な処理だけを早め、承認が必要な処理は実行前に止める、というのが本書の採用判断である。

二つは独立したオプションであり、`eager_input_streaming` は `run_tools_eagerly` の前提条件ではない。固定版のtools.mdは `stream=True` と `run_tools_eagerly=True` で実行を早める例を示しており、fine-grained入力配送を要求していない。

[一般的なtool権限・失敗ループ](tool-permissions-failure-loop.md)を前提とし、本書は新しいSDKの具体的なイベント順序、延期解除、途中終了に限定する。MCPのtransportや会話途中のtool定義変更は扱わない。

## 適用版・取得方法・ライセンス

- [v1.9.0 release notes](https://github.com/anthropics/anthropic-sdk-python/releases/tag/v1.9.0) は **2026-09-28** 付で、返信stream中にtoolを実行する機能の追加を記載している。既定動作の全面変更ではなく、以下のopt-inについて調べた
- 実装はrelease commit **a7285e919ab79998d9380b3b57f6315b7860b8d8** に固定し、[pyproject.toml](https://github.com/anthropics/anthropic-sdk-python/blob/a7285e919ab79998d9380b3b57f6315b7860b8d8/pyproject.toml) の `version = "1.9.0"` と照合した。より新しい版にも同じ挙動が続くという保証はしない
- 固定commitの [tools.md](https://github.com/anthropics/anthropic-sdk-python/blob/a7285e919ab79998d9380b3b57f6315b7860b8d8/tools.md)、[_beta_runner.py](https://github.com/anthropics/anthropic-sdk-python/blob/a7285e919ab79998d9380b3b57f6315b7860b8d8/src/anthropic/lib/tools/_beta_runner.py)、[_beta_messages.py](https://github.com/anthropics/anthropic-sdk-python/blob/a7285e919ab79998d9380b3b57f6315b7860b8d8/src/anthropic/lib/streaming/_beta_messages.py)、[test_runner_eager_tools.py](https://github.com/anthropics/anthropic-sdk-python/blob/a7285e919ab79998d9380b3b57f6315b7860b8d8/tests/lib/tools/test_runner_eager_tools.py) をGitHub connectorで取得し静的に照合した
- [LICENSE](https://github.com/anthropics/anthropic-sdk-python/blob/a7285e919ab79998d9380b3b57f6315b7860b8d8/LICENSE) はMIT。Web文書・releaseページの文章のライセンスは別に確定できず、catalogではunknownとした。本文は独自要約であり実装コードを転載していない
- API guideは **2026-10-02 UTC** に実際に開いた版固定のない文書。公開日やper-tool fieldの初出日は確認できていない。SDK調査の対象はPythonの `client.beta.messages.tool_runner` で、Managed Agentsや他言語SDKの実行契約には一般化しない

## 確認したAPI契約: 入力配送を早めてもcallは実行されない

[Fine-grained tool streaming](https://platform.claude.com/docs/en/agents-and-tools/tool-use/fine-grained-tool-streaming) が説明する `eager_input_streaming` は、user-defined toolの引数をserver側のbuffering・JSON検証を待たずに配送する設定である。request自体のstreamingも必要となる。

| toolのfield | 旧 `fine-grained-tool-streaming-2025-05-14` headerなし | 旧headerあり |
|---|---|---|
| 省略 | 通常のbuffered streaming | fine-grained streaming |
| `true` | fine-grained streaming | fine-grained streaming |
| `false` | buffered streaming | 明示falseが優先しbuffered streaming |

同guideは全モデルについてClaude API、Amazon Bedrock、Claude Platform on AWS、Google Cloud、Microsoft Foundryでの対応を記載する。本調査で確認したのはこの文書上の対応範囲であり、各platform・SDK・モデルを組み合わせた実測ではない。

[Tool reference](https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-reference) の適用先はuser-defined toolsのみ。`computer_toolset_20260801` / `browser_toolset_20260801` と旧headerを併用するとAPIは拒否する。これらのmember inputは一つの完全な `input_json_delta` で届く。user-defined tool向けfieldをtoolset entryやmember `configs` に移植して解決しない。

受信時の `input: {}` はplaceholderであり、完成した空引数ではない。`partial_json` を連結した結果も不正JSONになり得る。SDK内部の途中snapshotがobjectに見えることを、完全なraw JSONの証明にしない。

## 確認したSDK契約: block終了だけではeager実行しない

以下は1.9.0固定commitの `EagerToolCalls.track` と同期・非同期runner、対応テストの観察である。

- `run_tools_eagerly=True` は `stream=True` を必要とし、stream省略・falseとの組み合わせは `ValueError`。このオプションはclient側に留まりAPIへ送られない
- 既定falseでは、通常のtool-use返信のtoolは、その返信を扱う外側のloop body終了後に実行する
- trueでもcall自身の `content_block_stop` ではまだ実行しない。切れた最後のcallにもblock終了が届くためである
- 通常経路では、次の `content_block_start`、または `stop_reason: "tool_use"` を持つ `message_delta` により、モデルがそのcallの先へ進んだと判定する
- そのイベントをcallerへ渡した後、callerがさらに読み進めた際に実行する。実装はまず次イベントを取得するので、その読み取りが例外になれば待機callを開始しない
- callは一つずつ実行し、実行中はstreamの読み進めも待つ。async runnerでもこの経路は並列実行器ではない

例としてA、Bの順にcallが来る通常経路は、Aのblock終了、Bのblock開始をcallerへ渡す、次イベントを取得できるとA実行、という順になる。Bは `message_delta(tool_use)` 後の読み進めで実行できる。このため「blockを受け取ったら即実行」と「返信全体が終わってから実行」の二択ではない。

[Streaming messages](https://platform.claude.com/docs/en/build-with-claude/streaming) のblock `index`、block完了、最終 `message_stop` は別の情報である。stream中にはerrorも届く。生SSEを独自処理するなら、同SDKのeager判定をAPIの規範要件と取り違えず、自分の実行条件を別に定義する。

## 確認したSDK契約: deferは承認待ちを永続化しない

`defer_tool_call(block_or_id)` はその返信のcallを**外側のloop body終了まで延期**する。同じ返信の他callはeager実行を続ける。以下は固定版の注意点である。

| 操作・状態 | 確認した挙動 |
|---|---|
| callの `content_block_start` でdefer | 入力完成前に保留を登録できる。開始時点では引数内容を承認しない |
| `deferred_tool_calls` を読む | blockが終了した保留callがモデル順に並ぶ。streamを読み終えた後に確認する |
| 外側のrunner loopを継続 | 通常のtool-use返信なら保留callも実行対象になる |
| `generate_tool_call_response()` を呼ぶ | 結果を読むだけではなく、まだ実行していない保留callも実行する |
| **内側のevent loopだけbreak** | runnerが続きを消費し、残りcallを実行し得る。拒否操作にならない |
| 外側のrunner loopをbreakして再開しない | 以後の保留callを実行しない。既にeager実行したcallは取り消せない |
| 実行済みcallへdefer / loop開始前のdefer | 前者は遅すぎ、後者には現在の返信がないため効かない |

`text_stream`、`get_final_message()`、`until_done()` でも内部streamは消費され、eager実行が進む。したがって「画面にはtextしか表示していないから、toolはまだ動いていない」と考えない。eventを観察する承認処理を省略したまま、これらのhelperを呼ぶことも危険になる。

deferは一返信だけのclient状態である。翌返信の同名callまで自動で承認待ちにはしないし、承認者、承認期限、許可された対象・引数を保存する仕組みでもない。

## 確認した失敗境界: 途中終了と結果未配送

### 最後のcallを止めても、先行callは実行済みになり得る

固定テスト `test_never_runs_the_call_that_was_cut_off_by` は、二つ目のcallが `max_tokens` / refusal / fallbackにより途切れるケースで、先行Aだけが実行されることを期待する。返信全体が失敗したから副作用ゼロ、と解釈しない。

fallback blockを見た後は、その返信の残りcallをeager実行せず、通常の返信終了後の処理に回す。これは既に実行したcallのrollbackではない。tools.mdも、途中終了や履歴置換で結果がモデルへ届かなければ、モデルが同じ操作を再び要求し得ると注意している。

### 保留一覧は実行可能性の保証ではない

`deferred_tool_calls` は `max_tokens` で終わった返信にも保留callを載せる。自動runner loopはその終端理由で停止する一方、**明示的な `generate_tool_call_response()` はstop_reasonを理由に実行を止めない**。対応テスト `test_deferred_tool_calls_lists_the_held_calls_of_a_reply_that_is_cut_off` は、保留一覧を読んだ後にこのmethodを呼ぶとtoolが動くことを確認する構成である。

`_beta_messages.py` は入力deltaを `jiter.from_json` のpartial modeで途中解析する。通常は `partial_mode=True`、旧fine-grained headerありでは `"trailing-strings"` を使う。閉じ括弧などが欠けても途中snapshotの一部はobjectとして見え得る。最終 `content_block_stop` でtool inputを完全JSONとして再parseする処理は、この固定版の該当branchにはない。完全な引数の承認にsnapshotだけを使わない。

### 一返信内の重複抑止は永続的なexactly-onceではない

eager runnerは実行済み結果をその返信の `tool_use.id` で再利用する。同じ返信で結果生成を繰り返してもcallを再実行しないが、次の返信で同じIDが現れた場合には前の結果を流用しないテストがある。プロセス再起動、別request、外部サービスcommitと結果返送の間の障害まで覆う永続deduplicationではない。

tool例外は `is_error: true` の結果へ変換する経路を持つ。しかし外部サービスへの書き込み後に例外が発生した場合、そのerror表現は「外部状態が変更されなかった」という証拠にはならない。

## 独自の設計案: 表示・承認・外部commitを分離する

以下はAPIやSDKの保証ではなく、上記の境界を踏まえたアプリ設計案である。

1. **eager実行を許すtoolを明示する。** 通常の読取でも課金や機密送信を伴うなら無条件に安全扱いしない。監査上返信全体の完了を待つ必要がある処理では、`run_tools_eagerly=False` を選ぶ
2. **承認対象はblock開始でまずdeferする。** 入力内容によって許可が変わるなら、途中引数で早期許可せず全callをいったん保留する。表示は暫定表示とし、最終値・対象・操作を揃えて承認する
3. **実行に入る直前に別のgateを置く。** 完全なraw JSON parse、schema、業務制約、認可を順に確認する。raw deltaは `index` 別に蓄積し、blockのIDと対応させ、総byte数・深さ・受付時間にも上限を設ける。partial parseが値を返しただけでは通さない
4. **UIの「保留中」を制御状態に接続する。** 承認を別画面で待つ間、外側loopの継続や `generate_tool_call_response()` が走らないようにする。deferだけを置いて自然にloopを抜ける実装は承認gateにならない。長時間の人待ちには承認対象と状態を別途永続化し、古い承認を別引数へ使い回さない
5. **実行記録をモデル会話から独立させる。** request/返信ID、tool call ID、正規化した対象・引数、承認情報、業務operation ID、外部結果、モデルへの結果配送状態を区別する。call IDだけに依存せず、同じ業務操作の再送を重複抑止できる設計を選ぶ
6. **不明な結果を自動成功・自動再実行にしない。** stream切断後は未開始、失敗確定、成功確定、結果不明を分ける。結果不明の副作用は外部状態の照合から始める。取消要求やerror結果だけをrollbackの証拠にしない
7. **遅延短縮を分けて測る。** first fragmentまでの時間、入力完成までの時間、tool開始・終了、最終回答までの時間を別指標にする。eager実行中にstream消費が待つ特性も含め、体感遅延と副作用リスクを比較する

不正JSONを返す際にraw入力を診断へ残す場合も、秘密や個人情報をそのままログへ保存しない。修復要求を出したことは、修復された新しい引数を自動承認したことにはしない。

## 回帰試験に落とす境界と未確認事項

次の試験は本アプリへ導入する際の設計案であり、本リポジトリ内にSDK実行テストを実装したわけではない。

- A完了後にBを `max_tokens` で切り、Aの副作用とBの未開始を別に記録する
- AをdeferしBを先に動かしたとき、実行順と結果返送順を混同しない
- 拒否で内側だけbreakした誤実装を検出し、外側の停止・再開禁止を確認する
- 保留一覧にある途中callへ結果生成helperを誤って呼べないことを確認する
- raw JSONが途中でもsnapshotには必要fieldが見える入力を与え、承認・実行を拒否する
- 副作用成功後・結果返送前で切断し、再要求されても同じ業務操作を二重commitしない
- `text_stream` や `get_final_message()` へのUI切り替えが承認gateを迂回しないことを確認する

固定版の実装・upstreamテストは読んだが、upstream pytest、実API call、各hosted platform、後続SDK版での再現は未実施。ネットワーク切断時の外部副作用を含むend-to-endの保証はない。Webからの固定GitHub blobページ2件はcache missになったが、同じ固定refのファイルをGitHub connectorで取得して照合できた。新規evalは文書が検索で発見できることを検証するものであり、SDK契約や安全性の実証ではない。

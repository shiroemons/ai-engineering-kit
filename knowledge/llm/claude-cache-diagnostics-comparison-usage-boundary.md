---
{
  "id": "llm-claude-cache-diagnostics-comparison-usage-boundary",
  "title": "Claude cache diagnostics: GA移行・比較不能・実cache hitの判定境界",
  "kind": "knowledge",
  "technology": "llm",
  "version": "Claude API cache diagnostics GA (2026-09-23), fingerprint opt-in change (2026-09-09); former cache-diagnosis-2026-04-07 beta header; official docs checked 2026-10-03 UTC",
  "tags": [
    "research-domain:ai-engineering",
    "claude-api",
    "cache-diagnostics",
    "previous_message_id",
    "cache_miss_reason",
    "fingerprint",
    "usage",
    "migration",
    "inconclusive"
  ],
  "sources": [
    {
      "id": "claude-cache-diagnostics-release-20260923-verified-20261003",
      "url": "https://platform.claude.com/docs/en/release-notes/overview",
      "type": "release_notes"
    },
    {
      "id": "claude-cache-diagnostics-contract-20261003",
      "url": "https://platform.claude.com/docs/en/build-with-claude/cache-diagnostics",
      "type": "official_docs"
    },
    {
      "id": "claude-messages-diagnostics-api-20261003",
      "url": "https://platform.claude.com/docs/en/api/messages/create",
      "type": "official_docs"
    },
    {
      "id": "claude-cache-usage-troubleshooting-20261003",
      "url": "https://platform.claude.com/docs/en/build-with-claude/prompt-caching",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/ai-engineering.json"]
}
---

# Claude cache diagnostics の比較と実cache hitを分ける

## 問いと採用判断

会話の cache read が減ったとき、履歴の変更、比較データの欠落、cache 自体の不使用をどう切り分けるか。診断機能を有効にしたつもりでも、旧 beta header だけを送る実装や、応答の `null` を成功扱いする監視では判定を誤る。

本書は Claude API の診断契約と移行に絞る。TTL・breakpoint の一般設計は[既存の prompt caching 文書](prompt-caching-breakpoints-ttl-scope.md)、tool 定義を履歴へ追記する設計は[inline tools 文書](../agents/claude-inline-tools-mcp-list-pinning.md)を参照する。診断で示された差分が業務上必要なら、その変更を消して cache を優先する必要はない。

## 確認した変更とAPI境界

[公式 release notes](https://platform.claude.com/docs/en/release-notes/overview) の3日付を区別する。

- **2026-09-09:** fingerprint 保存の opt-in は request の `diagnostics` object。`cache-diagnosis-2026-04-07` header だけでは保存されず、その response ID を次回指定すると `previous_message_not_found` になる。
- **2026-09-18:** beta header を送る応答に `diagnostics` field が常に現れるようになった。request object を省いた場合は `null`。
- **2026-09-23:** Claude API で GA。旧 header は不要になったが、引き続き受理される。`POST /v1/messages` の応答は同 field を常に含む。header の日付をGA日と呼ばない。

[Create a Message API reference](https://platform.claude.com/docs/en/api/messages/create) は、request の `diagnostics.previous_message_id` が直前の Messages response の `id`（`msg_...`）を指し、最初は `null` を渡す契約を示す。application の会話ID、HTTP request ID、tool call IDで置き換えない。初回も object 自体を省かず、後続で比較できる状態を作る。

同 reference では Messages API は stateless な複数turn会話にも使われる。`previous_message_id` は履歴送信を省略する conversation storage APIではない。必要な `messages` は引き続き送る。診断の追加だけで既存の `cache_control` を代替するとも扱わない。

## 診断応答の読み分け

[Cache diagnostics の Response format / Limitations](https://platform.claude.com/docs/en/build-with-claude/cache-diagnostics#response-format) で確認した要点は次のとおり。

- `diagnostics: null`: opt-inなし、初回、または比較済みで差分なし。応答だけでは三者を区別できない。
- `cache_miss_reason: null` を持つobject: 比較が応答生成に間に合わなかった未確定状態。外側の `null` と別物。
- `model_changed` / `system_changed` / `tools_changed` / `messages_changed`: 最初の差分を報告する。
- `previous_message_not_found` / `unavailable`: 比較不能。履歴変更の証明ではない。

streamingでは `message_start` に診断が入る。対象はClaude APIのみで、比較元は同一organization・workspace。fingerprintの保持時間と長い履歴の比較範囲には制限があり、診断はbest-effort。`cache_missed_input_tokens` はbyte長由来の推定で、課金token数ではない。

## usage と組み合わせる

[Prompt caching の Tracking cache performance / Troubleshooting](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) では `usage.cache_read_input_tokens` が読み取り、`cache_creation_input_tokens` が書き込みの観測値になる。モデル別の最小長を満たさないpromptは、cache指定があってもcacheされない場合がある。期限やprefix一致条件も別途確認する。

したがって本書では、**「比較できたか」「差分があるか」「実際にcacheを読んだか」**を別軸で記録することを推奨する。以下は公式responseに追加されるfieldではなく、アプリ側の判定案である。

1. 送信時点で、diagnostics objectを付けたか、比較元IDが実値か、初回かを記録する。応答の `null` を単独で `cache_hit=true` に変換しない。
2. `previous_message_not_found` は診断経路の欠測として扱う。前回のopt-in、workspace、間隔を確認し、履歴を書き直さない。
3. 未確定objectと `unavailable` は判定保留にする。通常の次turnでも診断を継続するが、この理由だけで業務requestを再実行しない。
4. `*_changed` があれば、差分の意図を確認する。実際のread量が残っていれば部分的な再利用も考えられるため、全面missと決めつけない。
5. 比較が成立して差分なしでもread量が少なければ、間隔・cache設定・モデル最小長を調べる。「TTL切れを証明した」とまでは記録しない。
6. 推定された損失量とusageは別列に置く。`cache_missed_input_tokens` をusageへ足したり、損失額の請求根拠にしたりしない。

診断の分類名に `cache_miss` が含まれることと、実cache missの測定は同義ではない。利用者が意図したモデル変更を「バグ」として自動修復する運用も避ける。

## 本書の移行手順と未実施の境界テスト案

以下は上記契約を実装へ落とすための独自提案で、実行済みのAPI試験ではない。

- **送信adapter:** 初回objectと後続IDを保持し、SDKのserialize後のrequestでfieldが落ちないことを確認する。会話が分岐する場合、全worker共通の「最後のID」ではなく、その枝が比較したい直前responseを結び付ける。
- **受信adapter:** field省略を許す過去ログ、外側null、内側null、既知reason、将来の未知reasonをfixture化する。未知値を `messages_changed` に丸めず、生値と判定不能を残す。GAへの移行判定と過去ログの読込互換性を分ける。
- **streaming:** `message_start` を捨てるtext専用処理から診断を回収できるか確認する。後続イベントや取得APIで未確定結果を必ず回収できる、という保証は置かない。
- **判定テスト:** 初回nullをhit率の成功に数えない。object省略で応答nullになったケースも「差分なし」に数えない。`previous_message_not_found` と `unavailable` は、prefix不一致率の分母から分離する。
- **比較テスト:** tool順序やsystemを一つずつ変え、検出された最初の差分を直した後にも別の差分が残り得ることを確認する。一件のreasonを全差分一覧として表示しない。
- **可用性:** 診断の未確定を業務失敗へ昇格させない。観測を完全にするためだけの再送は、追加推論やtool実行を増やす可能性があるため行わない。
- **監視:** opt-in率、比較成立率、未確定率とcache read/write量を別々に集計する。診断対象を減らした結果だけでcache不具合が減ったと解釈しない。raw prompt全体をログへ複製することを移行要件にしない。

## 未確認事項・出典・鮮度

4件の公式一次資料を2026-10-03 UTCに開いて確認した。release notesは上記日付のentryが対象。他3件はrolling documentationで独立した公開日・改訂日を確認できず、取得日で識別する。SDKの最低対応版、全モデルの対応、fingerprintの正確な保持秒数、比較上限、診断結果の再取得API、実環境のhit率・性能・課金は未検証。未確定結果が後から必ず確定するとは主張しない。

documentationにopen licenseの表示を確認できないため、各sourceのlicenseは `unknown`。リンク付きの独自要約と設計提案だけを残し、公式サンプルの転載やmoduleへの取り込みは行わない。repository分析ではなくcommit pinはない。release_notesのTTL 30日が最短なので、明示期限は2026-11-02 UTC。診断機能を他platformへ展開する際は、GAという表現だけで対応を推測せず公式対応表を再確認する。

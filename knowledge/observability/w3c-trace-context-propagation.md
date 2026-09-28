---
{
  "id": "observability-w3c-trace-context-propagation",
  "title": "W3C Trace Context の伝搬範囲と保証 traceparent tracestate サンプリングと相関",
  "kind": "knowledge",
  "technology": "observability",
  "version": "Trace Context Level 1 REC 2021-11-23 (version 00) / Level 2 CRD 2024-03-28 (verified 2026-09-28)",
  "tags": [
    "research-domain:quality-operations",
    "observability",
    "traceparent",
    "tracestate",
    "trace-context",
    "w3c",
    "sampling",
    "sampled",
    "propagation",
    "correlation",
    "distributed-tracing",
    "random-trace-id"
  ],
  "sources": [
    {
      "id": "w3c-trace-context-1-rec-20211123",
      "url": "https://www.w3.org/TR/2021/REC-trace-context-1-20211123/",
      "type": "official_docs"
    },
    {
      "id": "w3c-trace-context-2-crd-20240328",
      "url": "https://www.w3.org/TR/2024/CRD-trace-context-2-20240328/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active"
}
---

# W3C Trace Context の伝搬範囲と保証 traceparent tracestate サンプリングと相関

サービス境界を越えたトレース相関のため、`traceparent` と `tracestate` をどこまで転送・変更してよいかの保証範囲を定める。以下は W3C Trace Context Level 1 勧告 (2021-11-23、version 00) と Level 2 草案 (2024-03-28) に記載された事実と、それを組み立てる設計案を分けて書く。2件はいずれも 2026-09-28 時点の検証済み目録に基づく。

## 要点（公式情報に記載された事実）

### traceparent の形式と禁止値（Level 1）

- `traceparent` の形式は `version-trace-id-parent-id-trace-flags` の4部構成である。`trace-id` は32桁、`parent-id` は16桁の小文字16進数である。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)
- 全ビットがゼロの `trace-id` と `parent-id` は禁止され、`version` の値 `ff` は禁止されている。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)

### sampled フラグは推奨であり強制ではない（Level 1）

- `sampled` フラグは `trace-flags` の最下位ビット (LSB) であり、サンプリング判断の推奨 (recommendation-only) である。発行側は判断を反映すること (SHOULD)、受信側は尊重すること (SHOULD) が求められる。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)
- `sampled` の値を変更する場合は新しい `parent-id` を発行しなければならない。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)

### tracestate は任意の不透明な同伴ヘッダー（Level 1）

- `tracestate` は任意 (optional) の同伴ヘッダーであり、ベンダー固有の相関データを運ぶ不透明 (opaque) な値として扱われる。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)
- メンバー数は最大32であり、少なくとも512文字以上を伝搬できること (SHOULD) が求められる。サイズ超過時の切詰めはエントリ単位 (whole-entry) で行う。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)

### 伝搬の保証範囲：転送・不変・失敗時の分離（Level 1）

- 両ヘッダーは転送しなければならない (MUST forward)。`traceparent` を変更せずに転送する場合、`tracestate` を変更してはならない (MUST NOT)。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)
- `traceparent` の構文解析に失敗したら `tracestate` を構文解析してはならず (MUST NOT)、`tracestate` の失敗は `traceparent` の解釈に影響させてはならない (MUST NOT)。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)
- `traceparent` を伴わない単独の `tracestate` は破棄しなければならない (MUST be discarded)。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)
- 許される変更は `parent-id` の更新、`sampled` の変更（新 `parent-id` 付き）、トレースの再開 (restart)、バージョンダウングレード (downgrade) に限られる。[Level 1](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/)

### Level 2 の追加：random-trace-id フラグとヘッダー・tracestate 規則（CRD）

- Level 2 は Level 1 の保証を継承し、`random-trace-id` フラグ (`0x02`) を追加する。右端7バイトをランダム生成した `trace-id` はこのフラグを立てること (SHOULD) が求められ、トレース開始者はそれに従って生成しなければならず (MUST)、同じ `trace-id` を継続する参加者はフラグ値を保持しなければならない (MUST)。[Level 2](https://www.w3.org/TR/2024/CRD-trace-context-2-20240328/)
- ヘッダー名は ASCII の大文字小文字を区別せず、送信時は小文字を使うこと (SHOULD) が求められる。[Level 2](https://www.w3.org/TR/2024/CRD-trace-context-2-20240328/)
- `tracestate` に非トレーシング属性を入れてはならず (MUST NOT)、その用途には Baggage を使う。キーは1エントリにつき1つであり、変更・追加したキーは左端へ移動し、未変更ペアの順序は保持しなければならない (MUST)。重複キーを追加して複数化してはならない (MUST NOT)。[Level 2](https://www.w3.org/TR/2024/CRD-trace-context-2-20240328/)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- 受信した両ヘッダーは組として転送し、素通しの中継では `tracestate` に触れない。`traceparent` を変えずに `tracestate` だけを書き換える実装を避ける。
- サンプリング判断を反転させる場合は必ず新しい `parent-id` を発行し、古い `parent-id` のまま `sampled` ビットだけを反転させない。
- 解析失敗時は分離規則に従う。`traceparent` が壊れていれば `tracestate` を読まずに新規トレースとして扱い、`tracestate` が壊れていても有効な `traceparent` の相関は維持する。単独の `tracestate` は相関に使わず破棄する。
- 送信するヘッダー名は小文字の `traceparent`・`tracestate` に統一し、受信側は大文字小文字の違いで拒否しない。
- `tracestate` にはトレース相関の情報だけを入れ、ユーザー属性などの非トレーシング情報は Baggage へ分ける。自ベンダーのエントリを更新したら左端へ移動し、他者の未変更エントリの順序は保つ。
- Level 2 の `random-trace-id` フラグは、ランダム生成した識別子の衝突回避の手がかりとして保持する。中継でフラグを付け替えず、開始時の生成方針だけを自サービスで決める。

## 避ける使い方

- `traceparent` だけ転送して `tracestate` を落とす。ベンダー固有の相関が失われ、下流のサンプリングや結合判断が崩れる。
- 無変更の `traceparent` に対して `tracestate` を書き換える。通過点での改変は MUST NOT であり、相関の追跡可能性を壊す。
- `sampled` を「強制」と読み替えて上流判断を無視する。Level 1 では SHOULD の推奨であり、変更時は新 `parent-id` の発行が必要である。
- `traceparent` 解析失敗時に `tracestate` を読んで相関を復元しようとする。失敗時の分離規則に反する。
- `traceparent` なしの `tracestate` を保持・転送して後段の相関に使う。単独 `tracestate` は破棄対象である。
- `tracestate` に非トレーシング属性を混載する、重複キーを追加する、未変更ペアの順序を並べ替える。いずれも Level 2 の MUST 規則に反する。
- `version` に `ff` を使う、全ゼロの識別子を送る、大文字ヘッダー送信を前提に受信実装を組む。禁止値と送受信規則に反する。

## 適用版と本番での注意

- 適用版: Trace Context Level 1 W3C Recommendation 23 November 2021 (version 00)、Trace Context Level 2 W3C Candidate Recommendation Draft 28 March 2024。将来の改訂や Level 2 の勧告化とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で技術固有 TTL の対象外のため、2026-12-27 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: 各 SDK・プロキシでの実装状況と既定のサンプラー動作、Baggage 仕様の詳細、`tracestate` のキー・値の文法と各ベンダーのエントリ形式、将来バージョン番号の割当て。これらは該当仕様と各実装の文書を別途確認する。
- 本ドキュメントの推奨構成は設計案であり、単一事例の一般化ではない。サンプリング率とヘッダー保持の可否は対象ワークロードの流量・保持コスト・下流ベンダーの対応に照らして採用する。

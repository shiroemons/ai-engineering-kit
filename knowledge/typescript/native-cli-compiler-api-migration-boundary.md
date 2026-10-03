---
{
  "id": "typescript-native-cli-compiler-api-migration-boundary",
  "title": "TypeScript 7.0 移行: CLI 成功と compiler API・editor tooling 互換性の境界",
  "kind": "knowledge",
  "technology": "typescript",
  "version": "TypeScript 7.0 release 2026-07-08 + TypeScript 6.0 release 2026-03-23; official download current-major 7.0 and 7.1 future iteration plan verified 2026-10-03 UTC; compiler not executed",
  "tags": [
    "research-domain:frontend",
    "typescript",
    "native",
    "cli",
    "compiler-api",
    "programmatic-api",
    "typescript6",
    "tsc6",
    "npm-alias",
    "peer-dependencies",
    "language-server",
    "plugins",
    "stableTypeOrdering",
    "migration",
    "7.0",
    "7.1"
  ],
  "sources": [
    {
      "id": "typescript-70-native-api-release-20261003",
      "url": "https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/",
      "type": "release_notes"
    },
    {
      "id": "typescript-60-stable-ordering-release-20261003",
      "url": "https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/",
      "type": "release_notes"
    },
    {
      "id": "typescript-download-major-status-20261003",
      "url": "https://www.typescriptlang.org/download/",
      "type": "official_docs"
    },
    {
      "id": "typescript-71-iteration-plan-20261003",
      "url": "https://github.com/microsoft/TypeScript/issues/63703",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# TypeScript 7.0 移行: CLI 成功と compiler API・editor tooling 互換性の境界

## 問いと結論

TypeScript の native compiler へ切り替えて `tsc` が通れば、lint・宣言生成ツール・editor も同時に移行できたと考えてよいか。**CLI、programmatic API、language server plugin の利用経路を別々に判定する。** この文書は API を利用する開発ツールの移行可否を扱い、既存の [module resolution 文書](module-resolution-bundler-vs-nodenext.md) の bundler/nodenext 選択を繰り返さない。

調査対象は 2026-07-08 公開の [7.0 発表](https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/) と 6.0 の移行手順。取得日は 2026-10-03 UTC であり、7.1 の将来の API を既に利用可能とはしない。

## 確認した事実

### 1. CLI と API は別の提供面

[7.0 発表の Running Side-by-Side / Embedded Languages](https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/) は、7.0 に programmatic API がないこと、6.0 と併用する方法を明記する。互換 package `@typescript/typescript6` は 6.0 API と `tsc6` を提供し、7.0 の実行名は `tsc`。`typescript` を直接 import する peer dependency 利用ツール向けには npm alias が案内されている。

このため、7.0 CLI で型検査できることから compiler API に依存するツールの互換性は導けない。同発表は language server plugin 不要の利用からの導入を推奨する。埋め込み言語や framework の当時の制約は発表時点の記述であり、個別ツールの将来版へ固定的に一般化しない。

### 2. 6.0 を比較の足場にする

[6.0 発表の stableTypeOrdering](https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/#the---stabletypeordering-flag) は、型の内部処理順による宣言出力の並びや、まれな推論診断の差を説明している。6.0 の `stableTypeOrdering` は 7.0 との差分の診断補助で、恒久的な性能改善オプションではない。記事は負荷増の可能性と、推論の問題に明示的な型引数・注釈が有効な場合を述べる。

同じ [6.0 発表](https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/) の `ignoreDeprecations: "6.0"` は 6.0 で非推奨を抑止する移行措置であり、7.0 でその設定・構文が利用できる保証ではない。6.0 の診断を隠しただけの状態を、7.0 への互換性確認として記録しない。

### 3. 7.1 は今回の確認日時では将来計画

2026-10-03 に開いた [公式 Download](https://www.typescriptlang.org/download/) は current major を **7.0** と表示する。[7.1 Iteration Plan](https://github.com/microsoft/TypeScript/issues/63703) は 2026-07-31 に開かれ、API 安定化を計画に含め、beta を **2026-10-06**、stable を **2026-11-24** の予定としている。どちらも取得日時より後であり、7.1 stable の出荷・API 契約は未確認である。

予定日は確約ではなく、公式配布ページの major 表示も最新 patch の特定ではない。本稿では「7.1 なら解決済み」という移行判断を行わない。採用時には対象ツールの対応版と実際に配布された API を再確認する。

## 推奨する移行判断（独自の設計案）

### 利用経路を棚卸しする

依存の package 名だけでなく、誰が何を呼ぶかを記録する。

- CLI 利用: package script、CI、開発時 watch から起動する compiler executable
- API 利用: `typescript` を読み込む lint parser、AST 処理、独自 declaration 加工、compiler 埋め込み
- editor 利用: language server の選択、追加 plugin、framework 固有の診断
- 配布物利用: JavaScript、`.d.ts`、source map を読む下流アプリ

一つの依存グラフに複数の compiler がある場合、各段階の package 解決先と実行版を記録する。root の package.json が期待どおりでも workspace 内の別ツールが別の TypeScript を読み込む可能性を試験対象にする。これは特定 package manager の不具合を確認したという主張ではない。

### 導入経路を選ぶ

1. **CLI だけを利用する経路**: 7.0 の型検査を候補にし、診断・出力・実行先を比較する。単に transpiler がファイルを変換できた結果を型検査の成功と混同しない。公式 [Download](https://www.typescriptlang.org/download/) も互換 transpiler と type-checking の役割を分けている
2. **6.0 API が必要な経路**: 一時的な併用を選び、`typescript` import 側を `@typescript/typescript6` へ向ける alias と、7.0 compiler 側を別名で導入する構成を検討する。公式例にある alias は `@typescript/native` という名前で 7.0 package を参照するもので、別の native API package を新しく発見したという意味ではない
3. **editor plugin が必要な経路**: editor の診断と CI の判定を別々に検証する。CLI 導入を理由に、対応未確認の plugin を外して診断範囲を縮めない
4. **併用が複雑すぎる経路**: 6.0 を維持して API 依存を減らす作業を先行させる。未出荷の 7.1 を必須にした締切を立てず、公開後に対応表を更新する

併用では `tsc --version` と `tsc6 --version` の確認だけで完了とせず、API import を行う実際のツールで smoke test を実施する。インストール結果・peer dependency の条件・lockfile を確認し、OS ごとの CI でも同じ意図の compiler が選ばれているか点検する。公式 [Download](https://www.typescriptlang.org/download/) の project-local install / lockfile の指針を、版を固定する起点にする。

### 比較の順序

- まず現在の 6.0 で警告、診断、生成物とツールの機能範囲を保存する
- 非推奨抑止を外して互換性の問題を列挙する。module/default の詳細は既存文書へ戻って対象版ごとに確認する
- 6.0 の `stableTypeOrdering` を比較時に用い、単なる declaration の並び替えと型意味の変化を分離する
- 7.0 の CLI 診断、生成 `.d.ts` を読む下流側、lint、editor を独立した受け入れ項目にする
- 性能はこの機能同等性を満たした後に測る。型検査・plugin 処理を省略した比較を高速化として採用しない

## 避ける使い方

- package を 7.0 に更新しただけで、`typescript` import を利用する compiler API tool も互換だと扱う
- alias の設定後も `tsc` と `tsc6` の実体を記録せず、どちらで CI が通ったか不明にする
- `stableTypeOrdering` を恒久的な高速化のために有効化する、または `ignoreDeprecations` を残して互換性完了とする
- editor の赤線が消えたことを、framework/plugin の診断がすべて実行された証拠とする
- 7.1 計画の予定日を公開済みの事実へ変える

## 受け入れ試験案（未実行）

1. API を使うツールが 6.0 を読み、型検査 CLI が 7.0 を使う併用ケースを作り、それぞれの実行版と診断範囲を記録する
2. workspace 内の別 package から起動した場合も、想定した executable と API import が選ばれるか確認する
3. declaration の順序だけが変わる変更と、型検査の合否が変わる変更を用意し、6.0 の診断補助で差を分類する
4. plugin 必須のファイルに意図的なエラーを入れ、CLI 成功のまま editor の検査範囲が欠落していないか確かめる
5. 7.0 を戻す rollback で、lockfile・API 解決・editor 設定をまとめて復元できるか確認する

## 適用版・証拠・未確認事項

- 根拠は 7.0/6.0 の公式 release announcement、live 配布案内、maintainer の公開 iteration plan。7.0 の個別 patch、typescript-eslint 等の個別版、各 package manager の peer 解決を実測していない
- TypeScript compiler、lint、framework build、editor は実行していない。検索評価の成功は compiler/toolchain 互換性試験の代替ではない
- 公式 7.0 Handbook URL の取得は失敗したため根拠に含めず、開けた公式発表と配布案内を採用した。repository 実装の解析・コードの転載・module 昇格は行っていない
- 参照した記事・Web ページの再利用ライセンスは未確認で catalog は `unknown`。独自の要約と設計案のみを記し、TypeScript repository のライセンスをブログへ転用しない
- 取得日 2026-10-03 UTC。明示期限 2026-11-02 は release_notes の 30 日 TTL。7.1 の公開や対象ツールの対応変更が分かった時点でも再確認する

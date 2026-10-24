---
{
  "id": "typescript-native-cli-compiler-api-migration-boundary",
  "title": "TypeScript 7.0/7.1 Beta 移行: CLI成功・API snapshot寿命・content mapper実行許可の境界",
  "kind": "knowledge",
  "technology": "typescript",
  "version": "TypeScript 7.0 release 2026-07-08; TypeScript 6.0 release 2026-03-23; TypeScript 7.1 Beta release 2026-10-20, tag v7.1-beta at 3e9236361fa97a9e6c685f4833ac457ae2d8c50a; live docs verified 2026-10-24 UTC; compiler/API runtime not executed",
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
    "7.1",
    "beta",
    "snapshot",
    "dispose",
    "using",
    "ensurePrograms",
    "contentMappers",
    "runExternalCode",
    "fileNotifications",
    "virtual-filesystem"
  ],
  "sources": [
    {
      "id": "typescript-70-native-api-release-20261024",
      "url": "https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/",
      "type": "release_notes"
    },
    {
      "id": "typescript-60-stable-ordering-release-20261024",
      "url": "https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/",
      "type": "release_notes"
    },
    {
      "id": "typescript-download-major-status-20261024",
      "url": "https://www.typescriptlang.org/download/",
      "type": "official_docs"
    },
    {
      "id": "typescript-71-iteration-plan-20261024",
      "url": "https://github.com/microsoft/TypeScript/issues/63703",
      "type": "maintainer_article"
    },
    {
      "id": "typescript-71-beta-api-release-20261024",
      "url": "https://devblogs.microsoft.com/typescript/announcing-typescript-7-1-beta/",
      "type": "release_notes"
    },
    {
      "id": "typescript-71-beta-release-tag-20261024",
      "url": "https://github.com/microsoft/TypeScript/releases/tag/v7.1-beta",
      "type": "release_notes"
    },
    {
      "id": "typescript-content-mappers-maintainer-20261024",
      "url": "https://github.com/microsoft/typescript-go/pull/4712",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# TypeScript 7.0/7.1 Beta: CLI成功とAPI移行完了を分ける

## 問いと今回の更新

TypeScript の native CLI が通り、7.1 に programmatic API も登場したなら、6.0依存の開発ツールを一括で置き換えられるか。**CLI、APIの互換性・snapshot寿命、editorの検査範囲、content mapperの実行許可を別々に受け入れる。** APIの存在は既存ツールの対応完了でも、任意の外部コードを実行してよい根拠でもない。

旧稿は2026-10-03時点の7.0と7.1の計画を扱った。その後、[7.1 Beta発表](https://devblogs.microsoft.com/typescript/announcing-typescript-7-1-beta/)が2026-10-20に公開されたため、別の紹介文を増やさず既存文書を更新した。取得日は2026-10-24 UTC。7.1 stableの出荷や全toolの互換性を主張しない。既存の[module resolution文書](module-resolution-bundler-vs-nodenext.md)とは対象を分ける。

## 確認した事実

### 1. 7.0のAPI未提供と6.0併用は版を限定した記録

[7.0発表（2026-07-08）](https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/)は、7.0にprogrammatic APIがないこと、互換package `@typescript/typescript6` が6.0 APIと `tsc6` を提供することを説明する。7.0のCLI実行名は `tsc`。`typescript` を直接importするpeer dependency利用toolにはnpm aliasによる併用を案内している。

7.0発表のlanguage server pluginや埋め込み言語に関する制約は発表時点のもの。これを「7.xはすべてAPI非対応」や「7.1なら既存pluginが全部動く」へ一般化しない。7.0 CLIと6.0 APIの併用経路は、7.1 Betaを導入する経路と区別して管理する。

### 2. 6.0の比較補助は移行完了の代わりではない

[6.0発表（2026-03-23）](https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/)は、`stableTypeOrdering` を6.0と7.0の型順序・宣言出力・診断差の調査用とする。恒久的な性能改善設定ではなく、負荷増の可能性もある。`ignoreDeprecations: "6.0"` は6.0の警告抑止であり、その非推奨機能が7.0でも使えるという互換性保証ではない。

### 3. Beta実出荷と古い予定日を区別する

[7.1 Iteration Plan](https://github.com/microsoft/TypeScript/issues/63703)の本文には、再取得時も `2026-10-06` のBeta予定、`2026-11-24` のstable予定が残る。一方、[公式release](https://github.com/microsoft/TypeScript/releases/tag/v7.1-beta)は2026-10-20公開の **Pre-release** を示す。tag `v7.1-beta` のリンク先commitは `3e9236361fa97a9e6c685f4833ac457ae2d8c50a`。これはreleaseの識別であって、そのsourceを実行・解析した証拠ではない。

[Download](https://www.typescriptlang.org/download/)は取得時にcurrent majorを7.0と表示し、7.1をupcomingとしてリンクする。major表示から最新patchを特定せず、Betaが未提供とも読まない。旧計画より実際のreleaseを優先し、今後のstable日付は予定のまま扱う。

### 4. 7.1 Beta APIの寿命・snapshot・入力境界

以下は[7.1 Beta発表のAPI節](https://devblogs.microsoft.com/typescript/announcing-typescript-7-1-beta/)の要約であり、stable契約や6.0 APIとのdrop-in互換性の保証ではない。

- 入口は `typescript/sync`、`typescript/async`、`typescript/fs`、`typescript/ast` などに分かれる。記事の明示filenameで作る最小例は、`tsc` と異なり `tsconfig.json` を自動探索せず `--project` も扱わない。これはその例の制限であり、API全体の設定読込み不能を意味しない。
- `API`・`Program`・`Snapshot` は明示解放が必要。`using` または対応するclose/dispose操作を使い、JavaScriptのGCだけに任せない。Go側の保持が残り得る。解放済みProgramやSnapshotは再利用しない。
- `snapshot.update()` で新しいsnapshotを導出しても旧snapshotは利用可能。継続するprogram IDとobjectの混用可否は別であり、異なるsnapshotのprogram・project・checker由来objectを混ぜない。`ensurePrograms` は新snapshotで必要なprogramの更新を要求する。
- 既定はhost filesystem。`createFileSystem` は完全な仮想入力、`createFileSystemWithLib` はdefault libraryのread-only mount付き、`createFileSystemLayer` はbaseまたはhostへのfallback付き。変更監視は別途必要で、`fileNotifications` と仮想fileの新layerを適切に渡す。

### 5. content mapperは宣言だけでなく外部processの実行

[Beta発表](https://devblogs.microsoft.com/typescript/announcing-typescript-7-1-beta/)はcontent mapperの実行に `--runExternalCode` を要求し、VS Codeではworkspace trustとmapper permissionも必要とする。記事中の `@vue/content-mapper` は未公開の例示名であり、そのまま導入可能なpackageとして推薦しない。

発表が参照する[maintainerのPR説明](https://github.com/microsoft/typescript-go/pull/4712)では、mapperは拡張子ごとに原文をTypeScript向け仮想表現へ変換し、別processとSTDIO/JSON-RPCで通信する。packageの実行commandはNode.jsに限定されない。content-mapped fileはJavaScript emitの対象ではなく、declaration emitは別に扱う。

同説明は、mapperの予期しない失敗を診断し、対象を空のTypeScript fileとして扱うこと、同一projectで5回失敗するとそのmapperの変換を止め追加診断することも述べる。これは**2026-10-24に読んだ編集可能なmaintainer説明**であり、Beta tagの固定実装や将来版の試験結果ではない。診断が出たのに「型エラーが少なくなった」とだけ判断する設計は避ける。

## 推奨する移行判断（独自の設計案）

### 利用経路を棚卸しし、置換単位を小さくする

依存package名だけではなく、CLI・API import・editor・生成物の利用者ごとに実際の版と入口を記録する。workspace内で別のtoolが別のTypeScriptを読む経路、`typescript` をpeer dependencyから解決する経路、language server pluginが担う診断範囲を分ける。root package.jsonやeditorの赤線だけを移行証拠にしない。

- **7.0 CLIだけを利用**: 診断と生成物を従来版と比較する。transpile成功をtype-check成功として数えない。
- **6.0 API依存が残る**: `typescript` import側をtypescript6へ向けるnpm aliasと7.0 CLIの別名導入を維持する選択肢を残す。公式例の `@typescript/native` はalias名であり、独立した互換APIを指すと解釈しない。
- **7.1 Beta APIを試す**: 対象toolの移植版とcompiler版を固定し、一つのadapterから切り替える。旧importを新packageへ向けるだけで完了とせず、設定の読み方、診断、出力、資源解放まで受け入れる。
- **editorやmapperが必要**: 対象拡張子を実際に検査した証拠をCLIとeditorの両方で集める。未対応機能を外して検査範囲を縮めた比較を移行成功にしない。

[Downloadのproject-local install/lockfile指針](https://www.typescriptlang.org/download/)を起点に、smoke testでは `tsc --version`・`tsc6 --version` だけでなく、APIをimportする実際のtoolの解決先と動作も記録する。Betaの追従を無条件に自動化せず、再現可能な依存とrollback点を残す。

### snapshot単位で結果と寿命を所有する

ここからは公開契約を使う独自の設計であり、TypeScriptが自動実装する機能ではない。

1. 一回の解析requestに、入力revision・snapshot・そこから得たprogram/checkerと診断をまとめる。program IDだけをcache keyにして世代を省かない。
2. 更新後の解析は新snapshotからprogramを取得する。旧objectを新checkerへ渡さず、比較が必要なら各snapshot内でplainな診断結果へ変換してから比較する。
3. 旧解析が遅れて完了しても、現在の入力revisionと一致しなければ画面やCI判定へ採用しない。`snapshot.update()` の呼出しだけを、旧requestの取消しや旧objectの自動破棄とは見なさない。
4. 解析終了・例外・取消しの全経路で解放する責任者を一つ決める。非同期consumerがまだ使うobjectを先にdisposeしない。長寿命cacheに生のchecker/objectを残すより、必要な結果を取り出す方針を検討する。
5. host diskを読まない解析が目的なら完全filesystemを明示する。layerだけを渡してhost fallbackが残る構成を、閉じた入力集合やOS security sandboxと呼ばない。
6. fileの作成・変更・削除それぞれを次snapshotへ通知する。仮想入力の更新とhostの監視通知を同じものと考えず、見落とし時に旧診断を採用しない試験を置く。

### mapper実行許可と検査完了を別のgateにする

`--runExternalCode` は信頼できる変換だけを自動選別するsandboxではない。導入するmapperの実体・版・実行command・依存変更をreviewし、未信頼checkoutを読むだけの処理へ一律にflagを追加しない。CLIの実行許可とeditorのworkspace trust/mapper permissionを一つの許可と見なさない。

成功条件は「processが起動した」「診断が0件」だけにしない。予定した拡張子・fileが含まれ、mapper failure診断がなく、意図的な誤りが検出され、製品に必要なJavaScript生成工程も別途成功することを確認する。mapper出力で型検査できたことから、runtime用のbundleが生成済みとは推測しない。

## 避ける使い方

- 7.1 Betaが出たことを、7.1 stableの出荷や6.0 APIの互換性完了と扱う。
- `stableTypeOrdering` を恒久高速化の設定にする、または `ignoreDeprecations` による警告抑止で移行を完了扱いする。
- 同じprogram IDなので旧snapshotのnode・type・symbolを新checkerへ渡してよいと考える。
- GC、`null` 代入、requestの取消しをAPI資源の確実な解放と同一視する。
- filesystem layerを閉じた仮想入力と勘違いする、変更通知なしで新しいdisk状態を反映済みとする。
- mapper failureや未検査fileを無視して診断件数の減少を品質改善とする。
- preview packageやmapperの例示名を、公開済み・互換性検証済みの依存として無確認で導入する。

## 受け入れ試験案（未実行）

1. 6.0 API + 7.0 CLIの併用と7.1 Beta APIの試験構成を分け、workspaceごとのimport/executable解決を記録する。
2. 設定を明示したCLIとAPIで同じ入力・compiler optionsを使い、診断と宣言出力を比較する。6.0/7.0の順序差調査には `stableTypeOrdering` を使い、合否の差と表示順の差を分ける。
3. snapshot Aでは誤り、Bでは修正済みのfileを用意し、Bの作成後もAの診断がAの入力を表すことを確認する。遅れたAの結果がBの表示を上書きしないことも試す。
4. 通常終了・例外・cancel・複数consumerで解放責任を確認し、繰り返しrequestのGo側保持を測る。API・Program・Snapshotの解放済み再利用を正しい経路にしない。
5. 仮想filesystemに存在しないがhostには存在する依存を置き、完全入力とlayer fallbackの違いを試す。作成・削除・仮想更新を次snapshotへ伝えた場合と伝えない場合を分ける。
6. mapper実行不許可、editor許可不足、mapper crash、対象拡張子の未包含、生成物欠落を独立した失敗として検出する。計画上の全fileを検査したという証拠を残す。
7. rollbackでlockfile・API adapter・editor設定・実行許可設定を対応する版へ戻す。性能測定は機能同等性と検査範囲が確立した後に行う。

## 適用版・証拠・未確認事項

- 6.0/7.0の公式発表、7.1 Beta発表とrelease tag、live Download、iteration plan、Beta発表がリンクするmapperのmaintainer説明を2026-10-24 UTCに開き直した。古いcatalogは残し、今回参照するrecordを新規IDで作った。
- 7.1 Betaの公開を確認しただけで、stable、全editor/plugin、個別framework packageの対応完了は未確認。mapper PR本文は後編集され得るため、固定beta実装への一致や失敗回数を本番契約として採用するには対象版の検証が必要。
- compiler/API、editor、mapper、resource lifetimeのruntime試験や性能測定は実行していない。検索eval成功は互換性やメモリ解放の証拠ではない。
- Beta tagの40桁commitはreleaseリンクから確認した。固定commitのREADME/LICENSE/API sourceはnative web取得で404またはcache missとなり、実装解析の根拠にしていない。失敗した取得を取得済みと扱わない。
- 参照Webページ・記事・PR本文の再利用ライセンスは未確認で、catalogは `unknown`。repositoryのライセンスをブログへ転用せず、コードの転載・module昇格は行わない。本文は独自要約と明示した設計・試験案。
- 明示期限2026-11-23はrelease_notesの30日TTL。Beta更新、stable公開、mapper/tool対応変更が分かった場合にも再確認する。

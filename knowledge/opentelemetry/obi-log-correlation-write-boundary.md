---
{
  "id": "opentelemetry-obi-log-correlation-write-boundary",
  "title": "OBI ログ相関: write時点の帰属・SDK span_id・NUL除去と分割の境界",
  "kind": "knowledge",
  "technology": "opentelemetry",
  "version": "OBI v0.15.2、commit b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec。2026-10-06公開記事と10-21更新の公式文書、Node.js v24.22.1表示のAPI文書を2026-10-24 UTC確認。実行時検証なし",
  "tags": ["research-domain:quality-operations", "OBI", "trace-log", "log_trace_annotation", "trace_id", "span_id", "writev", "all-NUL", "8KiB", "SDK", "provenance", "backpressure"],
  "sources": [
    {"id": "otel-obi-correlation-article-20261024", "url": "https://opentelemetry.io/blog/2026/obi-trace-log-correlation/", "type": "maintainer_article"},
    {"id": "otel-obi-correlation-docs-20261024", "url": "https://opentelemetry.io/docs/zero-code/obi/trace-log-correlation/", "type": "official_docs"},
    {"id": "otel-obi-correlation-devdocs-0152-20261024", "url": "https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/devdocs/trace-log-correlation.md", "type": "official_docs"},
    {"id": "otel-obi-correlation-source-0152-20261024", "url": "https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/tree/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec", "type": "github_repository_analysis"},
    {"id": "otel-obi-sdk-detection-0152-20261024", "url": "https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/devdocs/exclude-otel-instrumented-services.md", "type": "official_docs"},
    {"id": "node-obi-pipe-stdout-24221-20261024", "url": "https://nodejs.org/docs/latest-v24.x/api/process.html#process-a-note-on-process-io", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/quality-operations.json"]
}
---

# ログにIDが付いたことと、正しい要求へ帰属したことを分ける

## 問いと判断

OBIで既存stdoutへtrace_idを付与できたとき、障害時のログ検索をそのまま信頼してよいか。判断条件は、要求と実際のwriteの対応、IDの生成元、元レコードの完全性の三つである。相関率だけを上げると、別要求のIDを付ける誤相関や、分割されたログの片側だけを調べる失敗を見逃す。

[紹介記事][記事]はアプリ改修なしの導入を説明するが、ログの配送をOBIが引き受けるわけではない。OBIは元ストリームへ書き戻し、収集・保存は既存経路に任せる。[公式文書][文書] 既存の[filelog配送境界](filelog-checkpoint-delivery-boundary.md)とは別に、本稿は収集前に相関情報と本文を変える段階を扱う。以下の「実装確認」は固定版の静的読解、「運用案」は独自の未実行提案である。

## 適用版と出典の食い違い

OBI v0.15.2の注釈付きtagをcommit b28e4e7fe0a01f1961b7c4747a13b8ff486d6fecへ解決した。ブログは2026-10-06公開、取得した公式文書は2026-10-21のv0.15.2更新を表示する。ただし更新日が新しいページだけを設定の正本にしない。

- ブログはConfig v2のlog_trace_annotation.matchによる選択を説明する。一方、公式文書のv2例はenabledと書式だけを示し、capture選択と予約済みfilterを説明しており、必要なmatchが例にない。
- 固定版の[変換処理140–163行][設定検証]はenabled=trueかつ空matchを拒否する。同処理248–249行は非空filterを拒否する。したがって、matchとfilterを同一視したり、「captureさえあれば対象は暗黙に全件」と補ってはいけない。
- v2はstandalone用。captureで選択した集合をmatchでさらに絞る。v1のebpf.log_enricher.servicesから移行するときも対象集合を照合する。固定版の開発文書49行とintegrationのv2設定はこの選択を示す。[開発文書][開発]

本稿はこの固定版について判断する。他版のparserが同じとも、公開文書の不足が修正済みとも主張しない。配備イメージ、設定形式、実際の起動時検証結果を組にして記録する。

## 1. 相関の基準時刻はlogger呼び出しではなくwrite

開発文書では、OSのPID/TIDをキーとするtraces_ctx_v1から、書き込み時点の文脈を得る。ログ本文とその文脈をring bufferへ渡し、元バッファをNULへ置換した後、ユーザー空間で注釈付き本文を再出力する。JSONオブジェクト、NDJSON、プレーンテキストが対象だが、アプリのファイル直書きやネットワーク送信を、このstdout/stderrの経路と一括に扱わない。[開発]

ここで必要な同期性は「要求処理と実際のwriteの対応」である。loggerがその場で戻っても、後のflushや別スレッドのキュー処理なら同じ条件ではない。逆に、OBI内部の非同期再出力は、取得済みの文脈を運ぶ処理であり、アプリ側の非同期loggerと同じ問題だと決めつけない。

ランタイム別の注意も資料を照合する。

- Node.jsについて、固定版開発文書184行の表はstdoutを一律に同期としている。しかし[Node.js v24.22.1表示のAPI文書][Node]は、POSIXのpipe/socketを非同期、TTYを同期と区別する。ブログもコンテナpipeのbackpressure時に欠落・古い文脈が起き得ると注意している。端末での成功を本番pipeへ外挿しない。
- Pythonの非TTY出力には、開発文書がPYTHONUNBUFFERED=1を示す。ただし独自の非同期handlerやキューまで同じ設定で同期化できるという保証ではない。
- Java virtual threadsは、この版の開発文書では相関対象外。carrierのkernel threadだけでは要求を識別できないためであり、platform threadの結果と混ぜない。
- .NETのAddConsole()について、開発文書は背景writerを制約として挙げる。AutoFlushの指定だけで背景スレッドを要求スレッドへ変えられるとは扱わない。.NET全版の標準Console実装の既定値は本調査では検証していない。

運用案として、同時に複数の既知trace_idを流し、応答前・応答後・待機後のログを比較する。欠落したIDと、別要求の有効なIDが付いたケースを別集計する。後者は見た目が正常なので、単なる形式チェックでは検知できない。

## 2. SDKのspan_idはOBIのspan_idで代用しない

公式文書がいう「SDK利用時はtrace_idのみ」は、SDKの存在を無条件に認識する意味ではない。固定版の[shouldOmitSpanID 283–292行][SDK判定]では、ExcludeOTelInstrumentedServicesが有効で、対象サービスがExportsOTelTracesと判定済みの場合に、OBI生成span_idを省く。

[SDK検出文書][SDK検出]によれば、この検出は実際のOTLP送信を観測する方式であり、ライブラリの導入や起動引数の確認ではない。最初の検出前や、送信経路を解読できない条件を別に考える。SDK導入直後から全ログがtrace-onlyになるという受入条件は誤りである。相関の確認では、SDK側で保存されたtraceと照合し、検出前後の出力を分ける。

既存フィールドも重要である。[formatter 110–126行][既存値]はJSONのキーの存在で補完を決め、既存値の妥当性や生成元の整合性を検証していない。プレーンテキストは行頭またはASCII空白の後のname=valueを識別する。trace-onlyは「OBIのspan_idを追加しない」であり、アプリが既に出したspan_idを削除する意味ではない。

運用案として、片方だけ既存、空値、別名フィールドを用意し、SDK由来とOBI由来が混ざっていないかを確認する。trace_idが存在するという条件だけで、そのspan_idが同じtrace内の実在spanを指すと認めない。完全なSDKの子span相関が必要なら、logger自身が有効なSDK文脈を記録できる構成を選ぶ。

## 3. all-NUL除去は復号後のbodyを対象にする

元バッファ抑止の結果、コンテナログにはNULと終端改行のplaceholderが残る。開発文書の例は、収集側でコンテナ形式を解釈した後、bodyへ正規表現 `^[\x00\s]*$` を適用する。JSON外装上の `\u0000` という文字列と、復号したNUL byteは別物なので、外装全体へ同じ式を当てるだけでは同じ検査にならない。[開発]

この式はNUL専用ではなく、空文字列や空白だけの本文も一致する。業務上意味のある空行を保存すべきかは別の要件である。「NULを含む行は全部捨てる」に広げると本文付きレコードまで失い得るので、採用した収集器の復号順と実際の入力を確認する。

運用案では、正常JSON、通常の空行、NULだけのbody、文字としてのバックスラッシュu0000、NULと本文が混在するbodyを区別して投入する。抑止行が消えたことと、元の実イベントが一度残ったことを別々に数える。backend件数が減っただけではフィルタ成功の証拠にならない。

## 4. 8KiBはレコード完全性の保証値ではない

公式文書の8 KiB上限は一つのwrite()/writev()で捕捉・抑止する量の上限である。大きなwriteの未捕捉部分は未注釈のまま流れ、捕捉部分は別途再出力される。後半には本文があるのでall-NULフィルタでは消えず、一つの論理レコードが分割され得る。[文書]

さらに固定実装の[types.h 20–26行][上限定数]は、8192 byteに加えてiovecを最大8 segmentとしている。[copy_iovec 92–119行とsuppress_iovec 122–157行][分割処理]はsegmentごとの量を足し、合計上限を超える次のsegmentでは処理を止める。従って「writevなら常に先頭8192 byteを丸ごと処理する」とは言えない。例えば6000 byteずつの2 segmentでは、二つ目を足すと上限を超える。この数値例はソースからの推論であり、実行結果ではない。

multilineのfirst_line/last_line/each_lineも、一回のwrite内の物理行を選ぶ設定である。複数writeに分かれたスタックトレースを再構成しないし、短い行が多い一括writeなら各行が小さくても全体上限に達し得る。JSONの前半だけが捕捉されれば、完全なJSONとして扱えるとも限らない。

運用案として、アプリの最大ログ「文字数」ではなく、符号化後byte数、write単位、iovec数を測る。8191/8192/8193 byte、複数segment、改行あり/なし、NDJSON、複数writeの同一イベントで、ID・本文・件数を比較する。巨大レコードが必須なら、本機能の選択対象から除外するか、完全性を保てる別の相関経路を検討する。

## 5. 限定導入の判定と停止条件

以下は独自の未実行受入試験案である。

1. 実際の配備版で設定を検証し、capture内かつmatch内、capture内だけ、match内だけの三群を用意する。注釈対象を拡大する前に選択を確認する。
2. 要求ごとの既知IDとログ連番を控え、低負荷だけでなく収集遅延・backpressure下で、誤帰属・欠落・重複を別に照合する。
3. SDK検出前後、既存フィールドあり/なしを分け、生成元が不一致のspan_idを正常相関として採点しない。
4. 復号後placeholderの除去、巨大writeの後半、改行・JSON解析結果まで確認する。合計件数だけで欠落と重複が相殺されないよう連番単位で比較する。
5. 誤相関や分割が要件を超えたら、当該サービスをmatchから外すか注釈を無効化する。v2でenabled=trueのまま最後のmatchだけ空にすると固定版は設定エラーになる。停止後にログ形式と既存収集が戻ったことを確認する。

権限・kernel条件は適合性審査の入力であり、機能導入のために一律でCAP_SYS_ADMINを追加したりlockdownを解除する推奨ではない。開発文書は当該capability等を要求し、通常writeのITER_UBUFはLinux 6.0以降と説明する一方、利用者向け文書はLinux 6.0以上を要件としている。古いkernelのwritev限定経路を一般的な運用対応と解釈しない。要件を満たせなければ既存SDK・logger側の相関を検討する。

## 検証範囲と由来

実装・upstream testの読解のみ。formatterの既存値保持、trace-only、改行・NDJSON、integrationのSDK検出後とshipperフィルタの期待値を確認したが、これらのテストもOBI・Collector・Fluent Bitも起動していない。kernelの部分write、障害時の再出力、全loggerの互換性、実負荷性能、backendへの保存完了は未検証である。検索evalの成功は、この実行時品質を証明しない。

ブログ、公式文書、リンク先の旧固定開発文書、Node APIはnative webで開いた。v0.15.2の固定本文・実装・ライセンスはGitHub connectorで再取得した。固定SHAの一部web URLとNodeの版固定URLは取得失敗のため、取得できた一次資料だけを根拠にしている。OpenTelemetryサイトはCC-BY-4.0、OBIのLICENSEと読んだソースのSPDXはApache-2.0、Nodeの該当資料はMIT。BPFファイルのkernel向けlicense宣言はDual MIT/GPLであり、ファイル先頭のSPDXと区別した。コードの転載・実装追加・module昇格は行わない。

[記事]: https://opentelemetry.io/blog/2026/obi-trace-log-correlation/
[文書]: https://opentelemetry.io/docs/zero-code/obi/trace-log-correlation/
[開発]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/devdocs/trace-log-correlation.md
[設定検証]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/internal/config/convert/import.go#L140-L163
[SDK判定]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/pkg/internal/ebpf/logenricher/logenricher.go#L283-L292
[SDK検出]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/devdocs/exclude-otel-instrumented-services.md
[既存値]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/pkg/internal/ebpf/logenricher/formatter.go#L110-L126
[上限定数]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/bpf/logenricher/types.h#L20-L26
[分割処理]: https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/b28e4e7fe0a01f1961b7c4747a13b8ff486d6fec/bpf/logenricher/logenricher.c#L92-L157
[Node]: https://nodejs.org/docs/latest-v24.x/api/process.html#process-a-note-on-process-io

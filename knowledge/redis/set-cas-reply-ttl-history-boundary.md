---
{
  "id": "redis-set-cas-reply-ttl-history-boundary",
  "title": "Redis SET CAS: GET応答・TTL・値一致と変更履歴の境界",
  "kind": "knowledge",
  "technology": "redis",
  "version": "IFEQ/IFNE/IFDEQ/IFDNE introduced in Redis 8.4.0; option exclusivity fix in 8.10 release history; implementation/tests pinned to 8.10.2 (2026-09-17) @ 498ecd0d6d007db11ddb3aea9428552598a78622; docs checked 2026-10-04 UTC",
  "tags": [
    "research-domain:data",
    "redis",
    "SET",
    "CAS",
    "IFEQ",
    "IFNE",
    "IFDEQ",
    "IFDNE",
    "GET",
    "KEEPTTL",
    "DIGEST",
    "XXH3",
    "WATCH",
    "ABA",
    "optimistic-concurrency"
  ],
  "sources": [
    {
      "id": "redis-set-cas-contract-20261004",
      "url": "https://redis.io/docs/latest/commands/set/",
      "type": "official_docs"
    },
    {
      "id": "redis-set-cas-digest-20261004",
      "url": "https://redis.io/docs/latest/commands/digest/",
      "type": "official_docs"
    },
    {
      "id": "redis-set-cas-transactions-20261004",
      "url": "https://redis.io/docs/latest/develop/using-commands/transactions/",
      "type": "official_docs"
    },
    {
      "id": "redis-set-cas-release-8102-20261004",
      "url": "https://raw.githubusercontent.com/redis/redis/498ecd0d6d007db11ddb3aea9428552598a78622/00-RELEASENOTES",
      "type": "release_notes"
    },
    {
      "id": "redis-set-cas-implementation-8102-20261004",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_string.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-set-cas-tests-8102-20261004",
      "url": "https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/string.tcl",
      "type": "github_repository_analysis"
    },
    {
      "id": "redis-set-cas-option-fix-20261004",
      "url": "https://github.com/redis/redis/commit/2c9c04c1f87254976cb573b70c74f6724fc098b2",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/data.json"
  ]
}
---

# Redis SET CAS: 更新結果を old value や「変化なし」と取り違えない

## 問いと結論

read-modify-writeを `SET ... IFEQ` に置き換えるとき、返答・期限・競合の何を確認すればよいか。単一stringの置換は簡潔になるが、`GET` 付きreplyは更新成否のbooleanではない。値一致は変更履歴の不在も証明しない。採用時は **条件、成否の読み方、TTL方針、再試行の意味** をまとめて決める。

これは新しいstring CASの契約を扱う。既存のclient-side caching、Streams IDMP、HIMPORT、LMOVEM記事の分割ではない。以下の公式契約・固定実装の観察と、「設計案」と記した運用判断を区別する。

## 適用版と更新時の落とし穴

[SET公式履歴][set]では4つの `IF*` 条件が8.4.0で導入された。[8.10 release notes][release]の8.10-RC1（2026-07-20）欄には、`NX` / `XX` と `IF*` の相互排他を正しく検査しない不具合 #15291 の修正がある。8.10 GAは2026-07-29。本稿のコード・既存test読解は8.10.2（2026-09-17）の完全commitに固定した。

[修正commit][fix]では、先に `IF*` を渡してから `NX` / `XX` を加えると、不正な組合せが通っていた。固定版の[parserと既存test][tests]は両方の引数順で拒否する。したがって「既存clientがエラーにしなかった」は、組合せが有効だった根拠にならない。移行前にcommand builderを点検し、`XX IFEQ` を「存在して一致」の補強として足さない。`IFEQ` 自体が欠落keyを作成しない。

他の保守系列のbackport・最初の修正版は未確認であり、8.10以前がすべて未修正とは断定しない。最新版という主張でもない。

## 条件と応答の契約

[SET command reference][set]による要約。ここで一致は現在のstringまたはそのdigestとの一致であり、JSON構造や数値の同値ではない。

| 条件 | 既存string | 欠落key |
|---|---|---|
| `IFEQ expected` | bytesが一致すれば置換 | 作成しない |
| `IFNE expected` | bytesが異なれば置換 | 作成する |
| `IFDEQ digest` | digestが一致すれば置換 | 作成しない |
| `IFDNE digest` | digestが異なれば置換 | 作成する |

`NX`・`XX`・4つの `IF*` は条件の一択である。存在する非stringを `IF*` で比較すると、固定版の[既存test][tests]では `WRONGTYPE` になる。空文字のstringと欠落keyも区別する。

- `GET` なし: 適用した操作には `OK`、条件不成立にはnullが返る。syntax error、型エラー、通信失敗をnullにまとめない
- `GET` あり: 更新の有無にかかわらず操作前のstringを返す。欠落していた場合は、作成したか否かにかかわらずnullになる
- RESP2のnull bulk stringとRESP3のnullを、空のbulk stringと区別できるclient表現にする

固定版の [setGenericCommand][impl] は `GET` のreplyを条件判定より先に作り、不成立でもold valueを返す。[既存test][tests]も、値が一致する更新と不一致の無更新で同じold valueが返ることを検査している。したがって「返答が非nullなら成功」「nullなら失敗」という共通wrapperは壊れる。Tcl testの空表現だけからwire上のnull/空文字を推測しない。

設計案: 更新成否だけが要るCASでは `GET` を付けず、`OK` / 条件不成立 / error / 結果不明を分ける。old valueが必要な場合は、条件の種類と返された正確なbytesに基づく専用decoderを作る。たとえば `IFEQ` の比較対象が空文字なら、空文字replyは正当な一致候補である。

## TTLは条件に付随するが、比較対象ではない

[SETの期限契約][set]は、成功した通常のSETで既存TTLを除去し、`KEEPTTL` で維持できるとしている。`EX`・`PX`・`EXAT`・`PXAT`・`KEEPTTL` も一択である。[固定実装][impl]では、条件不成立のreturnは値・期限を書き換える処理より前にある。

ここから採用時に分けるべき状態は次の通り。

1. `IFEQ expected` の成功で期限指定を省く: 以前の有効期限を失い得る
2. `IFEQ expected KEEPTTL` の成功: その時点のTTLを維持する。以前readしたTTLとの一致は検査しない
3. `IFEQ expected PX duration` の成功: 新しい相対期限を設定する
4. 条件不成立: このcommandによるTTL更新はない。ただし自然な時間経過や別clientの更新は止めない

さらに固定実装は、条件成立後の `EXAT` / `PXAT` がすでに経過している場合、既存keyを削除する経路でも `GET` なしには `OK` を返す。これは実装読解の結果でありruntime再現は未実施。**OKは返答後もkeyが存在する保証ではない**。絶対期限を使う設計では時刻・単位と残り有効期間も別に検査する。

## DIGESTで削減できるものと検査されないもの

[DIGEST公式文書][digest]はstringのXXH3 digestをhexで返し、計算量を値の長さに対するO(N)とする。固定版の [stringDigest][impl] はXXH3_64bitsを使い、先頭ゼロを保持する16桁hexを生成する。整数encodingは文字列に戻して計算する。[既存test][tests]は同じ内容の別keyで同じdigest、大小hex文字の受理を確認している。digestはkey ID・TTL・世代番号を表していない。

設計案: `IFDEQ` は比較のために旧値全体を送る量を減らす選択肢だが、server側のdigest計算まで定数時間になるとは扱わない。64bitの値一致には衝突余地があり、暗号学的な本人確認・内容証明・絶対に衝突しないversionの代用品にしない。

固定版の `validateHexDigest` は名前やerror文に反して **長さだけを検査** し、hex文字種を検査しない。既存stringに対する16文字の非hexは、比較で不一致となるため `IFDNE` の成立側へ進み得る。また欠落keyの早期分岐ではdigest検査に到達しない。[既存test][tests]にも短いdigestの欠落keyに対し、`IFDEQ` は作成せず `IFDNE` は作成するケースがある。これらは固定コード/testの観察であり、入力検証として依存する契約にはしない。client側で16桁hexと先頭ゼロを検証し、数値へ変換して桁を落とさない。

## 値一致とWATCHの変更検知を分ける

[Transactions公式文書][txn]では、`WATCH` 後に監視keyが変更されれば `EXEC` がtransaction全体を中止する。expirationやevictionも含む。`EXEC` の実行、またはconnection切断でwatchは解除される。6.0.9より前のexpirationの例外を、本稿の8.10.2へ持ち込まない。

一方、固定SET実装は比較時点の値を調べる。この差から次の制約が導かれる（本稿の推論）。

- ABA: readした値Aが他clientによりBになり、再びAになった場合、`IFEQ A` はその変更履歴を検出しない。TTLだけが変わった場合も値比較をすり抜ける
- `GET` と `DIGEST` を別々に読むと、その間の更新により古い値Aと新しい値Bのdigestを組み合わせ得る。Aから計算した次値をBのdigest条件で書く手順は、lost updateを防げない
- `MULTI` に `SET ... IFEQ` と別の更新を並べても、SETのnull replyは後続commandの中止条件にならない。公式transaction契約ではruntime errorですら他commandは継続する

設計案: 単一値の楽観更新なら、readしたbytesをそのまま `IFEQ` の期待値に用いる。業務上の世代を識別したい場合は、再利用しないrevisionを同じ値に含める。複数keyの不変条件、途中変更の検出、条件に応じた複数操作が必要なら、WATCH transactionまたはserver-side scriptを検討し、それぞれの接続・cluster・error契約を別途検証する。

通信断でreplyが失われた場合も、再送の条件不成立だけでは「初回が成功した」「別writerが更新した」を区別できない。業務operation IDや保存済み結果と照合する必要がある。CASの採用だけで外部副作用のexactly-onceや永続化保証を得たとしない。

## 導入時の受け入れ確認案

以下は実施すべきruntime試験案であり、本調査の実行済み結果ではない。

- 成功・不一致・欠落・空文字・非stringを `GET` 有無、RESP2/RESP3で確認し、replyと最終値の両方を照合する
- 既存TTLに対する省略 / KEEPTTL / PX、条件不成立、期限経過したPXATを組み合わせる
- `IF* ... NX/XX` と `NX/XX ... IF*` の両順序を移行前後で確認する。不正な組合せを許す旧動作へfallbackしない
- digestの先頭ゼロ、uppercase、非hex16文字、長さ不正、欠落keyを検証する
- A→B→A、TTLのみ更新、GETとDIGEST間更新、reply受信前切断を競合試験へ入れる
- command wrapperがboolに変換する場合の対応版・bytes保持・null/空文字の区別を、そのclient版で確認する

## 確認範囲・出典

2026-10-04 UTCにSET・DIGEST・Transactions・tag付きrelease/sourceをnative webで開いた。完全commitのコード・test・修正diff・licenseはGitHub connectorでも確認した。全コーパスをindexした検索 `IFEQ`、`IFDEQ`、`redis WATCH` は執筆前に該当なし。`SET CAS` の部分一致候補と全文grepも確認し、同じ契約を扱う記事はなかった。

Redis server、SDK、cluster、failoverの実行試験はしていない。既存upstream testを読んだことと実行成功を区別する。未知のbackport、digest衝突の実測、Active-Activeでの並行更新契約は未確認。DIGESTページの一部SDK欄にはscript SHA1の説明が混在するため、server契約は本文と固定実装を根拠とした。資料は原文を転載せず独自に要約し、コードは移植していない。

[set]: https://redis.io/docs/latest/commands/set/
[digest]: https://redis.io/docs/latest/commands/digest/
[txn]: https://redis.io/docs/latest/develop/using-commands/transactions/
[release]: https://raw.githubusercontent.com/redis/redis/498ecd0d6d007db11ddb3aea9428552598a78622/00-RELEASENOTES
[impl]: https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/src/t_string.c
[tests]: https://github.com/redis/redis/blob/498ecd0d6d007db11ddb3aea9428552598a78622/tests/unit/type/string.tcl
[fix]: https://github.com/redis/redis/commit/2c9c04c1f87254976cb573b70c74f6724fc098b2

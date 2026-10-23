---
{
  "id": "http-range-if-range-resume-completion-boundary",
  "title": "HTTP Range download: If-Range・表現の符号化・206/416と再開ファイル完了の境界",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9110 / STD 97 (June 2022), RFC 9111 / STD 98 (June 2022), RFC 9530 (February 2024); relevant sections verified 2026-10-23 UTC",
  "tags": ["research-domain:api-distributed", "http", "Range", "If-Range", "Content-Range", "resume", "download", "strong-validator", "ETag", "Content-Encoding", "206", "416", "multipart/byteranges", "Repr-Digest", "durable-prefix"],
  "sources": [
    {"id": "rfc9110-range-resume-contract-20261023", "url": "https://www.rfc-editor.org/rfc/rfc9110.html", "type": "official_docs"},
    {"id": "rfc9111-incomplete-range-cache-20261023", "url": "https://www.rfc-editor.org/rfc/rfc9111.html", "type": "official_docs"},
    {"id": "rfc9530-range-representation-digest-20261023", "url": "https://www.rfc-editor.org/rfc/rfc9530.html", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2027-01-21",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/api-distributed.json"]
}
---

# HTTP Range download: If-Range・表現の符号化・206/416と再開ファイル完了の境界

## 問いと結論

大きな成果物の download が途切れた後、ローカルファイルの長さを `Range` の開始位置にして続きを追記してよいか。`206` や `416` を受けたら完了といえるか。

**再開位置は「同じ表現について検証・永続化済みの連続 prefix の長さ」。HTTP 応答成功、範囲の結合、ファイル全体の検証、利用者への公開を別の状態にする。** 同じ URL・同じサイズ・同じ見た目の本文だけでは別々の応答を安全に連結できない。

本稿は download の設計判断であり、新しい2026年の仕様の紹介ではない。[キャッシュ再検証](caching-cache-control-conditional.md)の freshness/304、[resumable upload](resumable-upload-offset-completion-boundary.md)の upload offset、[署名と digest](message-signatures-content-digest-replay.md)の認証 profile とは分ける。2026-10-23 の既存 KB 検索で `If-Range` はキャッシュ文書、`Content-Range` は upload 文書に到達したが、`206 416` は該当なしだったため、再開ファイルの誤結合という未充足の問いを選んだ。

## 確認した HTTP の契約

以下は一次資料の要約。後述する単一範囲 client の制約や journal は、この規格に追加された義務ではない。

### 表現とバリデータを固定する

[RFC 9110 §8.8、§13.1.5、§14.1.2、§15.3.7.3](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.5)では、部分の安全な結合に同じ strong validator が必要である。`If-Range` が不一致なら `Range` を無視するので、通常は新しい全体の `200` を受ける経路になり、不一致を必ず `412` として知らせる方式ではない。`If-Range` に弱い ETag は禁止。日付を使えるのは対応する ETag を持たず、その日付が §8.8.2.2 の strong 条件を満たす場合だけで、比較は日付でも完全一致である。

byte offset は0始まり、終端を含む。`Content-Encoding` があると、範囲は復号後ではなく符号化済みの byte 列に対する位置である。strong ETag は不透明な識別子であり、内容の hash と決めつけない。異なる resource 間で同じ値でも同一内容は保証されない（§8.8.1、§8.8.3）。

### status だけでは受信範囲を決められない

[RFC 9110 §14.2–14.4、§15.3.7、§15.5.17](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.7)の規定を、受信処理の分岐に結び付ける。

- `Range` は無視され得る。`Accept-Ranges: bytes` も次回の部分応答を保証せず、欠落していても範囲要求は可能。
- `206` では `Content-Type` と `Content-Range` を調べる。要求の一部だけ返る場合があり、`Content-Length` はその message の長さであって全体サイズとは限らない。全体長は `Content-Range` の `/L`、未確定なら `/*`。不正な範囲や未知の単位は保存済み表現と結合してはならない。
- `If-Range` への `206` は、規定上必要なもの以外の representation header を省く SHOULD がある。例えば `Content-Encoding` 欠落を、既存 metadata を破棄して identity に変更する根拠にはできない。
- multipart は各 part に範囲を持つ。順序・個数は要求と同じと仮定できない。単一範囲への multipart 応答は禁止され、multipart 非対応 client は複数範囲を要求してはならない。
- `416` は範囲集合の拒否である。byte-range 応答では現在長を `Content-Range: bytes */L` で知らせる SHOULD があるが、完了通知ではない。

### 部分保存と全体の検証は別

[RFC 9111 §3.3–3.4](https://www.rfc-editor.org/rfc/rfc9111.html#section-3.3)は、不完全な応答を保存する cache に不完全状態の記録と Range の理解を要求し、全体または保有範囲内だけを返す制約を設ける。同じ strong validator を持つ部分の結合は許可されるが、途中まであることを通常の完全応答として配る許可ではない。本稿のローカル一時ファイルを HTTP cache と同一視するのではなく、部分と完成品を区別する設計の根拠として参照する。

[RFC 9530 §2–3、§6.1–6.5](https://www.rfc-editor.org/rfc/rfc9530.html#section-3)では `Content-Digest` は message content、`Repr-Digest` は選択された representation data 全体を対象とする。部分応答の Content-Digest と完成ファイルの digest は別物である。content coding も digest 対象に関係し、digest 単独は送信者認証ではない。全体を復元してから確認する digest の用途は、再開 prefix の同一性判定を省略する理由にならない。

## 推奨 profile: 単一範囲・単一 writer・strong ETag（独自の設計案）

対象は HTTPS で取得する大きな成果物を専用一時ファイルへ保存する client。まず次の制約で実装を小さくする。汎用 HTTP client や全 CDN の適合性を主張する profile ではない。

### 1. 再開状態をファイルサイズから推測しない

一時ファイルと、それに結び付いた小さな journal に以下を持つ。

- download job の世代 ID、取得元の最終 URI、利用アカウント/tenant の論理 ID、表現選択に使った要求条件。秘密の認証 header 自体は journal やログへ複製しない
- strong ETag の元の値、選択された Content-Encoding と必要な representation metadata、既知なら全体長 `L`
- 永続化済みで検証済みの連続区間 `[0,N)`、一時ファイルの識別子、再開時に prefix を再確認できるローカル checksum/区間記録
- 期待する成果物の版と全体 digest が別の信頼済み manifest から得られるなら、その対象・算法・値。HTTP の encoded representation 用か、復号後の artifact 用かも別欄にする

`N` は受信 socket の積算値、UI の進捗値、最終 seek 位置ではない。sparse file は論理サイズが大きくても穴を持つ。複数 writer や別の download job が同じ一時ファイルを使うことを排除し、長さだけ一致するファイルを journal に付け替えない。

初期取得で strong ETag がない、弱い ETag しかない、または header を失った partial file なら、この profile では byte resume をしない。`W/` を削って strong に変造せず、弱い ETag があるから日付へ自由に fallback できるとも考えない。再取得の通信費を、誤結合を防ぐための明示的な代償とする。

### 2. identity coding を選ぶ場合も実データ層を確認する

初期版は `Accept-Encoding: identity` を送信し、自動展開を行う middleware を無効化するか、保存 API がどの層の byte を返すかを確認する。最初の全体応答で非 identity の content coding が選ばれていたら、この profile の再開候補に入れない。要求した header だけを証拠にせず、受信 metadata と保存 byte の層を照合する。

符号化済みの取得を追加するなら、それを別の profile として試験する。例えば gzip の途中までを自動展開して保存した長さを、gzip の `Range` offset に使わない。gzip の先頭と identity の末尾も連結しない。最初の応答が gzip で、後の If-Range 206 に Content-Encoding がない場合は、欠落を identity の証拠にせず既存 metadata を使う必要がある。明示された coding が変わった場合は、ETag が見かけ上一致していても異常として結合を止める。

content coding と transfer coding は別である。HTTP/1.1 の chunk framing、HTTP/2/3 の frame header、TLS record の byte 数を `N` に含めない。encoded byte の保存/復号と header の保持を library がどう扱うかは製品ごとに検証する。

### 3. response body を開く前に、追記・置換・拒否を決める

再開要求は `Range: bytes=N-` または転送上限付きの単一範囲と `If-Range: <保存したstrong ETag>` にする。`N=L` と既に検証できる場合は次の範囲を要求せず、完了検査へ進む。`If-None-Match` を再開制御として混ぜて 304 を得ても、欠けた byte は補えない。

| 応答・観測 | この profile での処理 |
| --- | --- |
| `206`、単一範囲、保存済み strong ETag と一致 | 下記の境界検査後に別の segment 一時領域へ受信する。status だけで追記を開始しない |
| `200` | 旧prefixへ追記しない。別世代の全体取得として、新しい一時ファイルを offset 0 から作る。古い prefix を先に破壊せず、どちらの世代のデータかを混ぜない |
| `416` | 本体を成果物へ入れず、length 情報を診断材料にする。`N=L` でも内容一致の証明にはしない。既存の独立した完了条件を満たさなければ全体を再取得する |
| `206` だがタグ欠落/不一致、未知の単位、不正な範囲、予期しない multipart | 結合しない。タグ欠落での拒否は本 profile の保守的制約であり、すべての HTTP client への追加 MUST ではない |
| redirect、304、4xx/5xx、接続失敗 | 各応答を正常な segment と扱わない。最終 URI/表現の同一性を再確認し、必要なら新世代の全体取得へ戻る。無制限に同じ要求を繰り返さない |

`200` は If-Range 不一致に限らず Range を無視した結果でも起こる。原因を推定して旧 byte の利用を正当化せず、同じ安全な置換経路に集約する。旧 ETag の版が必須な job では、新版 `200` の取得に成功しても要求達成とせず、manifest の版との不一致として停止する。

単一 `206` の `Content-Range: bytes a-b/L` には、次の検査を行う。

1. 整数の桁数・変換・加算には上限と overflow 検査を置く。`a <= b`、数値 `L` なら `b < L`、保存済み数値 `L` との一致を確認する
2. この連続 prefix profile では `a=N` を要求する。`a<N` は重複、`a>N` は穴として append しない。要求に終端を指定した場合は、その範囲を超えないことも確認する
3. 受信すべき segment 長は `b-a+1`。Content-Length があれば照合し、実際の受信量も照合する。HTTP library の message 完了/stream error と、採用した digest 検証も確認する
4. 要求した残りの一部だけの `206` でも、この検査に通る連続 segment なら受理できる。`N=b+1` として残りを再要求し、短い206を全体完了としない
5. `/L` が `/*` なら全体長は未知のまま。段階的な受信はできても「今回の EOF が全体 EOF」という推定で公開しない。予算内に信頼できる全体長/全体応答を得られなければ、未完了として停止する

segment を別領域に置く方式では、途中切断した segment は prefix に commit せず再取得する。壊れた応答の一部を salvage する最適化より、復旧手順の単純さを優先した設計である。

### 4. multipart は実装しない限り要求しない

本 profile は一つの連続範囲だけを要求する。後に並列・複数範囲へ拡張するなら、各 part の Content-Range、boundary、実長、重複区間と穴、同一 validator を追跡する interval map が必要になる。multipart envelope や part header を成果物の byte として保存しない。

サーバーは範囲を結合し得るため、「要求した範囲の数 = part 数」「第2 part = 第2要求範囲」という対応を実装しない。複数範囲を要求した結果、multipart が一つの part だけになる場合もある。並列化のために同じ URL の別 ETag の segment を混ぜると、通信は全件成功しても成果物は壊れる。これらを扱う余地がなければ、単一範囲を維持する。

## 完了を判定する順序（独自の設計案）

HTTP はローカル filesystem の crash consistency や成果物の公開手順を規定しない。以下はその不足を埋める client 側の提案である。

1. **segment 受理:** 受信範囲・世代・表現・実長を検査する。採用した Content-Digest が trailer にあるなら、trailer の受領/検証まで segment を確定しない
2. **prefix commit:** segment の byte を保存して必要な永続化を終え、その後 journal の `N` を更新する。metadata だけ先行させない。journal 更新失敗では旧 N を残し、再起動後に未 commit の tail を再確認または切り捨てる。逆に journal が示す byte が欠けた/壊れた状態は正常な resume と扱わない
3. **全体検査:** 同一世代の coverage が穴なく `[0,L)` を満たすことを確認する。`N=L` は必要な境界検査であり、重複受信量の合計や sparse file のサイズで置き換えない。Repr-Digest/信頼済み manifest の全体 checksum を採用した job では、適切な byte 層で再計算し一致を確認する
4. **公開:** 検証済みの一時ファイルを最終名へ公開する。単一 filesystem 上の atomic replacement 等を選び、必要な durability、directory 同期、既存ファイルの扱いを OS ごとに検証する。転送成功を atomic/durable publication の保証と混同しない
5. **再起動時の調停:** 公開直後・journal 更新前に crash しても、世代 ID と最終ファイルの検証結果から完了を復元できるようにする。単に `.part` が消えたことを成功の根拠にしない

Repr-Digest は全体 representation、Content-Digest は部分 message なので、部分ごとの checksum を文字列連結して全体 checksum にしない。digest なしの配信にも HTTP resume は成立し得るが、保存媒体の破損や誤ったサーバー metadata に対する検出能力は別である。本 profile で checksum を必須にするかは、成果物の用途と信頼モデルで決める。digest があるだけで署名や origin 認証の代わりにもしない。

### 小さな反例

旧表現 A の先頭4096 byteを保存し、`Range: bytes=4096-`、`If-Range: "artifact-A"` を送ったとする。

- 新表現 B の `200`、全体長8192が来た。追記すると12288 byteになり、先頭だけA、残りはBとなる。本案は新しい一時ファイルへBを保存する
- A の `206`、`Content-Range: bytes 4096-6143/8192`、実長2048が来た。segment は成功だが全体は未完了。次は6144から要求する
- `416`、`Content-Range: bytes */4096` が来た。保存済みAと現在の4096-byte表現が同じとは、この長さだけでは分からない。416を completion ACK と扱わない
- A の全8192 byteが disk buffer に書かれた直後に crash した。受信量8192の表示だけでは durable prefix 8192を保証せず、保存状態と journal の再確認が必要になる

## 受け入れ試験案（未実行）

正常系の速度より、誤った完成品を公開しないことを先に検証する。以下はテスト仕様であり、実装を起動して確認した結果ではない。

| 入力・障害 | 確認する結果 |
| --- | --- |
| Accept-Ranges ありだが Range に200を返す | 旧prefixへ追記せず新世代へ置換。header 広告を成功証拠にしない |
| strong ETag が変わり If-Range が不一致 | 200 fallback を扱い、古い byte と新しい byte を混ぜない |
| 弱い ETag のみ、または Date と Last-Modified だけ | この profile は再開せず全体取得。W/ の除去や無条件の日付転用をしない |
| 206が要求の一部だけを返す | 数値範囲に従い次の prefix から続け、接続 EOF を全体完了としない |
| a<N、a>N、b<a、b>=L、巨大な数字、途中でL変更 | commitせず診断。重複、穴、overflow、世代混在を見逃さない |
| Content-Range の全体長が * | 既知の完了条件がない限り最終名で公開しない |
| segment が1 byte不足/過剰、stream reset、digest不一致 | prefix N を進めず、受信途中ファイルだけを完成扱いしない |
| 416でN=L、N>L、Content-Range欠落 | どれも status/長さだけを completion ACK にしない |
| 初回gzipを復号保存、再開時identityに切替 | byte層の違いを検出し結合しない |
| gzipのIf-Range 206からContent-Encodingだけ省略 | identityと誤認せず、既存 metadata と encoded byte 層を維持する設計を検証 |
| 同じETag文字列を別URI/別tenantで返す | 独立したjobのprefixを流用しない |
| 複数範囲でpart順序逆転・結合・一部のみ返却 | 拡張実装のみ各Content-Rangeで配置。現profileはmultipartを要求しない |
| data永続化前後、journal更新前後、公開前後にcrash | journal先行や二重公開が起きず、復旧したprefix/最終成果物を検証できる |
| 最終fileのサイズだけ正しく内部に穴/破損 | coverage/全体検証で検出し、成功表示だけ残らない |

## 適用版・provenance・未確認事項

- RFC 9110（STD 97）と RFC 9111（STD 98）は2022年6月公開、RFC 9530 は2024年2月公開。本稿の取得日は2026-10-23 UTC。公開日と再取得日を分け、最新の実装対応版とは主張しない
- RFC 9110 の取得範囲は §8.4、§8.8、§13.1.5、§14、§15.3.7、§15.5.17。RFC 9111 は §3.2–3.4、RFC 9530 は §2–3、§6 の関係箇所。各 Copyright Notice の IETF Trust Legal Provisions / BCP 78、抽出 Code Components の Revised BSD 条項を確認。本文は独自の日本語要約と設計案で、原文例・コード・digest値の転載なし
- RFC 9530 の errata は公式 redirect と検索 endpoint の取得に失敗したため、2026-10-23 時点の状態は再確認できていない。以前の文書の errata 状態を新しい確認結果として転記しない。digest の具体的 example vector は採用していない
- HTTP/1.1・HTTP/2・HTTP/3 の framing 実装、ブラウザー/CORS、Go/curl等の自動展開設定、CDNやobject storage固有のETag生成、暗号検証、ファイルロックとfilesystemのdurabilityは未実測。サーバーが正しいstrong ETagを発行することも実環境で要確認
- multipart parser、日付 validator 対応、任意redirect間の再開、partial segment salvage、parallel downloaderの実装は範囲外。全体 digest が取得できない場合のリスク受容も利用側の判断として残す
- 本稿はコード/module 昇格を行わない。検索 eval は可発見性だけを検査し、上記 fault test は今回実行していない。3件とも official_docs の90日TTLで、再確認期限は2027-01-21

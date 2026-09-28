---
{
  "id": "http-caching-cache-control-conditional",
  "title": "HTTP キャッシュ: Cache-Control の freshness と ETag/Last-Modified 条件付き再検証・再利用・無効化範囲",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9111 (STD 98, June 2022), RFC 9110 (STD 97, June 2022)",
  "tags": ["research-domain:api-distributed", "http", "caching", "cache-control", "freshness", "ETag", "Last-Modified", "conditional-request", "If-None-Match", "If-Modified-Since", "If-Match", "If-Range", "Vary", "304", "invalidation", "rfc9111", "rfc9110"],
  "sources": [{"id": "rfc9111-http-caching", "url": "https://www.rfc-editor.org/rfc/rfc9111.html", "type": "official_docs"}, {"id": "rfc9110-http-semantics", "url": "https://www.rfc-editor.org/rfc/rfc9110.txt", "type": "official_docs"}],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active"
}
---

# HTTP キャッシュ: Cache-Control の freshness と ETag/Last-Modified 条件付き再検証・再利用・無効化範囲

GET レスポンスをキャッシュから再利用してよいか、古くなったエントリを条件付きリクエストで再検証するか、書き込み成功後にどの URI のエントリを無効化するかを、[RFC 9111 HTTP Caching](https://www.rfc-editor.org/rfc/rfc9111.html)（STD 98、2022年6月、RFC 7234 を obsolete）と [RFC 9110 HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.txt)（STD 97、2022年6月、RFC 7231/7232 を obsolete）を2026-09-28に確認して整理する。以下で「文書化された事実」と「設計案」を明示的に分ける。

## 要点（文書化された事実）

### キャッシュキーと再利用条件（RFC 9111）

- キャッシュキーはリクエストメソッドとターゲット URI に、格納レスポンスの `Vary` が指名したリクエストフィールドを加えたものが一次キーになる。`Vary` の指名フィールドが一致しない保存済みレスポンスは再利用できない。`Vary: *` のレスポンスは後続リクエストに一致しない（never matches）ため再利用できない。
- 保存済みレスポンスの再利用には、URI・メソッド・`Vary` の一致に加えて、次のいずれかが必要である。(a) レスポンスが fresh である、(b) stale の再利用が許されている、(c) オリジンサーバへの再検証（validation）に成功した。リクエストや保存レスポンスに未検証の `no-cache` がある場合は、再検証なしの再利用はできない。

### freshness の判定と優先順位（RFC 9111）

- fresh かどうかの判定式は `response_is_fresh = (freshness_lifetime > current_age)` である。freshness_lifetime が current_age を上回る間だけ fresh として再検証なしに再利用できる。
- freshness_lifetime の算出の優先順位は、共有キャッシュでは `s-maxage` があればそれが最優先、次にレスポンスの `max-age`、どちらもなければ `Expires` ヘッダの日付である。すなわち shared `s-maxage` > `max-age` > `Expires`-Date の順で適用される。`Expires` は `max-age` や共有キャッシュの `s-maxage` がある場合はそちらが優先される。

### バリデータのモデル（RFC 9110 §8.8–8.8.3）

- バリデータは `Last-Modified`（最終更新時刻）と `ETag`（entity-tag）の2種類である（§8.8）。
- `ETag` には strong と weak の比較があり（§8.8.3）、コンテンツネゴシエーションで選ばれた variant ごとに値が変わりうる。weak ETag は意味的に等価な範囲での比較に使い、バイト単位の同一性を要求する用途には strong 比較が必要になる。
- `304 Not Modified` はバリデータに基づく再検証への応答であり、コンテンツ本体を含まない（§15.4.5）。

### 条件付きリクエストと優先順位（RFC 9110 §13.1、§13.2）

- 前提条件ヘッダは `If-Match`、`If-None-Match`、`If-Modified-Since`、`If-Unmodified-Since`、`If-Range` である（§13.1）。
- 評価の優先順位（§13.2）では、`If-None-Match` が存在すれば `If-Modified-Since` より優先される。つまり両方が送られた場合の条件判定は `If-None-Match` が決め、`ETag` ベースの判定が時刻ベースの判定に優先する。

### 再検証リクエストの送り方と 304 の効果（RFC 9111）

- キャッシュは再検証時に、保存済みの entity-tag を `If-Match` / `If-None-Match` / `If-Range` で送らなければならない（MUST）。保存済みの `Last-Modified` は `If-Modified-Since` で送るべきである（SHOULD）。
- `304 Not Modified` を受信したら、§4.3.4 に従って保存済みエントリのヘッダを応答のヘッダで更新し、エントリを再利用可能にする。304 自体に本体はないため、保存済み本体と更新後のヘッダを組み合わせて応答を構成する。

### 無効化の範囲（RFC 9111 §4.4）

- unsafe メソッドのリクエストに対して 2xx または 3xx の応答が返ると、キャッシュはそのターゲット URI の保存エントリを無効化しなければならない（MUST）。
- `Location` / `Content-Location` ヘッダが指す URI の無効化は MAY（任意）であり、対象は同じオリジンの URI に限られる。他オリジンの URI を無効化してはならない。

## 推奨方法（独自の設計案。RFC の規定ではない）

- キャッシュ可能性を明示する設計にする。fresh 期間の主 directive は `max-age`（共有キャッシュで origin と中間で寿命を分けたい場合のみ `s-maxage` を追加）に統一し、`Expires` との二重指定で優先順位の混乱を招かない。`max-age` の値はオリジンの変更頻度と stale 許容度から決める。
- 再検証を前提にするリソースには `ETag` を付与する。`If-Modified-Since` だけに頼らず、`If-None-Match` を併用する（RFC 9110 §13.2 の優先順位により ETag 判定が勝つため、時刻丸めや1秒未満の更新の取りこぼしを避けられる）。
- `Vary` は実際に表現を変える軸（例: 言語・文字コード等の内容折衝軸）だけに絞る。軸を増やすほどキー空間が増えてヒット率が下がり、`Vary: *` は一切再利用できなくなるため使わない。
- unsafe メソッド（例: POST/PUT/DELETE/PATCH による更新系 API）の成功後は、少なくともターゲット URI の無効化を行う（RFC 9111 §4.4 の MUST に対応）。`Location` / `Content-Location` で派生 URI を返す設計では、同一オリジンに限って無効化対象に含めるかを事前に決める。他オリジンの無効化は規定上できないため、他オリジン側の整合性は別手段（短い freshness や明示的再取得）で確保する。
- stale を許す場合は許容条件（どのステータス・どの上限時間まで）を自組織で定義する。規定は「stale-allowed なら再利用できる」と枠組みだけを定め、許容可否の値は規定しないためである。

## 避ける使い方

- `Vary: *` のレスポンスを「キャッシュに載るから再利用される」と想定すること。再利用に一致しないと規定されている。
- `Expires` の日付が `max-age` や共有キャッシュの `s-maxage` を上書きすると想定すること。優先順位は逆で、`s-maxage` > `max-age` > `Expires` である。
- `If-Modified-Since` だけを送って ETag を無視する設計にすること。両方ある場合の判定は `If-None-Match` が優先されるため、ETag を送らないとサーバの判定根拠が時刻だけになる。
- 再検証なしで stale を無条件に返すこと。`no-cache` が未検証で付いている場合の再利用は規定に反する。
- unsafe メソッド成功時に `Location` / `Content-Location` の他オリジン URI まで無効化しようとすること。規定上は同一オリジンのみが対象であり、他オリジンの無効化はできない。
- `304` に本体が含まれると想定してパースすること。304 はバリデータ応答で本体なしと規定されている（RFC 9110 §15.4.5）。

## 適用版と本番での注意

- 適用版: RFC 9111（STD 98、2022年6月、RFC 7234 を obsolete）がキャッシュの基準文書。バリデータと条件付きリクエストの意味論は RFC 9110（STD 97、2022年6月、RFC 7231/7232 を obsolete）の §8.8–8.8.3、§13.1、§13.2、§15.4.5 による。
- 義務の強さは箇条ごとに違う。MUST: entity-tag の条件ヘッダ送出（`If-Match` / `If-None-Match` / `If-Range`）、unsafe メソッド成功時のターゲット URI 無効化。SHOULD: `Last-Modified` の `If-Modified-Since` 送出。MAY: 同一オリジンの `Location` / `Content-Location` URI の無効化、`If-None-Match` の `If-Modified-Since` に対する優先。
- 未確認・範囲外: `Age` の算出の詳細手順、ヒューリスティック freshness、キャッシュ不可メソッドの網羅一覧、私有キャッシュと共有キャッシュの全差異は本ドキュメントでは扱っていない。必要なら RFC 9111 の該当節を直接確認する。
- 再確認期限: 全 source が official_docs（TTL 90日）で、技術固有 TTL の対象外（`http` は config/freshness.json の technologies に未定義）。catalog の古い方の取得日（RFC 9110 は2026-09-26、RFC 9111 は2026-09-28）に90日を加えた最小日 2026-12-25 を文書の明示期限とした。期限到来時は両 RFC の該当節を再取得して確認する。

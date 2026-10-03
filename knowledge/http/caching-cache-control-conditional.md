---
{
  "id": "http-caching-cache-control-conditional",
  "title": "HTTP キャッシュ: Cache-Control の freshness と ETag/Last-Modified 条件付き再検証・再利用・無効化範囲",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9111 (STD 98, June 2022), RFC 9110 (STD 97, June 2022); whole document reverified 2026-10-03 UTC",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "caching",
    "cache-control",
    "freshness",
    "ETag",
    "Last-Modified",
    "conditional-request",
    "If-None-Match",
    "If-Modified-Since",
    "If-Match",
    "If-Range",
    "Vary",
    "304",
    "invalidation",
    "rfc9111",
    "rfc9110",
    "precondition-precedence",
    "request-directive",
    "304-selection",
    "client",
    "strong",
    "subrange",
    "advisory"
  ],
  "sources": [
    {
      "id": "rfc9111-cache-revalidation-contract-20261003",
      "url": "https://www.rfc-editor.org/rfc/rfc9111.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9110-conditional-validator-contract-20261003",
      "url": "https://www.rfc-editor.org/rfc/rfc9110.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# HTTP キャッシュ: Cache-Control の 鮮度 と ETag/Last-Modified 条件付き再検証・再利用・無効化範囲

GET レスポンスをキャッシュから再利用してよいか、古くなったエントリを条件付きリクエストで再検証するか、書き込み成功後にどの URI のエントリを無効化するかを、[RFC 9111 HTTP Caching](https://www.rfc-editor.org/rfc/rfc9111.html)（STD 98、2022年6月、RFC 7234 を obsolete）と [RFC 9110 HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html)（STD 97、2022年6月、RFC 7231/7232 を obsolete）を2026-10-03に本文全体と照合して整理する。以下で「文書化された事実」と「設計案」を明示的に分ける。

## 今回の訂正と問い

既存文書の「If-None-Match の優先が MAY」という要約は誤りだった。RFC 9110 §13.1.3は、`If-None-Match` がある場合に `If-Modified-Since` を無視することをMUSTとしている。また、Last-Modified送出のSHOULDから対象条件が落ち、リクエストのno-cacheとレスポンスのno-cacheを同じ強制規定としていた箇所を訂正した。新しい仕様変更ではなく、既存規定の再調査による同じ文書への修正である。

RFC 9111 §4.3.1のMUST規定自体に `If-Match` / `If-None-Match` / `If-Range` の列挙がある点は正しい。列挙を誤りとせず、用途と受信側の評価責務を補う。304の受信だけで、同じURIの全表現の変種を更新してよいとも解釈しない。

## 要点（文書化された事実）

### キャッシュキーと再利用条件（RFC 9111）

- キャッシュキーは少なくともリクエストメソッドとターゲット URI から成り、`Vary` が指名した元リクエストフィールドも表現の変種選択に組み込む。`Vary` の照合が不一致なら再検証なしで再利用できない。`Vary: *` はこの照合で常に不一致になる（never matches）（§2、§4.1）。
- 保存済みレスポンスの再利用には、URI・メソッド・`Vary` 等の適合に加えて、次のいずれかが必要である。(a) レスポンスが fresh、(b) stale の再利用が許されている、(c) 再検証に成功した。保存が許されること自体も別条件であり、§3の制限を満たす必要がある
- **レスポンスのno-cache**の引数なし形式は、再検証を送り成功応答を得るまで再利用を禁止するMUST NOTである。フィールド名付き形式には、そのフィールドを除外するか再検証して残りを使える条件があるが、特別扱いは広く実装されていない（§5.2.2.4）
- **リクエストのno-cache**は、オリジンによる再検証なしの再利用を望まないというクライアントの選好。リクエストディレクティブは助言的（advisory）で、キャッシュに実装義務を課すものではない（§5.2.1、§5.2.1.4）。requestだけで必ずオリジンへ到達した証拠にはしない

### 鮮度 の判定と優先順位（RFC 9111）

- fresh かどうかの判定式は `response_is_fresh = (freshness_lifetime > current_age)` である。freshness_lifetime が current_age を上回る間だけ fresh として再検証なしに再利用できる。
- freshness_lifetime の算出の優先順位は、共有キャッシュでは `s-maxage` があればそれが最優先、次にレスポンスの `max-age`、どちらもなければ `Expires` ヘッダの日付である。すなわち shared `s-maxage` > `max-age` > `Expires`-Date の順で適用される。`Expires` は `max-age` や共有キャッシュの `s-maxage` がある場合はそちらが優先される。

### バリデータのモデル（RFC 9110 §8.8–8.8.3）

- バリデータは `Last-Modified`（最終更新時刻）と `ETag`（entity-tag）の2種類である（§8.8）。
- `ETag` には strong と weak の比較があり（§8.8.3）、コンテンツネゴシエーションで選ばれた 表現の変種 ごとに値が変わりうる。弱いETag は意味的に等価な範囲での比較に使い、バイト単位の同一性を要求する用途には strong 比較が必要になる。
- `304 Not Modified` はバリデータに基づく再検証への応答であり、コンテンツ本体を含まない（§15.4.5）。

### 条件付きリクエストと優先順位（RFC 9110 §13.1、§13.2）

- 前提条件ヘッダは `If-Match`、`If-None-Match`、`If-Modified-Since`、`If-Unmodified-Since`、`If-Range` である（§13.1）。
- `If-None-Match` が存在すれば受信者は `If-Modified-Since` を無視しなければならない（**MUST ignore**、§13.1.3）。任意のMAYではない。両方を送るのは古い中継サーバーとの相互運用のためで、両条件をAND/OR結合する指示ではない
- 全前提条件には§13.2の評価順序もある。`If-Match`等を無視して、常にETag再検証だけを最優先とする意味ではない。`If-None-Match` のentity-tag比較にはweak比較を使うMUSTがあり、strong比較を使う `If-Match` や `If-Range` と一律に実装しない（§13.1）

### 再検証リクエストの送り方と 304 の効果（RFC 9111）

[RFC 9111 §4.3.1](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.3.1)の規定は条件付きで読む。

- 再検証対象の保存応答にentity-tagがあれば、関連するタグを `If-Match` / `If-None-Match` / `If-Range` で送るMUSTがある。これは三つの意味が同じという規定ではない。通常の応答の再検証は `If-None-Match`、既存表現の選択は `If-Match` / `If-Range` という区別を同節が示す
- Last-Modifiedの `If-Modified-Since` 送出がSHOULDとなるのは、**部分範囲（subrange）でなく、単一保存応答を検証し、その応答がLast-Modifiedを持つ**場合である
- 部分範囲（subrange）かつ単一保存応答で、Last-Modifiedだけを持ちentity-tagを持たない場合は、`If-Unmodified-Since` / `If-Range` による送出がMAY。ただしRFC 9110 §13.1.5の `If-Range` 条件も満たす必要があり、弱いentity-tagは禁止、日付には強いバリデータとしての条件がある
- 受信キャッシュはオリジン専用の前提条件を評価してはならない。`If-Match` / `If-Unmodified-Since` はキャッシュへ適用されず、これらをキャッシュ上のタグ比較で処理完了にしない（§4.3.2）。生成・転送できることと、その場で評価できることを区別する

[§4.3.4](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.3.4)の304処理はまず更新対象を選ぶ。URI等の再利用条件に適合する初期集合から、強いバリデータが一致する対象、strongがなくweakのみなら一致する最新対象、バリデータが全くない場合の限定条件、の順で絞る。強いバリデータがどの保存応答にも一致しない場合は、その304で保存応答を更新してはならない。

選ばれた対象だけを§3.2に従ってヘッダ更新する。これは全ヘッダの機械的上書きではなく、Content-Lengthや保存対象外フィールド等の例外がある。304には本文もトレーラもない（RFC 9110 §15.4.5）。元クライアントへ304を返すか、保存本体から200を作るかも同一ではない。例えばキャッシュが自身のETagをクライアントのリストへ追加して再検証し、304のタグが元クライアントのリストにない場合、§4.3.2は対応保存応答から200を生成するMUSTを定める。

### 無効化の範囲（RFC 9111 §4.4）

- unsafe（または安全性不明）メソッドのリクエストに対して、キャッシュが2xxまたは3xx応答を受けると、そのターゲットURIを無効化するMUSTがある。削除だけでなく、次回の再利用前に再検証を必須にする無効化も含む。通過しないキャッシュまで全世界的に無効化される保証はない。
- `Location` / `Content-Location` ヘッダが指す URI の無効化は MAY（任意）であり、対象は同じオリジンの URI に限られる。他オリジンの URI を無効化してはならない。

## 推奨方法（独自の設計案。RFC の規定ではない）

- キャッシュ可能性を明示する設計にする。fresh 期間の主 ディレクティブ は `max-age`（共有キャッシュで オリジン と中間で寿命を分けたい場合のみ `s-maxage` を追加）に統一し、`Expires` との二重指定で優先順位の混乱を招かない。`max-age` の値はオリジンの変更頻度と stale 許容度から決める。
- 再検証を前提にするリソースには `ETag` を付与する。`If-Modified-Since` だけに頼らず、`If-None-Match` を併用する（RFC 9110 §13.2 の優先順位により ETag 判定が勝つため、時刻丸めや1秒未満の更新の取りこぼしを避けられる）。
- `Vary` は実際に表現を変えるリクエストフィールド（例: Accept-Language / Accept-Encoding）に合わせる。必要な軸を省略してヒット率を稼がない。`Vary: *` を付けたまま通常のキャッシュヒットによる再利用を期待しない。
- unsafe メソッド（例: POST/PUT/DELETE/PATCH による更新系 API）の成功後は、少なくともターゲット URI の無効化を行う（RFC 9111 §4.4 の MUST に対応）。`Location` / `Content-Location` で派生 URI を返す設計では、同一オリジンに限って無効化対象に含めるかを事前に決める。他オリジンの無効化は規定上できないため、他オリジン側の整合性は別手段（短い 鮮度 や明示的再取得）で確保する。
- stale許容の時間・対象を設計し、レスポンスのno-cache / must-revalidate等の禁止を優先する。§4.2.4が認める切断時または明示的許可の条件を確認し、障害時なら何でもstaleを返せるとしない。

## 避ける使い方

- `Vary: *` の保存だけで、再検証不要のキャッシュヒットを得られると想定すること。
- `Expires` の日付が `max-age` や共有キャッシュの `s-maxage` を上書きすると想定すること。優先順位は逆で、`s-maxage` > `max-age` > `Expires` である。
- `If-Modified-Since` だけを送って ETag を無視する設計にすること。両方ある場合の判定は `If-None-Match` が優先されるため、ETag を送らないとサーバの判定根拠が時刻だけになる。
- リクエストのno-cacheだけを「必ずオリジン確認済み」の保証にすること、または引数なしの レスポンスのno-cacheを任意の選好へ弱めること。
- unsafe メソッド成功時に `Location` / `Content-Location` の他オリジン URI まで無効化しようとすること。規定上は同一オリジンのみが対象であり、他オリジンの無効化はできない。
- `304` に本体が含まれると想定してパースすること。304 はバリデータ応答で本体なしと規定されている（RFC 9110 §15.4.5）。

## 適用版と本番での注意

- 適用版: RFC 9111（STD 98、2022年6月、RFC 7234 を obsolete）がキャッシュの基準文書。バリデータと条件付きリクエストの意味論は RFC 9110（STD 97、2022年6月、RFC 7231/7232 を obsolete）の §8.8–8.8.3、§13.1、§13.2、§15.4.5 による。
- 義務は対象・条件とともに読む。MUST: タグが提供された再検証での関連タグ送出、If-None-Match存在時のIf-Modified-Since無視、unsafe非エラー応答時の対象URI無効化。SHOULD: 非部分範囲（subrange）・単一応答・Last-Modifiedありの送出。MAY: 同一オリジンの追加URI無効化など。優先順位をMAYの一覧へ入れない。
- 未確認・範囲外: `Age` の算出の詳細手順、ヒューリスティック 鮮度、キャッシュ不可メソッドの網羅一覧、私有キャッシュと共有キャッシュの全差異は本ドキュメントでは扱っていない。必要なら RFC 9111 の該当節を直接確認する。
- 取得日はUTC 2026-10-03。既存本文全体のキャッシュキー・鮮度・バリデータ・再検証・無効化・制約を両RFCの実ページと再照合し、新規出典記録 2件を参照した。旧出典記録は変更していない。両出典がofficial_docsの90日TTLなので明示期限は2027-01-01
- 両RFCのCopyright NoticeにBCP 78とIETF Trust Legal Provisions、Code ComponentsにはRevised BSDの条件がある。本文は独自要約で原文・コード転載なし
- CDN・ブラウザー・ライブラリ固有の適合性、リクエストディレクティブの実装差、実通信の障害試験は未実行。検索評価は可発見性の確認でありプロトコルの適合試験ではない

## 受け入れ試験案（未実行）

1. `If-None-Match` と `If-Modified-Since` に相反する条件を与え、MUST ignoreに従って日付側を判定へ混ぜない。ETagのweak比較と他前提条件の評価順も別に確認する
2. 通常の単一応答と部分範囲（subrange）を分け、Last-ModifiedのSHOULD条件を満たすか記録する。弱いETagをIf-Rangeへ転用しない
3. 複数表現の変種の保存状態で、304の強いバリデータが一致しないケースでは更新対象を作らない。weakのみの一致では最新対象に限定する
4. クライアントのETagリストにないキャッシュ自身のタグへ304が返った場合、保存本体から200を返す経路を検証する
5. リクエストのno-cacheと引数なしの レスポンスのno-cacheを分け、前者の助言的（advisory）を後者のMUST NOTと混同しない。フィールド名付き no-cacheの対応有無も製品ごとに確認する
6. unsafe更新を通らない別キャッシュにまで無効化されたと誤認せず、Location / Content-Locationが他オリジンの場合も無効化しない

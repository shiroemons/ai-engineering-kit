---
{
  "id": "http-retry-idempotency",
  "title": "HTTP API の再試行と冪等性: RFC 9110 の冪等メソッドと Retry-After",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9110 (STD 97, June 2022), RFC 6585 (April 2012)",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "api-design",
    "retry",
    "idempotency",
    "idempotent-methods",
    "retry-after",
    "idempotency-key",
    "429",
    "503",
    "rfc9110",
    "rfc6585",
    "rate-limiting"
  ],
  "sources": [
    {
      "id": "rfc9110-http-semantics",
      "url": "https://www.rfc-editor.org/rfc/rfc9110.txt",
      "type": "official_docs"
    },
    {
      "id": "rfc6585-additional-status-codes",
      "url": "https://www.rfc-editor.org/rfc/rfc6585.txt",
      "type": "official_docs"
    },
    {
      "id": "iana-http-status-code-registry",
      "url": "https://www.iana.org/assignments/http-status-codes",
      "type": "official_docs"
    },
    {
      "id": "ietf-idempotency-key-draft-status",
      "url": "https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/",
      "type": "official_docs"
    },
    {
      "id": "stripe-idempotent-requests-docs",
      "url": "https://docs.stripe.com/api/idempotent_requests",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# HTTP API の再試行と冪等性: RFC 9110 の冪等メソッドと Retry-After

通信失敗・過負荷・レートリミットのときに HTTP リクエストを再試行してよいか、再試行が重複副作用を生まないかを、標準文書の規定文言で整理する。冪等メソッドと自動再試行の可否は [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.txt)、429 Too Many Requests は [RFC 6585](https://www.rfc-editor.org/rfc/rfc6585.txt)、登録状況は [IANA HTTP Status Code Registry](https://www.iana.org/assignments/http-status-codes)、Idempotency-Key の標準化状態は [IETF datatracker](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)、実装例は [Stripe Idempotent requests](https://docs.stripe.com/api/idempotent_requests) を2026-09-26に取得して確認した。以下で「規定」「観測」「設計案」を明示的に分ける。

## 要点（標準文書に規定された事実）

### 冪等メソッドの定義（RFC 9110 §9.2.2 "Idempotent Methods"）

- 冪等の定義: "A request method is considered 'idempotent' if the intended effect on the server of multiple identical requests with that method is the same as the effect for a single such request." 本仕様が定義する方法のうち冪等なのは **PUT・DELETE と safe メソッド**（GET・HEAD・OPTIONS・TRACE、§9.2.1）で（§9.2.2 の一覧）、**POST は含まれない**。POST はターゲットリソースにリクエスト content をリソース固有の意味で処理させる方法であり（§9.3.3）、作成や追記のような機能を例に挙げる。
- 冪等性の適用範囲（§9.2.2）: 冪等の性質は「ユーザーが要求した効果」にのみ適用される。サーバは各リクエストを個別にログ記録したり、リビジョン履歴を保持したり、その他の非冪等な副作用を実装してよい。つまり冪等メソッドでもサーバ内部の監査ログ・メトリクスは非冪等でよい。
- なぜ区別するか（§9.2.2）: 冪等メソッドは「クライアントがサーバのレスポンスを読む前に通信失敗が起きた場合に、リクエストを自動的に繰り返せる」から区別される。PUT の例では、接続がレスポンス受信前に閉じられても新しい接続で再試行できる。元リクエストが成功済みでも同じ意図的効果だと分かっており、レスポンスの内容は異なる場合がある。
- 自動再試行の可否（§9.2.2）:
  - クライアントは、**(a) メソッドに関わらずリクエストの意味が実際に冪等だと知る手段**、または **(b) 元リクエストがまだ適用されていないことを検出する手段**がない限り、非冪等メソッドのリクエストを自動再試行してはならない（SHOULD NOT）。
  - 逆に、設計や設定から「あるリソースに対して安全だと分かる」場合、user agent は POST を自動で繰り返してよい、と例示される（§9.2.2）。
  - **プロキシは非冪等リクエストを自動再試行してはならない（MUST NOT）**: "A proxy MUST NOT automatically retry non-idempotent requests." クライアントは失敗した自動再試行を再び自動再試行してはならない（SHOULD NOT: "A client SHOULD NOT automatically retry a failed automatic retry."）（§9.2.2）。
  - §2.4 Error Handling は「一部のリクエストは underlying connection failure 時にクライアントが自動再試行できる」と §9.2.2 を参照する。
- 推測ベースの再試行（§9.2.2）: RFC 9110 は "Some clients take a riskier approach and attempt to guess when an automatic retry is possible." として、応答の一部も受信せずに接続が閉じた場合の POST 再試行を例に挙げる。推測ベースの自動再試行は仕様の中で riskier と記述されている。
- 421 Misdirected Request（§15.5.20）: クライアントはメソッドが冪等かどうかにかかわらず、別の接続で再試行してよい（MAY）。プロキシは 421 を生成してはならない（MUST NOT）。

### Retry-After と rate limiting 応答（§10.2.3、§15.6.4、§15.5.14、RFC 6585 §4）

- Retry-After（§10.2.3）: サーバが "how long the user agent ought to wait before making a follow-up request" を示すために送る。**503 と一緒ならサービスがどれだけ利用不能と予想されるか**、**3xx と一緒ならリダイレクト先リクエストを発行するまでの最低待ち時間**を示す。値は `Retry-After = HTTP-date / delay-seconds`。delay-seconds は `1*DIGIT` の非負整数（秒）で、`Retry-After: 120` は2分を意味する。
- 503 Service Unavailable（§15.6.4）: 一時的な過負荷か計画メンテナンスで現在リクエストを処理できない状態。サーバは Retry-After を送ってよい（MAY）。同節の注記に「503 の存在はサーバが過負荷時に 503 を使うことを意味しない。単に接続を拒否するサーバもある」とある。
- 413 Content Too Large（§15.5.14）: 条件が一時的な場合、サーバは Retry-After を生成すべき (SHOULD) で、一時であることと再試行可能な時刻を示す。
- **429 Too Many Requests は RFC 9110 には含まれない**。定義は RFC 6585 §4 で、IANA HTTP Status Code Registry（最終更新2025-09-15）でも429の参照は RFC 6585 のまま、503 の参照は RFC 9110 §15.6.4 である。RFC 6585 §4 の規定: 指定時間内にユーザーが多すぎるリクエストを送った（"rate limiting"）。応答表現は条件を説明する詳細を含むべき (SHOULD) で、新しいリクエストまで待つ時間を示す Retry-After を含めてよい (MAY)。429 応答はキャッシュに保存されてはならない (MUST NOT)。ユーザーをどう識別し、どうリクエストを数えるかは本仕様では定義しない。
- RFC 6585 §7.2（Security Considerations）: 攻撃下や大量リクエスト時に 429 を返し続けるとリソースを消費するため、サーバは 429 を使う義務はない（接続を落とす方が適切な場合もある）。
- 標準が埋めていない隙間: **RFC 9110 はステータスコードごとの再試行可否一覧を定義していない**。クライアントの再試行可否に関わる記述は §2.4・§9.2.2・§15.5.14・§15.5.20・§15.6.4（と Appendix B.3 の変更記録）に限られ、Retry-After が明示されるのは 503・3xx（§10.2.3）と 413（§15.5.14）で、429 との組み合わせは RFC 6585 §4 が定める。「5xx は再試行してよい」「4xx は再試行しない」といった分類は標準に存在せず、自組織の設計事項になる。

### Idempotency-Key は標準化されていない（2026-09-26 時点）

- draft-ietf-httpapi-idempotency-key-header-07 は **Expired Internet-Draft**（IESG state: Expired、WG state: WG Document）。datatracker は "This Internet-Draft is no longer active" と表示し、最新リビジョンは2025-10-15、ページ最終更新は2026-04-18。草案の要旨は「Idempotency-Key ヘッダで POST や PATCH のような非冪等メソッドを fault-tolerant にする」ものだが、**現時点で IETF が標準化した Idempotency-Key ヘッダは存在しない**。
- したがってヘッダ名・保持期間・同一性判定は各 API プロバイダの定義として扱う必要がある。

## 推奨方法（独自の設計案。RFC の規定ではない）

- 再試行を成立させるには、RFC 9110 §9.2.2 が認める2経路のどちらかを満たす設計にする。(1) リクエストの意味が実際に冪等だと分かるリソース設計、または (2) 元リクエストが適用されていないことを検出できる適用状態の記録。どちらでもない POST の自動再試行は SHOULD NOT の対象になる。
- 非冪等リクエストには、クライアント生成の一意キー（例: UUID）と、サーバが初回の結果を保存して同一キーの再送に同じ結果を返す仕組みを組み合わせる。これは上記 (2) の「検出手段」に当たる。標準ヘッダが失効済みのため、ヘッダ名は対象 API の仕様に合わせる。
- リトライ可否はステータスコードごとに自組織で定義する（標準に一覧がないため）。例: 503 / 429 は Retry-After を尊重して再試行、421 は別接続で再試行（冪等でなくても MAY、§15.5.20）、入力起因の 4xx は再試行しない。再試行には上限とバックオフを設け、失敗した自動再試行を再試行しない（§9.2.2 に沿う）。
- サーバ側では、一時的な過負荷時に Retry-After を返す（503 なら MAY、413 が一時条件なら SHOULD）。429 を返す場合も Retry-After を付けてよい（RFC 6585 §4）。ただし Retry-After のクライアント遵守は標準が強制しないので、自組織のクライアントでどう扱うかを明示的に決める。

## 実装例（観測事実: Stripe API ドキュメント。1社の API への事実であり一般化しない）

- 作成・更新時に idempotency key を渡すと、接続エラー時にリクエストを繰り返しても「2つ目のオブジェクトを作ったり更新を2回実行したりするリスクはない」とドキュメントは述べる。
- 仕組み: キーごとに最初のリクエストの結果（status code と body）を保存し、成功・失敗を問わず同一キーの後続リクエストは同じ結果を返す（500 エラーも含む）。
- キーはクライアントが生成する（V4 UUID や十分なエントロピーのあるランダム文字列を推奨、最大255文字、機密データは避ける）。
- キーは少なくとも24時間経過後に削除でき、削除後に同じキーが使われると新しいリクエストとして扱われる。パラメータが元リクエストと異なればエラーにする（誤用防止）。
- 結果はエンドポイントの実行開始後にのみ保存する。入力バリデーション失敗時や同時実行リクエストとの競合時は保存されないため、これらは再試行できるとドキュメントは述べる。
- POST はすべて idempotency key を受け付け、GET と DELETE には送らない（"These requests are idempotent by definition"）。

## 避ける使い方

- 応答を読めなかったというだけで非冪等リクエストを自動再試行し、元リクエストが適用済みかどうかを検証しない（§9.2.2 の SHOULD NOT に反する）。
- 再試行可能かどうかを推測する実装。RFC 9110 自体が riskier と記述する（§9.2.2）。
- プロキシ・ゲートウェイでの非冪等リクエストの自動再試行（MUST NOT、§9.2.2）。失敗した自動再試行の再自動再試行（SHOULD NOT）。
- Idempotency-Key を IETF 標準ヘッダとして扱うこと（草案は失効）。他社 API のヘッダ仕様を自社 API へ無確認で流用すること。
- Retry-After を無視した即時再試行を既定挙動にすること。標準はクライアントの遵守を強制しないので、待機と上限は設計として明示する。即時再試行の連打がレートリミットや過負荷を悪化させやすいという設計判断で避ける。

## 適用版と本番での注意

- 適用版: RFC 9110（STD 97、2022年6月、RFC 7231 ほかを obsolete）が HTTP semantics の基準文書。RFC 6585 は2012年4月で本文は RFC 2616 を参照するが、428 / 429 / 431 / 511 は現行 IANA レジストリに登録され続けている（最終更新2025-09-15）。
- 版の差: RFC 9110 の Appendix B.3 は「Restrictions on client retries have been loosened to reflect implementation behavior. (Section 9.2.2)」と記録し、RFC 7231 からクライアント再試行の制限が緩和されたことを示す。
- Retry-After の delay-seconds は非負整数のみ（`1*DIGIT`）。負値や小数は文法違反で扱われない。
- 義務は役割ごとに違う。プロキシ: 非冪等リクエストの自動再試行 MUST NOT。クライアント: 非冪等リクエストの自動再試行と、再試行の再試行は SHOULD NOT。サーバ: 503 の Retry-After は MAY、413 の一時条件では SHOULD。
- 未確認・範囲外: HTTP/2・HTTP/3 の接続断時の自動再試行挙動（トランスポート依存）は本調査の範囲外。idempotency key の保持期間・衝突処理はプロバイダごとに異なり RFC は規定しない。Webhook 送信側の再送スケジュールはプロバイダ固有で、本ドキュメントでは扱っていない。
- 再確認期限: 全 source が official_docs（TTL 90日）で、技術固有 TTL の対象外。2026-12-25 に再取得して内容を確認する。

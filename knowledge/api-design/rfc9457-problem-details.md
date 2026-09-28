---
{
  "id": "api-design-rfc9457-problem-details",
  "title": "RFC 9457 problem details による HTTP API エラー応答: type/title/status/detail/instance と validation 拡張の契約",
  "kind": "knowledge",
  "technology": "api-design",
  "version": "RFC 9457 (Standards Track, July 2023, obsoletes RFC 7807)、IANA HTTP Problem Types registry (Last Updated 2026-06-26)、いずれも 2026-09-28 取得",
  "tags": [
    "research-domain:api-distributed",
    "api-design",
    "http",
    "problem-details",
    "rfc9457",
    "error-response",
    "validation-error",
    "about-blank",
    "instance",
    "extension-member",
    "media-type",
    "pointer"
  ],
  "sources": [
    {
      "id": "rfc9457-problem-details",
      "url": "https://www.rfc-editor.org/rfc/rfc9457.html",
      "type": "official_docs"
    },
    {
      "id": "iana-http-problem-types-registry",
      "url": "https://www.iana.org/assignments/http-problem-types/http-problem-types.xhtml",
      "type": "official_docs"
    },
    {
      "id": "iana-problem-json-media-type",
      "url": "https://www.iana.org/assignments/media-types/application/problem+json",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# RFC 9457 problem details による HTTP API エラー応答: type/title/status/detail/instance と validation 拡張の契約

HTTP API のエラー応答を独自形式ではなく共通の problem details 形式で返すための契約を、3つの公式一次情報で整理する。メンバーの意味と制約は [RFC 9457 Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457.html) (Standards Track、July 2023、RFC 7807 を obsoletes)、問題種別の登録制度は [IANA HTTP Problem Types registry](https://www.iana.org/assignments/http-problem-types/http-problem-types.xhtml) (Created 2023-05-02、Last Updated 2026-06-26、Reference RFC 9457)、JSON 用メディアタイプの登録内容は [IANA application/problem+json](https://www.iana.org/assignments/media-types/application/problem+json) (Published specification RFC 9457) を2026-09-28に取得して確認した。以下で「記載事実」「設計案」を明示的に分ける。

## 要点（公式文書の記載事実）

### 5つの基本メンバーとその契約

- `type` は問題種別を識別する URI 参照である。省略時の既定値は `about:blank` であり、その場合は HTTP ステータスコード自体が種別の意味を持つ。
- `status` はその問題発生時の HTTP ステータスコードを繰り返す参考値 (advisory) である。存在する場合は実際の HTTP 応答コードと一致しなければならない (MUST)。
- `title` は問題種別の短く安定した要約である。発生ごとの個別事情ではなく、種別に結びついた固定の説明を入れる。
- `detail` はその発生 (occurrence) に固有の説明であり、利用者が誤りを正す助けになる内容を入れる。クライアントは `detail` を解析して処理分岐に使うべきではない (SHOULD NOT)。
- `instance` はその発生を識別する URI 参照である。同じ問題が再発したときに個々の発生を指し示す用途で使う。

### 拡張メンバーと複数種別混在時の扱い

- 問題種別ごとに定義された拡張メンバーを追加できる。受信者は認識できない拡張メンバーがあっても無視しなければならない (MUST)。この規則により、送信側は新しいメンバーを追加しても既存クライアントを壊さない。
- RFC 9457 が示す validation-error の例は、拡張として `errors` 配列を持ち、各要素が `detail` (個別エラーの説明) と `pointer` (RFC 6901 の JSON Pointer による対象箇所の指示) を持つ。つまり複数フィールドの検証失敗を1つの problem details 応答に束ねる形が公式の例として示されている。
- 複数の問題種別が混在する状況では、最も関連の高い問題 (most relevant problem) を応答すべきことが推奨されている。すべての問題を1つの応答に詰め込むのではなく、代表的なものを1つ選ぶ方針である。

### メディアタイプとレジストリ

- メディアタイプは JSON 用の `application/problem+json` と XML 用の `application/problem+xml` である。
- IANA の `application/problem+json` 登録では、必須・任意パラメータはなく、セキュリティ考慮は RFC 9457 の Section 5 にあり、想定用途は HTTP における COMMON である。
- IANA の HTTP Problem Types レジストリは Specification Required で、専門家 (experts) は Mark Nottingham と Sanjay Dalal である。`about:blank` の登録は Title が See HTTP Status Code で参照が RFC 9457 である。`https://iana.org/assignments/http-problem-types#` 接頭辞の利用が認められており、RFC 9458 による 400 系の追加登録が存在する。

## 推奨方法（独自の設計案。上記文書の規定ではない）

- エラー応答の既定形式を `application/problem+json` にし、5メンバーを次の方針で埋める。`type` は種別ごとに安定した URI を割り当て、種別化するほどでない一過性の失敗は `about:blank` のままにする。`title` は種別ごとに固定文言にする。`status` は必ず実際の応答コードと同じ値にする。`detail` には人間が次に取る行動が分かる文を入れ、エラーコードのような機械可読の分岐キーは `detail` ではなく拡張メンバーに置く。`instance` にはログと突き合わせ可能な発生 ID を含む URI を入れる。
- 検証失敗は RFC 9457 の validation-error の例にならい、トップレベルの `detail` に全体の要約、拡張の `errors` 配列に `{detail, pointer}` の組を入れる。`pointer` は RFC 6901 形式で対象フィールドを指す。クライアントは `pointer` で該当入力欄を特定し、`detail` は表示文として扱う。
- 拡張メンバーは追加してもよいが、クライアントが未知の拡張を無視する前提 (MUST ignore) で設計する。重要な分岐条件を拡張メンバーだけに依存させず、`type` と `status` だけで大枠の処理 (再試行可否、表示先) が決まる構成にする。
- 種別混在時 (例: 認証切れと検証失敗が同時) は most-relevant-problem の推奨に従い1つを選んで返す。落選した問題はサーバ側ログに残し、必要なら `detail` で存在に触れる。複数エラーの列挙が必要なのは検証失敗のような同種別の束ねだけで、異種別の詰め合わせはしない。

## 避ける使い方

- `status` メンバーと実際の HTTP 応答コードを食い違わせること (MUST 違反)。例: HTTP 200 で `status: 400` を返す、HTTP 422 で `status: 400` を返す。
- `detail` の文字列一致でクライアントの処理を分岐させること (`detail` は解析対象外と規定され、文言変更で壊れる)。
- `title` に発生ごとの個別情報 (ユーザー名、入力値、時刻) を埋め込むこと (`title` は種別に結びついた安定した要約が規定)。
- 未知の拡張メンバーがあるとエラー扱いで応答全体を破棄すること (未知の拡張は無視必須が規定)。
- 独自エラー形式と problem details をエンドポイントごとに混在させ、クライアントに両対応を強いること。共通形式を採用した意味が失われる。
- 問題種別の URI を登録・文書化せず、意味の不明な独自 `type` を量産すること。IANA レジストリは Specification Required であり、`about:blank` と接頭辞利用の範囲を超える種別は仕様文書が前提になる。

## 適用版と本番での注意

- 適用版: RFC 9457 (Standards Track、July 2023、RFC 7807 を obsoletes)、IANA HTTP Problem Types registry (Last Updated 2026-06-26)、IANA application/problem+json 登録 (Published specification RFC 9457)。いずれも 2026-09-28 取得。
- RFC 7807 との関係: RFC 9457 は RFC 7807 を obsoletes する。新規設計は RFC 9457 を基準にし、RFC 7807 時代の解説記事だけを根拠にしない。
- セキュリティ: `application/problem+json` の登録が指すセキュリティ考慮 (RFC 9457 Section 5) を本番前に読む。特に `detail` や `instance` に内部情報 (スタックトレース、内部ホスト名、個人情報) を漏らさないこと。本調査では Section 5 の条文ごとの確認はしていない。
- 未確認・範囲外: `application/problem+xml` の XML スキーマの詳細、RFC 9458 の 400 系各種別の定義内容、既存フレームワーク (言語・ミドルウェア) の problem details 対応状況、キャッシュや再試行との相互作用は本調査で確認していない。実装時は各フレームワークの対応版を確認する。
- 再確認期限: 全 source が official_docs (TTL 90日) で、技術 (api-design) 固有 TTL は設定されていない。2026-12-27 に RFC と両 IANA 登録を再取得し、レジストリの追加登録とメディアタイプ登録の変更を確認する。

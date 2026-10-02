---
{
  "id": "http-query-body-cache-cors-boundary",
  "title": "HTTP QUERY: RFC 10008 の body付き検索・cache key・CORS の配備境界",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 10008 (June 2026), IANA HTTP Method Registry (2026-06-17), RFC 9111 (June 2022), Fetch Living Standard / reported errata checked 2026-10-02",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "query",
    "rfc10008",
    "cache-key",
    "cors",
    "accept-query"
  ],
  "sources": [
    {
      "id": "rfc10008-query-contract-20261002",
      "url": "https://www.rfc-editor.org/rfc/rfc10008.html",
      "type": "official_docs"
    },
    {
      "id": "iana-query-method-registration-20261002",
      "url": "https://www.iana.org/assignments/http-methods",
      "type": "official_docs"
    },
    {
      "id": "whatwg-fetch-query-cors-20261002",
      "url": "https://fetch.spec.whatwg.org/",
      "type": "official_docs"
    },
    {
      "id": "rfc10008-reported-errata-20261002",
      "url": "https://errata.rfc-editor.org/search/?rfc_number=10008",
      "type": "official_docs"
    },
    {
      "id": "rfc9111-query-cache-storage-20261002",
      "url": "https://www.rfc-editor.org/rfc/rfc9111.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-12-31",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# QUERY の標準化だけで body付き検索を移行できるか

## 問いと採用判断

長い検索条件を POST body で送る API を、safe・idempotent な HTTP QUERY に置き換えるとき、既存 CDN・ブラウザー・再試行ポリシーをそのまま使えるか。

**判断:** client から origin までの method 対応、body を区別する cache、CORS を個別に確認してから限定導入する。以下の契約を基にした独自の設計判断であり、特定製品の対応表ではない。既存の `retry-idempotency.md` や `caching-cache-control-conditional.md` の一般則に対し、本稿は新しい method を通す境界に絞る。

## 確認した仕様

[RFC 10008](https://www.rfc-editor.org/rfc/rfc10008.html) は2026年6月公開の Proposed Standard。古い Internet-Draft の提案段階とは区別する。[IANA registry](https://www.iana.org/assignments/http-methods) でも QUERY は safe / idempotent と登録されている（更新日2026-06-17）。これは全製品の実装完了を意味しない。

RFC 10008 §2–3 の要点:

- QUERY は request content と Content-Type で検索を記述する。Content-Type の欠落・不一致は拒否が必須。未対応 media type には 415、処理不能な内容には 422 が候補となる。
- 応答は cacheable だが、cache key は body と関連 metadata を含める必要がある。意味を変えない正規化は任意であり、送信内容そのものの変更ではない。
- 2xx の Location は検索を再実行する equivalent resource、Content-Location は得られた結果への URI。両者を同じ snapshot と扱わない。
- 301 / 302 / 307 / 308 での移動は QUERY を保つ。303 は GET による取得へ進む。
- Accept-Query は利用可能な入力形式を示す Structured Fields の List。通常の Accept 用 parser を流用しない。

## cache と認証の境界

[RFC 9111 §3–4](https://www.rfc-editor.org/rfc/rfc9111.html) の保存・再利用条件も残る。cache が method を理解することが保存条件であり、URI だけを key にする既存 GET 用設定を QUERY へ拡張するのは危険。no-store、private、Authorization を含む共有 cache の条件、freshness と Vary は別々に確認する。

独自の推奨として、同じ endpoint に異なる filter、tenant、media type、encoding を与える対照試験を置く。単に cache hit が出ることではなく、異なる検索結果が混ざらないことを確認する。最初は共有 cache を無効化し、正規化と認証境界を検証した経路から有効にする。body に移した検索条件もログや tracing に残り得るため、入力の秘匿化と保存期限を別途設計する。

## Fetch と CORS の境界

[Fetch Standard §2.2.1 / §3.3](https://fetch.spec.whatwg.org/) では CORS-safelisted method は GET / HEAD / POST。QUERY の cross-origin 利用には preflight の許可が必要で、HTTP の Allow や Accept-Query だけでは CORS 許可にならない。no-cors は回避策として使えない。

また Fetch が大文字へ正規化する既定の method 一覧に QUERY はない。送信側は `QUERY` と明記し、小文字 `query` が自動補正されると期待しない。JavaScript から Accept-Query / Location / Content-Location を読む設計では、cross-origin response header の公開設定も別途必要。credentials を伴う場合、CORS の wildcard の意味が変わるため、method・header・origin を具体的に検証する。

## 段階導入の確認項目（独自案）

1. read-only の検索だけを移行対象にし、更新処理の endpoint を method 名だけ変えて流用しない。safe 宣言を認可の代替にしない。
2. SDK、gateway、WAF、router、origin の各 hop が同じ大文字 method と body を保持することを確認する。
3. 不正 Content-Type、未対応形式、query 失敗を分けて観測し、従来の POST へ暗黙に再送してエラーを隠さない。
4. 接続断を注入し、再試行の回数・deadline・負荷上限を確認する。idempotent は無制限 retry の許可ではない。
5. redirect と generated URI をテストし、結果を読む操作と検索を再実行する操作を利用者の UI でも区別する。
6. GET / POST の既存経路を契約付きで残し、実環境で拒否率・latency・cache correctness が確認できてから移行範囲を広げる。

## 版差・未確認事項

- 取得日は2026-10-02。RFC の公開月は6月であり、9月に初めて標準化されたとは記録しない。
- [errata index](https://errata.rfc-editor.org/search/?rfc_number=10008) の 9013 / 9016 はどちらも Reported。例示の request Vary と HTTP date 書式に対する報告で、承認済み訂正ではない。例文の丸写しを避け、規範節で契約を確認する。
- 本調査は一次資料レビュー。実際の browser / CDN / WAF / SDK の対応、cache normalization の安全性、性能は未検証。IANA 登録や Fetch の token 許容から対応版を推測しない。
- source のライセンスは catalog に記録した。本文は独自要約・独自運用案であり、仕様のコード例を転載していない。errata のライセンスは unknown のまま保持する。

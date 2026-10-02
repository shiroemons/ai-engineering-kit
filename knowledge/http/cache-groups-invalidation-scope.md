---
{
  "id": "http-cache-groups-invalidation-scope",
  "title": "HTTP Cache Groups: RFC 9875 の無効化範囲・MAY・非推移性と配備判定",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9875 (October 2025), RFC 9111 (June 2022), RFC 9651 (September 2024); retrieved 2026-10-02",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "cache-groups",
    "invalidation",
    "rfc9875",
    "origin",
    "structured-fields"
  ],
  "sources": [
    {
      "id": "rfc9875-cache-groups-20261002",
      "url": "https://www.rfc-editor.org/info/rfc9875/",
      "type": "official_docs"
    },
    {
      "id": "rfc9111-invalidation-boundary-20261002",
      "url": "https://www.rfc-editor.org/info/rfc9111/",
      "type": "official_docs"
    },
    {
      "id": "rfc9651-cache-group-strings-20261002",
      "url": "https://www.rfc-editor.org/info/rfc9651/",
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

# HTTP Cache Groups の無効化範囲をどこまで信用するか

## 問いと採用判断

商品更新の POST が成功したとき、一覧・詳細・おすすめの保存済み応答をまとめて無効化したい。`Cache-Group-Invalidation` を付ければ、ブラウザーから全 CDN 拠点まで一斉に消えるのか。

**判断:** RFC 9875 の group は関係を表す仕組みであり、分散 purge の完了通知ではない。利用中の cache が対応すること、処理対象の origin、更新応答が通る経路、無効化を任意から必須に強める実装契約を確認して採用する。これは以下の仕様を根拠にした独自の配備判断である。

本稿は2025年10月公開の [RFC 9875](https://www.rfc-editor.org/info/rfc9875/) を2026-10-02に再確認した coverage gap の補完で、2026年の新機能とは扱わない。既存の `caching-cache-control-conditional.md` が扱う freshness・条件付き再検証に対して、複数 URI の関係と無効化伝播の限界に焦点を絞る。

## RFC 9875 の契約

[§2–3](https://www.rfc-editor.org/info/rfc9875/) で確認した要点:

- `Cache-Groups` は応答が属する group、`Cache-Group-Invalidation` は無効化する group を表す。両方とも Structured Fields の String の List。順序に意味はなく、未知の Parameter は無視する。
- group の一致には、同じ cache 内、同じ URI origin、case-sensitive な文字列一致が必要。名前は opaque であり、prefix や path 階層として解釈しない。
- 保存済み応答を無効化するとき、同じ group の他の応答も無効化するのは **MAY**。group による無効化から別 group へさらに伝播させる仕組みではない（非推移性）。
- safe method（GET など）への応答では `Cache-Group-Invalidation` を **MUST ignore**。unsafe request への応答で対象 group を無効化するのは **MAY**。cache extension がこの要件を強める余地はある。
- 複数 cache の同期、別 origin の連携は規定対象外。対応実装の最低限の受理能力は1 field value に32 group、各32文字以上。これを送信可能な絶対上限と読み替えない。

### wire format は任意のラベル構文ではない

[RFC 9651 §3.3.3 / §4.2](https://www.rfc-editor.org/info/rfc9651/) の String は printable ASCII で、二重引用符を使う。quote と backslash の escape が定義されており、生の日本語や単引用符の値を同じ String として扱わない。Display String は別型なので、勝手に置換しない。

Structured Fields の parse failure は、field value 全体を無視するか message 全体を malformed として扱う。壊れた member だけを読み飛ばして部分的な purge を続ける独自 parser にしない。以下は文法説明用の独自例で、特定製品での動作検証済み設定ではない。

```http
Cache-Groups: "product:42", "catalog"
```

group を複数付けても `Cache-Control`、`Vary`、認証応答の保存条件は別途必要。[RFC 9111 §3–4](https://www.rfc-editor.org/info/rfc9111/) の保存・再利用条件を、このヘッダーだけで置き換えるものではない。

## URI 無効化の MUST と group の MAY を混ぜない

[RFC 9111 §4.4](https://www.rfc-editor.org/info/rfc9111/) は、unsafe method または安全性不明の method に対する2xx/3xx応答を受けた cache に、target URI の無効化を義務付ける。無効化とは一致する保存済み応答を除去するか、次の利用に必須の再検証が必要な状態にすること。

一方、RFC 9875 の group 連携は任意の動作である。更新先 URI が消えた証拠だけで、関連一覧も消えたとは判断しない。また RFC 9875 §3 は unsafe request への応答について定め、同節には RFC 9111 の2xx/3xx制限をそのまま繰り返していない。失敗応答に付ける際の動作を、根拠なく「必ず無視される」と決め付けない。アプリ側では、更新が確定した応答にだけ必要な signal を出す方針を明示するのが本稿の設計案である。

## 非推移性を確かめる独自の検証シナリオ

同一 cache / origin に次の保存済み応答を用意する。

- A: group `product:42`
- B: group `product:42` と `catalog`
- C: group `catalog`

A の無効化に伴って B が group 経由で無効化されても、それを理由に C まで連鎖させる規定ではない。C も更新対象なら、元のイベントで `catalog` を対象にするよう依存関係を設計する。逆に、B 自身を元の無効化対象にした場合と混同しない。

同じ文字列を持つ別 origin の D、大小文字だけ異なる E も用意し、広すぎる invalidation を検出する。これは仕様境界から作成したテスト案であり、このリポジトリで HTTP cache を立てて実測した結果ではない。

## 導入前のチェックリスト（独自の運用案）

1. 読み取りと更新の経路を描き、signal を受ける cache を列挙する。更新だけ origin へ直通する場合、他の cache に通知されるとは仮定しない。
2. 各 cache の製品・版・設定と、group 無効化を実際に行う契約を確認する。単にヘッダーが応答に残ることと、実装済みであることを区別する。
3. 同一 origin を複数 tenant が使うなら、信頼する層だけが両ヘッダーを発行・変更できるようにする。名前の prefix だけを権限検査の代わりにしない。RFC 9875 §5 は共有 origin 内で他者の資源に副作用を与え得る点を警告している。
4. cold / warm、GET / POST、大小文字、origin 差、非推移性、更新エラー、malformed field、上限超過の組み合わせを試す。期待する挙動は仕様の MUST と製品の任意機能を分けて記録する。
5. 更新後の実データと cache hit の両方を観測し、対象漏れと origin 負荷増大を測る。未対応経路には既存の purge、再検証、許容できる freshness の別手段を残す。

## 制約・取得失敗・再確認

- ブラウザー、CDN、reverse proxy の実装対応率や版別互換性は未調査。仕様の存在を実装保証にしない。全拠点での同時無効化、順序保証、更新応答を失った場合の再配布も本稿では保証しない。
- `https://www.rfc-editor.org/rfc/rfc9875.html` は429だったため、RFC Editor の info URL に掲載された全文・公開月・Copyright Notice を確認した。errata の2 URL は取得失敗し、errata の有無は未確認。配備前に再確認する。
- source の取得日はすべて2026-10-02。official_docs の90日 TTL に合わせ2026-12-31を再確認期限とする。IETF Trust の権利表示を catalog に記録し、RFC の長文・実装コードは転載していない。
- eval は検索で本稿に到達することを検査するだけで、HTTP の実装適合性・信頼境界の実測試験ではない。

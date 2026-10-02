---
{
  "id": "http-compression-dictionary-negotiation-cache-boundary",
  "title": "HTTP Compression Dictionary Transport: 辞書交渉・Vary・安全なfallbackの境界",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9842 (September 2025), RFC 9111 §4.1; Chrome 130 introduction and 2025-05-14 maintainer report, rechecked 2026-10-02 UTC",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "compression-dictionary",
    "dcb",
    "dcz",
    "available-dictionary",
    "vary",
    "fallback",
    "cors",
    "privacy"
  ],
  "sources": [
    {
      "id": "rfc9842-dictionary-negotiation-20261002",
      "url": "https://www.rfc-editor.org/rfc/rfc9842.html",
      "type": "official_docs"
    },
    {
      "id": "rfc9111-dictionary-vary-matching-20261002",
      "url": "https://www.rfc-editor.org/rfc/rfc9111.html",
      "type": "official_docs"
    },
    {
      "id": "chrome-dictionary-syntax-migration-20261002",
      "url": "https://developer.chrome.com/blog/shared-dictionary-compression",
      "type": "maintainer_article"
    },
    {
      "id": "chrome-search-dictionary-rollout-20261002",
      "url": "https://developer.chrome.com/blog/search-compression-dictionaries",
      "type": "maintainer_article"
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

# HTTP Compression Dictionary Transport: 辞書交渉・Vary・安全なfallbackの境界

## 問いと選定理由

辞書を使った差分圧縮を CDN/API 配信へ導入するとき、初回アクセスや古い辞書を持つ利用者にも同じ URL を安全に配信できるか。2026年の新機能とは主張せず、既存 corpus にない durable gap として [RFC 9842](https://www.rfc-editor.org/rfc/rfc9842.html) の交渉とキャッシュ境界を再確認した。通常のキャッシュ鮮度は [既存文書](caching-cache-control-conditional.md) に譲り、ここでは辞書に依存する表現の選択を扱う。

## 確認した契約

- サーバーは辞書応答の `Use-As-Dictionary` で利用対象を示す。`match` は必須で、正規表現を含む URLPattern は使えない。辞書と圧縮対象の origin は一致が必要。辞書は fresh、または stale 配信が許される状態でなければ候補にならない（RFC 9842 §2）。
- `Available-Dictionary` は選んだ1個の辞書本文の SHA-256 を表す。`Dictionary-ID` はサーバー用の識別子で、内容の保証には使えない。使用前の hash 検証を省略できない（§2.1.3、§2.2）。
- `Accept-Encoding` の `dcb` / `dcz` は、その要求に辞書圧縮を使えるという広告。合う辞書がない、または使用しないクライアントはこれらを送ってはならない。辞書 hash と content coding は別々に交渉する（§6）。
- cacheable な辞書圧縮応答は、非対応クライアントや別辞書へ流用されない `Vary` が必要。RFC の例は `accept-encoding, available-dictionary`（§6.2）。[RFC 9111 §4.1](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.1) では、指定された要求 header が一致しない応答を再検証なしに使えない。header の欠落は別の要求でも欠落している場合にのみ一致する。
- `rel="compression-dictionary"` は取得の手掛かり。リンクだけでは登録完了を保証せず、取得した辞書応答にも `Use-As-Dictionary` と caching header が必要（RFC 9842 §3）。

## 旧記事からの移行と事例の読み方

[Chrome の2024年3月6日記事](https://developer.chrome.com/blog/shared-dictionary-compression) は公開後の警告として、旧 `br-d` / `zstd-d` を `dcb` / `dcz`、旧 `rel="dictionary"` を `rel="compression-dictionary"` へ変更したと明示している。古いサンプルをそのまま設定へ持ち込まない。同記事に残る origin trial・flag・CDN の記述を2026年の現状とは扱わない。

[2025年5月14日の開発元報告](https://developer.chrome.com/blog/search-compression-dictionaries) は Chrome 130 での導入と Google Search の展開を説明する。例は初回 `br`、辞書取得後の要求で `Available-Dictionary` を送り `dcb` を受ける流れ。報告された平均23%の HTML 転送量削減は、標準 Brotli 圧縮との比較で、初回など辞書を使わない応答も含む Chrome 利用者全体での平均であり、全サイトの削減率ではない。現在の全ブラウザー対応表や特定 CDN の対応保証には外挿しない。

## 安全性は圧縮率と別に判定する

[RFC 9842 §8–10](https://www.rfc-editor.org/rfc/rfc9842.html#section-8) は HTTPS を要求する。ブラウザーでは辞書と対象応答の両方が読み取り可能である必要がある。ページからは cross-origin でも CORS に通る場合があるが、辞書と圧縮対象の same-origin 制約を解除する意味ではない。

公開辞書でも秘密を含む応答が安全になるとは限らず、攻撃者が変えられる内容と圧縮サイズ等から情報が漏れる可能性がある。辞書 hash は将来の要求で追跡子にもなり得るため、クライアントは cookie 同等以上の分離と cookie 消去時の辞書消去を扱う。これらを満たしたという実装検証は本調査では行っていない。

## 導入判断と受入テスト（独自の設計案）

以下は RFC の追加義務ではなく、配信システムに適用する提案である。

1. まず公開・versioned な静的資産で試し、対象応答と辞書を同じ release manifest で管理する。可変な `Dictionary-ID` だけで圧縮済み成果物を選ばず、hash と encoding の一致を必須にする。
2. 辞書なしの初回、既知辞書、古い辞書、未知 hash、期限切れ、cookie 消去後を別ケースにする。通常圧縮の fallback を保持し、未知 hash のとき別辞書用 `dcb` を返さない。すべての表現を復号した本文が同じであることを確認する。
3. 同じ URL に対して辞書 A、辞書 B、辞書なしの順序を入れ替えて共有キャッシュを試験する。CDN 独自のキー正規化や header 除去を含め、`Vary` が end-to-end に効くかを観察する。default 応答と辞書応答の片方だけに不十分なキーを付ける設計を避ける。
4. cross-origin の CORS 成功・失敗と `no-cors` を分ける。private data と攻撃者入力が混在する動的応答は、脅威評価が終わるまで辞書圧縮の対象外にする。HTTPS だけで圧縮 side channel が消えるとは扱わない。
5. cold/warm の転送量、辞書取得 bytes、CPU、decode failure、cache hit率、LCP を測る。辞書更新頻度と再訪率を含む総費用で判断し、旧辞書の保持期間と配信停止時の fallback を先に決める。

## 適用範囲・未確認事項・provenance

- RFC 9842 は2025年9月の Proposed Standard、RFC 9111 は2022年6月の STD 98。取得日は2026-10-02 UTC。新規公開日と取得日を混同しない。
- RFC 9842 の errata 用 URL と公式検索 endpoint は今回取得できなかった。訂正がないとは主張せず、実装時に [RFC の情報ページ](https://www.rfc-editor.org/info/rfc9842/) から再確認する。
- ブラウザー別の現在の対応版、HTTP/1～3 各実装の挙動、CDN 設定、encoder/decoder の実装、実測の性能・情報漏えい耐性は未確認。これは wire format の実装手引きではなく、導入可否と配信試験の文書である。
- RFC は IETF Trust の条項、Chrome 記事の本文はページ表示の CC-BY-4.0 を確認。原文・コードの転載はせず、事実の独自要約と設計提案を分けた。code/module 昇格なし。全 source の再確認期限は90日で、2026-12-31を明示期限とした。

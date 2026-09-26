---
{
  "id": "javascript-fetch-abort-cancellation",
  "title": "JavaScript fetch cancellation with AbortController and AbortSignal timeout scope and cleanup",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "MDN AbortSignal (2026-09-01) + WHATWG DOM s3 (2026-09-24) + WHATWG Fetch s4-5 (2026-09-21) + MDN fetch() (2025-12-16); retrieved 2026-09-26",
  "tags": [
    "research-domain:frontend",
    "fetch",
    "AbortController",
    "AbortSignal",
    "AbortError",
    "TimeoutError",
    "timeout",
    "cancellation",
    "signal",
    "cleanup",
    "removeEventListener",
    "throwIfAborted"
  ],
  "sources": [
    {
      "id": "mdn-abort-signal-docs",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal",
      "type": "official_docs"
    },
    {
      "id": "whatwg-dom-abort-signal",
      "url": "https://dom.spec.whatwg.org/#interface-AbortSignal",
      "type": "official_docs"
    },
    {
      "id": "whatwg-fetch-standard",
      "url": "https://fetch.spec.whatwg.org/",
      "type": "official_docs"
    },
    {
      "id": "mdn-window-fetch-docs",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/Window/fetch",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# JavaScript fetch cancellation with AbortController and AbortSignal timeout scope and cleanup

`fetch` の network 処理と response body 読み取りを `AbortController` / `AbortSignal` で取り消す方法と、timeout 適用範囲・後始末の条件。[MDN AbortSignal](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal) / [WHATWG DOM §3](https://dom.spec.whatwg.org/#interface-AbortSignal) / [WHATWG Fetch](https://fetch.spec.whatwg.org/) / [MDN fetch()](https://developer.mozilla.org/en-US/docs/Web/API/Window/fetch)

## 要点

以下は公式一次情報の記述であり、推奨構成そのものではない。

### signal の束縛と単回性 (MDN AbortSignal + WHATWG Fetch)

- `new AbortController()` の `controller.signal` を `fetch(url, { signal })` の `RequestInit.signal` に渡す。Fetch 標準は `Request.signal` を fetch-controller の abort / terminate 状態に束縛する。
- signal は単回使い切り (single-use) である。既に abort 済みの signal を新しい `fetch` に渡すと、network に出る前に即時拒否される。再試行には新しい `AbortController` を作る。
- API 統合パターンは、依存処理の前に `signal.aborted` の確認と abort 手順の追加を求める。`signal.throwIfAborted()` は abort 済みの場合に abort 理由で throw する。

### fetch の解決・拒否と HTTP error status (MDN fetch())

- `fetch(resource, options)` は headers 到達時点で解決する `Promise<Response>` を返す。`options` は `Request()` コンストラクタと同一の `RequestInit` であり、`signal` を含む。
- 拒否するのは network 失敗・不正 URL・block された要求、および `AbortController.abort()` による取り消し (後者は `AbortError` の `DOMException`) に限られる。
- HTTP error status (4xx / 5xx) では拒否されない。`Response.ok` / `Response.status` の確認が必須である。

### エラー名: AbortError と TimeoutError (MDN AbortSignal)

- `controller.abort()` による取り消しでは `fetch()` が `AbortError` の `DOMException` で拒否される。abort 後に response body を読む操作も `AbortError` で拒否される。
- `AbortSignal.timeout()` による期限切れパスの拒否は `TimeoutError` である。`catch` では `err.name` で区別する。
- `AbortSignal.any()` は timeout signal と明示的な signal を合成できるが、timeout 由来か abort 由来かの区別は失われる。どちらが発火したかを `name` だけで判別できない前提で扱う。

### signal 適用範囲: network + body 消費 (WHATWG Fetch)

- signal の適用範囲は network だけでなく response body の消費も覆う。`await response.text()` の待機中に abort しても拒否される。
- abort された fetch は、明示的な理由 (reason) が渡されない限り、直列化された `AbortError` の fallback 理由を使う。`controller.abort(reason)` で明示理由を渡した場合はその理由が用いられる。

### DOM 規範 interface と listener 意味 (WHATWG DOM §3)

- 規範 interface は `AbortController` / `AbortSignal`、signal の `abort` イベント、`reason`、`throwIfAborted()`、静的メソッド `AbortSignal.timeout()` / `AbortSignal.any()` / `AbortSignal.abort()` を定義する。
- `addEventListener` の options として `signal` (別 signal による自動解除)・`once`・`passive`・`capture` の意味が定義されている。一回限りの abort ハンドラには `once: true` が使える。

### GC 保持の違いと解放義務 (MDN AbortSignal + WHATWG DOM)

- controller 所有の signal は listener が付いていても GC 回収可能である。一方 `timeout()` / `any()` の signal は、pending の間または listener が付いている間は保持される。
- そのため `timeout()` / `any()` の signal に付けた listener は `finally` で `removeEventListener` し、`await response.text()` 等で body を消費または破棄して保持を解く。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が定める実装構成そのものではない。

- 要求ごとに `AbortController` を作る。連続する fetch で使い回さず、共同取り消しが意図の場合だけ `AbortSignal.any()` で合成する。

```js
const controller = new AbortController();
function onAbort() { /* 一回限りの後始末 */ }
controller.signal.addEventListener("abort", onAbort, { once: true });
try {
  const res = await fetch(url, { signal: controller.signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const text = await res.text(); // 読取中の abort も AbortError で拒否される
  controller.signal.throwIfAborted(); // 後続の依存処理前の guard
  return text;
} catch (err) {
  if (err?.name === "AbortError") { /* 取り消しとして扱う */ throw err; }
  throw err;
} finally {
  controller.signal.removeEventListener("abort", onAbort);
}
```

- 純粋な deadline には `AbortSignal.timeout(ms)` を使う。利用者の明示 abort と合成する場合は `AbortSignal.any()` を使うが、区別喪失を受け入れる。

```js
const combined = AbortSignal.any([controller.signal, AbortSignal.timeout(5000)]);
const res = await fetch(url, { signal: combined });
// NOTE: combined の拒否では timeout 由来か abort 由来かを name だけで区別できない。
// 区別が必須なら自前の flag (どちらが先に発火したか) で記録する設計にする。
```

- timeout と利用者 abort の区別が必須の場合は、`setTimeout` + `controller.abort()` の手動 timeout 構成にし、発火元を自前の flag で記録する (公式契約ではなく設計判断)。
- `timeout()` / `any()` の signal に付けた listener は必ず `finally` で `removeEventListener` する。body は消費 (`await response.text()`) または reader 破棄まで行い、保持を残さない。
- エラー分岐は `err.name === "AbortError"` と `err.name === "TimeoutError"` で行う。`any()` 合成時は上記の区別喪失に注意し、flag 併用を検討する。

## 避ける使い方

- **abort 済みの signal (使い終わった controller) を次の fetch に使い回す**。即時拒否される。要求ごとに作り直す。
- **HTTP error status が fetch を拒否するという想定**。拒否されず `Response.ok` が `false` の解決になる。status 確認を省くと失敗を見落とす。
- **`AbortSignal.any()` が発火元を保持するという想定**。timeout と abort の区別は失われる。区別が必要な設計では flag を持つ。
- **`timeout()` / `any()` の signal の listener を付けっぱなしにする**。pending または listener がある間は保持され、GC されない。`finally` で `removeEventListener` する。
- **body を消費せず放置する**。保持が残る。`await response.text()` 等で消費または破棄する。
- **abort 後の body 読み取りを通常失敗として扱う**。`AbortError` で拒否されるため、取り消し分岐に入れる。

## 適用版と本番での注意

- `MDN AbortSignal` (last modified 2026-09-01、Baseline Widely available)、`WHATWG DOM Living Standard §3` (last updated 2026-09-24)、`WHATWG Fetch Living Standard §4-5` (last updated 2026-09-21)、`MDN Window: fetch()` (last modified 2025-12-16、Baseline Widely available) を 2026-09-26 取得の内容で確認した。将来の最新とは扱わない。
- 本文は `expires_at` 2026-12-25 (official_docs TTL 90日)。Living Standard は継続改訂のため、期限到来時に §3・§4-5 の改訂を再確認する。
- **未確認**: `AbortSignal.timeout()` / `AbortSignal.any()` のブラウザ別対応版 (compat table は今回の根拠に含めない)。導入前に利用環境の対応表を確認する必要がある。
- **未確認**: abort 理由 (reason) の直列化のブラウザ間の差、React の effect cleanup や二重実行との組み合わせ、Service Worker 経由時の挙動。本文は Fetch / DOM の契約のみを根拠にし、適用構成の実測は含まない。
- 本文の timeout 構成・flag 併用・cleanup 順序は設計判断であり、公式 API 契約そのものではない。境界は各節の書き分けに従う。

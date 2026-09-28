---
{
  "id": "javascript-event-loop-microtask-rendering-order",
  "title": "Event loop の task と microtask の順序と requestAnimationFrame の描画タイミング",
  "kind": "knowledge",
  "technology": "javascript",
  "version": "WHATWG HTML Living Standard 8.1.6.6.4/8.1.7/8.7/8.8/8.12 (Last Updated 28 September 2026) + MDN queueMicrotask (2026-08-11) / requestAnimationFrame (2026-08-21); retrieved 2026-09-28",
  "tags": [
    "research-domain:frontend",
    "event-loop",
    "microtask",
    "task",
    "queueMicrotask",
    "setTimeout",
    "requestAnimationFrame",
    "Promise",
    "rendering",
    "animation-frame",
    "rendering-opportunity"
  ],
  "sources": [
    {
      "id": "whatwg-html-webappapis-event-loop",
      "url": "https://html.spec.whatwg.org/multipage/webappapis.html",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-timers-queue-microtask",
      "url": "https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html",
      "type": "official_docs"
    },
    {
      "id": "whatwg-html-animation-frames",
      "url": "https://html.spec.whatwg.org/multipage/imagebitmap-and-animations.html",
      "type": "official_docs"
    },
    {
      "id": "mdn-window-queuemicrotask-docs",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/Window/queueMicrotask",
      "type": "official_docs"
    },
    {
      "id": "mdn-window-requestanimationframe-docs",
      "url": "https://developer.mozilla.org/en-US/docs/Web/API/Window/requestAnimationFrame",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/frontend.json"
  ]
}
---

# Event loop の task と microtask の順序と requestAnimationFrame の描画タイミング

`queueMicrotask` / `Promise` と `setTimeout` の実行順序、そして `requestAnimationFrame` (rAF) コールバックと style/layout 再計算・ペイントの相対順序。[WHATWG HTML webappapis](https://html.spec.whatwg.org/multipage/webappapis.html) / [WHATWG HTML timers](https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html) / [WHATWG HTML animation frames](https://html.spec.whatwg.org/multipage/imagebitmap-and-animations.html) / [MDN queueMicrotask()](https://developer.mozilla.org/en-US/docs/Web/API/Window/queueMicrotask) / [MDN requestAnimationFrame()](https://developer.mozilla.org/en-US/docs/Web/API/Window/requestAnimationFrame)

## 要点

以下は公式一次情報の記述であり、推奨構成そのものではない。末尾の順序表は仕様の処理モデルからの導出である。

### タスクの直後にマイクロタスク checkpoint が走る (HTML §8.1.7.3)

- イベントループの処理モデルは、実行可能な oldestTask を 1 つ選んで「Perform oldestTask's steps」を実行し、**直後に**「Perform a microtask checkpoint」を行う。
- checkpoint は「While the event loop's microtask queue is not empty」で、キューが空になるまでマイクロタスクを実行する。再入を防ぐフラグがあり、マイクロタスク中で新たに積まれたマイクロタスクも同じ checkpoint で消化される。
- §8.1.7.1 は「The microtask queue is not a task queue」と明記する。マイクロタスクは「queue a microtask」で作られる。
- したがって現在のタスク中に `queueMicrotask` や `Promise.then` で積まれたコールバックは、そのタスク完了後・**次のタスクより前に**、登録順 (FIFO) で実行される。MDN queueMicrotask も「現在のタスク完了後、イベントループへ制御を返す前に実行される」(enqueued microtasks are executed after all pending tasks have completed but before yielding control to the browser's event loop) と説明する。

### Promise レクションはマイクロタスクとして実行される (HTML §8.1.6.6.4)

- `HostEnqueuePromiseJob` について仕様は「HTML schedules these operations in the microtask queue」「Queue a microtask to perform the following steps」と書く。`then` / `catch` / `finally` のレクションはマイクロタスクであり、タスク単位の checkpoint で消化される。

### setTimeout は通常のタスクであり、マイクロタスクではない (HTML §8.7)

- timer initialization steps は「queues a global task on the timer task source given global to run task」とし、`setTimeout` は**通常のタスク**をキューする。`queueMicrotask` とは別の階層にあり、現在のタスクの直後 checkpoint では走らない。
- 丸めと下限:「If timeout is less than 0, then set timeout to 0.」「If nestingLevel is greater than 5, and timeout is less than 4, then set timeout to 4.」。仕様の要約は「Timers can be nested; after five such nested timers, however, the interval is forced to be at least four milliseconds.」
- 時刻の保証はない:「This API does not guarantee that timers will run exactly on schedule. Delays due to CPU load, other tasks, etc, are to be expected.」さらに run steps after a timeout は同一 global・orderingIdentifier の先行タイマー待ちに加え、「Optionally, wait a further implementation-defined length of time.」(UA による追加遅延) を含む。
- `queueMicrotask(callback)` は「The queueMicrotask(callback) method must queue a microtask to invoke callback」で、`setTimeout(f, 0)` と違って「This doesn't yield control back to the event loop」。また「if the goal is to run code before the next rendering cycle, that is the purpose of requestAnimationFrame()」と明示する。

### rAF は登録だけして、描画更新タスクの中で style/layout より前に走る (HTML §8.12 + §8.1.7.3)

- `requestAnimationFrame` は handle を返し、target の ordered map「map of animation frame callbacks」に callback を登録するだけである。この時点では実行されない。
- To run the animation frame callbacks はキー順に「if handle exists in callbacks」を確認し、callback を map から**削除してから**「Invoke callback with « now » and "report"」する。つまりコールバックは登録順に **1 回だけ**実行され、次フレームは再登録が必要 (one-shot)。`window` と `DedicatedWorkerGlobalScope` が `AnimationFrameProvider` を満たす。
- 実行タイミング (webappapis §8.1.7.3): rendering opportunity 発生時に「queue a global task on the rendering task source … to update the rendering」という**通常のタスク**がキューされ、その中で resize steps → scroll steps → media queries → update animations and send events →「run the animation frame callbacks for doc」→「Recalculate styles and update layout for doc」→ … →「mark paint timing」→「update the rendering or user interface」の順に進む。つまり **rAF コールバックはそのフレームの style/layout 再計算とペイントの前**に走る。
- rendering opportunity は実装依存だが、60Hz なら最大で「every 60th of a second (about 16.7ms)」ごとが上限。「ensure certain tasks are executed immediately after each other, with only microtask checkpoints interleaved (and without, e.g., animation frame callbacks interleaved)」という最適化も仕様が明示的に許容している。
- MDN requestAnimationFrame:「requests the browser to call a user-supplied callback function before the next repaint」。頻度は一般にディスプレイリフレッシュレート (60Hz / 120Hz / 144Hz 等) に一致し、バックグラウンドタブでは停止され得る。timestamp は「the end time of the previous frame's rendering」で、同一フレームで発火する複数コールバックは同一 timestamp を受け取る。

### 順序のまとめ (仕様からの導出)

仕様が明示する範囲から、同一タスク中で次のコードを走らせたときの順序は次のとおり。**timer task と rendering task の相対順序は仕様が保証しない** (どちらも通常のタスクで、rendering task は rendering opportunity 時にのみキューされるため)。

```js
console.log("sync");
setTimeout(() => console.log("timeout"), 0);
queueMicrotask(() => console.log("microtask"));
Promise.resolve().then(() => console.log("promise"));
requestAnimationFrame(() => console.log("rAF"));
// sync → microtask → promise → (通常タスク) timeout / rAF の相対順序は保証されない
```

1. 同期コード (`sync`)
2. 直後のマイクロタスク checkpoint: 登録順に `microtask` → `promise` (FIFO、両者とも queue a microtask)
3. 以降の通常タスク: `setTimeout` の timer task と rAF を含む rendering task。rAF コールバックはその rendering task の中で style/layout・ペイントの前に実行される

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が定める実装構成そのものではない。

- **次の描画より前に必ず走らせたい処理**は rAF に入れる。仕様が「next rendering cycle の前」を rAF の目的と明示している。
- **現在のタスクが終わったら即座に、次のタスクより前に走らせたい処理**は `queueMicrotask` または `Promise.then` に入れる。マイクロタスクはタスク単位の checkpoint で全件消化される。
- **描画を 1 回挟みたい・処理を次のタスクに委譲したい**場合に `setTimeout(f, 0)` を使う (マイクロタスクと違いイベントループへ制御を返す)。ただし 4ms clamp と実装定義の遅延を前提に、時刻厳守は期待しない。
- rAF を毎フレーム連続で走らせるなら、コールバック内で再登録する (one-shot)。

```js
// 毎フレームの再登録 (one-shot である仕様への対応)
function tick(now) {
  // now は previous frame's rendering の終了時刻。同一フレームの複数 callback は同一値
  requestAnimationFrame(tick);
}
requestAnimationFrame(tick);

// タスク完了直後に確実に走らせたい後始末
queueMicrotask(() => updateDerivedState());
```

- rAF 内で行う DOM 読取 (getBoundingClientRect 等) は、そのフレームの style/layout 再計算の**前**に走るため、直前の変更の反映結果にならない可能性がある点を踏まえて配置する (これは順序の導出からの含意であり、仕様の推奨ではない)。

## 避ける使い方

- **`setTimeout(f, 0)` をマイクロタスクと同一視する**。timer task source の通常タスクであり、現在のタスクの直後 checkpoint では走らない。
- **rAF が登録するだけで毎フレーム自動で繰り返されると思い込む**。コールバックは map から削除されて 1 回だけ実行され、毎回の再登録が必要。
- **マイクロタスクの無限チェーンでも描画は保たれると思う**。HTML §8.8 / MDN は大量のマイクロタスクが `setTimeout(f, 0)` と同様にイベントループへ制御を返さず、同期実行同様に描画を妨げると明記する。
- **`setTimeout` の delay を正確なスケジュールとして扱う**。ネスト 5 回超で最小 4ms、CPU 負荷による遅延、UA の実装定義な追加遅延がある。
- **バックグラウンドタブでも rAF が一定頻度で走る前提で組む**。停止され得る (MDN)。
- **`timeout / rAF の前後が常にこうだ」と固定順を仮定する**。両者は別タスク源の通常タスクで、rendering opportunity の発生タイミングに依存する。

## 適用版と本番での注意

- `WHATWG HTML Living Standard` §8.1.6.6.4 / §8.1.7 (webappapis)、§8.7 / §8.8 (timers-and-user-prompts)、§8.12 (imagebitmap-and-animations) (いずれも Last Updated 28 September 2026)、`MDN Window: queueMicrotask()` (last modified 2026-08-11、Baseline Widely available)、`MDN Window: requestAnimationFrame()` (last modified 2026-08-21、Baseline Widely available) を 2026-09-28 取得の内容で確認した。将来の最新とは扱わない。
- 本文は `expires_at` 2026-12-27 (official_docs TTL 90日)。Living Standard は継続改訂のため、期限到来時に §8.1.7 の処理モデルと §8.12 の再編を再確認する。
- **未確認**: rendering opportunity の判定基準、UA による timer の実装定義遅延、バックグラウンドタブでの rAF 停止条件はいずれも実装依存であり、ブラウザ間の実測差は本ドキュメントの根拠に含めない。
- **未確認**: React 等のフレームワークのバッチングや Scheduler と本順序の組み合わせ、Worker での rAF (DedicatedWorkerGlobalScope は AnimationFrameProvider を満たすが、Worker 側の描画更新タスクの詳細は未確認)。
- 順序のまとめ (「順序のまとめ」節とコード例のコメント) は仕様の処理モデルからの**導出**であり、仕様が直接書いた固定順序ではない。timer task と rendering task の相対順序は仕様が保証しない。

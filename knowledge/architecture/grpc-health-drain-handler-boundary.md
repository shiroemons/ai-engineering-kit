---
{
  "id": "architecture-grpc-health-drain-handler-boundary",
  "title": "gRPC NOT_SERVING は drain ではない: health・新規受理・stream/handler 終了の境界",
  "kind": "knowledge",
  "technology": "architecture",
  "version": "gRPC live guides checked 2026-10-24 UTC; grpc-go v1.84.0 (2026-09-17, e84aa5ab15d1d2b29d54f838312ad490cb7551a8); source review only, not a latest-version claim",
  "tags": [
    "research-domain:api-distributed",
    "architecture",
    "gRPC",
    "health-checking",
    "NOT_SERVING",
    "admission",
    "GracefulStop",
    "streaming",
    "shutdown",
    "handler-lifetime"
  ],
  "sources": [
    {
      "id": "grpc-health-routing-contract-20261024",
      "url": "https://grpc.io/docs/guides/health-checking/",
      "type": "official_docs"
    },
    {
      "id": "grpc-graceful-stop-guide-20261024",
      "url": "https://grpc.io/docs/guides/server-graceful-stop/",
      "type": "official_docs"
    },
    {
      "id": "grpc-keepalive-health-boundary-20261024",
      "url": "https://grpc.io/docs/guides/keepalive/",
      "type": "official_docs"
    },
    {
      "id": "grpc-cancellation-drain-20261024",
      "url": "https://grpc.io/docs/guides/cancellation/",
      "type": "official_docs"
    },
    {
      "id": "grpc-go184-health-drain-implementation-20261024",
      "url": "https://github.com/grpc/grpc-go/tree/e84aa5ab15d1d2b29d54f838312ad490cb7551a8",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2027-01-22",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/api-distributed.json"]
}
---

# gRPC NOT_SERVING は drain ではない: health・新規受理・stream/handler 終了の境界

## 問いと結論

終了開始時に health を `NOT_SERVING` にしたのに業務 RPC が届くのはなぜか。Go の `health.Server.Shutdown()` を呼んだ後、`grpc.Server.GracefulStop()` が戻らない場合、何を待っているのか。

**health は協調する呼出し側への状態通知、admission は新しい仕事の受理判断、drain は受理済み RPC の終了待ち**である。さらに transport を閉じることと、handler・その子 goroutine・外部への副作用が終わることも別に観測する。似た API 名や health の赤表示を、一連の停止が完了した証拠にしてはいけない。

これは新機能の紹介ではなく、未収録だった持続的な API・運用上の gap を補う。既存の [gRPC retry と業務確定](grpc-retry-commit-replay-boundary.md)は失われた応答と replay、[Kubernetes Pod 終了](../kubernetes/pod-graceful-shutdown.md)は外側の終了期限と hook を扱う。本稿は gRPC 内部の health、受理、長寿命 stream と handler の停止に絞る。

## 1. health の状態が制御できる範囲（公式契約）

[Health Checking guide](https://grpc.io/docs/guides/health-checking/)では、health service を登録するだけで依存先や業務状態が自動判定されるわけではなく、application が状態を更新する。`Check` は unary の照会、`Watch` は状態更新を流す streaming RPC である。

client-side health checking は channel の service config などで有効化され、その言語実装と load balancing policy が対応している場合に機能する。接続先で `Watch` を開始し、対象 service が `SERVING` になるまで通常の要求を送らず、不健康になれば送信を避ける。したがって次を区別する。

- health を監視しない client、異なる service 名を監視する client、health checking を無効にする policy には、同じ送信抑制を期待しない。
- `Watch` が `UNIMPLEMENTED` を返すと health checking は無効になる。**health API 未実装を fail-closed な業務拒否としない**。固定 Go 実装の [clientHealthCheck](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/health/client.go)も、この場合は load balancing 用に `Ready` を報告する。
- 状態を更新した瞬間に全 client が通知を受信したとは限らない。通知は server-side の業務 admission gate ではない。厳密な受理停止が必要なら server 自身で受理条件を管理する、というのが本稿の設計上の帰結である。

[固定版の Go client 例](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/examples/features/health/client/main.go)は health package の import、`round_robin`、`healthCheckConfig.serviceName` を組み合わせる。JSON を配ったことだけで有効化済みと判断せず、実際の client 言語・版・resolver・policy で `Watch` と送信抑制を確認する。一般 guide の policy 例を、全言語・全版の対応表として転用しない。

### 空文字は wildcard ではない

全体の health を表す空文字 `""` と、`orders.v1.OrderService` のような個別名は別の監視キーである。[grpc-go v1.84.0 の health/server.go](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/health/server.go#L52-L70)では、`NewServer()` は空文字のキーだけを `SERVING` で初期化し、`Check` は要求されたキーをそのまま参照する。

- 個別名だけを `NOT_SERVING` にしても、空文字を監視する client の値が自動で変わるわけではない。逆も同様。空文字は wildcard や子サービスの自動集約ではない。
- 業務 service の登録と health のキー登録は別である。未登録キーへの `Check` は `NOT_FOUND`、[Watch](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/health/server.go#L90-L132)は `SERVICE_UNKNOWN` の通知から始まり、後の状態更新を待てる。同じ失敗形式とは扱わない。
- 起動中の readiness を表したいなら、公開前に監視対象キーを明示的に設定する。既定の空文字 `SERVING` を「DB 接続と全業務 handler の準備が完了した」という検査結果にしない。

## 2. 二つの Server と三つの終了 API（固定 Go 実装）

以下は **grpc-go v1.84.0** に限定した比較であり、他言語の API 名・timeout 引数・既定値を保証しない。[health の実装](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/health/server.go#L134-L187)と [gRPC server の実装](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/server.go#L1671-L1728)を分けて読む。

| 操作 | この版で行うこと | その操作だけでは証明できないこと |
|---|---|---|
| `health.Server.SetServingStatus(name, NOT_SERVING)` | 指定キーを更新し、そのキーの watcher に通知を用意する | 業務 RPC の server-side 拒否、他キーの変更、既存 stream の終了 |
| `health.Server.Shutdown()` | 登録済み全キーを `NOT_SERVING` にし、以後の `SetServingStatus` を無視する | listener の閉鎖、業務 admission の停止、Watch の終端 |
| `grpc.Server.GracefulStop()` | 新規接続/RPC の受理を停止し、既存 transport を drain、pending RPC/handler の終了を待つ | 一定時間以内の復帰。timeout引数はなく、別途上限を管理する |
| `grpc.Server.Stop()` | listener・接続を閉じ、進行中 RPC を cancel する | handler の全終了、子 goroutine の join、業務更新の rollback |

`health.Server.Resume()` は health の更新受付を再開し、登録済みの全キーを `SERVING` にする。以前の個別状態を復元する API ではなく、停止した `grpc.Server` の listener を再起動する API でもない。停止中の probe が `SetServingStatus` で健康に戻してしまうのを抑えるには `Shutdown` の性質が使えるが、安易な `Resume` はその抑制を解除する。再開時には実際の readiness と受理方針を別途整合させる。

### health.Shutdown 自体は Watch を閉じない

固定した [Watch の loop](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/health/server.go#L114-L132)は、更新を送信した後も次の更新を待つ。終了経路は送信失敗または stream context の cancellation であり、`NOT_SERVING` を送ったことだけでは return しない。`Shutdown` も stream の close や handler の join を行わない。

したがって **Watch も長寿命 RPC として停止設計に含める**。例えば直接開いた health Watch を client が保持したままなら、業務 unary RPC がすべて終わっていても、health の状態変更だけで全 pending RPC がなくなったとはいえない。この source 上の帰結を「どの client でも必ず停止が hang する」という実測結果へ拡張しない。client、proxy、cancellation の条件で最終的な stream 寿命は変わる。

## 3. transport を強制停止しても handler の寿命は別

[Graceful Shutdown guide](https://grpc.io/docs/guides/server-graceful-stop/)は、graceful な完了待ちに別の timer を設け、長引いた場合に forceful shutdown を呼ぶ構成を示す。Go の [公式例](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/examples/features/gracefulstop/server/main.go)も `GracefulStop` と timer からの `Stop` を分ける。ガイドの「timeout」を、Go の `GracefulStop` に組込みの時間制限があるという意味で読まない。

ただし fallback の意味も限定する。

1. [WaitForHandlers の契約](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/server.go#L645-L657)では、既定の `Stop` は handler 終了を待たない。接続が閉じた後も method handler が実行中の可能性がある。
2. `WaitForHandlers(true)` は `Stop` に handler 終了待ちを追加する Experimental option であり、handler を強制中断する機能でも timeout でもない。協調停止しない handler を待つ構成に変えるだけでは終了上限を保証できない。
3. [stop の L1721–L1723](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/server.go#L1687-L1728)は、`graceful` または `waitForHandlers` が真なら `handlersWG.Wait()` を実行する。**並行して Stop を呼んでも、GracefulStop が handler 待ちを省略する契約にはならない**。transport を閉じた事実だけから、GracefulStop 用の完了通知が必ず発火したと扱わない。

最後の項目は固定 source の分岐を確認したもので、特定の goroutine interleaving や deadlock を再現した主張ではない。`Stop` の呼出し時刻、接続終了、各 API の復帰、handler 終了を別のイベントとして採る。

[Cancellation guide](https://grpc.io/docs/guides/cancellation/)が説明するように、gRPC library は一般に任意の application handler を強制中断できず、handler が取消しを確認して処理を止める必要がある。CPU loop、DB 呼出し、下流 RPC、独自 worker の終了を application が協調させる。handler から切り離した子 goroutine は、gRPC の method handler が返ったことだけで join 済みにはならない。完了済みの業務更新を取消しが取り消すという保証も置かず、結果不明時の再実行判断は既存の retry 文書へ委ねる。

## 4. stream の停止を「新規 RPC の拒否」だけで済ませない

以下は上記契約から導く独自の設計案である。

- **bidirectional stream 内の新規 message と、新規 RPC の admission を分ける。** stream 開始時に一度だけ動く interceptor で受理停止しても、既に受理済みの stream が運ぶ新しい注文・書込みまで拒否したことにはならない。どの message まで受理したか、終了通知後の送信をどう扱うかを業務 protocol で決める。
- **長寿命 stream には終了・再接続の契約を用意する。** server 側 drain signal を handler が観測し、受理済みの有限な単位を完了させて return する設計にする。再接続先、最後に確定した cursor/sequence、重複処理の扱いは application の契約であり、GracefulStop は自動生成しない。
- **無期限に新しい仕事を読み続ける handler を、自然に空になる queue とみなさない。** stream の入力継続、待機、取消しを独立に試験する。health Watch のような制御用 RPC も棚卸しする。
- **keepalive を drain 完了検査にしない。** [Keepalive guide](https://grpc.io/docs/guides/keepalive/)の HTTP/2 PING は接続の生存確認であり、application health とは別である。PING ACK があっても `NOT_SERVING` や停止中の handler と両立する。逆に keepalive 失敗による切断は、進行中 stream の失敗になりうるので、正常な業務完了の代用ではない。

## 5. 停止シーケンスの設計案

単一の「shutdown 成功」フラグではなく、次の境界を管理する。順序・時間配分は本サービス用の設計であり、gRPC が全体を自動保証するものではない。

1. **受理境界を確定する。** 厳密な cutoff が必要なら、`draining` への遷移と新規業務の登録を同じ同期境界で判定する。単に flag を読んだ後で無同期に仕事を追加すると、停止と競合できる。stream 内の message も対象にする。health 自体まで一律に拒否して状態通知を壊さない。
2. **監視対象の health を不健康にする。** 全体停止なら登録済みキーへの `Shutdown`、一部の業務だけなら個別状態更新を選ぶ。client に迂回を促すための通知であって、全員の受信確認や受理停止の代わりではない。先に health を落として待機する方式を採る場合、その待機中に受理する仕事も予算へ含める。
3. **有限な drain を開始する。** handler へ終了要求を伝え、`GracefulStop` による新規 RPC 停止と進行中処理の完了待ちを行う。既存 RPC の deadline と、server 全体の drain 上限は別々に管理する。
4. **上限超過時は強制 transport 停止へ進む。** 独立した timer から `Stop` を呼ぶなどの fallback を設ける。ただし、それを handler 全終了の証拠にせず、残存仕事を観測する。`WaitForHandlers` の設定と graceful 側の待機条件も review する。
5. **外側の最終期限と後始末を分ける。** flush、子 worker の停止、資源解放に時間を残す。絶対的な process 終了上限を要する場合は、停止中の handler を無制限に待つ API とは別の外側の終了手段を設計する。強制終了を使った経路を正常 drain と報告しない。Pod の期限配分は既存 Kubernetes 文書と合わせて確認する。

観測には少なくとも health 更新、admission cutoff、受理済み RPC 数、長寿命 stream 数、handler/worker 終了、GracefulStop/Stop の開始と復帰、強制終了の有無を分ける。health の `NOT_SERVING` 件数だけでは停止進捗を評価できない。

## 6. 運用受け入れ試験案

下表は実施すべき fault/boundary test の提案であり、実行結果ではない。試験自体には watchdog と fixture の cleanup を設け、無限待ちを成功条件にしない。

| 条件 | 比較・観測すべき結果 |
|---|---|
| `NewServer` 直後、個別キーだけを NOT_SERVING | 空文字を監視する client と個別名の client を分け、同じ health 値になると仮定しない |
| 未登録キーの `Check` と `Watch` | NOT_FOUND と SERVICE_UNKNOWN を区別し、後のキー登録への追随を確認する |
| health 対応/未設定 client を同じ server へ接続 | 通知だけの状態と server-side admission gate を併用した状態で、業務 handler の新規開始数を比較する |
| `Watch` が UNIMPLEMENTED を返す server | health checking が fail-closed にならないことを検査し、必要な業務拒否は別途行う |
| 停止中に background probe が SERVING を再設定 | health.Shutdown 後は変更が無視されること、Resume は全登録キーを SERVING にすることを確認する |
| 有限 unary RPC の処理中に GracefulStop | 新規 RPC と受理済み RPC を分け、受理済み分の終端と graceful API の復帰を追う |
| 直接開いた health Watch を保持して health.Shutdown | NOT_SERVING の受信と Watch の終端を別に観測し、client cancel / Stop で確実に fixture を閉じる |
| bidirectional stream の開始後に cutoff、さらに新規 message | RPC 開始時の gate だけで message admission まで閉じたと判定しない |
| cancellation を無視する handler を置き、drain 上限から Stop | 接続/RPC の終了、Stop の復帰、GracefulStop の復帰、handler 実行継続を個別に記録する |
| 同じ fixture に WaitForHandlers(true) | Stop の handler 待ちと外側 watchdog を確認し、無期限待機を timeout 機能と取り違えない |
| PING ACK は継続するが health は NOT_SERVING | 接続の生存と業務受理可能性を別メトリクスで表示する |

## 版・出典・未確認事項

- 全資料の取得日は **2026-10-24 UTC**。一般ガイドは固定製品版を持たない。表示更新日は Health Checking が2024-05-20、Graceful Shutdown が2025-01-15、Keepalive が2025-11-10、Cancellation が2024-02-29。取得日やページ更新日を機能の導入日としない。
- 実装は grpc-go **v1.84.0**、tag が指す40桁 commit `e84aa5ab15d1d2b29d54f838312ad490cb7551a8` に固定。[version 更新 commit](https://github.com/grpc/grpc-go/commit/e84aa5ab15d1d2b29d54f838312ad490cb7551a8)の日時は2026-09-17T20:03:25Z、`version.go` と版付き API 文書を照合した。最新版という主張ではなく、この版で停止機構が新設されたとも主張しない。
- [grpc.io の LICENSE](https://github.com/grpc/grpc.io/blob/main/LICENSE)は documentation が CC-BY-4.0、code/code samples が Apache-2.0。[固定 grpc-go の LICENSE](https://github.com/grpc/grpc-go/blob/e84aa5ab15d1d2b29d54f838312ad490cb7551a8/LICENSE)および確認した source headers は Apache-2.0。本文は独自要約・設計案で、上流コードの転載・module 化なし。
- 一般ガイドと版付き API 文書は native web で確認。固定 GitHub file の web cache miss は成功と扱わず、read-only GitHub connector で source・LICENSE・tag・commit を取得して補った。
- **gRPC client/server を起動した runtime試験は今回実行していない**。通信、fault injection、upstream test、実負荷での停止時間、proxy/mesh の drain、Go 以外の実装、grpc.Server の ServeHTTP 経路は未検証。表は試験案であり、検索 eval の成功は停止挙動や副作用消失の証明ではない。

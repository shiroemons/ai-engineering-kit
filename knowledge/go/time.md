---
{
  "id": "go-time",
  "title": "Go time のタイマーと monotonic clock",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 (time package API)",
  "tags": [
    "time",
    "timer",
    "ticker",
    "deadline",
    "timeout",
    "monotonic",
    "clock",
    "duration",
    "sleep",
    "stop",
    "reset",
    "concurrency"
  ],
  "sources": [
    {
      "id": "go-time-docs",
      "url": "https://pkg.go.dev/time",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-23",
  "expires_at": "2026-12-22",
  "trust": "official",
  "status": "active",
  "evals": [
    "evals/knowledge/search.json"
  ]
}
---

# Go time のタイマーと monotonic clock

`time` は時刻の計測と表示、経過時間 `Duration`、単発の `Timer` と周期の `Ticker` を持つ標準ライブラリである。[公式 API](https://pkg.go.dev/time)

## 要点

- monotonic clock の役割: 公式は「wall clock は時刻を知るため、monotonic clock は時間を測るため」と明記する。`time.Now` は wall 読み取りと monotonic 読み取りの両方を持つ。両方の `Time` 同士での `After` / `Before` / `Equal` / `Compare` / `Sub` は monotonic 読み取りだけを使い、片方でも monotonic を持たなければ wall にフォールバックする。`Since(start)`、`Until(deadline)`、`Now().Before(deadline)` のような惯用も wall clock の変更に強いと公式は述べる。
- monotonic を消す操作: `AddDate`・`Round`・`Truncate` は wall 計算、`In`・`Local`・`UTC` は wall の解釈変更で、いずれも結果から monotonic 読み取りを必ず消す。公式が示す正規の消し方は `t = t.Round(0)`。`Date` / `Parse` / `ParseInLocation` / `Unix` で作った時刻や、シリアライズ（`GobEncode`・`MarshalBinary`・`MarshalJSON`・`MarshalText`）と `Format` も monotonic を含まない。monotonic はプロセス内にしか意味がなく、`Duration` や `t.Unix` 系の戻り値にも含まれない。
- 比較の落とし穴: Go の `==` は時刻の instant のほか `Location` と monotonic 読み取りまで比較する。公式は map key や DB key に `Time` をそのまま使わず、同一 `Location` に揃えて `t.Round(0)` で monotonic を消すか、一般的には `t.Equal(u)` を使えと明記する。`Equal` は異なる `Location` の同じ instant を等しいと判定する。
- スリープと monotonic: 一部のシステムではコンピュータがスリープすると monotonic clock が止まる。その場合 `t.Sub(u)` などが実経過を正確に反映せず、必要なら monotonic を消して wall で測る。
- `Timer` の契約: `Timer` は単一イベントを表し、公式は `NewTimer` か `AfterFunc` で作ることを要求する。`NewTimer(d)` は少なくとも `d` 経過後に現在時刻を `C` へ送る。`Stop` は発火を防ぎ、自分が停止に成功したら true、既に期限到来か停止済みなら false を返す。`Reset` は期限を `d` に変更し、元が active なら true、期限到来か停止済みなら false を返す。
- Go 1.23 の分岐: Go 1.23 以前の timer channel は非同期（buffered、capacity 1）で、`Stop` / `Reset` の後でも古い時刻を受信しうった。公式は「1.22 以前での唯一の安全な手順は `Stop` を呼び、false を返したら明示的に drain すること」と明記する。Go 1.23 以降は channel が同期的（unbuffered、capacity 0）になり、`Stop` 後の受信は必ずブロックし、`Reset` 後の受信が以前の設定の時刻を受け取ることはないと保証される。
- GC 回収: Go 1.23 以降、GC は参照されなくなった未期限到来・未停止の timer と ticker を回収する。公式は「Stop は GC を助けるためには不要。他の理由で Stop する場合は別」と明記する。`After(d)` は `NewTimer(d).C` と等価で、公式は「After で足りるなら NewTimer を選ぶ理由はない」と変更した。
- `AfterFunc` の戻り値: `AfterFunc(d, f)` は経過後に `f` を独自 goroutine で呼ぶ。返す `Timer` の `C` は使われず nil。`Stop` が false を返したら `f` は既に開始済みで、`Stop` は `f` の完了を待たない。`Reset` が false を返した場合も前の `f` の完了を待てず、次の `f` が前の `f` と並行する可能性がある。完了を知るには呼び出し側が `f` と明示的に協調する。
- `Ticker` の契約: `NewTicker(d)` は `d > 0` でないと panic する。受信が遅い側に対しては公式が「間隔（time interval）を調整するか、tick を落とす」と明記する。`Ticker.Reset` も `d > 0` でないと panic する。`Ticker.Stop` は channel を閉じない。理由は公式が `Reset` を許すためと、channel を読む並行 goroutine が誤った tick を見るのを防ぐためだと述べる。
- `Sleep` は少なくとも `d` 待つ。負または 0 の duration は即座に返る。
- `Duration` は int64 のナノ秒数で、最大は約 290 年。Day 以上の単位定義は「 daylight savings time zone 転換の混乱を避けるため」無い。`ParseDuration` の単位は `ns`、`us`（または `µs`）、`ms`、`s`、`m`、`h`。
- Timer resolution: 公式は runtime・OS・hardware 依存と明記し、Unix は約 1ms、Windows 1803 以降は約 0.5ms、古い Windows の既定は約 16ms（`golang.org/x/sys/windows.TimeBeginPeriod` で要求可能）と記載する。
- 並行利用: `Time` 値は複数 goroutine から同時に使えるが、`GobDecode` / `UnmarshalBinary` / `UnmarshalJSON` / `UnmarshalText` は並行安全でない。公式は時刻を value として保持・受け渡すことを例に示す。zero value は 0001-01-01 00:00:00 UTC で、`IsZero` で判定する。`Date` に nil の `Location` を渡すと panic する。

## 推奨方法

以下は上記の公式契約からの設計上のまとめであり、公式が推奨する数値や構成を指すものではない。

- プロセス内の経過時間の計測は `Now` + `Sub` / `Since`、残り時間の判定は `Until(deadline)` / `Before` を使う。保存・表示・プロセス間の比較は UTC に揃え、比較は `Equal` を使う。map key に使う場合は `UTC` と `Round(0)` で正規化するか、key に時刻そのものを入れない設計にする。
- Go 1.23 以降だけを対象にするコードでは `Stop` / `Reset` 後の drain 手順を省ける。Go 1.22 以前も支えるコードでは従来どおり「`Stop` が false を返したら drain」を維持し、対象バージョンで分岐する。
- `AfterFunc` の `f` は並行実行され得るものとして書き、完了同期は `f` の側に持たせる。
- `Ticker` は遅い受信で drop / 間隔調整が起きる前提で扱い、厳密な周期計測には使わない。終了時は `Stop` を呼ぶ。channel は閉じられないため、`range` で終了を待つ設計にはしない。
- 残り時間を deadline として持ち歩く場合は `Time` の value を渡し、判定は `Until` に集約する。wall clock の変更やスリープの影響を受ける環境では、計測と表示で monotonic の有無を意識して測り方を選ぶ。

## 避ける使い方

- `t == u` で時刻を比較する、または `Location` と monotonic の一致を保証せずに `Time` を map / DB key にする。公式は `Equal` の利用を明示的に推奨する。
- 対象バージョンを確認せず timer の drain 手順を書く。Go 1.23 以降では `Stop` 後に stale な時刻は受信できず、逆に 1.22 以前では drain なしの `Stop` / `Reset` が不安全だったと公式は明記する。
- `NewTicker` や `Ticker.Reset` に 0 以下の間隔を渡す。いずれも panic する。
- `Ticker.Stop` しても channel が閉じられて range が抜けると想定する。公式は閉じない理由を明示している。
- `AfterFunc` の `Stop` / `Reset` が false を返したことに「`f` は動いていない」「`f` は終わった」を読む。公式は開始済み・完了非保証・並行可能性を明記している。
- Day 単位の `Duration` 定数を探す。公式は daylight savings 転換の混乱を避けるため Day 以上を定義しない。単位の換算は `Duration` の割り算・乗算で行う。
- シリアライズした時刻や `Parse` で作った時刻に monotonic が残っていると考える。残らない。
- `Date` に nil の `Location` を渡す。panic する。
- timer resolution より細かい精度を保証として扱う。公式は OS 依存の目安として記載している。

## 適用版と本番での注意

- `Go 1.27.1 (time package API)` と pkg.go.dev の表示 `go1.27.1`（Published Sep 1, 2026）で確認する。将来の最新とは扱わない。
- 版で変わる項目: timer / ticker の GC 回収と channel の同期化、stale value の非受信保証は Go 1.23 以降。`Timer.Reset` は go1.1、`Until` は go1.8、`Duration.Milliseconds` / `Microseconds` は go1.13、`Ticker.Reset` は go1.15、`Duration.Abs` は go1.19 で導入。古い Go では該当 API が無い。
- monotonic はプロセス内の計測にしか使えない。永続化・ログ間の突合・プロセス跨ぎの deadline では wall clock で比較される前提に落とす。
- 一部システムのスリープ中は monotonic が止まる可能性があると公式は明記する。長時間の計測ではこの制約を確認する。
- 本文のバージョン分岐の踏まえ方と運用方針は設計判断であり、公式 API 契約そのものではない。公式契約と設計案の境界は上記の各節で区別する。

---
{
  "id": "linux-systemd-restart-jitter-start-limit-clock",
  "title": "systemd v262 の再起動分散: jitter 加算・StartLimit・suspend 時計の境界",
  "kind": "knowledge",
  "technology": "linux",
  "version": "systemd v262 (released 2026-09-22), source 8cc40e0c5e9234bf45084751ac53b1fbfe70b492; RestartSteps/RestartMaxDelaySec since v254; runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "linux",
    "systemd",
    "restart-jitter",
    "RestartRandomizedDelaySec",
    "RestartSteps",
    "StartLimitIntervalSec",
    "CLOCK_BOOTTIME",
    "CLOCK_MONOTONIC"
  ],
  "sources": [
    {
      "id": "systemd-v262-restart-release-20261003",
      "url": "https://github.com/systemd/systemd/releases/tag/v262",
      "type": "release_notes"
    },
    {
      "id": "systemd-v262-restart-service-manual-20261003",
      "url": "https://raw.githubusercontent.com/systemd/systemd/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/man/systemd.service.xml",
      "type": "official_docs"
    },
    {
      "id": "systemd-v262-start-limit-unit-manual-20261003",
      "url": "https://raw.githubusercontent.com/systemd/systemd/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/man/systemd.unit.xml",
      "type": "official_docs"
    },
    {
      "id": "systemd-v262-restart-timing-source-20261003",
      "url": "https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/core/service.c",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# systemd v262 の再起動分散を設計する

## 問いと今回の変更

多数の worker が同じ依存先の障害で終了したとき、固定の `RestartSec=` だけでは
同時再接続を繰り返しやすい。systemd v262 の `RestartRandomizedDelaySec=` を使えば
何が分散され、どこで再起動が止まり、どの値で待ち時間を観測できるか。

2026-09-22 公開の [v262 release notes](https://github.com/systemd/systemd/releases/tag/v262)
は、自動再起動へのランダムな追加待機と、start rate limit の時計変更を告知している。
本書の対象は Linux 上の systemd service manager。アプリの HTTP retry、初回 boot の起動分散、
Kubernetes の再起動制御をこの設定に置き換える話ではない。
既存の [systemd sandboxing](systemd-service-sandboxing.md) とも判断対象が異なる。

結論は、再起動する条件、次回までの待機、開始回数の上限を別々に決めること。
`RestartMaxDelaySec=` は jitter を含む総遅延の上限ではなく、StartLimit の期限切れも
新しい起動要求を自動生成する契機ではない。

## 公開契約: どの待機を追加するか

[固定版の service manual](https://raw.githubusercontent.com/systemd/systemd/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/man/systemd.service.xml)
で確認した設定は次の通り。これらは `[Service]` に置く。

| 設定 | 確認した役割と条件 |
|---|---|
| `Restart=` | 自動再起動の対象となる終了を選ぶ。`on-failure` でも手動 stop は再起動しない |
| `RestartSec=` | 再起動前の基本待機。文書上の既定は100ms |
| `RestartSteps=` | v254 で導入。既定0で指数 backoff 無効。正の段数と `RestartMaxDelaySec=`、非ゼロの `RestartSec=` が揃って有効 |
| `RestartMaxDelaySec=` | v254 で導入。backoff の基本待機が達する上限。既定 `infinity` はこの backoff 設定を無効にする |
| `RestartRandomizedDelaySec=` | v262 で導入。0から指定時間までの一様な追加待機。既定0。基本待機または backoff の結果へ加算する |

したがって jitter だけを設定しても `Restart=` の条件は変わらない。
停止させたサービスが勝手に復活する設定でもなく、アプリ内の再送一回ごとに働く設定でもない。
`RandomizedDelaySec=` は timer unit の別設定なので service のキーと取り違えない。

manual の式を使うと、基本待機は `RestartSec` と `RestartMaxDelaySec` の間を幾何的に増える。
以下は転載ではない独自の計算例で、実機の測定値ではない。
`RestartSec=2s`、`RestartSteps=3`、`RestartMaxDelaySec=16s`、
`RestartRandomizedDelaySec=4s` とした場合:

| 自動再起動の回 | jitter 加算前 | 設定上の待機範囲の目安 |
|---|---:|---:|
| 1回目 | 2秒 | 2〜6秒 |
| 2回目 | 4秒 | 4〜8秒 |
| 3回目 | 8秒 | 8〜12秒 |
| 4回目以降 | 16秒 | 16〜20秒 |

これは選ばれる restart 待機の範囲であり、障害発生からアプリの利用可能状態までの時間ではない。
停止処理、次の起動処理、依存関係の待機やスケジューリングの遅れは別に観測する。
上限に達した後も追加4秒は残る。固定16秒へ全 instance が再収束すると見積もらない。

## 固定実装の観察: 値の保持と観測の落とし穴

以下は v262 commit `8cc40e0c5e9234bf45084751ac53b1fbfe70b492` の静的読解であり、
将来版の内部構造を保証する公開API契約ではない。

- [service.c](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/core/service.c)
  の `service_restart_usec_next_jittered()` は、cap 適用後の基本待機に選択済み jitter を加える。
  `service_enter_dead()` で自動再起動ごとに値を一度選択し、
  `restart-randomized-delay-chosen-usec` として保存・復元する。
  coldplug でも同じ合算 helper を使うので、同じ設定の `daemon-reload` を
  jitter の再抽選手段と考えない。設定自体を変えたときまで期限が不変という主張ではない。
- 同じファイルの検証処理は `RestartRandomizedDelaySec=infinity` を警告して0に戻す。
  無期限の猶予を与える意味にはならない。有限値を使う。
- [dbus-service.c](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/core/dbus-service.c)
  の `RestartUSecNext` は jitter 非加算の `service_restart_usec_next()` に結び付く。
  `RestartRandomizedDelayUSec` も設定された幅であり、その回に選択された値ではない。
  この2プロパティを「実際の次回期限」と表示する監視は避ける。

運用上は、基本待機、jitter の設定幅、実際の再起動時刻を別々に記録するのがよい。
上の16秒という基本待機と18秒の観測値が違っていても、それだけで設定不良とは判断できない。
これは観測設計の提案で、ログ形式の長期互換性や実時間の精度を保証するものではない。

## StartLimit: 遅延とは別の開始制限

[固定版の unit manual](https://raw.githubusercontent.com/systemd/systemd/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/man/systemd.unit.xml)
では `StartLimitIntervalSec=` と `StartLimitBurst=` は `[Unit]` の設定である。
自動再起動だけでなく手動開始にも適用され、condition が不成立だった activation は数えない。
上限到達後は自動再起動を試し続けない。interval が過ぎた後の手動操作、timer、socket など
別の開始要求によって起動が許可されれば再び restart logic が有効になる。

このため「しばらく待てば自動で必ず復旧する」という監視条件は誤りになる。
`systemctl reset-failed` は rate counter を解除できるが、障害原因を直した証明ではない。
unit の GC による unload でも counter が消えるので、長期の累計試行予算にも向かない。

以下は独自の検討用 drop-in 例で、本書では適用・実行していない。
継続リトライではなく、短時間の繰り返し失敗を止めて調査する方針を表す。

```ini
[Unit]
StartLimitIntervalSec=2min
StartLimitBurst=5

[Service]
Restart=on-failure
RestartSec=2s
RestartSteps=3
RestartMaxDelaySec=16s
RestartRandomizedDelaySec=4s
```

counter が空で、プロセスの失敗と停止処理が即時、他の起動要求もない単純化では、初回を含め5回の開始後、
次の開始はまだ2分の窓内となって拒否される。基本待機だけなら次の試行は累計46秒、
各 jitter を最大4秒で見積もっても累計66秒だからである。
この計算は実測ではなく、実処理時間を無視した例である。実際には起動・停止が遅ければ
窓をまたぐため、必ず同じ回数で停止するとは限らない。

jitter は同期を崩す手段で、fleet 全体の同時実行上限や共有 backend の retry budget ではない。
ランダムな待機が0に近くなる場合もあるため、StartLimit 回避を jitter の平均値へ依存させない。
復旧を自動継続させたいのか、失敗を停止状態として人へ渡したいのかを先に決め、
上限到達時の通知と再開担当をセットで設計する。

## v262 の suspend 時計変更を分けて扱う

release notes は rate-limit の基準が `CLOCK_MONOTONIC` から `CLOCK_BOOTTIME` に変わり、
suspend 中の時間が期限に算入されると述べる。また upgrade/reexecution 時、
旧 manager が保存した timestamp と新しい時計の基準が異なるため、既存の制限が一度だけ
以前より早く切れる可能性を注意事項として挙げている。

固定コードでも [ratelimit.c](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/basic/ratelimit.c)
の `ratelimit_below()` は `CLOCK_BOOTTIME`、
[unit.c](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/core/unit.c)
の `unit_arm_timer()` は `CLOCK_MONOTONIC` を使う。
service の restart 待機は後者へ接続される。
従って v262 で全ての再起動関連タイマーが suspend を数えるようになった、と一括りにしない。

ここからの設計上の推論として、suspend/resume を挟む端末では、開始回数の制限窓が
期限を迎えていても、既に予約された restart 待機には別の残り時間がありうる。
上限到達で予約自体がなくなった unit と、まだ `auto-restart` 中の unit も区別する。
この resume 動作、旧版からの reexecution、配布版の backport は実機では未検証。

## 導入・rollback の確認手順（独自提案、未実施）

1. 対象 host の manager 版と配布元 patch を記録する。v254 で backoff が使えても v262 の
   jitter があるとは限らない。未知 option は警告しつつ unit を読み込むという文書上の扱いがあるので、
   unit が起動した事実だけを採用確認にしない。旧版への rollback では分散が失われる可能性を評価する。
2. 実効 unit と drop-in、`Restart=` の対象 exit、基本待機、cap、jitter、StartLimit を確認する。
   予算表にはアプリ内再送や外部 supervisor の再起動も含め、二重の再試行を見落とさない。
3. 隔離した検証環境で一斉失敗、即時失敗、遅い停止、正常終了、手動 stop を分けて試す。
   単発の待機値だけでなく複数 instance の開始時刻分布と backend の接続数を測る。
4. 上限到達後、窓の期限切れだけでは再開しないことと、許可された外部 trigger で再開できることを確認する。
   調査中に毎回 reset-failed する運用は limiter を繰り返し解除するので避ける。
5. 同一設定で待機中に daemon-reload した場合の期限、suspend/resume、旧版からの manager reexecution を
   別々のケースにする。restart の wall-clock 間隔を StartLimit の時計と同じものとして比較しない。

設定例はサービス本来の `Type=`、実行内容、権限、停止方法を決めるものではない。
再起動後の readiness、処理の二重実行、外部資源の整合性はアプリ側で別途検証する。

## 出典・ライセンス・未確認事項

- 取得日は2026-10-03 UTC。release 公開日時は GitHub release API の
  `published_at=2026-09-22T13:19:16Z` とページを照合した。最新版一般ではなく v262 に限定する。
- service/unit XML と固定実装の SPDX は LGPL-2.1-or-later。
  同じ commit の `LICENSE.LGPL2.1` と `LICENSES/README.md` も確認した。
  `ratelimit.c` は Linux の GPLv2 ratelimit を参考にした旨も記す。ソースコードの転載はなく、
  設定例・計算・導入判断は本書独自のもの。release ページ単体のライセンスは未確認として記録した。
- freedesktop の HTML manual は403等で取得できなかった。upstream の XML を native web で開き、
  full commit 版の unit manual とコードは GitHub connector でも確認した。読めなかったHTMLを根拠にしていない。
- systemd v262 のインストール、unit 起動、乱数分布、実時間、suspend、upstream test の実行は行っていない。
  検索 eval は本書を見つけるための確認であり、daemon 動作の再現試験ではない。

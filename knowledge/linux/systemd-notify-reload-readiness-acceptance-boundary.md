---
{
  "id": "linux-systemd-notify-reload-readiness-acceptance-boundary",
  "title": "systemd v262 notify-reload: READY受理・signal handler・reload完了の境界",
  "kind": "knowledge",
  "technology": "linux",
  "version": "systemd v262 (2026-09-22), source 8cc40e0c5e9234bf45084751ac53b1fbfe70b492; Linux service manager; runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "linux",
    "systemd",
    "notify-reload",
    "READY=1",
    "ReloadSignal",
    "MONOTONIC_USEC",
    "sd_notify_barrier",
    "NotifyAccess"
  ],
  "sources": [
    {"id": "systemd-v262-notify-reload-release-20261023", "url": "https://github.com/systemd/systemd/releases/tag/v262", "type": "release_notes"},
    {"id": "systemd-v262-notify-reload-service-manual-20261023", "url": "https://www.freedesktop.org/software/systemd/man/262/systemd.service.html", "type": "official_docs"},
    {"id": "systemd-v262-notify-protocol-manual-20261023", "url": "https://www.freedesktop.org/software/systemd/man/262/sd_notify.html", "type": "official_docs"},
    {"id": "systemd-v262-notify-cli-manual-20261023", "url": "https://www.freedesktop.org/software/systemd/man/262/systemd-notify.html", "type": "official_docs"},
    {"id": "systemd-v262-notify-reload-state-source-20261023", "url": "https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/core/service.c", "type": "github_repository_analysis"},
    {"id": "systemd-v262-notify-reload-tests-20261023", "url": "https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/test/units/TEST-80-NOTIFYACCESS.sh", "type": "github_repository_analysis"}
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/infrastructure.json"]
}
---

# systemd v262 notify-reload: READY受理・signal handler・reload完了の境界

## 解く問いと今回の変更

`Type=notify-reload` の daemon が `READY=1` を送れたのに、systemd 更新後に
起動失敗するのはなぜか。起動に成功すれば、その後の reload も安全なのか。
`sd_notify()` の戻り値や barrier だけで完了を判定してよいか。

[v262 release notes](https://github.com/systemd/systemd/releases/tag/v262) は、
2026-09-22 の変更として、初回 `READY=1` 時点で `ReloadSignal=` を catch または
block していない `notify-reload` service を protocol error にすることを告知した。
以前は起動できても、後日の reload signal の既定動作で終了する可能性があった。
これは再起動回数や sandbox 権限の変更ではなく、**起動完了通知を受理する条件の変更**である。

結論は、次の三段階を分けて確認すること。

1. 通知を送信し、manager がその通知を処理したか。
2. 適切な送信元と service 状態のもとで、要求した状態遷移が受理されたか。
3. アプリが本当に初期化・設定適用を終え、期待する動作を提供しているか。

以下は upstream v262 に固定した契約と静的分析であり、ディストリビューション固有の
backport、実機での systemd 起動、実サービスの reload は検証していない。
既存の [restart jitter](systemd-restart-jitter-start-limit-clock.md) と
[sandboxing](systemd-service-sandboxing.md) は別の判断を扱う。

## READYの前に、main processのreload受信を成立させる

[systemd.service v262](https://www.freedesktop.org/software/systemd/man/262/systemd.service.html#Type=)
によると、`notify` は初期化完了通知を待ち、`notify-reload` はこれに signal ベースの
reload 完了待ちを加える。`ReloadSignal=` の既定は `SIGHUP`。
`Type=exec` は execve 成功を待つが、アプリ固有の初期化完了を保証する型ではない。

v262 は **main process** の reload signal 受信状態を初回 `READY=1` 時に検査する。
補助プロセスだけに handler を置いても、この検査の対象を満たしたことにはならない。
通常の handler と、signalfd などで処理するための blocked signal が対象となる。
handler の設置や必要な block を先に完了し、その後で初回 READY を送る順序にする。

固定版の
[service_check_reload_signal_handler() と READY処理](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/core/service.c)
および
[process-util.c](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/basic/process-util.c)
から確認できる実装境界は次のとおり。

- `SigCgt` を調べ、該当しなければ `SigBlk` を調べる。`SIG_IGN` だけを設定し、
  catch も block もしていない状態を合格条件にはしていない。
- 両方が存在しないと `-EOPNOTSUPP` となり、初回 READY 処理は
  `SERVICE_FAILURE_PROTOCOL` で停止処理に進む。
- block は受信方式の必要条件を示すにすぎず、signalfd を読む処理や設定再読込が
  正しく動くことを検査してはいない。block だけで消費処理を実装しなければ、
  起動検査を通っても reload 完了通知が来ない設計になり得る。
- 固定版は main PID 未設定、および catch/block 不可能な `SIGKILL`・`SIGSTOP` を
  この検査から除外する。また検査エラー一般をすべて protocol error にするわけではない。
  したがって、これは全状態での安全性証明でも、reload 実装の機能試験でもない。

これらの例外を利用して検査を回避するのではなく、通常の reload signal を
継続して処理できる main process を準備するのが設計上の要件である。

## 起動時の拒否と、稼働後の警告は同じではない

同じ [v262 manual](https://www.freedesktop.org/software/systemd/man/262/systemd.service.html#Type=)
は、reload signal を送る直前にも検査するが、handler が失われていれば
**警告してなお signal を送る**と定めている。起動後に handler を解除した service を
manager が自動的に保護し続ける、という意味ではない。

固定版の
[TEST-80-NOTIFYACCESS.sh](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/test/units/TEST-80-NOTIFYACCESS.sh)
は、次の期待値を持つ。ここで述べるのは **上流testの内容を読んだ結果で、実行結果ではない**。

| 上流の条件 | testが確認している結果 |
|---|---|
| handlerなしで初回READYを送る | start失敗、SubState=failed、Result=protocol、handler欠落ログ |
| ReloadSignal=SIGSTOP | handler検査の例外として起動する。安全なreloadを証明するtestではない |
| 起動後にHUP handlerを解除 | reload要求後にserviceが非activeになる。SIGHUP終了と警告を想定 |
| handlerを維持してreload通知を実装 | reload-signalを経てrunningへ戻り、ReloadResult=success |

test 内の `systemctl reload --no-block` は要求の投入であり、成功完了の観測ではない。
その後の状態待ちと `ReloadResult` 確認が重要である。
固定版の
[harness](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/test/integration-tests/TEST-80-NOTIFYACCESS/TEST-80-NOTIFYACCESS.units/notify-reload-harness.sh)
も handler の登録を初回 READY より先に行っている。

## reloadサイクルには開始通知と現在の単調時計が必要

[公開protocol](https://www.freedesktop.org/software/systemd/man/262/systemd.service.html#Type=)
に沿う通常の流れは、manager が main process に reload signal を送り、アプリが
`RELOADING=1` と `MONOTONIC_USEC=` を同じ通知に入れ、適用完了後に `READY=1` を送る形である。
時刻は wall clock、Unix epoch、秒ではなく、**CLOCK_MONOTONIC のμs値を10進文字列**にする。

固定版の
[service_enter_reload() / service_notify_message_process_state()](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/core/service.c)
では、manager の reload 開始時刻を記録し、`reload-signal` 状態では
通知の `MONOTONIC_USEC` がその時刻以上である場合に `RELOADING=1` を今回の開始として扱う。
古い時刻や欠落した時刻の開始通知は、この遷移条件を満たさない。
これは以前から進行していた reload と今回の要求を取り違えないための境界である。

- `READY=1` だけを `reload-signal` に送っても、その経路では reload 完了へ進まない。
  開始通知を省略する「高速reload」は正しい最適化ではない。
- 正しい開始通知を受けた `reload-notify` では、後続の READY が完了を知らせる。
  通常の二通知経路で、完了側にも同じ timestamp が必須だとは解釈しない。
- 固定版は `RELOADING=1` と `READY=1` を一つにした即時完了通知も扱うが、
  `reload-signal` での時刻条件は残る。通常は開始と完了を分けた方が計測・障害解析しやすい。

[systemd-notify --reloading](https://www.freedesktop.org/software/systemd/man/262/systemd-notify.html#--reloading)
は `MONOTONIC_USEC` も設定する。一方、単に文字列 `RELOADING=1` を自前で送る実装は
timestamp も自分で用意する必要がある。
`ExecReload=` を併用する場合は、コマンド終了前に開始通知を受けたときに追加 signal を
省く経路があるため、signal を二重に送る前提でアプリを作らない。

## 送信成功・barrier・NotifyAccessを分離する

[sd_notify v262](https://www.freedesktop.org/software/systemd/man/262/sd_notify.html) の
戻り値は負数がエラー、`NOTIFY_SOCKET` 未設定なら0、送信できれば正数である。
正数は通知の enqueue を示し、manager が要求を意味的に受理した証拠ではない。
例えば READY が送れても、前述の handler 欠落で起動は失敗し得る。

`sd_notify_barrier()` は、それ以前の通知を manager が処理したことを待つ。
これは **受信同期であって意味的受理のACKではない**。
固定版の
[manager.c](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/src/core/manager.c)
は `BARRIER=1` を通常の unit notification dispatch より先に扱う。
barrier に個々の READY の成否やアプリの構成世代を返信する仕組みはない。

`NotifyAccess=main` はmain、`exec` はmanagerが起動したmain/control process、
`all` はservice cgroup内の送信元へ範囲を広げる。
`notify` / `notify-reload` で未指定なら暗黙の `main` となる。
ただし **NotifyAccess=all と送信元の即時終了の競合は別問題**である。
manager が直接追跡しない補助プロセスが送信直後に終了すると、unitへ帰属させられず
通知が無視される場合がある。必要な送信権限を満たしたうえで、補助送信元を
barrier 完了まで生存させることで、この帰属競合を避ける。

[systemd-notify](https://www.freedesktop.org/software/systemd/man/262/systemd-notify.html)
は通常この同期を行う。`--no-block` は同期を無効にし、該当する補助プロセスの競合を再導入する。
また親PIDとしての送信を試み、権限不足なら自身のPIDへfallbackするため、rootで成功した
shell wrapperが非特権でも同じ `NotifyAccess=main` で動くと推定しない。
権限を追加する前に、main process自身から通知できる構造かを見直す。

API利用時は `unset_environment` にも注意する。
初回 `sd_notify()` に非ゼロを渡すと、送信成否にかかわらず `NOTIFY_SOCKET` がunsetされる。
同じプロセスが後でreload通知や `sd_notify_barrier()` を行う設計で、初回だけで
環境を消してしまうと、その後の通知はできない。helperへの意図しない継承防止と、
mainの継続通知に必要な寿命を別々に設計する。

## timeoutをアプリのrollbackと取り違えない

[TimeoutStartSec](https://www.freedesktop.org/software/systemd/man/262/systemd.service.html#TimeoutStartSec=)
は reload にも適用される。manual は、期限内に完了しないreloadは失敗となり、
serviceは旧設定で稼働を続け、`systemctl reload` が失敗すると説明している。
ここから「systemdがアプリ内部の途中適用を原子的に巻き戻す」とは推論できない。

固定版の timeout 経路は、存在すれば control process を止め、reload失敗を記録し、
main processの状態を評価して稼働状態へ戻す処理である。設定内容の復元処理ではない。
したがって **ReloadResult=timeout とアプリのrollback完了は別の観測項目**とするのが
本書の設計提案である。候補設定を検証してから切り替え、失敗時に旧設定を保持する責務は
アプリ側で試験する。active表示だけを設定適用成功の判定に使わない。

## 移行時の受入れ確認（独自の設計案、未実施）

1. 実機のmanager版・ディストリビューション差分、unitのType、ReloadSignal、
   NotifyAccess、main PIDを記録する。v262という版名だけでbackport有無を判断しない。
2. 初回READY以前にmainのhandler/blockと実際の受信処理が成立することを確認する。
   handlerなし、SIG_IGNのみ、helperにだけhandlerがある負例を用意する。
3. 初期化完了前は後続unitが進まず、完了後はstart jobが成功することを確認する。
   `ExecStartPost=` があればその終了も順序制約に含まれるため、READYだけを全起動完了としない。
4. 正常reload、開始通知欠落、古いtimestamp、READY欠落を試す。
   reload jobの結果・SubState・ReloadResultと、アプリの設定世代を一緒に保存する。
5. 稼働中のhandler解除、非特権helper、helper即時終了、`--no-block` を負例として確認する。
   barrierが返ったという理由だけで成功扱いしない。
6. reloadの期限超過後に、旧設定維持・部分適用の有無をアプリ側で検証する。
   失敗を握りつぶしてREADYを送る実装も成功と判定しない。
7. 未対応daemonの応急処置として、単にTypeを変更して待機を消さない。
   reload非対応なら `Type=notify`、別の同期reload APIがあれば完了待ちをする
   `ExecReload=` など、失う契約を確認したうえで選択する。

## 出典・ライセンス・未確認事項

- 取得日: 2026-10-23 UTC。公開変更日はv262 releaseの2026-09-22を確認した。
  versioned manualはv262表示を確認し、ページ固有の公開日は確認していない。
- repository分析はすべて
  [8cc40e0c5e9234bf45084751ac53b1fbfe70b492](https://github.com/systemd/systemd/commit/8cc40e0c5e9234bf45084751ac53b1fbfe70b492)
  に固定した。別のmain commitをv262の実装根拠にはしていない。
- 各manualの元XML、service.c・manager.c・process-util.c、test/harnessのSPDXと
  [LICENSES/README.md](https://github.com/systemd/systemd/blob/8cc40e0c5e9234bf45084751ac53b1fbfe70b492/LICENSES/README.md)、
  LICENSE.LGPL2.1を照合した。対象本文・実装・testはLGPL-2.1-or-later。
  manual内のexampleはMIT-0として別扱いされるが、本書に上流コードを転記していない。
  GitHub releaseページ固有のライセンスはunknownとして、告知の独自要約に限定した。
- raw/github web取得で一部XML・licenseにCache missがあったため、同じ固定commitを
  GitHub connectorで取得した。HTMLの公式manual、release、service.c・manager.cは
  native web toolで本文を開いて確認した。
- 実daemon・コンテナ内PID1・複数threadのsignal設計・distribution patchの互換性は未検証。
  上流testは静的確認のみ。検索evalは文書への到達性を検査し、systemd実動作の試験ではない。

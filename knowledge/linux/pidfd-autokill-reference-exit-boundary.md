---
{
  "id": "linux-pidfd-autokill-reference-exit-boundary",
  "title": "Linux pidfd autokill: 最終参照の解放・終了通知・auto-reap の境界",
  "kind": "knowledge",
  "technology": "linux",
  "version": "CLONE_AUTOREAP / CLONE_NNP / CLONE_PIDFD_AUTOKILL introduced in upstream Linux v7.1 (2026-06-14); implementation and selftests verified at v7.2 8d3ae59288f1e7d58d76558a6ee96d533bc5019f (2026-08-16); man-pages 6.19 for older pidfd contracts; retrieved 2026-10-04 UTC; runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "linux",
    "clone3",
    "pidfd",
    "CLONE_PIDFD_AUTOKILL",
    "CLONE_AUTOREAP",
    "CLONE_NNP",
    "PIDFD_GET_INFO",
    "PIDFD_INFO_EXIT",
    "SIGKILL",
    "PID-reuse",
    "process-lifecycle"
  ],
  "sources": [
    {
      "id": "linux-pidfd-autokill-introduction-c8134b5f-20261004",
      "url": "https://github.com/torvalds/linux/commit/c8134b5f13ae959de2b3c8cc278e2602b0857345",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-pidfd-autokill-v71-inclusion-20261004",
      "url": "https://github.com/torvalds/linux/blob/8cd9520d35a6c38db6567e97dd93b1f11f185dc6/include/uapi/linux/sched.h",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-pidfd-autokill-v72-fork-20261004",
      "url": "https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/kernel/fork.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-pidfd-autokill-v72-pidfs-20261004",
      "url": "https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/fs/pidfs.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-pidfd-autokill-v72-selftests-20261004",
      "url": "https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/tools/testing/selftests/pidfd/pidfd_autoreap_test.c",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-pidfd-open-man619-20261004",
      "url": "https://man7.org/linux/man-pages/man2/pidfd_open.2.html",
      "type": "official_docs"
    },
    {
      "id": "linux-clone-pidfd-man619-20261004",
      "url": "https://man7.org/linux/man-pages/man2/clone.2.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2027-01-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Linux pidfd autokill: 最終参照の解放と子プロセスの終了を分ける

## 解く問いと結論

ジョブ実行器が起動した helper を、管理側の異常終了後も残さないようにしたい。
Linux 7.1 の `CLONE_PIDFD_AUTOKILL` を付ければ「親が終了した」「pidfd を一つ close した」
「子の終了を確認した」を同じイベントとして扱えるか。

扱えない。起点になるのは、`clone3()` が作った特定の `struct file` への最終参照の解放である。
そこから行われるのは子プロセスへの `SIGKILL` 送信であり、完了結果の受領ではない。
`dup()` で監視用 fd を残すとその参照も寿命を延ばす。一方、同じ子へ独立に
`pidfd_open()` した fd は autokill の参照数に参加しない。
[導入 commit](https://github.com/torvalds/linux/commit/c8134b5f13ae959de2b3c8cc278e2602b0857345)

本書は、管理プロセスが自分で生成する通常の、ptrace されていない子プロセスのライフサイクルを扱う。
既存の [systemd sandboxing](systemd-service-sandboxing.md) や
[再起動の待機・回数制限](systemd-restart-jitter-start-limit-clock.md) とは別の問いである。
以下で upstream の事実、固定実装の観察、独自の設計案を区別する。

## 導入版と調査版

- `CLONE_PIDFD_AUTOKILL` の導入 commit は `c8134b5f13ae959de2b3c8cc278e2602b0857345`
  （2026-03-11 UTC）。[upstream merge](https://github.com/torvalds/linux/commit/07c3ef58223e2c75ea209d8c416b976ec30d9413)
  は2026-04-13で、対象は `vfs-7.1-rc1.pidfs` である。パッチ投稿だけを採用済みの証拠とはしていない
- [v7.1 release commit](https://github.com/torvalds/linux/commit/8cd9520d35a6c38db6567e97dd93b1f11f185dc6)
  は2026-06-14。固定した UAPI header と `kernel/fork.c` に新フラグと実装があり、
  v7.0 `028ef9c96e96197026887c0f092424679298aae8` の header にはないことも確認した
- 本文の実装・selftest は [v7.2](https://github.com/torvalds/linux/commit/8d3ae59288f1e7d58d76558a6ee96d533bc5019f)
  （2026-08-16）の `8d3ae59288f1e7d58d76558a6ee96d533bc5019f` に固定した。
  安定版の最新性、vendor backport、コンテナ runtime の対応を保証するものではない
- 2026-10-04 UTC に開いた man-pages 6.19 の `clone(2)` には新3フラグの記載がない。
  同ページは従来の `CLONE_PIDFD` の参照に限り、新機能の根拠は固定した kernel と selftest に置く

## 生成時の契約と失敗の分類

[v7.2 の copy_process / clone3_args_valid](https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/kernel/fork.c)
と [UAPI header](https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/include/uapi/linux/sched.h)
で、次を確認した。

| 指定 | 確認した動作・制約 |
|---|---|
| `CLONE_AUTOREAP` | 子ごとの自動回収。`exit_signal` は0を要求し、`CLONE_PARENT` と `CLONE_THREAD` の併用を `EINVAL` にする |
| `CLONE_PIDFD_AUTOKILL` | `CLONE_PIDFD` と `CLONE_AUTOREAP` が両方必要。不足すると `EINVAL` |
| `CLONE_NNP` | 子へ生成時点で `no_new_privs` を設定。親自身へ新たに設定する操作ではない |
| AUTOKILL と NNP の併用 | AUTOKILL 固有の `CAP_SYS_ADMIN` 要件を不要にする。他の clone 条件や実行環境の制約は残る |
| AUTOKILL に NNP を付けない | 呼出元の user namespace で `CAP_SYS_ADMIN` がなければ `EPERM` |

通常の非特権 helper をこの方式で作る候補は、`CLONE_PIDFD`、`CLONE_AUTOREAP`、
`CLONE_PIDFD_AUTOKILL`、`CLONE_NNP` と `exit_signal=0` の組合せになる。
これは設計上の選択肢であり、あらゆる既存プログラムをこのフラグ列へ置換する提案ではない。
子が setuid 等による exec 時の権限獲得を必要とするなら、NNP の制約を先に評価する。
動作させるためだけに `CAP_SYS_ADMIN` を足すのは、この提案の手順に含めない。

`EINVAL` を新機能未対応と即断しない。対応版でもフラグの組合せや引数が誤れば返る。
`EPERM` も unsupported と同じ意味ではない。対象 kernel、UAPI header、clone の引数、
必要な capability、適用される seccomp 等の許可を別々に確認する。
失敗後に必須フラグだけを黙って落とす fallback は、親が失われたときの契約を変えてしまう。

## 同じ PID を指しても、同じ寿命を所有するとは限らない

[導入 commit の説明](https://github.com/torvalds/linux/commit/c8134b5f13ae959de2b3c8cc278e2602b0857345)
は、pidfd が指すプロセスと、autokill を担う `struct file` を区別している。

| 操作・状態 | autokill との関係 |
|---|---|
| 生成時の pidfd の参照を一つ close | 他の参照があれば最終解放にならない |
| 生成時の pidfd を `dup()`、または後の `fork()` で継承 | 同じ `struct file` を共有するため、その参照が残る間は最終解放を遅らせる |
| 同じ子へ `pidfd_open()` | 別の `struct file`。その fd の close は autokill を発火せず、その fd を残しても生成時 pidfd の最終解放を妨げない |
| 生成時の `struct file` の最終参照を解放 | 対象が存在すれば kernel がそのプロセスへ `SIGKILL` を送る |

したがって「管理側の親が crash したら必ず直ちに止まる」とは一般化できない。
別プロセスへ渡した複製や後から fork した監視役が参照を保持していないかまで設計する。
特に、終了監視のための `dup(creation_pidfd)` を残し、元の fd を close してから
その複製で終了を待つ構成は、自分で autokill の条件を満たせなくする。
これは参照共有の契約からの推論である。

逆向きの事故もある。`CLONE_PIDFD` で返る fd は close-on-exec である。
管理側が exec し、その結果が最終解放になれば、意図せず autokill の起点になり得る。
[clone(2)](https://man7.org/linux/man-pages/man2/clone.2.html) と
[v7.2 pidfd_prepare](https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/kernel/fork.c)
を合わせた設計上の注意であり、close-on-exec を一律に解除すべきという推奨ではない。

## auto-reap 後の終了観測と PID 再利用

### SIGCHLD と waitpid の前提を変える

`CLONE_AUTOREAP` は親の全子へ作用する `SIGCHLD=SIG_IGN` / `SA_NOCLDWAIT` と異なり、
作成した子の属性になる。reparenting 後も残るが、その子が通常の `fork()` で作る孫へ
自動継承される設定ではない。
[upstream merge 説明](https://github.com/torvalds/linux/commit/07c3ef58223e2c75ea209d8c416b976ec30d9413)

[v7.2 selftests](https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/tools/testing/selftests/pidfd/pidfd_autoreap_test.c)
の `autoreap_basic` は、poll で終了を観測して `PIDFD_GET_INFO` から終了情報を取り、
`waitpid` が `ECHILD` になることを検査する。
従来の「SIGCHLD を受けて waitpid から結果を収集する」実装を、そのまま残さない。
`ECHILD` は wait 可能な子がないことを示し、単独では子の終了証明にもジョブ成功の終了コードにもならない。
固定版の `kernel/signal.c` は `!tsk->ptrace` の条件付きで `signal->autoreap` を適用するため、
debugger が付いた実行へ、この通常の回収・SIGCHLD の説明をそのまま適用しない。

### 成功した ioctl と有効な終了情報を分ける

[v7.2 pidfd_info](https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/fs/pidfs.c)
では、入力の `mask` で `PIDFD_INFO_EXIT` を要求し、保存済みの終了情報がある場合に
出力の `mask` へ同じ bit を設定する。呼出し成功だけでゼロ初期化された `exit_code` を
正常終了と解釈しない。返却 mask に bit があることを確かめ、wait status 形式を
`WIFEXITED` / `WEXITSTATUS` または `WIFSIGNALED` / `WTERMSIG` で解釈する。

同じ実装は、回収済みで task が存在しなくても、終了情報の要求があれば保存情報を返す経路を持つ。
逆に、異なる PID namespace 系統からの照会には `EREMOTE` があり、常に情報取得できるわけではない。
情報未取得・観測失敗・正常終了は別状態として残す。

pidfd の `POLLIN` は、fd から終了コードの byte 列を `read()` できるという意味ではない。
[pidfd_open(2)](https://man7.org/linux/man-pages/man2/pidfd_open.2.html) は `read` の `EINVAL` を記載し、
固定実装では回収済みなら `EPOLLIN` と `EPOLLHUP` が併存する。
HUP を理由に即 fd を捨て、終了情報を失う実装にしない。

### 独立した観測 fd を後から開く競合

上流の `autokill_basic` は、生成時 pidfd を close する前に `pidfd_open()` で独立した
観測 fd を作り、close 後にその fd を poll する。
ただし、そのテストの子は終了せず待機している。任意の helper でもこの順序だけで
PID 再利用の競合をなくせる、と拡張してはならない。

`pidfd_open(2)` が fork 後の取得で元の子を保証する説明には、zombie が自動回収されず、
他の thread や handler にも回収されていないという条件がある。
auto-reap の子が先に終了する設計では、数値 PID を使った後続の `pidfd_open()` が
同じプロセスを指す前提を置けない。生成時 pidfd があっても、子の自然終了を止めるものではない。
これは manual の取得条件と auto-reap の組合せからの推論である。

## 解放は強制終了の要求であり、完了確認ではない

[v7.2 pidfs_file_release](https://github.com/torvalds/linux/blob/8d3ae59288f1e7d58d76558a6ee96d533bc5019f/fs/pidfs.c)
は `PIDTYPE_TGID` の対象へ `SIGKILL` を送り、子の終了を wait する処理を持たない。
そのため close の完了をもって、終了処理や I/O、アプリ側の仕事が完了したと記録しない。
対象はそのプロセスであり、通常の孫を含むプロセス木の一括終了機能でもない。

この契約は graceful shutdown、rollback、外部副作用の取り消しを追加しない。
強制終了でも整合性が保てる一時的な helper と、flush・応答・commit が必要な仕事を分ける。
プロセス木の管理やコンテナ全体の停止は、利用する管理機構の契約を別に検証する。
本書から systemd や runtime の stop 動作への置換を推奨することはしない。

## 採用・停止・検証の手順（独自の設計案）

1. **必要な失敗動作を決める。** 管理側を失ったら強制終了してよい helper か、
   引き継いで継続すべき仕事かを先に選ぶ。終了しても外部副作用を回収できるかは別の要件にする
2. **参照の所有者を明記する。** 生成時 pidfd の保有元、dup、fork、exec、受渡しの経路を棚卸しする。
   単一 fd 番号のログだけでは、最終参照が解放された証拠にならない
3. **通常停止は観測経路を保持する。** 管理側が生きている間は、同じ生成時 pidfd を保持したまま
   `pidfd_send_signal()` 等の明示的な停止要求と poll / `PIDFD_GET_INFO` を組み合わせる方が、
   数値 PID から監視 fd を再取得する必要を減らせる。autokill は管理側を失う経路の備えとして位置付ける。
   送信の許可・エラーと終了確認はそれぞれ扱う
4. **close による終了自体を試験するなら、観測 fd の同一性を保証する。** 上流テストのように
   子の生存を制御した試験で独立 fd を先に確保する。生成時 fd の dup を代用品にしない。
   先に自然終了していた場合の処理も設計し、数値 PID の再取得だけで同一性を認定しない
5. **結果の状態を分ける。** 起動失敗、実行中、停止要求済み、終了観測済み、終了結果取得済みを分ける。
   timeout は成功扱いせず、観測不能か参照残存かを調べる。期限や再試行間隔は workload に合わせ、
   応答しない fd を無制限の busy loop で poll し続けない
6. **unsupported と不正引数を分けて受入試験を行う。** 実際の kernel と sandbox で
   正常終了、signal 終了、親 crash、dup 残存、fork 継承、管理側 exec、自然終了との競合を確認する。
   フラグを落とす downgrade が必要なら、失う保証を明示した別モードとして承認・監視する

上流 selftest は、基本 autokill、独立 pidfd の close が無効なこと、必要フラグ、capability、
通常の終了コードと signal 終了、auto-reap 非継承を検査する。
一方、この固定ファイルには生成時 pidfd の dup を残す専用テストがなく、
待機用の固定 sleep もある。上流コードの存在を、本番 workload の時間上限や
本調査での実行成功の証拠とはしない。

## 出典・ライセンス・未確認事項

- 取得日はすべて2026-10-04 UTC。新機能の native web 確認は導入・merge commit、
  v7.1/v7.2 release commit、固定 v7.2 の raw `fork.c` / `pidfs.c`、selftest 導入 commit で実施した
- 固定 GitHub blob の一部と selftest raw view は取得失敗したため、GitHub connector で同じ40桁 SHA の
  ファイル全文を取得した。検索結果の抜粋を実装の根拠にはしていない
- kernel 実装・selftest はファイル SPDX の `GPL-2.0` / `GPL-2.0-only`、UAPI header は
  `GPL-2.0 WITH Linux-syscall-note` を確認した。man7 HTML のページ別ライセンスは未確定で `unknown`。
  本文は独自要約で、ソースコードやサンプルを転載・module 化していない
- 未実施: kernel build、selftest 実行、実機での crash/dup/exec 試験、vendor backport 調査、
  libc/runtime の API 対応、ptrace 下の全終了経路、PID namespace の init や `CLONE_FILES` 共有時の設計検証。
  traced process や特殊な生成条件へ通常の helper の説明をそのまま一般化しない
- 本文の提案は実測済みの実装レシピではない。再確認期限2027-01-02に、適用 kernel、
  関連修正、man-pages の新フラグ記載、利用 runtime の対応を再確認する

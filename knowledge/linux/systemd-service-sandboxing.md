---
{
  "id": "linux-systemd-service-sandboxing",
  "title": "Linux systemd サービスの sandboxing 指示の範囲と保証: ProtectSystem PrivateTmp NoNewPrivileges CapabilityBoundingSet",
  "kind": "knowledge",
  "technology": "linux",
  "version": "systemd v257 systemd.exec.xml; kernel docs 7.3.0-rc4 no_new_privs; Linux man-pages capabilities(7) (all retrieved 2026-09-27)",
  "tags": [
    "research-domain:infrastructure",
    "linux",
    "systemd",
    "systemd.exec",
    "ProtectSystem",
    "PrivateTmp",
    "NoNewPrivileges",
    "CapabilityBoundingSet",
    "ProtectProc",
    "DynamicUser",
    "MountAPIVFS",
    "RestrictSUIDSGID",
    "ProtectHome",
    "bounding",
    "ambient",
    "no_new_privs",
    "prctl",
    "execve",
    "hidepid",
    "sandboxing"
  ],
  "sources": [
    {
      "id": "systemd-exec-v257",
      "url": "https://raw.githubusercontent.com/systemd/systemd/v257/man/systemd.exec.xml",
      "type": "official_docs"
    },
    {
      "id": "kernel-no-new-privs-docs",
      "url": "https://docs.kernel.org/userspace-api/no_new_privs.html",
      "type": "official_docs"
    },
    {
      "id": "linux-capabilities-man7",
      "url": "https://man7.org/linux/man-pages/man7/capabilities.7.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active"
}
---

# Linux systemd サービスの sandboxing 指示の範囲と保証: ProtectSystem PrivateTmp NoNewPrivileges CapabilityBoundingSet

systemd のサービス sandboxing 指示がどこまでを保証し、どこからをカーネルの機構に委ねるのかを、確認できた範囲だけで記録する。以下は公式一次情報の記載事実と、それを組み立てる設計案を分けて書く。3件の source はいずれも 2026-09-27 に確認した内容に基づく。

## 要点（公式一次情報に記載された事実）

### systemd.exec v257 の範囲（systemd 側の事実）

- `ProtectProc=` は `hidepid=` / `invisible` / `ptraceable` / `default` の値を取ることが `systemd.exec.xml` v257 で確認できた。`MountAPIVFS=` の設定を含意する関係が同ファイルに記載されている。v247 以降は system サービス限定の制約があることが同ファイルで確認できた。[systemd.exec v257](https://raw.githubusercontent.com/systemd/systemd/v257/man/systemd.exec.xml)
- `PrivateTmp=` には mount ユニットへの依存関係があることが同ファイルで確認できた。ネットワーク名前空間やファイルシステム名前空間の完全な実装方式までは本調査の検証範囲に含めない。[systemd.exec v257](https://raw.githubusercontent.com/systemd/systemd/v257/man/systemd.exec.xml)
- `DynamicUser=` は `NoNewPrivileges=`、`RestrictSUIDSGID=`、`ProtectSystem=strict`、`ProtectHome=read-only` を含意することが同ファイルで確認できた。動的 UID の範囲は 61184–65519 で、`DynamicUser=` 自体は v232 導入であることが同ファイルで確認できた。[systemd.exec v257](https://raw.githubusercontent.com/systemd/systemd/v257/man/systemd.exec.xml)
- 本調査で `ProtectSystem=` について再確認できた事実は、上記の `DynamicUser=` が `ProtectSystem=strict` を含意する点に限る。`ProtectSystem=` の全オプション表（`yes` / `no` / `full` 等の列挙と各々のマウント動作）は本調査では再検証していないため、本ドキュメントには書かない。[systemd.exec v257](https://raw.githubusercontent.com/systemd/systemd/v257/man/systemd.exec.xml)

### no_new_privs の範囲（カーネル側の事実）

- `no_new_privs` フラグは Linux 3.5 以降で `prctl(PR_SET_NO_NEW_PRIVS, 1)` により設定することが kernel userspace API 文書で確認できた。[No New Privileges Flag](https://docs.kernel.org/userspace-api/no_new_privs.html)
- 同フラグは `fork` / `clone` / `execve` を跨いで継承され、一度設定すると解除できないことが同文書で確認できた。[No New Privileges Flag](https://docs.kernel.org/userspace-api/no_new_privs.html)
- `execve` 時の保証は、setuid / setgid / file capability / LSM 経由の権限獲得が起きないことであることが同文書で確認できた。一方で `execve` を伴わない `setuid` 系呼び出しそのものを禁止するものではないことが同文書で確認できた。[No New Privileges Flag](https://docs.kernel.org/userspace-api/no_new_privs.html)
- systemd の `NoNewPrivileges=` はこのカーネルフラグをサービスに適用する指示であり、`DynamicUser=` を使うと自動的に含意される（前節の通り）。systemd 側の既定値や他指示との組み合わせ表は本調査では再検証していない。[systemd.exec v257](https://raw.githubusercontent.com/systemd/systemd/v257/man/systemd.exec.xml) / [No New Privileges Flag](https://docs.kernel.org/userspace-api/no_new_privs.html)

### capability の bounding set と ambient set の範囲（カーネル側の事実）

- per-thread の bounding set は Linux 2.6.25 以降で継承され、`execve` を跨いで保持されることが `capabilities(7)` で確認できた。`PR_CAPBSET_DROP` による bounding set からの削除は不可逆であることが同ページで確認できた。[capabilities(7)](https://man7.org/linux/man-pages/man7/capabilities.7.html)
- `execve` 時の permitted set の計算式は `P'(permitted) = (P(inheritable) & F(inheritable)) | (F(permitted) & P(bounding)) | P'(ambient)` であることが同ページで確認できた。ここで `P` はプロセス（スレッド）の set、`F` は実行ファイルの set を指す。[capabilities(7)](https://man7.org/linux/man-pages/man7/capabilities.7.html)
- ambient set は Linux 4.3 以降に存在し、特権ファイルの実行時にはクリアされることが同ページで確認できた。[capabilities(7)](https://man7.org/linux/man-pages/man7/capabilities.7.html)
- systemd の `CapabilityBoundingSet=` はこの bounding set に対する上限として働く指示である。`NoNewPrivileges=` と併用したときの相互作用の詳細な真理値表は本調査では再検証していないため、本ドキュメントでは式の読み替え（bounding を絞ると式中の `F(permitted) & P(bounding)` の項が絞られる）だけを事実の範囲として残す。[capabilities(7)](https://man7.org/linux/man-pages/man7/capabilities.7.html)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- まず `DynamicUser=yes` で始められるかを検討する。単一指示で `NoNewPrivileges`・`RestrictSUIDSGID`・`ProtectSystem=strict`・`ProtectHome=read-only` が同時に入り、UID 範囲 61184–65519 の動的割り当てになることが確認済みのため、状態を持たないサービスの初期値として扱いやすい。状態の永続化が必要になった時点で `StateDirectory=` 等の明示的な永続化へ移す。
- 権限上昇の抑止は `NoNewPrivileges=yes` を明示する。`DynamicUser=` を使わないサービスでも同指示でカーネルの `no_new_privs` が適用され、`execve` 経由の setuid / setgid / file capability による上昇が起きない。ただし `execve` を伴わない権限変更は対象外のため、「root で起動したまま `setuid` 呼び出しで下げる」設計とは別に、`User=` / `DynamicUser=` で起動ユーザ自体を落とす。
- capability の絞り込みは bounding set の上限から決める。`execve` 式の `F(permitted) & P(bounding)` の項に着目し、サービスが必要としない capability を `CapabilityBoundingSet=` から外す。ambient set（Linux 4.3 以降）を使う場合、特権ファイル実行でクリアされる点を前提にし、ambient への追加は `CapabilityBoundingSet` の範囲内に収める。
- `/proc` の可視性は `ProtectProc=` で絞る。system サービスであることを適用条件として確認し（v247 以降の限定）、`hidepid=` 相当の隠蔽が目的か、`invisible` 相当の完全な隠蔽が目的か、デバッグ用の `ptraceable` 許容が必要かを選ぶ。`MountAPIVFS=` が含意される点を前提にし、`/proc` 以外の API ファイルシステムのマウントを別途上書きしない。
- `/tmp` の分離は `PrivateTmp=yes` で行い、mount ユニットへの依存があることを前提にする。他サービスの `/tmp` と共有する設計（ソケットや一時ファイルの受け渡し）を残したまま有効化しない。共有が必要な箇所は `JoinsNamespaceOf=` や明示的なディレクトリ共有の要否を別途確認する（本調査の範囲外のため方式の断定はしない）。

## 避ける使い方

- `DynamicUser=` の含意を把握せずに `ProtectSystem=` や `ProtectHome=` を重ねて矛盾させる。`DynamicUser=yes` は既に `ProtectSystem=strict` と `ProtectHome=read-only` を含むことが確認済みであり、緩めたい場合は `DynamicUser=` 自体の採用可否から見直す。
- `NoNewPrivileges=yes` を「全ての権限変更の禁止」と誤読する。確認済みの保証は `execve` 時の setuid / setgid / file capability / LSM 経由の権限獲得の抑止であり、`execve` を伴わない `setuid` 呼び出しは対象外である。
- bounding set を絞らずに ambient set だけを追加する。`execve` 式で permitted は bounding との AND 項を含むため、bounding の上限を超えた ambient の付与は期待通りに残らない。前提として `CapabilityBoundingSet` の範囲を確認する。
- `ProtectProc=` を user サービスに適用できると仮定する。v247 以降の system サービス限定が確認済みであり、user サービスでの等価な隠蔽は本ドキュメントの保証外とする。
- `PrivateTmp=` を有効にしたまま `/tmp` 経由のプロセス間連携を残す。mount ユニット依存の分離が入ることが確認済みのため、共有ソケットや共有一時ファイルを `/tmp` に置く設計と併用しない。

## 適用版と本番での注意

- 適用版: `systemd.exec.xml` の v257 タグ固定内容、kernel userspace API 文書の 7.3.0-rc4 版 `no_new_privs` ページ、`capabilities(7)` の man-pages ページ。いずれも 2026-09-27 に確認した内容に基づき、将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で、linux は技術固有 TTL の対象外のため、2026-12-26 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: `ProtectSystem=` の全オプションの動作表、`PrivateTmp=` の tmpfs 実装の詳細と `JoinsNamespaceOf=` 等の共有方式、`NoNewPrivileges=`・`CapabilityBoundingSet=`・`AmbientCapabilities=`・`SecureBits=` の組み合わせ真理値表、`ProtectProc=` の各値と `hidepid=` マウントオプションの対応表、サービス起動の timeout・cancel・shutdown 時の名前空間破棄の順序。これらは該当ページの該当節を別途確認する。
- 本ドキュメントの推奨方法は設計案であり、単一のベンチマークや障害事例の一般化ではない。必要な sandboxing 指示の組み合わせは対象サービスの権限・名前空間・共有要件ごとに実機で起動確認して決める。

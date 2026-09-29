---
{
  "id": "distributed-systems-raft-leader-election-log-replication",
  "title": "Raft consensus の leader election と log replication: election timeout、quorum commit、linearizable read",
  "kind": "knowledge",
  "technology": "distributed-systems",
  "version": "raft.github.io official site (current)、In Search of an Understandable Consensus Algorithm (USENIX ATC '14 Best Paper, pp. 305-319)、etcd-io/raft main branch README (stable, Apache-2.0)、go.etcd.io/raft v3.7.0 Go module docs (published 2026-06-03)",
  "tags": [
    "research-domain:api-distributed",
    "distributed-systems",
    "raft",
    "consensus",
    "leader-election",
    "log-replication",
    "quorum",
    "majority",
    "election-timeout",
    "heartbeat",
    "ElectionTick",
    "HeartbeatTick",
    "MsgVote",
    "MsgApp",
    "PreVote",
    "ReadIndex",
    "linearizable",
    "commit",
    "term",
    "candidate",
    "follower",
    "overlapping-majorities",
    "etcd",
    "state-machine"
  ],
  "sources": [
    {
      "id": "raft-github-io-site",
      "url": "https://raft.github.io/",
      "type": "official_docs"
    },
    {
      "id": "usenix-atc14-ongaro-raft-paper",
      "url": "https://www.usenix.org/conference/atc14/technical-sessions/presentation/ongaro",
      "type": "architecture_pattern"
    },
    {
      "id": "etcd-io-raft-readme",
      "url": "https://github.com/etcd-io/raft",
      "type": "official_docs"
    },
    {
      "id": "etcd-raft-go-pkg-docs-v3",
      "url": "https://pkg.go.dev/go.etcd.io/raft/v3",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-29",
  "expires_at": "2026-12-28",
  "trust": "maintainer",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# Raft consensus の leader election と log replication: election timeout、quorum commit、linearizable read

信頼できないプロセッサ群の合意 (consensus) を leader election、log replication、safety に分解して解く Raft の要点を、公式サイト・原論文・etcd-io/raft 実装文書の4件で整理する。公式サイト [The Raft Consensus Algorithm](https://raft.github.io/) (current)、原論文 [In Search of an Understandable Consensus Algorithm](https://www.usenix.org/conference/atc14/technical-sessions/presentation/ongaro) (USENIX ATC '14 Best Paper, pp. 305-319)、[etcd-io/raft README](https://github.com/etcd-io/raft) (main branch, stable, Apache-2.0, etcd/Kubernetes の基盤)、[go.etcd.io/raft v3.7.0 module docs](https://pkg.go.dev/go.etcd.io/raft/v3) (published 2026-06-03) を 2026-09-29 に取得して確認した。以下で「記載事実」「設計案」を明示的に分ける。

## 要点（確認した文書の記載事実）

### consensus の定義と Raft の分解

- 公式サイトは consensus を「信頼できないプロセッサ (unreliable processors) の間での合意」と定義し、progress の条件として majority を要求する。具体例として 5 servers が 2 failures に耐える (5 servers can tolerate 2 failures)。
- 適用対象は replicated state machine であり、各 state machine が合意した同一の command 列 (log) を実行することで整合する。
- Raft は問題を leader election、log replication、safety に分解する。公式サイトは Paxos と等価な fault-tolerance を持つと述べる。
- 原論文は同じ3要素の分離を骨子とし、Paxos と等価な efficiency と、より強い coherency (理解しやすさ・一貫性) を主張する。cluster membership change は overlapping majorities による方式が追加されている。

### leader election: term、election timeout、majority quorum

- Raft のノードは leader、follower、candidate の役割を持ち、term (任期) で世代を区切る。公式サイトと原論文がこの役割分担を Raft の基礎とする。
- follower が election timeout の間に leader からの通信を受け取らなければ election を開始し、candidate となって投票を要求する。etcd-io/raft の実装文書では、この時間管理が `ElectionTick` と `HeartbeatTick` の2つの抽象 tick として表れる。応用側が `Node.Tick` を周期的に呼び、election 側の経過で campaign (立候補)、heartbeat 側の経過で leader の broadcast が駆動される。
- etcd-io/raft の message flow では、`MsgHup` が自ノードの election 開始 (campaign)、`MsgBeat` が leader による heartbeat 送信の契機、`MsgVote` / `MsgVoteResp` が投票要求と応答を担う。candidate は majority の票を得ると leader になる (candidate-to-leader on majority votes)。
- 分断などで古い leader が残っている場合の攪乱を抑える選択肢として PreVote がある。etcd-io/raft README は majority-quorum election に対する option として PreVote を挙げる (本調査では既定の有効・無効までは確認しない。未確認)。
- follower は有効な leader からの Append (log 複製) や Heartbeat を受け取ると follower に留まる・復帰する (v3.7.0 docs の記載)。term の新しさで世代を判定し、古い世代の leader の要求は退ける。

### log replication: leader 主導の複製と quorum commit

- すべての client 要求は leader を経由する。leader が log entry を追記し、`MsgApp` / `MsgAppResp` で follower 群へ複製する (etcd-io/raft README および v3.7.0 docs の message flow)。
- entry は majority のノードに保存された時点で commit される (quorum commit)。commit された entry は state machine へ適用してよい。5台構成なら3台への保存で確定し、残り2台の故障に耐える (公式サイトの 5 servers / 2 failures と対応)。
- commit は leader のいる term の entry を起点に進む。leader は commit index を `MsgApp` に載せて follower へ伝え、follower も確定範囲を適用する (本調査で確認した文書の範囲では、旧 term の entry の commit 扱いなど safety 則の厳密な条件式までは引用しない)。

### linearizable read: quorum ReadIndex と lease-based local read

- log を経由しない読み取りを linearizable に保つ仕組みとして、etcd-io/raft は quorum による ReadIndex と lease-based local read の2経路を示す (README の記載)。
- v3.7.0 docs は `ReadOnlyOption` / `ReadIndex` を linearizable query の経路として公開する。ReadIndex 経路は leader が quorum に確認して現在の read index を確定させ、lease-based 経路は leader の lease 有効期間内の local read を使う (各経路の時計・ lease 前提の厳密な安全性条件は本調査の確認範囲外。未確認)。
- いずれも「leader 以外が古い値を返す」「失効した leader が応答する」ことの防止が目的であり、単なる follower への直接読み (stale read) とは区別される。

### etcd-io/raft の責務分割: transport と storage を持たない core library

- etcd-io/raft は minimal な core-algorithm library として設計され、transport (message 配送) と storage (log 永続化) を持たない。応用側が `MsgVote` / `MsgApp` などの message 配送と、log・snapshot の保存を提供する (README の記載)。
- 時間も抽象化されており、ライブラリは wall-clock を直接使わず、応用側の `Node.Tick` 駆動で `ElectionTick` / `HeartbeatTick` を消費する。このため election timeout と heartbeat interval の実時間は応用側の tick 周期と `Config` の tick 数の積で決まる。具体的な既定 tick 数は本調査の inventory で未確認のため、数値を引用しない。

## 推奨方法（独自の設計案。上記文書の規定ではない）

- Raft を選ぶのは、単一 leader による順序付けと majority による確定でよい場合に絞る。multi-leader の書き込み順序や leader を介さない確定が要る場合は Raft の適用外であり、別方式 (multi-Paxos 系、CRDT 等) を検討する。
- tick 設計は heartbeat と election を桁違いに離す。`HeartbeatTick` で leader の生存を細かく伝え、`ElectionTick` の満了でのみ campaign を起こす構成にし、短い network stall で選挙が連発しない余裕を持たせる。具体的な tick 数・ミリ秒換算は tick 周期と環境の RTT・GC 停止時間から実測で決める (文書に既定値の規定を確認していないため、数値の推奨はしない)。
- 選挙の攪乱が懸念される構成 (WAN 越え、preemptible なノード) では PreVote の有効化を検討する。分断側の古い term での campaign が現 leader を引きずり降ろす事象への備えである。有効化の判断と観測 (選挙回数、term の単調増加の監視) をセットにする。
- commit 待ちの client 応答は、apply まで待つか commit 確定で返すかを API ごとに決める。確定前の応答は failover 時に失われる可能性があり、確定後の応答は latency が増える。線形化が必要な読み取りは ReadIndex 経路または lease-based local read を使い、許容できる stale read のみ follower 直読みにする。経路の使い分けを読み取り API の仕様に明記する。
- transport / storage は応用側の責務として切り出す。message の配送 (at-least-once と重複排除の方針)、log entry の永続化 (fsync 範囲)、snapshot と log compaction の境界を、core library の外側の設計書に書く。etcd-io/raft が持たない部分こそ障害時の振る舞いを決める。
- membership 変更は overlapping majorities の考え方に従い、新旧両構成の majority が重なる手順でのみ進める。片方の構成だけでの一括置換は、2人の leader が並立する split-brain の原因になる。

## 避ける使い方

- majority を満たさない台数での commit 扱い (例: 5台中2台への保存で確定とする)。公式サイトの progress 条件 (5 servers tolerate 2 failures) に反し、故障時に確定済み entry が失われる。
- `MsgApp` / Heartbeat の停止を放置したまま election timeout を短くする構成。leader の生存通知が届かないだけで選挙が連発し、可用性が落ちる。
- PreVote を理解せず有効・無効を決めること (攪乱防止の option であり、選挙の正当性そのものを変えるものではない。既定値も本調査では未確認)。
- stale read を linearizable read として提供すること。follower への直接読みは ReadIndex / lease-based の経路を通らないため、線形化の保証はない。
- 失効した leader (lease 切れ・分断側) からの読み・書き応答を正として扱うこと。follower 側で term と commit index の検証なしに応答しない。
- transport / storage の実装なしに core library だけで合意が動くと想定すること (etcd-io/raft は配送と永続化を持たない。応用側の未実装は沈黙故障になる)。
- 新旧構成の majority が重ならない membership 変更 (overlapping majorities の要求に反し、split-brain を招く)。
- 単一 incident や単一 benchmark の数値を普遍的な tick 既定値として採用すること (本調査は既定 tick 数を確認しておらず、環境依存のため実測が必要)。

## 適用版と本番での注意

- 適用版: raft.github.io (current、版表記なし)、原論文 (USENIX ATC '14 Best Paper, pp. 305-319)、etcd-io/raft main branch README (stable、Apache-2.0、etcd/Kubernetes の基盤)、go.etcd.io/raft v3.7.0 (Go module docs、published 2026-06-03)。いずれも 2026-09-29 取得。
- 保証の範囲: 4文書が保証するのは Raft の分解 (leader election / log replication / safety)、majority progress (5台で2故障)、leader 主導の複製と quorum commit、etcd-io/raft の message flow (`MsgHup` / `MsgBeat` / `MsgVote` / `MsgApp` と各応答)、`ElectionTick` / `HeartbeatTick` + `Node.Tick` の時間抽象化、ReadIndex / lease-based の読み取り経路の存在までである。
- 未確認範囲: `ElectionTick` / `HeartbeatTick` の既定 tick 数と推奨実時間、election timeout のランダム化の有無と幅、PreVote の既定の有効・無効、旧 term entry の commit 条件の厳密な safety 則、ReadIndex / lease の厳密な安全性前提、snapshot・log compaction・membership 変更の API 詳細。これらを設計に使う場合は v3.7.0 docs と README の該当節を別途確認する。
- 信頼度: 文書全体の trust は `maintainer` とする (原論文・etcd 実装文書が maintainer 由来のため、4件中最も低い水準に合わせる)。
- 再確認期限: official_docs の TTL は90日、architecture_pattern の TTL は180日。technology (distributed-systems) 固有 TTL は設定されていない。最も早い期限 (official_docs 3件) に合わせて 2026-12-28 までに4文書を再取得して内容を確認する。

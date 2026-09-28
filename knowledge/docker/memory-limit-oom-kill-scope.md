---
{
  "id": "docker-memory-limit-oom-kill-scope",
  "title": "Docker コンテナのメモリ制限と OOM 終了のスコープ: --memory/--memory-swap の合計 semantics と cgroup v2 memory.max/memory.high",
  "kind": "knowledge",
  "technology": "docker",
  "version": "Docker Docs Resource constraints / CLI reference docker container run / daemon troubleshoot (retrieved 2026-09-28, no page version label, CLI options up to API 1.56+); Linux kernel docs Control Group v2 7.3.0-rc5; Docker Engine 20.10 release notes (20.10.0 2020-12-08 – 20.10.24 2023-04-04)",
  "tags": [
    "research-domain:infrastructure",
    "docker",
    "memory-limit",
    "--memory",
    "memory-swap",
    "--memory-swap",
    "memory-reservation",
    "oom-killer",
    "oom-kill-disable",
    "oom-score-adj",
    "cgroup-v2",
    "memory.max",
    "memory.high",
    "memory.swap.max",
    "swap-accounting",
    "swappiness",
    "resource-constraints"
  ],
  "sources": [
    {
      "id": "docker-resource-constraints-docs",
      "url": "https://docs.docker.com/engine/containers/resource-constraints/",
      "type": "official_docs"
    },
    {
      "id": "docker-container-run-cli-docs",
      "url": "https://docs.docker.com/reference/cli/docker/container/run/",
      "type": "official_docs"
    },
    {
      "id": "kernel-cgroup-v2-docs",
      "url": "https://docs.kernel.org/admin-guide/cgroup-v2.html",
      "type": "official_docs"
    },
    {
      "id": "docker-daemon-troubleshoot-docs",
      "url": "https://docs.docker.com/engine/daemon/troubleshoot/",
      "type": "official_docs"
    },
    {
      "id": "docker-engine-20-10-release-notes",
      "url": "https://docs.docker.com/engine/release-notes/20.10/",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-10-28",
  "trust": "official",
  "status": "active"
}
---

# Docker コンテナのメモリ制限と OOM 終了のスコープ: --memory/--memory-swap の合計 semantics と cgroup v2 memory.max/memory.high

コンテナのメモリ上限が何を意味するか（`--memory-swap` の合計 semantics）と、上限到達時にどの範囲のプロセスが OOM killer で殺されるのか（OOM 終了のスコープ）を、Docker の公式文書・CLI リファレンス・トラブルシューティング文書・リリースノートと、Linux カーネルの cgroup v2 文書に分けて記録する。以下は公式一次情報の記載事実と、それを組み立てる設計案を分けて書く。5件の source はいずれも 2026-09-28 に確認した内容に基づく。

## 要点（公式文書に記載された事実）

### `--memory` と `--memory-swap` の合計 semantics（Docker 側の事実）

- `-m` / `--memory` はメモリの上限で、最小値は `6m` である。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/)
- `--memory-swap` は `--memory` と同時に指定したときだけ意味を持つ修飾子であり、正の値は「memory + swap の合計」を表す。文書の例では `--memory=300m` と `--memory-swap=1g` を併せると 300m の RAM と 700m の swap が許可される。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/) [docker container run](https://docs.docker.com/reference/cli/docker/container/run/)
- `--memory-swap` を `--memory` と同じ値にするとスワップは使われない。`-1` はスワップ無制限。`--memory-swap` を未指定にすると `--memory` と同じ量のスワップが許可され、合計は `--memory` の2倍になる。`0` は指定されなかったのと同じ扱いで無視される。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/)
- CLI リファレンスも `--memory-swap` を「Swap limit equal to memory plus swap: '-1' to enable unlimited swap」と定義し、合計 semantics を明示している。[docker container run](https://docs.docker.com/reference/cli/docker/container/run/)
- `--memory-reservation` はソフトリミットで、値が `--memory` より小さいときだけ有効になる。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/) [docker container run](https://docs.docker.com/reference/cli/docker/container/run/)
- `--memory-swappiness` は 0-100 の値を取り、既定はホストから継承される（CLI リファレンス上のフラグ既定値は `-1`）。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/) [docker container run](https://docs.docker.com/reference/cli/docker/container/run/)

### OOM 終了のスコープ（Docker 側の事実）

- 既定では、コンテナがメモリ上限を超えるとカーネルの OOM killer が起動してコンテナ内のプロセスを殺す。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/)
- `--oom-kill-disable` は `-m` と併用したときだけ許可される。`-m` 無しで `--oom-kill-disable` を使うとホストが OOM になり、ホスト側のプロセスが殺される可能性がある。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/)
- Docker が調整する OOM 優先度はデーモンのものだけで、コンテナの OOM score は調整しない。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/) CLI リファレンスの `--oom-score-adj` は「Tune host's OOM preferences (-1000 to 1000)」と定義され、調整対象はホストの OOM 偏好である。[docker container run](https://docs.docker.com/reference/cli/docker/container/run/)
- ホストのメモリを超えてコンテナが使おうとすると OOM exception が起き、「a container, or the Docker daemon, might be stopped by the kernel OOM killer」と記載されており、殺害対象はコンテナに限らず Docker ダーモンにも及ぶ。[Troubleshooting the Docker daemon](https://docs.docker.com/engine/daemon/troubleshoot/)
- コンテナ内で `free` を実行するとホストのスワップが表示されるため、コンテナ内でスワップが使えるかどうかの判定には使えない。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/)

### cgroup v2 の memory.* と OOM スコープ（カーネル側の事実）

- `memory.max` はハードリミットで、上限に達し削減不能な場合「OOM killer is invoked in the cgroup」＝起動する OOM killer のスコープはその cgroup の内側である。上限は一時的に超過することがある。[Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- `memory.high` はスロットルリミットで、超過すると heavy reclaim pressure のなかプロセスが throttled される。ただし「Going over the high limit never invokes the OOM killer」であり、極端な条件下では上限を超えることもあるため、外部の監視プロセスを前提とする。[Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- `memory.swap.max` はスワップ専用のハードリミットで、既定値は `max` である。現在の使用量より下げた場合、既存のスワップは段階的にしか回収されず、長時間にわたり超過が継続することがある。[Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- `memory.low` はベストエフォートの保護として機能する。[Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- `memory.max` / `memory.high` を `O_NONBLOCK` で開いた場合、同期リクレーム（および oom-kill）は bypass され、次回のチャージ時に発動する。[Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)

### swap 会計の有効化と版履歴（troubleshoot / release notes の事実）

- `WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.` は、カーネルが swap 会計をサポートしない環境でメモリ/スワップ制限が破棄されることを示す警告である。Ubuntu/Debian では `GRUB_CMDLINE_LINUX="cgroup_enable=memory swapaccount=1"` を設定して有効化する手順が文書にある。会計のオーバーヘッドとして、文書は「約1%のメモリと全体で10%の性能劣化」と記載する。[Troubleshooting the Docker daemon](https://docs.docker.com/engine/daemon/troubleshoot/)
- Docker Engine 20.10.0（2020-12-08）で `docker run --kernel-memory` が非推奨になった（moby/moby#41254）。[Docker Engine 20.10 release notes](https://docs.docker.com/engine/release-notes/20.10/)
- 20.10.6（2021-04-12）の「cgroup2: Move cgroup v2 out of experimental」（moby/moby#42263）により、cgroup v2 サポートが実験扱いを解除された。[Docker Engine 20.10 release notes](https://docs.docker.com/engine/release-notes/20.10/)
- 20.10.8（2021-08-03）で、cgroups v2 実行時に `Your kernel does not support swap memory limit` 警告が誤って出る問題が修正された（moby/moby#42479）。この修正以前の cgroups v2 環境では、この警告が誤作動である可能性を排除できない。[Docker Engine 20.10 release notes](https://docs.docker.com/engine/release-notes/20.10/)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- swap を使いたくないレイテンシー感度の高いサービスは、`--memory` と `--memory-swap` を同値にして明示的に swap を閉じる。「未指定なら合計=2倍」という既定を暗黙のままにしない。
- swap を許容する場合は合計値を計算して指定する。RAM 300m + swap 700m なら `--memory=300m --memory-swap=1g` のように、`--memory-swap` に RAM 容量ではなく合計を入れる。
- ホストのメモリ計画では、コンテナの合計上限の総和に加え、Docker ダーモン自身も kernel OOM killer の対象になりうるという文書の記載を前提にヘッドルームを残す。
- swap 会計の警告（`Limitation discarded`）が出ているホストでは制限が実際に効いていない可能性があるため、起動時・CI のログにこの警告がないかを確認してから上限設定を信頼する。会計有効化のオーバーヘッド（文書記載で約1%のメモリと全体で10%の性能劣化）は計測対象に含める。
- `--memory-reservation` をソフトリミットとして使う場合、値は必ず `--memory` より小さくする（それ以外は有効にならないという公式契約のため）。ハードリミット到達時の挙動を変えたい場合の代替として `--memory-reservation` を使っていると誤解しない。
- OOM 時のスコープは2層で理解する。個別コンテナの上限超過は既定ではそのコンテナ内プロセスの殺害に収まるが、ホスト全体のメモリ枯渇ではコンテナと Docker ダーモンの双方が kernel OOM killer の対象になりうる。監視はコンテナ単体の usage だけでなくホストの使用率にも向ける。
- cgroup v2 上で「上限に達しても殺さず遅延だけ与える」挙動が必要な場合は、カーネル文書の `memory.high` が OOM killer を呼ばないスロットルであることを前提に設計する。ただし本調査では Docker の各フラグと cgroup v2 の各ファイルの対応（写像）は公式に確認していないため、対応関係は別途検証してから採用する。

## 避ける使い方

- `--memory-swap` に RAM の容量だけを入れる。合計 semantics のため、そのまま書くと期待より swap が狭くなり、意図せず swap 不能に近い状態になる。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/)
- `-m` 無しで `--oom-kill-disable` を付ける。公式に `-m` 併用時のみ許可とされ、`-m` 無しではホストが OOM になってホスト側プロセスが殺されうる。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/)
- コンテナ内の `free` の出力から「このコンテナはスワップを使える/使えない」を判断する。表示されるのはホストのスワップであり、コンテナの swap 上限は別に `--memory-swap` で決まる。[Resource constraints](https://docs.docker.com/engine/containers/resource-constraints/)
- swap 会計の警告を無視したまま `--memory-swap` の設定値が効いていると判断する。文書上、警告が出ている環境では制限が破棄される。[Troubleshooting the Docker daemon](https://docs.docker.com/engine/daemon/troubleshoot/)
- `memory.high` を「OOM を防ぐための上限」として扱う。カーネル文書は high の超過が OOM killer を絶対に呼ばないと明記しており、監視は外部プロセスが前提になる。[Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- `memory.swap.max` を現在使用量より下げて即座に効果を期待する。既存スワップは段階的にしか回収されず、超過が長時間継続しうることが文書にある。[Control Group v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)
- 20.10 系リリースノートの内容（`--kernel-memory` 非推奨、cgroup v2 の実験解除、swap 警告の誤作動修正）を現行版の挙動として無条件に当てはめる。版履歴として読み、対象環境の版を確認する。[Docker Engine 20.10 release notes](https://docs.docker.com/engine/release-notes/20.10/)

## 適用版と本番での注意

- 適用版: Docker Docs の Resource constraints / `docker container run` CLI リファレンス（API 1.56+ のオプションを含む）/ daemon troubleshoot の3ページはいずれも版ラベルのない現行ページで、2026-09-28 に確認した内容に基づく。カーネル側は Linux kernel documentation の Control Group v2（docs build 7.3.0-rc5）。履歴は Docker Engine 20.10 release notes（20.10.0 = 2020-12-08 〜 20.10.24 = 2023-04-04）。いずれも将来の最新とは扱わない。
- 再確認期限: 5件のうち4件は `official_docs`（TTL 90日）で 2026-12-27、`release_notes`（TTL 30日）が最短の 2026-10-28。docker は技術固有 TTL の対象外のため、明示期限はこの最小日である release notes の再確認日に合わせて 2026-10-28 とした。
- 未確認事項（本調査の範囲外として推測で埋めない）: Docker の `--memory` / `--memory-swap` / `--memory-reservation` と cgroup v2 の `memory.max` / `memory.high` / `memory.swap.max` の対応（写像）関係は本調査で公式に確認していない。cgroup v1 相当（`memory.limit_in_bytes` 等）の挙動、Docker Desktop（macOS/Windows）での差、Kubernetes の memory limit との関係、`docker update` による実行中コンテナの上限変更時の挙動、OOM score の具体的な計算式、コンテナの OOM 終了を検知するための docker events / exit status の確認手順も未確認である。これらは該当ページの該当節または一次情報で別途確認する。
- 本ドキュメントの推奨方法は設計案であり、単一のベンチマークや障害事例の一般化ではない。swap 会計のオーバーヘッドや上限到達時のレイテンシー影響はワークロードとホスト構成に依存するため、対象環境で測定して採用する。

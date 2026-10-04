---
{
  "id": "docker-umask-api-exec-initial-permission-boundary",
  "title": "Docker umask: API降格・exec/healthcheck・初期ファイル権限の境界",
  "kind": "knowledge",
  "technology": "docker",
  "version": "Docker Engine/CLI 29.8.0 feature (2026-09-03), Engine API 1.56; Moby 67755e065510390fb3839cd389200511733ba9ba, CLI 6239084af1a2550e01f887257def584779e7ddf7; Linux man-pages 6.19; verified 2026-10-04 UTC; Docker runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "docker",
    "umask",
    "HostConfig.Umask",
    "API-1.56",
    "docker-exec",
    "healthcheck",
    "OCI",
    "file-permissions",
    "default-ACL",
    "octal",
    "entrypoint"
  ],
  "sources": [
    {
      "id": "docker-umask-run-docs-20261004",
      "url": "https://docs.docker.com/reference/cli/docker/container/run/",
      "type": "official_docs"
    },
    {
      "id": "docker-umask-engine-2980-release-20261004",
      "url": "https://docs.docker.com/engine/release-notes/29/",
      "type": "release_notes"
    },
    {
      "id": "docker-umask-api-history-20261004",
      "url": "https://docs.docker.com/reference/api/engine/version-history/",
      "type": "official_docs"
    },
    {
      "id": "moby-umask-api-exec-67755e0-20261004",
      "url": "https://github.com/moby/moby/commit/67755e065510390fb3839cd389200511733ba9ba",
      "type": "github_repository_analysis"
    },
    {
      "id": "docker-cli-umask-parser-6239084-20261004",
      "url": "https://github.com/docker/cli/commit/6239084af1a2550e01f887257def584779e7ddf7",
      "type": "github_repository_analysis"
    },
    {
      "id": "linux-umask-man-pages-619-20261004",
      "url": "https://man7.org/linux/man-pages/man2/umask.2.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Docker umask: API降格・exec/healthcheck・初期ファイル権限の境界

## 解く問いと結論

entrypoint で `umask 077` を実行したのに、保守用の `docker exec` や healthcheck が別の権限でファイルを作るのはなぜか。Docker の `--umask` を設定すれば、すべてのファイルを常に private に保てるのか。

Docker Engine/CLI 29.8.0 には、コンテナを作るときに `HostConfig.Umask` を指定し、entrypoint・exec・healthcheck の OCI process 設定へ渡す経路が追加された。ただしこれは**起動時のプロセス属性**であり、ファイル権限を永久に強制する仕組みではない。API の版、数値表現、runtime の実装、アプリによる再設定、保存先の default ACL を分けて確認する。[29.8.0 release notes](https://docs.docker.com/engine/release-notes/29/#2980) / [CLI の umask 契約](https://docs.docker.com/reference/cli/docker/container/run/#umask)

特に、対応 daemon でも低い版の API を指定した create リクエストでは、この項目を捨てる互換処理がある。「作成に成功した」「PID 1 で期待値を表示した」だけを、後から起動する全経路への適用証拠にしない。以下は確認した契約と固定実装の観察を先に示し、その後で独自の移行・受入手順を提案する。

## 調査版と非重複の範囲

- 公式 release notes の導入日は 2026-09-03、版は 29.8.0。API history は追加を v1.56 に分類する。これは今回確認した機能の導入版であり、その版を現在の推奨更新先とする意味ではない
- 実装分析は Moby `67755e065510390fb3839cd389200511733ba9ba` と Docker CLI `6239084af1a2550e01f887257def584779e7ddf7` に固定した。両者は 2026-09-03 に merge された導入 PR の snapshot である。現行全 patch 版と全 runtime の動作を実機確認したものではない
- 既存の [multi-stage と non-root](multi-stage-nonroot-cache-secret.md) はビルド成果物・実行ユーザ・cache/secret mount が主題。本書は、同一コンテナ内でも異なる起動経路が持つ file creation mask と API wire contract を扱う。Docker DNS/TLS、embedded containerd、メモリ上限、systemd 再起動待機とは別の問いである

[API version history](https://docs.docker.com/reference/api/engine/version-history/#v156-api-changes) / [Moby 導入 commit](https://github.com/moby/moby/commit/67755e065510390fb3839cd389200511733ba9ba) / [CLI 導入 commit](https://github.com/docker/cli/commit/6239084af1a2550e01f887257def584779e7ddf7)

## 確認した契約と実装

### 1. CLI は8進数、API JSON は10進数

CLI の `--umask` は8進表記を受け付け、先頭ゼロは省略できる。たとえば `--umask 077` と `--umask 77` は同じ値である。一方、API の `HostConfig.Umask` は nullable な unsigned 32-bit integer であり、JSON の整数は10進数として扱う。CLI に渡した文字列を、そのまま JSON number に写してはならない。[CLI reference](https://docs.docker.com/reference/cli/docker/container/run/#umask) / [固定 API schema](https://github.com/moby/moby/blob/67755e065510390fb3839cd389200511733ba9ba/api/swagger.yaml)

| 意図した mask | CLI の値の例 | API JSON の値 | 備考 |
|---|---|---|---|
| 0022 | `022` または `22` | `18` | 典型的な一般ファイル用の選択肢 |
| 0077 | `077` または `77` | `63` | group/other の生成時権限を落とす |
| 0000 | `0` | `0` | mask する bit がない。未指定とは異なる |
| 0777 | `777` | `511` | 生成時の通常 permission bit を全部落とす |

数値変換表は schema の説明と8進数計算から作った独自の整理であり、権限ポリシーの推奨値一覧ではない。特に API に `77` と送ると 0077 ではなく 0115 となる。設定ファイルを生成する場合は、表示用8進文字列と wire 用整数を別の値として検査する。

固定 CLI の `opts/opts.go` は基数8の32-bit unsigned parse を行う。unit test では `0o22` と `9` を拒否し、`077777` のような大きい8進値も parser 自体は受理する。したがって、CLI parser の成功を「0000〜0777 の運用範囲を検証済み」と読み替えない。許容 mask をその範囲に制限するのは、この文書が提案する設定生成側の方針であり、Docker が同じ範囲を reject するという説明ではない。[固定 parser と test](https://github.com/docker/cli/commit/6239084af1a2550e01f887257def584779e7ddf7)

### 2. 未指定と明示 zero を保持する

Moby の型は `*uint32`、CLI の option も nil を保持できる。未指定では daemon は OCI process に umask を設定せず、runtime の既定動作に委ねる。明示した `0` は値ありとして渡され、permission bit を mask しない。設定の serialize、テンプレートの default、差分計算で両者を同一視しない。[HostConfig と WithUmask](https://github.com/moby/moby/commit/67755e065510390fb3839cd389200511733ba9ba)

公式 CLI 文書は runc の既定 mask の例として 0022 を示す一方、既定値は runtime 依存であると明記する。これを「未指定なら entrypoint・exec・healthcheck がどの runtime でも必ず0022」と一般化しない。明示値を OCI に渡すことと、選んだ runtime が各起動経路でその値を使うことも別の確認点である。[CLI reference](https://docs.docker.com/reference/cli/docker/container/run/#umask)

### 3. 低い API 版では設定が無視される経路がある

固定 Moby の `postContainersCreate` は、リクエスト API が 1.56 より前なら `HostConfig.Umask` を nil に戻してから backend の container creation を呼ぶ。inspect 側にも、1.56 未満では返す HostConfig から Umask を除く互換処理がある。このため、対応 Engine に接続している事実だけでは足りない。[create route](https://github.com/moby/moby/blob/67755e065510390fb3839cd389200511733ba9ba/daemon/server/router/container/container_routes.go) / [inspect route の同 commit 差分](https://github.com/moby/moby/commit/67755e065510390fb3839cd389200511733ba9ba)

ここでの観察は、固定 daemon に古い versioned API で直接送る場合の処理である。「現行 Docker CLI が無条件に黙って送る」という主張ではない。CLI 側には API 1.56 と Linux の flag annotation もある。SDK の negotiation、明示した API version、CLI の対応、daemon の対応を個別に確認する。低い版の inspect に項目がない場合も、それだけでサーバに保存された設定の有無を断定しない。[固定 CLI flag 定義](https://github.com/docker/cli/commit/6239084af1a2550e01f887257def584779e7ddf7)

この機能を要件とする deployment では、設定が無視されても続行する互換運転ではなく、実際の API version と確認結果を満たさなければリリースを止める方針が適する。これは独自の判断である。稼働中のコンテナへの変更方法として `docker update --umask` が使えると本調査からは言えない。確認した契約は create 時の設定なので、未検証の update 手段を前提にしない。

### 4. docker exec は PID 1 の現在の mask を複製する操作ではない

固定 `daemon/exec.go` は containerd に保存された OCI spec の Process を取り出し、exec 用の引数・環境などを設定する。Linux の user 解決では、その Process にある Umask を保持したまま UID/GID を差し替える。`docker exec --user ...` の指定だけで設定 mask を失わないよう実装されている。[exec の Process 読取](https://github.com/moby/moby/blob/67755e065510390fb3839cd389200511733ba9ba/daemon/exec.go) / [exec の user 解決](https://github.com/moby/moby/blob/67755e065510390fb3839cd389200511733ba9ba/daemon/exec_linux.go)

対して Linux の通常の `fork` は親の mask を継承し、`execve` はその値を変えない。この OS の契約と、上記 Docker exec の設定生成経路を組み合わせると、entrypoint の shell が `umask 077` に変更してから子アプリを起動することと、daemon 経由で別途 `docker exec` を起動することは区別しなければならない。名前に exec が入っていても、PID 1 が自身の子として行う `execve` と同じ設定伝播を想定しない。[umask(2)](https://man7.org/linux/man-pages/man2/umask.2.html)

この区別から、entrypoint 内の変更が保存済み OCI spec まで書き換えるとは扱わない。初期値を統一したい場合は create 時の設定を使い、それでも shell の初期化ファイル、entrypoint、アプリが自分の mask を変えないかは別に調べる。Docker の設定値を「変更不能なセキュリティ制約」と名付けない。

API history は healthcheck も設定対象に含める。固定 Moby の integration tests は、run・exec・healthcheck、未指定/数値/名前の user、明示 0000 と 0777 を確認するケースを持つ。これは調査者が上流 test の内容を読んだ証拠であり、今回その test を実行して合格させたという意味ではない。[API history](https://docs.docker.com/reference/api/engine/version-history/#v156-api-changes) / [Moby tests の導入差分](https://github.com/moby/moby/commit/67755e065510390fb3839cd389200511733ba9ba)

### 5. mask は生成 mode から bit を落とす。既存資産や ACL の監査は残る

通常の新規作成では、要求した mode から umask の bit を落とす。たとえば要求0666と mask0077なら0600、要求0777と同じ maskなら0700になる。要求0600を mask0022で0644へ広げることはない。これは mode と mask の関係からの計算例であり、すべてのアプリが常に0666でファイルを作るという保証ではない。[umask(2)](https://man7.org/linux/man-pages/man2/umask.2.html)

同 manual は、親ディレクトリに default ACL がある場合、単純な umask 計算ではなく ACL の継承と要求 mode の制約が使われることも定める。bind mount や volume 上のファイルを確認するときは、プロセスの mask だけで実際のアクセスを判定しない。既存ファイルを新規作成時の計算で評価し直したり、umask の設定でそのファイルを修復できたと扱ったりもしない。生成後にアプリが権限を変更する場合は、その処理も別途監査対象になる。

実行 UID/GID、所有権、親ディレクトリのアクセス権、ACL、read-only mount は別の軸である。`--umask 077` を設定したので non-root や保存先の境界を省略できる、という設計にはしない。逆に共有 group の読取りが必要な成果物へ0077を一律に当てれば、利用側が読めなくなる可能性がある。

## 移行と受入判定（独自の設計案）

次の順序は上記契約を組み合わせた運用案であり、Docker が指定する標準手順ではない。本調査では Docker daemon を起動せず、アプリケーションやホストの権限も変更していない。

1. **対象の経路を列挙する。** アプリ、保守用 exec、healthcheck、定期ジョブがどの保存先に何を作るかを表にする。秘密ファイルと group 共有ファイルを同じ要件にしない。初期 mask の責任を entrypoint だけに置いていた箇所を探す
2. **実際の対応条件を記録する。** CLI/SDK と daemon の版、使う API version、OS、OCI runtime とその版、image digest、create 時の HostConfig を保存する。旧 API で取得した inspect を確認済みの証拠にしない。Windows 向けに同じ指定を流用しない。固定 Moby は Windows の Umask 指定を拒否する
3. **設定生成を検査する。** 未指定と0を保持し、CLI の8進表記から JSON number への変換を検査する。0077を意図した API payload は63。権限の要件として 0000〜0777 の許容値を決め、parser が受けるだけの大きい値は運用設定から除く
4. **新規の隔離検証コンテナで観測する。** 本番データを mount せず、同じ runtime と image を使う。保存された設定値に加え、main・通常 exec・別 user の exec・healthcheck の実測 mask を比較する。Linux 4.7以降は `/proc/<pid>/status` の Umask 欄でも対象プロセスの値を読める。別プロセスを起動して `umask` と表示させた値を、観測対象の PID の値だと誤認しない
5. **実ファイルのアクセスを検査する。** 新規 file と directory の mode、所有者、ACL と実際の読取り可否を見る。まず default ACL なしで基本式を確認し、次に本番相当の共有先の条件で確認する。単に healthcheck が成功終了しただけでは、その生成物の権限要件まで満たしたとはしない
6. **経路差を受入条件にする。** entrypoint が意図的に mask を変えるケースを加え、その子アプリと後発 exec が別の値になる条件を記録する。旧 API、未指定、zero、別 user、アプリによる再設定をそれぞれ独立した境界ケースにする。期待値が合わなければ権限を広げて回避せず、API・runtime・process・保存先の順で原因を分ける

受入テストの最小構成は次のように整理できる。期待値は対象 runtime とアプリの仕様に合わせて確定し、無条件に表の値を全環境の保証としない。

| ケース | 観測するもの | 早まって結論にしないもの |
|---|---|---|
| 明示0077、対応API/runtime | 保存設定と main/exec/healthcheck の mask、生成物 | create の成功だけ |
| 同じ設定で exec user を変更 | UID/GID と mask の両方 | user だけ一致したこと |
| 未指定と明示0 | nil/値ありの区別、各起動経路の実測 | JSON zero-value の同一視 |
| entrypoint が後から変更 | その子アプリと daemon 起動 exec の差 | コンテナ内全プロセスが同じ値という仮定 |
| API 1.56未満で直接 create | 設定が保持されたか、実測値 | 対応版 daemon なので必ず適用されたという仮定 |
| default ACL のある保存先 | ACL・要求mode・生成物・実アクセス | umask の数値だけ |

## 避ける判断

- `--umask 077` を API の `"Umask": 77` に機械的に置換する
- 未指定を zero に正規化し、意図せず mask なしへ変更する
- API version を見ず、Engine の版や HTTP 成功応答だけで適用済みとする
- shell の `execve` と Docker exec を同じ設定継承と扱う
- mask を既存ファイルの修復、ACL の代替、アプリが変更できない強制境界として使う
- CLI parser の受理範囲を、実用的な permission mask の検証範囲と読み替える
- 上流テストの存在や repository の check 表示を、本環境での runtime 実証と記載する

## 出典、ライセンス、未確認事項

- 公式 Docker CLI、Engine release notes、API history、Linux man-pages の実ページを 2026-10-04 UTC に native web で読んだ。Docker の CLI/API history には個別の公開・更新日表示を確認できなかった。umask(2) は man-pages 6.19、本文日付2026-02-08、HTML生成日2026-09-09を区別して記録した
- Moby と CLI の PR 本文も native web で確認した。完全SHAの commit ページは cache miss、interactive API v1.56は実仕様本文を返さず Markdown版も取得失敗だった。これらの取得失敗を成功と数えず、固定SHAの差分・ファイル・API schema・LICENSEを GitHub connector で照合した。CLI commit の短縮SHA取得も一度失敗し、PR metadataで得た完全SHAでは成功した
- Moby と CLI の固定 LICENSE は Apache-2.0。HTML各ページ単体のライセンスは確定できず catalog では unknown とした。ここでは出典付きの日本語独自要約と独自の運用案だけを追加し、上流のコード・test・長い文章は転載していない
- Docker CLI/runtime/daemon の実行、Moby/CLIのビルド、実コンテナでのfile/ACL検証は行っていない。固定 CLI の `e2e/container/run_test.go` にある umask test は、その snapshot では `Skip` を含む。上流 integration test の記述と、実行合格の証拠を混同しない
- runc/crun/代替runtime各版の対応下限、Compose/Swarm/Kubernetesへの同名設定の可用性、既存コンテナをupdateする経路、Windows以外の全platform、全storage driverのACL動作、vendor backportは未確認。Dockerのcreate用flagがあることから他のorchestrator対応を推測しない
- release notes を参照するため、取得日から30日後の2026-11-03を再確認期限とする。検索 eval はこの文書を発見できることだけを検査し、umask の実動作・ファイルの機密性・runtime互換性を保証しない

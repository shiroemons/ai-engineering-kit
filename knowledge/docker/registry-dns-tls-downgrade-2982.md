---
{
  "id": "docker-registry-dns-tls-downgrade-2982",
  "title": "Docker Engine 29.8.2: registry DNS による TLS downgrade の修正と暫定対策の境界",
  "kind": "knowledge",
  "technology": "docker",
  "version": "Docker Engine 29.8.2 (2026-09-30); GHSA-7cfq-22r6-qp73 / CVE-2026-92543 (2026-10-01); Docker CLI rolling reference verified 2026-10-02; daemon未実検証",
  "tags": [
    "research-domain:infrastructure",
    "docker",
    "registry",
    "DNS",
    "TLS",
    "insecure-registries",
    "CVE-2026-92543",
    "29.8.2",
    "digest",
    "Server.Version"
  ],
  "sources": [
    {
      "id": "docker-engine-2982-registry-release-20261002",
      "url": "https://docs.docker.com/engine/release-notes/29/#2982",
      "type": "release_notes"
    },
    {
      "id": "moby-registry-dns-advisory-20261002",
      "url": "https://github.com/moby/moby/security/advisories/GHSA-7cfq-22r6-qp73",
      "type": "maintainer_article"
    },
    {
      "id": "docker-dockerd-registry-reference-20261002",
      "url": "https://docs.docker.com/reference/cli/dockerd/",
      "type": "official_docs"
    },
    {
      "id": "docker-version-server-reference-20261002",
      "url": "https://docs.docker.com/reference/cli/docker/version/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-02",
  "expires_at": "2026-11-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/infrastructure.json"
  ]
}
---

# Docker Engine 29.8.2: registry DNS による TLS downgrade の修正と暫定対策の境界

## 問いと今回の変更

「private registry に正しい CA を設定し、insecure-registries を空にすれば、安全な接続だと判断できるか」を扱う。Docker Engine **29.8.2（2026-09-30）** は、細工された DNS 応答によって registry 接続の TLS 証明書検証を省略したり HTTP fallback を許したりする **CVE-2026-92543** を修正した。資格情報の漏洩とイメージ置換につながる変更である。[29.8.2 release notes](https://docs.docker.com/engine/release-notes/29/#2982)

既存の multi-stage/secret mount 文書やメモリ制限文書とは異なり、daemon が registry に接続する経路の判断を対象とする。Docker Desktop 全体、独立した BuildKit、Kubernetes の CRI runtime の安全性判定へは一般化しない。

## 確認した契約と告知

### 通常の insecure registry の意味

dockerd reference は、TLS を使わない、または daemon が証明書を信頼できない registry を insecure と説明する。明示設定には hostname:port と CIDR があり、IPv4 loopback `127.0.0.0/8` の暗黙の例外も記載する。通常の自己署名 CA 運用では insecure 化より CA の信頼設定を推奨しているが、この一般論を今回の脆弱性への対策保証と読み替えてはいけない。[dockerd reference](https://docs.docker.com/reference/cli/dockerd/#insecure-registries)

### 影響版と不十分な対策

Moby maintainer の **2026-10-01 公開 advisory** が示す範囲は次のとおりである。[GHSA-7cfq-22r6-qp73](https://github.com/moby/moby/security/advisories/GHSA-7cfq-22r6-qp73)

- Docker Engine は **29.8.2 未満が影響、29.8.2 が修正版**。別に Go module `github.com/moby/moby/v2` は `v2.0.0-beta.25` を境界として挙げる。製品版と module 版を混同しない
- 判定時に解決した複数 IP のいずれかが insecure CIDR に合致すると、別 IP への実接続にも弱い設定が使われ得る。既定の例外には IPv6 `::1/128` もある
- 設定から insecure-registries を削除するだけでは暗黙の例外が残る。信頼済み CA の追加も、この検証回避の解消にはならない
- 更新できない期間は、registry 名が信頼した non-loopback IP だけを返す名前解決、または daemon の送信先を信頼した registry endpoint に限定する方法が告知されている。digest 固定はイメージ置換への多層防御だが、資格情報の漏洩は防がない

この説明は advisory の原著要約であり、脆弱な入力を再現する手順や修正コードの解析ではない。

### CLI と接続先 Engine の版を分ける

`docker --version` は CLI の版を表示する。一方、`docker version` の Client/Server は別々の構成要素であり、Server はリモートホスト上の場合もある。`docker version --format '{{.Server.Version}}'` は接続先 Engine の版を取得する公式の形式である。[docker version reference](https://docs.docker.com/reference/cli/docker/version/)

## 運用判断の提案（独自の設計案）

以下は上記資料を組み合わせた確認手順であり、Docker が定める監査基準ではない。設定変更や更新を実行した記録でもない。

1. CI runner、開発環境、共有 build host の「実際に接続する daemon」を棚卸しする。承認済みの接続先ごとに Server.Version と配布元を記録し、手元の CLI の版だけで判定しない。到達不能な daemon は未確認として残す
2. upstream Engine の影響範囲と照合する。修正版を採用する計画では、パッケージを配置した事実に加え、稼働中の接続先が修正版になったことを読み取りで確認する。distribution の backport はその配布元の根拠を別途必要とする
3. 更新待ちの例外運用には、対象 registry、名前解決または送信先制御の担当、解除条件を記録する。イメージの digest 固定だけで「対処済み」にしない
4. 認証情報の露出が疑われる場合は、更新の完了とインシデント対応を別項目にする。更新だけで過去の漏洩が取り消されるとは扱わず、管理者が影響資格情報と調査・失効範囲を判断する

`dockerd --dns` は公式には**コンテナ用 DNS** の設定として説明される。この値の変更だけで daemon の registry 名前解決を修復したと判断しないことも、本書の確認方針である。[Daemon DNS options](https://docs.docker.com/reference/cli/dockerd/#daemon-dns-options)

## 検証観点（本書では未実行）

実環境へ適用する担当者向けの確認案である。実資格情報や公開 registry を使った脆弱性再現は含めない。

- 更新前後で同じ承認済み接続先の Server.Version を記録し、対象 daemon の取り違えを検出する
- private CA、必要な registry/mirror、既存の承認済み認証方式について、更新後の通常利用に回帰がないか検証する
- 暫定対策を採る場合は、daemon が利用する resolver/egress とコンテナ内部の DNS を別々に確認する。名前が引けた、HTTP が成功した、といった単独の結果を TLS 検証の証拠にしない
- 移行対象が残る場合は、確認済み・暫定対策中・未確認を区別して保持し、一部の成功を全体の修正完了としない

## 適用範囲・未確認事項・出典管理

- 取得日は **2026-10-02 UTC**。release notes による最短 TTL 30 日に合わせ、再確認日は **2026-11-01**。その日まで他の脆弱性がないことを保証する期限ではない
- daemon を起動せず、実環境の更新、DNS/ネットワーク設定変更、registry 認証、脆弱性再現は実施していない。advisory の修正実装、各 distribution の backport、Desktop 同梱版、proxy/mirror ごとの詳細挙動、既存資格情報の侵害有無は未確認
- rolling CLI reference には個別の更新日・リリース版表示がなく、取得時点の契約として記録した。advisory の affected version 表から、各過去版の導入時期や全 OS での再現性まで推測しない
- Docker Docs は [docker/docs README](https://github.com/docker/docs/blob/main/README.md) と [LICENSE](https://github.com/docker/docs/blob/main/LICENSE) で Apache-2.0 を確認した。advisory 本文の個別ライセンスは unknown とし、出典を付けた独自要約のみを記載した。実装コードの転載・module への昇格はない

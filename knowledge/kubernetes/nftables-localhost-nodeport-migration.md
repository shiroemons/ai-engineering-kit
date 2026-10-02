---
{
  "id": "kubernetes-nftables-localhost-nodeport-migration",
  "title": "Kubernetes 1.37 nftables localhost NodePort: Alphaの二重opt-inとTCP・loopback・rollback境界",
  "kind": "knowledge",
  "technology": "kubernetes",
  "version": "Kubernetes v1.37.0 kube-proxy (f54c212e3a2f75d674b717a9b29052b20b60aefc), KubeProxyConfiguration v1alpha1; v1.36.0 validation comparison; verified 2026-10-02, runtime untested",
  "tags": [
    "research-domain:infrastructure",
    "kubernetes",
    "kube-proxy",
    "nftables",
    "NodePort",
    "localhost",
    "KubeProxyNFTablesLocalhostNodePorts",
    "nodePortAddresses",
    "TCP",
    "migration",
    "rollback"
  ],
  "sources": [
    {
      "id": "kubernetes-v137-selinux-release-20261002",
      "url": "https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/",
      "type": "release_notes"
    },
    {
      "id": "kubernetes-nftables-localhost-virtual-ips-20261002",
      "url": "https://kubernetes.io/docs/reference/networking/virtual-ips/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-nftables-localhost-cli-v137-20261002",
      "url": "https://kubernetes.io/docs/reference/command-line-tools-reference/kube-proxy/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-nftables-localhost-config-v137-20261002",
      "url": "https://kubernetes.io/docs/reference/config-api/kube-proxy-config.v1alpha1/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-selinux-feature-gates-20261002",
      "url": "https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/",
      "type": "official_docs"
    },
    {
      "id": "kubernetes-nftables-localhost-implementation-v1370-20261002",
      "url": "https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/proxy/nftables/proxier.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "kubernetes-nftables-localhost-validation-v1360-20261002",
      "url": "https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/pkg/proxy/apis/config/validation/validation.go",
      "type": "github_repository_analysis"
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

# Kubernetes 1.37 nftables localhost NodePort の移行境界

## 問いと適用範囲

iptables経由の `localhost:<NodePort>` に依存するnode上のclientを、nftablesへ移せるか。結論は「v1.37には選択肢があるが、upgradeだけでは有効にならず、TCP以外まで互換になるわけではない」。対象はLinuxのkube-proxyであり、Windows、kube-proxyを置換する独自dataplane、Docker Engineのfirewall backendは対象外とする。

2026-08-26の[公式release announcement](https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/#localhost-nodeport-userspace-proxy-for-nftables)が導入を明記する。以下はv1.37.0の固定commitと、取得時にv1.37を表示する公式資料の照合であり、実クラスタの疎通・性能試験結果ではない。

## 確認した契約

### 1. v1.37、Alpha、二重opt-inを分ける

[feature gate表](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/)と[v1.37.0のgate定義](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/features/kube_features.go)は、`KubeProxyNFTablesLocalhostNodePorts` をv1.37導入のAlpha・既定 `false` とする。nftables backend自身の利用と、このlocalhost機能の有効化は別である。

有効化には、対象nodeのkube-proxyで次をそろえる。

- proxy modeが `nftables`
- `KubeProxyNFTablesLocalhostNodePorts: true`
- `nodePortAddresses` に `localhost` または対象IP familyの明示的loopback CIDRを含める
- ServiceがTCP NodePortで、proxyが使えるendpointとbind可能なloopback portがある

`nftables` の未指定時のNodePortアドレスは `primary`。v1.37に更新しただけではlocalhost listenerは増えない。また、[CLI reference](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-proxy/)と[configuration API](https://kubernetes.io/docs/reference/config-api/kube-proxy-config.v1alpha1/)は、この版のLinuxのproxy mode既定を引き続き `iptables` と記す。「v1.37からnftablesが全クラスタで既定」と読み替えない。

### 2. `all` は明示的loopback指定ではない

`nodePortAddresses` は有効なCIDRとkeywordsを組み合わせられる。`primary` はnodeのprimary IPv4/IPv6アドレス、`localhost` は `127.0.0.0/8` と `::1/128`、`all` は `0.0.0.0/0` と `::/0` に展開される。ただし、範囲にloopbackが数学的に含まれることと、localhost proxyを選択したことは異なる。

[v1.37.0のkeyword展開](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/cmd/kube-proxy/app/server.go)と[ContainsExplicitLoopback](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/proxy/util/nodeport_addresses.go)を照合した結果は次のとおり。

| gate | nftablesのアドレス指定 | localhost userspace proxy |
|---|---|---|
| false | `primary,localhost` | 作られない |
| true | 未指定、`primary`、`all`、またはzero CIDRだけ | 作られない |
| true | `primary,localhost` | 対応するIPv4/IPv6 familyでTCP用proxyが選択される |
| true | `primary,127.0.0.1/32` | IPv4だけ。IPv6のlocalhostを有効にしたことにはならない |
| true | `all,localhost` | 明示loopback条件を満たすが、非loopback側の対象も広がる |

familyの選択後、[listener実装](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/proxy/localnodeportproxy/proxy.go)がbindするのはIPv4の `127.0.0.1`、IPv6の `::1`。`localhost` keywordのCIDR展開を根拠に、`127.0.0.2` など任意のloopback aliasでもlistenerができると解釈しない。IPv4だけの疎通成功からIPv6のService・endpoint経路まで合格にしない。

### 3. 設定ファイル使用時にCLIだけ変えても直らない

[CLI reference](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-proxy/)では、`--config` 使用時の `--nodeport-addresses` と `--proxy-mode` は無視される。実際の起動引数、mountされた設定、配布元を確認し、次のような項目を既存の `KubeProxyConfiguration` に反映する。これは独自の説明用抜粋であり、そのまま既存設定を置換する完全なmanifestではない。

```yaml
mode: nftables
featureGates:
  KubeProxyNFTablesLocalhostNodePorts: true
nodePortAddresses:
  - primary
  - localhost
```

API fieldは `nodePortAddresses`、CLI flagは `--nodeport-addresses` である。`iptables.localhostNodePorts` / `--iptables-localhost-nodeports` はiptables・IPv4側の旧動作を制御するため、この新機能のswitchとして使わない。v1.37の生成referenceにはその旧field説明に「他mode/IPv6では不可」という一般化が残るが、同じページの新 `nodePortAddresses` 契約、release、固定実装が示すnftables・TCPの例外を優先して範囲を読む。

### 4. TCP listenerとreject ruleの条件は同じではない

[v1.37.0 proxier](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/proxy/nftables/proxier.go)では、gateがtrueならlocalhost用reject処理を有効にし、さらに明示loopback条件を満たしたときだけuserspace proxyを作る。

- proxyあり: TCPだけをuserspace listenerの候補にする。UDP/SCTPのlocalhost NodePort trafficにはreject ruleを作る
- gate true・proxyなし: localhost NodePortのTCP/UDP/SCTPすべてをreject対象にする。gateだけONは疎通の準備完了を意味しない
- gate false: この追加のproxy/reject処理は有効にしない。host firewall等を含めた実際の失敗が常にtimeoutかconnection refusedかは、本稿では保証しない

これはNodePort Service自体にUDP/SCTPを使えないという意味ではなく、新しいlocalhost経路の制限である。対応外protocolのService作成が失敗するとは限らず、作成成功もlocalhost疎通の証拠にならない。

### 5. backend選択にも互換境界がある

固定版の `syncProxyRules` と `LocalNodePortProxy.SyncNodePorts` / `pickEndpoint` から確認した実装上の条件:

- `externalTrafficPolicy: Local` の場合、localhost proxyに渡す候補をlocal endpointsに限定する。候補が0ならlistenerを維持せず、別nodeのendpointへ自動fallbackしない
- `sessionAffinity: ClientIP` はlistener単位でendpointをpinする。複数のlocal client processを独立したIPとして分散する機能ではない
- affinity timeoutは新規接続時に更新されるsliding window。接続が続けば同じpinが維持されうるため、「timeout秒ごとに必ず別backendへ移る」としない
- backendへのTCP dialに失敗した接続は閉じる。この実装は、その接続内で別endpointへ順次retryする処理を持たない

以上は[v1.37.0のsource読解](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/proxy/localnodeportproxy/proxy.go)であり、実測した分散率や耐障害性の保証ではない。

## 移行判断と確認手順（独自提案）

1. node上のregistry client、agent、health check等がどのIP family・protocol・NodePortへ接続するかを棚卸しする。iptables側の `kubeproxy_iptables_localhost_nodeports_accepted_packets_total` の増加はlocalhost利用の手がかりになるが、増加ゼロだけで利用がないとは断定しない
2. v1.37 Alpha機能の採用が許容されるかを決める。localhostを使わないclientには機能を一律有効化せず、依存を残すnode群だけで検証する
3. 実効mode・gate・設定ファイル・TCP Service・EndpointSlice・空きportを確認し、対象nodeのkube-proxy再起動を伴うcanaryとして扱う。API serverだけをv1.37へ上げた状態を合格にしない
4. localhost、primary IP、必要な追加node IPを別々に試験する。IPv4とIPv6、TCPと対応外protocol、Local policyでlocal endpointがある場合/ない場合を分ける
5. bind成功、backend接続成功、application応答成功を別々に観測する。失敗時は全nodeへ展開せず、設定・port競合・endpoint・policyを切り分ける

localhost対応を追加しても、[従来のnftables移行差分](https://kubernetes.io/docs/reference/networking/virtual-ips/#migrating-from-iptables-mode-to-nftables)は消えない。Linux kernel 5.13以上が必要で、primary以外のNodePort到達範囲は明示設定し、host firewallの許可は別に用意する。kernel 6.1未満の長寿命TCP/conntrack問題についてもiptablesのworkaroundに依存していないかを確認する。`--conntrack-tcp-be-liberal` はその診断に応じて検討するもので、localhostの二重opt-inを代替しない。

## 観測とrollback

[v1.37.0 metrics定義](https://github.com/kubernetes/kubernetes/blob/f54c212e3a2f75d674b717a9b29052b20b60aefc/pkg/proxy/metrics/metrics.go)とlistener実装では次を確認できる。これらはAlpha metricで、将来版の名前・互換性は本稿の保証外。

- `kubeproxy_nftables_localhost_nodeport_listeners{ip_family}`: 現在のlistener数。正値でもbackend疎通や利用実績までは示さない
- `kubeproxy_nftables_localhost_nodeport_listener_creation_failures_total{ip_family}`: bind作成失敗の累積。別host processとのport競合等を調べる。一つのlistener作成失敗を記録しても他listenerのsyncは継続する
- `kubeproxy_nftables_localhost_nodeport_rejected_packets_total{ip_family,protocol}`: localhost NodePort拒否。TCP増加時はgateだけ有効でproxyを選択していないケースも調べる

機能を戻す際は、localhostに依存するclientの代替接続先と許容停止を先に確保する。gateを無効にしてkube-proxyを再起動すればuserspace listenerは提供されなくなる。`Shutdown` / listener停止はin-flight connectionも閉じる実装なので、rollbackを無停止の接続引継ぎとは扱わない。代替にnode IPを使う場合も、TLS identity・認証・到達範囲を別途設計し、単にinsecure registryの許可範囲を広げる解決を既定にしない。

さらにbinaryをv1.36へ戻す場合は設定構文も戻す。[v1.36.0のvalidator](https://github.com/kubernetes/kubernetes/blob/ecf6decece6a6de25a57aad9ba90b6ce580f6f78/pkg/proxy/apis/config/validation/validation.go)は `primary` 単独またはCIDRリストを受け付け、`localhost` / `all` keywordsや `primary` 混在を受け付けない。旧版に新gate設定や新構文を残さず、その版で検証済みの設定へ戻す。CIDRへ書き直せばv1.36 nftablesにもuserspace localhost機能が生える、という意味ではない。

## 根拠・日付・限界

- 導入日はrelease announcementの2026-08-26。repository分析はv1.37.0=`f54c212e3a2f75d674b717a9b29052b20b60aefc`、比較するv1.36.0=`ecf6decece6a6de25a57aad9ba90b6ce580f6f78` に固定し、各tagをcommitへ解決した。取得日は2026-10-02 UTC
- current docsのLast modifiedはvirtual-ipsが2026-08-06、CLI/config referenceが2026-08-26、feature gate表が2026-01-27と表示された。feature tableの表示日は導入日ではないため、releaseと固定版sourceを併用した
- 読んだupstream testsは `TestContainsExplicitLoopback`、`Test_useLocalhostNodePortProxy`、`TestLocalhostNodePortReject`、`TestLocalhostNodePortRejectAll`、session affinity、listener失敗、`TestShutdownClosesInFlightConnections` 等。テストsourceを確認しただけで、上流suiteを実行したとの主張はしない
- docsはKubernetes Authorsの[CC-BY-4.0](https://github.com/kubernetes/website/blob/main/LICENSE)、分析コードは各固定版LICENSEとheaderのApache-2.0を確認。本文は出典に基づく独自要約と独自の運用提案で、上流コードのコピーやmodule化はしていない
- 実クラスタ、CNIとの組合せ、managed distributionの設定可否、mixed-version rollout、throughput/latency、socket枯渇、停止時間、v1.38以降へのgraduationは未検証。Alphaの採用可否と本番SLOは利用側の試験・判断が必要

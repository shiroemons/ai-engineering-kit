---
{
  "id": "github-package-firewall-routing-cache-admission-boundary",
  "title": "GitHub package firewall preview: 経由確認・cache・overrideとinstall受理の境界",
  "kind": "knowledge",
  "technology": "supply-chain-security",
  "version": "GitHub Enterprise Cloud package firewall public preview 2026-10-20, clarification 2026-10-21; live Docs and npm CLI 12.2.1 documentation checked 2026-10-23 UTC; service/runtime untested",
  "tags": [
    "research-domain:security",
    "supply-chain-security",
    "github-package-firewall",
    "npm",
    "admission",
    "cache",
    "override",
    "registry-routing"
  ],
  "sources": [
    {
      "id": "github-package-firewall-preview-20261020-20261023",
      "url": "https://github.blog/changelog/2026-10-20-github-package-firewall-in-public-preview/",
      "type": "release_notes"
    },
    {
      "id": "github-package-firewall-controls-20261023",
      "url": "https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/protecting-endpoints-with-package-firewall",
      "type": "official_docs"
    },
    {
      "id": "npm-config-firewall-routing-1221-20261023",
      "url": "https://docs.npmjs.com/cli/v12/using-npm/config/",
      "type": "official_docs"
    },
    {
      "id": "npm-cache-firewall-integrity-1221-20261023",
      "url": "https://docs.npmjs.com/cli/v12/commands/npm-cache/",
      "type": "official_docs"
    },
    {
      "id": "npm-lock-firewall-origin-1221-20261023",
      "url": "https://docs.npmjs.com/cli/v12/configuring-npm/package-lock-json/",
      "type": "official_docs"
    },
    {
      "id": "npm-npmrc-firewall-auth-scope-1221-20261023",
      "url": "https://docs.npmjs.com/cli/v12/configuring-npm/npmrc/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# GitHub package firewall preview: 経由確認・cache・overrideとinstall受理の境界

## 問いと結論

GitHub package firewallを設定したCIでinstallが成功したら、依存tree全体が現在のsecurity policyを通過したと言えるか。

**その成功だけでは言えない。** 本文は新しいregistry admission機能の適用範囲を扱う。設定の存在、検査経路を通った事実、取得したbytesの同一性、後日の脆弱性判明、実行時の安全性を別の証拠にする。既存の[lockfileとintegrity](npm-lock-integrity-scoped-registry-ci.md)、[Actions cache権限](github-cache-mode-reusable-default-poisoning-boundary.md)、[artifact attestation](github-artifact-attestation-slsa-verify.md)の再説明ではない。

## 確認した事実

### 1. 2026年10月の提供範囲

[公式発表](https://github.blog/changelog/2026-10-20-github-package-firewall-in-public-preview/)は2026-10-20公開、2026-10-21に説明と画像の編集注記がある。GitHub Enterprise Cloud向けpublic previewでdata residencyを含む。対象ecosystemはnpmのみ。endpointまたは内部registryを経由させ、package一覧のmetadata要求と実downloadをpolicyで評価する。preview中は無料、GA時は従量課金予定であり、恒久無料やGA済みとは記録しない。

「npm ecosystem対応」を「npm・pnpm・Yarn・Bunの全versionで動作保証」と読み替えない。ここで直接確認したclient文書はnpm CLI 12.2.1だけである。

### 2. Firewallの判断は何か

[GitHub Docs](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/protecting-endpoints-with-package-firewall)で確認した範囲:

- 既定: malware拒否はon、vulnerabilityはCritical only。attestation downgradeと新たに加わったinstall scriptsの検出もon。
- attestation判定は、以前SLSA attestationがあったpackageの要求versionでそれが欠ける変化を扱う。
- 最小package ageは既定1日、1〜365日を設定でき、0で無効。
- 各要求はenterprise所属user/botのtokenを必要とし、public repositoryのread-onlyを超える権限は不要と説明される。
- Artifactory連携はexperimental。公式手順はpublic npmへ直行するremoteをvirtual repositoryから除外する。
- 承認されたoverrideはpolicyに入り、削除できる。version欄を`*`にすると全versionへ対象が広がる。

この短い一覧は機能の要約で、脅威を漏れなく検出する保証ではない。

### 3. npm側の経路とcacheは別の契約

[.npmrc](https://docs.npmjs.com/cli/v12/configuring-npm/npmrc/)はCLI、環境変数、複数のnpmrcを設定源とし、project固有設定やscope別registryを認める。認証情報はregistryのhost、必要ならpathまで限定する。したがって、user-levelの設定ファイルを1つ配置しただけでは、個々のjobの実効設定を確認したことにならない。

[package-lock.json](https://docs.npmjs.com/cli/v12/configuring-npm/package-lock-json/)の`resolved`はregistry tarballだけでなくGitやlinkにも対応し、`registry.npmjs.org`は現在設定されたregistryを意味する特別な値。`integrity`は展開元artifact（registryならtarball）のSRI値であり、展開後tree全体の現在状態を保証しない。`inBundle`はbundled dependencyを示す。lockfileがあることと、全entryが独立したregistry downloadになることは同義ではない。

[Config](https://docs.npmjs.com/cli/v12/using-npm/config/)では`replace-registry-host`既定値`npmjs`がpublic registry由来のdist URLを置換する。一方、`offline`はinstall中のnetwork要求を行わず、`prefer-offline`はcacheの鮮度確認を省くが不足分を取得する。`prefer-online`が強制するのは鮮度確認であり、全tarballを必ず再downloadするという説明ではない。

[npm-cache](https://docs.npmjs.com/cli/v12/commands/npm-cache/)の`npm cache verify`はlocal indexとcached dataのintegrityを検査する。cacheの主な保証は返したdataが挿入時と同じであることで、永久保存を保証しない。これは現在のmalware/advisory policyによる再審査コマンドではない。

## 推奨する導入判断と受入試験

以下は上の契約から組み立てた**独自の設計案**であり、GitHubが自動提供する機能や今回の実測結果ではない。

### A. 「設定済み」を「経由確認済み」に進める

1. 導入対象をjob、developer環境、agent環境の単位で列挙する。owner、使用client/version、registry設定源、scope別接続先、cache復元元、container build内でのinstall有無を台帳にする。
2. 秘密値を伏せて実効registryとscope設定を採取する。lockの`resolved`をpublic npm・社内registry・外部tarball・Git・link・bundleに分類する。既存lockを無差別に書き換えるのではなく、想定した経路のまま再現できるかを検証する。
3. disposableな検証環境でcold cacheとwarm cacheを分ける。組織が許可した無害なtest package/versionで、正常受理とpolicy拒否の両方を試す。既存の共有cacheを消すことを導入条件にしない。
4. installのexit codeだけでなく、観測できた宛先、要求の有無、対象package/version、判定理由を照合する。証拠がなければ「通過した」ではなく「未観測」に分類する。

`registry`表示は設定の証拠、実際の要求先は経路の証拠、拒否試験はその入力に対するenforcementの証拠である。この3つを1つのgreen checkにまとめない。私有registryを経由し続ける構成では、publicとprivateそれぞれの責任境界を明示する。

### B. Cache再利用の主張を狭くする

network要求が発生しないcache再利用について、このnetwork gateが新しい判定を行ったとは主張できない。これは経路からの推論であり、GitHubのcache bypass脆弱性を実証した報告ではない。

推奨する分離は「保存したbytesの同一性」と「現時点での使用許可」。前者の検査が成功しても、後者の観測日時は進めない。cache利用を許可するなら、検査済みartifact digest、確認時刻、採用policyの記録と再審査条件を別管理する。これらの情報をfirewallがAPIで返すと仮定せず、取得できない項目は未取得として扱う。

既存の`node_modules`、container layer、内部mirror、bundled内容も導入時のinventoryに含める。保存層ごとに「今回取得したもの」と「再利用したもの」を分け、展開済みtreeのSBOMや継続的な依存検査を別の検証工程に置く。firewallの受理を完全なSBOM生成やruntime保証へ昇格させない。

### C. 例外と停止を運用として決める

緊急security fixも公開直後という理由で待機対象になりうるため、cooldownを唯一の優先順位にしない。修正の必要性、影響version、変更内容、検証担当者を記録し、まず単一versionの例外を審査する。全version wildcardを定常的な「CI復旧ボタン」にしない。期限の記録・再確認担当・不要時の削除は運用案であり、自動expiry機能の存在は確認していない。

認証失敗、service障害、policy拒否は別の原因コードとして扱う。独自の安全側の方針として、失敗時にpublic registryへ自動fallbackしない。可用性上の例外を必要とする場合は事前審査済みartifactだけを使うなど、代替の許可根拠を決める。これはpreview serviceのSLAや公式fail-open/fail-closed契約の断言ではない。

## 落とし穴と最小の検証表

| 検証対象 | 確かめること | 成功と誤認してはいけないもの |
|---|---|---|
| cold cache + 許可対象 | 想定先への要求と成功 | 設定ファイルが存在するだけ |
| cold cache + 拒否対象 | downloadまたは解決が停止すること | logに警告が出るだけ |
| warm cache / offline | 要求なしでも完了する範囲と扱い | `npm cache verify`を再認可と呼ぶこと |
| scoped / external source | 公開・非公開の想定経路と適用外 | 全URLを機械的に置換したこと |
| override除去後 | 再取得時と再利用時の差 | policy削除が既存bytesを消すという推測 |
| token期限切れ / 通信失敗 | 秘密漏洩なしで停止し、分類できること | 無断fallbackによるgreen CI |

追加の注意点:

- attestation downgrade検出を「全packageの署名必須」と解釈しない。攻撃に対する採用基準は別途設計する。
- install scripts検出を、任意の実行やinstall後のnetworkアクセスを隔離するsandboxとみなさない。build workerには独立した最小権限方針を残す。
- Docsの環境変数名`GITHUB_TOKEN`だけから、Actionsが自動発行するrepository tokenの受入れを推測しない。認証主体と必要条件を実環境で確認する。
- log収集でtoken、認証header、private package名を不用意に共有しない。観測のためにTLS検証を無効化しない。

## 適用版・ライセンス・未確認事項

- 2026-10-23 UTC取得。GitHub Docsは固定版・公開日なしのlive page。npm資料は表示版12.2.1であり、pageのLast edited日をnpm release日と取り違えない。preview変更の再確認期限は2026-11-22とする。
- GitHub Docsは[github/docsのCC-BY-4.0](https://github.com/github/docs/blob/main/LICENSE)を確認。npmは[documentationのCC-BY-4.0](https://github.com/npm/documentation/blob/main/LICENSE)と[CLIのArtistic-2.0](https://github.com/npm/cli/blob/latest/LICENSE)を確認したが、個々の配信pageとlicenseの対応を固定できずcatalogはunknown。Changelogもopen-content license未特定のためunknown。出典付き独自要約だけを行い、source code・設定例を転載していない。
- service接続、token発行、enterprise設定変更、package install、実際の拒否試験は実施していない。本文の検証表は実行済みチェックリストではない。
- GHES対応、npm以外のclient互換性、最小client版、data residencyの具体的routing、rate limit/SLA、policy反映遅延、metadataのfilter方法と拒否status、private packageの扱い、全script判定の細則、全token種別、cache/mirrorの再審査挙動は未確認。
- overrideがどのcontrolに優先するか、適用scope・自動期限の詳細、過去に受理されたartifactの後発malware判明時の扱いも断言しない。安全性を優先する本番の受入条件はこれらを実測してから定める。

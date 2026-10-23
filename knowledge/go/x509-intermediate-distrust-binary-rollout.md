---
{
  "id": "go-x509-intermediate-distrust-binary-rollout",
  "title": "Go x509 intermediate distrust: SPKI chain filtering と稼働バイナリ更新の境界",
  "kind": "knowledge",
  "technology": "go",
  "version": "Fix announced in Go 1.27.3 / 1.26.10 (2026-10-19); implementation and APIs inspected at go1.27.3 (2aae1000ea1d4b28923b96db28acf4e3b99d6aff); no patched-runtime handshake test",
  "tags": [
    "research-domain:backend",
    "crypto/x509",
    "CVE-2026-111943",
    "SPKI",
    "distrust",
    "Certificate.Verify",
    "platform-verifier",
    "binary-rollout",
    "revocation",
    "TLS"
  ],
  "sources": [
    {
      "id": "go-x509-distrust-release-history-20261023",
      "url": "https://go.dev/doc/devel/release",
      "type": "release_notes"
    },
    {
      "id": "go-x509-distrust-announcement-20261023",
      "url": "https://groups.google.com/g/golang-announce/c/66pojueaQek",
      "type": "release_notes"
    },
    {
      "id": "go-x509-distrust-api-1-27-3-20261023",
      "url": "https://pkg.go.dev/crypto/x509@go1.27.3",
      "type": "official_docs"
    },
    {
      "id": "go-x509-distrust-verify-source-1-27-3-20261023",
      "url": "https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/crypto/x509/verify.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go-x509-distrust-windows-source-1-27-3-20261023",
      "url": "https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/crypto/x509/root_windows.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go-x509-distrust-darwin-source-1-27-3-20261023",
      "url": "https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/crypto/x509/root_darwin.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go-x509-distrust-go-version-source-1-27-3-20261023",
      "url": "https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/alldocs.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go-x509-distrust-buildinfo-api-1-27-3-20261023",
      "url": "https://pkg.go.dev/runtime/debug@go1.27.3",
      "type": "official_docs"
    },
    {
      "id": "go-x509-distrust-tls-api-1-27-3-20261023",
      "url": "https://pkg.go.dev/crypto/tls@go1.27.3",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-23",
  "expires_at": "2026-11-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Go x509 の intermediate distrust と配備完了の判定

## 問い・対象

Go の security release に「一部の中間証明書を信頼しない」とある。OS の CA bundle または開発機の Go を更新すれば、配布済みの Go backend / CLI も修正済みになるのか。証明書 fingerprint を照合するだけで、拒否される接続を予測できるのか。

結論は、**証明書の照合対象、実際の検証経路、稼働バイナリの build 版を別々に確認する**。本稿は CVE-2026-111943 の修正を根拠に、Go の証明書検証を含むアプリの再ビルド・配布・確認の境界を整理する。trust store の探索先や環境変数の移行は [既存の platform root override 記事](x509-platform-root-override-migration.md) を参照し、ここでは繰り返さない。

以下の「公開契約」「固定実装の観察」と「独自の運用案」を区別する。アプリの更新、CA store の変更、信頼の例外追加をこの文書が実行または許可するものではない。

## 公開情報: 修正版と変更範囲

[Go release history](https://go.dev/doc/devel/release) は Go 1.27.3 と 1.26.10 の公開日を **2026-10-19** とし、`crypto/x509` の security fix を記載する。[Go team の同日告知](https://groups.google.com/g/golang-announce/c/66pojueaQek) は、特定の運用上の懸念により少数の intermediate certificates を含む chain を `Verify` の返却対象から除外すると説明し、CVE-2026-111943 と早急な更新推奨を示す。

- これは全ての root / intermediate を新しい trust store に交換する発表ではない
- CA の名称、侵害の原因、全利用者の影響範囲は、この告知から断定しない
- 今回の詳細な実装分析は **go1.27.3** に固定する。1.26.10 への修正提供は告知で確認したが、両系列の実装・実行結果が全て同じだとは主張しない
- pkg.go.dev の go1.27.3 ページには Published: 2026-10-20 と表示される。release history の公開日とページ上の日付を混同せず、その差の理由は未確認とする

取得日は2026-10-23 UTC。これらの版を将来にわたり「最新版」とは扱わない。

## 固定実装: certificate fingerprint ではなく SPKI hash

`git ls-remote` で go1.27.3 tag が commit `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` を指すことを照合した。以下は同 SHA の [verify.go](https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/crypto/x509/verify.go) の観察で、将来の内部実装の契約ではない。

1. `distrustedSPKIHashes` には **8件**の SHA-256 hash がある。判定入力は `RawSubjectPublicKeyInfo`、すなわち DER 形式の SubjectPublicKeyInfo（SPKI）であり、証明書全体の `Raw` ではない。Subject / Issuer の表示名や serial number の照合でもない
2. `containsDistrustedSPKIHash` は chain の全 certificate を走査する。発表の対象は intermediate だが、実装は「配列の中間位置だけ」を見る条件ではない
3. Go verifier は構築した candidate chain のうち該当するものを除外し、残った chain を返す。独立した有効 chain が残るケースまで必ず全拒否する、という実装ではない
4. 全候補がなくなると失敗する。`NoValidChains` はこの問題専用の理由ではなく、他の検証エラーが先に返る場合もある。エラー文字列だけから対象 hash の一致を逆算しない

したがって、証明書全体の fingerprint が変わっても SPKI が同じなら同じ照合対象になる、というのがこの実装から導ける判断である。cross-sign や証明書再発行を見つけた時に「certificate fingerprint が別だから影響なし」と判定しない。一方、SPKI hash が一覧と一致しないことだけでは、期限・hostname・usage・通常の chain 構築を含む他の検証の成功を保証できない。

hash 一覧をアプリへ手作業でコピーした独自 denylist を修正版の代替にはしない。固定sourceは調査の基準であり、将来の追加・変更の追跡責任を負わない一覧の複製は別の更新漏れを作る。

## platform verifier も別途確認する

[Certificate.Verify の API](https://pkg.go.dev/crypto/x509@go1.27.3#Certificate.Verify) は、`Roots=nil` の時に platform verifier を使い得ることと、検証の詳細が異なり得ることを示す。今回の固定実装では Go verifier 以外にも同じSPKI判定がある。

- [Windows の root_windows.go](https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/crypto/x509/root_windows.go): OS 検証で得た chain 群から該当 chain を除外する。全除外時は、既存の上位候補エラーがなければ `NoValidChains`、あればそのエラーを返す
- [Darwin の root_darwin.go](https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/crypto/x509/root_darwin.go): SecTrust から取り出した chain が該当する場合に `NoValidChains` で拒否する

つまり **「OS が受理したから Go 側の追加判定はない」ではない**。また、custom root pool を使う Go verifier にも前節の処理があるため、単に独自 CA bundle を設定した事実は今回の判定を回避した証拠にならない。逆に OS 更新や bundle 更新は OS / roots 側の結果を変え得るが、既存アプリに組み込まれた Go の検証コードが更新された証拠とは別である。

この節はsource確認であり、macOS / Windows / iOS の実機 handshake や各OSの trust policy 全体を試験した結果ではない。

## 通常の失効確認を保証する修正ではない

[go1.27.3 Verify の文書](https://pkg.go.dev/crypto/x509@go1.27.3#Certificate.Verify) には、失効確認を行わない旨の警告が引き続き存在する。限定された SPKI による distrust を追加した事実から、全証明書の CRL / OCSP の取得・鮮度・失効判断を保証する機能になったとは推論しない。platform verifier の詳細も環境依存であり、全OSで同じ失効 policy になるとは扱わない。

[tls.Config](https://pkg.go.dev/crypto/tls@go1.27.3#Config) の `InsecureSkipVerify` で通常の chain / hostname 検証を省くことは、接続失敗の安全な回避策ではない。独自検証がなければ中間者攻撃に弱くなる。custom callback や外部TLSライブラリを使うアプリは、修正済みの `x509.Verify` を本当に通るのか別に確認する。

## 配備完了を判断する運用案

以下は上記のAPI・実装から導く独自の運用案である。公式が特定の rollout 手順を要求している、あるいは本稿で利用者の本番環境を確認済みだという意味ではない。

### 1. 検証を実行するアプリを棚卸しする

outbound HTTPS client、CLI、agent、定期job、社内proxyへの接続、client certificate を検証する mTLS server などを対象にする。重要なのは通信のどちら側が peer の証明書を検証するかであり、**接続先serverだけを再ビルドしても、利用者側に残る古いclientの検証コードは更新されない**。相手側の提示chainを変える対応と、こちら側の verifier を更新する対応を区別する。

明示的な `RootCAs` / `ClientCAs`、custom callback、外部ライブラリ、proxyでのTLS終端を記録する。`crypto/tls` を使う全プロセスが常に peer certificate を検証するわけではない。mTLS の検証有無は `ClientAuth` も確認する。[TLS API](https://pkg.go.dev/crypto/tls@go1.27.3#Config)

### 2. shell の Go 版ではなく、対象バイナリの build を確認する

[go version の固定版ドキュメント](https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/alldocs.go) では、file引数なしは `go` command 自身の版、file指定時はその実行ファイルをbuildしたGo版を表示する。`go version -m /path/to/service` のように **対象fileを指定**し、利用可能な埋込module情報も確認する。引数なしの `go version` が修正版でも、古い service の修正完了とはならない。

稼働プロセス側では、既存の診断手段があれば [runtime/debug.ReadBuildInfo](https://pkg.go.dev/runtime/debug@go1.27.3#ReadBuildInfo) と `BuildInfo.GoVersion` を使える。これはrunning binaryに埋め込まれた情報で、module supportでbuildされたbinaryに可用性が限定される。取得失敗を「修正版」とみなさず、別のbuild証跡を要求する。

Go の標準ライブラリの修正を取り込むには、通常の Go アプリは修正済みtoolchainで再ビルドし、その成果物へ置き換える必要がある。host上のGoやCA storeだけの更新を、静的に組み込まれた検証コードの更新の代用にしない。vendorによるbackportや特殊なlink方法は、版文字列だけで決めず配布元の修正証跡を確認する。

### 3. build、配布、稼働を別々に照合する

- build記録: 使用toolchain、対象OS / architecture、アプリrevisionとartifact digestを残す。Dockerのbuild stageで使うGo版とruntime imageの更新を混同しない
- 配布記録: 修正版artifactを新しいimage / packageへ含め、固定digestで照合する。base imageだけ更新して、古いbuild済みbinaryをコピーし直した場合は未修正になり得る
- 稼働記録: 全replica、worker、定期job、手配布CLIを対象に、実際のrevision / digest / build版を確認する。disk上のbinaryが新しくても旧プロセスが稼働中なら完了にしない
- rollback候補: 古いbinaryへ戻せば同じ修正を失い得る。復旧手順は修正版の範囲で準備し、例外的な戻しは残るriskを明示して別判断にする

version metadataは証跡の一部であり、暗号学的な真正性証明や到達可能な脆弱性の判定そのものではない。改変されたbuildや配布取り違えに備え、artifact同一性と検証試験を組み合わせる。

## 検証計画: 成功・拒否・再開を混同しない

本番前に、承認されたfixtureと隔離環境で次を確認する。これは試験案であり、実施済みの結果ではない。

| ケース | 判定したいこと |
|---|---|
| 問題のない有効chain | 必要な接続を維持し、hostname / usage / roots が期待通りである |
| 対象SPKIを含み、他の条件は有効なchain | 対象chainが返らず、他に有効chainがなければ失敗する |
| 同じSPKIを含む別certificate | 証明書全体のfingerprint差を、distrust判定対象の差と誤認していない |
| 独立した有効な代替chain | 問題のないchainが構築可能かと、提示chainに対象certが含まれるかを区別する |
| custom roots / platform roots | 実際に利用する検証経路ごとの結果を確認する |
| fresh handshake / resumption | 既存接続の再利用やsession再開を、新しい証明書検証の実行証跡と取り違えない |

`VerifyPeerCertificate` は resumed connection では呼ばれず、`VerifyConnection` は resumption を含めて呼ばれるという [TLS API の差](https://pkg.go.dev/crypto/tls@go1.27.3#Config) がある。試験はまず非再開の新規handshakeを作り、`ConnectionState.DidResume` も記録する。callbackが呼ばれた事実だけで、custom codeが適切な証明書検証を行ったとは判定しない。継続接続・session ticket・cacheの管理は別途評価し、一律の本番無効化を本稿から要求しない。

失敗時は対象endpoint、binary identity、rootsの選択、実際のchain、error型と理由を突き合わせる。`NoValidChains` だけで incident の影響と断定せず、必要なら証明書発行側と検証側の変更を分離して調べる。chainの表示・hash計算だけでは署名やhostname検証の代わりにならない。

## 未確認事項・出典・鮮度

- Go 1.27.3 / 1.26.10 での実際の遮断対象chain、TLS handshake、resumption、各OSの実行試験は未実施。1.26.10のsource全体、後続版のhash変更、個別vendorのbackportも未検証
- 特定組織の証明書、利用中endpoint、影響する実行ファイル、CAの事故詳細、一般的な失効確認の成否は調査していない。ネットワークprobeの成功だけから非影響を宣言しない
- release history、公式告知、版固定API、固定verify.goの本文はnative webで開いた。固定Windows / Darwin / LICENSEはnative webがcache miss、版固定cmd/goページは取得失敗だったため、同じ40桁SHAのpublic raw HTTPから取得して読んだ。source catalogにはこの取得経路の差を残す
- Go Authorsの資料を独自に要約した。go.devの文章は [CC-BY-4.0](https://go.dev/copyright)、版固定Go source / APIは [BSD-3-Clause](https://raw.githubusercontent.com/golang/go/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/LICENSE)。Google Groups告知の独立したlicenseは未確認のためunknownとし、コード・hash一覧の転載やmoduleへの昇格は行わない
- 全sourceの取得日は2026-10-23 UTC。release_notesの30日TTLに合わせ、明示期限は2026-11-22。検索evalは本稿の発見可能性を検査するもので、CVE対策の実行検証ではない

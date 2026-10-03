---
{
  "id": "cosign-legacy-bundle-identity-key-boundary",
  "title": "Cosign legacy bundle: 修正版・identity検証・明示鍵と形式移行の境界",
  "kind": "knowledge",
  "technology": "supply-chain-security",
  "version": "GHSA-fx35-mq7g-6g98 (2026-08-06), patched Cosign v2.6.5 / v3.1.3; v3.1.3 implementation pinned to 11926fa5bbbbde47e88fc006b625a17769b743b2; Sigstore Bundle Format guide 0.3.2 (2025-01-14); verified 2026-10-03 UTC",
  "tags": ["research-domain:security", "supply-chain-security", "cosign", "sigstore", "legacy-bundle", "identity", "public-key", "format-detection", "migration"],
  "sources": [
    {"id": "cosign-legacy-bundle-advisory-20261003", "url": "https://github.com/sigstore/cosign/security/advisories/GHSA-fx35-mq7g-6g98", "type": "maintainer_article"},
    {"id": "cosign-313-legacy-bundle-release-20261003", "url": "https://github.com/sigstore/cosign/releases/tag/v3.1.3", "type": "release_notes"},
    {"id": "cosign-265-legacy-bundle-release-20261003", "url": "https://github.com/sigstore/cosign/releases/tag/v2.6.5", "type": "release_notes"},
    {"id": "cosign-legacy-bundle-fix-11926fa-20261003", "url": "https://github.com/sigstore/cosign/commit/11926fa5bbbbde47e88fc006b625a17769b743b2", "type": "github_repository_analysis"},
    {"id": "sigstore-bundle-format-032-20261003", "url": "https://docs.sigstore.dev/about/bundle/", "type": "official_docs"}
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# Cosign: bundleから検証者の信頼方針を選ばせない

## 問い・変更点・適用範囲

ダウンロードしたbinaryやblobの署名確認が成功しても、「許可した発行者の成果物」という判断が成立するか。2026年8月6日の [GHSA-fx35-mq7g-6g98][advisory] は、legacy bundleの読み方によって、この対応が崩れる事例を公表した。本稿は修正版への更新、検証時の信頼元、保存済みbundleの移行を一つのリリースゲートとして整理する。一般的なGitHub attestationやSLSA predicateの解説は既存文書に譲る。

告知の対象は `cosign verify-blob` と `cosign verify-blob-attestation` が **legacy JSON bundle（LocalSignedPayload）** を読む経路。v2系列の `<=2.6.4`、v3系列の `<=3.1.2` がaffectedで、修正版は **v2.6.5 / v3.1.3**。`cosign verify` / `verify-attestation` によるOCI検証と、modern Sigstore protobuf bundleの検証は、この問題の非該当範囲として明記される。「Cosignを使う全成果物が侵害済み」とは結論できない。[v3.1.3 release][release3] と [v2.6.5 backport][release2] でも修正を確認した。

以下の実装観察はv3.1.3のcommit `11926fa5bbbbde47e88fc006b625a17769b743b2` に限定する。v2 backport内部の全分岐や、各社のpatched buildを同じコードと仮定しない。

## 一次資料で確認した検証境界

### 1. 署名が数学的に有効でも、許可したidentityとは限らない

告知によると、旧経路はlegacyの `cert` をX.509として読めない場合にraw public keyとして再解釈し、検証器を設定していた。このため、証明書chainと `CheckCertificatePolicy` の確認を通らず、指定した `--certificate-identity` / `--certificate-oidc-issuer` が強制されない場合があった。明示的な `--key` も、bundle由来の鍵で置き換わる問題があった。[advisory]

これはhashの強さや署名algorithmを変更すれば解消する問題ではない。**どの鍵を信頼するかを、検証対象自身の入力へ委ねた境界の不具合**である。この整理は本稿の設計上の解釈であり、暗号primitiveの破綻を示すものではない。

### 2. 修正版は「別の型なら通す」というfallbackを止める

[verify_blob.go][blob-code] と [verify_blob_attestation.go][attest-code] の修正版では、非空の `cert` は証明書として読み、parse失敗を返す。明示鍵がある場合でも、不正な `cert` を無視して先へ進む実装ではない。証明書も明示検証器もなければ失敗する。別途指定した証明書とbundleの証明書が異なる場合も拒否する。

同じcommitの [verify_blob_test.go][blob-tests] には、鍵が `cert` に入ったlegacy bundleの拒否と、`cert` が空で明示的な `KeyRef` が与えられた経路の成功期待がある。[修正差分][fix] は生成側も更新し、証明書がないときにraw keyを `cert` へ保存しなくした。したがって、修正版が昔のbundleを拒否すること自体は、攻撃の確定証拠ではない。過去の正規ツールが生成したkey-based bundleとの互換性問題も切り分ける必要がある。

これは上流コードとテストの**読取り結果**であり、本調査でCosignや上流テストを実行した結果ではない。

### 3. v3の標準形式の既定値は、legacyの受入れ禁止ではない

[v2.6.5の説明][release2] は、v3.1.xが新旧両形式を検証し、形式を自動判定することを明記している。署名生成時の既定形式と、検証時の受入れ形式は別の設定である。

固定commitでは、blobの両検証コマンドが `NewBundleFormat` と `checkNewBundle` の結果をANDで結ぶ。[checkNewBundle][bundle-code] は標準bundleとしてロードできるかをbooleanにする。従ってこの版で **`--new-bundle-format=true` を付けるだけでは、legacyを必ず拒否するstrict gateにならない**。modern形式のparseが成功しない入力がlegacy経路へ進み得る。ただしこれは修正版にも互換分岐があるという観察であり、修正された鍵fallbackが復活するという意味ではない。

「署名側がv3だから安全」「`.sigstore.json` という拡張子だからmodern」「flagを追加したから更新不要」という三つの判定を避ける。

### 4. 標準bundleも信頼元を内包して自己承認するものではない

[Sigstore Bundle Format guide][bundle-guide] は表示版0.3.2、更新日2025-01-14で、protobuf schema由来の **JSON serialization** を説明している。legacy JSONとmodern protobufは、単純なtext/binaryの区別ではない。

guideではX.509証明書と `publicKey.hint` を別に扱い、後者はout-of-bandで配布した鍵を識別する手掛かりで、鍵自体を格納しない。また `messageSignature.messageDigest` はbundleに存在するだけでは足りず、検証時にartifactから計算するか外部から与える。形式識別、外部の信頼元、artifactとの対応を別々に確認する必要がある。

## 移行・運用の判断案（本リポジトリ独自の提案）

### A. 先に検証環境を更新し、適用範囲を記録する

1. CI runner、release承認job、配布用container、ローカル配布scriptが実際に呼ぶCosignの版・パス・binary digestを集める。生成側の版だけで対応済みとしない。
2. 上記修正版または、当該修正を含むことを別途確認した配布版へ検証側を更新する。v2からv3へのmajor移行が難しい場合はv2.6.5 backportを評価する。未調査の将来版に無条件の安全保証を与えない。
3. 各呼出しについて「command、受入れ形式、期待したidentity/issuerまたは鍵、結果を使用した成果物」を記録する。入力形式や実行引数が残っていなければ **影響不明** とし、非該当へ寄せない。
4. 旧版による `Verified OK` だけが残る重要な配布物は、当時のartifactとbundleを保存したまま修正版・独立した信頼方針で再検証する。再検証成功は現在の方針への適合であり、過去に使われた全入力が無害だった証明ではない。

### B. identity方式と鍵方式を業務要件から選ぶ

- **identity方式**: 許可したidentityとOIDC issuerを事前設定する。成果物に含まれる値をそのまま期待値として採用しない。正しい証明書で署名されていても許可していない発行者なら拒否する。
- **鍵方式**: 信頼済みの配布経路で得た鍵や管理下の鍵識別子を固定する。bundleから抜いたraw keyをそのまま `--key` に渡す「修復」は、元の信頼問題をwrapper側へ移す。
- 二つの方式を障害時に自動切替しない。固定commitでは明示鍵とcertificate identity指定は相互排他であるため、両flagを足すことを二重検証とは呼べない。二つの保証が本当に必要なら、対応する構成と独立した検証手順を別途設計する。[blob-code]

### C. 形式変換を検証済みの印へ置き換えない

告知は `cosign bundle create` による移行を案内している。しかし、本稿の承認条件は「変換commandが成功した」ではなく、**変換後も同じartifactと期待した信頼元で検証が成功したこと**に置く。変換前後のdigest、利用した外部鍵またはidentity方針、実行版、失敗理由を残す。型の変更だけで欠落していた発行者の証明が生成されたと解釈しない。

legacyを廃止する場合は、schema対応のloaderによる形式確認と検証失敗時の停止をアプリ側の明示policyにする。`mediaType` 文字列だけの一致で承認せず、標準形式としての構造と対応版も検査する。これは上流CLIの既定動作についての主張ではない。移行中に古いbundleが失敗しても、旧binaryへ自動rollbackしたり、期待するidentityを削除して成功させたりしない。

### D. 検証から使用までを同じ入力に結び付ける

artifactとbundleは一旦管理下の保存場所へ取り込み、検証後に同じdigestのartifactだけを昇格させる。検証とdeployの間に同じURLを再取得して別の内容へ置き換わる経路を作らない。監査記録には単一の「成功」だけでなく、検証版、format、信頼方針の版、artifact digest、承認時刻を残す。これにより後日のadvisoryで影響対象を絞れる。

## 管理下で行う受入れ試験案（未実行）

本番serviceや公開Rekorへ試験用入力を送らず、隔離したfixtureとテスト用信頼素材を使う。wrapperの終了codeだけでなく、成果物が昇格していないことも確認する。

- **legacy cert型違反**: `cert` が証明書ではないfixtureは修正版で失敗する。明示鍵を併用してもparse失敗を成功へ変えない。
- **空certと明示鍵**: 許可した鍵方式で `cert` が空のfixtureを正常系にし、別の鍵や改変artifactは拒否する。legacy全部を不正とみなす試験にはしない。
- **keylessの発行者違い**: 正常な証明書・署名でも期待したidentityまたはissuerが異なる場合は拒否する。
- **自動判定とstrict gate**: modern限定のwrapperへlegacyを与えた場合、`--new-bundle-format` の有無に頼らず入口で拒否する。CLI単体の互換受付とwrapper方針を別々に試す。
- **変換後の実物照合**: 標準bundleへの変換後に別のartifactを与えて拒否されることを確かめる。bundle内digestだけで承認しない。
- **証跡不足**: 過去の実行版やformatが不明な記録を「影響なし」に分類しない。

## 版・provenance・未確認事項

一次資料の取得日はすべて2026-10-03 UTC。新規性は2026-08-06の具体的修正と未収録の運用上の問いにあり、数日前の変更と装わない。release notesの30日TTLに合わせて再確認期限を2026-11-02とする。

- Cosign sourceは上記40桁commitへ固定し、[LICENSE][cosign-license] と読んだGoファイルのApache-2.0 headerを確認した。コードやfixtureは転載せず、制御の流れを独自に要約した。
- bundle guideは公式docsの表示版と日付を記録し、[sigstore/docs LICENSE][docs-license] のMIT表記を確認した。advisoryとrelease本文の独立したライセンスは確認できず、catalogではunknownとして原文転載を避けた。
- v2.6.5での各flagの内部実装、bundle変換commandの全入力互換性、SDK埋込み先の到達可能性、vendorのbackport、実際の被害や過去の配布物は未確認。利用しているだけで影響を断定しない。
- 本調査ではCosign実行、署名生成、上流unit test、live registry/Rekor通信による再現試験を行っていない。検索evalは文書の発見を検査するもので、暗号・認可・互換性の適合試験ではない。

[advisory]: https://github.com/sigstore/cosign/security/advisories/GHSA-fx35-mq7g-6g98
[release3]: https://github.com/sigstore/cosign/releases/tag/v3.1.3
[release2]: https://github.com/sigstore/cosign/releases/tag/v2.6.5
[fix]: https://github.com/sigstore/cosign/commit/11926fa5bbbbde47e88fc006b625a17769b743b2
[blob-code]: https://github.com/sigstore/cosign/blob/11926fa5bbbbde47e88fc006b625a17769b743b2/cmd/cosign/cli/verify/verify_blob.go#L81-L213
[attest-code]: https://github.com/sigstore/cosign/blob/11926fa5bbbbde47e88fc006b625a17769b743b2/cmd/cosign/cli/verify/verify_blob_attestation.go#L301-L325
[blob-tests]: https://github.com/sigstore/cosign/blob/11926fa5bbbbde47e88fc006b625a17769b743b2/cmd/cosign/cli/verify/verify_blob_test.go#L291-L350
[bundle-code]: https://github.com/sigstore/cosign/blob/11926fa5bbbbde47e88fc006b625a17769b743b2/cmd/cosign/cli/verify/verify_bundle.go#L41-L44
[bundle-guide]: https://docs.sigstore.dev/about/bundle/
[cosign-license]: https://github.com/sigstore/cosign/blob/11926fa5bbbbde47e88fc006b625a17769b743b2/LICENSE
[docs-license]: https://github.com/sigstore/docs/blob/main/LICENSE

---
{
  "id": "security-sri-policy-admission-digest-boundary",
  "title": "SRIの検証境界: Integrity-Policyの要求許可とReport-Onlyでも残るdigest照合",
  "kind": "knowledge",
  "technology": "security",
  "version": "SRI 2 Working Draft 2026-03-20; Fetch Living Standard Last Updated 2026-09-21",
  "tags": [
    "research-domain:security",
    "sri",
    "integrity-policy",
    "report-only",
    "cors",
    "digest",
    "strongest-hash"
  ],
  "sources": [
    {
      "id": "w3c-sri2-wd20260320-policy-digest-20261003",
      "url": "https://www.w3.org/TR/2026/WD-sri-2-20260320/",
      "type": "official_docs"
    },
    {
      "id": "whatwg-fetch-ls20260921-sri-handover-20261003",
      "url": "https://fetch.spec.whatwg.org/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/security.json"
  ]
}
---

# SRI: 要求を許可しても応答のdigest検証は終わっていない

## 問いと調査の位置付け

Integrity-Policy-Report-Onlyを使えば、不一致のintegrity値が付いたscriptも動くのか。強いhashを追加したあとに古いhashが合っていればfallbackするのか。配信変更と監視設定を同時に行うと混同しやすい、要求許可と応答検証の境界を扱う。

2026-10-03 UTCに[現行SRI TR](https://www.w3.org/TR/sri-2/)と[固定版][sri]を照合し、2026-03-20 Working Draftを確認した。[Fetch][fetch]の表示更新日は2026-09-21。これは新機能の発表を装う記事ではなく、security領域の未収録の実務上の隙間を補う調査である。Working DraftはRecommendationではなく、各規則の導入日もここから推測しない。

既存のTrusted Types文書はDOM sinkの型変換、npm integrity文書はpackage取得時の検証を扱う。本稿はブラウザが外部resourceを読み込む場面に限定する。事前のIntegrity-Policy、SRI CORS検索は該当なしだった。

## 一次資料で確認した境界

[SRI §3.8.2][sri]では、metadataの解析結果が空でなく、modeがcorsまたはsame-originならpolicy判定を通る。対象destinationに対するenforceは不足を遮断し、Report-Onlyは報告する。ローカルURLなどの例外もあり、全通信への一律の検査ではない。sourcesのinlineはmetadataの供給元を指す。

これは応答内容の合格判定ではない。[SRI §3.3.3〜3.3.4][sri]のdigest照合では、対応する候補の最強algorithmだけを選び、その集合のどれかと一致する必要がある。弱い候補へのfallbackではない。同じ強さの複数候補は複数の内容を許容する。未知のalgorithmしかなく解析集合が空なら、その照合手順単体はtrueを返すため、属性が非空という確認だけでは足りない。

[Fetch §4.1 Main fetch][fetch]は、非空integrity metadataがある場合、bodyを読み終えてから照合し、不一致などをnetwork errorとして扱う。Report-Onlyで不足要求を止めないことと、付与済みdigestの検証を無効にすることは別である。[§4.10 CORS check][fetch]にも別の許可判定があり、digestが正しいだけでcross-origin応答の利用許可を得ることはできない。

## 独自の導入・配信レビュー案

以下は仕様の必須手順ではなく、運用を組み立てるための提案である。

### 三つの判定を記録する

1. 要求段階: destination、mode、解析できるmetadata、実際に配送されたpolicyを記録する
2. 取得段階: HTTP応答、CORS、通信障害を切り分ける
3. 内容段階: 配布したbytesと期待digestが対応するかを確認する

policy違反件数を単独の健全性指標にしない。ユーザーの操作到達率、scriptのerror、ロード成否、既知の違反を使った監視の到達確認を併用する。違反ゼロでも、対象画面を試していない可能性と収集障害を残して判定する。

### HTMLとassetを一つの配布単位で管理する

build成果物のmanifestに、immutableなasset URL、algorithm、digest、参照するHTMLのbuild IDを対応付ける。CDNによる変換の有無や古いHTMLからの参照を、deploy前の受け入れ項目に入れる。hashを計算する時点と最終配信bytesがずれていないかを点検する。

移行期間に旧版と新版を同時に許容するなら、何を許容し続けるのかをレビューする。便宜的にhash候補を増やし続けず、旧HTMLの寿命とrollback期間を根拠に除去時期を決める。digestだけを戻したり、同じURLの中身だけを差し替えたりする復旧を標準手順にしない。

### 監視から遮断への移行条件を具体化する

Report-Onlyの採用時も、付与済みintegrityの正当性はdeployの合格条件に含める。監視ヘッダーだけを戻して障害が解消すると決めつけず、失敗した段階に合わせて修復する。

対象browserの版とdestinationごとの実装状況を別途確認する。仕様にscriptとstyleが記載されていることから、配布先の全browserが同じ範囲を実装しているとは結論しない。主要画面、遅延ロード、エラー画面、旧build、rollbackを試験対象に含める。

## 未実行の境界試験案

次は設計レビュー用の試験案であり、実行成功の記録ではない。

- Report-Onlyのみでmetadataなしと正しいmetadataを比較し、要求・実行・reportを別々に観測する
- Report-Onlyでも認識されるalgorithmの誤ったdigestを与え、内容検証が残ることを確認する
- 強いdigestだけを誤らせ、弱いdigest一致が救済になるという誤解を検出する
- 同一algorithmの二つのdigestを与え、許容するassetの集合をレビューする
- 未知algorithmだけの入力について、policyなしとenforceありを分けて観測する
- cross-origin配信先のCORS許可を外し、digest修正だけで解消したと判断しない
- header、HTML、assetのいずれかだけが旧buildの組合せを再現し、復旧対象を確認する

## 適用限界と出典

ブラウザ実機、WPT、CDN配信、Reporting API収集の実行試験は未実行。最新browserの対応開始版、module graph全体、service worker経路の同等性は確認していない。ここでの検索evalは発見性の検証であり、runtimeの安全性を証明しない。

W3C原文とWHATWG Fetchを基にした独自の日本語整理で、コードや長文の転載はない。SRIのcopyrightリンクで[W3C Software and Document License 2023](https://www.w3.org/copyright/software-license-2023/)を確認した。FetchはAnne van Kesteren / WHATWG、文書部分CC-BY-4.0。取得時の内容は[commit snapshot](https://fetch.spec.whatwg.org/commit-snapshots/357bd98924d94b81fbe8608192a2ee1f123b82f4/)でも識別できるが、実装判断では現行Living Standardを再確認する。

[sri]: https://www.w3.org/TR/2026/WD-sri-2-20260320/
[fetch]: https://fetch.spec.whatwg.org/

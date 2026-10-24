---
{
  "id": "security-csp-strict-dynamic-loader-delegation-boundary",
  "title": "CSP strict-dynamic: script loader への委任と nonce・URL・cache の信頼境界",
  "kind": "knowledge",
  "technology": "security",
  "version": "Content Security Policy Level 3 W3C Working Draft 2026-09-16; web.dev strict CSP article updated 2024-09-13; verified 2026-10-24 UTC; browser runtime untested",
  "tags": ["research-domain:security", "csp", "strict-dynamic", "nonce", "script-loader", "parser-inserted", "base-uri", "cache"],
  "sources": [
    {"id": "w3c-csp3-wd20260916-loader-20261024", "url": "https://www.w3.org/TR/2026/WD-CSP3-20260916/", "type": "official_docs"},
    {"id": "webdev-strict-csp-loader-20261024", "url": "https://web.dev/articles/strict-csp", "type": "maintainer_article"}
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2027-01-22",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# CSP strict-dynamic: loader を許可しても、その入力を信頼したことにはならない

## 問いと採用判断

`script-src` に nonce と `'strict-dynamic'` を加えたあとも、同じ directive に書いた host allowlist が子 script の宛先を制限するのか。loader のコードを固定して hash で許可すれば、実行時に渡す URL まで安全と言えるのか。

本稿の判断は、**CSP による実行許可、loader が選ぶ宛先、入力値を選べる主体を別々に審査する**、である。nonce は URL の検証結果ではなく、固定された loader の hash も可変データの安全性を証明しない。移行の完了条件を「正規の script が動く」「違反が消えた」だけにしない。

2026-10-24 UTC に[現行 TR](https://www.w3.org/TR/CSP3/)と[固定版][csp]を照合し、2026-09-16 Working Draft を確認した。もう一つの一次資料は Lukas Weichselbaum による [web.dev の導入記事][webdev]で、表示更新日は 2024-09-13。草案を Recommendation と呼ばず、この日付を各規則の導入日とも扱わない。新機能紹介ではなく、既存の仕組みの実務上の未収録境界を補う。

事前の KB 検索で `strict-dynamic` と `CSP nonce` は該当なし。knowledge・patterns・modules 全体も照合した。[Trusted Types](trusted-types-csp-report-only-default-policy.md) は DOM sink の型と変換、[SRI](sri-policy-admission-digest-boundary.md) は要求条件と取得した bytes の digest を扱う。本稿は、それらの詳細を繰り返さず、script を選んで読み込む権限の委任を対象にする。

## 確認した契約と、読み違えやすい順序

### URL リストより前に決まる script の許可

[CSP3 §6.7.1.1–6.7.1.2][csp-checks] は、script-like request の nonce 一致、integrity metadata 一致を先に判定する。両方とも一致しない場合、`strict-dynamic` があれば parser-inserted を拒否し、それ以外を許可する。この分岐で host の照合には進まない。外部 script の integrity metadata の一致は単なる属性の存在ではない。

したがって本稿の「委任」は理解のための説明であり、ブラウザが URL の出所を追跡して安全性を認証するという意味ではない。また、「parser-inserted は常に拒否」ではなく、先行する nonce 等の一致を除いた場合の判定である。これは当該 directive の判断であり、他の policy や読み込み条件も含めた実行成功の保証ではない。

[§8.2][csp-dynamic] は非規範の説明で、`createElement` と `document.write` の違い、および動的な script の場所へ未信頼データを渡す危険を示す。[§8.5][csp-strict] も非規範であり、配備容易性を説明する一方、可能なら `strict-dynamic` を避けるよう注意している。無条件の有効化を仕様要件にしない。

### nonce と cache は別々に正しくする

[§7.1][csp-nonce] は policy を送信するたびの一意な nonce を MUST とし、符号化前 128 bit 以上と暗号学的乱数生成を SHOULD とする。[§7.3][csp-base] は、nonce 付き script の相対 URL でも base URL の変更が問題になると説明する。

[web.dev][webdev] は、応答ごとに生成できるサーバー描画には nonce、静的HTML・cache が必要なページには hash を選ぶ方針を示す。同記事は、nonce を付けた script の本文・src への注入や動的 loader の引数を、なお残る危険として挙げる。CSP を入力処理の代替にしない。

## loader を採用するための設計ゲート（独自提案）

以下は仕様の必須手順ではなく、本稿のレビュー案である。CSP の許可集合と、アプリケーションが業務上読み込みたい集合を一致させるための実務上の判断を追加する。

### 1. 何を誰が選べるかを小さくする

loader ごとに、所有者、呼び出し元、引数、許可する script、選択元の信頼度、再委任する loader を記録する。script のホスト名だけでなく、誰が引数を書き換えられるかをレビュー対象にする。

可能なら任意 URL を受け取る API を廃し、用途 ID からビルド管理下の固定マップを引く。ユーザーが選ぶ値は `chart` や `editor` のような許可済み用途だけとし、不明な値はロードしない。マップへ入れる URL と配布物の変更には通常のコードレビューを要求する。これは CSP の host allowlist に期待するのではなく、アプリ側の実行権限を限定する設計である。固定マップ自体は配信先の侵害や応答 bytes の改変を検出しないため、内容検証の審査は別に残す。

外部 URL の自由入力が本当に必要なら、その要件自体を別途審査する。DOM 属性、query、保存済み設定、外部 API の返値を「データだから安全」として src へ流さない。HTML のエスケープを通過したことと、その URL のコードを実行してよいことは別の判断にする。

固定マップでも、相対URLを使う場合は解決基準を明示する。意図しない base の変更を許容しない設計として、不要なページでは `base-uri 'none'` を候補にし、必要なページでは許容範囲と実際の解決先を試験する。文字列が定数であることだけを安全性の合格理由にしない。

### 2. 委任が必要な範囲だけを残す

動的ロードがないページまで同じ loader を配布しない。必要な初期 script を管理でき、追加の委任なしで要件を満たせるなら、まずその案を比較する。多数の third-party script のために委任する場合は、二段目以降の読み込み先と変更主体も棚卸しする。

`strict-dynamic` を置いた同じ directive に `https:` や CDN の host を足しても、それを対応環境での子 script 制限としてレビューしない。旧環境向け fallback の採用判断は対象 browser と期待する制約を明記した別項目にする。文書上の機能対応と、利用者の実環境の試験結果を混ぜない。

### 3. nonce の配送方式を cache 設計から決める

| 配信形態 | 本稿の設計ゲート | 却下する近道 |
|---|---|---|
| 応答ごとに HTML を描画 | 同一応答の header と許可対象 script に同じ新規 nonce を反映し、二つの応答では値が異なることを試す | 起動時だけ生成、全利用者で固定、日次更新 |
| 共有 cache が完全な HTML を返す | nonce を含む完成応答の使い回しを避ける。静的 loader の hash 方式、または最終配信時に header と本文を一貫して生成する方式を比較 | cache hit でも値が一致するので合格とする |
| 静的HTML と hash 済み loader | loader 本文と CSP を同じ build から生成し、実行時の入力制限は別に試す | hash が合うので可変 URL も承認済みとする |
| CDN が header だけを書き換える | 実際に届いた本文との対応を確認し、生成責務を一つに寄せる | header の nonce だけ毎回変え、古い本文をそのまま返す |

同じ応答内で nonce を複数の許可済み script に共有することと、別の policy 送信で再利用することを区別する。一括の後処理で発見したすべての script に nonce を付ける設計は採用しない。信頼対象をテンプレートや build の段階で特定し、未信頼の HTML に実行許可を付け足さない。

## レビュー用 fixture と移行条件（未実行）

以下は管理下のページと無害な script で行う試験案である。外部 classic script を対象に、当該有効 directive が `strict-dynamic` を含む enforce policy を基本条件とする。表の「許可」はその directive の判定だけで、ブラウザ実行の成功とは区別する。

| fixture | 観測・判定したい境界 |
|---|---|
| parser-inserted、正しい nonce、host リスト外 | nonce の先行一致を確認。「リスト外だから CSP が拒否する」という誤った期待を検出 |
| parser-inserted、nonce と integrity の一致なし、host リスト内 | host の掲載だけでは救済されないことを確認 |
| 実行を許可した loader が createElement で追加、子に nonce なし、host リスト外 | 非 parser-inserted 経路を確認。子へ nonce を複製したから通った、という誤判定を除く |
| 同じ loader の document.write 経路、nonce と integrity の一致なし | parser-inserted の違いを確認。動的な JavaScript 呼び出しをすべて同一に扱わない |
| loader の入力を許可外の用途 ID にする | CSP だけに頼らず、アプリが DOM 挿入前に拒否することを確認 |
| nonce 付き相対URLと、意図しない base 変更 | base-uri の制約と最終解決先を観測。元の文字列が定数という理由で省略しない |
| nonce 方式で cache miss と cache hit、別の二応答 | 応答内の対応と応答間の非再利用を別条件で確認 |
| hash 済み loader は不変、runtime 設定だけを変更 | コードの同一性と入力の許可を別々に判定 |
| 新 build、旧 HTML、rollback の組合せ | header・HTML・loader の対応と復旧結果を記録 |

Report-Only はまず正規機能への影響候補を観測するために使い、限定した enforce 試験へ進める。ただし本件で最も重要なのは、**許可済み経路からの望まないロードは違反件数だけでは見つけられない**点である。fixture の許可外用途がアプリ側で止まったことを、CSP 違反ゼロとは別の合格条件にする。

受入れ記録には browser 版、build、最終 CSP、挿入経路、想定した用途、実際のロード成否を残す。nonce 値や入力全文を通常ログへ追加することを前提にしない。障害の復旧は対応する build と policy の組で行い、`unsafe-inline` の追加や無条件の nonce 付与を標準の修復手順にしない。

## 限界・未確認事項・出典

ブラウザ実機、WPT、HTTP 配信、CDN cache、Reporting API の runtime 試験は未実行。browser の対応開始版・最新 patch・polyfill の同等性は主張しない。module graph、worker、import map、個別 framework の loader、安全な TrustedScriptURL policy の実装は今回の検証範囲外である。

複数 policy や `script-src-elem` による実効 directive の選択は実配信で確認する。本稿の単一 directive の判定を、CORS、SRI、取得失敗などを含む全体の許可に広げない。検索 eval は文書の発見性を確認するもので、XSS 耐性や fixture の実行結果を証明しない。

一次資料の事実を短く整理し、設計ゲート・配信判断・fixture は独自に作成した。コード、攻撃 payload、長文の転載はない。

- W3C, Content Security Policy Level 3, Working Draft 2026-09-16。Copyright © 2026 World Wide Web Consortium。本文の license リンクと [W3C Software and Document License 2023](https://www.w3.org/copyright/software-license-2023/) を確認した。
- Lukas Weichselbaum, “Mitigate cross-site scripting (XSS) with a strict Content Security Policy (CSP)”, web.dev, updated 2024-09-13。本文は [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)、コード例は [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0) と footer に明記されている。本稿は原文の翻訳転載ではなく、独自の日本語要約と設計提案である。

[csp]: https://www.w3.org/TR/2026/WD-CSP3-20260916/
[csp-checks]: https://www.w3.org/TR/2026/WD-CSP3-20260916/#script-directive-checks
[csp-dynamic]: https://www.w3.org/TR/2026/WD-CSP3-20260916/#strict-dynamic-usage
[csp-strict]: https://www.w3.org/TR/2026/WD-CSP3-20260916/#strict-csp
[csp-nonce]: https://www.w3.org/TR/2026/WD-CSP3-20260916/#security-nonces
[csp-base]: https://www.w3.org/TR/2026/WD-CSP3-20260916/#nonce-retargeting
[webdev]: https://web.dev/articles/strict-csp

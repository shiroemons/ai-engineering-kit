---
{
  "id": "ruby-regexp-compile-timeout-patch-boundary",
  "title": "Ruby Regexp: コンパイル修正と timeout・信頼境界・系列別patchの判定",
  "kind": "knowledge",
  "technology": "ruby",
  "version": "CRuby 4.0.9 / 3.4.12 / 3.3.13 (2026-10-23公開); Ruby 4.0 RDoc; GitLab CE/EE 2026-10-21 critical patch。2026-10-23 UTC確認、runtime試験未実行",
  "tags": [
    "research-domain:backend",
    "ruby",
    "Regexp",
    "Onigmo",
    "compile",
    "timeout",
    "patch-version",
    "untrusted-pattern",
    "ReDoS",
    "GitLab"
  ],
  "sources": [
    {
      "id": "ruby-regexp-fixes-advisory-20261023",
      "url": "https://www.ruby-lang.org/en/news/2026/10/23/regexp-fixes-for-gitlab-cves/",
      "type": "release_notes"
    },
    {
      "id": "ruby-regexp-release-4-0-9-20261023",
      "url": "https://www.ruby-lang.org/en/news/2026/10/23/ruby-4-0-9-released/",
      "type": "release_notes"
    },
    {
      "id": "ruby-regexp-release-3-4-12-20261023",
      "url": "https://www.ruby-lang.org/en/news/2026/10/23/ruby-3-4-12-released/",
      "type": "release_notes"
    },
    {
      "id": "ruby-regexp-release-3-3-13-20261023",
      "url": "https://www.ruby-lang.org/en/news/2026/10/23/ruby-3-3-13-released/",
      "type": "release_notes"
    },
    {
      "id": "ruby-regexp-api-4-0-20261023",
      "url": "https://docs.ruby-lang.org/en/4.0/Regexp.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-regexp-gitlab-critical-19-5-1-20261023",
      "url": "https://docs.gitlab.com/releases/patches/patch-release-gitlab-19-5-1-released/",
      "type": "release_notes"
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

# Ruby Regexp: コンパイル修正と timeout・信頼境界・系列別patchの判定

## 問いと採用判断

利用者が指定する正規表現を `Regexp.new` で受け付ける機能に、照合のtimeoutと入力長の上限がある。これだけでRubyの正規表現エンジンに渡す入力を安全と判断してよいか。Ruby 4.0.8へ更新済みなら、4.0.9も必要か。

**照合時間の制御と、未信頼パターンをコンパイルする際のメモリ安全性は別の境界である。今回の修正群を揃えるupstream版は4.0.9 / 3.4.12 / 3.3.13。4.0.8を全修正済みと扱わない。** 任意のパターンを受け入れる設計そのものの安全性まで、この更新で保証されたわけではない。

2026-10-23の[Ruby公式告知](https://www.ruby-lang.org/en/news/2026/10/23/regexp-fixes-for-gitlab-cves/)を起点に、公開済みreleaseとAPI文書を照合した。以下では、公式資料の確認事実と、運用へ適用する独自の設計判断を分ける。Ruby runtime試験は未実行であり、攻撃パターンの生成・実行も行っていない。

既存のRuby文書はSocket接続、fork hook、SIGINT、Ractor、Fiberを扱う。本稿はその再説明ではない。追加前にindexを再生成し、knowledge・patterns・modulesの検索で `Onigmo` と `regexp compilation` は該当なし、`Regexp` は無関係なRails動的route文書のみだった。source catalogとevalもCVE識別子で検索し、同一トピックがないことを確認した。

## 1. 公開日・系列・修正群を切り分ける

[Ruby告知](https://www.ruby-lang.org/en/news/2026/10/23/regexp-fixes-for-gitlab-cves/)は2026-10-23 03:11:22 UTC公開。表は、その告知が示す各系列での最初の収録版であり、独自に全旧版を再現試験した結果ではない。

| Ruby側の修正対象 | 関連するGitLab識別子 | 4.0系列 | 3.4系列 | 3.3系列 |
|---|---|---|---|---|
| パターン解析失敗時のdouble free | CVE-2026-89078 | 4.0.0 | 3.4.12 | 3.3.13 |
| コンパイル済みprogramのsize overflow | CVE-2026-93577 / CVE-2026-97229 | 4.0.8 | 3.4.12 | 3.3.13 |
| repeat関連の内部ID overflow | CVE-2026-110264 | 4.0.9 | 3.4.12 | 3.3.13 |

[4.0.9 release](https://www.ruby-lang.org/en/news/2026/10/23/ruby-4-0-9-released/)は最後の修正を、[3.4.12 release](https://www.ruby-lang.org/en/news/2026/10/23/ruby-3-4-12-released/)は4件に関連する修正を収録すると明記する。いずれも2026-10-23公開で、予告やmasterへのmergeだけではない。[3.3.13 release](https://www.ruby-lang.org/en/news/2026/10/23/ruby-3-3-13-released/)も同日公開。3.3はsecurity maintenance段階だが、このRegexp修正を例外的に含む。同時収録のresolv更新は別の修正であり、両者を一つのCVEとして集計しない。

独自の運用判断として、版比較は同じ系列内で行う。4.0.9・3.4.12・3.3.13という番号を別実装へ機械適用せず、distributionや製品が独自backportした場合は配布元のpatch証拠を残す。RDocの「4.0」表示はAPIの系列であり、稼働binaryに4.0.9の修正が入った証明ではない。

## 2. パターンと対象文字列のどちらを外部が決めるか

公式告知が問題にしているのは、攻撃者の制御する文字列から正規表現をコンパイルする経路である。`Regexp.new`、`Regexp.compile`、動的な正規表現literalへの補間など、パターンを組み立てる入口を区別する。開発者が書いた固定パターンのみを使うアプリは今回の不具合の対象外と説明されている。ただし、それは外部の対象文字列を照合する際のReDoSや資源消費まで不存在とする意味ではない。

独自の調査用分類は次のとおり。API名のgrepだけで「外部入力なし」と結論しない。

| 入力契約 | 確認する境界 | 設計判断 |
|---|---|---|
| 固定パターンで対象文字列を調べる | 外部値がpattern本体へ混入していないか | 今回のコンパイル問題と照合時の負荷を分けて評価する |
| 利用者が検索文字列を指定する | 要求はliteral検索か、regex構文か | literalで足りるなら文字列APIで実装し、不要なコンパイルをなくす |
| 利用者がregex構文を指定する | 入力者、保存先、実際にコンパイルするworkerとruntime | 更新・機能制限・資源管理を別々に設計する |
| 設定やファイルをライブラリが読む | 表面の項目名ではなく、後段の補間・変換先 | HTTP parameter以外の入口もデータフローで追う |

[RDocのescape](https://docs.ruby-lang.org/en/4.0/Regexp.html#method-c-escape)は、メタ文字をliteralとして扱うためのAPIである。一方、正規表現機能の要件を残したまま全入力をescapeすると意味が変わる。literalへの仕様変更とruntime修正を混同しない。どの合成APIもraw `Regexp.new`と同じ解釈をする、と決めつけず、引数の型とそのAPIの契約を確認する。

## 3. timeoutと文字数制限はコンパイル修正の代わりではない

公式告知は `Regexp.timeout` が今回の問題を防がず、短いパターンでも到達する不具合があるため文字数制限も十分ではない、と明示している。従って「timeoutを設定した」「patternを数十文字に制限した」というテスト結果から、コンパイル関連のメモリ破壊を防いだとは判断できない。

[Ruby 4.0 API](https://docs.ruby-lang.org/en/4.0/Regexp.html#method-c-timeout)の確認事実は、照合の設定に限って読む。

- `Regexp.timeout` はprocess-globalな既定の照合時間。初期値の `nil` は既定timeoutなし。
- instanceのtimeoutが `nil` ならglobalへfallbackする。instanceに値があれば、そちらが優先される。
- `Regexp.new` の `timeout:` keywordでinstanceの値を指定できるが、その引数の存在をコンパイラ全体のdeadlineと解釈しない。

独自の設計判断として、照合の負荷対策を取り除く必要はない。しかし、照合の最大時間、patternを受け入れる権限、構築中の資源消費、runtimeの既知不具合を、一つのtimeout設定へ押し込めない。コンパイルと照合を区別できない監視では、「遅いregex」がどちらで止まったかも分からない。記録するなら段階・runtime識別・失敗分類を分け、未信頼pattern全文や利用者データを無条件にログへ保存しない。

## 4. GitLabのCVEとRuby側の分類を混同しない

取得時点でRuby独自のCVEは未申請であり、Ruby側が今後どう分類するかは検討中と公式告知に記載される。番号がないことを「修正不要」の理由にせず、GitLabが割り当てた識別子とRubyの修正対応を追跡する。後日更新され得るため、この状態を永続的な方針と断言しない。

[GitLab公式critical patch](https://docs.gitlab.com/releases/patches/patch-release-gitlab-19-5-1-released/)は2026-10-21公開。CVE-2026-110264について、条件が揃うと未認証ユーザーがGitLab server上で任意コードを実行できた可能性を説明し、CVSS 10.0と評価する。修正版は19.5.1、19.4.3、19.3.6に加え、19.2.9、19.1.9、19.0.10、18.11.13にもbackportされた。GitLab.comはpatch済みとされる。

**GitLabのCVSSを全Rubyアプリへ転用しない。** アプリごとの到達経路、入力者の権限、使われる実装・配布patchは別に確認する。逆にGitLab以外だから対象外とも判断しない。Ruby側の対象条件と、自分のサービスのpattern生成経路を照合することが必要である。GitLab packageの更新と、任意のホストにあるRubyを更新することも同じ作業ではない。

## 5. 更新・一時対策・受入確認の実務判断

以下は公式資料から導いた独自の進め方であり、実行済みの対応記録ではない。

1. **実行環境を列挙する。** Web processだけでなくjob worker、import処理、管理CLI、image内のruntimeを確認する。Gemfileだけを更新しても、CRuby組込みエンジンの修正は確認できない。配布元・runtime版・imageやpackageの識別情報と、関連する修正の対応を記録する。
2. **外部入力の経路を閉じる。** 更新不能時の公式暫定策は、未信頼パターンをコンパイルしないこと。必要なら該当機能を限定し、固定選択肢やliteral検索へ要件を縮める。短い入力だけを通すことや、照合timeout追加を同等の修正として扱わない。
3. **新しい実体で動かしているか確認する。** artifactの作成・配備・process切替を分けて記録し、古いworkerや別imageが残っていないか確認する。成功したdependency更新jobだけを、実行中runtimeの置換完了としない。
4. **通常入力で意味を確認する。** literal検索なら点・括弧などを文字として探せるか、regex機能なら許可した通常構文・無効入力・エラー表示が期待どおりかを検査する。巨大なパターンやメモリ破壊の再現を本番の受入試験にしない。
5. **残余の要件を明記する。** 今回の修正収録と、任意の未信頼regexを安全に提供できる保証は別である。照合負荷、メモリ予算、機能の許可範囲、エラー時の処理を継続して評価する。元の未修正runtimeへrollbackするなら、その前に当該入力経路をどう止めるかも判断対象にする。

## 6. 検証範囲・provenance・未確認事項

- 6件の本文資料をnative webで開き、公開日・系列別修正版・APIの照合契約を2026-10-23 UTCに確認した。告知・release・GitLab側の適用条件はsource catalogに個別保存した。検索結果の見出しだけを根拠にはしていない。
- Ruby 4.0 RDocはpatch版と公開日を表示しない。API契約の出典として使い、修正収録は別のrelease資料で確認した。JRuby、TruffleRuby、vendor独自版、未記載系列の修正状況は未確認。
- Ruby runtime試験は未実行。全旧版の差分、各アプリでの到達可能性、実際の侵害有無、更新後の性能・互換性を証明していない。検索evalはこの説明を発見できるかの検査であり、脆弱性修正や安全性を実証するテストではない。
- RubyニュースとGitLab release本文の明示的なopen licenseは確認できず、該当catalogのlicenseは `unknown`。独自の要約のみを保存し、ソースコード・図・長文は転載せずmodule昇格もしない。
- Ruby distribution文書の利用条件は同系列の[COPYING](https://docs.ruby-lang.org/en/4.0/COPYING.html)と[LEGAL](https://docs.ruby-lang.org/en/4.0/LEGAL.html)を確認した。Ruby License / BSD-2-Clauseの選択肢をRDocの記録に示すが、ニュースやGitLabの本文へそのまま適用しない。

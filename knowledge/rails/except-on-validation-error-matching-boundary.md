---
{
  "id": "rails-except-on-validation-error-matching-boundary",
  "title": "Rails except_on: 検証条件とerror照合・detailsの修正版境界",
  "kind": "knowledge",
  "technology": "rails",
  "version": "Rails v8.1.4 @ c3466ea00d7121798e3aa3144ffdf7174b81d8cbと2026-09-27 main 8.2.0.alpha @ c9e85dbe297e248dd2f217d04f84a94881ac046a、後続snapshot @ a471124b39b3b5c3ccfbb56f7f52d66eac34147fを比較; 2026-10-04 UTC確認; runtime未実行",
  "tags": [
    "research-domain:backend",
    "rails",
    "activemodel",
    "except_on",
    "added?",
    "strict_match?",
    "of_kind?",
    "errors.details",
    "validation-context",
    "migration"
  ],
  "sources": [
    {
      "id": "rails-except-on-weekly-20261004",
      "url": "https://rubyonrails.org/2026/10/2/this-week-in-rails",
      "type": "maintainer_article"
    },
    {
      "id": "rails-except-on-main-error-c9e85db-20261004",
      "url": "https://github.com/rails/rails/blob/c9e85dbe297e248dd2f217d04f84a94881ac046a/activemodel/lib/active_model/error.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-except-on-stable-error-c3466ea-20261004",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activemodel/lib/active_model/error.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-except-on-post-error-a471124-20261004",
      "url": "https://github.com/rails/rails/blob/a471124b39b3b5c3ccfbb56f7f52d66eac34147f/activemodel/lib/active_model/error.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-except-on-post-changelog-a471124-20261004",
      "url": "https://github.com/rails/rails/blob/a471124b39b3b5c3ccfbb56f7f52d66eac34147f/activemodel/CHANGELOG.md",
      "type": "release_notes"
    },
    {
      "id": "rails-except-on-errors-api-8-1-4-20261004",
      "url": "https://api.rubyonrails.org/classes/ActiveModel/Errors.html",
      "type": "official_docs"
    },
    {
      "id": "rails-except-on-validations-class-api-8-1-4-20261004",
      "url": "https://api.rubyonrails.org/classes/ActiveModel/Validations/ClassMethods.html",
      "type": "official_docs"
    },
    {
      "id": "rails-except-on-validations-api-8-1-4-20261004",
      "url": "https://api.rubyonrails.org/classes/ActiveModel/Validations.html",
      "type": "official_docs"
    },
    {
      "id": "rails-except-on-error-tests-c9e85db-20261004",
      "url": "https://github.com/rails/rails/blob/c9e85dbe297e248dd2f217d04f84a94881ac046a/activemodel/test/cases/error_test.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-except-on-validation-tests-c9e85db-20261004",
      "url": "https://github.com/rails/rails/blob/c9e85dbe297e248dd2f217d04f84a94881ac046a/activemodel/test/cases/validations_test.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-except-on-release-8-1-4-20261004",
      "url": "https://rubyonrails.org/2026/9/24/Rails-Version-8-1-4-has-been-released",
      "type": "release_notes"
    }
  ],
  "retrieved_at": "2026-10-04",
  "expires_at": "2026-11-03",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Rails except_on: 検証条件と error 照合を混同しない

`except_on` を指定した検証がエラーを生成しても、`errors.added?` がそのエラーを見つけない場合がある。2026-09-27 の Rails main 修正は、検証をいつ実行するかではなく、生成済みエラーの option 分類を直す。移行では **検証結果・error 照合・公開する details の形** を別々に確認する。

[2026-10-02 の公式週報](https://rubyonrails.org/2026/10/2/this-week-in-rails) はこの修正を紹介する。本稿はその直近変更を優先した調査で、`except_on` 自体を新機能として紹介するものではない。既存の Rails transaction・job・schema-cache 文書や frontend のフォーム表示とは異なり、Active Model の生成済みエラーを機械判定する契約を扱う。以下の source 読解と独自の運用案を区別する。

## 1. 確認した版: main の修正と公開 gem は別

- [8.1.4 の公式公開日](https://rubyonrails.org/2026/9/24/Rails-Version-8-1-4-has-been-released) は **2026-09-24**。GitHub tag APIで `v8.1.4` の剥離先を `c3466ea00d7121798e3aa3144ffdf7174b81d8cb` と確認した。この版の [`error.rb`](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activemodel/lib/active_model/error.rb) は `CALLBACKS_OPTIONS` に `except_on` を含めない
- [PR #58876](https://github.com/rails/rails/pull/58876) は **2026-09-27** に main へ merge。commit は `c9e85dbe297e248dd2f217d04f84a94881ac046a`、同snapshotの `RAILS_VERSION` は **8.2.0.alpha**。[`error.rb`](https://github.com/rails/rails/blob/c9e85dbe297e248dd2f217d04f84a94881ac046a/activemodel/lib/active_model/error.rb) の本PRによる本体変更は callback option 一覧へ `:except_on` を加える一行である
- 同日、その修正を含む別の後続commit `a471124b39b3b5c3ccfbb56f7f52d66eac34147f` も確認した。そこでは `RAILS_VERSION` は **8.1.4のまま**だが、[CHANGELOG](https://github.com/rails/rails/blob/a471124b39b3b5c3ccfbb56f7f52d66eac34147f/activemodel/CHANGELOG.md) の修正項目は「8.1.4 (September 24, 2026)」見出しより上にある。これは公開tagと異なるrelease後snapshotであり、「8.1.4に修正済み」と判断する根拠にはならない

以上は固定sourceで確認した差分である。配布済みの最初の修正版番号、全stable branchへの適用、vendor patchの有無は確定していない。修正の存在を理由にmainへ直接移行する提案でもない。version文字列だけでなく、実際のgem由来・lockfile・利用commitと最小再現を確認する必要がある。

## 2. except_on は検証の実行条件

[Validations::ClassMethods](https://api.rubyonrails.org/classes/ActiveModel/Validations/ClassMethods.html) は `on` を実行する validation context、`except_on` を実行しない context の指定とし、Symbolまたは配列を受け付ける。表示sourceでは、除外条件と現在の `validation_context` を配列へ変換し、交差があるかを `unless` 条件として使う。指定contextが配列なら、除外対象と一つでも一致する場合の試験が必要になる。

ここで `except_on: :draft` は「このエラーはdraft由来」という分類情報ではない。通常の検証でpresenceエラーが生成されることと、draft指定ではそのvalidatorが走らないことは別の契約である。ほかのvalidatorまで止める指示でもない。[固定版のvalidation試験](https://github.com/rails/rails/blob/c9e85dbe297e248dd2f217d04f84a94881ac046a/activemodel/test/cases/validations_test.rb) には、presenceだけを除外してもlength検証が残るケースがある。

また、[ActiveModel::Validations#valid?](https://api.rubyonrails.org/classes/ActiveModel/Validations.html#method-i-valid-3F) は実行前に `errors.clear` し、終了時に検証contextを元へ戻す。`errors` の照会は検証を再実行しない。別contextで再検証した後のerror collectionを、以前の検証結果の履歴として読まない。

## 3. エラーがあるのに added? が false になる理由

公開8.1.4のsourceを追うと次の順序になる。

1. validatorが `errors.add` へ型とoptionを渡す。error objectはoptionを保持する
2. [Errors#added?](https://api.rubyonrails.org/classes/ActiveModel/Errors.html#method-i-added-3F) はSymbol型の検索で各errorの `strict_match?` を使う
3. 公開8.1.4の [`strict_match?`](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activemodel/lib/active_model/error.rb) は、属性・型の一致後、検索optionと、callback/message optionを除いた保存optionを比較する
4. `except_on` が除外一覧にないため、保存側にそれだけが残る。検索側を空にした `added?(:title, :blank)` はoption不一致となる

したがって、この失敗から「presence検証が実行されなかった」「モデルがvalidである」とは推定できない。PRの報告例でも検証は失敗し、表示用のblank messageは存在する。修正後は保存optionから `except_on` が照合時に除かれるため、同じ属性・型の検索が一致する。これは生成済みerror objectの `options` を消去する変更ではない。

独自の設計上の含意: `added?` で重複追加を抑えるcustom validatorや、特定errorだけを処理する分岐を使っているなら、この不一致が重複メッセージや誤った分岐を起こさないかを確認する。そうしたアプリ影響が実際に発生したというupstreamの障害報告ではない。

## 4. 全optionを無視する修正ではない

[Errors API](https://api.rubyonrails.org/classes/ActiveModel/Errors.html) と固定sourceから、呼び出し目的を分ける。

- `added?` はSymbol型では意味を持つoptionまで厳密に照合する。たとえば `:too_long` の `count: 25` を保存していれば、count省略や別値でfalseになるのは契約どおりである
- `of_kind?` は属性と型の存在を調べる。countなどの一致を必要としないときの別の問いであり、`added?` の一括置換として扱わない
- `where` は渡した属性・型・optionだけを照合する。内部の `match?` は保存optionを検索optionごとに比較するため、`strict_match?` と同じ正規化規則とはみなさない
- 文字列を渡す `added?` は表示messageの一致を見る。Symbol型を文字列へ変えて偶然通すと、翻訳・文言と機械判定が結び付く

さらに、[mainのsource](https://github.com/rails/rails/blob/c9e85dbe297e248dd2f217d04f84a94881ac046a/activemodel/lib/active_model/error.rb) は `strict_match?` の検索側と保存側をともにfilterするが、[後続a471124のsource](https://github.com/rails/rails/blob/a471124b39b3b5c3ccfbb56f7f52d66eac34147f/activemodel/lib/active_model/error.rb) と公開8.1.4は保存側だけをfilterする。検索側にもcallback/message optionを付けた場合の互換性は同じではない。**query側filterを今回の一行修正の効果として説明しない**。

設計案として、検索には属性・error型と必要な意味上のoptionを指定し、validatorの設定hash全体を渡さない。古い版で `except_on` を検索側へ足して合わせる回避策も、修正後に同じ結果を保証しないため、恒久的な照合契約にしない。

## 5. details の修正範囲と公開JSON

[`Error#details`](https://github.com/rails/rails/blob/c9e85dbe297e248dd2f217d04f84a94881ac046a/activemodel/lib/active_model/error.rb) はerror型と、callback/message optionを除いたhashを返す。この修正で `except_on` はdetailsから外れるが、`count` や独自optionがすべて除去されるわけではない。固定版の [`error_test.rb`](https://github.com/rails/rails/blob/c9e85dbe297e248dd2f217d04f84a94881ac046a/activemodel/test/cases/error_test.rb) は、callback optionを除きつつ独自の `foo: :bar` が残ることを確認している。

[`Errors#as_json`](https://api.rubyonrails.org/classes/ActiveModel/Errors.html#method-i-as_json) は通常messageを返す経路であり、`errors.details` と同じ構造ではない。今回の修正を「全エラーJSONから全内部情報が消える」と広げない。

独自の運用案: detailsをAPIへ直接返している場合、`except_on` の消失がresponse snapshotやクライアントschemaへ与える差分を確認する。新設APIでは公開するerror code・属性・必要なパラメータを自分の契約として選び、Railsの全optionを公開形式へ自動転送しない。既存APIのキーを削る判断には利用側との互換性確認が必要である。

## 6. 移行の判断手順と受入試験

以下は今回未実行のアプリ向け試験案であり、Rails公式の導入手順ではない。

1. `except_on` を使うvalidatorと、`added?`・`of_kind?`・`where`・`details` を使う処理を一緒に洗い出す。単にvalidator宣言の有無だけで影響を判定しない
2. 実際に配布しているActive Modelの版とsourceを固定する。公開8.1.4、修正を含む検証用snapshot、導入候補のリリースで同じ再現を比較する
3. presenceのみの小さいmodelで、通常検証のfalse、blank message、Symbol型のadded?、detailsのキーをそれぞれ記録する。修正前は「検証false・照合false」が同時に起き得ることを区別する
4. 除外contextでpresenceが走らないことを確認する。別のlength validatorを加えたケースでは、除外後もその失敗が残ることを確認する
5. 同じerror型にcountを変えたケースを用意し、正しいcountだけがadded?に一致すること、of_kind?では型の存在を判定することを確認する
6. callback optionを検索側に付けた場合と付けない場合を比較する。mainと後続a471124の差を一括の「修正済」扱いで隠さない
7. 通常検証、除外contextの検証、再び通常検証という順で同じobjectを使い、前回のerrorを読み続けていないことを確認する
8. APIの公開JSONではmessage形式とdetails形式を分け、除外されるexcept_onと残るcount・独自optionをそれぞれ検証する

## 7. 検証したこと・していないこと

- 公式週報・API・release告知をWebで開き、PR情報・diff・各固定commitのsourceとライセンスをGitHubのread-only取得でも照合した。固定commitのWeb表示は一部cache missだったため、読めたふりをせずconnector取得で内容を確認した
- mainの `test_validate_with_except_on` は通常検証後のadded?成功、detailsからの除外、除外contextでの成功とemptyを追加する。error単体の既存試験にもexcept_onが加わる。これは**試験sourceの読解**であり、本環境での実行結果ではない
- Ruby / Rails runtime試験は実行していない。PR本文の「3 runs / 8 assertions」等は作者報告であり、こちらの検証実績として数えない
- DBへのsave、Active Record独自の既定context、custom serializer、第三者validator、全patch版の影響範囲は未検証。最初の修正版release番号を推測しない
- 全取得日は2026-10-04 UTC。release_notesの30日TTLが最短なので明示期限は2026-11-03。各コードsnapshotの `activemodel/MIT-LICENSE` を確認した。記事・告知の個別ライセンスはunknownとして原文転載をせず、独自の日本語説明だけを保存する

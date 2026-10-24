---
{
  "id": "rails-message-serializer-marshal-retirement-boundary",
  "title": "Rails message_serializer: Marshal読込み廃止・fallback観測・既存tokenの移行境界",
  "kind": "knowledge",
  "technology": "rails",
  "version": "Rails main 8.2.0.alpha @ eaa8299527c069c256a695622b90129b15d9aef8 (2026-10-22); config.load_defaults 7.1-8.1 versus 8.2; unreleased snapshot, runtime untested",
  "tags": [
    "research-domain:backend",
    "rails",
    "message_serializer",
    "Marshal",
    "MessageVerifier",
    "MessageEncryptor",
    "fallback",
    "migration",
    "deserialization"
  ],
  "sources": [
    {
      "id": "rails-message-default-eaa82995-20261024",
      "url": "https://github.com/rails/rails/commit/eaa8299527c069c256a695622b90129b15d9aef8",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-message-contract-eaa82995-20261024",
      "url": "https://github.com/rails/rails/tree/eaa8299527c069c256a695622b90129b15d9aef8",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-safer-secrets-news-20261023-20261024",
      "url": "https://rubyonrails.org/2026/10/23/this-week-in-rails",
      "type": "maintainer_article"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2026-11-23",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/backend.json"]
}
---

# RailsのMarshal読込みを、既定値と観測の両方から廃止する

## 問い・結論・適用版

Railsを更新して `message_serializer = :json` にすれば、古いsigned messageを壊さず、
すべてのMarshal deserializationを止められるか。fallback通知が0なら移行完了と判断できるか。

結論は、**Railsの版、load_defaults、各instanceのserializerとrotation、まだ使われる旧messageを分けて確認する**。
`:json_allow_marshal` と `:json` はどちらもJSONを書き出すが、Marshalを読めるかが違う。
既定値を変更しても、明示したserializer、既に生成したinstance、rotationまで一括で廃止されるわけではない。
fallback通知は代替形式への移行を観測するもので、全Marshal読込みの監査ログではない。

[PR #59156](https://github.com/rails/rails/pull/59156) は2026-10-22 06:42:12 UTCに
`eaa8299527c069c256a695622b90129b15d9aef8` としてmainへmergeされた。
同snapshotの [RAILS_VERSION][version] は **8.2.0.alpha**。
[10月23日の開発元記事][news] が新既定値を紹介しているが、正式版8.2への収録を確認したという意味ではない。
本稿はこの開発snapshotのsource reviewであり、インストール済みのstable版へ無条件に当てはめない。

## 変更は「新しいJSON writer」ではなく、既定readerの許容範囲

以下は固定 [configuration.rb][config]、[変更commit][change]、[serializer実装][serializer]の事実。

| 条件 | writer / readerの意味 |
|---|---|
| `load_defaults "7.1"`〜`"8.1"`、別の明示設定なし | `:json_allow_marshal`を継承し、JSONを生成するがMarshalも読める |
| このsnapshotの `load_defaults "8.2"`、別の明示設定なし | `:json`になり、Marshal fallbackを許可しない |
| `:json` | JSONを生成・読込み。利用可能なMessagePackへのfallbackは残る |
| `:json_allow_marshal` | JSONを生成・読込み。MessagePackとMarshalへのfallbackを許可する |
| `:marshal` | Marshalを生成・読込み。JSON・MessagePackもfallbackで読める |
| 独自serializer object / instanceへの明示option | 上記の既定値だけでは契約を決められない。`dump` / `load`の実装と設定を別途確認する |

`:json`を「JSON以外は絶対に読まないstrict decoder」と説明しない。
固定実装は非Marshal形式へのfallbackを残し、MessagePackの検出には関連実装の利用可能性を調べる。
JSONから生成した通常messageを新旧の `:json*` readerで読むことと、
過去に生成したMarshal messageを受理することは別の互換性である。

固定 [new_framework_defaults_8_2.rb.tt][migration-template] は、個別に `:json` を有効化する前に
`message_serializer_fallback.active_support` でMarshal messageの残存を確認する方針を示す。
**独自の設計判断:** serializerだけを評価するために他の8.2既定値まで一度に有効化せず、
採用するrelease / bundleがこの設定を実際に扱えるかを確かめて個別に移行する。

## グローバルな既定値は、すべての生存instanceの置換ではない

固定 [Railtie][railtie] は `after_initialize` で設定値を
`ActiveSupport::Messages::Codec.default_serializer` に反映する。
固定 [Codec][codec] はconstructorで明示 `serializer:` を先に選び、なければclassの既定値を使い、
選んだserializerをinstance変数に保存する。これらは**固定sourceの実装観察**である。

この構造から、次を別々に監査する必要がある。

- gemの版だけでなく `config.load_defaults` のtargetと、その後の上書きを確認する。
  8.2系sourceを使っていても7.1〜8.1 defaultsを保持したアプリは自動的に`:json`にはならない
- `MessageVerifier.new` / `MessageEncryptor.new` の明示 `serializer:` を探す。
  `Marshal` objectや `:json_allow_marshal` の指定を全体設定が取り消すとは考えない
- boot途中に作成した独自instance、singleton、long-lived workerを確認する。
  classの既定値変更が保存済みinstanceのserializerを書き換える処理はCodecにはない
- [Rotator][rotator] の旧設定fallbackを確認する。
  主readerが拒否しても、旧serializerを持つrotationが次に読める場合がある

**独自の設計判断:** 確認済み設定でprocessを再生成し、通常のアプリ経路から実際に使うinstanceを試験する。
Rails外でActive Supportを直接使う処理や独自gemにも、アプリのdefaultsが同じように届くと仮定しない。
cookie、schema cache、Active Record encryption等の全serializationを、この設定一つで監査したことにしない。

## fallback通知が0でも、Marshal廃止を証明できない

固定 [SerializerWithFallback#load][serializer] は、検出形式が主形式と違い、fallbackが許可される場合に通知する。
payloadには `serializer`、`fallback`、`serialized` が入り、読込み結果が `deserialized` に設定される。
同形式の読込みは直接 `_load` へ進む。ここから、通知の意味を次のように限定する。

1. `fallback == :marshal` は、設定された主形式以外からMarshalを読もうとする経路の指標になる。
   全fallback数ではなく、移行で廃止したい形式を区別する。MessagePack fallbackまで失敗扱いにしない
2. 主serializerが `:marshal` ならMarshal読込みは同形式なので、このfallback通知は発生しない。
   また `serializer: Marshal` は組込みfallback wrapper自体を使わない
3. 明示Marshal rotationで読めた場合も、それを必ずこの通知で捕捉できるとは限らない。
   [Rotator][rotator] の `on_rotation` とserializer内部のfallbackは異なる層である
4. そもそも旧messageが観測期間中に送られなかった、対象workerにsubscriberがなかった場合も0になる。
   最終利用のない期間を、保存済みmessageが存在しない証拠にしない

**独自の設計判断:** 主形式・fallback形式の組合せと件数を収集し、コード・設定の棚卸しと突き合わせる。
payload全体をログへ流さない。`serialized` と `deserialized` はmessage本文や個人情報を含み得るため、
必要な低cardinality属性だけを採用する。通知が出たこと自体を、messageの業務認可や安全性の証明にもしない。

## 署名の正しさ、deserializeの成功、業務上の受理は別判定

固定 [MessageVerifier][verifier] では、`valid_message?` は署名付きmessageの形式とdigestを調べる経路であり、
deserialize完了を検査していない。`read_message` はdigest確認後にdecodeとmetadataを含むdeserializeへ進む。
したがって `valid_message? == true` だけでは、新serializerで旧messageが読めることを保証しない。

さらに、`verified` はserialization errorを例外として再送出する。
`:json`で拒否される旧形式を「必ずnilになり、既存のnil処理だけで救える」と想定しない。
`verify`もserialization errorをすべて `InvalidSignature` に変えるわけではない。
外側のAPI、rotation、metadata形式で見える失敗を対象bundleで確認する必要がある。
[固定Codecのerror分類][codec]、[固定MessageVerifier][verifier]

API説明は、署名は内容を暗号化しないこと、署名secretの漏えい時にはMarshalがdeserialization攻撃の経路になり得ることを明記する。
**独自の設計判断:** JSONへ変更しても漏れた鍵の信頼は回復しない。
署名検証や暗号化・認証の成功をMarshal payloadの安全性と同一視せず、漏えい時は鍵・発行済みmessage・
権限判断を別に対処する。単に復号できる、署名が合う、JSONとして読めることだけで業務上の権限を付与しない。

## 既存tokenを退役させる手順（独自の設計案）

1. 発行者と読込み経路を棚卸しする。signed IDs、signed GlobalIDs、Active Storage関連、
   独自verifier / encryptorについて、実際のserializer・rotation・purpose・有効期限・設定を記録する
2. writerがJSONを生成していても、過去のMarshal messageが残るか確認する。
   固定 [MessageVerifier][verifier] は明示期限のないmessageに自動有効期限を設けないため、
   「7.1へ移行してから十分時間が経った」だけでは退役日を決めない
3. 通知を対象processへ配置し、Marshal fallback件数と非観測経路を調べる。
   長期間使われないリンクや保存tokenには、再発行、期限導入、明示無効化などの方針を決める
4. 新旧readerを併用する期間は、実際のbundleで旧Marshalと新JSONのfixtureを各経路へ通す。
   serializerの変更と、鍵・digest・cipher・metadataの変更を分離し、どの境界で拒否されたか識別する
5. 許容した旧messageを退役させたうえで、`:json`と必要なrotation除去を配備する。
   全instanceの再生成後、拒否率と利用者の再発行経路を確認する
6. rollbackの判定を先に決める。可用性回復のためMarshal fallbackを戻す操作は読込み範囲の再拡大であり、
   とくに鍵漏えいを疑う状況では単なる互換性復旧ではない。旧設定を無条件に戻さない

これはRailsが自動的に旧messageを変換・再発行・失効する機能の説明ではない。
既定値変更は既存tokenの保存先や有効期限を走査しない。

## 確認した試験と、採用先で必要な試験

固定 [configuration_test.rb][config-test] は、新規アプリが`:json`、7.1 defaultsのアプリが
`:json_allow_marshal`、load_defaultsを外したケースが`:marshal`となるassertionを持つ。
固定 [serializer_with_fallback_test.rb][serializer-test] は、非Marshal形式の相互読込み、
Marshalを許可するserializerと拒否するserializerの分離、fallback payloadを確認する。
**いずれも今回はsourceを読んだだけで、upstream suiteを実行していない。**

採用先のテスト案:

- `:json_allow_marshal`で発行したJSONを`:json`で読めることと、旧Marshalの拒否を別々に確認する
- primary `:marshal`、明示 `Marshal`、旧Marshal rotationが残る構成で、通知0を廃止完了と誤判定しない
- 署名は正しいがserializer非対応のfixtureを使い、`valid_message?`と`verified`の結果を区別する
- key不一致、期限切れ、purpose不一致をserializer失敗と混ぜず、利用者向け再発行と監視に分類する
- 新旧process混在、boot前後のinstance作成、rollback後に許可される形式を確認する

本環境で `ruby` executableを確認できず、Rails runtime試験・性能測定・各gemの相互運用試験は未実施。
最初の正式収録release、全token経路、全custom serializer、実際の鍵管理と有効期限は未確認である。
検索evalは本知識を発見できるかの検査であり、これらのruntime挙動を検証しない。

## 出典・取得上の制約

2026-10-24 UTCにnative webで開発元記事とPRを実際に開き、GitHub read-only取得で40桁commit固定の
設定・serializer・Codec・Railtie・Rotator・MessageVerifier・テスト・RAILS_VERSION・MIT-LICENSEを読んだ。
commit / blobのnative web表示はcache missで取得できず、固定sourceはconnectorで補完した。
edge guidesは8.2の既定値行を含まない古い表示だったため、新既定値の根拠には使用していない。

固定Rails sourceはMIT。[個別license][license]を確認した。
開発元記事の個別再利用licenseはunknownとし、原文や実装コードを転載せず独自の日本語要約を記した。
module昇格はない。既存のJSON schema cache文書とは、対象がruntimeのsigned/encrypted messageであり、
変更点がreader許可・token寿命・fallback観測である点で異なる。

[change]: https://github.com/rails/rails/commit/eaa8299527c069c256a695622b90129b15d9aef8
[news]: https://rubyonrails.org/2026/10/23/this-week-in-rails
[version]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/RAILS_VERSION
[config]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/railties/lib/rails/application/configuration.rb
[migration-template]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/railties/lib/rails/generators/rails/app/templates/config/initializers/new_framework_defaults_8_2.rb.tt
[serializer]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/activesupport/lib/active_support/messages/serializer_with_fallback.rb
[codec]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/activesupport/lib/active_support/messages/codec.rb
[railtie]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/activesupport/lib/active_support/railtie.rb
[rotator]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/activesupport/lib/active_support/messages/rotator.rb
[verifier]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/activesupport/lib/active_support/message_verifier.rb
[config-test]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/railties/test/application/configuration_test.rb
[serializer-test]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/activesupport/test/messages/serializer_with_fallback_test.rb
[license]: https://github.com/rails/rails/blob/eaa8299527c069c256a695622b90129b15d9aef8/MIT-LICENSE

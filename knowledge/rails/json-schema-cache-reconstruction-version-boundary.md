---
{
  "id": "rails-json-schema-cache-reconstruction-version-boundary",
  "title": "Rails JSON schema cache: 型の再構築・整数境界・schema版検査の移行条件",
  "kind": "knowledge",
  "technology": "rails",
  "version": "Rails main 8.2.0.alpha @ f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d (2026-09-21); compared with v8.1.4 @ c3466ea00d7121798e3aa3144ffdf7174b81d8cb; json >= 2.10.0; runtime untested",
  "tags": [
    "research-domain:backend",
    "rails",
    "schema-cache",
    "JSON",
    "serialization",
    "FrozenError",
    "integer",
    "migration",
    "schema-version"
  ],
  "sources": [
    {
      "id": "rails-json-schema-cache-add-216b5901-20261004",
      "url": "https://github.com/rails/rails/commit/216b59017fd12fa3ffd3ecd632dc47203a58f353",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-json-schema-cache-bounds-f54d6ac4-20261004",
      "url": "https://github.com/rails/rails/tree/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-schema-cache-stable-814-c3466ea0-20261004",
      "url": "https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/schema_cache.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "rails-json-schema-cache-news-20260918-20261004",
      "url": "https://rubyonrails.org/2026/9/18/this-week-in-rails",
      "type": "maintainer_article"
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

# Rails JSON schema cache の移行で、読めた後に何を確かめるか

## 問い・結論・適用版

schema cache の読込みを速くするため、出力先を `schema_cache.json` に変えるだけでよいか。
JSON のparseが成功し、DBのschema版も一致すれば、従来のYAMLと同じ型処理ができるか。

結論は、**形式への対応、型オブジェクトの再構築、接続先のschema版検査を別々に確認する**こと。
JSONは任意のschema記述を取り込む公開交換形式ではなく、この時点のRailsが自分のcacheを
保存・復元するための内部形式である。ファイルが読めても、その後の整数serializationまで
正常とは限らなかった。JSON化だけでDB接続を不要にするものでもない。

| 確認したsnapshot | 事実 | 運用上の意味 |
|---|---|---|
| `216b59017fd12fa3ffd3ecd632dc47203a58f353`、2026-09-16 | PR #58769でJSON保存・復元をmainへ追加 | 紹介記事の日付をstable版への収録日と読み替えない |
| `f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d`、2026-09-21 | PR #58832で復元したIntegerの上下限を初期化 | 最初のJSON対応commitだけを採用する場合はこの修正も確認する |
| 上記修正snapshotの `RAILS_VERSION` | `8.2.0.alpha` | 本稿は固定した開発版の観察であり、8.2正式版の保証ではない |
| v8.1.4、`c3466ea00d7121798e3aa3144ffdf7174b81d8cb` | `_load_from` / `dump_to` にJSON分岐がない | `.json` という名前だけでは8.1.4に新serializerは導入されない |

mainへの導入・修正日時はcommit metadataと公式PRで照合し、v8.1.4はannotated tagを
上記commitへ解決した。固定版より後の全変更や、最初にこの機能を収録する正式releaseは
本稿では確定していない。
[導入PR](https://github.com/rails/rails/pull/58769)、
[修正PR](https://github.com/rails/rails/pull/58832)、
[8.1.4の実装](https://github.com/rails/rails/blob/c3466ea00d7121798e3aa3144ffdf7174b81d8cb/activerecord/lib/active_record/connection_adapters/schema_cache.rb)

## 形式は内容の自動判定ではない

以下は修正snapshotの `schema_cache.rb` と `schema_cache_serializer.rb` の**実装観察**。
将来も同じprivate APIであるという約束ではない。

- `schema_cache_path` に通常の `.json` 出力先を指定して新serializerを選ぶ。
  未指定時の既定名はprimaryが `db/schema_cache.yml`、その他が
  `db/<name>_schema_cache.yml` であり、自動的にJSONへ変更されてはいない
- 実際の分岐はfilenameの `.dump` 部分一致を先に、次に `.json` 部分一致を調べる。
  最後の拡張子だけを厳密に判定したり、内容をsniffしたりする実装ではない。
  たとえば `schema.dump.json` はMarshal経路になる
- `.gz` は最終拡張子で別途判定される。`.json.gz` のgzip展開後にはJSONを読み、
  dump側にもgzip書込みがある。gzipは別serializerではない
- JSON用ファイルをloadすると **json gem >= 2.10.0** を要求する。
  依存が満たされなければ `LoadError`。Gemfile.lockを含む実際のbundleを確認する
- 既存YAMLの拡張子だけを `.json` に変更しても内容は変換されない。
  対応するRailsからcacheを再生成する必要がある

特にローリングデプロイで同じcache fileを新旧Railsから共有すると、旧版が新形式を
同じ意味で読めるとは限らない。8.1.4はJSONを選ぶ分岐を持たず、`.dump` 以外をYAMLとして
扱う。これは「必ず同じ例外になる」という主張ではなく、**新しいcache objectの復元契約が
旧版には存在しない**という版差である。

根拠は [修正snapshotの形式選択](https://github.com/rails/rails/blob/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d/activerecord/lib/active_record/connection_adapters/schema_cache.rb)、
[json依存とserializer](https://github.com/rails/rails/blob/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d/activerecord/lib/active_record/connection_adapters/schema_cache_serializer.rb)、
[既定path](https://github.com/rails/rails/blob/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d/activerecord/lib/active_record/database_configurations/hash_config.rb)。

## JSON.parse の成功と型の復元完了は違う

固定serializerは、cache内のcolumn・index・型などをobject参照付きの配列に保存する。
各要素には `_type` があり、loadでは登録されたclassを選び、`allocate` でinstanceを作り、
`init_from_schema_json` へ値と既に復元した参照一覧を渡す。最後のobjectがloadの返り値になる。
通常の `new` / `initialize` を同じ引数で呼ぶ方式ではない。

この違いはcustom typeやthird-party adapterの移行判定にも効く。

- dump側は `as_schema_json` を持つことと、class名に対応する登録を使う。
  メソッドだけを追加しても、登録がなければ `type_for` の `fetch` は成功しない
- load側は既知の `_type` か `register` 済みの対応を使う。
  `_type` 文字列を任意のclass名として直接 `const_get` する実装ではない
- ただし、登録済みだから全ての内部状態が正しく復元されるという保証でもない。
  constructorで設定していた派生値・subtype・timezone等を復元hookでも整える必要がある
- serializer自身に `# :nodoc:` が付いている。`register` の存在を、アプリケーション向けの
  長期互換extension APIとして保証されたものと読み替えない

**独自の設計判断:** cacheは信頼できるbuild/deploy工程で作った内部artifactに限定する。
JSONという形式名や型registryの存在だけを、未信頼uploadを安全に受け入れられる根拠にしない。
本調査はserializerのsecurity auditではなく、未知型・不正参照・巨大入力を含む
全入力の安全性を検証してはいない。
[版固定serializer](https://github.com/rails/rails/blob/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d/activerecord/lib/active_record/connection_adapters/schema_cache_serializer.rb)

## IntegerのFrozenErrorはparse後の遅延不具合だった

導入時の共通type復元hookはprecision・scale・limitを戻すが、Integerのconstructorが設定する
`@max` / `@min` は初期化されなかった。Integerの範囲検査には未設定時の遅延初期化がある。
そのため、JSONから復元した型をRails側がfreezeした後、最初のserializationで上下限を
代入しようとすると `FrozenError` になる。修正PRはfixture loadingで表面化する事例を報告する。

9月21日の修正はIntegerの `init_from_schema_json` を追加し、共通復元の後で
`@max` と `@min` を計算する。**freeze前に派生状態を復元する**のが変更点であり、
全ての型をunfreezeしたり、整数の範囲検査を無効にしたりする修正ではない。

この版の符号付きIntegerではlimitがbytes、既定が4bytesである。
最大側は `1 << (limit * 8 - 1)` を排他的境界、最小側はその負値を包含境界として扱う。
4bytesなら `-2147483648` と `2147483647` が範囲内で、`2147483648` は範囲外になる。
範囲外のserializationには引き続き `ActiveModel::RangeError` があり得る。
これは固定したInteger実装からの計算で、全DB型・unsigned型へ一般化しない。

運用側で必要なのは「JSONを読めた」という試験に加え、**復元後にfreezeした型で値をserializeできるか**
という試験である。fixtures、通常のinsert/update、独自typeの派生状態を別々に確認する。
既存のcacheを読み込むだけの起動テストでは、初回の値処理まで遅れる不具合を取り逃がし得る。
[修正commit](https://github.com/rails/rails/commit/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d)、
[版固定Integer](https://github.com/rails/rails/blob/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d/activemodel/lib/active_model/type/integer.rb)

## schema版検査は、parse失敗の救済でも完全な整合性証明でもない

固定 `SchemaReflection` の既定は `use_schema_cache_dump=true` と
`check_schema_cache_dump_version=true`。通常のload経路は次の順である。

1. dump使用が有効で、pathがあり、そのfileが存在することを確認する
2. `SchemaCache._load_from` でfileをdeserializeする
3. 版検査が有効ならDB接続を借り、接続先の `schema_version` とcacheの版を比較する
4. 不一致ならwarningを出してそのcacheを無視し、呼出し側は空cacheから必要なmetadataを取得する

版検査部分の `ActiveRecordError` もwarningとcache不採用になる。しかしdeserializeは
そのrescue範囲の外である。壊れたJSON、未登録 `_type`、json gem不足の失敗を全て
「古いcacheなので無視される」と扱う包括的fallbackは、この実装にはない。

ここから区別すべき点は四つある。

- JSON cacheがあっても、版検査を有効にした通常loadではDB接続が必要になる。
  列metadataのquery削減と、起動経路のDB依存解消を同一視しない
- `check_schema_cache_dump_version=false` は比較を省く設定であり、
  互換性・新鮮さを別手段で検証する機能を追加するものではない
- 比較しているのはschema_versionである。全column定義のchecksumや、
  Rails / adapter / json gemの版の一致まで検査する仕組みではない
- これはfileをloadする段階の比較であり、採用済みcacheを毎回DBと照合する処理ではない。
  process起動後のmigrationや外部DDLへの対応は、別に運用を決める

**独自の設計判断:** version mismatchのwarningを成功扱いで見落とさず、cache不採用による
schema queryの増加も観測する。DBに接続できない起動を作るためだけに版検査を切る前に、
artifactとDBの対応、古いprocessの扱い、migration順序を定義する。
[版固定SchemaReflection](https://github.com/rails/rails/blob/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d/activerecord/lib/active_record/connection_adapters/schema_cache.rb)

## 導入とrollbackの手順（独自の設計案）

1. 利用Railsのcommit/releaseがJSON対応とInteger修正を含むか確認する。
   v8.1.4の設定だけを変更して対応版とみなさない。アプリケーションが依存するadapter、
   custom type、json gemを同じbundleで試験する
2. DBごとに出力先と読込み先を対応させる。新旧releaseのcacheを別pathにし、
   旧YAMLを拡張子変更で流用せず、対象環境のschemaから新しいcacheを生成する
3. deployと同じbundleを使うfresh processでloadし、column型、index、primary keyを確認する。
   さらにfreeze後のInteger serialization、境界値、fixture loadingを実行する
4. schema版不一致、file欠落、壊れたJSON、未対応type、依存不足を別ケースとして扱う。
   期待するwarning・起動失敗・DB query fallbackを実際に観測する
5. 起動時間、cache load時間、schema query数、メモリを自分のschema規模で比較する。
   serializer単体の高速化を、アプリケーション全体の同率高速化とみなさない
6. rollbackでは旧Rails用のpathと読める形式も戻す。新JSON fileを残したまま
   Rails gemだけを戻す運用や、全instanceが同じ可変fileを読む運用を避ける

[9月18日の開発元記事](https://rubyonrails.org/2026/9/18/this-week-in-rails)と導入PRは、
944 tablesのアプリケーションでJSON loadがYAMLより約22倍速かったと報告する。
これは投稿者の特定環境におけるload benchmarkであり、本稿の測定値でも、
全アプリケーションのcold boot改善率でもない。

## 確認したテスト・provenance・未確認事項

- 固定 `schema_cache_test.rb` の共通 `DumpAndLoadTests` がMarshal / YAML / JSONのclassで
  利用され、disk往復とgzip往復の後にcolumns・cast_type・primary key・indexを確認することを
  静的に読んだ。これは全custom typeのconstructor状態を検証する証拠ではない
- 修正PR本文はfreeze後のInteger回帰テストと42 tests成功を報告する一方、
  確認したmerge diffは `integer.rb` の6行追加だけだった。
  投稿者の実行報告と、今回確認したtree/diffを分け、回帰テストを実行済みとは記録しない
- この調査環境では `ruby` executableを確認できず、**Rails runtime試験は実行していない**。
  対象DBを使う移行試験、性能測定、旧版との相互運用、third-party adapterの網羅検証は未実施
- native webで公式PR 2件、開発元記事、commitを開き、GitHubのread-only取得で
  40桁commit固定の実装・テスト・RAILS_VERSION・MIT-LICENSEを確認した。
  edge APIページは古い `main@ed0f92c` の内容を返しJSON分岐がなかったため、
  今回の新機能の根拠には使っていない。修正commitのweb取得cache missはconnectorで補った
- Railsの固定sourceはMIT。開発元記事の個別再利用licenseはunknownとして原文を転載せず、
  独自の日本語要約と設計判断を保存した。実装コードの取込みやmodule昇格はない
- 検索evalはこの知識を検索できることの検査であり、Rails serializerの動作検証ではない

[確認した版固定テスト](https://github.com/rails/rails/blob/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d/activerecord/test/cases/connection_adapters/schema_cache_test.rb)、
[RailsのMIT-LICENSE](https://github.com/rails/rails/blob/f54d6ac4c70675a0e6a9c3a5ddbb01945d2e828d/MIT-LICENSE)

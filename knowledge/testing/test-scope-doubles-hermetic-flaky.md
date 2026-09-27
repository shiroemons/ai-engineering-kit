---
{
  "id": "testing-scope-doubles-hermetic-flaky",
  "title": "testing の scope 分類と test doubles 選択、hermetic 要件と flaky-test の retry・再実行管理",
  "kind": "knowledge",
  "technology": "testing",
  "version": "Bazel Test Encyclopedia rolling docs + Fowler Mocks Aren't Stubs (2007-01-02) + Jenkins Flaky Test Handler v1.3.172.v35b_30de7fa_5b_ (verified 2026-09-27)",
  "tags": [
    "research-domain:quality-operations",
    "testing",
    "test scope",
    "test doubles",
    "dummy",
    "fake",
    "stub",
    "spy",
    "mock",
    "state verification",
    "behavior verification",
    "classical TDD",
    "mockist TDD",
    "hermetic",
    "timeout",
    "runfiles",
    "TEST_TMPDIR",
    "flaky",
    "flake",
    "retry",
    "rerunFailingTestsCount",
    "deflake",
    "quarantine"
  ],
  "sources": [
    {
      "id": "bazel-test-encyclopedia",
      "url": "https://bazel.build/reference/test-encyclopedia",
      "type": "official_docs"
    },
    {
      "id": "fowler-mocks-arent-stubs",
      "url": "https://martinfowler.com/articles/mocksArentStubs.html",
      "type": "maintainer_article"
    },
    {
      "id": "jenkins-flaky-test-handler",
      "url": "https://plugins.jenkins.io/flaky-test-handler/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "maintainer",
  "status": "active"
}
---

# testing の scope 分類と test doubles 選択、hermetic 要件と flaky-test の retry・再実行管理

「どの粒度で何を doubles に置き換えるか」「テストを hermetic に保つ条件は何か」「flaky をどう検出して retry をどう bounded にするか」を、3件の検証済み出典の事実と組み立て提案に分けて書く。3件はいずれも 2026-09-27 時点の検証済み inventory に基づく。

## 要点（出典に記載された事実）

### テスト規模は timeout と tag で運用する（Bazel Test Encyclopedia）

- Bazel Test Encyclopedia は rolling current docs であり、テスト規模（size）に timeout の目安が対応する。small は short 60秒、medium は moderate 300秒、large は long 900秒、enormous は eternal 3600秒である。[Bazel Test Encyclopedia](https://bazel.build/reference/test-encyclopedia)
- テストの合否は exit code で判定し、exit code 0 が pass を意味する。[Bazel Test Encyclopedia](https://bazel.build/reference/test-encyclopedia)
- 同文書が挙げる tags には small / medium / large / smoke / exclusive / external / manual が含まれる。size 由来の tag（small / medium / large）と、実行特性を選別する tag（smoke / exclusive / external / manual）が同一の tag 機構に並ぶ。[Bazel Test Encyclopedia](https://bazel.build/reference/test-encyclopedia)
- 各 tag の詳細な選別意味（wildcard からの除外条件など）は本 inventory の検証範囲外であり、本ドキュメントは tag 名の存在のみを事実として扱う。運用への使い方は後述の設計案に分ける。

### hermetic とは結果が宣言済み入力にのみ依存すること（Bazel Test Encyclopedia）

- テストは hermetic であるべきであり、その結果は宣言済みの source・build 入力と runner が保証する入力にのみ依存しなければならない。[Bazel Test Encyclopedia](https://bazel.build/reference/test-encyclopedia)
- runfiles は読み取り専用であり、書き込み可能な出力は TEST_TMPDIR と TEST_UNDECLARED_OUTPUTS_DIR の指す場所に限られる。[Bazel Test Encyclopedia](https://bazel.build/reference/test-encyclopedia)
- すなわち「外側のファイル・ネットワーク・事前状態に触れたテストが時々落ちる」事象は、この定義に照らせば非 hermetic の疑いがあるという分類ができる。個別の原因断定は観測なしには行わない。

### Test Double は5分類、検証は state と behavior の2方式（Fowler, Mocks Aren't Stubs）

- Gerard Meszaros の分類として Dummy / Fake / Stub / Spy / Mock の5種を挙げる。[Mocks Aren't Stubs](https://martinfowler.com/articles/mocksArentStubs.html)
- Mock は振る舞い検証（behavior verification）で使う double であり、期待（expectation）を事前にプログラムしておく。呼び出しが期待通りかで検証する。[Mocks Aren't Stubs](https://martinfowler.com/articles/mocksArentStubs.html)
- 対照的に state verification は、実行後に対象オブジェクトや collaborator の状態を見て検証する方式である。[Mocks Aren't Stubs](https://martinfowler.com/articles/mocksArentStubs.html)
- classical TDD は実オブジェクト中心で、扱いにくい collaborator にだけ doubles を使う。mockist TDD は関心のある collaborator を mock 化する。[Mocks Aren't Stubs](https://martinfowler.com/articles/mocksArentStubs.html)
- 同記事は tradeoff として、fixture 再利用とテストごとの mock 設定、ならびに実装への結合（implementation coupling）を論じる。mock 側の期待設定はテストごとに書く傾向があり、実装の呼び出し方にテストが結合しやすい。[Mocks Aren't Stubs](https://martinfowler.com/articles/mocksArentStubs.html)

### flaky は同一構成で pass も fail もする（Jenkins Flaky Test Handler）

- 同 plugin docs は flaky test を、同じ構成（same configuration）で pass も fail もするテストと定義する。[Flaky Test Handler](https://plugins.jenkins.io/flaky-test-handler/)
- Maven Surefire の `rerunFailingTestsCount` により、失敗テストを最大 N 回まで retry できる。N 回以内に pass したものは Flake として印付けされ、flake のみであれば build 全体は成功扱いになる。[Flaky Test Handler](https://plugins.jenkins.io/flaky-test-handler/)
- Deflake action は、直前に失敗したテストだけを再実行する。失敗時の正確な Git revision を指定しての再実行も任意でできる。[Flaky Test Handler](https://plugins.jenkins.io/flaky-test-handler/)
- revision ごとの pass / fail 集計を、pass / fail / flake history として蓄積する。[Flaky Test Handler](https://plugins.jenkins.io/flaky-test-handler/)
- 対象版は Flaky Test Handler v1.3.172.v35b_30de7fa_5b_、要求 Jenkins 2.504.3 である。将来の版とは扱わない。

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- scope と timeout を先に決める。短時間の単体判定は small / 60秒側に、結合・システム側は medium / large 側に置き、smoke のような選別 tag は「速く全体の生死を見る実行」と定義して CI の入口に置く。tag の具体的な除外構文は runner の版で確認する。
- double の選択は5分類で行う。値の返却だけなら Stub、動作する軽量実装なら Fake、何もしない埋め合わせなら Dummy、記録して事後確認なら Spy、呼び出し期待そのものを検証したい場合だけ Mock を使う。classical を既定にし、mockist（ collaborator の mock 化）は「呼び出し順序・回数こそ仕様」という場合に限定する。
- hermetic チェックは Bazel 定義をそのまま手順化する。(1) 入力が宣言済み source・build 入力と runner 保証に閉じているか、(2) 読み取りは runfiles など宣言済み経路か、(3) 書き込みは TEST_TMPDIR / TEST_UNDECLARED_OUTPUTS_DIR の配下か、を確認する。外れがあれば scope を上げるか Fake に置き換える。
- flaky 対応は bounded retry と history 分離で行う。(1) retry 上限 N を明示し（`rerunFailingTestsCount` 相当）、N 以内 pass は Flake として印付けする。(2) Deflake 相当として失敗したテストのみを失敗時 revision で再実行し、全体再実行で時間を溶かさない。(3) revision ごとの pass / fail / flake history を残し、同 revision での再現性を flaky 判定の根拠にする。
- 隔離（quarantine）の具体的手法は本 inventory の検証範囲外であり、出典の契約規定として扱わない。設計案としては、flake history で再発するテストを通常の合否ゲートから分離し、修正まで追跡する別枠で実行する。分離の機構（tag・別 job・除外指定）は runner の版で確認し、黙って成功扱いにしない。

## 避ける使い方

- 非 hermetic なテストを retry で黙らせる。未宣言の外部状態・ネットワーク・共有ファイルへの依存が残ったままでは、retry 上限を上げても結果は宣言済み入力に依存しないままになる。
- 書き込みを runfiles や宣言外の場所に行う。読み取り専用と書き込み可能場所の区別を崩し、並列・再実行時の相互干渉を招く。
- 呼び出し期待の検証が不要な箇所まで Mock にする。テストごとの期待設定が増え、実装の呼び出し方への結合（implementation coupling）が強くなり、リファクタリングのたびにテストが壊れる。
- 無制限の retry で flaky を隠す。上限 N と Flake 印付け・history 蓄積なしの retry は、「同一構成で pass も fail もする」という定義の検出を不可能にし、build 成功の意味を壊す。
- 失敗テスト以外も含めて全体を再実行して flaky と断定する。revision 固定なし・失敗分離なしの再実行では、環境差と真の flaky を区別できない。
- smoke / exclusive / external / manual の tag を意味確認なしに運用する。本ドキュメントが確認したのは tag 名の存在のみであり、選別意味は runner の版で確認せずに使わない。

## 適用版と本番での注意

- 適用版: Bazel Test Encyclopedia（rolling current docs、2026-09-27 検証）、Martin Fowler Mocks Aren't Stubs（2007-01-02 記事）、Jenkins Flaky Test Handler v1.3.172.v35b_30de7fa_5b_（要求 Jenkins 2.504.3）。将来の改訂とは扱わない。
- 再確認期限: `official_docs` 2件と `maintainer_article` 1件はいずれも TTL 90日のため 2026-12-26。`testing` は `config/freshness.json` に技術 TTL の定義がないため技術期限は適用されない。文書の期限は最短に合わせて 2026-12-26。
- 文書の trust は `maintainer` とする。参照 source のうち最も低い trust（Fowler 記事の `maintainer`）を超えないためである。
- 未確認事項（本調査の範囲外として推測で埋めない）: 各 Bazel tag の選別意味の詳細、size と timeout の上書き条件、5分類それぞれの厳密な定義文、Surefire `rerunFailingTestsCount` の N の上限値や並列実行との相互作用、Deflake の revision 指定の既定動作、quarantine 機構の標準手順。これらは各出典の該当節を別途確認する。
- 本ドキュメントの推奨構成（scope 割付け・double 選択・bounded retry + history + 隔離枠）は設計案であり、単一事例の一般化ではない。timeout 値・retry 上限 N・隔離条件は対象ワークロードの所要時間と失敗率に照らして決める。

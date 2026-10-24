---
{
  "id": "http-conditional-write-validator-atomicity-boundary",
  "title": "HTTP条件付き更新: If-Matchの版照合・428/412・PUT変換後validatorの境界",
  "kind": "knowledge",
  "technology": "http",
  "version": "RFC 9110 / STD 97 (June 2022), RFC 6585 (April 2012); conditional-write clauses verified 2026-10-24 UTC; no server runtime test",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "conditional-write",
    "If-Match",
    "If-None-Match",
    "ETag",
    "lost-update",
    "428",
    "412",
    "PUT",
    "atomicity",
    "idempotency"
  ],
  "sources": [
    {
      "id": "rfc9110-conditional-write-admission-20261024",
      "url": "https://www.rfc-editor.org/rfc/rfc9110.html",
      "type": "official_docs"
    },
    {
      "id": "rfc6585-428-conditional-write-20261024",
      "url": "https://www.rfc-editor.org/rfc/rfc6585.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-24",
  "expires_at": "2027-01-22",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# 条件付き更新を存在確認や再送の重複排除と取り違えない

## 問いと判断

GETした設定を編集してPUTするAPIで、`If-Match`を付ければlost updateを防げるか。条件を確認した直後の別writer、保存時の正規化、応答喪失後の再送をどう扱うか。

**判断: 期待する表現のvalidator、更新を受理する原子的境界、操作の重複排除を別々に設計する。** `If-Match: *`だけでは読んだ版を保護できない。ヘッダを付けた事実だけでサーバー実装の競合安全性も証明できない。

これは新機能速報ではなく、既存標準の未収録部分を補う調査である。[キャッシュ再検証](caching-cache-control-conditional.md)は保存応答の再利用、[Range再開](range-if-range-resume-completion-boundary.md)は部分取得、[再試行](retry-idempotency.md)は再送可否を扱う。本稿は**オリジンサーバーの条件付き書込み受理と次の編集に使うvalidator**へ限定する。執筆前に`just index`とKB検索を行い、`If-Match`は既存キャッシュ文書だけ、`If-Match 428`は該当なしだった。

## 仕様で確認した契約

### 1. 比較対象と失敗時の分岐

[RFC 9110 §13.1.1][if-match]では、`If-Match`のタグをstrong比較する。`*`は現在表現の存在条件であり、特定版との一致条件ではない。偽なら操作を実行してはならず、通常は412へ進む。ただし、要求された変更が既に適用済みと判断できる場合の2xx応答は許される。[§13.2.2][precedence]

[§13.1.2][if-none-match]の`If-None-Match: *`は既存表現への上書きを防ぐ作成条件として使える。偽なら、GET/HEADは304、それ以外は412となる。If-Matchのstrong比較を、If-None-Matchのweak比較と共通化しない。

### 2. 428と評価順

[RFC 6585 §3][required]の428は、オリジンが条件付き要求を必要とする通知である。再送方法を説明すべきで、応答をキャッシュ保存してはならない。428を採用すること自体は全サーバーの義務ではない。

[RFC 9110 §13.2.1][evaluate]は通常のリクエスト検査を先に行う。無条件なら2xx/412以外となる要求では前提条件を無視する。通常検査後、本文処理または操作の直前に評価する。`If-Match`があれば`If-Unmodified-Since`は無視し、二つをAND条件にしない。[§13.1.4][date-condition]

### 3. 成功PUTとvalidator

[§9.3.4][put]では、受信表現を変換せず保存し、新表現を反映する場合以外、成功PUT応答へETagやLast-Modifiedを送ってはならない。成功しても、その後のGETまで状態が維持される保証はない。

## 実装・運用への落とし込み（独自設計）

以下は本稿の設計案であり、RFCが特定DB、SQL、ロック、重複排除ストアを要求するという意味ではない。

### 期待版を要求の目的と結び付ける

- **読んだ版の編集**には、そのGETで取得したstrong ETagを保持し、対象URI・tenant・表現選択条件と組にする。`If-Match: "rev-41"`と`If-Match: *`を同じ「条件あり」として受付けるだけでは、編集競合の保護にならない。
- **存在すれば更新**という業務用途ならwildcard existenceを選べるが、「自分が読んだ後に変わっていない」というUI表示には使わない。版必須APIではwildcardを許可しない方針と、そのエラー応答をAPI契約に明示する。これは追加の業務方針であり、RFCの文法違反という扱いではない。
- **存在しなければ作成**ならIf-None-Matchの作成条件を用い、別writerとの同時作成も一緒に試験する。PUT成功応答を受ける前の接続断は、未作成の証拠として扱わない。

### 条件判定と状態更新を一体化する

例としてwriter A/Bが同じ版41を読む。Aが版42を確定した後、Bの版41に基づく編集を拒否したい。gatewayで一度ETagを比較してから無条件のDB更新を行うだけでは、その間にAが入り込める。

そこで**現在版の照合と書込みを同じ原子的境界**に置く。例えば、DBのconditional update、適切な隔離・ロック、永続化層のcompare-and-swap（CAS）で実装し、競合時に変更件数や確定結果を検査する。単に同じtransactionを開始しただけで十分とせず、選んだ隔離レベルと書込み経路で並行writerを実際に防げるか確認する。

GET本文とETagも同じsnapshotから作る。本文を読んだ後で別に最新revisionを取得すると、古い内容と新しい条件を組み合わせて更新を許してしまう。背景job・管理画面・bulk更新も同じ版更新規則へ参加させる。HTTP入口だけのチェックを唯一のwriterと仮定しない。

表現のvalidatorと業務全体のrevisionも同一視しない。表示しない属性や別resourceの状態まで条件に含めたい場合は、その判断対象とrevision更新規則を別途定める。表現を選ぶ条件、削除後の再作成、複数形式のGETも設計レビューへ含める。

### 428、412、応答喪失を別の復旧経路にする

このAPIでは、必要な前提条件の欠落を428、評価した条件の偽を412として説明する。428には取得・再送に必要な手順を示す。401/403やredirectを機械的に428/412へ置き換えず、通常検査との順序を維持する。

412を受けたclientは現在表現を再GETし、元の編集意図を再評価・マージする。**新ETagだけへ差替えて古いbodyを再送しない。** そうすると保護したかった他writerの変更を、条件を満たす要求として上書きしてしまう。

応答喪失後の再送で古いIf-Matchが不一致になっても、最初の要求が失敗したとは限らない。反対に、現在値が期待どおりでも、自分の操作が実行された証拠とは限らない。特にcounterや予約のように各操作の意味が重要な場合、表現一致だけを「同じ操作の成功」と判定する実装を避ける。

ここでの**expected revisionとidempotencyは別の軸**である。前者は更新の基準状態、後者は同一操作を再送したかを識別する設計。必要なら操作ID、request fingerprint、確定結果の記録を設け、同一操作の再送と別操作を区別する。2xx例外を必須機能にせず、その採用条件と応答契約を決める。RFC 9110の例外を「同じuser agentの直前要求だけ」と狭めたり、逆に不一致でも新規更新を実行できる許可へ広げたりしない。

### 保存時の変換があるAPIは次の編集用の状態を取り直す

サーバーがPUT本文を正規化し、表現へ既定値を追加する設計では、送信bodyと保存後の表現が同一かを確認する。DB内部でのエンコード変更だけを機械的に「表現の変換」と呼ばず、HTTP上のrepresentation dataとの対応で判断する。

変換がある場合は、成功PUTへ新ETagを付ければよいという実装にしない。本稿のclient案では成功後にGETし、返された本文とvalidatorを一組で編集基準へ保存する。そのGETが他writerの変更後の状態を返す可能性も受け入れる。成功PUTから次のGETまでを暗黙のロック期間として扱わない。

## 受け入れ試験案と未確認範囲

以下は自サービス向けの試験案であり、今回HTTPサーバーのruntime試験は実行していない。応答コードだけでなく永続状態と業務効果も確認する。

1. 同じ版を読んだ二つのwriterをbarrierで同時実行し、片方の成功後に古い編集が確定しないことを確認する。ETag確認後・保存前にも別writerを差し込む。
2. strong tag一致、不一致、weak tag、wildcard existenceを分ける。wildcardが真でも、特定版を読んだ証拠にはならないことをUI・API両方で検査する。
3. 条件欠落の428と条件偽の412を区別する。認証失敗を条件失敗へ置換しない。428が中継cacheへ保存されないことも検査する。
4. If-Matchが一致しIf-Unmodified-Sinceが偽の場合、および逆の組合せを作り、日付条件が版条件を上書きしないことを確認する。
5. `If-None-Match: *`で同時作成し、二重作成や上書きがないこと、条件偽を304として返さないことを検査する。
6. 更新確定後に応答を失わせて再送する。412から初回失敗と断定せず、採用した2xx例外でも同じ業務効果を再実行しないことを確認する。
7. PUTで内容を正規化する経路とそのまま保存する経路を分け、前者が成功応答へvalidatorを送らないことを検査する。後続GET直前の第三者更新も再現する。
8. 古い本文＋新ETagの組を作り得るGET実装、別tenantへのETag転用、背景jobによる更新を故障注入対象にする。検索evalの成功をこれらの適合試験の代わりにしない。

## 出典・適用版・限界

- RFC 9110はSTD 97、2022年6月公開。RFC 6585は2012年4月公開のStandards Track。本調査は2026年10月の仕様変更を主張しない。
- 両一次ページをnative webで2026-10-24 UTCに開き、上記節とCopyright Noticeを確認した。RFC 9110はBCP 78 / IETF Trust Legal Provisions、Code ComponentsにRevised BSDを指定する。RFC 6585のCode Componentsの表記はSimplified BSDである。独自要約で、原文・コードの転載やmodule化はしていない。
- 公式文書90日TTLに従い再確認期限は2027-01-22。既存catalogの取得日は変更せず、今回参照する新規記録を作った。
- 特定framework・proxy・CDNの実装適合、DB隔離の実証、PATCHの各media type、複数resourceへの原子的更新、ETag生成アルゴリズムの暗号学的評価は範囲外。条件付き更新を認可、fencing、exactly-once全体の代替として扱わない。

[if-match]: https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1
[if-none-match]: https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.2
[date-condition]: https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.4
[evaluate]: https://www.rfc-editor.org/rfc/rfc9110.html#section-13.2.1
[precedence]: https://www.rfc-editor.org/rfc/rfc9110.html#section-13.2.2
[put]: https://www.rfc-editor.org/rfc/rfc9110.html#section-9.3.4
[required]: https://www.rfc-editor.org/rfc/rfc6585.html#section-3

---
{
  "id": "go-uuid-parse-v7-order-migration-boundary",
  "title": "Go 1.27 uuid: Parseの受入れ範囲・UUIDv7の並び順・移行境界",
  "kind": "knowledge",
  "technology": "go",
  "version": "Go 1.27.1 (862c888e612ac346c7c4d99c9392bdfd265f33b0; 2026-09-01); uuid導入は1.27.0 (2026-08-19); RFC 9562 (2024-05); Linux/amd64の限定実測、2026-10-05 UTC確認",
  "tags": [
    "research-domain:backend",
    "go",
    "uuid",
    "UUIDv7",
    "Parse",
    "canonicalization",
    "monotonicity",
    "clock-rollback",
    "migration"
  ],
  "sources": [
    {
      "id": "go127-uuid-release-20261005",
      "url": "https://go.dev/doc/go1.27",
      "type": "release_notes"
    },
    {
      "id": "go127-uuid-release-history-20261005",
      "url": "https://go.dev/doc/devel/release",
      "type": "release_notes"
    },
    {
      "id": "go1271-uuid-api-20261005",
      "url": "https://pkg.go.dev/uuid@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go1271-uuid-implementation-20261005",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/uuid/uuid.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "go1271-uuid-tests-20261005",
      "url": "https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/uuid/uuid_test.go",
      "type": "github_repository_analysis"
    },
    {
      "id": "rfc9562-go-uuid-acceptance-order-20261005",
      "url": "https://www.rfc-editor.org/rfc/rfc9562.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-05",
  "expires_at": "2026-11-04",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Go標準uuidへの移行で、受理できるIDと並べられるIDを分ける

## 問いと今回の追加理由

Go 1.27の標準`uuid`へ移れば、外部入力の`Parse`成功だけで「このAPIが発行するUUIDv7」と判定できるか。`NewV7`を選べば、複数workerの業務イベントをID順に処理できるか。どちらも、packageの保証にアプリケーション固有の意味を加えてはいけない。

[Go 1.27 release notes](https://go.dev/doc/go1.27#uuid)で標準packageの追加を確認した。[release history](https://go.dev/doc/devel/release#go1.27.0)による導入版1.27.0の公開日は2026-08-19、今回sourceとruntimeを調べた1.27.1は2026-09-01である。これは標準package導入に伴う採用判断であり、取得日に公開された機能や、将来を含む最新版という説明ではない。

既存の[HTTP再試行](../http/retry-idempotency.md)は一意キーを使う再送契約、[transactional outbox](../messaging/transactional-outbox.md)はDB更新とイベント記録の原子性を扱う。本稿は、それらに渡すGoのUUID値・入力形式・生成順の契約に絞る。全knowledge・patterns・modulesの横断検索で、UUIDはこれらの例や補足に現れたが、標準`uuid`の解析・生成契約を扱う文書はなかった。

実務上の結論は次の三つである。

- 特定の生成algorithmが契約なら`NewV4`または`NewV7`を明示する
- `Parse`の後で、必要な場合だけ表記・version・variantという別々の制約を適用する
- UUIDv7を索引の局所性に使う判断と、業務の順序・commit時刻・認可の保証を分ける

## 公開API: Newは現在V4、Parseは複数表記を受け付ける

[go1.27.1 API](https://pkg.go.dev/uuid@go1.27.1)では、`UUID`は`[16]byte`で、`==`比較やmap keyに使える。`New()`は現時点で`NewV4()`と等価だが、特定algorithmが不要な用途の入口である。V4を外部契約にするのに、現在の`New`の実装だけへ依存しない。V4の乱数部分は122bit、V7は上位48bitのtimestampと少なくとも62bitの乱数部分を持つ。

`Parse`は36文字のhyphen付き、波括弧付き、`urn:uuid:`付き、32文字のhyphenなしを受け付ける。hexadecimalの英字は大小どちらでもよい。`String`と`MarshalText`は小文字・hyphen付きへ揃え、`UnmarshalText`も`Parse`と同じ入力形式を受け付ける。`MustParse`は失敗時panicなので、未検証のrequest入力にそのまま使わない。

このpackageが受け付ける集合と、自分のAPIが許可する集合を明示的に区別する。後者を厳格にするのは設計判断であり、すべてのUUID利用者に小文字やV7を強制する標準規則ではない。

## 固定実装: Parse成功はversion・variantの承認ではない

[固定uuid.go](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/uuid/uuid.go)の`Parse`は`UnmarshalText`へ委譲する。`UnmarshalText`が確かめるのは長さ・対応するprefix/braces・hyphen位置・hexadecimalへの復号で、**version/variantの許可リスト検査はない**。

具体的には、Nil UUID、Max UUIDに加えて、`11111111-1111-f111-0111-111111111111`も1.27.1の`Parse`に成功する。最後の例は任意に作った試験値であり、RFCのV7として発行されたことを意味しない。Nil/Max自体は規格で定義された特殊値なので、これらを「構文不正」と説明しない。

[RFC 9562 §4.1–4.2](https://www.rfc-editor.org/rfc/rfc9562.html#section-4.1)では、versionはoctet 6の上位4bit、ここで対象にするRFC variantはoctet 8の上位2bit=`10`である。したがって「RFC variantのV7だけ」という業務要件なら、`u[6] >> 4 == 7`と`u[8] >> 6 == 2`を別に検証できる。通常の不透明なIDとして使うだけなら、不要なbit制約を追加しない。

### 正規化と拒否のどちらが必要かを決める

以下は独自の入力検証例であり、Go公式の推奨wrapperではない。APIが小文字・hyphen付きV7だけを契約としている場合を示す。

```go
func parseCanonicalV7(s string) (uuid.UUID, error) {
    u, err := uuid.Parse(s)
    if err != nil {
        return uuid.Nil(), err
    }
    if s != u.String() {
        return uuid.Nil(), errors.New("noncanonical UUID text")
    }
    if u[6]>>4 != 7 || u[8]>>6 != 2 {
        return uuid.Nil(), errors.New("UUIDv7 with RFC variant required")
    }
    return u, nil
}
```

この関数には`errors`と`uuid`のimportが必要である。`error != nil`なら返されたNilを有効値として使わない。大文字やhyphenなしも受け付けるAPIでは、最初の文字列表記比較を外し、parsed valueまたは`u.String()`を内部の比較・保存形式に使う方針にできる。

同じ128bit値を表す大文字、hyphenなし、URNをそれぞれ生文字列の重複排除keyにすると、意味的に同じIDが別keyになる。これはwrapperを導入する設計上の理由である。ただし、署名されたraw requestや既存idempotency protocolのkeyを、契約の確認なしに勝手に書き換えるという意味ではない。

### 受理形式を一般化しすぎない

固定実装はhexadecimalの大小を許しても、`URN:UUID:`という大文字prefix、前後の空白、hyphenなしの文字列を波括弧で包んだ形式は受理しない。たとえば「hyphenなしを許す」「bracesを許す」という二つの事実から、両者を組み合わせた形式も許すと推論しない。

さらに、`UnmarshalText`は一時的な`dst`を作り、成功時にだけreceiverへ代入する。**解析失敗で既存receiverはゼロ化されない**。同じ変数を再利用し、errorを無視して後続処理すると、直前のIDで処理を続ける危険がある。これは1.27.1の実装観察であり、errorを無視してよいという契約ではない。

## UUIDv7: 整列の基準は業務完了順ではない

[NewV7のAPI](https://pkg.go.dev/uuid@go1.27.1#NewV7)は、system clockが後退する場合を除き増加する順にUUIDを生成すると説明する。`Compare`はbig-endianのbyte順を使う。[RFC 9562 §6.11](https://www.rfc-editor.org/rfc/rfc9562.html#section-6.11)が示すUUIDv7の整列用途とも対応する。

Go 1.27.1では次の実装になっている。将来版の不変条件としてではなく、固定sourceで確認した機構として読む。

1. package内の`v7mu`でtimestamp用stateを直列化する
2. `time.Now()`のUnix秒とnanosecondから、millisecond部分48bitと1/4096 millisecond単位の12bit部分を組み立てる
3. Unix秒が前回より小さければ以前のtimestampを引き継がない。それ以外で今回のtimestampが前回以下なら、前回値に1を加える
4. stateを更新してlockを解放し、下位の乱数部分とversion/variantを設定して返す

### 時計後退と、論理的な繰上がり

後退検出の具体的な条件は`v7lastSecs > secs`である。同じUnix秒の中でclockが後退した場合と、前のUnix秒へ戻る場合の処理は同一ではない。公開APIがclock後退を例外としている以上、アプリケーションは「少しの後退なら常に順序を守る」といった実装詳細に基づく強い保証を外部へ出さない。

同時刻の連続生成では12bit部分を使い切っても、1.27.1は論理timestampを進める。後述の仮想時計試験では、**同じ瞬間の4097個で、実際の時計が0進行でも埋込millisecondは1増加**した。このためUUIDから取り出した時刻を、そのまま監査上の正確な生成時刻、期限判定、DB commit時刻に使わない。必要な実測時刻やcommit情報は別の属性として保存する、というのが独自の設計判断である。

### process内の採番と、到着・commitの順序

stateはprocess内のpackage変数で、別processや別hostと共有する機構はこのpackageにない。mutexの解放は乱数生成・関数returnより前なので、並行呼出しのreturn順やchannelへの送信順を、timestamp採番の順序と同一視しない。これらはsourceからの推論であり、全schedulerで順序逆転を再現した試験結果ではない。

たとえ一つのgoroutineがA、Bの順にUUIDを生成しても、AのDB transactionが遅延し、Bが先にcommitすることはアプリケーション側で起こり得る。これは独自の反例である。UUID順だけで配送offset、因果関係、全hostの発生順や「これより古い仕事は完了した」というwatermarkを作らない。順序要件に合った別のsequence・log位置・完了記録を設計する。

## 移行時に確かめること（独自の採用手順）

1. **生成algorithmを固定する必要を確認する。** 外部schemaにV4/V7を約束するなら専用関数を選び、既存のIDを一括変換することと新規発行を切り替えることを分ける
2. **既存入力を比較する。** 大小文字、URN、hyphenなし、Nil、Max、別version、誤ったvariant、空白、malformed hexの受理・拒否を旧parserと照合する。過去の保存値を新しいV7専用policyで読めなくしない
3. **Goの型とadapterを確認する。** 標準`uuid.UUID`は1.27.1のmethod集合では`sql.Scanner`も`driver.Valuer`も実装しない。Text系interfaceの実装だけで、既存DB driver/ORMへそのまま渡せると判断しない。driver固有の型対応、変換、NULL表現、round-tripを対象環境で試験する
4. **比較形式を揃える。** Go内ではparsed valueの`Compare`を基準にし、文字列比較するなら同じ正規表記に揃える。DBの型・collation・binary byte orderまでGoと同じとは推測しない
5. **業務成功は別に記録する。** UUID発行成功やIDの大小を、永続化・publish・commitの成功に置き換えない。collision時の扱いも、一意制約や保存処理のerror policyとして検証する

[RFC 9562 §8](https://www.rfc-editor.org/rfc/rfc9562.html#section-8)は、UUIDを持っているだけでaccessを与えるsecurity capabilityに使ってはならないとしている。V4/V7のどちらでもresource IDと認可を分ける。V7の時刻情報を公開してよいかも確認する。CSPRNGを使用することと、認可・完全な衝突不可能性の証明は別である。

## 実行した試験と、そこから言える範囲

2026-10-05 UTC、locked toolchainのGo 1.27.1・Linux/amd64で次を実行した。

| 試験 | 確認できたこと | 確認していないこと |
|---|---|---|
| 独自`TestParseFormsAndPolicy` | 五つの表記が同じ値へparseされ、上記wrapperは正規V7だけを受理。Nil/Max、reserved version、別version、非RFC variantをwrapperで拒否 | すべての旧UUID libraryとの互換性、fuzz全域 |
| 独自`TestFailedUnmarshalKeepsReceiver` | `UnmarshalText`失敗後も直前のUUIDを保持 | すべてのencoding層を通した再利用時の挙動 |
| 独自`TestFixedClockLogicalAdvanceAndRollback` | `synctest`で4097個の連続増加と埋込時刻の1ms先行を確認。別bubbleでUnix秒を戻すと、後で生成したUUIDが小さくなる | 実OSの時刻変更、NTP、process再起動、異なるhostのclock skew |
| 独自`TestUUIDTypeIsNotSQLAdapter` | 標準型の`sql.Scanner`/`driver.Valuer`へのtype assertionはfalse | 個々のdriver・ORMが独自に扱える型かどうか |
| 独自`TestConcurrentObservedUnique` | 16 goroutine・合計4096個で重複を観測せず、race detectorも成功 | 一般的な衝突確率の測定、return順・配送順の保証 |
| 上流uuid package tests | `go test -race -count=1 uuid`成功 | 異なるOS/architecture、実運用DB、性能比較 |

独自の5テストは`go test -race -count=1 -v <試験ファイル>`で成功した。probeは調査用の一時ファイルであり、repositoryのmoduleや恒常的なruntime testとして追加していない。検索evalはこの文書を見つける検査であって、UUIDの動作試験の代替ではない。

[固定uuid_test.go](https://github.com/golang/go/blob/862c888e612ac346c7c4d99c9392bdfd265f33b0/src/uuid/uuid_test.go)では、`TestNewV7Millis`が新しいbubbleによるclock後退を含むmillisecond値、`TestNewV7Collision`が固定時刻の連続生成とfraction繰上がり後の増加順を検証している。これを分散順序や衝突ゼロの数学的証明とは扱わない。

## 出典・版固定・未確認事項

公式tag `go1.27.1`は`862c888e612ac346c7c4d99c9392bdfd265f33b0`に解決した。commit固定の`uuid.go`、`uuid_test.go`、`LICENSE`をraw取得および公式Gitのsparse cloneで読み、locked toolchain内の同名ファイルとのbyte一致を`cmp`で確認した。GitHubのcommit固定web表示とtestページの取得は失敗したが、sourceを未読のまま推測せず、これらの取得経路で補った。API・release notes・release history・RFC本文はwebで開いている。

Go sourceとAPI documentationはBSD-3-Clause、Goサイトの文章は[CC-BY-4.0](https://go.dev/copyright)、RFCは冒頭のIETF Trust/BCP 78およびCode Components向けRevised BSD条件を確認した。上流実装やtest本文の転載はなく、本文は独自の日本語要約と設計上の反例、短い独自wrapperである。

macOS/Windows、32bit、DB/ORMを含むmigration、実clock rollback、process間の生成、性能・索引効率は未検証。Go 1.27.1以外のpatchに同じ内部stateや後退判定が維持されるとは断定しない。将来版や別libraryへ移すときは公開APIと対象sourceを再確認する。

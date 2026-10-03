---
{
  "id": "go-os-root-filesystem-confinement-boundary",
  "title": "Go os.Root: symlink・fs.Sub・patch版とファイルアクセス範囲の境界",
  "kind": "knowledge",
  "technology": "go",
  "version": "API / Linux probe: Go 1.27.1; Root導入1.24 / 拡張1.25; GO-2026-4970修正1.25.12・1.26.5・1.27.0-rc.2",
  "tags": [
    "research-domain:backend",
    "os.Root",
    "filesystem",
    "symlink",
    "path-traversal",
    "TOCTOU",
    "DirFS",
    "fs.Sub",
    "Localize",
    "tenant-isolation",
    "CVE-2026-39822"
  ],
  "sources": [
    {
      "id": "go127-os-root-confinement-api-20261003",
      "url": "https://pkg.go.dev/os@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-filepath-localize-confinement-20261003",
      "url": "https://pkg.go.dev/path/filepath@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go127-iofs-sub-confinement-20261003",
      "url": "https://pkg.go.dev/io/fs@go1.27.1",
      "type": "official_docs"
    },
    {
      "id": "go-osroot-traversal-article-20261003",
      "url": "https://go.dev/blog/osroot",
      "type": "maintainer_article"
    },
    {
      "id": "go-os-root-trailing-slash-advisory-20261003",
      "url": "https://pkg.go.dev/vuln/GO-2026-4970",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2027-01-01",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Go os.Root の導入で、何を閉じ込められるか

## 問い・結論・版

外から受け取ったファイル名を固定 directory に結合して読む backend を考える。`filepath.IsLocal`、`os.DirFS`、`fs.Sub`、`os.Root` は、同じアクセス境界を作るのか。

結論は、入力の文法、解決先の directory、利用者ごとの認可を分けること。Root を使っても、その Root が全 tenant を含めば tenant 間の認可境界にはならない。本稿は Go のローカル file API を選ぶための未収録分野の整理であり、新着リリースの紹介や既存 Python tarfile 記事の分割ではない。

[os API](https://pkg.go.dev/os@go1.27.1) の表示版・公開日は **go1.27.1 / 2026-09-01**。Root、OpenRoot、Root.FS は1.24追加、ReadFile・WriteFile・MkdirAll・Rename・Symlink等は1.25追加である。古い紹介記事の「未対応メソッド」を現在の制約として使わない。以下は公式契約、独自の限定実測、設計案を区別する。

## 字句検査と、実際の open を分ける

[filepath API](https://pkg.go.dev/path/filepath@go1.27.1#IsLocal) の `IsLocal` は、空名・絶対 path・directory 外へ出る相対 path などを字句解析だけで判定する。既存 symlink の行き先は調べない。`Localize` は io/fs 形式を OS path に変換し、その OS で表現できない名も拒否する。成功値は IsLocal を満たす。単に slash を置換する `FromSlash` と同じ検査ではない。

[Go 開発元の記事](https://go.dev/blog/osroot) は、`EvalSymlinks` で検査してから通常の open を呼んでも、その間の link 差替えという TOCTOU が残ることを説明する。Root の対象は、root 内に収まる相対成分と symlink を解決しながら行うアクセスであり、symlink の全面禁止ではない。

設計上の含意: 入力の `..` を全て削除してから安全と判断しない。まず受付形式を決め、最後のファイル操作まで root-relative API を通す。検査済みの文字列を作れたことと、その後の対象が固定されたことを同一視しない。

## Root と io/fs の二つの path 契約

[Root API](https://pkg.go.dev/os@go1.27.1#Root) の境界は次の通り。

- root 外への解決と絶対 symlink は拒否する。root 内へ向く相対 symlink は許す
- `os.OpenRoot` 自体は指定 directory の symlink をたどる。どの directory を root に選んでよいかは呼出側の責任
- `Root.Symlink` の oldname は検査されない。外向き link の作成成功は、その先の読み書きを許可した意味ではない

一方、[io/fs Path Names](https://pkg.go.dev/io/fs@go1.27.1#hdr-Path_Names) は全 OS で slash 区切りであり、途中の `.`・`..`・空成分や先頭/末尾 slash を許さない。root 自体を表す `.` は例外。したがって Root の直接メソッドで使える名前でも、`Root.FS()` を通すと文法エラーになり得る。

`fs.ValidPath` は backslash や colon を文字として許すが、それらを path 区切りとして再解釈してよいわけではない。[ValidPath](https://pkg.go.dev/io/fs@go1.27.1#ValidPath) と [Localize](https://pkg.go.dev/path/filepath@go1.27.1#Localize) を区別する。OS-native な受付名なら IsLocal、slash-separated な受付名なら Localize を検討し、io/fs に渡すときはその文法を維持する。

### fs.Sub を tenant 専用 Root と扱わない

[fs.Sub の契約](https://pkg.go.dev/io/fs@go1.27.1#Sub) は、SubFS を実装しない FS では directory prefix を付けて元の FS へ委譲するもの。`fs.Sub(os.DirFS(...), ...)` は symlink escape を防がず、directory が現在存在するかの確認もしない。

さらに今回、親 Root が `alice` と `bob` の両方を含む場合を実測した。`alice/cross-tenant` を `../bob/secret.txt` に向く link とし、次を比較した。

| 呼出し側の構成 | Linux go1.27.1 での観察 |
|---|---|
| 親の `Root.FS()` を `fs.Sub(..., "alice")` で包む | cross-tenant を読み、bob の試験データを取得 |
| 親から `Root.OpenRoot("alice")` を作り、その `FS()` を使う | 同じ link の読取りを拒否 |

これは本調査の独自観察。前者は親の Root 外へ出ていないため、親の閉じ込め自体が破られたという意味ではない。tenant ごとに別境界が必要なら、信頼できる directory 選択から子 Root を作る設計を検討する。tenant名をリクエストから無検証で選ばせれば、子 Root を作るだけでも認可は成立しない。

## API が存在することと修正済みであることを分ける

[GO-2026-4970 / CVE-2026-39822](https://pkg.go.dev/vuln/GO-2026-4970) は **2026-07-07** 公開の公式報告である。Unix で最終成分が外向き symlink、かつ path 末尾が slash の場合、Root による open が外側へ到達する不具合を扱う。

報告の修正境界は **1.25.12、1.26.5、1.27.0-rc.2**。公式 JSON の範囲は1.25.12未満、1.26.0-0以上1.26.5未満、1.27.0-0以上1.27.0-rc.2未満である。古い系列の全 patch を一括で安全と判断しない。影響する公開操作には OpenInRoot、Root.Open、Root.OpenFile、Root.OpenRoot、Create、ReadFile、WriteFile が挙げられる。[機械可読の公式 report](https://vuln.go.dev/ID/GO-2026-4970.json)

設計判断: 「Go 1.24以上だから Root を使える」をパッチ管理の完了条件にしない。配布済み binary の build toolchain、OS、該当操作を確認する。上の版番号はこの一件の修正境界であり、現在の全脆弱性に対する推奨版一覧ではない。今回、古い脆弱版を導入して再現はしていない。

## Linux 1.27.1 で行った限定実測

標準ライブラリだけの独自 probe を、`go version go1.27.1 linux/amd64` で実行した。専用の一時 directory 内に root・外側の試験ファイル・link を作成し、実在する機密ファイルや他者のデータを使わなかった。以下は runtime の観察であり、全 OS の保証や競合耐性の証明ではない。

| 確認項目 | 観察結果 |
|---|---|
| `IsLocal("outside-link")` | true。ただし link は root 外の試験ファイルを指す |
| root 内へ向く相対 link を Root.ReadFile | 成功し inside を取得 |
| 外向き link、絶対 path で内側へ向く link、`../outside.txt` | 全て Root.ReadFile が拒否 |
| `sub/../inside.txt` | Root.ReadFile は成功。Root.FS 経由と Localize は拒否 |
| 外側 directory への link を末尾 slash 付きで Root.Open | 拒否。上記 CVE 条件の一例に対する修正後の観察 |
| 同じ outside-link を DirFS / Root.FS で読む | DirFS は outside を取得、Root.FS は拒否 |
| 外向き target を Root.Symlink で作り Root.Readlink | 作成成功、Readlink は `../outside.txt` を返す。Root での読み取りは拒否 |
| root directory 自体を指す alias を os.OpenRoot | 成功 |
| root を rename し、元の名前に別 directory と replacement データを作る | 既存 Root は移動後の元 directory から inside を取得 |
| rename 後の Root.Name | OpenRoot に渡した元の表記を保持 |

`fs.Sub` と子 Root の比較は前節の通り。同じ入力を全 API へ総当たりしたものではなく、権限不足や storage failure の網羅試験でもない。

## 閉じ込めを壊さない組み込み方（独自の設計案）

1. **root の選択を認可と結ぶ。** 利用者から受け取るのは業務上の object ID または許可した相対名とし、どの root を使うかは検証済みの tenant 対応から決める。初期 OpenRoot は root 自体の trust を代替しない
2. **最終操作を root 経由に保つ。** `Root.Name()` と入力名を再結合して通常の os.ReadFile に渡す fallback は作らない。Readlink が返した文字列も、通常の open に渡せる安全な絶対位置として扱わない。下位層へ FS を渡すなら、元が DirFS なのか Root.FS なのかを構築箇所で確認する
3. **狭い境界を先に作る。** tenant 子 directory の閉じ込めが必要なら、前節の fs.Sub 比較を受入試験へ入れる。子 Root の作成先まで第三者が差し替えられる構成は別の脅威として扱い、配置権限・所有者・管理経路を見直す
4. **symlink 方針を追加要件として決める。** 「外へ出ない」と「link を一切許さない」は違う。生成物を通常 API や別プロセスへ渡すなら、Root.Symlink が作れる外向き link の扱いを決める。後段の consumer が Root を使うという前提を無断で置かない
5. **同じ root 内の書換えも想定する。** Root は内容不変・複数操作の transaction・業務上の一度性を意味しない。必要なら開いた file の属性確認、入力サイズ制限、更新の競合制御、別領域での生成と公開手順を設計する。一般の権限検査や tenant 認可の代わりにしない
6. **失敗時に範囲を広げない。** root-relative open が失敗したら、通常の os.Open で再試行して成功させない。ファイル不在・入力不正・権限・閉じ込め拒否を運用上区別し、利用者へ返す情報と内部 path の露出量を制限する

## OS 制約・運用前の追加確認

[Root API の制約](https://pkg.go.dev/os@go1.27.1#Root) は mount 境界、bind mount、/proc、Unix device を禁止しない。Unix の Chmod・Chown・Chtimes は競合時に link target でなく link 側へ操作が及ぶ場合がある。GOOS=js は symlink 検査の TOCTOU を防げず、root 外アクセスを排除できない。これらを「全操作・全 OS の sandbox」と言い換えない。

[開発元の記事](https://go.dev/blog/osroot) は WASI の閉じ込めが runtime 実装にも依存すること、Unix と Windows の directory rename の差を説明する。API の存在だけで各 OS の同一性を仮定しない。

追加の受入試験案は、実運用の GOOS と filesystem 上での link・rename 競合、子 Root 選択時の差替え、Root.Close の寿命管理、特殊 file / mount の配置、書込み失敗後の状態、tenant 越境拒否である。長い path や深い directory のコスト、並行負荷、macOS・Windows・JS・WASI・Plan 9、Chmod 等の競合は今回未検証。検索 eval の成功はこれらの実行試験を代替しない。

## 出典・ライセンス・鮮度

一次資料は2026-10-03 UTCに実際に開いた。版固定 API 3件は go1.27.1、表示公開日2026-09-01、BSD-3-Clause を確認し、[Go LICENSE](https://go.dev/LICENSE) と照合した。API 本文の独立した更新日は表示されていない。

Damien Neil の紹介記事は2025-03-12公開で、[go.dev copyright](https://go.dev/copyright) の本文 CC-BY-4.0 / code BSD に従う。vulnerability report の published / modified は公式 JSON 上でともに2026-07-07T21:34:47Z、review_status は REVIEWED。[vulndb LICENSE](https://github.com/golang/vulndb/blob/master/LICENSE) の data directory は CC-BY-4.0 と確認した。

外部コード・サンプルの転載や改変はなく、独自要約と独自 probe の観察を記録した。repository 実装を固定 commit で解析した文書ではなく、module 昇格でもない。最短 TTL は90日なので明示期限は2027-01-01。新しい advisory、配布 runtime、OS 別の制約を再確認して更新する。

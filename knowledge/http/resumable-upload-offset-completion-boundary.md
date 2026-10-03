---
{
  "id": "http-resumable-upload-offset-completion-boundary",
  "title": "HTTP resumable upload 草案12: offset・完了・失われた最終応答の境界",
  "kind": "knowledge",
  "technology": "http",
  "version": "draft-ietf-httpbis-resumable-upload-12 (2026-07-06, interop 9, expires 2027-01-07), historical -11 (2026-03-02, interop 8, expired 2026-09-03), RFC 9110 and IANA temporary 104; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:api-distributed",
    "http",
    "resumable-upload",
    "Upload-Offset",
    "Upload-Complete",
    "Upload-Draft-Interop-Version",
    "104",
    "draft-12",
    "interop-9",
    "completion",
    "retry",
    "lost-response",
    "HEAD",
    "PATCH",
    "partial-put"
  ],
  "sources": [
    {
      "id": "http-resumable-upload12-completion-20261003",
      "url": "https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-12",
      "type": "official_docs"
    },
    {
      "id": "http-resumable-upload11-history-20261003",
      "url": "https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-11",
      "type": "official_docs"
    },
    {
      "id": "iana-http104-temporary-status-20261003",
      "url": "https://www.iana.org/assignments/http-status-codes",
      "type": "official_docs"
    },
    {
      "id": "rfc9110-partial-put-upload-20261003",
      "url": "https://www.rfc-editor.org/rfc/rfc9110.html",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/api-distributed.json"
  ]
}
---

# HTTP resumable upload 草案12: offset・完了・失われた最終応答の境界

## 問いと結論

大きなアップロードの接続が切れたとき、送信済み byte 数から再開してよいか。upload の完了表示があれば、元の POST の処理も成功したといえるか。**転送進捗、upload resource の完了、元リクエストの業務結果を別々に記録する。** 最終応答の喪失は、offset の回復だけでは解決しない。

対象は [draft-ietf-httpbis-resumable-upload-12](https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-12)。2026-10-03 の取得時点で Active Internet-Draft であり、RFC として確定した標準ではない。既存の [HTTP retry](retry-idempotency.md) は一般的な再試行を扱うのに対し、本稿は upload resource を使う再開と結果照合の境界を扱う。

## 版と相互運用の確認

| 根拠 | 確認した版・日付 | 適用範囲 |
|---|---|---|
| [草案12](https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-12) | 公開 2026-07-06、期限 2027-01-07、interop version 9 | 今回の作業中プロトコル |
| [旧草案11](https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-11) | 公開 2026-03-02、期限 2026-09-03、interop version 8 | 期限切れの履歴比較のみ |
| [IANA HTTP Status Codes](https://www.iana.org/assignments/http-status-codes) | 最終更新表示 2025-09-15、104 登録期限 2026-11-13 | 暫定 status code の登録 |
| [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.html) | STD 97、2022年6月 | HTTP semantics と partial PUT |

IANA の `104 Upload Resumption Supported` は **TEMPORARY** で、登録参照は draft-05 のままである。この登録番号から草案12の実装対応を推定しない。104 の登録期限と草案自体の期限は別であり、延長や RFC 化は本稿では確認できていない。

[旧草案11 §4.1.2 / Appendix B](https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-11#section-4.1.2) は client による完了宣言と interop 8 を規定していた。草案12は server 側の完了も扱い、interop 9 に変わった。単に同じ `Upload-Complete` 名を持つことを wire compatibility の証拠にしない。草案12 Appendix B では `Upload-Draft-Interop-Version` の交換・照合が必要で、不一致または欠落した 104 をクライアントは無視する。既存の tus v1 や古い草案の実装がそのまま対応するという主張ではない。

## 確認した契約（草案12の要約）

以下の根拠は [§4–4.7](https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-12#section-4)。大文字の規範語は、この作業中草案の対象条件に限る。

- `Upload-Offset` はアプリケーション側で処理された byte 数。transport ACK やクライアントの送信済み量とは異なる。サーバーの offset は減少禁止で、状態喪失時は resource を無効化する。再開では `HEAD`（推奨）または `GET` で確認し、`PATCH` / `application/partial-upload` の開始位置を合わせる
- offset 不一致の append は `409`、`Upload-Complete: ?0`、正しい offset を返す MUST がある。同じ upload の並列転送は禁止。進行中の creation/append と offset 取得を重ねてはいけない
- creation/append 応答の `Upload-Complete` は応答の意味を区別する。true は元リクエストの処理結果であり業務成功とは限らず、全 byte 受信前の早期エラーでも true になりうる。false は再開プロトコル側の応答で、完了済み upload への不正操作でも返る。false を「必ず未完了」と読むこともできない
- 完了後の resource 保持期間は実装判断で、即時削除も可能。失われた最終応答の replay は有効な再 append に対してサーバーが選べる動作で、必須の結果復元保証ではない

## partial PUT と取り違えない

[RFC 9110 §14.4–14.5](https://www.rfc-editor.org/rfc/rfc9110.html#section-14.5) は、`Content-Range` による partial PUT を private agreement に依存する機能として扱う。対応は一貫せず、旧来の PUT と後方互換ではない。非対応先では部分データを全体の置換として扱う危険があり、非対応 target の origin server は `400` を返す SHOULD がある。

したがって「Range download があるから upload も Content-Range を送ればよい」という推定を採用しない。既存 API が明示的に partial PUT を契約している場合と、この草案の upload resource / PATCH を利用する場合を分ける。新しいプロトコルを使う意思決定は、通信経路とクライアントの実装対応を確認してから行う。

## 実務での状態管理（独自の設計案）

### 進捗表示と結果表示を分ける

クライアントの状態を、例えば次の観測値として持つ。

- ローカル送信量: UI の暫定的な進捗表示にだけ使う
- サーバー確認 offset: 転送再開の位置として使う
- upload の完了状態: 追加転送を続けるかの判断に使う
- 元の operation の結果: HTTP 最終応答と、アプリケーションが用意した結果照合から判定する

offset がファイル長に等しくなっても、勝手に「登録完了」と表示しない。ユーザーには必要に応じて「転送済み、処理結果を確認中」と示す。これは草案で定義された新しい状態名ではなく、誤った成功通知を防ぐ UI・業務設計である。

### 接続断からの回復を一本化する

単一 upload の操作は一つの担当処理にまとめる。接続断を検知したら、その処理だけが offset 再取得と append を進め、別のタブや progress poller が同時に再開しないようにする。再開処理には回数・時間の予算と取消を設ける。特定の並行制御方式を草案の必須実装とする意図ではない。

受信した offset が送信済み量より小さい場合に備え、未確認部分の再送元を保持する。一方、大きい offset を無条件に不正とする固定ルールも避ける。草案12 §4.3.1 は代理取得で先に進んだ場合を想定するため、アプリケーションで再現できるデータかを確かめ、提供できなければ処理を失敗扱いにする。

### 最終応答が失われた場合

「元の操作をもう一回作る」前に、同じ操作の結果を照合する経路を設計する。具体的には、クライアントが発行した operation ID と upload resource の対応、結果の照会権限、最終状態の保持期間を API 側で明示する。これは草案の必須 endpoint や idempotency key 標準を追加したという意味ではない。

upload resource が消えていたら、元リクエストが失敗したと断定しない。保持期限切れ・成功後削除・途中の状態喪失をクライアントが区別できるか、アプリケーションの結果照合契約で決める。結果を取り戻せない場合は「結果不明」を残し、二重登録が問題になる操作の無条件再実行を避ける。

最終応答を再生する実装を選ぶ場合も、誰に・どの操作の・どの結果を返すかを認可し、保持容量と期限を決める。転送を再開できることと、業務副作用を exactly-once にできることは別の設計課題である。

### 経路の対応を確認する

クライアントが interim response を読めるか、reverse proxy が 104 を通すか、offset response が意図したサーバー状態を返すかを試験する。HTTP status code の認識、草案 interop version、upload resource の寿命を別のチェック項目にする。104 が見えないというだけで送信がサーバーに届かなかったと推定しない。

## 避ける使い方

- socket の送信済み byte 数や transport ACK を Upload-Offset の代用にする
- 途中の 2xx や `Upload-Complete: ?1` だけで、元リクエストの業務成功を通知する
- 同じ resource へ複数の PATCH を同時送信する、または progress poller が転送中に HEAD を送る
- resource の不在を「副作用なし」と読み替え、新しい作成要求を無条件に再送する
- server が必ず最終応答を replay する、あるいは永久に resource を保持すると仮定する
- draft-11 / interop 8 と draft-12 / interop 9、104 暫定登録と確定 RFC を混同する

## 受け入れ試験案（未実行）

1. 一部の byte をサーバーが処理する前に接続を落とし、ローカル送信量より小さい offset からの復旧を確認する
2. 最終処理だけ成功させて応答を破棄し、resource 保持あり・即時削除・replay なしの三つで二重作成が起きないか結果照合する
3. 全 byte 受信前に `400 + Upload-Complete: ?1` を返し、同じデータを繰り返し転送しないことを確認する。完了済み upload への不正操作に false が返る場合も、状態を未完了へ巻き戻さない
4. 古い offset の PATCH に対する 409 と更新後 offset を確認し、再開中の同時 HEAD/二重 PATCH を試験する
5. interop 8/9 不一致とヘッダー欠落で、104 を採用しないことを確認する
6. 104 を中継しない proxy を通し、クライアントが曖昧な状態を成功・失敗へ勝手に確定しないことを確認する
7. 状態を失ったサーバーが offset を巻き戻して同じ upload を継続せず、invalid resource として扱うことを確認する

## 適用版・証拠・未確認事項

- 2026-10-03 UTC に草案12本文、草案11の比較箇所、IANA 登録、RFC 9110 の該当節を開いて確認した。草案11は期限切れ旧版としてのみ参照する
- 取得時点の作業中草案を整理したもので、ブラウザー・SDK・proxy・特定サーバーの実装対応を確認したものではない。wire test、障害注入、性能試験、結果照合 endpoint の実装は未実行
- 草案12を実装する製品一覧、全 HTTP バージョンの取消方法、content coding 全組み合わせ、認可・quota の実装設計は範囲外
- 草案と RFC の Copyright Notice で BCP 78 / IETF Trust Legal Provisions と Code Components の Revised BSD 条件を確認。IANA の [Licensing Terms](https://www.iana.org/help/licensing-terms) は protocol registry data に CC0-1.0 を示す。本文は独自要約・設計案で、コード転載なし
- 明示期限は 2026-11-02。official_docs の TTL 90 日より短く取り、104 の登録期限 2026-11-13 より前に再確認する。将来の登録延長・draft 更新・RFC 化はこの取得日を延ばすだけでは確認できない

---
{
  "id": "sre-incident-response-postmortem-culture",
  "title": "SRE の incident response ライフサイクルと blameless postmortem 文化",
  "kind": "knowledge",
  "technology": "sre",
  "version": "SRE Book (2017) Ch.14 Managing Incidents / Ch.15 Postmortem Culture, SRE Workbook (2018) Ch.9 Incident Response (verified 2026-09-28)",
  "tags": [
    "research-domain:quality-operations",
    "sre",
    "incident response",
    "incident management",
    "Incident Commander",
    "postmortem",
    "blameless",
    "on-call",
    "mitigation",
    "rollback",
    "Wheel of Misfortune",
    "DiRT"
  ],
  "sources": [
    {
      "id": "sre-book-managing-incidents",
      "url": "https://sre.google/sre-book/managing-incidents/",
      "type": "official_docs"
    },
    {
      "id": "sre-book-postmortem-culture",
      "url": "https://sre.google/sre-book/postmortem-culture/",
      "type": "official_docs"
    },
    {
      "id": "sre-workbook-incident-response",
      "url": "https://sre.google/workbook/incident-response/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active"
}
---

# SRE の incident response ライフサイクルと blameless postmortem 文化

インシデント対応を「緩和を最優先するライフサイクル」として回し、事後の postmortem を blameless（責任追及なし）に運用して学習につなげる。以下は Google の SRE Book 第14章・第15章と SRE Workbook 第9章に記載された事実と、それを組み立てる設計案を分けて書く。3件はいずれも 2026-09-28 時点の検証済み目録に基づく。

## 要点（公式情報に記載された事実）

### 役割は ICS 由来で分離し、再帰的に委譲する（SRE Book 第14章 / Workbook 第9章）

- SRE Book 第14章は Incident Command System（ICS）に由来する役割分担を示す。Incident Commander、Ops、Communication、Planning の役割を分け、責任の分離（separation of responsibilities）を再帰的に適用する。[Managing Incidents](https://sre.google/sre-book/managing-incidents/)
- Workbook 第9章は IC・CL・OL の階層を示し、対応の規模に応じて委譲（delegation）する。3Cs である coordinate（調整）、communicate（連絡）、control（統制）が対応者の行動原則である。[Incident Response](https://sre.google/workbook/incident-response/)
- 対応中はライブの incident state 文書（live incident state document）で状況を共有し、担当の引き継ぎ（handoff）には明示的な acknowledgment を求める。[Managing Incidents](https://sre.google/sre-book/managing-incidents/)
- インシデント宣言は早く行う。検証済み範囲の基準は、2つ目のチームの助けが必要になった場合、顧客可視の影響がある場合、1時間解决しない場合である。[Managing Incidents](https://sre.google/sre-book/managing-incidents/)
- 同章の best practice は prioritize（優先順位付け）、prepare（準備）、trust（信頼）、practice（訓練）である。[Managing Incidents](https://sre.google/sre-book/managing-incidents/)

### 緩和を先行し、原因究明は後にする（Workbook 第9章）

- 対応の順序は mitigation-first である。assess（評価）、mitigate（緩和）、root cause（根本原因）、postmortem（事後分析）の順に進める。[Incident Response](https://sre.google/workbook/incident-response/)
- 汎用 mitigation として rollback（ロールバック）と drain（トラフィック退避）が挙げられている。個別の原因が分かる前でも使える手札として用意する。[Incident Response](https://sre.google/workbook/incident-response/)
- 事前の準備チェックリストには、communication channel（連絡手段）、contact list（連絡先一覧）、incident criteria（インシデント判定基準）が含まれる。[Incident Response](https://sre.google/workbook/incident-response/)
- 訓練は DiRT（Disaster Recovery Testing）と Wheel of Misfortune（模擬障害ロールプレイ）で行う。[Incident Response](https://sre.google/workbook/incident-response/)

### postmortem は blameless で全件レビューする（SRE Book 第15章）

- postmortem は障害から学ぶための事後分析文書であり、目的は再発防止と組織学習である。関係者の善意（good intentions）を前提とする blameless 原則で運用する。[Postmortem Culture](https://sre.google/sre-book/postmortem-culture/)
- 客観的な作成トリガーとして、検証済み範囲では次が挙げられている。ユーザー可視の劣化（user-visible degradation）、データ損失（data loss）、on-call の介入（on-call intervention）、解決時間の閾値超過（resolution-time threshold）、監視の失敗（monitoring failure）である。[Postmortem Culture](https://sre.google/sre-book/postmortem-culture/)
- レビューは共同作業（collaborative review）で行い、未レビューの postmortem を残さない（no postmortem left unreviewed）。[Postmortem Culture](https://sre.google/sre-book/postmortem-culture/)
- 文化を定着させる施策として、postmortem of the month、reading club、Wheel of Misfortune、可視的な報奨（visible rewards）が挙げられている。[Postmortem Culture](https://sre.google/sre-book/postmortem-culture/)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- 対応チームは Incident Commander を頂点に Ops・Communication・Planning（Workbook の表記では IC・CL・OL）へ責任を分け、負荷が上がったら再帰的に委譲する。Commander が作業も連絡も抱え込まないことを開始条件にする。
- 対応開始直後に live incident state 文書を開き、assess → mitigate の順で動く。根本原因の特定より先に rollback・drain の可否を判断し、緩和後に root cause と postmortem へ進む。
- incident criteria を事前に文書化し、2チーム目の支援が必要・顧客可視・1時間未解決のいずれかに該当したら宣言する。宣言の遅れを責めないことをルールに添える。
- handoff は相手の acknowledgment をもって完了とする。口頭やチャットの「投げただけ」を引き継ぎとみなさない。
- postmortem は上記の客観トリガー（ユーザー可視劣化、データ損失、on-call 介入、解決時間超過、監視失敗）で機械的に作成し、共同レビューで全件に目を通す。善意を前提とした blameless の文面にし、個人名の責任追及を書かない。
- 訓練と文化施策を定期運用にする。DiRT と Wheel of Misfortune をローテーションで回し、優れた postmortem の表彰や reading club で学習を可視化する。

## 避ける使い方

- Incident Commander に Ops 作業と対外連絡を兼務させる。coordinate・communicate・control のいずれかが止まり、state 文書の更新も遅れる。
- 原因究明が終わるまで mitigation を待つ。assess-mitigate-root-cause-postmortem の順序を崩し、顧客影響の時間を延ばす。
- acknowledgment のない handoff で担当を切り替える。責任の所在が曖昧になり、対応が止まる。
- インシデント宣言を遅らせる。2チーム目の支援や顧客可視の目安を超えても小規模扱いを続け、緩和の初動が遅れる。
- postmortem を個人の責任追及に使う。blameless 原則が崩れ、報告が減って組織学習が止まる。
- postmortem を書いただけでレビューせず放置する。未レビューが残り、再発防止策が実行されない。
- 連絡手段・連絡先・判定基準を用意せずに対応に入る。初動の大半が連絡先探しに消費される。

## 適用版と本番での注意

- 適用版: SRE Book（2017）第14章 Managing Incidents、第15章 Postmortem Culture: Learning from Failure、SRE Workbook（2018）第9章 Incident Response。将来の改訂とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で技術固有 TTL の対象外のため、2026-12-27 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: ICS 各役割の詳細な職務記述、postmortem 文書の雛形項目、早期宣言基準の数値の背景条件、DiRT の具体的な実施手順、3Cs の原文の完全な定義文。これらは各章の該当節を別途確認する。
- 本ドキュメントの推奨構成は設計案であり、単一事例の一般化ではない。宣言基準やレビュー運用は対象サービスの顧客影響・体制・on-call 負荷に照らして採用する。

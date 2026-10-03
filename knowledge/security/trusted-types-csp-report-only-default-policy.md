---
{
  "id": "security-trusted-types-csp-report-only-default-policy",
  "title": "Trusted Types の移行境界: CSP enforcement・Report-Only・default policy と React 19.3",
  "kind": "knowledge",
  "technology": "security",
  "version": "Trusted Types W3C Working Draft 2026-06-23; CSP Level 3 W3C Working Draft 2026-09-16; React 19.3 release 2026-09-09",
  "tags": ["research-domain:security", "trusted-types", "csp", "dom-xss", "report-only", "default-policy", "sanitization", "react-19.3"],
  "sources": [
    {"id": "w3c-trusted-types-wd20260623-boundary-20261003", "url": "https://www.w3.org/TR/2026/WD-trusted-types-20260623/", "type": "official_docs"},
    {"id": "w3c-csp3-wd20260916-delivery-20261003", "url": "https://www.w3.org/TR/2026/WD-CSP3-20260916/", "type": "official_docs"},
    {"id": "react-193-trusted-types-release-20261003", "url": "https://react.dev/blog/2026/09/09/react-19-3", "type": "release_notes"}
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# Trusted Types: Report-Only と default policy の移行境界

## 問いと適用版

既存アプリに Trusted Types を導入するとき、監視用 CSP を配れば動作は変わらないのか。React の対応版へ更新するだけで入力が安全になるのか。本稿は、ブラウザの sink 制約とアプリ側変換処理を分けてレビューするための記録である。

2026-10-03 UTC に、現行 TR URL と固定版の見出しを照合した。

- [Trusted Types][tt]: 2026-06-23 Working Draft
- [CSP Level 3][csp]: 2026-09-16 Working Draft
- [React 19.3][react]: 2026-09-09 公式発表

新しい実装上の接点は React 19.3 の型保持である。W3C の規則が上記公開日に初めて導入されたとは主張せず、作業草案を Recommendation として扱わない。事前検索では Trusted Types の直接の既存文書はなく、セキュリティ領域の未収録境界を補った。

## Trusted Types 仕様の要点

[固定版 §2.4.1、§3.4〜§3.5][tt] では、require-trusted-types-for が sink の型制約、trusted-types が policy 作成名の制約を担う。

対象 sink の生文字列に適用される default policy は、Report-Only でも呼ばれる。変換が成功すれば変換後の値を使い、callback の例外は伝播する。default がない等で変換できない場合は違反となり、enforce では拒否、Report-Only のみなら元入力で続行する。別の enforce policy が存在する場合は、後者の条件から除く。

したがって「Report-Only なら無変化」とは言えない。§5.4 は policy 自体の安全性も重要とする。以下の導入案はこの区別を利用した独自提案であり、仕様の必須手順ではない。

## 配送と framework の別の契約

[CSP3 §3.1〜§3.3、§8.1][csp] では Report-Only の meta 配送はサポートされず、複数の policy はそれぞれ評価される。緩い設定を追加しても既存の enforce を緩和できない。CDN とアプリが別々にヘッダーを追加する構成では、応答全体を確認する必要がある。

[React Team の発表][react] が述べる変更は、従来 DOM API 前に行っていた文字列化をやめ、Trusted Types の値を保持すること。型の保持は、sanitizer が入力を安全化した証明ではない。アプリの wrapper が再び文字列化する経路や、サーバーが出力する HTML まで安全になるとも推測しない。

## 独自の導入・運用案

### 変更単位を小さくする

CSP ヘッダー、React の更新、policy 実装、sanitizer 設定を一括で変更しない。どの変更で障害や安全性の差が出たかを追える単位で記録する。採用前に、画面ごとの DOM 書き込み箇所、入力の出所、既存 default の有無を棚卸しする。

policy 所有者を決め、レビュー対象の実装と配布する build を対応付ける。許可名だけを台帳化して終わらせず、関数と設定を変更する手続きを管理する。一時的な互換経路には対象画面、期限、除去条件を付ける。未検証入力をそのまま包む処理を、sanitizer と呼んで承認しない。

### 監視が届いているかを先に確かめる

監視の導入時は、テスト環境の既知の違反で収集経路を確認する。画面・browser・build を識別し、CSP 違反件数に加えて UIエラー、代表シナリオの到達率、default 呼び出し箇所を別々に観測する。

レポートが届かない理由を、単に「問題なし」で済ませない。対象画面を訪れていない、収集が故障している、試験の前提と異なる設定で配信されている、という候補を切り分ける。通常ログに入力全文を追加する前に、必要性、収集先、保持期間、閲覧権限を審査する。

### 復旧方法も受け入れ条件に入れる

障害時にどの設定を戻すか、誰が判断するか、旧 build へ戻した場合に新 policy が残らないかを確認する。最終的な応答ヘッダーは通常画面だけでなく、ログイン後画面とエラー画面でも記録する。

例外を握り潰して生文字列を再代入する修正は、元の境界を回復したことにはならない。必要ならテキスト表示や対象機能の停止など、画面に合う退避を用意する。Report-Only の期間でも既存の入力処理を外さず、enforce の展開範囲を広げる条件を事前に決める。

## 未実行の境界試験案

次は独自のレビュー用試験案であり、成功済みテストではない。

1. 同じ入力で Report-Only と enforce を比較し、default の有無・変換・例外を切り替える。最終 DOM、例外の到達先、レポートを別々に記録する
2. CSP の二つの directive を個別に設定し、policy 作成と sink 利用を別の操作として観測する
3. CDN とアプリのヘッダーを併用し、実際に届いた設定と期待した設定を照合する
4. React 19.3 の直接経路と wrapper 経路で型の保持を比較する。初回表示、hydration、client update を別シナリオにする
5. 主要画面が正常でも、未訪問画面や旧 build がないか確認する。browser の版を明記して証跡を残す

## 限界と未確認事項

ブラウザ実機、WPT、React の実行テストは未実行。対応開始版、最新 patch、polyfill の同等性、特定 sanitizer の安全性は確認していない。配布先での対応状況を別途確認する必要がある。

本稿は一般的な sink 変換、policy 作成、CSP 配送に限定し、HTML/SVG 統合の全手順やサーバー側出力の安全性を評価したものではない。検索 eval は文書の発見性だけを検証し、XSS 耐性を保証しない。

## 出典と利用条件

一次資料を基にした独自の日本語整理。コードや長文は転載していない。W3C 両資料は [Software and Document License 2023](https://www.w3.org/copyright/software-license-2023/) をページの表示から確認した。

React 記事は The React Team、Copyright © Meta Platforms, Inc。react.dev の README と LICENSE-DOCS.md で CC-BY-4.0 を確認した。GitHub のライセンス表示は取得時に503だったため、公式 raw URL で照合した。URL と取得日は catalog に記録し、参照元のライセンスをリポジトリ全体の配布条件にはしていない。

[tt]: https://www.w3.org/TR/2026/WD-trusted-types-20260623/
[csp]: https://www.w3.org/TR/2026/WD-CSP3-20260916/
[react]: https://react.dev/blog/2026/09/09/react-19-3#trusted-types-support

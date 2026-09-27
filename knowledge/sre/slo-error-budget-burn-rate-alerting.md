---
{
  "id": "sre-slo-error-budget-burn-rate-alerting",
  "title": "SRE の SLI/SLO とエラーバジェット定義、バーンレートによるアラート設計",
  "kind": "knowledge",
  "technology": "sre",
  "version": "SRE Book (2017) Ch.3 Embracing Risk / SRE Workbook (2018) Ch.5 Alerting on SLOs (verified 2026-09-27)",
  "tags": [
    "research-domain:quality-operations",
    "sre",
    "sli",
    "slo",
    "error-budget",
    "error budget",
    "burn-rate",
    "burn rate",
    "alerting",
    "multiwindow",
    "paging",
    "prometheus",
    "availability"
  ],
  "sources": [
    {
      "id": "sre-book-embracing-risk",
      "url": "https://sre.google/sre-book/embracing-risk/",
      "type": "official_docs"
    },
    {
      "id": "sre-workbook-alerting-on-slos",
      "url": "https://sre.google/workbook/alerting-on-slos/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-27",
  "expires_at": "2026-12-26",
  "trust": "official",
  "status": "active"
}
---

# SRE の SLI/SLO とエラーバジェット定義、バーンレートによるアラート設計

SLI/SLO とエラーバジェットで「どこまで壊れてよいか」を数値化し、バーンレート（予算の消費速度）でアラートの緊急度を分ける。以下は Google の SRE Book 第3章と SRE Workbook 第5章に記載された事実と、それを組み立てる設計案を分けて書く。2件はいずれも 2026-09-27 時点の検証済み目録に基づく。

## 要点（公式情報に記載された事実）

### 信頼性はリスクであり 100% は目標にしない（SRE Book 第3章）

- 信頼性は連続的なリスクであり、信頼性を上げるほどコストが増す。100% の信頼性は間違った目標である。[Embracing Risk](https://sre.google/sre-book/embracing-risk/)
- 可用性の測り方は一般に2種類ある。稼働時間の割合で測る時間基準（time-based availability）と、成功リクエストの割合で測るリクエスト成功率基準（request-success availability）であり、どちらを選ぶかはサービスの性質による。[Embracing Risk](https://sre.google/sre-book/embracing-risk/)
- エラーバジェットは、SLO から測定期間内の実測稼働率（measured uptime）を差し引いた残量である。四半期（quarter）単位で管理する例が示されている。[Embracing Risk](https://sre.google/sre-book/embracing-risk/)
- エラーバジェットはリリース速度の調整弁になる。予算が残っている間はリリースを進め、使い切ったらリリースを止めて信頼性対策に集中する。予算の所有は SRE とプロダクトの共同責任である。[Embracing Risk](https://sre.google/sre-book/embracing-risk/)

### バーンレートは SLO に対する予算消費の相対速度（Workbook 第5章）

- バーンレート（burn rate）は、SLO に対してエラーバジェットを消費する速度である。バーンレート 1 は、遵守期間の末尾で予算をちょうど使い切る速度を意味する。[Alerting on SLOs](https://sre.google/workbook/alerting-on-slos/)
- 同章は、単純な閾値（threshold）ベースからマルチウィンドウ・マルチバーンレート（multiwindow, multi-burn-rate）まで、6段階のアラート手法を比較している。[Alerting on SLOs](https://sre.google/workbook/alerting-on-slos/)
- 推奨構成（Table 5-8）は paging 用2組と ticket 用1組である。paging はバーンレート 14.4・long window 1時間・short window 5分・予算消費 2%、およびバーンレート 6・long window 6時間・short window 30分・予算消費 5%。ticket はバーンレート 1・long window 3日・short window 6時間・予算消費 10% である。[Alerting on SLOs](https://sre.google/workbook/alerting-on-slos/)
- short window は long window の 1/12 であり、long window と short window の両方が条件を満たしたときにアラートを上げる。短い window を併用することで、アラート発火後の自動リセットなしに検出の確度を保つ仕組みである。[Alerting on SLOs](https://sre.google/workbook/alerting-on-slos/)
- 同章には Prometheus 向けの設定例が掲載されている（本ドキュメントは PromQL を転載しない。原文の該当節を参照する）。[Alerting on SLOs](https://sre.google/workbook/alerting-on-slos/)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- まず SLI（何を測るか）と SLO（目標値）を決め、エラーバジェット（SLO と実測の差分）を四半期などの固定期間で管理する。予算の残量で「リリース継続か凍結か」を判断する運用ルールを、SRE とプロダクトの合意として文書化する。
- アラートは3組で緊急度を分ける。高速消費（14.4x、1時間/5分、2%）と低速消費（6x、6時間/30分、5%）を paging（即応）に、等速消費（1x、3日/6時間、10%）を ticket（通常対応）に割り当てる。
- 各アラートは long window と short window（long の 1/12）の両方が条件を満たしたときだけ発火させる。long だけで発火させると回復途中の一時的な改善で誤って解除されるため、short による裏付けを必須にする。
- 可用性の測り方はサービスに合わせる。常時接続型は時間基準、リクエスト応答型は成功率基準を起点に選び、選定理由を SLO 定義に添えて記録する。

## 避ける使い方

- 100% の信頼性を SLO にする。コストが際限なく増え、リリース判断の基準（エラーバジェット）が機能しなくなる。
- エラーバジェットと連動しない生の閾値だけで paging する。予算の消費速度と結びつかないため、軽微な異常で叩き起こされるか、本当に危険な消費を見逃す。
- long window だけでバーンレートアラートを組む。回復途中のブレでアラートが点滅し、対応者が通知を無視し始める。
- 低速の予算消費（ticket 相当）を paging に格上げする。Table 5-8 の使い分けを崩し、即応疲れを招く。
- エラーバジェット切れ後もリリースを継続する。予算が release gate として働かず、SLO 違反が拡大する。

## 適用版と本番での注意

- 適用版: SRE Book（2017）第3章 Embracing Risk、SRE Workbook（2018）第5章 Alerting on SLOs。将来の改訂とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で技術固有 TTL の対象外のため、2026-12-26 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: 6手法それぞれの正式名称と完全なパラメータ表、Table 5-8 の数値が想定する SLO 遵守期間の長さ、転載していない Prometheus の PromQL 例の原文、時間基準と成功率基準の選択フローの詳細。これらは各章の該当節を別途確認する。
- 本ドキュメントの推奨構成は設計案であり、単一事例の一般化ではない。Table 5-8 の数値はそのまま使うのではなく、対象サービスの SLO 値・遵守期間・トラフィック量に照らして採用する。

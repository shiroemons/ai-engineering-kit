---
{
  "id": "messaging-rabbitmq-mqtt-retained-limit-ack-boundary",
  "title": "RabbitMQ 4.3.6 MQTT retained store: 上限超過・PUBACK・既存値保持の境界",
  "kind": "knowledge",
  "technology": "messaging",
  "version": "RabbitMQ v4.3.6 @ 7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095; GitHub release 2026-09-14, official release table 2026-09-16; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:api-distributed",
    "rabbitmq",
    "mqtt",
    "retained-messages",
    "puback",
    "capacity-limit",
    "node-local",
    "ets",
    "dets",
    "upgrade"
  ],
  "sources": [
    {
      "id": "rabbitmq-mqtt-retained-release-436-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.6",
      "type": "release_notes"
    },
    {
      "id": "rabbitmq-mqtt-retained-dates-436-20261003",
      "url": "https://www.rabbitmq.com/release-information",
      "type": "release_notes"
    },
    {
      "id": "rabbitmq-mqtt-retained-guide-43-20261003",
      "url": "https://www.rabbitmq.com/docs/mqtt",
      "type": "official_docs"
    },
    {
      "id": "rabbitmq-mqtt-retained-retainer-7a34a0ca-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_retainer.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-mqtt-retained-processor-7a34a0ca-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_processor.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-mqtt-retained-dets-7a34a0ca-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_retained_msg_store_dets.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-mqtt-retained-ets-7a34a0ca-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_retained_msg_store_ets.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-mqtt-retained-size-7a34a0ca-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/mc_mqtt.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-mqtt-retained-interface-7a34a0ca-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_retained_msg_store.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-mqtt-retained-tests-7a34a0ca-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/test/retainer_SUITE.erl",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-mqtt-retained-schema-7a34a0ca-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/priv/schema/rabbitmq_mqtt.schema",
      "type": "github_repository_analysis"
    },
    {
      "id": "rabbitmq-mqtt-retained-advisory-20261003",
      "url": "https://github.com/rabbitmq/rabbitmq-server/security/advisories/GHSA-5jgm-jxp7-gwjh",
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

# RabbitMQ MQTT retained storeの上限とpublish確認

## 問いと結論

機器の最新状態をMQTT retained messageで配り、QoS 1 publishが完了すれば後から接続した機器にも最新値が届く、と判断してよいか。RabbitMQ 4.3.6で確認したretained store上限では、その判断が成立しない。上限超過はretainer側で保存を見送り、既存値があればその値を残す。publishの確認とretained状態への反映を分けて検査する必要がある。

[4.3.6 release](https://github.com/rabbitmq/rabbitmq-server/releases/tag/v4.3.6)に追加が明記されている。GitHub APIの公開時刻は2026-09-14T07:24:14Zだが、[公式release一覧](https://www.rabbitmq.com/release-information)は2026-09-16を掲載する。日付の差を同一の公開時刻として解消せず、実装はtagのcommit `7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095` に固定して確認した。

[GHSA-5jgm-jxp7-gwjh](https://github.com/rabbitmq/rabbitmq-server/security/advisories/GHSA-5jgm-jxp7-gwjh)（2026-09-18公開）は、retained storeの無制限消費に関するpatched versionを4.3.5と記載する。4.3.6 release notesが設定追加を列挙することだけから、4.3.6が最初の導入版・最初のsecurity修正版だとは断定しない。本稿は4.3.6の挙動を確認したもので、4.3.5との実装差分や全backport範囲は未確認である。

既存の[AMQP publisher confirms](publisher-confirms-mandatory-retry.md)や[quorum queue priority](rabbitmq-strict-priority-prefetch-boundary.md)とは、対象protocol・保存先・失敗経路が異なる。執筆前のindexで `retained MQTT` と `max_messages max_size_bytes` は0件だった。

## 新しい設定が制限するもの

releaseと固定版の[設定schema](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/priv/schema/rabbitmq_mqtt.schema)を照合した。

| 設定 | 配布時の既定値 | 目的 |
|---|---|---|
| `mqtt.retained_message_store.max_messages` | 100000 | retained entryの件数 |
| `mqtt.retained_message_store.max_size_bytes` | 1073741824（1 GiB） | storeの大きさを使った受入れ判定 |

両方とも非負整数または `infinity` を受け付ける。`0` は無制限を表さない。特にbyte上限0では、後述のupstream testは非空messageが保存されないことを確かめる。片方をinfinityにしても、他方の上限は残る。

[MQTT guide](https://www.rabbitmq.com/docs/mqtt#retained-messages-and-stores)によれば、組込みETS/DETS storeはnode-localであり、他nodeへの複製も他nodeのretained値の照会も行わない。この構成で「vhostごとの上限」とは各nodeのvhost storeに対する判定であり、cluster全体を集計する一つのquotaではない（実装とguideからの帰結）。通常のsubscription queue件数やqueue長を制限する設定でもない。

同guideは、保存したnodeと同じnodeへのexact-topic subscriptionでretained値を返す一方、別nodeやwildcard topic filterでは返さないと記載する。上限を上げてもこの配送条件は変わらない。

## PUBACKと保存完了を分ける（固定版の実装観察）

[processor](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_processor.erl)の `publish_to_queues_with_checks` は権限確認と通常のqueueへのpublishを行い、その成功経路でretainerへ渡す。[retainer](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_retainer.erl)の `retain/3` は `gen_server:cast` で、callerへ保存の成否を同期的に返す仕組みではない。

retainerが上限判定を拒否した場合、保存処理へ進まずwarning用状態を更新する。その拒否をpublisherへ返す応答はこの経路にない。したがって、通常publishが受け付けられたことやPUBACKだけから、retained値が更新済みだと判断できない。反対に、すべてのpublishが必ず成功するという意味でもない。権限・routing・queue拒否・接続障害は別の失敗経路を持つ。

この区別は[upstream regression tests](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/test/retainer_SUITE.erl)にも現れる。`limit_max_messages` は件数上限2で第三topicのQoS 1 publishがclient API上の `{ok, _}` を返した後、第三topicへsubscribeしてretained messageが来ないことを検査する。client戻り値を保存保証へ読み替えない。

## 新規topicと上書きで結果が異なる

固定版 `retain_permitted` の判定は次のように整理できる。これは内部実装の読解であり、将来版に固定されたpublic APIの保証ではない。

1. 両上限がinfinityなら、上限判定を通す
2. そうでなければstoreの件数とsize、同じtopicの既存messageを読む
3. byte側は「storeのsize − 既存entryの見積り + 新entryの見積り」が上限以下かを検査する。既存entryがなければ差し引く値は0
4. 件数側は既存topicなら追加件数として数えない。新規topicなら現在件数+1が上限以下である必要がある
5. 両条件を満たしたときだけ保存する

従って件数が満杯でも、byte条件を満たす既存topicの上書きは可能。一方「上書きなら必ず通る」は誤りで、byte側は引き続き検査される。

拒否時は `do_retain` に入らないため、既存値と既存のexpiry timerを置換・更新しない。新しい値で更新したつもりでも、後続subscriberが古い値を読む可能性がある。旧timerがある場合は旧期限で消える。拒否したmessageを保存待ちに積んで容量回復後に自動再保存する経路や、古いentryをevictして新しい値を入れる経路ではない。

processorはretain付きの空payloadを `clear` に振り分ける。retainerのclear処理は上限受入れ判定を経ずにそのtopicを削除し、timerも取り消す。これは個別topicの削除経路であり、上限を0へ変更する操作が既存storeを一括消去することを意味しない。

## max_size_bytesはpayload総量の厳密な上限ではない

固定版の組込みstoreはsizeを別の単位から算出する。

- [ETS store](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_retained_msg_store_ets.erl): tableのmemory値にErlangのwordsizeを掛ける
- [DETS store](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_retained_msg_store_dets.erl): `dets:info` の `file_size` を使う
- entryの増減見積りは[mc_mqtt:size](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/mc_mqtt.erl)を使う。topic、payload、Content-Type・Response-Topic・Correlation-Data、User-Propertyのname/valueが計上される。すべてのwire overheadやstore metadataを同じ式で正確に足しているわけではない

実storeのsizeとentry見積りが異なるため、これを「payload合計がちょうど1 GiBまで入る」「書込み後のRSSやfile sizeが絶対に設定値を超えない」というhard limitとして扱わない。DETSのentry削除後も、file_sizeが期待通り小さくなるかは別に確認する。今回、削除・再利用・断片化後の実file sizeは測定していない。

組込みDETSにはguideが示す2 GBの制限もある。新しい1 GiB既定値とDETS自身の上限は別であり、infinityへの変更がDETSの構造上の上限を撤廃するわけではない。

## upgradeで確認する二つの境界

### 設定はretainer起動時に読む

`init` はstoreをrecoverしてから上限をstateへ取り込む。retainごとに設定を再読込する実装ではなく、上限を超えた既存entryを起動時にquota目的で削る処理もない。既存expiryの復元・期限切れ処理とquota整理を区別する。

upstreamの `set_limits` helperもretainerを再起動して試験する。ただし同helperは試験用storeファイルを削除しており、本番向けの設定反映手順として流用してはいけない。実運用のrolling restart、保存状態の維持、各nodeの実効値の照合は別途受入れ試験が必要である。

### custom storeにはinfo callbackが加わる

固定版の[store behaviour](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/deps/rabbitmq_mqtt/src/rabbit_mqtt_retained_msg_store.erl)は `info/1` を要求し、戻り値に `count` と `size_bytes` を定義する。上限判定はその値を読む。旧custom storeがこのcallbackを実装していない場合、有限上限の判定経路を無変更で通せると考えない。

これは固定版から読み取れる互換性確認点であり、特定の第三者pluginが故障すると実測した報告ではない。両上限をinfinityにした経路がinfo呼出しを省略することを、custom store移行の一般的な回避策として推奨しない。容量保護の意図とcallbackの計測単位を合わせて更新・検証する。

## 観測と導入試験（独自の設計案）

warningはretainerごとに初回と、その後は前回出力から少なくとも60000ms経過した拒否時に出る。拒否message一件につき一行を必ず出す実装ではない。ログ行数をdrop数のカウンタとして扱わない。

最新状態の正本として使うなら、publisherの成功率に加え、fresh subscriberが期待する業務versionを読み取れるかを確認する。単なるsubscribe成功・値の存在だけでは、古い値を取り残したcaseを見落とす。保持に失敗しても業務状態を再構築できる正本を別に持つ、というのが本稿の設計判断である。

次の試験を推奨する。実行済みの結果ではない。

1. 件数上限2・byte上限infinityで新規topicを3件送り、第三topicのpublish確認と後続subscriptionを別に記録する。既存topicの更新も対照caseにする
2. byte上限を小さくし、短いpayloadに大きなCorrelation-Dataを付ける。payloadだけでは容量判定を説明できないことを確かめる
3. 旧値とexpiryを入れたtopicへbyte条件で拒否される新値を送り、旧値・旧expiry・warningを照合する。空payloadのclearも別caseにする
4. 同じvhostでも保存nodeと別nodeでsubscribeし、exact topicとwildcardを分ける。LB経由の再接続を「保存失敗」と誤診しない
5. 実dataを残したupgradeで、起動前後の件数・store size・実効上限を保存する。ETSとDETS、custom storeは別々に確認する
6. warningが繰り返し抑制されている間にも、新しい業務versionの欠落を検知できるか確かめる。無制限retryや無条件のinfinity設定を容量問題の解決としない

## 検証済み範囲と未確認事項

- 2026-10-03 UTCにrelease、MQTT guide、release一覧、security advisoryをWebで開いた。固定commitのblobページ取得は失敗したが、GitHub connectorで実装・schema・test・LICENSEを読んだ。取得失敗を成功として扱っていない
- v4.3.6の `retainer_SUITE` を読解した。DETS/ETSとMQTT protocol level 4/5のgroupで件数・sizeのcaseを定義し、大きなCorrelation-Dataのcaseはv5だけに限定する。protocol level 4はMQTT 3.1.1に対応する。これらのupstream testsをこの環境で実行したわけではない
- RabbitMQ broker、MQTT client、Erlangを起動する動作試験は未実行。上書き拒否後の旧値・expiryは制御フローからの帰結で、独立したfault injection結果ではない。性能、正確なbyte境界、DETS compaction、cluster障害、custom pluginの対応版は未検証
- live guideの表示は4.3だが、新しいmax_messages/max_size_bytesの説明は確認できなかった。具体的な新設定と拒否処理はreleaseと固定commitで根拠を補った。MQTT標準に対する適合性評価や他brokerへの一般化は行っていない
- 実装・testは各ファイルのMPL-2.0 headerと[同commitのLICENSE](https://github.com/rabbitmq/rabbitmq-server/blob/7a34a0caf4fa5cf4009cb934ea7a94dbe7e08095/LICENSE)を確認した。Web guide・release本文・security advisoryへの適用ライセンスは直接確定できずunknownとした。コード転載・module昇格は行っていない
- 検索evalは文書の発見性だけを検査する。release_notesの30日TTLに合わせ、再確認期限を2026-11-02とする

---
{
  "id": "ruby-socket-connect-success-timeout-boundary",
  "title": "Ruby Socket.tcp: 接続拒否の成功誤判定・SO_ERROR・時間予算の境界",
  "kind": "knowledge",
  "technology": "ruby",
  "version": "CRuby 3.4.11 (592f1ffdb36153e8be83603ade3c2e9ab6138a77) / 4.0.7 (229531a6cfbf07e3caef30dbac24a2a3f3fed482); timeout契約はRuby 3.4 RDoc; verified 2026-10-03 UTC",
  "tags": [
    "research-domain:backend",
    "ruby",
    "Socket.tcp",
    "Addrinfo",
    "connect_nonblock",
    "connect_timeout",
    "resolv_timeout",
    "SO_ERROR",
    "EISCONN",
    "ECONNREFUSED",
    "Happy Eyeballs",
    "fast_fallback",
    "healthcheck"
  ],
  "sources": [
    {
      "id": "ruby-socket-release-3-4-11-20261003",
      "url": "https://github.com/ruby/ruby/releases/tag/v3_4_11",
      "type": "release_notes"
    },
    {
      "id": "ruby-socket-release-4-0-7-20261003",
      "url": "https://github.com/ruby/ruby/releases/tag/v4.0.7",
      "type": "release_notes"
    },
    {
      "id": "ruby-socket-refused-report-22223-20261003",
      "url": "https://bugs.ruby-lang.org/issues/22223",
      "type": "incident_report"
    },
    {
      "id": "ruby-socket-api-3-4-20261003",
      "url": "https://docs.ruby-lang.org/en/3.4/Socket.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-addrinfo-api-3-4-20261003",
      "url": "https://docs.ruby-lang.org/en/3.4/Addrinfo.html",
      "type": "official_docs"
    },
    {
      "id": "ruby-socket-implementation-592f1ffd-20261003",
      "url": "https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/ext/socket/lib/socket.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "ruby-socket-implementation-229531a6-20261003",
      "url": "https://github.com/ruby/ruby/blob/229531a6cfbf07e3caef30dbac24a2a3f3fed482/ext/socket/lib/socket.rb",
      "type": "github_repository_analysis"
    },
    {
      "id": "ruby-socket-tests-592f1ffd-20261003",
      "url": "https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/test/socket/test_socket.rb",
      "type": "github_repository_analysis"
    }
  ],
  "retrieved_at": "2026-10-03",
  "expires_at": "2026-11-02",
  "trust": "primary-source",
  "status": "active",
  "evals": [
    "evals/knowledge/backend.json"
  ]
}
---

# Ruby Socket.tcp の成功判定と時間予算

## 問い・対象・結論

短い `connect_timeout` を設定したTCP probeがsocketを返せば、接続先は本当に起動しているか。2026年9月公開のCRuby 3.4.11 / 4.0.7には、接続拒否を成功と誤判定する経路の修正がある。実務では、修正済みruntimeの確認、TCP接続成立の判定、接続後のアプリケーション応答、全体の時間予算を分ける必要がある。

[3.4.11 release](https://github.com/ruby/ruby/releases/tag/v3_4_11) は2026-09-23、[4.0.7 release](https://github.com/ruby/ruby/releases/tag/v4.0.7) は2026-09-15に公開され、両方がBug #22223を掲載する。以下は公開APIの契約、固定版sourceの観察、上流での再現報告、独自の運用案を区別する。既存のFiber scheduler hookやRactor通信の設計は扱わない。

## upstreamで起きたこと: writableは成功通知ではない

[Bug #22223](https://bugs.ruby-lang.org/issues/22223) は、macOS 27.0 beta、build `26A5388g`、arm64上のRuby 3.4.5 / 3.4.10で、待受けのないportへの `Socket.tcp(..., connect_timeout: ...)` がsocketを返し、最初のwriteで `Errno::EPIPE` になる現象を報告している。maintainerも同じOS buildで再現を確認した。これは報告環境の事実であり、macOS 27の全buildやLinuxで同じ障害が出るという主張ではない。

報告された順序は、最初のnonblocking connectが進行中となり、待機後にfdがwritableと判定され、connectを再試行すると `EISCONN` になる一方、socketの `SO_ERROR` には `ECONNREFUSED` が残る、というものだった。旧経路は再試行結果を成功と解釈したため、利用不能なsocketが返った。報告者は開発serverの起動probeが偽陽性になり、後続proxyが失敗する例も挙げている。

この調査から採る設計上の判断は、readinessの意味を明示することである。

- fdのwritable通知: 待機を終えて接続結果を確認する契機
- TCP接続成立: transport層の到達確認
- 想定protocolの応答: 対象アプリが要求を処理できるかの確認

三つを一つの成功counterへまとめない。最後の確認内容は各サービスの契約で決めるもので、Rubyの今回の修正がTLS認証やHTTP readinessを保証するわけではない。

## 修正後の分岐と残る低水準loop（固定版の観察）

[3.4.11のsocket.rb](https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/ext/socket/lib/socket.rb) と [4.0.7の同method](https://github.com/ruby/ruby/blob/229531a6cfbf07e3caef30dbac24a2a3f3fed482/ext/socket/lib/socket.rb) で `Addrinfo#connect_internal` を確認した。内部methodの実装観察であり、将来版での制御フローの固定保証ではない。

1. timeout指定時は `connect_nonblock(..., exception: false)` を使う。戻り値0なら接続処理を抜け、`:wait_writable` なら `wait_writable(timeout)` へ進む
2. 待機が満了して戻り値が偽なら `Errno::ETIMEDOUT` をraiseする
3. writableになったら、再試行する前に `getsockopt(Socket::SOL_SOCKET, Socket::SO_ERROR).int` を読む。非0ならそのerrnoによる `SystemCallError` をraiseする。接続拒否を一律ETIMEDOUTへ変換する修正ではない
4. `SO_ERROR == 0` でも、その地点で直ちにsocketを返す変更ではない。loopは再び `connect_nonblock` を呼び、0になると抜ける
5. 接続過程で例外になれば作成したsocketをcloseして再raiseする。timeout未指定の経路はblockingの `sock.connect` を呼ぶ

対象は `Addrinfo#connect` の `timeout:`、およびこの内部methodに到達するSocket.tcpの経路である。[Addrinfo API](https://docs.ruby-lang.org/en/3.4/Addrinfo.html#method-i-connect) では `connect_from` / `connect_to` も同じ内部処理を呼ぶ。これは呼出し関係の確認であり、全組合せをOS上で再現したという意味ではない。

注意点として、取得した [3.4 Socket#connect_nonblockの例](https://docs.ruby-lang.org/en/3.4/Socket.html#method-i-connect_nonblock) には、待機後の再試行で `EISCONN` をrescueするlow-level loopが残っている。高水準のAddrinfo実装に加わったチェックは、アプリやgemがコピーした独自loopへ自動的に追加されない。`exception: false` も待機用例外をsymbolにするoptionで、接続失敗をすべて成功値に変換してよいという契約ではない。独自loopを保持する必要があるなら、対象OS・runtimeで失敗結果の取り出しまでレビューする。

## fast_fallbackを変えても修正の代わりにはならない

[Socket API](https://docs.ruby-lang.org/en/3.4/Socket.html#method-c-tcp) はRuby 3.4からHappy Eyeballs Version 2を既定にしたと説明する。`fast_fallback: false` は呼出し単位で無効化し、`Socket.tcp_fast_fallback = false` はglobal設定を変える。`RUBY_TCP_NO_FAST_FALLBACK=1` は既定値をfalseにする。

固定した3.4.11 sourceでは、`fast_fallback` が真でも、hostがIP literalなら `tcp_without_fast_fallback` を選ぶ。従って `127.0.0.1` へのprobeでflagをfalseへ変えても、既に選ばれていた経路は変わらない。この点は上流報告とも一致する。数値IPの再現をHappy Eyeballs固有の障害と決めつけない。

また、Happy Eyeballs側でも、残った候補について `Addrinfo#connect` / `connect_from` へ進むfallbackがある。高速fallbackの有効・無効だけでは、この内部処理を使うかを判定できない。一方、上流報告で `TCPSocket.new` とtimeoutなしSocket.tcpが正しく拒否したことは、その環境での比較結果である。これを理由に全環境のtimeoutを削除したり、無条件に別APIへ置換したりしない。

## 同じconnect_timeoutでも待つ範囲が違う（公式契約）

[3.4 Socket.tcpのoption説明](https://docs.ruby-lang.org/en/3.4/Socket.html#method-c-tcp) は次の区別をしている。

| 設定・段階 | 確認した意味 | 運用上の読み方 |
|---|---|---|
| `resolv_timeout` | hostname解決開始からのtimeout秒数 | 接続後のread/writeを制限する値ではない |
| Happy Eyeballsの `connect_timeout` | 最後の接続候補への試行開始を起点にするtimeout | Socket.tcp呼出し開始からの単一deadlineとは読めない |
| `fast_fallback: false` の `connect_timeout` | 各候補の接続試行に個別timeoutを設定する | 複数候補を順に試す総経過時間は同じ値に収まるとは限らない |
| `Addrinfo#connect(timeout: ...)` | 指定addressへの接続timeout | Addrinfoを作る前の名前解決や、返却後の処理全体とは別 |

Happy Eyeballsは名前解決と候補接続の開始を並行化・時間差実行するため、全候補を単純に直列で待つ方式と同じ総予算ではない。flag変更時は平均速度だけでなく、失敗時の最大待機、選んだIP family、最終例外を再確認する。これは契約からの独自の試験方針であり、全OSでの厳密な上限値を計測した結果ではない。

sourceでも `tcp_without_fast_fallback` は候補ごとに同じ `connect_timeout` を渡し、`SystemCallError` を記録して次へ進む。全候補失敗時は最後の記録した例外を返すため、その例外だけで全候補が同じ理由で失敗したと断定しない。総時間予算が必要なclientでは、接続候補・名前解決・TLS・request/response・retryを含む上位のdeadlineと、各libraryの中断・後始末の契約を別途設計する。今回のpatchはそうしたend-to-end deadlineを追加していない。

## 更新・受入試験の順序（独自の運用案）

1. 実行artifactの `RUBY_DESCRIPTION` 相当、OS build、socket libraryの実体、利用gemと呼出しAPIを記録する。RDocのURLが3.4であるだけでは、実行中の3.4.xが修正済みか証明できない。独自patchや別配布のlibraryがあるならその実体を確認する
2. 更新対象の3.4.11 / 4.0.7に修正が入っていることを固定sourceで確認し、本番・開発・CIのruntimeを揃える。全旧patch版の影響開始時点や3.3への適用要否を、この二つのreleaseの掲載だけから推測しない
3. まずIP literalと待受けなしportで、接続拒否がsocket返却ではなく例外になることを確認する。成功caseだけのprobe試験にしない。portを一度確保して閉じる方式も、その後の別processによる再利用には注意する
4. 通常listenerへの接続成功、接続拒否、接続待機のtimeoutを別caseにする。さらにhostname、IPv4/IPv6、`fast_fallback` の両値、local bind、Addrinfo直接呼出しなど、実際に使う経路を選ぶ
5. TCP接続後にprotocol応答を返さないlistenerを用意し、application側のreadiness timeoutとcloseを検証する。connect_timeoutだけでこのcaseも止まると期待しない。相手の負荷や副作用を増やす本番probeへ無断で変更しない
6. `connect_nonblock`、`IO::WaitWritable`、`EISCONN` の独自処理をアプリと依存gemから探す。標準library更新済みという理由だけで、独自loopのレビューを省略しない
7. probeの再試行は上限と待機間隔を持たせる。接続拒否とtimeoutを区別して記録し、TCP成功・protocol成功・返却後のEPIPEを別に観測する。失敗を救済するための無期限blocking fallbackは入れない
8. resourceの所有者を明示する。Socket.tcpのblock形はblockの戻り値を返し、正常終了・例外のいずれでもsocketをcloseする。blockなしで返されたsocketはcallerがcloseする。block形にconnect_timeoutを渡してもblock全体の実行制限にはならない

## upstream testと今回の検証範囲

[3.4.11のtest_socket.rb](https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/test/socket/test_socket.rb) の `test_connect_timeout_connection_refused` は、loopback上で一時的にTCPServerを開いてportを取り、serverを閉じてから、`connect_timeout: 5` のSocket.tcpが `Errno::ECONNREFUSED` をraiseすることをassertする。`/mswin|mingw/` のplatformではこのtestを定義しない。

このtestの焦点は接続拒否の結果である。5秒以内というwall-clockのassert、全候補の総時間、hostname解決、TLS、protocol readinessを検査するtestではない。隣接する `test_connect_timeout` は通常接続を確認し、listener queueを満たしてETIMEDOUTを見る部分はLinuxに限定している。異なる条件のassertを合算して、全platformでの一律保証として扱わない。

今回はsourceとtestを読解したが、Ruby runtimeがこの環境にないため実行していない。特にmacOS 27 betaの再現、修正前後の比較、patch版別の全回帰、gem独自loop、OS schedulerやDNS障害時の時間精度は未検証である。検索evalは本稿の発見性を検査し、ネットワーク挙動の実証を代替しない。

## 出典・日付・ライセンス

- 取得日はすべて2026-10-03 UTC。公開日は3.4.11が2026-09-23、4.0.7が2026-09-15。元の修正 [PR #18203](https://github.com/ruby/ruby/pull/18203) は2026-08-06にmergeされている。issueの相対日付から正確な公開日を逆算していない
- RDocは3.4系列表示で、patch版とページ固有の公開日は表示しない。実装は3.4.11の `592f1ffdb36153e8be83603ade3c2e9ab6138a77` と4.0.7の `229531a6cfbf07e3caef30dbac24a2a3f3fed482` に固定した。将来の最新版との同一性は主張しない
- 両commitのCOPYINGはRuby License / 2-clause BSDLを示す。LEGALのsocket例外はaddrinfo.h・getaddrinfo.c・getnameinfo.cで、分析したext/socket/lib/socket.rbとtest/socket/test_socket.rbの例外項目は見つからなかった。[3.4.11 COPYING](https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/COPYING)、[LEGAL](https://github.com/ruby/ruby/blob/592f1ffdb36153e8be83603ade3c2e9ab6138a77/LEGAL) を確認済み
- release本文、issue投稿、RDocページ本文の個別ライセンス範囲は確定できないため、該当catalogはunknownとした。Socket RDocにはProgramming Rubyから許可を受けた記述を含むという表示もある。sourceやサンプルの転載はせず、独自要約と設計案に限定し、moduleへ昇格しない
- release、issue、Socket / Addrinfo / COPYING / LEGALのRDoc、PRはnative Webで開いた。releaseからのcommit画面取得は失敗したため、tagの完全SHA、固定source・test・licenseはGitHub connectorで確認した。失敗したWeb取得を読解済みの証拠にしていない

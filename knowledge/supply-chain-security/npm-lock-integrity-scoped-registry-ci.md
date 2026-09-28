---
{
  "id": "npm-lock-integrity-scoped-registry-ci",
  "title": "npm install のサプライチェーン防御: package-lock の integrity/resolved による真正性検証、scoped registry と replace-registry-host、npm ci の frozen install",
  "kind": "knowledge",
  "technology": "supply-chain-security",
  "version": "npm CLI v12.1.0 (Latest) 公式文書 (.npmrc のみ npm CLI v11.20.0 Legacy) / pacote commit c82bdcdd8010a9a87c95e1e09b0ba51322b4f93f (package.json version 22.0.0)",
  "tags": ["research-domain:security", "supply-chain-security", "npm", "package-lock", "lockfile", "integrity", "resolved", "subresource-integrity", "scoped-registry", "replace-registry-host", "npm-ci", "npmrc", "registry", "pacote", "eintegrity", "frozen-install"],
  "sources": [
    {"id": "npm-package-lock-json-docs", "url": "https://docs.npmjs.com/cli/v12/configuring-npm/package-lock-json", "type": "official_docs"},
    {"id": "npm-config-docs", "url": "https://docs.npmjs.com/cli/v12/using-npm/config", "type": "official_docs"},
    {"id": "npm-scope-docs", "url": "https://docs.npmjs.com/cli/v12/using-npm/scope", "type": "official_docs"},
    {"id": "npm-npmrc-docs", "url": "https://docs.npmjs.com/cli/v11/configuring-npm/npmrc", "type": "official_docs"},
    {"id": "npm-ci-docs", "url": "https://docs.npmjs.com/cli/v12/commands/npm-ci", "type": "official_docs"},
    {"id": "npm-pacote-readme-c82bdcdd", "url": "https://raw.githubusercontent.com/npm/pacote/c82bdcdd8010a9a87c95e1e09b0ba51322b4f93f/README.md", "type": "official_docs"}
  ],
  "retrieved_at": "2026-09-28",
  "expires_at": "2026-12-27",
  "trust": "official",
  "status": "active",
  "evals": ["evals/knowledge/security.json"]
}
---

# npm install のサプライチェーン防御: package-lock の integrity/resolved による真正性検証、scoped registry と replace-registry-host、npm ci の frozen install

npm の install 時に、取得する tarball が package-lock.json の記録と一致するか (integrity)、どの registry / host から取得するか (resolved・scoped registry・replace-registry-host) を npm CLI 公式文書と npm CLI が使う package handler pacote の README で確認した。以下は文書化された事実 (要点) と、そこからの設計案 (推奨方法) を区別して記載する。対象は npm CLI v12.1.0 (Latest) の文書で、`.npmrc` のみ v11.20.0 (Legacy) 版を参照した。

## 要点 (文書化された事実)

### package-lock.json の integrity と resolved

([package-lock.json](https://docs.npmjs.com/cli/v12/configuring-npm/package-lock-json))

- package-lock.json は npm が `node_modules` ツリーまたは package.json を変更する操作で自動生成される。「teammates, deployments, and continuous integration are guaranteed to install exactly the same dependencies」になるようコミットする前提のファイル (「This file is intended to be committed into source repositories」)。
- `packages` エントリの `resolved` は「The place where the package was actually resolved from」。registry パッケージでは tarball の URL、git 依存では commit sha 付きの full git url、link 依存では link 先の位置。`registry.npmjs.org` は「the currently configured registry」を意味する magic value であり、固定ホストの指定ではない。
- `integrity` は「A sha512 or sha1 Standard Subresource Integrity string for the artifact that was unpacked in this location」。すなわち、その場所へ unpack された artifact の SRI (Subresource Integrity) 値。
- legacy な `dependencies` (lockfileVersion 1) 形式では、`integrity` は registry source では sha512/sha1 の SRI 文字列、git source では commit sha。`resolved` は registry source では registry URL に対する tarball の相対 path で、registry と別サーバなら完全 URL。
- lockfileVersion: 3 は npm v9 以降 (npm v7 に後方互換)、2 は npm v7/v8、1 は npm v5/v6。`lockfile-version` config の既定は「Version 3 if no lockfile, auto-converting v1 lockfiles to v3; otherwise, maintain current lockfile version」。
- npm v12 では `npm-shrinkwrap.json` は npm に読み書きされない。従来 shrinkwrap を commit していたプロジェクトは `package-lock.json` にリネームする (形式は同一)。依存パッケージの tarball 内にある `npm-shrinkwrap.json` は無視される。
- hidden lockfile `node_modules/.package-lock.json` (npm v7 以降、lockfileVersion 3) は、参照する全 package folder が存在し、余分な folder がなく、ファイルの mtime が全 package folder より新しい場合のみ有効。他ツールがツリーを変更していれば検出して無視される。手動でファイルを編集した場合は hidden lockfile を削除するのが文書上の注意点。

### npm ci の frozen install

([npm ci](https://docs.npmjs.com/cli/v12/commands/npm-ci))

- `npm ci` は既存の `package-lock.json` を必須とし、lock と package.json が一致しない場合は lock を更新せず error で終了する。既存の `node_modules` は自動削除され、「It will never write to package.json or package-lock.json: installs are essentially frozen」。
- lock を生成したときの flags (`--legacy-peer-deps` や `--install-links` など) を同じく `npm ci` に渡さなければエラーになりやすい。文書は `npm config set legacy-peer-deps=true --location=project` で project の `.npmrc` を commit する方法を例示する。
- npm 12 の既定で `allow-git=none`、`allow-remote=none`。「Tarballs that share a hostname with the configured registry (the typical case for the npm registry, GitHub Packages, and most private registries) are still installed normally」。registry が別ホストに tarball を置く場合は `replace-registry-host` の設定か override が必要。git 依存は「Git dependencies run git against a remote repo and may install configuration the project does not control」ため、必要なプロジェクトで明示的に opt-in する。
- `allow-remote` / `allow-git` / `allow-file` / `allow-directory` の値は `all` / `none` / `root`。`root` は project の package.json で定義された依存のみ許可する。

### resolved の host 置換と lockfile の書き方

([Config](https://docs.npmjs.com/cli/v12/using-npm/config))

- `replace-registry-host` 既定 `"npmjs"` (type: `"npmjs"` / `"never"` / `"always"` / String)。既定では「replace package dist URLs from the default registry (https://registry.npmjs.org) to the configured registry」。`"never"` は registry の値をそのまま使い、`"always"` は毎回 configured registry の host に置換する。bare hostname (`"registry.npmjs.org"`) や URL+path prefix (`"https://old-registry.example.com/npm/path"`) を指定すると、host (と prefix の場合 path) が一致する resolved URL のみ prefix が丸ごと configured registry に置換され、path segment は重複しない。
- `omit-lockfile-registry-resolved` 既定 false。true にすると registry 依存に `resolved` キーを書かない lockfile を生成し、「Subsequent installs will need to resolve tarball endpoints with the configured registry, likely resulting in a longer install time」。この時に `integrity` がどう扱われるかは文書に明記がない (未確認)。
- `strict-npmrc` 既定 false。true にすると `.npmrc` の未知の configuration key を hard error にする。未知の command line flag と abbreviation はこの設定に関係なく常に error。
- `registry` の既定は `https://registry.npmjs.org/`。

### scoped registry と .npmrc の credential scoping

([Scope](https://docs.npmjs.com/cli/v12/using-npm/scope)、[.npmrc](https://docs.npmjs.com/cli/v11/configuring-npm/npmrc))

- scope と registry は many-to-one。「one registry can host multiple scopes, but a scope only ever points to one registry」。関連付けは `npm config set @myco:registry=http://reg.example.com` か `npm login --registry=http://reg.example.com --scope=@myco`。
- 関連付け後は「any npm install for a package with that scope will request packages from that registry instead」。publish も同様にその registry へ向かう。scoped package は `node_modules/@myorg/packagename` のように scope folder 配下に入る。
- npmrc は 4 ファイル (project / user `$HOME/.npmrc` / global `$PREFIX/etc/npmrc` / builtin) が優先順位付きで読み込まれる。ini 形式の `key = value`、`;` と `#` がコメント。例に `@myscope:registry=https://mycustomregistry.example.org` がある。
- 「The settings `_auth`, `_authToken`, `username`, `_password`, `certfile`, and `keyfile` must all be scoped to a specific registry. This ensures that npm will never send credentials to the wrong host」。scope 表記は `//registry.npmjs.org/:` のような host 単位、または `//my-custom-registry.org/unique/path:` のように path 単位。文書の例は `_authToken=MYTOKEN` の単独行を「bad config」とし、`//registry.npmjs.org/:_authToken=MYTOKEN` を good config とする。
- 未知の `.npmrc` key は npm v11.2.0 から警告され、「In a future major version of npm, these unknown keys may no longer be accepted」。third-party ツール用の key は `.npmrc` に置かない。

### pacote: npm CLI が使う integrity 検証の実装

([npm/pacote README at c82bdcdd8010a9a87c95e1e09b0ba51322b4f93f](https://raw.githubusercontent.com/npm/pacote/c82bdcdd8010a9a87c95e1e09b0ba51322b4f93f/README.md))

- README は pacote を「works with any kind of package specifier that npm can install」とし、npm CLI と同じ specifier を渡せると述べる (「(In fact, that's exactly what the npm CLI does.)」)。
- option `integrity` は「Expected integrity of fetched package tarball. If specified, tarballs with mismatched integrity values will raise an `EINTEGRITY` error」。
- registry manifest の `dist.integrity` は artifact の SRI 文字列で、「may not be present for older packages on the npm registry」。`dist.shasum` は legacy な hex sha1 で、`dist.integrity` が無いときに SRI 文字列へ変換され `manifest._integrity` にコピーされる。
- `verifySignatures` / `verifyAttestations` は manifest の integrity signature / Sigstore attestation を検証する pacote の option だが、「There must be a configured `_keys` entry in the config that is scoped to the registry the manifest is being fetched from」。これは pacote が README に記載した挙動であり、npm 全般の保証ではない。
- pacote 単体の `allowGit` / `allowRemote` / `allowFile` / `allowDirectory` の README 上の既定は `all`。一方 npm CLI v12 の config 既定は `none` (上記)。npm CLI がこの config を pacote へどう渡すかは README に書かれておらず未確認。

## 推奨方法 (上記からの設計上のまとめ)

以下は公式文書の事実ではなく、本リポジトリでの設計案である。

- CI と本番の導入は `npm ci` に固定し、package-lock.json を必ず commit する。npm docs は package-lock.json を teammate・deploy・CI が「install exactly the same dependencies」を保証するためのファイルと説明しており、lock なし install (`npm install`) を本番導入に使うと、その場の registry metadata 解決結果が tree に混入する。
- `integrity` の保証範囲は「lock に記録された SRI と取得した tarball の一致」と理解する (pacote は不一致を `EINTEGRITY` で拒否する)。lock 自身が信頼できる値か (誰がいつ取得したか) は検証対象外なので、lock の diff を監査対象に含める。integrity の出典は registry manifest の `dist.integrity` (古い package では `dist.shasum` の SRI 変換) であり、初回取得時の registry metadata を信頼していることになる。
- private registry / mirror を使うプロジェクトでは、scope への registry 割当を project `.npmrc` に書き、credential は `//host/:` で host (または path) に scoped して commit 可能な形にする。scope は常に 1 registry しか指せないため、registry 間で scope を移すときは割当そのものを変更する作業になる。
- registry をまたぐ運用 (mirror 化、registry 移行) では `replace-registry-host` を明示設定する。既定 `"npmjs"` は `https://registry.npmjs.org` 発の URL のみを置換する挙動で、`"never"` / `"always"` / 特定 host・prefix 指定のどれを取るかで lock の `resolved` が書き換わる範囲が変わる。registry が別ホストに tarball を置く構成では、npm 12 の `allow-remote=none` の hostname 共有例外に入らないため、`replace-registry-host` か override を併記する。
- registry ごとの到達先を完全に固定したい場合は `omit-lockfile-registry-resolved=true` で `resolved` を lock に書かない選択もあるが、後続 install の tarball endpoint 再解決と時間増を許容する設計にする (integrity の扱いは文書に明記がないため、導入前に実測で確認する)。
- `.npmrc` は `strict-npmrc=true` を project 設定に検討する。未知 key が hard error になるため、他ツール用の key の混入や、将来の major での不受理予告を早期に検出できる。
- lock 生成時と `npm ci` で同じ flags を揃える。`.npmrc` (`--location=project`) を commit して flags の再指定漏れを防ぐ。
- `verifySignatures` / `verifyAttestations` を使う場合も、registry スコープの `_keys` 設定が前提であり、pacote の README 上の挙動の範囲と割り切る。署名検証を要件にするなら、npm 側の署名検証コマンドとの使い分けを別途確認する (今回の sources では未確認)。

## 避ける使い方

- package-lock.json を commit しない運用。npm docs はコミット前提と明記しており、CI が tree を固定できない。
- lock 生成時と `npm ci` で異なる flags を混ぜる (`--legacy-peer-deps` / `--install-links` 等)。文書はエラーになりやすいと注意している。
- npm 12 で `allow-git=none` / `allow-remote=none` の既定を確認しないまま git 依存や URL tarball 依存を前提にした導入をする。git 依存は remote repo の configuration を install しうると文書が明記している。
- `resolved` の `registry.npmjs.org` を固定ホストと誤解する。「現在設定されている registry」を指す magic value であり、config により到達先が変わりうる。
- `replace-registry-host` を知らずに mirror/移行を行う。既定 `"npmjs"` の置換範囲と `"never"` / `"always"` の差を無視すると、想定しない host から tarball を取得したり、逆に置換されず旧 registry を参照し続ける。
- host に scoped しない credential 行を `.npmrc` に置く。npm は「never send credentials to the wrong host」を担保するため auth 系 key の registry scoping を要求している。
- 未知の key を `.npmrc` に眠らせる。npm v11.2.0 から警告され、将来の major で不受理になりうる。
- npm v12 で `npm-shrinkwrap.json` を使い続ける。npm v12 は読みも書もしない。
- `node_modules/.package-lock.json` を手動編集して残す。手動編集時は削除を文書が推奨している。
- `verifySignatures` / `verifyAttestations` を「npm 全体の保証」と解釈する。README は pacote の option として記載しているだけである。

## 適用版と本番での注意

- 2026-09-28 に npm Docs 5 ページ (package-lock.json / Config / Scope / npm ci は Select CLI Version 12.1.0 (Latest)、.npmrc は 11.20.0 (Legacy)) と、npm/pacote の commit `c82bdcdd8010a9a87c95e1e09b0ba51322b4f93f` 固定 README (package.json version 22.0.0、embedded CLI banner は v10.1.1) を取得して確認した。いずれも取得時点のページで、将来も最新とは扱わない。npm docs の content は npm/documentation リポジトリの CC-BY-4.0、pacote は ISC。
- 本文の `expires_at` は 2026-12-27 (official_docs TTL 90 日)。`supply-chain-security` は technology TTL の設定外。
- **未確認**: npm CLI が config の `allow-git` / `allow-remote` を pacote にどう渡すか (pacote 単体の README 上の既定は `all`)、`EINTEGRITY` 発生時に npm CLI が出力と exit code をどう扱うか、`omit-lockfile-registry-resolved=true` 時の `integrity` の書き方、lockfile への integrity 書き込み経路の内部実装、npm audit signatures / ECDSA registry signatures との統合、hidden lockfile の mtime 判定の境界ケース。実装前に該当公式ページを再取得して確認する。
- 本文の「要点」は取得した公式文書の記載の要約であり、「推奨方法」は本リポジトリの設計案と区別している。

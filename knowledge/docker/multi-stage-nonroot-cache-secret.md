---
{
  "id": "docker-multi-stage-nonroot-cache-secret",
  "title": "Docker multi-stage builds と non-root 実行ユーザ・cache/secret マウント",
  "kind": "knowledge",
  "technology": "docker",
  "version": "Docker Docs Build pages (retrieved 2026-09-26; example syntax docker/dockerfile:1, golang:1.26)",
  "tags": [
    "research-domain:infrastructure",
    "docker",
    "multi-stage",
    "non-root",
    "USER",
    "BuildKit",
    "COPY---from",
    "RUN---mount",
    "cache-mount",
    "secret-mount",
    "bind-mount"
  ],
  "sources": [
    {
      "id": "docker-multi-stage-builds-docs",
      "url": "https://docs.docker.com/build/building/multi-stage/",
      "type": "official_docs"
    },
    {
      "id": "docker-build-secrets-docs",
      "url": "https://docs.docker.com/build/building/secrets/",
      "type": "official_docs"
    },
    {
      "id": "docker-build-best-practices-docs",
      "url": "https://docs.docker.com/build/building/best-practices/",
      "type": "official_docs"
    },
    {
      "id": "docker-build-cache-optimize-docs",
      "url": "https://docs.docker.com/build/cache/optimize/",
      "type": "official_docs"
    }
  ],
  "retrieved_at": "2026-09-26",
  "expires_at": "2026-12-25",
  "trust": "official",
  "status": "active"
}
---

# Docker multi-stage builds と non-root 実行ユーザ・cache/secret マウント

ビルド用ツールと実行時成果物を分離し、runtime を non-root の USER で動かし、依存取得の高速化と secret の非残留を両立する。以下は公式文書の記載事実と、それを組み立てる設計案を分けて書く。各文の根拠ページは節末のリンクで示す。4ページはいずれも2026-09-26に確認した内容に基づく。

## 要点（公式文書に記載された事実）

### multi-stage の基本契約

- Dockerfile の各 `FROM` が新しい stage を開始する。`COPY --from=0` のように番号指定、または `FROM golang:1.26 AS base` のような `AS <NAME>` による名前指定で `COPY --from=<name>` と書ける。番号は `Dockerfile` 内の `FROM` の出現順に対応する。確認例のページは `syntax=docker/dockerfile:1` を指定し、`golang:1.26` の build stage から `scratch` の runtime stage へ成果物を運ぶ構成を示す。[Multi-stage builds](https://docs.docker.com/build/building/multi-stage/)
- stage 間の受け渡しは `COPY --from=<stage>` で行い、build に使ったツール類は前の stage に残して最終イメージに入れない。外部イメージからの `COPY --from=<image>`、直前の stage を基にする `FROM <prior-stage>` の形も記載されている。[Multi-stage builds](https://docs.docker.com/build/building/multi-stage/)
- BuildKit は target が依存する stage だけをビルドする。最終 target から到達できない stage はビルドされない。[Multi-stage builds](https://docs.docker.com/build/building/multi-stage/)

### non-root 実行（USER 節）

- サービスが権限なしで動けるなら `USER` で non-root に切り替えると best practices に明記されている。例として `groupadd -S appgroup && adduser -S appuser -G appgroup` のように明示のグループ・ユーザを作成し、`--no-log-init` の注記付きで示される。[Building best practices](https://docs.docker.com/build/building/best-practices/)
- `sudo` の使用を避け、昇格が必要な場面の代替として `gosu` の検討に触れる。`USER` の頻繁な切り替えを避けることも明記されている。[Building best practices](https://docs.docker.com/build/building/best-practices/)
- multi-stage builds を使い、必要なファイルだけを最終イメージに残すことと、stage の並列化に触れる。一時的な context ファイルには bind mounts、stage 間の受け渡しには `COPY` を選ぶことが推奨されている。[Building best practices](https://docs.docker.com/build/building/best-practices/)

### build secrets

- secret の利用は二段階である。まず `docker build --secret id=aws,src=$HOME/.aws/credentials` のように CLI 側で secret を渡し、次に `RUN --mount=type=secret,id=aws ...` のように `RUN --mount=type=secret` で参照する。[Build secrets](https://docs.docker.com/build/building/secrets/)
- secret はファイルとしてマウントされる。既定パスは `/run/secrets/<id>` で、`target=` や `env=` で配置や環境変数経由の参照に変えられることが記載されている。[Build secrets](https://docs.docker.com/build/building/secrets/)
- build args と `ENV` はイメージに残るため secret には不適切と明記されている。[Build secrets](https://docs.docker.com/build/building/secrets/)
- SSH による取得は `--ssh` マウントで行い、`GIT_AUTH_TOKEN` / `GIT_AUTH_HEADER` による pre-flight の secret 取得に触れる。[Build secrets](https://docs.docker.com/build/building/secrets/)

### cache mounts と bind mounts

- `RUN --mount=type=cache,target=...` はビルド間で保持されるキャッシュで、npm・Go・apt・pip・cargo・NuGet・composer 向けに用途別のパス例が示されている。[Optimize cache usage in builds](https://docs.docker.com/build/cache/optimize/)
- apt のキャッシュは排他アクセスのため `sharing=locked` が必要と記載されている。[Optimize cache usage in builds](https://docs.docker.com/build/cache/optimize/)
- bind mounts は既定で read-only であり、イメージにも cache にも保持されない。[Optimize cache usage in builds](https://docs.docker.com/build/cache/optimize/)
- cache の内容は性能目的の best-effort として扱う注意が記載されている。正確性のために cache に依存しない。[Optimize cache usage in builds](https://docs.docker.com/build/cache/optimize/)

## 推奨方法（上記の事実を組み立てる独自の設計案）

以下は公式の契約そのものではなく、本ドキュメントの組み立て提案である。

- build stage でコンパイル・依存解決し、runtime stage は最小基盤（例: `scratch` や slim 系）にして `COPY --from=<build-stage>` で成果物だけを運ぶ。最終 target から辿れる stage だけがビルドされる性質を利用し、検証用・デバッグ用の stage は既定 target の依存から外す。
- runtime stage で明示の UID/GID の non-root ユーザを作成して `USER` で切り替え、実行に不要な build ツール・package manager のキャッシュ・一時ファイルを持ち込まない。実行ユーザが成果物を読める配置にする（所有者と権限の付け方は基盤イメージの流儀に従って決める）。
- 依存取得の `RUN` には `type=cache` を付け、apt 系には `sharing=locked` を付ける。cache は高速化専用と割り切り、初回（cold cache）でも正しくビルドできる `RUN` にする。
- private リポジトリやレジストリの認証情報は `ARG`・`ENV` に入れず、`RUN --mount=type=secret` または `--ssh` で取得時のみ参照する。既定では `/run/secrets/<id>` のファイルとして読み、`target=`・`env=` は受け側ツールの形式に合わせるときだけ使う。
- 一時的な context ファイルの参照は bind mounts に寄せ、最終イメージや cache に残さない。stage 間の受け渡しが必要な成果物だけ `COPY --from` で運ぶ。

## 避ける使い方

- secret を build args や `ENV` で渡す。イメージの履歴・環境に残り、後の layer から読み出せる。公式も不適切と明記する。[Build secrets](https://docs.docker.com/build/building/secrets/)
- `sudo` を最終イメージに残す、不要なのに root のまま動かす、`USER` を頻繁に切り替える。いずれも best practices が避ける側に記す。[Building best practices](https://docs.docker.com/build/building/best-practices/)
- cache の存在を正しさの前提にする。cache は best-effort であり、消えても同じ成果物ができる `RUN` にしないと再現性が崩れる。[Optimize cache usage in builds](https://docs.docker.com/build/cache/optimize/)
- apt の cache mount に `sharing=locked` を付けずに並列実行する。排他が必要と文書が指定する。[Optimize cache usage in builds](https://docs.docker.com/build/cache/optimize/)
- bind mount の内容がイメージや cache に残る前提で書く。bind は既定 read-only で非保持と文書が指定する。[Optimize cache usage in builds](https://docs.docker.com/build/cache/optimize/)

## 適用版と本番での注意

- 適用版: Docker Docs の Build ページ群。ページ自体に版表示はなく、本ドキュメントは例示の `syntax=docker/dockerfile:1` と `golang:1.26` を適用条件として記録する。将来の最新とは扱わない。
- 再確認期限: 全 source が `official_docs`（TTL 90日）で技術固有 TTL の対象外のため、2026-12-25 に再取得して内容を確認する。
- 未確認事項（本調査の範囲外として推測で埋めない）: 各言語・ツール別の cache `target=` パスの完全一覧、SSH マウントの詳細フラグ体系、groupadd/adduser 以外の基盤（Alpine の addgroup/adduser 等）での等価手順、build の timeout・cancel・shutdown 時の mount 内容の扱い。これらは該当ページの該当節を別途確認する。
- 本ドキュメントの推奨構成は設計案であり、特定ベンチマークや単一事例の一般化ではない。効果（層サイズ・再ビルド時間）はワークロードと cache ヒット率に依存するため、対象リポジトリで測定して採用する。

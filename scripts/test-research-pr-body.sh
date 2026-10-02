#!/bin/bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
export GIT_AUTHOR_NAME='PR template test' GIT_COMMITTER_NAME='PR template test'
export GIT_AUTHOR_EMAIL='pr-test@example.invalid' GIT_COMMITTER_EMAIL='pr-test@example.invalid'
mkdir -p "$TEST_DIR/repo/scripts" "$TEST_DIR/repo/.github"
cp "$ROOT/scripts/research-pr-body.sh" "$TEST_DIR/repo/scripts/"
cp "$ROOT/.github/pull_request_template.md" "$TEST_DIR/repo/.github/"
cd "$TEST_DIR/repo"
git init --quiet -b main
git config maintenance.auto false
echo fixture > README.md
git add .
git commit --quiet -m fixture
base="$(git rev-parse HEAD)"
fail() { if "$@" > "$TEST_DIR/failure.log" 2>&1; then printf 'unexpected success: %s\n' "$*" >&2; exit 1; fi; }
render() { bash scripts/research-pr-body.sh "$@"; }
fail render topic "$base" 0 # No committed research artifacts.
mkdir -p knowledge/testing evals/knowledge sources/catalog
cat > knowledge/testing/one.md <<'DOC'
---
{"sources":[{"id":"existing-source","url":"https://example.test/existing?a=1&b=2"},{"id":"new-source","url":"https://example.test/new"}]}
---
# Fixture
DOC
cp knowledge/testing/one.md knowledge/testing/two.md
printf '{}\n' > evals/knowledge/testing.json
printf '{}\n' > sources/catalog/new-source.json
git add knowledge evals sources
git commit --quiet -m research
head="$(git rev-parse HEAD)"
# Read exact committed content, not subsequent dirty edits. Topic text stays literal.
printf 'uncommitted invalid metadata\n' > knowledge/testing/one.md
topic='日本語 / $(touch should-not-exist) / `not a command`'
render "$topic" "$base" 0 > "$TEST_DIR/body.md"
diff <(grep '^## ' .github/pull_request_template.md) <(grep '^## ' "$TEST_DIR/body.md")
grep -Fq "対象テーマ: $topic" "$TEST_DIR/body.md"
[[ ! -e should-not-exist ]]
grep -Fq -- '- `knowledge/testing/one.md`' "$TEST_DIR/body.md"
grep -Fq -- '- `evals/knowledge/testing.json`' "$TEST_DIR/body.md"
grep -Fq -- '- `sources/catalog/new-source.json`' "$TEST_DIR/body.md"
[[ "$(grep -Fc '[existing-source](https://example.test/existing?a=1&b=2)' "$TEST_DIR/body.md")" == 1 ]]
grep -Fq '[new-source](https://example.test/new)' "$TEST_DIR/body.md"
grep -Fq "baseline \`$base\` / PR 作成時 head \`$head\`" "$TEST_DIR/body.md"
grep -Fq -- '- 成功: commit 前のローカル検証' "$TEST_DIR/body.md"
grep -Fq -- '- 失敗:' "$TEST_DIR/body.md"
grep -Fq -- '- 未実行:' "$TEST_DIR/body.md"
grep -Fq -- '- CI: 未確認' "$TEST_DIR/body.md"
! grep -Fq '部分完了' "$TEST_DIR/body.md"
! grep -Eq '\[x\]|CI.*成功|github.com/.*/actions/runs/' "$TEST_DIR/body.md"
render topic "$base" 1 > "$TEST_DIR/partial.md"
grep -Fq '部分完了: 成功した worker の成果だけを含みます' "$TEST_DIR/partial.md"
fail render topic invalid 0
fail render topic 0000000000000000000000000000000000000000 0
fail render topic "$base" 2
fail render topic "$base"
# Missing/duplicate template sections, unknown sections, malformed metadata and missing
# source references must stop PR creation instead of emitting a misleading body.
grep -v '^## 出典$' .github/pull_request_template.md > "$TEST_DIR/incomplete.md"
cp "$TEST_DIR/incomplete.md" .github/pull_request_template.md
fail render topic "$base" 0
cp "$ROOT/.github/pull_request_template.md" .github/pull_request_template.md
sed 's/^## 出典$/## 変更内容/' .github/pull_request_template.md > "$TEST_DIR/duplicate.md"
cp "$TEST_DIR/duplicate.md" .github/pull_request_template.md
fail render topic "$base" 0
cp "$ROOT/.github/pull_request_template.md" .github/pull_request_template.md
printf '\n## Unsupported\n' >> .github/pull_request_template.md
fail render topic "$base" 0
cp "$ROOT/.github/pull_request_template.md" .github/pull_request_template.md
git add knowledge/testing/one.md
git commit --quiet -m malformed
fail render topic "$base" 0
printf '%s\n' '---' '{"sources":[]}' '---' > knowledge/testing/one.md
git add knowledge/testing/one.md
git commit --quiet -m missing-source
fail render topic "$base" 0
printf 'shared PR template rendering: passed (headings, committed sources, complete/partial, literal text, fail-closed inputs)\n'

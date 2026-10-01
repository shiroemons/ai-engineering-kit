#!/usr/bin/env bash
set -euo pipefail
source_root=$(cd "$(dirname "$0")/.." && pwd)
temp=$(mktemp -d)
trap 'rm -rf "$temp"' EXIT
export GIT_AUTHOR_NAME='Research test' GIT_COMMITTER_NAME='Research test'
export GIT_AUTHOR_EMAIL='research-test@example.invalid' GIT_COMMITTER_EMAIL='research-test@example.invalid'
git init --quiet --bare "$temp/remote.git"
git init --quiet -b main "$temp/one"
cd "$temp/one"
git config maintenance.auto false
mkdir scripts
cp "$source_root/scripts/research-gpt-state.sh" "$source_root/scripts/research-gpt-prepare.sh" scripts/
echo fixture > README.md
git add .
git commit --quiet -m fixture
git remote add origin "$temp/remote.git"
git push --quiet -u origin main
git --git-dir="$temp/remote.git" symbolic-ref HEAD refs/heads/main
git clone --quiet "$temp/remote.git" "$temp/two"
git -C "$temp/two" config maintenance.auto false
state() { bash scripts/research-gpt-state.sh "$@"; }
fail() { if "$@" >"$temp/failure.log" 2>&1; then echo "unexpected success: $*" >&2; exit 1; fi; }
publish() {
  local candidate=$1 blob tree commit ref parent
  blob=$(jq -c .state "$candidate" | git hash-object -w --stdin)
  tree=$(printf '100644 blob %s\tstate.json\n' "$blob" | git mktree)
  parent=$(jq -r .expected_parent "$candidate")
  commit=$(printf 'research lease test\n' | git commit-tree "$tree" -p "$parent")
  ref=$(jq -r .ref "$candidate")
  git push --quiet origin "$commit:$ref" || return
  printf '%s\n' "$commit"
}

state status | jq -e '.state == null and .expired == false' >/dev/null
fail state claim '../bad-id'
state claim test-run-one > "$temp/first.json"
(cd "$temp/two"; bash scripts/research-gpt-state.sh claim test-run-two) > "$temp/racing.json"
first=$(publish "$temp/first.json")
fail publish "$temp/racing.json"
state verify test-run-one "$first" | jq -e '.status == "active"' >/dev/null
fail state verify test-run-two "$first"
fail state verify test-run-one deadbeef
fail state claim test-run-two
fail state release test-run-two
bash scripts/research-gpt-prepare.sh test-run-one "$first" "$temp/worktree" >/dev/null
test "$(git -C "$temp/worktree" branch --show-current)" = research/gpt-test-run-one
test "$(git branch --show-current)" = main
fail bash scripts/research-gpt-prepare.sh test-run-one "$first" "$temp/worktree"

# Expiry stops the owner too, but never grants a new worker takeover rights.
state release test-run-one > "$temp/expired.json"
jq '.state.status="active" | .state.started_at=1 | .state.expires_at=7201' "$temp/expired.json" > "$temp/expired-active.json"
expired=$(publish "$temp/expired-active.json")
state status | jq -e .expired >/dev/null
fail state verify test-run-one "$expired"
fail state claim test-run-two
state release test-run-one > "$temp/release.json"
publish "$temp/release.json" >/dev/null
state status | jq -e '.state.status == "idle"' >/dev/null

echo dirty > unexpected.txt
fail state claim test-run-two
rm unexpected.txt
state claim test-run-two > "$temp/second.json"
second=$(publish "$temp/second.json")
echo advanced >> README.md
git commit --quiet -am advanced
git push --quiet origin main
fail bash scripts/research-gpt-prepare.sh test-run-two "$second" "$temp/new-worktree"
test ! -e "$temp/new-worktree"
# jq normally accepts JSON streams. A coordinator must be exactly one object.
git show "$second:state.json" > "$temp/malformed.json"
git show "$second:state.json" >> "$temp/malformed.json"
blob=$(git hash-object -w "$temp/malformed.json")
tree=$(printf '100644 blob %s\tstate.json\n' "$blob" | git mktree)
malformed=$(printf 'malformed fixture\n' | git commit-tree "$tree" -p "$second")
git push --quiet origin "$malformed:refs/heads/research/gpt-coordinator"
fail state status
fail state claim test-run-three
fail state verify test-run-two "$malformed"
fail state release test-run-two
git remote set-url origin "$temp/does-not-exist.git"
fail state status
echo 'GPT lease/prepare tests passed (cross-clone race, ownership, expiry, recovery, latest-main, fail-closed network)'

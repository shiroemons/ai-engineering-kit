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
mkdir -p nested
echo 'preserve nested content' > 'nested/file with spaces.txt'
printf '#!/bin/sh\nexit 0\n' > nested/executable.sh
chmod +x nested/executable.sh
ln -s ../README.md nested/link
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
  local candidate=$1 tree commit ref parent
  tree=$(state tree "$candidate") || return
  parent=$(jq -r .expected_parent "$candidate") || return
  commit=$(printf 'research lease test\n' | git commit-tree "$tree" -p "$parent") || return
  state verify-commit "$candidate" "$commit" >&2 || return
  ref=$(jq -r .ref "$candidate") || return
  git push --quiet origin "$commit:$ref" || return
  printf '%s\n' "$commit"
}
only_state_changed() {
  test "$(git diff-tree --no-commit-id --name-status -r --no-renames "$1" "$2")" = "$(printf '%s\tstate.json' "$3")"
}

state status | jq -e '.state == null and .expired == false' >/dev/null
fail state claim '../bad-id'
state claim test-run-one > "$temp/first.json"
(cd "$temp/two"; bash scripts/research-gpt-state.sh claim test-run-two) > "$temp/racing.json"
base=$(git rev-parse HEAD)
test "$(jq -r .base_tree "$temp/first.json")" = "$(git rev-parse 'HEAD^{tree}')"

# Local construction must preserve the caller's staged/unstaged work and index.
echo staged > local-only.txt
git add local-only.txt
echo unstaged >> local-only.txt
before_status=$(git status --porcelain --untracked-files=all)
before_index=$(git write-tree)
first_tree=$(state tree "$temp/first.json")
test "$(git status --porcelain --untracked-files=all)" = "$before_status"
test "$(git write-tree)" = "$before_index"
test "$(cat local-only.txt)" = "$(printf 'staged\nunstaged')"
git restore --staged local-only.txt
rm local-only.txt

# The previous state-only construction deletes the parent's source files.
blob=$(jq -c .state "$temp/first.json" | git hash-object -w --stdin)
unsafe_tree=$(printf '100644 blob %s\tstate.json\n' "$blob" | git mktree)
unsafe=$(printf 'unsafe initial fixture\n' | git commit-tree "$unsafe_tree" -p "$base")
fail state verify-commit "$temp/first.json" "$unsafe"
# Local replacement refs must not disguise the actual object sent to a server.
safe=$(printf 'safe initial fixture\n' | git commit-tree "$first_tree" -p "$base")
long_message=$(printf '%200000s' 'large commit message fixture')
long_commit=$(printf '%s\nparent ignored-message-line\n' "$long_message" | git commit-tree "$first_tree" -p "$base")
state verify-commit "$temp/first.json" "$long_commit" >/dev/null
git replace "$unsafe" "$safe"
fail state verify-commit "$temp/first.json" "$unsafe"
git replace -d "$unsafe" >/dev/null
# Fault injection: failed reads/writes must never fall through to a partial tree
# and accidentally approve the exact destructive candidate above.
mkdir "$temp/failing-tools"
real_git=$(command -v git)
cat > "$temp/failing-tools/git" <<'SH'
#!/usr/bin/env bash
for argument in "$@"; do
  if [ "$argument" = "$FAIL_GIT_COMMAND" ]; then
    echo "injected git failure: $argument" >&2
    exit 1
  fi
done
exec "$REAL_GIT" "$@"
SH
chmod +x "$temp/failing-tools/git"
for command in ls-tree mktree; do
  fail env PATH="$temp/failing-tools:$PATH" REAL_GIT="$real_git" FAIL_GIT_COMMAND="$command" bash scripts/research-gpt-state.sh tree "$temp/first.json"
  fail env PATH="$temp/failing-tools:$PATH" REAL_GIT="$real_git" FAIL_GIT_COMMAND="$command" bash scripts/research-gpt-state.sh verify-commit "$temp/first.json" "$unsafe"
done
fail env PATH="$temp/failing-tools:$PATH" REAL_GIT="$real_git" FAIL_GIT_COMMAND=ls-tree bash scripts/research-gpt-state.sh claim test-read-failure
# Even empty subtree entries and unusual path bytes must survive unchanged.
empty_tree=$(git mktree </dev/null)
unusual_blob=$(printf 'preserve exact names\n' | git hash-object -w --stdin)
{
  git ls-tree -z "$base"
  printf '040000 tree %s\tempty-dir\0' "$empty_tree"
  printf '100644 blob %s\ttab\tand\nnewline.txt\0' "$unusual_blob"
} > "$temp/unusual-entries"
unusual_tree=$(git mktree -z < "$temp/unusual-entries")
unusual_parent=$(printf 'unusual paths fixture\n' | git commit-tree "$unusual_tree" -p "$base")
jq --arg parent "$unusual_parent" --arg tree "$unusual_tree" '.expected_parent=$parent | .base_tree=$tree | .state.base_sha=$parent' "$temp/first.json" > "$temp/unusual-transition.json"
preserved_tree=$(state tree "$temp/unusual-transition.json")
test "$(git rev-parse "$preserved_tree:empty-dir")" = "$empty_tree"
test "$(git rev-parse "$preserved_tree:$(printf 'tab\tand\nnewline.txt')")" = "$unusual_blob"
preserved_commit=$(printf 'preserved unusual paths fixture\n' | git commit-tree "$preserved_tree" -p "$unusual_parent")
state verify-commit "$temp/unusual-transition.json" "$preserved_commit" >/dev/null
only_state_changed "$unusual_parent" "$preserved_commit" A
fail state verify-commit "$temp/first.json" "$base"
orphan=$(printf 'orphan fixture\n' | git commit-tree "$first_tree")
fail state verify-commit "$temp/first.json" "$orphan"
merge=$(printf 'merge fixture\n' | git commit-tree "$first_tree" -p "$base" -p "$orphan")
fail state verify-commit "$temp/first.json" "$merge"
# Legacy local grafts can hide a merge parent from revision traversal too.
printf '%s %s\n' "$merge" "$base" > .git/info/grafts
fail state verify-commit "$temp/first.json" "$merge"
rm .git/info/grafts
wrong_parent=$(printf 'wrong parent fixture\n' | git commit-tree "$first_tree" -p "$orphan")
fail state verify-commit "$temp/first.json" "$wrong_parent"
fail state verify-commit "$temp/first.json" HEAD
fail state verify-commit "$temp/first.json" "$first_tree"

# Malformed/stale transition metadata must fail before constructing a tree.
jq 'del(.base_tree)' "$temp/first.json" > "$temp/bad-transition.json"
fail state tree "$temp/bad-transition.json"
jq '.base_tree = .expected_parent' "$temp/first.json" > "$temp/bad-transition.json"
fail state tree "$temp/bad-transition.json"
jq '.ref = "refs/heads/main"' "$temp/first.json" > "$temp/bad-transition.json"
fail state tree "$temp/bad-transition.json"
jq '.state.base_sha = "bad"' "$temp/first.json" > "$temp/bad-transition.json"
fail state tree "$temp/bad-transition.json"
cat "$temp/first.json" "$temp/first.json" > "$temp/bad-transition.json"
fail state tree "$temp/bad-transition.json"

first=$(publish "$temp/first.json")
only_state_changed "$base" "$first" A
test "$(git rev-parse "$base:nested")" = "$(git rev-parse "$first:nested")"
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
released=$(publish "$temp/release.json")
only_state_changed "$expired" "$released" M
state status | jq -e '.state.status == "idle"' >/dev/null

echo dirty > unexpected.txt
fail state claim test-run-two
rm unexpected.txt
state claim test-run-two > "$temp/second.json"
second=$(publish "$temp/second.json")
only_state_changed "$released" "$second" M
echo advanced >> README.md
git commit --quiet -am advanced
git push --quiet origin main
fail bash scripts/research-gpt-prepare.sh test-run-two "$second" "$temp/new-worktree"
test ! -e "$temp/new-worktree"

# Candidate verification allows JSON formatting differences, but rejects every
# unrelated addition, edit, deletion, rename, or mode change, and state mismatch.
state release test-run-two > "$temp/candidate.json"
candidate_parent=$(jq -r .expected_parent "$temp/candidate.json")
candidate_tree=$(state tree "$temp/candidate.json")
edit_tree() (
  export GIT_INDEX_FILE="$temp/edit-index"
  rm -f "$GIT_INDEX_FILE"
  git read-tree "$candidate_tree"
  "$@"
  git write-tree
)
check_rejected_tree() {
  local commit
  commit=$(printf 'rejected candidate fixture\n' | git commit-tree "$1" -p "$candidate_parent")
  fail state verify-commit "$temp/candidate.json" "$commit"
}
other_blob=$(printf 'unexpected\n' | git hash-object -w --stdin)
check_rejected_tree "$(edit_tree git update-index --add --cacheinfo "100644,$other_blob,extra.txt")"
check_rejected_tree "$(edit_tree git update-index --cacheinfo "100644,$other_blob,README.md")"
check_rejected_tree "$(edit_tree git update-index --force-remove README.md)"
rename_readme() {
  git update-index --force-remove README.md
  git update-index --add --cacheinfo "100644,$readme_blob,renamed.md"
}
readme_blob=$(git rev-parse "$candidate_parent:README.md")
check_rejected_tree "$(edit_tree rename_readme)"
check_rejected_tree "$(edit_tree git update-index --cacheinfo "100755,$readme_blob,README.md")"
check_rejected_tree "$(edit_tree git update-index --force-remove state.json)"
check_rejected_tree "$(edit_tree git update-index --cacheinfo "120000,$other_blob,state.json")"
check_rejected_tree "$(edit_tree git update-index --cacheinfo "100755,$other_blob,state.json")"
wrong_state_blob=$(jq '.state.status = "active" | .state' "$temp/candidate.json" | git hash-object -w --stdin)
check_rejected_tree "$(edit_tree git update-index --cacheinfo "100644,$wrong_state_blob,state.json")"
pretty_blob=$(jq -S .state "$temp/candidate.json" | git hash-object -w --stdin)
pretty_tree=$(edit_tree git update-index --cacheinfo "100644,$pretty_blob,state.json")
pretty_commit=$(printf 'formatted candidate fixture\n' | git commit-tree "$pretty_tree" -p "$candidate_parent")
state verify-commit "$temp/candidate.json" "$pretty_commit" >/dev/null

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

# Legacy one-file branches remain usable without rewriting history or restoring
# main's files. Additional parent entries must also be retained, never dropped.
git init --quiet --bare "$temp/legacy.git"
git clone --quiet "$temp/remote.git" "$temp/legacy"
(
  cd "$temp/legacy"
  git remote set-url origin "$temp/legacy.git"
  git push --quiet origin main
  legacy_blob=$(jq -c .state "$temp/release.json" | git hash-object -w --stdin)
  legacy_tree=$(printf '100644 blob %s\tstate.json\n' "$legacy_blob" | git mktree)
  legacy=$(printf 'legacy fixture\n' | git commit-tree "$legacy_tree" -p HEAD)
  git push --quiet origin "$legacy:refs/heads/research/gpt-coordinator"
  state claim legacy-run-one > "$temp/legacy-claim.json"
  legacy_claim=$(publish "$temp/legacy-claim.json")
  only_state_changed "$legacy" "$legacy_claim" M
  test "$(git ls-tree --name-only "$legacy_claim")" = state.json
  # Simulate an existing extra entry; subsequent publication must preserve it.
  legacy_blob=$(git rev-parse "$legacy_claim:state.json")
  other_blob=$(printf 'unexpected\n' | git hash-object -w --stdin)
  extra_tree=$(printf '100644 blob %s\tstate.json\n100644 blob %s\tunexpected.txt\n' "$legacy_blob" "$other_blob" | git mktree)
  extra=$(printf 'extra entry fixture\n' | git commit-tree "$extra_tree" -p "$legacy_claim")
  git push --quiet origin "$extra:refs/heads/research/gpt-coordinator"
  state release legacy-run-one > "$temp/legacy-release.json"
  legacy_release=$(publish "$temp/legacy-release.json")
  only_state_changed "$extra" "$legacy_release" M
  test "$(git rev-parse "$legacy_release:unexpected.txt")" = "$other_blob"
  # Even valid JSON must not be accepted via symlink or executable state paths.
  for mode in 120000 100755; do
    bad_tree=$(printf '%s blob %s\tstate.json\n' "$mode" "$legacy_blob" | git mktree)
    bad=$(printf 'bad state mode fixture\n' | git commit-tree "$bad_tree" -p "$legacy_release")
    git push --quiet origin "$bad:refs/heads/research/gpt-coordinator"
    fail state status
    fail state claim legacy-run-two
    fail state release legacy-run-one
    legacy_release=$bad
  done
)

# A new coordinator must never overwrite a pre-existing main state.json path.
git init --quiet --bare "$temp/collision.git"
git clone --quiet "$temp/remote.git" "$temp/collision"
(
  cd "$temp/collision"
  git remote set-url origin "$temp/collision.git"
  echo 'application state' > state.json
  git add state.json
  git commit --quiet -m 'state path collision fixture'
  git push --quiet origin main
  fail state claim collision-run
  jq --arg parent "$(git rev-parse HEAD)" --arg tree "$(git rev-parse 'HEAD^{tree}')" '.expected_parent=$parent | .base_tree=$tree | .state.base_sha=$parent' "$temp/first.json" > "$temp/collision-transition.json"
  fail state tree "$temp/collision-transition.json"
)
git remote set-url origin "$temp/does-not-exist.git"
fail state status
echo 'GPT lease/prepare tests passed (tree preservation, candidate validation, legacy compatibility, cross-clone race, ownership, expiry, recovery, latest-main, fail-closed network)'

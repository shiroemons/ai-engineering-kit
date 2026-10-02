#!/usr/bin/env bash
# Produce reviewable remote lease transitions; publication belongs to the caller.
set -euo pipefail
# Validate actual published objects, not a local git-replace view of them.
export GIT_NO_REPLACE_OBJECTS=1

die() { printf 'research-gpt state: %s\n' "$*" >&2; exit 1; }
root=$(git rev-parse --show-toplevel)
cd "$root"
ref=refs/heads/research/gpt-coordinator

valid_state() {
  jq -se '
    length == 1 and (.[0] | type == "object" and .schema == 1 and
    (.status == "active" or .status == "idle") and
    (.run_id | type == "string" and test("^[a-z0-9][a-z0-9-]{7,79}$")) and
    (.base_sha | type == "string" and test("^[0-9a-f]{40}$")) and
    .branch == ("research/gpt-" + .run_id) and
    (.started_at | type == "number") and (.expires_at | type == "number") and
    .expires_at > .started_at and .expires_at - .started_at <= 7200)
  ' >/dev/null
}

read_state() {
  local entry mode kind oid path
  entry=$(git ls-tree "$1" -- state.json) || die 'cannot inspect state.json entry'
  IFS=$' \t' read -r mode kind oid path <<< "$entry"
  [ "$mode" = 100644 ] && [ "$kind" = blob ] && [ "$path" = state.json ] || die 'state.json must be a regular non-executable blob'
  git show "$1:state.json"
}

read_transition() {
  transition=$(cat -- "$1")
  printf '%s\n' "$transition" | jq -se --arg ref "$ref" '
    length == 1 and (.[0] | type == "object" and .ref == $ref and
    (.expected_parent | type == "string" and test("^[0-9a-f]{40}$")) and
    (.base_tree | type == "string" and test("^[0-9a-f]{40}$")) and
    (.create | type == "boolean"))
  ' >/dev/null || die 'invalid transition; regenerate it with claim or release'
  parent=$(printf '%s' "$transition" | jq -r .expected_parent)
  [ "$(git cat-file -t "$parent")" = commit ] || die 'expected_parent must be a commit'
  [ "$(printf '%s' "$transition" | jq -r .base_tree)" = "$(git rev-parse "$parent^{tree}")" ] || die 'base_tree does not match expected_parent'
  state=$(printf '%s' "$transition" | jq -c .state)
  printf '%s\n' "$state" | valid_state || die 'invalid transition state'
  if [ "$(printf '%s' "$transition" | jq -r .create)" = true ]; then
    initial_entry=$(git ls-tree "$parent" -- state.json) || die 'cannot inspect initial state.json path'
    [ -z "$initial_entry" ] || die 'initial parent already contains state.json; inspect rather than overwrite it'
    printf '%s' "$state" | jq -e --arg parent "$parent" '.status == "active" and .base_sha == $parent' >/dev/null || die 'initial claim must use expected_parent as its base'
  else
    read_state "$parent" | valid_state || die 'invalid parent coordinator state'
  fi
}

# Preserve root entries verbatim, retaining every nested tree by object ID. An
# index round-trip is insufficient: it can silently drop explicit empty trees.
# Neither the caller's index/worktree nor any local/remote ref is changed.
state_tree() (
  # Callers use command substitutions/conditions, where Bash may disable errexit.
  # Check each operation explicitly; failed reads must never yield a partial tree.
  temp=$(mktemp -d) || die 'cannot create temporary tree directory'
  trap 'rm -rf "$temp"' EXIT
  git ls-tree -z "$parent" > "$temp/parent" || die 'cannot read the parent tree'
  parent_tree=$(git rev-parse "$parent^{tree}") || die 'cannot resolve the parent tree'
  roundtrip=$(git mktree -z < "$temp/parent") || die 'cannot reconstruct the complete parent tree'
  [ "$roundtrip" = "$parent_tree" ] || die 'parent tree cannot be preserved exactly'
  while IFS= read -r -d '' entry; do
    if [ "${entry#*$'\t'}" != state.json ]; then
      printf '%s\0' "$entry" || die 'cannot preserve parent entry'
    fi
  done < "$temp/parent" > "$temp/updated" || die 'cannot preserve parent entries'
  printf '100644 blob %s\tstate.json\0' "$1" >> "$temp/updated" || die 'cannot add state.json entry'
  git mktree -z < "$temp/updated" || die 'cannot write the preserved tree'
)

action=${1:-status}
run=${2:-}
case "$action" in
  tree|verify-commit)
    if [ "$action" = tree ]; then
      [ "$#" -eq 2 ] || die 'usage: tree TRANSITION_FILE'
    else
      [ "$#" -eq 3 ] || die 'usage: verify-commit TRANSITION_FILE COMMIT_SHA'
    fi
    read_transition "$2"
    if [ "$action" = tree ]; then
      blob=$(printf '%s\n' "$state" | git hash-object -w --stdin)
      state_tree "$blob"
    else
      [[ "$3" =~ ^[0-9a-f]{40}$ ]] || die 'candidate must be a full commit SHA'
      [ "$(git cat-file -t "$3")" = commit ] || die 'candidate must be a commit'
      # Read raw headers so local grafts/shallow traversal cannot hide parents.
      candidate_parents=$(git cat-file commit "$3" | sed -n '1,/^$/s/^parent //p')
      [ "$candidate_parents" = "$parent" ] || die 'candidate must have exactly expected_parent as its single parent'
      candidate_state=$(read_state "$3")
      printf '%s\n' "$candidate_state" | valid_state || die 'invalid candidate state'
      printf '%s\n' "$candidate_state" | jq -e --argjson expected "$state" '. == $expected' >/dev/null || die 'candidate state differs from transition'
      blob=$(git rev-parse "$3:state.json")
      expected_tree=$(state_tree "$blob") || die 'cannot construct the expected tree; do not publish'
      [ "$(git rev-parse "$3^{tree}")" = "$expected_tree" ] || die 'candidate changes paths other than state.json; do not publish'
      printf 'Verified state.json-only change at %s\n' "$3"
    fi
    exit 0
    ;;
  status) [ "$#" -le 1 ] || die 'usage: status' ;;
  claim|release) [ "$#" -eq 2 ] || die 'usage: claim|release RUN_ID' ;;
  verify) [ "$#" -eq 3 ] || die 'usage: verify RUN_ID CLAIM_SHA' ;;
  *) die 'expected status, claim, release, verify, tree, or verify-commit' ;;
esac
if [ "$action" != status ]; then
  [[ "$run" =~ ^[a-z0-9][a-z0-9-]{7,79}$ ]] || die 'RUN_ID must be 8..80 lowercase letters, digits, or hyphens'
fi

# Do not treat network/authentication failures as an absent coordinator.
if remote=$(git ls-remote --exit-code --heads origin "$ref"); then
  sha=${remote%%[[:space:]]*}
  [[ "$sha" =~ ^[0-9a-f]{40}$ ]] || die 'invalid remote coordinator SHA'
  git fetch --quiet --no-tags origin "$sha"
  state=$(read_state "$sha")
  printf '%s\n' "$state" | valid_state || die 'invalid coordinator state; inspect rather than overwrite it'
  create=false
else
  rc=$?
  [ "$rc" -eq 2 ] || die 'cannot read coordinator from origin'
  sha=
  state='null'
  create=true
fi
now=$(date -u +%s)
case "$action" in
  status)
    jq -n --arg ref "$ref" --arg sha "$sha" --argjson state "$state" --argjson now "$now" \
      '{ref:$ref,sha:$sha,state:$state,expired:($state != null and $state.status == "active" and $state.expires_at <= $now)}'
    ;;
  claim)
    [ -z "$(git status --porcelain --untracked-files=all)" ] || die 'claim requires a clean checkout'
    [ "$(printf '%s' "$state" | jq -r '.status')" != active ] || die 'another run owns the remote lease; expiry never authorizes takeover'
    git fetch --quiet --no-tags origin refs/heads/main:refs/remotes/origin/main
    base=$(git rev-parse refs/remotes/origin/main)
    parent=${sha:-$base}
    if [ "$create" = true ]; then
      initial_entry=$(git ls-tree "$parent" -- state.json) || die 'cannot inspect initial state.json path'
      [ -z "$initial_entry" ] || die 'initial parent already contains state.json; inspect rather than overwrite it'
    fi
    base_tree=$(git rev-parse "$parent^{tree}")
    jq -n --arg ref "$ref" --arg parent "$parent" --arg tree "$base_tree" --argjson create "$create" --arg run "$run" --arg base "$base" --argjson now "$now" \
      '{ref:$ref,expected_parent:$parent,base_tree:$tree,create:$create,state:{schema:1,status:"active",run_id:$run,branch:("research/gpt-"+$run),base_sha:$base,started_at:$now,expires_at:($now+7200)}}'
    ;;
  verify)
    [ "$sha" = "$3" ] || die 'remote lease changed; stop this run'
    printf '%s' "$state" | jq -e --arg run "$run" --argjson now "$now" \
      '.status == "active" and .run_id == $run and .expires_at > $now' >/dev/null || die 'lease is not active, owned, and unexpired'
    printf '%s\n' "$state"
    ;;
  release)
    printf '%s' "$state" | jq -e --arg run "$run" '.status == "active" and .run_id == $run' >/dev/null || die 'cannot release another or inactive run'
    base_tree=$(git rev-parse "$sha^{tree}")
    jq -n --arg ref "$ref" --arg parent "$sha" --arg tree "$base_tree" --argjson state "$state" --argjson now "$now" \
      '{ref:$ref,expected_parent:$parent,base_tree:$tree,create:false,state:($state + {status:"idle",finished_at:$now})}'
    ;;
esac

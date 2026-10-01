#!/usr/bin/env bash
# Produce reviewable remote lease transitions; publication belongs to the caller.
set -euo pipefail

die() { printf 'research-gpt state: %s\n' "$*" >&2; exit 1; }
root=$(git rev-parse --show-toplevel)
cd "$root"
ref=refs/heads/research/gpt-coordinator
action=${1:-status}
run=${2:-}
case "$action" in
  status) [ "$#" -le 1 ] || die 'usage: status' ;;
  claim|release) [ "$#" -eq 2 ] || die 'usage: claim|release RUN_ID' ;;
  verify) [ "$#" -eq 3 ] || die 'usage: verify RUN_ID CLAIM_SHA' ;;
  *) die 'expected status, claim, release, or verify' ;;
esac
if [ "$action" != status ]; then
  [[ "$run" =~ ^[a-z0-9][a-z0-9-]{7,79}$ ]] || die 'RUN_ID must be 8..80 lowercase letters, digits, or hyphens'
fi

# Do not treat network/authentication failures as an absent coordinator.
if remote=$(git ls-remote --exit-code --heads origin "$ref"); then
  sha=${remote%%[[:space:]]*}
  [[ "$sha" =~ ^[0-9a-f]{40}$ ]] || die 'invalid remote coordinator SHA'
  git fetch --quiet --no-tags origin "$sha"
  state=$(git show "$sha:state.json")
  printf '%s\n' "$state" | jq -se '
    length == 1 and (.[0] | type == "object" and .schema == 1 and
    (.status == "active" or .status == "idle") and
    (.run_id | type == "string" and test("^[a-z0-9][a-z0-9-]{7,79}$")) and
    (.base_sha | type == "string" and test("^[0-9a-f]{40}$")) and
    .branch == ("research/gpt-" + .run_id) and
    (.started_at | type == "number") and (.expires_at | type == "number") and
    .expires_at > .started_at and .expires_at - .started_at <= 7200)
  ' >/dev/null || die 'invalid coordinator state; inspect rather than overwrite it'
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
    jq -n --arg ref "$ref" --arg parent "$parent" --argjson create "$create" --arg run "$run" --arg base "$base" --argjson now "$now" \
      '{ref:$ref,expected_parent:$parent,create:$create,state:{schema:1,status:"active",run_id:$run,branch:("research/gpt-"+$run),base_sha:$base,started_at:$now,expires_at:($now+7200)}}'
    ;;
  verify)
    [ "$sha" = "$3" ] || die 'remote lease changed; stop this run'
    printf '%s' "$state" | jq -e --arg run "$run" --argjson now "$now" \
      '.status == "active" and .run_id == $run and .expires_at > $now' >/dev/null || die 'lease is not active, owned, and unexpired'
    printf '%s\n' "$state"
    ;;
  release)
    printf '%s' "$state" | jq -e --arg run "$run" '.status == "active" and .run_id == $run' >/dev/null || die 'cannot release another or inactive run'
    jq -n --arg ref "$ref" --arg parent "$sha" --argjson state "$state" --argjson now "$now" \
      '{ref:$ref,expected_parent:$parent,create:false,state:($state + {status:"idle",finished_at:$now})}'
    ;;
esac

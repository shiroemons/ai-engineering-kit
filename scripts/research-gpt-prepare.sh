#!/usr/bin/env bash
# The lease must already have been published and verified by the caller.
set -euo pipefail
[ "$#" -eq 3 ] || { echo 'usage: research-gpt-prepare.sh RUN_ID CLAIM_SHA NEW_DIRECTORY' >&2; exit 1; }
root=$(git rev-parse --show-toplevel)
cd "$root"
[ -z "$(git status --porcelain --untracked-files=all)" ] || { echo 'prepare requires a clean checkout' >&2; exit 1; }
state=$(bash scripts/research-gpt-state.sh verify "$1" "$2")
base=$(printf '%s' "$state" | jq -r .base_sha)
branch=$(printf '%s' "$state" | jq -r .branch)
git fetch --quiet --no-tags origin refs/heads/main:refs/remotes/origin/main
[ "$base" = "$(git rev-parse origin/main)" ] || { echo 'main advanced before preparation; release and claim again' >&2; exit 1; }
[ ! -e "$3" ] && [ ! -L "$3" ] || { echo 'destination already exists; preserve it and inspect' >&2; exit 1; }
git worktree add -b "$branch" -- "$3" "$base"
printf 'Prepared %s at %s from %s\n' "$branch" "$3" "$base"

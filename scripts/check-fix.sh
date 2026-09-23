#!/bin/sh
set -eu

# go fix は差分があっても成功するため、検証時は出力の有無も判定する。
if output=$(go fix -diff ./... 2>&1); then
    if [ -n "$output" ]; then
        printf '%s\nRun just fix to apply Go modernization updates.\n' "$output" >&2
        exit 1
    fi
else
    status=$?
    printf '%s\n' "$output" >&2
    exit "$status"
fi

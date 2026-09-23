#!/bin/sh
set -eu

unformatted=$(gofmt -l cmd internal modules)
if [ -n "$unformatted" ]; then
    printf 'Run gofmt on these files:\n%s\n' "$unformatted" >&2
    exit 1
fi

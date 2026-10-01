#!/bin/bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source "$ROOT/scripts/research-platform.sh"
research_require_macos research-status
exec launchctl print "gui/$(id -u)/com.shiroemons.ai-engineering-kit.research"

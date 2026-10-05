#!/usr/bin/env bash
set -euo pipefail
task_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
task_framework="$task_root/build/deps/Sparkle-2.10.0"
[[ -d "$task_framework/Sparkle.framework" ]] || {
  printf 'Run scripts/bootstrap-updaters.sh before this fixture.\n' >&2
  exit 1
}
task_output="$(mktemp -d)"
trap 'rm -rf "$task_output"' EXIT
xcrun clang -fobjc-arc -fblocks -Wall -Werror \
  -F"$task_framework" -framework Sparkle -framework Foundation -framework AppKit \
  -Wl,-rpath,"$task_framework" \
  "$task_root/internal/platform/darwin/updater_bridge.m" \
  "$task_root/native/darwin/updater_smoke.m" -o "$task_output/updater-smoke"
"$task_output/updater-smoke"

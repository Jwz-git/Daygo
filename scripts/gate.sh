#!/usr/bin/env bash

# Runs the per-commit gate from docs/08 §8.8.
#
# The gate has an ordering requirement the document does not spell out: the Go
# commands import internal/app, which imports frontend, whose go:embed needs
# frontend/dist to exist. On a clean checkout dist does not exist, so the
# bootstrap has to run before any Go command — otherwise `go build ./...` fails
# with "pattern all:dist: no matching files found".
#
# The same bootstrap also produces frontend/wailsjs, which vue-tsc needs.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

printf '== frontend dependencies ==\n'
if [[ -d "$ROOT_DIR/frontend/node_modules" ]]; then
  printf 'node_modules present; skipping install\n'
else
  # docs/08 §8.8 lists `npm ci`. It is a clean install from the lockfile, which
  # is what a fresh runner needs; repeating it on every local run would wipe and
  # reinstall node_modules for no benefit, so it runs only when absent.
  npm --prefix frontend ci
fi

printf '\n== bootstrap: generated frontend artifacts ==\n'
./scripts/bootstrap-frontend.sh

printf '\n== go ==\n'
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go test ./internal/...
go vet ./...
gofmt -l .
printf 'gofmt: clean\n'

printf '\n== frontend ==\n'
npm --prefix frontend run test:unit
npm --prefix frontend run typecheck
npm --prefix frontend run build

printf '\n== docs ==\n'
# Link and anchor rot in docs/ is mechanical, so it is checked mechanically.
# Skipped rather than failed when python3 is absent: the documentation check
# must not be the reason a Go/frontend gate cannot run on a machine.
if command -v python3 >/dev/null 2>&1; then
  python3 scripts/check-docs.py
else
  printf 'python3 not found; skipping docs check\n'
fi

printf '\ngate: passed\n'

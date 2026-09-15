# scripts/

Every entry point that is not part of a normal `go test` / `npm test` run lives
here. The split is by **role**, not by language — three platform dev scripts
(`dev.sh`, `dev.ps1`, `dev-linux.sh`), Windows / Linux production build scripts,
one headless CI gate (`gate.sh`),
shared bootstrap and shell helpers (`bootstrap-frontend.sh`), a docs sanity
check (`check-docs.py`), and a `probe/` directory for one-off diagnostic
tools. The README documents the contract for each so it stays obvious which
script owns a responsibility and which one a new contributor should reach for.

| Script | Role | Caller |
|---|---|---|
| `bootstrap-frontend.sh` | Standalone: placeholder dist → bindings → real bundle. Sourced: exports `require_tool`, `webkit_tag`, `run_wails`, `daygo_done_sourcing` for the other scripts. | All `dev*` scripts and `gate.sh` |
| `dev.sh` | macOS `wails dev` entry; sources bootstrap for shared helpers, keeps the macOS-only `clang` check inline. | Local development on macOS |
| `dev.ps1` | Windows `wails dev` entry; applies the Go 1.25 cgo debug workaround only when needed. | Local development on Windows |
| `windows-common.ps1` | Shared Windows tool checks, frontend bootstrap and Go 1.25 DWARF workaround. | `dev.ps1`, `build.ps1` |
| `build.ps1` | Reproducible `windows/amd64` build; verifies both EXE and helper DLL. `-RunSmoke` additionally runs native smoke tests. | Windows production packaging |
| `dev-linux.sh` | Linux `wails dev` entry; sources bootstrap for `webkit_tag` and the wails invocation. | Local development on Linux |
| `build-linux.sh` | Linux `wails build` entry; identical tag handling to `dev-linux.sh`, replaces `npm install` with `npm ci` because production builds run from a clean clone. | Linux production packaging |
| `gate.sh` | Headless commit gate: bootstrap + `go build / test / vet / gofmt` + frontend `typecheck / unit / build` + `check-docs.py`. Skipped only when `python3` is missing (Python is for docs only). | CI runner, also local pre-commit |
| `check-docs.py` | Markdown link + anchor + orphan-document check. Standard library only so it runs on any host. | `gate.sh`, manual |
| `probe/analysis.go` | Provider-agnostic diagnostic: runs the production transcription + card-generation pipeline against the user-configured provider, never writes the database. | Manual, when debugging AI integration |

## Shared helpers — where logic lives

Both `bootstrap-frontend.sh` and the three dev scripts used to duplicate
host checks, Wails tag selection, and the `go run wails … dev/build`
invocation. After the 2026-09-14 cleanup:

- **`scripts/bootstrap-frontend.sh`** owns `require_tool`, `webkit_tag`,
  `run_wails`. Sourcing it gives a script those three helpers and the
  `ROOT_DIR` / `WAILS_VERSION` variables without running the placeholder-dist
  pipeline.
- **`scripts/dev.sh`** keeps the macOS-only `clang` check inline. That check
  is the only piece that is genuinely platform-specific (clang is required
  on macOS for the cgo build, not on Linux/Windows).
- **`scripts/windows-common.ps1`** is dot-sourced by the two Windows entry
  points and owns their shared tool checks, frontend bootstrap and Go 1.25
  debug-linker workaround. `dev.ps1` uses `npm install`; `build.ps1` uses the
  lockfile-strict `npm ci` production path.
- **`scripts/dev-linux.sh`** and **`scripts/build-linux.sh`** are both thin
  wrappers — the only difference between them is whether `run_wails` is
  called with `dev` or `build`, and whether `npm install` (dev) or
  `npm ci` (production) runs.

Adding a new helper (e.g. a future `darwin_sdk_path` resolver) belongs in
`bootstrap-frontend.sh` so the three dev scripts pick it up automatically.

## Naming

- `probe/` collects one-off diagnostic tools. Add a new diagnostic as
  `scripts/probe/<topic>.go` or `scripts/probe/<topic>.sh`; do not put
  diagnostic scripts at the `scripts/` root.
- `dev-*` means "run `wails dev`". `build-*` means "run `wails build`".
  Anything that does neither is misfiled.
- `*.sh` is bash, `*.ps1` is PowerShell, `*.py` is Python. The single
  `.py` script (`check-docs.py`) is the only Python in the repo and is
  intentionally stdlib-only so it runs anywhere.

## Conventions

- All shell scripts use `set -euo pipefail`. The PowerShell script uses
  `Set-StrictMode -Version Latest` and `$ErrorActionPreference = 'Stop'`.
- Every script resolves its anchor with `cd "$(dirname "$0")/.."` (bash)
  or `(Resolve-Path (Join-Path $PSScriptRoot '..')).Path` (PowerShell),
  so they work regardless of the caller's CWD.
- `bootstrap-frontend.sh` is the only place where `WAILS_VERSION` lives.
  Bumping the Wails version means one edit, not five.
- `webkit_tag` honours `DAYGO_LINUX_WEBKIT_TAG` for the rare hosts that
  only ship one WebKit2GTK ABI; pkg-config detection is the default.

## When to add a new script

- The action runs more than one external binary in a fixed order.
- The action is repeated by more than one caller (CI, docs, humans).
- The action needs an environment check or environment mutation that the
  rest of the repo should not have to know about.

Anything that fits in `go run ./cmd/foo` or `npm run foo` does not belong
here — the project's own build/test runners are the right home for it.

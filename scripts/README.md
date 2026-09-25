# scripts/

Every entry point that is not part of a normal `go test` / `npm test` run lives
here. The split is by **role**, not by language — three platform dev scripts
(`dev.sh`, `dev.ps1`, `dev-linux.sh`), Windows / Linux production build scripts,
platform packagers (`package-macos.sh` → signed DMG, `package-windows.ps1` →
signable NSIS installer), one headless CI gate (`gate.sh`),
shared bootstrap and shell helpers (`bootstrap-frontend.sh`), a docs sanity
check (`check-docs.py`), and a `probe/` directory for one-off diagnostic
tools. The README documents the contract for each so it stays obvious which
script owns a responsibility and which one a new contributor should reach for.

| Script | Role | Caller |
|---|---|---|
| `bootstrap-frontend.sh` | Standalone: placeholder dist → bindings → real bundle. Sourced: exports `require_tool`, `webkit_tag`, `run_wails`, `daygo_bootstrap` for the other scripts. | All `dev*` / `build-*` scripts, `gate.sh`, `package-macos.sh` |
| `dev.sh` | macOS `wails dev` entry; sources bootstrap for shared helpers, keeps the macOS-only `clang` check inline, and recreates the generated `.app` so Dock does not retain a stale application icon. | Local development on macOS |
| `dev.ps1` | Windows `wails dev` entry; applies the Go 1.25 cgo debug workaround only when needed. | Local development on Windows |
| `windows-common.ps1` | Shared Windows tool checks, frontend bootstrap, PE import guard against unbundled MinGW runtimes, and Go 1.25 DWARF workaround. | `dev.ps1`, `build.ps1`, `package-windows.ps1` |
| `build.ps1` | Reproducible `windows/amd64` build; verifies both EXE and helper DLL. `-RunSmoke` additionally runs native smoke tests. | Windows production packaging |
| `dev-linux.sh` | Linux `wails dev` entry; sources bootstrap for `webkit_tag` and the wails invocation. | Local development on Linux |
| `build-linux.sh` | Linux `wails build` entry; identical tag handling to `dev-linux.sh`, replaces `npm install` with `npm ci` because production builds run from a clean clone. | Linux production packaging |
| `package-macos.sh` | macOS packager: bootstrap → `wails build` → `Info.plist` (min-OS / version) → `codesign` → `create-dmg` → optional notarize + staple. Ad-hoc signs by default; Developer ID + notarization via `DAYGO_SIGN_IDENTITY` / `DAYGO_NOTARY_PROFILE`. | macOS release packaging |
| `package-windows.ps1` | Windows packager: bootstrap → Wails/NSIS materialisation → sign EXE + native DLL → repackage those final bytes → sign installer → emit SHA-256 acceptance manifest. The tracked `windows-installer/project.nsi` is required because stock Wails only installs the EXE. Supports `-InstallScope machine|user`; unsigned by default. Certificate file via `DAYGO_WIN_CERT_FILE` / `DAYGO_WIN_CERT_PASSWORD`, or installed cert via `DAYGO_WIN_CERT_THUMBPRINT`. Windows-only; host run still required (see delivery module). | Windows release packaging |
| `bootstrap-updaters.sh` | Downloads Sparkle 2.10.0 into ignored `build/deps`, verifies the pinned SHA-256, and exposes the framework plus signing tools. | macOS release build / appcast job |
| `generate-appcast.py` | Builds the two-platform RSS appcast from already Ed25519-signed macOS and Windows assets. It never receives the private key. | protected GitHub `release` environment |
| `windows-installer/project.nsi` | Wails-compatible NSIS project that installs `Daygo.exe` and the required `daygo_windows_native.dll` together. Copied into ignored `build/windows/installer/` at package time. | `package-windows.ps1` |
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
  on macOS for the cgo build, not on Linux/Windows). Before starting Wails it
  also recreates the generated `build/bin/Daygo.app`; Wails otherwise updates
  the bundle contents in place and macOS may continue displaying a cached
  icon after `build/appicon.png` changes.
- **`scripts/windows-common.ps1`** is dot-sourced by the Windows entry points
  and owns their shared tool checks, native-before-bindings bootstrap, frontend
  bootstrap and Go 1.25 debug-linker workaround. `dev.ps1` uses `npm install`;
  `build.ps1` and the packager use the lockfile-strict `npm ci` production path.
- **`scripts/dev-linux.sh`** and **`scripts/build-linux.sh`** are both thin
  wrappers — the only difference between them is whether `run_wails` is
  called with `dev` or `build`, and whether `npm install` (dev) or
  `npm ci` (production) runs.

Adding a new helper (e.g. a future `darwin_sdk_path` resolver) belongs in
`bootstrap-frontend.sh` so the three dev scripts pick it up automatically.

## Windows acceptance packaging

Run the release-candidate path only on Windows with MinGW-w64, Visual Studio 2022 C++ tools,
Windows SDK 26100 and NSIS available. `-RunSmoke` is required for an acceptance record:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows.ps1 `
  -Version 0.1.0 -InstallScope machine -RunSmoke
```

The packager deliberately performs two NSIS passes. The first materialises Wails' generated
`wails_tools.nsh` and WebView2 bootstrapper. It then signs `Daygo.exe` and
`daygo_windows_native.dll`, rebuilds the installer from those final bytes using the tracked
project file, and signs the installer. A one-pass `wails build -nsis` is not an equivalent
release path: Wails' stock template omits the native DLL, and signing after packaging would
leave the embedded executable unsigned. The resulting `dist/windows-package.json` is evidence
for artifact identity only; the WD matrix still requires installed-file signature checks,
interactive and silent install/uninstall, upgrade, and clean-machine startup.

## Naming

- `probe/` collects one-off diagnostic tools. Add a new diagnostic as
  `scripts/probe/<topic>.go` or `scripts/probe/<topic>.sh`; do not put
  diagnostic scripts at the `scripts/` root.
- `dev-*` means "run `wails dev`". `build-*` means "run `wails build`".
  `package-*` means "build a signed, distributable artifact" (more than a
  bare `wails build`: signing, notarization, DMG/installer packaging). Its
  extension follows the host it runs on — `package-macos.sh` is bash,
  `package-windows.ps1` is PowerShell, because the signing and installer tools
  are host-native.
  Anything that does none of these is misfiled.
- `*.sh` is bash, `*.ps1` is PowerShell, `*.py` is Python. Python scripts are
  intentionally stdlib-only so they run anywhere.

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

## Protected release environment

`.github/workflows/publish-release.yml` runs only after a GitHub Release is published and uses the protected
`release` environment. Configure these environment secrets before the first formal release:

- `SPARKLE_ED25519_PRIVATE_KEY` — already generated; also retained in the local macOS keychain.
- `DAYGO_MAC_TEAM_ID` / `DAYGO_WIN_SIGNER_THUMBPRINT` — protected `release` environment values that pin the macOS Developer ID team and Windows Authenticode certificate for formal appcast generation. Missing or mismatched values stop the appcast job; the verifier also checks notarization, installed binaries and asset hashes.
- `MACOS_CERTIFICATE_P12_BASE64`, `MACOS_CERT_PASSWORD`, `APPLE_API_KEY_P8_BASE64`,
  `APPLE_API_KEY_ID`, `APPLE_API_ISSUER_ID` — Developer ID signing and notarization.
- `WINDOWS_CERTIFICATE_PFX_BASE64`, `WINDOWS_CERT_PASSWORD` — Authenticode signing.

For a published `vX.Y.Z` Release, the workflow uploads both platform assets. For a formal release only, it then signs
their final bytes with Sparkle Ed25519 and uploads `appcast.xml` to that same Release. A prerelease skips the appcast
job. A missing signing key or installer leaves a formal release's appcast absent. The `releases/latest` URL can return
404 between publishing a formal Release and this final upload. After promoting a prerelease to formal, manually
dispatch this workflow with its tag to generate the appcast; verify the asset exists before treating promotion as ready.
The `release` environment retains the signing secret but has no required reviewer; the job runs automatically.
For an existing published Release whose original event was skipped or missed, use the workflow's manual dispatch with
its `tag` input. The prepare job checks the live Release state before building.

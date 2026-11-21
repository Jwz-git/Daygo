# scripts/

Entry points outside standard `go test` / `npm test` runs live here, organized by **role** rather than language: platform dev launchers, desktop builders, release packagers, headless commit gates, shared bootstrap helpers, and diagnostic/regression tools.

> Daygo runs on **Wails v3.1.0** with **Go 1.26.10**. Root [`Taskfile.yml`](../Taskfile.yml) and [`build/config.yml`](../build/config.yml) coordinate `wails3 dev` / task dispatch, while native libraries are built explicitly by the scripts below (no v2 `preBuildHooks`, and v2 flags like `-platform`, `-nsis`, `-s` are not forwarded). See the [Wails v3 GA migration record](../docs/decisions/wails-v3-migration.md).

## Script inventory

### Bootstrap & shared helpers

| Script | Purpose | Typical caller |
|---|---|---|
| `bootstrap-frontend.sh` | Standalone: placeholder `dist` → Wails TS bindings (`-ts -i -names` with `-tags=daygo_desktop,server`) → frontend build. Sourced: exports `ROOT_DIR`, `WAILS_VERSION`, `require_tool`, `webkit_tag`, `run_wails`, `daygo_bootstrap`. | `dev*.sh`, `build-*.sh`, `gate.sh`, `package-macos.sh` |
| `windows-common.ps1` | Shared Windows tool checks, native-before-bindings bootstrap, `Build-DaygoWindowsDesktop`, PE import guard against unbundled MinGW DLLs, and Go 1.25 DWARF-5 debug workaround. | `dev.ps1`, `build.ps1`, `package-windows.ps1` |
| `bootstrap-updaters.sh` | Downloads Sparkle 2.10.0 into ignored `build/deps`, verifies pinned SHA-256, and exposes the framework and signing tools. | macOS release build / appcast job |

### Local development & desktop builds

| Script | Purpose | Typical caller |
|---|---|---|
| `dev.sh` | macOS `wails3 dev` launcher; checks `clang`, recreates `build/bin/Daygo.app` (preventing stale Dock icon cache), then runs Wails dev. | Local macOS development |
| `dev.ps1` | Windows `wails3 dev` launcher; prepares native artifacts + `npm install` and applies the DWARF-5 debug workaround when needed. | Local Windows development |
| `dev-linux.sh` | Linux `wails3 dev` launcher; detects GTK4/WebKitGTK 6.0 (or legacy `gtk3`) via `webkit_tag`. | Local Linux development |
| `build-desktop.sh` | Compiles the macOS / Linux desktop host with `daygo_desktop` (plus `production` and `-ldflags '-s -w'` unless `DAYGO_DEV=1`) after building native prerequisites. | `Taskfile.yml`, `package-macos.sh` |
| `build.ps1` | Reproducible `windows/amd64` build (`npm ci`); verifies `Daygo.exe` and `daygo_windows_native.dll`. Pass `-RunSmoke` to run native smoke tests. | `Taskfile.yml`, Windows packaging |
| `build-linux.sh` | Production Linux `wails3 build` wrapper using `npm ci` and `webkit_tag`. | Linux production builds |

### Packaging & release signing

| Script | Purpose | Typical caller |
|---|---|---|
| `package-macos.sh` | macOS packager: bootstrap → `build-desktop.sh` → `Info.plist` metadata → `codesign` → `create-dmg` → optional notarize + staple. Defaults to ad-hoc locally; supports `DAYGO_DEV_SIGN_IDENTITY` (stable self-signed) or `DAYGO_SIGN_IDENTITY` / `DAYGO_NOTARY_PROFILE` (Developer ID). | macOS release packaging |
| `dev-cert-macos.sh` | Creates a reusable local self-signed code-signing certificate with optional password-protected PKCS#12 export outside the repo. Does not notarize or publish. | Local TCC-stable signing / CI cert setup |
| `package-windows.ps1` | Windows packager: bootstrap → build EXE + DLLs → sign payloads → compile NSIS (`windows-installer/project.nsi`) → sign installer → write `dist/windows-package.json` SHA-256 manifest. Supports `-InstallScope machine\|user` and `-RunSmoke`. | Windows release packaging |
| `windows-installer/project.nsi` | Modern UI 2 NSIS installer (9 languages, optional desktop shortcut, WebView2 & in-use-file guards). Installs `Daygo.exe`, `daygo_windows_native.dll`, and `WinSparkle.dll`; uninstall preserves user data. | `package-windows.ps1` |
| `generate-appcast.py` | Generates dual-platform RSS `appcast.xml` from pre-signed macOS/Windows assets without touching the Ed25519 private key. | Protected GitHub `release` job |

### Gates, fixtures & diagnostics

| Script | Purpose | Typical caller |
|---|---|---|
| `gate.sh` | Headless pre-commit gate: checks/installs frontend deps → bootstrap → `CGO_ENABLED=0` Go build/test/vet/`gofmt` → 3-OS core cross-build → frontend unit/typecheck/build → Python doc/entrypoint/installer checks. | Local pre-commit & CI |
| `check-docs.py` | Stdlib-only checker for broken relative Markdown links, missing heading anchors, and unlinked docs under `docs/`. | `gate.sh`, manual |
| `test-entrypoints.py` | Runs macOS/Linux dev & build scripts in temporary checkouts with tool doubles to verify bootstrap and Wails CLI dispatch without GUI or real process mutation. | `gate.sh`, manual |
| `test-build-flags.py` / `test-build-flags.ps1` | Verifies dev vs production linker flags (`-s -w`, `-H windowsgui`) using anonymous temp fixtures without compiling an app. | Manual / build-flag regression |
| `windows-installer/test_installer.py` | Anonymous NSIS fixtures: 9-language coverage, machine/user `makensis -WX` compiles, missing-DLL rejection, plus Windows-only silent install/uninstall, WebView2 failure, and locked-file checks. | `gate.sh`, Windows installer workflow |
| [`native/darwin/updater-smoke.sh`](../native/darwin/updater-smoke.sh) | Compiles the production Sparkle delegate and exercises anonymous discovery/download/late-finalize/cancel/failure callbacks (requires bootstrapped Sparkle framework). | Updater changes (`bash`); [workflow](../.github/workflows/macos-updater-check.yml) |
| `sample-memory.py` | Read-only macOS fixed-PID RSS / physical-footprint CSV sampler; verifies process identity and collects no user activity content. | Manual memory diagnostics |

> **Gate caveats**: `gofmt -l .` inside `gate.sh` prints unformatted files without failing the exit code — always confirm it emits no file paths. When `python3` is missing, `gate.sh` skips the Python checks; never report skipped checks as passed. Historical acceptance scope and evidence boundaries are tracked in [09 §9.1.1](../docs/09-roadmap.md#911-本轮验收记录与证据边界).

---

## Shared helpers and orchestration

- **`bootstrap-frontend.sh`** is the single source of truth for `WAILS_VERSION` (`v3.1.0`), `require_tool`, `webkit_tag`, and `run_wails`. Note that binding generation uses `-tags=daygo_desktop,server` purely for Go type analysis without requiring native SDKs — Daygo does not ship a server-mode product.
- **`webkit_tag`** auto-detects GTK4 + WebKitGTK 6.0 via `pkg-config` (falling back to GTK3 + WebKit2GTK 4.1) and honors `DAYGO_LINUX_WEBKIT_TAG=gtk3` for legacy hosts.
- **`windows-common.ps1`** centralizes Windows tool discovery, native-before-bindings ordering, `npm install` (dev) vs `npm ci` (production), and PE runtime import checks.

---

## Windows build dependencies

Windows entry points target `windows/amd64`. Inventory verified against [`windows-common.ps1`](windows-common.ps1), [`native/windows/build.ps1`](../native/windows/build.ps1), [`build.ps1`](build.ps1), and [`package-windows.ps1`](package-windows.ps1):

| Dependency | Role and setup |
|---|---|
| Git for Windows | Source checkout and revision metadata (`git` on PATH). Git Bash is needed to run `gate.sh`, not the PowerShell build scripts. |
| Go | Windows `amd64` toolchain matching [`go.mod`](../go.mod). Desktop builds require cgo; core gate uses `CGO_ENABLED=0`. |
| Node.js & npm | Node 20.19+ (20.x) or 22.12+ (release CI uses Node 22) with npm and the existing `package-lock.json`. |
| MinGW-w64 x64 (GCC & binutils) | `gcc`, `g++`, `ar`, and `objdump` from the same x64 toolchain on PATH. Used by Go cgo, `libdaygo_capture.a`, and PE DLL import checks (CI installs Chocolatey `mingw`). |
| Visual Studio 2022 C++ tools | **Desktop development with C++** workload (Build Tools suffices), including `Microsoft.VisualStudio.Component.VC.Tools.x86.x64`, to compile C++20/C++/WinRT `daygo_windows_native.dll`. Discovered via `vswhere.exe` + `vcvars64.bat`. |
| Windows SDK 10.0.26100 | Installed via Visual Studio Installer (headers & libs). The build looks for `windows.ui.interop.h` under `Windows Kits\10\Include`. |
| Windows PowerShell | Entry points invoke `powershell -NoProfile -ExecutionPolicy Bypass` (PowerShell 7 alone does not substitute). |
| WebView2 Runtime | Required at runtime for `dev.ps1` and desktop UI execution (separate from build compilers). |
| NSIS (`makensis`) | Required only for `package-windows.ps1` and installer fixture compilation. |
| Windows SDK `signtool` | Required only when signing is enabled (`bin\<version>\x64` on PATH + cert env vars). |
| Python 3 | Required only for doc / installer fixture checks (invoke `python` on Windows). |

Both MinGW-w64 and MSVC/Windows SDK are required for a full Windows build: if SDK headers are missing, `native/windows/build.ps1` warns and emits only the baseline archive, which `scripts/build.ps1` then rejects for lacking `daygo_windows_native.dll`. Runtime OS requirements are documented in the [project README](../README.md#下载与安装).

No global Wails installation is needed — scripts run `go run github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`. The native build also fetches and SHA-256-verifies WinSparkle 0.9.4 and places x64 `WinSparkle.dll` in `build/bin`. Always keep `Daygo.exe`, `daygo_windows_native.dll`, and `WinSparkle.dll` together.

Verify discovery from a fresh PowerShell session at the repo root before building:

```powershell
Get-Command git, go, node, npm, gcc, g++, ar, objdump, powershell
go version
node --version
npm --version
gcc -dumpmachine  # expect an x86_64 Windows target, not i686
$vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'
& $vswhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\Include" -Filter windows.ui.interop.h -Recurse
```

Start development with `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev.ps1` or produce an EXE with `scripts/build.ps1` — do not begin a clean checkout with bare `go build ./...` or `npm run build`.

---

## Windows acceptance packaging

Run release-candidate packaging on a Windows host with MinGW-w64, Visual Studio 2022 C++ tools, Windows SDK 26100, and NSIS (`-RunSmoke` required for acceptance records):

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-windows.ps1 `
  -Version 0.1.0 -InstallScope machine -RunSmoke
```

- **Signing**: Optional locally (`DAYGO_WIN_CERT_FILE` / `DAYGO_WIN_CERT_PASSWORD` or `DAYGO_WIN_CERT_THUMBPRINT`). When configured, `Daygo.exe`, `daygo_windows_native.dll`, and `WinSparkle.dll` are signed before NSIS packaging, then the installer is signed and `dist/windows-package.json` records SHA-256 digests.
- **Installer behavior**: Follows Windows UI language across the 9 supported locales (`windows-installer/languages.nsh`, fallback English). Desktop shortcuts are opt-in on new installs and preserved on upgrade; `/S` creates only the Start menu shortcut. Machine-scope installs never launch Daygo from an elevated token.
- **Silent exit codes & `/DAYGO_UPDATE`**: `10` (required files in use/inaccessible), `20` (WebView2 unavailable after bootstrap), `30` (missing update target), `64`/`65` (Wails OS/arch checks). Setup never force-kills the resident agent; `/DAYGO_UPDATE` waits up to 30 seconds for EXE/DLL release and resolves existing install location (via trailing `/D=` or registry `InstallLocation` / legacy `DisplayIcon`). Launch/UAC failures restore previously active recording.
- **Fixture checks without a full release build**:

```bash
python3 scripts/windows-installer/test_installer.py   # use `python` on Windows
```

---

## Release appcast signing

[`.github/workflows/publish-release.yml`](../.github/workflows/publish-release.yml) builds any missing installers for a published GitHub Release. For formal releases (prereleases skipped), the appcast job in the protected `release` environment signs the final macOS and Windows assets with `SPARKLE_ED25519_PRIVATE_KEY` and uploads `appcast.xml`; missing keys or assets fail the job without uploading.

- `SPARKLE_ED25519_PRIVATE_KEY` authenticates update archives for Sparkle / WinSparkle only — it does **not** grant Apple Developer ID / Gatekeeper trust or Windows Authenticode / SmartScreen reputation.
- When promoting a prerelease or recovering a missed release event, manually dispatch `publish-release.yml` with the `tag` input and confirm `appcast.xml` is uploaded (existing installers are reused).

---

## Naming and conventions

- **Prefixes**: `dev-*` runs `wails3 dev`; `build-*` builds a desktop binary; `package-*` produces signed/distributable installers (`*.sh` on Unix/macOS, `*.ps1` on Windows).
- **Strict mode & CWD independence**: Bash scripts use `set -euo pipefail` and anchor at `cd "$(dirname "$0")/.."`; PowerShell scripts use `Set-StrictMode -Version Latest`, `$ErrorActionPreference = 'Stop'`, and `$PSScriptRoot`. Python scripts stay stdlib-only.
- **Placement**: Keep one-off scripts in `.tmp/` or system temp (never commit them). Reusable diagnostics belong in `scripts/probe/<topic>.{go,sh}`, not at `scripts/` root. Add a script only when orchestrating multiple tools in a fixed order, shared across callers, or encapsulating host checks.

---

## Retired diagnostics

- **2026-10-07**: Removed legacy `probe/analysis.go` and `internal/analysis/probe.go` (read deprecated staging files directly, rejected nonzero frame indices, and opened the DB read-write via `storage.Open`). Also removed machine-specific `internal/analysis/local_probe_test.go` (hardcoded Windows `APPDATA` path and local endpoint; historical observations remain in [providers verification records](../docs/modules/providers.md#验证记录)).
- **2026-10-13**: Audited all tracked scripts against CI, Wails hooks, npm scripts, and documented manual checks. Removed untracked/ignored local snippets (`build/diagnostics/readme-locales/render.py` and `/tmp/daygo-*` probes); no tracked entry point was removed.

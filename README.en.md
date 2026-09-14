# Daygo

[简体中文](README.md) · **English**

Daygo is a private, local-first work journal for macOS. It captures the current primary display at a fixed interval, uses an AI provider chosen by the user to understand that activity, and turns it into a searchable daily timeline, standup summary, and review.

> **Project status: under development, not ready for general installation.** What exists today:
> the desktop shell and settings pages, the SQLite foundation (migrations, instance locks,
> settings, backups, diagnostics), three protocol AI clients with a connection probe, the
> platform ports with a Capture fake, a macOS single-shot capture implementation, and ten
> Wails bindings. The recording loop, timeline, and daily/weekly reviews are not implemented.
> Per-module status lives in [docs/09-roadmap.md §9.1](docs/09-roadmap.md#91-模块总表).

## Why Daygo

Conventional time trackers usually know which application was active. Daygo aims to preserve the context of the work itself: what was being built, investigated, discussed, or reviewed.

- Automatic activity timeline without manually starting timers
- Daily summaries and standup preparation
- Weekly review and distraction analysis
- Natural-language questions grounded in your work history
- Local-first storage and configurable retention
- Local or cloud AI providers selected by the user

## Privacy model

Privacy is an architectural constraint, not an optional mode:

- Recordings, timeline data, and the database remain on the Mac by default.
- Screen data may leave the device only when it is sent to an AI provider explicitly configured by the user.
- Local models can be used to keep analysis on-device.
- Blocked applications are excluded from capture, with a redacted placeholder used when necessary.
- Analytics and crash reporting are opt-in and must never contain screen content, window titles, file paths, credentials, or LLM payloads.

Everything lives in one directory:

```text
~/Library/Application Support/Daygo/
```

The database, the recordings, and the backups are all there; API keys live in the system keychain. There is no second copy — uninstalling deletes everything.

## Architecture

```text
Vue 3 + TypeScript
        ↓ Wails bindings
Go core
  ├── storage and settings
  ├── analysis and AI providers
  ├── timeline, daily, and weekly insights
  └── lifecycle orchestration
        ↓ internal/platform ports
platform adapter (form undecided; screen capture has darwin / windows implementations)
  └── screen capture, system permissions, keychain, status item, updates
```

Go owns all portable business logic and is the single SQLite writer. The parts that need macOS system capabilities sit behind a small set of ports whose **implementation is not yet chosen** — the docs define only the contract any implementation must satisfy. That boundary is what lets the Go core build and test without macOS (`CGO_ENABLED=0`), which the whole testing strategy depends on.

See the [design docs](docs/README.md) for requirements, interfaces, data model, testing strategy, feature modules, and risks.

## Current repository layout

```text
cmd/daygo/                  Go command entry point and wails.json
internal/
  app/                      Wails bindings, DTOs, error codes, events
  storage/                  the single SQLite writer: connection, migrations, locks,
                            maintenance, diagnostics
  settings/                 typed access over app_settings
  ai/                       three protocol clients, retry/fallback, structured output,
                            connection probe
  platform/                 ports + fake + contract suites + darwin / windows adapters
                            (Linux uses `//go:build !darwin && !windows` placeholders
                            that return `unsupported` rather than failing the build)
  timeutil/                 the 4 a.m. logical day
native/                     native capture implementations (one shared C ABI)
  include/daygo_capture.h   ABI v1
  darwin/                   Swift + ScreenCaptureKit
  windows/                  C++ + DXGI (experimental, unverified)
frontend/                   Vue 3 + TypeScript frontend
scripts/                    bootstrap, commit gate, and dev scripts
build/                      Wails build assets and output
docs/                       design documentation
```

Most paths described under `docs/` are still target state: the analysis pipeline, timeline,
daily/weekly views, the recorder, and the background lifecycle are not implemented.
See [docs/09-roadmap.md](docs/09-roadmap.md) for the plan.

**About Linux:** the Wails v2 desktop shell (GTK3 + WebKit2GTK) already launches and loads the
Vue frontend on Linux, and the Go core plus SQLite layer behave identically to macOS. Capabilities
that need native code (screen capture, system permissions, status item, keychain) follow the
`internal/platform` convention of returning `unsupported` when no adapter is implemented. A real
Linux adapter belongs to the undecided designs in [docs/09 §9.8](docs/09-roadmap.md#98-待定设计清单),
and will not be implemented at scale before a decision record exists.

**About Windows:** the tree contains an experimental Windows capture implementation. It has
never been verified on real hardware, it is not in release scope, and Windows has no instance
lock implementation yet (so it runs without a database). macOS remains the only target
platform — see the [decision record](docs/decisions/recording-screen-capture-windows.md).

## Build and run

Requirements:

- **macOS (primary):** macOS 14+, Go 1.25+, Node.js 20.19+ (or 22.12+), npm, Xcode Command Line Tools
- **Windows (experimental):** Windows 10/11, Go 1.25+, Node.js 20.19+, npm, optional MinGW-w64
- **Linux (early adapter):** any modern distribution, Go 1.25+, Node.js 20.19+, npm, GTK3 and WebKit2GTK development headers (see below)

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo

# macOS: run in development (installs dependencies and bootstraps generated artifacts)
./scripts/dev.sh

# Windows: run in development (PowerShell)
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev.ps1

# Linux: run in development (selects the WebKit2GTK build tag automatically)
./scripts/dev-linux.sh

# Commit gate: bootstrap + Go build/test/vet/gofmt + frontend typecheck/build
./scripts/gate.sh
```

**A fresh clone must be bootstrapped before any `go build` or `npm run build`.** The two
generated directories depend on each other: `frontend/dist` is referenced by `go:embed all:dist`
(without it the Go module does not compile), and `frontend/wailsjs` is imported by the frontend
sources (without it `vue-tsc` fails) — yet generating the bindings requires a compilable Go tree.
`scripts/bootstrap-frontend.sh` breaks the cycle with "placeholder dist → bindings → real bundle",
and `dev.sh` / `dev.ps1` / `dev-linux.sh` and `gate.sh` all call it.

Go 1.25's Windows+cgo debug builds suffer from a linker bug; `dev.ps1` temporarily sets
`GOEXPERIMENT=nodwarf5` before invoking Wails so the resulting PE is acceptable to the Windows
loader, and restores the original environment on exit.

### Linux extras

Wails v2 on Linux needs GTK3 + WebKit2GTK. The ABI is auto-detected from `pkg-config`:

```bash
# Debian / Ubuntu 22.04+
sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev

# Fedora 40+
sudo dnf install gcc-c++ pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel

# Arch
sudo pacman -S --needed base-devel pkgconf gtk3 webkit2gtk-4.1
```

On older distributions (Debian 11, Ubuntu 20.04, RHEL/CentOS 8-9) that only ship ABI 4.0, install
`libwebkit2gtk-4.0-dev` and pass the tag explicitly:

```bash
DAYGO_LINUX_WEBKIT_TAG=webkit2_40 ./scripts/dev-linux.sh
```

Packaging:

```bash
cd cmd/daygo

# macOS
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -platform darwin/arm64

# Windows
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -platform windows/amd64

# Linux (scripts/build-linux.sh picks the WebKit2GTK ABI tag automatically)
../scripts/build-linux.sh
```

The outputs are `build/bin/Daygo.app`, `build/bin/Daygo.exe`, and `build/bin/Daygo` respectively.
`wails` must run from `cmd/daygo`; it resolves `frontend/` and `build/` at the repo root and builds
the native static library through `preBuildHooks` — currently only `darwin/*` and `windows/*` are
hooked. The Linux native adapter is one of the undecided designs in docs/09 §9.8 (#1).

### Feature differences across platforms

The `internal/platform` port layer is designed to return `unsupported` on Linux and real
implementations on darwin/windows, so the Wails shell launches and renders the Vue frontend on all
three. The native capabilities still differ substantially:

| Capability                | macOS | Windows | Linux (today) |
|---------------------------|:-----:|:-------:|:------:|
| Screen capture            | ✅ ScreenCaptureKit | ⚠️ DXGI / WGC (experimental, limited smoke) | ❌ not implemented (`CaptureUnsupported`) |
| System permission / TCC   | ✅ | ⚠️ partial | ❌ not implemented |
| Status item / tray        | ✅ | ⚠️ partial | ❌ not implemented |
| Keychain / credentials    | ✅ `security` subprocess | ✅ Credential Manager | ❌ `SecretUnsupported` |
| Launch at login / activation policy | ✅ | ⚠️ partial | ❌ not implemented |
| System event subscription | ✅ | ⚠️ partial | ❌ not implemented |
| SQLite + settings + timeline UI | ✅ | ✅ | ✅ (identical to macOS) |

**macOS remains the target platform.** Completing the Linux and Windows feature matrices belongs to
docs/09 §9.8 and requires its own decision record before implementation. Until then, what already
works on Linux and Windows is everything that does not need a platform adapter: viewing existing
data, settings, AI provider configuration and connection probing, and the chat surface — all of
which live in the Go core.

The release pipeline (signing, notarization, auto-update) does not exist yet.

## Contributing

Work follows the [feature module roadmap](docs/09-roadmap.md) and each module's execution guide. Modules may develop concurrently and integrate when their specific capabilities are ready. Work must respect the constraints in [docs/07-privacy-security.md](docs/07-privacy-security.md). Read [AGENTS.md](AGENTS.md) before making implementation changes.

For substantial changes, open an issue first and state which feature module, shared capabilities, and verification gates the work addresses.

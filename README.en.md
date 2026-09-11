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

**About Windows:** the tree contains an experimental Windows capture implementation. It has
never been verified on real hardware, it is not in release scope, and Windows has no instance
lock implementation yet (so it runs without a database). macOS remains the only target
platform — see the [decision record](docs/decisions/recording-screen-capture-windows.md).

## Build and run

Requirements: macOS 14+, Go 1.25+, Node.js 20.19+ (or 22.12+), npm, Xcode Command Line Tools.

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo

# Run in development (installs dependencies and bootstraps generated artifacts)
./scripts/dev.sh

# Commit gate: bootstrap + Go build/test/vet/gofmt + frontend typecheck/build
./scripts/gate.sh
```

**A fresh clone must be bootstrapped before any `go build` or `npm run build`.** The two
generated directories depend on each other: `frontend/dist` is referenced by `go:embed all:dist`
(without it the Go module does not compile), and `frontend/wailsjs` is imported by the frontend
sources (without it `vue-tsc` fails) — yet generating the bindings requires a compilable Go tree.
`scripts/bootstrap-frontend.sh` breaks the cycle with "placeholder dist → bindings → real bundle",
and both `dev.sh` and `gate.sh` call it. On Windows use `scripts/dev.ps1`.

Packaging:

```bash
cd cmd/daygo
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -platform darwin/arm64
```

The output is `build/bin/Daygo.app`. Run `wails` from `cmd/daygo`; it resolves `frontend/` and
`build/` at the repository root and builds the native static library through `preBuildHooks`.
The release pipeline (signing, notarization, auto-update) does not exist yet.

## Contributing

Work follows the [feature module roadmap](docs/09-roadmap.md) and each module's execution guide. Modules may develop concurrently and integrate when their specific capabilities are ready. Work must respect the constraints in [docs/07-privacy-security.md](docs/07-privacy-security.md). Read [AGENTS.md](AGENTS.md) before making implementation changes.

For substantial changes, open an issue first and state which feature module, shared capabilities, and verification gates the work addresses.

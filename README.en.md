# Daygo

[简体中文](README.md) · **English**

Daygo is a private, local-first work journal for macOS. It captures screen activity at intervals, uses an AI provider chosen by the user to understand that activity, and turns it into a searchable daily timeline, standup summary, and review.

> **Project status:** Daygo is being built with a Go core, Wails, and Vue. What exists today is the desktop shell and the frontend page skeleton; it is not ready for general installation yet.

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
platform adapter (implementation undecided)
  └── screen capture, system permissions, keychain, status item, updates
```

Go owns all portable business logic and is the single SQLite writer. The parts that need macOS system capabilities sit behind a small set of ports whose **implementation is not yet chosen** — the docs define only the contract any implementation must satisfy. That boundary is what lets the Go core build and test without macOS (`CGO_ENABLED=0`), which the whole testing strategy depends on.

See the [design docs](docs/README.md) for requirements, interfaces, data model, testing strategy, milestones, and risks.

## Current repository layout

```text
cmd/daygo/                  Go command entry point and wails.json
internal/                   Go core (currently the app shell only)
frontend/                   Vue 3 + TypeScript frontend
build/                      Wails build assets and output
testdata/                   fixtures and reference databases (not yet present)
docs/                       design documentation
```

Most Go paths described under `docs/` are still target state. **What exists today is the Wails shell, the frontend page skeleton, and the settings screen** — without storage, analysis, AI, platform adapter, or background lifecycle. See [docs/09-roadmap.md](docs/09-roadmap.md) for the plan.

## Build and run

Requirements: macOS 14+, Go 1.25+, Node.js 20.19+ (or 22.12+), npm.

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo

# Install and build the frontend
npm --prefix frontend ci
npm --prefix frontend run build

# Go checks
go test ./...
go vet ./...

# Build the macOS app (wails.json lives next to the entry point)
cd cmd/daygo
go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -platform darwin/arm64
```

The output is `build/bin/Daygo.app`. Build the frontend first after a fresh clone, otherwise `frontend/dist` is missing and the Go build fails. Run `wails` from `cmd/daygo`; it resolves `frontend/` and `build/` at the repository root.

## Contributing

Work should follow the milestone order in [docs/09-roadmap.md](docs/09-roadmap.md) and respect the constraints in [docs/07-privacy-security.md](docs/07-privacy-security.md). Read [AGENTS.md](AGENTS.md) before making implementation changes.

For substantial changes, open an issue first and state which milestone and verification gate the work addresses.

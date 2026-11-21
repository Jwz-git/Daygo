<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — Remember what you actually worked on today." />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=release" alt="Latest release" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square&logo=windows&logoColor=white" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%203%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails 3 · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>Download</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>Website</b></a> ·
  <a href="../docs/README.md">Design Docs</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <strong>English</strong> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.de.md">Deutsch</a> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>Zero manual time-tracking. Capture your workday and see every day clearly.</strong></p>

Daygo is a local-first work log and review tool for **macOS** and **Windows**. It captures discrete screenshots of your primary display in the background and uses the AI provider you configure to organize your activity into an **automatic timeline, daily standup recaps, weekly reviews, and plan reminders** — with no timers to toggle and no cloud account required.

<p align="center"><b>Discrete screenshots · Local storage · Bring your own AI (or local model) · 9 UI languages</b></p>

<p align="center">
  <a href="#features">Features</a> ·
  <a href="#download-and-install">Download &amp; Install</a> ·
  <a href="#quick-start">Quick Start</a> ·
  <a href="#privacy-and-security">Privacy &amp; Security</a> ·
  <a href="#contributing">Contributing</a>
</p>

---

## Features

<sub>All screenshots below use anonymous sample data.</sub>

### Automatic timeline

Daygo takes discrete screenshots of the system primary display and turns batches of activity into timeline cards with timestamps, titles, summaries, and categories. Expand any card to inspect raw frames, switch between day and week views, edit or delete cards, reprocess a single card or an entire day, clear a day's failed batches in one click, and schedule today's plan blocks in the inspector.

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="Automatic timeline: chronological activity cards with detail inspector" />

### Daily review

Review your workday at a glance and generate standup recaps with highlights, completed work, and blockers. Capture notes in a daily journal, track daily goals, enable daily journal reminder notifications, and optionally display a Token usage chart at the bottom of the daily report (off by default).

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="Daily review: workday overview and standup recap" />

### Weekly review

Step back with a weekly workflow overview, focus vs. distraction heatmap, category breakdown, top apps, and time flow — with the same opt-in Token usage chart (tracked duration excludes the `System` category).

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="Weekly review: category breakdown, top apps, and time flow" />

<details>
<summary>View dark mode</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo dark mode preview" /></p>

</details>

### Resident background & customization

| Capability | Details |
|---|---|
| Background recording | Closing the main window keeps recording active (hiding the macOS Dock running indicator); reopen anytime from the macOS menu bar or Windows notification area |
| Pause & system events | Pause for 15 / 30 / 60 minutes or indefinitely (timed pauses auto-resume); automatically pauses on sleep, screen lock, or screensaver; manually stopping recording is never overridden by system events |
| Capture & storage | Intervals: 1 / 5 / 10 / 20 / 30 / 60 s (default 10 s); height: 720 / 1080 px (default 1080); configurable disk cap with full-segment cleanup; Windows supports relocating the recordings folder |
| Multi-protocol AI & fallback | API Key supports OpenAI Chat Completions, OpenAI Responses, Anthropic Messages, and Google Gemini (prefers direct upload of eligible finalized MP4 segments for timeline analysis, falling back to screenshots); also supports ChatGPT subscription OAuth; multi-model providers and ordered fallback chains |
| Plans & notifications | Today's plan blocks with completion tracking; native macOS / Windows notifications for daily journal reminders, plan start, and distraction alerts |
| Theme, categories & i18n | Light / Dark / System; onboarding career category presets with editable names, order, and colors; launch at login; 9 UI languages |

> **4 AM logical day**: Timeline, plans, journal, and goals split days at **4:00 AM local time**, while standup recaps group by calendar day. Late-night work can therefore appear under different days across these views.

---

## Download and install

Grab the installer for your platform from the [latest GitHub Release](https://github.com/Jwz-git/Daygo/releases/latest):

| Platform | OS & architecture | Package |
|---|---|---|
| macOS | macOS 14+, Apple Silicon (`arm64`) | `Daygo-<version>-arm64.dmg` |
| Windows | Windows 11 24H2+ (build 26100+), x64 (`amd64`) | `Daygo-<version>-amd64-installer.exe` |

macOS is the primary development platform and Windows ships an x64 installer; other architectures and Linux do not yet have release packages. Source code on `test` may be ahead of published releases — see the [module status table](../docs/09-roadmap.md#91-模块总表) for implementation status and [delivery](../docs/modules/delivery.md) for signing, notarization, and installation records.

---

## Quick start

1. **Install & onboard**: Launch Daygo and pick a career category preset and basic preferences during onboarding. On macOS, grant **Screen Recording** permission and restart when prompted.
2. **Connect AI**: During onboarding or in Settings, choose **ChatGPT Subscription** (browser OAuth) or **API Key** (endpoint, key, and typed or fetched multi-select models). After saving, test with text or a single image in **Model test & playground**.
3. **Configure capture & privacy**: Adjust capture interval, blocked apps, and disk limit (on Windows, relocate the recordings folder if desired), then start recording.
4. **Review your day**: Wait for the first analysis batch to appear in Timeline, inspect cards and raw frames, edit or reprocess as needed, and review Daily and Weekly summaries.

> Daygo does not include hosted AI credits. Automatic analysis requires vision capability and structured output for the chosen protocol (including local models); receiving a reply in the playground does not guarantee full pipeline compatibility.

---

## Privacy and security

- **Local-first storage**: Screenshot segments, timelines, journals, plans, settings, and the SQLite database stay on your device. Daygo has no first-party backend, account system, or cloud sync.
- **You control where data goes**: Screen data leaves your machine only for an AI provider you explicitly configure. With a compatible local model, analysis stays entirely on-device; third-party providers are governed by their own privacy policies.
- **Two-layer app privacy**: Blocked apps are excluded from captures, and whenever a blocked app is in the foreground, Daygo saves only a redacted placeholder frame. Both layers remain active together.
- **Write-only system credentials**: API keys and ChatGPT OAuth credentials live exclusively in system credential storage (macOS Keychain / Windows Credential Manager). The UI and binding layer never read credential secrets back, and secrets never enter SQLite, `localStorage`, or error messages.

Capture uses **discrete screenshots** rather than a continuous screen-recording stream. Analytics and crash reporting are off by default (reporting consumers are not implemented). Uninstalling preserves your data and credentials. See [Privacy and security](../docs/07-privacy-security.md) for full boundaries.

---

## Contributing

Active development happens on `test` with **Go 1.26.10 + Wails v3.1.0 + Vue 3**; `main` is the stable branch (see the [Wails v3 GA migration record](../docs/decisions/wails-v3-migration.md)). Go owns product logic and SQLite writes, platform adapters sit behind ports, and Vue talks to Go through generated TypeScript bindings.

### Prerequisites

- Go version pinned in [go.mod](../go.mod)
- Node.js `20.19+` (20.x) or `22.12+` with npm
- **macOS**: Xcode Command Line Tools
- **Windows**: MinGW-w64 (with binutils), Visual Studio 2022 C++ tools, Windows SDK 26100, and WebView2 Runtime — see [Windows build dependencies](../scripts/README.md#windows-build-dependencies)
- **Linux**: GTK4 and WebKitGTK 6.0 development headers by default (arm64 host build verified; screen capture and system integration pending)

### Clone, verify, and run

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # Clean-checkout bootstrap, build, Go / frontend tests, and doc checks
```

| Platform | Start the dev app |
|---|---|
| macOS | `./scripts/dev.sh` |
| Windows | `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dev.ps1` |
| Linux | `./scripts/dev-linux.sh` |

See [scripts/README.md](../scripts/README.md) for build and packaging commands, and [Design docs](../docs/README.md) plus [AGENTS.md](../AGENTS.md) for architecture and engineering rules.

To report bugs, open an [Issue](https://github.com/Jwz-git/Daygo/issues) with your OS, app version, reproduction steps, and redacted errors. **Never upload real screenshots, databases, or API keys.**

---

## Roadmap

- Plugin system development
- Linux screen capture and system integration

---

## License

Open-sourced under the [MIT License](../LICENSE).

<sub>Inspired by <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> (MIT, © 2025 Jerry Liu).</sub>

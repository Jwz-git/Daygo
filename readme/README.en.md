<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — At the end of the day, do you remember what you worked on?" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=version" alt="Latest stable release" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>Download</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>Website</b></a> ·
  <a href="../docs/README.md">Design docs</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <strong>English</strong> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.de.md">Deutsch</a> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>Record your work on screen. Look back on your day.</strong></p>

Daygo is a work recording and review tool for macOS and Windows. It takes screenshots at intervals in the background and uses your chosen AI provider to organize screen activity into a **timeline, daily review, and weekly summary** for standups, retrospectives, and recalling work details.

**Discrete screenshots · Local storage · Your choice of AI · Nine interface languages**

[Download and install](#download-and-install) · [Quick start](#quick-start) · [Feature preview](#feature-preview) · [Privacy and data](#privacy-and-data) · [Contributing](#contributing)

## Download and install

Choose the installer for your platform under [Assets in the latest stable release](https://github.com/Jwz-git/Daygo/releases/latest):

| Platform | System and architecture | Installer |
|---|---|---|
| macOS | macOS 14+, Apple Silicon (arm64) | `Daygo-<version>-arm64.dmg` |
| Windows | Windows 11 24H2+ (build 26100+), x64 (amd64) | `Daygo-<version>-amd64-installer.exe` |

macOS is the primary development platform, and Windows has an x64 installer. Other architectures and Linux do not have published installers yet. Source changes may be ahead of the released version; see the [module status table](../docs/09-roadmap.md#91-模块总表) for feature status and [Delivery](../docs/modules/delivery.md) for signing, notarization, and installation validation.

## Quick start

1. **Install and grant permission**: launch Daygo. On macOS, grant Screen Recording permission and restart when the app prompts you to.
2. **Configure AI**: add a service URL, API key, and model in Settings. Save, then open “Model test and playground” to check a reply using text or an image.
3. **Choose what to record**: set the screenshot interval, blocked apps, and disk limit, then enable recording.
4. **Review the results**: after the first analysis batch completes, inspect cards and original frames in the timeline, edit as needed, and open the daily and weekly pages to review your activity.

Daygo does not include an AI service or credits. Automatic analysis requires image recognition and structured output support for the configured protocols; a reply in the playground does not verify the full analysis pipeline. Local models need these capabilities too.

## Feature preview

<sub>The screenshots below use anonymous sample data.</sub>

### Automatic timeline

Daygo captures the main display at intervals and uses AI to create cards with times, titles, summaries, and categories. Expand a card to inspect the original frames, or edit, delete, and reprocess results.

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="Timeline: activity cards arranged by time with a detail panel" />

### Daily review

Review your daily workflow and generate a standup recap with highlights, completed work, and blockers. Add your own perspective with a journal and daily goals.

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="Daily review: workflow overview and standup recap" />

### Weekly review

Look back on the week through its workflow, focus and distraction heat map, category shares, most used apps, and time flow. Tracked-time totals exclude the System category.

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="Weekly review: most used apps by category and a time flow chart" />

### Background recording and customization

| Capability | Details |
|---|---|
| Background recording | Recording continues after you close the window; reopen it from the macOS menu bar or Windows notification area |
| Pause and resume | Pause for 15 / 30 / 60 minutes or indefinitely; timed pauses resume automatically |
| System events | Capture pauses during sleep, screen lock, and the screensaver; system events do not restart recording you turned off |
| Screenshot settings | Intervals of 1 / 5 / 10 / 20 / 30 / 60 seconds, default 10; heights of 720 / 1080 pixels, default 1080 |
| AI services | OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages; multiple models per provider and an ordered fallback chain |
| Appearance and categories | Light / dark / system theme; editable category names, order, and colors; launch at login and macOS Dock icon preferences |

The interface supports Simplified Chinese, Traditional Chinese, English, Japanese, Korean, German, French, Spanish, and Brazilian Portuguese.

<details>
<summary>View dark mode</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo in dark mode" /></p>

</details>

Timeline, journal, and goal days start at **4 AM local time**; standup recaps use calendar days. Late-night activity can therefore belong to different days in these views.

## Privacy and data

- **Local storage**: screenshots, timelines, journals, settings, and the database stay on your device. Daygo has no first-party backend, account, or sync service.
- **Your choice of destination**: screen data leaves the device only for an AI service you explicitly configure. A compatible local model can run screen analysis on your device; third-party services handle data according to their own privacy policies.
- **App blocking and foreground redaction**: blocked apps are excluded from screenshots. When a blocked app is in the foreground, Daygo stores a redacted placeholder frame. Both safeguards remain in place.
- **System credential storage**: API keys stay in macOS Keychain or Windows Credential Manager. The interface can write them but cannot read them; they do not enter the app database, localStorage, or error messages.

Recording uses discrete screenshots to avoid a continuous screen recording stream. Analytics and crash reporting are off by default; reporting consumers are not implemented yet.

Uninstalling preserves user data and credentials. See [Privacy and security](../docs/07-privacy-security.md) for the complete data boundary.

## Contributing

Go owns product logic and database writes. Platform capabilities sit behind interfaces, and Vue accesses Go through generated Wails bindings. `test` is the development branch; `main` is the stable branch.

Development on macOS requires Go, Node.js/npm, and Xcode Command Line Tools. See [go.mod](../go.mod) for the Go version requirement and the [development entry](../scripts/dev.sh) for Node.js requirements.

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # Bootstrap, build, Go / frontend tests, and documentation checks
```

The gate prepares frontend artifacts and Wails bindings in the required order. See [Script entry points](../scripts/README.md) for platform development commands, and [Design documentation](../docs/README.md) and [AGENTS.md](../AGENTS.md) for design and contribution rules.

Report problems through an [issue](https://github.com/Jwz-git/Daygo/issues) with your system, app version, reproduction steps, and redacted errors. Do not upload real screenshots, databases, or API keys.

## License

Released under the [MIT License](../LICENSE).

<sub>Inspired by <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> (MIT, © 2025 Jerry Liu).</sub>

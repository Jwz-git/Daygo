# Daygo

[简体中文](README.md) · **English**

> At the end of the day, do you remember what you worked on?

Daygo quietly records your work in the background. With an AI provider you choose, it turns a scattered day into a timeline, daily review, and weekly summary. Coding, research, meetings, conversations, and thinking no longer disappear when you close a window.

Daygo puts local storage and privacy first. Screen content stays on your computer by default; its only path off the device is to an AI provider you explicitly configure. Daygo takes **discrete screenshots**, not a continuous screen recording, so the system's screen recording indicator does not have to stay on continuously.

## Why Daygo

The usual tools leave gaps when you look back on your work:

- **Manual timers** require you to remember to start and stop them, especially when you are focused.
- **App usage statistics** can say “three hours in VS Code” but cannot say what you did there.
- **Calendars** show what you planned, not necessarily what happened.

Daygo captures the context of the work itself: what you built, investigated, discussed, and reviewed. When it is time for a standup or retrospective, you have a record to work from.

## What Daygo does

**Automatic timeline**

- At intervals, Daygo captures the main display and uses AI to organize activity into cards with times, titles, summaries, and categories.
- Each card links to the original frames, which you can inspect in its frame strip.

**Daily and weekly reviews**

- **Daily:** a calendar-day recap of highlights, completed work, and blockers for your standup.
- **Weekly:** tracked and focused time, plus category shares; the total excludes the System category.

**Control over the results**

- Edit a card's category, title, or summary, and soft-delete cards you do not need.
- Reprocess a time range when the first result is not useful, without creating duplicate cards.

**Your choice of AI**

- Supports OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages protocols.
- Configure multiple models per provider and an ordered fallback chain across providers. A compatible local model can keep analysis on your device.
- Daygo has no first-party backend and does not provide or choose a default provider.

**Settings for your workflow**

- Adjust the screenshot interval (1 / 5 / 10 / 20 / 30 / 60 seconds; 10 by default), resolution (720 / 1080; 1080 by default), blocked apps, and disk limit.
- Add, remove, reorder, and recolor categories. Choose whether Daygo starts at login and whether its Dock icon is shown.
- Use the interface in Simplified Chinese, Traditional Chinese, English, Japanese, Korean, German, French, Spanish, or Brazilian Portuguese, with light, dark, or system theme.

## Always available

Daygo runs in the background after you close its window:

- Recording continues when the window closes. Open it again, pause recording, or check its state from the menu bar.
- Pause for 15, 30, or 60 minutes, or indefinitely. A timed pause resumes automatically.
- Capture stops during sleep, screen lock, and the screensaver, then resumes afterward. System events do not restart recording if you turned it off yourself.

## Your data, your choice

Daygo handles highly sensitive screen information, so privacy is a product boundary:

- Screenshots, timelines, journals, and the database stay on your computer by default. There is no Daygo server, account, or sync service.
- Screen data leaves your computer only for the AI provider you explicitly configure. With a compatible local model, analysis can stay on your device.
- Block apps by bundle ID. When a blocked app is in the foreground, Daygo writes a redacted placeholder frame: the timeline still shows that a period of activity occurred, without its content. The blocklist and placeholder are both required safeguards.
- API keys go only into the operating system's credential store (macOS Keychain or Windows Credential Manager). The frontend can write keys but cannot read them; the backend reads them only for provider calls. Keys never enter frontend localStorage, the app database, or error messages.
- Analytics and crash reporting settings are off by default; reporting consumers are not implemented yet. The privacy contract forbids screen content, window titles, file paths, API keys, or AI request payloads in reports.

See [Privacy and security](docs/07-privacy-security.md) for the complete boundary.

## Download

Get the latest installer for your platform from the [Releases page](https://github.com/Jwz-git/Daygo/releases):

| Platform | System and architecture | Installer |
|---|---|---|
| macOS | macOS 14+, Apple Silicon (arm64) | `Daygo-<version>-arm64.dmg` |
| Windows | Windows 11 24H2 (build 26100+), x64 (amd64) | `Daygo-<version>-amd64.exe` |

macOS is the primary development platform; a Windows installer is also available. Other architectures and Linux do not have downloadable builds yet.
Installer availability and feature / distribution acceptance are tracked separately in the [module status table](docs/09-roadmap.md#91-模块总表).
Windows uninstall preserves user data and credentials. Removing the macOS app also does not delete its application support directory or Keychain entries.

## First run

1. Install and launch Daygo.
2. On macOS, grant Screen Recording permission when prompted. Daygo may need to restart afterward.
3. Add your AI provider, API key, and model in Settings, then run the connection test.
4. Choose a screenshot interval, blocked apps, and a disk limit, then start recording.
5. When the first analysis batch completes, review the timeline and edit categories or summaries as needed.

Daygo does not include an AI service or API credits. How a third-party provider handles data you send depends on that service and its privacy policy.

## Contributing

Daygo is built with Go, Wails, Vue 3, and SQLite. Go owns product logic and is the sole database writer. Platform capabilities sit behind ports, and the frontend accesses Go through generated Wails bindings.

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # Run the complete commit gate: bootstrap, build, tests, frontend
```

For design specifications, architecture, and contribution constraints, see [Design documentation](docs/README.md) and [AGENTS.md](AGENTS.md).

Found a problem? Open an [issue](https://github.com/Jwz-git/Daygo/issues). Do not attach real screenshots, API keys, databases, or other sensitive information.

## License

Released under the [MIT License](LICENSE).

## Acknowledgements

Product concept inspired by projects such as [Dayflow](https://github.com/JerryZLiu/Dayflow).

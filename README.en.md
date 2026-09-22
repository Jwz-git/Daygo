# Daygo

[简体中文](README.md) · **English**

**Let the day happen. Let the record write itself.**

Daygo is a local-first, privacy-conscious desktop app for personal work history. It takes discrete screenshots
in the background and uses the AI provider you configure to turn scattered activity into a timeline, daily
reviews, and weekly summaries—so coding, research, meetings, conversations, and thinking do not disappear
when their windows close.

> Current release: **v0.1.0 pre-release** · Available for macOS and Windows · Bring your own AI provider

## Download

Get the installer for your platform from the [v0.1.0 release](https://github.com/Jwz-git/Daygo/releases/tag/v0.1.0):

| Platform | System and architecture | Installer |
|---|---|---|
| macOS | macOS 14+, Apple Silicon (arm64) | [`Daygo-0.1.0-arm64.dmg`](https://github.com/Jwz-git/Daygo/releases/download/v0.1.0/Daygo-0.1.0-arm64.dmg) |
| Windows | Windows 11, x64 (amd64) | [`Daygo-0.1.0-amd64.exe`](https://github.com/Jwz-git/Daygo/releases/download/v0.1.0/Daygo-0.1.0-amd64.exe) |

This is an early pre-release. GitHub Actions builds installers after a Release is published. Signing, notarization, in-app automatic updates, the complete install/upgrade matrix,
and long-running stability tests are still in progress, so your operating system may show an unknown-developer
or security warning. Download Daygo only from this repository's Releases page and keep an independent backup
of important data while evaluating it. Other architectures and Linux do not have downloadable builds yet.

## What Daygo does

- **Records work context automatically:** takes discrete screenshots in the background without asking you to run a timer.
- **Builds a timeline you can revisit:** turns activity into cards with times, titles, summaries, categories, and related frames.
- **Creates daily and weekly reviews:** shows the day's progress and the week's time distribution, category shares, and trends.
- **Keeps results editable:** change card titles, summaries, and categories; remove unwanted content; or reprocess a time range.
- **Uses your choice of AI:** supports OpenAI Chat Completions, OpenAI Responses, and Anthropic-compatible protocols,
  with multiple models and an ordered fallback chain.
- **Adapts to your workflow:** offers Chinese and English, light and dark themes, capture intervals, blocked apps, and storage limits.

## Privacy boundaries

Daygo handles highly sensitive screen information, so privacy is a product boundary rather than an optional mode.

- Screenshots, timelines, journals, and the database stay on your computer by default.
- Daygo has no first-party backend and neither provides nor selects a default AI provider.
- Screen data is sent only to a provider you explicitly configure. You can also connect a compatible local model.
- You can block specific applications. When one is frontmost, Daygo records a redacted placeholder frame.
- API keys are stored only in the operating system's credential store, not in frontend localStorage or the app database.
- Analytics and crash reporting are off by default and must not contain screen content, window titles, file paths,
  API keys, or LLM request payloads.

See [Privacy and security](docs/07-privacy-security.md) for the complete boundary.

## First run

1. Install and launch Daygo.
2. On macOS, grant Screen Recording permission when prompted. Daygo may need to restart afterward.
3. Add an AI provider, API key, and model in Settings, then run the connection test.
4. Choose a capture interval, blocked applications, and a storage limit, then start recording.
5. When the first analysis batch completes, review the timeline and edit the results if needed.

Daygo does not include an AI service or API credits. How data sent to a third-party provider is handled depends
on the service you choose and its privacy policy.

## Project status

v0.1.0 provides installers for macOS and Windows, but publishing an installer does not mean every feature has
completed its real-user acceptance loop. Evidence is still being collected for real providers, background
operation after closing the window, both privacy safeguards, install/upgrade behavior, and 7/14-day runs.
See the [roadmap module table](docs/09-roadmap.md#91-模块总表) for implemented scope, verification evidence,
and remaining risks.

If you find a problem, open an [issue](https://github.com/Jwz-git/Daygo/issues) with your OS version, Daygo
version, reproduction steps, and expected/actual behavior. Do not attach real screenshots, API keys, databases,
recordings, window titles, or other sensitive information.

## Run from source

You need Go 1.25+, Node.js 20.19+ (or 22.12+), and npm. macOS also needs Xcode Command Line Tools; see the
[script guide](scripts/README.md) for Windows native build dependencies.

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo

# macOS
./scripts/dev.sh

# Windows PowerShell
./scripts/dev.ps1
```

## Contributing

Daygo is built with Go, Wails, Vue 3, TypeScript, Pinia, and SQLite. The Go core owns product logic and is the
sole database writer. Platform capabilities sit behind narrow ports, and the frontend accesses Go only through
generated Wails bindings.

```text
Vue 3 + TypeScript
        ↓ Wails bindings
Go services and foundation
        ↓ platform ports
macOS / Windows native adapters
```

Run the complete gate before submitting a change:

```bash
./scripts/gate.sh
```

Read more: [Design documentation](docs/README.md) · [Testing strategy](docs/08-testing-strategy.md) ·
[Contribution constraints](AGENTS.md)

## License

Released under the [MIT License](LICENSE).

## Acknowledgements

Product concept inspired by projects such as [Dayflow](https://github.com/JerryZLiu/Dayflow).

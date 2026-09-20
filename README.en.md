# Daygo

[简体中文](README.md) · **English**

**Let the work happen. Let the record write itself.**

> **Source-built preview for macOS** · Bring your own AI provider · Data stays local by default

Daygo brings discrete screen capture, AI analysis, and work reflection into one privacy-first, local-first desktop app. It organizes activity cards over time, helps you review the day, and reveals weekly patterns from the records you already have.

There is no timer to manage and no end-of-day struggle to remember where the hours went. Open Daygo and see how your work actually unfolded: coding, research, meetings, conversations, design, and the transitions between them.

## Why Daygo

- **Automatic, not another habit to maintain:** discrete screenshots become activity cards with times, titles, summaries, and categories.
- **Context you can return to:** open a timeline card to review its related frames and quickly recover what you were working on.
- **A useful daily review:** generate a recap from the day's activity, then add your own journal and goals.
- **A clearer view of the week:** review tracked time, focus time, category shares, daily distribution, and trends.
- **Results stay under your control:** edit titles, summaries, and categories; remove cards you do not need; or reprocess a period of activity.
- **Your choice of AI:** use OpenAI Chat Completions, OpenAI Responses, or Anthropic-compatible providers, configure multiple models per provider, and arrange an ordered fallback chain.

## Designed for privacy

Daygo handles deeply personal data, so privacy is a product boundary rather than an optional mode.

- Screenshots, timelines, journals, and the database stay on your computer by default.
- Daygo has no first-party backend and does not select a default AI provider for you.
- Screen data is sent only to a provider you explicitly configure. You can also connect a compatible local model.
- You can block specific applications. When a blocked app is frontmost, Daygo records a redacted placeholder frame.
- API keys are stored only in the system credential store; the frontend cannot read their plaintext values.
- Analytics and crash reporting are off by default and must never contain screen content, window titles, file paths, API keys, or LLM payloads.

On macOS, your data lives in:

```text
~/Library/Application Support/Daygo/
```

The database, recording segments, and backups are kept together there. API keys remain in the system keychain. See [Privacy and security](docs/07-privacy-security.md) for the full boundary.

## Try Daygo

Daygo is currently available as a source-built preview for macOS 14 or later. You will need Go 1.25+, Node.js 20.19+ (or 22.12+), npm, and Xcode Command Line Tools.

```bash
git clone https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/dev.sh
```

After the first launch:

1. Grant screen-recording permission when macOS asks.
2. Add an AI provider, API key, and model in Settings, then run the connection test.
3. Choose your capture interval, blocked applications, interface language, and storage limit.
4. Start recording. Your timeline will appear after the first analysis batch completes.

> macOS may require the application to restart before a newly granted screen-recording permission takes effect.

## You stay in control

- Start, pause, or stop recording at any time.
- Use light, dark, or system appearance and switch between Chinese and English.
- Set a disk limit for recordings. Cleanup operates on complete segments and never removes data that is still being written.
- Review recording storage usage and diagnostics that exclude sensitive content.
- Use automatic checkpoints, retained backups, and corruption recovery to protect the local database.

## For contributors

Daygo is built with Go, Wails, Vue 3, TypeScript, Pinia, and SQLite. The Go core owns the product logic and is the sole database writer. Platform capabilities sit behind narrow ports, and the frontend reaches Go only through generated Wails bindings.

```text
Vue 3 + TypeScript
        ↓ Wails bindings
Go services and foundation
        ↓ platform ports
macOS native adapters
```

Run the complete gate before submitting a change:

```bash
./scripts/gate.sh
```

The gate bootstraps frontend artifacts and Wails bindings, then runs Go builds, tests, and static checks together with frontend type checking, unit tests, the production build, and documentation checks.

Read more:

- [Design documentation](docs/README.md)
- [Feature modules and verification evidence](docs/09-roadmap.md#91-模块总表)
- [Testing strategy](docs/08-testing-strategy.md)
- [Contribution constraints](AGENTS.md)

Reproducible bug reports, thoughtful improvements, and clearly scoped contributions are welcome.

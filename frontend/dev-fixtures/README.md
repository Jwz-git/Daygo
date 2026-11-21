# frontend/dev-fixtures/

Anonymous JSON fixtures served **only** by the Vite development server when Wails bindings are unavailable. These files stay outside `frontend/src/`, are never imported by application code, and are excluded from production `frontend/dist/`.

Whenever a store or API module falls back to these fixtures, the UI displays a **“sample data · development only”** badge so preview data is never mistaken for real recordings.

## Fixture files

| File | Contents |
|---|---|
| `timeline.json` | Seven anonymous activities covering normal blocks, an idle gap, and adjacent short cards |
| `daily.json` | Six anonymous activities plus a short standup recap |
| `weekly.json` | Single anonymous aggregate week |
| `settings.json` | Settings DTO fixture (including UI visibility and native-menu payloads) |
| `applications.json` | Installed-application DTO fixture for privacy picker previews |

None of these files contains recordings from a real person or project.

## Toggling fixtures in development

Browser automation or manual Vite previews can control fixture fallback via the `testData` query parameter (either before or after the hash):

- **Enable fixtures (dev default)**: `http://127.0.0.1:5173/#/timeline?testData=on`
- **Disable all fixtures & in-memory stand-ins**: `http://127.0.0.1:5173/#/timeline?testData=off` (exposes empty / backend-unavailable states)

The switch applies across timeline, daily, weekly, settings, providers, and chat. It stays in memory across SPA route transitions (never persisted to `localStorage`), and **production builds always disable test data** regardless of URL parameters.

Development previews are not acceptance evidence; real feature verification is tracked in the [module status table](../../docs/09-roadmap.md#91-模块总表).

## Removal checklist

To remove dev fixtures completely, delete:

1. This directory (`frontend/dev-fixtures/`)
2. `frontend/src/api/developmentFixtures.ts` and the `developmentFixtures()` Vite plugin
3. The fixture fallback branches in the timeline, daily, weekly, settings, and applications stores / API modules

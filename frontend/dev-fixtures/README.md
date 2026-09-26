# Frontend development fixtures

These anonymous payloads are served only by Vite's development server. They are
outside `src`, are not imported by the application, and must not appear in
`frontend/dist`.

The matching store / API module (timeline, daily, weekly, settings, applications)
uses its JSON file only when the required Wails bindings are missing. A visible
“sample data · development only” badge keeps that state distinct from real activity.
`daily.json` contains six anonymous activities and a short recap. `timeline.json`
contains seven anonymous activities, including an idle range and adjacent short
cards. `weekly.json` contains only one anonymous aggregate week. `settings.json`
and `applications.json` mirror their respective binding DTOs. None of these files
is a recording of a real person or project.

Browser automation can select either state at navigation time:

- `http://127.0.0.1:5173/#/timeline?testData=on` uses anonymous test data.
- `http://127.0.0.1:5173/#/timeline?testData=off` disables every fixture and
  in-memory stand-in, exposing the unavailable or empty state.

The query also works before the hash. Omitting it keeps the development default
(`on`). Production always disables test data, regardless of the URL. Timeline,
daily, weekly, settings, providers, and chat share this switch.
The selection remains in memory across SPA route changes, including when a route drops the query string;
it does not persist in localStorage. Settings fixtures also supply UI visibility and native-menu payloads for
the anonymous preview. These stand-ins are development evidence; real functionality acceptance is tracked
in the [module status table](../../docs/09-roadmap.md#91-模块总表).

To remove the previews later, delete this directory,
`src/api/developmentFixtures.ts`, the `developmentFixtures()` Vite plugin, and
the fallback branches in the timeline, daily, weekly, settings, and applications
stores / API modules.

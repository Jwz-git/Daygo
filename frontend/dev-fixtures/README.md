# Frontend development fixtures

These anonymous payloads are served only by Vite's development server. They are
outside `src`, are not imported by the application, and must not appear in
`frontend/dist`.

The timeline store uses `timeline.json` only when a timeline Wails binding is
missing. A visible “sample data · development only” badge keeps that state
distinct from real activity.

To remove the preview later, delete this directory, `src/api/developmentFixtures.ts`,
the `developmentFixtures()` Vite plugin, and the fallback branch in the timeline
store.

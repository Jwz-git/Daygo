# Frontend development fixtures

These anonymous payloads are served only by Vite's development server. They are
outside `src`, are not imported by the application, and must not appear in
`frontend/dist`.

The timeline and daily stores use their matching JSON file only when the
required Wails bindings are missing. A visible “sample data · development
only” badge keeps that state distinct from real activity. `daily.json` contains
six anonymous activities and a short recap; it is not a recording of a real
person or project.

To remove the previews later, delete this directory,
`src/api/developmentFixtures.ts`, the `developmentFixtures()` Vite plugin, and
the fallback branches in the timeline and daily stores.

# Frontend development fixtures

These anonymous payloads are served only by Vite's development server. They are
outside `src`, are not imported by the application, and must not appear in
`frontend/dist`.

The timeline, daily, and weekly stores use their matching JSON file only when the
required Wails bindings are missing. A visible “sample data · development
only” badge keeps that state distinct from real activity. `daily.json` contains
six anonymous activities and a short recap. `timeline.json` contains seven
anonymous activities, including an idle range and adjacent short cards.
`weekly.json` contains only one anonymous aggregate week. None of these files is
a recording of a real person or project.

To remove the previews later, delete this directory,
`src/api/developmentFixtures.ts`, the `developmentFixtures()` Vite plugin, and
the fallback branches in the timeline, daily, and weekly stores.

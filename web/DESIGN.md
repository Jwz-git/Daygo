# Daygo site · design notes

Imported design notes for the pinned source version in [README.md](README.md).
These describe the website replica; they do not certify parity with every current desktop component.

> **Daygo takes design inspiration from [Dayflow](https://dayflow.so)** ([source](https://github.com/JerryZLiu/Dayflow)) and is built with Go + Wails + Vue for macOS and Windows. The hero pill, inspiration section and footer credit Dayflow as a design reference.
>
> **One scroll = one day.** The page runs from sunrise (06:00) to midnight. The sky, the sun and the nav clock follow the scroll; when the sun sets the whole page — including the app window — switches to Daygo's dark appearance.

## 1. Structure

| Section | Sky time | What it shows |
|---|---|---|
| Hero | 06:00 dawn | Headline, two CTAs, the Daygo window waiting tilted below |
| 时间线 / Timeline (pinned) | 09:00 | The window rises and pins; scrolling walks the track through a whole day (09:00 → 22:00), a card opens in the inspector halfway, and the window still being analysed turns into a card at the end |
| 日报 / Daily | 11:30 | Workflow overview (15-minute cells, distraction track, day total) + the standup |
| 周报 / Weekly | 14:00 | "Most used per category" treemap + "Time between categories and apps" sankey |
| 对话 / Chat (preview) | 16:00 | The chat panel answering a question |
| 原理 / How | 18:40 dusk | Screenshots flow into an AI lens and come out as timeline cards |
| 设计参考 / Inspiration | 19:30 | Dayflow as a design reference alongside Daygo and its Go · Wails · Vue stack |
| 隐私 / Privacy | 20:30 night | Four short points: local first, custom models, blocked apps, keys in Keychain |
| 下载 / Download | 23:00 midnight | Stars, moon, app icon, download buttons |

Only the opening shows the whole window; every other feature is one component on its own.

## 2. Built from the real UI

Ported from `Daygo/frontend/src`, not imitated:

- **Tokens and components** → `src/app.css` (tokens.light/dark, TimelineTrack, TimelineActivityCard, GeneratingCard, the timeline footer's review badge (visual only) and copy button, InspectorCardDetail, CardVideoPlayer, DailyWorkflowOverview, WeeklyChartCard/Treemap/Sankey).
- **Layout maths** → `src/charts.js`: `squarify` (lib/chartLayout.ts), `sankeyLayout` / ribbon paths / gradient stops (lib/sankeyLayout.ts), `appColor` (stores/weeklyCharts.ts). The track uses the app's 2.6 px per minute; the treemap lays out in 800×400 and picks tile tiers from rendered size; the sankey uses Dayflow's 1748×933 columns.
- **Copy** → `src/i18n.js`, taken from the app's zh-CN and en locales.

Sample data (`src/data.js`) is anonymous but shaped like a real day: gaps between activities, distraction cards and embedded distraction records, two plan blocks drawn as dashed gutter lines, a week with change-vs-last-week per app.

## Language

`中 / EN` in the nav switches everything — site copy, app strings, sample data, date and duration formats — and remembers the choice. The first visit defaults to English, matching the imported implementation.

## Type

Same faces as the app: Figtree for UI, Instrument Serif for titles. In Chinese, display titles use the sans face at 650 (the app's CJK rule); in English, display titles and the period title ("Wed, Sep 30") use Instrument Serif.

## 3. Category palette

One set for both appearances, matching Daygo's default categories (`--cat-*` in `src/app.css`, hex in `src/data.js`):

| Colour | Category |
|---|---|
| `#6E7DF7` | Focus Work 专注工作 |
| `#78CCEA` | Learning 学习 |
| `#A15DF6` | Research 研究 |
| `#F3B292` | Communication 沟通 |
| `#EB5635` | Distraction 分心 |
| `#B8E1E2` | Personal 个人 |

## 4. Sky

`main.js` maps scroll position to minutes of the day through anchors (section tops), then interpolates a 12-stop palette (`SKY`) into `--sky-top/mid/bot` and `--sun`. The sun travels an arc from 06:00 to 19:10; a horizon haze strengthens when the sun is low; stars (canvas) and the moon fade in after dusk. At 17:50 `html.is-night` and `data-dg-appearance="dark"` flip together.

## 5. Motion

- Easing: `cubic-bezier(.22,1,.36,1)` for settle, `cubic-bezier(.32,.72,0,1)` (the app's `--dg-ease-glide`) for movement
- Headline lines rise out of a mask with a blur; sections reveal with rise + unblur
- Lenis smooth scroll; magnetic primary buttons; shine sweep on hover
- `prefers-reduced-motion`: no smooth scroll, animations collapse to their end state

## 6. Files

```
index.html        structure and copy
src/main.js       scroll, sky, pinned hero, feature mounts, pipeline, reveals
src/app.js        timeline window + feature components
src/charts.js     treemap + sankey geometry (ported)
src/i18n.js       zh / en copy and formatters
src/app.css       Daygo tokens + component styles
src/data.js       anonymous sample day / week
src/styles.css    site styles
```

```bash
npm run dev     # http://127.0.0.1:5180
npm run build   # dist/, deployable to any static host
```

Deployment follows the parent repository’s [release workflow](../.github/workflows/deploy-web.yml), documented in [README.md](README.md). The upstream local gh-pages deployment script is not used here.

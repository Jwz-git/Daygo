import { readFile } from 'node:fs/promises'
import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { defineConfig, type Plugin } from 'vite'

function developmentFixtures(): Plugin {
  const timelineFixture = fileURLToPath(
    new URL('./dev-fixtures/timeline.json', import.meta.url),
  )
  const dailyFixture = fileURLToPath(
    new URL('./dev-fixtures/daily.json', import.meta.url),
  )
  const weeklyFixture = fileURLToPath(
    new URL('./dev-fixtures/weekly.json', import.meta.url),
  )
  const settingsFixture = fileURLToPath(
    new URL('./dev-fixtures/settings.json', import.meta.url),
  )
  const applicationsFixture = fileURLToPath(
    new URL('./dev-fixtures/applications.json', import.meta.url),
  )

  return {
    name: 'daygo-development-fixtures',
    apply: 'serve',
    configureServer(server) {
      const serveFixture = (path: string, fixture: string) => {
        server.middlewares.use(path, async (_request, response) => {
          try {
            response.statusCode = 200
            response.setHeader('Content-Type', 'application/json; charset=utf-8')
            response.setHeader('Cache-Control', 'no-store')
            response.end(await readFile(fixture, 'utf8'))
          } catch {
            response.statusCode = 500
            response.end('development fixture unavailable')
          }
        })
      }

      serveFixture('/__daygo_dev__/timeline', timelineFixture)
      serveFixture('/__daygo_dev__/daily', dailyFixture)
      serveFixture('/__daygo_dev__/weekly', weeklyFixture)
      serveFixture('/__daygo_dev__/settings', settingsFixture)
      serveFixture('/__daygo_dev__/applications', applicationsFixture)
    },
  }
}

function frameFallthrough(): Plugin {
  return {
    name: 'daygo-frame-fallthrough',
    configureServer(server) {
      // In wails dev the Wails devserver forwards GETs here and falls back to
      // the Go asset-server handler only when this server answers 404. Vite's
      // SPA fallback would rewrite these to index.html for any fetch-like
      // Accept (it matches the wildcard), which is why frames never rendered;
      // answering 404 hands the request to the Go handler. /favicon is the
      // Go-side favicon resolver and needs the same treatment.
      const toGoHandler = (_request: unknown, response: { statusCode: number; end: () => void }) => {
        response.statusCode = 404
        response.end()
      }
      server.middlewares.use('/media', toGoHandler)
      server.middlewares.use('/favicon', toGoHandler)
    },
  }
}

export default defineConfig(({ mode }) => ({
  define: {
    // The test-tools UI (sidebar Test page, capture test, data reset) is a
    // dev/QA surface, not a shipped feature. It rides in dev builds and in an
    // explicit opt-in build (VITE_DAYGO_TEST_TOOLS=1 for a real signed build to
    // hand QA); the production installer tree-shakes it out entirely.
    __DAYGO_TEST_TOOLS__: JSON.stringify(
      mode !== 'production' || process.env.VITE_DAYGO_TEST_TOOLS === '1',
    ),
  },
  plugins: [vue(), developmentFixtures(), frameFallthrough()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
}))

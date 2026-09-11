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
    },
  }
}

export default defineConfig({
  plugins: [vue(), developmentFixtures()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})

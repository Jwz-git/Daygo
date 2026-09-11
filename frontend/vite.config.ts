import { readFile } from 'node:fs/promises'
import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { defineConfig, type Plugin } from 'vite'

function developmentFixtures(): Plugin {
  const timelineFixture = fileURLToPath(
    new URL('./dev-fixtures/timeline.json', import.meta.url),
  )

  return {
    name: 'daygo-development-fixtures',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use('/__daygo_dev__/timeline', async (_request, response) => {
        try {
          response.statusCode = 200
          response.setHeader('Content-Type', 'application/json; charset=utf-8')
          response.setHeader('Cache-Control', 'no-store')
          response.end(await readFile(timelineFixture, 'utf8'))
        } catch {
          response.statusCode = 500
          response.end('development fixture unavailable')
        }
      })
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

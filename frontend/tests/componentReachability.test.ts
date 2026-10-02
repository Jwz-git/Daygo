import assert from 'node:assert/strict'
import test from 'node:test'

// A component nothing imports is dead UI: it still costs review and keeps
// copy alive in nine locales, and when it is a settings section it hides a
// setting the backend still honours. ChatSidebar.vue (replaced by ChatDrawer)
// and OutputLanguageSection.vue (dropped from SettingsView) both sat unmounted
// unnoticed. App.vue is the root and is mounted by main.ts.
test('every Vue component is imported somewhere', async () => {
  const { readFile, readdir } = await import('node:fs/promises')
  const { basename, join } = await import('node:path')

  async function walk(directory: string): Promise<string[]> {
    const entries = await readdir(directory, { withFileTypes: true })
    const nested = await Promise.all(entries.map(async (entry) => {
      const path = join(directory, entry.name)
      if (entry.isDirectory()) return walk(path)
      return /\.(vue|ts)$/.test(entry.name) ? [path] : []
    }))
    return nested.flat()
  }

  const files = await walk('src')
  const sources = new Map(await Promise.all(files.map(async (file) => [file, await readFile(file, 'utf8')] as const)))
  const unreferenced = files
    .filter((file) => file.endsWith('.vue') && !file.endsWith('/App.vue'))
    .filter((file) => {
      const name = `${basename(file)}'`
      return ![...sources].some(([other, source]) => other !== file && source.includes(name))
    })
  assert.deepEqual(unreferenced, [])
})

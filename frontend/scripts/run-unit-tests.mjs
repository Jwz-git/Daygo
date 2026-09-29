import { spawnSync } from 'node:child_process'
import { mkdtemp, readFile, readdir, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { basename, dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { build } from 'esbuild'
import { compileScript, parse } from '@vue/compiler-sfc'

const testsDirectory = fileURLToPath(new URL('../tests', import.meta.url))
const outputDirectory = await mkdtemp(join(tmpdir(), 'daygo-frontend-tests-'))
const testFiles = (await readdir(testsDirectory))
  .filter((name) => name.endsWith('.test.ts'))
  .sort()
  .map((name) => join(testsDirectory, name))

try {
  await build({
    entryPoints: testFiles,
    outdir: outputDirectory,
    outExtension: { '.js': '.mjs' },
    bundle: true,
    format: 'esm',
    platform: 'node',
    target: 'node20',
    logLevel: 'silent',
    plugins: [{
      name: 'vue-component-fixtures',
      setup(builder) {
        builder.onLoad({ filter: /\.vue$/ }, async ({ path }) => {
          const { descriptor } = parse(await readFile(path, 'utf8'), { filename: path })
          const script = compileScript(descriptor, { id: path, inlineTemplate: true })
          return { contents: script.content, loader: 'ts', resolveDir: dirname(path) }
        })
      },
    }],
    define: {
      'import.meta.env.DEV': 'false',
    },
  })
  const outputFiles = testFiles.map((file) =>
    join(outputDirectory, `${basename(file, '.ts')}.mjs`),
  )

  const result = spawnSync(process.execPath, ['--test', ...outputFiles], { stdio: 'inherit' })
  if (result.error) throw result.error
  process.exitCode = result.status ?? 1
} finally {
  await rm(outputDirectory, { recursive: true, force: true })
}

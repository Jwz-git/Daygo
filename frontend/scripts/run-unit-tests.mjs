import { spawnSync } from 'node:child_process'
import { mkdtemp, readdir, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { basename, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { build } from 'esbuild'

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

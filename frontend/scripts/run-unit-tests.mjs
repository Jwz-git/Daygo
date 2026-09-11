import { spawnSync } from 'node:child_process'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import { build } from 'esbuild'

const outputDirectory = await mkdtemp(join(tmpdir(), 'daygo-frontend-tests-'))
const outputFile = join(outputDirectory, 'appSiteIcon.test.mjs')

try {
  await build({
    entryPoints: [new URL('../tests/appSiteIcon.test.ts', import.meta.url).pathname],
    outfile: outputFile,
    bundle: true,
    format: 'esm',
    platform: 'node',
    target: 'node20',
    logLevel: 'silent',
  })

  const result = spawnSync(process.execPath, ['--test', outputFile], { stdio: 'inherit' })
  if (result.error) throw result.error
  process.exitCode = result.status ?? 1
} finally {
  await rm(outputDirectory, { recursive: true, force: true })
}

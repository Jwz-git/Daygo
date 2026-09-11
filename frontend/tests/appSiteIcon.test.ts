import assert from 'node:assert/strict'
import test from 'node:test'

import {
  appSiteValues,
  displayHost,
  preferredAppSite,
  resolveAppSiteIdentity,
} from '../src/lib/appSiteIcon'

test('normalizes a display host without treating application names as hosts', () => {
  assert.equal(displayHost('https://www.github.com/openai/codex?q=1'), 'github.com')
  assert.equal(displayHost('Visual Studio Code'), null)
})

test('maps known applications and sites using only local rules', () => {
  assert.equal(resolveAppSiteIdentity('Visual Studio Code').kind, 'vscode')
  assert.equal(resolveAppSiteIdentity('https://docs.google.com/document/d/example').kind, 'google-docs')
  assert.equal(resolveAppSiteIdentity('Safari').kind, 'safari')
})

test('keeps unknown domains visible through a stable monogram fallback', () => {
  assert.deepEqual(resolveAppSiteIdentity('planning.example'), {
    kind: 'generic',
    label: 'planning.example',
    host: 'planning.example',
    monogram: 'PL',
  })
})

test('deduplicates sites and falls back to the secondary value', () => {
  assert.deepEqual(appSiteValues({ primary: ' Notes ', secondary: 'notes' }), ['Notes'])
  assert.equal(preferredAppSite({ primary: ' ', secondary: 'Safari' }), 'Safari')
})

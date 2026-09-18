import assert from 'node:assert/strict'
import test from 'node:test'

import {
  appSiteValues,
  displayHost,
  preferredAppSite,
  resolveAppSiteIdentity,
} from '../src/lib/appSiteIcon'
import { hostOf } from '../src/lib/favicon'

test('normalizes a display host without treating application names as hosts', () => {
  assert.equal(displayHost('https://www.github.com/openai/codex?q=1'), 'github.com')
  assert.equal(displayHost('Visual Studio Code'), null)
  assert.equal(hostOf('pinterest'), 'pinterest.com')
  assert.equal(hostOf('bilibili.com'), 'bilibili.com')
  assert.equal(hostOf('Visual Studio Code'), null)
})

test('maps known applications and sites using only local rules', () => {
  assert.equal(resolveAppSiteIdentity('Visual Studio Code').kind, 'vscode')
  assert.equal(resolveAppSiteIdentity('https://docs.google.com/document/d/example').kind, 'google-docs')
  assert.equal(resolveAppSiteIdentity('Safari').kind, 'safari')
  assert.equal(resolveAppSiteIdentity('bilibili.com').kind, 'bilibili')
  assert.equal(resolveAppSiteIdentity('哔哩哔哩').kind, 'bilibili')
  assert.equal(resolveAppSiteIdentity('aistudio.google.com').kind, 'gemini')
  assert.equal(resolveAppSiteIdentity('Google AI Studio').kind, 'gemini')
  assert.equal(resolveAppSiteIdentity('google studio').kind, 'gemini')
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

test('prioritizes browsed site or application over enclosing browser', () => {
  // When primary is browser and secondary is website/app, the website takes precedence
  assert.equal(
    preferredAppSite({ primary: 'Microsoft Edge', secondary: 'Pinterest' }),
    'Pinterest',
  )
  assert.deepEqual(
    appSiteValues({ primary: 'Google Chrome', secondary: 'bilibili.com' }),
    ['bilibili.com', 'Google Chrome'],
  )
  // When primary is website and secondary is browser, website remains first
  assert.equal(
    preferredAppSite({ primary: 'github.com', secondary: 'Microsoft Edge' }),
    'github.com',
  )
  // When both or neither are browsers, order is preserved
  assert.equal(
    preferredAppSite({ primary: 'Safari', secondary: 'Google Chrome' }),
    'Safari',
  )
})

test('normalizes single-word site names without dot to .com domain', () => {
  assert.equal(displayHost('pinterest'), 'pinterest.com')
  assert.equal(displayHost('bilibili'), 'bilibili.com')
})


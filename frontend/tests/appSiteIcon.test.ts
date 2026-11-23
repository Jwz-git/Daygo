import assert from 'node:assert/strict'
import test from 'node:test'

import {
  appSiteValues,
  canonicalHostForKind,
  displayHost,
  findBestInstalledApp,
  preferredAppSite,
  resolveAppSiteIdentity,
  type IndexedApplication,
} from '../src/lib/appSiteIcon'
import { hostOf, isExplicitDomain } from '../src/lib/favicon'

test('normalizes a display host without treating application names as hosts', () => {
  assert.equal(displayHost('https://www.github.com/openai/codex?q=1'), 'github.com')
  assert.equal(displayHost('Visual Studio Code'), null)
  assert.equal(hostOf('pinterest'), 'pinterest.com')
  assert.equal(hostOf('bilibili.com'), 'bilibili.com')
  assert.equal(hostOf('Visual Studio Code'), null)
})

test('maps known applications and sites using only local rules', () => {
  assert.equal(resolveAppSiteIdentity('Visual Studio Code').kind, 'vscode')
  assert.equal(resolveAppSiteIdentity('Daygo').kind, 'daygo')
  assert.equal(resolveAppSiteIdentity('daygo').kind, 'daygo')
  // The model returns the browsed site as often as the application name, so the
  // domain form has to reach the same mark - otherwise that card falls through
  // to the installed-app and favicon probes for an icon the build already ships.
  assert.equal(resolveAppSiteIdentity('code.visualstudio.com').kind, 'vscode')
  assert.equal(resolveAppSiteIdentity('https://docs.google.com/document/d/example').kind, 'google-docs')
  assert.equal(resolveAppSiteIdentity('Safari').kind, 'safari')
  assert.equal(resolveAppSiteIdentity('bilibili.com').kind, 'bilibili')
  assert.equal(resolveAppSiteIdentity('哔哩哔哩').kind, 'bilibili')
  assert.equal(resolveAppSiteIdentity('aistudio.google.com').kind, 'gemini')
  assert.equal(resolveAppSiteIdentity('Google AI Studio').kind, 'gemini')
  assert.equal(resolveAppSiteIdentity('google studio').kind, 'gemini')
})

test('Finder is the system app, never the website finder.com', () => {
  // A dotless name normalizes to "<name>.com" for favicons, which turned the
  // macOS Finder into finder.com. Only the exact app names map to the brand.
  assert.equal(resolveAppSiteIdentity('Finder').kind, 'finder')
  assert.equal(resolveAppSiteIdentity('finder').kind, 'finder')
  assert.equal(resolveAppSiteIdentity('Finder.app').kind, 'finder')
  assert.equal(resolveAppSiteIdentity('访达').kind, 'finder')
  assert.equal(resolveAppSiteIdentity('finder.com').kind, 'generic')
  assert.equal(resolveAppSiteIdentity('Path Finder').kind, 'generic')
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


test('localized names of bundled system apps reach their marks', () => {
  assert.equal(resolveAppSiteIdentity('终端').kind, 'terminal')
  assert.equal(resolveAppSiteIdentity('終端機').kind, 'terminal')
  assert.equal(resolveAppSiteIdentity('信息').kind, 'messages')
  assert.equal(resolveAppSiteIdentity('訊息').kind, 'messages')
  assert.equal(resolveAppSiteIdentity('备忘录').kind, 'notes')
  assert.equal(resolveAppSiteIdentity('備忘錄').kind, 'notes')
  assert.equal(resolveAppSiteIdentity('訪達').kind, 'finder')
  // Exact names only: a page title that merely contains the word is not the app.
  assert.equal(resolveAppSiteIdentity('信息安全周报').kind, 'generic')
})

test('dotless non-Latin names are not guessed as .com sites', () => {
  assert.equal(hostOf('终端'), null)
  assert.equal(displayHost('终端'), null)
  assert.equal(hostOf('微信'), null)
  assert.equal(hostOf('pinterest'), 'pinterest.com')
  assert.notEqual(hostOf('知乎.com'), null, 'a written-out domain still resolves')
})

test('unknown bare app names never guess unrelated .com domains while curated sites resolve accurately', () => {
  for (const appName of ['Preview', 'Mail', 'Calendar', 'Photos', 'Antigravity', 'WeChat.app', 'Code.exe']) {
    assert.equal(hostOf(appName), null, `${appName} must not become a guessed .com host`)
    assert.equal(displayHost(appName), null, `${appName} must not have a guessed display host`)
  }
  assert.equal(hostOf('Obsidian'), 'obsidian.md')
  assert.equal(hostOf('Linear'), 'linear.app')
  assert.equal(hostOf('Telegram'), 'telegram.org')
  assert.equal(hostOf('Zed'), 'zed.dev')
  assert.equal(hostOf('Notion'), 'notion.so')
  assert.equal(hostOf('Google Docs'), 'docs.google.com')
  assert.equal(hostOf('哔哩哔哩'), 'bilibili.com')
  assert.equal(isExplicitDomain('linear.app'), true)
  assert.equal(isExplicitDomain('WeChat.app'), false)
  assert.equal(isExplicitDomain('Google Docs'), false)
})

test('brand rules respect domain and exact-name boundaries without substring hijacking', () => {
  assert.equal(resolveAppSiteIdentity('messages.google.com').kind, 'generic')
  assert.equal(resolveAppSiteIdentity('GoodNotes').kind, 'generic')
  assert.equal(resolveAppSiteIdentity('release-notes.example.com').kind, 'generic')
  assert.equal(resolveAppSiteIdentity('Windows Terminal').kind, 'generic')
  assert.equal(resolveAppSiteIdentity('web-terminal.dev').kind, 'generic')
  assert.equal(resolveAppSiteIdentity('chromewebstore.google.com').kind, 'generic')
  assert.equal(resolveAppSiteIdentity('user.github.io').kind, 'generic')

  assert.equal(resolveAppSiteIdentity('gist.github.com').kind, 'github')
  assert.equal(resolveAppSiteIdentity('developer.apple.com/xcode').kind, 'xcode')
  assert.equal(resolveAppSiteIdentity('https://www.icloud.com/notes/abc').kind, 'notes')
  assert.equal(resolveAppSiteIdentity('Cursor').kind, 'cursor')
  assert.equal(canonicalHostForKind('google-docs'), 'docs.google.com')
  assert.equal(canonicalHostForKind('notion'), 'notion.so')
  assert.equal(canonicalHostForKind('cursor'), 'cursor.com')
  assert.equal(canonicalHostForKind('warp'), 'warp.dev')
})

test('installed-app matching handles bilingual names, aliases, extensions, and bundle IDs', () => {
  const apps: IndexedApplication[] = [
    { id: 'com.tencent.xinWeChat', names: ['微信'] },
    { id: 'com.apple.Preview', names: ['预览'] },
    { id: 'md.obsidian', names: ['Obsidian'] },
    { id: 'com.microsoft.VSCode', names: ['Visual Studio Code'] },
    { id: 'io.github.clash-verge-rev.clash-verge-rev', names: ['Clash Verge'] },
    { id: 'com.todesktop.230313mzl4w4u92', names: ['Cursor'] },
  ]
  assert.equal(findBestInstalledApp('WeChat', apps)?.id, 'com.tencent.xinWeChat')
  assert.equal(findBestInstalledApp('WeChat.app', apps)?.id, 'com.tencent.xinWeChat')
  assert.equal(findBestInstalledApp('Preview', apps)?.id, 'com.apple.Preview')
  assert.equal(findBestInstalledApp('VSCode', apps)?.id, 'com.microsoft.VSCode')
  assert.equal(findBestInstalledApp('Clash Verge Rev', apps)?.id, 'io.github.clash-verge-rev.clash-verge-rev')
  assert.equal(findBestInstalledApp('Cursor', apps)?.id, 'com.todesktop.230313mzl4w4u92')
  assert.equal(findBestInstalledApp('Completely Unknown Tool', apps), null)
})

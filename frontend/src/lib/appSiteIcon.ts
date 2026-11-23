import type { AppSitesDTO } from '@/api/dto'
import { extractExplicitHost, hostOf, looksLikeBundleFilename } from '@/lib/favicon'

export type AppSiteIconKind =
  | 'chatgpt'
  | 'chrome'
  | 'claude'
  | 'ghostty'
  | 'gemini'
  | 'iterm2'
  | 'cursor'
  | 'daygo'
  | 'discord'
  | 'figma'
  | 'finder'
  | 'github'
  | 'google-docs'
  | 'messages'
  | 'notes'
  | 'notion'
  | 'safari'
  | 'slack'
  | 'terminal'
  | 'vscode'
  | 'warp'
  | 'xcode'
  | 'youtube'
  | 'bilibili'
  | 'generic'

export interface AppSiteIdentity {
  kind: AppSiteIconKind
  label: string
  host: string | null
  monogram: string
}

interface BrandRule {
  kind: Exclude<AppSiteIconKind, 'generic'>
  /** Exact domain or subdomain match (e.g. "github.com" matches "gist.github.com", never "foo.github.io"). */
  hosts?: readonly string[]
  /** Exact host + path prefix match (e.g. "icloud.com/notes", "developer.apple.com/xcode"). */
  hostPaths?: readonly string[]
  /** Exact trimmed lowercase display-name match (never loose substring). */
  exactNames?: readonly string[]
}

const brandRules: readonly BrandRule[] = [
  { kind: 'daygo', exactNames: ['daygo'] },
  {
    kind: 'finder',
    exactNames: ['finder', 'finder.app', '访达', '訪達'],
  },
  {
    kind: 'claude',
    hosts: ['claude.ai', 'anthropic.com'],
    exactNames: ['claude', 'anthropic', 'claude desktop', 'claude code'],
  },
  {
    kind: 'gemini',
    hosts: ['gemini.google.com', 'aistudio.google.com', 'makersuite.google.com'],
    exactNames: ['gemini', 'google gemini', 'google ai studio', 'ai studio', 'google studio', 'makersuite'],
  },
  {
    kind: 'bilibili',
    hosts: ['bilibili.com', 'b23.tv'],
    exactNames: ['bilibili', '哔哩哔哩', 'b站'],
  },
  {
    kind: 'ghostty',
    hosts: ['ghostty.org'],
    exactNames: ['ghostty'],
  },
  {
    kind: 'iterm2',
    hosts: ['iterm2.com'],
    exactNames: ['iterm2', 'iterm'],
  },
  {
    kind: 'cursor',
    hosts: ['cursor.com', 'cursor.sh'],
    exactNames: ['cursor'],
  },
  {
    kind: 'warp',
    hosts: ['warp.dev'],
    exactNames: ['warp'],
  },
  {
    kind: 'vscode',
    hosts: ['code.visualstudio.com'],
    exactNames: ['visual studio code', 'vscode', 'vs code'],
  },
  {
    kind: 'github',
    hosts: ['github.com'],
    exactNames: ['github', 'github desktop'],
  },
  {
    kind: 'chatgpt',
    hosts: ['chatgpt.com', 'chat.openai.com', 'openai.com', 'codex.com', 'codex.so'],
    exactNames: ['chatgpt', 'openai', 'codex'],
  },
  {
    kind: 'google-docs',
    hosts: ['docs.google.com'],
    exactNames: ['google docs'],
  },
  {
    kind: 'youtube',
    hosts: ['youtube.com', 'youtu.be'],
    exactNames: ['youtube'],
  },
  {
    kind: 'chrome',
    exactNames: ['google chrome', 'chrome'],
  },
  {
    kind: 'safari',
    exactNames: ['safari'],
  },
  {
    kind: 'messages',
    exactNames: ['imessage', 'messages', '信息', '訊息'],
  },
  {
    kind: 'notes',
    hostPaths: ['icloud.com/notes'],
    exactNames: ['apple notes', 'notes', '备忘录', '備忘錄'],
  },
  {
    kind: 'terminal',
    exactNames: ['terminal', 'apple terminal', '终端', '終端機'],
  },
  {
    kind: 'xcode',
    hostPaths: ['developer.apple.com/xcode'],
    exactNames: ['xcode'],
  },
  {
    kind: 'figma',
    hosts: ['figma.com'],
    exactNames: ['figma'],
  },
  {
    kind: 'slack',
    hosts: ['slack.com'],
    exactNames: ['slack'],
  },
  {
    kind: 'discord',
    hosts: ['discord.com', 'discord.gg'],
    exactNames: ['discord'],
  },
  {
    kind: 'notion',
    hosts: ['notion.so', 'notion.site', 'notion.com'],
    exactNames: ['notion'],
  },
]

const CANONICAL_KIND_HOSTS: Partial<Record<AppSiteIconKind, string>> = {
  bilibili: 'bilibili.com',
  chatgpt: 'chatgpt.com',
  claude: 'claude.ai',
  cursor: 'cursor.com',
  discord: 'discord.com',
  figma: 'figma.com',
  gemini: 'gemini.google.com',
  github: 'github.com',
  'google-docs': 'docs.google.com',
  notion: 'notion.so',
  slack: 'slack.com',
  warp: 'warp.dev',
  youtube: 'youtube.com',
}

export function canonicalHostForKind(kind: AppSiteIconKind): string | null {
  return CANONICAL_KIND_HOSTS[kind] ?? null
}

function nonEmpty(value: string | null | undefined): string | null {
  const trimmed = value?.trim()
  return trimmed ? trimmed.slice(0, 200) : null
}

const BROWSER_NAMES = new Set([
  'edge',
  'microsoft edge',
  'chrome',
  'google chrome',
  'safari',
  'firefox',
  'arc',
  'brave',
  'brave browser',
  'opera',
  'vivaldi',
  'tor browser',
  'chromium',
])

export function isBrowserName(name: string): boolean {
  return BROWSER_NAMES.has(name.trim().toLowerCase())
}

/*
 * Website brands resolve to their real network favicon (or installed desktop
 * app icon when referenced by app name) before falling back to drawn marks.
 */
const WEBSITE_KINDS: ReadonlySet<AppSiteIconKind> = new Set([
  'bilibili',
  'github',
  'youtube',
  'chatgpt',
  'claude',
  'gemini',
  'google-docs',
  'notion',
  'discord',
  'figma',
  'slack',
])

export function isWebsiteKind(kind: AppSiteIconKind): boolean {
  return WEBSITE_KINDS.has(kind)
}

/**
 * Extracts a display host from an explicit URL/domain or curated website name.
 * Never invents `<word>.com` for unknown desktop application names.
 */
export function displayHost(value: string): string | null {
  return hostOf(value)
}

export function appSiteValues(sites: AppSitesDTO | null): string[] {
  if (sites === null) return []

  const candidates = [sites.primary, sites.secondary]
  const p = nonEmpty(sites.primary)
  const s = nonEmpty(sites.secondary)
  // If primary is an enclosing browser and secondary is a website/target app,
  // prioritize the target so the card highlights what was browsed.
  if (p && s && isBrowserName(p) && !isBrowserName(s)) {
    candidates[0] = s
    candidates[1] = p
  }

  const result: string[] = []
  const seen = new Set<string>()
  for (const candidate of candidates) {
    const value = nonEmpty(candidate)
    if (value === null) continue
    const key = value.toLocaleLowerCase('en-US')
    if (seen.has(key)) continue
    seen.add(key)
    result.push(value)
  }
  return result
}

export function preferredAppSite(sites: AppSitesDTO | null): string | null {
  return appSiteValues(sites)[0] ?? null
}

function monogramFor(value: string, host: string | null): string {
  const source = host?.split('.')[0] ?? value
  const words = source
    .split(/[\s._/-]+/u)
    .map((word) => word.trim())
    .filter(Boolean)

  if (words.length >= 2) {
    return `${Array.from(words[0])[0] ?? ''}${Array.from(words[1])[0] ?? ''}`.toUpperCase()
  }
  return Array.from(words[0] ?? '?').slice(0, 2).join('').toUpperCase()
}

function extractHostAndPath(value: string): { host: string; hostPath: string } | null {
  const host = extractExplicitHost(value)
  if (host === null) return null
  const trimmed = value.trim()
  const candidate = /^[a-z][a-z\d+.-]*:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`
  try {
    const parsed = new URL(candidate)
    const path = parsed.pathname.toLowerCase().replace(/\/+$/, '')
    return { host, hostPath: `${host}${path}` }
  } catch {
    return { host, hostPath: host }
  }
}

function hostMatches(actual: string, pattern: string): boolean {
  return actual === pattern || actual.endsWith(`.${pattern}`)
}

function hostPathMatches(actualHostPath: string, pattern: string): boolean {
  return actualHostPath === pattern || actualHostPath.startsWith(`${pattern}/`)
}

function exactLabelForBrand(label: string): string {
  const lower = label.toLocaleLowerCase('en-US')
  if (looksLikeBundleFilename(label)) {
    return lower.replace(/\.(app|exe)$/i, '')
  }
  return lower
}

export function resolveAppSiteIdentity(value: string): AppSiteIdentity {
  const label = nonEmpty(value) ?? '?'
  const exactLabel = exactLabelForBrand(label)
  const explicit = extractHostAndPath(label)

  let kind: AppSiteIconKind = 'generic'
  for (const rule of brandRules) {
    if (rule.exactNames?.includes(exactLabel)) {
      kind = rule.kind
      break
    }
    if (explicit !== null) {
      if (rule.hostPaths?.some((hp) => hostPathMatches(explicit.hostPath, hp))) {
        kind = rule.kind
        break
      }
      if (rule.hosts?.some((h) => hostMatches(explicit.host, h))) {
        kind = rule.kind
        break
      }
    }
  }

  const host = explicit?.host ?? (kind === 'finder' ? null : displayHost(label))
  return {
    kind,
    label,
    host,
    monogram: monogramFor(label, explicit?.host ?? null),
  }
}


// ---------------------------------------------------------------------------
// Installed-application icon resolution
// ---------------------------------------------------------------------------

/*
 * Card appSites often carry installed-application names ("WeChat", "Preview",
 * "Cursor", "Clash Verge", "Microsoft Edge"). Because the UI locale and the
 * LLM's output language can differ (e.g. zh-CN UI lists "微信" / "预览" while
 * the model writes "WeChat" / "Preview"), we index BOTH the active UI locale
 * and English names, plus bundle-identifier tail segments and curated bilingual
 * aliases, to resolve the real offline desktop icon reliably.
 */

import { describeApplications, listInstalledApplications } from '@/api/application'
import { i18n } from '@/i18n'

export interface IndexedApplication {
  id: string
  names: string[]
}

const APP_ALIAS_GROUPS: readonly (readonly string[])[] = [
  ['wechat', '微信'],
  ['preview', '预览', '預覽'],
  ['calendar', '日历', '日曆'],
  ['mail', 'apple mail', '邮件', '郵件'],
  ['photos', '照片'],
  ['system settings', 'system preferences', '系统设置', '系統設定'],
  ['activity monitor', '活动监视器', '活動監視器'],
  ['reminders', '提醒事项', '提醒事項'],
  ['freeform', '无边记', '無邊記'],
  ['music', 'apple music', '音乐', '音樂'],
  ['podcasts', '播客'],
  ['books', 'apple books', '图书', '書籍'],
  ['finder', '访达', '訪達'],
  ['notes', 'apple notes', '备忘录', '備忘錄'],
  ['messages', 'imessage', '信息', '訊息'],
  ['terminal', 'apple terminal', '终端', '終端機'],
  ['feishu', '飞书', '飛書', 'lark'],
  ['dingtalk', '钉钉', '釘釘'],
  ['neteasemusic', 'netease cloud music', '网易云音乐'],
  ['qqmusic', 'qq音乐'],
  ['tencent meeting', 'voov meeting', '腾讯会议'],
  ['wps', 'wps office'],
  ['jianying', 'jianyingpro', 'capcut', '剪映', '剪映专业版'],
  ['visual studio code', 'vscode', 'vs code'],
  ['iterm2', 'iterm'],
  ['google chrome', 'chrome'],
  ['microsoft edge', 'edge'],
  ['clash verge', 'clash verge rev'],
]

const APP_ALIASES: ReadonlyMap<string, readonly string[]> = (() => {
  const map = new Map<string, readonly string[]>()
  for (const group of APP_ALIAS_GROUPS) {
    const normalized = group.map(normalizeName).filter(Boolean)
    for (const key of normalized) {
      map.set(key, normalized)
    }
  }
  return map
})()

let indexedAppsCache: { locale: string; apps: IndexedApplication[] } | null = null

async function installedApps(): Promise<IndexedApplication[]> {
  const locale = i18n.global.locale.value
  if (indexedAppsCache?.locale === locale) return indexedAppsCache.apps

  try {
    const wantsEnglish = !locale.toLocaleLowerCase('en-US').startsWith('en')
    const [localizedList, englishList] = await Promise.all([
      listInstalledApplications(locale).catch(() => []),
      wantsEnglish ? listInstalledApplications('en').catch(() => []) : Promise.resolve([]),
    ])

    const byId = new Map<string, IndexedApplication>()
    const add = (id: string, name: string) => {
      const trimmed = name.trim()
      if (!id || !trimmed) return
      const existing = byId.get(id)
      if (existing) {
        if (!existing.names.some((n) => n.toLocaleLowerCase('en-US') === trimmed.toLocaleLowerCase('en-US'))) {
          existing.names.push(trimmed)
        }
      } else {
        byId.set(id, { id, names: [trimmed] })
      }
    }
    for (const app of localizedList) add(app.id, app.name)
    for (const app of englishList) add(app.id, app.name)

    const apps = Array.from(byId.values())
    if (apps.length > 0) {
      indexedAppsCache = { locale, apps }
    }
    return apps
  } catch {
    return []
  }
}

function stripBundleExtension(value: string): string {
  return value.trim().replace(/\.(app|exe)$/i, '')
}

function normalizeName(value: string): string {
  return stripBundleExtension(value).toLocaleLowerCase('en-US').replace(/[^\p{L}\p{N}]+/gu, ' ').trim()
}

function compactName(value: string): string {
  return normalizeName(value).replace(/\s+/g, '')
}

function bundleIdentifierHints(id: string): string[] {
  // Windows privacy IDs are opaque hashes ("win-exe-sha256:..."); macOS IDs
  // are reverse-DNS bundle identifiers ("com.apple.Preview", "md.obsidian").
  if (!id.includes('.') || id.includes(':')) return []
  const segments = id.split('.').map((s) => s.trim()).filter(Boolean)
  if (segments.length < 2) return []
  const last = segments[segments.length - 1]
  // Skip meaningless generic/hash suffixes like "desktop", "app", "client", "mac".
  const genericTail = new Set(['app', 'desktop', 'client', 'mac', 'macos', 'electron', 'main'])
  const hints: string[] = []
  const lastNorm = normalizeName(last)
  if (lastNorm && !genericTail.has(lastNorm) && !/^[a-z0-9]{12,}$/.test(lastNorm)) {
    hints.push(last)
  } else if (segments.length >= 3) {
    const prev = segments[segments.length - 2]
    const prevNorm = normalizeName(prev)
    if (prevNorm && !genericTail.has(prevNorm)) hints.push(prev)
  }
  return hints
}

function nameScore(siteNorm: string, appName: string): number {
  const appNorm = normalizeName(appName)
  if (!siteNorm || !appNorm) return 0
  if (siteNorm === appNorm) return 2.0

  const siteCompact = siteNorm.replace(/\s+/g, '')
  const appCompact = appNorm.replace(/\s+/g, '')
  if (siteCompact.length >= 2 && siteCompact === appCompact) return 1.85

  const siteTokens = siteNorm.split(' ').filter(Boolean)
  const appTokens = appNorm.split(' ').filter(Boolean)
  const appTokenSet = new Set(appTokens)

  if (siteTokens.every((token) => appTokenSet.has(token))) {
    // Guard against a single 1-char token matching a long unrelated name.
    if (siteTokens.length === 1 && siteTokens[0].length < 2) return 0
    return siteTokens.length / Math.max(1, appTokens.length) + 0.5
  }

  const siteTokenSet = new Set(siteTokens)
  // Allow a slightly more specific model label ("Clash Verge Rev") to match
  // the installed base name ("Clash Verge") when the app name is distinctive.
  if (
    (appTokens.length >= 2 || appCompact.length >= 4) &&
    appTokens.every((token) => siteTokenSet.has(token))
  ) {
    return appTokens.length / Math.max(1, siteTokens.length) + 0.45
  }

  return 0
}

function scoreIndexedApplication(siteVariants: readonly string[], app: IndexedApplication): number {
  let best = 0
  for (const variant of siteVariants) {
    for (const appName of app.names) {
      const score = nameScore(variant, appName)
      if (score > best) best = score
    }
    const variantCompact = variant.replace(/\s+/g, '')
    if (variantCompact.length >= 3) {
      for (const hint of bundleIdentifierHints(app.id)) {
        const hintCompact = compactName(hint)
        if (hintCompact === variantCompact) {
          if (1.55 > best) best = 1.55
        } else if (hintCompact.endsWith(variantCompact) && variantCompact.length >= 5) {
          // Matches e.g. "wechat" against "com.tencent.xinWeChat" ("xinwechat").
          if (1.3 > best) best = 1.3
        }
      }
    }
  }
  return best
}

/**
 * Pure matching helper for installed applications (exported for deterministic
 * unit tests without mocking Wails bindings).
 */
export function findBestInstalledApp(
  site: string,
  apps: readonly IndexedApplication[],
): IndexedApplication | null {
  if (apps.length === 0) return null
  const primaryNorm = normalizeName(site)
  if (!primaryNorm) return null

  const variants = APP_ALIASES.get(primaryNorm) ?? [primaryNorm]

  let best: IndexedApplication | null = null
  let bestScore = 0
  for (const app of apps) {
    const score = scoreIndexedApplication(variants, app)
    if (score > bestScore) {
      bestScore = score
      best = app
    }
  }
  return best !== null && bestScore >= 0.75 ? best : null
}

/**
 * Best-effort real icon for an installed application matching the site
 * string, or null when nothing matches well enough.
 */
export async function matchInstalledAppIcon(site: string): Promise<string | null> {
  const apps = await installedApps()
  const best = findBestInstalledApp(site, apps)
  if (best === null) return null

  const described = await describeApplications([best.id])
  const icon = described.find((app) => app.id === best.id)?.iconDataUrl ?? ''
  return icon !== '' ? icon : null
}

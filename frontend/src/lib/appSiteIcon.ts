import type { AppSitesDTO } from '@/api/dto'

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
  matches: readonly string[]
}

const brandRules: readonly BrandRule[] = [
  { kind: 'daygo', matches: ['daygo'] },
  { kind: 'claude', matches: ['claude', 'anthropic'] },
  {
    kind: 'gemini',
    matches: [
      'gemini',
      'aistudio.google',
      'google ai studio',
      'ai studio',
      'google studio',
      'makersuite',
    ],
  },
  { kind: 'bilibili', matches: ['bilibili', 'b23.tv', '哔哩哔哩'] },
  { kind: 'ghostty', matches: ['ghostty'] },
  { kind: 'iterm2', matches: ['iterm2', 'iterm'] },
  { kind: 'cursor', matches: ['cursor'] },
  { kind: 'warp', matches: ['warp'] },
  { kind: 'vscode', matches: ['visual studio code', 'vscode', 'code.visualstudio'] },
  { kind: 'github', matches: ['github'] },
  { kind: 'chatgpt', matches: ['chatgpt', 'chat.openai', 'openai.com', 'codex'] },
  { kind: 'google-docs', matches: ['docs.google', 'google docs'] },
  { kind: 'youtube', matches: ['youtube', 'youtu.be'] },
  { kind: 'chrome', matches: ['google chrome', 'chrome'] },
  { kind: 'safari', matches: ['safari'] },
  { kind: 'messages', matches: ['imessage', 'messages'] },
  { kind: 'notes', matches: ['apple notes', 'icloud.com/notes', 'notes'] },
  { kind: 'terminal', matches: ['terminal', 'iterm', 'ghostty'] },
  { kind: 'xcode', matches: ['xcode'] },
  { kind: 'figma', matches: ['figma'] },
  { kind: 'slack', matches: ['slack'] },
  { kind: 'discord', matches: ['discord'] },
  { kind: 'notion', matches: ['notion'] },
]

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

/**
 * Extracts a display host only. The result is never turned into a request URL;
 * appSites is model-produced, untrusted activity metadata.
 * Following Dayflow's normalizedHost: single-word sites without a dot normalize to .com.
 */
export function displayHost(value: string): string | null {
  let trimmed = value.trim()
  if (trimmed === '' || /\s/.test(trimmed)) return null

  if (!trimmed.includes('.')) {
    trimmed = `${trimmed}.com`
  }

  try {
    const candidate = /^[a-z][a-z\d+.-]*:\/\//i.test(trimmed)
      ? trimmed
      : `https://${trimmed}`
    const host = new URL(candidate).hostname.toLowerCase().replace(/^www\./, '')
    return host || null
  } catch {
    return null
  }
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

export function resolveAppSiteIdentity(value: string): AppSiteIdentity {
  const label = nonEmpty(value) ?? '?'
  const host = displayHost(label)
  const lookup = `${label} ${host ?? ''}`.toLocaleLowerCase('en-US')
  const kind = brandRules.find((rule) => rule.matches.some((pattern) => lookup.includes(pattern)))
    ?.kind ?? 'generic'

  return {
    kind,
    label,
    host,
    monogram: monogramFor(label, host),
  }
}


// ---------------------------------------------------------------------------
// Installed-application icon resolution
// ---------------------------------------------------------------------------

/*
 * Card appSites are usually installed-application names ("Clash Verge",
 * "Microsoft Edge"). Those rarely have brand marks and rarely have hosts, but
 * the platform enumeration (the privacy grid's source) can supply the real
 * bundle icon offline. Match by normalized name tokens, best score wins.
 */

import { describeApplications, listInstalledApplications } from '@/api/application'
import type { ApplicationDTO } from '@/api/application'
import { i18n } from '@/i18n'

let installedAppsCache: ApplicationDTO[] | null = null

async function installedApps(): Promise<ApplicationDTO[]> {
  if (installedAppsCache !== null) return installedAppsCache
  try {
    // The listing resolves names in the UI language; icons are filled per id
    // by the describe cache afterwards.
    installedAppsCache = await listInstalledApplications(i18n.global.locale.value)
  } catch {
    installedAppsCache = []
  }
  return installedAppsCache
}

function normalizeName(value: string): string {
  return value.toLocaleLowerCase('en-US').replace(/[^\p{L}\p{N}]+/gu, ' ').trim()
}

function nameScore(site: string, appName: string): number {
  const siteTokens = normalizeName(site).split(' ').filter(Boolean)
  if (siteTokens.length === 0) return 0
  const appTokens = new Set(normalizeName(appName).split(' ').filter(Boolean))
  let hits = 0
  for (const token of siteTokens) {
    if (appTokens.has(token)) hits += 1
    else return 0
  }
  // Every site token matched; closer names score higher.
  return siteTokens.length / Math.max(1, appTokens.size) + 0.5
}

/**
 * Best-effort real icon for an installed application matching the site
 * string, or null when nothing matches well enough.
 */
export async function matchInstalledAppIcon(site: string): Promise<string | null> {
  const apps = await installedApps()
  if (apps.length === 0) return null

  let best: ApplicationDTO | null = null
  let bestScore = 0
  for (const app of apps) {
    const score = nameScore(site, app.name)
    if (score > bestScore) {
      bestScore = score
      best = app
    }
  }
  if (best === null || bestScore < 0.6) return null

  const described = await describeApplications([best.id])
  const icon = described.find((app) => app.id === best.id)?.iconDataUrl ?? ''
  return icon !== '' ? icon : null
}

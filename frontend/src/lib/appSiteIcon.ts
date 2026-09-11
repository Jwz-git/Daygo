import type { AppSitesDTO } from '@/api/dto'

export type AppSiteIconKind =
  | 'chatgpt'
  | 'chrome'
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
  | 'xcode'
  | 'youtube'
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

/**
 * Extracts a display host only. The result is never turned into a request URL;
 * appSites is model-produced, untrusted activity metadata.
 */
export function displayHost(value: string): string | null {
  const trimmed = value.trim()
  if (trimmed === '' || /\s/.test(trimmed)) return null

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

  const result: string[] = []
  const seen = new Set<string>()
  for (const candidate of [sites.primary, sites.secondary]) {
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

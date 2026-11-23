/*
 * Favicon resolution for timeline cards. The actual network fetch happens in Go
 * (internal/favicon), reached through the same-origin `/favicon?host=` asset
 * route. Go queries the site's origin alongside Google S2 and icon.horse,
 * rejects synthetic fallback placeholders, caches the result on disk with a
 * TTL, and honours the process proxy — which is what lets icons load where the
 * webview cannot reach Google directly.
 *
 * This module keeps only the thin client concerns: derive a canonical host
 * (explicit domain or curated known website, never blind `<word>.com` guesses
 * for unknown desktop apps), hit the same-origin resource, and cache the
 * decoded data URL for the session with in-flight de-duplication and a short
 * negative cache.
 *
 * Privacy boundary (docs/decisions/timeline-favicon-fetch.md): the only thing
 * that leaves the device is the hostname the card already displays — never
 * screen content. Go enforces the same boundary and validates the host.
 */

import { LruCache } from './lruCache'

const FETCH_TIMEOUT_MS = 8000
const NEGATIVE_TTL_MS = 10 * 60 * 1000

// Bounded so a long session that surfaces many distinct hosts can not grow the
// webview heap without limit; an evicted host simply re-resolves on next view.
const cache = new LruCache<string, string | null>(256, {
  maxWeight: 4 * 1024 * 1024,
  weigh: (value) => (value?.length ?? 0) * 2,
})
const negativeUntil = new LruCache<string, number>(256)
const inflight = new Map<string, Promise<string | null>>()

const NON_DOMAIN_TLDS = new Set([
  'exe',
  'dmg',
  'pkg',
  'msi',
  'lnk',
  'local',
  'internal',
  'localhost',
])

/*
 * Curated display-name -> canonical-host map for well-known websites and
 * web-backed tools. This replaces blind `<word>.com` guessing, which turned
 * desktop apps like "Preview" / "Mail" / "Obsidian" / "Linear" / "Telegram"
 * into unrelated or parked `.com` domains.
 */
const KNOWN_SITE_HOSTS: ReadonlyMap<string, string> = new Map([
  // Website & AI brands
  ['bilibili', 'bilibili.com'],
  ['哔哩哔哩', 'bilibili.com'],
  ['b站', 'bilibili.com'],
  ['google docs', 'docs.google.com'],
  ['google sheets', 'sheets.google.com'],
  ['google slides', 'slides.google.com'],
  ['google drive', 'drive.google.com'],
  ['google calendar', 'calendar.google.com'],
  ['gmail', 'mail.google.com'],
  ['notion', 'notion.so'],
  ['figma', 'figma.com'],
  ['slack', 'slack.com'],
  ['discord', 'discord.com'],
  ['github', 'github.com'],
  ['youtube', 'youtube.com'],
  ['chatgpt', 'chatgpt.com'],
  ['openai', 'chatgpt.com'],
  ['codex', 'chatgpt.com'],
  ['claude', 'claude.ai'],
  ['anthropic', 'claude.ai'],
  ['gemini', 'gemini.google.com'],
  ['google gemini', 'gemini.google.com'],
  ['google ai studio', 'aistudio.google.com'],
  ['ai studio', 'aistudio.google.com'],
  ['google studio', 'aistudio.google.com'],
  ['makersuite', 'aistudio.google.com'],

  // Non-.com & high-frequency websites / web-backed apps
  ['pinterest', 'pinterest.com'],
  ['obsidian', 'obsidian.md'],
  ['linear', 'linear.app'],
  ['telegram', 'telegram.org'],
  ['zed', 'zed.dev'],
  ['warp', 'warp.dev'],
  ['arc', 'arc.net'],
  ['cursor', 'cursor.com'],
  ['raycast', 'raycast.com'],
  ['zoom', 'zoom.us'],
  ['wikipedia', 'wikipedia.org'],
  ['维基百科', 'wikipedia.org'],
  ['twitter', 'x.com'],
  ['x', 'x.com'],
  ['reddit', 'reddit.com'],
  ['zhihu', 'zhihu.com'],
  ['知乎', 'zhihu.com'],
  ['xiaohongshu', 'xiaohongshu.com'],
  ['小红书', 'xiaohongshu.com'],
  ['juejin', 'juejin.cn'],
  ['掘金', 'juejin.cn'],
  ['yuque', 'yuque.com'],
  ['语雀', 'yuque.com'],
  ['feishu', 'feishu.cn'],
  ['飞书', 'feishu.cn'],
  ['lark', 'larksuite.com'],
  ['dingtalk', 'dingtalk.com'],
  ['钉钉', 'dingtalk.com'],
  ['weibo', 'weibo.com'],
  ['微博', 'weibo.com'],
  ['douban', 'douban.com'],
  ['豆瓣', 'douban.com'],
  ['douyin', 'douyin.com'],
  ['抖音', 'douyin.com'],
  ['taobao', 'taobao.com'],
  ['淘宝', 'taobao.com'],
  ['jd', 'jd.com'],
  ['京东', 'jd.com'],
  ['v2ex', 'v2ex.com'],
  ['csdn', 'csdn.net'],
  ['cnblogs', 'cnblogs.com'],
  ['博客园', 'cnblogs.com'],
  ['deepseek', 'deepseek.com'],
  ['doubao', 'doubao.com'],
  ['豆包', 'doubao.com'],
  ['kimi', 'kimi.moonshot.cn'],
  ['perplexity', 'perplexity.ai'],
  ['stackoverflow', 'stackoverflow.com'],
  ['stack overflow', 'stackoverflow.com'],
  ['hacker news', 'news.ycombinator.com'],
  ['hackernews', 'news.ycombinator.com'],
  ['medium', 'medium.com'],
  ['substack', 'substack.com'],
  ['vercel', 'vercel.com'],
  ['cloudflare', 'cloudflare.com'],
  ['gitlab', 'gitlab.com'],
  ['bitbucket', 'bitbucket.org'],
  ['huggingface', 'huggingface.co'],
  ['hugging face', 'huggingface.co'],
  ['npmjs', 'npmjs.com'],
  ['npm', 'npmjs.com'],
  ['pypi', 'pypi.org'],
  ['mdn', 'developer.mozilla.org'],
  ['netflix', 'netflix.com'],
  ['spotify', 'spotify.com'],
  ['twitch', 'twitch.tv'],
  ['instagram', 'instagram.com'],
  ['linkedin', 'linkedin.com'],
  ['facebook', 'facebook.com'],
  ['whatsapp', 'whatsapp.com'],
  ['dribbble', 'dribbble.com'],
  ['behance', 'behance.net'],
  ['dropbox', 'dropbox.com'],
  ['trello', 'trello.com'],
  ['jira', 'atlassian.com'],
  ['confluence', 'atlassian.com'],
  ['postman', 'postman.com'],
  ['docker', 'docker.com'],
  ['duckduckgo', 'duckduckgo.com'],
  ['google', 'google.com'],
  ['bing', 'bing.com'],
  ['baidu', 'baidu.com'],
  ['百度', 'baidu.com'],
  ['aliyun', 'aliyun.com'],
  ['阿里云', 'aliyun.com'],
  ['tencent cloud', 'cloud.tencent.com'],
  ['腾讯云', 'cloud.tencent.com'],
  ['aws', 'aws.amazon.com'],
  ['amazon', 'amazon.com'],
  ['apple', 'apple.com'],
  ['leetcode', 'leetcode.com'],
  ['力扣', 'leetcode.com'],
  ['excalidraw', 'excalidraw.com'],
  ['miro', 'miro.com'],
  ['canva', 'canva.com'],
  ['airtable', 'airtable.com'],
  ['supabase', 'supabase.com'],
  ['firebase', 'firebase.google.com'],
  ['stripe', 'stripe.com'],
])

function isIPv4Literal(host: string): boolean {
  return /^\d{1,3}(\.\d{1,3}){3}$/.test(host)
}

/**
 * Reports whether `site` looks like a macOS/Windows desktop bundle filename
 * (e.g. "Finder.app", "WeChat.app", "Code.exe") rather than a web domain.
 * Lowercase `.app` domains such as "linear.app" stay treated as web domains.
 */
export function looksLikeBundleFilename(site: string): boolean {
  const trimmed = site.trim()
  if (trimmed === '' || trimmed.includes('://') || trimmed.includes('/')) return false
  const lower = trimmed.toLocaleLowerCase('en-US')
  if (lower.endsWith('.exe') || lower.endsWith('.dmg') || lower.endsWith('.pkg')) return true
  if (lower.endsWith('.app') && /[A-Z]/.test(trimmed.slice(0, -4))) return true
  return false
}

/**
 * Extracts a bare hostname ONLY when `site` is explicitly a URL or dotted
 * domain (never guessing `.com` for bare words).
 */
export function extractExplicitHost(site: string): string | null {
  const trimmed = site.trim()
  if (trimmed === '' || /\s/.test(trimmed) || !trimmed.includes('.')) return null
  if (looksLikeBundleFilename(trimmed)) return null

  const candidate = /^[a-z][a-z\d+.-]*:\/\//i.test(trimmed)
    ? trimmed
    : `https://${trimmed}`
  try {
    const parsed = new URL(candidate)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null
    const host = parsed.hostname.toLowerCase().replace(/^www\./, '')
    if (!host || !host.includes('.')) return null
    if (host.startsWith('[') || isIPv4Literal(host)) return null
    const labels = host.split('.')
    const tld = labels[labels.length - 1] ?? ''
    if (tld.length < 2 || /\d/.test(tld) || NON_DOMAIN_TLDS.has(tld)) return null
    return host
  } catch {
    return null
  }
}

export function isExplicitDomain(site: string): boolean {
  return extractExplicitHost(site) !== null
}

/**
 * Resolves a canonical favicon host from an explicit URL/domain or a curated
 * known website name ("edge.com/x" → "edge.com", "Obsidian" → "obsidian.md",
 * "Google Docs" → "docs.google.com"). Unknown bare names ("Preview", "微信")
 * return null so desktop apps are not probed as unrelated `.com` domains.
 */
export function hostOf(site: string): string | null {
  const trimmed = site.trim()
  if (trimmed === '') return null
  const known = KNOWN_SITE_HOSTS.get(trimmed.toLocaleLowerCase('en-US'))
  if (known !== undefined) return known
  return extractExplicitHost(trimmed)
}

async function fetchFromResource(host: string): Promise<string | null> {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS)
  try {
    const response = await fetch(`/favicon?host=${encodeURIComponent(host)}`, {
      signal: controller.signal,
    })
    if (!response.ok) return null
    const blob = await response.blob()
    if (blob.size === 0) return null
    // Guard against an SPA/dev fallback (or any non-image response) becoming a
    // broken <img>: only real image bytes may pass to the icon.
    if (!blob.type.startsWith('image/')) return null
    return await new Promise<string | null>((resolve) => {
      const reader = new FileReader()
      reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : null)
      reader.onerror = () => resolve(null)
      reader.readAsDataURL(blob)
    })
  } catch {
    return null
  } finally {
    window.clearTimeout(timer)
  }
}

/**
 * Resolves a favicon data URL for the site, or null when the host yields
 * nothing. Concurrent calls for one host share a single request; failures are
 * negatively cached for ten minutes so a dead host is not re-requested.
 */
export function fetchFaviconDataUrl(site: string): Promise<string | null> {
  const host = hostOf(site)
  if (host === null) return Promise.resolve(null)

  const cached = cache.get(host)
  if (cached !== undefined) return Promise.resolve(cached)

  const blockedUntil = negativeUntil.get(host)
  if (blockedUntil !== undefined && blockedUntil > Date.now()) return Promise.resolve(null)

  const existing = inflight.get(host)
  if (existing) return existing

  const task = fetchFromResource(host)
    .then((dataUrl) => {
      cache.set(host, dataUrl)
      if (dataUrl === null) negativeUntil.set(host, Date.now() + NEGATIVE_TTL_MS)
      return dataUrl
    })
    .finally(() => {
      inflight.delete(host)
    })
  inflight.set(host, task)
  return task
}

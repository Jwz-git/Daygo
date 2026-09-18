/*
 * Favicon resolution for timeline cards, following Dayflow's FaviconService:
 * host-derived lookups against Google's public S2 favicon endpoint, with an
 * in-memory cache, in-flight de-duplication and a short negative cache.
 *
 * Privacy boundary (docs/decisions/timeline-favicon-fetch.md): the only thing
 * that leaves the device is the hostname the card already displays — never
 * screen content. Results live in memory for the session; nothing persists.
 */

const S2_ENDPOINT = 'https://www.google.com/s2/favicons?sz=64&domain='
const FETCH_TIMEOUT_MS = 5000
const NEGATIVE_TTL_MS = 10 * 60 * 1000

const cache = new Map<string, string | null>()
const negativeUntil = new Map<string, number>()
const inflight = new Map<string, Promise<string | null>>()

/** Extract the bare host from a raw site string ("edge.com/x" → "edge.com", "pinterest" → "pinterest.com"). */
export function hostOf(site: string): string | null {
  let trimmed = site.trim()
  if (trimmed === '' || /\s/.test(trimmed)) return null
  if (!trimmed.includes('.')) {
    trimmed = `${trimmed}.com`
  }
  const candidate = trimmed.includes('://') ? trimmed : `https://${trimmed}`
  try {
    return new URL(candidate).hostname.toLowerCase().replace(/^www\./, '') || null
  } catch {
    return null
  }
}

async function fetchBlobAsDataUrl(url: string, signal: AbortSignal): Promise<string | null> {
  try {
    const response = await fetch(url, {
      signal,
      credentials: 'omit',
      referrerPolicy: 'no-referrer',
    })
    if (!response.ok) return null
    const blob = await response.blob()
    if (blob.size === 0) return null
    return await new Promise<string | null>((resolve) => {
      const reader = new FileReader()
      reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : null)
      reader.onerror = () => resolve(null)
      reader.readAsDataURL(blob)
    })
  } catch {
    return null
  }
}

function loadFaviconUrl(url: string, timeoutMs: number): Promise<string | null> {
  if (typeof window === 'undefined' || typeof Image === 'undefined') {
    return Promise.resolve(null)
  }
  return new Promise((resolve) => {
    const img = new Image()
    let settled = false
    const timer = window.setTimeout(() => {
      if (settled) return
      settled = true
      img.src = ''
      resolve(null)
    }, timeoutMs)

    img.onload = () => {
      if (settled) return
      settled = true
      window.clearTimeout(timer)
      if (img.naturalWidth > 0 && img.naturalHeight > 0) {
        resolve(url)
      } else {
        resolve(null)
      }
    }
    img.onerror = () => {
      if (settled) return
      settled = true
      window.clearTimeout(timer)
      resolve(null)
    }
    img.src = url
  })
}

async function requestFavicon(host: string): Promise<string | null> {
  const timeoutMs = 2500
  const s2Url = `${S2_ENDPOINT}${encodeURIComponent(host)}`

  // 1. Browser Image probe: loads cross-origin images without CORS restrictions
  const imageResult = await loadFaviconUrl(s2Url, timeoutMs)
  if (imageResult !== null) return imageResult

  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), timeoutMs)
  try {
    // 2. Fetch blob fallback
    const s2Result = await fetchBlobAsDataUrl(s2Url, controller.signal)
    if (s2Result !== null) return s2Result

    // 3. Fallback: direct site /favicon.ico
    const directUrl = `https://${host}/favicon.ico`
    const directImage = await loadFaviconUrl(directUrl, timeoutMs)
    if (directImage !== null) return directImage

    return await fetchBlobAsDataUrl(directUrl, controller.signal)
  } finally {
    window.clearTimeout(timer)
  }
}

/**
 * Resolves a favicon data URL for the site, or null when the host yields
 * nothing. Concurrent calls for one host share a single request; failures are
 * negatively cached for ten minutes so a dead host is not hammered.
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

  const task = requestFavicon(host)
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

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

/** Extract the bare host from a raw site string ("edge.com/x" → "edge.com"). */
function hostOf(site: string): string | null {
  const trimmed = site.trim()
  if (trimmed === '') return null
  const candidate = trimmed.includes('://') ? trimmed : `https://${trimmed}`
  try {
    return new URL(candidate).hostname || null
  } catch {
    return null
  }
}

async function requestFavicon(host: string): Promise<string | null> {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS)
  try {
    const response = await fetch(`${S2_ENDPOINT}${encodeURIComponent(host)}`, {
      signal: controller.signal,
      // The default image accept header is all this endpoint needs; the host
      // is already in the URL, so no credentials or extra headers are sent.
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

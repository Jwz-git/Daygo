/*
 * Favicon resolution for timeline cards. The actual network fetch happens in Go
 * (internal/favicon), reached through the same-origin `/favicon?host=` asset
 * route. Go races Google's S2 aggregator against the site's own /favicon.ico,
 * caches the result on disk, and honours the process proxy — which is what lets
 * icons load where the webview cannot reach Google directly.
 *
 * This module keeps only the thin client concerns: derive the host, hit the
 * same-origin resource, and cache the decoded data URL for the session with
 * in-flight de-duplication and a short negative cache.
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
const cache = new LruCache<string, string | null>(256)
const negativeUntil = new LruCache<string, number>(256)
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

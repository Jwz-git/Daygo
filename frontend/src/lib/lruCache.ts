/**
 * Minimal insertion-order LRU over a Map. Bounds session-lifetime caches that
 * would otherwise grow unbounded (icon/favicon data URLs), so long uptime can
 * not inflate the webview JS heap. Reading a key refreshes its recency; setting
 * an over-capacity key evicts the least-recently-used entry.
 */
export class LruCache<K, V> {
  private readonly map = new Map<K, V>()

  constructor(private readonly capacity: number) {}

  get(key: K): V | undefined {
    const value = this.map.get(key)
    if (value !== undefined && this.map.delete(key)) {
      this.map.set(key, value)
    }
    return value
  }

  has(key: K): boolean {
    return this.map.has(key)
  }

  set(key: K, value: V): void {
    if (this.map.has(key)) {
      this.map.delete(key)
    } else if (this.map.size >= this.capacity) {
      const oldest = this.map.keys().next().value
      if (oldest !== undefined) this.map.delete(oldest)
    }
    this.map.set(key, value)
  }

  get size(): number {
    return this.map.size
  }
}

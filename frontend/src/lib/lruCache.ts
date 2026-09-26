export interface LruBudget<V> {
  maxWeight: number
  weigh: (value: V) => number
}

/** Entry-limited LRU with an optional payload budget; eviction only drops memoized data. */
export class LruCache<K, V> {
  private readonly map = new Map<K, { value: V; weight: number }>()
  private used = 0

  constructor(private readonly capacity: number, private readonly budget?: LruBudget<V>) {
    if (!Number.isInteger(capacity) || capacity < 1) throw new RangeError('invalid LRU capacity')
    if (budget && (!Number.isFinite(budget.maxWeight) || budget.maxWeight < 0)) {
      throw new RangeError('invalid LRU budget')
    }
  }

  get(key: K): V | undefined {
    const entry = this.map.get(key)
    if (entry !== undefined) {
      this.map.delete(key)
      this.map.set(key, entry)
    }
    return entry?.value
  }

  has(key: K): boolean { return this.map.has(key) }

  set(key: K, value: V): void {
    const weight = this.budget?.weigh(value) ?? 0
    if (!Number.isFinite(weight) || weight < 0) throw new RangeError('invalid LRU weight')
    this.delete(key)
    if (this.budget && weight > this.budget.maxWeight) return
    this.map.set(key, { value, weight })
    this.used += weight
    while (this.map.size > this.capacity || (this.budget && this.used > this.budget.maxWeight)) {
      const oldest = this.map.keys().next()
      if (oldest.done) break
      this.delete(oldest.value)
    }
  }

  delete(key: K): boolean {
    const entry = this.map.get(key)
    if (entry === undefined) return false
    this.used -= entry.weight
    return this.map.delete(key)
  }

  clear(): void { this.map.clear(); this.used = 0 }
  get size(): number { return this.map.size }
  get weight(): number { return this.used }
}

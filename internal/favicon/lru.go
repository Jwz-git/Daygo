package favicon

import "container/list"

// lruCache is serialized by Resolver.mu. Entry and optional payload budgets
// bound retention; eviction never affects the disk source of truth.
type lruCache[V any] struct {
	cap    int
	budget int
	used   int
	weigh  func(V) int
	ll     *list.List
	items  map[string]*list.Element
}
type lruEntry[V any] struct {
	key    string
	value  V
	weight int
}

func newLRUCache[V any](capacity int) *lruCache[V] {
	return newWeightedLRUCache[V](capacity, 0, nil)
}
func newWeightedLRUCache[V any](capacity, budget int, weigh func(V) int) *lruCache[V] {
	if capacity < 1 {
		capacity = 1
	}
	return &lruCache[V]{cap: capacity, budget: budget, weigh: weigh, ll: list.New(), items: make(map[string]*list.Element)}
}
func (c *lruCache[V]) get(key string) (V, bool) {
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*lruEntry[V]).value, true
	}
	var zero V
	return zero, false
}
func (c *lruCache[V]) put(key string, value V) {
	weight := 0
	if c.weigh != nil {
		weight = c.weigh(value)
	}
	c.delete(key)
	// A caller can still use an oversized result without retaining it here.
	if weight < 0 || (c.budget > 0 && weight > c.budget) {
		return
	}
	c.items[key] = c.ll.PushFront(&lruEntry[V]{key: key, value: value, weight: weight})
	c.used += weight
	for c.ll.Len() > c.cap || (c.budget > 0 && c.used > c.budget) {
		c.delete(c.ll.Back().Value.(*lruEntry[V]).key)
	}
}
func (c *lruCache[V]) delete(key string) {
	if el, ok := c.items[key]; ok {
		c.used -= el.Value.(*lruEntry[V]).weight
		c.ll.Remove(el)
		delete(c.items, key)
	}
}
func (c *lruCache[V]) len() int    { return c.ll.Len() }
func (c *lruCache[V]) weight() int { return c.used }

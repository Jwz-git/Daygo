package favicon

import "container/list"

// lruCache is a fixed-capacity, least-recently-used cache keyed by string. It
// is NOT safe for concurrent use on its own: the Resolver serializes every call
// under its own mutex, so adding a second lock here would be dead weight.
//
// It exists so the resolver's session caches are bounded. Without a cap a
// resident agent accumulates one entry per distinct host it has ever seen, for
// the whole process lifetime — the memory-growth this package was leaking.
type lruCache[V any] struct {
	cap   int
	ll    *list.List // front = most recently used, back = eviction candidate
	items map[string]*list.Element
}

type lruEntry[V any] struct {
	key   string
	value V
}

func newLRUCache[V any](capacity int) *lruCache[V] {
	if capacity < 1 {
		capacity = 1
	}
	return &lruCache[V]{
		cap:   capacity,
		ll:    list.New(),
		items: make(map[string]*list.Element),
	}
}

// get returns the value for key and, on a hit, marks it most-recently-used.
func (c *lruCache[V]) get(key string) (V, bool) {
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*lruEntry[V]).value, true
	}
	var zero V
	return zero, false
}

// put inserts or updates key, evicting the least-recently-used entry when the
// insert would exceed the capacity.
func (c *lruCache[V]) put(key string, value V) {
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*lruEntry[V]).value = value
		return
	}
	c.items[key] = c.ll.PushFront(&lruEntry[V]{key: key, value: value})
	if c.ll.Len() > c.cap {
		if oldest := c.ll.Back(); oldest != nil {
			c.ll.Remove(oldest)
			delete(c.items, oldest.Value.(*lruEntry[V]).key)
		}
	}
}

func (c *lruCache[V]) delete(key string) {
	if el, ok := c.items[key]; ok {
		c.ll.Remove(el)
		delete(c.items, key)
	}
}

func (c *lruCache[V]) len() int { return c.ll.Len() }

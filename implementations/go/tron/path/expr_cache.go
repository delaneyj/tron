package path

import (
	"container/list"
	"sync"
)

const defaultCompileCacheSize = 256

type exprCache struct {
	mu         sync.Mutex
	maxEntries int
	ll         *list.List
	cache      map[string]*list.Element
}

type cacheEntry struct {
	key  string
	expr *Expr
}

func newExprCache(maxEntries int) *exprCache {
	if maxEntries < 0 {
		maxEntries = 0
	}
	return &exprCache{
		maxEntries: maxEntries,
		ll:         list.New(),
		cache:      make(map[string]*list.Element),
	}
}

func (c *exprCache) get(key string) (*Expr, bool) {
	if c == nil || c.maxEntries == 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		return ele.Value.(*cacheEntry).expr, true
	}
	return nil, false
}

func (c *exprCache) add(key string, expr *Expr) {
	if c == nil || c.maxEntries == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ele, ok := c.cache[key]; ok {
		ele.Value.(*cacheEntry).expr = expr
		c.ll.MoveToFront(ele)
		return
	}
	ele := c.ll.PushFront(&cacheEntry{key: key, expr: expr})
	c.cache[key] = ele
	if c.maxEntries > 0 && c.ll.Len() > c.maxEntries {
		c.removeOldest()
	}
}

func (c *exprCache) setMax(maxEntries int) {
	if c == nil {
		return
	}
	if maxEntries < 0 {
		maxEntries = 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxEntries = maxEntries
	if maxEntries == 0 {
		c.ll.Init()
		clear(c.cache)
		return
	}
	for c.ll.Len() > maxEntries {
		c.removeOldest()
	}
}

func (c *exprCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ll.Init()
	clear(c.cache)
}

func (c *exprCache) removeOldest() {
	ele := c.ll.Back()
	if ele == nil {
		return
	}
	c.ll.Remove(ele)
	entry := ele.Value.(*cacheEntry)
	delete(c.cache, entry.key)
}

var compileCache = newExprCache(defaultCompileCacheSize)

// SetCompileCacheSize sets the maximum number of compiled expressions to cache.
// Set to 0 to disable caching.
func SetCompileCacheSize(maxEntries int) {
	compileCache.setMax(maxEntries)
}

// ClearCompileCache drops all cached expressions.
func ClearCompileCache() {
	compileCache.clear()
}

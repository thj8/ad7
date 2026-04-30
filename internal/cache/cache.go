// Package cache 提供基于泛型的内存 TTL 缓存。
// 支持 Get 时懒淘汰和可选的后台定期清理。
package cache

import (
	"strings"
	"sync"
	"time"
)

// entry 是缓存中的一个条目，包含值和过期时间。
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// expired 判断条目是否已过期。
func (e entry[V]) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && now.After(e.expiresAt)
}

// Cache 是基于泛型的内存 TTL 缓存，并发安全。
type Cache[V any] struct {
	mu              sync.RWMutex
	items           map[string]entry[V]
	defaultTTL      time.Duration
	stopCh          chan struct{}
	stopped         bool
	cleanupInterval time.Duration
}

// Options 是创建 Cache 的可选配置。
type Options struct {
	// DefaultTTL 默认缓存条目生存时间，0 表示永不过期。
	DefaultTTL time.Duration
	// CleanupInterval 后台清理间隔，0 表示不启动后台清理。
	CleanupInterval time.Duration
}

// New 创建一个新的 Cache 实例。
func New[V any](opts Options) *Cache[V] {
	c := &Cache[V]{
		items:           make(map[string]entry[V]),
		defaultTTL:      opts.DefaultTTL,
		cleanupInterval: opts.CleanupInterval,
		stopCh:          make(chan struct{}),
	}
	if opts.CleanupInterval > 0 {
		go c.cleanupLoop(opts.CleanupInterval)
	}
	return c
}

// Get 从缓存中获取值。如果 key 不存在或已过期，返回零值和 false。
// 过期条目会被懒删除。
func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	if e.expired(time.Now()) {
		// 升级为写锁删除过期条目
		c.mu.Lock()
		// 双重检查：避免在锁升级期间被其他 goroutine 删除后又被 Set
		if e2, ok2 := c.items[key]; ok2 && e2.expired(time.Now()) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		var zero V
		return zero, false
	}

	return e.value, true
}

// Set 设置缓存条目，使用默认 TTL。
func (c *Cache[V]) Set(key string, value V) {
	c.SetWithTTL(key, value, c.defaultTTL)
}

// SetWithTTL 设置缓存条目，使用自定义 TTL。
func (c *Cache[V]) SetWithTTL(key string, value V, ttl time.Duration) {
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	c.mu.Lock()
	c.items[key] = entry[V]{value: value, expiresAt: expiresAt}
	c.mu.Unlock()
}

// GetOrSet 获取缓存值，如果不存在则调用 fn 获取并存入缓存。
// 使用默认 TTL。
func (c *Cache[V]) GetOrSet(key string, fn func() (V, error)) (V, error) {
	return c.GetOrSetWithTTL(key, fn, c.defaultTTL)
}

// GetOrSetWithTTL 获取缓存值，如果不存在则调用 fn 获取并存入缓存。
// 使用自定义 TTL。
func (c *Cache[V]) GetOrSetWithTTL(key string, fn func() (V, error), ttl time.Duration) (V, error) {
	// 快速路径：读锁检查缓存
	if v, ok := c.Get(key); ok {
		return v, nil
	}

	// 慢速路径：调用 fn 获取值
	v, err := fn()
	if err != nil {
		var zero V
		return zero, err
	}

	// 存入缓存
	c.SetWithTTL(key, v, ttl)
	return v, nil
}

// Delete 删除指定 key 的缓存条目。
func (c *Cache[V]) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// DeleteByPrefix 删除所有匹配前缀的缓存条目。
func (c *Cache[V]) DeleteByPrefix(prefix string) {
	c.mu.Lock()
	for k := range c.items {
		if strings.HasPrefix(k, prefix) {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}

// Len 返回缓存中的条目数量（包含可能过期但尚未清理的条目）。
func (c *Cache[V]) Len() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Stop 停止后台清理 goroutine。之后不可再调用 StartCleanup。
func (c *Cache[V]) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		c.stopped = true
		close(c.stopCh)
	}
}

// cleanupLoop 定期清理过期条目。
func (c *Cache[V]) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.removeExpired()
		}
	}
}

// removeExpired 删除所有过期条目。
func (c *Cache[V]) removeExpired() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.items {
		if e.expired(now) {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}

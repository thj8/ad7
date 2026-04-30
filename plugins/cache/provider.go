package cache

import (
	"ad7/internal/cache"
	"ad7/internal/pluginutil"
)

// Ensure cacheProvider implements pluginutil.CacheManager
var _ pluginutil.CacheManager = &cacheProvider{}

// cacheProvider 是 pluginutil.CacheManager 的实现
type cacheProvider struct {
	cache *cache.Cache[any]
}

func newCacheProvider(c *cache.Cache[any]) *cacheProvider {
	return &cacheProvider{cache: c}
}

func (p *cacheProvider) Get(key string) (any, bool) {
	return p.cache.Get(key)
}

func (p *cacheProvider) Set(key string, value any) {
	p.cache.Set(key, value)
}

func (p *cacheProvider) Delete(key string) {
	p.cache.Delete(key)
}

func (p *cacheProvider) DeleteByPrefix(prefix string) {
	p.cache.DeleteByPrefix(prefix)
}

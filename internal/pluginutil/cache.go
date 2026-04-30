package pluginutil

// CacheProvider 定义缓存提供器的接口，供消费者使用
type CacheProvider interface {
	Get(key string) (any, bool)
	Set(key string, value any)
}

// CacheManager 定义缓存管理的接口，供缓存插件使用
type CacheManager interface {
	CacheProvider
	Delete(key string)
	DeleteByPrefix(prefix string)
}

// NoOpProvider 是无缓存实现，返回 (nil, false) 并忽略 Set/Delete
type NoOpProvider struct{}

func (n NoOpProvider) Get(key string) (any, bool) {
	return nil, false
}

func (n NoOpProvider) Set(key string, value any) {
}

func (n NoOpProvider) Delete(key string) {
}

func (n NoOpProvider) DeleteByPrefix(prefix string) {
}

// WithCache 通用缓存辅助函数
// 如果缓存中有数据就直接返回，否则调用 fn 获取数据并缓存
func WithCache(cache CacheProvider, key string, fn func() (any, error)) (any, error) {
	if cache != nil {
		if cached, ok := cache.Get(key); ok {
			return cached, nil
		}
	}
	result, err := fn()
	if err == nil && cache != nil {
		cache.Set(key, result)
	}
	return result, err
}

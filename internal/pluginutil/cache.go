package pluginutil

// CacheProvider 定义缓存提供器的接口
type CacheProvider interface {
	Get(key string) (any, bool)
	Set(key string, value any)
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

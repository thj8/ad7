// Package cache 提供可插拔的缓存插件，为其他插件和中间件提供缓存能力。
package cache

import (
	"ad7/internal/cache"
)

// Provider 是缓存提供器接口，暴露给其他插件使用。
// 其他插件通过此接口使用缓存，而不直接依赖具体实现。
type Provider interface {
	// Get 获取缓存值，不存在返回 (zero, false)
	Get(key string) (any, bool)

	// Set 设置缓存值，使用默认 TTL
	Set(key string, value any)

	// SetWithTTL 设置缓存值，使用自定义 TTL
	SetWithTTL(key string, value any, ttl any)

	// Delete 删除缓存值
	Delete(key string)

	// InvalidateByCompetition 清除指定比赛的所有相关缓存
	InvalidateByCompetition(compID string)

	// InvalidateByChallenge 清除指定题目的所有相关缓存
	InvalidateByChallenge(compID, chalID string)
}

// cacheProvider 是 Provider 的实现。
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

func (p *cacheProvider) SetWithTTL(key string, value any, _ any) {
	// 简化实现，暂不支持单独条目自定义 TTL
	p.cache.Set(key, value)
}

func (p *cacheProvider) Delete(key string) {
	p.cache.Delete(key)
}

func (p *cacheProvider) InvalidateByCompetition(compID string) {
	// 清除比赛相关缓存的模式匹配（这里简化为删除已知前缀的 key）
	// 实际实现可能需要更复杂的索引机制
	p.Delete("leaderboard:" + compID)
	p.Delete("topthree:" + compID)
}

func (p *cacheProvider) InvalidateByChallenge(compID, chalID string) {
	p.Delete("topthree:" + compID + ":" + chalID)
}

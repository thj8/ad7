// Package cache 提供可插拔的缓存插件，为其他插件和中间件提供缓存能力。
package cache

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/cache"
	"ad7/internal/event"
	"ad7/internal/middleware"
	"ad7/internal/plugin"
)

// topThreeProvider 是本地接口，只定义我们需要的方法
type topThreeProvider interface {
	IsTopThreeFull(ctx context.Context, compID, chalID string) bool
}

// Plugin 是缓存插件，提供通用缓存能力并处理缓存失效。
type Plugin struct {
	db        *sql.DB
	cache     *cache.Cache[any]
	provider  Provider
	authCache *cache.Cache[middleware.CachedToken]
	topThree  topThreeProvider
}

// New 创建缓存插件实例。
func New() *Plugin {
	return &Plugin{}
}

// Name 返回插件名称。
func (p *Plugin) Name() string {
	return plugin.NameCache
}

// Get 获取缓存值（实现 Provider 接口）。
func (p *Plugin) Get(key string) (any, bool) {
	if p.provider != nil {
		return p.provider.Get(key)
	}
	return nil, false
}

// Set 设置缓存值（实现 Provider 接口）。
func (p *Plugin) Set(key string, value any) {
	if p.provider != nil {
		p.provider.Set(key, value)
	}
}

// SetWithTTL 设置缓存值（实现 Provider 接口）。
func (p *Plugin) SetWithTTL(key string, value any, ttl any) {
	if p.provider != nil {
		p.provider.SetWithTTL(key, value, ttl)
	}
}

// Delete 删除缓存值（实现 Provider 接口）。
func (p *Plugin) Delete(key string) {
	if p.provider != nil {
		p.provider.Delete(key)
	}
}

// InvalidateByCompetition 清除比赛缓存（实现 Provider 接口）。
func (p *Plugin) InvalidateByCompetition(compID string) {
	if p.provider != nil {
		p.provider.InvalidateByCompetition(compID)
	}
}

// InvalidateByChallenge 清除题目缓存（实现 Provider 接口）。
func (p *Plugin) InvalidateByChallenge(compID, chalID string) {
	if p.provider != nil {
		p.provider.InvalidateByChallenge(compID, chalID)
	}
}

// Register 注册缓存插件，初始化缓存并订阅事件。
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db

	// 初始化通用缓存（5分钟 TTL，10分钟清理间隔）
	p.cache = cache.New[any](cache.Options{
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 10 * time.Minute,
	})

	// 初始化 token 缓存（5分钟 TTL）
	p.authCache = cache.New[middleware.CachedToken](cache.Options{
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 10 * time.Minute,
	})

	p.provider = newCacheProvider(p.cache)

	// 从依赖中获取 topthree 插件
	if topThreePlugin, ok := deps[plugin.NameTopThree]; ok {
		if provider, ok := topThreePlugin.(topThreeProvider); ok {
			p.topThree = provider
		}
	}

	// 订阅正确提交事件，用于清除相关缓存
	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	// 设置缓存到 auth 中间件
	if auth != nil {
		type cacheSetter interface {
			SetCache(*cache.Cache[middleware.CachedToken])
		}
		if setter, ok := any(auth).(cacheSetter); ok {
			setter.SetCache(p.authCache)
		}
	}
}

// GetProvider 返回缓存提供器，供其他插件使用。
func (p *Plugin) GetProvider() Provider {
	return p.provider
}

// GetAuthCache 返回 token 缓存，供 auth 中间件使用。
func (p *Plugin) GetAuthCache() *cache.Cache[middleware.CachedToken] {
	return p.authCache
}

// handleCorrectSubmission 处理正确提交事件，清除相关缓存。
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	compID := e.CompetitionID
	chalID := e.ChallengeID

	// 清除比赛的排行榜缓存
	p.Delete("leaderboard:" + compID)

	// 清除分析缓存
	p.Delete("analytics:overview:" + compID)
	p.Delete("analytics:categories:" + compID)
	p.Delete("analytics:users:" + compID)
	p.Delete("analytics:challenges:" + compID)

	// 检查该题目的 top3 是否已填满（3项），如果未满才清除缓存
	ctx := context.Background()
	if p.topThree != nil {
		if !p.topThree.IsTopThreeFull(ctx, compID, chalID) {
			p.Delete("topthree:" + compID)
			p.Delete("topthree:" + compID + ":" + chalID)
			p.Delete("topthree:" + compID + ":map")
		}
	} else {
		// 如果没有 topthree 插件，保守清除缓存
		p.Delete("topthree:" + compID)
		p.Delete("topthree:" + compID + ":" + chalID)
		p.Delete("topthree:" + compID + ":map")
	}
}

// Stop 停止缓存后台清理。
func (p *Plugin) Stop() {
	if p.cache != nil {
		p.cache.Stop()
	}
	if p.authCache != nil {
		p.authCache.Stop()
	}
}

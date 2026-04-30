// Package cache 提供可插拔的缓存插件，为其他插件和中间件提供缓存能力。
package cache

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-chi/chi/v5"

	"ad7/internal/cache"
	"ad7/internal/config"
	"ad7/internal/event"
	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/pluginutil"
)

// topThreeProvider 是本地接口，只定义我们需要的方法
type topThreeProvider interface {
	IsTopThreeFull(ctx context.Context, compID, chalID string) bool
}

// Plugin 是缓存插件，提供通用缓存能力并处理缓存失效。
type Plugin struct {
	db        *sql.DB
	cache     *cache.Cache[any]
	manager   pluginutil.CacheManager
	authCache *cache.Cache[middleware.CachedToken]
	topThree  topThreeProvider
	enabled   bool
	modules   map[string]bool
}

// New 创建缓存插件实例。
func New(cfg config.CacheConfig) *Plugin {
	return &Plugin{
		enabled: cfg.Enabled,
		modules: cfg.Modules,
	}
}

// Name 返回插件名称。
func (p *Plugin) Name() string {
	return plugin.NameCache
}

// GetProvider 返回指定模块的缓存提供器。
func (p *Plugin) GetProvider(module string) pluginutil.CacheProvider {
	if !p.enabled {
		return pluginutil.NoOpProvider{}
	}
	if p.modules != nil {
		if !p.modules[module] {
			return pluginutil.NoOpProvider{}
		}
	}
	return p.manager
}

// isModuleEnabled 检查模块是否启用（内部使用）。
func (p *Plugin) isModuleEnabled(module string) bool {
	if !p.enabled {
		return false
	}
	if p.modules == nil {
		return true
	}
	return p.modules[module]
}

// Register 注册缓存插件，初始化缓存并订阅事件。
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]plugin.Plugin) {
	p.db = db

	if p.enabled {
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

		p.manager = newCacheProvider(p.cache)
	}

	// 从依赖中获取 topthree 插件
	if topThreePlugin, ok := deps[plugin.NameTopThree]; ok {
		if provider, ok := topThreePlugin.(topThreeProvider); ok {
			p.topThree = provider
		}
	}

	// 订阅正确提交事件，用于清除相关缓存
	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	// 设置缓存到 auth 中间件（仅当 auth 模块启用）
	if auth != nil && p.isModuleEnabled("auth") {
		type cacheSetter interface {
			SetCache(*cache.Cache[middleware.CachedToken])
		}
		if setter, ok := any(auth).(cacheSetter); ok {
			setter.SetCache(p.authCache)
		}
	}
}

// GetAuthCache 返回 token 缓存，供 auth 中间件使用（保留用于向后兼容）。
func (p *Plugin) GetAuthCache() *cache.Cache[middleware.CachedToken] {
	return p.authCache
}

// handleCorrectSubmission 处理正确提交事件，清除相关缓存。
func (p *Plugin) handleCorrectSubmission(e event.Event) {
	if !p.enabled {
		return
	}

	compID := e.CompetitionID
	chalID := e.ChallengeID

	// 前缀批量清除
	p.manager.DeleteByPrefix("leaderboard:" + compID)
	p.manager.DeleteByPrefix("analytics:" + compID + ":")

	// 检查该题目的 top3 是否已填满（3项），如果未满才清除缓存
	ctx := context.Background()
	if p.topThree != nil {
		if !p.topThree.IsTopThreeFull(ctx, compID, chalID) {
			p.manager.DeleteByPrefix("topthree:" + compID)
		}
	} else {
		// 如果没有 topthree 插件，保守清除缓存
		p.manager.DeleteByPrefix("topthree:" + compID)
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

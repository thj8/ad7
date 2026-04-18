// Package dashboard 实现比赛仪表盘插件。
// 提供比赛状态总览（比赛信息、题目状态、排行榜、统计、最近事件）和一血追踪功能。
// 通过订阅事件系统实时追踪正确提交，维护内存中的最近事件列表。
package dashboard

import (
	"database/sql"
	"sync"

	"github.com/go-chi/chi/v5"

	"ad7/internal/event"
	"ad7/internal/middleware"
)

// Plugin 是仪表盘插件，持有数据库连接和内存中的最近事件列表。
type Plugin struct {
	db           *sql.DB        // 数据库连接
	recentEvents []recentEvent  // 最近事件列表（内存缓存，最多 100 条）
	mu           sync.RWMutex   // 保护 recentEvents 的并发读写
}

// New 创建仪表盘插件实例，初始化最近事件列表（容量 100）。
func New() *Plugin {
	return &Plugin{
		recentEvents: make([]recentEvent, 0, 100),
	}
}

// Register 注册仪表盘路由并订阅正确提交事件。
// 路由：
//   - GET /api/v1/dashboard/competitions/{id}/state（获取比赛状态总览，无需认证）
//   - GET /api/v1/dashboard/competitions/{id}/firstblood（获取一血列表，无需认证）
//
// 同时订阅 EventCorrectSubmission 事件，用于实时追踪一血和解题动态。
func (p *Plugin) Register(r chi.Router, db *sql.DB, auth *middleware.Auth) {
	p.db = db

	// 订阅正确提交事件，触发一血检测
	event.Subscribe(event.EventCorrectSubmission, p.handleCorrectSubmission)

	// 注册无需认证的仪表盘路由（查看类接口）
	r.Get("/api/v1/dashboard/competitions/{id}/state", p.getState)
	r.Get("/api/v1/dashboard/competitions/{id}/firstblood", p.getFirstBlood)
}

// addRecentEvent 在内存中添加一个最近事件。
// 新事件添加到列表头部，最多保留 100 条。
func (p *Plugin) addRecentEvent(e recentEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 将新事件插入到列表头部
	p.recentEvents = append([]recentEvent{e}, p.recentEvents...)
	// 限制列表长度为 100
	if len(p.recentEvents) > 100 {
		p.recentEvents = p.recentEvents[:100]
	}
}

// getRecentEvents 获取内存中所有最近事件的副本。
// 返回副本以避免外部修改影响内部状态。
func (p *Plugin) getRecentEvents() []recentEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]recentEvent, len(p.recentEvents))
	copy(result, p.recentEvents)
	return result
}

// Package plugin 定义插件系统的核心接口。
// 所有插件（排行榜、通知、提示、仪表盘、分析等）都实现此接口，
// 在服务启动时统一注册路由和接收依赖。
package plugin

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

// Plugin 是所有插件必须实现的接口。
// Register 方法在服务启动时被调用，插件在此方法中：
//   - 保存数据库连接和认证中间件的引用
//   - 在 chi 路由器上注册自己的路由
//
// 参数：
//   - r: chi 路由器，用于注册路由
//   - db: 数据库连接，供插件查询自己的表
//   - auth: 认证中间件，用于保护插件路由
//   - deps: 已初始化的依赖插件，key 是插件名称
type Plugin interface {
	// Name 返回插件的唯一名称，用于依赖管理
	Name() string

	// Register 方法在服务启动时被调用
	Register(r chi.Router, db *sql.DB, auth *middleware.Auth, deps map[string]Plugin)
}

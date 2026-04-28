// Package config 提供 YAML 配置文件的加载与解析功能。
// 配置分为三部分：服务器端口、数据库连接、JWT 认证参数。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是顶层配置结构体，包含服务器、数据库和 JWT 三个子配置。
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	DB        DBConfig        `yaml:"db"`
	JWT       JWTConfig       `yaml:"jwt"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Log       LogConfig       `yaml:"log"`
	Auth      AuthConfig      `yaml:"auth"`
}

// ServerConfig 定义 HTTP 服务器的监听端口。
type ServerConfig struct {
	Port int `yaml:"port"`
}

// DBConfig 定义 MySQL 数据库连接参数。
type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// DSN 根据数据库配置生成 MySQL 数据源名称（Data Source Name）。
// 返回格式：user:password@tcp(host:port)/dbname?parseTime=true
func (d *DBConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		d.User, d.Password, d.Host, d.Port, d.DBName)
}

// JWTConfig 定义 JWT 认证所需的密钥和管理员角色名。
type JWTConfig struct {
	Secret    string `yaml:"secret"`
	AdminRole string `yaml:"admin_role"`
}

// RateLimitRule 定义限流规则，包含时间窗口内的最大请求数。
type RateLimitRule struct {
	Requests int           `yaml:"requests"` // 最大请求数
	Window   time.Duration `yaml:"window"`   // 限流时间窗口
}

// RateLimitConfig 包含各端点的限流配置。
type RateLimitConfig struct {
	Submission RateLimitRule `yaml:"submission"` // Flag 提交限流规则
	Auth       RateLimitRule `yaml:"auth"`      // 认证端点限流规则（注册/登录）
}

// LogConfig 定义日志输出配置。
type LogConfig struct {
	Path  string `yaml:"path"`  // 日志文件路径，空则仅输出到 stdout
	Level string `yaml:"level"` // 日志级别：debug / info / warn / error
}

// AuthConfig 定义认证服务的连接参数。
type AuthConfig struct {
	URL string `yaml:"url"` // 认证服务地址，如 "http://localhost:8081"
}

// Load 从指定路径读取 YAML 配置文件并解析为 Config 结构体。
// 参数：
//   - path: 配置文件的文件系统路径
//
// 返回：
//   - *Config: 解析后的配置对象
//   - error: 文件读取或 YAML 解析失败时返回错误
//
// 默认值：如果 Server.Port 未设置，默认 8080；如果 JWT.AdminRole 未设置，默认 "admin"。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// 设置默认端口
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	// 设置默认管理员角色名
	if cfg.JWT.AdminRole == "" {
		cfg.JWT.AdminRole = "admin"
	}
	// 设置默认提交限流：10 秒内最多 3 次请求
	if cfg.RateLimit.Submission.Requests == 0 {
		cfg.RateLimit.Submission.Requests = 3
	}
	if cfg.RateLimit.Submission.Window == 0 {
		cfg.RateLimit.Submission.Window = 10 * time.Second
	}
	// 设置默认认证限流：1 分钟内最多 10 次请求
	if cfg.RateLimit.Auth.Requests == 0 {
		cfg.RateLimit.Auth.Requests = 10
	}
	if cfg.RateLimit.Auth.Window == 0 {
		cfg.RateLimit.Auth.Window = 1 * time.Minute
	}
	// 设置默认日志级别
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	// 设置默认认证服务地址
	if cfg.Auth.URL == "" {
		cfg.Auth.URL = "http://localhost:8081"
	}
	// 验证必填字段
	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("jwt.secret is required")
	}
	if len(cfg.JWT.Secret) < 32 {
		return nil, fmt.Errorf("jwt.secret must be at least 32 characters")
	}
	defaultSecrets := map[string]bool{
		"change-me-in-production": true,
		"secret":                  true,
		"jwt-secret":              true,
		"your-secret-key":         true,
	}
	if defaultSecrets[cfg.JWT.Secret] {
		return nil, fmt.Errorf("jwt.secret must not be a known default value")
	}
	if cfg.DB.Host == "" {
		return nil, fmt.Errorf("db.host is required")
	}
	return &cfg, nil
}

package auth

import (
	"fmt"
	"os"
	"time"

	"ad7/internal/config"

	"gopkg.in/yaml.v3"
)

// AuthServerConfig 是认证服务的顶层配置结构体，只包含认证服务需要的字段。
type AuthServerConfig struct {
	Server    config.ServerConfig    `yaml:"server"`
	DB        config.DBConfig        `yaml:"db"`
	JWT       config.JWTConfig       `yaml:"jwt"`
	Log       config.LogConfig       `yaml:"log"`
	RateLimit AuthRateLimitConfig    `yaml:"rate_limit"`
}

// AuthRateLimitConfig 包含认证端点的限流配置。
type AuthRateLimitConfig struct {
	Auth config.RateLimitRule `yaml:"auth"` // 注册/登录限流规则
}

// LoadAuthConfig 从指定路径读取 YAML 配置文件并解析为 AuthServerConfig。
func LoadAuthConfig(path string) (*AuthServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg AuthServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// 设置默认端口
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8081
	}
	// 设置默认管理员角色名
	if cfg.JWT.AdminRole == "" {
		cfg.JWT.AdminRole = "admin"
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
	// 验证必填字段
	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("jwt.secret is required")
	}
	if len(cfg.JWT.Secret) < 32 {
		return nil, fmt.Errorf("jwt.secret must be at least 32 characters")
	}
	if cfg.DB.Host == "" {
		return nil, fmt.Errorf("db.host is required")
	}
	return &cfg, nil
}

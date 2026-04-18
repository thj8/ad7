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

// RateLimitRule defines a rate limit rule with requests per time window.
type RateLimitRule struct {
	Requests int           `yaml:"requests"` // Maximum number of requests
	Window   time.Duration `yaml:"window"`   // Time window for the limit
}

// RateLimitConfig contains rate limit configurations for different endpoints.
type RateLimitConfig struct {
	Submission RateLimitRule `yaml:"submission"` // Rate limit for flag submissions
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
	// Set default rate limit for submissions: 3 requests per 10 seconds
	if cfg.RateLimit.Submission.Requests == 0 {
		cfg.RateLimit.Submission.Requests = 3
	}
	if cfg.RateLimit.Submission.Window == 0 {
		cfg.RateLimit.Submission.Window = 10 * time.Second
	}
	return &cfg, nil
}

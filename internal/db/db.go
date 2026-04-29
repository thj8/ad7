// Package db 提供数据库连接管理。
// 仅负责创建、关闭和暴露 *sql.DB 连接，不包含任何业务逻辑。
package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// Connect 连接到 MySQL 数据库并验证连接可用性。
// 参数 dsn 为 MySQL 数据源名称，格式：user:password@tcp(host:port)/dbname?parseTime=true
func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return db, nil
}

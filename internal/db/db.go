// Package db 提供数据库连接管理。
// 仅负责创建、关闭和暴露 *sql.DB 连接，不包含任何业务逻辑。
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"
)

const loggingDriverName = "mysql-logged"

// loggingDriver 包装 MySQL 驱动，拦截 SQL 执行并打印日志。
type loggingDriver struct{}

func (d *loggingDriver) Open(dsn string) (driver.Conn, error) {
	conn, err := (&mysql.MySQLDriver{}).Open(dsn)
	if err != nil {
		return nil, err
	}
	return &loggingConn{conn: conn}, nil
}

type loggingConn struct{ conn driver.Conn }

func (c *loggingConn) Prepare(query string) (driver.Stmt, error) { return c.conn.Prepare(query) }
func (c *loggingConn) Close() error                               { return c.conn.Close() }
func (c *loggingConn) Begin() (driver.Tx, error)                  { return c.conn.Begin() }

func (c *loggingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	logSQL(query, args)
	return c.conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *loggingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	logSQL(query, args)
	return c.conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (c *loggingConn) Ping(ctx context.Context) error {
	if p, ok := c.conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *loggingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if pc, ok := c.conn.(driver.ConnPrepareContext); ok {
		return pc.PrepareContext(ctx, query)
	}
	return c.conn.Prepare(query)
}

func (c *loggingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if bt, ok := c.conn.(driver.ConnBeginTx); ok {
		return bt.BeginTx(ctx, opts)
	}
	return c.conn.Begin()
}

func (c *loggingConn) CheckNamedValue(v *driver.NamedValue) error {
	if nc, ok := c.conn.(driver.NamedValueChecker); ok {
		return nc.CheckNamedValue(v)
	}
	return driver.ErrSkip
}

func (c *loggingConn) ResetSession(ctx context.Context) error {
	if rs, ok := c.conn.(driver.SessionResetter); ok {
		return rs.ResetSession(ctx)
	}
	return nil
}

func logSQL(query string, args []driver.NamedValue) {
	if len(args) > 0 {
		vals := make([]any, len(args))
		for i, a := range args {
			vals[i] = a.Value
		}
		log.Printf("[SQL] %s | %v", query, vals)
	} else {
		log.Printf("[SQL] %s", query)
	}
}

var loggingDriverRegistered bool

// Connect 连接到 MySQL 数据库并验证连接可用性。
// 设置 DB_LOG=1 环境变量启用 SQL 日志打印。
func Connect(dsn string) (*sql.DB, error) {
	driverName := "mysql"
	if os.Getenv("DB_LOG") == "1" {
		if !loggingDriverRegistered {
			sql.Register(loggingDriverName, &loggingDriver{})
			loggingDriverRegistered = true
		}
		driverName = loggingDriverName
		log.Println("[DB] SQL logging enabled")
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return db, nil
}

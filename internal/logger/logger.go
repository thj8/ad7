// Package logger 提供统一的结构化日志功能。
// 基于 Go 标准库 log/slog，支持同时输出到 stdout（文本）和文件（JSON）。
package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"ad7/internal/config"
)

// Init 根据配置初始化全局 logger。
// 如果 cfg.Path 非空，同时写文件（JSON）和 stdout（Text）。
// 如果 cfg.Path 为空，仅写 stdout（Text）。
func Init(cfg config.LogConfig) error {
	level := parseLevel(cfg.Level)

	opts := &slog.HandlerOptions{Level: level}

	textHandler := slog.NewTextHandler(os.Stdout, opts)

	if cfg.Path == "" {
		slog.SetDefault(slog.New(textHandler))
		return nil
	}

	// 创建日志文件目录
	dir := filepath.Dir(cfg.Path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	jsonHandler := slog.NewJSONHandler(f, opts)
	handler := newMultiHandler(textHandler, jsonHandler)
	slog.SetDefault(slog.New(handler))
	return nil
}

// multiHandler 同时写入多个 slog.Handler。
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return newMultiHandler(handlers...)
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return newMultiHandler(handlers...)
}

// parseLevel 将字符串转为 slog.Level，默认 info。
func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Info 记录 Info 级别日志。
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn 记录 Warn 级别日志。
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error 记录 Error 级别日志。
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

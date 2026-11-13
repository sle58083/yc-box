// Package logger 提供控制面结构化日志（uber-go/zap）与 trace_id / 脱敏支持。
package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// KeyTraceID 是贯穿请求与 Agent 消息的追踪字段名（system_design.md §9.3）。
const KeyTraceID = "trace_id"

// global 是进程级 logger，默认 Nop，由 Init 替换。
var global = zap.NewNop()

// Init 初始化全局 logger。
//
// level 取值：debug / info / warn / error（非法值回退 info）。
// format 取值：json / console。
func Init(level string, format string) error {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // ISO8601 UTC
	if format == "console" {
		cfg.Encoding = "console"
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}

	logger, err := cfg.Build()
	if err != nil {
		return err
	}
	global = logger
	return nil
}

// L 返回全局 logger。
func L() *zap.Logger { return global }

// WithTraceID 派生携带 trace_id 的 logger，供中间件注入后贯穿整个请求链路。
func WithTraceID(traceID string) *zap.Logger {
	return global.With(zap.String(KeyTraceID, traceID))
}

// Sync 刷新日志缓冲，优雅退出时调用。
func Sync() { _ = global.Sync() }

// Redact 对敏感值脱敏（§9.3：UUID/私钥/令牌永不打印明文）。
// 保留首尾各 2 位，中间替换为 *；长度 ≤4 时整体掩码。
func Redact(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

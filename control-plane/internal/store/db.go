// Package store 负责 GORM 连接、连接池与迁移。
//
// 约定（system_design.md §9.4）：数据库访问一律经 store 的 Repository 接口；
// 写操作显式事务；外键约束开启；JSONB 用 json.RawMessage 承载。
//
// 单机轻量化：数据库由 PostgreSQL 迁移为 SQLite（github.com/glebarez/sqlite，
// 纯 Go 实现、禁用 CGO），不再依赖 Redis。SQLite 单文件即可运行，适配「一核一G」
// 服务器；连接池保持单连接以规避 SQLITE_BUSY，并开启 WAL 提升并发读写。
package store

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/singbox-panel/control-plane/internal/model"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB 是对 *gorm.DB 的轻量封装。
type DB struct {
	*gorm.DB
}

// New 建立 SQLite 连接并配置 WAL / busy_timeout / 外键等。
//
// dsn 为 SQLite 文件路径（如 /opt/singbox-panel/data/panel.db）或 DSN；
// 传入 ":memory:" 表示内存库（主要用于测试）。
func New(dsn string, logLevel string) (*DB, error) {
	dialector, err := openSQLite(dsn)
	if err != nil {
		return nil, err
	}

	gormDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(parseLogLevel(logLevel)),
		// 外键约束开启（§9.4），AutoMigrate 时创建/维护 FK 约束。
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql db: %w", err)
	}

	// SQLite 单写多读：连接池保持 1，配合 WAL 与 busy_timeout 规避 SQLITE_BUSY。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	return &DB{DB: gormDB}, nil
}

// openSQLite 根据 dsn 构造 SQLite Dialector，并确保文件数据库的父目录存在。
func openSQLite(dsn string) (gorm.Dialector, error) {
	if isMemoryDSN(dsn) {
		// 内存库无需目录与持久化 pragma；busy_timeout/foreign_keys 对内存库作用有限，
		// 由数据库层默认行为兜底，测试场景足够。
		return sqlite.Open(dsn), nil
	}

	dir := filepath.Dir(dsn)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite data dir %q: %w", dir, err)
		}
	}

	return sqlite.Open(buildFileDSN(dsn)), nil
}

// isMemoryDSN 判断是否为内存数据库 DSN。
func isMemoryDSN(dsn string) bool {
	return dsn == ":memory:" ||
		strings.HasPrefix(dsn, "file::memory:") ||
		strings.HasPrefix(dsn, "file:memory:")
}

// buildFileDSN 构造带 pragma 的 SQLite 文件 DSN：
//   - journal_mode=WAL：提升并发读写能力；
//   - busy_timeout：写锁等待，避免立即返回 SQLITE_BUSY；
//   - foreign_keys：运行时启用外键约束。
func buildFileDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	return "file:" + path + "?" + q.Encode()
}

// AutoMigrate 创建 / 更新表结构，与 migrations/001_init.sql 等价。
// 顺序先父表后子表，保证外键约束可建立。
func (db *DB) AutoMigrate() error {
	return db.DB.AutoMigrate(
		&model.Group{},
		&model.Role{},
		&model.Node{},
		&model.PanelUser{},
		&model.InboundConfig{},
		&model.OutboundConfig{},
		&model.RoutingRule{},
		&model.ProxyUser{},
		&model.ConfigVersion{},
		&model.TrafficRecord{},
		&model.AuditLog{},
	)
}

func parseLogLevel(level string) gormlogger.LogLevel {
	switch strings.ToLower(level) {
	case "debug", "trace":
		return gormlogger.Info
	case "warn", "warning":
		return gormlogger.Warn
	case "error":
		return gormlogger.Error
	case "silent":
		return gormlogger.Silent
	default:
		return gormlogger.Warn
	}
}

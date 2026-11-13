// Package store 负责 GORM 连接、连接池与迁移。
//
// 约定（system_design.md §9.4）：数据库访问一律经 store 的 Repository 接口；
// 写操作显式事务；外键约束开启；JSONB 用 json.RawMessage 承载。
// 各领域（node/config/user/...）的 Repository 接口与实现在 T02-T04 中分别落地。
package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/singbox-panel/control-plane/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB 是对 *gorm.DB 的轻量封装。
type DB struct {
	*gorm.DB
}

// New 建立 PostgreSQL 连接并配置连接池。
func New(dsn string, logLevel string) (*DB, error) {
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(parseLogLevel(logLevel)),
		// 外键约束开启（§9.4），AutoMigrate 时创建/维护 FK 约束。
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &DB{DB: gormDB}, nil
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

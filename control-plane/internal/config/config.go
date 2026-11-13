// Package config 负责控制面配置的加载与解析（spf13/viper）。
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 是控制面全局配置。
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Crypto    CryptoConfig    `mapstructure:"crypto"`
	Bootstrap BootstrapConfig `mapstructure:"bootstrap"`
	Log       LogConfig       `mapstructure:"log"`
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // gin 模式：debug / release / test
}

// DatabaseConfig PostgreSQL 配置，可直接提供 DSN 或离散字段。
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
	TimeZone string `mapstructure:"timezone"`
	DSN      string `mapstructure:"dsn"` // 直接指定时优先使用
}

// RedisConfig 会话 / 心跳 / 缓存配置。
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// JWTConfig JWT 签发配置。
type JWTConfig struct {
	Secret     string        `mapstructure:"secret"` // # 签名密钥
	Issuer     string        `mapstructure:"issuer"`
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

// CryptoConfig 加密密钥配置。
type CryptoConfig struct {
	Key     string `mapstructure:"key"`      // 🔐 32 字节 AES-256 密钥（hex 编码）
	KeyFile string `mapstructure:"key_file"` // 🔐 密钥文件路径（优先级高于 key）
}

// BootstrapConfig 首次启动引导配置：在面板用户为空时创建首个超级管理员，
// 打破「创建面板用户需要 super_admin 权限，但没有 super_admin 用户」的死锁。
type BootstrapConfig struct {
	AdminUsername string `mapstructure:"admin_username"` // 首个超级管理员用户名（仅当面板无用户时创建）
	AdminPassword string `mapstructure:"admin_password"` // 🔐 初始密码；留空则不自动创建
}

// LogConfig 日志配置。
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug/info/warn/error
	Format string `mapstructure:"format"` // json/console
}

// ConnectionString 拼装 PostgreSQL 连接串；优先使用显式 DSN。
func (c *DatabaseConfig) ConnectionString() string {
	if c.DSN != "" {
		return c.DSN
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode, c.TimeZone,
	)
}

// Addr 返回监听地址。
func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// CryptoKey 返回 32 字节 AES-256 密钥。
func (c *Config) CryptoKey() ([]byte, error) {
	if c.Crypto.KeyFile != "" {
		raw, err := os.ReadFile(c.Crypto.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("read crypto key file: %w", err)
		}
		return decodeHexKey(strings.TrimSpace(string(raw)))
	}
	if c.Crypto.Key == "" {
		return nil, errors.New("config: crypto.key 或 crypto.key_file 必须配置（32 字节 AES-256 密钥）")
	}
	return decodeHexKey(c.Crypto.Key)
}

func decodeHexKey(s string) ([]byte, error) {
	key, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode crypto key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto key must be 32 bytes, got %d", len(key))
	}
	return key, nil
}

// Load 加载配置：依次读取 YAML 文件 + 环境变量（前缀 SBP_），并合并默认值。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	if path != "" {
		v.AddConfigPath(path)
	}
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")

	setDefaults(v)

	v.SetEnvPrefix("SBP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// 配置文件不存在时使用默认值 + 环境变量。
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	applyDerivedDefaults(&cfg)
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")

	v.SetDefault("database.host", "127.0.0.1")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "singbox")
	v.SetDefault("database.password", "")
	v.SetDefault("database.name", "singbox_panel")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.timezone", "UTC")

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)

	v.SetDefault("jwt.issuer", "singbox-panel")
	v.SetDefault("jwt.access_ttl", "15m")
	v.SetDefault("jwt.refresh_ttl", "168h") // 168h = 7 天；Go time.Duration 不支持 "d" 单位

	v.SetDefault("crypto.key", "")

	v.SetDefault("bootstrap.admin_username", "admin")
	v.SetDefault("bootstrap.admin_password", "")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
}

func applyDerivedDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "debug"
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.TimeZone == "" {
		cfg.Database.TimeZone = "UTC"
	}
	if cfg.Bootstrap.AdminUsername == "" {
		cfg.Bootstrap.AdminUsername = "admin"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
}

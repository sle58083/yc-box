// Package config 负责边缘 Agent 配置的加载与解析（spf13/viper）。
//
// 新增键见 docs/agent-cross-platform.md §5.2。
package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 是 Agent 全局配置。
type Config struct {
	Agent   AgentConfig   `mapstructure:"agent"`
	Control ControlConfig `mapstructure:"control"`
	Log     LogConfig     `mapstructure:"log"`
}

// AgentConfig Agent 自身配置。
type AgentConfig struct {
	DataDir string       `mapstructure:"datadir"`  // 数据目录（bin/etc/run/logs 布局）
	KeyFile string       `mapstructure:"key_file"` // 🔐 本地主密钥文件路径
	Kernel  KernelConfig `mapstructure:"kernel"`
	Proc    ProcConfig   `mapstructure:"proc"`
}

// KernelConfig 内核版本三元组与下载策略。
type KernelConfig struct {
	OS          string `mapstructure:"os"`           // 默认 auto=runtime.GOOS
	Arch        string `mapstructure:"arch"`         // 默认 auto=runtime.GOARCH
	Version     string `mapstructure:"version"`      // 空=跟随控制面下发
	DownloadURL string `mapstructure:"download_url"` // 控制面内核下载地址（或走 WS 下发）
}

// ProcConfig 进程托管参数。
type ProcConfig struct {
	StopGrace      time.Duration `mapstructure:"stop_grace"`      // Stop 优雅超时
	HealthInterval time.Duration `mapstructure:"health_interval"` // 健康探测间隔
}

// ControlConfig 控制面连接配置。
type ControlConfig struct {
	ServerURL string        `mapstructure:"server_url"` // wss:// 控制面地址
	Token     string        `mapstructure:"token"`      // # 初始令牌（首次启动后由 secret 管理）
	RetryMin  time.Duration `mapstructure:"retry_min"`  // 断线重连最小退避
	RetryMax  time.Duration `mapstructure:"retry_max"`  // 断线重连最大退避
	Heartbeat time.Duration `mapstructure:"heartbeat"`  // 心跳间隔（默认 5s）
}

// LogConfig 日志配置。
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// DefaultDataDir 返回各平台的默认数据目录。
func DefaultDataDir() string {
	if runtime.GOOS == "windows" {
		return `C:\ProgramData\singbox-agent`
	}
	return "/opt/singbox-agent"
}

// Load 加载配置：YAML 文件 + 环境变量（前缀 SBAGENT_）+ 默认值。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("agent")
	v.SetConfigType("yaml")
	if path != "" {
		v.AddConfigPath(path)
	}
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")

	setDefaults(v)

	v.SetEnvPrefix("SBAGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := applyDerivedDefaults(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("agent.datadir", DefaultDataDir())
	v.SetDefault("agent.kernel.os", "auto")
	v.SetDefault("agent.kernel.arch", "auto")
	v.SetDefault("agent.kernel.version", "")
	v.SetDefault("agent.kernel.download_url", "")
	v.SetDefault("agent.proc.stop_grace", "10s")
	v.SetDefault("agent.proc.health_interval", "3s")

	v.SetDefault("control.server_url", "")
	v.SetDefault("control.token", "")
	v.SetDefault("control.retry_min", "1s")
	v.SetDefault("control.retry_max", "30s")
	v.SetDefault("control.heartbeat", "5s")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
}

func applyDerivedDefaults(cfg *Config) error {
	if cfg.Agent.DataDir == "" {
		cfg.Agent.DataDir = DefaultDataDir()
	}
	if cfg.Agent.KeyFile == "" {
		cfg.Agent.KeyFile = filepath.Join(cfg.Agent.DataDir, "etc", "agent.key")
	}
	if cfg.Agent.Kernel.OS == "" {
		cfg.Agent.Kernel.OS = "auto"
	}
	if cfg.Agent.Kernel.Arch == "" {
		cfg.Agent.Kernel.Arch = "auto"
	}
	if cfg.Control.ServerURL == "" {
		return errors.New("config: control.server_url 必须配置（如 wss://panel.example.com/ws）")
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	return nil
}

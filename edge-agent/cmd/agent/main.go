// Command agent 是边缘 Agent 入口：加载配置 → 装配 proc/kernel/singbox/apply/client → 运行。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/singbox-panel/edge-agent/internal/apply"
	"github.com/singbox-panel/edge-agent/internal/client"
	"github.com/singbox-panel/edge-agent/internal/config"
	"github.com/singbox-panel/edge-agent/internal/host"
	"github.com/singbox-panel/edge-agent/internal/kernel"
	"github.com/singbox-panel/edge-agent/internal/proc"
	"github.com/singbox-panel/edge-agent/internal/secret"
	"github.com/singbox-panel/edge-agent/internal/singbox"
)

// version 由构建时 ldflags 注入。
var version = "dev"

func main() {
	// 1. 加载配置
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 日志
	logger, err := newLogger(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 3. 防双实例：run/agent.lock（记录 PID，崩溃残留自动清理）。
	mgr := proc.NewManager()
	reloader := proc.NewReloader(mgr)

	lock, err := acquireLock(filepath.Join(cfg.Agent.DataDir, "run", "agent.lock"), mgr)
	if err != nil {
		logger.Fatal("acquire agent.lock failed", zap.Error(err))
	}
	defer func() {
		_ = lock.Close()
		_ = os.Remove(filepath.Join(cfg.Agent.DataDir, "run", "agent.lock"))
	}()

	// 4. 本地令牌（config → secret 回退）。
	key, err := secret.LoadOrCreateKey(cfg.Agent.KeyFile)
	if err != nil {
		logger.Fatal("init secret key failed", zap.Error(err))
	}
	store, err := secret.NewStore(filepath.Join(cfg.Agent.DataDir, "etc", "secrets"), key)
	if err != nil {
		logger.Fatal("init secret store failed", zap.Error(err))
	}
	token := resolveToken(cfg, store)
	if token == "" {
		logger.Fatal("缺少 Agent 令牌：请配置 control.token 或先写入本地 secret")
	}

	// 5. 装配 kernel / singbox / apply。
	km := kernel.NewManager(kernel.Config{
		DataDir:     cfg.Agent.DataDir,
		DownloadURL: cfg.Agent.Kernel.DownloadURL,
	}, logger)
	sup := singbox.NewSupervisor(mgr, reloader, km, cfg.Agent.DataDir, logger,
		cfg.Agent.Proc.StopGrace, cfg.Agent.Proc.HealthInterval)
	applyer := apply.NewApplyer(sup, cfg.Agent.DataDir, logger)

	// 6. WS 客户端。
	hostInfo := host.Detect()
	cli := client.New(cfg, logger, hostInfo, client.Handlers{
		OnConfigPush: func(ver int, content []byte) client.ApplyResult {
			status, graceful, aerr := applyer.Apply(context.Background(), content)
			res := client.ApplyResult{Version: ver, Status: status, Graceful: graceful}
			if aerr != nil {
				res.Error = aerr.Error()
			}
			return res
		},
		OnRestart: func() error {
			return sup.Restart(context.Background())
		},
	}, token, version)

	// 7. 若已有配置则启动 sing-box（否则等待首次 config_push）。
	if _, err := os.Stat(filepath.Join(cfg.Agent.DataDir, "etc", "config.json")); err == nil {
		if serr := sup.Start(context.Background()); serr != nil {
			logger.Warn("启动既有配置失败，等待下发", zap.Error(serr))
		}
	}

	// 8. 运行。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- cli.Run(ctx) }()

	logger.Info("agent started",
		zap.String("datadir", cfg.Agent.DataDir),
		zap.String("control_server", cfg.Control.ServerURL),
		zap.String("os", hostInfo.OS),
		zap.String("arch", hostInfo.Arch),
		zap.String("libc", hostInfo.Libc),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("agent stopping", zap.String("signal", sig.String()))
		cli.Stop()
		cancel()
	case err := <-errCh:
		logger.Info("client 退出", zap.Error(err))
	}

	// 优雅停止 sing-box。
	stopCtx, stopCancel := context.WithTimeout(context.Background(), cfg.Agent.Proc.StopGrace)
	defer stopCancel()
	_ = sup.Stop(stopCtx)
	logger.Info("agent stopped")
}

func resolveToken(cfg *config.Config, store *secret.Store) string {
	if cfg.Control.Token != "" {
		_ = store.Set("agent_token", []byte(cfg.Control.Token))
		return cfg.Control.Token
	}
	if v, err := store.GetString("agent_token"); err == nil {
		return v
	}
	return ""
}

func acquireLock(path string, mgr proc.ProcessManager) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
		return f, nil
	}
	// 锁已存在：读取持有者 PID，若已不在运行（崩溃残留）则清理后重试一次。
	if stale, _ := os.ReadFile(path); len(stale) > 0 {
		if pid, perr := strconv.Atoi(strings.TrimSpace(string(stale))); perr == nil && pid > 0 && !mgr.IsRunning(pid) {
			_ = os.Remove(path)
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err == nil {
				_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
				return f, nil
			}
		}
	}
	return nil, fmt.Errorf("agent 已在运行（agent.lock 被占用）: %w", err)
}

func newLogger(level string, format string) (*zap.Logger, error) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}
	zcfg := zap.NewProductionConfig()
	zcfg.Level = zap.NewAtomicLevelAt(lvl)
	zcfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if format == "console" {
		zcfg.Encoding = "console"
		zcfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	return zcfg.Build()
}

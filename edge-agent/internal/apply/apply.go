// Package apply 实现配置应用统一流程：预校验 → 备份 → 写盘 → reload → 健康探测 → 失败回滚
// （docs/agent-cross-platform.md §2.3）。
package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/singbox-panel/edge-agent/internal/singbox"
)

// 应用结果状态（与控制面 config_apply_result.status 对齐）。
const (
	StatusSuccess    = "success"
	StatusFailed     = "failed"
	StatusRolledBack = "rolled_back"
)

// Supervisor 是 apply 依赖的生命周期接口（消费方定义，便于单测注入）。
type Supervisor interface {
	Reload(ctx context.Context) (graceful bool, err error)
	HealthCheck(ctx context.Context) error
	BinaryPath() (string, error)
}

// Applyer 配置应用器。
type Applyer struct {
	sup        Supervisor
	configPath string
	log        *zap.Logger
	precheck   func(ctx context.Context, content []byte) error
}

// NewApplyer 构造应用器。
func NewApplyer(sup Supervisor, dataDir string, log *zap.Logger) *Applyer {
	a := &Applyer{
		sup:        sup,
		configPath: filepath.Join(dataDir, "etc", "config.json"),
		log:        log,
	}
	// 默认预校验：sing-box check -c。
	a.precheck = func(ctx context.Context, content []byte) error {
		binary, err := sup.BinaryPath()
		if err != nil {
			return err
		}
		tmp := a.configPath + ".tmp"
		if err := os.MkdirAll(filepath.Dir(tmp), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(tmp, content, 0o644); err != nil {
			return err
		}
		defer os.Remove(tmp)
		return singbox.Check(ctx, binary, tmp)
	}
	return a
}

// SetPrecheck 覆盖预校验逻辑（测试注入）。
func (a *Applyer) SetPrecheck(f func(ctx context.Context, content []byte) error) { a.precheck = f }

// Apply 应用配置，返回状态（success/failed/rolled_back）与是否热重载。
func (a *Applyer) Apply(ctx context.Context, content []byte) (string, bool, error) {
	// 1. 预校验。
	if err := a.precheck(ctx, content); err != nil {
		return StatusFailed, false, err
	}

	// 2. 备份旧配置。
	if err := a.backup(); err != nil {
		return StatusFailed, false, err
	}

	// 3. 原子写盘。
	if err := a.writeConfig(content); err != nil {
		return StatusFailed, false, err
	}

	// 4. 生效。
	graceful, err := a.sup.Reload(ctx)
	if err != nil {
		_ = a.rollback(ctx)
		return StatusRolledBack, graceful, err
	}

	// 5. 健康探测。
	if err := a.sup.HealthCheck(ctx); err != nil {
		rbErr := a.rollback(ctx)
		if rbErr != nil {
			err = fmt.Errorf("%w（回滚失败: %v）", err, rbErr)
		}
		return StatusRolledBack, graceful, err
	}

	return StatusSuccess, graceful, nil
}

func (a *Applyer) backup() error {
	if _, err := os.Stat(a.configPath); err != nil {
		if os.IsNotExist(err) {
			return nil // 首次下发无备份
		}
		return err
	}
	bak := a.configPath + ".bak"
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return err
	}
	return os.WriteFile(bak, data, 0o644)
}

func (a *Applyer) writeConfig(content []byte) error {
	if err := os.MkdirAll(filepath.Dir(a.configPath), 0o755); err != nil {
		return err
	}
	tmp := a.configPath + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	_ = os.Remove(a.configPath)
	return os.Rename(tmp, a.configPath)
}

// rollback 恢复旧配置并再次生效。
func (a *Applyer) rollback(ctx context.Context) error {
	bak := a.configPath + ".bak"
	data, err := os.ReadFile(bak)
	if err != nil {
		return fmt.Errorf("apply: 读取备份失败: %w", err)
	}
	if err := a.writeConfig(data); err != nil {
		return fmt.Errorf("apply: 回滚写盘失败: %w", err)
	}
	if _, err := a.sup.Reload(ctx); err != nil {
		return fmt.Errorf("apply: 回滚 reload 失败: %w", err)
	}
	return nil
}

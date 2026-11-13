package apply

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/singbox-panel/edge-agent/internal/proc"
)

type fakeSupervisor struct {
	reloadCalls int
	startCalls  int
	healthOk    bool
	graceful    bool
	reloadErr   error
	startErr    error
}

func (f *fakeSupervisor) Reload(context.Context) (bool, error) {
	f.reloadCalls++
	if f.reloadErr != nil {
		return f.graceful, f.reloadErr
	}
	return f.graceful, nil
}

func (f *fakeSupervisor) Start(context.Context) error {
	f.startCalls++
	return f.startErr
}

func (f *fakeSupervisor) HealthCheck(context.Context) error {
	if f.healthOk {
		return nil
	}
	return errors.New("unhealthy")
}

func (f *fakeSupervisor) BinaryPath() (string, error) { return "/fake/sing-box", nil }

func TestApplySuccess(t *testing.T) {
	dir := t.TempDir()
	sup := &fakeSupervisor{healthOk: true, graceful: true}
	a := NewApplyer(sup, dir, nil)
	a.SetPrecheck(func(context.Context, []byte) error { return nil })

	status, graceful, err := a.Apply(context.Background(), []byte(`{"inbounds":[]}`))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if status != StatusSuccess || !graceful {
		t.Fatalf("期望 success+graceful: %s %v", status, graceful)
	}
	if sup.reloadCalls != 1 {
		t.Fatalf("reload 应调用 1 次: %d", sup.reloadCalls)
	}
	cfg := filepath.Join(dir, "etc", "config.json")
	if b, _ := os.ReadFile(cfg); string(b) != `{"inbounds":[]}` {
		t.Fatalf("配置未写盘")
	}
}

func TestApplyPrecheckFailed(t *testing.T) {
	dir := t.TempDir()
	sup := &fakeSupervisor{healthOk: true}
	a := NewApplyer(sup, dir, nil)
	a.SetPrecheck(func(context.Context, []byte) error { return errors.New("check failed") })

	status, _, err := a.Apply(context.Background(), []byte(`{}`))
	if err == nil || status != StatusFailed {
		t.Fatalf("期望 failed: %s %v", status, err)
	}
	if sup.reloadCalls != 0 {
		t.Fatalf("预校验失败不应 reload")
	}
}

func TestApplyHealthFailedRollback(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "etc", "config.json")
	_ = os.MkdirAll(filepath.Dir(cfg), 0o755)
	_ = os.WriteFile(cfg, []byte("old-config"), 0o644)

	sup := &fakeSupervisor{healthOk: false}
	a := NewApplyer(sup, dir, nil)
	a.SetPrecheck(func(context.Context, []byte) error { return nil })

	status, _, err := a.Apply(context.Background(), []byte("new-config"))
	if err == nil || status != StatusRolledBack {
		t.Fatalf("期望 rolled_back: %s %v", status, err)
	}
	if sup.reloadCalls != 2 {
		t.Fatalf("应 reload 2 次（生效+回滚）: %d", sup.reloadCalls)
	}
	if b, _ := os.ReadFile(cfg); string(b) != "old-config" {
		t.Fatalf("回滚后应恢复旧配置")
	}
}

func TestApplyFirstStartWhenNotRunning(t *testing.T) {
	dir := t.TempDir()
	sup := &fakeSupervisor{
		healthOk:  true,
		reloadErr: proc.ErrNotRunning, // 首次下发：sing-box 未运行
	}
	a := NewApplyer(sup, dir, nil)
	a.SetPrecheck(func(context.Context, []byte) error { return nil })

	status, graceful, err := a.Apply(context.Background(), []byte(`{"inbounds":[]}`))
	if err != nil {
		t.Fatalf("首次启动应成功: %v", err)
	}
	if status != StatusSuccess {
		t.Fatalf("期望 success: %s", status)
	}
	if graceful {
		t.Fatalf("首次启动应为冷启动 graceful=false")
	}
	if sup.startCalls != 1 {
		t.Fatalf("应调用 Start 1 次: %d", sup.startCalls)
	}
}

// Package singbox 负责 sing-box 内核的进程托管（supervisor）与预校验。
package singbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/singbox-panel/edge-agent/internal/kernel"
	"github.com/singbox-panel/edge-agent/internal/proc"
)

// Supervisor 面向 proc.ProcessManager / proc.Reloader 托管 sing-box 生命周期。
// OS 差异全部下沉到 proc 包，本层不出现 runtime.GOOS 分支。
type Supervisor struct {
	mgr      proc.ProcessManager
	reloader proc.Reloader
	kernel   kernel.Manager
	dataDir  string
	log      *zap.Logger

	stopGrace      time.Duration
	healthInterval time.Duration

	mu sync.Mutex
	rt *proc.Runtime
}

// NewSupervisor 构造 supervisor。
func NewSupervisor(mgr proc.ProcessManager, reloader proc.Reloader, km kernel.Manager, dataDir string, log *zap.Logger, stopGrace, healthInterval time.Duration) *Supervisor {
	return &Supervisor{
		mgr:            mgr,
		reloader:       reloader,
		kernel:         km,
		dataDir:        dataDir,
		log:            log,
		stopGrace:      stopGrace,
		healthInterval: healthInterval,
	}
}

// BinaryPath 返回当前激活内核二进制路径。
func (s *Supervisor) BinaryPath() (string, error) { return s.kernel.Current() }

// configPath 返回当前配置路径。
func (s *Supervisor) configPath() string { return filepath.Join(s.dataDir, "etc", "config.json") }

// pidFile 返回 PID 文件路径。
func (s *Supervisor) pidFile() string { return filepath.Join(s.dataDir, "run", "sing-box.pid") }

// Start 启动 sing-box（含启动前 PID 文件对账，防残留）。
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reconcile()

	binary, err := s.kernel.Current()
	if err != nil {
		return err
	}
	spec := proc.ProcessSpec{
		Binary: binary,
		Args:   []string{"run", "-c", s.configPath()},
		Dir:    s.dataDir,
	}
	p, err := s.mgr.Start(ctx, spec)
	if err != nil {
		return fmt.Errorf("singbox: 启动失败: %w", err)
	}
	s.rt = &proc.Runtime{Spec: spec, Proc: p}
	s.writePidFile(p.PID())
	if s.log != nil {
		s.log.Info("sing-box started", zap.Int("pid", p.PID()), zap.String("binary", binary))
	}
	return nil
}

// Stop 优雅停止当前进程。
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	rt := s.rt
	s.mu.Unlock()
	if rt == nil || rt.Proc == nil {
		return proc.ErrNotRunning
	}
	if err := s.mgr.Stop(ctx, rt.Proc, s.stopGrace); err != nil && err != proc.ErrStopTimeout {
		return err
	}
	s.mu.Lock()
	s.rt = nil
	s.mu.Unlock()
	_ = os.Remove(s.pidFile())
	return nil
}

// Restart 重启（Stop → Start）。
func (s *Supervisor) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil && err != proc.ErrNotRunning {
		return err
	}
	return s.Start(ctx)
}

// Reload 让当前进程加载新配置；返回 graceful（linux SIGHUP true / windows 重启式 false）。
func (s *Supervisor) Reload(ctx context.Context) (bool, error) {
	s.mu.Lock()
	rt := s.rt
	s.mu.Unlock()
	if rt == nil || rt.Proc == nil {
		return false, proc.ErrNotRunning
	}
	return s.reloader.Reload(ctx, rt)
}

// HealthCheck 健康探测：进程存活（sing-box API 探活为 P1）。
func (s *Supervisor) HealthCheck(ctx context.Context) error {
	s.mu.Lock()
	rt := s.rt
	s.mu.Unlock()
	if rt == nil || rt.Proc == nil {
		return proc.ErrNotRunning
	}
	if !s.mgr.IsRunning(rt.Proc.PID()) {
		return fmt.Errorf("singbox: 进程 %d 未存活", rt.Proc.PID())
	}
	return nil
}

// PID 返回当前进程 PID（未运行返回 0）。
func (s *Supervisor) PID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rt == nil || s.rt.Proc == nil {
		return 0
	}
	return s.rt.Proc.PID()
}

// Wait 等待当前进程退出（供 main 监听意外退出）。
func (s *Supervisor) Wait() error {
	s.mu.Lock()
	rt := s.rt
	s.mu.Unlock()
	if rt == nil || rt.Proc == nil {
		return proc.ErrNotRunning
	}
	return s.mgr.Wait(rt.Proc)
}

// reconcile 启动前 PID 文件对账：若残留进程存活，先终止再接管。
func (s *Supervisor) reconcile() {
	data, err := os.ReadFile(s.pidFile())
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return
	}
	if s.mgr.IsRunning(pid) {
		if s.log != nil {
			s.log.Warn("reconcile: 终止残留 sing-box 进程", zap.Int("pid", pid))
		}
		// best-effort 信号终止；孤儿回收的最终保证是 Pdeathsig（linux）/ Job Object（windows）。
		if err := proc.Terminate(pid); err != nil && s.log != nil {
			s.log.Warn("reconcile: 终止残留进程失败", zap.Int("pid", pid), zap.Error(err))
		}
	}
	_ = os.Remove(s.pidFile())
}

func (s *Supervisor) writePidFile(pid int) {
	_ = os.MkdirAll(filepath.Dir(s.pidFile()), 0o755)
	_ = os.WriteFile(s.pidFile(), []byte(strconv.Itoa(pid)), 0o644)
}

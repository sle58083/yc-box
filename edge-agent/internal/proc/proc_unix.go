//go:build linux

package proc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func newManager() ProcessManager { return &unixManager{} }

type unixManager struct{}

type unixProcess struct {
	cmd *exec.Cmd
	pid int
}

func (p *unixProcess) PID() int                   { return p.pid }
func (p *unixProcess) Signal(sig os.Signal) error { return p.cmd.Process.Signal(sig) }
func (p *unixProcess) Wait() error                { return p.cmd.Wait() }

func (m *unixManager) Start(ctx context.Context, spec ProcessSpec) (Process, error) {
	cmd := exec.CommandContext(ctx, spec.Binary, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	// 关键：独立进程组 + 父进程死亡信号（孤儿回收，docs/agent-cross-platform.md §1.2）。
	// 注意：不用 Setsid——容器内（尤其 agent 为 PID 1 时）setsid 可能返回 EPERM，
	// 且容器 namespace 已提供会话隔离，Setpgid 已足够按组管理进程。
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,            // 新进程组，便于按组发信号（负 PID）
		Pdeathsig: syscall.SIGTERM, // 父进程(Agent)死亡 → 内核自动 SIGTERM 子进程
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &unixProcess{cmd: cmd, pid: cmd.Process.Pid}, nil
}

func (m *unixManager) Stop(ctx context.Context, p Process, grace time.Duration) error {
	up, ok := p.(*unixProcess)
	if !ok {
		return ErrNotRunning
	}
	_ = syscall.Kill(up.pid, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- up.cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(grace):
		_ = syscall.Kill(-up.pid, syscall.SIGKILL) // 负 PID → 整组强杀
		<-done
		return ErrStopTimeout
	case <-ctx.Done():
		_ = syscall.Kill(-up.pid, syscall.SIGKILL)
		return ctx.Err()
	}
}

func (m *unixManager) Restart(ctx context.Context, spec ProcessSpec) (Process, error) {
	// 旧进程的 Stop 由上层 supervisor 负责；此处仅按 spec 启动新进程。
	return m.Start(ctx, spec)
}

func (m *unixManager) IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	// 1) signal 0 探测进程存在。
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	// 2) 读 /proc/<pid>/stat 排除僵尸态（stat 第 3 字段 = 状态字母）。
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	fields := bytes.Fields(data)
	return len(fields) > 2 && string(fields[2]) != "Z"
}

func (m *unixManager) Wait(p Process) error {
	up, ok := p.(*unixProcess)
	if !ok {
		return ErrNotRunning
	}
	return up.cmd.Wait()
}

// Terminate 尽力向指定 PID 发送终止信号（供 supervisor 启动前对账）。
func Terminate(pid int) error {
	if pid <= 0 {
		return ErrNotRunning
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

// signalReloader：Linux 通过 SIGHUP 无中断热重载，graceful=true。
type signalReloader struct{ sig syscall.Signal }

func newReloader(mgr ProcessManager) Reloader { return &signalReloader{sig: syscall.SIGHUP} }

func (r *signalReloader) Reload(ctx context.Context, rt *Runtime) (bool, error) {
	if rt == nil || rt.Proc == nil {
		return false, ErrNotRunning
	}
	if err := rt.Proc.Signal(r.sig); err != nil {
		return false, err
	}
	return true, nil // 无中断热重载
}

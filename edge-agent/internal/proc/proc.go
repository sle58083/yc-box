// Package proc 是 sing-box 子进程托管的 OS 抽象层，屏蔽 Linux / Windows 差异
// （docs/agent-cross-platform.md §1）。
package proc

import (
	"context"
	"errors"
	"os"
	"time"
)

var (
	ErrAlreadyRunning = errors.New("proc: process already running")
	ErrNotRunning     = errors.New("proc: process not running")
	ErrStopTimeout    = errors.New("proc: stop grace expired, force killed")
)

// ProcessSpec 描述一个待启动的 sing-box 子进程。
type ProcessSpec struct {
	Binary string   // 二进制绝对路径（由 kernel 包定位后传入）
	Args   []string // 如 ["run", "-c", configPath]
	Dir    string   // 工作目录（Agent 数据目录）
	Env    []string // 附加环境变量，K=V
}

// Process 是已启动子进程的句柄（平台相关实现）。
type Process interface {
	PID() int
	Signal(sig os.Signal) error
	Wait() error
}

// ProcessManager 进程托管抽象，屏蔽 Linux/Windows 差异。
type ProcessManager interface {
	// Start 启动子进程并返回句柄。
	Start(ctx context.Context, spec ProcessSpec) (Process, error)
	// Stop 优雅停止：先发信号，超 grace 后强杀。
	Stop(ctx context.Context, p Process, grace time.Duration) error
	// Restart 重新启动（上层 supervisor 负责 Stop 旧进程后再 Start）。
	Restart(ctx context.Context, spec ProcessSpec) (Process, error)
	// IsRunning 判断 PID 对应进程是否存活（含僵尸态识别）。
	IsRunning(pid int) bool
	// Wait 转发子进程 Wait，供 supervisor 监听意外退出。
	Wait(p Process) error
}

// Runtime 描述 sing-box 当前运行上下文，供 Reloader 与 supervisor 共享。
type Runtime struct {
	Spec ProcessSpec
	Proc Process // 当前进程句柄，nil 表示未运行
}

// NewManager 工厂：按 build tag 返回平台实现。
func NewManager() ProcessManager { return newManager() }

// NewReloader 工厂：按 build tag 返回平台实现（Reloader 定义见 reload.go）。
func NewReloader(mgr ProcessManager) Reloader { return newReloader(mgr) }

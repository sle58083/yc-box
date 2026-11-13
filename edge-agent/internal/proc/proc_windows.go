//go:build windows

package proc

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func newManager() ProcessManager { return &winManager{} }

type winManager struct{}

type winProcess struct {
	cmd *exec.Cmd
	pid int
	job windows.Handle // Job Object，用于 KILL_ON_JOB_CLOSE
}

func (p *winProcess) PID() int                   { return p.pid }
func (p *winProcess) Signal(sig os.Signal) error { return p.cmd.Process.Signal(sig) }
func (p *winProcess) Wait() error                { return p.cmd.Wait() }

func (m *winManager) Start(ctx context.Context, spec ProcessSpec) (Process, error) {
	cmd := exec.CommandContext(ctx, spec.Binary, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, // 独立进程组，可收 CTRL_BREAK
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// 关键：挂到 Job Object，KILL_ON_JOB_CLOSE 保证 Agent 退出/崩溃时子进程被系统回收。
	wp := &winProcess{cmd: cmd, pid: cmd.Process.Pid}
	job, err := windows.CreateJobObject(nil, nil)
	if err == nil {
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(
			job,
			uint32(windows.JobObjectExtendedLimitInformation),
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		); err == nil {
			// PROCESS_SET_QUOTA(0x0100) | PROCESS_TERMINATE(0x0001)：AssignProcessToJobObject 所需权限。
			if h, herr := windows.OpenProcess(0x0100|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid)); herr == nil {
				if windows.AssignProcessToJobObject(job, h) == nil {
					wp.job = job
				}
				_ = windows.CloseHandle(h)
			}
		}
	}
	return wp, nil
}

func (m *winManager) Stop(ctx context.Context, p Process, grace time.Duration) error {
	wp, ok := p.(*winProcess)
	if !ok {
		return ErrNotRunning
	}
	// 1) 尽力优雅：向进程组发 CTRL_BREAK（仅当 sing-box 附着控制台时有效）。
	_ = windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(wp.pid))
	done := make(chan error, 1)
	go func() { done <- wp.cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(grace):
		// 2) 超时强杀。
		m.terminate(wp.pid)
		<-done
		return ErrStopTimeout
	case <-ctx.Done():
		m.terminate(wp.pid)
		return ctx.Err()
	}
}

func (m *winManager) terminate(pid int) {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}
	_ = windows.TerminateProcess(h, 1)
	_ = windows.CloseHandle(h)
}

func (m *winManager) Restart(ctx context.Context, spec ProcessSpec) (Process, error) {
	// 旧进程的 Stop 由上层 supervisor 负责；此处仅按 spec 启动新进程。
	return m.Start(ctx, spec)
}

func (m *winManager) IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	// PROCESS_QUERY_LIMITED_INFORMATION(0x1000)：GetExitCodeProcess 所需访问权限。
	h, err := windows.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive // 259
}

// stillActive 是 Windows 进程仍存活的退出码（STILL_ACTIVE）。
const stillActive uint32 = 259

func (m *winManager) Wait(p Process) error {
	wp, ok := p.(*winProcess)
	if !ok {
		return ErrNotRunning
	}
	return wp.cmd.Wait()
}

// Terminate 尽力向指定 PID 发送终止（供 supervisor 启动前对账）。
func Terminate(pid int) error {
	if pid <= 0 {
		return ErrNotRunning
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}

// restartReloader：Windows 无 SIGHUP，降级为「优雅重启」（Stop→Start），graceful=false。
type restartReloader struct{ mgr ProcessManager }

const stopGrace = 10 * time.Second

func newReloader(mgr ProcessManager) Reloader { return &restartReloader{mgr: mgr} }

func (r *restartReloader) Reload(ctx context.Context, rt *Runtime) (bool, error) {
	if rt == nil {
		return false, ErrNotRunning
	}
	if rt.Proc != nil {
		if err := r.mgr.Stop(ctx, rt.Proc, stopGrace); err != nil && err != ErrStopTimeout {
			return false, err
		}
	}
	p, err := r.mgr.Start(ctx, rt.Spec)
	if err != nil {
		return false, err
	}
	rt.Proc = p
	return false, nil // graceful=false：走了重启式
}

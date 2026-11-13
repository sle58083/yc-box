package proc

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestProcessManagerLifecycle 覆盖 Start/IsRunning/Stop。
// 采用「测试二进制自拉起」helper 进程，跨平台无需外部命令。
func TestProcessManagerLifecycle(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Skipf("无法获取测试二进制: %v", err)
	}
	mgr := NewManager()
	p, err := mgr.Start(context.Background(), ProcessSpec{
		Binary: binary,
		Args:   []string{"-test.run=TestHelperProcess"},
		Env:    []string{"GO_WANT_HELPER_PROCESS=1"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if p.PID() <= 0 {
		t.Fatalf("PID 非法: %d", p.PID())
	}
	if !mgr.IsRunning(p.PID()) {
		t.Fatalf("进程应存活")
	}
	// 立即停止（helper 会 sleep，等信号）。
	if err := mgr.Stop(context.Background(), p, 5*time.Second); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if mgr.IsRunning(p.PID()) {
		t.Fatalf("进程应已退出")
	}
}

func TestIsRunningNonexistent(t *testing.T) {
	mgr := NewManager()
	if mgr.IsRunning(99999999) {
		t.Fatalf("不存在的 PID 不应存活")
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		t.Skip("非 helper 进程")
	}
	time.Sleep(30 * time.Second)
}

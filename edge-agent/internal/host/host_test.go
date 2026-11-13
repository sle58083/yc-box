package host

import (
	"runtime"
	"testing"
)

func TestDetect(t *testing.T) {
	info := Detect()
	if info.OS != runtime.GOOS {
		t.Fatalf("OS 探测错误: got %s want %s", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Fatalf("Arch 探测错误: got %s want %s", info.Arch, runtime.GOARCH)
	}
	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		if info.Libc == "" {
			t.Fatalf("Libc 探测不应为空")
		}
	}
}

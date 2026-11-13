// Package host 负责探测 Agent 主机信息（OS/ARCH/libc），供 register 上报。
package host

import (
	"path/filepath"
	"runtime"
)

// Info 是 register.hostinfo 的结构。
type Info struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	Libc string `json:"libc"`
}

// Detect 探测主机信息。
func Detect() Info {
	return Info{OS: runtime.GOOS, Arch: runtime.GOARCH, Libc: detectLibc()}
}

// detectLibc 探测 C 运行库（linux 下区分 glibc / musl）。
func detectLibc() string {
	switch runtime.GOOS {
	case "linux":
		if isMusl() {
			return "musl"
		}
		return "glibc"
	case "darwin":
		return "libSystem"
	case "windows":
		return "msvcrt"
	default:
		return ""
	}
}

func isMusl() bool {
	// 常见 musl 动态链接器路径（x86_64/aarch64/arm/mips 等）。
	matches, _ := filepath.Glob("/lib/ld-musl-*.so.1")
	if len(matches) > 0 {
		return true
	}
	// 某些发行版把 musl ld 放在 /usr/lib 下。
	matches, _ = filepath.Glob("/usr/lib/ld-musl-*.so.1")
	return len(matches) > 0
}

package proc

import "context"

// Reloader 抽象「让 sing-box 使用新配置生效」的动作，屏蔽 SIGHUP 与无信号差异
// （docs/agent-cross-platform.md §2.2）。
type Reloader interface {
	// Reload 使运行中的进程加载新配置。
	// graceful=true 表示无中断热重载（SIGHUP）；
	// graceful=false 表示平台不支持，已降级为「优雅重启」（Stop 旧进程 → Start 新进程）。
	// 返回错误时上层走统一回滚流程。
	Reload(ctx context.Context, rt *Runtime) (graceful bool, err error)
}

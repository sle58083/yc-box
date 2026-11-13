package singbox

import (
	"context"
	"fmt"
	"os/exec"
)

// Check 封装 `sing-box check -c <config>`（预校验，退出码 + stderr）。
func Check(ctx context.Context, binary string, configPath string) error {
	cmd := exec.CommandContext(ctx, binary, "check", "-c", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box check 失败: %w: %s", err, string(out))
	}
	return nil
}

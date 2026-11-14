// Package files 提供控制面下发文件的落盘工具（证书等）。
//
// 安全约定（sui-gap-design.md §9）：路径一律为相对 dataDir 的路径，
// 写入前做目录穿越校验，禁止越出 dataDir。
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/singbox-panel/edge-agent/internal/client"
)

// WriteAll 将 files 逐条写入 dataDir 下的相对路径，任一失败即停止并返回错误。
func WriteAll(dataDir string, files []client.FilePayload) error {
	for _, f := range files {
		if err := writeOne(dataDir, f); err != nil {
			return err
		}
	}
	return nil
}

// writeOne 校验相对路径并写单个文件（自动创建父目录，按 Mode 设权限）。
func writeOne(dataDir string, f client.FilePayload) error {
	if f.Path == "" {
		return fmt.Errorf("files: 文件路径不能为空")
	}
	if filepath.IsAbs(f.Path) {
		return fmt.Errorf("files: 路径必须是相对 dataDir 的路径: %q", f.Path)
	}

	absBase, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("files: 解析 dataDir 失败: %w", err)
	}
	target := filepath.Join(absBase, filepath.FromSlash(f.Path))

	// 防目录穿越：Rel 后若以 ".." 开头则说明越出 dataDir。
	rel, err := filepath.Rel(absBase, target)
	if err != nil {
		return fmt.Errorf("files: 非法路径 %q: %w", f.Path, err)
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("files: 非法路径 %q 越出 dataDir", f.Path)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("files: 创建目录失败: %w", err)
	}

	mode := os.FileMode(f.Mode)
	if mode == 0 {
		mode = 0o644
	}
	if err := os.WriteFile(target, f.Content, mode); err != nil {
		return fmt.Errorf("files: 写入 %q 失败: %w", f.Path, err)
	}
	return nil
}

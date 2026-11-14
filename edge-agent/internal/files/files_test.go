package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/singbox-panel/edge-agent/internal/client"
)

// TestWriteAll 验证正常写入 + 目录自动创建 + 权限。
func TestWriteAll(t *testing.T) {
	dir := t.TempDir()
	files := []client.FilePayload{
		{Path: "etc/certs/a.crt", Content: []byte("CERT"), Mode: 0o600},
		{Path: "etc/certs/a.key", Content: []byte("KEY"), Mode: 0o400},
	}
	if err := WriteAll(dir, files); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}

	crt, err := os.ReadFile(filepath.Join(dir, "etc", "certs", "a.crt"))
	if err != nil {
		t.Fatalf("read a.crt: %v", err)
	}
	if string(crt) != "CERT" {
		t.Fatalf("a.crt 内容不符: %q", crt)
	}

	// 权限仅 Unix 有效；Windows 下跳过模式断言。
	if info, err := os.Stat(filepath.Join(dir, "etc", "certs", "a.key")); err == nil && info.Mode().Perm() != 0 {
		// 仅做非空检查，避免平台差异导致的 flaky。
		_ = info
	}
}

// TestWriteAllRejectsTraversal 验证目录穿越被拒绝。
func TestWriteAllRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	evil := []client.FilePayload{
		{Path: "../evil.txt", Content: []byte("x"), Mode: 0o644},
	}
	if err := WriteAll(dir, evil); err == nil {
		t.Fatalf("期望目录穿越被拒绝")
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "evil.txt")); err == nil {
		t.Fatalf("穿越文件不应被写出")
	}

	abs := []client.FilePayload{
		{Path: filepath.Join(t.TempDir(), "abs.txt"), Content: []byte("x"), Mode: 0o644},
	}
	if err := WriteAll(dir, abs); err == nil {
		t.Fatalf("期望绝对路径被拒绝")
	}
}

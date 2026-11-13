package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func shaOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestVerify(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "sing-box")
	writeFile(t, f, "binary-content")
	m := NewManager(Config{DataDir: dir}, nil)

	if err := m.Verify(f, shaOf("binary-content")); err != nil {
		t.Fatalf("Verify 应通过: %v", err)
	}
	if err := m.Verify(f, "deadbeef"); err == nil {
		t.Fatalf("Verify 应失败")
	}
}

func TestActivateRollback(t *testing.T) {
	dir := t.TempDir()
	osName, archName := runtime.GOOS, runtime.GOARCH
	specV1 := Spec{OS: osName, Arch: archName, Version: "1.0.0"}
	specV2 := Spec{OS: osName, Arch: archName, Version: "2.0.0"}

	m := NewManager(Config{DataDir: dir}, nil)
	poolV1 := filepath.Join(dir, "bin", "pool", fmt.Sprintf("sing-box-1.0.0-%s-%s", osName, archName))
	poolV2 := filepath.Join(dir, "bin", "pool", fmt.Sprintf("sing-box-2.0.0-%s-%s", osName, archName))
	role := filepath.Join(dir, "bin", "current", fmt.Sprintf("sing-box-%s-%s", osName, archName))
	writeFile(t, poolV1, "v1")
	writeFile(t, poolV2, "v2")

	// 激活 v1。
	prev, err := m.Activate(context.Background(), specV1)
	if err != nil {
		t.Fatalf("Activate v1: %v", err)
	}
	if prev != "" {
		t.Fatalf("首次激活 prev 应为空: %q", prev)
	}
	if got := readFile(t, role); got != "v1" {
		t.Fatalf("role 应为 v1: %q", got)
	}

	// 激活 v2，prev 应为 v1 pool 路径。
	prev, err = m.Activate(context.Background(), specV2)
	if err != nil {
		t.Fatalf("Activate v2: %v", err)
	}
	if prev != poolV1 {
		t.Fatalf("prev 应为 v1 pool 路径: %q", prev)
	}
	if got := readFile(t, role); got != "v2" {
		t.Fatalf("role 应为 v2: %q", got)
	}

	// 回滚到 v1。
	if err := m.Rollback(context.Background(), prev); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if got := readFile(t, role); got != "v1" {
		t.Fatalf("回滚后 role 应为 v1: %q", got)
	}
}

func TestEnsureUsesLocalPool(t *testing.T) {
	dir := t.TempDir()
	osName, archName := runtime.GOOS, runtime.GOARCH
	spec := Spec{OS: osName, Arch: archName, Version: "1.0.0"}
	m := NewManager(Config{DataDir: dir}, nil)
	pool := filepath.Join(dir, "bin", "pool", fmt.Sprintf("sing-box-1.0.0-%s-%s", osName, archName))
	writeFile(t, pool, "x")

	got, err := m.Ensure(context.Background(), spec)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got != pool {
		t.Fatalf("Ensure 应返回本地 pool 路径: %q", got)
	}
}

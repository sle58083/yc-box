package singbox

import (
	"context"
	"testing"
)

func TestCheckNonexistentBinary(t *testing.T) {
	err := Check(context.Background(), "/nonexistent/sing-box", "/tmp/x.json")
	if err == nil {
		t.Fatalf("不存在的二进制应返回错误")
	}
}

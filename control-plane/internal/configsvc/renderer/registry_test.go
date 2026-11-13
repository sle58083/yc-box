package renderer

import (
	"testing"
)

func TestRegistryDispatch(t *testing.T) {
	for _, proto := range []string{"vless", "vmess", "trojan", "shadowsocks", "hysteria2", "tuic", "anytls", "direct", "block", "selector", "urltest"} {
		if _, ok := Get(proto); !ok {
			t.Fatalf("协议 %q 未注册", proto)
		}
	}
	if _, ok := Get("foobar"); ok {
		t.Fatalf("未注册协议 foobar 不应命中")
	}
}

func TestRegistryList(t *testing.T) {
	list := List()
	if len(list) < 7 {
		t.Fatalf("注册协议数异常: %v", list)
	}
	seen := map[string]bool{}
	for _, p := range list {
		if seen[p] {
			t.Fatalf("协议重复: %s", p)
		}
		seen[p] = true
	}
}

func TestMetaDirections(t *testing.T) {
	if r, ok := Get("direct"); ok {
		m := r.Meta()
		if !hasDirection(m, DirectionOutbound) || hasDirection(m, DirectionInbound) {
			t.Fatalf("direct 应仅支持 outbound: %v", m.Directions)
		}
	}
}

// TestMetaDefaults 验证关键枚举字段标注了默认值（供前端展示 + 提交兜底）。
func TestMetaDefaults(t *testing.T) {
	assertFieldDefault := func(proto, dir, key, want string) {
		t.Helper()
		r, ok := Get(proto)
		if !ok {
			t.Fatalf("协议 %q 未注册", proto)
		}
		m := r.Meta()
		fields := m.Inbound
		if dir == "outbound" {
			fields = m.Outbound
		}
		for _, f := range fields {
			if f.Key == key {
				if string(f.Default) != want {
					t.Fatalf("%s %s.%s Default = %s，期望 %s", proto, dir, key, f.Default, want)
				}
				return
			}
		}
		t.Fatalf("%s %s 缺少字段 %s", proto, dir, key)
	}

	assertFieldDefault("shadowsocks", "inbound", "method", `"aes-128-gcm"`)
	assertFieldDefault("shadowsocks", "outbound", "method", `"aes-128-gcm"`)
	assertFieldDefault("tuic", "inbound", "congestion_control", `"cubic"`)
	assertFieldDefault("tuic", "outbound", "congestion_control", `"cubic"`)
	assertFieldDefault("vmess", "inbound", "alter_id", `0`)
	assertFieldDefault("vmess", "outbound", "alter_id", `0`)
}

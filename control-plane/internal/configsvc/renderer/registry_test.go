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

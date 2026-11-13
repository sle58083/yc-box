package subscribe

import (
	"encoding/json"
	"testing"

	"github.com/singbox-panel/control-plane/internal/model"
)

func TestBuildClientOutbound(t *testing.T) {
	ib := model.InboundConfig{
		Name:      "in-vless",
		Type:      "vless",
		Port:      443,
		Settings:  json.RawMessage(`{"flow":"xtls-rprx-vision"}`),
		TLS:       json.RawMessage(`{"enabled":true,"server_name":"example.com"}`),
		Transport: json.RawMessage(`{"type":"ws","path":"/ws"}`),
	}
	n := model.Node{Name: "hk", Address: "hk.example.com"}

	ob, err := buildClientOutbound(ib, n, "uuid-1", "pw-1")
	if err != nil {
		t.Fatalf("buildClientOutbound: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(ob.Settings, &settings); err != nil {
		t.Fatalf("settings 解析: %v", err)
	}
	if settings["server"] != "hk.example.com" {
		t.Fatalf("server 注入错误: %v", settings["server"])
	}
	if settings["uuid"] != "uuid-1" || settings["password"] != "pw-1" {
		t.Fatalf("UUID/密码解密注入错误: %v %v", settings["uuid"], settings["password"])
	}
	if settings["flow"] != "xtls-rprx-vision" {
		t.Fatalf("flow 应保留: %v", settings["flow"])
	}
	if ob.TLS == nil || string(ob.TLS) == "" {
		t.Fatalf("客户端 TLS 应生成")
	}
	if ob.Transport == nil || string(ob.Transport) == "" {
		t.Fatalf("transport 应透传")
	}
}

func TestFirstN(t *testing.T) {
	if firstN("abcdef", 4) != "abcd" {
		t.Fatalf("firstN 错误")
	}
	if firstN("ab", 4) != "ab" {
		t.Fatalf("firstN 短串错误")
	}
}

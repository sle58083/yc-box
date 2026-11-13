package configsvc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/singbox-panel/control-plane/internal/model"
)

func TestGenerateDispatchesViaRegistry(t *testing.T) {
	g := NewGenerator()
	ctx := &NodeConfigContext{
		Global: DefaultGlobalSettings(),
		Inbounds: []model.InboundConfig{{
			Type:     "anytls",
			Name:     "in-anytls",
			Listen:   "0.0.0.0",
			Port:     443,
			Settings: json.RawMessage(`{"users":[{"name":"alice","password":"secret"}],"padding_scheme":["tls-tcp"]}`),
			TLS:      json.RawMessage(`{"enabled":true,"certificate_path":"/c.pem","key_path":"/k.pem"}`),
			Status:   model.StatusEnabled,
		}},
		Outbounds: []model.OutboundConfig{
			{Type: "direct", Name: "direct", Status: model.StatusEnabled},
			{Type: "selector", Name: "select", Settings: json.RawMessage(`{"outbounds":["direct"],"default":"direct"}`), Status: model.StatusEnabled},
			{Type: "anytls", Name: "proxy-hk",
				Settings: json.RawMessage(`{"server":"hk.example.com","server_port":443,"password":"secret","idle_session_timeout":"30s"}`),
				TLS:      json.RawMessage(`{"enabled":true,"server_name":"hk.example.com"}`),
				Status:   model.StatusEnabled},
		},
	}
	doc, err := g.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	inbounds := doc["inbounds"].([]map[string]any)
	if len(inbounds) != 1 || inbounds[0]["type"] != "anytls" {
		t.Fatalf("inbounds 渲染错误: %#v", inbounds)
	}
	outbounds := doc["outbounds"].([]map[string]any)
	if len(outbounds) != 3 {
		t.Fatalf("outbounds 渲染错误: %#v", outbounds)
	}
	// route.final 应自动推导为 selector tag
	route := doc["route"].(map[string]any)
	if route["final"] != "select" {
		t.Fatalf("route.final 应推导为 selector: %v", route["final"])
	}
}

func TestGenerateFiltersDisabled(t *testing.T) {
	g := NewGenerator()
	ctx := &NodeConfigContext{
		Global: DefaultGlobalSettings(),
		Inbounds: []model.InboundConfig{
			{Type: "vless", Name: "enabled-in", Port: 443, Settings: json.RawMessage(`{"users":[{"uuid":"c1d5f0a0-0000-4000-8000-000000000001"}]}`), Status: model.StatusEnabled},
			{Type: "vless", Name: "disabled-in", Port: 8443, Status: model.StatusDisabled},
		},
		Outbounds: []model.OutboundConfig{
			{Type: "direct", Name: "direct", Status: model.StatusEnabled},
			{Type: "block", Name: "block-disabled", Status: model.StatusDisabled},
		},
	}
	doc, err := g.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(doc["inbounds"].([]map[string]any)) != 1 {
		t.Fatalf("disabled inbound 应被过滤")
	}
	if len(doc["outbounds"].([]map[string]any)) != 1 {
		t.Fatalf("disabled outbound 应被过滤")
	}
}

func TestGenerateAssertsUniqueTags(t *testing.T) {
	g := NewGenerator()
	ctx := &NodeConfigContext{
		Global: DefaultGlobalSettings(),
		Inbounds: []model.InboundConfig{
			{Type: "vless", Name: "dup", Port: 443, Status: model.StatusEnabled},
		},
		Outbounds: []model.OutboundConfig{
			{Type: "direct", Name: "dup", Status: model.StatusEnabled},
		},
	}
	if _, err := g.Generate(ctx); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("应断言 tag 全局唯一: %v", err)
	}
}

func TestGenerateUnknownProtocol(t *testing.T) {
	g := NewGenerator()
	ctx := &NodeConfigContext{
		Global:   DefaultGlobalSettings(),
		Inbounds: []model.InboundConfig{{Type: "foobar", Name: "in", Port: 443, Status: model.StatusEnabled}},
	}
	if _, err := g.Generate(ctx); err == nil || !strings.Contains(err.Error(), "unknown protocol") {
		t.Fatalf("应返回未知协议错误: %v", err)
	}
}

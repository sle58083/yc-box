package renderer

import (
	"strings"
	"testing"

	"github.com/singbox-panel/control-plane/internal/model"
)

func TestVLESSRenderOutboundTLSAndTransport(t *testing.T) {
	r := &VLESSRenderer{}
	cfg := model.OutboundConfig{
		Type:      "vless",
		Name:      "proxy-hk",
		Settings:  raw(`{"server":"hk.example.com","server_port":443,"uuid":"c1d5f0a0-0000-4000-8000-000000000001"}`),
		TLS:       raw(`{"enabled":true,"server_name":"hk.example.com"}`),
		Transport: raw(`{"type":"ws","path":"/ws","headers":{"Host":"hk.example.com"}}`),
	}
	out, err := r.RenderOutbound(&RenderContext{}, cfg)
	if err != nil {
		t.Fatalf("RenderOutbound: %v", err)
	}
	if out["tls"] == nil || out["transport"] == nil {
		t.Fatalf("出站 tls/transport 未渲染: %#v", out)
	}
	if out["server"] != "hk.example.com" {
		t.Fatalf("server 渲染错误: %v", out["server"])
	}
}

func TestVLESSValidateInboundRejectsRealityOnWrongTLS(t *testing.T) {
	r := &VLESSRenderer{}
	cfg := model.InboundConfig{
		Type: "vless", Name: "in", Port: 443,
		Settings: raw(`{"users":[{"uuid":"c1d5f0a0-0000-4000-8000-000000000001"}]}`),
		TLS:      raw(`{"enabled":true,"reality":{"enabled":true}}`),
	}
	if err := r.ValidateInbound(cfg); err != nil {
		t.Fatalf("vless 应允许 Reality: %v", err)
	}
}

func TestTrojanValidateRequiresTLS(t *testing.T) {
	r := &TrojanRenderer{}
	cfg := model.OutboundConfig{
		Type:     "trojan",
		Name:     "out",
		Settings: raw(`{"server":"h","server_port":443,"password":"p"}`),
	}
	if err := r.ValidateOutbound(cfg); err == nil || !strings.Contains(err.Error(), "enabled") {
		t.Fatalf("trojan 应强制 TLS: %v", err)
	}
}

package configsvc

import (
	"encoding/json"
	"testing"

	"github.com/singbox-panel/control-plane/internal/model"
)

// TestSchemaValidateGeneratedConfig 用真实 schema 片段校验生成的 anytls 配置。
func TestSchemaValidateGeneratedConfig(t *testing.T) {
	v, err := NewValidator("../../singbox-schema")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	g := NewGenerator()
	ctx := &NodeConfigContext{
		Global: DefaultGlobalSettings(),
		Inbounds: []model.InboundConfig{{
			Type: "anytls", Name: "in-anytls", Listen: "0.0.0.0", Port: 443,
			Settings: json.RawMessage(`{"users":[{"name":"a","password":"p"}]}`),
			TLS:      json.RawMessage(`{"enabled":true,"certificate_path":"/c","key_path":"/k"}`),
			Status:   model.StatusEnabled,
		}},
		Outbounds: []model.OutboundConfig{
			{Type: "direct", Name: "direct", Status: model.StatusEnabled},
			{Type: "anytls", Name: "proxy",
				Settings: json.RawMessage(`{"server":"h","server_port":443,"password":"p"}`),
				TLS:      json.RawMessage(`{"enabled":true,"server_name":"h"}`),
				Status:   model.StatusEnabled},
		},
	}
	doc, err := g.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := v.ValidateDocument(doc); err != nil {
		t.Fatalf("ValidateDocument: %v", err)
	}
}

// TestSchemaValidateRejectsUnknownType 未注册协议无 schema 片段时应报错。
func TestSchemaValidateRejectsUnknownType(t *testing.T) {
	v, err := NewValidator("../../singbox-schema")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	doc := map[string]any{
		"inbounds": []any{map[string]any{"type": "foobar", "tag": "x", "listen": "0.0.0.0", "listen_port": 443}},
	}
	if err := v.ValidateDocument(doc); err == nil {
		t.Fatalf("应因缺少 schema 片段而报错")
	}
}

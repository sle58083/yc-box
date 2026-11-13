package renderer

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/singbox-panel/control-plane/internal/model"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func TestAnyTLSRenderInboundBase64Password(t *testing.T) {
	r := &AnyTLSRenderer{}
	cfg := model.InboundConfig{
		Type:     "anytls",
		Name:     "in-anytls",
		Listen:   "0.0.0.0",
		Port:     443,
		Settings: raw(`{"users":[{"name":"alice","password":"secret"}],"padding_scheme":["tls-tcp"]}`),
		TLS:      raw(`{"enabled":true,"certificate_path":"/c.pem","key_path":"/k.pem"}`),
	}
	out, err := r.RenderInbound(&RenderContext{}, cfg, nil)
	if err != nil {
		t.Fatalf("RenderInbound: %v", err)
	}
	users, ok := out["users"].([]map[string]any)
	if !ok || len(users) != 1 {
		t.Fatalf("users 渲染错误: %#v", out["users"])
	}
	want := base64.StdEncoding.EncodeToString([]byte("secret"))
	if users[0]["password"] != want {
		t.Fatalf("password 应为 base64 编码: got %v want %v", users[0]["password"], want)
	}
	if _, ok := out["padding_scheme"].([]any); !ok {
		t.Fatalf("padding_scheme 应仅 inbound 且为数组: %#v", out["padding_scheme"])
	}
}

func TestAnyTLSRenderInboundInjectsProxyUsers(t *testing.T) {
	r := &AnyTLSRenderer{}
	cfg := model.InboundConfig{
		Type:          "anytls",
		Name:          "in-anytls",
		Listen:        "0.0.0.0",
		Port:          443,
		IsUserInbound: true,
		TLS:           raw(`{"enabled":true,"certificate_path":"/c.pem","key_path":"/k.pem"}`),
	}
	users := []ProxyUser{{Name: "user-1", Password: "pw"}}
	out, err := r.RenderInbound(&RenderContext{Users: users}, cfg, users)
	if err != nil {
		t.Fatalf("RenderInbound: %v", err)
	}
	us := out["users"].([]map[string]any)
	if us[0]["password"] != base64.StdEncoding.EncodeToString([]byte("pw")) {
		t.Fatalf("proxy user password 应 base64 编码")
	}
}

func TestAnyTLSRenderOutboundSessionPool(t *testing.T) {
	r := &AnyTLSRenderer{}
	cfg := model.OutboundConfig{
		Type: "anytls",
		Name: "out-anytls",
		Settings: raw(`{"server":"hk.example.com","server_port":443,"password":"secret",
			"idle_session_check_interval":"30s","idle_session_timeout":"30s","min_idle_session":5,"client_metadata":"m"}`),
		TLS: raw(`{"enabled":true,"server_name":"hk.example.com"}`),
	}
	out, err := r.RenderOutbound(&RenderContext{}, cfg)
	if err != nil {
		t.Fatalf("RenderOutbound: %v", err)
	}
	if out["password"] != base64.StdEncoding.EncodeToString([]byte("secret")) {
		t.Fatalf("outbound password 应 base64 编码")
	}
	for _, k := range []string{"idle_session_check_interval", "idle_session_timeout", "min_idle_session", "client_metadata"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("outbound 缺少 %s", k)
		}
	}
	// 会话池字段不应出现在 inbound（此用例为 outbound，仅验证 out 结构）
	if _, ok := out["padding_scheme"]; ok {
		t.Fatalf("outbound 不应有 padding_scheme")
	}
}

func TestAnyTLSValidateInboundRejectsRealityAndECH(t *testing.T) {
	r := &AnyTLSRenderer{}
	base := model.InboundConfig{
		Type: "anytls", Name: "in", Listen: "0.0.0.0", Port: 443,
		Settings: raw(`{"users":[{"name":"a","password":"p"}]}`),
	}
	base.TLS = raw(`{"enabled":true,"certificate_path":"/c","key_path":"/k","reality":{"enabled":true}}`)
	if err := r.ValidateInbound(base); err == nil || !strings.Contains(err.Error(), "Reality") {
		t.Fatalf("应拒绝 Reality: %v", err)
	}
	base.TLS = raw(`{"enabled":true,"certificate_path":"/c","key_path":"/k","ech":{"enabled":true}}`)
	if err := r.ValidateInbound(base); err == nil || !strings.Contains(err.Error(), "ECH") {
		t.Fatalf("应拒绝 ECH: %v", err)
	}
}

func TestAnyTLSValidateInboundRequiresTLS(t *testing.T) {
	r := &AnyTLSRenderer{}
	cfg := model.InboundConfig{Type: "anytls", Name: "in", Port: 443, Settings: raw(`{"users":[{"name":"a","password":"p"}]}`)}
	if err := r.ValidateInbound(cfg); err == nil || !strings.Contains(err.Error(), "enabled") {
		t.Fatalf("应要求 tls.enabled=true: %v", err)
	}
}

func TestAnyTLSValidateOutboundSessionPool(t *testing.T) {
	r := &AnyTLSRenderer{}
	base := model.OutboundConfig{
		Type: "anytls", Name: "out",
		Settings: raw(`{"server":"h","server_port":443,"password":"p"}`),
		TLS:      raw(`{"enabled":true,"server_name":"h"}`),
	}
	if err := r.ValidateOutbound(base); err != nil {
		t.Fatalf("合法 outbound 应通过: %v", err)
	}
	base.Settings = raw(`{"server":"h","server_port":443,"password":"p","idle_session_timeout":"notaduration"}`)
	if err := r.ValidateOutbound(base); err == nil || !strings.Contains(err.Error(), "duration") {
		t.Fatalf("应拒绝非法 duration: %v", err)
	}
	base.Settings = raw(`{"server":"h","server_port":443,"password":"p","min_idle_session":-1}`)
	if err := r.ValidateOutbound(base); err == nil || !strings.Contains(err.Error(), "min_idle_session") {
		t.Fatalf("应拒绝负数 min_idle_session: %v", err)
	}
}

func TestAnyTLSSubscriptionURI(t *testing.T) {
	r := &AnyTLSRenderer{}
	cfg := model.OutboundConfig{
		Type: "anytls", Name: "out-anytls",
		Settings: raw(`{"server":"hk.example.com","server_port":443,"password":"secret"}`),
		TLS:      raw(`{"enabled":true,"server_name":"hk.example.com"}`),
	}
	uri, err := r.SubscriptionURI(cfg, ProxyUser{Password: "secret"})
	if err != nil {
		t.Fatalf("SubscriptionURI: %v", err)
	}
	if !strings.HasPrefix(uri, "anytls://") || !strings.Contains(uri, "sni=hk.example.com") {
		t.Fatalf("订阅 URI 格式错误: %s", uri)
	}
}

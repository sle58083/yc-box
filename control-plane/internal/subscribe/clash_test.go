package subscribe

import (
	"encoding/json"
	"testing"

	"github.com/singbox-panel/control-plane/internal/model"
)

func j(s string) json.RawMessage { return json.RawMessage(s) }

func TestProxyFromOutboundProtocols(t *testing.T) {
	cases := []struct {
		name  string
		cfg   model.OutboundConfig
		check func(t *testing.T, p map[string]any)
	}{
		{
			name: "vless-ws-tls",
			cfg: model.OutboundConfig{
				Type:      "vless",
				Settings:  j(`{"server":"hk.example.com","server_port":443,"uuid":"c1d5f0a0-0000-4000-8000-000000000001","flow":"xtls-rprx-vision"}`),
				TLS:       j(`{"enabled":true,"server_name":"hk.example.com"}`),
				Transport: j(`{"type":"ws","path":"/ws","headers":{"Host":"hk.example.com"}}`),
			},
			check: func(t *testing.T, p map[string]any) {
				if p["type"] != "vless" || p["uuid"] == "" {
					t.Fatalf("vless 映射错误: %#v", p)
				}
				if p["flow"] != "xtls-rprx-vision" || p["tls"] != true || p["servername"] != "hk.example.com" {
					t.Fatalf("vless flow/tls/servername 映射错误: %#v", p)
				}
				if p["network"] != "ws" {
					t.Fatalf("vless network 映射错误: %#v", p)
				}
				if ws, _ := p["ws-opts"].(map[string]any); ws == nil || ws["path"] != "/ws" {
					t.Fatalf("vless ws-opts 映射错误: %#v", p)
				}
			},
		},
		{
			name: "vmess-ws",
			cfg: model.OutboundConfig{
				Type:      "vmess",
				Settings:  j(`{"server":"hk.example.com","server_port":443,"uuid":"c1d5f0a0-0000-4000-8000-000000000001","alter_id":0}`),
				Transport: j(`{"type":"ws","path":"/ws"}`),
			},
			check: func(t *testing.T, p map[string]any) {
				if p["type"] != "vmess" || p["cipher"] != "auto" || p["alterId"] != 0 {
					t.Fatalf("vmess 映射错误: %#v", p)
				}
				if _, hasTLS := p["tls"]; hasTLS {
					t.Fatalf("vmess 不应输出 tls: %#v", p)
				}
				if p["network"] != "ws" {
					t.Fatalf("vmess network 映射错误: %#v", p)
				}
			},
		},
		{
			name: "trojan",
			cfg: model.OutboundConfig{
				Type:     "trojan",
				Settings: j(`{"server":"hk.example.com","server_port":443,"password":"secret"}`),
				TLS:      j(`{"enabled":true,"server_name":"hk.example.com"}`),
			},
			check: func(t *testing.T, p map[string]any) {
				if p["type"] != "trojan" || p["password"] != "secret" || p["sni"] != "hk.example.com" {
					t.Fatalf("trojan 映射错误: %#v", p)
				}
				if _, hasTLS := p["tls"]; hasTLS {
					t.Fatalf("trojan 不应输出显式 tls: %#v", p)
				}
			},
		},
		{
			name: "shadowsocks",
			cfg: model.OutboundConfig{
				Type:     "shadowsocks",
				Settings: j(`{"server":"hk.example.com","server_port":8388,"method":"aes-128-gcm","password":"secret"}`),
			},
			check: func(t *testing.T, p map[string]any) {
				if p["type"] != "ss" || p["cipher"] != "aes-128-gcm" || p["password"] != "secret" {
					t.Fatalf("shadowsocks 映射错误: %#v", p)
				}
			},
		},
		{
			name: "hysteria2",
			cfg: model.OutboundConfig{
				Type:     "hysteria2",
				Settings: j(`{"server":"hk.example.com","server_port":443,"password":"secret"}`),
				TLS:      j(`{"enabled":true,"server_name":"hk.example.com","insecure":true}`),
			},
			check: func(t *testing.T, p map[string]any) {
				if p["type"] != "hysteria2" || p["password"] != "secret" || p["sni"] != "hk.example.com" {
					t.Fatalf("hysteria2 映射错误: %#v", p)
				}
				if p["skip-cert-verify"] != true {
					t.Fatalf("hysteria2 skip-cert-verify 映射错误: %#v", p)
				}
			},
		},
		{
			name: "tuic",
			cfg: model.OutboundConfig{
				Type:     "tuic",
				Settings: j(`{"server":"hk.example.com","server_port":443,"uuid":"c1d5f0a0-0000-4000-8000-000000000001","password":"secret","congestion_control":"bbr"}`),
				TLS:      j(`{"enabled":true,"server_name":"hk.example.com"}`),
			},
			check: func(t *testing.T, p map[string]any) {
				if p["type"] != "tuic" || p["uuid"] == "" || p["password"] != "secret" {
					t.Fatalf("tuic 映射错误: %#v", p)
				}
				if p["congestion-controller"] != "bbr" || p["udp-relay-mode"] != "native" {
					t.Fatalf("tuic 拥塞/UDP 映射错误: %#v", p)
				}
			},
		},
		{
			name: "anytls",
			cfg: model.OutboundConfig{
				Type:     "anytls",
				Settings: j(`{"server":"hk.example.com","server_port":443,"password":"secret"}`),
				TLS:      j(`{"enabled":true,"server_name":"hk.example.com"}`),
			},
			check: func(t *testing.T, p map[string]any) {
				if p["type"] != "anytls" || p["password"] != "secret" || p["sni"] != "hk.example.com" {
					t.Fatalf("anytls 映射错误: %#v", p)
				}
			},
		},
		{
			name: "unsupported",
			cfg:  model.OutboundConfig{Type: "direct"},
			check: func(t *testing.T, p map[string]any) {
				if p != nil {
					t.Fatalf("不支持的协议应返回 nil: %#v", p)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := proxyFromOutbound(tc.cfg, "proxy")
			if tc.check == nil {
				return
			}
			tc.check(t, p)
		})
	}
}

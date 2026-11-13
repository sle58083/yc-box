package renderer

import (
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/singbox-panel/control-plane/internal/model"
)

// ShadowsocksRenderer 实现 shadowsocks 协议渲染（单 method+password，无原生 TLS）。
type ShadowsocksRenderer struct{}

func (r *ShadowsocksRenderer) Protocol() string { return "shadowsocks" }

func (r *ShadowsocksRenderer) RenderInbound(_ *RenderContext, cfg model.InboundConfig, _ []ProxyUser) (map[string]any, error) {
	out := map[string]any{}
	renderInboundCommon(out, cfg)

	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return nil, err
	}
	out[fMethod] = str(settings, fMethod)
	out[fPassword] = str(settings, fPassword)

	if tr, err := renderTransport(cfg.Transport); err != nil {
		return nil, err
	} else if tr != nil {
		out[fTransport] = tr
	}
	return out, nil
}

func (r *ShadowsocksRenderer) RenderOutbound(_ *RenderContext, cfg model.OutboundConfig) (map[string]any, error) {
	out := map[string]any{}
	renderOutboundCommon(out, cfg)

	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return nil, err
	}
	out[fServer] = str(settings, fServer)
	if v := settings[fServerPort]; v != nil {
		out[fServerPort] = v
	}
	out[fMethod] = str(settings, fMethod)
	out[fPassword] = str(settings, fPassword)
	return out, nil
}

func (r *ShadowsocksRenderer) ValidateInbound(cfg model.InboundConfig) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("shadowsocks inbound: listen_port 必须在 1-65535")
	}
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	if str(settings, fMethod) == "" {
		return fmt.Errorf("shadowsocks inbound: method 必填")
	}
	if str(settings, fPassword) == "" {
		return fmt.Errorf("shadowsocks inbound: password 必填")
	}
	return nil
}

func (r *ShadowsocksRenderer) ValidateOutbound(cfg model.OutboundConfig) error {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	if str(settings, fServer) == "" || intVal(settings, fServerPort) <= 0 {
		return fmt.Errorf("shadowsocks outbound: server/server_port 必填")
	}
	if str(settings, fMethod) == "" || str(settings, fPassword) == "" {
		return fmt.Errorf("shadowsocks outbound: method/password 必填")
	}
	return nil
}

func (r *ShadowsocksRenderer) SubscriptionURI(cfg model.OutboundConfig, _ ProxyUser) (string, error) {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return "", err
	}
	server, port := str(settings, fServer), portStr(settings, fServerPort)
	method, password := str(settings, fMethod), str(settings, fPassword)
	if server == "" || port == "" || method == "" || password == "" {
		return "", fmt.Errorf("shadowsocks: 订阅 URI 缺少 server/port/method/password")
	}
	userinfo := base64.StdEncoding.EncodeToString([]byte(method + ":" + password))
	return fmt.Sprintf("ss://%s@%s:%s#%s", userinfo, server, port, url.PathEscape(cfg.Name)), nil
}

func (r *ShadowsocksRenderer) Meta() ProtocolMeta {
	return ProtocolMeta{
		Protocol:    "shadowsocks",
		Directions:  []Direction{DirectionInbound, DirectionOutbound},
		TLSAllowed:  []TLSMode{TLSModeNone},
		TLSRequired: false,
		MultiUser:   false,
		Inbound: []FieldSchema{
			{Key: "method", Label: "加密方式", Type: "select", Required: true, Default: defaultJSON("aes-128-gcm"), Options: []FieldOption{
				{Label: "2022-blake3-aes-128-gcm", Value: "2022-blake3-aes-128-gcm"},
				{Label: "2022-blake3-aes-256-gcm", Value: "2022-blake3-aes-256-gcm"},
				{Label: "aes-128-gcm", Value: "aes-128-gcm"},
				{Label: "aes-256-gcm", Value: "aes-256-gcm"},
				{Label: "chacha20-ietf-poly1305", Value: "chacha20-ietf-poly1305"},
			}},
			{Key: "password", Label: "密码", Type: "text", Required: true, Encrypt: true,
				Help: "2022-blake3 系列需填 base64 编码的密钥（16/32 字节），非任意字符串"},
		},
		Outbound: []FieldSchema{
			{Key: "server", Label: "服务器地址", Type: "text", Required: true},
			{Key: "server_port", Label: "端口", Type: "number", Required: true},
			{Key: "method", Label: "加密方式", Type: "select", Required: true, Default: defaultJSON("aes-128-gcm"), Options: []FieldOption{
				{Label: "2022-blake3-aes-128-gcm", Value: "2022-blake3-aes-128-gcm"},
				{Label: "2022-blake3-aes-256-gcm", Value: "2022-blake3-aes-256-gcm"},
				{Label: "aes-128-gcm", Value: "aes-128-gcm"},
				{Label: "aes-256-gcm", Value: "aes-256-gcm"},
				{Label: "chacha20-ietf-poly1305", Value: "chacha20-ietf-poly1305"},
			}},
			{Key: "password", Label: "密码", Type: "text", Required: true, Encrypt: true},
		},
	}
}

func init() { Register(&ShadowsocksRenderer{}) }

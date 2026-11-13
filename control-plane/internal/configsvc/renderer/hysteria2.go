package renderer

import (
	"fmt"
	"net/url"

	"github.com/singbox-panel/control-plane/internal/model"
)

// Hysteria2Renderer 实现 hysteria2 协议渲染（内置 QUIC TLS，必选 TLS，禁 Reality）。
type Hysteria2Renderer struct{}

func (r *Hysteria2Renderer) Protocol() string { return "hysteria2" }

func (r *Hysteria2Renderer) RenderInbound(_ *RenderContext, cfg model.InboundConfig, users []ProxyUser) (map[string]any, error) {
	out := map[string]any{}
	renderInboundCommon(out, cfg)

	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return nil, err
	}
	if cfg.IsUserInbound && len(users) > 0 {
		us := make([]map[string]any, 0, len(users))
		for _, u := range users {
			us = append(us, map[string]any{fName: u.Name, fPassword: u.Password})
		}
		out[fUsers] = us
	} else if static := parseUsers(settings); len(static) > 0 {
		out[fUsers] = static
	}

	if t, err := renderTLS(cfg.TLS); err != nil {
		return nil, err
	} else if t != nil {
		out[fTLS] = t
	}
	if m := str(settings, "masquerade"); m != "" {
		out["masquerade"] = m
	}
	return out, nil
}

func (r *Hysteria2Renderer) RenderOutbound(_ *RenderContext, cfg model.OutboundConfig) (map[string]any, error) {
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
	out[fPassword] = str(settings, fPassword)

	if t, err := renderTLS(cfg.TLS); err != nil {
		return nil, err
	} else if t != nil {
		out[fTLS] = t
	}
	return out, nil
}

func (r *Hysteria2Renderer) ValidateInbound(cfg model.InboundConfig) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("hysteria2 inbound: listen_port 必须在 1-65535")
	}
	if err := validateInboundTLSCommon(cfg, []TLSMode{TLSModePlain}, true); err != nil {
		return err
	}
	if cfg.IsUserInbound {
		return nil
	}
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	us := parseUsers(settings)
	if len(us) == 0 {
		return fmt.Errorf("hysteria2 inbound: settings.users 不能为空（或标记 is_user_inbound）")
	}
	for i, u := range us {
		if str(u, fPassword) == "" {
			return fmt.Errorf("hysteria2 inbound: users[%d].password 不能为空", i)
		}
	}
	return nil
}

func (r *Hysteria2Renderer) ValidateOutbound(cfg model.OutboundConfig) error {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	if str(settings, fServer) == "" || intVal(settings, fServerPort) <= 0 {
		return fmt.Errorf("hysteria2 outbound: server/server_port 必填")
	}
	if str(settings, fPassword) == "" {
		return fmt.Errorf("hysteria2 outbound: password 必填")
	}
	return validateOutboundTLSCommon(cfg, []TLSMode{TLSModePlain}, true)
}

func (r *Hysteria2Renderer) SubscriptionURI(cfg model.OutboundConfig, user ProxyUser) (string, error) {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return "", err
	}
	server, port := str(settings, fServer), portStr(settings, fServerPort)
	if server == "" || port == "" || user.Password == "" {
		return "", fmt.Errorf("hysteria2: 订阅 URI 缺少 server/port/password")
	}
	q := url.Values{}
	if sni := sniFromTLS(cfg); sni != "" {
		q.Set("sni", sni)
	}
	return fmt.Sprintf("hysteria2://%s@%s:%s?%s#%s", url.PathEscape(user.Password), server, port, q.Encode(), cfg.Name), nil
}

func (r *Hysteria2Renderer) Meta() ProtocolMeta {
	return ProtocolMeta{
		Protocol:    "hysteria2",
		Directions:  []Direction{DirectionInbound, DirectionOutbound},
		TLSAllowed:  []TLSMode{TLSModePlain},
		TLSRequired: true,
		MultiUser:   true,
		Inbound: []FieldSchema{
			{Key: "password", Label: "密码", Type: "text", Required: true, Encrypt: true},
			{Key: "masquerade", Label: "Masquerade", Type: "text", Required: false},
		},
		Outbound: []FieldSchema{
			{Key: "server", Label: "服务器地址", Type: "text", Required: true},
			{Key: "server_port", Label: "端口", Type: "number", Required: true},
			{Key: "password", Label: "密码", Type: "text", Required: true, Encrypt: true},
		},
	}
}

func init() { Register(&Hysteria2Renderer{}) }

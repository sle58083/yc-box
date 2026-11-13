package renderer

import (
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// TUICRenderer 实现 tuic 协议渲染（内置 QUIC TLS，禁嵌套 transport / Reality）。
type TUICRenderer struct{}

func (r *TUICRenderer) Protocol() string { return "tuic" }

func (r *TUICRenderer) RenderInbound(_ *RenderContext, cfg model.InboundConfig, users []ProxyUser) (map[string]any, error) {
	out := map[string]any{}
	renderInboundCommon(out, cfg)

	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return nil, err
	}
	if cfg.IsUserInbound && len(users) > 0 {
		pwd := str(settings, fPassword)
		us := make([]map[string]any, 0, len(users))
		for _, u := range users {
			m := map[string]any{fName: u.Name, fUUID: u.UUID}
			if u.Password != "" {
				m[fPassword] = u.Password
			} else if pwd != "" {
				m[fPassword] = pwd
			}
			us = append(us, m)
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
	if cc := str(settings, "congestion_control"); cc != "" {
		out["congestion_control"] = cc
	}
	return out, nil
}

func (r *TUICRenderer) RenderOutbound(_ *RenderContext, cfg model.OutboundConfig) (map[string]any, error) {
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
	out[fUUID] = str(settings, fUUID)
	out[fPassword] = str(settings, fPassword)
	if cc := str(settings, "congestion_control"); cc != "" {
		out["congestion_control"] = cc
	}

	if t, err := renderTLS(cfg.TLS); err != nil {
		return nil, err
	} else if t != nil {
		out[fTLS] = t
	}
	return out, nil
}

func (r *TUICRenderer) ValidateInbound(cfg model.InboundConfig) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("tuic inbound: listen_port 必须在 1-65535")
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
		return fmt.Errorf("tuic inbound: settings.users 不能为空（或标记 is_user_inbound）")
	}
	for i, u := range us {
		if _, err := uuid.Parse(str(u, fUUID)); err != nil {
			return fmt.Errorf("tuic inbound: users[%d].uuid 非法: %w", i, err)
		}
		if str(u, fPassword) == "" {
			return fmt.Errorf("tuic inbound: users[%d].password 不能为空", i)
		}
	}
	return nil
}

func (r *TUICRenderer) ValidateOutbound(cfg model.OutboundConfig) error {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	if str(settings, fServer) == "" || intVal(settings, fServerPort) <= 0 {
		return fmt.Errorf("tuic outbound: server/server_port 必填")
	}
	if _, err := uuid.Parse(str(settings, fUUID)); err != nil {
		return fmt.Errorf("tuic outbound: uuid 非法: %w", err)
	}
	if str(settings, fPassword) == "" {
		return fmt.Errorf("tuic outbound: password 必填")
	}
	// tuic 自带 QUIC，禁止嵌套 transport。
	if len(cfg.Transport) > 0 && string(cfg.Transport) != "null" {
		return fmt.Errorf("tuic outbound: 内置 QUIC，不支持嵌套 transport")
	}
	return validateOutboundTLSCommon(cfg, []TLSMode{TLSModePlain}, true)
}

func (r *TUICRenderer) SubscriptionURI(cfg model.OutboundConfig, user ProxyUser) (string, error) {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return "", err
	}
	server, port := str(settings, fServer), portStr(settings, fServerPort)
	pwd := str(settings, fPassword)
	if pwd == "" {
		pwd = user.Password
	}
	if server == "" || port == "" || user.UUID == "" || pwd == "" {
		return "", fmt.Errorf("tuic: 订阅 URI 缺少 server/port/uuid/password")
	}
	q := url.Values{}
	if sni := sniFromTLS(cfg); sni != "" {
		q.Set("sni", sni)
	}
	return fmt.Sprintf("tuic://%s:%s@%s:%s?%s#%s", user.UUID, url.PathEscape(pwd), server, port, q.Encode(), cfg.Name), nil
}

func (r *TUICRenderer) Meta() ProtocolMeta {
	return ProtocolMeta{
		Protocol:    "tuic",
		Directions:  []Direction{DirectionInbound, DirectionOutbound},
		TLSAllowed:  []TLSMode{TLSModePlain},
		TLSRequired: true,
		MultiUser:   true,
		Inbound: []FieldSchema{
			{Key: "uuid", Label: "UUID", Type: "text", Required: true, Encrypt: true},
			{Key: "password", Label: "密码", Type: "text", Required: true, Encrypt: true},
			{Key: "congestion_control", Label: "拥塞控制", Type: "select", Required: false, Default: defaultJSON("cubic"), Options: []FieldOption{
				{Label: "cubic", Value: "cubic"},
				{Label: "new_reno", Value: "new_reno"},
				{Label: "bbr", Value: "bbr"},
			}},
		},
		Outbound: []FieldSchema{
			{Key: "server", Label: "服务器地址", Type: "text", Required: true},
			{Key: "server_port", Label: "端口", Type: "number", Required: true},
			{Key: "uuid", Label: "UUID", Type: "text", Required: true, Encrypt: true},
			{Key: "password", Label: "密码", Type: "text", Required: true, Encrypt: true},
			{Key: "congestion_control", Label: "拥塞控制", Type: "select", Required: false, Default: defaultJSON("cubic"), Options: []FieldOption{
				{Label: "cubic", Value: "cubic"},
				{Label: "new_reno", Value: "new_reno"},
				{Label: "bbr", Value: "bbr"},
			}},
		},
	}
}

func init() { Register(&TUICRenderer{}) }

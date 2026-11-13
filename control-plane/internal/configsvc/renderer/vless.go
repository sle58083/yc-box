package renderer

import (
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// VLESSRenderer 实现 vless 协议渲染。
type VLESSRenderer struct{}

func (r *VLESSRenderer) Protocol() string { return "vless" }

func (r *VLESSRenderer) RenderInbound(_ *RenderContext, cfg model.InboundConfig, users []ProxyUser) (map[string]any, error) {
	out := map[string]any{}
	renderInboundCommon(out, cfg)

	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return nil, err
	}
	// 用户入口：解密注入的 proxy users；静态入口：settings.users[]。
	if cfg.IsUserInbound && len(users) > 0 {
		flow := str(settings, fFlow)
		us := make([]map[string]any, 0, len(users))
		for _, u := range users {
			m := map[string]any{fName: u.Name, fUUID: u.UUID}
			if flow != "" {
				m[fFlow] = flow
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
	if tr, err := renderTransport(cfg.Transport); err != nil {
		return nil, err
	} else if tr != nil {
		out[fTransport] = tr
	}
	return out, nil
}

func (r *VLESSRenderer) RenderOutbound(_ *RenderContext, cfg model.OutboundConfig) (map[string]any, error) {
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
	if flow := str(settings, fFlow); flow != "" {
		out[fFlow] = flow
	}

	if t, err := renderTLS(cfg.TLS); err != nil {
		return nil, err
	} else if t != nil {
		out[fTLS] = t
	}
	if tr, err := renderTransport(cfg.Transport); err != nil {
		return nil, err
	} else if tr != nil {
		out[fTransport] = tr
	}
	return out, nil
}

func (r *VLESSRenderer) ValidateInbound(cfg model.InboundConfig) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("vless inbound: listen_port 必须在 1-65535")
	}
	if err := validateInboundTLSCommon(cfg, []TLSMode{TLSModeNone, TLSModePlain, TLSModeReality, TLSModeECH}, false); err != nil {
		return err
	}
	if cfg.IsUserInbound {
		return nil // 用户由代理用户注入
	}
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	us := parseUsers(settings)
	if len(us) == 0 {
		return fmt.Errorf("vless inbound: settings.users 不能为空（或标记 is_user_inbound）")
	}
	for i, u := range us {
		if _, err := uuid.Parse(str(u, fUUID)); err != nil {
			return fmt.Errorf("vless inbound: users[%d].uuid 非法: %w", i, err)
		}
	}
	return nil
}

func (r *VLESSRenderer) ValidateOutbound(cfg model.OutboundConfig) error {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	if str(settings, fServer) == "" || intVal(settings, fServerPort) <= 0 {
		return fmt.Errorf("vless outbound: server/server_port 必填")
	}
	if _, err := uuid.Parse(str(settings, fUUID)); err != nil {
		return fmt.Errorf("vless outbound: uuid 非法: %w", err)
	}
	return validateOutboundTLSCommon(cfg, []TLSMode{TLSModeNone, TLSModePlain, TLSModeReality, TLSModeECH}, false)
}

func (r *VLESSRenderer) SubscriptionURI(cfg model.OutboundConfig, user ProxyUser) (string, error) {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return "", err
	}
	server, port := str(settings, fServer), portStr(settings, fServerPort)
	if server == "" || port == "" || user.UUID == "" {
		return "", fmt.Errorf("vless: 订阅 URI 缺少 server/port/uuid")
	}
	q := url.Values{}
	q.Set("type", "tcp")
	q.Set("security", "tls")
	return fmt.Sprintf("vless://%s@%s:%s?%s#%s", user.UUID, server, port, q.Encode(), cfg.Name), nil
}

func (r *VLESSRenderer) Meta() ProtocolMeta {
	return ProtocolMeta{
		Protocol:    "vless",
		Directions:  []Direction{DirectionInbound, DirectionOutbound},
		TLSAllowed:  []TLSMode{TLSModeNone, TLSModePlain, TLSModeReality, TLSModeECH},
		TLSRequired: false,
		MultiUser:   true,
		Inbound: []FieldSchema{
			{Key: "flow", Label: "Flow", Type: "text", Required: false},
		},
		Outbound: []FieldSchema{
			{Key: "server", Label: "服务器地址", Type: "text", Required: true},
			{Key: "server_port", Label: "端口", Type: "number", Required: true},
			{Key: "uuid", Label: "UUID", Type: "text", Required: true, Encrypt: true},
			{Key: "flow", Label: "Flow", Type: "text", Required: false},
		},
	}
}

func init() { Register(&VLESSRenderer{}) }

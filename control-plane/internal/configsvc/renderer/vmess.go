package renderer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// VMessRenderer 实现 vmess 协议渲染（无原生 tls，TLS 走 transport 层）。
type VMessRenderer struct{}

func (r *VMessRenderer) Protocol() string { return "vmess" }

func (r *VMessRenderer) RenderInbound(_ *RenderContext, cfg model.InboundConfig, users []ProxyUser) (map[string]any, error) {
	out := map[string]any{}
	renderInboundCommon(out, cfg)

	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return nil, err
	}
	alterID := settings["alter_id"]
	if alterID == nil {
		alterID = 0
	}
	if cfg.IsUserInbound && len(users) > 0 {
		us := make([]map[string]any, 0, len(users))
		for _, u := range users {
			us = append(us, map[string]any{fName: u.Name, fUUID: u.UUID, "alter_id": alterID})
		}
		out[fUsers] = us
	} else if static := parseUsers(settings); len(static) > 0 {
		out[fUsers] = static
	}

	// vmess 无原生 tls 字段，TLS 仅在 transport 内。
	if tr, err := renderTransport(cfg.Transport); err != nil {
		return nil, err
	} else if tr != nil {
		out[fTransport] = tr
	}
	return out, nil
}

func (r *VMessRenderer) RenderOutbound(_ *RenderContext, cfg model.OutboundConfig) (map[string]any, error) {
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
	out["security"] = "auto"
	if alterID := settings["alter_id"]; alterID != nil {
		out["alter_id"] = alterID
	} else {
		out["alter_id"] = 0
	}
	if tr, err := renderTransport(cfg.Transport); err != nil {
		return nil, err
	} else if tr != nil {
		out[fTransport] = tr
	}
	return out, nil
}

func (r *VMessRenderer) ValidateInbound(cfg model.InboundConfig) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("vmess inbound: listen_port 必须在 1-65535")
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
		return fmt.Errorf("vmess inbound: settings.users 不能为空（或标记 is_user_inbound）")
	}
	for i, u := range us {
		if _, err := uuid.Parse(str(u, fUUID)); err != nil {
			return fmt.Errorf("vmess inbound: users[%d].uuid 非法: %w", i, err)
		}
	}
	return nil
}

func (r *VMessRenderer) ValidateOutbound(cfg model.OutboundConfig) error {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	if str(settings, fServer) == "" || intVal(settings, fServerPort) <= 0 {
		return fmt.Errorf("vmess outbound: server/server_port 必填")
	}
	if _, err := uuid.Parse(str(settings, fUUID)); err != nil {
		return fmt.Errorf("vmess outbound: uuid 非法: %w", err)
	}
	return nil
}

func (r *VMessRenderer) SubscriptionURI(cfg model.OutboundConfig, user ProxyUser) (string, error) {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"v":    "2",
		"ps":   cfg.Name,
		"add":  str(settings, fServer),
		"port": portStr(settings, fServerPort),
		"id":   user.UUID,
		"aid":  "0",
		"net":  "tcp",
		"type": "none",
		"tls":  "",
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(b), nil
}

func (r *VMessRenderer) Meta() ProtocolMeta {
	return ProtocolMeta{
		Protocol:    "vmess",
		Directions:  []Direction{DirectionInbound, DirectionOutbound},
		TLSAllowed:  []TLSMode{TLSModeNone},
		TLSRequired: false,
		MultiUser:   true,
		Inbound: []FieldSchema{
			{Key: "alter_id", Label: "Alter ID", Type: "number", Required: false, Default: defaultJSON(0)},
		},
		Outbound: []FieldSchema{
			{Key: "server", Label: "服务器地址", Type: "text", Required: true},
			{Key: "server_port", Label: "端口", Type: "number", Required: true},
			{Key: "uuid", Label: "UUID", Type: "text", Required: true, Encrypt: true},
			{Key: "alter_id", Label: "Alter ID", Type: "number", Required: false, Default: defaultJSON(0)},
		},
	}
}

func init() { Register(&VMessRenderer{}) }

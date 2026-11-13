package renderer

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"time"

	"github.com/singbox-panel/control-plane/internal/model"
)

// AnyTLSRenderer 实现 anytls 协议渲染。
//
// 关键语义（docs/protocol-matrix-renderer.md §3.1）：
//   - password 必须 base64 编码（非明文）；
//   - padding_scheme 仅 inbound；
//   - 会话池三件套 + client_metadata 仅 outbound；
//   - inbound+outbound 必选普通 TLS，禁 Reality/ECH。
type AnyTLSRenderer struct{}

func (r *AnyTLSRenderer) Protocol() string { return "anytls" }

func (r *AnyTLSRenderer) RenderInbound(_ *RenderContext, cfg model.InboundConfig, users []ProxyUser) (map[string]any, error) {
	out := map[string]any{}
	renderInboundCommon(out, cfg)

	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return nil, err
	}

	// users：密码必须 base64（渲染器负责编码，不依赖前端存 base64）。
	if cfg.IsUserInbound && len(users) > 0 {
		us := make([]map[string]any, 0, len(users))
		for _, u := range users {
			us = append(us, map[string]any{
				fName:     u.Name,
				fPassword: base64.StdEncoding.EncodeToString([]byte(u.Password)),
			})
		}
		out[fUsers] = us
	} else if static := parseUsers(settings); len(static) > 0 {
		us := make([]map[string]any, 0, len(static))
		for _, u := range static {
			m := map[string]any{}
			if name := str(u, fName); name != "" {
				m[fName] = name
			}
			m[fPassword] = base64.StdEncoding.EncodeToString([]byte(str(u, fPassword)))
			us = append(us, m)
		}
		out[fUsers] = us
	}

	// tls：inbound 必选普通 TLS。
	t, err := renderTLS(cfg.TLS)
	if err != nil {
		return nil, err
	}
	out[fTLS] = t

	// padding_scheme：仅 inbound，来自 settings JSONB。
	if ps, ok := settings["padding_scheme"]; ok {
		out["padding_scheme"] = ps
	}
	return out, nil
}

func (r *AnyTLSRenderer) RenderOutbound(_ *RenderContext, cfg model.OutboundConfig) (map[string]any, error) {
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
	// ★ 关键：outbound 密码也是 base64。
	out[fPassword] = base64.StdEncoding.EncodeToString([]byte(str(settings, fPassword)))

	// 会话池三件套 + client_metadata（仅 outbound，存在才渲染）。
	for _, k := range []string{
		"idle_session_check_interval",
		"idle_session_timeout",
		"min_idle_session",
		"client_metadata",
	} {
		if v, ok := settings[k]; ok {
			out[k] = v
		}
	}

	t, err := renderTLS(cfg.TLS)
	if err != nil {
		return nil, err
	}
	out[fTLS] = t
	return out, nil
}

func (r *AnyTLSRenderer) ValidateInbound(cfg model.InboundConfig) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("anytls inbound: listen_port 必须在 1-65535")
	}
	m, err := tlsMap(cfg.TLS)
	if err != nil {
		return err
	}
	if !tlsEnabled(m) {
		return fmt.Errorf("anytls inbound: tls.enabled 必须为 true")
	}
	if realityEnabled(m) || echEnabled(m) {
		return fmt.Errorf("anytls inbound: 不支持 Reality/ECH")
	}
	if str(m, "certificate_path") == "" || str(m, "key_path") == "" {
		return fmt.Errorf("anytls inbound: 缺少 certificate_path/key_path")
	}

	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	if ps, ok := settings["padding_scheme"]; ok {
		if _, ok := ps.([]any); !ok {
			return fmt.Errorf("anytls inbound: padding_scheme 必须为数组")
		}
	}
	if cfg.IsUserInbound {
		return nil
	}
	us := parseUsers(settings)
	if len(us) == 0 {
		return fmt.Errorf("anytls inbound: settings.users 不能为空（或标记 is_user_inbound）")
	}
	for i, u := range us {
		if str(u, fPassword) == "" {
			return fmt.Errorf("anytls inbound: users[%d].password 不能为空", i)
		}
	}
	return nil
}

func (r *AnyTLSRenderer) ValidateOutbound(cfg model.OutboundConfig) error {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return err
	}
	if str(settings, fServer) == "" || intVal(settings, fServerPort) <= 0 {
		return fmt.Errorf("anytls outbound: server/server_port 必填")
	}
	if str(settings, fPassword) == "" {
		return fmt.Errorf("anytls outbound: password 必填")
	}

	m, err := tlsMap(cfg.TLS)
	if err != nil {
		return err
	}
	if !tlsEnabled(m) {
		return fmt.Errorf("anytls outbound: tls.enabled 必须为 true")
	}
	if realityEnabled(m) || echEnabled(m) {
		return fmt.Errorf("anytls outbound: 不支持 Reality/ECH（mihomo 亦不支持 AnyTLS+Reality）")
	}
	if str(m, fServerName) == "" {
		return fmt.Errorf("anytls outbound: 缺少 tls.server_name")
	}
	return validateAnyTLSSessionPool(settings)
}

func (r *AnyTLSRenderer) SubscriptionURI(cfg model.OutboundConfig, user ProxyUser) (string, error) {
	settings, err := parseMap(cfg.Settings)
	if err != nil {
		return "", err
	}
	server, port := str(settings, fServer), portStr(settings, fServerPort)
	pwd := user.Password
	if pwd == "" {
		pwd = str(settings, fPassword)
	}
	if server == "" || port == "" || pwd == "" {
		return "", fmt.Errorf("anytls: 订阅 URI 缺少 server/port/password")
	}
	// anytls://<base64-password>@<server>:<port>?sni=<server_name>#<tag>
	encoded := base64.StdEncoding.EncodeToString([]byte(pwd))
	q := url.Values{}
	if sni := sniFromTLS(cfg); sni != "" {
		q.Set("sni", sni)
	}
	raw := fmt.Sprintf("anytls://%s@%s:%s", encoded, server, port)
	if e := q.Encode(); e != "" {
		raw += "?" + e
	}
	return raw + "#" + cfg.Name, nil
}

func (r *AnyTLSRenderer) Meta() ProtocolMeta {
	return ProtocolMeta{
		Protocol:    "anytls",
		Directions:  []Direction{DirectionInbound, DirectionOutbound},
		TLSAllowed:  []TLSMode{TLSModePlain}, // 仅普通 TLS
		TLSRequired: true,
		MultiUser:   true,
		Inbound: []FieldSchema{
			{Key: "padding_scheme", Label: "Padding 方案", Type: "array", Required: false},
		},
		Outbound: []FieldSchema{
			{Key: "server", Label: "服务器地址", Type: "text", Required: true},
			{Key: "server_port", Label: "端口", Type: "number", Required: true},
			{Key: "password", Label: "密码", Type: "text", Required: true, Encrypt: true}, // 明文入、渲染时 base64
			{Key: "idle_session_check_interval", Label: "会话检查间隔", Type: "duration", Required: false},
			{Key: "idle_session_timeout", Label: "会话空闲超时", Type: "duration", Required: false},
			{Key: "min_idle_session", Label: "最小空闲会话数", Type: "number", Required: false},
			{Key: "client_metadata", Label: "Client Metadata", Type: "text", Required: false},
		},
	}
}

// validateAnyTLSSessionPool 校验会话池字段：duration 合法、min_idle_session ≥ 0。
func validateAnyTLSSessionPool(settings map[string]any) error {
	for _, k := range []string{"idle_session_check_interval", "idle_session_timeout"} {
		if _, ok := settings[k]; ok {
			d, err := time.ParseDuration(str(settings, k))
			if err != nil || d <= 0 {
				return fmt.Errorf("anytls outbound: %s 必须为正 duration", k)
			}
		}
	}
	if _, ok := settings["min_idle_session"]; ok {
		if intVal(settings, "min_idle_session") < 0 {
			return fmt.Errorf("anytls outbound: min_idle_session 必须 ≥ 0")
		}
	}
	return nil
}

func init() { Register(&AnyTLSRenderer{}) }

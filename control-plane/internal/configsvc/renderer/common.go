package renderer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/singbox-panel/control-plane/internal/model"
)

// sing-box 通用字段名常量。协议无关的「中立模型 ↔ sing-box 字段」映射集中于此，
// 各渲染器复用，避免散落硬编码（docs/protocol-matrix-renderer.md §5.2）。
const (
	fType       = "type"
	fTag        = "tag"
	fListen     = "listen"
	fListenPort = "listen_port"
	fServer     = "server"
	fServerPort = "server_port"
	fUsers      = "users"
	fTLS        = "tls"
	fTransport  = "transport"
	fUUID       = "uuid"
	fName       = "name"
	fPassword   = "password"
	fFlow       = "flow"
	fMethod     = "method"
	fNetwork    = "network"
)

// base64Encode 返回 base64 标准编码（anytls 密码等）。
func base64Encode(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// parseMap 解析 JSONB 为 map；nil / 空 / "null" 一律返回空 map。
func parseMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("renderer: parse jsonb: %w", err)
	}
	return m, nil
}

// str 取 map 中的字符串值，缺失返回空串。
func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// intVal 取 map 中的数值（兼容 float64 / json.Number / int / int64）。
func intVal(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

// portStr 返回 map 中数值字段的字符串形式（订阅 URI 拼接用）。
func portStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case string:
		return v
	default:
		return ""
	}
}

// renderInboundCommon 填充 type/tag/listen/listen_port。
func renderInboundCommon(out map[string]any, cfg model.InboundConfig) {
	out[fType] = cfg.Type
	out[fTag] = cfg.Name
	out[fListen] = cfg.Listen
	out[fListenPort] = cfg.Port
}

// renderOutboundCommon 填充 type/tag。
func renderOutboundCommon(out map[string]any, cfg model.OutboundConfig) {
	out[fType] = cfg.Type
	out[fTag] = cfg.Name
}

// parseUsers 解析 settings["users"] 数组（静态入站用户）。
func parseUsers(m map[string]any) []map[string]any {
	raw, _ := m[fUsers].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		if um, ok := v.(map[string]any); ok {
			out = append(out, um)
		}
	}
	return out
}

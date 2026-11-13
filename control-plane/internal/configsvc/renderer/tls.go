package renderer

import (
	"encoding/json"
	"fmt"

	"github.com/singbox-panel/control-plane/internal/model"
)

// TLS 相关字段名（共享，供 inbound/outbound 两方向复用）。
const (
	fEnabled    = "enabled"
	fServerName = "server_name"
	fReality    = "reality"
	fECH        = "ech"
)

// tlsMap 解析 tls JSONB 为 map；无则返回 nil。
func tlsMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return parseMap(raw)
}

// tlsEnabled 判断 tls.enabled 是否为 true。
func tlsEnabled(m map[string]any) bool {
	if m == nil {
		return false
	}
	if b, ok := m[fEnabled].(bool); ok {
		return b
	}
	return false
}

// realityEnabled 判断是否启用了 Reality（存在 reality 对象即视为启用）。
func realityEnabled(m map[string]any) bool {
	if m == nil {
		return false
	}
	if _, ok := m[fReality].(map[string]any); ok {
		return true
	}
	return false
}

// echEnabled 判断是否启用了 ECH（存在 ech 对象即视为启用）。
func echEnabled(m map[string]any) bool {
	if m == nil {
		return false
	}
	if _, ok := m[fECH].(map[string]any); ok {
		return true
	}
	return false
}

// hasMode 判断允许清单中是否包含指定 TLSMode。
func hasMode(allowed []TLSMode, mode TLSMode) bool {
	for _, a := range allowed {
		if a == mode {
			return true
		}
	}
	return false
}

// validateTLS 校验 tls JSONB 是否符合协议允许的 TLS 组合。
// required=true 时强制 tls.enabled == true（trojan / anytls / hysteria2 / tuic）。
func validateTLS(raw json.RawMessage, allowed []TLSMode, required bool) error {
	m, err := tlsMap(raw)
	if err != nil {
		return err
	}
	if required && !tlsEnabled(m) {
		return fmt.Errorf("tls: enabled 必须为 true")
	}
	if m == nil {
		return nil
	}
	if realityEnabled(m) && !hasMode(allowed, TLSModeReality) {
		return fmt.Errorf("tls: 该协议不支持 Reality")
	}
	if echEnabled(m) && !hasMode(allowed, TLSModeECH) {
		return fmt.Errorf("tls: 该协议不支持 ECH")
	}
	return nil
}

// renderTLS 渲染 tls（透传解析后的 map；nil 表示不输出 tls 字段）。
func renderTLS(raw json.RawMessage) (any, error) {
	m, err := tlsMap(raw)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	return m, nil
}

// sniFromTLS 提取出站 tls.server_name（订阅 URI 的 sni 参数）。
func sniFromTLS(cfg model.OutboundConfig) string {
	m, err := tlsMap(cfg.TLS)
	if err != nil {
		return ""
	}
	return str(m, fServerName)
}

// validateOutboundTLSCommon 是出站 TLS 的通用组合校验（协议 renderer 复用）。
func validateOutboundTLSCommon(cfg model.OutboundConfig, allowed []TLSMode, required bool) error {
	return validateTLS(cfg.TLS, allowed, required)
}

// validateInboundTLSCommon 是入站 TLS 的通用组合校验（协议 renderer 复用）。
func validateInboundTLSCommon(cfg model.InboundConfig, allowed []TLSMode, required bool) error {
	return validateTLS(cfg.TLS, allowed, required)
}

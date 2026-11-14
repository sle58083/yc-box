package subscribe

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/singbox-panel/control-plane/internal/model"
	"gopkg.in/yaml.v3"
)

// Clash 分组名（固定 4 组，见 sui-gap-design.md §1.4.2）。
const (
	clashGroupSelect  = "🚀 节点选择"
	clashGroupAuto    = "♻️ 自动选择"
	clashGroupDirect  = "🎯 全局直连"
	clashGroupAdblock = "🛑 广告拦截"
)

// buildClashYAML 生成 Clash 完整 YAML 订阅（proxies + proxy-groups + rules）。
//
// 复用 resolveSubscription 拉取全部客户端出站，逐条映射为 Clash proxy dict，
// 再组装固定 4 组与默认 rules 后由 yaml.v3 序列化。
func (s *Service) buildClashYAML(ctx context.Context, subToken string) ([]byte, error) {
	_, outbounds, err := s.resolveSubscription(ctx, subToken)
	if err != nil {
		return nil, err
	}

	proxies := make([]map[string]any, 0, len(outbounds))
	names := make([]string, 0, len(outbounds))
	for _, co := range outbounds {
		name := co.inbound.Name + " - " + co.node.Name
		proxy := proxyFromOutbound(co.outbound, name)
		if proxy == nil {
			continue
		}
		proxies = append(proxies, proxy)
		names = append(names, name)
	}
	if len(proxies) == 0 {
		return nil, fmt.Errorf("subscribe: 无可用订阅节点/入站")
	}

	// select 全量节点 + DIRECT；url-test 仅节点。
	selectProxies := append(append([]string{}, names...), "DIRECT")
	groups := []map[string]any{
		{
			"name":    clashGroupSelect,
			"type":    "select",
			"proxies": selectProxies,
		},
		{
			"name":     clashGroupAuto,
			"type":     "url-test",
			"url":      "http://www.gstatic.com/generate_204",
			"interval": 300,
			"proxies":  append([]string{}, names...),
		},
		{
			"name":    clashGroupDirect,
			"type":    "select",
			"proxies": []string{"DIRECT"},
		},
		{
			"name":    clashGroupAdblock,
			"type":    "select",
			"proxies": []string{"REJECT"},
		},
	}

	rules := []string{
		"GEOIP,CN,DIRECT",
		"MATCH," + clashGroupSelect,
	}

	return yaml.Marshal(map[string]any{
		"proxies":      proxies,
		"proxy-groups": groups,
		"rules":        rules,
	})
}

// proxyFromOutbound 将中立出站映射为 Clash proxy dict；不支持的协议返回 nil。
//
// 字段名以 Mihomo 为准（docs/sui-gap-design.md §1.4/§8.5）：vless/vmess/trojan
// 映射 type/server/port/uuid/password/network/ws-opts/tls/reality/udp/flow；
// shadowsocks → type ss + cipher/password/udp/plugin；hysteria2/tuic/anytls 按各自
// 内置 QUIC/TLS 语义映射。
func proxyFromOutbound(cfg model.OutboundConfig, name string) map[string]any {
	settings, err := parseJSONObject(cfg.Settings)
	if err != nil {
		return nil
	}
	// TLS/Transport 解析失败仅视为缺省，不阻断订阅生成。
	tls, _ := parseJSONObject(cfg.TLS)
	transport, _ := parseJSONObject(cfg.Transport)

	base := map[string]any{
		"name":   name,
		"server": mapString(settings, "server"),
		"port":   mapInt(settings, "server_port"),
	}

	switch cfg.Type {
	case "vless":
		return proxyVLESS(base, settings, tls, transport)
	case "vmess":
		return proxyVMess(base, settings, tls, transport)
	case "trojan":
		return proxyTrojan(base, settings, tls, transport)
	case "shadowsocks":
		return proxyShadowsocks(base, settings, transport)
	case "hysteria2":
		return proxyHysteria2(base, settings, tls)
	case "tuic":
		return proxyTUIC(base, settings, tls)
	case "anytls":
		return proxyAnyTLS(base, settings, tls)
	default:
		return nil
	}
}

// proxyVLESS 映射 vless：uuid/udp/flow + 普通 TLS/Reality + 传输层。
func proxyVLESS(p, s, tls, transport map[string]any) map[string]any {
	p["type"] = "vless"
	p["uuid"] = mapString(s, "uuid")
	p["udp"] = true
	if flow := mapString(s, "flow"); flow != "" {
		p["flow"] = flow
	}
	applyClashTLS(p, tls, "servername")
	applyClashNetwork(p, transport)
	return p
}

// proxyVMess 映射 vmess：uuid/alterId/cipher/udp + 传输层（vmess 无原生 TLS）。
func proxyVMess(p, s, _ map[string]any, transport map[string]any) map[string]any {
	p["type"] = "vmess"
	p["uuid"] = mapString(s, "uuid")
	p["alterId"] = mapInt(s, "alter_id")
	p["cipher"] = "auto"
	p["udp"] = true
	applyClashNetwork(p, transport)
	return p
}

// proxyTrojan 映射 trojan：password/udp + SNI（trojan 始终 TLS）+ 传输层。
func proxyTrojan(p, s, tls, transport map[string]any) map[string]any {
	p["type"] = "trojan"
	p["password"] = mapString(s, "password")
	p["udp"] = true
	applyClashSNI(p, tls)
	applyClashNetwork(p, transport)
	return p
}

// proxyShadowsocks 映射 shadowsocks：cipher/password/udp + 可选 SIP003 plugin。
func proxyShadowsocks(p, s, transport map[string]any) map[string]any {
	p["type"] = "ss"
	p["cipher"] = mapString(s, "method")
	p["password"] = mapString(s, "password")
	p["udp"] = true
	applyClashSSPlugin(p, transport)
	return p
}

// proxyHysteria2 映射 hysteria2：password + SNI/skip-cert-verify（内置 QUIC TLS）。
func proxyHysteria2(p, s, tls map[string]any) map[string]any {
	p["type"] = "hysteria2"
	p["password"] = mapString(s, "password")
	// up/down/obfs 属客户端调优项，服务端 outbound 通常不携带；存在则透传。
	for _, k := range []string{"up", "down", "obfs"} {
		if v, ok := s[k]; ok {
			p[k] = v
		}
	}
	if obfsPwd := mapString(s, "obfs_password"); obfsPwd != "" {
		p["obfs-password"] = obfsPwd
	}
	applyClashSNI(p, tls)
	return p
}

// proxyTUIC 映射 tuic：uuid/password/congestion-controller/udp-relay-mode + SNI。
func proxyTUIC(p, s, tls map[string]any) map[string]any {
	p["type"] = "tuic"
	p["uuid"] = mapString(s, "uuid")
	p["password"] = mapString(s, "password")
	p["udp-relay-mode"] = "native"
	if cc := mapString(s, "congestion_control"); cc != "" {
		p["congestion-controller"] = cc
	} else {
		p["congestion-controller"] = "cubic"
	}
	applyClashSNI(p, tls)
	return p
}

// proxyAnyTLS 映射 anytls：password/udp + SNI/skip-cert-verify（始终 TLS）。
// Mihomo 侧 password 为明文（与服务端用户配置一致），故直接透传。
func proxyAnyTLS(p, s, tls map[string]any) map[string]any {
	p["type"] = "anytls"
	p["password"] = mapString(s, "password")
	p["udp"] = true
	applyClashSNI(p, tls)
	if fp := fingerprintOf(tls); fp != "" {
		p["client-fingerprint"] = fp
	}
	return p
}

// applyClashTLS 处理 vless 的普通 TLS/Reality：tls/servername/skip-cert-verify/
// client-fingerprint/reality-opts。sniField 指定 SNI 字段名（vless/vmess 用 servername）。
func applyClashTLS(p, tls map[string]any, sniField string) {
	if len(tls) == 0 || !mapBool(tls, "enabled") {
		return
	}
	p["tls"] = true
	if sni := mapString(tls, "server_name"); sni != "" {
		p[sniField] = sni
	}
	if mapBool(tls, "insecure") {
		p["skip-cert-verify"] = true
	}
	if fp := fingerprintOf(tls); fp != "" {
		p["client-fingerprint"] = fp
	}
	if r, ok := tls["reality"].(map[string]any); ok {
		ro := map[string]any{}
		if pk := mapString(r, "public_key"); pk != "" {
			ro["public-key"] = pk
		}
		if sid, ok := r["short_id"]; ok {
			ro["short-id"] = sid
		}
		if len(ro) > 0 {
			p["reality-opts"] = ro
		}
	}
}

// applyClashSNI 处理始终启用 TLS 协议（trojan/hysteria2/tuic/anytls）的 SNI 与
// 证书校验开关（不输出 tls: true，因为这些协议在 Mihomo 中隐式启用 TLS）。
func applyClashSNI(p, tls map[string]any) {
	if sni := mapString(tls, "server_name"); sni != "" {
		p["sni"] = sni
	}
	if mapBool(tls, "insecure") {
		p["skip-cert-verify"] = true
	}
}

// applyClashNetwork 映射 sing-box transport → Clash network/ws-opts/grpc-opts。
func applyClashNetwork(p, t map[string]any) {
	if len(t) == 0 {
		return
	}
	switch mapString(t, "type") {
	case "", "tcp":
		return
	case "ws":
		p["network"] = "ws"
		ws := map[string]any{}
		if path := mapString(t, "path"); path != "" {
			ws["path"] = path
		}
		if headers, ok := t["headers"].(map[string]any); ok && len(headers) > 0 {
			ws["headers"] = headers
		}
		if v, ok := t["max_early_data"]; ok {
			ws["max-early-data"] = v
		}
		if v := mapString(t, "early_data_header_name"); v != "" {
			ws["early-data-header-name"] = v
		}
		if len(ws) > 0 {
			p["ws-opts"] = ws
		}
	case "grpc":
		p["network"] = "grpc"
		grpc := map[string]any{}
		if sn := mapString(t, "service_name"); sn != "" {
			grpc["grpc-service-name"] = sn
		}
		if len(grpc) > 0 {
			p["grpc-opts"] = grpc
		}
	}
}

// applyClashSSPlugin 映射 shadowsocks 的 SIP003 plugin（obfs / v2ray-plugin）。
func applyClashSSPlugin(p, t map[string]any) {
	if len(t) == 0 {
		return
	}
	switch mapString(t, "type") {
	case "obfs", "obfs-local":
		p["plugin"] = "obfs"
		opts := map[string]any{}
		if mode := mapString(t, "mode"); mode != "" {
			opts["mode"] = mode
		}
		if host := mapString(t, "host"); host != "" {
			opts["host"] = host
		}
		if len(opts) > 0 {
			p["plugin-opts"] = opts
		}
	case "v2ray-plugin":
		p["plugin"] = "v2ray-plugin"
		opts := map[string]any{}
		if mode := mapString(t, "mode"); mode != "" {
			opts["mode"] = mode
		}
		if host := mapString(t, "host"); host != "" {
			opts["host"] = host
		}
		if path := mapString(t, "path"); path != "" {
			opts["path"] = path
		}
		if mapBool(t, "tls") {
			opts["tls"] = true
		}
		if len(opts) > 0 {
			p["plugin-opts"] = opts
		}
	}
}

// fingerprintOf 提取 TLS uTLS 指纹（sing-box: tls.utls.fingerprint）。
func fingerprintOf(tls map[string]any) string {
	if u, ok := tls["utls"].(map[string]any); ok {
		return mapString(u, "fingerprint")
	}
	return mapString(tls, "fingerprint")
}

// parseJSONObject 解析 JSONB 为 map；nil/空/"null" 一律返回空 map。
func parseJSONObject(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// mapString 取 map 中的字符串值，缺失返回空串。
func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// mapInt 取 map 中的数值（兼容 float64 / json.Number / int / int64）。
func mapInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
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

// mapBool 取 map 中的布尔值，缺失返回 false。
func mapBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	b, _ := m[key].(bool)
	return b
}

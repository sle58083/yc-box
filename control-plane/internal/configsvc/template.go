package configsvc

import (
	"encoding/json"

	"github.com/singbox-panel/control-plane/internal/model"
)

// 全局结构字段名常量。中立模型 → sing-box 全局结构（log/dns/route）的
// 映射集中于此（§9.6），协议级字段映射在 renderer 包。
const (
	kLog       = "log"
	kDNS       = "dns"
	kRoute     = "route"
	kInbounds  = "inbounds"
	kOutbounds = "outbounds"

	kLevel     = "level"
	kTimestamp = "timestamp"
	kServers   = "servers"
	kTag       = "tag"
	kType      = "type"
	kAddress   = "address"
	kServer    = "server"
	kDetour    = "detour"
	kRules     = "rules"
	kFinal     = "final"

	kInbound  = "inbound"
	kOutbound = "outbound"

	// 路由规则类型 → sing-box route.rules[].{...} 字段映射。
	ruleFieldGeosite  = "geosite"
	ruleFieldGeoip    = "geoip"
	ruleFieldDomain   = "domain"
	ruleFieldIPCIDR   = "ip_cidr"
	ruleFieldProtocol = "protocol"
)

// GlobalSettings 全局配置（log / dns / route.final）。MVP 无全局配置表，
// 使用默认值；后续可扩展为 DB 存储。
type GlobalSettings struct {
	LogLevel   string
	DNS        DNSSettings
	RouteFinal string // 空=自动推导（优先 selector，其次第一个出站，兜底 direct）
}

// DNSSettings 全局 DNS 配置。
type DNSSettings struct {
	Servers []DNSServer
	Final   string
}

// DNSServer 单个 DNS 服务器。
type DNSServer struct {
	Type    string // udp / tcp / tls / https / h3 / quic（sing-box 1.12+ 新格式必需）
	Tag     string
	Server  string // 服务器地址（新格式字段，取代旧 address）
	Port    int    // 服务器端口（0=用类型默认值）
	Path    string // DoH 路径（https 类型默认 /dns-query）
	Detour  string // 旧格式出站；新格式保留为空（由 dialer 控制）
}

// DefaultGlobalSettings 返回默认全局配置（使用 sing-box 1.12+ 新 DNS 格式）。
func DefaultGlobalSettings() GlobalSettings {
	return GlobalSettings{
		LogLevel: "info",
		DNS: DNSSettings{
			Servers: []DNSServer{
				{Type: "https", Tag: "dns-remote", Server: "1.1.1.1", Port: 443, Path: "/dns-query"},
				{Type: "udp", Tag: "dns-local", Server: "223.5.5.5", Port: 53, Detour: "direct"},
			},
			Final: "dns-remote",
		},
		RouteFinal: "",
	}
}

// applyGlobalTemplate 拼装全局结构 log / dns / route。
func applyGlobalTemplate(doc map[string]any, g GlobalSettings, rules []model.RoutingRule) {
	doc[kLog] = map[string]any{kLevel: g.LogLevel, kTimestamp: true}

	if len(g.DNS.Servers) > 0 {
		servers := make([]map[string]any, 0, len(g.DNS.Servers))
		for _, s := range g.DNS.Servers {
			servers = append(servers, renderDNSServer(s))
		}
		doc[kDNS] = map[string]any{kServers: servers, kFinal: g.DNS.Final}
	}

	doc[kRoute] = buildRoute(g, rules)
}

// renderDNSServer 渲染单个 DNS server（sing-box 1.12+ 新格式）。
// 新格式必需 type 字段；server 取代旧 address 字段。
func renderDNSServer(s DNSServer) map[string]any {
	m := map[string]any{
		kType: s.Type,
		kTag:  s.Tag,
	}
	// server 字段：新格式用 server（而非旧 address）
	if s.Server != "" {
		m["server"] = s.Server
	}
	if s.Port > 0 {
		m["server_port"] = s.Port
	}
	if s.Path != "" {
		m["path"] = s.Path
	}
	return m
}

// buildRoute 渲染 route.rules[]（按 priority 升序）。
func buildRoute(g GlobalSettings, rules []model.RoutingRule) map[string]any {
	rendered := make([]map[string]any, 0, len(rules))
	for _, r := range rules {
		m := map[string]any{}

		if len(r.InboundTags) > 0 {
			var tags []string
			if err := json.Unmarshal(r.InboundTags, &tags); err == nil && len(tags) > 0 {
				m[kInbound] = tags
			}
		}

		switch r.RuleType {
		case ruleFieldGeosite:
			m[ruleFieldGeosite] = []string{r.Value}
		case ruleFieldGeoip:
			m[ruleFieldGeoip] = []string{r.Value}
		case ruleFieldDomain:
			m[ruleFieldDomain] = []string{r.Value}
		case "ip":
			m[ruleFieldIPCIDR] = []string{r.Value}
		case ruleFieldProtocol:
			m[ruleFieldProtocol] = r.Value
		}

		if r.OutboundTag != "" {
			m[kOutbound] = r.OutboundTag
		}
		rendered = append(rendered, m)
	}

	final := g.RouteFinal
	if final == "" {
		final = "direct"
	}

	route := map[string]any{kRules: rendered, kFinal: final}

	// sing-box 1.14 要求 route.default_domain_resolver 必须指定（否则报
	// "missing route.default_domain_resolver ... deprecated"）。默认指向
	// 第一个 DNS server 的 tag（若无 DNS 则回退 dns-local）。
	resolver := ""
	if len(g.DNS.Servers) > 0 {
		resolver = g.DNS.Servers[0].Tag
	}
	if resolver == "" {
		resolver = "dns-local"
	}
	route["default_domain_resolver"] = resolver

	return route
}

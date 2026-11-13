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
	kAddress   = "address"
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
	Tag     string
	Address string
	Detour  string
}

// DefaultGlobalSettings 返回默认全局配置（与 §5.3 示例对齐）。
func DefaultGlobalSettings() GlobalSettings {
	return GlobalSettings{
		LogLevel: "info",
		DNS: DNSSettings{
			Servers: []DNSServer{
				{Tag: "dns-remote", Address: "https://1.1.1.1/dns-query"},
				{Tag: "dns-local", Address: "223.5.5.5", Detour: "direct"},
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
			m := map[string]any{kTag: s.Tag, kAddress: s.Address}
			if s.Detour != "" {
				m[kDetour] = s.Detour
			}
			servers = append(servers, m)
		}
		doc[kDNS] = map[string]any{kServers: servers, kFinal: g.DNS.Final}
	}

	doc[kRoute] = buildRoute(g, rules)
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
	return map[string]any{kRules: rendered, kFinal: final}
}

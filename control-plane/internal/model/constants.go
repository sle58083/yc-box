package model

// 统一响应业务码（system_design.md §9.1）。
// code=0 表示成功；code≠0 为错误，按段划分：
//
//	1xxx 通用  2xxx 认证  3xxx 权限  4xxx 配置  5xxx 节点/Agent
const (
	CodeOK = 0

	// 1xxx 通用错误
	CodeBadRequest       = 1000
	CodeInternalError    = 1001
	CodeNotFound         = 1002
	CodeConflict         = 1003
	CodeValidationFailed = 1004
	CodeTooManyRequests  = 1005

	// 2xxx 认证
	CodeUnauthorized       = 2000
	CodeInvalidCredentials = 2001
	CodeTokenExpired       = 2002
	CodeTokenInvalid       = 2003
	CodeTokenRevoked       = 2004

	// 3xxx 权限
	CodeForbidden              = 3000
	CodeInsufficientPermission = 3001

	// 4xxx 配置
	CodeConfigInvalid      = 4000
	CodeConfigGenerateFail = 4001
	CodeConfigSchemaFail   = 4002
	CodeConfigDeployFail   = 4003
	CodeTagConflict        = 4004

	// 5xxx 节点 / Agent
	CodeNodeOffline       = 5000
	CodeNodeError         = 5001
	CodeAgentUnregistered = 5002
	CodeAgentTimeout      = 5003
)

// 节点状态（system_design.md §4.2「node.status」）。
const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
	NodeStatusPending = "pending"
	NodeStatusError   = "error"
)

// 通用启停状态（inbound/outbound/proxy_user/panel_user 共用语义）。
const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	StatusExpired  = "expired"
)

// 配置版本状态机（system_design.md §9.10）：
// draft → deploying → success|failed → rolled_back
const (
	ConfigVersionDraft      = "draft"
	ConfigVersionDeploying  = "deploying"
	ConfigVersionSuccess    = "success"
	ConfigVersionFailed     = "failed"
	ConfigVersionRolledBack = "rolled_back"
)

// 入站协议白名单（system_design.md §4.2 + §5.7 anytls）。
// type 字段在 DB 层无约束，靠代码白名单校验。
var InboundProtocolTypes = []string{
	"vmess", "vless", "trojan", "shadowsocks", "hysteria2", "tuic", "anytls",
}

// 出站协议白名单（system_design.md §4.2「outbound_config.type」）。
var OutboundProtocolTypes = []string{
	"direct", "block", "selector", "urltest",
	"vmess", "vless", "trojan", "shadowsocks", "hysteria2", "tuic", "anytls",
}

// 路由规则类型（system_design.md §4.2「routing_rule.rule_type」）。
var RoutingRuleTypes = []string{
	"geosite", "geoip", "domain", "ip", "protocol",
}

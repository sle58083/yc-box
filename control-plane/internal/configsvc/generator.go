package configsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/singbox-panel/control-plane/internal/configsvc/renderer"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ErrUnknownProtocol 表示未注册的协议类型。
var ErrUnknownProtocol = errors.New("configsvc: unknown protocol")

// NodeConfigContext 是 Generator 的输入（单节点聚合后的配置上下文）。
type NodeConfigContext struct {
	Global     GlobalSettings
	Inbounds   []model.InboundConfig
	Outbounds  []model.OutboundConfig
	Rules      []model.RoutingRule
	ProxyUsers []renderer.ProxyUser // 该分组 active 用户（UUID 已解密）
}

// Generator 是中立模型 → sing-box JSON 的纯函数生成器。
// 只做「查表分发 + 全局结构拼装」，不感知协议字段。
type Generator struct{}

// NewGenerator 构造 Generator。
func NewGenerator() *Generator { return &Generator{} }

// Generate 输入 NodeConfigContext，输出 sing-box 原生 JSON（map 形式）。
func (g *Generator) Generate(ctx *NodeConfigContext) (map[string]any, error) {
	global := ctx.Global
	if global.LogLevel == "" {
		global = DefaultGlobalSettings()
	}

	// 启停过滤（§5.2）：status=disabled 不写入 inbounds[]/outbounds[]。
	// aggregate 已过滤一次，此处再防御一次，保证 Generator 纯函数自洽。
	enabledInbounds := filterInbounds(ctx.Inbounds)
	enabledOutbounds := filterOutbounds(ctx.Outbounds)

	// 1. 渲染 inbounds（查表分发）。
	inbounds := make([]map[string]any, 0, len(enabledInbounds))
	for _, cfg := range enabledInbounds {
		r, ok := renderer.Get(cfg.Type)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownProtocol, cfg.Type)
		}
		var users []renderer.ProxyUser
		if cfg.IsUserInbound {
			users = ctx.ProxyUsers
		}
		el, err := r.RenderInbound(&renderer.RenderContext{Ctx: context.Background(), Users: users}, cfg, users)
		if err != nil {
			return nil, fmt.Errorf("render inbound %q: %w", cfg.Name, err)
		}
		inbounds = append(inbounds, el)
	}

	// 2. 渲染 outbounds（查表分发）。
	outbounds := make([]map[string]any, 0, len(enabledOutbounds))
	for _, cfg := range enabledOutbounds {
		r, ok := renderer.Get(cfg.Type)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownProtocol, cfg.Type)
		}
		el, err := r.RenderOutbound(&renderer.RenderContext{Ctx: context.Background()}, cfg)
		if err != nil {
			return nil, fmt.Errorf("render outbound %q: %w", cfg.Name, err)
		}
		outbounds = append(outbounds, el)
	}

	// 兜底直连：无任何出站时注入默认 direct，避免 route.final="direct" 引用不存在的
	// tag 导致 sing-box 启动失败（"default outbound not found: direct"）。
	if len(outbounds) == 0 {
		outbounds = append(outbounds, map[string]any{kType: "direct", kTag: "direct"})
	}

	// 3. tag 全局唯一断言（§9.8，跨 inbound/outbound 全量）。
	if err := assertUniqueTags(inbounds, outbounds); err != nil {
		return nil, err
	}

	// 4. 全局结构拼装。
	doc := map[string]any{
		kInbounds:     inbounds,
		kOutbounds:    outbounds,
		kExperimental: buildV2RayAPI(enabledInbounds, enabledOutbounds, ctx.ProxyUsers),
	}
	global.RouteFinal = resolveRouteFinal(outbounds)
	applyGlobalTemplate(doc, global, ctx.Rules)
	return doc, nil
}

// buildV2RayAPI 渲染 experimental.v2ray_api 节点（§3.2）：启用入站/出站 tag
// 与活跃用户 name（= proxy_user DB UUID），供 edge-agent gRPC 采集流量。
func buildV2RayAPI(inbounds []model.InboundConfig, outbounds []model.OutboundConfig, users []renderer.ProxyUser) map[string]any {
	inboundTags := make([]string, 0, len(inbounds))
	for _, c := range inbounds {
		if c.Name != "" {
			inboundTags = append(inboundTags, c.Name)
		}
	}
	outboundTags := make([]string, 0, len(outbounds))
	for _, c := range outbounds {
		if c.Name != "" {
			outboundTags = append(outboundTags, c.Name)
		}
	}
	userNames := make([]string, 0, len(users))
	for _, u := range users {
		if u.Name != "" {
			userNames = append(userNames, u.Name)
		}
	}
	return map[string]any{
		"v2ray_api": map[string]any{
			"listen": "127.0.0.1:9001",
			"stats": map[string]any{
				"enabled":   true,
				"inbounds":  inboundTags,
				"outbounds": outboundTags,
				"users":     userNames,
			},
		},
	}
}

// assertUniqueTags 断言 inbounds[] 与 outbounds[] 的 tag 全局唯一。
func assertUniqueTags(inbounds, outbounds []map[string]any) error {
	seen := map[string]string{}
	for _, el := range inbounds {
		tag, _ := el[kTag].(string)
		if tag == "" {
			return errors.New("configsvc: inbound tag 不能为空")
		}
		if prev, dup := seen[tag]; dup {
			return fmt.Errorf("configsvc: tag %q 重复（inbound 与 %s）", tag, prev)
		}
		seen[tag] = "inbound"
	}
	for _, el := range outbounds {
		tag, _ := el[kTag].(string)
		if tag == "" {
			return errors.New("configsvc: outbound tag 不能为空")
		}
		if prev, dup := seen[tag]; dup {
			return fmt.Errorf("configsvc: tag %q 重复（outbound 与 %s）", tag, prev)
		}
		seen[tag] = "outbound"
	}
	return nil
}

// resolveRouteFinal 推导 route.final：优先 selector，其次第一个出站，兜底 direct。
func resolveRouteFinal(outbounds []map[string]any) string {
	fallback := "direct"
	for _, el := range outbounds {
		tag, _ := el[kTag].(string)
		if tag != "" && fallback == "direct" {
			fallback = tag
		}
		typ, _ := el["type"].(string)
		if typ == "selector" {
			return tag
		}
	}
	return fallback
}

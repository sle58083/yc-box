// Package renderer 是「中立配置模型 → sing-box 原生 JSON」的协议渲染器插件包。
//
// 一个协议 = 一个 ProtocolRenderer 实现（渲染 + 校验 + 订阅 URI + 元数据），
// 通过 init() + registry 自注册；Generator 只做 renderer.Get(type) 查表分发，
// 绝不硬编码协议名（docs/protocol-matrix-renderer.md）。
package renderer

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/singbox-panel/control-plane/internal/model"
)

// ErrUnsupported 表示该协议不支持当前操作（如无标准订阅格式、方向不支持）。
var ErrUnsupported = errors.New("renderer: operation not supported for this protocol")

// ProxyUser 是渲染时注入的代理用户视图（敏感值已由 configsvc 解密）。
// 渲染器负责「编码规则」（如 anytls 要 base64），但不得负责「解密」。
type ProxyUser struct {
	ID       string
	Name     string
	UUID     string // vless/vmess/tuic 用，已解密
	Password string // 协议密码明文（已解密；anytls 渲染器会再 base64）
}

// RenderContext 提供渲染期间共享数据与已解密的敏感值。
type RenderContext struct {
	Ctx   context.Context
	Users []ProxyUser
}

// ProtocolRenderer 协议渲染器接口：一个协议 = 一个实现。
type ProtocolRenderer interface {
	// Protocol 返回协议标识，必须与 inbound_config.type / outbound_config.type 一致。
	Protocol() string

	// RenderInbound 渲染 sing-box inbounds[] 的一个元素。
	// users 仅在 cfg.IsUserInbound 为真时非空。
	RenderInbound(ctx *RenderContext, cfg model.InboundConfig, users []ProxyUser) (map[string]any, error)

	// RenderOutbound 渲染 sing-box outbounds[] 的一个元素。
	RenderOutbound(ctx *RenderContext, cfg model.OutboundConfig) (map[string]any, error)

	// ValidateInbound / ValidateOutbound：保存前业务校验（含协议×TLS 组合约束，唯一权威）。
	ValidateInbound(cfg model.InboundConfig) error
	ValidateOutbound(cfg model.OutboundConfig) error

	// SubscriptionURI 生成订阅 URI；协议无标准订阅格式时返回 ErrUnsupported。
	SubscriptionURI(cfg model.OutboundConfig, user ProxyUser) (string, error)

	// Meta 返回前端动态表单所需的协议元数据（方向、字段 schema、允许的 TLS 组合）。
	Meta() ProtocolMeta
}

// Direction 协议支持方向。
type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

// TLSMode 协议允许的 TLS 组合。
type TLSMode string

const (
	TLSModeNone    TLSMode = "none"
	TLSModePlain   TLSMode = "plain"
	TLSModeReality TLSMode = "reality"
	TLSModeECH     TLSMode = "ech"
)

// ProtocolMeta 协议元数据，前端据此渲染动态表单与约束。
type ProtocolMeta struct {
	Protocol    string
	Directions  []Direction
	Inbound     []FieldSchema
	Outbound    []FieldSchema
	TLSAllowed  []TLSMode
	TLSRequired bool
	MultiUser   bool
}

// FieldSchema 是前端动态表单的字段描述（JSON-schema 风格最小子集）。
type FieldSchema struct {
	Key      string          `json:"key"`
	Label    string          `json:"label"`
	Type     string          `json:"type"` // text/number/bool/select/array/duration
	Required bool            `json:"required"`
	Encrypt  bool            `json:"encrypt"` // 是否 🔐 加密落库（如 anytls password）
	Options  []FieldOption   `json:"options,omitempty"`
	Default  json.RawMessage `json:"default,omitempty"`
}

// FieldOption 是 select 字段的可选项。
type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// hasDirection 判断协议是否支持指定方向。
func hasDirection(meta ProtocolMeta, d Direction) bool {
	for _, v := range meta.Directions {
		if v == d {
			return true
		}
	}
	return false
}

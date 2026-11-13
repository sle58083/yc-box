package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// RoutingRule 路由规则（中立模型）。
//
// 表结构见 system_design.md §4.2「routing_rule」。
type RoutingRule struct {
	Base
	GroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"group_id"`
	Group   *Group    `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`

	NodeID *uuid.UUID `gorm:"type:uuid;index" json:"node_id"`
	Node   *Node      `gorm:"foreignKey:NodeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`

	Name        string          `gorm:"type:varchar(64);not null" json:"name"`
	InboundTags json.RawMessage `gorm:"type:jsonb" json:"inbound_tags"` // 入站 tag 数组
	OutboundTag string          `gorm:"type:varchar(64)" json:"outbound_tag"`
	RuleType    string          `gorm:"type:varchar(16)" json:"rule_type"` // geosite/geoip/domain/ip/protocol
	Value       string          `gorm:"type:text" json:"value"`
	Priority    int             `gorm:"not null;default:0" json:"priority"`
}

// TableName 返回设计约定的表名。
func (RoutingRule) TableName() string { return "routing_rule" }

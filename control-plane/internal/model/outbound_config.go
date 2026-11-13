package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// OutboundConfig 出站配置（中立模型）。
//
// 表结构见 system_design.md §4.2「outbound_config」。
// 注意：除 settings 外，必须包含 tls / transport / status 三列（与 inbound 对称）。
type OutboundConfig struct {
	Base
	GroupID uuid.UUID `gorm:"type:uuid;not null;index:idx_outbound_group_id;uniqueIndex:uniq_outbound_scope_name" json:"group_id"`
	Group   *Group    `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`

	NodeID *uuid.UUID `gorm:"type:uuid;index:idx_outbound_node_id;uniqueIndex:uniq_outbound_scope_name" json:"node_id"` // 空=分组模板
	Node   *Node      `gorm:"foreignKey:NodeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`

	Name      string          `gorm:"type:varchar(64);not null;uniqueIndex:uniq_outbound_scope_name" json:"name"` // 即 sing-box tag
	Type      string          `gorm:"type:varchar(32);not null" json:"type"`
	Settings  json.RawMessage `gorm:"type:jsonb" json:"settings"`  // 🔐 出站参数，服务器密码等敏感值加密
	TLS       json.RawMessage `gorm:"type:jsonb" json:"tls"`       // 🔐 出站 TLS/Reality，private_key 等私钥加密（nullable）
	Transport json.RawMessage `gorm:"type:jsonb" json:"transport"` // 出站传输层配置 ws/grpc/httpupgrade 等（nullable）
	Status    string          `gorm:"type:varchar(16);not null;default:'enabled'" json:"status"`
}

// TableName 返回设计约定的表名。
func (OutboundConfig) TableName() string { return "outbound_config" }

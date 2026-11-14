package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// InboundConfig 入站配置（中立模型），与 sing-box 原生 JSON Schema 解耦。
//
// 表结构见 system_design.md §4.2「inbound_config」。
type InboundConfig struct {
	Base
	GroupID uuid.UUID `gorm:"type:uuid;not null;index:idx_inbound_group_id;uniqueIndex:uniq_inbound_scope_name" json:"group_id"`
	Group   *Group    `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`

	// NodeID 为空表示分组级模板；非空表示节点级覆盖。
	NodeID *uuid.UUID `gorm:"type:uuid;index:idx_inbound_node_id;uniqueIndex:uniq_inbound_scope_name" json:"node_id"`
	Node   *Node      `gorm:"foreignKey:NodeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`

	Name          string          `gorm:"type:varchar(64);not null;uniqueIndex:uniq_inbound_scope_name" json:"name"` // 即 sing-box tag
	Type          string          `gorm:"type:varchar(32);not null" json:"type"`
	Listen        string          `gorm:"type:varchar(64)" json:"listen"`
	Port          int             `gorm:"not null;default:0" json:"port"`
	Settings      json.RawMessage `gorm:"type:jsonb" json:"settings"`     // 协议参数（clients/users 等）
	TLS           json.RawMessage `gorm:"type:jsonb" json:"tls"`          // 🔐 TLS/Reality 配置，private_key 等私钥加密
	CertID        *uuid.UUID      `gorm:"type:uuid;index" json:"cert_id"` // 绑定的证书（certificate.id），可空
	Transport     json.RawMessage `gorm:"type:jsonb" json:"transport"`
	IsUserInbound bool            `gorm:"not null;default:false" json:"is_user_inbound"` // 是否代理用户入口，生成时注入用户 UUID
	Status        string          `gorm:"type:varchar(16);not null;default:'enabled'" json:"status"`
}

// TableName 返回设计约定的表名。
func (InboundConfig) TableName() string { return "inbound_config" }

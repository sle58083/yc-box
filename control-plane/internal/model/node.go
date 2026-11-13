package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Node 边缘节点。
//
// 表结构见 system_design.md §4.2「node」。
type Node struct {
	Base
	GroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"group_id"`
	Group   *Group    `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`

	Name           string          `gorm:"type:varchar(128);not null" json:"name"`
	Address        string          `gorm:"type:varchar(255)" json:"address"`
	AgentTokenHash string          `gorm:"type:varchar(64);not null" json:"-"` // # Agent 令牌 SHA-256，仅创建时明文返回一次
	AgentVersion   string          `gorm:"type:varchar(32)" json:"agent_version"`
	SingboxVersion string          `gorm:"type:varchar(32)" json:"singbox_version"`
	Status         string          `gorm:"type:varchar(16);index;not null;default:'pending'" json:"status"`
	LastHeartbeat  *time.Time      `gorm:"type:timestamptz;index" json:"last_heartbeat"`
	Tags           json.RawMessage `gorm:"type:jsonb" json:"tags"` // 标签数组
}

// TableName 返回设计约定的表名。
func (Node) TableName() string { return "node" }

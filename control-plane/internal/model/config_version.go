package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConfigVersion 配置版本（只追加，不修改）。
//
// 表结构见 system_design.md §4.2「config_version」。
type ConfigVersion struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	NodeID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uniq_config_version_node_version" json:"node_id"`
	Node   *Node     `gorm:"foreignKey:NodeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`

	Version    int       `gorm:"not null;uniqueIndex:uniq_config_version_node_version" json:"version"` // 每节点单调递增
	ContentEnc string    `gorm:"type:text" json:"-"`                                                   // 🔐 完整 sing-box JSON 快照，AES-256-GCM 加密
	Hash       string    `gorm:"type:varchar(64);index" json:"hash"`                                   // 明文 SHA-256
	Status     string    `gorm:"type:varchar(16);not null;default:'draft'" json:"status"`
	CreatedBy  uuid.UUID `gorm:"type:uuid" json:"created_by"` // 操作面板用户
	CreatedAt  time.Time `gorm:"type:datetime;not null" json:"created_at"`
}

// BeforeCreate 在插入前生成 UUID v4 主键。
func (c *ConfigVersion) BeforeCreate(_ *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// TableName 返回设计约定的表名。
func (ConfigVersion) TableName() string { return "config_version" }

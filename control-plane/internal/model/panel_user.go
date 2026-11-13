package model

import "github.com/google/uuid"

// PanelUser 面板用户。
//
// 表结构见 system_design.md §4.2「panel_user」。
type PanelUser struct {
	Base
	Username     string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"type:varchar(128);not null" json:"-"` // # argon2id
	RoleID       uuid.UUID `gorm:"type:uuid;not null;index" json:"role_id"`
	Role         *Role     `gorm:"foreignKey:RoleID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
	Status       string    `gorm:"type:varchar(16);not null;default:'enabled'" json:"status"`
}

// TableName 返回设计约定的表名。
func (PanelUser) TableName() string { return "panel_user" }

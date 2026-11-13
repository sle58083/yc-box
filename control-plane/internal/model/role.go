package model

import "encoding/json"

// Role 面板角色。
//
// 表结构见 system_design.md §4.2「role」。
type Role struct {
	Base
	Name        string          `gorm:"type:varchar(64);uniqueIndex;not null" json:"name"` // super_admin/admin/viewer
	Permissions json.RawMessage `gorm:"type:jsonb" json:"permissions"`                     // 权限码数组
}

// TableName 返回设计约定的表名。
func (Role) TableName() string { return "role" }

package model

import "github.com/google/uuid"

// Group 分组 / 集群 / 区域，支持层级隔离（MVP 单层，parent_id 可空）。
//
// 表结构见 system_design.md §4.2「group」。
type Group struct {
	Base
	Name     string     `gorm:"type:varchar(128);uniqueIndex;not null" json:"name"`
	Desc     string     `gorm:"type:text" json:"desc"`
	ParentID *uuid.UUID `gorm:"type:uuid;index" json:"parent_id"` // 父分组，层级隔离
}

// TableName 返回设计约定的表名。
func (Group) TableName() string { return "group" }

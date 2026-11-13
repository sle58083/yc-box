package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// AuditLog 审计日志。
//
// 表结构见 system_design.md §4.2「audit_log」。
type AuditLog struct {
	Base
	OperatorID uuid.UUID       `gorm:"type:uuid;index" json:"operator_id"` // 操作者（面板用户）
	Action     string          `gorm:"type:varchar(64);index;not null" json:"action"`
	Resource   string          `gorm:"type:varchar(255)" json:"resource"`
	Detail     json.RawMessage `gorm:"type:jsonb" json:"detail"` // 变更前后快照
	IP         string          `gorm:"type:varchar(64)" json:"ip"`
}

// TableName 返回设计约定的表名。
func (AuditLog) TableName() string { return "audit_log" }

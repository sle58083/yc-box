package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Base 是所有实体的公共字段：UUID v4 主键 + GORM 自动维护的创建/更新时间。
//
// 约定（system_design.md §9.7）：
//   - 主键为 UUID v4，由应用层（google/uuid）生成；
//   - 时间字段一律存 ISO8601 UTC（timestamptz）；
//   - created_at / updated_at 由 GORM 自动填充。
type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"type:datetime;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:datetime;not null" json:"updated_at"`
}

// BeforeCreate 在插入前生成 UUID v4 主键，保证与 migrations/*.sql 的 uuid 列一致。
func (b *Base) BeforeCreate(_ *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

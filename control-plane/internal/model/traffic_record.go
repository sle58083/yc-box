package model

import (
	"time"

	"github.com/google/uuid"
)

// TrafficRecord 流量时序记录（TimescaleDB hypertable，P1）。
//
// 表结构见 system_design.md §4.2「traffic_record」。使用组合主键而非 UUID。
// proxy_user_id 可为空（节点级聚合），故不参与主键。
type TrafficRecord struct {
	Ts          time.Time `gorm:"type:datetime;not null;primaryKey" json:"ts"` // 时间桶
	NodeID      uuid.UUID `gorm:"type:uuid;not null;primaryKey" json:"node_id"`
	InboundTag  string    `gorm:"type:varchar(64);not null;primaryKey" json:"inbound_tag"`
	OutboundTag string    `gorm:"type:varchar(64);not null;primaryKey" json:"outbound_tag"`

	ProxyUserID *uuid.UUID `gorm:"type:uuid;index" json:"proxy_user_id"` // 用户级聚合用 UUID
	UpBytes     int64      `gorm:"not null;default:0" json:"up_bytes"`
	DownBytes   int64      `gorm:"not null;default:0" json:"down_bytes"`
}

// TableName 返回设计约定的表名。
func (TrafficRecord) TableName() string { return "traffic_record" }

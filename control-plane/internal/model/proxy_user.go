package model

import (
	"time"

	"github.com/google/uuid"
)

// ProxyUser 代理用户（终端用户）。
//
// 表结构见 system_design.md §4.2「proxy_user」。
type ProxyUser struct {
	Base
	GroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"group_id"`
	Group   *Group    `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`

	UUID         string     `gorm:"type:text" json:"-"`         // 🔐 vless/vmess 用户 UUID，AES-256-GCM 加密
	PasswordHash string     `gorm:"type:varchar(128)" json:"-"` // # 协议密码的不可逆哈希（argon2id），审计/校验用
	PasswordEnc  string     `gorm:"type:text" json:"-"`         // 🔐 协议明文密码的可逆加密（AES-256-GCM），订阅/下发渲染时解密注入
	Remark       string     `gorm:"type:varchar(255)" json:"remark"`
	QuotaBytes   int64      `gorm:"not null;default:-1" json:"quota_bytes"` // -1=无限
	UsedBytes    int64      `gorm:"not null;default:0" json:"used_bytes"`   // 已用流量（聚合缓存）
	ExpireAt     *time.Time `gorm:"type:datetime" json:"expire_at"`
	Status       string     `gorm:"type:varchar(16);index;not null;default:'enabled'" json:"status"`
	SubTokenHash string     `gorm:"type:varchar(64);index" json:"-"` // # 订阅令牌 SHA-256，明文仅创建返回
}

// TableName 返回设计约定的表名。
func (ProxyUser) TableName() string { return "proxy_user" }

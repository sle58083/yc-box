package model

import "time"

// Certificate 证书实体，供 TLS / Reality / Hysteria2 / TUIC 等需证书的协议复用。
//
// 表结构见 sui-gap-design.md §3.1「certificate」。签发来源：self_signed（一键自签）
// 或 manual（手动导入 PEM）；不做 ACME。
type Certificate struct {
	Base
	Name      string     `gorm:"type:varchar(128);not null" json:"name"`
	Type      string     `gorm:"type:varchar(16);not null" json:"type"` // self_signed | manual
	Domains   string     `gorm:"type:text" json:"domains"`              // JSON []string（SAN）
	CertPEM   string     `gorm:"type:text" json:"-"`                    // 🔐 证书 PEM（service 层 EncryptField 加密落库）
	KeyPEM    string     `gorm:"type:text" json:"-"`                    // 🔐 私钥 PEM（service 层 EncryptField 加密落库）
	Issuer    string     `gorm:"type:varchar(255)" json:"issuer"`
	NotBefore *time.Time `gorm:"type:datetime" json:"not_before"`
	NotAfter  *time.Time `gorm:"type:datetime" json:"not_after"`
	Status    string     `gorm:"type:varchar(16);not null;default:'valid'" json:"status"` // valid | invalid | expired
	LastError string     `gorm:"type:text" json:"last_error"`
}

// TableName 返回设计约定的表名。
func (Certificate) TableName() string { return "certificate" }

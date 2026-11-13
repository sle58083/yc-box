// Package traffic 负责流量上报摄入与节点级/用户级聚合查询。
package traffic

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
	"gorm.io/gorm"
)

// Record 是 Agent 上报的单条流量记录。
type Record struct {
	TS          int64
	ProxyUserID *string // 代理用户 UUID 明文（空=节点级）
	InboundTag  string
	OutboundTag string
	UpBytes     int64
	DownBytes   int64
}

// NodeStat 节点级聚合结果。
type NodeStat struct {
	NodeID    uuid.UUID `json:"node_id"`
	UpBytes   int64     `json:"up_bytes"`
	DownBytes int64     `json:"down_bytes"`
}

// UserStat 用户级聚合结果。
type UserStat struct {
	ProxyUserID *uuid.UUID `json:"proxy_user_id"`
	UpBytes     int64      `json:"up_bytes"`
	DownBytes   int64      `json:"down_bytes"`
}

// Service 流量服务。
type Service struct {
	db    *store.DB
	codec *crypto.Codec
}

// NewService 构造流量服务。
func NewService(db *store.DB, codec *crypto.Codec) *Service {
	return &Service{db: db, codec: codec}
}

// Ingest 摄入一批流量记录（写入 traffic_record，时间桶取秒级）。
func (s *Service) Ingest(ctx context.Context, nodeID uuid.UUID, records []Record) error {
	for _, r := range records {
		var proxyUserID *uuid.UUID
		if r.ProxyUserID != nil && *r.ProxyUserID != "" {
			if id, err := s.resolveByUUID(ctx, *r.ProxyUserID); err == nil {
				proxyUserID = &id
			}
		}
		rec := toTrafficRecord(r, nodeID, proxyUserID)
		if err := s.db.WithContext(ctx).Create(&rec).Error; err != nil {
			return err
		}
	}
	return nil
}

// toTrafficRecord 将上报记录转换为落库模型（纯函数，便于单测）。
func toTrafficRecord(r Record, nodeID uuid.UUID, proxyUserID *uuid.UUID) model.TrafficRecord {
	return model.TrafficRecord{
		Ts:          time.Unix(r.TS, 0).UTC(),
		NodeID:      nodeID,
		ProxyUserID: proxyUserID,
		InboundTag:  r.InboundTag,
		OutboundTag: r.OutboundTag,
		UpBytes:     r.UpBytes,
		DownBytes:   r.DownBytes,
	}
}

// NodeTraffic 节点级聚合（按节点分组求和，可选时间窗）。
func (s *Service) NodeTraffic(ctx context.Context, nodeID *uuid.UUID, from, to *time.Time) ([]NodeStat, error) {
	q := s.db.WithContext(ctx).Model(&model.TrafficRecord{}).
		Select("node_id, COALESCE(SUM(up_bytes),0) AS up_bytes, COALESCE(SUM(down_bytes),0) AS down_bytes").
		Group("node_id")
	if nodeID != nil {
		q = q.Where("node_id = ?", *nodeID)
	}
	q = window(q, from, to)
	var out []NodeStat
	if err := q.Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// UserTraffic 用户级聚合（按代理用户分组求和，可选时间窗）。
func (s *Service) UserTraffic(ctx context.Context, proxyUserID *uuid.UUID, from, to *time.Time) ([]UserStat, error) {
	q := s.db.WithContext(ctx).Model(&model.TrafficRecord{}).
		Select("proxy_user_id, COALESCE(SUM(up_bytes),0) AS up_bytes, COALESCE(SUM(down_bytes),0) AS down_bytes").
		Group("proxy_user_id")
	if proxyUserID != nil {
		q = q.Where("proxy_user_id = ?", *proxyUserID)
	}
	q = window(q, from, to)
	var out []UserStat
	if err := q.Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func window(q *gorm.DB, from, to *time.Time) *gorm.DB {
	if from != nil {
		q = q.Where("ts >= ?", *from)
	}
	if to != nil {
		q = q.Where("ts <= ?", *to)
	}
	return q
}

// resolveByUUID 按明文 UUID 解析代理用户 ID（MVP：遍历解密匹配，用户量级小）。
func (s *Service) resolveByUUID(ctx context.Context, plainUUID string) (uuid.UUID, error) {
	var users []model.ProxyUser
	if err := s.db.WithContext(ctx).Find(&users).Error; err != nil {
		return uuid.Nil, err
	}
	for _, u := range users {
		if u.UUID == "" {
			continue
		}
		if dec, err := s.codec.DecryptString(u.UUID); err == nil && dec == plainUUID {
			return u.ID, nil
		}
	}
	return uuid.Nil, fmt.Errorf("traffic: 未找到 UUID 对应的代理用户")
}

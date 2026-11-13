// Package audit 负责审计日志的 append-only 记录与查询。
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
)

// Service 审计服务。
type Service struct {
	db *store.DB
}

// NewService 构造审计服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// Record 写入一条审计日志（append-only，普通管理员不可删除）。
func (s *Service) Record(ctx context.Context, operatorID uuid.UUID, action, resource string, detail any, ip string) error {
	var detailRaw json.RawMessage
	if detail != nil {
		b, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		detailRaw = b
	}
	entry := &model.AuditLog{
		OperatorID: operatorID,
		Action:     action,
		Resource:   resource,
		Detail:     detailRaw,
		IP:         ip,
	}
	return s.db.WithContext(ctx).Create(entry).Error
}

// List 分页查询审计日志（按时间倒序）。
func (s *Service) List(ctx context.Context, action string, operatorID *uuid.UUID, from, to *time.Time, limit, offset int) ([]model.AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := s.db.WithContext(ctx).Model(&model.AuditLog{}).Order("created_at DESC")
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if operatorID != nil {
		q = q.Where("operator_id = ?", *operatorID)
	}
	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("created_at <= ?", *to)
	}
	var out []model.AuditLog
	if err := q.Limit(limit).Offset(offset).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

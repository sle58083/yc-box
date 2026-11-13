// Package group 负责分组 CRUD 与默认分组初始化（供前端端到端流程使用）。
package group

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
)

var errNameRequired = errors.New("group: name 必填")

// Service 分组服务。
type Service struct {
	db *store.DB
}

// NewService 构造分组服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// List 列出分组。
func (s *Service) List(ctx context.Context) ([]model.Group, error) {
	var out []model.Group
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Create 创建分组。
func (s *Service) Create(ctx context.Context, g *model.Group) error {
	if g.Name == "" {
		return errNameRequired
	}
	g.ID = uuid.Nil
	return s.db.WithContext(ctx).Create(g).Error
}

// Get 获取分组。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Group, error) {
	var g model.Group
	if err := s.db.WithContext(ctx).First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// EnsureDefaultGroup 幂等初始化默认分组，返回其 ID。
func (s *Service) EnsureDefaultGroup(ctx context.Context) (uuid.UUID, error) {
	var g model.Group
	if err := s.db.WithContext(ctx).First(&g, "name = ?", "default").Error; err == nil {
		return g.ID, nil
	}
	g = model.Group{Name: "default", Desc: "默认分组（自动创建）"}
	if err := s.db.WithContext(ctx).Create(&g).Error; err != nil {
		return uuid.Nil, err
	}
	return g.ID, nil
}

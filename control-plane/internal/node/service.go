// Package node 负责节点纳管与 Agent 令牌。
package node

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
)

// Service 节点管理服务。
type Service struct {
	db    *store.DB
	codec *crypto.Codec
}

// NewService 构造节点服务。
func NewService(db *store.DB, codec *crypto.Codec) *Service {
	return &Service{db: db, codec: codec}
}

// Create 创建节点并生成 Agent 令牌；明文令牌仅本次返回。
func (s *Service) Create(ctx context.Context, n *model.Node) (string, error) {
	if n.Name == "" {
		return "", fmt.Errorf("node: name 必填")
	}
	if n.GroupID == uuid.Nil {
		return "", fmt.Errorf("node: group_id 必填")
	}
	token, err := crypto.RandomToken(32)
	if err != nil {
		return "", err
	}
	n.ID = uuid.Nil
	n.AgentTokenHash = crypto.HashToken(token)
	if n.Status == "" {
		n.Status = model.NodeStatusPending
	}
	if err := s.db.WithContext(ctx).Create(n).Error; err != nil {
		return "", err
	}
	return token, nil
}

// Get 获取节点。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Node, error) {
	var n model.Node
	if err := s.db.WithContext(ctx).First(&n, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// List 列出节点。
func (s *Service) List(ctx context.Context, groupID *uuid.UUID) ([]model.Node, error) {
	var out []model.Node
	q := s.db.WithContext(ctx).Model(&model.Node{}).Order("created_at ASC")
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Update 更新节点（name/address/tags 等；token 不可经此修改）。
func (s *Service) Update(ctx context.Context, n *model.Node) error {
	existing, err := s.Get(ctx, n.ID)
	if err != nil {
		return err
	}
	// 不可变字段：group_id、agent_token_hash。
	n.GroupID = existing.GroupID
	n.AgentTokenHash = existing.AgentTokenHash
	return s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", n.ID).Save(n).Error
}

// Delete 删除节点。
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&model.Node{}, "id = ?", id).Error
}

// RegenerateToken 重新生成 Agent 令牌；明文仅本次返回。
func (s *Service) RegenerateToken(ctx context.Context, id uuid.UUID) (string, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return "", err
	}
	token, err := crypto.RandomToken(32)
	if err != nil {
		return "", err
	}
	updates := map[string]any{
		"agent_token_hash": crypto.HashToken(token),
		"updated_at":       time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return "", err
	}
	return token, nil
}

// Authenticate 校验 Agent 令牌，返回匹配的节点。
func (s *Service) Authenticate(ctx context.Context, token string) (*model.Node, error) {
	if token == "" {
		return nil, fmt.Errorf("node: 空令牌")
	}
	hash := crypto.HashToken(token)
	var n model.Node
	if err := s.db.WithContext(ctx).First(&n, "agent_token_hash = ?", hash).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// MarkHeartbeat 更新心跳与运行信息。
func (s *Service) MarkHeartbeat(ctx context.Context, id uuid.UUID, agentVersion, singboxVersion string) error {
	updates := map[string]any{
		"status":         model.NodeStatusOnline,
		"last_heartbeat": time.Now().UTC(),
		"updated_at":     time.Now().UTC(),
	}
	if agentVersion != "" {
		updates["agent_version"] = agentVersion
	}
	if singboxVersion != "" {
		updates["singbox_version"] = singboxVersion
	}
	return s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", id).Updates(updates).Error
}

// SetStatus 更新节点状态。
func (s *Service) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	return s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", id).
		Update("status", status).Error
}

// MarkOffline 将超时节点标记离线（由心跳巡检调用，T04 落地）。
func (s *Service) MarkOffline(ctx context.Context, before time.Time) error {
	return s.db.WithContext(ctx).Model(&model.Node{}).
		Where("status = ? AND (last_heartbeat IS NULL OR last_heartbeat < ?)", model.NodeStatusOnline, before).
		Update("status", model.NodeStatusOffline).Error
}

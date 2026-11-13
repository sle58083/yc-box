package configsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
)

// Preview 生成节点配置预览（不落版本）。
func (s *Service) Preview(ctx context.Context, nodeID uuid.UUID) (map[string]any, error) {
	agg, err := s.aggregate(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	doc, err := s.generator.Generate(agg)
	if err != nil {
		return nil, err
	}
	if s.schema != nil {
		if err := s.schema.ValidateDocument(doc); err != nil {
			return nil, err
		}
	}
	return doc, nil
}

// CreateVersion 生成配置并创建版本（draft，全文加密快照 + SHA-256，version 单调递增）。
// 实际下发（config_push）在 T03 由 deploy 编排执行。
func (s *Service) CreateVersion(ctx context.Context, nodeID uuid.UUID, operatorID uuid.UUID) (*model.ConfigVersion, error) {
	doc, err := s.Preview(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("configsvc: marshal config: %w", err)
	}
	enc, err := s.codec.EncryptString(string(raw))
	if err != nil {
		return nil, fmt.Errorf("configsvc: encrypt snapshot: %w", err)
	}

	next, err := s.nextVersion(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	v := &model.ConfigVersion{
		NodeID:     nodeID,
		Version:    next,
		ContentEnc: enc,
		Hash:       crypto.SHA256Hex(string(raw)),
		Status:     model.ConfigVersionDraft,
		CreatedBy:  operatorID,
	}
	if err := s.db.WithContext(ctx).Create(v).Error; err != nil {
		return nil, err
	}
	return v, nil
}

// ListVersions 列出节点历史版本（不含加密快照）。
func (s *Service) ListVersions(ctx context.Context, nodeID uuid.UUID) ([]model.ConfigVersion, error) {
	var out []model.ConfigVersion
	if err := s.db.WithContext(ctx).
		Model(&model.ConfigVersion{}).
		Select("id, node_id, version, hash, status, created_by, created_at").
		Where("node_id = ?", nodeID).
		Order("version DESC").
		Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// GetVersion 获取单个版本。
func (s *Service) GetVersion(ctx context.Context, id uuid.UUID) (*model.ConfigVersion, error) {
	var v model.ConfigVersion
	if err := s.db.WithContext(ctx).First(&v, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// DecryptVersionContent 解密版本快照，返回明文 sing-box JSON。
func (s *Service) DecryptVersionContent(v *model.ConfigVersion) ([]byte, error) {
	plain, err := s.codec.Decrypt(v.ContentEnc)
	if err != nil {
		return nil, fmt.Errorf("configsvc: decrypt snapshot: %w", err)
	}
	return plain, nil
}

// Rollback 回滚到指定历史版本：复制其加密快照为新的 draft 版本（version+1）。
// 重新走 config_push 由 T03 编排。
func (s *Service) Rollback(ctx context.Context, nodeID uuid.UUID, targetVersionID uuid.UUID, operatorID uuid.UUID) (*model.ConfigVersion, error) {
	target, err := s.GetVersion(ctx, targetVersionID)
	if err != nil {
		return nil, err
	}
	if target.NodeID != nodeID {
		return nil, fmt.Errorf("configsvc: 回滚目标版本不属于该节点")
	}
	next, err := s.nextVersion(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	v := &model.ConfigVersion{
		NodeID:     nodeID,
		Version:    next,
		ContentEnc: target.ContentEnc,
		Hash:       target.Hash,
		Status:     model.ConfigVersionDraft,
		CreatedBy:  operatorID,
	}
	if err := s.db.WithContext(ctx).Create(v).Error; err != nil {
		return nil, err
	}
	return v, nil
}

// UpdateVersionStatus 状态机流转（draft→deploying→success|failed→rolled_back）。
func (s *Service) UpdateVersionStatus(ctx context.Context, id uuid.UUID, status string) error {
	if !validVersionStatus(status) {
		return fmt.Errorf("configsvc: 非法版本状态 %q", status)
	}
	return s.db.WithContext(ctx).Model(&model.ConfigVersion{}).Where("id = ?", id).
		Update("status", status).Error
}

func (s *Service) nextVersion(ctx context.Context, nodeID uuid.UUID) (int, error) {
	var max int
	if err := s.db.WithContext(ctx).Model(&model.ConfigVersion{}).
		Where("node_id = ?", nodeID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&max).Error; err != nil {
		return 0, err
	}
	return max + 1, nil
}

func validVersionStatus(status string) bool {
	switch status {
	case model.ConfigVersionDraft, model.ConfigVersionDeploying,
		model.ConfigVersionSuccess, model.ConfigVersionFailed, model.ConfigVersionRolledBack:
		return true
	}
	return false
}

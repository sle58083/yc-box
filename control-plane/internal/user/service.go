// Package user 负责代理用户、面板用户与角色的 CRUD。
package user

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/rbac"
	"github.com/singbox-panel/control-plane/internal/store"
)

// Service 用户域服务。
type Service struct {
	db    *store.DB
	codec *crypto.Codec
}

// NewService 构造用户域服务。
func NewService(db *store.DB, codec *crypto.Codec) *Service {
	return &Service{db: db, codec: codec}
}

// ============================= 代理用户 =============================

// CreateProxyUser 创建代理用户：自动生成 UUID（🔐）、协议密码（🔐+ #）、订阅令牌（#）。
// 返回订阅令牌明文（仅一次）。
func (s *Service) CreateProxyUser(ctx context.Context, u *model.ProxyUser, plainPassword string) (string, error) {
	if u.GroupID == uuid.Nil {
		return "", fmt.Errorf("user: group_id 必填")
	}
	userUUID := uuid.NewString()
	encUUID, err := s.codec.EncryptString(userUUID)
	if err != nil {
		return "", err
	}
	u.UUID = encUUID

	// 协议密码：用户未提供时自动生成随机密码，保证 is_user_inbound 入站
	// 始终有可用密码（否则 sing-box 校验报 "missing password"）。
	if plainPassword == "" {
		plainPassword, err = crypto.RandomToken(16)
		if err != nil {
			return "", err
		}
	}
	if err := s.setPassword(u, plainPassword); err != nil {
		return "", err
	}

	subToken, err := crypto.RandomToken(32)
	if err != nil {
		return "", err
	}
	u.SubTokenHash = crypto.HashToken(subToken)

	if u.Status == "" {
		u.Status = model.StatusEnabled
	}
	if u.QuotaBytes == 0 {
		u.QuotaBytes = -1
	}
	u.ID = uuid.Nil
	if err := s.db.WithContext(ctx).Create(u).Error; err != nil {
		return "", err
	}
	return subToken, nil
}

// UpdateProxyUser 更新备注/配额/过期/状态；plainPassword 非空则重置协议密码。
func (s *Service) UpdateProxyUser(ctx context.Context, u *model.ProxyUser, plainPassword *string) error {
	existing, err := s.GetProxyUser(ctx, u.ID)
	if err != nil {
		return err
	}
	u.GroupID = existing.GroupID
	u.UUID = existing.UUID
	u.SubTokenHash = existing.SubTokenHash
	u.PasswordHash = existing.PasswordHash
	u.PasswordEnc = existing.PasswordEnc

	if plainPassword != nil && *plainPassword != "" {
		if err := s.setPassword(u, *plainPassword); err != nil {
			return err
		}
	}
	return s.db.WithContext(ctx).Model(&model.ProxyUser{}).Where("id = ?", u.ID).Save(u).Error
}

// DeleteProxyUser 删除代理用户。
func (s *Service) DeleteProxyUser(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&model.ProxyUser{}, "id = ?", id).Error
}

// GetProxyUser 获取代理用户（敏感字段不序列化）。
func (s *Service) GetProxyUser(ctx context.Context, id uuid.UUID) (*model.ProxyUser, error) {
	var u model.ProxyUser
	if err := s.db.WithContext(ctx).First(&u, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ListProxyUsers 列出代理用户（可按分组）。
func (s *Service) ListProxyUsers(ctx context.Context, groupID *uuid.UUID) ([]model.ProxyUser, error) {
	var out []model.ProxyUser
	q := s.db.WithContext(ctx).Model(&model.ProxyUser{}).Order("created_at ASC")
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ResetSubToken 重置订阅令牌，返回明文（仅一次）。
func (s *Service) ResetSubToken(ctx context.Context, id uuid.UUID) (string, error) {
	if _, err := s.GetProxyUser(ctx, id); err != nil {
		return "", err
	}
	token, err := crypto.RandomToken(32)
	if err != nil {
		return "", err
	}
	updates := map[string]any{
		"sub_token_hash": crypto.HashToken(token),
		"updated_at":     time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Model(&model.ProxyUser{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return "", err
	}
	return token, nil
}

// Credentials 解密代理用户的 UUID 与协议密码（订阅/下发渲染时调用）。
func (s *Service) Credentials(u *model.ProxyUser) (plainUUID, plainPassword string, err error) {
	if u.UUID != "" {
		plainUUID, err = s.codec.DecryptString(u.UUID)
		if err != nil {
			return "", "", fmt.Errorf("user: 解密 UUID 失败: %w", err)
		}
	}
	if u.PasswordEnc != "" {
		plainPassword, err = s.codec.DecryptString(u.PasswordEnc)
		if err != nil {
			return "", "", fmt.Errorf("user: 解密密码失败: %w", err)
		}
	}
	return plainUUID, plainPassword, nil
}

// ResolveBySubToken 按订阅令牌明文查找代理用户（订阅公开端点用）。
func (s *Service) ResolveBySubToken(ctx context.Context, subToken string) (*model.ProxyUser, error) {
	var u model.ProxyUser
	hash := crypto.HashToken(subToken)
	if err := s.db.WithContext(ctx).First(&u, "sub_token_hash = ?", hash).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) setPassword(u *model.ProxyUser, plain string) error {
	enc, err := s.codec.EncryptString(plain)
	if err != nil {
		return err
	}
	u.PasswordEnc = enc
	u.PasswordHash, err = crypto.HashPassword(plain)
	return err
}

// ============================= 面板用户 =============================

// CreatePanelUser 创建面板用户（密码 argon2id，不可逆）。
func (s *Service) CreatePanelUser(ctx context.Context, u *model.PanelUser, plainPassword string) error {
	if u.Username == "" || plainPassword == "" {
		return fmt.Errorf("user: username/password 必填")
	}
	if u.RoleID == uuid.Nil {
		return fmt.Errorf("user: role_id 必填")
	}
	if _, err := s.getRole(ctx, u.RoleID); err != nil {
		return err
	}
	hash, err := crypto.HashPassword(plainPassword)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	if u.Status == "" {
		u.Status = model.StatusEnabled
	}
	u.ID = uuid.Nil
	return s.db.WithContext(ctx).Create(u).Error
}

// UpdatePanelUser 更新面板用户（plainPassword 非空则重置登录密码）。
func (s *Service) UpdatePanelUser(ctx context.Context, u *model.PanelUser, plainPassword *string) error {
	existing, err := s.getPanelUser(ctx, u.ID)
	if err != nil {
		return err
	}
	u.PasswordHash = existing.PasswordHash
	if plainPassword != nil && *plainPassword != "" {
		hash, err := crypto.HashPassword(*plainPassword)
		if err != nil {
			return err
		}
		u.PasswordHash = hash
	}
	return s.db.WithContext(ctx).Model(&model.PanelUser{}).Where("id = ?", u.ID).Save(u).Error
}

// DeletePanelUser 删除面板用户（super_admin 仅允许删除非本人）。
func (s *Service) DeletePanelUser(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&model.PanelUser{}, "id = ?", id).Error
}

// ListPanelUsers 列出面板用户。
func (s *Service) ListPanelUsers(ctx context.Context) ([]model.PanelUser, error) {
	var out []model.PanelUser
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) getPanelUser(ctx context.Context, id uuid.UUID) (*model.PanelUser, error) {
	var u model.PanelUser
	if err := s.db.WithContext(ctx).First(&u, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// GetPanelUser 获取面板用户（公开导出）。
func (s *Service) GetPanelUser(ctx context.Context, id uuid.UUID) (*model.PanelUser, error) {
	return s.getPanelUser(ctx, id)
}

// ============================= 角色 =============================

// SeedRoles 幂等写入内置三角色（权限来自 rbac 包）。
func (s *Service) SeedRoles(ctx context.Context) error {
	for _, name := range []string{rbac.RoleSuperAdmin, rbac.RoleAdmin, rbac.RoleViewer} {
		var cnt int64
		if err := s.db.WithContext(ctx).Model(&model.Role{}).Where("name = ?", name).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt > 0 {
			continue
		}
		perms, _ := json.Marshal(rbac.PermissionsFor(name))
		r := &model.Role{Name: name, Permissions: perms}
		if err := s.db.WithContext(ctx).Create(r).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedAdmin 幂等创建首个超级管理员（仅当面板用户数为 0 时）。
// 返回 created=true 表示本次确实创建了初始管理员；plainPassword 为空时跳过。
func (s *Service) SeedAdmin(ctx context.Context, username, plainPassword string) (bool, error) {
	if plainPassword == "" {
		return false, nil
	}

	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.PanelUser{}).Count(&cnt).Error; err != nil {
		return false, err
	}
	if cnt > 0 {
		return false, nil
	}

	var role model.Role
	if err := s.db.WithContext(ctx).First(&role, "name = ?", rbac.RoleSuperAdmin).Error; err != nil {
		return false, err
	}

	u := &model.PanelUser{
		Username: username,
		RoleID:   role.ID,
	}
	if err := s.CreatePanelUser(ctx, u, plainPassword); err != nil {
		return false, err
	}
	return true, nil
}

// ListRoles 列出角色。
func (s *Service) ListRoles(ctx context.Context) ([]model.Role, error) {
	var out []model.Role
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// CreateRole 创建角色（MVP 仅允许内置角色名；自定义角色需同步 rbac 包）。
func (s *Service) CreateRole(ctx context.Context, r *model.Role) error {
	if !rbac.IsValidRole(r.Name) {
		return fmt.Errorf("user: 仅支持内置角色 %s", rbac.RoleSuperAdmin+"/"+rbac.RoleAdmin+"/"+rbac.RoleViewer)
	}
	r.ID = uuid.Nil
	return s.db.WithContext(ctx).Create(r).Error
}

// UpdateRole 更新角色权限（仅内置角色）。
func (s *Service) UpdateRole(ctx context.Context, r *model.Role) error {
	if !rbac.IsValidRole(r.Name) {
		return fmt.Errorf("user: 仅支持内置角色")
	}
	return s.db.WithContext(ctx).Model(&model.Role{}).Where("id = ?", r.ID).Save(r).Error
}

// DeleteRole 删除角色（被面板用户引用时拒绝）。
func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) error {
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.PanelUser{}).Where("role_id = ?", id).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return fmt.Errorf("user: 角色被 %d 个面板用户引用，禁止删除", cnt)
	}
	return s.db.WithContext(ctx).Delete(&model.Role{}, "id = ?", id).Error
}

func (s *Service) getRole(ctx context.Context, id uuid.UUID) (*model.Role, error) {
	var r model.Role
	if err := s.db.WithContext(ctx).First(&r, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

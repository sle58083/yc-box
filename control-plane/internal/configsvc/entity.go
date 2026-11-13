// Package configsvc 是配置管理域：inbound/outbound/routing CRUD、
// 中立模型 → sing-box JSON 生成、Schema 校验、配置版本管理。
package configsvc

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/configsvc/renderer"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
)

// Service 是配置管理域的服务入口。
type Service struct {
	db        *store.DB
	codec     *crypto.Codec
	schema    *Validator
	generator *Generator
}

// NewService 构造配置服务；schemaDir 为空则跳过第二层 Schema 校验。
func NewService(db *store.DB, codec *crypto.Codec, schemaDir string) (*Service, error) {
	s := &Service{db: db, codec: codec, generator: NewGenerator()}
	if schemaDir != "" {
		v, err := NewValidator(schemaDir)
		if err != nil {
			return nil, err
		}
		s.schema = v
	}
	return s, nil
}

// ============================= Inbound CRUD =============================

// CreateInbound 创建入站配置。
func (s *Service) CreateInbound(ctx context.Context, cfg *model.InboundConfig) error {
	if err := validateInbound(ctx, s.db.DB, cfg, nil); err != nil {
		return err
	}
	normalizeInbound(cfg)
	if err := s.encryptInboundTLS(cfg); err != nil {
		return err
	}
	cfg.ID = uuid.Nil // 由 Base.BeforeCreate 生成 UUID v4
	return s.db.WithContext(ctx).Create(cfg).Error
}

// UpdateInbound 更新入站配置。
func (s *Service) UpdateInbound(ctx context.Context, cfg *model.InboundConfig) error {
	existing, err := s.GetInbound(ctx, cfg.ID)
	if err != nil {
		return err
	}
	// 保留不可变归属字段，避免越权改作用域。
	cfg.GroupID = existing.GroupID
	cfg.NodeID = existing.NodeID
	if err := validateInbound(ctx, s.db.DB, cfg, existing); err != nil {
		return err
	}
	normalizeInbound(cfg)
	if err := s.encryptInboundTLS(cfg); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&model.InboundConfig{}).Where("id = ?", cfg.ID).Save(cfg).Error
}

// DeleteInbound 删除入站（被路由规则引用时拒绝）。
func (s *Service) DeleteInbound(ctx context.Context, id uuid.UUID) error {
	cfg, err := s.GetInbound(ctx, id)
	if err != nil {
		return err
	}
	if err := checkInboundReferences(ctx, s.db.DB, cfg.Name); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Delete(&model.InboundConfig{}, "id = ?", id).Error
}

// GetInbound 获取单个入站。
func (s *Service) GetInbound(ctx context.Context, id uuid.UUID) (*model.InboundConfig, error) {
	var cfg model.InboundConfig
	if err := s.db.WithContext(ctx).First(&cfg, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ListInbounds 列出某分组（或全部）的入站。
func (s *Service) ListInbounds(ctx context.Context, groupID *uuid.UUID) ([]model.InboundConfig, error) {
	var out []model.InboundConfig
	q := s.db.WithContext(ctx).Model(&model.InboundConfig{}).Order("created_at ASC")
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ============================= Outbound CRUD =============================

// CreateOutbound 创建出站配置。
func (s *Service) CreateOutbound(ctx context.Context, cfg *model.OutboundConfig) error {
	if err := validateOutbound(ctx, s.db.DB, cfg, nil); err != nil {
		return err
	}
	normalizeOutbound(cfg)
	if err := s.encryptOutboundTLS(cfg); err != nil {
		return err
	}
	cfg.ID = uuid.Nil
	return s.db.WithContext(ctx).Create(cfg).Error
}

// UpdateOutbound 更新出站配置。
func (s *Service) UpdateOutbound(ctx context.Context, cfg *model.OutboundConfig) error {
	existing, err := s.GetOutbound(ctx, cfg.ID)
	if err != nil {
		return err
	}
	cfg.GroupID = existing.GroupID
	cfg.NodeID = existing.NodeID
	if err := validateOutbound(ctx, s.db.DB, cfg, existing); err != nil {
		return err
	}
	normalizeOutbound(cfg)
	if err := s.encryptOutboundTLS(cfg); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&model.OutboundConfig{}).Where("id = ?", cfg.ID).Save(cfg).Error
}

// DeleteOutbound 删除出站（被路由规则引用时拒绝）。
func (s *Service) DeleteOutbound(ctx context.Context, id uuid.UUID) error {
	cfg, err := s.GetOutbound(ctx, id)
	if err != nil {
		return err
	}
	if err := checkOutboundReferences(ctx, s.db.DB, cfg.Name); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Delete(&model.OutboundConfig{}, "id = ?", id).Error
}

// GetOutbound 获取单个出站。
func (s *Service) GetOutbound(ctx context.Context, id uuid.UUID) (*model.OutboundConfig, error) {
	var cfg model.OutboundConfig
	if err := s.db.WithContext(ctx).First(&cfg, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ListOutbounds 列出某分组（或全部）的出站。
func (s *Service) ListOutbounds(ctx context.Context, groupID *uuid.UUID) ([]model.OutboundConfig, error) {
	var out []model.OutboundConfig
	q := s.db.WithContext(ctx).Model(&model.OutboundConfig{}).Order("created_at ASC")
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ============================= Routing CRUD =============================

// CreateRoutingRule 创建路由规则。
func (s *Service) CreateRoutingRule(ctx context.Context, cfg *model.RoutingRule) error {
	if err := validateRoutingRule(ctx, s.db.DB, cfg, nil); err != nil {
		return err
	}
	cfg.ID = uuid.Nil
	return s.db.WithContext(ctx).Create(cfg).Error
}

// UpdateRoutingRule 更新路由规则。
func (s *Service) UpdateRoutingRule(ctx context.Context, cfg *model.RoutingRule) error {
	existing, err := s.GetRoutingRule(ctx, cfg.ID)
	if err != nil {
		return err
	}
	cfg.GroupID = existing.GroupID
	cfg.NodeID = existing.NodeID
	if err := validateRoutingRule(ctx, s.db.DB, cfg, existing); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&model.RoutingRule{}).Where("id = ?", cfg.ID).Save(cfg).Error
}

// DeleteRoutingRule 删除路由规则。
func (s *Service) DeleteRoutingRule(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&model.RoutingRule{}, "id = ?", id).Error
}

// GetRoutingRule 获取单个路由规则。
func (s *Service) GetRoutingRule(ctx context.Context, id uuid.UUID) (*model.RoutingRule, error) {
	var cfg model.RoutingRule
	if err := s.db.WithContext(ctx).First(&cfg, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ListRoutingRules 列出某分组（或全部）的路由规则。
func (s *Service) ListRoutingRules(ctx context.Context, groupID *uuid.UUID) ([]model.RoutingRule, error) {
	var out []model.RoutingRule
	q := s.db.WithContext(ctx).Model(&model.RoutingRule{}).Order("priority ASC, created_at ASC")
	if groupID != nil {
		q = q.Where("group_id = ?", *groupID)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ============================= 聚合（模板 + 覆盖合并） =============================

// aggregate 组装单节点 NodeConfigContext：合并分组模板与节点覆盖，
// 过滤 status=disabled，并解密该分组 active 代理用户。
func (s *Service) aggregate(ctx context.Context, nodeID uuid.UUID) (*NodeConfigContext, error) {
	var node model.Node
	if err := s.db.WithContext(ctx).First(&node, "id = ?", nodeID).Error; err != nil {
		return nil, fmt.Errorf("configsvc: 节点不存在: %w", err)
	}

	inbounds, err := s.aggregateInbounds(ctx, node.GroupID, nodeID)
	if err != nil {
		return nil, err
	}
	outbounds, err := s.aggregateOutbounds(ctx, node.GroupID, nodeID)
	if err != nil {
		return nil, err
	}
	rules, err := s.aggregateRules(ctx, node.GroupID, nodeID)
	if err != nil {
		return nil, err
	}
	users, err := s.activeProxyUsers(ctx, node.GroupID)
	if err != nil {
		return nil, err
	}

	return &NodeConfigContext{
		Global:     DefaultGlobalSettings(),
		Inbounds:   inbounds,
		Outbounds:  outbounds,
		Rules:      rules,
		ProxyUsers: users,
	}, nil
}

func (s *Service) aggregateInbounds(ctx context.Context, groupID, nodeID uuid.UUID) ([]model.InboundConfig, error) {
	var templates, overrides []model.InboundConfig
	if err := s.db.WithContext(ctx).Where("group_id = ? AND node_id IS NULL", groupID).Find(&templates).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("node_id = ?", nodeID).Find(&overrides).Error; err != nil {
		return nil, err
	}
	merged := mergeInbounds(templates, overrides)
	out := filterInbounds(merged)
	for i := range out {
		if err := s.decryptInboundTLS(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) aggregateOutbounds(ctx context.Context, groupID, nodeID uuid.UUID) ([]model.OutboundConfig, error) {
	var templates, overrides []model.OutboundConfig
	if err := s.db.WithContext(ctx).Where("group_id = ? AND node_id IS NULL", groupID).Find(&templates).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("node_id = ?", nodeID).Find(&overrides).Error; err != nil {
		return nil, err
	}
	merged := mergeOutbounds(templates, overrides)
	out := filterOutbounds(merged)
	for i := range out {
		if err := s.decryptOutboundTLS(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) aggregateRules(ctx context.Context, groupID, nodeID uuid.UUID) ([]model.RoutingRule, error) {
	var templates, overrides []model.RoutingRule
	if err := s.db.WithContext(ctx).Where("group_id = ? AND node_id IS NULL", groupID).Find(&templates).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("node_id = ?", nodeID).Find(&overrides).Error; err != nil {
		return nil, err
	}
	merged := mergeRules(templates, overrides)
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Priority < merged[j].Priority })
	return merged, nil
}

// activeProxyUsers 返回该分组 active 代理用户（UUID/密码均解密注入）。
func (s *Service) activeProxyUsers(ctx context.Context, groupID uuid.UUID) ([]renderer.ProxyUser, error) {
	var users []model.ProxyUser
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND status = ? AND (expire_at IS NULL OR expire_at > ?)", groupID, model.StatusEnabled, now).
		Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]renderer.ProxyUser, 0, len(users))
	for _, u := range users {
		out = append(out, decryptProxyUser(u, s.codec))
	}
	return out, nil
}

// decryptProxyUser 将落库的代理用户解密为渲染视图（UUID 与协议密码均为可逆 🔐）。
func decryptProxyUser(u model.ProxyUser, codec *crypto.Codec) renderer.ProxyUser {
	pu := renderer.ProxyUser{ID: u.ID.String()}
	if u.UUID != "" {
		if plain, err := codec.DecryptString(u.UUID); err == nil {
			pu.UUID = plain
			pu.Name = "user-" + firstN(plain, 4)
		}
	}
	// 密码类协议（trojan/hysteria2/tuic/anytls）需明文密码注入订阅/下发。
	if u.PasswordEnc != "" {
		if plain, err := codec.DecryptString(u.PasswordEnc); err == nil {
			pu.Password = plain
		}
	}
	return pu
}

// ============================= 合并与过滤 =============================

func mergeInbounds(templates, overrides []model.InboundConfig) []model.InboundConfig {
	m := map[string]model.InboundConfig{}
	for _, t := range templates {
		m[t.Name] = t
	}
	for _, o := range overrides {
		m[o.Name] = o // 同名覆盖
	}
	return sortedInbounds(m)
}

func mergeOutbounds(templates, overrides []model.OutboundConfig) []model.OutboundConfig {
	m := map[string]model.OutboundConfig{}
	for _, t := range templates {
		m[t.Name] = t
	}
	for _, o := range overrides {
		m[o.Name] = o
	}
	return sortedOutbounds(m)
}

func mergeRules(templates, overrides []model.RoutingRule) []model.RoutingRule {
	m := map[string]model.RoutingRule{}
	for _, t := range templates {
		m[t.Name] = t
	}
	for _, o := range overrides {
		m[o.Name] = o
	}
	out := make([]model.RoutingRule, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func sortedInbounds(m map[string]model.InboundConfig) []model.InboundConfig {
	out := make([]model.InboundConfig, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedOutbounds(m map[string]model.OutboundConfig) []model.OutboundConfig {
	out := make([]model.OutboundConfig, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// filterInbounds 过滤 status=disabled（§5.2 启停过滤）。
func filterInbounds(in []model.InboundConfig) []model.InboundConfig {
	out := make([]model.InboundConfig, 0, len(in))
	for _, c := range in {
		if c.Status != model.StatusDisabled {
			out = append(out, c)
		}
	}
	return out
}

// filterOutbounds 过滤 status=disabled。
func filterOutbounds(in []model.OutboundConfig) []model.OutboundConfig {
	out := make([]model.OutboundConfig, 0, len(in))
	for _, c := range in {
		if c.Status != model.StatusDisabled {
			out = append(out, c)
		}
	}
	return out
}

// ============================= 辅助 =============================

// encryptInboundTLS 加密入站 tls JSONB 中的敏感键（Reality private_key 等）。
func (s *Service) encryptInboundTLS(cfg *model.InboundConfig) error {
	enc, err := s.codec.EncryptJSONSecrets(cfg.TLS)
	if err != nil {
		return fmt.Errorf("configsvc: 加密入站 tls 失败: %w", err)
	}
	cfg.TLS = enc
	return nil
}

// encryptOutboundTLS 加密出站 tls JSONB 中的敏感键。
func (s *Service) encryptOutboundTLS(cfg *model.OutboundConfig) error {
	enc, err := s.codec.EncryptJSONSecrets(cfg.TLS)
	if err != nil {
		return fmt.Errorf("configsvc: 加密出站 tls 失败: %w", err)
	}
	cfg.TLS = enc
	return nil
}

// decryptInboundTLS 解密入站 tls JSONB 中的敏感键（生成/下发前）。
func (s *Service) decryptInboundTLS(cfg *model.InboundConfig) error {
	dec, err := s.codec.DecryptJSONSecrets(cfg.TLS)
	if err != nil {
		return fmt.Errorf("configsvc: 解密入站 tls 失败: %w", err)
	}
	cfg.TLS = dec
	return nil
}

// decryptOutboundTLS 解密出站 tls JSONB 中的敏感键。
func (s *Service) decryptOutboundTLS(cfg *model.OutboundConfig) error {
	dec, err := s.codec.DecryptJSONSecrets(cfg.TLS)
	if err != nil {
		return fmt.Errorf("configsvc: 解密出站 tls 失败: %w", err)
	}
	cfg.TLS = dec
	return nil
}

func normalizeInbound(cfg *model.InboundConfig) {
	if cfg.Status == "" {
		cfg.Status = model.StatusEnabled
	}
	if cfg.Listen == "" {
		cfg.Listen = "0.0.0.0"
	}
}

func normalizeOutbound(cfg *model.OutboundConfig) {
	if cfg.Status == "" {
		cfg.Status = model.StatusEnabled
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

package configsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/configsvc/renderer"
	"github.com/singbox-panel/control-plane/internal/model"
	"gorm.io/gorm"
)

// ValidationError 是可识别的业务校验错误（映射为统一响应码 4xxx）。
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func newValidationError(msg string) *ValidationError { return &ValidationError{msg: msg} }

// ValidationErrorf 构造校验错误。
func ValidationErrorf(format string, a ...any) error {
	return &ValidationError{msg: fmt.Sprintf(format, a...)}
}

// IsValidationError 判断是否为业务校验错误（含 %w 包裹）。
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// 业务校验错误哨兵（保存前第一层校验，system_design.md §5.5）。
var (
	ErrNameRequired      = newValidationError("configsvc: name 必填")
	ErrInvalidPort       = newValidationError("configsvc: 端口必须在 1-65535")
	ErrTagConflict       = newValidationError("configsvc: tag 在分组+节点范围内已存在")
	ErrPortConflict      = newValidationError("configsvc: 相同 listen+port 的入站已存在")
	ErrReferenceNotFound = newValidationError("configsvc: 引用的 tag 不存在")
	ErrInvalidRuleType   = newValidationError("configsvc: rule_type 非法")
	ErrInvalidRuleValue  = newValidationError("configsvc: rule value 非法")
	ErrProtocolNotFound  = newValidationError("configsvc: 未注册的协议类型")
)

var domainRegex = regexp.MustCompile(`^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// validateInbound 入站保存前业务校验 + 委托协议校验。
func validateInbound(ctx context.Context, db *gorm.DB, cfg *model.InboundConfig, existing *model.InboundConfig) error {
	if cfg.Name == "" {
		return ErrNameRequired
	}
	if cfg.Type == "" {
		return ErrProtocolNotFound
	}
	r, ok := renderer.Get(cfg.Type)
	if !ok {
		return fmt.Errorf("%w: %s", ErrProtocolNotFound, cfg.Type)
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return ErrInvalidPort
	}
	if err := r.ValidateInbound(*cfg); err != nil {
		return ValidationErrorf("%v", err)
	}

	if err := checkInboundTagUnique(ctx, db, cfg, existing); err != nil {
		return err
	}
	return checkPortConflict(ctx, db, cfg, existing)
}

// validateOutbound 出站保存前业务校验 + 委托协议校验。
func validateOutbound(ctx context.Context, db *gorm.DB, cfg *model.OutboundConfig, existing *model.OutboundConfig) error {
	if cfg.Name == "" {
		return ErrNameRequired
	}
	if cfg.Type == "" {
		return ErrProtocolNotFound
	}
	r, ok := renderer.Get(cfg.Type)
	if !ok {
		return fmt.Errorf("%w: %s", ErrProtocolNotFound, cfg.Type)
	}
	if err := r.ValidateOutbound(*cfg); err != nil {
		return ValidationErrorf("%v", err)
	}
	return checkOutboundTagUnique(ctx, db, cfg, existing)
}

// validateRoutingRule 路由规则保存前业务校验（含引用完整性）。
func validateRoutingRule(ctx context.Context, db *gorm.DB, cfg *model.RoutingRule, existing *model.RoutingRule) error {
	if cfg.Name == "" {
		return ErrNameRequired
	}
	if !validRuleType(cfg.RuleType) {
		return ErrInvalidRuleType
	}
	if cfg.Value == "" {
		return ErrInvalidRuleValue
	}
	if cfg.RuleType == "domain" && !domainRegex.MatchString(cfg.Value) {
		return ValidationErrorf("rule value 域名 %q 非法", cfg.Value)
	}
	if cfg.RuleType == "ip" {
		if net.ParseIP(cfg.Value) == nil {
			if _, _, err := net.ParseCIDR(cfg.Value); err != nil {
				return ValidationErrorf("rule value IP/CIDR %q 非法", cfg.Value)
			}
		}
	}
	if cfg.Priority < 0 {
		return ValidationErrorf("priority 不能为负")
	}

	if cfg.OutboundTag != "" && !outboundTagExists(ctx, db, cfg.GroupID, cfg.NodeID, cfg.OutboundTag) {
		return fmt.Errorf("%w: outbound %q", ErrReferenceNotFound, cfg.OutboundTag)
	}
	var tags []string
	if len(cfg.InboundTags) > 0 {
		if err := json.Unmarshal(cfg.InboundTags, &tags); err != nil {
			return ValidationErrorf("inbound_tags 必须为字符串数组: %v", err)
		}
	}
	for _, t := range tags {
		if !inboundTagExists(ctx, db, cfg.GroupID, cfg.NodeID, t) {
			return fmt.Errorf("%w: inbound %q", ErrReferenceNotFound, t)
		}
	}
	_ = existing
	return nil
}

func validRuleType(t string) bool {
	switch t {
	case "geosite", "geoip", "domain", "ip", "protocol":
		return true
	}
	return false
}

func checkInboundTagUnique(ctx context.Context, db *gorm.DB, cfg *model.InboundConfig, existing *model.InboundConfig) error {
	q := db.WithContext(ctx).Model(&model.InboundConfig{}).
		Where("group_id = ? AND name = ?", cfg.GroupID, cfg.Name)
	q = scopeNode(q, "node_id", cfg.NodeID)
	if existing != nil {
		q = q.Where("id <> ?", existing.ID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return ErrTagConflict
	}
	return nil
}

func checkOutboundTagUnique(ctx context.Context, db *gorm.DB, cfg *model.OutboundConfig, existing *model.OutboundConfig) error {
	q := db.WithContext(ctx).Model(&model.OutboundConfig{}).
		Where("group_id = ? AND name = ?", cfg.GroupID, cfg.Name)
	q = scopeNode(q, "node_id", cfg.NodeID)
	if existing != nil {
		q = q.Where("id <> ?", existing.ID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return ErrTagConflict
	}
	return nil
}

func checkPortConflict(ctx context.Context, db *gorm.DB, cfg *model.InboundConfig, existing *model.InboundConfig) error {
	q := db.WithContext(ctx).Model(&model.InboundConfig{}).
		Where("group_id = ? AND port = ? AND status <> ?", cfg.GroupID, cfg.Port, model.StatusDisabled)
	if cfg.Listen != "" {
		q = q.Where("listen = ?", cfg.Listen)
	}
	q = scopeNode(q, "node_id", cfg.NodeID)
	if existing != nil {
		q = q.Where("id <> ?", existing.ID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return ErrPortConflict
	}
	return nil
}

// scopeNode 处理 nullable node_id 的范围过滤。
func scopeNode(q *gorm.DB, col string, nodeID *uuid.UUID) *gorm.DB {
	if nodeID != nil {
		return q.Where(col+" = ?", *nodeID)
	}
	return q.Where(col + " IS NULL")
}

// outboundTagExists 判断分组模板 + 节点覆盖中是否存在指定 enabled 出站 tag。
func outboundTagExists(ctx context.Context, db *gorm.DB, groupID uuid.UUID, nodeID *uuid.UUID, tag string) bool {
	var tpl, ovr int64
	db.WithContext(ctx).Model(&model.OutboundConfig{}).
		Where("group_id = ? AND node_id IS NULL AND name = ? AND status <> ?", groupID, tag, model.StatusDisabled).
		Count(&tpl)
	if nodeID != nil {
		db.WithContext(ctx).Model(&model.OutboundConfig{}).
			Where("node_id = ? AND name = ? AND status <> ?", *nodeID, tag, model.StatusDisabled).
			Count(&ovr)
	}
	return tpl+ovr > 0
}

// inboundTagExists 判断分组模板 + 节点覆盖中是否存在指定 enabled 入站 tag。
func inboundTagExists(ctx context.Context, db *gorm.DB, groupID uuid.UUID, nodeID *uuid.UUID, tag string) bool {
	var tpl, ovr int64
	db.WithContext(ctx).Model(&model.InboundConfig{}).
		Where("group_id = ? AND node_id IS NULL AND name = ? AND status <> ?", groupID, tag, model.StatusDisabled).
		Count(&tpl)
	if nodeID != nil {
		db.WithContext(ctx).Model(&model.InboundConfig{}).
			Where("node_id = ? AND name = ? AND status <> ?", *nodeID, tag, model.StatusDisabled).
			Count(&ovr)
	}
	return tpl+ovr > 0
}

// checkInboundReferences 删除入站前检查路由规则引用。
//
// SQLite 无 PostgreSQL 的 jsonb @> 包含运算符（且 inbound_tags 在 SQLite 下为 BLOB），
// 故改为读取引用列表后在应用层判断包含关系。面板路由规则规模有限，性能可接受。
func checkInboundReferences(ctx context.Context, db *gorm.DB, tag string) error {
	var rules []model.RoutingRule
	if err := db.WithContext(ctx).Model(&model.RoutingRule{}).
		Select("inbound_tags").Find(&rules).Error; err != nil {
		return err
	}

	cnt := 0
	for _, r := range rules {
		var tags []string
		if err := json.Unmarshal(r.InboundTags, &tags); err != nil {
			continue
		}
		for _, t := range tags {
			if t == tag {
				cnt++
				break
			}
		}
	}
	if cnt > 0 {
		return ValidationErrorf("入站 %q 被 %d 条路由规则引用，禁止删除", tag, cnt)
	}
	return nil
}

func checkOutboundReferences(ctx context.Context, db *gorm.DB, tag string) error {
	var cnt int64
	if err := db.WithContext(ctx).Model(&model.RoutingRule{}).
		Where("outbound_tag = ?", tag).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return ValidationErrorf("出站 %q 被 %d 条路由规则引用，禁止删除", tag, cnt)
	}
	return nil
}

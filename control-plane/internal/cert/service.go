// Package cert 是证书管理域：自签证书 + 手动导入、加密落库、CRUD。
//
// 设计约定（sui-gap-design.md §1.3 / §3.1 / §9）：
//   - 证书 PEM（CertPEM / KeyPEM）落库前必须经 crypto.Codec.EncryptField 加密；
//   - 下发前由 configsvc 调用 DecryptField 还原明文，经 WS file_push 下发到 Agent；
//   - 删除证书前检查 inbound_config.cert_id 引用，被引用则拒绝。
package cert

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
)

// Service 是证书管理域的服务入口。
type Service struct {
	db    *store.DB
	codec *crypto.Codec
}

// New 构造证书服务。
func New(db *store.DB, codec *crypto.Codec) *Service {
	return &Service{db: db, codec: codec}
}

// CreateSelfSigned 一键生成自签证书并加密落库（type=self_signed, status=valid）。
func (s *Service) CreateSelfSigned(ctx context.Context, name string, domains []string, validDays int) (*model.Certificate, error) {
	if name == "" {
		return nil, validationErrorf("证书名称必填")
	}
	if len(domains) == 0 {
		return nil, validationErrorf("自签证书至少需要一个域名（SAN）")
	}

	certPEM, keyPEM, err := generateSelfSigned(domains, validDays)
	if err != nil {
		return nil, err
	}
	parsed, err := parseCert(certPEM)
	if err != nil {
		return nil, err
	}
	if err := validateKeyPEM(keyPEM); err != nil {
		return nil, err
	}

	return s.persist(ctx, name, "self_signed", domains, certPEM, keyPEM, parsed)
}

// ImportCert 手动导入 PEM 证书 + 私钥并加密落库（type=manual, status=valid）。
func (s *Service) ImportCert(ctx context.Context, name string, certPEM, keyPEM string) (*model.Certificate, error) {
	if name == "" {
		return nil, validationErrorf("证书名称必填")
	}
	if certPEM == "" || keyPEM == "" {
		return nil, validationErrorf("cert 与 key 均必填")
	}

	parsed, err := parseCert(certPEM)
	if err != nil {
		return nil, err
	}
	if err := validateKeyPEM(keyPEM); err != nil {
		return nil, err
	}
	return s.persist(ctx, name, "manual", parsed.Domains, certPEM, keyPEM, parsed)
}

// List 返回全部证书（CertPEM/KeyPEM 为 json:"-"，不会随响应泄露明文/密文）。
func (s *Service) List(ctx context.Context) ([]model.Certificate, error) {
	var out []model.Certificate
	if err := s.db.WithContext(ctx).Model(&model.Certificate{}).
		Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Get 返回单个证书。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Certificate, error) {
	var c model.Certificate
	if err := s.db.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// Delete 删除证书；被任一入站绑定（inbound_config.cert_id）时拒绝。
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.InboundConfig{}).
		Where("cert_id = ?", id).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return validationErrorf("证书被 %d 个入站引用，禁止删除", cnt)
	}
	return s.db.WithContext(ctx).Delete(&model.Certificate{}, "id = ?", id).Error
}

// persist 加密敏感字段并落库。
func (s *Service) persist(
	ctx context.Context,
	name, typ string,
	domains []string,
	certPEM, keyPEM string,
	parsed *parsedCert,
) (*model.Certificate, error) {
	encCert, err := s.codec.EncryptField(certPEM)
	if err != nil {
		return nil, fmt.Errorf("cert: 加密证书 PEM 失败: %w", err)
	}
	encKey, err := s.codec.EncryptField(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("cert: 加密私钥 PEM 失败: %w", err)
	}

	domainsJSON, err := json.Marshal(domains)
	if err != nil {
		return nil, fmt.Errorf("cert: 序列化 domains 失败: %w", err)
	}

	c := &model.Certificate{
		Name:      name,
		Type:      typ,
		Domains:   string(domainsJSON),
		CertPEM:   encCert,
		KeyPEM:    encKey,
		Issuer:    parsed.Issuer,
		NotBefore: parsed.NotBefore,
		NotAfter:  parsed.NotAfter,
		Status:    "valid",
	}
	if err := s.db.WithContext(ctx).Create(c).Error; err != nil {
		return nil, err
	}
	return c, nil
}

// parsedCert 是从 PEM 解析出的证书元信息（不包含私钥）。
type parsedCert struct {
	Issuer    string
	NotBefore *time.Time
	NotAfter  *time.Time
	Domains   []string // DNS SAN + IP SAN（手动导入时回填）
}

// parseCert 解析证书 PEM 并提取 issuer / 有效期 / SAN。
func parseCert(certPEM string) (*parsedCert, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, validationErrorf("证书 PEM 无法解析（需包含 CERTIFICATE 块）")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, validationErrorf("证书解析失败: %v", err)
	}

	issuer := c.Issuer.CommonName
	if issuer == "" {
		issuer = c.Issuer.String()
	}

	domains := make([]string, 0, len(c.DNSNames)+len(c.IPAddresses))
	domains = append(domains, c.DNSNames...)
	for _, ip := range c.IPAddresses {
		domains = append(domains, ip.String())
	}

	nb := c.NotBefore.UTC()
	na := c.NotAfter.UTC()
	return &parsedCert{
		Issuer:    issuer,
		NotBefore: &nb,
		NotAfter:  &na,
		Domains:   domains,
	}, nil
}

// validateKeyPEM 校验私钥 PEM 可解析（PKCS#8 / SEC1 EC / PKCS#1 RSA）。
func validateKeyPEM(keyPEM string) error {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return validationErrorf("私钥 PEM 无法解析（需包含 PRIVATE KEY 块）")
	}
	switch block.Type {
	case "PRIVATE KEY":
		if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
			return validationErrorf("私钥解析失败: %v", err)
		}
	case "EC PRIVATE KEY":
		if _, err := x509.ParseECPrivateKey(block.Bytes); err != nil {
			return validationErrorf("私钥解析失败: %v", err)
		}
	case "RSA PRIVATE KEY":
		if _, err := x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
			return validationErrorf("私钥解析失败: %v", err)
		}
	default:
		return validationErrorf("不支持的私钥 PEM 类型 %q", block.Type)
	}
	return nil
}

// ValidationError 是可识别的业务校验错误（映射为统一响应码 4xxx）。
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func validationErrorf(format string, a ...any) error {
	return &ValidationError{msg: fmt.Sprintf(format, a...)}
}

// IsValidationError 判断是否为业务校验错误（含 %w 包裹）。
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

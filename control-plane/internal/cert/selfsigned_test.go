package cert

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/store"
)

// TestGenerateSelfSigned 验证自签证书可解析、SAN 覆盖、有效期正确。
func TestGenerateSelfSigned(t *testing.T) {
	domains := []string{"panel.example.com", "*.panel.example.com", "127.0.0.1"}
	certPEM, keyPEM, err := generateSelfSigned(domains, 365)
	if err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}

	block, _ := pem.Decode([]byte(certPEM))
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("证书 PEM 缺少 CERTIFICATE 块")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if cert.Subject.CommonName != domains[0] {
		t.Fatalf("CommonName = %q, want %q", cert.Subject.CommonName, domains[0])
	}
	if len(cert.DNSNames) != 2 {
		t.Fatalf("DNSNames = %v, want 2 个 DNS SAN", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 1 || cert.IPAddresses[0].String() != "127.0.0.1" {
		t.Fatalf("IP SAN 未正确解析: %v", cert.IPAddresses)
	}
	wantAfter := time.Now().UTC().Add(365 * 24 * time.Hour)
	if cert.NotAfter.Before(wantAfter.Add(-24*time.Hour)) || cert.NotAfter.After(wantAfter.Add(24*time.Hour)) {
		t.Fatalf("NotAfter 与 365 天不符: %v", cert.NotAfter)
	}

	if !strings.Contains(keyPEM, "PRIVATE KEY") {
		t.Fatalf("私钥 PEM 缺少 PRIVATE KEY 块")
	}
	if err := validateKeyPEM(keyPEM); err != nil {
		t.Fatalf("自签私钥应可解析: %v", err)
	}
}

// TestServiceEncryptsAndStripsPEM 验证创建/导入后 PEM 密文落库、列表不暴露。
func TestServiceEncryptsAndStripsPEM(t *testing.T) {
	km, err := crypto.NewStaticKeyManager(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewStaticKeyManager: %v", err)
	}
	codec := crypto.New(km)
	db, err := store.New(filepath.Join(t.TempDir(), "panel.db"), "silent")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	svc := New(db, codec)

	created, err := svc.CreateSelfSigned(context.Background(), "panel", []string{"panel.example.com"}, 0)
	if err != nil {
		t.Fatalf("CreateSelfSigned: %v", err)
	}
	if created.Type != "self_signed" || created.Status != "valid" {
		t.Fatalf("type/status 不符: %+v", created)
	}
	if !crypto.IsEncryptedField(created.CertPEM) || !crypto.IsEncryptedField(created.KeyPEM) {
		t.Fatalf("证书/私钥应为密文落库")
	}
	if strings.Contains(created.CertPEM, "BEGIN CERTIFICATE") {
		t.Fatalf("证书明文不应出现在密文中")
	}

	// 手动导入：复用自签生成的一对 PEM。
	certPEM, keyPEM, err := generateSelfSigned([]string{"manual.example.com"}, 30)
	if err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}
	imported, err := svc.ImportCert(context.Background(), "manual", certPEM, keyPEM)
	if err != nil {
		t.Fatalf("ImportCert: %v", err)
	}
	if imported.Type != "manual" {
		t.Fatalf("导入类型应为 manual: %q", imported.Type)
	}
	if !crypto.IsEncryptedField(imported.CertPEM) {
		t.Fatalf("导入证书应为密文落库")
	}

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List 期望 2 条，实际 %d", len(list))
	}
	// json:"-" 确保 JSON 序列化时不出现 cert_pem/key_pem（明文或密文都不应泄露）。
	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(raw), "cert_pem") || strings.Contains(string(raw), "key_pem") {
		t.Fatalf("列表 JSON 不应包含 cert_pem/key_pem: %s", raw)
	}
}

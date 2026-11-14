package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// defaultValidDays 是自签证书默认有效期（天），与 sui-gap-design.md §8 一致。
const defaultValidDays = 365

// generateSelfSigned 生成一张 ECDSA P-256 自签服务器证书。
//
// domains 同时作为 Subject.CommonName（取首个）与 SAN（DNS/IP 自动区分），
// 供 sing-box TLS / Hysteria2 / TUIC 等协议直接引用。validDays<=0 时取默认 365 天。
// 返回 PEM 编码的证书与私钥（私钥为标准库 EC PRIVATE KEY 块）。
func generateSelfSigned(domains []string, validDays int) (certPEM, keyPEM string, err error) {
	if validDays <= 0 {
		validDays = defaultValidDays
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("cert: 生成 ECDSA 密钥失败: %w", err)
	}

	// 128-bit 随机序列号，避免碰撞（RFC 5280 要求正整数且尽量唯一）。
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("cert: 生成序列号失败: %w", err)
	}

	now := time.Now().UTC()
	notAfter := now.Add(time.Duration(validDays) * 24 * time.Hour)

	cn := "self-signed"
	if len(domains) > 0 {
		cn = domains[0]
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"yc-box"},
		},
		NotBefore:             now.Add(-5 * time.Minute), // 回拨 5 分钟，规避节点时钟轻微偏差
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, d := range domains {
		if ip := net.ParseIP(d); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, d)
		}
	}

	// 父证书 = 自身，即自签。
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("cert: 签发证书失败: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", fmt.Errorf("cert: 序列化私钥失败: %w", err)
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM, nil
}

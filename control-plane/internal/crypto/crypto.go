// Package crypto 提供控制面安全内核：
//   - AES-256-GCM 加解密（🔐 敏感字段落库）
//   - argon2id 密码哈希（# 不可逆）
//   - SHA-256 令牌哈希（#）
//   - 安全随机数
//
// 设计约定（system_design.md §9.5）：入库前统一走本包；明文只存在于
// 生成 / 下发内存链路中。KeyManager 接口为后续接入 KMS/Vault 预留（§10.8）。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id 参数（OWASP 推荐基线）。
const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024 // 64 MiB
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

// KeyManager 抽象密钥来源，便于后续接入 KMS/Vault（§10.8）。
type KeyManager interface {
	// DataKey 返回 32 字节 AES-256 密钥。
	DataKey() []byte
}

// StaticKeyManager 应用级静态密钥实现。
type StaticKeyManager struct {
	key []byte
}

// NewStaticKeyManager 构造静态密钥管理器，要求密钥为 32 字节。
func NewStaticKeyManager(key []byte) (*StaticKeyManager, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: AES-256 key must be 32 bytes, got %d", len(key))
	}
	return &StaticKeyManager{key: append([]byte(nil), key...)}, nil
}

// DataKey 返回密钥副本。
func (m *StaticKeyManager) DataKey() []byte { return append([]byte(nil), m.key...) }

// Codec 是敏感字段加解密器。
type Codec struct {
	km KeyManager
}

// New 构造 Codec。
func New(km KeyManager) *Codec { return &Codec{km: km} }

// Encrypt AES-256-GCM 加密，返回 base64(nonce || ciphertext)。
func (c *Codec) Encrypt(plaintext []byte) (string, error) {
	gcm, err := c.newGCM()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: read nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 Encrypt 的输出。
func (c *Codec) Decrypt(encoded string) ([]byte, error) {
	gcm, err := c.newGCM()
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode ciphertext: %w", err)
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce, sealed := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}
	return plain, nil
}

// EncryptString 是 Encrypt 的字符串便捷方法。
func (c *Codec) EncryptString(plaintext string) (string, error) {
	return c.Encrypt([]byte(plaintext))
}

// DecryptString 是 Decrypt 的字符串便捷方法。
func (c *Codec) DecryptString(encoded string) (string, error) {
	plain, err := c.Decrypt(encoded)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (c *Codec) newGCM() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.km.DataKey())
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	return gcm, nil
}

// HashPassword 使用 argon2id 对密码哈希，返回 PHC 风格字符串。
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("crypto: read salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword 校验密码与 PHC 风格哈希是否匹配（常量时间比较）。
func VerifyPassword(password string, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// 期望：["", "argon2id", "v=19", "m=65536,t=3,p=4", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("crypto: invalid argon2id hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("crypto: parse argon2 version: %w", err)
	}
	var memory, timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false, fmt.Errorf("crypto: parse argon2 params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("crypto: decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("crypto: decode hash: %w", err)
	}
	if len(want) == 0 {
		return false, errors.New("crypto: empty hash")
	}

	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, nil
	}
	return true, nil
}

// HashToken 对令牌做 SHA-256，返回 64 位 hex（# 不可逆）。
// 用于 Agent 令牌、订阅令牌的落库哈希。
func HashToken(token string) string {
	return SHA256Hex(token)
}

// SHA256Hex 返回字符串的 SHA-256 十六进制摘要（64 位 hex）。
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// RandomBytes 生成 n 字节安全随机数。
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("crypto: read random: %w", err)
	}
	return b, nil
}

// RandomHex 生成 n 字节安全随机数并编码为 2n 位 hex。
func RandomHex(n int) (string, error) {
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// RandomToken 生成 URL 安全随机令牌（base64url，无填充）。
func RandomToken(n int) (string, error) {
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

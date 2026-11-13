// Package secret 负责 Agent 侧敏感信息的落盘加密存储（令牌、私钥等）。
//
// 与控制面不同：Agent 只做「加密」不做「哈希」，令牌/私钥需要可逆回读以便
// 连接控制面或应用配置。密钥由本地密钥文件提供，权限 0600。
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store 是本地加密存储，文件格式为 base64(nonce || ciphertext)。
type Store struct {
	dir string
	key [32]byte
}

// LoadOrCreateKey 读取或生成 32 字节本地主密钥文件（权限 0600）。
// 密钥文件内容为 base64 编码的 32 字节密钥。
func LoadOrCreateKey(path string) ([]byte, error) {
	if raw, err := os.ReadFile(path); err == nil {
		key, err := decodeKey(raw)
		if err != nil {
			return nil, fmt.Errorf("secret: load key: %w", err)
		}
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("secret: read key file: %w", err)
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("secret: generate key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("secret: mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, fmt.Errorf("secret: write key file: %w", err)
	}
	return key, nil
}

func decodeKey(raw []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("secret: decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("secret: key must be 32 bytes")
	}
	return key, nil
}

// NewStore 构造存储实例；key 可为任意长度的密钥材料，内部经 SHA-256 派生为 32 字节。
func NewStore(dir string, key []byte) (*Store, error) {
	if len(key) == 0 {
		return nil, errors.New("secret: empty key")
	}
	return &Store{dir: dir, key: sha256.Sum256(key)}, nil
}

// Set 加密写入名为 name 的敏感值。
func (s *Store) Set(name string, value []byte) error {
	if err := validateName(name); err != nil {
		return err
	}
	gcm, err := s.newGCM()
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("secret: read nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, value, nil)

	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("secret: mkdir: %w", err)
	}
	path := filepath.Join(s.dir, name+".enc")
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(sealed)), 0o600); err != nil {
		return fmt.Errorf("secret: write: %w", err)
	}
	return nil
}

// Get 解密读取名为 name 的敏感值。
func (s *Store) Get(name string) ([]byte, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, name+".enc"))
	if err != nil {
		return nil, fmt.Errorf("secret: read: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("secret: decode: %w", err)
	}
	gcm, err := s.newGCM()
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("secret: corrupt ciphertext")
	}
	nonce, sealed := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("secret: decrypt: %w", err)
	}
	return plain, nil
}

// GetString 是 Get 的字符串便捷方法。
func (s *Store) GetString(name string) (string, error) {
	b, err := s.Get(name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Delete 删除名为 name 的敏感值。
func (s *Store) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.dir, name+".enc")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("secret: delete: %w", err)
	}
	return nil
}

func (s *Store) newGCM() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		return nil, fmt.Errorf("secret: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new gcm: %w", err)
	}
	return gcm, nil
}

// validateName 防止路径穿越。
func validateName(name string) error {
	if name == "" {
		return errors.New("secret: empty name")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return errors.New("secret: invalid name")
	}
	return nil
}

package subscribe

import (
	"github.com/singbox-panel/control-plane/internal/crypto"
)

// tokenLength 订阅令牌随机字节数。
const tokenLength = 32

// GenerateToken 生成订阅令牌明文（仅创建/重置时返回一次）。
func GenerateToken() (string, error) {
	return crypto.RandomToken(tokenLength)
}

// HashToken 计算订阅令牌哈希（落库用）。
func HashToken(token string) string {
	return crypto.HashToken(token)
}

package crypto

import (
	"encoding/json"
	"strings"
)

// fieldEncPrefix 是字段级加密的标记前缀，保证「加密幂等 + 解密可识别」。
const fieldEncPrefix = "enc:v1:"

// IsEncryptedField 判断字符串是否为字段级加密后的密文。
func IsEncryptedField(v string) bool { return strings.HasPrefix(v, fieldEncPrefix) }

// EncryptField 对单个敏感字段值做 AES-256-GCM 加密并加前缀标记。
// 幂等：已加密（带前缀）的值原样返回，避免重复加密。
func (c *Codec) EncryptField(plain string) (string, error) {
	if IsEncryptedField(plain) {
		return plain, nil
	}
	enc, err := c.EncryptString(plain)
	if err != nil {
		return "", err
	}
	return fieldEncPrefix + enc, nil
}

// DecryptField 解密带前缀标记的字段值；无标记的原样返回（视为非密文）。
func (c *Codec) DecryptField(v string) (string, error) {
	if !IsEncryptedField(v) {
		return v, nil
	}
	return c.DecryptString(strings.TrimPrefix(v, fieldEncPrefix))
}

// EncryptJSONSecrets 遍历 JSONB，对敏感键（private_key）的字符串值加密。
// 用于 inbound/outbound 的 tls 等 JSONB 字段落库前加密。
func (c *Codec) EncryptJSONSecrets(raw json.RawMessage) (json.RawMessage, error) {
	return c.transformJSONSecrets(raw, true)
}

// DecryptJSONSecrets 遍历 JSONB，对敏感键（private_key）的密文解密。
// 用于生成/下发/订阅渲染前解密注入。
func (c *Codec) DecryptJSONSecrets(raw json.RawMessage) (json.RawMessage, error) {
	return c.transformJSONSecrets(raw, false)
}

func (c *Codec) transformJSONSecrets(raw json.RawMessage, encrypt bool) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return raw, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	if err := c.transformSecrets(v, encrypt); err != nil {
		return nil, err
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// transformSecrets 递归遍历 JSON 值，处理 map 中键名为 private_key 的字符串值。
func (c *Codec) transformSecrets(v any, encrypt bool) error {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == "private_key" {
				if s, ok := val.(string); ok {
					var err error
					if encrypt {
						val, err = c.EncryptField(s)
					} else {
						val, err = c.DecryptField(s)
					}
					if err != nil {
						return err
					}
					t[k] = val
				}
				continue
			}
			if err := c.transformSecrets(val, encrypt); err != nil {
				return err
			}
		}
	case []any:
		for i := range t {
			if err := c.transformSecrets(t[i], encrypt); err != nil {
				return err
			}
		}
	}
	return nil
}

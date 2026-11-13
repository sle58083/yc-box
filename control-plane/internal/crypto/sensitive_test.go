package crypto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncryptDecryptJSONSecrets(t *testing.T) {
	km, err := NewStaticKeyManager(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewStaticKeyManager: %v", err)
	}
	c := New(km)
	tls := json.RawMessage(`{"enabled":true,"server_name":"example.com","reality":{"enabled":true,"private_key":"PRIVATE_KEY_SECRET","short_id":["abcd"]}}`)

	enc, err := c.EncryptJSONSecrets(tls)
	if err != nil {
		t.Fatalf("EncryptJSONSecrets: %v", err)
	}
	if strings.Contains(string(enc), "PRIVATE_KEY_SECRET") {
		t.Fatalf("明文私钥不应出现在密文中: %s", enc)
	}
	if !strings.Contains(string(enc), "example.com") {
		t.Fatalf("非敏感字段 server_name 不应被加密")
	}

	dec, err := c.DecryptJSONSecrets(enc)
	if err != nil {
		t.Fatalf("DecryptJSONSecrets: %v", err)
	}
	if !strings.Contains(string(dec), "PRIVATE_KEY_SECRET") {
		t.Fatalf("解密后应还原明文私钥: %s", dec)
	}
}

func TestEncryptFieldIdempotent(t *testing.T) {
	km, _ := NewStaticKeyManager(make([]byte, 32))
	c := New(km)
	enc1, err := c.EncryptField("secret")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}
	if !IsEncryptedField(enc1) {
		t.Fatalf("加密值应带前缀标记")
	}
	enc2, err := c.EncryptField(enc1)
	if err != nil {
		t.Fatalf("重复 EncryptField: %v", err)
	}
	if enc1 != enc2 {
		t.Fatalf("重复加密应幂等")
	}
	dec, err := c.DecryptField(enc2)
	if err != nil {
		t.Fatalf("DecryptField: %v", err)
	}
	if dec != "secret" {
		t.Fatalf("解密失败: %q", dec)
	}
}

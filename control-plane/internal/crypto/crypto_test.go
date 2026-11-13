package crypto

import (
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	km, err := NewStaticKeyManager(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewStaticKeyManager: %v", err)
	}
	c := New(km)
	plain := "c1d5f0a0-0000-4000-8000-000000000001"
	enc, err := c.EncryptString(plain)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	if enc == plain {
		t.Fatalf("密文不应等于明文")
	}
	got, err := c.DecryptString(enc)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if got != plain {
		t.Fatalf("解密不一致: %q != %q", got, plain)
	}
}

func TestHashPasswordVerify(t *testing.T) {
	hash, err := HashPassword("secret-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if ok, err := VerifyPassword("secret-password", hash); err != nil || !ok {
		t.Fatalf("应校验通过: %v %v", ok, err)
	}
	if ok, _ := VerifyPassword("wrong", hash); ok {
		t.Fatalf("错误密码不应通过")
	}
}

func TestHashToken(t *testing.T) {
	h := HashToken("abc")
	if len(h) != 64 {
		t.Fatalf("SHA-256 应为 64 hex: %s", h)
	}
	if HashToken("abc") != HashToken("abc") {
		t.Fatalf("哈希应确定")
	}
}

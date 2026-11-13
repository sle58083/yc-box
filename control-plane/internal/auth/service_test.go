package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/config"
)

func TestIssueParseRoundTrip(t *testing.T) {
	svc := NewService(nil, config.JWTConfig{
		Secret:     "test-secret-32bytes-long-enough",
		Issuer:     "test",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	uid := uuid.New()
	pair, err := svc.issue(uid, "alice", "admin")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := svc.ParseToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != uid.String() || claims.Username != "alice" || claims.Role != "admin" {
		t.Fatalf("claims 不一致: %+v", claims)
	}
}

func TestParseInvalidToken(t *testing.T) {
	svc := NewService(nil, config.JWTConfig{Secret: "test-secret", Issuer: "test"})
	if _, err := svc.ParseToken("garbage.token.value"); err == nil {
		t.Fatalf("非法令牌应报错")
	}
}

func TestParseWrongSignature(t *testing.T) {
	svc := NewService(nil, config.JWTConfig{Secret: "secret-a", Issuer: "test"})
	other := NewService(nil, config.JWTConfig{Secret: "secret-b", Issuer: "test"})
	pair, _ := other.issue(uuid.New(), "bob", "viewer")
	if _, err := svc.ParseToken(pair.AccessToken); err == nil {
		t.Fatalf("错误签名应被拒绝")
	}
}

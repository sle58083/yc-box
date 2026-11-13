package configsvc

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/configsvc/renderer"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
)

// TestDecryptProxyUserPasswordInjection 覆盖 Bug-1：密码类协议配置生成的密码解密注入。
func TestDecryptProxyUserPasswordInjection(t *testing.T) {
	km, err := crypto.NewStaticKeyManager(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewStaticKeyManager: %v", err)
	}
	codec := crypto.New(km)

	plainUUID := "c1d5f0a0-0000-4000-8000-000000000001"
	plainPassword := "secret-password"
	encUUID, _ := codec.EncryptString(plainUUID)
	encPwd, _ := codec.EncryptString(plainPassword)

	u := model.ProxyUser{
		Base:        model.Base{ID: uuid.New()},
		UUID:        encUUID,
		PasswordEnc: encPwd,
	}
	pu := decryptProxyUser(u, codec)
	if pu.UUID != plainUUID {
		t.Fatalf("UUID 解密注入失败: %+v", pu)
	}
	if pu.Password != plainPassword {
		t.Fatalf("密码解密注入失败: %+v", pu)
	}

	// 集成：anytls is_user_inbound 渲染时 users[].password 应为 base64(明文密码)。
	ctx := &NodeConfigContext{
		Global: DefaultGlobalSettings(),
		Inbounds: []model.InboundConfig{{
			Type:          "anytls",
			Name:          "in-anytls",
			Listen:        "0.0.0.0",
			Port:          443,
			IsUserInbound: true,
			TLS:           json.RawMessage(`{"enabled":true,"certificate_path":"/c","key_path":"/k"}`),
			Status:        model.StatusEnabled,
		}},
		ProxyUsers: []renderer.ProxyUser{pu},
	}
	doc, err := NewGenerator().Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	inbounds := doc["inbounds"].([]map[string]any)
	users := inbounds[0]["users"].([]map[string]any)
	want := base64.StdEncoding.EncodeToString([]byte(plainPassword))
	if users[0]["password"] != want {
		t.Fatalf("密码类协议密码未解密注入: got %v want %v", users[0]["password"], want)
	}
}

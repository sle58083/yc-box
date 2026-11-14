package subscribe

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/singbox-panel/control-plane/internal/model"
)

func TestBuildClientOutbound(t *testing.T) {
	ib := model.InboundConfig{
		Name:      "in-vless",
		Type:      "vless",
		Port:      443,
		Settings:  json.RawMessage(`{"flow":"xtls-rprx-vision"}`),
		TLS:       json.RawMessage(`{"enabled":true,"server_name":"example.com"}`),
		Transport: json.RawMessage(`{"type":"ws","path":"/ws"}`),
	}
	n := model.Node{Name: "hk", Address: "hk.example.com"}

	ob, err := buildClientOutbound(ib, n, "uuid-1", "pw-1", false)
	if err != nil {
		t.Fatalf("buildClientOutbound: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(ob.Settings, &settings); err != nil {
		t.Fatalf("settings 解析: %v", err)
	}
	if settings["server"] != "hk.example.com" {
		t.Fatalf("server 注入错误: %v", settings["server"])
	}
	if settings["uuid"] != "uuid-1" || settings["password"] != "pw-1" {
		t.Fatalf("UUID/密码解密注入错误: %v %v", settings["uuid"], settings["password"])
	}
	if settings["flow"] != "xtls-rprx-vision" {
		t.Fatalf("flow 应保留: %v", settings["flow"])
	}
	if ob.TLS == nil || string(ob.TLS) == "" {
		t.Fatalf("客户端 TLS 应生成")
	}
	if ob.Transport == nil || string(ob.Transport) == "" {
		t.Fatalf("transport 应透传")
	}
}

func TestFirstN(t *testing.T) {
	if firstN("abcdef", 4) != "abcd" {
		t.Fatalf("firstN 错误")
	}
	if firstN("ab", 4) != "ab" {
		t.Fatalf("firstN 短串错误")
	}
}

// TestDeriveRealityPublicKey 用 RFC 7748 §6.1 测试向量（X25519(a, 9)）验证
// X25519 公钥推导与 base64 编解码正确性（非循环断言）。
func TestDeriveRealityPublicKey(t *testing.T) {
	privBytes, _ := hex.DecodeString("77076d0a7318a57d3c16c17251b26645df4c2f87ebc0992ab177fba51db92c2a")
	wantPub, _ := hex.DecodeString("8520f0098930a754748b7ddcb43ef75a0dbf3a0d26381af4eba4a98eaa9b4e6a")

	pubB64, err := deriveRealityPublicKey(base64.StdEncoding.EncodeToString(privBytes))
	if err != nil {
		t.Fatalf("deriveRealityPublicKey: %v", err)
	}
	gotPub, err := base64.RawURLEncoding.DecodeString(pubB64)
	if err != nil {
		t.Fatalf("public_key 应为 base64url: %v", err)
	}
	if !bytes.Equal(gotPub, wantPub) {
		t.Fatalf("公钥推导错误:\n got  %x\n want %x", gotPub, wantPub)
	}
}

// TestBuildClientOutboundReality 走真实链路：Reality 入站 → buildClientOutbound →
// proxyFromOutbound，验证 reality 未被丢弃，且能映射为 reality-opts。
func TestBuildClientOutboundReality(t *testing.T) {
	priv := make([]byte, 32)
	for i := range priv {
		priv[i] = byte(i + 1)
	}
	tlsRaw, _ := json.Marshal(map[string]any{
		"enabled":     true,
		"server_name": "www.microsoft.com",
		"reality": map[string]any{
			"enabled":     true,
			"private_key": base64.StdEncoding.EncodeToString(priv),
			"short_id":    "abcd1234",
		},
	})

	ib := model.InboundConfig{
		Name: "in-vless-reality",
		Type: "vless",
		Port: 443,
		TLS:  tlsRaw,
	}
	n := model.Node{Name: "hk", Address: "1.2.3.4"}

	ob, err := buildClientOutbound(ib, n, "uuid-1", "", false)
	if err != nil {
		t.Fatalf("buildClientOutbound: %v", err)
	}

	var clientTLS map[string]any
	if err := json.Unmarshal(ob.TLS, &clientTLS); err != nil {
		t.Fatalf("客户端 TLS 解析: %v", err)
	}
	if clientTLS["server_name"] != "www.microsoft.com" {
		t.Fatalf("reality SNI 应取握手域名: %#v", clientTLS["server_name"])
	}
	reality, ok := clientTLS["reality"].(map[string]any)
	if !ok {
		t.Fatalf("客户端 TLS 应保留 reality: %#v", clientTLS)
	}
	pub, _ := reality["public_key"].(string)
	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil || len(pubBytes) != 32 {
		t.Fatalf("public_key 应为 32 字节 base64url: %q err=%v", pub, err)
	}
	if reality["short_id"] != "abcd1234" {
		t.Fatalf("short_id 透传错误: %#v", reality["short_id"])
	}

	p := proxyFromOutbound(ob, "in-vless-reality - hk")
	if p == nil {
		t.Fatalf("proxyFromOutbound 返回 nil")
	}
	if p["servername"] != "www.microsoft.com" {
		t.Fatalf("clash servername 映射错误: %#v", p["servername"])
	}
	ro, ok := p["reality-opts"].(map[string]any)
	if !ok {
		t.Fatalf("clash reality-opts 未映射: %#v", p)
	}
	if ro["public-key"] != pub || ro["short-id"] != "abcd1234" {
		t.Fatalf("clash reality-opts 映射错误: %#v", ro)
	}
}

// TestBuildClientOutboundSelfSigned 走真实链路：自签证书入站 → buildClientOutbound
// （selfSigned=true）→ proxyFromOutbound，验证 insecure → skip-cert-verify。
func TestBuildClientOutboundSelfSigned(t *testing.T) {
	ib := model.InboundConfig{
		Name: "in-trojan",
		Type: "trojan",
		Port: 443,
		TLS:  json.RawMessage(`{"enabled":true,"server_name":"example.com"}`),
	}
	n := model.Node{Name: "hk", Address: "example.com"}

	ob, err := buildClientOutbound(ib, n, "", "pw", true)
	if err != nil {
		t.Fatalf("buildClientOutbound: %v", err)
	}

	var clientTLS map[string]any
	if err := json.Unmarshal(ob.TLS, &clientTLS); err != nil {
		t.Fatalf("客户端 TLS 解析: %v", err)
	}
	if insecure, _ := clientTLS["insecure"].(bool); !insecure {
		t.Fatalf("自签证书客户端 TLS 应带 insecure=true: %#v", clientTLS)
	}

	p := proxyFromOutbound(ob, "in-trojan - hk")
	if p == nil {
		t.Fatalf("proxyFromOutbound 返回 nil")
	}
	if p["type"] != "trojan" {
		t.Fatalf("trojan 映射错误: %#v", p)
	}
	if skip, _ := p["skip-cert-verify"].(bool); !skip {
		t.Fatalf("自签证书应映射 skip-cert-verify=true: %#v", p)
	}
}

// TestBuildClientOutboundPlainTLS 普通 CA 证书行为不变：无 insecure、无 reality。
func TestBuildClientOutboundPlainTLS(t *testing.T) {
	ib := model.InboundConfig{
		Name: "in-vless",
		Type: "vless",
		Port: 443,
		TLS:  json.RawMessage(`{"enabled":true,"server_name":"example.com"}`),
	}
	n := model.Node{Name: "hk", Address: "hk.example.com"}

	ob, err := buildClientOutbound(ib, n, "uuid-1", "", false)
	if err != nil {
		t.Fatalf("buildClientOutbound: %v", err)
	}
	var clientTLS map[string]any
	if err := json.Unmarshal(ob.TLS, &clientTLS); err != nil {
		t.Fatalf("客户端 TLS 解析: %v", err)
	}
	if _, hasInsecure := clientTLS["insecure"]; hasInsecure {
		t.Fatalf("普通 CA 证书不应有 insecure: %#v", clientTLS)
	}
	if _, hasReality := clientTLS["reality"]; hasReality {
		t.Fatalf("普通 CA 证书不应有 reality: %#v", clientTLS)
	}
	if clientTLS["server_name"] != "example.com" {
		t.Fatalf("server_name 透传错误: %#v", clientTLS["server_name"])
	}
}

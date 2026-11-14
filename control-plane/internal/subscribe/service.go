// Package subscribe 负责订阅链接与订阅内容生成（公开端点，sub_token 鉴权）。
package subscribe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"

	"github.com/singbox-panel/control-plane/internal/configsvc/renderer"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
	"github.com/singbox-panel/control-plane/internal/user"
)

// certTypeSelfSigned 是证书类型「一键自签」的取值（model.Certificate.Type）。
// 客户端订阅到自签证书节点时需跳过证书校验（insecure）。
const certTypeSelfSigned = "self_signed"

// Service 订阅服务。
type Service struct {
	db      *store.DB
	userSvc *user.Service
	codec   *crypto.Codec
}

// NewService 构造订阅服务。
func NewService(db *store.DB, userSvc *user.Service, codec *crypto.Codec) *Service {
	return &Service{db: db, userSvc: userSvc, codec: codec}
}

// clientOutbound 保存一条客户端出站及其命名上下文（入站/节点）。
type clientOutbound struct {
	inbound  model.InboundConfig
	node     model.Node
	outbound model.OutboundConfig
}

// Content 生成 sing-box 原生订阅内容（每行一个协议 URI）。
func (s *Service) Content(ctx context.Context, subToken string) ([]byte, error) {
	uris, err := s.uris(ctx, subToken)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(uris, "\n")), nil
}

// ContentClash 生成 Clash 完整 YAML 订阅（proxies + proxy-groups + rules）。
func (s *Service) ContentClash(ctx context.Context, subToken string) ([]byte, error) {
	return s.buildClashYAML(ctx, subToken)
}

// uris 拉取用户可用的全部订阅 URI（UUID/密码解密注入）。
func (s *Service) uris(ctx context.Context, subToken string) ([]string, error) {
	pu, outbounds, err := s.resolveSubscription(ctx, subToken)
	if err != nil {
		return nil, err
	}

	var uris []string
	for _, co := range outbounds {
		r, ok := renderer.Get(co.inbound.Type)
		if !ok {
			continue
		}
		uri, err := r.SubscriptionURI(co.outbound, pu)
		if err != nil {
			if err == renderer.ErrUnsupported {
				continue
			}
			return nil, err
		}
		uris = append(uris, uri)
	}
	if len(uris) == 0 {
		return nil, fmt.Errorf("subscribe: 无可用订阅节点/入站")
	}
	return uris, nil
}

// resolveSubscription 鉴权并加载订阅上下文：解密用户凭证 + 拉取全部客户端出站。
//
// 该函数是 Content（sing-box URI）与 ContentClash（YAML）的公共入口，保证两者
// 鉴权、解密与节点/入站枚举逻辑一致。
func (s *Service) resolveSubscription(ctx context.Context, subToken string) (renderer.ProxyUser, []clientOutbound, error) {
	u, err := s.userSvc.ResolveBySubToken(ctx, subToken)
	if err != nil {
		return renderer.ProxyUser{}, nil, fmt.Errorf("subscribe: 订阅令牌无效")
	}
	if u.Status != model.StatusEnabled {
		return renderer.ProxyUser{}, nil, fmt.Errorf("subscribe: 用户已禁用")
	}
	if u.ExpireAt != nil && u.ExpireAt.Before(time.Now().UTC()) {
		return renderer.ProxyUser{}, nil, fmt.Errorf("subscribe: 用户已过期")
	}
	if u.QuotaBytes >= 0 && u.UsedBytes >= u.QuotaBytes {
		return renderer.ProxyUser{}, nil, fmt.Errorf("subscribe: 用户流量已耗尽")
	}

	plainUUID, plainPassword, err := s.userSvc.Credentials(u)
	if err != nil {
		return renderer.ProxyUser{}, nil, err
	}
	pu := renderer.ProxyUser{
		ID:       u.ID.String(),
		Name:     u.ID.String(), // Name 仅作 sing-box 用户标识（DB UUID），不进订阅 URI fragment
		UUID:     plainUUID,
		Password: plainPassword,
	}

	// 分组模板的 is_user_inbound 入站。
	var inbounds []model.InboundConfig
	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND node_id IS NULL AND is_user_inbound = ? AND status = ?", u.GroupID, true, model.StatusEnabled).
		Find(&inbounds).Error; err != nil {
		return renderer.ProxyUser{}, nil, err
	}
	// 订阅渲染前解密 TLS 敏感键（private_key 等）；同时判断是否绑定自签证书
	// （自签证书客户端需跳过校验，见 buildClientTLS）。
	selfSigned := make([]bool, len(inbounds))
	for i := range inbounds {
		if dec, err := s.codec.DecryptJSONSecrets(inbounds[i].TLS); err == nil {
			inbounds[i].TLS = dec
		}
		if inbounds[i].CertID != nil {
			var cert model.Certificate
			if err := s.db.WithContext(ctx).First(&cert, "id = ?", *inbounds[i].CertID).Error; err == nil {
				selfSigned[i] = cert.Type == certTypeSelfSigned
			}
		}
	}
	var nodes []model.Node
	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND address <> ''", u.GroupID).
		Find(&nodes).Error; err != nil {
		return renderer.ProxyUser{}, nil, err
	}

	outbounds := make([]clientOutbound, 0, len(nodes)*len(inbounds))
	for _, n := range nodes {
		for i, ib := range inbounds {
			outbound, err := buildClientOutbound(ib, n, plainUUID, plainPassword, selfSigned[i])
			if err != nil {
				return renderer.ProxyUser{}, nil, err
			}
			outbounds = append(outbounds, clientOutbound{inbound: ib, node: n, outbound: outbound})
		}
	}
	return pu, outbounds, nil
}

// buildClientOutbound 由「服务端入站」构造「客户端出站」，供渲染器生成订阅 URI。
//
// selfSigned 表示入站绑定的是自签证书：客户端出站 TLS 会注入 insecure=true，
// 使 Clash 侧映射为 skip-cert-verify。
func buildClientOutbound(ib model.InboundConfig, n model.Node, uuidStr, password string, selfSigned bool) (model.OutboundConfig, error) {
	settings := map[string]any{}
	if len(ib.Settings) > 0 {
		if err := json.Unmarshal(ib.Settings, &settings); err != nil {
			return model.OutboundConfig{}, err
		}
	}
	// 客户端出站关键字段。
	settings["server"] = n.Address
	settings["server_port"] = ib.Port
	if uuidStr != "" {
		settings["uuid"] = uuidStr
	}
	if password != "" {
		settings["password"] = password
	}

	tls, err := buildClientTLS(ib, n, selfSigned)
	if err != nil {
		return model.OutboundConfig{}, err
	}

	return model.OutboundConfig{
		Name:      ib.Name + "-" + n.Name,
		Type:      ib.Type,
		Settings:  mustJSON(settings),
		TLS:       tls,
		Transport: ib.Transport,
	}, nil
}

// buildClientTLS 由「服务端入站 TLS」构造「客户端出站 TLS」。
//
// 普通 CA 证书：仅 enabled + server_name（正常校验证书）；
// Reality：透传 reality（public_key 由 private_key 推导 + short_id）；
// 自签证书 / 显式 insecure：注入 insecure=true（Clash 侧映射 skip-cert-verify）。
func buildClientTLS(ib model.InboundConfig, n model.Node, selfSigned bool) (json.RawMessage, error) {
	serverName := sniOf(ib, n)
	// 解析失败视为无 TLS 信息（与旧行为一致：仅保留 server_name）。
	inboundTLS, _ := parseJSONObject(ib.TLS)

	out := map[string]any{
		"enabled":     true,
		"server_name": serverName,
	}

	if selfSigned || mapBool(inboundTLS, "insecure") {
		out["insecure"] = true
	}

	if r, ok := inboundTLS["reality"].(map[string]any); ok {
		// 显式 enabled=false 视为未启用（与前端 toggle 语义一致）。
		if enabled, has := r["enabled"].(bool); has && !enabled {
			return mustJSON(out), nil
		}
		cr, err := clientReality(r)
		if err != nil {
			return nil, err
		}
		out["reality"] = cr
	}

	return mustJSON(out), nil
}

// clientReality 把服务端 reality 对象转换为客户端 reality（public_key + short_id）。
func clientReality(r map[string]any) (map[string]any, error) {
	out := map[string]any{"enabled": true}

	if pk := mapString(r, "public_key"); pk != "" {
		out["public_key"] = pk
	} else if priv := mapString(r, "private_key"); priv != "" {
		pub, err := deriveRealityPublicKey(priv)
		if err != nil {
			return nil, fmt.Errorf("subscribe: 推导 Reality public_key 失败: %w", err)
		}
		out["public_key"] = pub
	} else {
		return nil, fmt.Errorf("subscribe: Reality 入站缺少 public_key/private_key")
	}

	if sid := shortIDOf(r); sid != "" {
		out["short_id"] = sid
	}
	return out, nil
}

// shortIDOf 归一化 reality short_id（兼容 string 与 []string/[]any）。
func shortIDOf(r map[string]any) string {
	switch v := r["short_id"].(type) {
	case string:
		return v
	case []string:
		if len(v) > 0 {
			return v[0]
		}
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// deriveRealityPublicKey 由 X25519 私钥（base64/base64url，带/不带 padding）推导公钥。
func deriveRealityPublicKey(priv string) (string, error) {
	key, err := decodeBase64Flex(priv)
	if err != nil {
		return "", err
	}
	pub, err := curve25519.X25519(key, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	// Mihomo reality-opts.public-key 约定为 base64url（无 padding）。
	return base64.RawURLEncoding.EncodeToString(pub), nil
}

// decodeBase64Flex 兼容标准/URL、带/不带 padding 的 base64，返回 32 字节密钥。
func decodeBase64Flex(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("Reality private_key 无法解码为 32 字节 X25519 密钥")
}

func sniOf(ib model.InboundConfig, n model.Node) string {
	m, _ := parseJSONObject(ib.TLS)
	if s := mapString(m, "server_name"); s != "" {
		return s
	}
	// Reality：SNI 取 reality 握手域名（reality.server_name / reality.handshake.server）。
	if r, ok := m["reality"].(map[string]any); ok {
		if s := mapString(r, "server_name"); s != "" {
			return s
		}
		if h, ok := r["handshake"].(map[string]any); ok {
			if s := mapString(h, "server"); s != "" {
				return s
			}
		}
	}
	return n.Address
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

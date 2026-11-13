// Package subscribe 负责订阅链接与订阅内容生成（公开端点，sub_token 鉴权）。
package subscribe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/singbox-panel/control-plane/internal/configsvc/renderer"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
	"github.com/singbox-panel/control-plane/internal/user"
)

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

// Content 生成 sing-box 原生订阅内容（每行一个协议 URI）。
func (s *Service) Content(ctx context.Context, subToken string) ([]byte, error) {
	uris, err := s.uris(ctx, subToken)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(uris, "\n")), nil
}

// ContentClash 生成 Clash 订阅（P1：base64(uri 列表)，完整 YAML 留待后续）。
func (s *Service) ContentClash(ctx context.Context, subToken string) ([]byte, error) {
	uris, err := s.uris(ctx, subToken)
	if err != nil {
		return nil, err
	}
	payload := base64.StdEncoding.EncodeToString([]byte(strings.Join(uris, "\n")))
	return []byte(payload), nil
}

// uris 拉取用户可用的全部订阅 URI（UUID/密码解密注入）。
func (s *Service) uris(ctx context.Context, subToken string) ([]string, error) {
	u, err := s.userSvc.ResolveBySubToken(ctx, subToken)
	if err != nil {
		return nil, fmt.Errorf("subscribe: 订阅令牌无效")
	}
	if u.Status != model.StatusEnabled {
		return nil, fmt.Errorf("subscribe: 用户已禁用")
	}
	if u.ExpireAt != nil && u.ExpireAt.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("subscribe: 用户已过期")
	}
	if u.QuotaBytes >= 0 && u.UsedBytes >= u.QuotaBytes {
		return nil, fmt.Errorf("subscribe: 用户流量已耗尽")
	}

	plainUUID, plainPassword, err := s.userSvc.Credentials(u)
	if err != nil {
		return nil, err
	}
	pu := renderer.ProxyUser{
		ID:       u.ID.String(),
		Name:     "user-" + firstN(plainUUID, 4),
		UUID:     plainUUID,
		Password: plainPassword,
	}

	// 分组模板的 is_user_inbound 入站。
	var inbounds []model.InboundConfig
	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND node_id IS NULL AND is_user_inbound = ? AND status = ?", u.GroupID, true, model.StatusEnabled).
		Find(&inbounds).Error; err != nil {
		return nil, err
	}
	// 订阅渲染前解密 TLS 敏感键（private_key 等）。
	for i := range inbounds {
		if dec, err := s.codec.DecryptJSONSecrets(inbounds[i].TLS); err == nil {
			inbounds[i].TLS = dec
		}
	}
	var nodes []model.Node
	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND address <> ''", u.GroupID).
		Find(&nodes).Error; err != nil {
		return nil, err
	}

	var uris []string
	for _, n := range nodes {
		for _, ib := range inbounds {
			r, ok := renderer.Get(ib.Type)
			if !ok {
				continue
			}
			outbound, err := buildClientOutbound(ib, n, plainUUID, plainPassword)
			if err != nil {
				return nil, err
			}
			uri, err := r.SubscriptionURI(outbound, pu)
			if err != nil {
				if err == renderer.ErrUnsupported {
					continue
				}
				return nil, err
			}
			uris = append(uris, uri)
		}
	}
	if len(uris) == 0 {
		return nil, fmt.Errorf("subscribe: 无可用订阅节点/入站")
	}
	return uris, nil
}

// buildClientOutbound 由「服务端入站」构造「客户端出站」，供渲染器生成订阅 URI。
func buildClientOutbound(ib model.InboundConfig, n model.Node, uuidStr, password string) (model.OutboundConfig, error) {
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

	// 服务端 TLS → 客户端 TLS（仅保留 SNI；Reality/ECH 订阅属 P1）。
	tls := json.RawMessage(`{"enabled":true,"server_name":"` + sniOf(ib, n) + `"}`)

	return model.OutboundConfig{
		Name:      ib.Name + "-" + n.Name,
		Type:      ib.Type,
		Settings:  mustJSON(settings),
		TLS:       tls,
		Transport: ib.Transport,
	}, nil
}

func sniOf(ib model.InboundConfig, n model.Node) string {
	var m map[string]any
	if len(ib.TLS) > 0 {
		_ = json.Unmarshal(ib.TLS, &m)
		if s, _ := m["server_name"].(string); s != "" {
			return s
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

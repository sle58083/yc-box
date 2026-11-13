// Package auth 负责面板用户认证与 JWT 签发/校验。
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/config"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
)

// Claims 是 JWT 自定义载荷。
type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// TokenPair 是访问令牌 + 刷新令牌。
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// Service 认证服务。
type Service struct {
	db      *store.DB
	secret  []byte
	issuer  string
	access  time.Duration
	refresh time.Duration
}

// NewService 构造认证服务。
func NewService(db *store.DB, cfg config.JWTConfig) *Service {
	return &Service{
		db:      db,
		secret:  []byte(cfg.Secret),
		issuer:  cfg.Issuer,
		access:  cfg.AccessTTL,
		refresh: cfg.RefreshTTL,
	}
}

// Login 校验用户名/密码（argon2id）并签发令牌对。
func (s *Service) Login(ctx context.Context, username, password string) (*TokenPair, *model.PanelUser, error) {
	var user model.PanelUser
	if err := s.db.WithContext(ctx).First(&user, "username = ?", username).Error; err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if user.Status != model.StatusEnabled {
		return nil, nil, ErrUserDisabled
	}
	ok, err := crypto.VerifyPassword(password, user.PasswordHash)
	if err != nil || !ok {
		return nil, nil, ErrInvalidCredentials
	}
	role, err := s.roleName(ctx, user.RoleID)
	if err != nil {
		return nil, nil, err
	}
	pair, err := s.issue(user.ID, user.Username, role)
	if err != nil {
		return nil, nil, err
	}
	return pair, &user, nil
}

// Refresh 用刷新令牌换新访问令牌。
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := s.parse(refreshToken)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	var user model.PanelUser
	if err := s.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		return nil, ErrTokenInvalid
	}
	if user.Status != model.StatusEnabled {
		return nil, ErrUserDisabled
	}
	role, err := s.roleName(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}
	return s.issue(user.ID, user.Username, role)
}

// ParseToken 解析并校验访问令牌，返回 claims。
func (s *Service) ParseToken(tokenStr string) (*Claims, error) {
	claims, err := s.parse(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.UserID == "" || claims.Role == "" {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func (s *Service) parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func (s *Service) issue(userID uuid.UUID, username, role string) (*TokenPair, error) {
	now := time.Now()
	accessClaims := &Claims{
		UserID:   userID.String(),
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.access)),
			ID:        uuid.NewString(),
		},
	}
	refreshClaims := &Claims{
		UserID:   userID.String(),
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refresh)),
			ID:        uuid.NewString(),
		},
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(s.secret)
	if err != nil {
		return nil, err
	}
	refresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(s.secret)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.access.Seconds()),
	}, nil
}

func (s *Service) roleName(ctx context.Context, roleID uuid.UUID) (string, error) {
	var role model.Role
	if err := s.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		return "", fmt.Errorf("auth: 角色不存在: %w", err)
	}
	return role.Name, nil
}

// 认证错误哨兵。
var (
	ErrInvalidCredentials = errors.New("auth: 用户名或密码错误")
	ErrUserDisabled       = errors.New("auth: 用户已禁用")
	ErrTokenInvalid       = errors.New("auth: 令牌无效或已过期")
)

// Package middleware 提供认证 / RBAC / 审计中间件。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/auth"
	"github.com/singbox-panel/control-plane/internal/model"
)

// 上下文键。
const (
	CtxUserID   = "auth.user_id"
	CtxUsername = "auth.username"
	CtxRole     = "auth.role"
)

// Auth 解析 Bearer JWT 并注入用户身份到 context（§9.2 / §6.1）。
func Auth(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.Err(model.CodeUnauthorized, "缺少访问令牌"))
			return
		}
		claims, err := svc.ParseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.Err(model.CodeUnauthorized, "令牌无效或已过期"))
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

// UserID 从 context 取用户 ID（未登录返回空串）。
func UserID(c *gin.Context) string {
	v, _ := c.Get(CtxUserID)
	s, _ := v.(string)
	return s
}

// Username 从 context 取用户名。
func Username(c *gin.Context) string {
	v, _ := c.Get(CtxUsername)
	s, _ := v.(string)
	return s
}

// Role 从 context 取角色。
func Role(c *gin.Context) string {
	v, _ := c.Get(CtxRole)
	s, _ := v.(string)
	return s
}

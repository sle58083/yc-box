package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/middleware"
	"github.com/singbox-panel/control-plane/internal/model"
)

// Login POST /api/v1/auth/login（公开）
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	pair, user, err := h.Auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		// 审计：登录失败（匿名）。
		_ = h.Audit.Record(c.Request.Context(), uuid.Nil, "auth.login.fail", "/auth/login", map[string]any{"username": req.Username}, c.ClientIP())
		c.JSON(http.StatusOK, model.Err(model.CodeInvalidCredentials, err.Error()))
		return
	}
	_ = h.Audit.Record(c.Request.Context(), user.ID, "auth.login", "/auth/login", map[string]any{"username": req.Username}, c.ClientIP())
	h.ok(c, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token_type":    pair.TokenType,
		"expires_in":    pair.ExpiresIn,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role_id":  user.RoleID,
		},
	})
}

// Refresh POST /api/v1/auth/refresh（公开）
func (h *Handler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	pair, err := h.Auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeTokenInvalid, err.Error()))
		return
	}
	h.ok(c, pair)
}

// Me GET /api/v1/auth/me（登录）
func (h *Handler) Me(c *gin.Context) {
	userID := middleware.UserID(c)
	if userID == "" {
		c.JSON(http.StatusOK, model.Err(model.CodeUnauthorized, "未登录"))
		return
	}
	id, err := uuid.Parse(userID)
	if err != nil {
		h.fail(c, err)
		return
	}
	u, err := h.User.GetPanelUser(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{
		"id":       u.ID,
		"username": u.Username,
		"role":     middleware.Role(c),
		"role_id":  u.RoleID,
		"status":   u.Status,
	})
}

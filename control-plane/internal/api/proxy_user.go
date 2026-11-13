package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListProxyUsers GET /api/v1/proxy-users?group_id=
func (h *Handler) ListProxyUsers(c *gin.Context) {
	var groupID *uuid.UUID
	if s := c.Query("group_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "group_id 非法"))
			return
		}
		groupID = &id
	}
	out, err := h.User.ListProxyUsers(c.Request.Context(), groupID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreateProxyUser POST /api/v1/proxy-users
func (h *Handler) CreateProxyUser(c *gin.Context) {
	var req struct {
		GroupID       uuid.UUID `json:"group_id"`
		Remark        string    `json:"remark"`
		QuotaBytes    int64     `json:"quota_bytes"`
		ExpireAt      *string   `json:"expire_at"` // RFC3339
		PlainPassword string    `json:"password"`  // 协议密码（可选）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	u := &model.ProxyUser{
		GroupID:    req.GroupID,
		Remark:     req.Remark,
		QuotaBytes: req.QuotaBytes,
	}
	if req.ExpireAt != nil {
		if t, err := parseTime(*req.ExpireAt); err == nil {
			u.ExpireAt = &t
		}
	}
	subToken, err := h.User.CreateProxyUser(c.Request.Context(), u, req.PlainPassword)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"user": u, "sub_token": subToken, "subscribe_url": "/api/v1/sub/" + subToken})
}

// UpdateProxyUser PUT /api/v1/proxy-users/:id
func (h *Handler) UpdateProxyUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		Remark        string  `json:"remark"`
		QuotaBytes    int64   `json:"quota_bytes"`
		ExpireAt      *string `json:"expire_at"`
		Status        string  `json:"status"`
		PlainPassword *string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	u := &model.ProxyUser{
		Remark:     req.Remark,
		QuotaBytes: req.QuotaBytes,
		Status:     req.Status,
	}
	u.ID = id
	if req.ExpireAt != nil {
		if t, err := parseTime(*req.ExpireAt); err == nil {
			u.ExpireAt = &t
		}
	}
	if err := h.User.UpdateProxyUser(c.Request.Context(), u, req.PlainPassword); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, u)
}

// DeleteProxyUser DELETE /api/v1/proxy-users/:id
func (h *Handler) DeleteProxyUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.User.DeleteProxyUser(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"id": id.String()})
}

// ResetSubToken POST /api/v1/proxy-users/:id/reset-sub
func (h *Handler) ResetSubToken(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	token, err := h.User.ResetSubToken(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"sub_token": token, "subscribe_url": "/api/v1/sub/" + token})
}

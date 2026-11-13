package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListPanelUsers GET /api/v1/panel-users（super_admin）
func (h *Handler) ListPanelUsers(c *gin.Context) {
	out, err := h.User.ListPanelUsers(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreatePanelUser POST /api/v1/panel-users（super_admin）
func (h *Handler) CreatePanelUser(c *gin.Context) {
	var req struct {
		Username string    `json:"username"`
		Password string    `json:"password"`
		RoleID   uuid.UUID `json:"role_id"`
		Status   string    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	u := &model.PanelUser{Username: req.Username, RoleID: req.RoleID, Status: req.Status}
	if err := h.User.CreatePanelUser(c.Request.Context(), u, req.Password); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, u)
}

// UpdatePanelUser PUT /api/v1/panel-users/:id（super_admin）
func (h *Handler) UpdatePanelUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		Username string    `json:"username"`
		RoleID   uuid.UUID `json:"role_id"`
		Status   string    `json:"status"`
		Password *string   `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	u := &model.PanelUser{Username: req.Username, RoleID: req.RoleID, Status: req.Status}
	u.ID = id
	if err := h.User.UpdatePanelUser(c.Request.Context(), u, req.Password); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, u)
}

// DeletePanelUser DELETE /api/v1/panel-users/:id（super_admin）
func (h *Handler) DeletePanelUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.User.DeletePanelUser(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"id": id.String()})
}

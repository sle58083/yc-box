package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListRoles GET /api/v1/roles（super_admin）
func (h *Handler) ListRoles(c *gin.Context) {
	out, err := h.User.ListRoles(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreateRole POST /api/v1/roles（super_admin，仅内置角色）
func (h *Handler) CreateRole(c *gin.Context) {
	var body model.Role
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	if err := h.User.CreateRole(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// UpdateRole PUT /api/v1/roles/:id（super_admin）
func (h *Handler) UpdateRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body model.Role
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	body.ID = id
	if err := h.User.UpdateRole(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// DeleteRole DELETE /api/v1/roles/:id（super_admin）
func (h *Handler) DeleteRole(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.User.DeleteRole(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"id": id.String()})
}

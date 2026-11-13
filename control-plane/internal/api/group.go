package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListGroups GET /api/v1/groups
func (h *Handler) ListGroups(c *gin.Context) {
	out, err := h.Group.List(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreateGroup POST /api/v1/groups
func (h *Handler) CreateGroup(c *gin.Context) {
	var body model.Group
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	if err := h.Group.Create(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

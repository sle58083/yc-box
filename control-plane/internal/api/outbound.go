package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListOutbounds GET /api/v1/outbounds?group_id=
func (h *Handler) ListOutbounds(c *gin.Context) {
	var groupID *uuid.UUID
	if s := c.Query("group_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "group_id 非法"))
			return
		}
		groupID = &id
	}
	out, err := h.Config.ListOutbounds(c.Request.Context(), groupID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreateOutbound POST /api/v1/outbounds
func (h *Handler) CreateOutbound(c *gin.Context) {
	var body model.OutboundConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	if err := h.Config.CreateOutbound(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// UpdateOutbound PUT /api/v1/outbounds/:id
func (h *Handler) UpdateOutbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body model.OutboundConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	body.ID = id
	if err := h.Config.UpdateOutbound(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// DeleteOutbound DELETE /api/v1/outbounds/:id
func (h *Handler) DeleteOutbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.Config.DeleteOutbound(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"id": id.String()})
}

// GetOutbound GET /api/v1/outbounds/:id
func (h *Handler) GetOutbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cfg, err := h.Config.GetOutbound(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, cfg)
}

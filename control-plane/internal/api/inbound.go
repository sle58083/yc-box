package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListInbounds GET /api/v1/inbounds?group_id=
func (h *Handler) ListInbounds(c *gin.Context) {
	var groupID *uuid.UUID
	if s := c.Query("group_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "group_id 非法"))
			return
		}
		groupID = &id
	}
	out, err := h.Config.ListInbounds(c.Request.Context(), groupID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreateInbound POST /api/v1/inbounds
func (h *Handler) CreateInbound(c *gin.Context) {
	var body model.InboundConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	if err := h.Config.CreateInbound(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// UpdateInbound PUT /api/v1/inbounds/:id
func (h *Handler) UpdateInbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body model.InboundConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	body.ID = id
	if err := h.Config.UpdateInbound(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// DeleteInbound DELETE /api/v1/inbounds/:id
func (h *Handler) DeleteInbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.Config.DeleteInbound(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"id": id.String()})
}

// GetInbound GET /api/v1/inbounds/:id
func (h *Handler) GetInbound(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cfg, err := h.Config.GetInbound(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, cfg)
}

// parseID 解析路径参数 :id 为 UUID。
func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "id 非法"))
		return uuid.Nil, false
	}
	return id, true
}

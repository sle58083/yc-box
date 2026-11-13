package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListNodes GET /api/v1/nodes?group_id=
func (h *Handler) ListNodes(c *gin.Context) {
	var groupID *uuid.UUID
	if s := c.Query("group_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "group_id 非法"))
			return
		}
		groupID = &id
	}
	out, err := h.Node.List(c.Request.Context(), groupID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreateNode POST /api/v1/nodes （返回明文令牌一次）
func (h *Handler) CreateNode(c *gin.Context) {
	var body model.Node
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	token, err := h.Node.Create(c.Request.Context(), &body)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"node": body, "agent_token": token})
}

// GetNode GET /api/v1/nodes/:id
func (h *Handler) GetNode(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	n, err := h.Node.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, n)
}

// UpdateNode PUT /api/v1/nodes/:id
func (h *Handler) UpdateNode(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body model.Node
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	body.ID = id
	if err := h.Node.Update(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// DeleteNode DELETE /api/v1/nodes/:id
func (h *Handler) DeleteNode(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.Node.Delete(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"id": id.String()})
}

// RegenerateNodeToken POST /api/v1/nodes/:id/regenerate-token （返回明文令牌一次）
func (h *Handler) RegenerateNodeToken(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	token, err := h.Node.RegenerateToken(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"agent_token": token})
}

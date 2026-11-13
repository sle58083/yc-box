package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListRoutingRules GET /api/v1/routing?group_id=
func (h *Handler) ListRoutingRules(c *gin.Context) {
	var groupID *uuid.UUID
	if s := c.Query("group_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "group_id 非法"))
			return
		}
		groupID = &id
	}
	out, err := h.Config.ListRoutingRules(c.Request.Context(), groupID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreateRoutingRule POST /api/v1/routing
func (h *Handler) CreateRoutingRule(c *gin.Context) {
	var body model.RoutingRule
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	if err := h.Config.CreateRoutingRule(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// UpdateRoutingRule PUT /api/v1/routing/:id
func (h *Handler) UpdateRoutingRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body model.RoutingRule
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}
	body.ID = id
	if err := h.Config.UpdateRoutingRule(c.Request.Context(), &body); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, body)
}

// DeleteRoutingRule DELETE /api/v1/routing/:id
func (h *Handler) DeleteRoutingRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.Config.DeleteRoutingRule(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"id": id.String()})
}

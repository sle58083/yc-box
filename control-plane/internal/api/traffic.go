package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// NodeTraffic GET /api/v1/traffic/nodes?node_id=&from=&to=
func (h *Handler) NodeTraffic(c *gin.Context) {
	nodeID, from, to, ok := parseTrafficQuery(c)
	if !ok {
		return
	}
	out, err := h.Traffic.NodeTraffic(c.Request.Context(), nodeID, from, to)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// UserTraffic GET /api/v1/traffic/users?proxy_user_id=&from=&to=
func (h *Handler) UserTraffic(c *gin.Context) {
	var proxyUserID *uuid.UUID
	if s := c.Query("proxy_user_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "proxy_user_id 非法"))
			return
		}
		proxyUserID = &id
	}
	var from, to *time.Time
	if s := c.Query("from"); s != "" {
		if t, err := parseTime(s); err == nil {
			from = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := parseTime(s); err == nil {
			to = &t
		}
	}
	out, err := h.Traffic.UserTraffic(c.Request.Context(), proxyUserID, from, to)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

func parseTrafficQuery(c *gin.Context) (*uuid.UUID, *time.Time, *time.Time, bool) {
	var nodeID *uuid.UUID
	if s := c.Query("node_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "node_id 非法"))
			return nil, nil, nil, false
		}
		nodeID = &id
	}
	var from, to *time.Time
	if s := c.Query("from"); s != "" {
		if t, err := parseTime(s); err == nil {
			from = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := parseTime(s); err == nil {
			to = &t
		}
	}
	return nodeID, from, to, true
}

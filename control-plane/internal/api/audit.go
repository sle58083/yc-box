package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// ListAuditLogs GET /api/v1/audit-logs?action=&operator_id=&from=&to=&limit=&offset=
func (h *Handler) ListAuditLogs(c *gin.Context) {
	action := c.Query("action")
	var operatorID *uuid.UUID
	if s := c.Query("operator_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "operator_id 非法"))
			return
		}
		operatorID = &id
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
	limit, offset := 100, 0
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	if s := c.Query("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			offset = n
		}
	}
	out, err := h.Audit.List(c.Request.Context(), action, operatorID, from, to, limit, offset)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

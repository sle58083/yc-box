package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// DeployConfig POST /api/v1/config/deploy  {node_id}
func (h *Handler) DeployConfig(c *gin.Context) {
	nodeID, ok := parseNodeID(c)
	if !ok {
		return
	}
	v, err := h.Deploy.Deploy(c.Request.Context(), nodeID, uuid.Nil)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, v)
}

// RetryConfig POST /api/v1/config/retry  {node_id, version_id}
func (h *Handler) RetryConfig(c *gin.Context) {
	var req struct {
		NodeID    uuid.UUID `json:"node_id"`
		VersionID uuid.UUID `json:"version_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NodeID == uuid.Nil || req.VersionID == uuid.Nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "node_id/version_id 必填且合法"))
		return
	}
	v, err := h.Deploy.Retry(c.Request.Context(), req.NodeID, req.VersionID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, v)
}

// RollbackConfig POST /api/v1/config/rollback  {node_id, version_id}
// 全量回滚：复制历史快照为新版本并下发到节点。
func (h *Handler) RollbackConfig(c *gin.Context) {
	var req struct {
		NodeID    uuid.UUID `json:"node_id"`
		VersionID uuid.UUID `json:"version_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NodeID == uuid.Nil || req.VersionID == uuid.Nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "node_id/version_id 必填且合法"))
		return
	}
	v, err := h.Deploy.Rollback(c.Request.Context(), req.NodeID, req.VersionID, uuid.Nil)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, v)
}

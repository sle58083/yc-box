package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/model"
)

// PreviewConfig POST /api/v1/config/preview  {node_id}
func (h *Handler) PreviewConfig(c *gin.Context) {
	nodeID, ok := parseNodeID(c)
	if !ok {
		return
	}
	doc, err := h.Config.Preview(c.Request.Context(), nodeID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, doc)
}

// ListVersions GET /api/v1/config/versions?node_id=
func (h *Handler) ListVersions(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Query("node_id"))
	if err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "node_id 非法"))
		return
	}
	versions, err := h.Config.ListVersions(c.Request.Context(), nodeID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, versions)
}

// GetVersionContent GET /api/v1/config/versions/:id （返回解密后的配置内容）
func (h *Handler) GetVersionContent(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	v, err := h.Config.GetVersion(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	content, err := h.Config.DecryptVersionContent(v)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"version": v.Version, "hash": v.Hash, "content": string(content)})
}

// RollbackConfig 已迁移至 deploy.go（全量回滚需经下发编排，见 api/deploy.go）。

// parseNodeID 解析 body 中的 node_id。
func parseNodeID(c *gin.Context) (uuid.UUID, bool) {
	var req struct {
		NodeID uuid.UUID `json:"node_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NodeID == uuid.Nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "node_id 必填且合法"))
		return uuid.Nil, false
	}
	return req.NodeID, true
}

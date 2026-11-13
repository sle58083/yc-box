package api

import (
	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/configsvc/renderer"
)

// ListProtocols GET /api/v1/protocols
// 返回全部已注册协议的元数据（字段 schema、默认值、TLS 约束等），
// 供前端动态渲染表单，消除前端本地枚举与后端渲染器的漂移。
func (h *Handler) ListProtocols(c *gin.Context) {
	h.ok(c, renderer.Metas())
}

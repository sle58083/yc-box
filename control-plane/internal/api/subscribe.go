package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/model"
)

// SubscribeContent GET /api/v1/sub/:token （公开，sub_token 鉴权）
func (h *Handler) SubscribeContent(c *gin.Context) {
	token := c.Param("token")
	content, err := h.Subscribe.Content(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeNotFound, err.Error()))
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

// SubscribeContentClash GET /api/v1/sub/:token/clash （公开）
func (h *Handler) SubscribeContentClash(c *gin.Context) {
	token := c.Param("token")
	content, err := h.Subscribe.ContentClash(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeNotFound, err.Error()))
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
}

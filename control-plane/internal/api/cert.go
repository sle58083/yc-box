package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/model"
)

// certRequest 是 POST /api/v1/certificates 的请求体。
// 按 type 分派：
//   - self_signed：需 name + domains，valid_days 可选（默认 365）；
//   - manual：需 name + cert + key（PEM 文本）。
type certRequest struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Domains   []string `json:"domains"`
	ValidDays int      `json:"valid_days"`
	Cert      string   `json:"cert"`
	Key       string   `json:"key"`
}

// ListCertificates GET /api/v1/certificates
func (h *Handler) ListCertificates(c *gin.Context) {
	out, err := h.Cert.List(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, out)
}

// CreateCertificate POST /api/v1/certificates
func (h *Handler) CreateCertificate(c *gin.Context) {
	var body certRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "请求体非法: "+err.Error()))
		return
	}

	switch body.Type {
	case "self_signed":
		cert, err := h.Cert.CreateSelfSigned(c.Request.Context(), body.Name, body.Domains, body.ValidDays)
		if err != nil {
			h.fail(c, err)
			return
		}
		h.ok(c, cert)
	case "manual":
		cert, err := h.Cert.ImportCert(c.Request.Context(), body.Name, body.Cert, body.Key)
		if err != nil {
			h.fail(c, err)
			return
		}
		h.ok(c, cert)
	default:
		c.JSON(http.StatusOK, model.Err(model.CodeBadRequest, "type 必须为 self_signed 或 manual"))
	}
}

// DeleteCertificate DELETE /api/v1/certificates/:id
func (h *Handler) DeleteCertificate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.Cert.Delete(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	h.ok(c, gin.H{"id": id.String()})
}

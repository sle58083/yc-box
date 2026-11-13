package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/singbox-panel/control-plane/internal/audit"
)

// Audit 记录所有已认证的写操作（POST/PUT/DELETE）到审计日志（append-only）。
// 公共端点（登录/订阅）由各自 handler 显式调用 audit service 记录。
func Audit(svc *audit.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		w := &statusWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = w
		c.Next()

		if !isMutating(c.Request.Method) {
			return
		}
		operator := UserID(c)
		if operator == "" {
			return
		}
		id, err := uuid.Parse(operator)
		if err != nil {
			return
		}
		action := c.Request.Method + " " + c.FullPath()
		resource := c.Request.URL.Path
		detail := map[string]any{"status": w.status}
		// 审计失败只记日志，不阻断业务。
		_ = svc.Record(context.Background(), id, action, resource, detail, c.ClientIP())
	}
}

func isMutating(method string) bool {
	return method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH"
}

type statusWriter struct {
	gin.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

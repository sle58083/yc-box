package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/rbac"
)

// RequirePermission 基于角色做资源级权限拦截（super_admin 通配，越权 100% 拦截）。
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := Role(c)
		if role == "" || !rbac.HasPermission(role, perm) {
			c.AbortWithStatusJSON(http.StatusOK, model.Err(model.CodeForbidden, "无权限执行该操作"))
			return
		}
		c.Next()
	}
}

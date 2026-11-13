package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/rbac"
)

func TestRequirePermissionForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(CtxRole, rbac.RoleViewer)

	RequirePermission(rbac.PermNodeWrite)(c)
	if !c.IsAborted() {
		t.Fatalf("viewer 写操作应被拦截")
	}
}

func TestRequirePermissionAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(CtxRole, rbac.RoleAdmin)

	RequirePermission(rbac.PermNodeWrite)(c)
	if c.IsAborted() {
		t.Fatalf("admin 写操作应放行")
	}
}

func TestStatusWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	sw := &statusWriter{ResponseWriter: c.Writer, status: 0}
	sw.WriteHeader(http.StatusCreated)
	if sw.status != http.StatusCreated {
		t.Fatalf("status 未记录: %d", sw.status)
	}
}

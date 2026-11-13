// Package api 是 REST handler 层（统一前缀 /api/v1）。
package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/agent"
	"github.com/singbox-panel/control-plane/internal/audit"
	"github.com/singbox-panel/control-plane/internal/auth"
	"github.com/singbox-panel/control-plane/internal/configsvc"
	"github.com/singbox-panel/control-plane/internal/deploy"
	"github.com/singbox-panel/control-plane/internal/group"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/node"
	"github.com/singbox-panel/control-plane/internal/subscribe"
	"github.com/singbox-panel/control-plane/internal/traffic"
	"github.com/singbox-panel/control-plane/internal/user"
	"gorm.io/gorm"
)

// Handler 持有各领域服务，供路由注册。
type Handler struct {
	Config    *configsvc.Service
	Node      *node.Service
	Deploy    *deploy.Orchestrator
	Agent     *agent.Handler
	Auth      *auth.Service
	User      *user.Service
	Subscribe *subscribe.Service
	Traffic   *traffic.Service
	Audit     *audit.Service
	Group     *group.Service
}

// NewHandler 构造 handler。
func NewHandler(
	cfg *configsvc.Service,
	nodeSvc *node.Service,
	deploySvc *deploy.Orchestrator,
	agentH *agent.Handler,
	authSvc *auth.Service,
	userSvc *user.Service,
	subSvc *subscribe.Service,
	trafficSvc *traffic.Service,
	auditSvc *audit.Service,
	groupSvc *group.Service,
) *Handler {
	return &Handler{
		Config:    cfg,
		Node:      nodeSvc,
		Deploy:    deploySvc,
		Agent:     agentH,
		Auth:      authSvc,
		User:      userSvc,
		Subscribe: subSvc,
		Traffic:   trafficSvc,
		Audit:     auditSvc,
		Group:     groupSvc,
	}
}

// ok 返回统一成功响应（system_design.md §9.1）。
func (h *Handler) ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, model.OK(data))
}

// fail 返回统一错误响应：404→1xxx、校验错误→4xxx、其余→1xxx 内部错误。
func (h *Handler) fail(c *gin.Context, err error) {
	code := model.CodeInternalError
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		code = model.CodeNotFound
	case configsvc.IsValidationError(err):
		code = model.CodeValidationFailed
	}
	c.JSON(http.StatusOK, model.Err(code, err.Error()))
}

// parseTime 解析时间参数（RFC3339 或常见格式）。
func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05", s)
	}
	return t, err
}

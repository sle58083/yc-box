// Package server 负责 HTTP 路由注册（最终集成入口）。
package server

import (
	"github.com/gin-gonic/gin"
	"github.com/singbox-panel/control-plane/internal/api"
	"github.com/singbox-panel/control-plane/internal/audit"
	"github.com/singbox-panel/control-plane/internal/auth"
	"github.com/singbox-panel/control-plane/internal/middleware"
	"github.com/singbox-panel/control-plane/internal/rbac"
)

// NewRouter 注册统一前缀 /api/v1 的 REST 路由 + Agent WS 长连接 + 认证/RBAC/审计。
func NewRouter(h *api.Handler, authSvc *auth.Service, auditSvc *audit.Service) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORS())

	// Agent 主动出站 WS 长连接（令牌在 register 消息中鉴权）。
	if h.Agent != nil {
		r.GET("/ws/agent", h.Agent.ServeWS)
	}

	v1 := r.Group("/api/v1")

	// 公开端点（无需面板 JWT）。
	v1.POST("/auth/login", h.Login)
	v1.POST("/auth/refresh", h.Refresh)
	v1.GET("/sub/:token", h.SubscribeContent)
	v1.GET("/sub/:token/clash", h.SubscribeContentClash)

	// 需登录 + 审计的端点。
	authed := v1.Group("")
	authed.Use(middleware.Auth(authSvc), middleware.Audit(auditSvc))
	{
		authed.GET("/auth/me", h.Me)

		// 协议元数据（前端动态表单）
		authed.GET("/protocols", h.ListProtocols)

		// 分组
		authed.GET("/groups", middleware.RequirePermission(rbac.PermGroupRead), h.ListGroups)
		authed.POST("/groups", middleware.RequirePermission(rbac.PermGroupWrite), h.CreateGroup)

		// 节点
		authed.GET("/nodes", middleware.RequirePermission(rbac.PermNodeRead), h.ListNodes)
		authed.POST("/nodes", middleware.RequirePermission(rbac.PermNodeWrite), h.CreateNode)
		authed.GET("/nodes/:id", middleware.RequirePermission(rbac.PermNodeRead), h.GetNode)
		authed.PUT("/nodes/:id", middleware.RequirePermission(rbac.PermNodeWrite), h.UpdateNode)
		authed.DELETE("/nodes/:id", middleware.RequirePermission(rbac.PermNodeDelete), h.DeleteNode)
		authed.POST("/nodes/:id/regenerate-token", middleware.RequirePermission(rbac.PermNodeRegenerateToken), h.RegenerateNodeToken)

		// 入站
		authed.GET("/inbounds", middleware.RequirePermission(rbac.PermInboundRead), h.ListInbounds)
		authed.POST("/inbounds", middleware.RequirePermission(rbac.PermInboundWrite), h.CreateInbound)
		authed.GET("/inbounds/:id", middleware.RequirePermission(rbac.PermInboundRead), h.GetInbound)
		authed.PUT("/inbounds/:id", middleware.RequirePermission(rbac.PermInboundWrite), h.UpdateInbound)
		authed.DELETE("/inbounds/:id", middleware.RequirePermission(rbac.PermInboundWrite), h.DeleteInbound)

		// 出站
		authed.GET("/outbounds", middleware.RequirePermission(rbac.PermOutboundRead), h.ListOutbounds)
		authed.POST("/outbounds", middleware.RequirePermission(rbac.PermOutboundWrite), h.CreateOutbound)
		authed.GET("/outbounds/:id", middleware.RequirePermission(rbac.PermOutboundRead), h.GetOutbound)
		authed.PUT("/outbounds/:id", middleware.RequirePermission(rbac.PermOutboundWrite), h.UpdateOutbound)
		authed.DELETE("/outbounds/:id", middleware.RequirePermission(rbac.PermOutboundWrite), h.DeleteOutbound)

		// 路由
		authed.GET("/routing", middleware.RequirePermission(rbac.PermRoutingRead), h.ListRoutingRules)
		authed.POST("/routing", middleware.RequirePermission(rbac.PermRoutingWrite), h.CreateRoutingRule)
		authed.PUT("/routing/:id", middleware.RequirePermission(rbac.PermRoutingWrite), h.UpdateRoutingRule)
		authed.DELETE("/routing/:id", middleware.RequirePermission(rbac.PermRoutingWrite), h.DeleteRoutingRule)

		// 配置
		authed.POST("/config/preview", middleware.RequirePermission(rbac.PermConfigPreview), h.PreviewConfig)
		authed.POST("/config/deploy", middleware.RequirePermission(rbac.PermConfigDeploy), h.DeployConfig)
		authed.POST("/config/retry", middleware.RequirePermission(rbac.PermConfigDeploy), h.RetryConfig)
		authed.POST("/config/rollback", middleware.RequirePermission(rbac.PermConfigRollback), h.RollbackConfig)
		authed.GET("/config/versions", middleware.RequirePermission(rbac.PermConfigPreview), h.ListVersions)
		authed.GET("/config/versions/:id", middleware.RequirePermission(rbac.PermConfigPreview), h.GetVersionContent)

		// 代理用户
		authed.GET("/proxy-users", middleware.RequirePermission(rbac.PermProxyUserRead), h.ListProxyUsers)
		authed.POST("/proxy-users", middleware.RequirePermission(rbac.PermProxyUserWrite), h.CreateProxyUser)
		authed.PUT("/proxy-users/:id", middleware.RequirePermission(rbac.PermProxyUserWrite), h.UpdateProxyUser)
		authed.DELETE("/proxy-users/:id", middleware.RequirePermission(rbac.PermProxyUserWrite), h.DeleteProxyUser)
		authed.POST("/proxy-users/:id/reset-sub", middleware.RequirePermission(rbac.PermProxyUserResetSub), h.ResetSubToken)

		// 流量 / 审计
		authed.GET("/traffic/nodes", middleware.RequirePermission(rbac.PermTrafficRead), h.NodeTraffic)
		authed.GET("/traffic/users", middleware.RequirePermission(rbac.PermTrafficRead), h.UserTraffic)
		authed.GET("/audit-logs", middleware.RequirePermission(rbac.PermAuditRead), h.ListAuditLogs)

		// 面板用户 / 角色（super_admin）
		authed.GET("/panel-users", middleware.RequirePermission(rbac.PermPanelUserRead), h.ListPanelUsers)
		authed.POST("/panel-users", middleware.RequirePermission(rbac.PermPanelUserWrite), h.CreatePanelUser)
		authed.PUT("/panel-users/:id", middleware.RequirePermission(rbac.PermPanelUserWrite), h.UpdatePanelUser)
		authed.DELETE("/panel-users/:id", middleware.RequirePermission(rbac.PermPanelUserWrite), h.DeletePanelUser)
		authed.GET("/roles", middleware.RequirePermission(rbac.PermRoleRead), h.ListRoles)
		authed.POST("/roles", middleware.RequirePermission(rbac.PermRoleWrite), h.CreateRole)
		authed.PUT("/roles/:id", middleware.RequirePermission(rbac.PermRoleWrite), h.UpdateRole)
		authed.DELETE("/roles/:id", middleware.RequirePermission(rbac.PermRoleWrite), h.DeleteRole)
	}

	return r
}

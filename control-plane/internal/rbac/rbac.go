// Package rbac 提供角色-权限模型基础（system_design.md §4.2「role」+ §9 RBAC）。
//
// MVP 三角色：super_admin / admin / viewer；super_admin 持有通配符 "*"。
// 权限点采用 "resource:action" 命名，资源与 §6.1 REST API 对齐。
package rbac

import "sort"

// 内置角色名。
const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleViewer     = "viewer"
)

// 资源权限点定义。
const (
	PermGroupRead   = "group:read"
	PermGroupWrite  = "group:write"
	PermGroupDelete = "group:delete"

	PermNodeRead            = "node:read"
	PermNodeWrite           = "node:write"
	PermNodeDelete          = "node:delete"
	PermNodeRegenerateToken = "node:regenerate_token"

	PermInboundRead  = "inbound:read"
	PermInboundWrite = "inbound:write"

	PermOutboundRead  = "outbound:read"
	PermOutboundWrite = "outbound:write"

	PermCertRead  = "cert:read"
	PermCertWrite = "cert:write"

	PermRoutingRead  = "routing:read"
	PermRoutingWrite = "routing:write"

	PermConfigPreview  = "config:preview"
	PermConfigDeploy   = "config:deploy"
	PermConfigRollback = "config:rollback"

	PermProxyUserRead     = "proxy_user:read"
	PermProxyUserWrite    = "proxy_user:write"
	PermProxyUserResetSub = "proxy_user:reset_sub"

	PermPanelUserRead  = "panel_user:read"
	PermPanelUserWrite = "panel_user:write"
	PermRoleRead       = "role:read"
	PermRoleWrite      = "role:write"

	PermTrafficRead = "traffic:read"
	PermAuditRead   = "audit:read"
	PermMonitorRead = "monitor:read"
)

// AllPermissions 是全部权限点（super_admin 之外角色的可分配全集）。
var AllPermissions = []string{
	PermGroupRead, PermGroupWrite, PermGroupDelete,
	PermNodeRead, PermNodeWrite, PermNodeDelete, PermNodeRegenerateToken,
	PermInboundRead, PermInboundWrite,
	PermOutboundRead, PermOutboundWrite,
	PermCertRead, PermCertWrite,
	PermRoutingRead, PermRoutingWrite,
	PermConfigPreview, PermConfigDeploy, PermConfigRollback,
	PermProxyUserRead, PermProxyUserWrite, PermProxyUserResetSub,
	PermPanelUserRead, PermPanelUserWrite,
	PermRoleRead, PermRoleWrite,
	PermTrafficRead, PermAuditRead, PermMonitorRead,
}

// rolePermissions 定义内置角色的权限集合。
var rolePermissions = map[string]map[string]bool{
	RoleSuperAdmin: {"*": true},
	RoleAdmin: buildSet(
		PermGroupRead, PermGroupWrite, PermGroupDelete,
		PermNodeRead, PermNodeWrite, PermNodeDelete,
		PermInboundRead, PermInboundWrite,
		PermOutboundRead, PermOutboundWrite,
		PermCertRead, PermCertWrite,
		PermRoutingRead, PermRoutingWrite,
		PermConfigPreview, PermConfigDeploy, PermConfigRollback,
		PermProxyUserRead, PermProxyUserWrite, PermProxyUserResetSub,
		PermTrafficRead, PermAuditRead, PermMonitorRead,
	),
	RoleViewer: buildSet(
		PermGroupRead,
		PermNodeRead,
		PermInboundRead,
		PermOutboundRead,
		PermCertRead,
		PermRoutingRead,
		PermProxyUserRead,
		PermTrafficRead, PermAuditRead, PermMonitorRead,
	),
}

func buildSet(perms ...string) map[string]bool {
	s := make(map[string]bool, len(perms))
	for _, p := range perms {
		s[p] = true
	}
	return s
}

// IsValidRole 判断角色名是否为内置角色。
func IsValidRole(name string) bool {
	_, ok := rolePermissions[name]
	return ok
}

// HasPermission 判断角色是否拥有指定权限点。
func HasPermission(roleName string, perm string) bool {
	set, ok := rolePermissions[roleName]
	if !ok {
		return false
	}
	if set["*"] {
		return true
	}
	return set[perm]
}

// PermissionsFor 返回角色的权限点列表（super_admin 返回 "*"）。
func PermissionsFor(roleName string) []string {
	set, ok := rolePermissions[roleName]
	if !ok {
		return nil
	}
	if set["*"] {
		return []string{"*"}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

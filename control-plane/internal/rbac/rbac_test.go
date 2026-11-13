package rbac

import "testing"

func TestHasPermission(t *testing.T) {
	cases := []struct {
		role string
		perm string
		want bool
	}{
		{RoleSuperAdmin, "anything:any", true}, // 通配
		{RoleAdmin, PermNodeWrite, true},
		{RoleAdmin, PermPanelUserWrite, false}, // admin 不可管理面板用户
		{RoleViewer, PermNodeRead, true},
		{RoleViewer, PermNodeWrite, false}, // viewer 无法写
		{RoleViewer, PermProxyUserWrite, false},
		{RoleViewer, PermTrafficRead, true},
	}
	for _, c := range cases {
		if got := HasPermission(c.role, c.perm); got != c.want {
			t.Fatalf("HasPermission(%s, %s) = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}

func TestIsValidRole(t *testing.T) {
	if !IsValidRole(RoleAdmin) || IsValidRole("foobar") {
		t.Fatalf("IsValidRole 错误")
	}
}

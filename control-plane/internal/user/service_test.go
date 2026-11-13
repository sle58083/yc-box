package user

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/rbac"
	"github.com/singbox-panel/control-plane/internal/store"
	"gorm.io/gorm"
)

// newTestService 用内存 SQLite 构造用户域服务，规避 PostgreSQL 外部依赖。
// 测试策略说明：项目尚无 DB mock 基础设施，且构建环境禁用 CGO，
// 故引入纯 Go 的 glebarez/sqlite 作为测试专用内存库；只建 user 域涉及的两张表。
// 时间列用 SQLite 可识别的 DATETIME，避免 model 的 timestamptz 类型无法回读。
func newTestService(t *testing.T) *Service {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	stmts := []string{
		`CREATE TABLE role (
			id TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			name TEXT NOT NULL UNIQUE,
			permissions BLOB
		)`,
		`CREATE TABLE panel_user (
			id TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'enabled'
		)`,
	}
	for _, stmt := range stmts {
		if err := gdb.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	km, err := crypto.NewStaticKeyManager(make([]byte, 32))
	if err != nil {
		t.Fatalf("new key manager: %v", err)
	}
	return NewService(&store.DB{DB: gdb}, crypto.New(km))
}

func seedRoles(t *testing.T, svc *Service) {
	t.Helper()
	if err := svc.SeedRoles(context.Background()); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
}

// TestSeedAdmin_CreatesFirstSuperAdmin 覆盖「首次创建成功」路径。
func TestSeedAdmin_CreatesFirstSuperAdmin(t *testing.T) {
	svc := newTestService(t)
	seedRoles(t, svc)
	ctx := context.Background()

	created, err := svc.SeedAdmin(ctx, "admin", "secret-password")
	if err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}
	if !created {
		t.Fatalf("首次调用应创建初始管理员")
	}

	var users []model.PanelUser
	if err := svc.db.WithContext(ctx).Find(&users).Error; err != nil {
		t.Fatalf("list panel users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("应仅有 1 个面板用户，实际 %d", len(users))
	}

	u := users[0]
	if u.Username != "admin" {
		t.Fatalf("用户名不符: %q", u.Username)
	}

	var role model.Role
	if err := svc.db.WithContext(ctx).First(&role, "name = ?", rbac.RoleSuperAdmin).Error; err != nil {
		t.Fatalf("find super_admin role: %v", err)
	}
	if u.RoleID != role.ID {
		t.Fatalf("角色不符: %s != %s", u.RoleID, role.ID)
	}
	if ok, err := crypto.VerifyPassword("secret-password", u.PasswordHash); err != nil || !ok {
		t.Fatalf("初始密码应通过 argon2id 校验: ok=%v err=%v", ok, err)
	}
}

// TestSeedAdmin_IdempotentSkipWhenUsersExist 覆盖「二次调用跳过（幂等）」路径。
func TestSeedAdmin_IdempotentSkipWhenUsersExist(t *testing.T) {
	svc := newTestService(t)
	seedRoles(t, svc)
	ctx := context.Background()

	if created, err := svc.SeedAdmin(ctx, "admin", "first-password"); err != nil || !created {
		t.Fatalf("首次调用应创建: created=%v err=%v", created, err)
	}

	created, err := svc.SeedAdmin(ctx, "admin", "second-password")
	if err != nil {
		t.Fatalf("二次调用不应报错: %v", err)
	}
	if created {
		t.Fatalf("二次调用不应重复创建")
	}

	var users []model.PanelUser
	if err := svc.db.WithContext(ctx).Find(&users).Error; err != nil {
		t.Fatalf("list panel users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("应仍仅有 1 个面板用户，实际 %d", len(users))
	}
	if ok, _ := crypto.VerifyPassword("first-password", users[0].PasswordHash); !ok {
		t.Fatalf("已有初始密码不应被覆盖")
	}
	if ok, _ := crypto.VerifyPassword("second-password", users[0].PasswordHash); ok {
		t.Fatalf("二次密码不应生效")
	}
}

// TestSeedAdmin_SkipsEmptyPassword 覆盖「空密码跳过」路径。
func TestSeedAdmin_SkipsEmptyPassword(t *testing.T) {
	svc := newTestService(t)
	seedRoles(t, svc)
	ctx := context.Background()

	created, err := svc.SeedAdmin(ctx, "admin", "")
	if err != nil {
		t.Fatalf("空密码应跳过而非报错: %v", err)
	}
	if created {
		t.Fatalf("空密码不应创建初始管理员")
	}

	var cnt int64
	if err := svc.db.WithContext(ctx).Model(&model.PanelUser{}).Count(&cnt).Error; err != nil {
		t.Fatalf("count panel users: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("空密码时不应创建面板用户，实际 %d", cnt)
	}
}

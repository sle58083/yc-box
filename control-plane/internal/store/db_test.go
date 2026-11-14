package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/singbox-panel/control-plane/internal/model"
)

// newTestDB 在临时目录创建 SQLite 数据库并完成迁移。
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(filepath.Join(t.TempDir(), "panel.db"), "silent")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// TestAutoMigrateSQLite 验证 12 张表在 SQLite 下可创建，且 AutoMigrate 幂等。
func TestAutoMigrateSQLite(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "panel.db"), "silent")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("first automigrate: %v", err)
	}
	// 幂等：再次迁移不应报错。
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("second automigrate (idempotent): %v", err)
	}

	var count int64
	if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if count != 12 {
		t.Fatalf("期望 12 张业务表，实际 %d", count)
	}

	// 验证 SQLite 关键 pragma 生效（WAL / 外键）。
	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("期望 journal_mode=wal，实际 %q", journalMode)
	}

	var fk int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&fk).Error; err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("期望 foreign_keys=1，实际 %d", fk)
	}
}

// TestSQLiteCRUDRoundTrip 验证 uuid / json.RawMessage / time.Time 往返一致，
// 并覆盖 12 张表的基本 CRUD（先父表后子表）。
func TestSQLiteCRUDRoundTrip(t *testing.T) {
	db := newTestDB(t)

	// 1. role：permissions 为 json.RawMessage（jsonb → BLOB）
	role := model.Role{Name: "super_admin", Permissions: json.RawMessage(`["*"]`)}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	var roleOut model.Role
	if err := db.First(&roleOut, "name = ?", "super_admin").Error; err != nil {
		t.Fatalf("read role: %v", err)
	}
	if roleOut.ID != role.ID {
		t.Fatalf("role uuid 往返不一致: %s != %s", roleOut.ID, role.ID)
	}
	if !jsonBytesEqual(roleOut.Permissions, role.Permissions) {
		t.Fatalf("role permissions 往返不一致: %s != %s", roleOut.Permissions, role.Permissions)
	}

	// 2. group：name/desc/parent_id
	parent := model.Group{Name: "parent", Desc: "顶层分组"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent group: %v", err)
	}
	group := model.Group{Name: "default", Desc: "默认分组", ParentID: &parent.ID}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	var groupOut model.Group
	if err := db.First(&groupOut, "name = ?", "default").Error; err != nil {
		t.Fatalf("read group: %v", err)
	}
	if groupOut.ID != group.ID || groupOut.ParentID == nil || *groupOut.ParentID != parent.ID {
		t.Fatalf("group uuid/parent_id 往返不一致")
	}

	// 3. node：LastHeartbeat 为 *time.Time，Tags 为 json.RawMessage
	hb := time.Date(2024, 1, 2, 3, 4, 5, 123000000, time.UTC)
	node := model.Node{
		GroupID:        group.ID,
		Name:           "edge-1",
		Address:        "127.0.0.1",
		AgentTokenHash: "deadbeef",
		Status:         model.NodeStatusPending,
		LastHeartbeat:  &hb,
		Tags:           json.RawMessage(`["vmess","vless"]`),
	}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	var nodeOut model.Node
	if err := db.First(&nodeOut, "id = ?", node.ID).Error; err != nil {
		t.Fatalf("read node: %v", err)
	}
	if nodeOut.ID != node.ID {
		t.Fatalf("node uuid 往返不一致")
	}
	if nodeOut.LastHeartbeat == nil || !nodeOut.LastHeartbeat.Equal(hb) {
		t.Fatalf("node last_heartbeat 往返不一致: %v != %v", nodeOut.LastHeartbeat, hb)
	}
	if !jsonBytesEqual(nodeOut.Tags, node.Tags) {
		t.Fatalf("node tags 往返不一致: %s != %s", nodeOut.Tags, node.Tags)
	}
	if nodeOut.CreatedAt.IsZero() || nodeOut.UpdatedAt.IsZero() {
		t.Fatalf("node 自动时间字段未写入")
	}

	// 4. panel_user：引用 role
	user := model.PanelUser{Username: "admin", PasswordHash: "hash", RoleID: role.ID, Status: model.StatusEnabled}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create panel_user: %v", err)
	}
	var userOut model.PanelUser
	if err := db.First(&userOut, "username = ?", "admin").Error; err != nil {
		t.Fatalf("read panel_user: %v", err)
	}
	if userOut.ID != user.ID || userOut.RoleID != role.ID {
		t.Fatalf("panel_user uuid/role_id 往返不一致")
	}

	// 5. inbound_config：settings/tls/transport 为 json.RawMessage，含 nullable node_id
	inbound := model.InboundConfig{
		GroupID:   group.ID,
		NodeID:    &node.ID,
		Name:      "vmess-in",
		Type:      "vmess",
		Listen:    "0.0.0.0",
		Port:      10000,
		Settings:  json.RawMessage(`{"clients":[]}`),
		TLS:       json.RawMessage(`{"enabled":false}`),
		Transport: json.RawMessage(`{"type":"ws"}`),
		Status:    model.StatusEnabled,
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound_config: %v", err)
	}
	var inboundOut model.InboundConfig
	if err := db.First(&inboundOut, "id = ?", inbound.ID).Error; err != nil {
		t.Fatalf("read inbound_config: %v", err)
	}
	if !jsonBytesEqual(inboundOut.Settings, inbound.Settings) || !jsonBytesEqual(inboundOut.TLS, inbound.TLS) || !jsonBytesEqual(inboundOut.Transport, inbound.Transport) {
		t.Fatalf("inbound jsonb 字段往返不一致")
	}
	if inboundOut.NodeID == nil || *inboundOut.NodeID != node.ID {
		t.Fatalf("inbound node_id 往返不一致")
	}

	// 6. outbound_config
	outbound := model.OutboundConfig{
		GroupID:   group.ID,
		Name:      "proxy-out",
		Type:      "vless",
		Settings:  json.RawMessage(`{"vnext":[]}`),
		TLS:       json.RawMessage(`{}`),
		Transport: json.RawMessage(`{}`),
		Status:    model.StatusEnabled,
	}
	if err := db.Create(&outbound).Error; err != nil {
		t.Fatalf("create outbound_config: %v", err)
	}
	var outboundOut model.OutboundConfig
	if err := db.First(&outboundOut, "id = ?", outbound.ID).Error; err != nil {
		t.Fatalf("read outbound_config: %v", err)
	}
	if !jsonBytesEqual(outboundOut.Settings, outbound.Settings) {
		t.Fatalf("outbound settings 往返不一致")
	}

	// 7. routing_rule
	routing := model.RoutingRule{
		GroupID:     group.ID,
		NodeID:      &node.ID,
		Name:        "rule-1",
		InboundTags: json.RawMessage(`["vmess-in"]`),
		OutboundTag: "proxy-out",
		RuleType:    "domain",
		Value:       "example.com",
		Priority:    10,
	}
	if err := db.Create(&routing).Error; err != nil {
		t.Fatalf("create routing_rule: %v", err)
	}
	var routingOut model.RoutingRule
	if err := db.First(&routingOut, "id = ?", routing.ID).Error; err != nil {
		t.Fatalf("read routing_rule: %v", err)
	}
	if !jsonBytesEqual(routingOut.InboundTags, routing.InboundTags) {
		t.Fatalf("routing inbound_tags 往返不一致")
	}

	// 8. proxy_user：ExpireAt 为 *time.Time，int64 配额
	expire := time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)
	proxy := model.ProxyUser{
		GroupID:     group.ID,
		UUID:        "enc-uuid",
		PasswordEnc: "enc-pass",
		Remark:      "终端用户",
		QuotaBytes:  1024 * 1024 * 1024,
		UsedBytes:   12345,
		ExpireAt:    &expire,
		Status:      model.StatusEnabled,
	}
	if err := db.Create(&proxy).Error; err != nil {
		t.Fatalf("create proxy_user: %v", err)
	}
	var proxyOut model.ProxyUser
	if err := db.First(&proxyOut, "id = ?", proxy.ID).Error; err != nil {
		t.Fatalf("read proxy_user: %v", err)
	}
	if proxyOut.QuotaBytes != proxy.QuotaBytes || proxyOut.UsedBytes != proxy.UsedBytes {
		t.Fatalf("proxy_user 配额字段往返不一致")
	}
	if proxyOut.ExpireAt == nil || !proxyOut.ExpireAt.Equal(expire) {
		t.Fatalf("proxy_user expire_at 往返不一致")
	}

	// 9. config_version：独立 CreatedAt（无 UpdatedAt）
	ver := model.ConfigVersion{
		NodeID:     node.ID,
		Version:    1,
		ContentEnc: "encrypted-content",
		Hash:       "hash-1",
		Status:     model.ConfigVersionDraft,
		CreatedBy:  user.ID,
	}
	if err := db.Create(&ver).Error; err != nil {
		t.Fatalf("create config_version: %v", err)
	}
	var verOut model.ConfigVersion
	if err := db.First(&verOut, "id = ?", ver.ID).Error; err != nil {
		t.Fatalf("read config_version: %v", err)
	}
	if verOut.NodeID != node.ID || verOut.Version != 1 || verOut.CreatedAt.IsZero() {
		t.Fatalf("config_version 字段往返不一致")
	}

	// 10. traffic_record：组合主键含 time.Time
	ts := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	rec := model.TrafficRecord{
		Ts:          ts,
		NodeID:      node.ID,
		InboundTag:  "vmess-in",
		OutboundTag: "proxy-out",
		ProxyUserID: &proxy.ID,
		UpBytes:     100,
		DownBytes:   200,
	}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create traffic_record: %v", err)
	}
	var recOut model.TrafficRecord
	if err := db.First(&recOut, "node_id = ? AND inbound_tag = ? AND outbound_tag = ?", node.ID, "vmess-in", "proxy-out").Error; err != nil {
		t.Fatalf("read traffic_record: %v", err)
	}
	if !recOut.Ts.Equal(ts) || recOut.UpBytes != 100 || recOut.DownBytes != 200 {
		t.Fatalf("traffic_record 字段往返不一致")
	}
	if recOut.ProxyUserID == nil || *recOut.ProxyUserID != proxy.ID {
		t.Fatalf("traffic_record proxy_user_id 往返不一致")
	}

	// 11. audit_log：detail 为 json.RawMessage
	audit := model.AuditLog{
		OperatorID: user.ID,
		Action:     "node.create",
		Resource:   "node",
		Detail:     json.RawMessage(`{"before":null,"after":{"name":"edge-1"}}`),
		IP:         "127.0.0.1",
	}
	if err := db.Create(&audit).Error; err != nil {
		t.Fatalf("create audit_log: %v", err)
	}
	var auditOut model.AuditLog
	if err := db.First(&auditOut, "id = ?", audit.ID).Error; err != nil {
		t.Fatalf("read audit_log: %v", err)
	}
	if auditOut.OperatorID != user.ID || !jsonBytesEqual(auditOut.Detail, audit.Detail) {
		t.Fatalf("audit_log 字段往返不一致")
	}

	// 12. certificate：*time.Time 时间字段 + PEM 字段（T01 仅建模型，加密在 T03 service 层）
	nb := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	na := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	cert := model.Certificate{
		Name:      "panel.example.com",
		Type:      "self_signed",
		Domains:   `["panel.example.com","*.panel.example.com"]`,
		CertPEM:   "enc:v1:cert",
		KeyPEM:    "enc:v1:key",
		Issuer:    "CN=panel.example.com",
		NotBefore: &nb,
		NotAfter:  &na,
		Status:    "valid",
	}
	if err := db.Create(&cert).Error; err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	var certOut model.Certificate
	if err := db.First(&certOut, "id = ?", cert.ID).Error; err != nil {
		t.Fatalf("read certificate: %v", err)
	}
	if certOut.NotBefore == nil || !certOut.NotBefore.Equal(nb) {
		t.Fatalf("certificate not_before 往返不一致")
	}
	if certOut.NotAfter == nil || !certOut.NotAfter.Equal(na) {
		t.Fatalf("certificate not_after 往返不一致")
	}
	if certOut.CertPEM != cert.CertPEM || certOut.KeyPEM != cert.KeyPEM {
		t.Fatalf("certificate PEM 字段往返不一致")
	}
}

func jsonBytesEqual(a, b json.RawMessage) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var av, bv interface{}
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	as, _ := json.Marshal(av)
	bs, _ := json.Marshal(bv)
	return string(as) == string(bs)
}

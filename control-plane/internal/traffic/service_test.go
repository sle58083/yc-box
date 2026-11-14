package traffic

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/store"
)

func TestToTrafficRecord(t *testing.T) {
	nodeID := uuid.New()
	userID := uuid.New()
	ts := int64(1700000000)
	plainUUID := "uuid-plain"
	rec := toTrafficRecord(Record{
		TS:          ts,
		ProxyUserID: &plainUUID,
		InboundTag:  "in",
		OutboundTag: "out",
		UpBytes:     10,
		DownBytes:   20,
	}, nodeID, &userID)

	if rec.NodeID != nodeID || rec.ProxyUserID == nil || *rec.ProxyUserID != userID {
		t.Fatalf("字段映射错误: %+v", rec)
	}
	if rec.UpBytes != 10 || rec.DownBytes != 20 {
		t.Fatalf("流量数值错误")
	}
	if !rec.Ts.Equal(time.Unix(ts, 0).UTC()) {
		t.Fatalf("时间转换错误: %v", rec.Ts)
	}
}

// TestIngestMultiUserSameSecondNoConflict 回归：≥2 个活跃用户在同一秒采集落库时，
// traffic_record 组合主键 (ts,node_id,inbound_tag,outbound_tag) 不应冲突。
// 用户级记录 OutboundTag 采用 "user-"+DB UUID，保证每个用户主键唯一。
func TestIngestMultiUserSameSecondNoConflict(t *testing.T) {
	db, err := store.New(filepath.Join(t.TempDir(), "panel.db"), "silent")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	svc := NewService(db, nil) // Ingest 仅经 resolveByID 解析 UUID，不触碰 codec
	nodeID := uuid.New()
	userA := uuid.NewString()
	userB := uuid.NewString()
	ts := int64(1700000000)

	records := []Record{
		{TS: ts, ProxyUserID: &userA, InboundTag: "-", OutboundTag: "user-" + userA, UpBytes: 10, DownBytes: 20},
		{TS: ts, ProxyUserID: &userB, InboundTag: "-", OutboundTag: "user-" + userB, UpBytes: 30, DownBytes: 40},
	}

	if err := svc.Ingest(context.Background(), nodeID, records); err != nil {
		t.Fatalf("多用户同秒摄入不应冲突: %v", err)
	}

	var cnt int64
	if err := db.Model(&model.TrafficRecord{}).
		Where("node_id = ? AND ts = ?", nodeID, time.Unix(ts, 0).UTC()).
		Count(&cnt).Error; err != nil {
		t.Fatalf("count traffic_record: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("应落库 2 条用户记录，实际 %d", cnt)
	}
}

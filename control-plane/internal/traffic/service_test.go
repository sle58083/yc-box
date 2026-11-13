package traffic

import (
	"testing"
	"time"

	"github.com/google/uuid"
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

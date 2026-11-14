package stats

import (
	"testing"

	statsproto "github.com/singbox-panel/edge-agent/internal/stats/proto"
)

func TestParseCounterName(t *testing.T) {
	cases := []struct {
		name                     string
		category, key, direction string
		ok                       bool
	}{
		{"user>>>u1>>>traffic>>>uplink", "user", "u1", "uplink", true},
		{"user>>>u1>>>traffic>>>downlink", "user", "u1", "downlink", true},
		{"inbound>>>vless-in>>>traffic>>>uplink", "inbound", "vless-in", "uplink", true},
		{"outbound>>>direct>>>traffic>>>downlink", "outbound", "direct", "downlink", true},
		{"user>>>u1>>>traffic>>>other", "", "", "", false},
		{"bogus", "", "", "", false},
		{"user>>>a>>>b>>>c>>>uplink", "", "", "", false},
	}
	for _, c := range cases {
		category, key, direction, ok := parseCounterName(c.name)
		if category != c.category || key != c.key || direction != c.direction || ok != c.ok {
			t.Fatalf("parseCounterName(%q) = (%q,%q,%q,%v), want (%q,%q,%q,%v)",
				c.name, category, key, direction, ok, c.category, c.key, c.direction, c.ok)
		}
	}
}

func TestAggregateRecords(t *testing.T) {
	stats := []statsproto.Stat{
		{Name: "user>>>u1>>>traffic>>>uplink", Value: 100},
		{Name: "user>>>u1>>>traffic>>>downlink", Value: 200},
		{Name: "inbound>>>vless-in>>>traffic>>>uplink", Value: 5},
		{Name: "outbound>>>direct>>>traffic>>>downlink", Value: 7},
		{Name: "unknown>>>x>>>traffic>>>uplink", Value: 99},
	}
	records := aggregateRecords(stats, 1700000000)
	if len(records) != 3 {
		t.Fatalf("记录数: got %d want 3: %+v", len(records), records)
	}

	// 按 (类别, 标识) 排序：inbound -> outbound -> user。
	if records[0].InboundTag != "vless-in" || records[0].UpBytes != 5 {
		t.Fatalf("inbound 记录错误: %+v", records[0])
	}
	if records[1].OutboundTag != "direct" || records[1].DownBytes != 7 {
		t.Fatalf("outbound 记录错误: %+v", records[1])
	}
	if records[2].ProxyUserID == nil || *records[2].ProxyUserID != "u1" ||
		records[2].UpBytes != 100 || records[2].DownBytes != 200 {
		t.Fatalf("user 记录错误: %+v", records[2])
	}
	if records[2].InboundTag != "-" || records[2].OutboundTag != "user-u1" {
		t.Fatalf("user 记录哨兵 tag 错误: %+v", records[2])
	}
}

// TestAggregateRecordsMultiUserDistinctTags 回归：多用户同一秒采集时，
// 用户级 OutboundTag 必须含用户标识（"user-"+uuid），保证落库主键不冲突。
func TestAggregateRecordsMultiUserDistinctTags(t *testing.T) {
	stats := []statsproto.Stat{
		{Name: "user>>>u1>>>traffic>>>uplink", Value: 1},
		{Name: "user>>>u1>>>traffic>>>downlink", Value: 2},
		{Name: "user>>>u2>>>traffic>>>uplink", Value: 3},
		{Name: "user>>>u2>>>traffic>>>downlink", Value: 4},
	}
	records := aggregateRecords(stats, 1700000000)
	if len(records) != 2 {
		t.Fatalf("记录数: got %d want 2: %+v", len(records), records)
	}
	if records[0].OutboundTag == records[1].OutboundTag {
		t.Fatalf("多用户 OutboundTag 不应相同: %+v", records)
	}
	if records[0].OutboundTag != "user-u1" || records[1].OutboundTag != "user-u2" {
		t.Fatalf("用户级 OutboundTag 应为 user-<uuid>: %+v", records)
	}
}

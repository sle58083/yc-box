// Package stats 负责从 sing-box 采集流量统计并组装 traffic_report。
//
// sing-box 通过 experimental.v2ray_api 暴露 gRPC 接口
// StatsService/QueryStats；本包用最小化 gRPC 客户端周期拉取计数器，
// 按哨兵 tag 归并为用户级/节点级 Record。
package stats

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statsproto "github.com/singbox-panel/edge-agent/internal/stats/proto"
)

// Record 单条流量记录（与控制面 traffic_report 的 record 对齐）。
type Record struct {
	TS          int64   `json:"ts"`
	ProxyUserID *string `json:"proxy_user_id,omitempty"` // proxy_user DB UUID 字符串（空=节点级）
	InboundTag  string  `json:"inbound_tag"`
	OutboundTag string  `json:"outbound_tag"`
	UpBytes     int64   `json:"up_bytes"`
	DownBytes   int64   `json:"down_bytes"`
}

// Collector 是 sing-box 流量采集器。
type Collector struct {
	statsAPI string // sing-box v2ray_api gRPC 地址（如 127.0.0.1:9001），空=未启用
}

// NewCollector 构造采集器。
func NewCollector(statsAPI string) *Collector {
	return &Collector{statsAPI: statsAPI}
}

// Collect 拉取并归并流量记录（reset=true 每次清零，取增量）。
func (c *Collector) Collect(ctx context.Context) ([]Record, error) {
	if c.statsAPI == "" {
		return nil, nil
	}

	// v2ray_api 是本地回环 gRPC，明文通道即可。
	conn, err := grpc.DialContext(ctx, c.statsAPI, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("stats: dial %s: %w", c.statsAPI, err)
	}
	defer conn.Close()

	cli := statsproto.NewStatsServiceClient(conn)
	resp, err := cli.QueryStats(ctx, &statsproto.QueryStatsRequest{Pattern: "", Reset: true})
	if err != nil {
		return nil, fmt.Errorf("stats: query: %w", err)
	}

	return aggregateRecords(resp.Stat, time.Now().Unix()), nil
}

// counterKey 用于把 uplink/downlink 两条计数器配对到同一条 Record。
type counterKey struct {
	category string // user | inbound | outbound
	key      string // 用户 DB UUID 或入站/出站 tag
}

// aggregateRecords 按哨兵 tag 将计数器归并为 []Record（up/down 配对）。
func aggregateRecords(stats []statsproto.Stat, ts int64) []Record {
	m := map[counterKey]*Record{}
	for _, st := range stats {
		category, key, direction, ok := parseCounterName(st.Name)
		if !ok {
			continue
		}
		ck := counterKey{category: category, key: key}
		rec := m[ck]
		if rec == nil {
			rec = newRecord(category, key, ts)
			if rec == nil {
				continue
			}
			m[ck] = rec
		}
		switch direction {
		case "uplink":
			rec.UpBytes = st.Value
		case "downlink":
			rec.DownBytes = st.Value
		}
	}

	// 按 (类别, 标识) 排序输出，保证结果确定、便于联调与测试。
	keys := make([]counterKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].category != keys[j].category {
			return keys[i].category < keys[j].category
		}
		return keys[i].key < keys[j].key
	})

	out := make([]Record, 0, len(keys))
	for _, k := range keys {
		out = append(out, *m[k])
	}
	return out
}

// newRecord 按计数器类别构造 Record 骨架（§3.4 哨兵 tag 规则）。
func newRecord(category, key string, ts int64) *Record {
	switch category {
	case "user":
		id := key
		// 用户级 OutboundTag 使用 "user-"+DB UUID 而非固定 "-"，使同一秒内
		// 多个用户的 traffic_record 组合主键 (ts,node_id,inbound_tag,outbound_tag)
		// 互不冲突（UserTraffic/NodeTraffic 均按 proxy_user_id/node_id 聚合、忽略 tag）。
		return &Record{TS: ts, ProxyUserID: &id, InboundTag: "-", OutboundTag: "user-" + key}
	case "inbound":
		return &Record{TS: ts, InboundTag: key, OutboundTag: "-"}
	case "outbound":
		return &Record{TS: ts, InboundTag: "-", OutboundTag: key}
	default:
		return nil
	}
}

// parseCounterName 解析 sing-box 计数器名：
//
//	user>>>{uuid}>>>traffic>>>uplink/downlink
//	inbound>>>{tag}>>>traffic>>>uplink/downlink
//	outbound>>>{tag}>>>traffic>>>uplink/downlink
//
// 返回 (类别, 标识, 方向, 是否有效)。
func parseCounterName(name string) (category, key, direction string, ok bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 || parts[2] != "traffic" {
		return "", "", "", false
	}
	if parts[3] != "uplink" && parts[3] != "downlink" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[3], true
}

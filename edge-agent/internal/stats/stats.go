// Package stats 负责从 sing-box 采集流量统计并组装 traffic_report。
//
// sing-box 通过 V2Ray API（experimental）暴露 /traffic 流量接口；本包是采集点
// 的预留封装。MVP 阶段可先返回空（流量统计走控制面 heartbeat 摘要），
// 待启用 sing-box stats API 后接入。
package stats

import (
	"context"
)

// Record 单条流量记录（与控制面 traffic_report 的 record 对齐）。
type Record struct {
	TS          int64   `json:"ts"`
	ProxyUserID *string `json:"proxy_user_id,omitempty"` // 代理用户 UUID 明文（空=节点级）
	InboundTag  string  `json:"inbound_tag"`
	OutboundTag string  `json:"outbound_tag"`
	UpBytes     int64   `json:"up_bytes"`
	DownBytes   int64   `json:"down_bytes"`
}

// Collector 是 sing-box 流量采集器。
type Collector struct {
	statsAPI string // sing-box V2Ray API 基地址（如 http://127.0.0.1:9001），空=未启用
}

// NewCollector 构造采集器。
func NewCollector(statsAPI string) *Collector {
	return &Collector{statsAPI: statsAPI}
}

// Collect 拉取流量记录。
// TODO(P1)：调用 sing-box stats API（GET {statsAPI}/traffic）解析用户级流量；
// 当前 MVP 未启用 stats API，返回空切片。
func (c *Collector) Collect(ctx context.Context) ([]Record, error) {
	if c.statsAPI == "" {
		return nil, nil
	}
	// 预留：GET c.statsAPI + "/traffic"，按 tag 聚合为用户级/节点级 Record。
	return nil, nil
}

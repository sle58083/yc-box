// Package agent 实现控制面 Agent Hub（WebSocket 长连接中心）与消息协议。
package agent

import (
	"encoding/json"
	"time"
)

// 消息类型（system_design.md §6.2）。
const (
	MsgRegister          = "register"            // Agent→控制面
	MsgHeartbeat         = "heartbeat"           // Agent→控制面
	MsgTrafficReport     = "traffic_report"      // Agent→控制面
	MsgConfigApplyResult = "config_apply_result" // Agent→控制面
	MsgLog               = "log"                 // Agent→控制面

	MsgConfigPush = "config_push" // 控制面→Agent
	MsgRestart    = "restart"     // 控制面→Agent
	MsgPing       = "ping"        // 控制面→Agent
)

// Envelope 是统一消息信封（§9.9：带 id 幂等 + ts）。
type Envelope struct {
	Type string          `json:"type"`
	ID   string          `json:"id"` // UUID v4，控制面按 id 去重
	TS   int64           `json:"ts"` // Unix 秒
	Data json.RawMessage `json:"data"`
}

// NewEnvelope 构造信封。
func NewEnvelope(typ string, id string, data any) (*Envelope, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Envelope{Type: typ, ID: id, TS: time.Now().Unix(), Data: raw}, nil
}

// UnmarshalData 解析 data 到目标结构。
func (e *Envelope) UnmarshalData(v any) error {
	if len(e.Data) == 0 {
		return nil
	}
	return json.Unmarshal(e.Data, v)
}

// RegisterData 是 register 消息的 data。
type RegisterData struct {
	Token          string   `json:"token"`
	HostInfo       HostInfo `json:"hostinfo"`
	AgentVersion   string   `json:"agent_version"`
	SingboxVersion string   `json:"singbox_version"`
}

// HostInfo 是注册时上报的主机信息。
type HostInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	Libc string `json:"libc"`
}

// HeartbeatData 是 heartbeat 消息的 data（含流量摘要，P1）。
type HeartbeatData struct {
	AgentVersion   string `json:"agent_version"`
	SingboxVersion string `json:"singbox_version"`
	UpBytes        int64  `json:"up_bytes,omitempty"`
	DownBytes      int64  `json:"down_bytes,omitempty"`
}

// ConfigApplyResultData 是 config_apply_result 消息的 data（§9.9：必须回传 version）。
type ConfigApplyResultData struct {
	Version  int    `json:"version"`
	Status   string `json:"status"`   // success | failed | rolled_back
	Graceful bool   `json:"graceful"` // linux 热重载 true / windows 重启式 false
	Error    string `json:"error,omitempty"`
}

// ConfigPushData 是 config_push 消息的 data。
type ConfigPushData struct {
	Version int    `json:"version"`
	Content []byte `json:"content"` // sing-box JSON 全文
}

// RestartData 是 restart 消息的 data（预留）。
type RestartData struct {
	Version int `json:"version,omitempty"`
}

// TrafficReportData 是 traffic_report 消息的 data（T04 流量上报）。
type TrafficReportData struct {
	Records []TrafficRecordEntry `json:"records"`
}

// TrafficRecordEntry 单条流量记录。
type TrafficRecordEntry struct {
	TS          int64   `json:"ts"`
	ProxyUserID *string `json:"proxy_user_id,omitempty"` // 代理用户 UUID 明文（空=节点级）
	InboundTag  string  `json:"inbound_tag"`
	OutboundTag string  `json:"outbound_tag"`
	UpBytes     int64   `json:"up_bytes"`
	DownBytes   int64   `json:"down_bytes"`
}

// 应用结果状态（与 config_version 状态机对齐）。
const (
	ApplyStatusSuccess    = "success"
	ApplyStatusFailed     = "failed"
	ApplyStatusRolledBack = "rolled_back"
)

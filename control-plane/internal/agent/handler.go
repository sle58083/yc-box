package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/singbox-panel/control-plane/internal/node"
)

// ApplyResult 是 config_apply_result 的归一化结果，交给下发编排消费。
type ApplyResult struct {
	NodeID   uuid.UUID
	Version  int
	Status   string
	Graceful bool
	Error    string
}

// ApplyResultHandler 处理 config_apply_result（由 deploy 编排注入，避免循环依赖）。
type ApplyResultHandler func(ApplyResult)

// TrafficReportHandler 处理 traffic_report（由 traffic service 注入）。
type TrafficReportHandler func(nodeID uuid.UUID, data TrafficReportData)

// Handler 是 Agent WS 服务入口。
type Handler struct {
	nodes           *node.Service
	hub             *Hub
	log             *zap.Logger
	upgrader        websocket.Upgrader
	onApplyResult   ApplyResultHandler
	onTrafficReport TrafficReportHandler
	registerWait    time.Duration
	readLimit       int64

	seenMu      sync.Mutex
	seen        map[string]time.Time
	dedupWindow time.Duration
}

// NewHandler 构造 Agent WS handler。
func NewHandler(nodes *node.Service, hub *Hub, log *zap.Logger, onApplyResult ApplyResultHandler, onTrafficReport TrafficReportHandler) *Handler {
	return &Handler{
		nodes:           nodes,
		hub:             hub,
		log:             log,
		onApplyResult:   onApplyResult,
		onTrafficReport: onTrafficReport,
		registerWait:    15 * time.Second,
		readLimit:       4 << 20, // 4 MiB
		seen:            map[string]time.Time{},
		dedupWindow:     5 * time.Minute,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(*http.Request) bool { return true }, // 生产可加来源校验/mTLS
		},
	}
}

// ServeWS 处理 /ws/agent 连接。
func (h *Handler) ServeWS(c *gin.Context) {
	ws, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Warn("ws upgrade failed", zap.Error(err))
		return
	}

	// 1. 等待 register 消息并鉴权。
	ws.SetReadLimit(h.readLimit)
	_ = ws.SetReadDeadline(time.Now().Add(h.registerWait))
	mt, raw, err := ws.ReadMessage()
	if err != nil {
		_ = ws.Close()
		return
	}
	env, err := decodeEnvelope(raw)
	if err != nil || env.Type != MsgRegister {
		h.log.Warn("first message not register", zap.Error(err))
		_ = ws.Close()
		return
	}
	var reg RegisterData
	if err := env.UnmarshalData(&reg); err != nil {
		h.log.Warn("bad register data", zap.Error(err))
		_ = ws.Close()
		return
	}
	nd, err := h.nodes.Authenticate(c.Request.Context(), reg.Token)
	if err != nil {
		h.log.Warn("agent auth failed", zap.Error(err))
		_ = ws.Close()
		return
	}

	// 2. 绑定连接。
	conn := &Conn{NodeID: nd.ID, ws: ws, write: make(chan []byte, 64), done: make(chan struct{})}
	h.hub.Add(nd.ID, conn)
	h.log.Info("agent connected",
		zap.String("node_id", nd.ID.String()),
		zap.String("os", reg.HostInfo.OS),
		zap.String("arch", reg.HostInfo.Arch),
		zap.String("libc", reg.HostInfo.Libc),
	)

	// 更新注册信息。
	_ = h.nodes.MarkHeartbeat(c.Request.Context(), nd.ID, reg.AgentVersion, reg.SingboxVersion)

	// 3. 读循环。
	go conn.runWriter()
	defer func() {
		h.hub.Remove(nd.ID)
		conn.Close()
		h.log.Info("agent disconnected", zap.String("node_id", nd.ID.String()))
	}()

	_ = ws.SetReadDeadline(time.Time{}) // 取消注册期 deadline
	for {
		mt, raw, err = ws.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.TextMessage {
			continue
		}
		env, err := decodeEnvelope(raw)
		if err != nil {
			h.log.Warn("bad envelope", zap.Error(err))
			continue
		}
		if h.duplicate(env) {
			continue // §9.9：按 id 去重
		}
		h.dispatch(c.Request.Context(), nd.ID, env)
	}
}

// duplicate 判断消息 id 是否在去重窗口内已处理（简单 LRU 窗口）。
func (h *Handler) duplicate(env *Envelope) bool {
	if env.ID == "" {
		return false
	}
	h.seenMu.Lock()
	defer h.seenMu.Unlock()
	now := time.Now()
	if t, ok := h.seen[env.ID]; ok && now.Sub(t) < h.dedupWindow {
		return true
	}
	h.seen[env.ID] = now
	// 惰性清理过期 id。
	for id, t := range h.seen {
		if now.Sub(t) >= h.dedupWindow {
			delete(h.seen, id)
		}
	}
	return false
}

// dispatch 按类型分发消息。
func (h *Handler) dispatch(ctx context.Context, nodeID uuid.UUID, env *Envelope) {
	switch env.Type {
	case MsgHeartbeat:
		var d HeartbeatData
		if err := env.UnmarshalData(&d); err == nil {
			_ = h.nodes.MarkHeartbeat(ctx, nodeID, d.AgentVersion, d.SingboxVersion)
		}
	case MsgConfigApplyResult:
		var d ConfigApplyResultData
		if err := env.UnmarshalData(&d); err != nil {
			h.log.Warn("bad config_apply_result", zap.Error(err))
			return
		}
		h.log.Info("config apply result",
			zap.String("node_id", nodeID.String()),
			zap.Int("version", d.Version),
			zap.String("status", d.Status),
			zap.Bool("graceful", d.Graceful),
		)
		if h.onApplyResult != nil {
			h.onApplyResult(ApplyResult{
				NodeID:   nodeID,
				Version:  d.Version,
				Status:   d.Status,
				Graceful: d.Graceful,
				Error:    d.Error,
			})
		}
	case MsgTrafficReport:
		var d TrafficReportData
		if err := env.UnmarshalData(&d); err != nil {
			h.log.Warn("bad traffic_report", zap.Error(err))
			return
		}
		if h.onTrafficReport != nil {
			h.onTrafficReport(nodeID, d)
		}
	case MsgLog:
		h.log.Debug("agent log message", zap.String("node_id", nodeID.String()))
	default:
		h.log.Warn("unknown agent message type", zap.String("type", env.Type))
	}
}

func decodeEnvelope(raw []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Type == "" {
		return nil, errEmptyType{}
	}
	return &env, nil
}

type errEmptyType struct{}

func (errEmptyType) Error() string { return "agent: 消息缺少 type" }

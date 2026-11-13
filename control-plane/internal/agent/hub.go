package agent

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Conn 是一个已认证的 Agent 连接。
type Conn struct {
	NodeID uuid.UUID
	ws     *websocket.Conn
	write  chan []byte
	done   chan struct{}
	once   sync.Once
}

// send 投递一条已序列化消息（写协程消费）。
func (c *Conn) send(data []byte) error {
	select {
	case c.write <- data:
		return nil
	case <-c.done:
		return ErrConnClosed
	}
}

// Close 关闭连接。
func (c *Conn) Close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.ws.Close()
	})
}

// ErrConnClosed 表示连接已关闭。
var ErrConnClosed = errConnClosed{}

type errConnClosed struct{}

func (errConnClosed) Error() string { return "agent: connection closed" }

// Hub 是 Agent WebSocket 连接中心（nodeID → Conn 映射）。
type Hub struct {
	mu    sync.RWMutex
	conns map[uuid.UUID]*Conn
	log   *zap.Logger
}

// NewHub 构造 Hub。
func NewHub(log *zap.Logger) *Hub {
	return &Hub{conns: map[uuid.UUID]*Conn{}, log: log}
}

// Add 绑定节点连接；重复绑定会替换旧连接。
func (h *Hub) Add(nodeID uuid.UUID, c *Conn) {
	h.mu.Lock()
	if old, ok := h.conns[nodeID]; ok {
		old.Close()
	}
	h.conns[nodeID] = c
	h.mu.Unlock()
}

// Remove 解绑节点连接。
func (h *Hub) Remove(nodeID uuid.UUID) {
	h.mu.Lock()
	delete(h.conns, nodeID)
	h.mu.Unlock()
}

// SendToNode 向指定节点发送消息（节点离线返回错误）。
func (h *Hub) SendToNode(nodeID uuid.UUID, env *Envelope) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	h.mu.RLock()
	c, ok := h.conns[nodeID]
	h.mu.RUnlock()
	if !ok {
		return errNodeOffline{nodeID: nodeID}
	}
	return c.send(raw)
}

// OnlineNodes 返回当前在线节点。
func (h *Hub) OnlineNodes() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]uuid.UUID, 0, len(h.conns))
	for id := range h.conns {
		out = append(out, id)
	}
	return out
}

// IsOnline 判断节点是否在线。
func (h *Hub) IsOnline(nodeID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.conns[nodeID]
	return ok
}

type errNodeOffline struct{ nodeID uuid.UUID }

func (e errNodeOffline) Error() string { return "agent: 节点不在线 " + e.nodeID.String() }

// IsNodeOffline 判断错误是否为节点离线。
func IsNodeOffline(err error) bool {
	_, ok := err.(errNodeOffline)
	return ok
}

// writeTimeout 写超时。
const writeTimeout = 10 * time.Second

// runWriter 写协程。
func (c *Conn) runWriter() {
	for {
		select {
		case data := <-c.write:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
				c.Close()
				return
			}
		case <-c.done:
			return
		}
	}
}

// Package client 是 Agent 的 WebSocket 客户端：主动出站、令牌注册、心跳、断线重连。
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/singbox-panel/edge-agent/internal/config"
	"github.com/singbox-panel/edge-agent/internal/host"
)

// 消息类型（与控制面 agent 包保持一致）。
const (
	msgRegister          = "register"
	msgHeartbeat         = "heartbeat"
	msgConfigApplyResult = "config_apply_result"
	msgConfigPush        = "config_push"
	msgRestart           = "restart"
	msgPing              = "ping"
)

// Envelope 是统一消息信封（§9.9：id 幂等 + ts）。
type Envelope struct {
	Type string          `json:"type"`
	ID   string          `json:"id"`
	TS   int64           `json:"ts"`
	Data json.RawMessage `json:"data"`
}

// ApplyResult 是 config_apply_result 的 data。
type ApplyResult struct {
	Version  int    `json:"version"`
	Status   string `json:"status"`
	Graceful bool   `json:"graceful"`
	Error    string `json:"error,omitempty"`
}

// Handlers 是 client 对控制面下发指令的回调。
type Handlers struct {
	OnConfigPush func(version int, content []byte) ApplyResult
	OnRestart    func() error
}

// Client 是 Agent WS 客户端。
type Client struct {
	cfg          *config.Config
	log          *zap.Logger
	hostInfo     host.Info
	handlers     Handlers
	token        string
	agentVersion string

	mu       sync.Mutex
	conn     *websocket.Conn
	done     chan struct{}
	stopOnce sync.Once
}

// New 构造客户端。
func New(cfg *config.Config, log *zap.Logger, hostInfo host.Info, handlers Handlers, token string, agentVersion string) *Client {
	return &Client{
		cfg:          cfg,
		log:          log,
		hostInfo:     hostInfo,
		handlers:     handlers,
		token:        token,
		agentVersion: agentVersion,
		done:         make(chan struct{}),
	}
}

// Run 阻塞运行：连接 → 注册 → 心跳/读循环，断线指数退避重连。
func (c *Client) Run(ctx context.Context) error {
	backoff := c.cfg.Control.RetryMin
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return nil
		default:
		}

		err := c.session(ctx)
		if err == nil {
			return nil // 正常退出
		}
		c.log.Warn("ws session 断开，准备重连", zap.Error(err), zap.Duration("backoff", backoff))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return nil
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > c.cfg.Control.RetryMax {
			backoff = c.cfg.Control.RetryMax
		}
	}
}

// Stop 停止客户端。
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		close(c.done)
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
		}
		c.mu.Unlock()
	})
}

// session 建立一次连接并运行注册/心跳/读循环，返回后连接已关闭。
func (c *Client) session(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.cfg.Control.ServerURL, nil)
	if err != nil {
		return fmt.Errorf("client: dial: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		_ = conn.Close()
	}()

	// 1. 注册。
	if err := c.sendRegister(); err != nil {
		return err
	}

	// 2. 心跳 goroutine。
	hbCtx, cancelHB := context.WithCancel(ctx)
	defer cancelHB()
	go c.heartbeatLoop(hbCtx)

	// 3. 读循环。
	for {
		mt, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("client: read: %w", err)
		}
		if mt != websocket.TextMessage {
			continue
		}
		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			c.log.Warn("bad envelope", zap.Error(err))
			continue
		}
		c.dispatch(ctx, env)
	}
}

func (c *Client) sendRegister() error {
	version := c.cfg.Agent.Kernel.Version
	if version == "" || version == "auto" {
		version = "unknown"
	}
	return c.send(Envelope{
		Type: msgRegister,
		ID:   uuid.New().String(),
		TS:   time.Now().Unix(),
		Data: mustJSON(map[string]any{
			"token":           c.token,
			"hostinfo":        c.hostInfo,
			"agent_version":   c.agentVersion,
			"singbox_version": version,
		}),
	})
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	interval := c.cfg.Control.Heartbeat
	if interval <= 0 {
		interval = 5 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-t.C:
			_ = c.send(Envelope{
				Type: msgHeartbeat,
				ID:   uuid.New().String(),
				TS:   time.Now().Unix(),
				Data: mustJSON(map[string]any{
					"agent_version":   c.agentVersion,
					"singbox_version": c.cfg.Agent.Kernel.Version,
				}),
			})
		}
	}
}

func (c *Client) dispatch(ctx context.Context, env Envelope) {
	switch env.Type {
	case msgConfigPush:
		var d struct {
			Version int    `json:"version"`
			Content []byte `json:"content"`
		}
		if err := json.Unmarshal(env.Data, &d); err != nil {
			c.log.Warn("bad config_push", zap.Error(err))
			return
		}
		c.log.Info("收到 config_push", zap.Int("version", d.Version))
		var res ApplyResult
		if c.handlers.OnConfigPush != nil {
			res = c.handlers.OnConfigPush(d.Version, d.Content)
		} else {
			res = ApplyResult{Version: d.Version, Status: "failed", Error: "未配置 config_push 处理器"}
		}
		res.Version = d.Version
		_ = c.sendResult(res)
	case msgRestart:
		c.log.Info("收到 restart")
		if c.handlers.OnRestart != nil {
			if err := c.handlers.OnRestart(); err != nil {
				c.log.Warn("restart 失败", zap.Error(err))
			}
		}
	case msgPing:
		// 保活探测：立即回一个心跳。
		_ = c.send(Envelope{
			Type: msgHeartbeat,
			ID:   uuid.New().String(),
			TS:   time.Now().Unix(),
			Data: mustJSON(map[string]any{"agent_version": c.agentVersion}),
		})
	default:
		c.log.Debug("未知消息类型", zap.String("type", env.Type))
	}
}

func (c *Client) sendResult(res ApplyResult) error {
	return c.send(Envelope{
		Type: msgConfigApplyResult,
		ID:   uuid.New().String(),
		TS:   time.Now().Unix(),
		Data: mustJSON(res),
	})
}

// send 线程安全写。
func (c *Client) send(env Envelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("client: 未连接")
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, raw)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

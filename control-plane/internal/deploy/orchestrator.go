// Package deploy 负责配置下发编排：生成版本 → config_push → 等待 apply 结果 → 状态机。
package deploy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/singbox-panel/control-plane/internal/agent"
	"github.com/singbox-panel/control-plane/internal/configsvc"
	"github.com/singbox-panel/control-plane/internal/model"
)

// Orchestrator 下发编排器。
type Orchestrator struct {
	cfgSvc  *configsvc.Service
	hub     *agent.Hub
	log     *zap.Logger
	timeout time.Duration

	mu      sync.Mutex
	pending map[string]chan agent.ApplyResult // key: nodeID:version
}

// NewOrchestrator 构造编排器。
func NewOrchestrator(cfgSvc *configsvc.Service, hub *agent.Hub, log *zap.Logger, timeout time.Duration) *Orchestrator {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Orchestrator{
		cfgSvc:  cfgSvc,
		hub:     hub,
		log:     log,
		timeout: timeout,
		pending: map[string]chan agent.ApplyResult{},
	}
}

// Deploy 生成新版本并下发到节点（同步等待 apply 结果）。
func (o *Orchestrator) Deploy(ctx context.Context, nodeID uuid.UUID, operatorID uuid.UUID) (*model.ConfigVersion, error) {
	v, err := o.cfgSvc.CreateVersion(ctx, nodeID, operatorID)
	if err != nil {
		return nil, fmt.Errorf("deploy: 生成版本失败: %w", err)
	}
	if err := o.pushVersion(ctx, nodeID, v); err != nil {
		return v, err
	}
	return v, nil
}

// Retry 重新下发一个已存在的版本（draft/failed/rolled_back 均可重试）。
func (o *Orchestrator) Retry(ctx context.Context, nodeID uuid.UUID, versionID uuid.UUID) (*model.ConfigVersion, error) {
	v, err := o.cfgSvc.GetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if v.NodeID != nodeID {
		return nil, fmt.Errorf("deploy: 版本不属于该节点")
	}
	if err := o.pushVersion(ctx, nodeID, v); err != nil {
		return v, err
	}
	return v, nil
}

// Rollback 回滚到指定历史版本：复制快照为新版本并下发。
func (o *Orchestrator) Rollback(ctx context.Context, nodeID uuid.UUID, targetVersionID uuid.UUID, operatorID uuid.UUID) (*model.ConfigVersion, error) {
	v, err := o.cfgSvc.Rollback(ctx, nodeID, targetVersionID, operatorID)
	if err != nil {
		return nil, err
	}
	if err := o.pushVersion(ctx, nodeID, v); err != nil {
		return v, err
	}
	return v, nil
}

// pushVersion 下发单个版本：状态机 deploying → 推送 → 等待结果 → 更新状态。
func (o *Orchestrator) pushVersion(ctx context.Context, nodeID uuid.UUID, v *model.ConfigVersion) error {
	if err := o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionDeploying); err != nil {
		return err
	}

	content, err := o.cfgSvc.DecryptVersionContent(v)
	if err != nil {
		_ = o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionFailed)
		return err
	}

	// 证书文件先于 config_push 下发：config.json 引用的相对路径必须已存在，
	// 否则 sing-box check 会因找不到 certificate_path/key_path 失败。
	files, err := o.cfgSvc.NodeCertFiles(ctx, nodeID)
	if err != nil {
		_ = o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionFailed)
		return err
	}
	if len(files) > 0 {
		payloads := make([]agent.FilePayload, 0, len(files))
		for _, f := range files {
			payloads = append(payloads, agent.FilePayload{Path: f.Path, Content: f.Content, Mode: f.Mode})
		}
		fenv, err := agent.NewEnvelope(agent.MsgFilePush, uuid.New().String(), agent.FilePushData{Files: payloads})
		if err != nil {
			_ = o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionFailed)
			return err
		}
		if err := o.hub.SendToNode(nodeID, fenv); err != nil {
			_ = o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionFailed)
			return fmt.Errorf("deploy: 证书下发失败: %w", err)
		}
	}

	ch := o.register(nodeID, v.Version)
	defer o.unregister(nodeID, v.Version)

	env, err := agent.NewEnvelope(agent.MsgConfigPush, uuid.New().String(), agent.ConfigPushData{
		Version: v.Version,
		Content: content,
	})
	if err != nil {
		_ = o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionFailed)
		return err
	}
	if err := o.hub.SendToNode(nodeID, env); err != nil {
		// 节点离线，标记失败（可重试）。
		_ = o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionFailed)
		return fmt.Errorf("deploy: 推送失败: %w", err)
	}

	select {
	case res := <-ch:
		return o.applyResultToStatus(ctx, v.ID, res)
	case <-ctx.Done():
		_ = o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionFailed)
		return ctx.Err()
	case <-time.After(o.timeout):
		_ = o.cfgSvc.UpdateVersionStatus(ctx, v.ID, model.ConfigVersionFailed)
		return fmt.Errorf("deploy: 等待 apply 结果超时")
	}
}

func (o *Orchestrator) applyResultToStatus(ctx context.Context, versionID uuid.UUID, res agent.ApplyResult) error {
	status := model.ConfigVersionFailed
	switch res.Status {
	case agent.ApplyStatusSuccess:
		status = model.ConfigVersionSuccess
	case agent.ApplyStatusRolledBack:
		status = model.ConfigVersionRolledBack
	case agent.ApplyStatusFailed:
		status = model.ConfigVersionFailed
	}
	if err := o.cfgSvc.UpdateVersionStatus(ctx, versionID, status); err != nil {
		return err
	}
	if status == model.ConfigVersionFailed || status == model.ConfigVersionRolledBack {
		return fmt.Errorf("deploy: apply 结果 %s: %s", res.Status, res.Error)
	}
	return nil
}

// NotifyApplyResult 由 agent handler 回调，唤醒等待中的 pushVersion。
func (o *Orchestrator) NotifyApplyResult(res agent.ApplyResult) {
	o.mu.Lock()
	ch, ok := o.pending[fmt.Sprintf("%s:%d", res.NodeID.String(), res.Version)]
	o.mu.Unlock()
	if ok {
		select {
		case ch <- res:
		default:
		}
	}
}

func (o *Orchestrator) register(nodeID uuid.UUID, version int) chan agent.ApplyResult {
	key := fmt.Sprintf("%s:%d", nodeID.String(), version)
	ch := make(chan agent.ApplyResult, 1)
	o.mu.Lock()
	o.pending[key] = ch
	o.mu.Unlock()
	return ch
}

func (o *Orchestrator) unregister(nodeID uuid.UUID, version int) {
	key := fmt.Sprintf("%s:%d", nodeID.String(), version)
	o.mu.Lock()
	delete(o.pending, key)
	o.mu.Unlock()
}

package renderer

import (
	"fmt"
	"sort"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]ProtocolRenderer{}
)

// Register 由各渲染器在 init() 中调用；重复注册同协议直接 panic（启动期暴露错误）。
func Register(r ProtocolRenderer) {
	registryMu.Lock()
	defer registryMu.Unlock()
	p := r.Protocol()
	if p == "" {
		panic("renderer: protocol name must not be empty")
	}
	if _, dup := registry[p]; dup {
		panic(fmt.Sprintf("renderer: protocol %q already registered", p))
	}
	registry[p] = r
}

// Get 按 type 查表；Generator 只依赖此函数，绝不硬编码协议名。
func Get(proto string) (ProtocolRenderer, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	r, ok := registry[proto]
	return r, ok
}

// List 返回已注册协议（排序，供前端枚举 / 校验白名单使用）。
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for p := range registry {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Metas 返回全部已注册协议的元数据（排序，供 GET /protocols 下发）。
func Metas() []ProtocolMeta {
	names := List()
	out := make([]ProtocolMeta, 0, len(names))
	for _, n := range names {
		if r, ok := Get(n); ok {
			out = append(out, r.Meta())
		}
	}
	return out
}

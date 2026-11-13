package renderer

import (
	"encoding/json"
	"fmt"
)

// transportMap 解析 transport JSONB 为 map；无则返回 nil。
func transportMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return parseMap(raw)
}

// renderTransport 渲染 transport（透传）；nil 表示不输出 transport 字段。
func renderTransport(raw json.RawMessage) (any, error) {
	m, err := transportMap(raw)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	// 传输层必须声明 type，否则 sing-box 无法识别。
	if m[fType] == nil || m[fType] == "" {
		return nil, fmt.Errorf("transport: 缺少 type 字段")
	}
	return m, nil
}

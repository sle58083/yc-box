package configsvc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Validator 是第二层校验：对生成后的 sing-box JSON 按协议片段做 Schema 校验
// （system_design.md §5.5 第②层；最终权威是 T03 Agent 的 sing-box check -c）。
type Validator struct {
	compiler *jsonschema.Compiler
}

// NewValidator 从 schemaDir 加载全部 *.json 协议片段。
// 片段命名约定：inbound-<type>.json / outbound-<type>.json。
func NewValidator(schemaDir string) (*Validator, error) {
	c := jsonschema.NewCompiler()
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil, fmt.Errorf("schema: read dir %s: %w", schemaDir, err)
	}
	count := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		f, err := os.Open(filepath.Join(schemaDir, name))
		if err != nil {
			return nil, fmt.Errorf("schema: open %s: %w", name, err)
		}
		if err := c.AddResource(name, f); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("schema: add %s: %w", name, err)
		}
		_ = f.Close()
		count++
	}
	if count == 0 {
		return nil, fmt.Errorf("schema: %s 下无 schema 文件", schemaDir)
	}
	return &Validator{compiler: c}, nil
}

// ValidateDocument 校验整份文档：逐元素对照 inbound-<type>.json / outbound-<type>.json。
func (v *Validator) ValidateDocument(doc map[string]any) error {
	// 归一化为标准 JSON 值（map[string]interface{} / []interface{}）。
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("schema: marshal doc: %w", err)
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("schema: unmarshal doc: %w", err)
	}
	m, ok := root.(map[string]any)
	if !ok {
		return fmt.Errorf("schema: 文档必须是对象")
	}

	if arr, ok := m[kInbounds].([]any); ok {
		for i, el := range arr {
			if err := v.validateElement(el, "inbound", i); err != nil {
				return err
			}
		}
	}
	if arr, ok := m[kOutbounds].([]any); ok {
		for i, el := range arr {
			if err := v.validateElement(el, "outbound", i); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateElement 校验单个 inbound/outbound 元素。
func (v *Validator) validateElement(el any, kind string, idx int) error {
	em, ok := el.(map[string]any)
	if !ok {
		return fmt.Errorf("schema: %ss[%d] 必须是对象", kind, idx)
	}
	typ, _ := em["type"].(string)
	if typ == "" {
		return fmt.Errorf("schema: %ss[%d] 缺少 type", kind, idx)
	}
	url := fmt.Sprintf("%s-%s.json", kind, typ)
	sch, err := v.compiler.Compile(url)
	if err != nil {
		return fmt.Errorf("schema: %ss[%d] 协议 %q 无 schema 片段: %w", kind, idx, typ, err)
	}
	if err := sch.Validate(em); err != nil {
		return fmt.Errorf("schema: %ss[%d](%s) 校验失败: %w", kind, idx, typ, err)
	}
	return nil
}

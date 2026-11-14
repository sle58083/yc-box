package statsproto

// 独立交叉验证：用手写的编解码与 google.golang.org/protobuf 的 protowire
// 生成的真实 proto3 wire 字节逐一比对，证明线上格式与 protoc 生成代码完全一致。

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

// protowire 编码 QueryStatsRequest（等价 protoc 生成的 Marshal）。
func protowireReq(pattern string, reset bool) []byte {
	var b []byte
	if pattern != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, pattern)
	}
	if reset {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	return b
}

func protowireStat(name string, value int64) []byte {
	var b []byte
	if name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, name)
	}
	if value != 0 {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(value))
	}
	return b
}

func protowireResp(stats []Stat) []byte {
	var b []byte
	for _, s := range stats {
		inner := protowireStat(s.Name, s.Value)
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, inner)
	}
	return b
}

func TestCrossCheckRequestMarshal(t *testing.T) {
	cases := []QueryStatsRequest{
		{Pattern: "", Reset: true},
		{Pattern: "prefix", Reset: false},
		{Pattern: "u>>>x", Reset: true},
	}
	for _, c := range cases {
		got, _ := c.Marshal()
		want := protowireReq(c.Pattern, c.Reset)
		if !bytes.Equal(got, want) {
			t.Fatalf("QueryStatsRequest{%q,%v} Marshal: got %x want %x", c.Pattern, c.Reset, got, want)
		}
	}
}

func TestCrossCheckStatMarshal(t *testing.T) {
	cases := []Stat{
		{Name: "a", Value: 0},
		{Name: "a", Value: 1},
		{Name: "a", Value: -1},
		{Name: "user>>>u1>>>traffic>>>uplink", Value: 9223372036854775807},
		{Name: "", Value: 5},
	}
	for _, c := range cases {
		got, _ := c.Marshal()
		want := protowireStat(c.Name, c.Value)
		if !bytes.Equal(got, want) {
			t.Fatalf("Stat{%q,%d} Marshal: got %x want %x", c.Name, c.Value, got, want)
		}
	}
}

func TestCrossCheckResponseMarshal(t *testing.T) {
	resp := QueryStatsResponse{Stat: []Stat{
		{Name: "inbound>>>in>>>traffic>>>uplink", Value: 100},
		{Name: "inbound>>>in>>>traffic>>>downlink", Value: 0},
		{Name: "user>>>u1>>>traffic>>>uplink", Value: -42},
	}}
	got, _ := resp.Marshal()
	want := protowireResp(resp.Stat)
	if !bytes.Equal(got, want) {
		t.Fatalf("QueryStatsResponse Marshal mismatch:\n got %x\nwant %x", got, want)
	}
}

// 反方向：用真实 protobuf 字节（含未知字段、负数、空响应）喂给手写 Unmarshal。
func TestCrossCheckResponseUnmarshalFromRealWire(t *testing.T) {
	// 构造含未知字段（field 99 varint，模拟未来新增字段）的响应。
	var b []byte
	inner := protowireStat("user>>>u1>>>traffic>>>uplink", 777)
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendBytes(b, inner)
	// 未知字段 field 99 (varint) —— 真实 protobuf 客户端应跳过。
	b = protowire.AppendTag(b, 99, protowire.VarintType)
	b = protowire.AppendVarint(b, 12345)

	var got QueryStatsResponse
	if err := got.Unmarshal(b); err != nil {
		t.Fatalf("Unmarshal 含未知字段响应失败: %v", err)
	}
	if len(got.Stat) != 1 || got.Stat[0].Name != "user>>>u1>>>traffic>>>uplink" || got.Stat[0].Value != 777 {
		t.Fatalf("Unmarshal 结果错误: %+v", got.Stat)
	}

	// 空响应。
	var empty QueryStatsResponse
	if err := empty.Unmarshal(nil); err != nil {
		t.Fatalf("空响应 Unmarshal 失败: %v", err)
	}
	if len(empty.Stat) != 0 {
		t.Fatalf("空响应应得到 0 条 stat，got %d", len(empty.Stat))
	}
}

func TestCrossCheckRequestUnmarshalFromRealWire(t *testing.T) {
	// reset=false 时 protobuf 不会序列化该字段（默认值省略）。
	raw := protowireReq("abc", false)
	var req QueryStatsRequest
	if err := req.Unmarshal(raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if req.Pattern != "abc" || req.Reset {
		t.Fatalf("got %+v", req)
	}
}

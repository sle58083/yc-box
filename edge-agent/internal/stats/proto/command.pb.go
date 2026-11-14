// Package statsproto 是 v2ray stats 的最小化 proto 契约（手写实现）。
//
// sing-box 的 experimental.v2ray_api 暴露的是 gRPC 接口（并非 HTTP），
// 方法路径为 /v2ray.core.app.stats.command.StatsService/QueryStats。
// 为不引入巨型 v2ray-core 依赖，本包只保留 QueryStats 所需的三个 message
// 与一个 service，并用手写的 proto3 wire 编解码 + gRPC Codec 实现，保证与
// protoc 生成的代码在线上格式上完全兼容。
//
// 本目录同时保留了 command.proto，待 CI/本地具备 protoc 后可用
// protoc-gen-go / protoc-gen-go-grpc 重新生成等价文件（go_package 已冻结）。
package statsproto

import "fmt"

// QueryStatsRequest 对应 message QueryStatsRequest。
type QueryStatsRequest struct {
	Pattern string
	Reset   bool
}

// Stat 对应 message Stat（单个计数器）。
type Stat struct {
	Name  string
	Value int64
}

// QueryStatsResponse 对应 message QueryStatsResponse。
type QueryStatsResponse struct {
	Stat []Stat
}

// ============================ proto3 wire 编解码 ============================

// proto3 wire type 常量。
const (
	wireVarint  = 0 // bool/int64 等变长整数
	wireFixed64 = 1
	wireBytes   = 2 // string / 嵌套 message
	wireFixed32 = 5
)

// appendUvarint 追加一个 LEB128 无符号变长整数。
func appendUvarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

// appendTag 追加字段 tag（field_number<<3 | wire_type）。
func appendTag(dst []byte, field int, wire int) []byte {
	return appendUvarint(dst, uint64(field<<3|wire))
}

// consumeUvarint 读取一个 LEB128 变长整数，返回 (值, 消费字节数)。
// 返回 n<=0 表示数据非法/不足。
func consumeUvarint(buf []byte) (uint64, int) {
	var v uint64
	for i := 0; i < len(buf) && i < 10; i++ {
		b := buf[i]
		v |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return v, i + 1
		}
	}
	return 0, 0
}

// skipField 跳过未知字段，返回剩余字节。
func skipField(wire int, data []byte) ([]byte, error) {
	switch wire {
	case wireVarint:
		_, n := consumeUvarint(data)
		if n <= 0 {
			return nil, fmt.Errorf("statsproto: 非法 varint")
		}
		return data[n:], nil
	case wireFixed64:
		if len(data) < 8 {
			return nil, fmt.Errorf("statsproto: fixed64 数据不足")
		}
		return data[8:], nil
	case wireBytes:
		l, n := consumeUvarint(data)
		if n <= 0 || uint64(len(data[n:])) < l {
			return nil, fmt.Errorf("statsproto: bytes 长度非法")
		}
		data = data[n:]
		return data[l:], nil
	case wireFixed32:
		if len(data) < 4 {
			return nil, fmt.Errorf("statsproto: fixed32 数据不足")
		}
		return data[4:], nil
	default:
		return nil, fmt.Errorf("statsproto: 未知 wire 类型 %d", wire)
	}
}

// Marshal 序列化 QueryStatsRequest。
func (m *QueryStatsRequest) Marshal() ([]byte, error) {
	var buf []byte
	if m.Pattern != "" {
		buf = appendTag(buf, 1, wireBytes)
		buf = appendUvarint(buf, uint64(len(m.Pattern)))
		buf = append(buf, m.Pattern...)
	}
	if m.Reset {
		buf = appendTag(buf, 2, wireVarint)
		buf = appendUvarint(buf, 1)
	}
	return buf, nil
}

// Unmarshal 反序列化 QueryStatsRequest。
func (m *QueryStatsRequest) Unmarshal(data []byte) error {
	for len(data) > 0 {
		tag, n := consumeUvarint(data)
		if n <= 0 {
			return fmt.Errorf("statsproto: 非法 tag")
		}
		data = data[n:]
		field, wire := int(tag>>3), int(tag&7)

		switch field {
		case 1: // string pattern
			if wire != wireBytes {
				return fmt.Errorf("statsproto: pattern 字段 wire 类型错误")
			}
			l, n := consumeUvarint(data)
			if n <= 0 || uint64(len(data[n:])) < l {
				return fmt.Errorf("statsproto: pattern 长度非法")
			}
			data = data[n:]
			m.Pattern = string(data[:l])
			data = data[l:]
		case 2: // bool reset
			if wire != wireVarint {
				return fmt.Errorf("statsproto: reset 字段 wire 类型错误")
			}
			v, n := consumeUvarint(data)
			if n <= 0 {
				return fmt.Errorf("statsproto: reset 值非法")
			}
			data = data[n:]
			m.Reset = v != 0
		default:
			rest, err := skipField(wire, data)
			if err != nil {
				return err
			}
			data = rest
		}
	}
	return nil
}

// Marshal 序列化 Stat。
func (m *Stat) Marshal() ([]byte, error) {
	var buf []byte
	if m.Name != "" {
		buf = appendTag(buf, 1, wireBytes)
		buf = appendUvarint(buf, uint64(len(m.Name)))
		buf = append(buf, m.Name...)
	}
	if m.Value != 0 {
		buf = appendTag(buf, 2, wireVarint)
		// int64 负数按 protobuf 规则以 10 字节补码 varint 编码；uint64 强转
		// 后 LEB128 输出即为其正确 wire 表示。
		buf = appendUvarint(buf, uint64(m.Value))
	}
	return buf, nil
}

// Unmarshal 反序列化 Stat。
func (m *Stat) Unmarshal(data []byte) error {
	for len(data) > 0 {
		tag, n := consumeUvarint(data)
		if n <= 0 {
			return fmt.Errorf("statsproto: 非法 tag")
		}
		data = data[n:]
		field, wire := int(tag>>3), int(tag&7)

		switch field {
		case 1: // string name
			if wire != wireBytes {
				return fmt.Errorf("statsproto: name 字段 wire 类型错误")
			}
			l, n := consumeUvarint(data)
			if n <= 0 || uint64(len(data[n:])) < l {
				return fmt.Errorf("statsproto: name 长度非法")
			}
			data = data[n:]
			m.Name = string(data[:l])
			data = data[l:]
		case 2: // int64 value
			if wire != wireVarint {
				return fmt.Errorf("statsproto: value 字段 wire 类型错误")
			}
			v, n := consumeUvarint(data)
			if n <= 0 {
				return fmt.Errorf("statsproto: value 值非法")
			}
			data = data[n:]
			m.Value = int64(v)
		default:
			rest, err := skipField(wire, data)
			if err != nil {
				return err
			}
			data = rest
		}
	}
	return nil
}

// Marshal 序列化 QueryStatsResponse。
func (m *QueryStatsResponse) Marshal() ([]byte, error) {
	var buf []byte
	for i := range m.Stat {
		msg, err := m.Stat[i].Marshal()
		if err != nil {
			return nil, err
		}
		buf = appendTag(buf, 1, wireBytes)
		buf = appendUvarint(buf, uint64(len(msg)))
		buf = append(buf, msg...)
	}
	return buf, nil
}

// Unmarshal 反序列化 QueryStatsResponse。
func (m *QueryStatsResponse) Unmarshal(data []byte) error {
	m.Stat = m.Stat[:0]
	for len(data) > 0 {
		tag, n := consumeUvarint(data)
		if n <= 0 {
			return fmt.Errorf("statsproto: 非法 tag")
		}
		data = data[n:]
		field, wire := int(tag>>3), int(tag&7)

		if field == 1 { // repeated Stat stat
			if wire != wireBytes {
				return fmt.Errorf("statsproto: stat 字段 wire 类型错误")
			}
			l, n := consumeUvarint(data)
			if n <= 0 || uint64(len(data[n:])) < l {
				return fmt.Errorf("statsproto: stat 长度非法")
			}
			data = data[n:]
			var st Stat
			if err := st.Unmarshal(data[:l]); err != nil {
				return err
			}
			data = data[l:]
			m.Stat = append(m.Stat, st)
			continue
		}

		rest, err := skipField(wire, data)
		if err != nil {
			return err
		}
		data = rest
	}
	return nil
}

// Codec 是 gRPC 使用的 proto codec（线上格式与 protobuf 完全一致）。
// Name 固定返回 "proto"，使 content-type 为 application/grpc+proto。
type Codec struct{}

// Name 返回 codec 名称。
func (Codec) Name() string { return "proto" }

// Marshal 按消息类型分发序列化。
func (Codec) Marshal(v any) ([]byte, error) {
	switch m := v.(type) {
	case *QueryStatsRequest:
		return m.Marshal()
	case *QueryStatsResponse:
		return m.Marshal()
	default:
		return nil, fmt.Errorf("statsproto: 不支持 marshal 类型 %T", v)
	}
}

// Unmarshal 按消息类型分发反序列化。
func (Codec) Unmarshal(data []byte, v any) error {
	switch m := v.(type) {
	case *QueryStatsRequest:
		return m.Unmarshal(data)
	case *QueryStatsResponse:
		return m.Unmarshal(data)
	default:
		return fmt.Errorf("statsproto: 不支持 unmarshal 类型 %T", v)
	}
}

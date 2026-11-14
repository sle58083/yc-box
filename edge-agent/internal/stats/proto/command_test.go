package statsproto

import "testing"

func TestQueryStatsRequestRoundTrip(t *testing.T) {
	req := &QueryStatsRequest{Pattern: "", Reset: true}
	b, err := req.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// reset=true 且 pattern 为空时，wire 应为 field2(varint,true) = {0x10, 0x01}。
	if len(b) != 2 || b[0] != 0x10 || b[1] != 0x01 {
		t.Fatalf("wire 编码不符合预期: % x", b)
	}

	var got QueryStatsRequest
	if err := got.Unmarshal(b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Pattern != "" || !got.Reset {
		t.Fatalf("roundtrip: got %+v", got)
	}
}

func TestQueryStatsRequestPatternRoundTrip(t *testing.T) {
	req := &QueryStatsRequest{Pattern: "prefix", Reset: false}
	b, err := req.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got QueryStatsRequest
	if err := got.Unmarshal(b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Pattern != "prefix" || got.Reset {
		t.Fatalf("roundtrip: got %+v", got)
	}
}

func TestQueryStatsResponseRoundTrip(t *testing.T) {
	resp := &QueryStatsResponse{Stat: []Stat{
		{Name: "user>>>u1>>>traffic>>>uplink", Value: 123},
		{Name: "inbound>>>in>>>traffic>>>downlink", Value: -1},
	}}
	b, err := resp.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got QueryStatsResponse
	if err := got.Unmarshal(b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Stat) != 2 {
		t.Fatalf("stat 数量: got %d", len(got.Stat))
	}
	if got.Stat[0].Name != resp.Stat[0].Name || got.Stat[0].Value != 123 {
		t.Fatalf("stat[0]: got %+v", got.Stat[0])
	}
	if got.Stat[1].Name != resp.Stat[1].Name || got.Stat[1].Value != -1 {
		t.Fatalf("stat[1]: got %+v", got.Stat[1])
	}
}

func TestCodec(t *testing.T) {
	c := Codec{}
	if c.Name() != "proto" {
		t.Fatalf("codec name: %s", c.Name())
	}
	b, err := c.Marshal(&QueryStatsRequest{Reset: true})
	if err != nil {
		t.Fatalf("codec marshal: %v", err)
	}
	var out QueryStatsRequest
	if err := c.Unmarshal(b, &out); err != nil {
		t.Fatalf("codec unmarshal: %v", err)
	}
	if !out.Reset {
		t.Fatalf("codec roundtrip 失败")
	}
}

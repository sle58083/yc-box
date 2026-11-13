package agent

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	env, err := NewEnvelope(MsgHeartbeat, "id-1", HeartbeatData{AgentVersion: "dev"})
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}
	if env.Type != MsgHeartbeat || env.ID != "id-1" {
		t.Fatalf("信封字段错误: %+v", env)
	}
	if env.TS <= 0 {
		t.Fatalf("ts 必须存在且 >0")
	}

	raw, _ := json.Marshal(env)
	var decoded Envelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Type != MsgHeartbeat || decoded.ID != "id-1" || decoded.TS != env.TS {
		t.Fatalf("往返后字段不一致: %+v", decoded)
	}
	var hb HeartbeatData
	if err := decoded.UnmarshalData(&hb); err != nil {
		t.Fatalf("UnmarshalData: %v", err)
	}
	if hb.AgentVersion != "dev" {
		t.Fatalf("data 解析错误: %+v", hb)
	}
}

func TestConfigApplyResultData(t *testing.T) {
	env, _ := NewEnvelope(MsgConfigApplyResult, "id-2", ConfigApplyResultData{Version: 3, Status: "success", Graceful: true})
	var d ConfigApplyResultData
	if err := env.UnmarshalData(&d); err != nil {
		t.Fatalf("UnmarshalData: %v", err)
	}
	if d.Version != 3 || d.Status != "success" || !d.Graceful {
		t.Fatalf("config_apply_result 解析错误: %+v", d)
	}
}

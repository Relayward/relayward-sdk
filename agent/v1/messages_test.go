package agentv1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Relayward/relayward-sdk/contract"
	"github.com/Relayward/relayward-sdk/protocol"
)

func TestAgentHelloEnvelope(t *testing.T) {
	value, err := NewEnvelope(MessageAgentHello, AgentHello{
		NodeID: "123e4567-e89b-42d3-a456-426614174000", AgentVersion: "0.1.0",
		StartedAt:    time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC),
		Capabilities: []string{"control.heartbeat"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	if value.APIVersion != contract.AgentAPIVersion {
		t.Fatalf("api_version = %q", value.APIVersion)
	}
	if err := ValidateEnvelope(value); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}
}

func TestValidateEnvelopeRejectsUnknownTypeAndPayloadField(t *testing.T) {
	value, err := protocol.NewEnvelopeFor(APIVersion, "agent.unknown", map[string]bool{"ok": true})
	if err != nil {
		t.Fatalf("NewEnvelopeFor() error = %v", err)
	}
	if err := ValidateEnvelope(value); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("ValidateEnvelope() unknown type error = %v", err)
	}

	value.Type = MessageAgentHeartbeat
	value.Payload = json.RawMessage(`{
  "session_id":"0123456789abcdef0123456789abcdef",
  "agent_version":"0.1.0",
  "observed_at":"2026-08-02T08:00:00Z",
  "other":true
}`)
	if err := ValidateEnvelope(value); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("ValidateEnvelope() payload error = %v", err)
	}
}

func TestCenterHelloHeartbeatBounds(t *testing.T) {
	value := CenterHello{
		SessionID:                "0123456789abcdef0123456789abcdef",
		HeartbeatIntervalSeconds: int(DefaultHeartbeatInterval.Seconds()),
		ServerTime:               time.Now().UTC(),
	}
	if err := ValidateCenterHello(value); err != nil {
		t.Fatalf("ValidateCenterHello() error = %v", err)
	}
	value.HeartbeatIntervalSeconds = 1
	if err := ValidateCenterHello(value); err == nil {
		t.Fatal("ValidateCenterHello() accepted a one-second heartbeat")
	}
}

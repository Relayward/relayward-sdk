package agentv1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAgentUpdateCommandRoundTrip(t *testing.T) {
	now := time.Date(2026, time.August, 2, 10, 0, 0, 0, time.UTC)
	command, err := NewAgentUpdateCommand("0.2.0", now, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("NewAgentUpdateCommand() error = %v", err)
	}
	payload, err := DecodeAgentUpdateCommand(command)
	if err != nil || payload.Version != "0.2.0" {
		t.Fatalf("DecodeAgentUpdateCommand() = %+v, %v", payload, err)
	}
	command.Payload = json.RawMessage(`{"version":"0.2.0","unexpected":true}`)
	if _, err := DecodeAgentUpdateCommand(command); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodeAgentUpdateCommand() unknown field error = %v", err)
	}
}

func TestAgentUpdateRejectsInvalidVersions(t *testing.T) {
	for _, version := range []string{"", "v1.2.3", "1.2", "1.2.3-01", strings.Repeat("1", 129)} {
		if err := ValidateAgentUpdateCommand(AgentUpdateCommand{Version: version}); err == nil {
			t.Fatalf("ValidateAgentUpdateCommand(%q) succeeded", version)
		}
	}
}

func TestAgentUpdateOutputRoundTrip(t *testing.T) {
	raw, err := EncodeAgentUpdateOutput(AgentUpdateOutput{Version: "0.2.0", State: AgentUpdateStateActivated})
	if err != nil {
		t.Fatalf("EncodeAgentUpdateOutput() error = %v", err)
	}
	value, err := DecodeAgentUpdateOutput(raw)
	if err != nil || value.Version != "0.2.0" || value.State != AgentUpdateStateActivated {
		t.Fatalf("DecodeAgentUpdateOutput() = %+v, %v", value, err)
	}
}

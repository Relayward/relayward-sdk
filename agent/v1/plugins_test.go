package agentv1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPluginReconcileCommandRoundTrip(t *testing.T) {
	now := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
	request := testPluginReconcileCommand()
	command, err := NewPluginReconcileCommand(request, now, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("NewPluginReconcileCommand() error = %v", err)
	}
	decoded, err := DecodePluginReconcileCommand(command)
	if err != nil || decoded.PluginID != request.PluginID || decoded.Generation != request.Generation || decoded.Artifact == nil || decoded.Artifact.Size != request.Artifact.Size {
		t.Fatalf("DecodePluginReconcileCommand() = %+v, %v", decoded, err)
	}
	command.Payload = json.RawMessage(`{"plugin_id":"io.relayward.test","generation":1,"desired_state":"absent","unexpected":true}`)
	if _, err := DecodePluginReconcileCommand(command); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodePluginReconcileCommand() unknown field error = %v", err)
	}
}

func TestPluginReconcileAbsentState(t *testing.T) {
	value := PluginReconcileCommand{PluginID: "io.relayward.test", Generation: 2, DesiredState: PluginStateAbsent}
	if err := ValidatePluginReconcileCommand(value); err != nil {
		t.Fatalf("ValidatePluginReconcileCommand() absent error = %v", err)
	}
	value.Version = "1.0.0"
	if err := ValidatePluginReconcileCommand(value); err == nil {
		t.Fatal("ValidatePluginReconcileCommand() accepted an absent plugin version")
	}
}

func TestPluginReconcileRejectsUnsafeArtifactAndConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PluginReconcileCommand)
	}{
		{name: "HTTP URL", mutate: func(value *PluginReconcileCommand) { value.Artifact.DownloadURL = "http://github.com/plugin" }},
		{name: "URL credentials", mutate: func(value *PluginReconcileCommand) { value.Artifact.DownloadURL = "https://token@github.com/plugin" }},
		{name: "oversized artifact", mutate: func(value *PluginReconcileCommand) { value.Artifact.Size = MaximumPluginArtifactBytes + 1 }},
		{name: "invalid digest", mutate: func(value *PluginReconcileCommand) { value.Artifact.SHA256 = "invalid" }},
		{name: "non-object configuration", mutate: func(value *PluginReconcileCommand) { value.Configuration = json.RawMessage(`[]`) }},
		{name: "invalid plugin ID", mutate: func(value *PluginReconcileCommand) { value.PluginID = "Invalid" }},
		{name: "zero generation", mutate: func(value *PluginReconcileCommand) { value.Generation = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := testPluginReconcileCommand()
			test.mutate(&value)
			if err := ValidatePluginReconcileCommand(value); err == nil {
				t.Fatalf("ValidatePluginReconcileCommand() accepted %+v", value)
			}
		})
	}
}

func TestPluginConfigurationDigestCompactsWhitespace(t *testing.T) {
	first, err := PluginConfigurationDigest(json.RawMessage(`{"port":443,"enabled":true}`))
	if err != nil {
		t.Fatalf("PluginConfigurationDigest() first error = %v", err)
	}
	second, err := PluginConfigurationDigest(json.RawMessage("{\n  \"port\": 443, \"enabled\": true\n}"))
	if err != nil || second != first {
		t.Fatalf("PluginConfigurationDigest() = %q, %v; want %q", second, err, first)
	}
}

func TestPluginReconcileOutputAndStatusEvent(t *testing.T) {
	digest := strings.Repeat("a", 64)
	output := PluginReconcileOutput{
		PluginID: "io.relayward.test", Generation: 3, State: PluginStateRunning,
		Version: "1.2.3", ConfigurationSHA256: digest,
	}
	raw, err := EncodePluginReconcileOutput(output)
	if err != nil {
		t.Fatalf("EncodePluginReconcileOutput() error = %v", err)
	}
	decoded, err := DecodePluginReconcileOutput(raw)
	if err != nil || decoded != output {
		t.Fatalf("DecodePluginReconcileOutput() = %+v, %v", decoded, err)
	}
	status := PluginStatusEvent{
		PluginID: output.PluginID, Generation: output.Generation, State: output.State,
		Version: output.Version, ConfigurationSHA256: digest, Health: PluginHealthHealthy,
	}
	if err := ValidatePluginStatusEvent(status); err != nil {
		t.Fatalf("ValidatePluginStatusEvent() running error = %v", err)
	}
	status.State = PluginStateFailed
	status.Health = PluginHealthUnhealthy
	status.Reason = "process exited unexpectedly"
	if err := ValidatePluginStatusEvent(status); err != nil {
		t.Fatalf("ValidatePluginStatusEvent() failed error = %v", err)
	}
	status.Reason = ""
	if err := ValidatePluginStatusEvent(status); err == nil {
		t.Fatal("ValidatePluginStatusEvent() accepted a failed plugin without a reason")
	}
}

func testPluginReconcileCommand() PluginReconcileCommand {
	return PluginReconcileCommand{
		PluginID: "io.relayward.test", Generation: 1, DesiredState: PluginStateRunning,
		Version: "1.2.3",
		Artifact: &PluginArtifact{
			DownloadURL: "https://github.com/Relayward/test/releases/download/v1.2.3/plugin-linux-amd64",
			Size:        1024, SHA256: strings.Repeat("a", 64),
		},
		Configuration: json.RawMessage(`{"port":443}`),
	}
}

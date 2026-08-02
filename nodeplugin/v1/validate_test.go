package nodepluginv1

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	agentv1 "github.com/Relayward/relayward-sdk/agent/v1"
	"github.com/Relayward/relayward-sdk/contract"
)

func TestValidateInfoResponse(t *testing.T) {
	value := &GetInfoResponse{ApiVersion: contract.NodePluginAPIVersion, PluginId: "io.relayward.test", Version: "1.2.3", Capabilities: []string{CapabilityServiceControl, CapabilityTrafficCounters}}
	if err := ValidateInfoResponse(value, value.PluginId, value.Version); err != nil {
		t.Fatalf("ValidateInfoResponse() error = %v", err)
	}
	if err := ValidateInfoResponse(value, "io.relayward.other", value.Version); err == nil {
		t.Fatal("ValidateInfoResponse() accepted a different plugin ID")
	}
	value.Capabilities = []string{CapabilityRecentActivity, CapabilityServiceControl, CapabilityTrafficCounters}
	if err := ValidateInfoResponse(value, value.PluginId, value.Version); err == nil {
		t.Fatal("ValidateInfoResponse() accepted activity capability without a stream ID")
	}
	value.TelemetryStreamId = "0123456789abcdef0123456789abcdef"
	if err := ValidateInfoResponse(value, value.PluginId, value.Version); err != nil {
		t.Fatalf("ValidateInfoResponse() stream error = %v", err)
	}
}

func TestValidateTelemetryAndEnforcement(t *testing.T) {
	authorizationID := "123e4567-e89b-42d3-a456-426614174000"
	request := &CollectTelemetryRequest{MaximumEvents: 2}
	response := &CollectTelemetryResponse{
		ObservedAtUnixNano: time.Now().UnixNano(), NextSequence: 1,
		Counters: []*TrafficCounter{{AuthorizationId: authorizationID, ServiceId: "main", CounterEpoch: "boot-1", UploadBytes: 10}},
		Events: []*AccessEvent{{Sequence: 1, EventId: "access-1", ObservedAtUnixNano: time.Now().UnixNano(),
			AuthorizationId: authorizationID, ServiceId: "main", SourceIp: "192.0.2.1", Destination: "example.com",
			DestinationPort: 443, Network: "tcp", Protocol: "tls", Action: agentv1.AccessActionAccepted}},
	}
	if err := ValidateCollectTelemetryResponse(request, response); err != nil {
		t.Fatalf("ValidateCollectTelemetryResponse() error = %v", err)
	}
	response.Events[0].Sequence = 2
	if err := ValidateCollectTelemetryResponse(request, response); err == nil || !strings.Contains(err.Error(), "contiguous") {
		t.Fatalf("telemetry gap error = %v", err)
	}

	state := &SetServiceStateRequest{PolicyGeneration: 3, StateRevision: 1, AuthorizationId: authorizationID, ServiceId: "main", Enabled: false,
		Reason: ServiceStateReason_SERVICE_STATE_REASON_QUOTA_EXCEEDED}
	stateResponse := &SetServiceStateResponse{PolicyGeneration: state.PolicyGeneration, StateRevision: state.StateRevision, AuthorizationId: state.AuthorizationId,
		ServiceId: state.ServiceId, Enabled: state.Enabled, Reason: state.Reason}
	if err := ValidateSetServiceStateResponse(state, stateResponse); err != nil {
		t.Fatalf("ValidateSetServiceStateResponse() error = %v", err)
	}
	blocks := &ReplaceDynamicBlocksRequest{PolicyGeneration: 3, BlockRevision: 1, Blocks: []*DynamicBlock{{
		AuthorizationId: authorizationID, ServiceId: "main", SourceIp: "192.0.2.2", ExpiresAtUnixNano: time.Now().Add(time.Minute).UnixNano(),
	}}}
	if err := ValidateReplaceDynamicBlocksResponse(blocks, &ReplaceDynamicBlocksResponse{PolicyGeneration: 3, BlockRevision: 1, BlockCount: 1}); err != nil {
		t.Fatalf("ValidateReplaceDynamicBlocksResponse() error = %v", err)
	}
}

func TestValidateConfigurationRoundTrip(t *testing.T) {
	raw := json.RawMessage(`{"port":443}`)
	digest, err := agentv1.PluginConfigurationDigest(raw)
	if err != nil {
		t.Fatalf("PluginConfigurationDigest() error = %v", err)
	}
	request := &ConfigurationRequest{Generation: 7, Sha256: digest, Json: raw}
	if err := ValidateConfigurationRequest(request); err != nil {
		t.Fatalf("ValidateConfigurationRequest() error = %v", err)
	}
	if err := ValidateConfigurationValidated(request, &ConfigurationValidated{Generation: 7, Sha256: digest}); err != nil {
		t.Fatalf("ValidateConfigurationValidated() error = %v", err)
	}
	if err := ValidateConfigurationApplied(request, &ConfigurationApplied{Generation: 7, Sha256: digest}); err != nil {
		t.Fatalf("ValidateConfigurationApplied() error = %v", err)
	}
	request.Json = json.RawMessage(`{"port":80}`)
	if err := ValidateConfigurationRequest(request); err == nil {
		t.Fatal("ValidateConfigurationRequest() accepted a stale digest")
	}
}

func TestValidateStatusResponse(t *testing.T) {
	digest, err := agentv1.PluginConfigurationDigest(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("PluginConfigurationDigest() error = %v", err)
	}
	value := &GetStatusResponse{Generation: 1, ConfigurationSha256: digest, Health: Health_HEALTH_HEALTHY}
	if err := ValidateStatusResponse(value); err != nil {
		t.Fatalf("ValidateStatusResponse() error = %v", err)
	}
	value.Generation = 0
	value.ConfigurationSha256 = ""
	if err := ValidateStatusResponse(value); err == nil {
		t.Fatal("ValidateStatusResponse() accepted healthy state without configuration")
	}
	value.Health = Health_HEALTH_STARTING
	if err := ValidateStatusResponse(value); err != nil {
		t.Fatalf("ValidateStatusResponse() starting error = %v", err)
	}
}

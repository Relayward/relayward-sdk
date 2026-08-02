package nodepluginv1

import (
	"encoding/json"
	"testing"

	agentv1 "github.com/Relayward/relayward-sdk/agent/v1"
	"github.com/Relayward/relayward-sdk/contract"
)

func TestValidateInfoResponse(t *testing.T) {
	value := &GetInfoResponse{ApiVersion: contract.NodePluginAPIVersion, PluginId: "io.relayward.test", Version: "1.2.3"}
	if err := ValidateInfoResponse(value, value.PluginId, value.Version); err != nil {
		t.Fatalf("ValidateInfoResponse() error = %v", err)
	}
	if err := ValidateInfoResponse(value, "io.relayward.other", value.Version); err == nil {
		t.Fatal("ValidateInfoResponse() accepted a different plugin ID")
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

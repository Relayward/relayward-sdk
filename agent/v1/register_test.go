package agentv1

import (
	"strings"
	"testing"
)

const testRegistrationToken = "rwr_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const testCredential = "rwc_BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"

func TestDecodeRegisterRequest(t *testing.T) {
	input := `{
  "api_version":"relayward.agent/v1",
  "token":"` + testRegistrationToken + `",
  "agent_version":"0.1.0",
  "hostname":"edge-one",
  "os":"linux",
  "arch":"amd64",
  "capabilities":["control.heartbeat"]
}`
	value, err := DecodeRegisterRequest(strings.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeRegisterRequest() error = %v", err)
	}
	if value.Hostname != "edge-one" || value.Token != testRegistrationToken {
		t.Fatalf("registration request = %+v", value)
	}
}

func TestDecodeRegisterRequestRejectsUnknownField(t *testing.T) {
	input := `{
  "api_version":"relayward.agent/v1",
  "token":"` + testRegistrationToken + `",
  "agent_version":"0.1.0",
  "hostname":"edge-one",
  "os":"linux",
  "arch":"amd64",
  "capabilities":[],
  "other":true
}`
	if _, err := DecodeRegisterRequest(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodeRegisterRequest() error = %v, want unknown field", err)
	}
}

func TestValidateRegisterRequestRejectsUnsupportedPlatformAndUnsortedCapabilities(t *testing.T) {
	base := RegisterRequest{
		APIVersion: APIVersion, Token: testRegistrationToken, AgentVersion: "dev",
		Hostname: "edge-one", OS: "linux", Arch: "amd64",
	}
	base.Arch = "arm64"
	if err := ValidateRegisterRequest(base); err == nil || !strings.Contains(err.Error(), "arch") {
		t.Fatalf("ValidateRegisterRequest() architecture error = %v", err)
	}
	base.Arch = "amd64"
	base.Capabilities = []string{"plugin.supervision", "control.heartbeat"}
	if err := ValidateRegisterRequest(base); err == nil || !strings.Contains(err.Error(), "sorted") {
		t.Fatalf("ValidateRegisterRequest() capabilities error = %v", err)
	}
}

func TestValidateRegisterResponse(t *testing.T) {
	value := RegisterResponse{
		APIVersion: APIVersion,
		NodeID:     "123e4567-e89b-42d3-a456-426614174000",
		NodeName:   "Edge one",
		Credential: testCredential,
	}
	if err := ValidateRegisterResponse(value); err != nil {
		t.Fatalf("ValidateRegisterResponse() error = %v", err)
	}
	value.Credential = testRegistrationToken
	if err := ValidateRegisterResponse(value); err == nil {
		t.Fatal("ValidateRegisterResponse() accepted a registration token as a credential")
	}
}

func TestValidateRegisterResponseCountsDisplayNameRunes(t *testing.T) {
	value := RegisterResponse{
		APIVersion: APIVersion,
		NodeID:     "123e4567-e89b-42d3-a456-426614174000",
		NodeName:   strings.Repeat("界", 100),
		Credential: testCredential,
	}
	if err := ValidateRegisterResponse(value); err != nil {
		t.Fatalf("ValidateRegisterResponse() Unicode name error = %v", err)
	}
	value.NodeName += "界"
	if err := ValidateRegisterResponse(value); err == nil {
		t.Fatal("ValidateRegisterResponse() accepted a 101-character name")
	}
}

// Package agentv1 defines the Relayward center-to-Agent control contract.
package agentv1

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Relayward/relayward-sdk/contract"
)

const APIVersion = contract.AgentAPIVersion

type RegisterRequest struct {
	APIVersion   string   `json:"api_version"`
	Token        string   `json:"token"`
	AgentVersion string   `json:"agent_version"`
	Hostname     string   `json:"hostname"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	Capabilities []string `json:"capabilities"`
}

type RegisterResponse struct {
	APIVersion string `json:"api_version"`
	NodeID     string `json:"node_id"`
	NodeName   string `json:"node_name"`
	Credential string `json:"credential"`
}

func DecodeRegisterRequest(reader io.Reader) (RegisterRequest, error) {
	var value RegisterRequest
	if err := decodeStrict(reader, &value); err != nil {
		return RegisterRequest{}, fmt.Errorf("decode Agent registration request: %w", err)
	}
	if err := ValidateRegisterRequest(value); err != nil {
		return RegisterRequest{}, err
	}
	return value, nil
}

func ValidateRegisterRequest(value RegisterRequest) error {
	if value.APIVersion != APIVersion {
		return fmt.Errorf("api_version: unsupported value %q", value.APIVersion)
	}
	if err := ValidateRegistrationToken(value.Token); err != nil {
		return err
	}
	if err := validateAgentVersion(value.AgentVersion); err != nil {
		return err
	}
	if err := validateHostname(value.Hostname); err != nil {
		return err
	}
	if value.OS != "linux" {
		return fmt.Errorf("os: unsupported value %q", value.OS)
	}
	if value.Arch != "amd64" {
		return fmt.Errorf("arch: unsupported value %q", value.Arch)
	}
	return validateCapabilities(value.Capabilities)
}

func ValidateRegisterResponse(value RegisterResponse) error {
	if value.APIVersion != APIVersion {
		return fmt.Errorf("api_version: unsupported value %q", value.APIVersion)
	}
	if err := ValidateNodeID(value.NodeID); err != nil {
		return err
	}
	if err := validateDisplayName("node_name", value.NodeName, 100); err != nil {
		return err
	}
	return ValidateCredential(value.Credential)
}

func ValidateRegistrationToken(value string) error {
	if !validSecret(value, "rwr_") {
		return fmt.Errorf("token: invalid registration token")
	}
	return nil
}

func ValidateCredential(value string) error {
	if !validSecret(value, "rwc_") {
		return fmt.Errorf("credential: invalid node credential")
	}
	return nil
}

func ValidateNodeID(value string) error {
	if !nodeIDPattern.MatchString(value) {
		return fmt.Errorf("node_id: invalid node ID")
	}
	return nil
}

func decodeStrict(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

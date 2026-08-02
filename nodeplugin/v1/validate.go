// Package nodepluginv1 defines the Unix-socket gRPC contract between the Relayward Agent and a node plugin.
package nodepluginv1

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	agentv1 "github.com/Relayward/relayward-sdk/agent/v1"
	"github.com/Relayward/relayward-sdk/contract"
)

const (
	EnvironmentSocketPath    = "RELAYWARD_NODE_PLUGIN_SOCKET"
	EnvironmentDataDirectory = "RELAYWARD_NODE_PLUGIN_DATA_DIR"
	EnvironmentPluginID      = "RELAYWARD_NODE_PLUGIN_ID"

	MaximumStatusMessageBytes = 512
)

func ValidateInfoResponse(value *GetInfoResponse, expectedPluginID, expectedVersion string) error {
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if value.ApiVersion != contract.NodePluginAPIVersion {
		return fmt.Errorf("api_version: unsupported value %q", value.ApiVersion)
	}
	if err := contract.ValidatePluginID(value.PluginId); err != nil {
		return fmt.Errorf("plugin_id: %w", err)
	}
	if err := contract.ValidateSemanticVersion(value.Version); err != nil {
		return fmt.Errorf("version: %w", err)
	}
	if expectedPluginID != "" && value.PluginId != expectedPluginID {
		return fmt.Errorf("plugin_id: reported %q instead of %q", value.PluginId, expectedPluginID)
	}
	if expectedVersion != "" && value.Version != expectedVersion {
		return fmt.Errorf("version: reported %q instead of %q", value.Version, expectedVersion)
	}
	return nil
}

func ValidateConfigurationRequest(value *ConfigurationRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if value.Generation == 0 || value.Generation > math.MaxInt64 {
		return fmt.Errorf("generation: must be between 1 and %d", int64(math.MaxInt64))
	}
	digest, err := agentv1.PluginConfigurationDigest(value.Json)
	if err != nil {
		return fmt.Errorf("json: %w", err)
	}
	if value.Sha256 != digest {
		return fmt.Errorf("sha256: does not match the configuration")
	}
	return nil
}

func ValidateConfigurationValidated(request *ConfigurationRequest, response *ConfigurationValidated) error {
	if err := ValidateConfigurationRequest(request); err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("response: required")
	}
	if response.Generation != request.Generation || response.Sha256 != request.Sha256 {
		return fmt.Errorf("response: generation and SHA-256 must match the request")
	}
	return nil
}

func ValidateConfigurationApplied(request *ConfigurationRequest, response *ConfigurationApplied) error {
	if err := ValidateConfigurationRequest(request); err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("response: required")
	}
	if response.Generation != request.Generation || response.Sha256 != request.Sha256 {
		return fmt.Errorf("response: generation and SHA-256 must match the request")
	}
	return nil
}

func ValidateStatusResponse(value *GetStatusResponse) error {
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if value.Generation == 0 {
		if value.ConfigurationSha256 != "" {
			return fmt.Errorf("configuration_sha256: must be absent before a configuration is applied")
		}
	} else {
		if value.Generation > math.MaxInt64 {
			return fmt.Errorf("generation: must not exceed %d", int64(math.MaxInt64))
		}
		if len(value.ConfigurationSha256) != 64 {
			return fmt.Errorf("configuration_sha256: invalid SHA-256 digest")
		}
		for _, character := range value.ConfigurationSha256 {
			if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
				return fmt.Errorf("configuration_sha256: invalid SHA-256 digest")
			}
		}
	}
	switch value.Health {
	case Health_HEALTH_STARTING, Health_HEALTH_HEALTHY, Health_HEALTH_UNHEALTHY:
	default:
		return fmt.Errorf("health: unsupported value %q", value.Health)
	}
	if value.Health == Health_HEALTH_HEALTHY && value.Generation == 0 {
		return fmt.Errorf("health: a plugin without applied configuration cannot be healthy")
	}
	if len(value.Message) > MaximumStatusMessageBytes || value.Message != strings.TrimSpace(value.Message) {
		return fmt.Errorf("message: must contain at most %d trimmed bytes", MaximumStatusMessageBytes)
	}
	for _, character := range value.Message {
		if unicode.IsControl(character) {
			return fmt.Errorf("message: must not contain control characters")
		}
	}
	return nil
}

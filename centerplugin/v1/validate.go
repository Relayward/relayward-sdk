// Package centerpluginv1 defines the Unix-socket gRPC contract between the Relayward center and a center plugin.
package centerpluginv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Relayward/relayward-sdk/contract"
)

const (
	EnvironmentPluginSocket  = "RELAYWARD_CENTER_PLUGIN_SOCKET"
	EnvironmentHostSocket    = "RELAYWARD_CENTER_HOST_SOCKET"
	EnvironmentDataDirectory = "RELAYWARD_CENTER_PLUGIN_DATA_DIR"
	EnvironmentPluginID      = "RELAYWARD_CENTER_PLUGIN_ID"

	PermissionNodesRead = "core.nodes.read"

	MaximumStatusMessageBytes = 512
	MaximumUIJSONBytes        = 512 << 10
)

var uiMethodPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func ValidateInfoResponse(value *GetInfoResponse, expectedPluginID, expectedVersion string) error {
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if value.ApiVersion != contract.CenterPluginAPIVersion {
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

func ValidateActivateRequest(value *ActivateRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	return validatePermissions(value.Permissions)
}

func ValidateActivated(request *ActivateRequest, response *Activated) error {
	if err := ValidateActivateRequest(request); err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("response: required")
	}
	if err := validatePermissions(response.Permissions); err != nil {
		return fmt.Errorf("response: %w", err)
	}
	if len(request.Permissions) != len(response.Permissions) {
		return fmt.Errorf("response: permissions do not match the request")
	}
	for index := range request.Permissions {
		if request.Permissions[index] != response.Permissions[index] {
			return fmt.Errorf("response: permissions do not match the request")
		}
	}
	return nil
}

func ValidateStatusResponse(value *GetStatusResponse) error {
	if value == nil {
		return fmt.Errorf("response: required")
	}
	switch value.Health {
	case Health_HEALTH_STARTING, Health_HEALTH_HEALTHY, Health_HEALTH_UNHEALTHY:
	default:
		return fmt.Errorf("health: unsupported value %q", value.Health)
	}
	return validateDiagnostic("message", value.Message, MaximumStatusMessageBytes)
}

func ValidateInvokeUIRequest(value *InvokeUIRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if len(value.Method) > 128 || !uiMethodPattern.MatchString(value.Method) {
		return fmt.Errorf("method: invalid plugin UI method")
	}
	if err := validateJSONObject(value.Json); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	return nil
}

func ValidateInvokeUIResponse(value *InvokeUIResponse) error {
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if len(value.Json) == 0 || len(value.Json) > MaximumUIJSONBytes || !json.Valid(value.Json) {
		return fmt.Errorf("json: must contain at most %d bytes of valid JSON", MaximumUIJSONBytes)
	}
	return nil
}

func ValidateListNodesResponse(value *ListNodesResponse) error {
	if value == nil {
		return fmt.Errorf("response: required")
	}
	seen := make(map[string]struct{}, len(value.Nodes))
	for index, node := range value.Nodes {
		if node == nil {
			return fmt.Errorf("nodes[%d]: required", index)
		}
		if err := validateOpaqueID(node.Id); err != nil {
			return fmt.Errorf("nodes[%d].id: %w", index, err)
		}
		if _, exists := seen[node.Id]; exists {
			return fmt.Errorf("nodes[%d].id: duplicate value", index)
		}
		seen[node.Id] = struct{}{}
		if node.Name != strings.TrimSpace(node.Name) || node.Name == "" || len(node.Name) > 80 {
			return fmt.Errorf("nodes[%d].name: must contain 1 to 80 trimmed bytes", index)
		}
	}
	return nil
}

func validatePermissions(values []string) error {
	if !sort.StringsAreSorted(values) {
		return fmt.Errorf("permissions: must be sorted")
	}
	for index, value := range values {
		if value != PermissionNodesRead {
			return fmt.Errorf("permissions[%d]: unsupported permission %q", index, value)
		}
		if index > 0 && value == values[index-1] {
			return fmt.Errorf("permissions[%d]: duplicate permission %q", index, value)
		}
	}
	return nil
}

func validateJSONObject(raw []byte) error {
	if len(raw) == 0 || len(raw) > MaximumUIJSONBytes || !json.Valid(raw) {
		return fmt.Errorf("must contain at most %d bytes of valid JSON", MaximumUIJSONBytes)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return fmt.Errorf("must be a JSON object")
	}
	return nil
}

func validateDiagnostic(field, value string, maximum int) error {
	if len(value) > maximum || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s: must contain at most %d trimmed bytes", field, maximum)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%s: must not contain control characters", field)
		}
	}
	return nil
}

func validateOpaqueID(value string) error {
	if value == "" || len(value) > 128 || value != strings.TrimSpace(value) {
		return fmt.Errorf("must contain 1 to 128 trimmed bytes")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("must not contain control characters")
		}
	}
	return nil
}

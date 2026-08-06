// Package centerpluginv1 defines the Unix-socket gRPC contract between the Relayward center and a center plugin.
package centerpluginv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	agentv1 "github.com/Relayward/relayward-sdk/agent/v1"
	"github.com/Relayward/relayward-sdk/contract"
)

const (
	EnvironmentPluginSocket  = "RELAYWARD_CENTER_PLUGIN_SOCKET"
	EnvironmentHostSocket    = "RELAYWARD_CENTER_HOST_SOCKET"
	EnvironmentDataDirectory = "RELAYWARD_CENTER_PLUGIN_DATA_DIR"
	EnvironmentPluginID      = "RELAYWARD_CENTER_PLUGIN_ID"

	PermissionNodesRead     = "core.nodes.read"
	PermissionEventsRead    = "core.events.read"
	PermissionEventsWrite   = "core.events.write"
	PermissionNodeConfigure = "core.node_plugins.configure"
	PermissionServicesWrite = "core.services.write"

	MaximumStatusMessageBytes = 512
	MaximumUIJSONBytes        = 512 << 10
	MaximumServices           = 256
	MaximumFragmentsPerFormat = 32
	MaximumFragmentBytes      = 64 << 10
	MaximumSubscriptionBytes  = 512 << 10
	MaximumEventBatchEvents   = 128
	MaximumEventPayloadBytes  = 256 << 10
	MaximumEventBatchBytes    = 768 << 10
)

var (
	uiMethodPattern      = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	serviceIDPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	uuidPattern          = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	sha256Pattern        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	eventKindPattern     = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)+$`)
	sourceEventIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$`)
)

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

func ValidateGetNodePluginConfigurationRequest(value *GetNodePluginConfigurationRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if !uuidPattern.MatchString(value.NodeId) {
		return fmt.Errorf("node_id: invalid node ID")
	}
	return nil
}

func ValidateNodePluginConfiguration(request *GetNodePluginConfigurationRequest, value *NodePluginConfiguration) error {
	if err := ValidateGetNodePluginConfigurationRequest(request); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if value.Generation == 0 || value.Generation > math.MaxInt64 {
		return fmt.Errorf("generation: must be between 1 and %d", int64(math.MaxInt64))
	}
	if err := contract.ValidateSemanticVersion(value.Version); err != nil {
		return fmt.Errorf("version: %w", err)
	}
	digest, err := pluginConfigurationDigest(value.Json)
	if err != nil {
		return fmt.Errorf("json: %w", err)
	}
	if value.Sha256 != digest {
		return fmt.Errorf("sha256: does not match the configuration")
	}
	return nil
}

func ValidateConfigureNodePluginRequest(value *ConfigureNodePluginRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if !uuidPattern.MatchString(value.NodeId) {
		return fmt.Errorf("node_id: invalid node ID")
	}
	if value.ExpectedGeneration >= math.MaxInt64 {
		return fmt.Errorf("expected_generation: must be less than %d", int64(math.MaxInt64))
	}
	if _, err := pluginConfigurationDigest(value.Json); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	return nil
}

func ValidateNodePluginConfigured(request *ConfigureNodePluginRequest, value *NodePluginConfigured) error {
	if err := ValidateConfigureNodePluginRequest(request); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if value.Generation != request.ExpectedGeneration+1 {
		return fmt.Errorf("generation: must advance the expected generation by one")
	}
	digest, err := pluginConfigurationDigest(request.Json)
	if err != nil {
		return fmt.Errorf("json: %w", err)
	}
	if value.Sha256 != digest {
		return fmt.Errorf("sha256: does not match the configuration")
	}
	return nil
}

func ValidateReplaceServicesRequest(value *ReplaceServicesRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if !uuidPattern.MatchString(value.NodeId) {
		return fmt.Errorf("node_id: invalid node ID")
	}
	if len(value.Services) > MaximumServices {
		return fmt.Errorf("services: must contain at most %d values", MaximumServices)
	}
	previous := ""
	for index, service := range value.Services {
		if service == nil {
			return fmt.Errorf("services[%d]: required", index)
		}
		if !serviceIDPattern.MatchString(service.Id) {
			return fmt.Errorf("services[%d].id: invalid service ID", index)
		}
		if index > 0 && service.Id <= previous {
			return fmt.Errorf("services: values must be sorted by ID and unique")
		}
		previous = service.Id
		if err := validateDisplayName(fmt.Sprintf("services[%d].display_name", index), service.DisplayName); err != nil {
			return err
		}
		if err := validateCapabilities(fmt.Sprintf("services[%d].capabilities", index), service.Capabilities); err != nil {
			return err
		}
		if !sha256Pattern.MatchString(service.SubscriptionSha256) {
			return fmt.Errorf("services[%d].subscription_sha256: must be a lowercase SHA-256 digest", index)
		}
	}
	return nil
}

func ValidateServicesReplaced(request *ReplaceServicesRequest, response *ServicesReplaced) error {
	if err := ValidateReplaceServicesRequest(request); err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("response: required")
	}
	if int(response.ServiceCount) != len(request.Services) {
		return fmt.Errorf("service_count: does not match the request")
	}
	return nil
}

func ValidatePublishEventsRequest(value *PublishEventsRequest, pluginID string) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if err := contract.ValidatePluginID(pluginID); err != nil {
		return fmt.Errorf("plugin_id: %w", err)
	}
	if len(value.Events) == 0 || len(value.Events) > MaximumEventBatchEvents {
		return fmt.Errorf("events: must contain 1 to %d values", MaximumEventBatchEvents)
	}
	previous := ""
	totalBytes := 0
	for index, event := range value.Events {
		if event == nil {
			return fmt.Errorf("events[%d]: required", index)
		}
		if !sourceEventIDPattern.MatchString(event.SourceEventId) {
			return fmt.Errorf("events[%d].source_event_id: invalid source event ID", index)
		}
		if index > 0 && event.SourceEventId <= previous {
			return fmt.Errorf("events: source event IDs must be sorted and unique")
		}
		previous = event.SourceEventId
		if !uuidPattern.MatchString(event.NodeId) {
			return fmt.Errorf("events[%d].node_id: invalid node ID", index)
		}
		if len(event.Kind) > 128 || !eventKindPattern.MatchString(event.Kind) {
			return fmt.Errorf("events[%d].kind: invalid event kind", index)
		}
		if event.Kind != EventNotificationRequest && !strings.HasPrefix(event.Kind, "plugin."+pluginID+".") {
			return fmt.Errorf("events[%d].kind: must use the publisher namespace", index)
		}
		if event.ObservedAtUnixNano == 0 {
			return fmt.Errorf("events[%d].observed_at_unix_nano: must be set", index)
		}
		if len(event.Json) == 0 || len(event.Json) > MaximumEventPayloadBytes || !json.Valid(event.Json) {
			return fmt.Errorf("events[%d].json: must contain at most %d bytes of valid JSON", index, MaximumEventPayloadBytes)
		}
		if event.Kind == EventNotificationRequest {
			if _, err := DecodeNotificationRequest(event.Json); err != nil {
				return fmt.Errorf("events[%d].json: %w", index, err)
			}
		}
		totalBytes += len(event.Json)
		if totalBytes > MaximumEventBatchBytes {
			return fmt.Errorf("events: payloads exceed %d bytes", MaximumEventBatchBytes)
		}
	}
	return nil
}

func ValidateEventsPublished(request *PublishEventsRequest, pluginID string, response *EventsPublished) error {
	if err := ValidatePublishEventsRequest(request, pluginID); err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("response: required")
	}
	if int(response.EventCount) != len(request.Events) {
		return fmt.Errorf("event_count: does not match the request")
	}
	return nil
}

func ValidateRenderSubscriptionRequest(value *RenderSubscriptionRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if !uuidPattern.MatchString(value.AuthorizationId) {
		return fmt.Errorf("authorization_id: invalid authorization ID")
	}
	if !uuidPattern.MatchString(value.NodeId) {
		return fmt.Errorf("node_id: invalid node ID")
	}
	if err := validateOptionalText("public_address", value.PublicAddress, 255); err != nil {
		return err
	}
	if len(value.Services) == 0 || len(value.Services) > MaximumServices {
		return fmt.Errorf("services: must contain 1 to %d values", MaximumServices)
	}
	previous := ""
	for index, service := range value.Services {
		if service == nil {
			return fmt.Errorf("services[%d]: required", index)
		}
		if !serviceIDPattern.MatchString(service.ServiceId) {
			return fmt.Errorf("services[%d].service_id: invalid service ID", index)
		}
		if index > 0 && service.ServiceId <= previous {
			return fmt.Errorf("services: values must be sorted by service ID and unique")
		}
		previous = service.ServiceId
		if err := validateDisplayName(fmt.Sprintf("services[%d].display_name", index), service.DisplayName); err != nil {
			return err
		}
	}
	return nil
}

func ValidateRenderSubscriptionResponse(request *RenderSubscriptionRequest, response *RenderSubscriptionResponse) error {
	if err := ValidateRenderSubscriptionRequest(request); err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("response: required")
	}
	if len(response.Services) != len(request.Services) {
		return fmt.Errorf("services: must contain one contribution for every requested service")
	}
	totalBytes := 0
	for index, contribution := range response.Services {
		if contribution == nil {
			return fmt.Errorf("services[%d]: required", index)
		}
		if contribution.ServiceId != request.Services[index].ServiceId {
			return fmt.Errorf("services[%d].service_id: does not match the request", index)
		}
		if err := validateDisplayName(fmt.Sprintf("services[%d].display_name", index), contribution.DisplayName); err != nil {
			return err
		}
		if len(contribution.Uris)+len(contribution.MihomoProxiesJson)+len(contribution.SingBoxOutboundsJson) == 0 {
			return fmt.Errorf("services[%d]: must contribute at least one subscription fragment", index)
		}
		if len(contribution.Uris) > MaximumFragmentsPerFormat || len(contribution.MihomoProxiesJson) > MaximumFragmentsPerFormat || len(contribution.SingBoxOutboundsJson) > MaximumFragmentsPerFormat {
			return fmt.Errorf("services[%d]: each format must contain at most %d fragments", index, MaximumFragmentsPerFormat)
		}
		previousURI := ""
		for uriIndex, raw := range contribution.Uris {
			if raw <= previousURI && uriIndex > 0 {
				return fmt.Errorf("services[%d].uris: values must be sorted and unique", index)
			}
			previousURI = raw
			if err := validateSubscriptionURI(raw); err != nil {
				return fmt.Errorf("services[%d].uris[%d]: %w", index, uriIndex, err)
			}
			totalBytes += len(raw)
		}
		for _, fragments := range []struct {
			name   string
			values [][]byte
		}{{"mihomo_proxies_json", contribution.MihomoProxiesJson}, {"sing_box_outbounds_json", contribution.SingBoxOutboundsJson}} {
			for fragmentIndex, raw := range fragments.values {
				if len(raw) > MaximumFragmentBytes {
					return fmt.Errorf("services[%d].%s[%d]: must contain at most %d bytes", index, fragments.name, fragmentIndex, MaximumFragmentBytes)
				}
				if err := validateJSONObject(raw); err != nil {
					return fmt.Errorf("services[%d].%s[%d]: %w", index, fragments.name, fragmentIndex, err)
				}
				totalBytes += len(raw)
			}
		}
		if totalBytes > MaximumSubscriptionBytes {
			return fmt.Errorf("response: subscription fragments exceed %d bytes", MaximumSubscriptionBytes)
		}
	}
	return nil
}

func ValidateConsumeEventsRequest(value *ConsumeEventsRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if len(value.Events) == 0 || len(value.Events) > MaximumEventBatchEvents {
		return fmt.Errorf("events: must contain 1 to %d values", MaximumEventBatchEvents)
	}
	var previous uint64
	totalBytes := 0
	for index, event := range value.Events {
		if event == nil {
			return fmt.Errorf("events[%d]: required", index)
		}
		if event.Cursor == 0 || event.Cursor > uint64(^uint64(0)>>1) {
			return fmt.Errorf("events[%d].cursor: must be between 1 and the maximum signed 64-bit integer", index)
		}
		if index > 0 && event.Cursor <= previous {
			return fmt.Errorf("events: cursors must be strictly increasing")
		}
		previous = event.Cursor
		if !sha256Pattern.MatchString(event.EventId) {
			return fmt.Errorf("events[%d].event_id: invalid event ID", index)
		}
		if !uuidPattern.MatchString(event.NodeId) {
			return fmt.Errorf("events[%d].node_id: invalid node ID", index)
		}
		if len(event.Kind) > 128 || !eventKindPattern.MatchString(event.Kind) {
			return fmt.Errorf("events[%d].kind: invalid event kind", index)
		}
		if event.ObservedAtUnixNano == 0 {
			return fmt.Errorf("events[%d].observed_at_unix_nano: must be set", index)
		}
		if event.ReceivedAtUnixNano <= 0 {
			return fmt.Errorf("events[%d].received_at_unix_nano: must be positive", index)
		}
		if len(event.Json) == 0 || len(event.Json) > MaximumEventPayloadBytes || !json.Valid(event.Json) {
			return fmt.Errorf("events[%d].json: must contain at most %d bytes of valid JSON", index, MaximumEventPayloadBytes)
		}
		totalBytes += len(event.Json)
		if totalBytes > MaximumEventBatchBytes {
			return fmt.Errorf("events: payloads exceed %d bytes", MaximumEventBatchBytes)
		}
	}
	return nil
}

func ValidateEventsConsumed(request *ConsumeEventsRequest, response *EventsConsumed) error {
	if err := ValidateConsumeEventsRequest(request); err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("response: required")
	}
	want := request.Events[len(request.Events)-1].Cursor
	if response.ThroughCursor != want {
		return fmt.Errorf("through_cursor: must acknowledge the complete batch")
	}
	return nil
}

func validatePermissions(values []string) error {
	if !sort.StringsAreSorted(values) {
		return fmt.Errorf("permissions: must be sorted")
	}
	for index, value := range values {
		if value != PermissionEventsRead && value != PermissionEventsWrite && value != PermissionNodeConfigure && value != PermissionNodesRead && value != PermissionServicesWrite {
			return fmt.Errorf("permissions[%d]: unsupported permission %q", index, value)
		}
		if index > 0 && value == values[index-1] {
			return fmt.Errorf("permissions[%d]: duplicate permission %q", index, value)
		}
	}
	return nil
}

func pluginConfigurationDigest(raw []byte) (string, error) {
	if err := agentv1.ValidatePluginConfiguration(raw); err != nil {
		return "", err
	}
	if err := validateJSONObject(raw); err != nil {
		return "", err
	}
	return agentv1.PluginConfigurationDigest(raw)
}

func validateDisplayName(field, value string) error {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 100 {
		return fmt.Errorf("%s: must contain 1 to 100 trimmed characters", field)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%s: must not contain control characters", field)
		}
	}
	return nil
}

func validateCapabilities(field string, values []string) error {
	if len(values) > 64 {
		return fmt.Errorf("%s: must contain at most 64 values", field)
	}
	previous := ""
	for index, value := range values {
		if len(value) > 64 || !uiMethodPattern.MatchString(value) {
			return fmt.Errorf("%s[%d]: invalid capability", field, index)
		}
		if index > 0 && value <= previous {
			return fmt.Errorf("%s: values must be sorted and unique", field)
		}
		previous = value
	}
	return nil
}

func validateOptionalText(field, value string, maximum int) error {
	if value != strings.TrimSpace(value) || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return fmt.Errorf("%s: must contain at most %d trimmed characters", field, maximum)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%s: must not contain control characters", field)
		}
	}
	return nil
}

func validateSubscriptionURI(value string) error {
	if value == "" || len(value) > 8192 || value != strings.TrimSpace(value) {
		return fmt.Errorf("must contain 1 to 8192 trimmed bytes")
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return fmt.Errorf("must not contain whitespace or control characters")
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("must be an absolute URI")
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

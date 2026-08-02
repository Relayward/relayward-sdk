// Package nodepluginv1 defines the Unix-socket gRPC contract between the Relayward Agent and a node plugin.
package nodepluginv1

import (
	"fmt"
	"math"
	"net"
	"regexp"
	"sort"
	"strings"
	"unicode"

	agentv1 "github.com/Relayward/relayward-sdk/agent/v1"
	"github.com/Relayward/relayward-sdk/contract"
	policyv1 "github.com/Relayward/relayward-sdk/policy/v1"
)

const (
	EnvironmentSocketPath    = "RELAYWARD_NODE_PLUGIN_SOCKET"
	EnvironmentDataDirectory = "RELAYWARD_NODE_PLUGIN_DATA_DIR"
	EnvironmentPluginID      = "RELAYWARD_NODE_PLUGIN_ID"

	MaximumStatusMessageBytes = 512
	MaximumTelemetryEvents    = 500
	MaximumTrafficCounters    = 4096
	MaximumDynamicBlocks      = 16384

	CapabilityTrafficCounters = "traffic.counters"
	CapabilityRecentActivity  = "activity.recent"
	CapabilityServiceControl  = "service.control"
	CapabilityDynamicBlocking = "blocking.dynamic"
)

var componentIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

var capabilityPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)+$`)
var telemetryStreamIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

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
	previous := ""
	for index, capability := range value.Capabilities {
		if len(capability) > 64 || !capabilityPattern.MatchString(capability) {
			return fmt.Errorf("capabilities[%d]: invalid capability", index)
		}
		if index > 0 && capability <= previous {
			return fmt.Errorf("capabilities: values must be sorted and unique")
		}
		previous = capability
	}
	hasActivity := HasCapability(value.Capabilities, CapabilityRecentActivity)
	if hasActivity && !telemetryStreamIDPattern.MatchString(value.TelemetryStreamId) {
		return fmt.Errorf("telemetry_stream_id: required for recent activity capability")
	}
	if !hasActivity && value.TelemetryStreamId != "" {
		return fmt.Errorf("telemetry_stream_id: requires recent activity capability")
	}
	return nil
}

func HasCapability(values []string, expected string) bool {
	index := sort.SearchStrings(values, expected)
	return index < len(values) && values[index] == expected
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

func ValidateCollectTelemetryRequest(value *CollectTelemetryRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if value.AfterSequence > math.MaxInt64 {
		return fmt.Errorf("after_sequence: must not exceed %d", int64(math.MaxInt64))
	}
	if value.MaximumEvents < 1 || value.MaximumEvents > MaximumTelemetryEvents {
		return fmt.Errorf("maximum_events: must be between 1 and %d", MaximumTelemetryEvents)
	}
	return nil
}

func ValidateCollectTelemetryResponse(request *CollectTelemetryRequest, value *CollectTelemetryResponse) error {
	if err := ValidateCollectTelemetryRequest(request); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if value.ObservedAtUnixNano <= 0 {
		return fmt.Errorf("observed_at_unix_nano: must be greater than zero")
	}
	if len(value.Counters) > MaximumTrafficCounters {
		return fmt.Errorf("counters: must contain at most %d values", MaximumTrafficCounters)
	}
	previous := ""
	for index, counter := range value.Counters {
		if err := validateTrafficCounter(counter); err != nil {
			return fmt.Errorf("counters[%d]: %w", index, err)
		}
		key := counter.AuthorizationId + "\x00" + counter.ServiceId
		if index > 0 && key <= previous {
			return fmt.Errorf("counters: values must be sorted by authorization_id and service_id and unique")
		}
		previous = key
	}
	if len(value.Events) > int(request.MaximumEvents) {
		return fmt.Errorf("events: exceeds requested maximum")
	}
	next := request.AfterSequence
	for index, event := range value.Events {
		if err := validateAccessEvent(event); err != nil {
			return fmt.Errorf("events[%d]: %w", index, err)
		}
		if event.Sequence != next+1 {
			return fmt.Errorf("events[%d]: sequence must be contiguous after request cursor", index)
		}
		next = event.Sequence
	}
	if value.NextSequence != next {
		return fmt.Errorf("next_sequence: must equal the last returned sequence or request cursor")
	}
	if value.HasMore && len(value.Events) != int(request.MaximumEvents) {
		return fmt.Errorf("has_more: requires a full event page")
	}
	return nil
}

func ValidateSetServiceStateRequest(value *SetServiceStateRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if value.PolicyGeneration == 0 || value.PolicyGeneration > math.MaxInt64 {
		return fmt.Errorf("policy_generation: must be between 1 and %d", int64(math.MaxInt64))
	}
	if value.StateRevision == 0 || value.StateRevision > math.MaxInt64 {
		return fmt.Errorf("state_revision: must be between 1 and %d", int64(math.MaxInt64))
	}
	if err := policyIdentifier("authorization_id", value.AuthorizationId); err != nil {
		return err
	}
	if !componentIDPattern.MatchString(value.ServiceId) {
		return fmt.Errorf("service_id: invalid service ID")
	}
	if err := validateServiceState(value.Enabled, value.Reason); err != nil {
		return err
	}
	return nil
}

func ValidateSetServiceStateResponse(request *SetServiceStateRequest, value *SetServiceStateResponse) error {
	if err := ValidateSetServiceStateRequest(request); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if value.PolicyGeneration != request.PolicyGeneration || value.StateRevision != request.StateRevision || value.AuthorizationId != request.AuthorizationId ||
		value.ServiceId != request.ServiceId || value.Enabled != request.Enabled || value.Reason != request.Reason {
		return fmt.Errorf("response: must echo the applied service state")
	}
	return nil
}

func ValidateReplaceDynamicBlocksRequest(value *ReplaceDynamicBlocksRequest) error {
	if value == nil {
		return fmt.Errorf("request: required")
	}
	if value.PolicyGeneration == 0 || value.PolicyGeneration > math.MaxInt64 {
		return fmt.Errorf("policy_generation: must be between 1 and %d", int64(math.MaxInt64))
	}
	if value.BlockRevision == 0 || value.BlockRevision > math.MaxInt64 {
		return fmt.Errorf("block_revision: must be between 1 and %d", int64(math.MaxInt64))
	}
	if len(value.Blocks) > MaximumDynamicBlocks {
		return fmt.Errorf("blocks: must contain at most %d values", MaximumDynamicBlocks)
	}
	previous := ""
	for index, block := range value.Blocks {
		if block == nil {
			return fmt.Errorf("blocks[%d]: required", index)
		}
		if err := policyIdentifier("authorization_id", block.AuthorizationId); err != nil {
			return fmt.Errorf("blocks[%d]: %w", index, err)
		}
		if !componentIDPattern.MatchString(block.ServiceId) {
			return fmt.Errorf("blocks[%d].service_id: invalid service ID", index)
		}
		ip := net.ParseIP(block.SourceIp)
		if ip == nil || ip.String() != block.SourceIp {
			return fmt.Errorf("blocks[%d].source_ip: must be a canonical IP address", index)
		}
		if block.ExpiresAtUnixNano <= 0 {
			return fmt.Errorf("blocks[%d].expires_at_unix_nano: must be greater than zero", index)
		}
		key := block.AuthorizationId + "\x00" + block.ServiceId + "\x00" + block.SourceIp
		if index > 0 && key <= previous {
			return fmt.Errorf("blocks: values must be sorted and unique")
		}
		previous = key
	}
	return nil
}

func ValidateReplaceDynamicBlocksResponse(request *ReplaceDynamicBlocksRequest, value *ReplaceDynamicBlocksResponse) error {
	if err := ValidateReplaceDynamicBlocksRequest(request); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("response: required")
	}
	if value.PolicyGeneration != request.PolicyGeneration || value.BlockRevision != request.BlockRevision || value.BlockCount != uint32(len(request.Blocks)) {
		return fmt.Errorf("response: must echo the applied block set")
	}
	return nil
}

func validateTrafficCounter(value *TrafficCounter) error {
	if value == nil {
		return fmt.Errorf("required")
	}
	if err := policyIdentifier("authorization_id", value.AuthorizationId); err != nil {
		return err
	}
	if !componentIDPattern.MatchString(value.ServiceId) {
		return fmt.Errorf("service_id: invalid service ID")
	}
	if !componentIDPattern.MatchString(value.CounterEpoch) {
		return fmt.Errorf("counter_epoch: invalid counter epoch")
	}
	return nil
}

func validateAccessEvent(value *AccessEvent) error {
	if value == nil {
		return fmt.Errorf("required")
	}
	if value.Sequence == 0 || value.Sequence > math.MaxInt64 {
		return fmt.Errorf("sequence: must be between 1 and %d", int64(math.MaxInt64))
	}
	standard := agentv1.AccessEvent{
		SourceEventID: value.EventId, ServiceID: value.ServiceId, AuthorizationID: value.AuthorizationId,
		SourceIP: value.SourceIp, Destination: value.Destination, DestinationPort: value.DestinationPort,
		Network: value.Network, Protocol: value.Protocol, Action: value.Action,
		PluginID: "io.relayward.validation",
	}
	if err := agentv1.ValidateAccessEvent(standard); err != nil {
		return err
	}
	if value.ObservedAtUnixNano <= 0 {
		return fmt.Errorf("observed_at_unix_nano: must be greater than zero")
	}
	return nil
}

func validateServiceState(enabled bool, reason ServiceStateReason) error {
	if enabled && reason != ServiceStateReason_SERVICE_STATE_REASON_ACTIVE {
		return fmt.Errorf("reason: enabled services must use active")
	}
	if !enabled {
		switch reason {
		case ServiceStateReason_SERVICE_STATE_REASON_ADMINISTRATOR_DISABLED,
			ServiceStateReason_SERVICE_STATE_REASON_EXPIRED,
			ServiceStateReason_SERVICE_STATE_REASON_QUOTA_EXCEEDED:
			return nil
		}
		return fmt.Errorf("reason: disabled services require an enforcement reason")
	}
	return nil
}

func policyIdentifier(field, value string) error {
	return policyv1.ValidateIdentifier(field, value)
}

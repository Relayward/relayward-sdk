package agentv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"regexp"
	"strings"
	"unicode"

	"github.com/Relayward/relayward-sdk/contract"
	policyv1 "github.com/Relayward/relayward-sdk/policy/v1"
)

const (
	EventTrafficSnapshot = "traffic.snapshot"
	EventAccess          = "access.observed"

	AccessActionAccepted = "accepted"
	AccessActionBlocked  = "blocked"
)

var sourceEventIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type TrafficSnapshotEvent struct {
	AuthorizationID string          `json:"authorization_id"`
	Period          policyv1.Period `json:"period"`
	Revision        uint64          `json:"revision"`
	UploadBytes     uint64          `json:"upload_bytes"`
	DownloadBytes   uint64          `json:"download_bytes"`
}

type AccessEvent struct {
	SourceStreamID  string `json:"source_stream_id"`
	SourceEventID   string `json:"source_event_id"`
	PluginID        string `json:"plugin_id"`
	ServiceID       string `json:"service_id"`
	AuthorizationID string `json:"authorization_id"`
	SourceIP        string `json:"source_ip,omitempty"`
	Destination     string `json:"destination,omitempty"`
	DestinationPort uint32 `json:"destination_port,omitempty"`
	Network         string `json:"network,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	Action          string `json:"action"`
}

func ValidateTrafficSnapshotEvent(value TrafficSnapshotEvent) error {
	if err := policyv1.ValidateIdentifier("authorization_id", value.AuthorizationID); err != nil {
		return err
	}
	if err := policyv1.ValidatePeriod(value.Period); err != nil {
		return err
	}
	if value.Revision == 0 || value.Revision > math.MaxInt64 {
		return fmt.Errorf("revision: must be between 1 and %d", int64(math.MaxInt64))
	}
	if value.UploadBytes > math.MaxInt64 || value.DownloadBytes > math.MaxInt64 {
		return fmt.Errorf("traffic bytes: must not exceed %d", int64(math.MaxInt64))
	}
	return nil
}

func DecodeTrafficSnapshotEvent(raw json.RawMessage) (TrafficSnapshotEvent, error) {
	var value TrafficSnapshotEvent
	if err := decodeStrict(bytes.NewReader(raw), &value); err != nil {
		return TrafficSnapshotEvent{}, fmt.Errorf("decode traffic snapshot event: %w", err)
	}
	if err := ValidateTrafficSnapshotEvent(value); err != nil {
		return TrafficSnapshotEvent{}, err
	}
	return value, nil
}

func ValidateAccessEvent(value AccessEvent) error {
	if !messageIDPattern.MatchString(value.SourceStreamID) {
		return fmt.Errorf("source_stream_id: invalid stream ID")
	}
	if !sourceEventIDPattern.MatchString(value.SourceEventID) {
		return fmt.Errorf("source_event_id: invalid event ID")
	}
	if err := contract.ValidatePluginID(value.PluginID); err != nil {
		return fmt.Errorf("plugin_id: %w", err)
	}
	if !componentIDPattern.MatchString(value.ServiceID) {
		return fmt.Errorf("service_id: invalid service ID")
	}
	if err := policyv1.ValidateIdentifier("authorization_id", value.AuthorizationID); err != nil {
		return err
	}
	if value.SourceIP != "" {
		parsed := net.ParseIP(value.SourceIP)
		if parsed == nil || parsed.String() != value.SourceIP {
			return fmt.Errorf("source_ip: must be a canonical IP address")
		}
	}
	if err := validateTelemetryText("destination", value.Destination, 253); err != nil {
		return err
	}
	if value.DestinationPort > 65535 {
		return fmt.Errorf("destination_port: must not exceed 65535")
	}
	if value.Network != "" && value.Network != "tcp" && value.Network != "udp" {
		return fmt.Errorf("network: must be tcp or udp")
	}
	if err := validateTelemetryText("protocol", value.Protocol, 64); err != nil {
		return err
	}
	switch value.Action {
	case AccessActionAccepted, AccessActionBlocked:
	default:
		return fmt.Errorf("action: unsupported value %q", value.Action)
	}
	return nil
}

func DecodeAccessEvent(raw json.RawMessage) (AccessEvent, error) {
	var value AccessEvent
	if err := decodeStrict(bytes.NewReader(raw), &value); err != nil {
		return AccessEvent{}, fmt.Errorf("decode access event: %w", err)
	}
	if err := ValidateAccessEvent(value); err != nil {
		return AccessEvent{}, err
	}
	return value, nil
}

func validateTelemetryText(field, value string, maximum int) error {
	if value != strings.TrimSpace(value) || len(value) > maximum {
		return fmt.Errorf("%s: must contain at most %d trimmed bytes", field, maximum)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%s: must not contain control characters", field)
		}
	}
	return nil
}
